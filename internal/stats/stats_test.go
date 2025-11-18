package stats

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/richardwooding/txtr/internal/extractor"
)

func TestNew(t *testing.T) {
	s := New(4)
	if s.MinLength != 4 {
		t.Errorf("New() MinLength = %d, want 4", s.MinLength)
	}
	if s.EncodingCounts == nil {
		t.Error("New() EncodingCounts is nil")
	}
	if s.LengthBuckets == nil {
		t.Error("New() LengthBuckets is nil")
	}
	if s.LongestStrings == nil {
		t.Error("New() LongestStrings is nil")
	}
}

func TestSetFileInfo(t *testing.T) {
	s := New(4)
	s.SetFileInfo("test.bin", "ELF", []string{".data", ".rodata"})

	if s.Filename != "test.bin" {
		t.Errorf("Filename = %q, want %q", s.Filename, "test.bin")
	}
	if s.BinaryFormat != "ELF" {
		t.Errorf("BinaryFormat = %q, want %q", s.BinaryFormat, "ELF")
	}
	if len(s.Sections) != 2 {
		t.Errorf("len(Sections) = %d, want 2", len(s.Sections))
	}
}

func TestAddUnfiltered(t *testing.T) {
	s := New(4)
	s.AddUnfiltered()
	s.AddUnfiltered()
	s.AddUnfiltered()

	if s.UnfilteredCount != 3 {
		t.Errorf("UnfilteredCount = %d, want 3", s.UnfilteredCount)
	}
}

func TestAdd(t *testing.T) {
	s := New(4)
	config := extractor.Config{Encoding: "s"}

	// Add a few strings
	s.Add([]byte("test"), "file.bin", 0x1000, config)
	s.Add([]byte("hello world"), "file.bin", 0x2000, config)
	s.Add([]byte("a"), "file.bin", 0x3000, config)

	if s.TotalStrings != 3 {
		t.Errorf("TotalStrings = %d, want 3", s.TotalStrings)
	}
	if s.FilteredCount != 3 {
		t.Errorf("FilteredCount = %d, want 3", s.FilteredCount)
	}
	if s.TotalBytes != 16 { // 4 + 11 + 1
		t.Errorf("TotalBytes = %d, want 16", s.TotalBytes)
	}
	if s.MaxLength != 11 {
		t.Errorf("MaxLength = %d, want 11", s.MaxLength)
	}
}

func TestDetectEncoding(t *testing.T) {
	tests := []struct {
		name     string
		str      []byte
		config   extractor.Config
		want     string
	}{
		{
			name:   "7-bit ASCII",
			str:    []byte("hello world"),
			config: extractor.Config{Encoding: "s"},
			want:   "ascii-7bit",
		},
		{
			name:   "8-bit ASCII (high bytes)",
			str:    []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x80, 0xff},
			config: extractor.Config{Encoding: "S"},
			want:   "ascii-8bit",
		},
		{
			name:   "UTF-8",
			str:    []byte("hello 世界"),
			config: extractor.Config{Encoding: "s"},
			want:   "utf-8",
		},
		{
			name:   "UTF-16 from config",
			str:    []byte("test"),
			config: extractor.Config{Encoding: "b"},
			want:   "utf-16",
		},
		{
			name:   "UTF-32 from config",
			str:    []byte("test"),
			config: extractor.Config{Encoding: "L"},
			want:   "utf-32",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(4)
			got := s.detectEncoding(tt.str, tt.config)
			if got != tt.want {
				t.Errorf("detectEncoding() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetBucket(t *testing.T) {
	tests := []struct {
		length int
		want   string
	}{
		{4, "4-10"},
		{10, "4-10"},
		{11, "11-50"},
		{50, "11-50"},
		{51, "51-100"},
		{100, "51-100"},
		{101, "100+"},
		{1000, "100+"},
	}

	s := New(4)
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := s.getBucket(tt.length)
			if got != tt.want {
				t.Errorf("getBucket(%d) = %q, want %q", tt.length, got, tt.want)
			}
		})
	}
}

