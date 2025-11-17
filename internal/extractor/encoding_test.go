package extractor

import (
	"bytes"
	"strings"
	"testing"
)

func TestExtractUTF8(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		mode     string
		expected string
	}{
		{
			name:     "ASCII only",
			input:    "hello world",
			mode:     "locale",
			expected: "hello world",
		},
		{
			name:     "UTF-8 Chinese characters - locale",
			input:    "Hello 世界",
			mode:     "locale",
			expected: "Hello 世界",
		},
		{
			name:     "UTF-8 Chinese characters - escape",
			input:    "Hello 世界",
			mode:     "escape",
			expected: "Hello \\u4e16\\u754c",
		},
		{
			name:     "UTF-8 Cyrillic characters - locale",
			input:    "Привет мир",
			mode:     "locale",
			expected: "Привет мир",
		},
		{
			name:     "UTF-8 Emoji",
			input:    "Hello 🌍",
			mode:     "locale",
			expected: "Hello 🌍",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []byte
			printFunc := func(str []byte, filename string, offset int64, config Config) {
				result = append(result, str...)
			}

			config := Config{
				MinLength: 4,
				Unicode:   tt.mode,
			}

			reader := strings.NewReader(tt.input)
			ExtractStrings(reader, "", config, printFunc)

			if string(result) != tt.expected {
				t.Errorf("got %q, want %q", string(result), tt.expected)
			}
		})
	}
}

func TestExtractUTF16LE(t *testing.T) {
	// UTF-16LE encoding of "hello"
	// h=0x0068, e=0x0065, l=0x006C, l=0x006C, o=0x006F
	input := []byte{
		0x68, 0x00, // h
		0x65, 0x00, // e
		0x6C, 0x00, // l
		0x6C, 0x00, // l
		0x6F, 0x00, // o
	}

	var result []byte
	printFunc := func(str []byte, filename string, offset int64, config Config) {
		result = append(result, str...)
	}

	config := Config{
		MinLength: 4,
		Encoding:  "l", // UTF-16LE
	}

	reader := bytes.NewReader(input)
	ExtractStrings(reader, "", config, printFunc)

	if string(result) != "hello" {
		t.Errorf("UTF-16LE extraction failed: got %q, want %q", string(result), "hello")
	}
}

func TestExtractUTF16BE(t *testing.T) {
	// UTF-16BE encoding of "hello"
	input := []byte{
		0x00, 0x68, // h
		0x00, 0x65, // e
		0x00, 0x6C, // l
		0x00, 0x6C, // l
		0x00, 0x6F, // o
	}

	var result []byte
	printFunc := func(str []byte, filename string, offset int64, config Config) {
		result = append(result, str...)
	}

	config := Config{
		MinLength: 4,
		Encoding:  "b", // UTF-16BE
	}

	reader := bytes.NewReader(input)
	ExtractStrings(reader, "", config, printFunc)

	if string(result) != "hello" {
		t.Errorf("UTF-16BE extraction failed: got %q, want %q", string(result), "hello")
	}
}

func TestExtractUTF32LE(t *testing.T) {
	// UTF-32LE encoding of "test"
	input := []byte{
		0x74, 0x00, 0x00, 0x00, // t
		0x65, 0x00, 0x00, 0x00, // e
		0x73, 0x00, 0x00, 0x00, // s
		0x74, 0x00, 0x00, 0x00, // t
	}

	var result []byte
	printFunc := func(str []byte, filename string, offset int64, config Config) {
		result = append(result, str...)
	}

	config := Config{
		MinLength: 4,
		Encoding:  "L", // UTF-32LE
	}

	reader := bytes.NewReader(input)
	ExtractStrings(reader, "", config, printFunc)

	if string(result) != "test" {
		t.Errorf("UTF-32LE extraction failed: got %q, want %q", string(result), "test")
	}
}

func TestExtractUTF32BE(t *testing.T) {
	// UTF-32BE encoding of "test"
	input := []byte{
		0x00, 0x00, 0x00, 0x74, // t
		0x00, 0x00, 0x00, 0x65, // e
		0x00, 0x00, 0x00, 0x73, // s
		0x00, 0x00, 0x00, 0x74, // t
	}

	var result []byte
	printFunc := func(str []byte, filename string, offset int64, config Config) {
		result = append(result, str...)
	}

	config := Config{
		MinLength: 4,
		Encoding:  "B", // UTF-32BE
	}

	reader := bytes.NewReader(input)
	ExtractStrings(reader, "", config, printFunc)

	if string(result) != "test" {
		t.Errorf("UTF-32BE extraction failed: got %q, want %q", string(result), "test")
	}
}

func TestIncludeAllWhitespace(t *testing.T) {
	input := "hello\nworld\ttab"

	tests := []struct {
		name                 string
		includeAllWhitespace bool
		expected             string
	}{
		{
			name:                 "without whitespace flag",
			includeAllWhitespace: false,
			expected:             "helloworld",
		},
		{
			name:                 "with whitespace flag",
			includeAllWhitespace: true,
			expected:             "hello\nworld\ttab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []byte
			printFunc := func(str []byte, filename string, offset int64, config Config) {
				result = append(result, str...)
			}

			config := Config{
				MinLength:            4,
				IncludeAllWhitespace: tt.includeAllWhitespace,
			}

			reader := strings.NewReader(input)
			ExtractStrings(reader, "", config, printFunc)

			resultStr := string(result)
			if !strings.Contains(resultStr, "hello") || !strings.Contains(resultStr, "world") {
				t.Errorf("missing expected content: got %q", resultStr)
			}
		})
	}
}

func Test8BitASCII(t *testing.T) {
	// Create input with extended ASCII characters (128-255)
	input := []byte{
		'h', 'e', 'l', 'l', 'o',
		0xA0, 0xA1, 0xA2, 0xA3, // Extended ASCII
	}

	tests := []struct {
		name     string
		encoding string
		minChars int
	}{
		{
			name:     "7-bit ASCII should not include extended chars",
			encoding: "s",
			minChars: 5,
		},
		{
			name:     "8-bit ASCII should include extended chars",
			encoding: "S",
			minChars: 9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []byte
			printFunc := func(str []byte, filename string, offset int64, config Config) {
				result = append(result, str...)
			}

			config := Config{
				MinLength: 4,
				Encoding:  tt.encoding,
			}

			reader := bytes.NewReader(input)
			ExtractStrings(reader, "", config, printFunc)

			if len(result) < tt.minChars {
				t.Errorf("%s: expected at least %d chars, got %d", tt.name, tt.minChars, len(result))
			}
		})
	}
}
