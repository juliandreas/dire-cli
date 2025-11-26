package flags_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/juliandreas/dire/internal/flags"
	"github.com/spf13/cobra"
)

// fromCobraCommandTestCase represents a single test case for ParseFromCobraCommand + ValidateFlagCombinations.
type fromCobraCommandTestCase struct {
	name        string
	flagValues  map[string]any
	expected    *flags.Flags
	expectError bool
}

// updateDerivedFlagsTestCase represents a test case for UpdateDerivedFlags.
type updateDerivedFlagsTestCase struct {
	name                               string
	stubMode                           bool
	sourcedOnly                        bool
	check                              bool
	expectedRequiresTranslationService bool
}

// flagsValidateTestCase represents a test case for Flags.Validate().
type flagsValidateTestCase struct {
	name        string
	flags       *flags.Flags
	expectError bool
}

// getFromCobraCommandTestCases returns all test cases for ParseFromCobraCommand + ValidateFlagCombinations.
func getFromCobraCommandTestCases() []fromCobraCommandTestCase {
	var testCases []fromCobraCommandTestCase
	testCases = append(testCases, getBasicFlagTestCases()...)
	testCases = append(testCases, getComplexFlagTestCases()...)
	testCases = append(testCases, getConflictTestCases()...)
	return testCases
}

// newTestFlags creates a default Flags struct for testing with optional overrides.
//
//nolint:gocyclo,exhaustruct // Complexity from switch statement is acceptable for test helper
func newTestFlags(overrides map[string]any) *flags.Flags {
	flagSet := &flags.Flags{
		TrimStrings: true, StubMode: false, IncludeStubs: false,
		SourcedOnly: false, Rephrase: false, Check: false, Format: "", Prune: false,
		Keys: nil, Locale: "", Context: "", Directory: "", Locales: nil, Indent: -1,
		Formality: "", ProjectName: "", ProjectDesc: "", ProjectDom: "",
		ProviderActive: "", ProviderModel: "", ProviderTemperature: -1, ProviderMaxTokens: -1,
		RequiresTranslationService: true,
	}

	for key, value := range overrides {
		switch key {
		case "TrimStrings":
			flagSet.TrimStrings = value.(bool)
		case "StubMode":
			flagSet.StubMode = value.(bool)
		case "IncludeStubs":
			flagSet.IncludeStubs = value.(bool)
		case "SourcedOnly":
			flagSet.SourcedOnly = value.(bool)
		case "Rephrase":
			flagSet.Rephrase = value.(bool)
		case "Check":
			flagSet.Check = value.(bool)
		case "Format":
			flagSet.Format = value.(string)
		case "Keys":
			flagSet.Keys = value.([]string)
		case "Locale":
			flagSet.Locale = value.(string)
		case "Context":
			flagSet.Context = value.(string)
		case "Directory":
			flagSet.Directory = value.(string)
		case "Formality":
			flagSet.Formality = value.(string)
		case "ProjectName":
			flagSet.ProjectName = value.(string)
		case "ProjectDesc":
			flagSet.ProjectDesc = value.(string)
		case "ProjectDom":
			flagSet.ProjectDom = value.(string)
		case "RequiresTranslationService":
			flagSet.RequiresTranslationService = value.(bool)
		}
	}
	return flagSet
}

// getBasicFlagTestCases returns basic individual flag test cases.
func getBasicFlagTestCases() []fromCobraCommandTestCase {
	return []fromCobraCommandTestCase{
		{
			name:        "default values",
			flagValues:  map[string]any{},
			expected:    newTestFlags(nil),
			expectError: false,
		},
		{
			name:        "no-trim flag",
			flagValues:  map[string]any{"no-trim": true},
			expected:    newTestFlags(map[string]any{"TrimStrings": false}),
			expectError: false,
		},
		{
			name:        "stub mode",
			flagValues:  map[string]any{"stub": true},
			expected:    newTestFlags(map[string]any{"StubMode": true, "RequiresTranslationService": false}),
			expectError: false,
		},
		{
			name:        "sourced only mode",
			flagValues:  map[string]any{"sourced": true},
			expected:    newTestFlags(map[string]any{"SourcedOnly": true, "RequiresTranslationService": false}),
			expectError: false,
		},
		{
			name:        "include stubs",
			flagValues:  map[string]any{"include-stubs": true},
			expected:    newTestFlags(map[string]any{"IncludeStubs": true}),
			expectError: false,
		},
		{
			name:        "context flag",
			flagValues:  map[string]any{"context": "Dashboard UI components"},
			expected:    newTestFlags(map[string]any{"Context": "Dashboard UI components"}),
			expectError: false,
		},
		{
			name:        "directory flag (raw)",
			flagValues:  map[string]any{"directory": "  ./i18n  "},
			expected:    newTestFlags(map[string]any{"Directory": "  ./i18n  "}),
			expectError: false,
		},
	}
}

