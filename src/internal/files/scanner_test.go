package files_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/testutil"
	"github.com/juliandreas/dire/internal/types"
	"go.uber.org/goleak"
)

// Constants.
const testSensitiveContent = `{"password": "super-secret-password", "api_key": "sk-secret123"}`

// Helper functions (private)

// createTestConfig creates a config for testing with custom directory/locales.
func createTestConfig(directory string, locales map[string]string) *config.Config {
	cfg := testutil.MockConfig()
	cfg.Files.Directory = directory
	cfg.Files.Locales = locales
	return cfg
}

// testSecurityBoundary tests a single security boundary case.
func testSecurityBoundary(t *testing.T, root *os.Root, testPath string) {
	t.Helper()
	file, err := root.Open(testPath)
	if err != nil {
		t.Logf("os.Root correctly blocked access to %s: %v", testPath, err)
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Logf("Failed to close file: %v", closeErr)
		}
	}()

	// If os.Root allowed access, verify it's not sensitive content
	data := make([]byte, 1024)
	bytesRead, readErr := file.Read(data)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		t.Logf("Read from file failed (this may be expected): %v", readErr)
		return
	}

	content := string(data[:bytesRead])
	if content == testSensitiveContent {
		t.Errorf("SECURITY VIOLATION: os.Root allowed reading sensitive content via %s!", testPath)
	} else {
		t.Logf("os.Root allowed access to %s but content is safe", testPath)
	}
}

// createSymlinkAttackScenario sets up a symlink that tries to escape the locale directory.
func createSymlinkAttackScenario(t *testing.T, tempDir, localeDir string) {
	t.Helper()

	// Create directories
	if err := os.Mkdir(localeDir, constants.SecureDirPermissions); err != nil {
		t.Fatalf("Failed to create locale directory: %v", err)
	}
	sensitiveDir := filepath.Join(tempDir, "sensitive")
	if err := os.Mkdir(sensitiveDir, constants.SecureDirPermissions); err != nil {
		t.Fatalf("Failed to create sensitive directory: %v", err)
	}

	// Create sensitive file outside locale directory
	sensitiveFile := filepath.Join(sensitiveDir, "secret.json")
	if err := os.WriteFile(sensitiveFile, []byte(testSensitiveContent), constants.SecureFilePermissions); err != nil {
		t.Fatalf("Failed to create sensitive file: %v", err)
	}

	// Create malicious symlink inside locale directory pointing outside
	symlinkPath := filepath.Join(localeDir, "en.json")
	if err := os.Symlink(sensitiveFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}
}

// verifyScanIsSecure checks that scanning doesn't leak sensitive data through symlinks.
func verifyScanIsSecure(t *testing.T, err error, localeFiles map[types.Locale]*types.LocaleFile) {
	t.Helper()

	if err != nil {
		t.Logf("ScanLocaleFiles correctly failed with symlink escape attempt: %v", err)
		if len(localeFiles) != 0 {
			t.Errorf("Expected no files when scan fails, got %d files", len(localeFiles))
		}
		return
	}

	// If scan succeeded, verify no sensitive data leaked by checking parsed contents
	// This tests the actual security boundary rather than bypassing it with os.ReadFile
	t.Logf("ScanLocaleFiles succeeded with %d files", len(localeFiles))
	for locale, file := range localeFiles {
		// Check for sensitive keys that shouldn't be present if os.Root worked correctly
		if _, hasPassword := file.Contents["password"]; hasPassword {
			t.Errorf("SECURITY VIOLATION: parsed content contains 'password' key via symlink in locale %s", locale)
		}
		if _, hasAPIKey := file.Contents["api_key"]; hasAPIKey {
			t.Errorf("SECURITY VIOLATION: parsed content contains 'api_key' key via symlink in locale %s", locale)
		}
	}
}

// verifyRootBlocksSymlinks tests that os.Root directly blocks symlink access.
func verifyRootBlocksSymlinks(t *testing.T, localeDir string) {
	t.Helper()

	root, err := os.OpenRoot(localeDir)
	if err != nil {
		t.Fatalf("Failed to create os.Root: %v", err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Logf("Warning: failed to close root: %v", closeErr)
		}
	}()

	testSecurityBoundary(t, root, "en.json")
}

