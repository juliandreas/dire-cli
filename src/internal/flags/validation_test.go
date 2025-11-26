package flags_test

import (
	"strings"
	"testing"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/flags"
)

// flagCombinationTestCase represents a test case for flag validation.
type flagCombinationTestCase struct {
	name         string
	stub         bool
	includeStubs bool
	sourced      bool
	glossary     bool
	memory       bool
	expectError  bool
}

// translatorCompatibilityTestCase represents a test case for translator compatibility validation.
type translatorCompatibilityTestCase struct {
	name        string
	provider    string
	context     string
	rephrase    bool
	keys        []string
	locale      string
	expectError bool
	description string
}

// noTrimTestCase represents a test case for no-trim flag compatibility testing.
type noTrimTestCase struct {
	name         string
	stub         bool
	includeStubs bool
	sourced      bool
	description  string
}

// rephraseTestCase represents a test case for rephrase flag validation.
type rephraseTestCase struct {
	name         string
	rephrase     bool
	keys         []string // Keys for both normal translations and rephrases (single key for rephrase)
	locale       string   // Single locale for rephrases
	stub         bool
	sourced      bool
	includeStubs bool
	expectError  bool
	description  string
}

// contextValidationTestCase represents a test case for context flag validation.
type contextValidationTestCase struct {
	name         string
	context      string
	stubMode     bool
	sourcedOnly  bool
	glossaryOnly bool
	memoryOnly   bool
	expectError  bool
}

// checkValidationTestCase represents a test case for check mode validation.
type checkValidationTestCase struct {
	name         string
	check        bool
	stubMode     bool
	sourcedOnly  bool
	glossaryOnly bool
	memoryOnly   bool
	rephrase     bool
	expectError  bool
	description  string
}

// formatValidationTestCase represents a test case for format flag validation.
type formatValidationTestCase struct {
	name        string
	check       bool
	format      string
	expectError bool
	description string
}

// overwriteValidationTestCase represents a test case for overwrite flag validation.
type overwriteValidationTestCase struct {
	name         string
	overwrite    bool
	glossaryOnly bool
	sourcedOnly  bool
	memoryOnly   bool
	stubMode     bool
	prune        bool
	check        bool
	rephrase     bool
	keys         []string
	locale       string
	expectError  bool
	description  string
}

// getFlagCombinationTestCases returns test cases for ValidateFlagCombinations.
//
//nolint:funlen // Table-driven test data is clearer kept together
func getFlagCombinationTestCases() []flagCombinationTestCase {
	return []flagCombinationTestCase{
		{
			name:         "no flags",
			stub:         false,
			includeStubs: false,
			sourced:      false,
			glossary:     false,
			memory:       false,
			expectError:  false,
		},
		{
			name:         "stub and sourced conflict",
			stub:         true,
			includeStubs: false,
			sourced:      true,
			glossary:     false,
			memory:       false,
			expectError:  true,
		},
		{
			name:         "stub and include-stubs conflict",
			stub:         true,
			includeStubs: true,
			sourced:      false,
			glossary:     false,
			memory:       false,
			expectError:  true,
		},
		{
			name:         "sourced and include-stubs allowed",
			stub:         false,
			includeStubs: true,
			sourced:      true,
			glossary:     false,
			memory:       false,
			expectError:  false,
		},
		{
			name:         "stub only allowed",
			stub:         true,
			includeStubs: false,
			sourced:      false,
			glossary:     false,
			memory:       false,
			expectError:  false,
		},
		{
			name:         "sourced only allowed",
			stub:         false,
			includeStubs: false,
			sourced:      true,
			glossary:     false,
			memory:       false,
			expectError:  false,
		},
		{
			name:         "include-stubs only allowed",
			stub:         false,
			includeStubs: true,
			sourced:      false,
			glossary:     false,
			memory:       false,
			expectError:  false,
		},
		// New tests for glossary and memory flags
		{
			name:         "glossary only allowed",
			stub:         false,
			includeStubs: false,
			sourced:      false,
			glossary:     true,
			memory:       false,
			expectError:  false,
		},
		{
			name:         "memory only allowed",
			stub:         false,
			includeStubs: false,
			sourced:      false,
			glossary:     false,
			memory:       true,
			expectError:  false,
		},
		{
			name:         "glossary and memory conflict",
			stub:         false,
			includeStubs: false,
			sourced:      false,
			glossary:     true,
			memory:       true,
			expectError:  true,
		},
		{
			name:         "sourced and glossary conflict",
			stub:         false,
			includeStubs: false,
			sourced:      true,
			glossary:     true,
			memory:       false,
			expectError:  true,
		},
		{
			name:         "sourced and memory conflict",
			stub:         false,
			includeStubs: false,
			sourced:      true,
			glossary:     false,
			memory:       true,
			expectError:  true,
		},
		{
			name:         "sourced with both glossary and memory conflict",
			stub:         false,
			includeStubs: false,
			sourced:      true,
			glossary:     true,
			memory:       true,
			expectError:  true,
		},
		{
			name:         "stub and glossary conflict",
			stub:         true,
			includeStubs: false,
			sourced:      false,
			glossary:     true,
			memory:       false,
			expectError:  true,
		},
		{
			name:         "stub and memory conflict",
			stub:         true,
			includeStubs: false,
			sourced:      false,
			glossary:     false,
			memory:       true,
			expectError:  true,
		},
	}
}