// getComplexFlagTestCases returns complex multi-flag combination test cases.
func getComplexFlagTestCases() []fromCobraCommandTestCase {
	return []fromCobraCommandTestCase{
		{
			name:        "keys filter",
			flagValues:  map[string]any{"keys": []string{"auth.login", "auth.logout"}},
			expected:    newTestFlags(map[string]any{"Keys": []string{"auth.login", "auth.logout"}}),
			expectError: false,
		},
		{
			name: "complex valid combination",
			flagValues: map[string]any{
				"sourced": true, "include-stubs": true, "no-trim": true, "keys": []string{"cart.title"},
			},
			expected: newTestFlags(map[string]any{
				"TrimStrings": false, "IncludeStubs": true, "SourcedOnly": true, "RequiresTranslationService": false,
				"Keys": []string{"cart.title"},
			}),
			expectError: false,
		},
		{
			name:        "check with json format",
			flagValues:  map[string]any{"check": true, "format": "json"},
			expected:    newTestFlags(map[string]any{"Check": true, "Format": "json", "RequiresTranslationService": false}),
			expectError: false,
		},
		{
			name:        "check with default format",
			flagValues:  map[string]any{"check": true, "format": "default"},
			expected:    newTestFlags(map[string]any{"Check": true, "Format": "default", "RequiresTranslationService": false}),
			expectError: false,
		},
		{
			name:        "check with JSON format (uppercase)",
			flagValues:  map[string]any{"check": true, "format": "JSON"},
			expected:    newTestFlags(map[string]any{"Check": true, "Format": "json", "RequiresTranslationService": false}),
			expectError: false,
		},
		{
			name:        "check with Default format (mixed case)",
			flagValues:  map[string]any{"check": true, "format": "Default"},
			expected:    newTestFlags(map[string]any{"Check": true, "Format": "default", "RequiresTranslationService": false}),
			expectError: false,
		},
		{
			name:        "check with format having leading/trailing spaces",
			flagValues:  map[string]any{"check": true, "format": " JSON "},
			expected:    newTestFlags(map[string]any{"Check": true, "Format": "json", "RequiresTranslationService": false}),
			expectError: false,
		},
		{
			name:        "check with format having mixed case and spaces",
			flagValues:  map[string]any{"check": true, "format": "  Default  "},
			expected:    newTestFlags(map[string]any{"Check": true, "Format": "default", "RequiresTranslationService": false}),
			expectError: false,
		},
	}
}

// getConflictTestCases returns test cases for conflicting flag combinations.
func getConflictTestCases() []fromCobraCommandTestCase {
	return []fromCobraCommandTestCase{
		{
			name:        "stub and sourced conflict",
			flagValues:  map[string]any{"stub": true, "sourced": true},
			expected:    nil,
			expectError: true,
		},
		{
			name:        "stub and include-stubs conflict",
			flagValues:  map[string]any{"stub": true, "include-stubs": true},
			expected:    nil,
			expectError: true,
		},
		{
			name:        "format without check conflict",
			flagValues:  map[string]any{"format": "json"},
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid format value",
			flagValues:  map[string]any{"check": true, "format": "YAML"},
			expected:    nil,
			expectError: true,
		},
		{
			name:        "context with check conflict",
			flagValues:  map[string]any{"check": true, "context": "Dashboard UI"},
			expected:    nil,
			expectError: true,
		},
	}
}

func TestParseAndValidateFromCobraCommand(t *testing.T) {
	for _, tt := range getFromCobraCommandTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			runFromCobraCommandTest(t, tt.flagValues, tt.expected, tt.expectError)
		})
	}
}

// runFromCobraCommandTest executes a single test case for ParseFromCobraCommand + ValidateFlagCombinations.
func runFromCobraCommandTest(t *testing.T, flagValues map[string]any, expected *flags.Flags, expectError bool) {
	cmd := createTestCommand()
	setTestFlags(cmd, flagValues)

	result, err := flags.ParseFromCobraCommand(cmd)
	if err != nil {
		if expectError {
			return // Parsing error is acceptable for error test cases
		}
		t.Errorf("Unexpected parsing error: %v", err)
		return
	}

	// Now validate the parsed flags (pass nil config for basic testing)
	err = flags.ValidateFlagCombinations(result, nil)

	if expectError {
		if err == nil {
			t.Errorf("Expected error but got none")
		}
		return
	}

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	assertFlagsEqual(t, expected, result)
}

