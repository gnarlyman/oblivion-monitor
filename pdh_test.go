package main

import (
	"testing"
)

func TestPDHOpenCloseQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("PDH integration test")
	}
	q, err := PdhOpenQuery()
	if err != nil {
		t.Fatalf("PdhOpenQuery: %v", err)
	}
	if err := q.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestPDHReadSystemAvailableMBytes(t *testing.T) {
	if testing.Short() {
		t.Skip("PDH integration test")
	}
	q, err := PdhOpenQuery()
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()

	c, err := q.AddEnglishCounter(`\Memory\Available MBytes`)
	if err != nil {
		t.Fatalf("AddEnglishCounter: %v", err)
	}

	// Two collects required: first establishes baseline for rate counters.
	// Single-instantaneous counters return after the first collect, but
	// we standardize on two for safety.
	if err := q.Collect(); err != nil {
		t.Fatal(err)
	}
	if err := q.Collect(); err != nil {
		t.Fatal(err)
	}

	val, err := c.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	if val <= 0 {
		t.Errorf("Available MBytes should be > 0, got %v", val)
	}
	t.Logf("Available MBytes = %.0f", val)
}

func TestPDHExpandWildcardForGPUMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("PDH integration test")
	}
	// Find any GPU process memory instance — there's always at least one
	// (dwm.exe, explorer.exe, or any GPU-using process).
	paths, err := PdhExpandWildCardPath(`\GPU Process Memory(*)\Dedicated Usage`)
	if err != nil {
		t.Fatalf("PdhExpandWildCardPath: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("expected >=1 expanded path, got 0")
	}
	t.Logf("expanded %d paths; first = %q", len(paths), paths[0])
}
