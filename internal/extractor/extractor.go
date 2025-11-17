package extractor

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"unicode/utf16"
	"unicode/utf8"
)

// Config holds the configuration for string extraction
type Config struct {
	MinLength            int
	PrintFileName        bool
	Radix                string
	PrintOffset          bool
	Encoding             string
	Unicode              string // UTF-8 handling mode: default/invalid/locale/escape/hex/highlight
	OutputSeparator      string
	IncludeAllWhitespace bool
	ScanAll              bool // Scan entire file
	ScanDataOnly         bool // Scan only data sections (requires binary format detection)
	TargetFormat         string // Target binary format: elf/pe/macho/binary
}

// ExtractStrings reads from reader and extracts printable strings
func ExtractStrings(reader io.Reader, filename string, config Config, printFunc func([]byte, string, int64, Config)) {
	switch config.Encoding {
	case "s": // 7-bit ASCII
		extractASCII(reader, filename, config, printFunc, false)
	case "S": // 8-bit ASCII
		extractASCII(reader, filename, config, printFunc, true)
	case "b": // 16-bit big-endian (UTF-16BE)
		extractUTF16(reader, filename, config, printFunc, binary.BigEndian)
	case "l": // 16-bit little-endian (UTF-16LE)
		extractUTF16(reader, filename, config, printFunc, binary.LittleEndian)
	case "B": // 32-bit big-endian (UTF-32BE)
		extractUTF32(reader, filename, config, printFunc, binary.BigEndian)
	case "L": // 32-bit little-endian (UTF-32LE)
		extractUTF32(reader, filename, config, printFunc, binary.LittleEndian)
	default:
		extractASCII(reader, filename, config, printFunc, false)
	}
}

// extractASCII extracts 7-bit or 8-bit ASCII strings
func extractASCII(reader io.Reader, filename string, config Config, printFunc func([]byte, string, int64, Config), allow8bit bool) {
	// If Unicode mode is not default/invalid, use UTF-8 aware extraction
	if config.Unicode != "default" && config.Unicode != "invalid" && config.Unicode != "" {
		extractUTF8Aware(reader, filename, config, printFunc)
		return
	}

	bufReader := bufio.NewReader(reader)
	var currentString []byte
	var offset int64
	var stringStartOffset int64

	for {
		b, err := bufReader.ReadByte()
		if err != nil {
			if err == io.EOF {
				// Print the last string if it meets the criteria
				if len(currentString) >= config.MinLength {
					printFunc(currentString, filename, stringStartOffset, config)
				}
				break
			}
			fmt.Fprintf(os.Stderr, "strings: error reading: %v\n", err)
			return
		}

		if isPrintableASCII(b, allow8bit, config.IncludeAllWhitespace) {
			if len(currentString) == 0 {
				stringStartOffset = offset
			}
			currentString = append(currentString, b)
		} else {
			// Non-printable character, check if we have a valid string
			if len(currentString) >= config.MinLength {
				printFunc(currentString, filename, stringStartOffset, config)
			}
			currentString = currentString[:0]
		}

		offset++
	}
}

// extractUTF8Aware extracts strings with UTF-8 awareness and special display modes
func extractUTF8Aware(reader io.Reader, filename string, config Config, printFunc func([]byte, string, int64, Config)) {
	bufReader := bufio.NewReader(reader)
	var currentString []byte
	var currentOutput []byte // May differ from currentString based on Unicode mode
	var offset int64
	var stringStartOffset int64

	for {
		b, err := bufReader.ReadByte()
		if err != nil {
			if err == io.EOF {
				// Print the last string if it meets the criteria
				if len(currentString) >= config.MinLength {
					printFunc(currentOutput, filename, stringStartOffset, config)
				}
				break
			}
			fmt.Fprintf(os.Stderr, "strings: error reading: %v\n", err)
			return
		}

		// Check if this starts a UTF-8 sequence
		if b < 128 {
			// ASCII character
			if isPrintableASCII(b, false, config.IncludeAllWhitespace) {
				if len(currentString) == 0 {
					stringStartOffset = offset
				}
				currentString = append(currentString, b)
				currentOutput = append(currentOutput, b)
			} else {
				// Non-printable, flush current string
				if len(currentString) >= config.MinLength {
					printFunc(currentOutput, filename, stringStartOffset, config)
				}
				currentString = currentString[:0]
				currentOutput = currentOutput[:0]
			}
		} else {
			// Potential UTF-8 multi-byte sequence
			runeBytes := []byte{b}
			expectedBytes := 0

			// Determine how many bytes this UTF-8 character should have
			if b&0xE0 == 0xC0 {
				expectedBytes = 1 // 2-byte sequence
			} else if b&0xF0 == 0xE0 {
				expectedBytes = 2 // 3-byte sequence
			} else if b&0xF8 == 0xF0 {
				expectedBytes = 3 // 4-byte sequence
			}

			// Read the continuation bytes
			valid := true
			for i := 0; i < expectedBytes; i++ {
				nextByte, err := bufReader.ReadByte()
				if err != nil || (nextByte&0xC0) != 0x80 {
					valid = false
					if err == nil {
						bufReader.UnreadByte()
					}
					break
				}
				runeBytes = append(runeBytes, nextByte)
				offset++
			}

			if valid && utf8.Valid(runeBytes) {
				r, _ := utf8.DecodeRune(runeBytes)
				if isPrintableRune(r, config.IncludeAllWhitespace) {
					if len(currentString) == 0 {
						stringStartOffset = offset - int64(len(runeBytes)) + 1
					}
					currentString = append(currentString, runeBytes...)

					// Format based on Unicode mode
					switch config.Unicode {
					case "locale":
						currentOutput = append(currentOutput, runeBytes...)
					case "escape":
						currentOutput = append(currentOutput, []byte(fmt.Sprintf("\\u%04x", r))...)
					case "hex":
						currentOutput = append(currentOutput, []byte(fmt.Sprintf("<%02x>", r))...)
					case "highlight":
						currentOutput = append(currentOutput, []byte(fmt.Sprintf("\033[1m\\u%04x\033[0m", r))...)
					default:
						currentOutput = append(currentOutput, runeBytes...)
					}
				} else {
					// Non-printable rune
					if len(currentString) >= config.MinLength {
						printFunc(currentOutput, filename, stringStartOffset, config)
					}
					currentString = currentString[:0]
					currentOutput = currentOutput[:0]
				}
			} else {
				// Invalid UTF-8 sequence, treat as non-printable
				if len(currentString) >= config.MinLength {
					printFunc(currentOutput, filename, stringStartOffset, config)
				}
				currentString = currentString[:0]
				currentOutput = currentOutput[:0]
			}
		}

		offset++
	}
}

