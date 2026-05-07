package main

import (
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWalkProcessVMSelf(t *testing.T) {
	pid := uint32(os.Getpid())
	h, err := OpenProcessForVMWalk(pid)
	if err != nil {
		t.Fatalf("OpenProcessForVMWalk: %v", err)
	}
	defer windows.CloseHandle(h)

	t0 := time.Now()
	stats, err := WalkProcessVM(h)
	dur := time.Since(t0)
	if err != nil {
		t.Fatalf("WalkProcessVM: %v", err)
	}

	if stats.Regions == 0 {
		t.Fatal("Regions=0; sweep walked nothing")
	}
	if stats.PrivateCommitMB <= 0 {
		t.Errorf("PrivateCommitMB=%.2f, expected > 0 for self process", stats.PrivateCommitMB)
	}
	if stats.LargestFreeMB <= 0 {
		// On 64-bit user VA (~128TB) the largest free hole is usually huge.
		// On 32-bit (LAA) it's still reliably > 0.
		t.Errorf("LargestFreeMB=%.2f, expected > 0", stats.LargestFreeMB)
	}
	if stats.LargestFreeMB > stats.FreeMB {
		t.Errorf("LargestFreeMB=%.2f > FreeMB=%.2f, invariant broken",
			stats.LargestFreeMB, stats.FreeMB)
	}
	t.Logf("self: %d regions, commit=%.0f MB, mapped=%.0f MB, free=%.0f MB, LFB=%.0f MB, walk=%v",
		stats.Regions, stats.PrivateCommitMB, stats.MappedCommitMB,
		stats.FreeMB, stats.LargestFreeMB, dur)
}

func TestOpenProcessForVMWalkBadPID(t *testing.T) {
	// PID 0 is the System Idle Process; OpenProcess should fail.
	_, err := OpenProcessForVMWalk(0)
	if err == nil {
		t.Fatal("expected OpenProcess to fail for PID 0")
	}
}
