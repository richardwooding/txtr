// Package triage provides security-triage analysis of extracted strings:
// Shannon entropy scoring, content classification (URL/IP/email/path/etc.),
// and secret/credential detection.
//
// The package has no dependencies on other internal packages so it can be
// reused by the printer (text output), JSON output, and statistics modes
// without creating import cycles.
package triage

import "math"

// Entropy returns the Shannon entropy of data in bits per byte, a value in the
// range [0, 8]. Higher values indicate more randomness: natural-language text
// typically scores ~4.0-4.5, base64-encoded data ~6, and cryptographic key
// material approaches 8.
func Entropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}

	var counts [256]int
	for _, b := range data {
		counts[b]++
	}

	n := float64(len(data))
	var h float64
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		h -= p * math.Log2(p)
	}
	return h
}
