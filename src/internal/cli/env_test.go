package cli_test

import (
	"testing"
)

func TestGetEnvFileWeight(t *testing.T) {
	testCases := getEnvFileWeightTests()

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			weight := testGetEnvFileWeight(tt.filename, ".env")
			if weight != tt.expectedWeight {
				t.Errorf("getEnvFileWeight(%q) = %d, want %d", tt.filename, weight, tt.expectedWeight)
			}
		})
	}
}

func getEnvFileWeightTests() []struct {
	name           string
	filename       string
	expectedWeight int
} {
	return []struct {
		name           string
		filename       string
		expectedWeight int
	}{
		{"base .env file", ".env", 0},
		{".env.local file", ".env.local", 2},
		{".env.development file", ".env.development", 1},
		{".env.production file", ".env.production", 1},
		{".env.test file", ".env.test", 1},
		{".env.development.local file", ".env.development.local", 3},
		{".env.production.local file", ".env.production.local", 3},
		{".env.staging.local file", ".env.staging.local", 3},
		{".env.custom file", ".env.custom", 1},
		{".env.custom.local file", ".env.custom.local", 3},
	}
}

func TestEnvFileFiltering(t *testing.T) {
	testCases := getEnvFileFilteringTests()

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			accepted := shouldAcceptEnvFile(tt.filename)
			if accepted != tt.shouldAccept {
				t.Errorf("filtering %q: got %v, want %v", tt.filename, accepted, tt.shouldAccept)
			}
		})
	}
}

func getEnvFileFilteringTests() []struct {
	name         string
	filename     string
	shouldAccept bool
} {
	return []struct {
		name         string
		filename     string
		shouldAccept bool
	}{
		{"accept .env", ".env", true},
		{"accept .env.local", ".env.local", true},
		{"accept .env.development", ".env.development", true},
		{"accept .env.production.local", ".env.production.local", true},
		{"reject .env.example", ".env.example", false},
		{"reject .env.local.example", ".env.local.example", false},
		{"reject .env.development.example", ".env.development.example", false},
		{"reject .envrc", ".envrc", false},
		{"reject .environment", ".environment", false},
		{"reject .envoy", ".envoy", false},
		{"reject random.env", "random.env", false},
		{"accept .env.123", ".env.123", true},
	}
}

func shouldAcceptEnvFile(filename string) bool {
	const baseEnvFile = ".env"
	return (filename == baseEnvFile ||
		(len(filename) > len(baseEnvFile+".") &&
			filename[:len(baseEnvFile+".")] == baseEnvFile+".")) &&
		!endsWith(filename, ".example")
}

func TestEnvFileSorting(t *testing.T) {
	testCases := getEnvFileSortingTests()

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			testFiles := make([]string, len(tt.files))
			copy(testFiles, tt.files)

			testSortEnvFiles(testFiles, ".env")

			if len(testFiles) != len(tt.expected) {
				t.Fatalf("length mismatch: got %d, want %d", len(testFiles), len(tt.expected))
			}

			for idx, file := range testFiles {
				if file != tt.expected[idx] {
					t.Errorf("position %d: got %q, want %q", idx, file, tt.expected[idx])
				}
			}
		})
	}
}

func getEnvFileSortingTests() []struct {
	name     string
	files    []string
	expected []string
} {
	return []struct {
		name     string
		files    []string
		expected []string
	}{
		{
			name:     "basic precedence order",
			files:    []string{".env", ".env.local", ".env.development", ".env.development.local"},
			expected: []string{".env.development.local", ".env.local", ".env.development", ".env"},
		},
		{
			name:     "alphabetical within same weight",
			files:    []string{".env.production", ".env.development", ".env.staging"},
			expected: []string{".env.development", ".env.production", ".env.staging"},
		},
		{
			name: "comprehensive mix",
			files: []string{
				".env", ".env.local", ".env.development", ".env.production",
				".env.development.local", ".env.production.local", ".env.staging",
			},
			expected: []string{
				".env.development.local", ".env.production.local", ".env.local",
				".env.development", ".env.production", ".env.staging", ".env",
			},
		},
		{
			name:     "single file",
			files:    []string{".env"},
			expected: []string{".env"},
		},
	}
}

// testGetEnvFileWeight duplicates the logic from env.go for testing purposes.
func testGetEnvFileWeight(filename, baseEnvFile string) int {
	const (
		baseEnvWeight    = 0 // .env has lowest priority
		namedEnvWeight   = 1 // .env.<name> files
		localEnvWeight   = 2 // .env.local
		namedLocalWeight = 3 // .env.<name>.local has highest priority
	)

	if filename == baseEnvFile {
		return baseEnvWeight
	}

	if endsWith(filename, ".local") {
		if filename == baseEnvFile+".local" {
			return localEnvWeight
		}
		return namedLocalWeight
	}

	return namedEnvWeight
}

// testSortEnvFiles duplicates the sorting logic from env.go for testing purposes.
func testSortEnvFiles(envFiles []string, baseEnvFile string) {
	for outerIdx := range envFiles {
		for innerIdx := outerIdx + 1; innerIdx < len(envFiles); innerIdx++ {
			weight1 := testGetEnvFileWeight(envFiles[outerIdx], baseEnvFile)
			weight2 := testGetEnvFileWeight(envFiles[innerIdx], baseEnvFile)

			shouldSwap := false
			if weight1 < weight2 {
				shouldSwap = true
			} else if weight1 == weight2 {
				shouldSwap = envFiles[outerIdx] > envFiles[innerIdx]
			}

			if shouldSwap {
				envFiles[outerIdx], envFiles[innerIdx] = envFiles[innerIdx], envFiles[outerIdx]
			}
		}
	}
}

// Helper function to check if string ends with suffix.
func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
