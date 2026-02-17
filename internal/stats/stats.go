// Package stats provides statistics aggregation for extracted strings.
package stats

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/richardwooding/txtr/internal/extractor"
	"github.com/richardwooding/txtr/internal/printer"
)

// Statistics holds aggregated statistics about extracted strings
type Statistics struct {
	// File metadata
	Filename     string
	BinaryFormat string
	Sections     []string

	// Count statistics
	TotalStrings    int
	TotalBytes      int64
	MinLength       int // From config
	MaxLength       int
	FilteredCount   int // Strings that passed filters
	UnfilteredCount int // Total strings before filtering

	// Distribution maps
	EncodingCounts map[string]int
	LengthBuckets  map[string]int

	// Longest strings
	LongestStrings []LongestString
}

// LongestString represents one of the longest strings found
type LongestString struct {
	Value  string
	Length int
	Offset int64
}

// New creates a new Statistics instance with initialized maps
func New(minLength int) *Statistics {
	return &Statistics{
		MinLength:      minLength,
		EncodingCounts: make(map[string]int),
		LengthBuckets:  make(map[string]int),
		LongestStrings: make([]LongestString, 0, 5),
	}
}

// SetFileInfo sets file metadata (filename, format, sections)
func (s *Statistics) SetFileInfo(filename, format string, sections []string) {
	s.Filename = filename
	s.BinaryFormat = format
	s.Sections = sections
}

// AddUnfiltered tracks a string before filtering (for filter statistics)
func (s *Statistics) AddUnfiltered() {
	s.UnfilteredCount++
}

// Add adds a string to the statistics (for strings that passed filters)
// This method signature matches the printFunc signature for easy integration
func (s *Statistics) Add(str []byte, _ string, offset int64, config extractor.Config) {
	s.TotalStrings++
	s.FilteredCount++
	s.TotalBytes += int64(len(str))

	// Update max length
	length := len(str)
	if length > s.MaxLength {
		s.MaxLength = length
	}

	// Update longest strings list
	s.updateLongest(str, offset, length)

	// Classify encoding
	encoding := s.detectEncoding(str, config)
	s.EncodingCounts[encoding]++

	// Update length bucket
	bucket := s.getBucket(length)
	s.LengthBuckets[bucket]++
}

// detectEncoding classifies the encoding type of a string
func (s *Statistics) detectEncoding(str []byte, config extractor.Config) string {
	// UTF-16 or UTF-32 based on config encoding
	if config.Encoding == "b" || config.Encoding == "l" {
		return "utf-16"
	}
	if config.Encoding == "B" || config.Encoding == "L" {
		return "utf-32"
	}

	// Check for UTF-8 multibyte sequences
	if utf8.Valid(str) && hasMultibyteUTF8(str) {
		return "utf-8"
	}

	// Check for 8-bit ASCII (high bytes)
	for _, b := range str {
		if b >= 128 {
			return "ascii-8bit"
		}
	}

	// Default to 7-bit ASCII
	return "ascii-7bit"
}

// hasMultibyteUTF8 checks if string contains multi-byte UTF-8 sequences
func hasMultibyteUTF8(str []byte) bool {
	for _, b := range str {
		if b >= 128 {
			return true
		}
	}
	return false
}

// getBucket returns the length bucket for a string
func (s *Statistics) getBucket(length int) string {
	switch {
	case length >= 4 && length <= 10:
		return "4-10"
	case length >= 11 && length <= 50:
		return "11-50"
	case length >= 51 && length <= 100:
		return "51-100"
	default:
		return "100+"
	}
}

// updateLongest updates the list of longest strings
func (s *Statistics) updateLongest(str []byte, offset int64, length int) {
	// Create new entry
	entry := LongestString{
		Value:  string(str),
		Length: length,
		Offset: offset,
	}

	// Add to list
	s.LongestStrings = append(s.LongestStrings, entry)

	// Sort by length (descending)
	sort.Slice(s.LongestStrings, func(i, j int) bool {
		return s.LongestStrings[i].Length > s.LongestStrings[j].Length
	})

	// Keep only top 5
	if len(s.LongestStrings) > 5 {
		s.LongestStrings = s.LongestStrings[:5]
	}
}

// AvgLength calculates the average string length
func (s *Statistics) AvgLength() float64 {
	if s.TotalStrings == 0 {
		return 0.0
	}
	return float64(s.TotalBytes) / float64(s.TotalStrings)
}

