package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Sample holds one polling tick of metrics.
type Sample struct {
	Timestamp       time.Time
	UptimeSec       int
	VRAMDedicatedMB float64
	VRAMSharedMB    float64
	RAMPrivateMB    float64
	PageFileMB      float64
	CPUPct          float64
	SystemFreeRAMMB float64
}

// CSVLogger writes one CSV file per session. Header written on construction;
// each Write flushes immediately so a crash doesn't lose the most recent data.
type CSVLogger struct {
	file *os.File
	bw   *bufio.Writer
}

const csvHeader = "timestamp_iso,uptime_sec,vram_dedicated_mb,vram_shared_mb,ram_private_mb,page_file_mb,cpu_pct,system_free_ram_mb\n"

// NewCSVLogger creates dir if needed and opens a new timestamped CSV.
func NewCSVLogger(dir string) (*CSVLogger, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	name := time.Now().Format("2006-01-02-150405") + ".csv"
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	bw := bufio.NewWriter(f)
	if _, err := bw.WriteString(csvHeader); err != nil {
		f.Close()
		return nil, err
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		return nil, err
	}
	return &CSVLogger{file: f, bw: bw}, nil
}

// Write appends one sample row and flushes.
func (c *CSVLogger) Write(s Sample) error {
	line := fmt.Sprintf("%s,%d,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f\n",
		s.Timestamp.UTC().Format(time.RFC3339),
		s.UptimeSec,
		s.VRAMDedicatedMB,
		s.VRAMSharedMB,
		s.RAMPrivateMB,
		s.PageFileMB,
		s.CPUPct,
		s.SystemFreeRAMMB,
	)
	if _, err := c.bw.WriteString(line); err != nil {
		return err
	}
	return c.bw.Flush()
}

// Close flushes and releases the file.
func (c *CSVLogger) Close() error {
	var flushErr error
	if c.bw != nil {
		flushErr = c.bw.Flush()
	}
	if c.file != nil {
		closeErr := c.file.Close()
		if flushErr != nil {
			return flushErr
		}
		return closeErr
	}
	return flushErr
}
