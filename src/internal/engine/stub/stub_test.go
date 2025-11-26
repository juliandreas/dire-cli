package stub_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/juliandreas/dire/internal/engine/stub"
	"github.com/juliandreas/dire/internal/testutil"
	"github.com/juliandreas/dire/internal/types"
)

func TestApplyStubsEmpty(t *testing.T) {
	runApplyStubsTest(t, []types.MissingTranslation{}, 0, nil)
}

func TestApplyStubsSingle(t *testing.T) {
	missing := []types.MissingTranslation{
		{
			Path:         []string{"auth", "login"},
			SourceLocale: "en-US",
			SourceValue:  "Login",
			TargetLocale: "sv-SE",
		},
	}
	expected := []types.LocaleContent{
		{
			Locale: "sv-SE",
			Path:   []string{"auth", "login"},
			Value:  "",
		},
	}
	runApplyStubsTest(t, missing, 1, expected)
}

func TestApplyStubsMultiple(t *testing.T) {
	missing := []types.MissingTranslation{
		{
			Path:         []string{"auth", "login"},
			SourceLocale: "en-US",
			SourceValue:  "Login",
			TargetLocale: "sv-SE",
		},
		{
			Path:         []string{"auth", "logout"},
			SourceLocale: "en-US",
			SourceValue:  "Logout",
			TargetLocale: "sv-SE",
		},
	}
	expected := []types.LocaleContent{
		{
			Locale: "sv-SE",
			Path:   []string{"auth", "login"},
			Value:  "",
		},
		{
			Locale: "sv-SE",
			Path:   []string{"auth", "logout"},
			Value:  "",
		},
	}
	runApplyStubsTest(t, missing, 2, expected)
}

func TestApplyStubsNested(t *testing.T) {
	missing := []types.MissingTranslation{
		{
			Path:         []string{"auth", "form"},
			SourceLocale: "en-US",
			SourceValue: map[string]any{
				"title":    "Login Form",
				"subtitle": "Please enter your credentials",
			},
			TargetLocale: "sv-SE",
		},
	}
	expected := []types.LocaleContent{
		{
			Locale: "sv-SE",
			Path:   []string{"auth", "form"},
			Value: map[string]any{
				"title":    "",
				"subtitle": "",
			},
		},
	}
	runApplyStubsTest(t, missing, 2, expected)
}

func TestApplyStubsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := testutil.MockConfig()

	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "stub_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			t.Logf("Failed to remove temp dir: %v", removeErr)
		}
	}()

	// Create mock locale files with temp paths
	localeFiles := createMockLocaleFiles(tempDir)

	missing := []types.MissingTranslation{
		{
			Path:         []string{"auth", "login"},
			SourceLocale: "en-US",
			SourceValue:  "Login",
			TargetLocale: "sv-SE",
		},
	}

	// Cancel context before calling ApplyStubs
	cancel()

	count, content, err := stub.ApplyStubs(ctx, missing, localeFiles, "en-US", cfg)

	// Should return context cancellation error
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ApplyStubs() with canceled context error = %v, expected %v", err, context.Canceled)
	}

	// Should return partial results
	if count != 0 {
		t.Errorf("ApplyStubs() with canceled context count = %v, expected 0", count)
	}

	if len(content) != 0 {
		t.Errorf("ApplyStubs() with canceled context content length = %v, expected 0", len(content))
	}
}

// runApplyStubsTest is a helper function to run ApplyStubs tests.
func runApplyStubsTest(
	t *testing.T,
	missing []types.MissingTranslation,
	expectedCount int,
	expectedContent []types.LocaleContent,
) {
	ctx := context.Background()
	cfg := testutil.MockConfig()

	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "stub_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			t.Logf("Failed to remove temp dir: %v", removeErr)
		}
	}()

	// Create mock locale files with temp paths
	localeFiles := createMockLocaleFiles(tempDir)

	count, content, err := stub.ApplyStubs(ctx, missing, localeFiles, "en-US", cfg)
	// Check error
	if err != nil {
		t.Errorf("ApplyStubs() error = %v, expected no error", err)
		return
	}

	// Check count
	if count != expectedCount {
		t.Errorf("ApplyStubs() count = %v, expected %v", count, expectedCount)
	}

	// Check content length
	if len(content) != len(expectedContent) {
		t.Errorf("ApplyStubs() content length = %v, expected %v", len(content), len(expectedContent))
		return
	}

	// Check each translated content
	for index, expected := range expectedContent {
		validateLocaleContent(t, index, content, expected)
	}
}

// createMockLocaleFiles creates mock locale files for testing.
func createMockLocaleFiles(tempDir string) map[types.Locale]*types.LocaleFile {
	return map[types.Locale]*types.LocaleFile{
		"en-US": {
			Path:     filepath.Join(tempDir, "en.json"),
			Locale:   "en-US",
			Contents: make(map[string]any),
		},
		"sv-SE": {
			Path:     filepath.Join(tempDir, "sv.json"),
			Locale:   "sv-SE",
			Contents: make(map[string]any),
		},
	}
}

// validateLocaleContent validates a single translated content item.
func validateLocaleContent(
	t *testing.T,
	index int,
	content []types.LocaleContent,
	expected types.LocaleContent,
) {
	if index >= len(content) {
		t.Errorf("ApplyStubs() missing content at index %v", index)
		return
	}

	actual := content[index]

	// Check locale
	if actual.Locale != expected.Locale {
		t.Errorf("ApplyStubs() content[%v].Locale = %v, expected %v", index, actual.Locale, expected.Locale)
	}

	// Check path
	validatePath(t, index, actual.Path, expected.Path)

	// Check value
	validateValue(t, index, actual.Value, expected.Value)
}

// validatePath validates the path array.
func validatePath(t *testing.T, index int, actualPath, expectedPath []string) {
	if len(actualPath) != len(expectedPath) {
		t.Errorf("ApplyStubs() content[%v].Path length = %v, expected %v", index, len(actualPath), len(expectedPath))
		return
	}

	for j, pathPart := range expectedPath {
		if actualPath[j] != pathPart {
			t.Errorf("ApplyStubs() content[%v].Path[%v] = %v, expected %v", index, j, actualPath[j], pathPart)
		}
	}
}

// validateValue validates the translated value (simple or nested).
func validateValue(t *testing.T, index int, actualValue, expectedValue any) {
	// For nested objects, check structure
	if expectedMap, ok := expectedValue.(map[string]any); ok {
		validateMapValue(t, index, actualValue, expectedMap)
	} else if actualValue != expectedValue {
		// For simple values
		t.Errorf("ApplyStubs() content[%v].Value = %v, expected %v", index, actualValue, expectedValue)
	}
}

// validateMapValue validates a map value.
func validateMapValue(t *testing.T, index int, actualValue any, expectedMap map[string]any) {
	actualMap, ok := actualValue.(map[string]any)
	if !ok {
		t.Errorf("ApplyStubs() content[%v].Value is not a map", index)
		return
	}

	if len(actualMap) != len(expectedMap) {
		t.Errorf("ApplyStubs() content[%v].Value map length = %v, expected %v", index, len(actualMap), len(expectedMap))
		return
	}

	for key, expectedVal := range expectedMap {
		actualVal, exists := actualMap[key]
		if !exists {
			t.Errorf("ApplyStubs() content[%v].Value missing key %v", index, key)
			continue
		}
		if actualVal != expectedVal {
			t.Errorf("ApplyStubs() content[%v].Value[%v] = %v, expected %v", index, key, actualVal, expectedVal)
		}
	}
}
