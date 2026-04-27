package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
)

// Config holds runtime settings persisted to config.json.
type Config struct {
	WindowX              *int   `json:"window_x"`
	WindowY              *int   `json:"window_y"`
	PollingIntervalMS    int    `json:"polling_interval_ms"`
	ScanIntervalMS       int    `json:"scan_interval_ms"`
	LogDir               string `json:"log_dir"`
	ProcessName          string `json:"process_name"`
	FallbackToLocalUsage bool   `json:"fallback_to_local_usage"`
}

func defaultConfig() Config {
	return Config{
		WindowX:              nil,
		WindowY:              nil,
		PollingIntervalMS:    1000,
		ScanIntervalMS:       2000,
		LogDir:               "logs",
		ProcessName:          "Oblivion.exe",
		FallbackToLocalUsage: true,
	}
}

// LoadConfig reads config from path. Missing file returns defaults (no error).
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return defaultConfig(), nil
	}
	if err != nil {
		return Config{}, err
	}
	cfg := defaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// SaveConfig writes cfg to path as pretty-printed JSON.
func SaveConfig(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