// createTestCommand creates a cobra command with all the required flags for testing.
func createTestCommand() *cobra.Command {
	//nolint:exhaustruct // Test helper - only need flags, not all cobra fields
	cmd := &cobra.Command{}
	flags.AddToCommand(cmd)
	return cmd
}

// setTestFlags sets the flag values on the command for testing.
func setTestFlags(cmd *cobra.Command, flagValues map[string]any) {
	for flagName, value := range flagValues {
		switch val := value.(type) {
		case bool:
			if err := cmd.Flags().Set(flagName, strconv.FormatBool(val)); err != nil {
				panic(fmt.Sprintf("failed to set flag %s: %v", flagName, err))
			}
		case string:
			if err := cmd.Flags().Set(flagName, val); err != nil {
				panic(fmt.Sprintf("failed to set flag %s: %v", flagName, err))
			}
		case []string:
			for _, s := range val {
				if err := cmd.Flags().Set(flagName, s); err != nil {
					panic(fmt.Sprintf("failed to set flag %s: %v", flagName, err))
				}
			}
		}
	}
}

// assertFlagsEqual compares two Flags structs and reports any differences.
func assertFlagsEqual(t *testing.T, expected, actual *flags.Flags) {
	if actual.TrimStrings != expected.TrimStrings {
		t.Errorf("TrimStrings: expected %v, got %v", expected.TrimStrings, actual.TrimStrings)
	}
	if actual.StubMode != expected.StubMode {
		t.Errorf("StubMode: expected %v, got %v", expected.StubMode, actual.StubMode)
	}
	if actual.IncludeStubs != expected.IncludeStubs {
		t.Errorf("IncludeStubs: expected %v, got %v", expected.IncludeStubs, actual.IncludeStubs)
	}
	if actual.SourcedOnly != expected.SourcedOnly {
		t.Errorf("SourcedOnly: expected %v, got %v", expected.SourcedOnly, actual.SourcedOnly)
	}
	if actual.Rephrase != expected.Rephrase {
		t.Errorf("Rephrase: expected %v, got %v", expected.Rephrase, actual.Rephrase)
	}
	if actual.Check != expected.Check {
		t.Errorf("Check: expected %v, got %v", expected.Check, actual.Check)
	}
	if actual.Format != expected.Format {
		t.Errorf("Format: expected %q, got %q", expected.Format, actual.Format)
	}
	if actual.Context != expected.Context {
		t.Errorf("Context: expected %q, got %q", expected.Context, actual.Context)
	}
	if actual.Locale != expected.Locale {
		t.Errorf("Locale: expected %q, got %q", expected.Locale, actual.Locale)
	}
	if actual.Directory != expected.Directory {
		t.Errorf("Directory: expected %q, got %q", expected.Directory, actual.Directory)
	}
	if actual.RequiresTranslationService != expected.RequiresTranslationService {
		t.Errorf("RequiresTranslationService: expected %v, got %v",
			expected.RequiresTranslationService, actual.RequiresTranslationService)
	}

	assertKeysEqual(t, expected.Keys, actual.Keys)
}

// assertKeysEqual compares two string slices and reports any differences.
func assertKeysEqual(t *testing.T, expected, actual []string) {
	if len(actual) != len(expected) {
		t.Errorf("Keys length: expected %d, got %d", len(expected), len(actual))
		return
	}

	for i, key := range actual {
		if key != expected[i] {
			t.Errorf("Keys[%d]: expected %s, got %s", i, expected[i], key)
		}
	}
}

// getUpdateDerivedFlagsTestCases returns test cases for UpdateDerivedFlags.
func getUpdateDerivedFlagsTestCases() []updateDerivedFlagsTestCase {
	return []updateDerivedFlagsTestCase{
		{
			name:                               "normal mode needs translation service",
			stubMode:                           false,
			sourcedOnly:                        false,
			check:                              false,
			expectedRequiresTranslationService: true,
		},
		{
			name:                               "stub mode doesn't need translation service",
			stubMode:                           true,
			sourcedOnly:                        false,
			check:                              false,
			expectedRequiresTranslationService: false,
		},
		{
			name:                               "sourced only doesn't need translation service",
			stubMode:                           false,
			sourcedOnly:                        true,
			check:                              false,
			expectedRequiresTranslationService: false,
		},
		{
			name:                               "check mode doesn't need translation service",
			stubMode:                           false,
			sourcedOnly:                        false,
			check:                              true,
			expectedRequiresTranslationService: false,
		},
		{
			name:                               "both stub and sourced doesn't need translation service",
			stubMode:                           true,
			sourcedOnly:                        true,
			check:                              false,
			expectedRequiresTranslationService: false,
		},
	}
}

