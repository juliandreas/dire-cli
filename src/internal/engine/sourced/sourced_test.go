package sourced_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/engine/sourced"
	"github.com/juliandreas/dire/internal/types"
)

const (
	// Common test translations used across multiple tests.
	frSalut    = "Salut"
	frAuRevoir = "Au revoir"
)

func TestApplySourced_Basic(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	missing := []types.MissingTranslation{
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "Hello",
			Path:         []string{"greeting"},
		},
	}

	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {
			Path:   filepath.Join(tempDir, "en.json"),
			Locale: "en",
			Contents: map[string]any{
				"greeting": "Hello",
			},
		},
		"fr": {
			Path:     filepath.Join(tempDir, "fr.json"),
			Locale:   "fr",
			Contents: map[string]any{},
		},
	}

	cfg := &config.Config{} //nolint:exhaustruct // Empty config for testing

	count, err := sourced.ApplySourced(ctx, missing, localeFiles, "en", cfg, true, true)
	if err != nil {
		t.Fatalf("ApplySourced failed: %v", err)
	}

	// Should find 0 matches since no glossary or memory exists
	if count != 0 {
		t.Errorf("Expected 0 sourced translations, got %d", count)
	}

	// Verify no accidental writes when no sources exist
	if greeting, exists := localeFiles["fr"].Contents["greeting"]; exists {
		t.Errorf("Expected no greeting in fr.json, but found: %v", greeting)
	}
}

func TestApplySourced_WithMemory(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	missing := []types.MissingTranslation{
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "Hello",
			Path:         []string{"greeting"},
		},
	}

	// Set up locale files with existing French translation for memory
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {
			Path:   filepath.Join(tempDir, "en.json"),
			Locale: "en",
			Contents: map[string]any{
				"greeting": "Hello",
				"other":    "Hello", // Same source text in different location
			},
		},
		"fr": {
			Path:   filepath.Join(tempDir, "fr.json"),
			Locale: "fr",
			Contents: map[string]any{
				"other": "Bonjour", // Memory match for "Hello"
			},
		},
	}

	cfg := &config.Config{} //nolint:exhaustruct // Empty config for testing

	count, err := sourced.ApplySourced(ctx, missing, localeFiles, "en", cfg, true, true)
	if err != nil {
		t.Fatalf("ApplySourced failed: %v", err)
	}

	// Should find 1 match via translation memory
	if count != 1 {
		t.Errorf("Expected 1 sourced translation, got %d", count)
	}

	// Verify the value written via memory
	got, ok := localeFiles["fr"].Contents["greeting"]
	if !ok || got != "Bonjour" {
		t.Fatalf("Expected fr.greeting to be %q from memory, got %#v (ok=%v)", "Bonjour", got, ok)
	}
}

func TestApplySourced_EmptyInput(t *testing.T) {
	ctx := context.Background()

	var missing []types.MissingTranslation
	localeFiles := map[types.Locale]*types.LocaleFile{}
	cfg := &config.Config{} //nolint:exhaustruct // Empty config for testing

	count, err := sourced.ApplySourced(ctx, missing, localeFiles, "en", cfg, true, true)
	if err != nil {
		t.Fatalf("ApplySourced failed: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 sourced translations for empty input, got %d", count)
	}
}

