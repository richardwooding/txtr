package extractor

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

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
func ExtractStringsFromFile(ctx context.Context, path string, config Config, printFunc printFunc) error {
	// Decide whether to use mmap
	if shouldUseMmap(path, config) {
		// Try mmap first
		err := extractStringsWithMmap(ctx, path, config, printFunc)
		// Success or cancellation: return as-is. Only a genuine mmap failure
		// (permissions, OS limits, etc.) should trigger the buffered fallback —
		// a cancelled context must not be retried.
		if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		fmt.Fprintf(os.Stderr, "warning: mmap failed for %s: %v, falling back to buffered I/O\n", path, err)
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

	return ExtractStrings(ctx, file, path, config, printFunc)
}

// extractStringsWithMmap extracts strings using memory-mapped I/O.
// It uses the golang.org/x/exp/mmap package to map the file into memory
// and then delegates to the appropriate *FromBytes() function.
func extractStringsWithMmap(ctx context.Context, path string, config Config, printFunc printFunc) error {
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

	// Get the file size from the mmap reader (avoids redundant syscall)
	fileSize := int64(reader.Len())

	// Bail out before the (uninterruptible) bulk read if already cancelled.
	if err := ctx.Err(); err != nil {
		return err
	}

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
			return extractUTF8AwareFromBytes(ctx, data, path, config, printFunc)
		}
		return extractASCIIFromBytes(ctx, data, 0, path, config, printFunc, false)
	case "S":
		// 8-bit ASCII
		return extractASCIIFromBytes(ctx, data, 0, path, config, printFunc, true)
	case "b":
		// UTF-16 big-endian
		return extractUTF16FromBytes(ctx, data, 0, path, config, printFunc, binary.BigEndian)
	case "l":
		// UTF-16 little-endian
		return extractUTF16FromBytes(ctx, data, 0, path, config, printFunc, binary.LittleEndian)
	case "B":
		// UTF-32 big-endian
		return extractUTF32FromBytes(ctx, data, 0, path, config, printFunc, binary.BigEndian)
	case "L":
		// UTF-32 little-endian
		return extractUTF32FromBytes(ctx, data, 0, path, config, printFunc, binary.LittleEndian)
	default:
		return fmt.Errorf("unsupported encoding: %s", config.Encoding)
	}
}

// extractUTF8AwareFromBytes is a helper that wraps the byte-slice extraction
// for UTF-8 aware mode. This function didn't exist before, so we create it here.
func extractUTF8AwareFromBytes(ctx context.Context, data []byte, filename string, config Config, printFunc printFunc) error {
	// For UTF-8 aware mode, we need to process byte-by-byte like the streaming version
	// We can't use the simple ASCII extractor because we need UTF-8 validation
	var currentString []byte
	var startOffset int64
	nextCheck := 0 // index at which to next poll ctx (variable-stride loop)

	for i := 0; i < len(data); {
		if i >= nextCheck {
			if err := ctx.Err(); err != nil {
				return err
			}
			nextCheck = i + ctxChunk
		}
		b := data[i]

		// Check if this is the start of a UTF-8 sequence
		if b >= 0x80 {
			// Decode the UTF-8 rune using standard library
			r, size := utf8.DecodeRune(data[i:])

			// Check if the rune is valid (not replacement character due to invalid UTF-8)
			if r != utf8.RuneError || size == 1 {
				// Valid UTF-8 sequence - add to current string
				if len(currentString) == 0 {
					startOffset = int64(i)
				}
				currentString = append(currentString, data[i:i+size]...)
				i += size
				continue
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
	return nil
}
