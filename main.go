package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
)

type Config struct {
	minLength     int
	printFileName bool
	radix         string
	printOffset   bool
}

func main() {
	config := Config{}

	flag.IntVar(&config.minLength, "n", 4, "Minimum string length")
	flag.IntVar(&config.minLength, "bytes", 4, "Minimum string length (alias)")
	flag.BoolVar(&config.printFileName, "f", false, "Print file name before each string")
	flag.BoolVar(&config.printFileName, "print-file-name", false, "Print file name before each string")
	flag.StringVar(&config.radix, "t", "", "Print offset in radix (o=octal, d=decimal, x=hex)")
	flag.StringVar(&config.radix, "radix", "", "Print offset in radix (o=octal, d=decimal, x=hex)")

	oFlag := flag.Bool("o", false, "Print offset in octal")

	flag.Parse()

	if *oFlag {
		config.radix = "o"
	}

	config.printOffset = config.radix != ""

	args := flag.Args()

	if len(args) == 0 {
		// Read from stdin
		extractStrings(os.Stdin, "", config)
	} else {
		// Process each file
		for _, filename := range args {
			file, err := os.Open(filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
				continue
			}
			extractStrings(file, filename, config)
			file.Close()
		}
	}
}

func extractStrings(reader io.Reader, filename string, config Config) {
	bufReader := bufio.NewReader(reader)
	var currentString []byte
	var offset int64
	var stringStartOffset int64

	for {
		b, err := bufReader.ReadByte()
		if err != nil {
			if err == io.EOF {
				// Print the last string if it meets the criteria
				if len(currentString) >= config.minLength {
					printString(currentString, filename, stringStartOffset, config)
				}
				break
			}
			fmt.Fprintf(os.Stderr, "strings: error reading: %v\n", err)
			return
		}

		if isPrintable(b) {
			if len(currentString) == 0 {
				stringStartOffset = offset
			}
			currentString = append(currentString, b)
		} else {
			// Non-printable character, check if we have a valid string
			if len(currentString) >= config.minLength {
				printString(currentString, filename, stringStartOffset, config)
			}
			currentString = currentString[:0]
		}

		offset++
	}
}

func isPrintable(b byte) bool {
	// A character is printable if it's a graphic character or space
	// ASCII printable range is 32-126 (space to tilde)
	return b >= 32 && b <= 126
}

func printString(str []byte, filename string, offset int64, config Config) {
	prefix := ""

	if config.printFileName && filename != "" {
		prefix = filename + ": "
	}

	if config.printOffset {
		var offsetStr string
		switch config.radix {
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

	fmt.Printf("%s%s\n", prefix, string(str))
}