// Format outputs human-readable statistics to the writer with optional colors
//
//nolint:errcheck // Writing to stdout/buffer, errors are not critical
func (s *Statistics) Format(w io.Writer, colorMode extractor.ColorMode) {
	// Determine if colors should be used
	useColor := printer.ShouldUseColor(colorMode)

	// Header
	if s.Filename != "" {
		header := "Statistics for " + s.Filename + ":"
		header = printer.ColorString(header, printer.AnsiBold+printer.AnsiCyan, useColor)
		fmt.Fprintf(w, "%s\n", header)
	} else {
		header := printer.ColorString("Statistics:", printer.AnsiBold+printer.AnsiCyan, useColor)
		fmt.Fprintf(w, "%s\n", header)
	}

	// Binary format info
	if s.BinaryFormat != "" {
		format := printer.ColorString(s.BinaryFormat, printer.AnsiCyan, useColor)
		fmt.Fprintf(w, "  Binary format:     %s\n", format)
	}
	if len(s.Sections) > 0 {
		sections := printer.ColorString(strings.Join(s.Sections, ", "), printer.AnsiCyan, useColor)
		fmt.Fprintf(w, "  Sections scanned:  %s\n", sections)
	}
	if s.BinaryFormat != "" || len(s.Sections) > 0 {
		fmt.Fprintln(w)
	}

	// Count statistics
	if s.UnfilteredCount > 0 {
		// Show filter statistics
		unfilteredNum := printer.ColorString(formatNumber(s.UnfilteredCount), printer.AnsiYellow, useColor)
		fmt.Fprintf(w, "  Total strings extracted:  %s\n", unfilteredNum)

		filteredNum := printer.ColorString(formatNumber(s.FilteredCount), printer.AnsiYellow, useColor)
		pct := printer.ColorString(fmt.Sprintf("%.1f%%", percentage(s.FilteredCount, s.UnfilteredCount)), printer.AnsiGreen, useColor)
		fmt.Fprintf(w, "  Matched filters:          %s (%s)\n", filteredNum, pct)
	} else {
		// No filtering
		totalNum := printer.ColorString(formatNumber(s.TotalStrings), printer.AnsiYellow, useColor)
		fmt.Fprintf(w, "  Total strings:     %s\n", totalNum)
	}

	bytesNum := printer.ColorString(formatNumber(int(s.TotalBytes)), printer.AnsiYellow, useColor)
	fmt.Fprintf(w, "  Total bytes:       %s\n", bytesNum)

	minNum := printer.ColorString(fmt.Sprintf("%d", s.MinLength), printer.AnsiYellow, useColor)
	fmt.Fprintf(w, "  Min length:        %s (configured)\n", minNum)

	maxNum := printer.ColorString(fmt.Sprintf("%d", s.MaxLength), printer.AnsiYellow, useColor)
	fmt.Fprintf(w, "  Max length:        %s\n", maxNum)

	avgNum := printer.ColorString(fmt.Sprintf("%.1f", s.AvgLength()), printer.AnsiYellow, useColor)
	fmt.Fprintf(w, "  Avg length:        %s\n", avgNum)
	fmt.Fprintln(w)

	// Encoding distribution
	if len(s.EncodingCounts) > 0 {
		header := printer.ColorString("Encoding distribution:", printer.AnsiBold+printer.AnsiCyan, useColor)
		fmt.Fprintf(w, "  %s\n", header)

		// Sort encoding types for consistent output
		encodings := make([]string, 0, len(s.EncodingCounts))
		for enc := range s.EncodingCounts {
			encodings = append(encodings, enc)
		}
		sort.Strings(encodings)

		for _, enc := range encodings {
			count := s.EncodingCounts[enc]
			encName := printer.ColorString(formatEncodingName(enc)+":", printer.AnsiMagenta, useColor)
			countNum := printer.ColorString(formatNumber(count), printer.AnsiYellow, useColor)
			pct := printer.ColorString(fmt.Sprintf("%5.1f%%", percentage(count, s.TotalStrings)), printer.AnsiGreen, useColor)
			fmt.Fprintf(w, "    %-15s %6s (%s)\n", encName, countNum, pct)
		}
		fmt.Fprintln(w)
	}

	// Length distribution
	if len(s.LengthBuckets) > 0 {
		header := printer.ColorString("Length distribution:", printer.AnsiBold+printer.AnsiCyan, useColor)
		fmt.Fprintf(w, "  %s\n", header)

		// Fixed bucket order
		buckets := []string{"4-10", "11-50", "51-100", "100+"}
		for _, bucket := range buckets {
			if count, ok := s.LengthBuckets[bucket]; ok {
				countNum := printer.ColorString(formatNumber(count), printer.AnsiYellow, useColor)
				pct := printer.ColorString(fmt.Sprintf("%5.1f%%", percentage(count, s.TotalStrings)), printer.AnsiGreen, useColor)
				fmt.Fprintf(w, "    %s chars:    %6s (%s)\n", bucket, countNum, pct)
			}
		}
		fmt.Fprintln(w)
	}

	// Longest strings
	if len(s.LongestStrings) > 0 {
		header := printer.ColorString("Longest strings:", printer.AnsiBold+printer.AnsiCyan, useColor)
		fmt.Fprintf(w, "  %s\n", header)

		for _, ls := range s.LongestStrings {
			preview := ls.Value
			if len(preview) > 50 {
				preview = preview[:47] + "..."
			}
			lengthNum := printer.ColorString(fmt.Sprintf("%d", ls.Length), printer.AnsiYellow, useColor)
			offsetNum := printer.ColorString(fmt.Sprintf("0x%x", ls.Offset), printer.AnsiYellow, useColor)
			previewStr := printer.ColorString(fmt.Sprintf("%q", preview), printer.AnsiDim, useColor)
			fmt.Fprintf(w, "    %s chars at %s: %s\n", lengthNum, offsetNum, previewStr)
		}
	}
}

