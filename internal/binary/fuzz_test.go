package binary

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzParseELF tests ELF binary parsing with random inputs
func FuzzParseELF(f *testing.F) {
	// Seed corpus: minimal valid ELF headers and malformed data
	// ELF magic: \x7fELF
	f.Add([]byte("\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00")) // Minimal ELF header
	f.Add([]byte("\x7fELF\x01\x01\x01\x00"))                                   // Short ELF header
	f.Add([]byte("\x7fELF"))                                                   // Just magic
	f.Add([]byte("not an elf file"))                                           // Invalid
	f.Add([]byte(""))                                                           // Empty
	f.Add([]byte("\x7fELF\xff\xff\xff\xff"))                                   // ELF with invalid fields

	f.Fuzz(func(t *testing.T, data []byte) {
		// Skip extremely large inputs to prevent resource exhaustion
		if len(data) > 10*1024*1024 { // 10MB limit
			t.Skip("Input too large")
		}

		// Create temporary file from fuzz data
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "fuzz.elf")

		if err := os.WriteFile(tmpFile, data, 0600); err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}

		// Should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Panic: %v\nInput: %q", r, data)
			}
		}()

		// Parse ELF file - errors are expected for invalid input
		sections, err := ParseELF(tmpFile)

		// Invariant 1: If no error, sections must be valid
		if err == nil {
			for i, sect := range sections {
				// Section must have a name
				if sect.Name == "" {
					t.Errorf("Section #%d has empty name", i)
				}
				// Size must match data length
				if sect.Size != int64(len(sect.Data)) {
					t.Errorf("Section #%d size mismatch: Size=%d, len(Data)=%d",
						i, sect.Size, len(sect.Data))
				}
				// Offset should be non-negative
				if sect.Offset < 0 {
					t.Errorf("Section #%d has negative offset: %d", i, sect.Offset)
				}
			}
		}

		// Invariant 2: Function should always return (no infinite loops)
		// (implicitly tested by lack of timeout)
	})
}

// FuzzParsePE tests PE (Windows) binary parsing with random inputs
func FuzzParsePE(f *testing.F) {
	// Seed corpus: PE magic signatures and malformed data
	// PE magic: "MZ" at start, "PE\x00\x00" later
	f.Add([]byte("MZ"))                                                          // DOS stub
	f.Add([]byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff\x00\x00")) // Extended DOS header
	f.Add([]byte("PE\x00\x00"))                                                  // PE signature only
	f.Add([]byte("not a pe file"))                                               // Invalid
	f.Add([]byte(""))                                                             // Empty
	f.Add([]byte("MZ\xff\xff\xff\xff"))                                          // MZ with invalid fields

	f.Fuzz(func(t *testing.T, data []byte) {
		// Skip extremely large inputs
		if len(data) > 10*1024*1024 {
			t.Skip("Input too large")
		}

		// Create temporary file
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "fuzz.exe")

		if err := os.WriteFile(tmpFile, data, 0600); err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}

		// Should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Panic: %v\nInput: %q", r, data)
			}
		}()

		// Parse PE file - errors are expected for invalid input
		sections, err := ParsePE(tmpFile)

		// Invariant 1: If no error, sections must be valid
		if err == nil {
			for i, sect := range sections {
				if sect.Name == "" {
					t.Errorf("Section #%d has empty name", i)
				}
				if sect.Size != int64(len(sect.Data)) {
					t.Errorf("Section #%d size mismatch: Size=%d, len(Data)=%d",
						i, sect.Size, len(sect.Data))
				}
				if sect.Offset < 0 {
					t.Errorf("Section #%d has negative offset: %d", i, sect.Offset)
				}
			}
		}

		// Invariant 2: Only .data or .rdata sections should be returned
		if err == nil {
			for i, sect := range sections {
				if sect.Name != ".data" && sect.Name != ".rdata" {
					t.Errorf("Section #%d has unexpected name: %s (expected .data or .rdata)",
						i, sect.Name)
				}
			}
		}
	})
}