func TestValidateFlagCombinations(t *testing.T) {
	t.Parallel()
	for _, tt := range getFlagCombinationTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testFlags := &flags.Flags{ //nolint:exhaustruct // Test case only sets relevant fields
				TrimStrings:  true,
				StubMode:     tt.stub,
				IncludeStubs: tt.includeStubs,
				SourcedOnly:  tt.sourced,
				GlossaryOnly: tt.glossary,
				MemoryOnly:   tt.memory,
				Rephrase:     false,
				Check:        false,
				Format:       "",
				Keys:         nil,
				Locale:       "",
				Context:      "",
				Directory:    "",
			}
			testFlags.UpdateDerivedFlags()
			err := flags.ValidateFlagCombinations(testFlags, nil)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for combination: stub=%v, includeStubs=%v, sourced=%v, glossary=%v, memory=%v",
					tt.stub, tt.includeStubs, tt.sourced, tt.glossary, tt.memory)
			}
			if !tt.expectError && err != nil {
				t.Errorf(
					"Unexpected error for combination: stub=%v, includeStubs=%v, sourced=%v, glossary=%v, memory=%v, error: %v",
					tt.stub, tt.includeStubs, tt.sourced, tt.glossary, tt.memory, err)
			}
		})
	}
}

// TestNoTrimFlagCompatibility verifies that --no-trim works with all valid flag combinations.
func TestNoTrimFlagCompatibility(t *testing.T) {
	t.Parallel()
	// Note: --no-trim flag itself doesn't need validation since it doesn't conflict with any other flags
	// This test documents that --no-trim can be used with any valid combination
	validCombinations := getNoTrimTestCombinations()

	for _, combo := range validCombinations {
		t.Run(combo.name, func(t *testing.T) {
			t.Parallel()
			// Verify the base combination is valid with trimming enabled
			testFlags := &flags.Flags{ //nolint:exhaustruct // Test case only sets relevant fields
				TrimStrings:  true,
				StubMode:     combo.stub,
				IncludeStubs: combo.includeStubs,
				SourcedOnly:  combo.sourced,
				Rephrase:     false,
				Check:        false,
				Format:       "",
				Keys:         nil,
				Locale:       "",
				Context:      "",
				Directory:    "",
			}
			testFlags.UpdateDerivedFlags()
			err := flags.ValidateFlagCombinations(testFlags, nil)
			if err != nil {
				t.Errorf("Combination should be valid: %s, error: %v", combo.description, err)
			}

			// Verify the same combination works with no-trim (TrimStrings: false)
			testFlags.TrimStrings = false
			testFlags.UpdateDerivedFlags()
			err = flags.ValidateFlagCombinations(testFlags, nil)
			if err != nil {
				t.Errorf("Combination with no-trim should be valid: %s, error: %v", combo.description, err)
			}
		})
	}
}

// getNoTrimTestCombinations returns test combinations for --no-trim compatibility testing.
func getNoTrimTestCombinations() []noTrimTestCase {
	return []noTrimTestCase{
		{
			name:         "no-trim with default behavior",
			stub:         false,
			includeStubs: false,
			sourced:      false,
			description:  "--no-trim with standard translation",
		},
		{
			name:         "no-trim with sourced",
			stub:         false,
			includeStubs: false,
			sourced:      true,
			description:  "--no-trim with glossary/memory only",
		},
		{
			name:         "no-trim with include-stubs",
			stub:         false,
			includeStubs: true,
			sourced:      false,
			description:  "--no-trim including empty strings for translation",
		},
		{
			name:         "no-trim with sourced and include-stubs",
			stub:         false,
			includeStubs: true,
			sourced:      true,
			description:  "--no-trim with glossary/memory including empty strings",
		},
		{
			name:         "no-trim with stub",
			stub:         true,
			includeStubs: false,
			sourced:      false,
			description:  "--no-trim when creating stub translations",
		},
	}
}

