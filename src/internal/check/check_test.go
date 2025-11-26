package check_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/juliandreas/dire/internal/check"
	"github.com/juliandreas/dire/internal/flags"
	"github.com/juliandreas/dire/internal/testutil"
	"github.com/juliandreas/dire/internal/types"
)

var (
	errScanFailed             = errors.New("scan failed")
	errDetermineMissingFailed = errors.New("determine missing failed")
)

func TestService_Run_Complete(t *testing.T) {
	t.Parallel()

	// Setup: Complete translations (no missing keys)
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {
			Locale: "en",
			Contents: map[string]any{
				"greeting": "Hello",
				"farewell": "Goodbye",
			},
		},
		"es": {
			Locale: "es",
			Contents: map[string]any{
				"greeting": "Hola",
				"farewell": "Adiós",
			},
		},
	}

	mockRepo := &testutil.MockRepository{
		LocaleFiles:      localeFiles,
		Missing:          []types.MissingTranslation{}, // No missing translations
		MissingWithStubs: nil,
		ReferenceLocale:  "en",
		ScanErr:          nil,
		IdentifyErr:      nil,
	}

	cfg := testutil.MockConfig()
	testFlags := createTestFlags()
	service := check.NewService(cfg, testFlags, mockRepo)

	ctx := context.Background()
	result, err := service.Run(ctx)
	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Complete {
		t.Errorf("Expected complete=true, got: %v", result.Complete)
	}

	if len(result.MissingByLocale) != 0 {
		t.Errorf("Expected no missing keys by locale, got: %v", result.MissingByLocale)
	}
}

func TestService_Run_Incomplete(t *testing.T) {
	t.Parallel()

	localeFiles, missing := createIncompleteTestData()
	testIncompleteTranslations(t, localeFiles, missing)
}

func createIncompleteTestData() (map[types.Locale]*types.LocaleFile, []types.MissingTranslation) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {
			Locale: "en",
			Contents: map[string]any{
				"greeting": "Hello",
				"farewell": "Goodbye",
				"nested": map[string]any{
					"welcome": "Welcome",
				},
			},
		},
		"es": {
			Locale: "es",
			Contents: map[string]any{
				"greeting": "Hola",
				// Missing "farewell" and "nested.welcome"
			},
		},
	}

	missing := []types.MissingTranslation{
		{
			Path:         []string{"farewell"},
			TargetLocale: "es",
			SourceLocale: "",
			SourceValue:  "",
		},
		{
			Path:         []string{"nested", "welcome"},
			TargetLocale: "es",
			SourceLocale: "",
			SourceValue:  "",
		},
	}

	return localeFiles, missing
}

func testIncompleteTranslations(
	t *testing.T,
	localeFiles map[types.Locale]*types.LocaleFile,
	missing []types.MissingTranslation,
) {
	t.Helper()
	mockRepo := &testutil.MockRepository{
		LocaleFiles:      localeFiles,
		Missing:          missing,
		MissingWithStubs: nil,
		ReferenceLocale:  "en",
		ScanErr:          nil,
		IdentifyErr:      nil,
	}

	cfg := testutil.MockConfig()
	testFlags := createTestFlags()
	service := check.NewService(cfg, testFlags, mockRepo)

	ctx := context.Background()
	result, err := service.Run(ctx)
	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Complete {
		t.Errorf("Expected complete=false, got: %v", result.Complete)
	}

	// Check missing by locale
	esMissing, exists := result.MissingByLocale["es"]
	if !exists {
		t.Errorf("Expected missing keys for 'es' locale")
		return
	}

	if len(esMissing) != 2 {
		t.Errorf("Expected 2 missing keys for 'es', got: %d", len(esMissing))
		return
	}

	// Check specific missing keys
	found := make(map[string]bool)
	for _, missing := range esMissing {
		found[missing.Key] = true
	}
	if !found["farewell"] {
		t.Errorf("Expected 'farewell' to be missing")
	}
	if !found["nested.welcome"] {
		t.Errorf("Expected 'nested.welcome' to be missing")
	}
}

