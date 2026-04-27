package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadMissingReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(filepath.Join(dir, "missing.json"))
	if err != nil {
		t.Fatalf("LoadConfig on missing file should not error, got %v", err)
	}
	if cfg.PollingIntervalMS != 1000 {
		t.Errorf("default PollingIntervalMS = %d, want 1000", cfg.PollingIntervalMS)
	}
	if cfg.ScanIntervalMS != 2000 {
		t.Errorf("default ScanIntervalMS = %d, want 2000", cfg.ScanIntervalMS)
	}
	if cfg.LogDir != "logs" {
		t.Errorf("default LogDir = %q, want %q", cfg.LogDir, "logs")
	}
	if cfg.ProcessName != "Oblivion.exe" {
		t.Errorf("default ProcessName = %q, want %q", cfg.ProcessName, "Oblivion.exe")
	}
	if cfg.WindowX != nil || cfg.WindowY != nil {
		t.Errorf("default WindowX/Y should be nil, got %v %v", cfg.WindowX, cfg.WindowY)
	}
	if !cfg.FallbackToLocalUsage {
		t.Errorf("default FallbackToLocalUsage should be true")
	}
}

func TestConfigLoadExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	x, y := 100, 200
	original := Config{
		WindowX:              &x,
		WindowY:              &y,
		PollingIntervalMS:    500,
		ScanIntervalMS:       1500,
		LogDir:               "custom-logs",
		ProcessName:          "OtherGame.exe",
		FallbackToLocalUsage: false,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if *loaded.WindowX != 100 || *loaded.WindowY != 200 {
		t.Errorf("WindowX/Y = %v %v, want 100 200", *loaded.WindowX, *loaded.WindowY)
	}
	if loaded.PollingIntervalMS != 500 {
		t.Errorf("PollingIntervalMS = %d, want 500", loaded.PollingIntervalMS)
	}
	if loaded.ProcessName != "OtherGame.exe" {
		t.Errorf("ProcessName = %q, want %q", loaded.ProcessName, "OtherGame.exe")
	}
}

func TestConfigSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	x, y := 50, 75
	cfg := Config{
		WindowX:              &x,
		WindowY:              &y,
		PollingIntervalMS:    1000,
		ScanIntervalMS:       2000,
		LogDir:               "logs",
		ProcessName:          "Oblivion.exe",
		FallbackToLocalUsage: true,
	}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if *loaded.WindowX != 50 || *loaded.WindowY != 75 {
		t.Errorf("round-trip lost WindowX/Y: %v %v", *loaded.WindowX, *loaded.WindowY)
	}
}