func TestUpdateDerivedFlags(t *testing.T) {
	for _, tt := range getUpdateDerivedFlagsTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			//nolint:exhaustruct // Test case only sets relevant fields
			flagSet := &flags.Flags{
				TrimStrings:                true,
				StubMode:                   tt.stubMode,
				IncludeStubs:               false,
				SourcedOnly:                tt.sourcedOnly,
				Rephrase:                   false,
				Check:                      tt.check,
				Format:                     "",
				Keys:                       nil,
				Locale:                     "",
				Context:                    "",
				Directory:                  "",
				Locales:                    nil,
				Indent:                     -1,
				Formality:                  "",
				ProjectName:                "",
				ProjectDesc:                "",
				ProjectDom:                 "",
				ProviderActive:             "",
				ProviderModel:              "",
				ProviderTemperature:        -1,
				ProviderMaxTokens:          -1,
				RequiresTranslationService: true,
			}

			flagSet.UpdateDerivedFlags()

			if flagSet.RequiresTranslationService != tt.expectedRequiresTranslationService {
				t.Errorf("RequiresTranslationService: expected %v, got %v",
					tt.expectedRequiresTranslationService, flagSet.RequiresTranslationService)
			}
		})
	}
}

func TestUpdateDerivedFlags_WithCheck(t *testing.T) {
	//nolint:exhaustruct // Test case only sets relevant fields
	flagSet := &flags.Flags{
		TrimStrings:                true,
		StubMode:                   false,
		IncludeStubs:               false,
		SourcedOnly:                false,
		Rephrase:                   false,
		Check:                      true,
		Format:                     "",
		Keys:                       nil,
		Locale:                     "",
		Context:                    "",
		Directory:                  "",
		Locales:                    nil,
		Indent:                     -1,
		Formality:                  "",
		ProjectName:                "",
		ProjectDesc:                "",
		ProjectDom:                 "",
		ProviderActive:             "",
		ProviderModel:              "",
		ProviderTemperature:        -1,
		ProviderMaxTokens:          -1,
		RequiresTranslationService: true, // will be recomputed
	}
	flagSet.UpdateDerivedFlags()
	if flagSet.RequiresTranslationService {
		t.Errorf("RequiresTranslationService: expected false when Check=true, got %v", flagSet.RequiresTranslationService)
	}
}

// getFlagsValidateTestCases returns test cases for Flags.Validate().
func getFlagsValidateTestCases() []flagsValidateTestCase {
	var testCases []flagsValidateTestCase
	testCases = append(testCases, getValidFlagsTestCases()...)
	testCases = append(testCases, getConflictFlagsTestCases()...)
	return testCases
}

// getValidFlagsTestCases returns valid flag combination test cases.
func getValidFlagsTestCases() []flagsValidateTestCase {
	return []flagsValidateTestCase{
		{
			name:        "valid default flags",
			flags:       newTestFlags(nil),
			expectError: false,
		},
		{
			name: "sourced and include-stubs valid",
			flags: newTestFlags(map[string]any{
				"IncludeStubs": true, "SourcedOnly": true, "RequiresTranslationService": false,
			}),
			expectError: false,
		},
		{
			name:        "check only valid",
			flags:       newTestFlags(map[string]any{"Check": true, "RequiresTranslationService": false}),
			expectError: false,
		},
		{
			name: "check with keys valid",
			flags: newTestFlags(map[string]any{
				"Check": true, "Keys": []string{"auth.login"}, "RequiresTranslationService": false,
			}),
			expectError: false,
		},
		{
			name:        "check with include-stubs valid",
			flags:       newTestFlags(map[string]any{"Check": true, "IncludeStubs": true, "RequiresTranslationService": false}),
			expectError: false,
		},
		{
			name:        "check with default format valid",
			flags:       newTestFlags(map[string]any{"Check": true, "Format": "default", "RequiresTranslationService": false}),
			expectError: false,
		},
		{
			name:        "check with json format valid",
			flags:       newTestFlags(map[string]any{"Check": true, "Format": "json", "RequiresTranslationService": false}),
			expectError: false,
		},
	}
}