func TestService_Run_EmptyFiles(t *testing.T) {
	t.Parallel()

	// Setup: Empty locale files
	localeFiles := map[types.Locale]*types.LocaleFile{}

	mockRepo := &testutil.MockRepository{
		LocaleFiles:      localeFiles,
		Missing:          []types.MissingTranslation{},
		MissingWithStubs: nil,
		ReferenceLocale:  "",
		ScanErr:          nil,
		IdentifyErr:      nil,
	}

	cfg := testutil.MockConfig()
	testFlags := createTestFlags()
	service := check.NewService(cfg, testFlags, mockRepo)

	ctx := context.Background()
	result, err := service.Run(ctx)
	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Complete {
		t.Errorf("Expected complete=true for empty files, got: %v", result.Complete)
	}
}

func TestService_Run_IncludeStubs_False(t *testing.T) {
	t.Parallel()

	mockRepo := setupIncludeStubsTest()
	cfg := testutil.MockConfig()

	// Test with includeStubs = false
	//nolint:exhaustruct // Test case only sets relevant fields
	testFlags := &flags.Flags{
		TrimStrings:                true,
		StubMode:                   false,
		IncludeStubs:               false, // Exclude stub translations
		SourcedOnly:                false,
		Rephrase:                   false,
		Check:                      false,
		Format:                     "",
		Context:                    "",
		Keys:                       nil,
		Locale:                     "",
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

	service := check.NewService(cfg, testFlags, mockRepo)
	ctx := context.Background()
	result, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !result.Complete {
		t.Errorf("Expected complete=true when excluding stubs, got: %v", result.Complete)
	}
	if len(result.MissingByLocale) != 0 {
		t.Errorf("Expected no missing keys when excluding stubs, got: %v", result.MissingByLocale)
	}
}

func TestService_Run_IncludeStubs_True(t *testing.T) {
	t.Parallel()

	mockRepo := setupIncludeStubsTest()
	cfg := testutil.MockConfig()

	// Test with includeStubs = true
	//nolint:exhaustruct // Test case only sets relevant fields
	testFlags := &flags.Flags{
		TrimStrings:                true,
		StubMode:                   false,
		IncludeStubs:               true, // Include stub translations
		SourcedOnly:                false,
		Rephrase:                   false,
		Check:                      false,
		Format:                     "",
		Context:                    "",
		Keys:                       nil,
		Locale:                     "",
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

	service := check.NewService(cfg, testFlags, mockRepo)
	ctx := context.Background()
	result, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Complete {
		t.Errorf("Expected complete=false when including stubs, got: %v", result.Complete)
	}

	// Check that stub translation is detected as missing
	esMissing, exists := result.MissingByLocale["es"]
	if !exists {
		t.Errorf("Expected missing keys for 'es' locale when including stubs")
		return
	}
	if len(esMissing) != 1 {
		t.Errorf("Expected 1 missing key for 'es' when including stubs, got: %d", len(esMissing))
		return
	}
	if esMissing[0].Key != "farewell" {
		t.Errorf("Expected missing key 'farewell', got: %s", esMissing[0].Key)
	}
}

func setupIncludeStubsTest() *testutil.MockRepository {
	// Setup: Test includeStubs flag behavior
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {
			Locale: "en",
			Contents: map[string]any{
				"greeting": "Hello",
				"farewell": "Goodbye",
			},
		},
		"es": {
			Locale: "es",
			Contents: map[string]any{
				"greeting": "Hola",
				"farewell": "", // Stub translation (empty string)
			},
		},
	}

	// Missing without stubs (only completely missing keys)
	missingWithoutStubs := []types.MissingTranslation{}

	// Missing with stubs (includes stub translations)
	missingWithStubs := []types.MissingTranslation{
		{
			Path:         []string{"farewell"},
			TargetLocale: "es",
			SourceLocale: "",
			SourceValue:  "",
		},
	}

	mockRepo := &testutil.MockRepository{
		LocaleFiles:      localeFiles,
		Missing:          missingWithoutStubs,
		MissingWithStubs: missingWithStubs,
		ReferenceLocale:  "en",
		ScanErr:          nil,
		IdentifyErr:      nil,
	}

	return mockRepo
}

func createTestFlags() *flags.Flags {
	//nolint:exhaustruct // Test helper only sets relevant fields
	return &flags.Flags{
		TrimStrings:                true,
		StubMode:                   false,
		IncludeStubs:               false,
		SourcedOnly:                false,
		Rephrase:                   false,
		Check:                      true, // Reflect check-mode semantics
		Format:                     "",
		Context:                    "",
		Keys:                       nil,
		Locale:                     "",
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
		RequiresTranslationService: false, // Check mode doesn't require AI
	}
}

// Test exit codes returned by OutputResult.
func TestService_OutputResult_ExitCodes(t *testing.T) {
	t.Parallel()

	// Test success case
	completeResult := &check.LintResult{
		Complete:        true,
		MissingByLocale: map[string][]check.MissingKey{},
	}

	service := check.NewService(testutil.MockConfig(), createTestFlags(), &testutil.MockRepository{
		LocaleFiles:      map[types.Locale]*types.LocaleFile{},
		Missing:          []types.MissingTranslation{},
		MissingWithStubs: nil,
		ReferenceLocale:  "",
		ScanErr:          nil,
		IdentifyErr:      nil,
	})

	exitCode := service.OutputResult(completeResult, false)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for complete translations, got %d", exitCode)
	}

	// Test failure case
	incompleteResult := &check.LintResult{
		Complete: false,
		MissingByLocale: map[string][]check.MissingKey{
			"es": {{Key: "test.key", KeyPath: []string{"test", "key"}}},
		},
	}

	exitCode = service.OutputResult(incompleteResult, false)
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for incomplete translations, got %d", exitCode)
	}

	// Test nil result case
	exitCode = service.OutputResult(nil, false)
	if exitCode != 2 {
		t.Errorf("Expected exit code 2 for nil result, got %d", exitCode)
	}
}