// extractUTF16 extracts UTF-16 encoded strings
func extractUTF16(reader io.Reader, filename string, config Config, printFunc func([]byte, string, int64, Config), byteOrder binary.ByteOrder) {
	bufReader := bufio.NewReader(reader)
	var currentRunes []rune
	var offset int64
	var stringStartOffset int64

	for {
		var rawBytes [2]byte
		n, err := io.ReadFull(bufReader, rawBytes[:])
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				// Print the last string if it meets the criteria
				if len(currentRunes) >= config.MinLength {
					printFunc([]byte(string(currentRunes)), filename, stringStartOffset, config)
				}
				break
			}
			fmt.Fprintf(os.Stderr, "strings: error reading: %v\n", err)
			return
		}

		if n == 2 {
			u16 := byteOrder.Uint16(rawBytes[:])
			r := rune(u16)

			// Handle surrogate pairs
			if utf16.IsSurrogate(r) {
				var nextBytes [2]byte
				n2, err2 := io.ReadFull(bufReader, nextBytes[:])
				if err2 == nil && n2 == 2 {
					u16_2 := byteOrder.Uint16(nextBytes[:])
					r = utf16.DecodeRune(r, rune(u16_2))
					offset += 2
				}
			}

			if isPrintableRune(r, config.IncludeAllWhitespace) {
				if len(currentRunes) == 0 {
					stringStartOffset = offset
				}
				currentRunes = append(currentRunes, r)
			} else {
				if len(currentRunes) >= config.MinLength {
					printFunc([]byte(string(currentRunes)), filename, stringStartOffset, config)
				}
				currentRunes = currentRunes[:0]
			}

			offset += 2
		}
	}
}

// extractUTF32 extracts UTF-32 encoded strings
func extractUTF32(reader io.Reader, filename string, config Config, printFunc func([]byte, string, int64, Config), byteOrder binary.ByteOrder) {
	bufReader := bufio.NewReader(reader)
	var currentRunes []rune
	var offset int64
	var stringStartOffset int64

	for {
		var rawBytes [4]byte
		n, err := io.ReadFull(bufReader, rawBytes[:])
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				// Print the last string if it meets the criteria
				if len(currentRunes) >= config.MinLength {
					printFunc([]byte(string(currentRunes)), filename, stringStartOffset, config)
				}
				break
			}
			fmt.Fprintf(os.Stderr, "strings: error reading: %v\n", err)
			return
		}

		if n == 4 {
			u32 := byteOrder.Uint32(rawBytes[:])
			r := rune(u32)

			if isPrintableRune(r, config.IncludeAllWhitespace) && utf8.ValidRune(r) {
				if len(currentRunes) == 0 {
					stringStartOffset = offset
				}
				currentRunes = append(currentRunes, r)
			} else {
				if len(currentRunes) >= config.MinLength {
					printFunc([]byte(string(currentRunes)), filename, stringStartOffset, config)
				}
				currentRunes = currentRunes[:0]
			}

			offset += 4
		}
	}
}

// IsPrintable returns true if the byte is a printable ASCII character (7-bit)
func IsPrintable(b byte) bool {
	return isPrintableASCII(b, false, false)
}

