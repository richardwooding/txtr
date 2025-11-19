package binary

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestDetectMachOUniversal tests detection of Mach-O universal binaries
func TestDetectMachOUniversal(t *testing.T) {
	tests := []struct {
		name       string
		magic      uint32
		wantFormat Format
	}{
		{
			name:       "Universal binary big-endian magic",
			magic:      0xcafebabe,
			wantFormat: FormatMachO,
		},
		{
			name:       "Universal binary little-endian magic",
			magic:      0xbebafeca,
			wantFormat: FormatMachO,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file with magic number
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.bin")

			f, err := os.Create(tmpFile)
			if err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			// Write magic number
			if err := binary.Write(f, binary.BigEndian, tt.magic); err != nil {
				t.Fatalf("failed to write magic: %v", err)
			}

			// Add some padding to make it look more like a real file
			padding := make([]byte, 100)
			if _, err := f.Write(padding); err != nil {
				t.Fatalf("failed to write padding: %v", err)
			}

			if err := f.Close(); err != nil {
				t.Fatalf("failed to close file: %v", err)
			}

			// Test detection
			format, err := DetectFormat(tmpFile)
			if err != nil {
				t.Errorf("DetectFormat() error = %v", err)
				return
			}

			if format != tt.wantFormat {
				t.Errorf("DetectFormat() = %v, want %v", format, tt.wantFormat)
			}
		})
	}
}

// TestDetectMachOUniversalRealBinary tests with real macOS system binary if available
func TestDetectMachOUniversalRealBinary(t *testing.T) {
	// Only run on macOS
	if runtime.GOOS != "darwin" {
		t.Skip("skipping test: only runs on macOS (darwin)")
	}

	// Test with /bin/ls if it exists (macOS system binary)
	lsPath := "/bin/ls"
	if _, err := os.Stat(lsPath); os.IsNotExist(err) {
		t.Skip("skipping test: /bin/ls not found")
	}

	format, err := DetectFormat(lsPath)
	if err != nil {
		t.Fatalf("DetectFormat(/bin/ls) error = %v", err)
	}

	if format != FormatMachO {
		t.Errorf("DetectFormat(/bin/ls) = %v, want FormatMachO", format)
	}
}

// TestParseMachOUniversalRealBinary tests parsing of real universal binary if available
func TestParseMachOUniversalRealBinary(t *testing.T) {
	// Only run on macOS
	if runtime.GOOS != "darwin" {
		t.Skip("skipping test: only runs on macOS (darwin)")
	}

	// Test with /bin/ls if it exists (macOS system binary)
	lsPath := "/bin/ls"
	if _, err := os.Stat(lsPath); os.IsNotExist(err) {
		t.Skip("skipping test: /bin/ls not found")
	}

	sections, err := ParseMachO(lsPath)
	if err != nil {
		t.Fatalf("ParseMachO(/bin/ls) error = %v", err)
	}

	if len(sections) == 0 {
		t.Error("ParseMachO(/bin/ls) returned no sections, expected at least one")
	}

	// Verify we got data sections
	foundDataSection := false
	for _, sect := range sections {
		t.Logf("Found section: %s (offset: %d, size: %d)", sect.Name, sect.Offset, sect.Size)
		if sect.Name == "__TEXT.__cstring" || sect.Name == "__DATA.__data" {
			foundDataSection = true
		}
		if len(sect.Data) == 0 {
			t.Errorf("Section %s has no data", sect.Name)
		}
	}

	if !foundDataSection {
		t.Error("Expected to find at least one data section like __TEXT.__cstring or __DATA.__data")
	}
}

// TestFormatString tests the String() method of Format
func TestFormatString(t *testing.T) {
	tests := []struct {
		format Format
		want   string
	}{
		{FormatELF, "ELF"},
		{FormatPE, "PE"},
		{FormatMachO, "Mach-O"},
		{FormatRaw, "Raw"},
		{FormatUnknown, "Unknown"},
		{Format(999), "Unknown"}, // Invalid format
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.format.String(); got != tt.want {
				t.Errorf("Format.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