func TestService_OutputResult_JSON_ExitCodes(t *testing.T) {
	t.Parallel()

	// Test JSON format exit codes
	service := check.NewService(testutil.MockConfig(), createTestFlags(), &testutil.MockRepository{
		LocaleFiles:      map[types.Locale]*types.LocaleFile{},
		Missing:          []types.MissingTranslation{},
		MissingWithStubs: nil,
		ReferenceLocale:  "",
		ScanErr:          nil,
		IdentifyErr:      nil,
	})

	// Test success case with JSON format
	completeResult := &check.LintResult{
		Complete:        true,
		MissingByLocale: map[string][]check.MissingKey{},
	}

	exitCode := service.OutputResult(completeResult, true) // JSON format
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for complete translations with JSON format, got %d", exitCode)
	}

	// Test failure case with JSON format
	incompleteResult := &check.LintResult{
		Complete: false,
		MissingByLocale: map[string][]check.MissingKey{
			"es": {{Key: "test.key", KeyPath: []string{"test", "key"}}},
		},
	}

	exitCode = service.OutputResult(incompleteResult, true) // JSON format
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for incomplete translations with JSON format, got %d", exitCode)
	}

	// Test nil result case with JSON format
	exitCode = service.OutputResult(nil, true) // JSON format
	if exitCode != 2 {
		t.Errorf("Expected exit code 2 for nil result with JSON format, got %d", exitCode)
	}
}

func TestService_GroupMissingByLocale(t *testing.T) {
	t.Parallel()

	// Test the grouping logic through the public API
	missing := []types.MissingTranslation{
		{Path: []string{"key1"}, TargetLocale: "es", SourceLocale: "", SourceValue: ""},
		{Path: []string{"section", "key2"}, TargetLocale: "es", SourceLocale: "", SourceValue: ""},
		{Path: []string{"key3"}, TargetLocale: "fr", SourceLocale: "", SourceValue: ""},
	}

	// Create a mock service and test through the Run method
	localeFiles := map[types.Locale]*types.LocaleFile{"en": {Locale: "en"}}
	mockRepo := &testutil.MockRepository{
		LocaleFiles:      localeFiles,
		Missing:          missing,
		MissingWithStubs: nil,
		ReferenceLocale:  "en",
		ScanErr:          nil,
		IdentifyErr:      nil,
	}

	cfg := testutil.MockConfig()
	testFlags := createTestFlags()
	service := check.NewService(cfg, testFlags, mockRepo)

	result, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Test the grouped results
	groupedResult := result.MissingByLocale

	// Check es locale
	esMissing, exists := groupedResult["es"]
	switch {
	case !exists:
		t.Errorf("Expected missing keys for 'es' locale")
	case len(esMissing) != 2:
		t.Errorf("Expected 2 missing keys for 'es', got %d", len(esMissing))
	}

	// Check fr locale
	frMissing, exists := groupedResult["fr"]
	switch {
	case !exists:
		t.Errorf("Expected missing keys for 'fr' locale")
	case len(frMissing) != 1:
		t.Errorf("Expected 1 missing key for 'fr', got %d", len(frMissing))
	}

	// Check specific keys
	if len(esMissing) >= 2 {
		found := make(map[string]bool)
		for _, missing := range esMissing {
			found[missing.Key] = true
		}
		if !found["key1"] {
			t.Errorf("Expected 'key1' to be missing for es")
		}
		if !found["section.key2"] {
			t.Errorf("Expected 'section.key2' to be missing for es")
		}
	}
}