func TestApplySourced_GlossaryCasing(t *testing.T) { //nolint:funlen // Table-driven test is better kept together
	ctx := context.Background()

	// Test cases for different casing patterns
	testCases := []struct {
		name           string
		sourceValue    string
		expectedTarget string
		description    string
	}{
		{"lowercase", "hello", "salut", "all lowercase should remain lowercase"},
		{"uppercase", "HELLO", "SALUT", "all uppercase should become uppercase"},
		{"title case", "Hello", "Salut", "title case should become title case"},
		{"mixed case preserved", "HeLLo", "salut", "mixed case should leave target unchanged"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Isolated temp dir per case
			caseDir := t.TempDir()
			missing := []types.MissingTranslation{
				{
					SourceLocale: "en",
					TargetLocale: "fr",
					SourceValue:  tc.sourceValue,
					Path:         []string{"greeting"},
				},
			}

			localeFiles := map[types.Locale]*types.LocaleFile{
				"en": {
					Path:   filepath.Join(caseDir, "en.json"),
					Locale: "en",
					Contents: map[string]any{
						"greeting": tc.sourceValue,
					},
				},
				"fr": {
					Path:     filepath.Join(caseDir, "fr.json"),
					Locale:   "fr",
					Contents: map[string]any{},
				},
			}

			// Create glossary with lowercase entries that should have casing applied
			cfg := &config.Config{} //nolint:exhaustruct // Minimal config for testing
			cfg.Glossary.AutoSort = false
			cfg.Glossary.Entries = []types.GlossaryEntry{
				{"en": "hello", "fr": "salut"},
			}

			count, err := sourced.ApplySourced(ctx, missing, localeFiles, "en", cfg, true, true)
			if err != nil {
				t.Fatalf("ApplySourced failed: %v", err)
			}

			// Should find 1 match via glossary
			if count != 1 {
				t.Errorf("Expected 1 sourced translation, got %d", count)
			}

			// Verify the casing was applied correctly by checking the file contents
			expectedContents := map[string]any{
				"greeting": tc.expectedTarget,
			}

			if !compareMapContents(t, localeFiles["fr"].Contents, expectedContents) {
				t.Errorf("Casing test '%s' failed: expected %q, got %q (%s)",
					tc.name, tc.expectedTarget, localeFiles["fr"].Contents["greeting"], tc.description)
			}
		})
	}
}

// compareMapContents compares two map[string]any for equality.
func compareMapContents(t *testing.T, actual, expected map[string]any) bool {
	t.Helper()
	if len(actual) != len(expected) {
		return false
	}
	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists || actualValue != expectedValue {
			return false
		}
	}
	return true
}

func TestApplySourced_GlossaryBeatsMemory(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	missing := []types.MissingTranslation{
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "Hello",
			Path:         []string{"greeting"},
		},
	}

	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {
			Path:   filepath.Join(tempDir, "en.json"),
			Locale: "en",
			Contents: map[string]any{
				"greeting": "Hello",
				"other":    "Hello", // Same source text for memory
			},
		},
		"fr": {
			Path:   filepath.Join(tempDir, "fr.json"),
			Locale: "fr",
			Contents: map[string]any{
				"other": "Bonjour", // Memory has "Hello" → "Bonjour"
			},
		},
	}

	// Configure both glossary and memory - glossary should win
	cfg := &config.Config{} //nolint:exhaustruct // Minimal config for testing
	cfg.Glossary.AutoSort = false
	cfg.Glossary.Entries = []types.GlossaryEntry{
		{"en": "Hello", "fr": frSalut},
	}

	count, err := sourced.ApplySourced(ctx, missing, localeFiles, "en", cfg, true, true)
	if err != nil {
		t.Fatalf("ApplySourced failed: %v", err)
	}

	// Should find 1 match
	if count != 1 {
		t.Fatalf("Expected 1 sourced translation, got %d", count)
	}

	// Verify glossary wins over memory: should be "Salut" not "Bonjour"
	if got := localeFiles["fr"].Contents["greeting"]; got != frSalut {
		t.Fatalf("Expected %q from glossary (not 'Bonjour' from memory), got %#v", frSalut, got)
	}

	t.Logf("Correctly prioritized glossary %q over memory 'Bonjour'", frSalut)
}