// getRephraseTestCases returns test cases for rephrase-specific validation.
func getRephraseTestCases() []rephraseTestCase {
	var testCases []rephraseTestCase

	// Valid rephrase configurations
	testCases = append(testCases, getValidRephraseTestCases()...)

	// Invalid rephrase configurations - missing requirements
	testCases = append(testCases, getInvalidRephraseRequirementTestCases()...)

	// Note: Rephrase conflicts with other translation/modification modes (suggestions are standalone)

	// Miscellaneous rephrase test cases
	testCases = append(testCases, getMiscRephraseTestCases()...)

	return testCases
}

// getValidRephraseTestCases returns test cases for valid rephrase configurations.
func getValidRephraseTestCases() []rephraseTestCase {
	return []rephraseTestCase{
		{
			name:         "rephrase with singular key and locale - valid",
			rephrase:     true,
			keys:         []string{"auth.login"},
			locale:       "sv",
			stub:         false,
			sourced:      false,
			includeStubs: false,
			expectError:  false,
			description:  "Valid rephrase configuration with singular key and locale flags",
		},
		{
			name:         "rephrase with stub mode - invalid",
			rephrase:     true,
			keys:         []string{"auth.login"},
			locale:       "sv",
			stub:         true,
			sourced:      false,
			includeStubs: false,
			expectError:  true,
			description:  "Rephrase cannot be combined with stub mode",
		},
		{
			name:         "rephrase with sourced mode - invalid",
			rephrase:     true,
			keys:         []string{"auth.login"},
			locale:       "sv",
			stub:         false,
			sourced:      true,
			includeStubs: false,
			expectError:  true,
			description:  "Rephrase cannot be combined with sourced mode",
		},
		{
			name:         "rephrase with include-stubs mode - valid",
			rephrase:     true,
			keys:         []string{"auth.login"},
			locale:       "sv",
			stub:         false,
			sourced:      false,
			includeStubs: true,
			expectError:  false,
			description:  "Rephrase suggestions work with include-stubs mode",
		},
	}
}

// getInvalidRephraseRequirementTestCases returns test cases for invalid rephrase configs missing requirements.
func getInvalidRephraseRequirementTestCases() []rephraseTestCase {
	return []rephraseTestCase{
		{
			name:         "rephrase without key - invalid",
			rephrase:     true,
			keys:         nil,
			locale:       "sv",
			stub:         false,
			sourced:      false,
			includeStubs: false,
			expectError:  true,
			description:  "Rephrase requires exactly one key to be specified",
		},
		{
			name:         "rephrase with multiple keys - invalid",
			rephrase:     true,
			keys:         []string{"auth.login", "auth.logout"},
			locale:       "sv",
			stub:         false,
			sourced:      false,
			includeStubs: false,
			expectError:  true,
			description:  "Rephrase accepts only one key, not multiple",
		},
		{
			name:         "rephrase without locale - invalid",
			rephrase:     true,
			keys:         []string{"auth.login"},
			locale:       "",
			stub:         false,
			sourced:      false,
			includeStubs: false,
			expectError:  true,
			description:  "Rephrase requires locale to be specified",
		},
		{
			name:         "rephrase without key and locale - invalid",
			rephrase:     true,
			keys:         nil,
			locale:       "",
			stub:         false,
			sourced:      false,
			includeStubs: false,
			expectError:  true,
			description:  "Rephrase requires both key and locale to be specified",
		},
	}
}

// getMiscRephraseTestCases returns miscellaneous rephrase test cases.
func getMiscRephraseTestCases() []rephraseTestCase {
	return []rephraseTestCase{
		{
			name:         "rephrase disabled - no requirements",
			rephrase:     false,
			keys:         nil,
			locale:       "",
			stub:         false,
			sourced:      false,
			includeStubs: false,
			expectError:  false,
			description:  "When rephrase is disabled, no keys/locales required",
		},
		{
			name:         "locale without rephrase - invalid",
			rephrase:     false,
			keys:         nil,
			locale:       "sv",
			stub:         false,
			sourced:      false,
			includeStubs: false,
			expectError:  true,
			description:  "Locale flag can only be used with rephrase",
		},
	}
}

