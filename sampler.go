package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// Sampler drives the PDH counters for one Oblivion PID and emits Sample values
// on Samples until ctx is cancelled.
type Sampler struct {
	Config  Config
	PID     uint32
	Samples chan<- Sample
}

// Run binds the counters and polls until ctx is done. It does NOT close the
// Samples channel — that's the caller's responsibility (multiple Samplers may
// share one channel across the program lifetime).
func (s *Sampler) Run(ctx context.Context) {
	q, err := PdhOpenQuery()
	if err != nil {
		return
	}
	defer q.Close()

	// Bind \Process(<name>) counters. PDH requires the instance name without
	// .exe and may append #N when names collide. We use the bare name and
	// rely on the fact that we already have the PID from Watcher; if name
	// collisions become a problem in practice, future work can match on
	// \Process(*)\ID Process == s.PID.
	bareName := strings.TrimSuffix(s.Config.ProcessName, ".exe")
	procPath := func(metric string) string {
		return fmt.Sprintf(`\Process(%s)\%s`, bareName, metric)
	}

	cWorkingSetPriv, _ := q.AddEnglishCounter(procPath("Working Set - Private"))
	cWorkingSet, _ := q.AddEnglishCounter(procPath("Working Set"))
	cPageFile, _ := q.AddEnglishCounter(procPath("Page File Bytes"))
	cCPU, _ := q.AddEnglishCounter(procPath("% Processor Time"))
	cFreeRAM, _ := q.AddEnglishCounter(`\Memory\Available MBytes`)

	// Bind GPU memory counters via wildcard expansion. There may be multiple
	// instances per PID (one per physical heap); we sum them.
	pidPrefix := fmt.Sprintf("pid_%d_", s.PID)
	dedicatedPaths := s.expandGPUPaths(pidPrefix, "Dedicated Usage")
	if len(dedicatedPaths) == 0 && s.Config.FallbackToLocalUsage {
		dedicatedPaths = s.expandGPUPaths(pidPrefix, "Local Usage")
	}
	sharedPaths := s.expandGPUPaths(pidPrefix, "Shared Usage")
	if len(sharedPaths) == 0 && s.Config.FallbackToLocalUsage {
		sharedPaths = s.expandGPUPaths(pidPrefix, "Non Local Usage")
	}

	dedicatedCounters := make([]*PdhCounter, 0, len(dedicatedPaths))
	for _, p := range dedicatedPaths {
		if c, err := q.AddCounter(p); err == nil {
			dedicatedCounters = append(dedicatedCounters, c)
		}
	}
	sharedCounters := make([]*PdhCounter, 0, len(sharedPaths))
	for _, p := range sharedPaths {
		if c, err := q.AddCounter(p); err == nil {
			sharedCounters = append(sharedCounters, c)
		}
	}

	// First Collect() establishes baseline; rate counters return zero on the
	// first read.
	q.Collect()

	start := time.Now()
	t := time.NewTicker(time.Duration(s.Config.PollingIntervalMS) * time.Millisecond)
	defer t.Stop()
	numCPU := float64(runtime.NumCPU())

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}

		if err := q.Collect(); err != nil {
			continue
		}

		readSum := func(cs []*PdhCounter) float64 {
			var sum float64
			for _, c := range cs {
				if v, err := c.Value(); err == nil {
					sum += v
				}
			}
			return sum
		}
		readOne := func(c *PdhCounter, fallback *PdhCounter) float64 {
			if c != nil {
				if v, err := c.Value(); err == nil {
					return v
				}
			}
			if fallback != nil {
				if v, err := fallback.Value(); err == nil {
					return v
				}
			}
			return 0
		}

		bytesToMB := 1.0 / (1024 * 1024)
		sample := Sample{
			Timestamp:       time.Now(),
			UptimeSec:       int(time.Since(start).Seconds()),
			VRAMDedicatedMB: readSum(dedicatedCounters) * bytesToMB,
			VRAMSharedMB:    readSum(sharedCounters) * bytesToMB,
			RAMPrivateMB:    readOne(cWorkingSetPriv, cWorkingSet) * bytesToMB,
			PageFileMB:      readOne(cPageFile, nil) * bytesToMB,
			CPUPct:          readOne(cCPU, nil) / numCPU,
			SystemFreeRAMMB: readOne(cFreeRAM, nil),
		}

		select {
		case s.Samples <- sample:
		case <-ctx.Done():
			return
		}
	}
}

func (s *Sampler) expandGPUPaths(pidPrefix, metric string) []string {
	wild := fmt.Sprintf(`\GPU Process Memory(%s*)\%s`, pidPrefix, metric)
	paths, err := PdhExpandWildCardPath(wild)
	if err != nil {
		return nil
	}
	return paths
}