// getConflictFlagsTestCases returns conflicting flag combination test cases.
func getConflictFlagsTestCases() []flagsValidateTestCase {
	return []flagsValidateTestCase{
		{
			name: "stub and sourced conflict",
			flags: newTestFlags(map[string]any{
				"StubMode": true, "SourcedOnly": true, "RequiresTranslationService": false,
			}),
			expectError: true,
		},
		{
			name: "stub and include-stubs conflict",
			flags: newTestFlags(map[string]any{
				"StubMode": true, "IncludeStubs": true, "RequiresTranslationService": false,
			}),
			expectError: true,
		},
		{
			name:        "format without check conflict",
			flags:       newTestFlags(map[string]any{"Format": "json"}),
			expectError: true,
		},
		{
			name: "check with rephrase conflict",
			flags: newTestFlags(map[string]any{
				"Check": true, "Rephrase": true, "Keys": []string{"auth.login"},
				"Locale": "en", "RequiresTranslationService": false,
			}),
			expectError: true,
		},
		{
			name:        "check with stub mode conflict",
			flags:       newTestFlags(map[string]any{"Check": true, "StubMode": true, "RequiresTranslationService": false}),
			expectError: true,
		},
		{
			name:        "check with sourced only conflict",
			flags:       newTestFlags(map[string]any{"Check": true, "SourcedOnly": true, "RequiresTranslationService": false}),
			expectError: true,
		},
		{
			name:        "unsupported format value conflict",
			flags:       newTestFlags(map[string]any{"Check": true, "Format": "yaml", "RequiresTranslationService": false}),
			expectError: true,
		},
	}
}

func TestFlags_Validate(t *testing.T) {
	for _, tt := range getFlagsValidateTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			err := flags.ValidateFlagCombinations(tt.flags, nil)

			if tt.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

func TestNew(t *testing.T) {
	flagSet := flags.New()

	// Check default values
	if !flagSet.TrimStrings {
		t.Error("Expected TrimStrings to default to true")
	}
	if flagSet.StubMode {
		t.Error("Expected StubMode to default to false")
	}
	if flagSet.IncludeStubs {
		t.Error("Expected IncludeStubs to default to false")
	}
	if flagSet.SourcedOnly {
		t.Error("Expected SourcedOnly to default to false")
	}
	if flagSet.Keys != nil {
		t.Error("Expected Keys to default to nil")
	}
	if flagSet.Context != "" {
		t.Error("Expected Context to default to empty string")
	}
	if flagSet.Check {
		t.Error("Expected Check to default to false")
	}
	if flagSet.Format != "" {
		t.Error("Expected Format to default to empty string")
	}
	if !flagSet.RequiresTranslationService {
		t.Error("Expected RequiresTranslationService to default to true")
	}
	if flagSet.Directory != "" {
		t.Error("Expected Directory to default to empty string")
	}
	if flagSet.Indent != -1 {
		t.Error("Expected Indent to default to -1")
	}
}

func TestAddToCommand(t *testing.T) {
	//nolint:exhaustruct // Test helper - only need flags, not all cobra fields
	cmd := &cobra.Command{}
	flags.AddToCommand(cmd)

	// Test that all expected flags are present
	expectedFlags := []string{
		"keys", "stub", "include-stubs", "sourced", "no-trim",
		"context", "check", "format", "directory",
	}
	for _, flagName := range expectedFlags {
		if cmd.Flags().Lookup(flagName) == nil {
			t.Errorf("Expected flag %s to be present", flagName)
		}
	}

	// Test directory flag default value
	if got := cmd.Flags().Lookup("directory").DefValue; got != "" {
		t.Errorf("Expected directory default to be empty, got %q", got)
	}

	// Test persistent flags
	expectedPersistentFlags := []string{"debug"}
	for _, flagName := range expectedPersistentFlags {
		if cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("Expected persistent flag %s to be present", flagName)
		}
	}
}

func TestAddInitFlags(t *testing.T) {
	//nolint:exhaustruct // Test helper - only need flags, not all cobra fields
	cmd := &cobra.Command{}
	flags.AddInitFlags(cmd)

	// Test that the force flag is present
	if cmd.Flags().Lookup("force") == nil {
		t.Error("Expected force flag to be present")
	}

	// Test that the shorthand is set
	if cmd.Flags().ShorthandLookup("f") == nil {
		t.Error("Expected force flag shorthand 'f' to be present")
	}
}

