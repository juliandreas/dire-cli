package cli_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/juliandreas/dire/internal/cli"
	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/flags"
)

type endToEndTestCase struct {
	name          string
	configDir     string // Directory in config file ("" means omit from config)
	flagDir       string // Directory from --directory flag ("" means no flag)
	expectError   bool
	expectedError error // Sentinel error to check with errors.Is()
}

type directoryFlagTestCase struct {
	name        string
	args        []string
	expectedDir string
}

const testConfigDir = "config-dir"

func TestSourcedFlagValidation(t *testing.T) {
	tests := []struct {
		name      string
		stub      bool
		sourced   bool
		check     bool
		expectErr bool
	}{
		{"sourced flag alone", false, true, false, false},
		{"stub flag alone", true, false, false, false},
		{"neither flag", false, false, false, false},
		{"both flags conflict", true, true, false, true},
		{"check only", false, false, true, false},
		{"sourced + check (conflict)", false, true, true, true},
		{"stub + check (conflict)", true, false, true, true},
		{"stub + sourced + check (multiple conflicts)", true, true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFlags := &flags.Flags{ //nolint:exhaustruct // Only need specific fields for this test
				TrimStrings: true,
				StubMode:    tt.stub,
				SourcedOnly: tt.sourced,
				Check:       tt.check,
			}
			testFlags.UpdateDerivedFlags()
			err := flags.ValidateFlagCombinations(testFlags, nil)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.name)
				}
			} else if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

func TestSourcedFlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{"sourced flag present", []string{"--sourced"}, true},
		{"sourced flag not present", []string{}, false},
		{"sourced flag with other flags", []string{"--sourced", "--keys", "auth.login"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := cli.NewRootCommand()
			rootCmd.SetArgs(tt.args)

			if err := rootCmd.ParseFlags(tt.args); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			sourced, err := flags.GetSourcedFlag(rootCmd)
			if err != nil {
				t.Fatalf("Failed to get sourced flag: %v", err)
			}

			if sourced != tt.expected {
				t.Errorf("Expected sourced flag to be %t, got %t", tt.expected, sourced)
			}
		})
	}
}

func TestCheckAndFormatFlagParsing(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedCheck  bool
		expectedFormat string
	}{
		{"check flag alone", []string{"--check"}, true, ""},
		{"check with default format", []string{"--check", "--format", "default"}, true, "default"},
		{"check with json format", []string{"--check", "--format", "json"}, true, "json"},
		{"check with JSON uppercase", []string{"--check", "--format", "JSON"}, true, "json"},
		{"check with padded format", []string{"--check", "--format", "  Json  "}, true, "json"},
		{"check with DEFAULT uppercase", []string{"--check", "--format", "DEFAULT"}, true, "default"},
		{"no flags", []string{}, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := cli.NewRootCommand()
			rootCmd.SetArgs(tt.args)

			if err := rootCmd.ParseFlags(tt.args); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			fs, err := flags.ParseFromCobraCommand(rootCmd)
			if err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}
			err = flags.ValidateFlagCombinations(fs, nil)
			if err != nil {
				t.Fatalf("ParseFromCobraCommand validation failed: %v", err)
			}

			if fs.Check != tt.expectedCheck {
				t.Errorf("Expected Check=%v, got Check=%v", tt.expectedCheck, fs.Check)
			}

			if fs.Format != tt.expectedFormat {
				t.Errorf("Expected Format=%q, got Format=%q", tt.expectedFormat, fs.Format)
			}

			// RequiresTranslationService should be computed as: !StubMode && !SourcedOnly && !Check
			expectedRequiresTranslationService := !fs.StubMode && !fs.SourcedOnly && !fs.Check
			if fs.RequiresTranslationService != expectedRequiresTranslationService {
				t.Errorf("Expected RequiresTranslationService=%v, got %v",
					expectedRequiresTranslationService, fs.RequiresTranslationService)
			}
		})
	}
}

func TestFormatWithoutCheckReturnsError(t *testing.T) {
	// Test that --format without --check returns validation error
	rootCmd := cli.NewRootCommand()
	args := []string{"--format", "json"} // no --check flag
	rootCmd.SetArgs(args)

	if err := rootCmd.ParseFlags(args); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	// This should fail validation because format requires check
	fs, err := flags.ParseFromCobraCommand(rootCmd)
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}
	err = flags.ValidateFlagCombinations(fs, nil)
	if err == nil {
		t.Fatalf("Expected error when --format is used without --check, got nil")
	}
}

