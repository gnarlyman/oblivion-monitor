package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindPIDByNameSelf(t *testing.T) {
	if testing.Short() {
		t.Skip("process integration test")
	}
	// The test binary itself is running. Its name ends in .test or .test.exe.
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	exeName := filepath.Base(exe)

	pid, err := FindPIDByName(exeName)
	if err != nil {
		t.Fatalf("FindPIDByName(%q): %v", exeName, err)
	}
	if pid == 0 {
		t.Errorf("expected non-zero PID for self (%s)", exeName)
	}
	if pid != uint32(os.Getpid()) {
		t.Errorf("got PID %d, want self PID %d", pid, os.Getpid())
	}
}

func TestFindPIDByNameMissingReturnsZero(t *testing.T) {
	if testing.Short() {
		t.Skip("process integration test")
	}
	pid, err := FindPIDByName("definitely-not-a-real-process-12345.exe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != 0 {
		t.Errorf("expected 0 for missing process, got %d", pid)
	}
}

func TestFindPIDByNameCaseInsensitive(t *testing.T) {
	if testing.Short() {
		t.Skip("process integration test")
	}
	exe, _ := os.Executable()
	exeName := filepath.Base(exe)
	pid, err := FindPIDByName(strings.ToUpper(exeName))
	if err != nil {
		t.Fatal(err)
	}
	if pid == 0 {
		t.Errorf("expected non-zero PID for uppercase name")
	}
}
