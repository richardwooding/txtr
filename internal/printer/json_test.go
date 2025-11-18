package printer

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/richardwooding/txtr/internal/extractor"
)

func TestJSONPrinter(t *testing.T) {
	tests := []struct {
		name     string
		config   extractor.Config
		strings  []struct {
			value    string
			offset   int64
			filename string
		}
		wantStrings int
		wantBytes   int64
	}{
		{
			name: "basic ASCII strings",
			config: extractor.Config{
				MinLength: 4,
				Encoding:  "s",
			},
			strings: []struct {
				value    string
				offset   int64
				filename string
			}{
				{"Hello", 0, ""},
				{"World", 100, ""},
				{"test", 200, ""},
			},
			wantStrings: 3,
			wantBytes:   14,
		},
		{
			name: "with filename",
			config: extractor.Config{
				MinLength:     4,
				Encoding:      "s",
				PrintFileName: true,
			},
			strings: []struct {
				value    string
				offset   int64
				filename string
			}{
				{"test", 0, "file.bin"},
			},
			wantStrings: 1,
			wantBytes:   4,
		},
		{
			name: "UTF-16 encoding",
			config: extractor.Config{
				MinLength: 4,
				Encoding:  "l",
			},
			strings: []struct {
				value    string
				offset   int64
				filename string
			}{
				{"Hello", 0, ""},
			},
			wantStrings: 1,
			wantBytes:   5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			jp := NewJSONPrinter(tt.config, &buf)

			// Add test strings
			for _, s := range tt.strings {
				jp.PrintString([]byte(s.value), s.filename, s.offset, tt.config)
			}

			// Flush to get JSON output
			if err := jp.Flush(); err != nil {
				t.Fatalf("Flush() error = %v", err)
			}

			// Parse JSON output
			var output JSONOutput
			if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
				t.Fatalf("JSON unmarshal error = %v, output = %s", err, buf.String())
			}

			// Verify summary
			if output.Summary.TotalStrings != tt.wantStrings {
				t.Errorf("TotalStrings = %d, want %d", output.Summary.TotalStrings, tt.wantStrings)
			}
			if output.Summary.TotalBytes != tt.wantBytes {
				t.Errorf("TotalBytes = %d, want %d", output.Summary.TotalBytes, tt.wantBytes)
			}
			if output.Summary.MinLength != tt.config.MinLength {
				t.Errorf("MinLength = %d, want %d", output.Summary.MinLength, tt.config.MinLength)
			}

			// Verify strings
			if len(output.Files) == 0 {
				t.Fatal("No files in output")
			}
			if len(output.Files[0].Strings) != tt.wantStrings {
				t.Errorf("Number of strings = %d, want %d", len(output.Files[0].Strings), tt.wantStrings)
			}

			// Verify string values
			for i, s := range tt.strings {
				if i >= len(output.Files[0].Strings) {
					break
				}
				result := output.Files[0].Strings[i]
				if result.Value != s.value {
					t.Errorf("String[%d].Value = %q, want %q", i, result.Value, s.value)
				}
				if result.Offset != s.offset {
					t.Errorf("String[%d].Offset = %d, want %d", i, result.Offset, s.offset)
				}
				if result.Length != len(s.value) {
					t.Errorf("String[%d].Length = %d, want %d", i, result.Length, len(s.value))
				}
			}
		})
	}
}

func TestJSONPrinterWithFileInfo(t *testing.T) {
	var buf bytes.Buffer
	config := extractor.Config{
		MinLength:     4,
		Encoding:      "s",
		PrintFileName: true,
	}

	jp := NewJSONPrinter(config, &buf)
	jp.SetFileInfo("test.bin", "ELF", []string{".data", ".rodata"})

	jp.PrintString([]byte("test"), "test.bin", 0, config)

	if err := jp.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	var output JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("JSON unmarshal error = %v", err)
	}

	if len(output.Files) == 0 {
		t.Fatal("No files in output")
	}

	file := output.Files[0]
	if file.File != "test.bin" {
		t.Errorf("File = %q, want %q", file.File, "test.bin")
	}
	if file.Format != "ELF" {
		t.Errorf("Format = %q, want %q", file.Format, "ELF")
	}
	if len(file.Sections) != 2 {
		t.Errorf("Sections length = %d, want 2", len(file.Sections))
	}
}

func TestGetEncodingName(t *testing.T) {
	tests := []struct {
		encoding string
		want     string
	}{
		{"s", "ascii-7bit"},
		{"S", "ascii-8bit"},
		{"b", "utf-16be"},
		{"l", "utf-16le"},
		{"B", "utf-32be"},
		{"L", "utf-32le"},
		{"", "ascii-7bit"},
		{"invalid", "ascii-7bit"},
	}

	for _, tt := range tests {
		t.Run(tt.encoding, func(t *testing.T) {
			got := getEncodingName(tt.encoding)
			if got != tt.want {
				t.Errorf("getEncodingName(%q) = %q, want %q", tt.encoding, got, tt.want)
			}
		})
	}
}

func TestJSONPrinterNilWriter(t *testing.T) {
	// Should default to os.Stdout
	jp := NewJSONPrinter(extractor.Config{}, nil)
	if jp.writer != os.Stdout {
		t.Errorf("NewJSONPrinter with nil writer should default to os.Stdout")
	}
}

func TestJSONOutputValid(t *testing.T) {
	// Test that output is valid JSON
	var buf bytes.Buffer
	config := extractor.Config{
		MinLength: 4,
		Encoding:  "s",
	}

	jp := NewJSONPrinter(config, &buf)
	jp.PrintString([]byte("test"), "", 0, config)

	if err := jp.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	// Verify JSON is valid and properly formatted
	jsonStr := buf.String()
	if !strings.Contains(jsonStr, "\"files\"") {
		t.Error("JSON output missing 'files' field")
	}
	if !strings.Contains(jsonStr, "\"summary\"") {
		t.Error("JSON output missing 'summary' field")
	}
	if !strings.Contains(jsonStr, "\"total_strings\"") {
		t.Error("JSON output missing 'total_strings' field")
	}

	// Verify it can be unmarshaled
	var output JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}
}