func TestService_Run_ScanError(t *testing.T) {
	t.Parallel()

	// Test Run method when Scan returns an error
	mockRepo := &testutil.MockRepository{
		LocaleFiles:      nil,
		Missing:          nil,
		MissingWithStubs: nil,
		ReferenceLocale:  "",
		ScanErr:          errScanFailed, // Simulate scan error
		IdentifyErr:      nil,
	}

	cfg := testutil.MockConfig()
	testFlags := createTestFlags()
	service := check.NewService(cfg, testFlags, mockRepo)

	ctx := context.Background()
	result, err := service.Run(ctx)

	// Should return error and nil result
	if err == nil {
		t.Errorf("Expected error from Run when Scan fails, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result when Scan fails, got: %v", result)
	}
	if !strings.Contains(err.Error(), "failed to scan locale files") {
		t.Errorf("Expected error message to contain 'failed to scan locale files', got: %v", err)
	}
}

func TestService_Run_DetermineMissingError(t *testing.T) {
	t.Parallel()

	// Test Run method when DetermineMissing returns an error
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {Locale: "en", Contents: map[string]any{"test": "value"}},
	}
	mockRepo := &testutil.MockRepository{
		LocaleFiles:      localeFiles,
		Missing:          nil,
		MissingWithStubs: nil,
		ReferenceLocale:  "en",
		ScanErr:          nil,
		IdentifyErr:      errDetermineMissingFailed, // Simulate determine missing error
	}

	cfg := testutil.MockConfig()
	testFlags := createTestFlags()
	service := check.NewService(cfg, testFlags, mockRepo)

	ctx := context.Background()
	result, err := service.Run(ctx)

	// Should return error and nil result
	if err == nil {
		t.Errorf("Expected error from Run when DetermineMissing fails, got nil")
	}
	if result != nil {
		t.Errorf("Expected nil result when DetermineMissing fails, got: %v", result)
	}
	if !strings.Contains(err.Error(), "failed to determine missing translations") {
		t.Errorf("Expected error message to contain 'failed to determine missing translations', got: %v", err)
	}
}

func TestService_Run_Deduplication(t *testing.T) {
	t.Parallel()

	// Test that check service deduplicates missing translations from multiple sources
	cfg := testutil.MockConfig()
	testFlags := createTestFlags()

	// Create mock repository that returns duplicate missing translations
	mockRepo := &testutil.MockRepository{
		LocaleFiles: map[types.Locale]*types.LocaleFile{
			"en-US": {Locale: "en-US", Contents: map[string]any{"cart": map[string]any{"checkout": "Checkout"}}},
			"sv-SE": {Locale: "sv-SE", Contents: map[string]any{"cart": map[string]any{}}},
		},
		Missing: []types.MissingTranslation{
			{
				Path:         []string{"cart", "checkout"},
				SourceLocale: "en-US",
				SourceValue:  "Checkout",
				TargetLocale: "sv-SE",
			},
			{
				Path:         []string{"cart", "checkout"}, // Same key, different source
				SourceLocale: "da-DK",
				SourceValue:  "Gå til betaling",
				TargetLocale: "sv-SE",
			},
		},
		MissingWithStubs: nil,
		ReferenceLocale:  "en-US",
		ScanErr:          nil,
		IdentifyErr:      nil,
	}

	service := check.NewService(cfg, testFlags, mockRepo)
	ctx := context.Background()
	result, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("Unexpected error from Run: %v", err)
	}

	// Should deduplicate the missing translation
	if len(result.MissingByLocale["sv-SE"]) != 1 {
		t.Errorf("Expected 1 missing key for sv-SE after deduplication, got %d", len(result.MissingByLocale["sv-SE"]))
	}

	if result.MissingByLocale["sv-SE"][0].Key != "cart.checkout" {
		t.Errorf("Expected deduplicated key 'cart.checkout' for sv-SE, got '%s'", result.MissingByLocale["sv-SE"][0].Key)
	}

	if result.Complete {
		t.Error("Expected result.Complete to be false when translations are missing")
	}
}