func getDirectoryFlagTestCases() []directoryFlagTestCase {
	return []directoryFlagTestCase{
		{
			name:        "directory flag (raw in flags)",
			args:        []string{"--directory", "  ./i18n  "},
			expectedDir: "  ./i18n  ",
		},
		{
			name:        "directory flag without spaces",
			args:        []string{"--directory", "/path/to/locales"},
			expectedDir: "/path/to/locales",
		},
		{
			name:        "directory flag empty",
			args:        []string{"--directory", ""},
			expectedDir: "",
		},
		{
			name:        "no directory flag",
			args:        []string{},
			expectedDir: "",
		},
		{
			name:        "directory flag with other flags",
			args:        []string{"--directory", "  ./custom  ", "--check"},
			expectedDir: "  ./custom  ",
		},
		{
			name:        "directory with trailing slash",
			args:        []string{"--directory", "./i18n/"},
			expectedDir: "./i18n/",
		},
		{
			name:        "directory with parent segment",
			args:        []string{"--directory", "../locales"},
			expectedDir: "../locales",
		},
		{
			name:        "directory with double separators",
			args:        []string{"--directory", "./a//b"},
			expectedDir: "./a//b",
		},
		{
			name:        "directory with env token stays raw",
			args:        []string{"--directory", "${TEST_DIR}/locales"},
			expectedDir: "${TEST_DIR}/locales",
		},
	}
}

func TestDirectoryFlagParsing_RawValue(t *testing.T) {
	tests := getDirectoryFlagTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test CLI parsing
			rootCmd := cli.NewRootCommand()
			rootCmd.SetArgs(tt.args)

			if err := rootCmd.ParseFlags(tt.args); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			// Test flag extraction
			fs, err := flags.ParseFromCobraCommand(rootCmd)
			if err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}
			err = flags.ValidateFlagCombinations(fs, nil)
			if err != nil {
				t.Fatalf("ParseFromCobraCommand validation failed: %v", err)
			}

			// Verify directory value is passed through unmodified (raw)
			if fs.Directory != tt.expectedDir {
				t.Errorf("Expected Directory=%q, got Directory=%q", tt.expectedDir, fs.Directory)
			}
		})
	}
}

func TestDirectoryFlagOverridesPrecedence(t *testing.T) {
	// Integration test: CLI flag should override config file value
	rootCmd := cli.NewRootCommand()
	args := []string{"--directory", "  ./cli-dir  "}
	rootCmd.SetArgs(args)

	if err := rootCmd.ParseFlags(args); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	// Extract flags from CLI
	fs, err := flags.ParseFromCobraCommand(rootCmd)
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}
	err = flags.ValidateFlagCombinations(fs, nil)
	if err != nil {
		t.Fatalf("ParseFromCobraCommand validation failed: %v", err)
	}

	// Create a config with a different directory value
	//nolint:exhaustruct // Only need Files field for this test
	cfg := &config.Config{}
	cfg.Files.Directory = testConfigDir // This should be overridden

	// Verify precondition: initial config directory is set
	if cfg.Files.Directory != testConfigDir {
		t.Fatalf("Precondition failed: expected initial config dir %q, got %q", testConfigDir, cfg.Files.Directory)
	}

	// Apply CLI flag overrides (this is what happens in real command execution)
	cfg.ApplyDirectoryOverride(fs.Directory)
	cfg.ApplyLocaleOverrides(fs.Locales)
	cfg.ApplyIndentOverride(fs.Indent)

	// Verify that CLI flag wins (and gets normalized by config layer)
	expectedDir := filepath.Clean("cli-dir") // Normalized: trimmed and cleaned; OS-portable
	if cfg.Files.Directory != expectedDir {
		t.Errorf("Expected CLI flag to override config: got %q, want %q", cfg.Files.Directory, expectedDir)
	}
}

func TestDirectoryFlag_EmptyDoesNotOverrideConfig(t *testing.T) {
	// Test that empty directory flag preserves config directory value
	rootCmd := cli.NewRootCommand()
	args := []string{"--directory", ""}
	rootCmd.SetArgs(args)

	if err := rootCmd.ParseFlags(args); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	fs, err := flags.ParseFromCobraCommand(rootCmd)
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}
	err = flags.ValidateFlagCombinations(fs, nil)
	if err != nil {
		t.Fatalf("ParseFromCobraCommand validation failed: %v", err)
	}

	// Create config with existing directory
	//nolint:exhaustruct // Only need Files field for this test
	cfg := &config.Config{}
	cfg.Files.Directory = testConfigDir

	// Apply empty flag override - should NOT change config
	cfg.ApplyDirectoryOverride(fs.Directory)
	cfg.ApplyLocaleOverrides(fs.Locales)
	cfg.ApplyIndentOverride(fs.Indent)

	if cfg.Files.Directory != testConfigDir {
		t.Errorf("Expected config directory to remain %q, got %q", testConfigDir, cfg.Files.Directory)
	}
}

