package extractor

import (
	"bytes"
	"regexp"
	"testing"
)

// Benchmark: Pattern compilation overhead

func BenchmarkCompilePatterns_Single(b *testing.B) {
	patterns := []string{`\S+@\S+`}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CompilePatterns(patterns, false)
	}
}

func BenchmarkCompilePatterns_Multiple(b *testing.B) {
	patterns := []string{
		`\S+@\S+\.\S+`,
		`https?://\S+`,
		`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`,
		`(?i)(error|warning|fatal)`,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CompilePatterns(patterns, false)
	}
}

func BenchmarkCompilePatterns_CaseInsensitive(b *testing.B) {
	patterns := []string{
		`error`,
		`warning`,
		`fatal`,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CompilePatterns(patterns, true)
	}
}

// Benchmark: Pattern matching overhead

func BenchmarkShouldPrintString_NoFilter(b *testing.B) {
	str := []byte("test@example.com")
	config := Config{
		MinLength: 4,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ShouldPrintString(str, config)
	}
}

func BenchmarkShouldPrintString_SimpleMatch(b *testing.B) {
	str := []byte("test@example.com")
	matchPatterns := []*regexp.Regexp{regexp.MustCompile(`\S+@\S+`)}
	config := Config{
		MinLength:     4,
		MatchPatterns: matchPatterns,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ShouldPrintString(str, config)
	}
}

func BenchmarkShouldPrintString_ComplexMatch(b *testing.B) {
	str := []byte("user.name+tag@example.com")
	matchPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
	}
	config := Config{
		MinLength:     4,
		MatchPatterns: matchPatterns,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ShouldPrintString(str, config)
	}
}

func BenchmarkShouldPrintString_MultiplePatterns(b *testing.B) {
	str := []byte("Error: Connection failed")
	matchPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\S+@\S+`),
		regexp.MustCompile(`https?://\S+`),
		regexp.MustCompile(`(?i)(error|warning|fatal)`),
		regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`),
	}
	config := Config{
		MinLength:     4,
		MatchPatterns: matchPatterns,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ShouldPrintString(str, config)
	}
}

func BenchmarkShouldPrintString_ExcludePattern(b *testing.B) {
	str := []byte("debug_symbol_name")
	excludePatterns := []*regexp.Regexp{regexp.MustCompile(`debug_.*`)}
	config := Config{
		MinLength:       4,
		ExcludePatterns: excludePatterns,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ShouldPrintString(str, config)
	}
}

func BenchmarkShouldPrintString_MatchAndExclude(b *testing.B) {
	str := []byte("test@example.com")
	matchPatterns := []*regexp.Regexp{regexp.MustCompile(`\S+@\S+`)}
	excludePatterns := []*regexp.Regexp{regexp.MustCompile(`spam.*`)}
	config := Config{
		MinLength:       4,
		MatchPatterns:   matchPatterns,
		ExcludePatterns: excludePatterns,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ShouldPrintString(str, config)
	}
}

// Benchmark: Filter impact on extraction

func BenchmarkExtractWithFilter_NoFilter(b *testing.B) {
	data := createASCIIBenchmarkData(1 * 1024 * 1024)
	config := Config{
		MinLength: 4,
		Encoding:  "s",
	}
	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		extractASCII(reader, "", config, printFunc, false)
	}

	throughput := float64(len(data)) * float64(b.N) / b.Elapsed().Seconds() / 1e6
	b.ReportMetric(throughput, "MB/s")
}

func BenchmarkExtractWithFilter_SimplePattern(b *testing.B) {
	data := createASCIIBenchmarkData(1 * 1024 * 1024)
	matchPatterns := []*regexp.Regexp{regexp.MustCompile(`Benchmark`)}
	config := Config{
		MinLength:     4,
		Encoding:      "s",
		MatchPatterns: matchPatterns,
	}
	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		extractASCII(reader, "", config, printFunc, false)
	}

	throughput := float64(len(data)) * float64(b.N) / b.Elapsed().Seconds() / 1e6
	b.ReportMetric(throughput, "MB/s")
}

func BenchmarkExtractWithFilter_ComplexPattern(b *testing.B) {
	data := createASCIIBenchmarkData(1 * 1024 * 1024)
	matchPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
	}
	config := Config{
		MinLength:     4,
		Encoding:      "s",
		MatchPatterns: matchPatterns,
	}
	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		extractASCII(reader, "", config, printFunc, false)
	}

	throughput := float64(len(data)) * float64(b.N) / b.Elapsed().Seconds() / 1e6
	b.ReportMetric(throughput, "MB/s")
}

func BenchmarkExtractWithFilter_MultiplePatterns(b *testing.B) {
	data := createASCIIBenchmarkData(1 * 1024 * 1024)
	matchPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\S+@\S+`),
		regexp.MustCompile(`https?://\S+`),
		regexp.MustCompile(`(?i)(error|warning)`),
	}
	config := Config{
		MinLength:     4,
		Encoding:      "s",
		MatchPatterns: matchPatterns,
	}
	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		extractASCII(reader, "", config, printFunc, false)
	}

	throughput := float64(len(data)) * float64(b.N) / b.Elapsed().Seconds() / 1e6
	b.ReportMetric(throughput, "MB/s")
}

// Benchmark: Pattern complexity comparison

func BenchmarkPatternComplexity(b *testing.B) {
	testStrings := [][]byte{
		[]byte("simple text"),
		[]byte("test@example.com"),
		[]byte("https://www.example.com/path/to/resource"),
		[]byte("192.168.1.1"),
		[]byte("Error: something went wrong"),
	}

	patterns := map[string]string{
		"Literal":      `example`,
		"Simple":       `\S+@\S+`,
		"Moderate":     `https?://\S+`,
		"Complex":      `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
		"VeryComplex":  `^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`,
		"Alternation":  `(?i)(error|warning|fatal|critical|alert)`,
	}

	for name, pattern := range patterns {
		b.Run(name, func(b *testing.B) {
			re := regexp.MustCompile(pattern)
			config := Config{
				MinLength:     4,
				MatchPatterns: []*regexp.Regexp{re},
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, str := range testStrings {
					_ = ShouldPrintString(str, config)
				}
			}
		})
	}
}