// TestRephraseValidation tests rephrase-specific flag validation.
func TestRephraseValidation(t *testing.T) {
	t.Parallel()
	for _, tt := range getRephraseTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testFlags := &flags.Flags{ //nolint:exhaustruct // Test case only sets relevant fields
				TrimStrings:  true,
				StubMode:     tt.stub,
				IncludeStubs: tt.includeStubs,
				SourcedOnly:  tt.sourced,
				Rephrase:     tt.rephrase,
				Check:        false,
				Format:       "",
				Keys:         tt.keys,
				Locale:       tt.locale,
				Context:      "",
				Directory:    "",
			}
			testFlags.UpdateDerivedFlags()

			err := flags.ValidateFlagCombinations(testFlags, nil)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for case: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for case: %s, error: %v", tt.description, err)
			}
		})
	}
}

// TestContextValidation tests context flag validation rules.
func TestContextValidation(t *testing.T) {
	t.Parallel()
	tests := getContextValidationTestCases()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runContextValidationTest(t, tt)
		})
	}
}

// getContextValidationTestCases returns test cases for context validation.
//
//nolint:funlen // Table-driven test data is clearer kept together
func getContextValidationTestCases() []contextValidationTestCase {
	return []contextValidationTestCase{
		{
			name:    "empty context is valid",
			context: "", stubMode: false, sourcedOnly: false, glossaryOnly: false, memoryOnly: false, expectError: false,
		},
		{
			name:    "short context in normal mode is valid",
			context: "Dashboard UI", stubMode: false, sourcedOnly: false, glossaryOnly: false, memoryOnly: false,
			expectError: false,
		},
		{
			name:     "context just under max length is valid",
			context:  strings.Repeat("a", flags.MaxContextLength-1),
			stubMode: false, sourcedOnly: false, glossaryOnly: false, memoryOnly: false, expectError: false,
		},
		{
			name:     "context at max length is valid",
			context:  strings.Repeat("a", flags.MaxContextLength),
			stubMode: false, sourcedOnly: false, glossaryOnly: false, memoryOnly: false, expectError: false,
		},
		{
			name:     "context over max length is invalid",
			context:  strings.Repeat("a", flags.MaxContextLength+1),
			stubMode: false, sourcedOnly: false, glossaryOnly: false, memoryOnly: false, expectError: true,
		},
		{
			name:     "very long context is invalid",
			context:  strings.Repeat("a", flags.MaxContextLength*2),
			stubMode: false, sourcedOnly: false, glossaryOnly: false, memoryOnly: false, expectError: true,
		},
		{
			name:    "context with stub mode is invalid",
			context: "Dashboard UI", stubMode: true, sourcedOnly: false, glossaryOnly: false, memoryOnly: false,
			expectError: true,
		},
		{
			name:    "context with sourced mode is invalid",
			context: "Dashboard UI", stubMode: false, sourcedOnly: true, glossaryOnly: false, memoryOnly: false,
			expectError: true,
		},
		{
			name:    "empty context with stub mode is valid",
			context: "", stubMode: true, sourcedOnly: false, glossaryOnly: false, memoryOnly: false, expectError: false,
		},
		{
			name:    "empty context with sourced mode is valid",
			context: "", stubMode: false, sourcedOnly: true, glossaryOnly: false, memoryOnly: false, expectError: false,
		},
		// New tests for glossary and memory flags
		{
			name:    "context with glossary mode is invalid",
			context: "Dashboard UI", stubMode: false, sourcedOnly: false, glossaryOnly: true, memoryOnly: false,
			expectError: true,
		},
		{
			name:    "context with memory mode is invalid",
			context: "Dashboard UI", stubMode: false, sourcedOnly: false, glossaryOnly: false, memoryOnly: true,
			expectError: true,
		},
		{
			name:    "empty context with glossary mode is valid",
			context: "", stubMode: false, sourcedOnly: false, glossaryOnly: true, memoryOnly: false, expectError: false,
		},
		{
			name:    "empty context with memory mode is valid",
			context: "", stubMode: false, sourcedOnly: false, glossaryOnly: false, memoryOnly: true, expectError: false,
		},
	}
}

