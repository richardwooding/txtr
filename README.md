# txtr

A clone of GNU strings utility written in Go 1.25.

Extracts printable strings from binary files. Aims to be compatible with GNU strings. Built with the Kong CLI library and following Standard Go Project Layout.

## Installation

```bash
# Build the binary
go build -o txtr ./cmd/txtr

# Or install it
go install ./cmd/txtr
```

## Usage

```bash
# Extract strings from a file (default minimum length: 4)
txtr file.bin

# Set minimum string length
txtr -n 8 file.bin

# Print filename before each string
txtr -f file.bin

# Print offset in hexadecimal
txtr -t x file.bin

# Print offset in decimal
txtr -t d file.bin

# Print offset in octal
txtr -t o file.bin
# or
txtr -o file.bin

# Read from stdin
cat file.bin | txtr

# Process multiple files
txtr -f file1.bin file2.bin
```

## Supported Options

- `-n <number>`: Minimum string length (default: 4)
- `-f`, `--print-file-name`: Print the filename before each string
- `-t <radix>`, `--radix=<radix>`: Print offset in specified radix (o=octal, d=decimal, x=hex)
- `-o`: Print offset in octal (alias for `-t o`)

## Features

- Extracts printable ASCII strings (characters 32-126)
- Configurable minimum string length
- Multiple file processing
- Stdin support
- Offset printing in multiple radixes (octal, decimal, hexadecimal)
- Compatible output format with GNU strings
- Modern CLI with Kong library
- Clean architecture following Standard Go Project Layout

## Project Structure

```
txtr/
├── cmd/txtr/           # Application entry point
├── internal/
│   ├── extractor/      # String extraction logic
│   └── printer/        # Output formatting
├── go.mod              # Go 1.25 module definition
└── README.md
```

## Dependencies

- Go 1.25
- [Kong](https://github.com/alecthomas/kong) - Command-line parser