// Test functions (public)

// TestScanLocaleFiles_SymlinkEscapePrevention tests that os.Root prevents symlink-based directory traversal attacks.
func TestScanLocaleFiles_SymlinkEscapePrevention(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Skip on Windows where symlink creation requires privileges
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows (requires admin privileges)")
	}

	tempDir := t.TempDir()
	localeDir := filepath.Join(tempDir, "locales")
	createSymlinkAttackScenario(t, tempDir, localeDir)

	// Test that ScanLocaleFiles handles symlink attacks securely
	cfg := createTestConfig(localeDir, map[string]string{"en.json": "en-US"})
	ctx := context.Background()
	localeFiles, err := files.ScanLocaleFiles(ctx, cfg)

	verifyScanIsSecure(t, err, localeFiles)

	// Test that os.Root blocks direct symlink access
	verifyRootBlocksSymlinks(t, localeDir)

	t.Log("Symlink escape prevention test completed successfully")
}

// TestScanLocaleFiles_NormalOperation tests that normal locale file scanning still works
// after implementing security measures.
//
//nolint:funlen,nestif // Security test requires comprehensive verification
func TestScanLocaleFiles_NormalOperation(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Create a temporary directory structure for testing
	tempDir := t.TempDir()
	localeDir := filepath.Join(tempDir, "locales")
	if err := os.Mkdir(localeDir, constants.SecureDirPermissions); err != nil {
		t.Fatalf("Failed to create locale directory: %v", err)
	}

	// Create legitimate locale files
	enContent := `{"hello": "Hello", "world": "World"}`
	frContent := `{"hello": "Bonjour", "world": "Monde"}`

	enFile := filepath.Join(localeDir, "en.json")
	frFile := filepath.Join(localeDir, "fr.json")

	if err := os.WriteFile(enFile, []byte(enContent), constants.SecureFilePermissions); err != nil {
		t.Fatalf("Failed to create en.json: %v", err)
	}
	if err := os.WriteFile(frFile, []byte(frContent), constants.SecureFilePermissions); err != nil {
		t.Fatalf("Failed to create fr.json: %v", err)
	}

	// Create configuration
	cfg := createTestConfig(localeDir, map[string]string{
		"en.json": "en-US",
		"fr.json": "fr-FR",
	})

	// Scan locale files
	ctx := context.Background()
	localeFiles, err := files.ScanLocaleFiles(ctx, cfg)
	if err != nil {
		t.Fatalf("ScanLocaleFiles failed: %v", err)
	}

	// Verify we got the expected files
	if len(localeFiles) != 2 {
		t.Errorf("Expected 2 locale files, got %d", len(localeFiles))
	}

	// Verify content is correct
	enLocale := localeFiles["en-US"]
	if enLocale == nil {
		t.Errorf("Missing en-US locale")
	} else if enLocale.Contents["hello"] != "Hello" {
		t.Errorf("Incorrect en-US content: got %v", enLocale.Contents["hello"])
	}

	frLocale := localeFiles["fr-FR"]
	if frLocale == nil {
		t.Errorf("Missing fr-FR locale")
	} else if frLocale.Contents["hello"] != "Bonjour" {
		t.Errorf("Incorrect fr-FR content: got %v", frLocale.Contents["hello"])
	}

	t.Log("Normal operation test completed successfully")
}

