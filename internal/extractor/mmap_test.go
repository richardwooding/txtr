package extractor

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestShouldUseMmap tests the logic for determining when mmap should be used
func TestShouldUseMmap(t *testing.T) {
	// Create temporary test files of different sizes
	tmpDir := t.TempDir()
	smallFile := filepath.Join(tmpDir, "small.bin")
	largeFile := filepath.Join(tmpDir, "large.bin")

	// Create a small file (1KB)
	if err := os.WriteFile(smallFile, bytes.Repeat([]byte("test"), 256), 0644); err != nil {
		t.Fatalf("Failed to create small file: %v", err)
	}

	// Create a large file (20MB)
	largeData := bytes.Repeat([]byte("test"), 5*1024*1024) // 20MB
	if err := os.WriteFile(largeFile, largeData, 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		config    Config
		wantMmap  bool
	}{
		{
			name: "Large file with mmap enabled",
			path: largeFile,
			config: Config{
				DisableMmap:   false,
				MmapThreshold: 1 * 1024 * 1024, // 1MB
			},
			wantMmap: true,
		},
		{
			name: "Small file below threshold",
			path: smallFile,
			config: Config{
				DisableMmap:   false,
				MmapThreshold: 1 * 1024 * 1024, // 1MB
			},
			wantMmap: false,
		},
		{
			name: "Large file with mmap disabled",
			path: largeFile,
			config: Config{
				DisableMmap:   true,
				MmapThreshold: 1 * 1024 * 1024, // 1MB
			},
			wantMmap: false,
		},
		{
			name: "Nonexistent file",
			path: filepath.Join(tmpDir, "nonexistent.bin"),
			config: Config{
				DisableMmap:   false,
				MmapThreshold: 1 * 1024 * 1024, // 1MB
			},
			wantMmap: false,
		},
		{
			name: "Small threshold makes small file use mmap",
			path: smallFile,
			config: Config{
				DisableMmap:   false,
				MmapThreshold: 512, // 512 bytes
			},
			wantMmap: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldUseMmap(tt.path, tt.config)
			if got != tt.wantMmap {
				t.Errorf("shouldUseMmap() = %v, want %v", got, tt.wantMmap)
			}
		})
	}
}

// TestExtractStringsFromFile tests the high-level wrapper function
func TestExtractStringsFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")

	// Create test file with known strings
	testData := []byte("Hello\x00\x00World\x00\x00Test")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name           string
		config         Config
		expectedStrs   []string
		wantErr        bool
	}{
		{
			name: "Normal extraction with mmap disabled",
			config: Config{
				MinLength:     4,
				Encoding:      "s",
				DisableMmap:   true,
				MmapThreshold: 1 * 1024 * 1024,
			},
			expectedStrs: []string{"Hello", "World", "Test"},
			wantErr:      false,
		},
		{
			name: "Normal extraction with mmap enabled (below threshold)",
			config: Config{
				MinLength:     4,
				Encoding:      "s",
				DisableMmap:   false,
				MmapThreshold: 1 * 1024 * 1024, // 1MB, file is smaller
			},
			expectedStrs: []string{"Hello", "World", "Test"},
			wantErr:      false,
		},
		{
			name: "Normal extraction with low mmap threshold (uses mmap)",
			config: Config{
				MinLength:     4,
				Encoding:      "s",
				DisableMmap:   false,
				MmapThreshold: 10, // Very low threshold to trigger mmap
			},
			expectedStrs: []string{"Hello", "World", "Test"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var collectedStrings []string
			printFunc := func(str []byte, _ string, _ int64, _ Config) {
				collectedStrings = append(collectedStrings, string(str))
			}

			err := ExtractStringsFromFile(testFile, tt.config, printFunc)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractStringsFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(collectedStrings) != len(tt.expectedStrs) {
					t.Errorf("Got %d strings, want %d", len(collectedStrings), len(tt.expectedStrs))
				}

				for i, expected := range tt.expectedStrs {
					if i >= len(collectedStrings) {
						t.Errorf("Missing string at index %d: %s", i, expected)
						continue
					}
					if collectedStrings[i] != expected {
						t.Errorf("String %d: got %q, want %q", i, collectedStrings[i], expected)
					}
				}
			}
		})
	}
}