// formatNumber adds thousand separators to numbers
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	// Convert to string and add commas
	str := fmt.Sprintf("%d", n)
	var result []byte
	for i, digit := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(digit))
	}
	return string(result)
}

// percentage calculates percentage with 1 decimal place
func percentage(part, total int) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(part) * 100.0 / float64(total)
}

// formatEncodingName converts internal encoding names to display names
func formatEncodingName(enc string) string {
	switch enc {
	case "ascii-7bit":
		return "ASCII (7-bit)"
	case "ascii-8bit":
		return "High-byte"
	case "utf-8":
		return "UTF-8"
	case "utf-16":
		return "UTF-16"
	case "utf-32":
		return "UTF-32"
	default:
		return enc
	}
}

// ToJSON converts statistics to JSON format
func (s *Statistics) ToJSON() ([]byte, error) {
	output := map[string]any{
		"total_strings": s.TotalStrings,
		"total_bytes":   s.TotalBytes,
		"min_length":    s.MinLength,
		"max_length":    s.MaxLength,
		"avg_length":    s.AvgLength(),
	}

	// Add file info if available
	if s.Filename != "" {
		output["filename"] = s.Filename
	}
	if s.BinaryFormat != "" {
		output["format"] = s.BinaryFormat
	}
	if len(s.Sections) > 0 {
		output["sections"] = s.Sections
	}

	// Add filter statistics if applicable
	if s.UnfilteredCount > 0 {
		output["unfiltered_count"] = s.UnfilteredCount
		output["filtered_count"] = s.FilteredCount
		output["filter_percentage"] = percentage(s.FilteredCount, s.UnfilteredCount)
	}

	// Add distributions
	if len(s.EncodingCounts) > 0 {
		output["encoding_distribution"] = s.EncodingCounts
	}
	if len(s.LengthBuckets) > 0 {
		output["length_distribution"] = s.LengthBuckets
	}

	// Add longest strings
	if len(s.LongestStrings) > 0 {
		longest := make([]map[string]any, len(s.LongestStrings))
		for i, ls := range s.LongestStrings {
			preview := ls.Value
			if len(preview) > 50 {
				preview = preview[:47] + "..."
			}
			longest[i] = map[string]any{
				"length":     ls.Length,
				"offset":     ls.Offset,
				"offset_hex": fmt.Sprintf("0x%x", ls.Offset),
				"preview":    preview,
			}
		}
		output["longest_strings"] = longest
	}

	return json.MarshalIndent(output, "", "  ")
}

// Merge combines another Statistics instance into this one (for aggregation)
func (s *Statistics) Merge(other *Statistics) {
	s.TotalStrings += other.TotalStrings
	s.FilteredCount += other.FilteredCount
	s.UnfilteredCount += other.UnfilteredCount
	s.TotalBytes += other.TotalBytes

	// Update max length
	if other.MaxLength > s.MaxLength {
		s.MaxLength = other.MaxLength
	}

	// Merge encoding counts
	for enc, count := range other.EncodingCounts {
		s.EncodingCounts[enc] += count
	}

	// Merge length buckets
	for bucket, count := range other.LengthBuckets {
		s.LengthBuckets[bucket] += count
	}

	// Merge longest strings
	s.LongestStrings = append(s.LongestStrings, other.LongestStrings...)
	sort.Slice(s.LongestStrings, func(i, j int) bool {
		return s.LongestStrings[i].Length > s.LongestStrings[j].Length
	})
	if len(s.LongestStrings) > 5 {
		s.LongestStrings = s.LongestStrings[:5]
	}
}