func TestUpdateLongest(t *testing.T) {
	s := New(4)

	// Add strings of varying lengths
	strings := []struct {
		str    []byte
		offset int64
	}{
		{[]byte("short"), 0x1000},
		{[]byte("this is a much longer string"), 0x2000},
		{[]byte("medium length string"), 0x3000},
		{[]byte("x"), 0x4000},
		{[]byte("another very long string for testing"), 0x5000},
		{[]byte("the longest string in this entire test suite"), 0x6000},
	}

	for _, item := range strings {
		s.updateLongest(item.str, item.offset, len(item.str))
	}

	// Should keep only top 5
	if len(s.LongestStrings) != 5 {
		t.Errorf("len(LongestStrings) = %d, want 5", len(s.LongestStrings))
	}

	// Verify sorted by length (descending)
	for i := 0; i < len(s.LongestStrings)-1; i++ {
		if s.LongestStrings[i].Length < s.LongestStrings[i+1].Length {
			t.Errorf("LongestStrings not sorted: [%d]=%d < [%d]=%d",
				i, s.LongestStrings[i].Length,
				i+1, s.LongestStrings[i+1].Length)
		}
	}

	// Verify longest is actually the longest
	if s.LongestStrings[0].Length != 44 {
		t.Errorf("Longest string length = %d, want 44", s.LongestStrings[0].Length)
	}
}

