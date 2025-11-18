package extractor

import (
	"regexp"
	"testing"
)

// TestCompilePatterns tests pattern compilation
func TestCompilePatterns(t *testing.T) {
	tests := []struct {
		name       string
		patterns   []string
		ignoreCase bool
		wantErr    bool
	}{
		{
			name:       "empty patterns",
			patterns:   []string{},
			ignoreCase: false,
			wantErr:    false,
		},
		{
			name:       "nil patterns",
			patterns:   nil,
			ignoreCase: false,
			wantErr:    false,
		},
		{
			name:       "valid single pattern",
			patterns:   []string{"test"},
			ignoreCase: false,
			wantErr:    false,
		},
		{
			name:       "valid multiple patterns",
			patterns:   []string{"test", "hello", "world"},
			ignoreCase: false,
			wantErr:    false,
		},
		{
			name:       "valid regex patterns",
			patterns:   []string{"\\S+@\\S+", "https?://\\S+", "\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}"},
			ignoreCase: false,
			wantErr:    false,
		},
		{
			name:       "case insensitive",
			patterns:   []string{"ERROR", "WARNING"},
			ignoreCase: true,
			wantErr:    false,
		},
		{
			name:       "invalid regex pattern",
			patterns:   []string{"[invalid"},
			ignoreCase: false,
			wantErr:    true,
		},
		{
			name:       "mix of valid and invalid",
			patterns:   []string{"valid", "(invalid"},
			ignoreCase: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompilePatterns(tt.patterns, tt.ignoreCase)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompilePatterns() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if len(tt.patterns) == 0 {
					if got != nil {
						t.Errorf("CompilePatterns() returned non-nil for empty patterns")
					}
				} else {
					if len(got) != len(tt.patterns) {
						t.Errorf("CompilePatterns() returned %d patterns, want %d", len(got), len(tt.patterns))
					}
				}
			}
		})
	}
}

// TestCompilePatternsIgnoreCase tests case-insensitive flag
func TestCompilePatternsIgnoreCase(t *testing.T) {
	patterns := []string{"ERROR"}

	// Case-sensitive
	regexps, err := CompilePatterns(patterns, false)
	if err != nil {
		t.Fatalf("CompilePatterns() error = %v", err)
	}

	if regexps[0].MatchString("error") {
		t.Error("Case-sensitive pattern matched lowercase")
	}
	if !regexps[0].MatchString("ERROR") {
		t.Error("Case-sensitive pattern did not match uppercase")
	}

	// Case-insensitive
	regexps, err = CompilePatterns(patterns, true)
	if err != nil {
		t.Fatalf("CompilePatterns() error = %v", err)
	}

	if !regexps[0].MatchString("error") {
		t.Error("Case-insensitive pattern did not match lowercase")
	}
	if !regexps[0].MatchString("ERROR") {
		t.Error("Case-insensitive pattern did not match uppercase")
	}
	if !regexps[0].MatchString("Error") {
		t.Error("Case-insensitive pattern did not match mixed case")
	}
}