func TestApplySourced_NestedStructures(t *testing.T) { //nolint:funlen // Comprehensive nested structure test
	ctx := context.Background()
	tempDir := t.TempDir()

	missing := []types.MissingTranslation{
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue: map[string]any{
				"messages": []any{
					"Hello",                             // Should match glossary
					map[string]any{"nested": "Welcome"}, // Should match memory
					[]any{"Goodbye", map[string]any{"deep": "Yes"}}, // Mixed nesting
				},
			},
			Path: []string{"ui"},
		},
	}

	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {
			Path:   filepath.Join(tempDir, "en.json"),
			Locale: "en",
			Contents: map[string]any{
				"ui": map[string]any{
					"messages": []any{
						"Hello",
						map[string]any{"nested": "Welcome"},
						[]any{"Goodbye", map[string]any{"deep": "Yes"}},
					},
				},
				// Memory sources
				"other": map[string]any{
					"greeting": "Welcome",
					"farewell": "Goodbye",
					"confirm":  "Yes",
				},
			},
		},
		"fr": {
			Path:   filepath.Join(tempDir, "fr.json"),
			Locale: "fr",
			Contents: map[string]any{
				// Memory translations
				"other": map[string]any{
					"greeting": "Bienvenue", // Welcome → Bienvenue
					"farewell": frAuRevoir,  // Goodbye → Au revoir
					"confirm":  "Oui",       // Yes → Oui
				},
			},
		},
	}

	// Configure glossary for "Hello"
	cfg := &config.Config{} //nolint:exhaustruct // Minimal config for testing
	cfg.Glossary.AutoSort = false
	cfg.Glossary.Entries = []types.GlossaryEntry{
		{"en": "Hello", "fr": frSalut},
	}

	count, err := sourced.ApplySourced(ctx, missing, localeFiles, "en", cfg, true, true)
	if err != nil {
		t.Fatalf("ApplySourced failed: %v", err)
	}

	// Should find matches for nested strings
	if count == 0 {
		t.Fatalf("Expected some sourced translations for nested structures, got 0")
	}

	// Verify that some translations were found (the test verifies the system can handle nested structures)
	t.Logf("Successfully processed nested structure with %d sourced translations", count)

	// Assert selected leaves (glossary should win for "Hello"; memory for others)
	ui, ok := localeFiles["fr"].Contents["ui"].(map[string]any)
	if !ok {
		t.Fatalf("Expected 'ui' object in fr.json, got: %#v", localeFiles["fr"].Contents["ui"])
	}
	msgs, ok := ui["messages"].(map[string]any)
	if !ok {
		t.Fatalf("Expected 'ui.messages' object, got: %#v", ui["messages"])
	}
	if msgs["0"] != frSalut {
		t.Errorf("messages[0]: expected %q (glossary), got %#v", frSalut, msgs["0"])
	}
	if obj, _ := msgs["1"].(map[string]any); obj["nested"] != "Bienvenue" {
		t.Errorf("messages[1].nested: expected %q (memory), got %#v", "Bienvenue", obj["nested"])
	}
	if arr, _ := msgs["2"].(map[string]any); arr["0"] != frAuRevoir {
		t.Errorf("messages[2][0]: expected %q (memory), got %#v", frAuRevoir, arr["0"])
	}
}

// TestApplySourced_GlossaryOnly verifies that only glossary translations are applied
// when enableGlossary=true and enableMemory=false.
//
//nolint:dupl // Test setup intentionally similar to MemoryOnly test for clarity
func TestApplySourced_GlossaryOnly(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Setup: Both glossary and memory are available
	missing := []types.MissingTranslation{
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "Hello", // Matches glossary
			Path:         []string{"greeting"},
		},
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "Goodbye", // Matches memory
			Path:         []string{"farewell"},
		},
	}

	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {
			Path:   filepath.Join(tempDir, "en.json"),
			Locale: "en",
			Contents: map[string]any{
				"greeting": "Hello",
				"farewell": "Goodbye",
				"other":    "Goodbye", // Memory source
			},
		},
		"fr": {
			Path:   filepath.Join(tempDir, "fr.json"),
			Locale: "fr",
			Contents: map[string]any{
				"other": frAuRevoir, // Memory match for "Goodbye"
			},
		},
	}

	// Config with glossary
	cfg := &config.Config{} //nolint:exhaustruct // Minimal config for testing
	cfg.Glossary.AutoSort = false
	cfg.Glossary.Entries = []types.GlossaryEntry{
		{"en": "Hello", "fr": frSalut},
	}

	// Call with glossary enabled, memory disabled
	count, err := sourced.ApplySourced(ctx, missing, localeFiles, "en", cfg, true, false)
	if err != nil {
		t.Fatalf("ApplySourced failed: %v", err)
	}

	// Should find only 1 match (glossary only, not memory)
	if count != 1 {
		t.Errorf("Expected 1 sourced translation (glossary only), got %d", count)
	}

	// Verify glossary was applied
	if val := localeFiles["fr"].Contents["greeting"]; val != frSalut {
		t.Errorf("Expected %q from glossary, got %v", frSalut, val)
	}

	// Verify memory was NOT applied (farewell should not exist)
	if val, exists := localeFiles["fr"].Contents["farewell"]; exists {
		t.Errorf("Expected memory to be skipped, but 'farewell' was translated to: %v", val)
	}
}

