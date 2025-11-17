package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/richardwooding/txtr/internal/extractor"
	"github.com/richardwooding/txtr/internal/printer"
)

// CLI defines the command-line interface structure
type CLI struct {
	MinLength     int      `short:"n" name:"bytes" default:"4" help:"Minimum string length"`
	PrintFileName bool     `short:"f" name:"print-file-name" help:"Print file name before each string"`
	Radix         string   `short:"t" name:"radix" enum:"o,d,x," default:"" help:"Print offset in radix (o=octal, d=decimal, x=hex)"`
	OctalOffset   bool     `short:"o" help:"Print offset in octal (alias for -t o)"`
	Files         []string `arg:"" optional:"" name:"file" help:"Files to extract strings from" type:"path"`
}

func main() {
	var cli CLI

	kong.Parse(&cli,
		kong.Name("txtr"),
		kong.Description("Extract printable strings from binary files. GNU strings compatible."),
		kong.UsageOnError(),
	)

	// Handle -o flag (alias for -t o)
	if cli.OctalOffset {
		cli.Radix = "o"
	}

	// Build config from CLI args
	config := extractor.Config{
		MinLength:     cli.MinLength,
		PrintFileName: cli.PrintFileName,
		Radix:         cli.Radix,
		PrintOffset:   cli.Radix != "",
	}

	// Process files or stdin
	if len(cli.Files) == 0 {
		// Read from stdin
		extractor.ExtractStrings(os.Stdin, "", config, printer.PrintString)
	} else {
		// Process each file
		for _, filename := range cli.Files {
			file, err := os.Open(filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
				continue
			}
			extractor.ExtractStrings(file, filename, config, printer.PrintString)
			file.Close()
		}
	}
}
