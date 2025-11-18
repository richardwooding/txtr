package extractor

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"golang.org/x/exp/mmap"
)

// shouldUseMmap determines if memory-mapped I/O should be used for the given file.
// It returns false if:
// - mmap is disabled via config
// - the file is below the threshold size
// - the file cannot be stat'd
// - the file is not a regular file (e.g., pipe, device)
func shouldUseMmap(path string, config Config) bool {
	// Check if mmap is disabled
	if config.DisableMmap {
		return false
	}

	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Only use mmap for regular files
	if !info.Mode().IsRegular() {
		return false
	}

	// Check if file size meets threshold
	return info.Size() >= config.MmapThreshold
}

// ExtractStringsFromFile extracts strings from a file, automatically choosing
// between memory-mapped I/O (for large files) or buffered I/O (for small files).
//
// This function provides transparent optimization - it will use mmap when
// beneficial and fall back to buffered I/O when appropriate.
func ExtractStringsFromFile(path string, config Config, printFunc func([]byte, string, int64, Config)) error {
	// Decide whether to use mmap
	if shouldUseMmap(path, config) {
		// Try mmap first
		err := extractStringsWithMmap(path, config, printFunc)
		if err == nil {
			return nil
		}
		// If mmap fails, fall back to buffered I/O
		// This can happen due to permissions, OS limits, etc.
	}

	// Fall back to traditional buffered I/O
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log error but don't override successful extraction
			fmt.Fprintf(os.Stderr, "warning: error closing file %s: %v\n", path, closeErr)
		}
	}()

	ExtractStrings(file, path, config, printFunc)
	return nil
}

// extractStringsWithMmap extracts strings using memory-mapped I/O.
// It uses the golang.org/x/exp/mmap package to map the file into memory
// and then delegates to the appropriate *FromBytes() function.
func extractStringsWithMmap(path string, config Config, printFunc func([]byte, string, int64, Config)) error {
	// Open the file with mmap
	reader, err := mmap.Open(path)
	if err != nil {
		return fmt.Errorf("error memory-mapping file: %w", err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			// Log error but don't override successful extraction
			fmt.Fprintf(os.Stderr, "warning: error closing mmap reader for %s: %v\n", path, closeErr)
		}
	}()

	// Get the file size
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error getting file info: %w", err)
	}
	fileSize := info.Size()

	// Read the entire file into memory
	// Note: mmap.ReaderAt implements ReadAt, we need to read into a slice
	data := make([]byte, fileSize)
	n, err := reader.ReadAt(data, 0)
	if err != nil && err != io.EOF {
		return fmt.Errorf("error reading memory-mapped file: %w", err)
	}
	data = data[:n]

	// Delegate to the appropriate extraction function based on encoding
	// These functions are already optimized for in-memory byte slices
	switch config.Encoding {
	case "s":
		// 7-bit ASCII
		if config.Unicode != "" && config.Unicode != "default" && config.Unicode != "invalid" {
			// UTF-8 aware mode
			extractUTF8AwareFromBytes(data, path, config, printFunc)
		} else {
			extractASCIIFromBytes(data, 0, path, config, printFunc, false)
		}
	case "S":
		// 8-bit ASCII
		extractASCIIFromBytes(data, 0, path, config, printFunc, true)
	case "b":
		// UTF-16 big-endian
		extractUTF16FromBytes(data, 0, path, config, printFunc, binary.BigEndian)
	case "l":
		// UTF-16 little-endian
		extractUTF16FromBytes(data, 0, path, config, printFunc, binary.LittleEndian)
	case "B":
		// UTF-32 big-endian
		extractUTF32FromBytes(data, 0, path, config, printFunc, binary.BigEndian)
	case "L":
		// UTF-32 little-endian
		extractUTF32FromBytes(data, 0, path, config, printFunc, binary.LittleEndian)
	default:
		return fmt.Errorf("unsupported encoding: %s", config.Encoding)
	}

	return nil
}

// extractUTF8AwareFromBytes is a helper that wraps the byte-slice extraction
// for UTF-8 aware mode. This function didn't exist before, so we create it here.
func extractUTF8AwareFromBytes(data []byte, filename string, config Config, printFunc func([]byte, string, int64, Config)) {
	// For UTF-8 aware mode, we need to process byte-by-byte like the streaming version
	// We can't use the simple ASCII extractor because we need UTF-8 validation
	var currentString []byte
	var startOffset int64

	for i := 0; i < len(data); {
		b := data[i]

		// Check if this is the start of a UTF-8 sequence
		if b >= 0x80 {
			// Multi-byte UTF-8 sequence
			seqLen := 0
			if b&0xE0 == 0xC0 {
				seqLen = 2
			} else if b&0xF0 == 0xE0 {
				seqLen = 3
			} else if b&0xF8 == 0xF0 {
				seqLen = 4
			}

			// Validate we have enough bytes
			if seqLen > 0 && i+seqLen <= len(data) {
				// Validate continuation bytes
				valid := true
				for j := 1; j < seqLen; j++ {
					if data[i+j]&0xC0 != 0x80 {
						valid = false
						break
					}
				}

				if valid {
					// Valid UTF-8 sequence - add to current string
					if len(currentString) == 0 {
						startOffset = int64(i)
					}
					currentString = append(currentString, data[i:i+seqLen]...)
					i += seqLen
					continue
				}
			}

			// Invalid UTF-8 - treat as non-printable
			if len(currentString) >= config.MinLength {
				if ShouldPrintString(currentString, config) {
					printFunc(currentString, filename, startOffset, config)
				}
			}
			currentString = currentString[:0]
			i++
			continue
		}

		// Single-byte character
		if isPrintableASCII(b, config.Encoding == "S", config.IncludeAllWhitespace) {
			if len(currentString) == 0 {
				startOffset = int64(i)
			}
			currentString = append(currentString, b)
		} else {
			// Non-printable character
			if len(currentString) >= config.MinLength {
				if ShouldPrintString(currentString, config) {
					printFunc(currentString, filename, startOffset, config)
				}
			}
			currentString = currentString[:0]
		}
		i++
	}

	// Handle any remaining string at EOF
	if len(currentString) >= config.MinLength {
		if ShouldPrintString(currentString, config) {
			printFunc(currentString, filename, startOffset, config)
		}
	}
}