// TestApplySourced_MemoryOnly verifies that only memory translations are applied
// when enableGlossary=false and enableMemory=true.
//
//nolint:dupl // Test setup intentionally similar to GlossaryOnly test for clarity
func TestApplySourced_MemoryOnly(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Setup: Both glossary and memory are available
	missing := []types.MissingTranslation{
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "Hello", // Matches glossary
			Path:         []string{"greeting"},
		},
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "Goodbye", // Matches memory
			Path:         []string{"farewell"},
		},
	}

	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {
			Path:   filepath.Join(tempDir, "en.json"),
			Locale: "en",
			Contents: map[string]any{
				"greeting": "Hello",
				"farewell": "Goodbye",
				"other":    "Goodbye", // Memory source
			},
		},
		"fr": {
			Path:   filepath.Join(tempDir, "fr.json"),
			Locale: "fr",
			Contents: map[string]any{
				"other": frAuRevoir, // Memory match for "Goodbye"
			},
		},
	}

	// Config with glossary (should be ignored)
	cfg := &config.Config{} //nolint:exhaustruct // Minimal config for testing
	cfg.Glossary.AutoSort = false
	cfg.Glossary.Entries = []types.GlossaryEntry{
		{"en": "Hello", "fr": frSalut},
	}

	// Call with glossary disabled, memory enabled
	count, err := sourced.ApplySourced(ctx, missing, localeFiles, "en", cfg, false, true)
	if err != nil {
		t.Fatalf("ApplySourced failed: %v", err)
	}

	// Should find only 1 match (memory only, not glossary)
	if count != 1 {
		t.Errorf("Expected 1 sourced translation (memory only), got %d", count)
	}

	// Verify memory was applied
	if val := localeFiles["fr"].Contents["farewell"]; val != frAuRevoir {
		t.Errorf("Expected %q from memory, got %v", frAuRevoir, val)
	}

	// Verify glossary was NOT applied (greeting should not exist)
	if val, exists := localeFiles["fr"].Contents["greeting"]; exists {
		t.Errorf("Expected glossary to be skipped, but 'greeting' was translated to: %v", val)
	}
}

// TestApplySourced_NeitherEnabled verifies that no translations are applied
// when both enableGlossary=false and enableMemory=false.
func TestApplySourced_NeitherEnabled(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Setup: Both glossary and memory are available
	missing := []types.MissingTranslation{
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "Hello",
			Path:         []string{"greeting"},
		},
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "Goodbye",
			Path:         []string{"farewell"},
		},
	}

	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {
			Path:   filepath.Join(tempDir, "en.json"),
			Locale: "en",
			Contents: map[string]any{
				"greeting": "Hello",
				"farewell": "Goodbye",
				"other":    "Goodbye",
			},
		},
		"fr": {
			Path:   filepath.Join(tempDir, "fr.json"),
			Locale: "fr",
			Contents: map[string]any{
				"other": frAuRevoir,
			},
		},
	}

	// Config with glossary (should be ignored)
	cfg := &config.Config{} //nolint:exhaustruct // Minimal config for testing
	cfg.Glossary.AutoSort = false
	cfg.Glossary.Entries = []types.GlossaryEntry{
		{"en": "Hello", "fr": frSalut},
	}

	// Call with both disabled
	count, err := sourced.ApplySourced(ctx, missing, localeFiles, "en", cfg, false, false)
	if err != nil {
		t.Fatalf("ApplySourced failed: %v", err)
	}

	// Should find 0 matches (both sources disabled)
	if count != 0 {
		t.Errorf("Expected 0 sourced translations (both disabled), got %d", count)
	}

	// Verify no translations were applied
	if _, exists := localeFiles["fr"].Contents["greeting"]; exists {
		t.Error("Expected no glossary translations when disabled")
	}
	if _, exists := localeFiles["fr"].Contents["farewell"]; exists {
		t.Error("Expected no memory translations when disabled")
	}
}

