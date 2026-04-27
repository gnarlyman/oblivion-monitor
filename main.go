package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	exeDir := filepath.Dir(exe)
	configPath := filepath.Join(exeDir, "config.json")

	// Set up file logging next to the exe.
	logFile, err := os.OpenFile(
		filepath.Join(exeDir, "oblivion-monitor.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		log.Printf("config load failed, using defaults: %v", err)
		cfg = Config{
			PollingIntervalMS:    1000,
			ScanIntervalMS:       2000,
			LogDir:               "logs",
			ProcessName:          "Oblivion.exe",
			FallbackToLocalUsage: true,
		}
	}
	logDir := cfg.LogDir
	if !filepath.IsAbs(logDir) {
		logDir = filepath.Join(exeDir, logDir)
	}

	ui, err := NewUI(cfg, configPath)
	if err != nil {
		log.Fatalf("UI init: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan PIDEvent, 4)
	w := &Watcher{
		ProcessName: cfg.ProcessName,
		Interval:    time.Duration(cfg.ScanIntervalMS) * time.Millisecond,
		Events:      events,
	}
	go w.Run(ctx)

	// Goroutine that owns the Sampler lifecycle: starts one when PID
	// appears, stops it when PID disappears, manages CSV file rotation.
	go func() {
		var (
			sampler       *Sampler
			samplerCtx    context.Context
			samplerCancel context.CancelFunc
			csvLog        *CSVLogger
			samples       chan Sample
		)
		stop := func() {
			if samplerCancel != nil {
				samplerCancel()
				samplerCancel = nil
			}
			if csvLog != nil {
				csvLog.Close()
				csvLog = nil
			}
			ui.OnPIDLost()
		}
		for {
			select {
			case <-ctx.Done():
				stop()
				return
			case ev, ok := <-events:
				if !ok {
					stop()
					return
				}
				if ev.PID == 0 {
					stop()
					continue
				}
				stop()
				csvLog, err = NewCSVLogger(logDir)
				if err != nil {
					log.Printf("CSV logger: %v", err)
				}
				samples = make(chan Sample, 4)
				samplerCtx, samplerCancel = context.WithCancel(ctx)
				sampler = &Sampler{
					Config:  cfg,
					PID:     ev.PID,
					Samples: samples,
				}
				ui.OnPIDFound(ev.PID, cfg.ProcessName)
				go sampler.Run(samplerCtx)
				go func(s chan Sample, c *CSVLogger) {
					for sample := range s {
						ui.OnSample(sample)
						if c != nil {
							_ = c.Write(sample)
						}
					}
				}(samples, csvLog)
			}
		}
	}()

	ui.Run() // blocks on message loop
	ui.DisposeTray()
	cancel()
}