// isPrintableASCII checks if a byte is printable with options for 8-bit and whitespace
func isPrintableASCII(b byte, allow8bit bool, includeAllWhitespace bool) bool {
	// Include all whitespace if requested
	if includeAllWhitespace && (b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f') {
		return true
	}

	// 7-bit ASCII printable range is 32-126 (space to tilde)
	if b >= 32 && b <= 126 {
		return true
	}

	// 8-bit ASCII includes 128-255
	if allow8bit && b >= 128 {
		return true
	}

	return false
}

// isPrintableRune checks if a rune is printable
func isPrintableRune(r rune, includeAllWhitespace bool) bool {
	// Include all whitespace if requested
	if includeAllWhitespace && (r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f') {
		return true
	}

	// Check if rune is printable (graphic character or space)
	// Basic ASCII printable range
	if r >= 32 && r <= 126 {
		return true
	}

	// Extended Unicode printable characters
	if r >= 0xA0 && r <= 0xD7FF {
		return true
	}
	if r >= 0xE000 && r <= 0xFFFD {
		return true
	}
	if r >= 0x10000 && r <= 0x10FFFF {
		return true
	}

	return false
}

// ExtractFromSection extracts strings from a specific section's data
func ExtractFromSection(data []byte, sectionName string, sectionOffset int64, filename string, config Config, printFunc func([]byte, string, int64, Config)) {
	// Use appropriate extraction based on encoding
	switch config.Encoding {
	case "s": // 7-bit ASCII
		extractASCIIFromBytes(data, sectionOffset, filename, config, printFunc, false)
	case "S": // 8-bit ASCII
		extractASCIIFromBytes(data, sectionOffset, filename, config, printFunc, true)
	case "b": // UTF-16BE
		extractUTF16FromBytes(data, sectionOffset, filename, config, printFunc, binary.BigEndian)
	case "l": // UTF-16LE
		extractUTF16FromBytes(data, sectionOffset, filename, config, printFunc, binary.LittleEndian)
	case "B": // UTF-32BE
		extractUTF32FromBytes(data, sectionOffset, filename, config, printFunc, binary.BigEndian)
	case "L": // UTF-32LE
		extractUTF32FromBytes(data, sectionOffset, filename, config, printFunc, binary.LittleEndian)
	default:
		extractASCIIFromBytes(data, sectionOffset, filename, config, printFunc, false)
	}
}

// extractASCIIFromBytes is a helper for extracting from byte slices
func extractASCIIFromBytes(data []byte, baseOffset int64, filename string, config Config, printFunc func([]byte, string, int64, Config), allow8bit bool) {
	var currentString []byte
	var stringStartOffset int64

	for i, b := range data {
		if isPrintableASCII(b, allow8bit, config.IncludeAllWhitespace) {
			if len(currentString) == 0 {
				stringStartOffset = baseOffset + int64(i)
			}
			currentString = append(currentString, b)
		} else {
			if len(currentString) >= config.MinLength {
				printFunc(currentString, filename, stringStartOffset, config)
			}
			currentString = currentString[:0]
		}
	}

	// Handle last string
	if len(currentString) >= config.MinLength {
		printFunc(currentString, filename, stringStartOffset, config)
	}
}

// extractUTF16FromBytes extracts UTF-16 from byte slice
func extractUTF16FromBytes(data []byte, baseOffset int64, filename string, config Config, printFunc func([]byte, string, int64, Config), byteOrder binary.ByteOrder) {
	var currentRunes []rune
	var stringStartOffset int64

	for i := 0; i < len(data)-1; i += 2 {
		u16 := byteOrder.Uint16(data[i : i+2])
		r := rune(u16)

		if isPrintableRune(r, config.IncludeAllWhitespace) {
			if len(currentRunes) == 0 {
				stringStartOffset = baseOffset + int64(i)
			}
			currentRunes = append(currentRunes, r)
		} else {
			if len(currentRunes) >= config.MinLength {
				printFunc([]byte(string(currentRunes)), filename, stringStartOffset, config)
			}
			currentRunes = currentRunes[:0]
		}
	}

	if len(currentRunes) >= config.MinLength {
		printFunc([]byte(string(currentRunes)), filename, stringStartOffset, config)
	}
}

// extractUTF32FromBytes extracts UTF-32 from byte slice
func extractUTF32FromBytes(data []byte, baseOffset int64, filename string, config Config, printFunc func([]byte, string, int64, Config), byteOrder binary.ByteOrder) {
	var currentRunes []rune
	var stringStartOffset int64

	for i := 0; i < len(data)-3; i += 4 {
		u32 := byteOrder.Uint32(data[i : i+4])
		r := rune(u32)

		if isPrintableRune(r, config.IncludeAllWhitespace) && utf8.ValidRune(r) {
			if len(currentRunes) == 0 {
				stringStartOffset = baseOffset + int64(i)
			}
			currentRunes = append(currentRunes, r)
		} else {
			if len(currentRunes) >= config.MinLength {
				printFunc([]byte(string(currentRunes)), filename, stringStartOffset, config)
			}
			currentRunes = currentRunes[:0]
		}
	}

	if len(currentRunes) >= config.MinLength {
		printFunc([]byte(string(currentRunes)), filename, stringStartOffset, config)
	}
}