func getEndToEndValidationTestCases() []endToEndTestCase {
	return []endToEndTestCase{
		{
			name:          "config has directory, no flag - should succeed",
			configDir:     "demo",
			flagDir:       "",
			expectError:   false,
			expectedError: nil,
		},
		{
			name:          "no config directory, flag provided - should succeed",
			configDir:     "",
			flagDir:       "demo",
			expectError:   false,
			expectedError: nil,
		},
		{
			name:          "both config and flag, flag wins - should succeed",
			configDir:     testConfigDir,
			flagDir:       "flag-dir",
			expectError:   false,
			expectedError: nil,
		},
		{
			name:          "no config directory, no flag - should fail",
			configDir:     "",
			flagDir:       "",
			expectError:   true,
			expectedError: config.ErrFilesDirRequired,
		},
	}
}

func TestEndToEndValidationFlow(t *testing.T) {
	t.Parallel()

	tests := getEndToEndValidationTestCases()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runEndToEndValidationTest(t, tt)
		})
	}
}

func runEndToEndValidationTest(t *testing.T, tt endToEndTestCase) {
	t.Helper()

	// Create temporary directory and files for non-error test cases
	var localesDir string
	var cleanup func()
	if !tt.expectError {
		localesDir, cleanup = createTempTestFiles(t)
		defer cleanup()
	}

	// Create test config
	testConfig := createTestConfig(tt.configDir, localesDir, tt.expectError)

	// Parse CLI args and flags
	flagsInstance := parseTestFlags(t, tt.flagDir, localesDir, tt.expectError)

	// For the precedence scenario, force config to a different value before applying overrides
	if tt.configDir != "" && tt.flagDir != "" && !tt.expectError {
		testConfig.Files.Directory = tt.configDir // intentionally different (may not exist)
	}

	// Apply flag overrides and validate
	testConfig.ApplyDirectoryOverride(flagsInstance.Directory)
	testConfig.ApplyLocaleOverrides(flagsInstance.Locales)
	testConfig.ApplyIndentOverride(flagsInstance.Indent)
	err := testConfig.Validate(flagsInstance.RequiresTranslationService)

	// Check results
	switch {
	case tt.expectError:
		if err == nil {
			t.Errorf("Expected validation error, but got none")
		} else if !errors.Is(err, tt.expectedError) {
			t.Errorf("Expected error %v, got %v", tt.expectedError, err)
		}
	case err != nil:
		t.Errorf("Expected no validation error, got: %v", err)
	case tt.configDir != "" && tt.flagDir != "":
		// Assert that the flag override actually took effect
		expected := filepath.Clean(flagsInstance.Directory)
		if testConfig.Files.Directory != expected {
			t.Errorf("Expected directory to be overridden to %q, got %q", expected, testConfig.Files.Directory)
		}
	case tt.configDir != "" && tt.flagDir == "":
		// Assert that config directory without flag override uses temp localesDir
		if testConfig.Files.Directory != localesDir {
			t.Errorf("Expected config-only directory to be %q, got %q", localesDir, testConfig.Files.Directory)
		}
	}
}

func createTestConfig(configDir, localesDir string, expectError bool) *config.Config {
	//nolint:exhaustruct // Only need specific fields for this test
	testConfig := &config.Config{}
	testConfig.Project.Name = "Test Project"
	testConfig.Project.Description = "Test Description"
	testConfig.Files.Locales = map[string]string{
		"en.json": "en-US",
		"fr.json": "fr-FR",
	}

	// For success cases, use temp directory (with real files) to pass validation
	// For error cases, use the intended configDir (which may be empty to trigger error).
	// Note: In success paths we prefer localesDir so validation checks real files,
	// not the synthetic configDir value.
	if !expectError && localesDir != "" && configDir != "" {
		testConfig.Files.Directory = localesDir // Use temp dir with real locale files
	} else {
		testConfig.Files.Directory = configDir // Use intended config value
	}

	return testConfig
}

func parseTestFlags(t *testing.T, flagDir, localesDir string, expectError bool) *flags.Flags {
	t.Helper()

	var args []string
	if flagDir != "" {
		flagDirValue := flagDir
		if !expectError && localesDir != "" {
			flagDirValue = localesDir
		}
		args = append(args, "--directory", flagDirValue)
	}
	args = append(args, "--stub") // Add a mode flag to make it valid

	rootCmd := cli.NewRootCommand()
	rootCmd.SetArgs(args)
	if err := rootCmd.ParseFlags(args); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	flagsInstance, err := flags.ParseFromCobraCommand(rootCmd)
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}
	err = flags.ValidateFlagCombinations(flagsInstance, nil)
	if err != nil {
		t.Fatalf("Failed to extract flags: %v", err)
	}
	return flagsInstance
}