// TestScanLocaleFiles_SecurityBoundaries tests that os.Root properly constrains file access
// to the configured directory, providing a cross-platform security test.
//
//nolint:funlen // Security test complexity is justified for comprehensive coverage
func TestScanLocaleFiles_SecurityBoundaries(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Create a temporary directory structure for testing
	tempDir := t.TempDir()

	// Create the locale directory
	localeDir := filepath.Join(tempDir, "locales")
	if err := os.Mkdir(localeDir, constants.SecureDirPermissions); err != nil {
		t.Fatalf("Failed to create locale directory: %v", err)
	}

	// Create a sensitive file outside the locale directory
	sensitiveDir := filepath.Join(tempDir, "sensitive")
	if err := os.Mkdir(sensitiveDir, constants.SecureDirPermissions); err != nil {
		t.Fatalf("Failed to create sensitive directory: %v", err)
	}

	sensitiveFile := filepath.Join(sensitiveDir, "secret.json")
	sensitiveContent := `{"password": "super-secret-password", "api_key": "sk-secret123"}`
	if err := os.WriteFile(sensitiveFile, []byte(sensitiveContent), constants.SecureFilePermissions); err != nil {
		t.Fatalf("Failed to create sensitive file: %v", err)
	}

	// Test: Try to access files outside the root directory using os.Root directly
	root, err := os.OpenRoot(localeDir)
	if err != nil {
		t.Fatalf("Failed to create os.Root: %v", err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Logf("Warning: failed to close root: %v", closeErr)
		}
	}()

	// Attempt to access files outside the root using relative path traversal
	testCases := []struct {
		name string
		path string
	}{
		{"parent directory access", "../sensitive/secret.json"},
		{"double parent directory access", "../../sensitive/secret.json"},
		{"absolute path outside root", sensitiveFile},
		{"complex traversal", "../../../" + sensitiveFile},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testSecurityBoundary(t, root, tc.path)
		})
	}

	t.Log("Security boundaries test completed successfully")
}

// TestScanLocaleFiles_FileSizeLimitError tests that oversized files produce clear error messages.
func TestScanLocaleFiles_FileSizeLimitError(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Create a temporary directory structure for testing
	tempDir := t.TempDir()
	localeDir := filepath.Join(tempDir, "locales")
	if err := os.Mkdir(localeDir, constants.SecureDirPermissions); err != nil {
		t.Fatalf("Failed to create locale directory: %v", err)
	}

	// Create an oversized locale file (larger than 5 MiB limit)
	oversizedFile := filepath.Join(localeDir, "huge.json")
	// Create content larger than maxLocaleFileBytes (5 MiB)
	largeContent := make([]byte, 6*1024*1024) // 6 MiB
	for i := range largeContent {
		largeContent[i] = 'a' // Fill with 'a' characters
	}

	if err := os.WriteFile(oversizedFile, largeContent, constants.SecureFilePermissions); err != nil {
		t.Fatalf("Failed to create oversized file: %v", err)
	}

	// Create configuration to scan the oversized file
	cfg := createTestConfig(localeDir, map[string]string{
		"huge.json": "en-US",
	})

	// Attempt to scan - should fail with clear error message
	ctx := context.Background()
	localeFiles, err := files.ScanLocaleFiles(ctx, cfg)

	// Verify we get the expected error
	if err == nil {
		t.Errorf("Expected error for oversized file, but scan succeeded")
		return
	}

	// Check the sentinel error for robustness
	if !errors.Is(err, files.ErrFileTooLarge) {
		t.Errorf("Expected files.ErrFileTooLarge, got: %v", err)
	}

	// Check that the error message mentions file size (human context)
	errorMsg := err.Error()
	if !strings.Contains(errorMsg, "exceeds maximum allowed size") {
		t.Errorf("Error message doesn't mention size limit: %s", errorMsg)
	}

	// Check that the error message includes actual file size
	if !strings.Contains(errorMsg, "6291456 bytes") { // 6 MiB in bytes
		t.Errorf("Error message doesn't include actual file size: %s", errorMsg)
	}

	// Check that the error message includes the limit
	if !strings.Contains(errorMsg, "5242880 bytes") { // 5 MiB in bytes
		t.Errorf("Error message doesn't include size limit: %s", errorMsg)
	}

	// Verify we got no files back when the operation failed
	if len(localeFiles) != 0 {
		t.Errorf("Expected no files when scan fails due to size limit, got %d files", len(localeFiles))
	}

	t.Logf("File size limit error test completed successfully with error: %s", errorMsg)
}

