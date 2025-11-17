package extractor

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

// Config holds the configuration for string extraction
type Config struct {
	MinLength     int
	PrintFileName bool
	Radix         string
	PrintOffset   bool
}

// ExtractStrings reads from reader and extracts printable strings
func ExtractStrings(reader io.Reader, filename string, config Config, printFunc func([]byte, string, int64, Config)) {
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

		if IsPrintable(b) {
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

// IsPrintable returns true if the byte is a printable ASCII character
func IsPrintable(b byte) bool {
	// A character is printable if it's a graphic character or space
	// ASCII printable range is 32-126 (space to tilde)
	return b >= 32 && b <= 126
}