// TestShouldPrintString tests the filtering logic
func TestShouldPrintString(t *testing.T) {
	// Helper to compile patterns
	mustCompile := func(patterns []string) []*regexp.Regexp {
		result := make([]*regexp.Regexp, len(patterns))
		for i, p := range patterns {
			result[i] = regexp.MustCompile(p)
		}
		return result
	}

	tests := []struct {
		name     string
		str      string
		config   Config
		want     bool
	}{
		{
			name: "no patterns - allow all",
			str:  "anything",
			config: Config{
				MatchPatterns:   nil,
				ExcludePatterns: nil,
			},
			want: true,
		},
		{
			name: "match pattern - matches",
			str:  "test@example.com",
			config: Config{
				MatchPatterns:   mustCompile([]string{"\\S+@\\S+"}),
				ExcludePatterns: nil,
			},
			want: true,
		},
		{
			name: "match pattern - no match",
			str:  "no email here",
			config: Config{
				MatchPatterns:   mustCompile([]string{"\\S+@\\S+"}),
				ExcludePatterns: nil,
			},
			want: false,
		},
		{
			name: "multiple match patterns - first matches",
			str:  "test@example.com",
			config: Config{
				MatchPatterns:   mustCompile([]string{"\\S+@\\S+", "https?://\\S+"}),
				ExcludePatterns: nil,
			},
			want: true,
		},
		{
			name: "multiple match patterns - second matches",
			str:  "http://example.com",
			config: Config{
				MatchPatterns:   mustCompile([]string{"\\S+@\\S+", "https?://\\S+"}),
				ExcludePatterns: nil,
			},
			want: true,
		},
		{
			name: "multiple match patterns - none match",
			str:  "plain text",
			config: Config{
				MatchPatterns:   mustCompile([]string{"\\S+@\\S+", "https?://\\S+"}),
				ExcludePatterns: nil,
			},
			want: false,
		},
		{
			name: "exclude pattern - matches (excluded)",
			str:  "debug_symbol",
			config: Config{
				MatchPatterns:   nil,
				ExcludePatterns: mustCompile([]string{"debug.*"}),
			},
			want: false,
		},
		{
			name: "exclude pattern - no match (allowed)",
			str:  "normal_string",
			config: Config{
				MatchPatterns:   nil,
				ExcludePatterns: mustCompile([]string{"debug.*"}),
			},
			want: true,
		},
		{
			name: "multiple exclude patterns - first matches",
			str:  "debug_info",
			config: Config{
				MatchPatterns:   nil,
				ExcludePatterns: mustCompile([]string{"debug.*", "__.*"}),
			},
			want: false,
		},
		{
			name: "multiple exclude patterns - second matches",
			str:  "__internal",
			config: Config{
				MatchPatterns:   nil,
				ExcludePatterns: mustCompile([]string{"debug.*", "__.*"}),
			},
			want: false,
		},
		{
			name: "match and exclude - exclude takes precedence",
			str:  "debug_test@example.com",
			config: Config{
				MatchPatterns:   mustCompile([]string{"\\S+@\\S+"}),
				ExcludePatterns: mustCompile([]string{"debug.*"}),
			},
			want: false,
		},
		{
			name: "match and exclude - matches and not excluded",
			str:  "test@example.com",
			config: Config{
				MatchPatterns:   mustCompile([]string{"\\S+@\\S+"}),
				ExcludePatterns: mustCompile([]string{"debug.*"}),
			},
			want: true,
		},
		{
			name: "match and exclude - excluded but no match",
			str:  "debug_symbol",
			config: Config{
				MatchPatterns:   mustCompile([]string{"\\S+@\\S+"}),
				ExcludePatterns: mustCompile([]string{"debug.*"}),
			},
			want: false,
		},
		{
			name: "match and exclude - neither",
			str:  "plain text",
			config: Config{
				MatchPatterns:   mustCompile([]string{"\\S+@\\S+"}),
				ExcludePatterns: mustCompile([]string{"debug.*"}),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldPrintString([]byte(tt.str), tt.config)
			if got != tt.want {
				t.Errorf("ShouldPrintString() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestShouldPrintStringSpecialPatterns tests special regex patterns
func TestShouldPrintStringSpecialPatterns(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		str     string
		want    bool
	}{
		{
			name:    "URL pattern",
			pattern: "https?://\\S+",
			str:     "http://example.com",
			want:    true,
		},
		{
			name:    "Email pattern",
			pattern: "\\S+@\\S+\\.\\S+",
			str:     "user@example.com",
			want:    true,
		},
		{
			name:    "IP address pattern",
			pattern: "\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}",
			str:     "192.168.1.1",
			want:    true,
		},
		{
			name:    "Error message pattern",
			pattern: "(?i)(error|warning|fatal)",
			str:     "ERROR: something went wrong",
			want:    true,
		},
		{
			name:    "Hex address pattern",
			pattern: "0x[0-9a-fA-F]+",
			str:     "0xDEADBEEF",
			want:    true,
		},
		{
			name:    "UUID pattern",
			pattern: "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}",
			str:     "550e8400-e29b-41d4-a716-446655440000",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := regexp.MustCompile(tt.pattern)
			config := Config{
				MatchPatterns: []*regexp.Regexp{re},
			}
			got := ShouldPrintString([]byte(tt.str), config)
			if got != tt.want {
				t.Errorf("ShouldPrintString() = %v, want %v for pattern %q and string %q", got, tt.want, tt.pattern, tt.str)
			}
		})
	}
}
