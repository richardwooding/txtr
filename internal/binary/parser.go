// Package binary provides utilities for detecting and parsing binary file formats
// including ELF, PE, and Mach-O.
package binary

import (
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"os"
)

// Format represents the type of binary file
type Format int

const (
	// FormatUnknown indicates an unknown or unsupported binary format
	FormatUnknown Format = iota
	// FormatELF indicates an ELF (Executable and Linkable Format) binary
	FormatELF
	// FormatPE indicates a PE (Portable Executable) binary
	FormatPE
	// FormatMachO indicates a Mach-O (Mach Object) binary
	FormatMachO
	// FormatRaw indicates a raw binary with no specific structure
	FormatRaw
)

// String returns the string representation of the Format
func (f Format) String() string {
	switch f {
	case FormatELF:
		return "ELF"
	case FormatPE:
		return "PE"
	case FormatMachO:
		return "Mach-O"
	case FormatRaw:
		return "Raw"
	case FormatUnknown:
		return "Unknown"
	default:
		return "Unknown"
	}
}

// Section represents a section in a binary file
type Section struct {
	Name   string
	Offset int64
	Size   int64
	Data   []byte
}

// DetectFormat attempts to auto-detect the binary format
func DetectFormat(path string) (Format, error) {
	file, err := os.Open(path)
	if err != nil {
		return FormatUnknown, err
	}
	defer func() {
		_ = file.Close()
	}()

	// Try ELF
	if _, err := elf.NewFile(file); err == nil {
		return FormatELF, nil
	}

	// Reset file pointer
	if _, err := file.Seek(0, 0); err != nil {
		return FormatUnknown, fmt.Errorf("failed to seek: %w", err)
	}

	// Try PE
	if _, err := pe.NewFile(file); err == nil {
		return FormatPE, nil
	}

	// Reset file pointer
	if _, err := file.Seek(0, 0); err != nil {
		return FormatUnknown, fmt.Errorf("failed to seek: %w", err)
	}

	// Try Mach-O universal binary first
	// Universal binaries have magic number 0xcafebabe (BE) or 0xbebafeca (LE)
	var magic uint32
	if err := binary.Read(file, binary.BigEndian, &magic); err == nil {
		if magic == 0xcafebabe || magic == 0xbebafeca {
			return FormatMachO, nil
		}
	}

	// Reset file pointer
	if _, err := file.Seek(0, 0); err != nil {
		return FormatUnknown, fmt.Errorf("failed to seek: %w", err)
	}

	// Try Mach-O single architecture
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
	defer func() {
		_ = file.Close()
	}()

	elfFile, err := elf.NewFile(file)
	if err != nil {
		return nil, fmt.Errorf("not a valid ELF file: %w", err)
	}

	var sections []Section

	// Data section names to extract
	dataSectionNames := []string{
		".data",        // Initialized data
		".rodata",      // Read-only data
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
	defer func() {
		_ = file.Close()
	}()

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
	defer func() {
		_ = file.Close()
	}()

	// Data section patterns to extract
	dataPatterns := map[string]bool{
		"__DATA.__data":    true, // Initialized data
		"__DATA.__const":   true, // Constant data
		"__TEXT.__cstring": true, // C strings
		"__TEXT.__const":   true, // Constants in text
	}

	// Helper function to extract sections from a Mach-O file
	extractSections := func(machoFile *macho.File) []Section {
		var sections []Section
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
		return sections
	}

	// Try universal binary first
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek: %w", err)
	}

	fatFile, err := macho.NewFatFile(file)
	if err == nil {
		// Universal binary - extract from first architecture
		if len(fatFile.Arches) > 0 {
			sections := extractSections(fatFile.Arches[0].File)
			_ = fatFile.Close()
			return sections, nil
		}
		_ = fatFile.Close()
		return nil, fmt.Errorf("universal binary has no architectures")
	}

	// Not a universal binary, try single architecture
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek: %w", err)
	}

	machoFile, err := macho.NewFile(file)
	if err != nil {
		return nil, fmt.Errorf("not a valid Mach-O file: %w", err)
	}

	return extractSections(machoFile), nil
}

// ParseBinary parses a binary file based on the specified format
func ParseBinary(path string, format Format) ([]Section, error) {
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
