package printer

import (
	"fmt"

	"github.com/richardwooding/txtr/internal/extractor"
)

// PrintString formats and prints a string with optional filename and offset prefix
func PrintString(str []byte, filename string, offset int64, config extractor.Config) {
	prefix := ""

	if config.PrintFileName && filename != "" {
		prefix = filename + ": "
	}

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
		prefix += offsetStr
	}

	// Use custom output separator if specified, otherwise use newline
	separator := config.OutputSeparator
	if separator == "" {
		separator = "\n"
	}

	fmt.Printf("%s%s%s", prefix, string(str), separator)
}
