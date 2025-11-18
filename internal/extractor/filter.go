package extractor

import (
	"fmt"
	"regexp"
)

// CompilePatterns compiles a list of regex pattern strings into compiled regexps.
// If ignoreCase is true, the patterns are compiled with case-insensitive flag.
// Returns an error if any pattern is invalid.
func CompilePatterns(patterns []string, ignoreCase bool) ([]*regexp.Regexp, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for i, pattern := range patterns {
		// Add case-insensitive flag if requested
		if ignoreCase {
			pattern = "(?i)" + pattern
		}

		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern #%d (%q): %w", i+1, patterns[i], err)
		}
		compiled = append(compiled, re)
	}

	return compiled, nil
}

// ShouldPrintString determines if a string should be printed based on
// match and exclude patterns in the config.
//
// Filtering logic:
// 1. If exclude patterns exist and any match, return false (exclude takes precedence)
// 2. If match patterns exist, at least one must match to return true
// 3. If no patterns are defined, return true (no filtering)
func ShouldPrintString(str []byte, config Config) bool {
	// Check exclude patterns first (blacklist has priority)
	if len(config.ExcludePatterns) > 0 {
		for _, pattern := range config.ExcludePatterns {
			if pattern.Match(str) {
				return false // Excluded
			}
		}
	}

	// Check match patterns (whitelist)
	if len(config.MatchPatterns) > 0 {
		for _, pattern := range config.MatchPatterns {
			if pattern.Match(str) {
				return true // At least one match found
			}
		}
		return false // No match patterns matched
	}

	// No filtering configured, allow all strings
	return true
}
