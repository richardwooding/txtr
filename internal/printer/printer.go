// Package printer provides functions for formatting and printing extracted strings
// with optional filename and offset prefixes.
package printer

import (
	"fmt"
	"io"

	"github.com/richardwooding/txtr/internal/extractor"
)

// PrintString formats and prints a string with optional filename and offset prefix
func PrintString(str []byte, filename string, offset int64, config extractor.Config) {
	// Determine if colors should be used
	useColor := ShouldUseColor(config.ColorMode)

	prefix := ""

	// Add filename prefix with color
	if config.PrintFileName && filename != "" {
		filenameStr := filename + ": "
		if useColor {
			filenameStr = ColorString(filename, AnsiBold+AnsiCyan, true) + ": "
		}
		prefix = filenameStr
	}

	// Add offset prefix with color
	if config.PrintOffset {
		var offsetStr string
		switch config.Radix {
		case "o":
			offsetStr = fmt.Sprintf("%7o ", offset)
		case "d":
			offsetStr = fmt.Sprintf("%7d ", offset)
		case "x":
			offsetStr = fmt.Sprintf("%7x ", offset)
		default:
			offsetStr = ""
		}
		if useColor && offsetStr != "" {
			// Color the offset yellow
			offsetStr = ColorString(offsetStr[:len(offsetStr)-1], AnsiYellow, true) + " "
		}
		prefix += offsetStr
	}

	// Determine string color based on encoding
	stringOutput := string(str)
	if useColor {
		switch config.Encoding {
		case "S": // 8-bit ASCII (high-byte)
			stringOutput = ColorString(stringOutput, AnsiMagenta, true)
		case "b", "l", "B", "L": // UTF-16 or UTF-32 (UTF-8 output)
			stringOutput = ColorString(stringOutput, AnsiGreen, true)
		case "s": // 7-bit ASCII
			// Check if UTF-8 mode is enabled for locale/escape/hex/highlight
			if config.Unicode != "" && config.Unicode != "default" && config.Unicode != "invalid" {
				// UTF-8 aware mode
				stringOutput = ColorString(stringOutput, AnsiGreen, true)
			}
			// Default: no color (white/default terminal color)
		}
	}

	// Use custom output separator if specified, otherwise use newline
	separator := config.OutputSeparator
	if separator == "" {
		separator = "\n"
	}
	if useColor && separator != "\n" {
		// Dim the separator if it's custom
		separator = ColorString(separator, AnsiDim, true)
	}

	fmt.Printf("%s%s%s", prefix, stringOutput, separator)
}

// PrintStringToWriter is like PrintString but writes to a specific io.Writer
func PrintStringToWriter(w io.Writer, str []byte, filename string, offset int64, config extractor.Config) {
	// Determine if colors should be used
	useColor := ShouldUseColor(config.ColorMode)

	prefix := ""

	// Add filename prefix with color
	if config.PrintFileName && filename != "" {
		filenameStr := filename + ": "
		if useColor {
			filenameStr = ColorString(filename, AnsiBold+AnsiCyan, true) + ": "
		}
		prefix = filenameStr
	}

	// Add offset prefix with color
	if config.PrintOffset {
		var offsetStr string
		switch config.Radix {
		case "o":
			offsetStr = fmt.Sprintf("%7o ", offset)
		case "d":
			offsetStr = fmt.Sprintf("%7d ", offset)
		case "x":
			offsetStr = fmt.Sprintf("%7x ", offset)
		default:
			offsetStr = ""
		}
		if useColor && offsetStr != "" {
			// Color the offset yellow
			offsetStr = ColorString(offsetStr[:len(offsetStr)-1], AnsiYellow, true) + " "
		}
		prefix += offsetStr
	}

	// Determine string color based on encoding
	stringOutput := string(str)
	if useColor {
		switch config.Encoding {
		case "S": // 8-bit ASCII (high-byte)
			stringOutput = ColorString(stringOutput, AnsiMagenta, true)
		case "b", "l", "B", "L": // UTF-16 or UTF-32 (UTF-8 output)
			stringOutput = ColorString(stringOutput, AnsiGreen, true)
		case "s": // 7-bit ASCII
			// Check if UTF-8 mode is enabled for locale/escape/hex/highlight
			if config.Unicode != "" && config.Unicode != "default" && config.Unicode != "invalid" {
				// UTF-8 aware mode
				stringOutput = ColorString(stringOutput, AnsiGreen, true)
			}
			// Default: no color (white/default terminal color)
		}
	}

	// Use custom output separator if specified, otherwise use newline
	separator := config.OutputSeparator
	if separator == "" {
		separator = "\n"
	}
	if useColor && separator != "\n" {
		// Dim the separator if it's custom
		separator = ColorString(separator, AnsiDim, true)
	}

	if _, err := fmt.Fprintf(w, "%s%s%s", prefix, stringOutput, separator); err != nil {
		// Error writing to writer, but we can't do much about it in this context
		// The caller should handle writer errors appropriately
		return
	}
}
