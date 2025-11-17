# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`txtr` is a clone of the GNU strings utility written in Go 1.25. It extracts printable ASCII strings (characters 32-126) from binary files, with output compatible with GNU strings. The project follows the Standard Go Project Layout conventions.

## Development Commands

### Build
```bash
# Build the binary
go build -o txtr ./cmd/txtr

# Or use go install
go install ./cmd/txtr
```

### Run Tests
```bash
# Run all tests
set -o pipefail; go test -v ./...

# Run tests for a specific package
set -o pipefail; go test -v ./internal/extractor
set -o pipefail; go test -v ./internal/printer

# Run a specific test
set -o pipefail; go test -v -run TestIsPrintable ./internal/extractor
```

### Run the Binary
```bash
# After building
./txtr file.bin

# Or run directly with go
go run ./cmd/txtr file.bin
```

## Architecture

This project follows the Standard Go Project Layout with clear separation of concerns:

### Directory Structure

```
txtr/
├── cmd/txtr/           # Application entry point
│   └── main.go         # CLI using Kong library
├── internal/           # Private application code
│   ├── extractor/      # String extraction logic
│   │   ├── extractor.go
│   │   └── extractor_test.go
│   └── printer/        # Output formatting logic
│       ├── printer.go
│       └── printer_test.go
├── go.mod              # Go module definition (Go 1.25)
├── go.sum              # Dependency checksums
├── README.md           # User documentation
└── CLAUDE.md           # Developer documentation (this file)
```

### Core Components

**cmd/txtr/main.go** - CLI entry point:
- `CLI` struct: Kong-tagged structure for command-line argument parsing
- `main()`: Parses CLI args using Kong, handles stdin vs file input
- Uses Kong library for modern, declarative CLI parsing
- Backward compatible with original GNU strings flags

**internal/extractor/extractor.go** - Core extraction logic:
- `Config` struct: Configuration for extraction options (MinLength, PrintFileName, Radix, PrintOffset)
- `ExtractStrings()`: Core extraction logic - reads byte-by-byte, accumulates printable sequences
- `IsPrintable()`: Determines if a byte is in printable ASCII range (32-126)

**internal/printer/printer.go** - Output formatting:
- `PrintString()`: Formats and outputs strings with optional filename prefix and offset

### String Extraction Algorithm

The extraction logic in `ExtractStrings()` works as follows:
1. Reads input byte-by-byte using bufio.Reader
2. Tracks current offset and string start offset
3. Accumulates consecutive printable bytes into `currentString`
4. When a non-printable byte is encountered, checks if accumulated string meets minimum length
5. Calls the print function for valid strings and resets the accumulator
6. Handles EOF by printing any remaining valid string

### Testing

Tests are organized by package:

**internal/extractor/extractor_test.go**:
- `TestIsPrintable`: Boundary testing for printable character detection
- `TestExtractStrings`: Integration testing of extraction logic with various inputs

**internal/printer/printer_test.go**:
- `TestPrintString`: Output formatting tests (currently skipped - requires stdout capture)

### CLI Flag Handling

The application uses Kong for declarative CLI argument parsing:
- Kong struct tags define flags, their types, defaults, and help text
- Multiple flag names supported: `-n` / `--bytes`, `-f` / `--print-file-name`, `-t` / `--radix`
- Special handling for `-o` flag as alias for `-t o` (octal offset) using post-parse logic
- Radix options: `o` (octal), `d` (decimal), `x` (hexadecimal)
- Full backward compatibility with GNU strings

### Dependencies

- **Kong** (`github.com/alecthomas/kong`): Modern, declarative command-line parser
- Go 1.25 standard library

## Code Patterns

- Standard Go Project Layout for clear code organization
- Error handling: Errors written to stderr with GNU strings-compatible format
- Dependency injection: Print function passed to extractor for testability
- Package separation: CLI, extraction logic, and output formatting are independent
