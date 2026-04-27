package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCSVLoggerHeaderAndRow(t *testing.T) {
	dir := t.TempDir()
	lg, err := NewCSVLogger(dir)
	if err != nil {
		t.Fatalf("NewCSVLogger: %v", err)
	}

	sample := Sample{
		Timestamp:       time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC),
		UptimeSec:       42,
		VRAMDedicatedMB: 1024.5,
		VRAMSharedMB:    128.0,
		RAMPrivateMB:    2048.0,
		PageFileMB:      3072.0,
		CPUPct:          37.5,
		SystemFreeRAMMB: 16384.0,
	}
	if err := lg.Write(sample); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := lg.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(dir, "*.csv"))
	if len(files) != 1 {
		t.Fatalf("expected 1 csv file, got %d", len(files))
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	wantHeader := "timestamp_iso,uptime_sec,vram_dedicated_mb,vram_shared_mb,ram_private_mb,page_file_mb,cpu_pct,system_free_ram_mb\n"
	if !strings.HasPrefix(got, wantHeader) {
		t.Errorf("header mismatch:\n got: %q\nwant: %q", got, wantHeader)
	}
	wantRow := "2026-04-26T12:00:00Z,42,1024.50,128.00,2048.00,3072.00,37.50,16384.00\n"
	if !strings.HasSuffix(got, wantRow) {
		t.Errorf("row mismatch:\n got: %q\nwant: %q", got, wantRow)
	}
}

func TestCSVLoggerNewSessionNewFile(t *testing.T) {
	dir := t.TempDir()
	lg1, _ := NewCSVLogger(dir)
	lg1.Write(Sample{Timestamp: time.Now()})
	lg1.Close()

	// Sleep just enough to guarantee a different filename second.
	time.Sleep(1100 * time.Millisecond)

	lg2, _ := NewCSVLogger(dir)
	lg2.Write(Sample{Timestamp: time.Now()})
	lg2.Close()

	files, _ := filepath.Glob(filepath.Join(dir, "*.csv"))
	if len(files) != 2 {
		t.Errorf("expected 2 csv files, got %d", len(files))
	}
}

func TestCSVLoggerCreatesDirIfMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "logs")
	lg, err := NewCSVLogger(dir)
	if err != nil {
		t.Fatalf("NewCSVLogger: %v", err)
	}
	defer lg.Close()
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("log dir not created: %v", err)
	}
}