// TestScanLocaleFiles_DuplicateLocaleDetection tests that duplicate locale mappings are logged.
func TestScanLocaleFiles_DuplicateLocaleDetection(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Create a temporary directory structure for testing
	tempDir := t.TempDir()
	localeDir := filepath.Join(tempDir, "locales")
	if err := os.Mkdir(localeDir, constants.SecureDirPermissions); err != nil {
		t.Fatalf("Failed to create locale directory: %v", err)
	}

	// Create two files that will map to the same locale
	enContent1 := `{"hello": "Hello from file 1"}`
	enContent2 := `{"hello": "Hello from file 2"}`

	enFile1 := filepath.Join(localeDir, "en.json")
	enFile2 := filepath.Join(localeDir, "english.json")

	if err := os.WriteFile(enFile1, []byte(enContent1), constants.SecureFilePermissions); err != nil {
		t.Fatalf("Failed to create en.json: %v", err)
	}
	if err := os.WriteFile(enFile2, []byte(enContent2), constants.SecureFilePermissions); err != nil {
		t.Fatalf("Failed to create english.json: %v", err)
	}

	// Create configuration that maps both files to the same locale
	cfg := createTestConfig(localeDir, map[string]string{
		"en.json":      "en-US", // First mapping
		"english.json": "en-US", // Duplicate mapping to same locale
	})

	// Scan locale files - should succeed but log a warning about duplicate
	ctx := context.Background()
	localeFiles, err := files.ScanLocaleFiles(ctx, cfg)
	if err != nil {
		t.Fatalf("ScanLocaleFiles failed: %v", err)
	}

	// Verify we got exactly one locale (the later file should overwrite the earlier)
	if len(localeFiles) != 1 {
		t.Errorf("Expected 1 locale file, got %d", len(localeFiles))
	}

	// Verify the final content is from the second file (alphabetically later filename)
	enLocale := localeFiles["en-US"]
	if enLocale == nil {
		t.Errorf("Missing en-US locale")
	} else {
		// The second file (english.json) should have overwritten the first (en.json)
		expectedContent := "Hello from file 2"
		if enLocale.Contents["hello"] != expectedContent {
			t.Errorf("Expected content from second file, got: %v", enLocale.Contents["hello"])
		}
		// Verify the path reflects the final file
		if !strings.Contains(enLocale.Path, "english.json") {
			t.Errorf("Expected path to contain 'english.json', got: %s", enLocale.Path)
		}
	}

	t.Log("Duplicate locale detection test completed successfully")
}

// TestScanLocaleFiles_NumericFidelity tests that numeric values are preserved with exact precision.
//
//nolint:funlen // Test requires comprehensive verification of different numeric types
func TestScanLocaleFiles_NumericFidelity(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Create a temporary directory structure for testing
	tempDir := t.TempDir()
	localeDir := filepath.Join(tempDir, "locales")
	if err := os.Mkdir(localeDir, constants.SecureDirPermissions); err != nil {
		t.Fatalf("Failed to create locale directory: %v", err)
	}

	// Create a locale file with various numeric types
	numericContent := `{
		"largeInteger": 9007199254740991,
		"smallInteger": 42,
		"preciseDecimal": 123.456789012345,
		"scientificNotation": 1.23e-10,
		"negativeNumber": -999.999
	}`

	numericFile := filepath.Join(localeDir, "numbers.json")
	if err := os.WriteFile(numericFile, []byte(numericContent), constants.SecureFilePermissions); err != nil {
		t.Fatalf("Failed to create numbers.json: %v", err)
	}

	// Create configuration
	cfg := createTestConfig(localeDir, map[string]string{
		"numbers.json": "en-US",
	})

	// Scan locale files
	ctx := context.Background()
	localeFiles, err := files.ScanLocaleFiles(ctx, cfg)
	if err != nil {
		t.Fatalf("ScanLocaleFiles failed: %v", err)
	}

	// Verify we got the expected file
	if len(localeFiles) != 1 {
		t.Errorf("Expected 1 locale file, got %d", len(localeFiles))
	}

	enLocale := localeFiles["en-US"]
	if enLocale == nil {
		t.Fatalf("Missing en-US locale")
	}

	// Verify numeric values are preserved as json.Number (not converted to float64)
	testCases := []struct {
		key           string
		expectedValue string
	}{
		{"largeInteger", "9007199254740991"},
		{"smallInteger", "42"},
		{"preciseDecimal", "123.456789012345"},
		{"scientificNotation", "1.23e-10"},
		{"negativeNumber", "-999.999"},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			value, exists := enLocale.Contents[tc.key]
			if !exists {
				t.Errorf("Key %s not found in locale contents", tc.key)
				return
			}

			// With json.Decoder.UseNumber(), numeric values should be json.Number
			switch numValue := value.(type) {
			case json.Number:
				if string(numValue) != tc.expectedValue {
					t.Errorf("Key %s: expected %s, got %s", tc.key, tc.expectedValue, string(numValue))
				}
			default:
				t.Errorf("Key %s: expected json.Number, got %T (%v)", tc.key, numValue, numValue)
			}
		})
	}

	t.Log("Numeric fidelity test completed successfully")
}