// createTempTestFiles creates a temporary directory with test locale files.
func createTempTestFiles(t *testing.T) (string, func()) {
	t.Helper()

	// Create temporary directory (auto-cleaned by the testing framework)
	tempDir := t.TempDir()

	// Create test locale files
	files := map[string]string{
		"en.json": `{"test": "value"}`,
		"fr.json": `{"test": "valeur"}`,
	}

	for filename, content := range files {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), constants.SecureFilePermissions); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	cleanup := func() {
		// No-op: t.TempDir() automatically handles cleanup
	}

	return tempDir, cleanup
}

// TestCheckModeValidationPath tests that Check=true results in RequiresTranslationService=false
// and that config validation works correctly for the no-translation path.
func TestCheckModeValidationPath(t *testing.T) {
	// Create flags with Check=true
	//nolint:exhaustruct // Only need specific fields for this test
	testFlags := &flags.Flags{
		Check:  true,
		Format: "json", // Valid combination with check
	}
	testFlags.UpdateDerivedFlags()

	// Verify that Check mode disables translation service requirement
	if testFlags.RequiresTranslationService {
		t.Error("Expected RequiresTranslationService=false when Check=true")
	}

	// Create temporary directory with locale files
	tempDir := t.TempDir()

	// Create test locale files
	files := map[string]string{
		"en.json": `{"test": "value"}`,
		"es.json": `{"test": "valor"}`,
	}

	for filename, content := range files {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), constants.SecureFilePermissions); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Create a minimal config and test validation with RequiresTranslationService=false
	//nolint:exhaustruct // Only need specific fields for this test
	cfg := &config.Config{}
	cfg.Project.Name = "Test Project"            // Required field
	cfg.Project.Description = "Test Description" // Required field
	cfg.Files.Directory = tempDir                // Use temp directory with real files
	cfg.Files.Locales = map[string]string{
		"en.json": "en-US",
		"es.json": "es-ES",
	}

	// This should not fail even without translation service configuration
	err := cfg.Validate(testFlags.RequiresTranslationService)
	if err != nil {
		t.Errorf("Config validation failed for Check mode (no translation service required): %v", err)
	}
}

func TestRephraseInvalidArgs(t *testing.T) {
	setupRephraseTestDir(t)
	tests := getRephraseInvalidArgsTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test command with rephrase and invalid args
			rootCmd := cli.NewRootCommand()
			args := []string{"--rephrase"}

			// Add keys args
			for _, key := range tt.keys {
				args = append(args, "--keys", key)
			}

			// Add locale arg if not empty
			if tt.locale != "" {
				args = append(args, "--locale", tt.locale)
			}

			rootCmd.SetArgs(args)

			// Execute command - should return validation error
			err := rootCmd.Execute()

			if err == nil {
				t.Errorf("Expected validation error but got no error")
			}
			// The error should be related to rephrase validation
			// (either from flag validation or from executeRephrase)
		})
	}
}

// setupRephraseTestDir creates a temporary directory with a valid config file for rephrase tests.
func setupRephraseTestDir(t *testing.T) {
	t.Helper()
	// Create temporary directory with config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".dire.toml")

	configContent := `[project]
name = "Test Project"
description = "Test Description"

[files]
directory = "."

[files.locales]
"en.json" = "en-US"

[providers]
active = "claude"

[providers.claude]
model = "claude-3-haiku-20240307"
`

	if err := os.WriteFile(configPath, []byte(configContent), constants.SecureFilePermissions); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Change to temp directory for test
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	})

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
}

// getRephraseInvalidArgsTestCases returns test cases for rephrase invalid arguments.
func getRephraseInvalidArgsTestCases() []struct {
	name   string
	keys   []string
	locale string
} {
	return []struct {
		name   string
		keys   []string
		locale string
	}{
		{
			name:   "no keys provided",
			keys:   []string{},
			locale: "en",
		},
		{
			name:   "multiple keys provided",
			keys:   []string{"key1", "key2"},
			locale: "en",
		},
		{
			name:   "empty locale",
			keys:   []string{"test.key"},
			locale: "",
		},
		{
			name:   "whitespace-only locale",
			keys:   []string{"test.key"},
			locale: "   ",
		},
	}
}
