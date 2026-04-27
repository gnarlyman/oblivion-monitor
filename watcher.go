package main

import (
	"context"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// FindPIDByName scans running processes via Toolhelp32Snapshot and returns
// the PID of the first process whose executable name (case-insensitively)
// matches `name`. Returns (0, nil) if not found.
func FindPIDByName(name string) (uint32, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snap)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snap, &entry); err != nil {
		return 0, err
	}

	target := strings.ToLower(name)
	for {
		exeName := strings.ToLower(syscall.UTF16ToString(entry.ExeFile[:]))
		if exeName == target {
			return entry.ProcessID, nil
		}
		err := windows.Process32Next(snap, &entry)
		if err != nil {
			// ERROR_NO_MORE_FILES marks end of enumeration.
			if err == syscall.ERROR_NO_MORE_FILES {
				return 0, nil
			}
			return 0, err
		}
	}
}

// PIDEvent is emitted by Watcher when the target process appears or disappears.
type PIDEvent struct {
	PID uint32 // 0 means lost
}

// Watcher polls for the named process every interval and emits events on
// transitions.
type Watcher struct {
	ProcessName string
	Interval    time.Duration
	Events      chan PIDEvent
}

// Run blocks until ctx is cancelled. Closes Events on exit.
func (w *Watcher) Run(ctx context.Context) {
	defer close(w.Events)
	var current uint32
	t := time.NewTicker(w.Interval)
	defer t.Stop()

	scan := func() {
		pid, err := FindPIDByName(w.ProcessName)
		if err != nil {
			return // transient errors swallowed; next tick retries
		}
		if pid != current {
			current = pid
			select {
			case w.Events <- PIDEvent{PID: pid}:
			case <-ctx.Done():
			}
		}
	}

	scan() // immediate first scan
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			scan()
		}
	}
}
