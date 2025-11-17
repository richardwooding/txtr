// Package main implements txtr, a GNU strings compatible utility for extracting
// printable strings from binary files.
package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/richardwooding/txtr/internal/binary"
	"github.com/richardwooding/txtr/internal/extractor"
	"github.com/richardwooding/txtr/internal/printer"
)

const version = "2.0.0"

// CLI defines the command-line interface structure
type CLI struct {
	MinLength            int      `short:"n" name:"bytes" default:"4" help:"Minimum string length"`
	PrintFileName        bool     `short:"f" name:"print-file-name" help:"Print file name before each string"`
	Radix                string   `short:"t" name:"radix" enum:"o,d,x," default:"" help:"Print offset in radix (o=octal, d=decimal, x=hex)"`
	OctalOffset          bool     `short:"o" help:"Print offset in octal (alias for -t o)"`
	Encoding             string   `short:"e" name:"encoding" enum:"s,S,b,l,B,L," default:"s" help:"Character encoding (s=7-bit, S=8-bit, b=16-bit BE, l=16-bit LE, B=32-bit BE, L=32-bit LE)"`
	Unicode              string   `short:"U" name:"unicode" enum:"default,invalid,locale,escape,hex,highlight," default:"default" help:"How to handle UTF-8 sequences (default/invalid/locale/escape/hex/highlight)"`
	OutputSeparator      string   `short:"s" name:"output-separator" default:"\\n" help:"Output record separator (default: newline)"`
	IncludeAllWhitespace bool     `short:"w" name:"include-all-whitespace" help:"Include all whitespace characters in strings"`
	ScanAll              bool     `short:"a" name:"all" help:"Scan entire file"`
	ScanDataOnly         bool     `short:"d" name:"data" help:"Scan only initialized data sections of binary files"`
	TargetFormat         string   `short:"T" name:"target" enum:"elf,pe,macho,binary," default:"" help:"Specify binary format (elf/pe/macho/binary)"`
	Version              bool     `short:"v" name:"version" help:"Display version information"`
	VersionAlt           bool     `short:"V" hidden:"" help:"Display version information (alias)"`
	Files                []string `arg:"" optional:"" name:"file" help:"Files to extract strings from" type:"path"`
}

func main() {
	var cli CLI

	kong.Parse(&cli,
		kong.Name("txtr"),
		kong.Description("Extract printable strings from binary files. GNU strings compatible."),
		kong.UsageOnError(),
	)

	// Handle version flag
	if cli.Version || cli.VersionAlt {
		fmt.Printf("txtr %s\n", version)
		fmt.Println("GNU strings compatible utility written in Go")
		os.Exit(0)
	}

	// Handle -o flag (alias for -t o)
	if cli.OctalOffset {
		cli.Radix = "o"
	}

	// Process output separator escape sequences
	outputSep := cli.OutputSeparator
	switch outputSep {
	case "\\n":
		outputSep = "\n"
	case "\\t":
		outputSep = "\t"
	case "\\r":
		outputSep = "\r"
	}

	// Validate -d flag can only be used with files, not stdin
	if cli.ScanDataOnly && len(cli.Files) == 0 {
		fmt.Fprintf(os.Stderr, "error: -d/--data flag requires file arguments (cannot be used with stdin)\n")
		os.Exit(1)
	}

	// Build config from CLI args
	config := extractor.Config{
		MinLength:            cli.MinLength,
		PrintFileName:        cli.PrintFileName,
		Radix:                cli.Radix,
		PrintOffset:          cli.Radix != "",
		Encoding:             cli.Encoding,
		Unicode:              cli.Unicode,
		OutputSeparator:      outputSep,
		IncludeAllWhitespace: cli.IncludeAllWhitespace,
		ScanAll:              cli.ScanAll,
		ScanDataOnly:         cli.ScanDataOnly,
		TargetFormat:         cli.TargetFormat,
	}

	// Process files or stdin
	if len(cli.Files) == 0 {
		// Read from stdin
		extractor.ExtractStrings(os.Stdin, "", config, printer.PrintString)
	} else {
		// Process each file
		for _, filename := range cli.Files {
			if config.ScanDataOnly {
				// Parse binary and extract from data sections only
				processFileWithBinaryParsing(filename, config)
			} else {
				// Regular full-file scanning
				file, err := os.Open(filename)
				if err != nil {
					fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
					continue
				}
				extractor.ExtractStrings(file, filename, config, printer.PrintString)
				if err := file.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
				}
			}
		}
	}
}

// processFileWithBinaryParsing handles binary format detection and section extraction
func processFileWithBinaryParsing(filename string, config extractor.Config) {
	// Determine format
	var format binary.Format
	var err error

	if config.TargetFormat != "" && config.TargetFormat != "binary" {
		// User specified a format
		switch config.TargetFormat {
		case "elf":
			format = binary.FormatELF
		case "pe":
			format = binary.FormatPE
		case "macho":
			format = binary.FormatMachO
		default:
			format = binary.FormatRaw
		}
	} else {
		// Auto-detect format
		format, err = binary.DetectFormat(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
			return
		}
	}

	// Parse binary to get sections
	sections, err := binary.ParseBinary(filename, format)
	if err != nil {
		// Fall back to regular scanning if parsing fails
		fmt.Fprintf(os.Stderr, "strings: %s: warning: cannot parse as %v, falling back to full scan: %v\n",
			filename, format, err)

		file, err := os.Open(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
			return
		}
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
			}
		}()

		extractor.ExtractStrings(file, filename, config, printer.PrintString)
		return
	}

	// If no sections found (raw binary), scan the whole file
	if len(sections) == 0 {
		file, err := os.Open(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
			return
		}
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
			}
		}()

		extractor.ExtractStrings(file, filename, config, printer.PrintString)
		return
	}

	// Extract strings from each data section
	for _, section := range sections {
		extractor.ExtractFromSection(section.Data, section.Name, section.Offset, filename, config, printer.PrintString)
	}
}