// TestMmapEquivalence verifies that mmap produces identical output to buffered I/O
func TestMmapEquivalence(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "equivalence.bin")

	// Create test file with mixed content
	testData := []byte("String1\x00\x01\x02String2\x00\x00\xFFString3\nNewline\tTab\r\nCRLF")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	encodings := []string{"s", "S"}

	for _, encoding := range encodings {
		t.Run("Encoding_"+encoding, func(t *testing.T) {
			baseConfig := Config{
				MinLength: 4,
				Encoding:  encoding,
			}

			// Extract with mmap disabled (buffered I/O)
			var bufferedStrings []string
			configBuffered := baseConfig
			configBuffered.DisableMmap = true
			configBuffered.MmapThreshold = 10 * 1024 * 1024

			printFuncBuffered := func(str []byte, _ string, _ int64, _ Config) {
				bufferedStrings = append(bufferedStrings, string(str))
			}

			if err := ExtractStringsFromFile(testFile, configBuffered, printFuncBuffered); err != nil {
				t.Fatalf("Buffered extraction failed: %v", err)
			}

			// Extract with mmap enabled (low threshold)
			var mmapStrings []string
			configMmap := baseConfig
			configMmap.DisableMmap = false
			configMmap.MmapThreshold = 10 // Very low threshold

			printFuncMmap := func(str []byte, _ string, _ int64, _ Config) {
				mmapStrings = append(mmapStrings, string(str))
			}

			if err := ExtractStringsFromFile(testFile, configMmap, printFuncMmap); err != nil {
				t.Fatalf("Mmap extraction failed: %v", err)
			}

			// Compare results
			if len(bufferedStrings) != len(mmapStrings) {
				t.Errorf("String count mismatch: buffered=%d, mmap=%d",
					len(bufferedStrings), len(mmapStrings))
			}

			for i := 0; i < len(bufferedStrings) && i < len(mmapStrings); i++ {
				if bufferedStrings[i] != mmapStrings[i] {
					t.Errorf("String %d mismatch: buffered=%q, mmap=%q",
						i, bufferedStrings[i], mmapStrings[i])
				}
			}
		})
	}
}

// TestMmapFallback tests that errors in mmap fallback to buffered I/O
func TestMmapFallback(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "fallback.bin")

	// Create test file
	testData := []byte("Test String")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := Config{
		MinLength:     4,
		Encoding:      "s",
		DisableMmap:   false,
		MmapThreshold: 1, // Very low to trigger mmap attempt
	}

	var extracted []string
	printFunc := func(str []byte, _ string, _ int64, _ Config) {
		extracted = append(extracted, string(str))
	}

	// This should succeed even if mmap fails, as it falls back to buffered I/O
	if err := ExtractStringsFromFile(testFile, config, printFunc); err != nil {
		t.Errorf("ExtractStringsFromFile() should fallback on mmap failure, but got error: %v", err)
	}

	if len(extracted) == 0 {
		t.Error("Expected to extract strings after fallback, got none")
	}
}

// TestMmapWithUTF16 tests mmap with UTF-16 encoding
func TestMmapWithUTF16(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "utf16.bin")

	// Create UTF-16LE test data: "Test"
	testData := []byte{0x54, 0x00, 0x65, 0x00, 0x73, 0x00, 0x74, 0x00, 0x00, 0x00}
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := Config{
		MinLength:     4,
		Encoding:      "l", // UTF-16LE
		DisableMmap:   false,
		MmapThreshold: 1, // Force mmap
	}

	var extracted []string
	printFunc := func(str []byte, _ string, _ int64, _ Config) {
		extracted = append(extracted, string(str))
	}

	if err := ExtractStringsFromFile(testFile, config, printFunc); err != nil {
		t.Fatalf("ExtractStringsFromFile() failed: %v", err)
	}

	if len(extracted) != 1 || extracted[0] != "Test" {
		t.Errorf("Expected [Test], got %v", extracted)
	}
}

// TestMmapWithUTF32 tests mmap with UTF-32 encoding
func TestMmapWithUTF32(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "utf32.bin")

	// Create UTF-32LE test data: "Test"
	testData := []byte{
		0x54, 0x00, 0x00, 0x00, // T
		0x65, 0x00, 0x00, 0x00, // e
		0x73, 0x00, 0x00, 0x00, // s
		0x74, 0x00, 0x00, 0x00, // t
		0x00, 0x00, 0x00, 0x00, // null
	}
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := Config{
		MinLength:     4,
		Encoding:      "L", // UTF-32LE
		DisableMmap:   false,
		MmapThreshold: 1, // Force mmap
	}

	var extracted []string
	printFunc := func(str []byte, _ string, _ int64, _ Config) {
		extracted = append(extracted, string(str))
	}

	if err := ExtractStringsFromFile(testFile, config, printFunc); err != nil {
		t.Fatalf("ExtractStringsFromFile() failed: %v", err)
	}

	if len(extracted) != 1 || extracted[0] != "Test" {
		t.Errorf("Expected [Test], got %v", extracted)
	}
}

// TestMmapNonexistentFile tests error handling for nonexistent files
func TestMmapNonexistentFile(t *testing.T) {
	config := Config{
		MinLength:     4,
		Encoding:      "s",
		DisableMmap:   false,
		MmapThreshold: 1,
	}

	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	err := ExtractStringsFromFile("/nonexistent/file.bin", config, printFunc)
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}