// FuzzParseMachO tests Mach-O (macOS) binary parsing with random inputs
func FuzzParseMachO(f *testing.F) {
	// Seed corpus: Mach-O magic signatures
	// Mach-O magics: 0xfeedface (32-bit), 0xfeedfacf (64-bit), 0xcafebabe (universal/fat)
	f.Add([]byte("\xfe\xed\xfa\xce"))                   // 32-bit big-endian magic
	f.Add([]byte("\xce\xfa\xed\xfe"))                   // 32-bit little-endian magic
	f.Add([]byte("\xfe\xed\xfa\xcf"))                   // 64-bit big-endian magic
	f.Add([]byte("\xcf\xfa\xed\xfe"))                   // 64-bit little-endian magic
	f.Add([]byte("\xca\xfe\xba\xbe"))                   // Universal binary magic
	f.Add([]byte("not a macho file"))                   // Invalid
	f.Add([]byte(""))                                    // Empty
	f.Add([]byte("\xfe\xed\xfa\xce\xff\xff\xff\xff"))  // Magic with invalid fields

	f.Fuzz(func(t *testing.T, data []byte) {
		// Skip extremely large inputs
		if len(data) > 10*1024*1024 {
			t.Skip("Input too large")
		}

		// Create temporary file
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "fuzz.macho")

		if err := os.WriteFile(tmpFile, data, 0600); err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}

		// Should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Panic: %v\nInput: %q", r, data)
			}
		}()

		// Parse Mach-O file - errors are expected for invalid input
		sections, err := ParseMachO(tmpFile)

		// Invariant 1: If no error, sections must be valid
		if err == nil {
			for i, sect := range sections {
				if sect.Name == "" {
					t.Errorf("Section #%d has empty name", i)
				}
				if sect.Size != int64(len(sect.Data)) {
					t.Errorf("Section #%d size mismatch: Size=%d, len(Data)=%d",
						i, sect.Size, len(sect.Data))
				}
				if sect.Offset < 0 {
					t.Errorf("Section #%d has negative offset: %d", i, sect.Offset)
				}
			}
		}

		// Invariant 2: Only known data sections should be returned
		if err == nil {
			validSections := map[string]bool{
				"__DATA.__data":    true,
				"__DATA.__const":   true,
				"__TEXT.__cstring": true,
				"__TEXT.__const":   true,
			}
			for i, sect := range sections {
				if !validSections[sect.Name] {
					t.Errorf("Section #%d has unexpected name: %s", i, sect.Name)
				}
			}
		}
	})
}

// FuzzDetectFormat tests binary format detection with random inputs
func FuzzDetectFormat(f *testing.F) {
	// Seed corpus: all magic signatures
	f.Add([]byte("\x7fELF"))                  // ELF
	f.Add([]byte("MZ"))                       // PE
	f.Add([]byte("\xfe\xed\xfa\xce"))        // Mach-O 32-bit BE
	f.Add([]byte("\xce\xfa\xed\xfe"))        // Mach-O 32-bit LE
	f.Add([]byte("\xfe\xed\xfa\xcf"))        // Mach-O 64-bit BE
	f.Add([]byte("\xcf\xfa\xed\xfe"))        // Mach-O 64-bit LE
	f.Add([]byte("\xca\xfe\xba\xbe"))        // Mach-O universal
	f.Add([]byte("random data"))              // Unknown
	f.Add([]byte(""))                         // Empty

	f.Fuzz(func(t *testing.T, data []byte) {
		// Skip extremely large inputs
		if len(data) > 10*1024*1024 {
			t.Skip("Input too large")
		}

		// Create temporary file
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "fuzz.bin")

		if err := os.WriteFile(tmpFile, data, 0600); err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}

		// Should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Panic: %v\nInput: %q", r, data)
			}
		}()

		// Detect format - errors are OK for invalid files
		format, err := DetectFormat(tmpFile)

		// If no error, format must be valid
		if err == nil {
			// Invariant 1: Format must be one of the known values
			validFormats := map[Format]bool{
				FormatELF:     true,
				FormatPE:      true,
				FormatMachO:   true,
				FormatRaw:     true,
				FormatUnknown: true,
			}
			if !validFormats[format] {
				t.Errorf("Invalid format returned: %v", format)
			}

			// Invariant 2: Format detection is deterministic
			format2, err2 := DetectFormat(tmpFile)
			if err2 != nil {
				t.Errorf("Second detection failed but first succeeded: %v", err2)
			}
			if format != format2 {
				t.Errorf("Non-deterministic format detection: first=%v, second=%v",
					format, format2)
			}
		}

		// Note: We don't test parsers here since they have their own fuzz tests
		// (FuzzParseELF, FuzzParsePE, FuzzParseMachO)
	})
}
