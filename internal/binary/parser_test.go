package binary

import (
	"os"
	"runtime"
	"testing"
)

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
