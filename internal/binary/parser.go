package binary

import (
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"fmt"
	"os"
)

// BinaryFormat represents the type of binary file
type BinaryFormat int

const (
	FormatUnknown BinaryFormat = iota
	FormatELF
	FormatPE
	FormatMachO
	FormatRaw // Raw binary, no structure
)

// Section represents a section in a binary file
type Section struct {
	Name   string
	Offset int64
	Size   int64
	Data   []byte
}

// DetectFormat attempts to auto-detect the binary format
func DetectFormat(path string) (BinaryFormat, error) {
	file, err := os.Open(path)
	if err != nil {
		return FormatUnknown, err
	}
	defer file.Close()

	// Try ELF
	if _, err := elf.NewFile(file); err == nil {
		return FormatELF, nil
	}

	// Reset file pointer
	file.Seek(0, 0)

	// Try PE
	if _, err := pe.NewFile(file); err == nil {
		return FormatPE, nil
	}

	// Reset file pointer
	file.Seek(0, 0)

	// Try Mach-O
	if _, err := macho.NewFile(file); err == nil {
		return FormatMachO, nil
	}

	// If all fail, treat as raw binary
	return FormatRaw, nil
}

// ParseELF extracts data sections from an ELF file
func ParseELF(path string) ([]Section, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	elfFile, err := elf.NewFile(file)
	if err != nil {
		return nil, fmt.Errorf("not a valid ELF file: %w", err)
	}

	var sections []Section

	// Data section names to extract
	dataSectionNames := []string{
		".data",      // Initialized data
		".rodata",    // Read-only data
		".data.rel.ro", // Read-only after relocation
	}

	for _, name := range dataSectionNames {
		sect := elfFile.Section(name)
		if sect == nil {
			continue
		}

		data, err := sect.Data()
		if err != nil {
			continue // Skip sections we can't read
		}

		sections = append(sections, Section{
			Name:   sect.Name,
			Offset: int64(sect.Offset),
			Size:   int64(sect.Size),
			Data:   data,
		})
	}

	return sections, nil
}

// ParsePE extracts data sections from a PE file
func ParsePE(path string) ([]Section, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	peFile, err := pe.NewFile(file)
	if err != nil {
		return nil, fmt.Errorf("not a valid PE file: %w", err)
	}

	var sections []Section

	// Look for data sections
	for _, sect := range peFile.Sections {
		// Include .data and .rdata (read-only data) sections
		if sect.Name == ".data" || sect.Name == ".rdata" {
			data, err := sect.Data()
			if err != nil {
				continue
			}

			sections = append(sections, Section{
				Name:   sect.Name,
				Offset: int64(sect.Offset),
				Size:   int64(sect.Size),
				Data:   data,
			})
		}
	}

	return sections, nil
}

// ParseMachO extracts data sections from a Mach-O file
func ParseMachO(path string) ([]Section, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	machoFile, err := macho.NewFile(file)
	if err != nil {
		return nil, fmt.Errorf("not a valid Mach-O file: %w", err)
	}

	var sections []Section

	// Data section patterns to extract
	dataPatterns := map[string]bool{
		"__DATA.__data":       true, // Initialized data
		"__DATA.__const":      true, // Constant data
		"__TEXT.__cstring":    true, // C strings
		"__TEXT.__const":      true, // Constants in text
	}

	for _, sect := range machoFile.Sections {
		// Construct full section name (Segment.Section)
		fullName := sect.Seg + "." + sect.Name

		if dataPatterns[fullName] {
			data, err := sect.Data()
			if err != nil {
				continue
			}

			sections = append(sections, Section{
				Name:   fullName,
				Offset: int64(sect.Offset),
				Size:   int64(sect.Size),
				Data:   data,
			})
		}
	}

	return sections, nil
}

// ParseBinary parses a binary file based on the specified format
func ParseBinary(path string, format BinaryFormat) ([]Section, error) {
	switch format {
	case FormatELF:
		return ParseELF(path)
	case FormatPE:
		return ParsePE(path)
	case FormatMachO:
		return ParseMachO(path)
	case FormatRaw, FormatUnknown:
		// For raw binaries, return nil to indicate full file scan
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported format: %d", format)
	}
}