func TestGetForceFlag(t *testing.T) {
	tests := []struct {
		name      string
		forceFlag bool
		expectErr bool
	}{
		{"force flag true", true, false},
		{"force flag false", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//nolint:exhaustruct // Test helper - only need flags, not all cobra fields
			cmd := &cobra.Command{}
			flags.AddInitFlags(cmd)

			// Set the flag value
			err := cmd.Flags().Set("force", "true")
			if tt.forceFlag && err != nil {
				t.Fatalf("Failed to set force flag: %v", err)
			}
			if !tt.forceFlag {
				err = cmd.Flags().Set("force", "false")
				if err != nil {
					t.Fatalf("Failed to set force flag: %v", err)
				}
			}

			force, err := flags.GetForceFlag(cmd)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if force != tt.forceFlag {
					t.Errorf("Expected force flag to be %t, got %t", tt.forceFlag, force)
				}
			}
		})
	}
}

func TestHandleLoggingFlags(t *testing.T) {
	tests := []struct {
		name        string
		verboseFlag bool
		debugFlag   bool
	}{
		{"both disabled", false, false},
		{"verbose enabled", true, false},
		{"debug enabled", false, true},
		{"both enabled", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//nolint:exhaustruct // Test helper - only need flags, not all cobra fields
			cmd := &cobra.Command{}
			cmd.PersistentFlags().Bool("verbose", tt.verboseFlag, "show detailed progress")
			cmd.PersistentFlags().Bool("debug", tt.debugFlag, "enable debug logging")

			// This function doesn't return errors, so we just test it doesn't panic
			flags.HandleLoggingFlags(cmd)
		})
	}
}

func TestIsLoggingDerivedFlag(t *testing.T) {
	tests := []struct {
		name              string
		verboseFlag       bool
		debugFlag         bool
		expectedIsLogging bool
	}{
		{"both disabled", false, false, false},
		{"verbose only", true, false, true},
		{"debug only", false, true, true},
		{"both enabled", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := createTestCommand()

			// Set persistent flags for verbose and debug
			if err := cmd.PersistentFlags().Set("verbose", strconv.FormatBool(tt.verboseFlag)); err != nil {
				t.Fatalf("set verbose: %v", err)
			}
			if err := cmd.PersistentFlags().Set("debug", strconv.FormatBool(tt.debugFlag)); err != nil {
				t.Fatalf("set debug: %v", err)
			}

			// Parse flags to Flags struct
			parsed, err := flags.ParseFromCobraCommand(cmd)
			if err != nil {
				t.Fatalf("ParseFromCobraCommand failed: %v", err)
			}

			// Verify IsLogging is correctly computed
			if parsed.IsLogging != tt.expectedIsLogging {
				t.Errorf("IsLogging: expected %v, got %v", tt.expectedIsLogging, parsed.IsLogging)
			}
		})
	}
}

func TestGetDebugFlag(t *testing.T) {
	for _, tc := range []bool{true, false} {
		t.Run(fmt.Sprintf("debug=%t", tc), func(t *testing.T) {
			cmd := createTestCommand()

			if err := cmd.PersistentFlags().Set("debug", strconv.FormatBool(tc)); err != nil {
				t.Fatalf("set debug: %v", err)
			}
			got, err := flags.GetDebugFlag(cmd)
			if err != nil {
				t.Fatalf("GetDebugFlag error: %v", err)
			}
			if got != tc {
				t.Fatalf("want %t, got %t", tc, got)
			}
		})
	}
}

func TestGetSourcedFlag(t *testing.T) {
	tests := []struct {
		name        string
		sourcedFlag bool
		expectErr   bool
	}{
		{"sourced flag true", true, false},
		{"sourced flag false", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//nolint:exhaustruct // Test helper - only need flags, not all cobra fields
			cmd := &cobra.Command{}
			flags.AddToCommand(cmd)

			// Set the flag value
			err := cmd.Flags().Set("sourced", strconv.FormatBool(tt.sourcedFlag))
			if err != nil {
				t.Fatalf("Failed to set sourced flag: %v", err)
			}

			sourced, err := flags.GetSourcedFlag(cmd)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if sourced != tt.sourcedFlag {
					t.Errorf("Expected sourced flag to be %t, got %t", tt.sourcedFlag, sourced)
				}
			}
		})
	}
}
