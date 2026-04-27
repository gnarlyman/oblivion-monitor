package main

import (
	"strings"
	"testing"
)

func TestSparklineEmpty(t *testing.T) {
	got := Sparkline(nil, 10)
	if got != strings.Repeat(" ", 10) {
		t.Errorf("empty samples should produce all spaces, got %q", got)
	}
}

func TestSparklineMinMaxFullRange(t *testing.T) {
	got := Sparkline([]float64{0, 100}, 2)
	want := "▁█"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSparklineMonotonicIncreasing(t *testing.T) {
	got := Sparkline([]float64{1, 2, 3, 4, 5, 6, 7, 8}, 8)
	want := "▁▂▃▄▅▆▇█"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSparklineConstantValue(t *testing.T) {
	got := Sparkline([]float64{50, 50, 50}, 3)
	// All-equal samples: render at the bottom block (no variation).
	want := "▁▁▁"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSparklineWidthLargerThanSamples(t *testing.T) {
	got := Sparkline([]float64{1, 2}, 5)
	// Pad on the left with spaces; samples render at the right.
	if !strings.HasSuffix(got, "▁█") {
		t.Errorf("got %q, want suffix %q", got, "▁█")
	}
	if len([]rune(got)) != 5 {
		t.Errorf("got width %d, want 5", len([]rune(got)))
	}
}

func TestSparklineWidthSmallerThanSamples(t *testing.T) {
	// More samples than width: take the most recent N.
	got := Sparkline([]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 3)
	want := "▆▇█"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
