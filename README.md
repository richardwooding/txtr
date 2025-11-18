# txtr

A clone of GNU strings utility written in Go 1.25.

Extracts printable strings from binary files. Aims to be compatible with GNU strings. Built with the Kong CLI library and following Standard Go Project Layout.

## Installation

### Pre-built Binaries (Recommended)

Download the latest release for your platform from the [GitHub Releases page](https://github.com/richardwooding/txtr/releases).

**Current stable version: v2.0.1**

Available for:
- **Linux**: amd64, arm64, armv6, armv7
- **macOS**: amd64 (Intel), arm64 (Apple Silicon)
- **Windows**: amd64, arm64
- **FreeBSD**: amd64, arm64

**Example (Linux amd64):**
```bash
# Download
curl -LO https://github.com/richardwooding/txtr/releases/download/v2.0.1/txtr_2.0.1_linux_amd64.tar.gz

# Extract
tar -xzf txtr_2.0.1_linux_amd64.tar.gz

# Move to PATH
sudo mv txtr /usr/local/bin/

# Verify
txtr --version
```

**Verify checksums:**
```bash
# Download checksums
curl -LO https://github.com/richardwooding/txtr/releases/download/v2.0.1/checksums.txt

# Verify (Linux/macOS)
sha256sum -c checksums.txt 2>&1 | grep OK
```

### Container Images

Pull from GitHub Container Registry:

```bash
# Latest release
docker pull ghcr.io/richardwooding/txtr:latest

# Specific version
docker pull ghcr.io/richardwooding/txtr:v2.0.1

# Run on a file
docker run --rm -v $(pwd):/data ghcr.io/richardwooding/txtr:latest /data/file.bin

# Run with stdin
cat file.bin | docker run --rm -i ghcr.io/richardwooding/txtr:latest
```

**Multi-platform support**: linux/amd64, linux/arm64

### Go Install

For Go developers:

```bash
# Latest version
go install github.com/richardwooding/txtr/cmd/txtr@latest

# Specific version
go install github.com/richardwooding/txtr/cmd/txtr@v2.0.1
```

### Build from Source

For development or if pre-built binaries aren't available for your platform:

```bash
# Clone the repository
git clone https://github.com/richardwooding/txtr.git
cd txtr

# Build
go build -o txtr ./cmd/txtr

# Or install to $GOPATH/bin
go install ./cmd/txtr
```

## Usage

```bash
# Show version information
txtr --version
# Output:
# txtr v2.0.1
#   commit: cec6729
#   built: 2025-11-17T12:00:00Z
#   built by: goreleaser
# GNU strings compatible utility written in Go

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

# Process multiple files in parallel (uses all CPU cores by default)
txtr -f file1.bin file2.bin file3.bin

# Control parallelism (4 workers)
txtr -P 4 -f *.bin

# Force sequential processing
txtr -P 1 -f *.bin

# JSON output for automation
txtr --json file.bin

# JSON output with jq filtering
txtr --json file.bin | jq '.files[0].strings[] | select(.length > 20)'

# JSON output with binary format detection
txtr --json -d binary.exe | jq '.files[0].format'

# Colored output (auto-detects terminal)
txtr --color=auto file.bin

# Force colored output (e.g., for piping to 'less -R')
txtr --color=always file.bin | less -R

# Disable colors
txtr --color=never file.bin

# Colors with filename and offset
txtr -f -t x --color=always file.bin
```

### JSON Output Format

The `--json` flag outputs results in structured JSON format, perfect for automation, CI/CD pipelines, and integration with tools like `jq`:

```json
{
  "files": [
    {
      "file": "binary.exe",
      "format": "PE",
      "sections": [".data", ".rdata"],
      "strings": [
        {
          "value": "Hello World",
          "offset": 1024,
          "offset_hex": "0x400",
          "length": 11,
          "encoding": "ascii-7bit"
        }
      ]
    }
  ],
  "summary": {
    "total_strings": 42,
    "total_bytes": 1234,
    "min_length": 4,
    "encoding": "ascii-7bit"
  }
}
```

**Use cases:**
- Filter strings by length: `txtr --json file.bin | jq '.files[0].strings[] | select(.length > 20)'`
- Extract offsets: `txtr --json file.bin | jq '.files[0].strings[].offset_hex'`
- Count strings: `txtr --json file.bin | jq '.summary.total_strings'`
- Analyze binary format: `txtr --json -d file.bin | jq '.files[0].format'`

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
- `-j`, `--json`: Output results in JSON format for automation and tool integration
- `--color=<mode>`: When to use colored output (default: auto)
  - `auto`: Automatically detect if output is a terminal (respects NO_COLOR)
  - `always`: Force colored output
  - `never`: Disable colored output

### Performance Options
- `-P <workers>`, `--parallel=<workers>`: Number of parallel workers for processing multiple files (default: 0)
  - `0`: Auto-detect number of CPUs (default, enables automatic parallelism)
  - `1`: Sequential processing (disables parallelism)
  - `N`: Use N parallel workers

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
- **Colored Output**: Visual distinction with ANSI colors for filenames, offsets, and string types (auto/always/never)
- **Configurable Minimum Length**: Set minimum string length threshold
- **Multiple File Processing**: Process multiple files in one command
- **Parallel Processing**: Automatic multi-core utilization for 2-8x speedup on multiple files
- **Stdin Support**: Read from standard input for pipeline integration
- **Flexible Offset Printing**: Display offsets in octal, decimal, or hexadecimal
- **Custom Output Separators**: Use custom delimiters between strings
- **Whitespace Handling**: Optionally include all whitespace characters in strings
- **Binary Format Support**: Parse ELF, PE, and Mach-O binaries to scan only data sections
- **JSON Output**: Structured output for automation and tool integration
- **GNU strings Compatible**: 100% feature parity with GNU strings (12/12 major features)
- **Modern CLI**: Built with Kong for clean, declarative argument parsing
- **Clean Architecture**: Follows Standard Go Project Layout for maintainability
- **Comprehensive Testing**: Full test coverage for all encoding formats
- **Continuous Fuzzing**: 8 fuzz targets with daily automated execution for security

## Performance

### Parallel Processing

When processing multiple files, `txtr` automatically uses all available CPU cores to achieve significant speedup:

**Performance expectations:**
- **2 cores**: ~1.8x speedup
- **4 cores**: ~3.5x speedup
- **8 cores**: ~6-7x speedup

**How it works:**
- Default behavior (`-P 0`): Automatically detects and uses all CPU cores
- Single file: Processed sequentially (no parallelism overhead)
- Multiple files: Distributed across worker pool with ordered output
- Per-file errors: One file failure doesn't stop processing others

**Example:**
```bash
# Analyze 100 binaries using all CPUs (automatic)
txtr -f *.bin

# Limit to 4 workers (useful for resource-constrained environments)
txtr -P 4 -f *.bin

# Force sequential processing (debugging or single-core systems)
txtr -P 1 -f *.bin
```

**Use cases:**
- Bulk malware analysis
- Firmware analysis (multiple files in extracted filesystem)
- CI/CD scanning of build artifacts
- Directory-wide binary scanning

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

## Releases

**Current stable version**: v2.0.1

Releases are automatically built and published via GitHub Actions when a new git tag is pushed. Each release includes:

- **Pre-built binaries** for 10+ OS/architecture combinations
- **Container images** published to ghcr.io
- **Checksums** (SHA256) for verification
- **SBOM** (Software Bill of Materials) in SPDX format
- **Automated changelog** from commit history

**Platform Support Matrix:**

| OS | Architectures |
|---|---|
| Linux | amd64, arm64, armv6, armv7 |
| macOS | amd64 (Intel), arm64 (Apple Silicon) |
| Windows | amd64, arm64 |
| FreeBSD | amd64, arm64 |

**Container Images:**

| Registry | Platforms |
|---|---|
| ghcr.io/richardwooding/txtr | linux/amd64, linux/arm64 |

Visit the [Releases page](https://github.com/richardwooding/txtr/releases) to download the latest version.

## Dependencies

**Runtime:**
- [Kong v1.7.0](https://github.com/alecthomas/kong) - Command-line parser (only external dependency)

**Build:**
- Go 1.25

**Features:**
- ✅ Zero CGO dependencies - fully static binaries
- ✅ No external libraries required at runtime
- ✅ Works on any system without libc or other dependencies

## Security

- **Static binaries**: No dynamic dependencies, reducing attack surface
- **SBOM included**: Every release includes Software Bill of Materials for supply chain security
- **Checksums**: SHA256 checksums provided for verification
- **Minimal container images**: Based on Chainguard static image (~2MB base)
- **Reproducible builds**: Same source produces identical binaries