// runContextValidationTest executes a single context validation test case.
func runContextValidationTest(t *testing.T, tt contextValidationTestCase) {
	t.Helper()
	testFlags := &flags.Flags{ //nolint:exhaustruct // Test case only sets relevant fields
		TrimStrings:  true,
		StubMode:     tt.stubMode,
		IncludeStubs: false,
		SourcedOnly:  tt.sourcedOnly,
		GlossaryOnly: tt.glossaryOnly,
		MemoryOnly:   tt.memoryOnly,
		Rephrase:     false,
		Check:        false,
		Format:       "",
		Keys:         nil,
		Locale:       "",
		Context:      tt.context,
		Directory:    "",
	}
	testFlags.UpdateDerivedFlags()

	err := flags.ValidateFlagCombinations(testFlags, nil)

	if tt.expectError && err == nil {
		t.Errorf("Expected error for case: %s", tt.name)
	}
	if !tt.expectError && err != nil {
		t.Errorf("Unexpected error for case: %s, error: %v", tt.name, err)
	}
}

// getCheckValidationTestCases returns test cases for check mode validation.
//
//nolint:funlen // Table-driven test data is clearer kept together
func getCheckValidationTestCases() []checkValidationTestCase {
	return []checkValidationTestCase{
		{
			name:         "check mode alone is valid",
			check:        true,
			stubMode:     false,
			sourcedOnly:  false,
			glossaryOnly: false,
			memoryOnly:   false,
			rephrase:     false,
			expectError:  false,
			description:  "Check mode by itself should be valid",
		},
		{
			name:         "check with stub mode is invalid",
			check:        true,
			stubMode:     true,
			sourcedOnly:  false,
			glossaryOnly: false,
			memoryOnly:   false,
			rephrase:     false,
			expectError:  true,
			description:  "Check mode conflicts with stub mode",
		},
		{
			name:         "check with sourced mode is invalid",
			check:        true,
			stubMode:     false,
			sourcedOnly:  true,
			glossaryOnly: false,
			memoryOnly:   false,
			rephrase:     false,
			expectError:  true,
			description:  "Check mode conflicts with sourced mode",
		},
		{
			name:         "check with rephrase mode is invalid",
			check:        true,
			stubMode:     false,
			sourcedOnly:  false,
			glossaryOnly: false,
			memoryOnly:   false,
			rephrase:     true,
			expectError:  true,
			description:  "Check mode conflicts with rephrase mode",
		},
		{
			name:         "check disabled with other modes is valid",
			check:        false,
			stubMode:     true,
			sourcedOnly:  false,
			glossaryOnly: false,
			memoryOnly:   false,
			rephrase:     false,
			expectError:  false,
			description:  "When check is disabled, other modes should work",
		},
		// New tests for glossary and memory flags
		{
			name:         "check with glossary mode is invalid",
			check:        true,
			stubMode:     false,
			sourcedOnly:  false,
			glossaryOnly: true,
			memoryOnly:   false,
			rephrase:     false,
			expectError:  true,
			description:  "Check mode conflicts with glossary mode",
		},
		{
			name:         "check with memory mode is invalid",
			check:        true,
			stubMode:     false,
			sourcedOnly:  false,
			glossaryOnly: false,
			memoryOnly:   true,
			rephrase:     false,
			expectError:  true,
			description:  "Check mode conflicts with memory mode",
		},
	}
}

// getFormatValidationTestCases returns test cases for format flag validation.
func getFormatValidationTestCases() []formatValidationTestCase {
	return []formatValidationTestCase{
		{
			name:        "json format with check is valid",
			check:       true,
			format:      "json",
			expectError: false,
			description: "JSON format should work with check mode",
		},
		{
			name:        "default format with check is valid",
			check:       true,
			format:      "default",
			expectError: false,
			description: "Default format should work with check mode",
		},
		{
			name:        "empty format with check is valid",
			check:       true,
			format:      "",
			expectError: false,
			description: "Empty format should work with check mode",
		},
		{
			name:        "format without check is invalid",
			check:       false,
			format:      "json",
			expectError: true,
			description: "Format flag requires check mode",
		},
		{
			name:        "invalid format value is invalid",
			check:       true,
			format:      "xml",
			expectError: true,
			description: "Invalid format values should be rejected",
		},
		{
			name:        "no format and no check is valid",
			check:       false,
			format:      "",
			expectError: false,
			description: "Default flags should be valid",
		},
		{
			name:        "check without format is valid",
			check:       true,
			format:      "",
			expectError: false,
			description: "Check mode should work without format flag",
		},
	}
}