// TestApplySourced_BothEnabled verifies that both glossary and memory work together
// when both are enabled, and that glossary takes priority.
func TestApplySourced_BothEnabled(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Setup: Translation that matches both glossary AND memory
	missing := []types.MissingTranslation{
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "Hello",
			Path:         []string{"greeting"},
		},
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "Goodbye", // Only in memory
			Path:         []string{"farewell"},
		},
	}

	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {
			Path:   filepath.Join(tempDir, "en.json"),
			Locale: "en",
			Contents: map[string]any{
				"greeting": "Hello",
				"farewell": "Goodbye",
				"hello2":   "Hello",   // Same source for memory
				"goodbye2": "Goodbye", // Same source for memory
			},
		},
		"fr": {
			Path:   filepath.Join(tempDir, "fr.json"),
			Locale: "fr",
			Contents: map[string]any{
				"hello2":   "Bonjour",  // Memory match for "Hello"
				"goodbye2": frAuRevoir, // Memory match for "Goodbye"
			},
		},
	}

	// Config with glossary that conflicts with memory
	cfg := &config.Config{} //nolint:exhaustruct // Minimal config for testing
	cfg.Glossary.AutoSort = false
	cfg.Glossary.Entries = []types.GlossaryEntry{
		{"en": "Hello", "fr": frSalut}, // Glossary says "Salut", memory says "Bonjour"
	}

	// Call with both enabled
	count, err := sourced.ApplySourced(ctx, missing, localeFiles, "en", cfg, true, true)
	if err != nil {
		t.Fatalf("ApplySourced failed: %v", err)
	}

	// Should find 2 matches (1 glossary + 1 memory)
	if count != 2 {
		t.Errorf("Expected 2 sourced translations, got %d", count)
	}

	// Verify glossary takes priority over memory for "Hello"
	if val := localeFiles["fr"].Contents["greeting"]; val != frSalut {
		t.Errorf("Expected %q from glossary (not 'Bonjour' from memory), got %v", frSalut, val)
	}

	// Verify memory was applied for "Goodbye"
	if val := localeFiles["fr"].Contents["farewell"]; val != frAuRevoir {
		t.Errorf("Expected %q from memory, got %v", frAuRevoir, val)
	}
}

