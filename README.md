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

### Basic Options
- `-n <number>`, `--bytes=<number>`: Minimum string length (default: 4)
- `-f`, `--print-file-name`: Print the filename before each string
- `-t <radix>`, `--radix=<radix>`: Print offset in specified radix (o=octal, d=decimal, x=hex)
- `-o`, `--octal-offset`: Print offset in octal (alias for `-t o`)

### Encoding Options
- `-e <encoding>`, `--encoding=<encoding>`: Character encoding
  - `s`: 7-bit ASCII (default)
  - `S`: 8-bit ASCII
  - `b`: 16-bit big-endian (UTF-16BE)
  - `l`: 16-bit little-endian (UTF-16LE)
  - `B`: 32-bit big-endian (UTF-32BE)
  - `L`: 32-bit little-endian (UTF-32LE)

- `-U <mode>`, `--unicode=<mode>`: UTF-8 multibyte character handling
  - `default`: Treat invalid UTF-8 as non-printable (default)
  - `invalid`: Same as default
  - `locale`: Display UTF-8 characters in system locale
  - `escape`: Show as escape sequences (e.g., `\u4e16`)
  - `hex`: Show as hex sequences (e.g., `<4e16>`)
  - `highlight`: Highlighted escape sequences with ANSI codes

### Output Options
- `-s <sep>`, `--output-separator=<sep>`: Custom output record separator (default: newline)
- `-w`, `--include-all-whitespace`: Treat all whitespace characters as valid string components

### Scan Options
- `-a`, `--all`: Scan entire file (default behavior)
- `-d`, `--data`: Scan only initialized data sections (ELF, PE, Mach-O binaries)
- `-T <format>`, `--target=<format>`: Specify binary format
  - `elf`: Force ELF parsing (Linux/Unix)
  - `pe`: Force PE parsing (Windows)
  - `macho`: Force Mach-O parsing (macOS/iOS)
  - `binary`: Treat as raw binary (no parsing)

### Utility Options
- `-v`, `-V`, `--version`: Display version information
- `-h`, `--help`: Show help message

## Features

- **Multi-Encoding Support**: Extract strings in 7-bit ASCII, 8-bit ASCII, UTF-16 (BE/LE), and UTF-32 (BE/LE)
- **UTF-8 Unicode Support**: Full UTF-8 multibyte character handling with multiple display modes
- **Configurable Minimum Length**: Set minimum string length threshold
- **Multiple File Processing**: Process multiple files in one command
- **Stdin Support**: Read from standard input for pipeline integration
- **Flexible Offset Printing**: Display offsets in octal, decimal, or hexadecimal
- **Custom Output Separators**: Use custom delimiters between strings
- **Whitespace Handling**: Optionally include all whitespace characters in strings
- **Binary Format Support**: Parse ELF, PE, and Mach-O binaries to scan only data sections
- **GNU strings Compatible**: 100% feature parity with GNU strings (12/12 major features)
- **Modern CLI**: Built with Kong for clean, declarative argument parsing
- **Clean Architecture**: Follows Standard Go Project Layout for maintainability
- **Comprehensive Testing**: Full test coverage for all encoding formats

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

