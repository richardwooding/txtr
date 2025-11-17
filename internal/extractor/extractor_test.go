package extractor

import (
	"bytes"
	"testing"
)

func TestIsPrintable(t *testing.T) {
	tests := []struct {
		name     string
		input    byte
		expected bool
	}{
		{"space", ' ', true},
		{"letter a", 'a', true},
		{"letter Z", 'Z', true},
		{"digit 0", '0', true},
		{"digit 9", '9', true},
		{"tilde", '~', true},
		{"null byte", 0, false},
		{"newline", '\n', false},
		{"tab", '\t', false},
		{"below space", 31, false},
		{"above tilde", 127, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPrintable(tt.input)
			if result != tt.expected {
				t.Errorf("IsPrintable(%d) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractStrings(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		minLength int
		expected  []string
	}{
		{
			name:      "simple strings",
			input:     []byte("hello\x00world\x00"),
			minLength: 4,
			expected:  []string{"hello", "world"},
		},
		{
			name:      "short string filtered",
			input:     []byte("ab\x00test\x00"),
			minLength: 4,
			expected:  []string{"test"},
		},
		{
			name:      "min length 3",
			input:     []byte("ab\x00abc\x00"),
			minLength: 3,
			expected:  []string{"abc"},
		},
		{
			name:      "consecutive separators",
			input:     []byte("test\x00\x00\x00data"),
			minLength: 4,
			expected:  []string{"test", "data"},
		},
		{
			name:      "string at end",
			input:     []byte("\x00\x00hello"),
			minLength: 4,
			expected:  []string{"hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Collect strings using a custom print function
			var found []string
			printFunc := func(str []byte, filename string, offset int64, config Config) {
				found = append(found, string(str))
			}

			config := Config{
				MinLength: tt.minLength,
			}

			bufReader := bytes.NewReader(tt.input)
			ExtractStrings(bufReader, "", config, printFunc)

			if len(found) != len(tt.expected) {
				t.Errorf("found %d strings, expected %d", len(found), len(tt.expected))
			}

			for i, s := range found {
				if i >= len(tt.expected) || s != tt.expected[i] {
					t.Errorf("string %d = %q, want %q", i, s, tt.expected[i])
				}
			}
		})
	}
}
