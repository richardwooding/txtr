// Package printer handles output formatting for extracted strings.
package printer

import (
	"os"

	"github.com/richardwooding/txtr/internal/extractor"
)

// ANSI color codes for terminal output.
const (
	// AnsiReset resets all color and style attributes.
	AnsiReset = "\x1b[0m"
	// AnsiCyan sets text color to cyan.
	AnsiCyan = "\x1b[36m"
	// AnsiYellow sets text color to yellow.
	AnsiYellow = "\x1b[33m"
	// AnsiGreen sets text color to green.
	AnsiGreen = "\x1b[32m"
	// AnsiMagenta sets text color to magenta.
	AnsiMagenta = "\x1b[35m"
	// AnsiDim sets text to dim/faint.
	AnsiDim = "\x1b[2m"
	// AnsiBold sets text to bold.
	AnsiBold = "\x1b[1m"
)

// ShouldUseColor determines if colored output should be used based on the mode,
// NO_COLOR environment variable, and whether stdout is a TTY.
func ShouldUseColor(mode extractor.ColorMode) bool {
	// Respect NO_COLOR environment variable (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	switch mode {
	case extractor.ColorNever:
		return false
	case extractor.ColorAlways:
		return true
	case extractor.ColorAuto:
		// Auto-detect if stdout is a terminal
		return isTerminal(os.Stdout)
	default:
		return false
	}
}

// isTerminal checks if the given file is a terminal.
// This is a simple implementation that checks the file descriptor.
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}

	// Get file info and check if it's a character device (TTY)
	fi, err := f.Stat()
	if err != nil {
		return false
	}

	// Check if it's a character device (mode & ModeCharDevice != 0)
	// This works on Unix-like systems
	mode := fi.Mode()
	return (mode & os.ModeCharDevice) != 0
}

// ColorString wraps a string with ANSI color codes if colors are enabled.
func ColorString(s, colorCode string, enabled bool) string {
	if !enabled || s == "" {
		return s
	}
	return colorCode + s + AnsiReset
}
