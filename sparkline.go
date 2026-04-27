package main

import "strings"

// Block characters from U+2581 to U+2588: ▁▂▃▄▅▆▇█
var blocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders samples as `width` Unicode block characters. If there are
// more samples than width, only the most recent `width` samples are used.
// If fewer, the result is left-padded with spaces. Empty samples produce
// `width` spaces.
func Sparkline(samples []float64, width int) string {
	if width <= 0 {
		return ""
	}
	if len(samples) == 0 {
		return strings.Repeat(" ", width)
	}

	// Find min/max for normalization across all samples before clipping.
	min, max := samples[0], samples[0]
	for _, v := range samples {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min

	// Clip to most recent `width` samples.
	if len(samples) > width {
		samples = samples[len(samples)-width:]
	}

	var b strings.Builder
	pad := width - len(samples)
	for i := 0; i < pad; i++ {
		b.WriteRune(' ')
	}
	for _, v := range samples {
		var idx int
		if span == 0 {
			idx = 0 // all-equal: render at bottom
		} else {
			normalized := (v - min) / span
			idx = int(normalized * float64(len(blocks)-1))
			if idx < 0 {
				idx = 0
			}
			if idx >= len(blocks) {
				idx = len(blocks) - 1
			}
		}
		b.WriteRune(blocks[idx])
	}
	return b.String()
}