// TestFormatValidation tests format flag validation rules.
func TestFormatValidation(t *testing.T) {
	t.Parallel()
	for _, tt := range getFormatValidationTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testFlags := &flags.Flags{ //nolint:exhaustruct // Test case only sets relevant fields
				TrimStrings:  true,
				StubMode:     false,
				IncludeStubs: false,
				SourcedOnly:  false,
				Rephrase:     false,
				Check:        tt.check,
				Format:       tt.format,
				Keys:         nil,
				Locale:       "",
				Context:      "",
				Directory:    "",
			}
			testFlags.UpdateDerivedFlags()

			err := flags.ValidateFlagCombinations(testFlags, nil)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for case: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for case: %s, error: %v", tt.description, err)
			}
		})
	}
}

// TestCheckValidation tests check mode flag validation rules.
func TestCheckValidation(t *testing.T) {
	t.Parallel()
	for _, tt := range getCheckValidationTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testFlags := &flags.Flags{ //nolint:exhaustruct // Test case only sets relevant fields
				TrimStrings:  true,
				StubMode:     tt.stubMode,
				IncludeStubs: false,
				SourcedOnly:  tt.sourcedOnly,
				GlossaryOnly: tt.glossaryOnly,
				MemoryOnly:   tt.memoryOnly,
				Rephrase:     tt.rephrase,
				Check:        tt.check,
				Format:       "", // Default format for testing
				Keys:         []string{"test.key"},
				Locale:       "en",
				Context:      "",
				Directory:    "",
			}
			testFlags.UpdateDerivedFlags()

			// Handle rephrase requirements - locale already set above
			if !tt.rephrase {
				testFlags.Locale = ""
			}

			err := flags.ValidateFlagCombinations(testFlags, nil)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for case: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for case: %s, error: %v", tt.description, err)
			}
		})
	}
}

func TestDirectoryFlagValidationIndependence(t *testing.T) {
	t.Parallel()
	// Test that Directory flag does not interfere with existing validation rules
	tests := getDirectoryValidationTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testFlags := &flags.Flags{ //nolint:exhaustruct // Test case only sets relevant fields
				TrimStrings:  true,
				StubMode:     tt.stub,
				IncludeStubs: false,
				SourcedOnly:  tt.sourced,
				Rephrase:     false,
				Check:        false,
				Format:       "",
				Keys:         nil,
				Locale:       "",
				Context:      "",
				Directory:    tt.directory,
			}
			testFlags.UpdateDerivedFlags()

			err := flags.ValidateFlagCombinations(testFlags, nil)

			if tt.expectError && err == nil {
				t.Errorf("Expected validation error for %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error for %s: %v", tt.description, err)
			}
		})
	}
}

// getDirectoryValidationTestCases returns test cases for directory flag validation independence.
func getDirectoryValidationTestCases() []struct {
	name        string
	directory   string
	stub        bool
	sourced     bool
	expectError bool
	description string
} {
	return []struct {
		name        string
		directory   string
		stub        bool
		sourced     bool
		expectError bool
		description string
	}{
		{
			name:        "directory with valid flags",
			directory:   "./custom-locales",
			stub:        false,
			sourced:     false,
			expectError: false,
			description: "Directory should not interfere with valid flag combinations",
		},
		{
			name:        "directory with conflicting stub+sourced flags",
			directory:   "/path/to/i18n",
			stub:        true,
			sourced:     true,
			expectError: true,
			description: "Directory should not mask validation errors",
		},
		{
			name:        "empty directory with valid flags",
			directory:   "",
			stub:        false,
			sourced:     false,
			expectError: false,
			description: "Empty Directory should not cause issues",
		},
		{
			name:        "whitespace-only directory with valid flags",
			directory:   "   ",
			stub:        false,
			sourced:     false,
			expectError: false,
			description: "Whitespace-only Directory should be harmless for validation",
		},
	}
}