// TestApplySourced_CaseInsensitiveFiltering verifies that translations differing only in case are skipped.
// This test ensures we don't unnecessarily overwrite translations when the only difference is casing.
//
//nolint:funlen // Table-driven test with multiple cases is better kept together
func TestApplySourced_CaseInsensitiveFiltering(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tempDir := t.TempDir()

	testCases := []struct {
		name          string
		existingValue string
		glossaryValue string
		expectedCount int
		expectedValue string
		description   string
	}{
		{
			name:          "exact match - skip",
			existingValue: "Cost-Benefit Analysis",
			glossaryValue: "Cost-Benefit Analysis",
			expectedCount: 0,
			expectedValue: "Cost-Benefit Analysis",
			description:   "Exact match should be skipped",
		},
		{
			name:          "case difference only - skip",
			existingValue: "Cost-Benefit Analysis",
			glossaryValue: "Cost-benefit analysis",
			expectedCount: 0,
			expectedValue: "Cost-Benefit Analysis", // Should preserve existing casing
			description:   "Case-only difference should be skipped to preserve existing casing",
		},
		{
			name:          "all lowercase vs title case - skip",
			existingValue: "cost-benefit analysis",
			glossaryValue: "Cost-Benefit Analysis",
			expectedCount: 0,
			expectedValue: "cost-benefit analysis",
			description:   "Should preserve existing lowercase when glossary has title case",
		},
		{
			name:          "actual content difference - apply",
			existingValue: "Cost-Benefit Analysis",
			glossaryValue: "Analyse Coûts-Avantages",
			expectedCount: 1,
			expectedValue: "Analyse Coûts-Avantages",
			description:   "Real content difference should be applied",
		},
		{
			name:          "empty existing vs glossary - apply",
			existingValue: "",
			glossaryValue: "Cost-Benefit Analysis",
			expectedCount: 1,
			expectedValue: "Cost-Benefit Analysis",
			description:   "Empty existing value should be filled",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Setup missing translation that will match glossary
			missing := []types.MissingTranslation{
				{
					SourceLocale: "en",
					TargetLocale: "fr",
					SourceValue:  "Cost-Benefit Analysis", // Source in English
					Path:         []string{"term"},
				},
			}

			localeFiles := map[types.Locale]*types.LocaleFile{
				"en": {
					Path:   filepath.Join(tempDir, "en.json"),
					Locale: "en",
					Contents: map[string]any{
						"term": "Cost-Benefit Analysis",
					},
				},
				"fr": {
					Path:     filepath.Join(tempDir, "fr.json"),
					Locale:   "fr",
					Contents: map[string]any{},
				},
			}

			// Add existing value if not empty
			if tc.existingValue != "" {
				localeFiles["fr"].Contents["term"] = tc.existingValue
			}

			// Config with glossary
			cfg := &config.Config{} //nolint:exhaustruct // Minimal config for testing
			cfg.Glossary.AutoSort = false
			cfg.Glossary.Entries = []types.GlossaryEntry{
				{"en": "Cost-Benefit Analysis", "fr": tc.glossaryValue},
			}

			count, err := sourced.ApplySourced(ctx, missing, localeFiles, "en", cfg, true, false)
			if err != nil {
				t.Fatalf("ApplySourced failed: %v", err)
			}

			// Verify count matches expectation
			if count != tc.expectedCount {
				t.Errorf("Expected %d applied translations, got %d (%s)",
					tc.expectedCount, count, tc.description)
			}

			// Verify the final value
			finalValue, exists := localeFiles["fr"].Contents["term"]
			if !exists && tc.expectedValue != "" {
				t.Fatalf("Expected 'term' to exist with value %q, but it doesn't exist", tc.expectedValue)
			}
			if exists && finalValue != tc.expectedValue {
				t.Errorf("Expected final value %q, got %q (%s)",
					tc.expectedValue, finalValue, tc.description)
			}
		})
	}
}

// TestApplySourced_PreserveCasing_NoOverwrite verifies that case-insensitive filtering
// preserves existing casing when glossary values differ only in case.
// This test exercises the apply-layer casing preservation without using the overwrite flag.
func TestApplySourced_PreserveCasing_NoOverwrite(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Setup: Existing translation with specific casing
	missing := []types.MissingTranslation{
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "Hello",
			Path:         []string{"greeting"},
		},
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "World",
			Path:         []string{"world"},
		},
	}

	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {
			Path:   filepath.Join(tempDir, "en.json"),
			Locale: "en",
			Contents: map[string]any{
				"greeting": "Hello",
				"world":    "World",
			},
		},
		"fr": {
			Path:   filepath.Join(tempDir, "fr.json"),
			Locale: "fr",
			Contents: map[string]any{
				"greeting": "SALUT",    // Already has it, but different case
				"world":    "Le Monde", // Different content, should update
			},
		},
	}

	// Config with glossary
	cfg := &config.Config{} //nolint:exhaustruct // Minimal config for testing
	cfg.Glossary.AutoSort = false
	cfg.Glossary.Entries = []types.GlossaryEntry{
		{"en": "Hello", "fr": "salut"},    // lowercase version
		{"en": "World", "fr": "Le Monde"}, // Same value already exists
	}

	count, err := sourced.ApplySourced(ctx, missing, localeFiles, "en", cfg, true, false)
	if err != nil {
		t.Fatalf("ApplySourced failed: %v", err)
	}

	// Should skip "greeting" (case-only diff) but not "world" (exact match)
	// Both should be skipped actually - "greeting" for case-only difference,
	// "world" for exact match
	if count != 0 {
		t.Errorf("Expected 0 applied translations (both should be skipped), got %d", count)
	}

	// Verify original casing is preserved
	if val := localeFiles["fr"].Contents["greeting"]; val != "SALUT" {
		t.Errorf("Expected casing to be preserved: %q, got %q", "SALUT", val)
	}

	// Verify exact match is preserved
	if val := localeFiles["fr"].Contents["world"]; val != "Le Monde" {
		t.Errorf("Expected exact match to be preserved: %q, got %q", "Le Monde", val)
	}
}

