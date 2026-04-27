package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSamplerEndToEndAgainstSelf(t *testing.T) {
	if testing.Short() {
		t.Skip("PDH+process integration test")
	}
	// Use the test binary itself as the "monitored process".
	exe, _ := os.Executable()
	exeName := filepath.Base(exe)

	cfg := Config{
		PollingIntervalMS:    200,
		ScanIntervalMS:       200,
		LogDir:               t.TempDir(),
		ProcessName:          exeName,
		FallbackToLocalUsage: true,
	}

	pid := uint32(os.Getpid())
	samples := make(chan Sample, 4)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s := &Sampler{
		Config:  cfg,
		PID:     pid,
		Samples: samples,
	}
	go s.Run(ctx)

	var got Sample
	select {
	case got = <-samples:
	case <-ctx.Done():
		t.Fatal("no sample in 2s")
	}

	if got.RAMPrivateMB <= 0 {
		t.Errorf("RAMPrivateMB = %v, expected > 0 for self process", got.RAMPrivateMB)
	}
	if got.SystemFreeRAMMB <= 0 {
		t.Errorf("SystemFreeRAMMB = %v, expected > 0", got.SystemFreeRAMMB)
	}
	t.Logf("self RAM private = %.0f MB, free system RAM = %.0f MB, CPU = %.1f%%, NumCPU=%d",
		got.RAMPrivateMB, got.SystemFreeRAMMB, got.CPUPct, runtime.NumCPU())
}