func TestAvgLength(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*Statistics)
		want   float64
	}{
		{
			name: "zero strings",
			setup: func(_ *Statistics) {
				// No strings added
			},
			want: 0.0,
		},
		{
			name: "single string",
			setup: func(s *Statistics) {
				s.TotalStrings = 1
				s.TotalBytes = 10
			},
			want: 10.0,
		},
		{
			name: "multiple strings",
			setup: func(s *Statistics) {
				s.TotalStrings = 4
				s.TotalBytes = 100
			},
			want: 25.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(4)
			tt.setup(s)
			got := s.AvgLength()
			if got != tt.want {
				t.Errorf("AvgLength() = %.1f, want %.1f", got, tt.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	s := New(4)
	s.SetFileInfo("test.bin", "ELF", []string{".data", ".rodata"})

	config := extractor.Config{Encoding: "s"}
	s.Add([]byte("test"), "test.bin", 0x1000, config)
	s.Add([]byte("hello world"), "test.bin", 0x2000, config)
	s.Add([]byte("foo bar baz"), "test.bin", 0x3000, config)

	var buf bytes.Buffer
	s.Format(&buf, extractor.ColorNever)

	output := buf.String()

	// Check for expected content
	expectedStrings := []string{
		"Statistics for test.bin:",
		"Binary format:     ELF",
		"Sections scanned:  .data, .rodata",
		"Total strings:     3",
		"Total bytes:       26",
		"Min length:        4",
		"Max length:        11",
		"Encoding distribution:",
		"ASCII (7-bit)",
		"Length distribution:",
		"4-10 chars:",
		"11-50 chars:",
		"Longest strings:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Format() output missing %q", expected)
		}
	}
}

func TestFormatWithFiltering(t *testing.T) {
	s := New(4)
	s.UnfilteredCount = 100
	s.FilteredCount = 75
	s.TotalStrings = 75
	s.TotalBytes = 1000

	var buf bytes.Buffer
	s.Format(&buf, extractor.ColorNever)

	output := buf.String()

	// Check for filter statistics
	if !strings.Contains(output, "Total strings extracted:  100") {
		t.Error("Format() missing unfiltered count")
	}
	if !strings.Contains(output, "Matched filters:          75") {
		t.Error("Format() missing filtered count")
	}
	if !strings.Contains(output, "75.0%") {
		t.Error("Format() missing filter percentage")
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatNumber(tt.input)
			if got != tt.want {
				t.Errorf("formatNumber(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPercentage(t *testing.T) {
	tests := []struct {
		part  int
		total int
		want  float64
	}{
		{0, 100, 0.0},
		{25, 100, 25.0},
		{50, 100, 50.0},
		{100, 100, 100.0},
		{1, 3, 33.333333333333336},
		{0, 0, 0.0}, // Edge case: division by zero
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := percentage(tt.part, tt.total)
			if got != tt.want {
				t.Errorf("percentage(%d, %d) = %.2f, want %.2f", tt.part, tt.total, got, tt.want)
			}
		})
	}
}

func TestFormatEncodingName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ascii-7bit", "ASCII (7-bit)"},
		{"ascii-8bit", "High-byte"},
		{"utf-8", "UTF-8"},
		{"utf-16", "UTF-16"},
		{"utf-32", "UTF-32"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatEncodingName(tt.input)
			if got != tt.want {
				t.Errorf("formatEncodingName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToJSON(t *testing.T) {
	s := New(4)
	s.SetFileInfo("test.bin", "PE", []string{".data", ".rdata"})
	s.UnfilteredCount = 100
	s.FilteredCount = 75
	s.TotalStrings = 75
	s.TotalBytes = 1500
	s.MaxLength = 50
	s.EncodingCounts["ascii-7bit"] = 75
	s.LengthBuckets["11-50"] = 75

	jsonBytes, err := s.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	// Parse JSON to verify structure
	var output map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &output); err != nil {
		t.Fatalf("JSON unmarshal error = %v", err)
	}

	// Verify required fields
	requiredFields := []string{
		"total_strings", "total_bytes", "min_length", "max_length", "avg_length",
		"filename", "format", "sections",
		"unfiltered_count", "filtered_count", "filter_percentage",
	}

	for _, field := range requiredFields {
		if _, ok := output[field]; !ok {
			t.Errorf("ToJSON() missing field %q", field)
		}
	}

	// Verify values
	if output["total_strings"].(float64) != 75 {
		t.Errorf("total_strings = %.0f, want 75", output["total_strings"].(float64))
	}
	if output["filename"].(string) != "test.bin" {
		t.Errorf("filename = %q, want %q", output["filename"].(string), "test.bin")
	}
}

func TestMerge(t *testing.T) {
	// Create first statistics
	s1 := New(4)
	config := extractor.Config{Encoding: "s"}
	s1.Add([]byte("test1"), "file1.bin", 0x1000, config)
	s1.Add([]byte("hello world"), "file1.bin", 0x2000, config)

	// Create second statistics
	s2 := New(4)
	s2.Add([]byte("test2"), "file2.bin", 0x3000, config)
	s2.Add([]byte("foo bar baz quux"), "file2.bin", 0x4000, config)

	// Merge s2 into s1
	s1.Merge(s2)

	// Verify merged counts
	if s1.TotalStrings != 4 {
		t.Errorf("After merge, TotalStrings = %d, want 4", s1.TotalStrings)
	}
	if s1.TotalBytes != 37 { // "test1"=5, "hello world"=11, "test2"=5, "foo bar baz quux"=16, Total=37
		t.Errorf("After merge, TotalBytes = %d, want 37", s1.TotalBytes)
	}
	if s1.MaxLength != 16 {
		t.Errorf("After merge, MaxLength = %d, want 16", s1.MaxLength)
	}

	// Verify encoding counts merged
	expectedASCII := 4
	if s1.EncodingCounts["ascii-7bit"] != expectedASCII {
		t.Errorf("After merge, ascii-7bit count = %d, want %d",
			s1.EncodingCounts["ascii-7bit"], expectedASCII)
	}

	// Verify longest strings merged and sorted
	if len(s1.LongestStrings) > 5 {
		t.Errorf("After merge, len(LongestStrings) = %d, want <= 5", len(s1.LongestStrings))
	}
}

func TestEncodingDistribution(t *testing.T) {
	s := New(4)

	// Add strings with different encodings
	s.Add([]byte("ascii"), "test.bin", 0x1000, extractor.Config{Encoding: "s"})
	s.Add([]byte("hello 世界"), "test.bin", 0x2000, extractor.Config{Encoding: "s"})
	s.Add([]byte{0x48, 0x69, 0x80, 0xff}, "test.bin", 0x3000, extractor.Config{Encoding: "S"})

	// Verify encoding counts
	if s.EncodingCounts["ascii-7bit"] != 1 {
		t.Errorf("ascii-7bit count = %d, want 1", s.EncodingCounts["ascii-7bit"])
	}
	if s.EncodingCounts["utf-8"] != 1 {
		t.Errorf("utf-8 count = %d, want 1", s.EncodingCounts["utf-8"])
	}
	if s.EncodingCounts["ascii-8bit"] != 1 {
		t.Errorf("ascii-8bit count = %d, want 1", s.EncodingCounts["ascii-8bit"])
	}
}

func TestLengthBuckets(t *testing.T) {
	s := New(4)
	config := extractor.Config{Encoding: "s"}

	// Add strings in different length buckets
	s.Add([]byte("short"), "test.bin", 0x1000, config)           // 5 bytes -> 4-10
	s.Add([]byte("this is medium"), "test.bin", 0x2000, config)  // 14 bytes -> 11-50
	s.Add([]byte(strings.Repeat("x", 75)), "test.bin", 0x3000, config)  // 75 bytes -> 51-100
	s.Add([]byte(strings.Repeat("y", 150)), "test.bin", 0x4000, config) // 150 bytes -> 100+

	// Verify bucket counts
	if s.LengthBuckets["4-10"] != 1 {
		t.Errorf("4-10 bucket = %d, want 1", s.LengthBuckets["4-10"])
	}
	if s.LengthBuckets["11-50"] != 1 {
		t.Errorf("11-50 bucket = %d, want 1", s.LengthBuckets["11-50"])
	}
	if s.LengthBuckets["51-100"] != 1 {
		t.Errorf("51-100 bucket = %d, want 1", s.LengthBuckets["51-100"])
	}
	if s.LengthBuckets["100+"] != 1 {
		t.Errorf("100+ bucket = %d, want 1", s.LengthBuckets["100+"])
	}
}

// TestFormatWithColors tests colored output
func TestFormatWithColors(t *testing.T) {
	s := New(4)
	s.SetFileInfo("test.bin", "ELF", []string{".data"})
	config := extractor.Config{Encoding: "s"}
	s.Add([]byte("test string"), "test.bin", 0x1000, config)
	s.Add([]byte("hello world"), "test.bin", 0x2000, config)

	var buf bytes.Buffer
	s.Format(&buf, extractor.ColorAlways)

	output := buf.String()

	// Check for ANSI color codes
	if !strings.Contains(output, "\x1b[") {
		t.Error("Format() with ColorAlways should contain ANSI color codes")
	}

	// Check for specific colors
	if !strings.Contains(output, "\x1b[1m\x1b[36m") { // Bold Cyan for headers
		t.Error("Format() should contain bold cyan for headers")
	}
	if !strings.Contains(output, "\x1b[33m") { // Yellow for numbers
		t.Error("Format() should contain yellow for numbers")
	}
	if !strings.Contains(output, "\x1b[0m") { // Reset
		t.Error("Format() should contain ANSI reset codes")
	}
}

// TestFormatWithoutColors tests that no ANSI codes appear with ColorNever
func TestFormatWithoutColors(t *testing.T) {
	s := New(4)
	s.SetFileInfo("test.bin", "PE", []string{".data", ".rdata"})
	config := extractor.Config{Encoding: "s"}
	s.Add([]byte("test string"), "test.bin", 0x1000, config)

	var buf bytes.Buffer
	s.Format(&buf, extractor.ColorNever)

	output := buf.String()

	// Check for no ANSI codes
	if strings.Contains(output, "\x1b[") {
		t.Error("Format() with ColorNever should not contain ANSI color codes")
	}

	// Verify content is still present
	if !strings.Contains(output, "Statistics for test.bin:") {
		t.Error("Format() should contain header even without colors")
	}
	if !strings.Contains(output, "PE") {
		t.Error("Format() should contain binary format")
	}
}

// TestFormatColorModes tests all color modes
func TestFormatColorModes(t *testing.T) {
	s := New(4)
	config := extractor.Config{Encoding: "s"}
	s.Add([]byte("test"), "test.bin", 0x1000, config)

	tests := []struct {
		name          string
		mode          extractor.ColorMode
		shouldHaveESC bool
	}{
		{"ColorNever", extractor.ColorNever, false},
		{"ColorAlways", extractor.ColorAlways, true},
		// ColorAuto would depend on TTY, skip in test
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			s.Format(&buf, tt.mode)
			output := buf.String()

			hasESC := strings.Contains(output, "\x1b[")
			if hasESC != tt.shouldHaveESC {
				t.Errorf("Format() with %s: hasESC=%v, want %v", tt.name, hasESC, tt.shouldHaveESC)
			}
		})
	}
}

// TestFormatColoredElements tests that specific elements are colored correctly
func TestFormatColoredElements(t *testing.T) {
	s := New(4)
	s.SetFileInfo("binary.exe", "PE", []string{".data", ".rdata"})
	s.UnfilteredCount = 100
	s.FilteredCount = 75
	s.TotalStrings = 75
	s.TotalBytes = 1500
	s.MaxLength = 50
	s.EncodingCounts["ascii-7bit"] = 70
	s.EncodingCounts["utf-8"] = 5
	s.LengthBuckets["11-50"] = 75

	var buf bytes.Buffer
	s.Format(&buf, extractor.ColorAlways)

	output := buf.String()

	// Verify different color codes are present for different elements
	colorCodes := []struct {
		name string
		code string
	}{
		{"Bold Cyan (headers)", "\x1b[1m\x1b[36m"},
		{"Yellow (numbers)", "\x1b[33m"},
		{"Green (percentages)", "\x1b[32m"},
		{"Magenta (encoding names)", "\x1b[35m"},
		{"Cyan (file metadata)", "\x1b[36m"},
	}

	for _, cc := range colorCodes {
		if !strings.Contains(output, cc.code) {
			t.Errorf("Format() should contain %s (%s)", cc.name, cc.code)
		}
	}
}