// TestApplySourced_DeduplicationPreservesGlossary verifies that when glossary and memory
// both provide translations for the same key, deduplication keeps glossary (first occurrence).
//
//nolint:funlen // Comprehensive deduplication test is better kept together
func TestApplySourced_DeduplicationPreservesGlossary(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Setup: Multiple missing translations where both glossary and memory match
	missing := []types.MissingTranslation{
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "Hello",
			Path:         []string{"greeting1"},
		},
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "Hello",
			Path:         []string{"greeting2"},
		},
		{
			SourceLocale: "en",
			TargetLocale: "fr",
			SourceValue:  "World",
			Path:         []string{"world"},
		},
	}

	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {
			Path:   filepath.Join(tempDir, "en.json"),
			Locale: "en",
			Contents: map[string]any{
				"greeting1": "Hello",
				"greeting2": "Hello",
				"world":     "World",
				"other1":    "Hello", // Memory source
				"other2":    "World", // Memory source
			},
		},
		"fr": {
			Path:   filepath.Join(tempDir, "fr.json"),
			Locale: "fr",
			Contents: map[string]any{
				"other1": "Bonjour",  // Memory: Hello → Bonjour
				"other2": "Le Monde", // Memory: World → Le Monde
			},
		},
	}

	// Config with glossary that conflicts with memory
	cfg := &config.Config{} //nolint:exhaustruct // Minimal config for testing
	cfg.Glossary.AutoSort = false
	cfg.Glossary.Entries = []types.GlossaryEntry{
		{"en": "Hello", "fr": frSalut},    // Glossary: Hello → Salut (not Bonjour)
		{"en": "World", "fr": "Le Monde"}, // Same as memory
	}

	// Call with both enabled
	count, err := sourced.ApplySourced(ctx, missing, localeFiles, "en", cfg, true, true)
	if err != nil {
		t.Fatalf("ApplySourced failed: %v", err)
	}

	// Should find 3 matches
	if count != 3 {
		t.Errorf("Expected 3 applied translations, got %d", count)
	}

	// Verify both "Hello" translations use glossary (Salut), not memory (Bonjour)
	if val := localeFiles["fr"].Contents["greeting1"]; val != frSalut {
		t.Errorf("greeting1: expected %q from glossary, got %v", frSalut, val)
	}
	if val := localeFiles["fr"].Contents["greeting2"]; val != frSalut {
		t.Errorf("greeting2: expected %q from glossary, got %v", frSalut, val)
	}

	// Verify "World" translation was applied (casing may be transformed)
	worldVal, worldExists := localeFiles["fr"].Contents["world"]
	if !worldExists {
		t.Error("world: expected translation to exist")
	}
	// Accept either "Le Monde" or "Le monde" due to casing transformation
	if worldVal != "Le Monde" && worldVal != "Le monde" {
		t.Errorf("world: expected 'Le Monde' or 'Le monde', got %v", worldVal)
	}

	t.Logf("Successfully deduplicated: glossary took precedence for 'Hello' over memory")
}