func TestTranslatorCompatibilityValidation(t *testing.T) {
	t.Parallel()
	testCases := getTranslatorCompatibilityTestCases()

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testFlags := &flags.Flags{ //nolint:exhaustruct // Test case only sets relevant fields
				TrimStrings:  true,
				StubMode:     false,
				IncludeStubs: false,
				SourcedOnly:  false,
				Rephrase:     tt.rephrase,
				Check:        false,
				Format:       "",
				Keys:         tt.keys,
				Locale:       tt.locale,
				Context:      tt.context,
				Directory:    "",
			}

			// Update derived flags
			testFlags.UpdateDerivedFlags()

			// Create config based on test case
			var appConfig *config.Config
			if tt.provider != "" {
				//nolint:exhaustruct // Test helper - only need Active provider field
				appConfig = &config.Config{}
				appConfig.Providers.Active = tt.provider
			}

			err := flags.ValidateFlagCombinations(testFlags, appConfig)

			if tt.expectError && err == nil {
				t.Errorf("Expected validation error for %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error for %s: %v", tt.description, err)
			}
		})
	}
}

// getTranslatorCompatibilityTestCases returns test cases for translator compatibility validation.
//
//nolint:funlen // Test cases are verbose but clear
func getTranslatorCompatibilityTestCases() []translatorCompatibilityTestCase {
	return []translatorCompatibilityTestCase{
		{
			name:        "context_and_rephrase_with_deepl_translator_invalid",
			provider:    constants.ProviderDeepL,
			context:     "Some context",
			rephrase:    true,
			keys:        []string{"test.key"},
			locale:      "en",
			expectError: true,
			description: "Context + rephrase together should be rejected for DeepL translator",
		},
		{
			name:        "context_with_deepl_translator_invalid",
			provider:    constants.ProviderDeepL,
			context:     "Some context",
			rephrase:    false,
			keys:        nil,
			locale:      "",
			expectError: true,
			description: "Context flag should not work with DeepL translator",
		},
		{
			name:        "context_with_claude_llm_valid",
			provider:    constants.ProviderClaude,
			context:     "Some context",
			rephrase:    false,
			keys:        nil,
			locale:      "",
			expectError: false,
			description: "Context flag should work with Claude LLM",
		},
		{
			name:        "rephrase_with_deepl_translator_invalid",
			provider:    constants.ProviderDeepL,
			context:     "",
			rephrase:    true,
			keys:        []string{"test.key"},
			locale:      "en",
			expectError: true,
			description: "Rephrase flag should not work with DeepL translator",
		},
		{
			name:        "rephrase_with_claude_llm_valid",
			provider:    constants.ProviderClaude,
			context:     "",
			rephrase:    true,
			keys:        []string{"test.key"},
			locale:      "en",
			expectError: false,
			description: "Rephrase flag should work with Claude LLM",
		},
		{
			name:        "no_special_flags_with_deepl_valid",
			provider:    constants.ProviderDeepL,
			context:     "",
			rephrase:    false,
			keys:        nil,
			locale:      "",
			expectError: false,
			description: "Normal flags should work with DeepL translator",
		},
		{
			name:        "context_with_nil_config_valid",
			provider:    "", // Will use nil config
			context:     "Some context",
			rephrase:    false,
			keys:        nil,
			locale:      "",
			expectError: false,
			description: "Context should work with nil config (testing scenario)",
		},
	}
}