// TestScanLocaleFiles_Cancellation tests that context cancellation is properly handled.
func TestScanLocaleFiles_Cancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Create a temporary directory with locale files
	tempDir := t.TempDir()
	localeDir := filepath.Join(tempDir, "locales")
	if err := os.Mkdir(localeDir, constants.SecureDirPermissions); err != nil {
		t.Fatalf("Failed to create locale directory: %v", err)
	}

	// Create multiple dummy files to increase chance of hitting the loop cancellation check
	for i := range 5 {
		filename := filepath.Join(localeDir, fmt.Sprintf("locale-%d.json", i))
		content := `{"key": "value"}`
		if err := os.WriteFile(filename, []byte(content), constants.SecureFilePermissions); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Create configuration with glob pattern to match all files
	cfg := createTestConfig(localeDir, map[string]string{"*.json": "en-US"})

	// Test immediate cancellation (tests early context check)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := files.ScanLocaleFiles(ctx, cfg)
	if err == nil {
		t.Fatalf("Expected context cancellation error, got nil")
	}

	// Verify it's actually a context cancellation error
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}

	t.Log("Context cancellation test completed successfully")
}

// TestScanLocaleFiles_EmptyFile tests that empty files are handled correctly.
func TestScanLocaleFiles_EmptyFile(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Create a temporary directory structure for testing
	tempDir := t.TempDir()
	localeDir := filepath.Join(tempDir, "locales")
	if err := os.Mkdir(localeDir, constants.SecureDirPermissions); err != nil {
		t.Fatalf("Failed to create locale directory: %v", err)
	}

	// Test cases for different "empty" scenarios
	testCases := []struct {
		name     string
		content  string
		expected map[string]any
	}{
		{"completely empty", "", map[string]any{}},
		{"whitespace only", "   \n\t  \n  ", map[string]any{}},
		{"tabs and spaces", "\t\t   \t", map[string]any{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create empty/whitespace file
			fileName := strings.ReplaceAll(tc.name, " ", "_") + ".json"
			filePath := filepath.Join(localeDir, fileName)
			if err := os.WriteFile(filePath, []byte(tc.content), constants.SecureFilePermissions); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Create configuration
			cfg := createTestConfig(localeDir, map[string]string{fileName: "en-US"})

			// Scan locale files
			ctx := context.Background()
			localeFiles, err := files.ScanLocaleFiles(ctx, cfg)
			if err != nil {
				t.Fatalf("ScanLocaleFiles failed: %v", err)
			}

			// Verify we got exactly one locale file
			if len(localeFiles) != 1 {
				t.Errorf("Expected 1 locale file, got %d", len(localeFiles))
			}

			// Verify empty file results in empty object
			enLocale := localeFiles["en-US"]
			if enLocale == nil {
				t.Fatalf("Missing en-US locale")
			}

			// Check that contents is an empty map
			if len(enLocale.Contents) != 0 {
				t.Errorf("Expected empty contents for %s file, got: %v", tc.name, enLocale.Contents)
			}

			// Verify it's the expected empty map
			if fmt.Sprintf("%v", enLocale.Contents) != fmt.Sprintf("%v", tc.expected) {
				t.Errorf("Contents don't match expected empty map. Got: %v, Expected: %v",
					enLocale.Contents, tc.expected)
			}
		})
	}

	t.Log("Empty file test completed successfully")
}