// getOverwriteValidationTestCases returns test cases for overwrite flag validation.
//
//nolint:funlen // Table-driven test data is clearer kept together
func getOverwriteValidationTestCases() []overwriteValidationTestCase {
	return []overwriteValidationTestCase{
		{
			name:         "overwrite with glossary is valid",
			overwrite:    true,
			glossaryOnly: true,
			sourcedOnly:  false,
			memoryOnly:   false,
			stubMode:     false,
			prune:        false,
			check:        false,
			rephrase:     false,
			keys:         nil,
			locale:       "",
			expectError:  false,
			description:  "Overwrite flag should work with glossary-only mode",
		},
		{
			name:         "overwrite without glossary is invalid",
			overwrite:    true,
			glossaryOnly: false,
			sourcedOnly:  false,
			memoryOnly:   false,
			stubMode:     false,
			prune:        false,
			check:        false,
			rephrase:     false,
			keys:         nil,
			locale:       "",
			expectError:  true,
			description:  "Overwrite flag requires glossary flag",
		},
		{
			name:         "overwrite with sourced is invalid",
			overwrite:    true,
			glossaryOnly: false,
			sourcedOnly:  true,
			memoryOnly:   false,
			stubMode:     false,
			prune:        false,
			check:        false,
			rephrase:     false,
			keys:         nil,
			locale:       "",
			expectError:  true,
			description:  "Overwrite flag cannot be used with sourced flag",
		},
		{
			name:         "overwrite with memory is invalid",
			overwrite:    true,
			glossaryOnly: false,
			sourcedOnly:  false,
			memoryOnly:   true,
			stubMode:     false,
			prune:        false,
			check:        false,
			rephrase:     false,
			keys:         nil,
			locale:       "",
			expectError:  true,
			description:  "Overwrite flag cannot be used with memory flag",
		},
		{
			name:         "overwrite with glossary and memory is invalid",
			overwrite:    true,
			glossaryOnly: true,
			sourcedOnly:  false,
			memoryOnly:   true,
			stubMode:     false,
			prune:        false,
			check:        false,
			rephrase:     false,
			keys:         nil,
			locale:       "",
			expectError:  true,
			description:  "Overwrite flag requires glossary only, not glossary+memory",
		},
		{
			name:         "overwrite with glossary and sourced is invalid",
			overwrite:    true,
			glossaryOnly: true,
			sourcedOnly:  true,
			memoryOnly:   false,
			stubMode:     false,
			prune:        false,
			check:        false,
			rephrase:     false,
			keys:         nil,
			locale:       "",
			expectError:  true,
			description:  "Overwrite flag requires glossary only, not glossary+sourced",
		},
		{
			name:         "overwrite with stub is invalid",
			overwrite:    true,
			glossaryOnly: true,
			sourcedOnly:  false,
			memoryOnly:   false,
			stubMode:     true,
			prune:        false,
			check:        false,
			rephrase:     false,
			keys:         nil,
			locale:       "",
			expectError:  true,
			description:  "Overwrite flag cannot be used with stub mode",
		},
		{
			name:         "overwrite with prune is invalid",
			overwrite:    true,
			glossaryOnly: true,
			sourcedOnly:  false,
			memoryOnly:   false,
			stubMode:     false,
			prune:        true,
			check:        false,
			rephrase:     false,
			keys:         nil,
			locale:       "",
			expectError:  true,
			description:  "Overwrite flag cannot be used with prune flag",
		},
		{
			name:         "overwrite with check is invalid",
			overwrite:    true,
			glossaryOnly: true,
			sourcedOnly:  false,
			memoryOnly:   false,
			stubMode:     false,
			prune:        false,
			check:        true,
			rephrase:     false,
			keys:         nil,
			locale:       "",
			expectError:  true,
			description:  "Overwrite flag cannot be used with check mode",
		},
		{
			name:         "overwrite with rephrase is invalid",
			overwrite:    true,
			glossaryOnly: true,
			sourcedOnly:  false,
			memoryOnly:   false,
			stubMode:     false,
			prune:        false,
			check:        false,
			rephrase:     true,
			keys:         []string{"test.key"},
			locale:       "en",
			expectError:  true,
			description:  "Overwrite flag cannot be used with rephrase mode",
		},
		{
			name:         "glossary without overwrite is valid",
			overwrite:    false,
			glossaryOnly: true,
			sourcedOnly:  false,
			memoryOnly:   false,
			stubMode:     false,
			prune:        false,
			check:        false,
			rephrase:     false,
			keys:         nil,
			locale:       "",
			expectError:  false,
			description:  "Glossary flag should work without overwrite",
		},
		{
			name:         "no flags is valid",
			overwrite:    false,
			glossaryOnly: false,
			sourcedOnly:  false,
			memoryOnly:   false,
			stubMode:     false,
			prune:        false,
			check:        false,
			rephrase:     false,
			keys:         nil,
			locale:       "",
			expectError:  false,
			description:  "No flags should be valid",
		},
	}
}

// TestOverwriteValidation tests overwrite flag validation rules.
func TestOverwriteValidation(t *testing.T) {
	t.Parallel()
	for _, tt := range getOverwriteValidationTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testFlags := &flags.Flags{ //nolint:exhaustruct // Test case only sets relevant fields
				TrimStrings:  true,
				StubMode:     tt.stubMode,
				IncludeStubs: false,
				SourcedOnly:  tt.sourcedOnly,
				GlossaryOnly: tt.glossaryOnly,
				MemoryOnly:   tt.memoryOnly,
				Overwrite:    tt.overwrite,
				Rephrase:     tt.rephrase,
				Check:        tt.check,
				Format:       "",
				Context:      "",
				Prune:        tt.prune,
				Keys:         tt.keys,
				Locale:       tt.locale,
				Directory:    "",
			}
			testFlags.UpdateDerivedFlags()

			err := flags.ValidateFlagCombinations(testFlags, nil)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for case: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for case: %s, error: %v", tt.description, err)
			}
		})
	}
}
