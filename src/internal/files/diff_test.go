package files_test

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

func TestIdentifyMissingKeysBasic(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"auth": map[string]any{
					"login":    "Login",
					"password": "Password",
				},
				"common": "Common",
			},
		},
		"fr-FR": {
			Locale:     "fr-FR",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"auth": map[string]any{
					"login": "Connexion",
				},
			},
		},
	}

	ctx := context.Background()
	result := files.IdentifyMissingKeys(ctx, localeFiles, false)

	expected := []types.MissingTranslation{
		{
			Path:         []string{"auth", "password"},
			SourceLocale: "en-US",
			SourceValue:  "Password",
			TargetLocale: "fr-FR",
		},
		{
			Path:         []string{"common"},
			SourceLocale: "en-US",
			SourceValue:  "Common",
			TargetLocale: "fr-FR",
		},
	}

	assertMissingTranslations(t, result, expected)
}

func TestIdentifyMissingKeysObjectLevel(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"dashboard": map[string]any{
					"title": "Dashboard",
					"stats": map[string]any{
						"users":  "Active Users",
						"growth": "Growth Rate",
					},
				},
				"existing": "Already there",
			},
		},
		"sv-SE": {
			Locale: "sv-SE",
			Contents: map[string]any{
				"existing": "Redan där",
			},
		},
	}

	ctx := context.Background()
	result := files.IdentifyMissingKeys(ctx, localeFiles, false)

	expected := []types.MissingTranslation{
		{
			Path:         []string{"dashboard"},
			SourceLocale: "en-US",
			SourceValue: map[string]any{
				"title": "Dashboard",
				"stats": map[string]any{
					"users":  "Active Users",
					"growth": "Growth Rate",
				},
			},
			TargetLocale: "sv-SE",
		},
	}

	assertMissingTranslations(t, result, expected)
}

func TestIdentifyMissingKeysStructureMismatch(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"auth": map[string]any{
					"login": map[string]any{
						"title": "Sign In",
						"email": "Email",
					},
				},
			},
		},
		"es-ES": {
			Locale: "es-ES",
			Contents: map[string]any{
				"auth": map[string]any{
					"login": "Iniciar Sesión",
				},
			},
		},
	}

	ctx := context.Background()
	result := files.IdentifyMissingKeys(ctx, localeFiles, false)

	expected := []types.MissingTranslation{
		{
			Path:         []string{"auth", "login"},
			SourceLocale: "en-US",
			SourceValue: map[string]any{
				"title": "Sign In",
				"email": "Email",
			},
			TargetLocale: "es-ES",
		},
		{
			Path:         []string{"auth", "login"},
			SourceLocale: "es-ES",
			SourceValue:  "Iniciar Sesión",
			TargetLocale: "en-US",
		},
	}

	assertMissingTranslations(t, result, expected)
}

func TestIdentifyMissingKeysReferenceSelection(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"partial": {
			Locale:     "partial",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"key1": "value1",
			},
		},
		"complete": {
			Locale:     "complete",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		},
		"target": {
			Locale: "target",
			Contents: map[string]any{
				"key1": "translated1",
			},
		},
	}

	ctx := context.Background()
	result := files.IdentifyMissingKeys(ctx, localeFiles, false)

	expected := []types.MissingTranslation{
		{
			Path:         []string{"key2"},
			SourceLocale: "complete",
			SourceValue:  "value2",
			TargetLocale: "target",
		},
		{
			Path:         []string{"key3"},
			SourceLocale: "complete",
			SourceValue:  "value3",
			TargetLocale: "target",
		},
		{
			Path:         []string{"key2"},
			SourceLocale: "complete",
			SourceValue:  "value2",
			TargetLocale: "partial",
		},
		{
			Path:         []string{"key3"},
			SourceLocale: "complete",
			SourceValue:  "value3",
			TargetLocale: "partial",
		},
	}

	assertMissingTranslations(t, result, expected)
}

func TestIdentifyMissingKeysEmptyTarget(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
		},
		"empty": {
			Locale:     "empty",
			LineEnding: utils.LineEndingLF,
			Contents:   map[string]any{},
		},
	}

	ctx := context.Background()
	result := files.IdentifyMissingKeys(ctx, localeFiles, false)

	expected := []types.MissingTranslation{
		{
			Path:         []string{"key1"},
			SourceLocale: "en-US",
			SourceValue:  "value1",
			TargetLocale: "empty",
		},
		{
			Path:         []string{"key2"},
			SourceLocale: "en-US",
			SourceValue:  "value2",
			TargetLocale: "empty",
		},
	}

	assertMissingTranslations(t, result, expected)
}

func TestIdentifyMissingKeysComplete(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
		},
		"fr-FR": {
			Locale:     "fr-FR",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"key1": "valeur1",
				"key2": "valeur2",
			},
		},
	}

	ctx := context.Background()
	result := files.IdentifyMissingKeys(ctx, localeFiles, false)

	expected := []types.MissingTranslation{}

	assertMissingTranslations(t, result, expected)
}

func TestIdentifyMissingKeysDeepNesting(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": map[string]any{
							"deep": "Deep Value",
						},
					},
				},
			},
		},
		"fr-FR": {
			Locale:     "fr-FR",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{},
				},
			},
		},
	}

	ctx := context.Background()
	result := files.IdentifyMissingKeys(ctx, localeFiles, false)

	expected := []types.MissingTranslation{
		{
			Path:         []string{"level1", "level2", "level3"},
			SourceLocale: "en-US",
			SourceValue: map[string]any{
				"deep": "Deep Value",
			},
			TargetLocale: "fr-FR",
		},
	}

	assertMissingTranslations(t, result, expected)
}

func TestIdentifyMissingKeysFullObject(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"dashboard": map[string]any{
					"title": "Dashboard",
					"nested": map[string]any{
						"deep": map[string]any{
							"value": "Deep Value",
						},
					},
				},
			},
		},
		"fr-FR": {
			Locale:   "fr-FR",
			Contents: map[string]any{},
		},
	}

	ctx := context.Background()
	result := files.IdentifyMissingKeys(ctx, localeFiles, false)

	expected := []types.MissingTranslation{
		{
			Path:         []string{"dashboard"},
			SourceLocale: "en-US",
			SourceValue: map[string]any{
				"title": "Dashboard",
				"nested": map[string]any{
					"deep": map[string]any{
						"value": "Deep Value",
					},
				},
			},
			TargetLocale: "fr-FR",
		},
	}

	assertMissingTranslations(t, result, expected)
}

func TestIdentifyMissingKeysContextCancellation(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
		},
		"fr-FR": {
			Locale:   "fr-FR",
			Contents: map[string]any{},
		},
	}

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := files.IdentifyMissingKeys(ctx, localeFiles, false)

	// Should return nil/empty when context is canceled
	if len(result) > 0 {
		t.Errorf("expected no results when context is cancelled, got %d results", len(result))
	}
}

// Helper functions.
func assertMissingTranslations(t *testing.T, result, expected []types.MissingTranslation) {
	t.Helper()

	keyFn := func(m types.MissingTranslation) string {
		return strings.Join(m.Path, ".") + "|" + string(m.SourceLocale) + "|" + string(m.TargetLocale)
	}

	resultMap := make(map[string]types.MissingTranslation)
	for _, r := range result {
		key := keyFn(r)
		resultMap[key] = r
	}

	expectedMap := make(map[string]types.MissingTranslation)
	for _, e := range expected {
		key := keyFn(e)
		expectedMap[key] = e
	}

	if len(resultMap) != len(expectedMap) {
		t.Errorf("expected %d missing translations, got %d", len(expectedMap), len(resultMap))
		t.Logf("Expected keys: %v", getKeys(expectedMap))
		t.Logf("Actual keys: %v", getKeys(resultMap))
	}

	for key, expected := range expectedMap {
		result, exists := resultMap[key]
		if !exists {
			t.Errorf("missing expected translation: %s", key)
			continue
		}

		if !reflect.DeepEqual(result.Path, expected.Path) {
			t.Errorf("path mismatch for %s: expected %v, got %v", key, expected.Path, result.Path)
		}

		if !reflect.DeepEqual(result.SourceValue, expected.SourceValue) {
			t.Errorf("source value mismatch for %s: expected %v, got %v", key, expected.SourceValue, result.SourceValue)
		}
	}
}

func getKeys(m map[string]types.MissingTranslation) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func verifyTieBreaking(
	t *testing.T, result []types.MissingTranslation, path, targetLocale, expectedSource, expectedValue string,
) {
	t.Helper()

	var found types.MissingTranslation
	foundMatch := false
	for _, mt := range result {
		if strings.Join(mt.Path, ".") == path && string(mt.TargetLocale) == targetLocale {
			found = mt
			foundMatch = true
			break
		}
	}
	if !foundMatch {
		t.Fatalf("Expected to find missing translation for %s|%s in result", path, targetLocale)
	}
	if string(found.SourceLocale) != expectedSource {
		t.Errorf("Expected '%s' to be chosen as source for %s|%s due to lexicographic tie-break, got %s",
			expectedSource, path, targetLocale, found.SourceLocale)
	}
	if sv, ok := found.SourceValue.(string); !ok || sv != expectedValue {
		t.Errorf("Expected SourceValue for %s|%s to be '%s' from %s, got %#v",
			path, targetLocale, expectedValue, expectedSource, found.SourceValue)
	}
}

func TestIdentifyMissingKeysWithStubs_BasicStubs(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"auth": map[string]any{
					"login":    "Login",
					"password": "Password",
				},
				"common": "Common text",
			},
		},
		"fr-FR": {
			Locale:     "fr-FR",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"auth": map[string]any{
					"login":    "Connexion",
					"password": "", // Empty stub
				},
				"common": "", // Empty stub
			},
		},
	}

	ctx := context.Background()

	// Test normal mode - should NOT detect empty strings as missing
	normalResult := files.IdentifyMissingKeys(ctx, localeFiles, false)
	if len(normalResult) != 0 {
		t.Errorf("Normal mode should not detect stubs as missing, got %d results", len(normalResult))
	}

	// Test stub mode - should detect empty strings as missing
	stubResult := files.IdentifyMissingKeysWithStubs(ctx, localeFiles, false)
	expected := []types.MissingTranslation{
		{
			Path:         []string{"auth", "password"},
			SourceLocale: "en-US",
			SourceValue:  "Password",
			TargetLocale: "fr-FR",
		},
		{
			Path:         []string{"common"},
			SourceLocale: "en-US",
			SourceValue:  "Common text",
			TargetLocale: "fr-FR",
		},
	}

	assertMissingTranslations(t, stubResult, expected)
}

func TestIdentifyMissingKeysWithStubs_ReferenceSelection(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"incomplete": {
			Locale:     "incomplete",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"key1": "value1",
				"key2": "", // Stub
			},
		},
		"complete": {
			Locale:     "complete",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"key1": "complete1",
				"key2": "complete2",
				"key3": "complete3",
			},
		},
		"target": {
			Locale: "target",
			Contents: map[string]any{
				"key1": "target1",
				"key2": "", // Stub
			},
		},
	}

	ctx := context.Background()
	result := files.IdentifyMissingKeysWithStubs(ctx, localeFiles, false)

	// All missing translations should use 'complete' as the source in this setup
	for _, translation := range result {
		if translation.SourceLocale != "complete" {
			t.Errorf("Expected all sources to be 'complete', got %s for key %s",
				translation.SourceLocale, strings.Join(translation.Path, "."))
		}
	}

	// Should detect stubs as missing translations with deduplication
	// With bi-directional comparison and deduplication:
	// - incomplete: key3 missing, key2 is stub (2 unique target keys)
	// - target: key3 missing, key2 is stub (2 unique target keys)
	// Total: 4 missing translations (no duplicates for same target key)
	expectedCount := 4
	if len(result) != expectedCount {
		t.Errorf("Expected %d missing translations, got %d", expectedCount, len(result))
	}
}

func TestIdentifyMissingKeysNoDuplicates(t *testing.T) {
	// Test that when multiple locales can provide the same missing key,
	// only one translation is returned (no duplicates)
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"shared":  "English value",
				"only_en": "English only",
			},
		},
		"da-DK": {
			Locale:     "da-DK",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"shared":  "Danish value",
				"only_da": "Danish only",
			},
		},
		"sv-SE": {
			Locale:   "sv-SE",
			Contents: map[string]any{
				// Missing both "shared" and locale-specific keys
			},
		},
	}

	ctx := context.Background()
	result := files.IdentifyMissingKeys(ctx, localeFiles, false)

	// Count how many times each target key appears
	targetKeyCounts := make(map[string]int)
	for _, translation := range result {
		key := strings.Join(translation.Path, ".") + "|" + string(translation.TargetLocale)
		targetKeyCounts[key]++
	}

	// Verify no duplicates - each target key should appear exactly once
	for targetKey, count := range targetKeyCounts {
		if count != 1 {
			t.Errorf("Target key %s appears %d times, expected 1 (no duplicates)", targetKey, count)
		}
	}

	// Bi-directional algorithm finds all missing keys between all locale pairs:
	// - en-US → da-DK: only_en
	// - en-US → sv-SE: only_en, shared
	// - da-DK → en-US: only_da
	// - da-DK → sv-SE: only_da, shared (but shared is deduplicated)
	// Total: 5 unique missing translations (no duplicates for same target key)
	expectedCount := 5
	if len(result) != expectedCount {
		t.Errorf("Expected %d missing translations, got %d", expectedCount, len(result))
		for i, translation := range result {
			t.Errorf("  [%d] %s → %s: %s", i, translation.SourceLocale, translation.TargetLocale,
				strings.Join(translation.Path, "."))
		}
	}

	// Verify tie-breaking: da-DK should win over en-US for shared|sv-SE
	verifyTieBreaking(t, result, "shared", "sv-SE", "da-DK", "Danish value")
}

func TestIdentifyMissingKeysWithStubs_MixedContent(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"existing": "Existing text",
				"stubbed":  "Real content",
				"missing":  "Also real",
			},
		},
		"fr-FR": {
			Locale:     "fr-FR",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"existing": "Texte existant",
				"stubbed":  "", // Empty stub
				// "missing" key completely absent
			},
		},
	}

	ctx := context.Background()
	result := files.IdentifyMissingKeysWithStubs(ctx, localeFiles, false)

	expected := []types.MissingTranslation{
		{
			Path:         []string{"stubbed"},
			SourceLocale: "en-US",
			SourceValue:  "Real content",
			TargetLocale: "fr-FR",
		},
		{
			Path:         []string{"missing"},
			SourceLocale: "en-US",
			SourceValue:  "Also real",
			TargetLocale: "fr-FR",
		},
	}

	assertMissingTranslations(t, result, expected)
}

func TestIdentifyMissingKeysWithStubs_NestedStubs(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"auth": map[string]any{
					"login": map[string]any{
						"title":    "Sign In",
						"email":    "Email Address",
						"password": "Password",
					},
					"logout": "Sign Out",
				},
				"dashboard": "Dashboard",
			},
		},
		"fr-FR": {
			Locale:     "fr-FR",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"auth": map[string]any{
					"login": map[string]any{
						"title":    "Connexion",
						"email":    "", // Nested stub
						"password": "", // Nested stub
					},
					"logout": "Déconnexion",
				},
				"dashboard": "", // Top-level stub
			},
		},
	}

	ctx := context.Background()
	result := files.IdentifyMissingKeysWithStubs(ctx, localeFiles, false)

	expected := []types.MissingTranslation{
		{
			Path:         []string{"auth", "login", "email"},
			SourceLocale: "en-US",
			SourceValue:  "Email Address",
			TargetLocale: "fr-FR",
		},
		{
			Path:         []string{"auth", "login", "password"},
			SourceLocale: "en-US",
			SourceValue:  "Password",
			TargetLocale: "fr-FR",
		},
		{
			Path:         []string{"dashboard"},
			SourceLocale: "en-US",
			SourceValue:  "Dashboard",
			TargetLocale: "fr-FR",
		},
	}

	assertMissingTranslations(t, result, expected)
}

// TestIdentifyMissingKeysLexicographicTieBreaking tests the tie-breaking logic
// when multiple source locales have equal completeness scores.
func TestIdentifyMissingKeysLexicographicTieBreaking(t *testing.T) {
	// Set up locales with identical key counts to test lexicographic tie-breaking
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"key1": "English Value 1",
				"key2": "English Value 2",
				"key3": "English Value 3",
			},
		},
		"da-DK": {
			Locale:     "da-DK",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"key1": "Danish Value 1",
				"key2": "Danish Value 2",
				"key3": "Danish Value 3",
			},
		},
		"sv-SE": {
			Locale:   "sv-SE",
			Contents: map[string]any{
				// Missing all keys - this will be our target locale
			},
		},
	}

	ctx := context.Background()
	result := files.IdentifyMissingKeys(ctx, localeFiles, false)

	// Both en-US and da-DK have 3 keys (equal completeness)
	// Due to lexicographic ordering, da-DK should be preferred over en-US
	// since "da-DK" < "en-US" alphabetically
	expected := []types.MissingTranslation{
		{
			Path:         []string{"key1"},
			SourceLocale: "da-DK", // da-DK wins tie-break over en-US
			SourceValue:  "Danish Value 1",
			TargetLocale: "sv-SE",
		},
		{
			Path:         []string{"key2"},
			SourceLocale: "da-DK", // da-DK wins tie-break over en-US
			SourceValue:  "Danish Value 2",
			TargetLocale: "sv-SE",
		},
		{
			Path:         []string{"key3"},
			SourceLocale: "da-DK", // da-DK wins tie-break over en-US
			SourceValue:  "Danish Value 3",
			TargetLocale: "sv-SE",
		},
	}

	assertMissingTranslations(t, result, expected)

	// Additional verification: ensure all translations use da-DK as source
	for _, translation := range result {
		if translation.SourceLocale != "da-DK" {
			t.Errorf("Expected da-DK to win lexicographic tie-break, but got source locale: %s", translation.SourceLocale)
		}
	}
}

// TestIdentifyMissingKeys_OverwriteMode tests that overwrite mode treats existing translations as missing.
func TestIdentifyMissingKeys_OverwriteMode(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"greeting": "Hello",
				"farewell": "Goodbye",
			},
		},
		"fr-FR": {
			Locale:     "fr-FR",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"greeting": "Bonjour", // Existing translation
				// "farewell" is missing
			},
		},
	}

	ctx := context.Background()

	// Normal mode (overwrite=false) - only finds missing keys
	normalResult := files.IdentifyMissingKeys(ctx, localeFiles, false)
	expectedNormal := []types.MissingTranslation{
		{
			Path:         []string{"farewell"},
			SourceLocale: "en-US",
			SourceValue:  "Goodbye",
			TargetLocale: "fr-FR",
		},
	}
	assertMissingTranslations(t, normalResult, expectedNormal)

	// Overwrite mode (overwrite=true) - treats existing as missing (bidirectional)
	overwriteResult := files.IdentifyMissingKeys(ctx, localeFiles, true)
	expectedOverwrite := []types.MissingTranslation{
		{
			Path:         []string{"greeting"},
			SourceLocale: "en-US",
			SourceValue:  "Hello",
			TargetLocale: "fr-FR",
		},
		{
			Path:         []string{"greeting"},
			SourceLocale: "fr-FR",
			SourceValue:  "Bonjour",
			TargetLocale: "en-US",
		},
		{
			Path:         []string{"farewell"},
			SourceLocale: "en-US",
			SourceValue:  "Goodbye",
			TargetLocale: "fr-FR",
		},
	}
	assertMissingTranslations(t, overwriteResult, expectedOverwrite)
}

// TestIdentifyMissingKeys_OverwriteWithNestedStructures tests overwrite mode with nested objects.
func TestIdentifyMissingKeys_OverwriteWithNestedStructures(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"auth": map[string]any{
					"login":    "Login",
					"password": "Password",
					"confirm":  "Confirm",
				},
			},
		},
		"fr-FR": {
			Locale:     "fr-FR",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"auth": map[string]any{
					"login":    "Connexion",    // Existing
					"password": "Mot de passe", // Existing
					// "confirm" is missing
				},
			},
		},
	}

	ctx := context.Background()

	// Overwrite mode - should find all nested keys including existing ones (bidirectional)
	overwriteResult := files.IdentifyMissingKeys(ctx, localeFiles, true)
	expectedOverwrite := []types.MissingTranslation{
		{
			Path:         []string{"auth", "login"},
			SourceLocale: "en-US",
			SourceValue:  "Login",
			TargetLocale: "fr-FR",
		},
		{
			Path:         []string{"auth", "login"},
			SourceLocale: "fr-FR",
			SourceValue:  "Connexion",
			TargetLocale: "en-US",
		},
		{
			Path:         []string{"auth", "password"},
			SourceLocale: "en-US",
			SourceValue:  "Password",
			TargetLocale: "fr-FR",
		},
		{
			Path:         []string{"auth", "password"},
			SourceLocale: "fr-FR",
			SourceValue:  "Mot de passe",
			TargetLocale: "en-US",
		},
		{
			Path:         []string{"auth", "confirm"},
			SourceLocale: "en-US",
			SourceValue:  "Confirm",
			TargetLocale: "fr-FR",
		},
	}
	assertMissingTranslations(t, overwriteResult, expectedOverwrite)
}

// TestIdentifyMissingKeys_OverwriteWithStructureMismatch tests overwrite mode when structure types differ.
func TestIdentifyMissingKeys_OverwriteWithStructureMismatch(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"item": "Simple String",
			},
		},
		"fr-FR": {
			Locale:     "fr-FR",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"item": map[string]any{
					"nested": "Nested Value",
				},
			},
		},
	}

	ctx := context.Background()

	// Overwrite mode with structure mismatch - should detect both directions
	overwriteResult := files.IdentifyMissingKeys(ctx, localeFiles, true)

	// Should find bidirectional conflicts since structures don't match
	if len(overwriteResult) != 2 {
		t.Errorf("Expected 2 missing translations for structure mismatch in overwrite mode, got %d", len(overwriteResult))
	}

	// Verify we get conflicts in both directions
	hasEnToFr := false
	hasFrToEn := false
	for _, mt := range overwriteResult {
		if mt.SourceLocale == "en-US" && mt.TargetLocale == "fr-FR" {
			hasEnToFr = true
		}
		if mt.SourceLocale == "fr-FR" && mt.TargetLocale == "en-US" {
			hasFrToEn = true
		}
	}

	if !hasEnToFr || !hasFrToEn {
		t.Errorf("Expected bidirectional structure mismatches, got hasEnToFr=%v, hasFrToEn=%v", hasEnToFr, hasFrToEn)
	}
}

// TestIdentifyMissingKeys_OverwriteModeEmpty tests overwrite mode with empty target.
func TestIdentifyMissingKeys_OverwriteModeEmpty(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
		},
		"fr-FR": {
			Locale:     "fr-FR",
			LineEnding: utils.LineEndingLF,
			Contents:   map[string]any{},
		},
	}

	ctx := context.Background()

	// Both normal and overwrite mode should behave the same with empty target
	normalResult := files.IdentifyMissingKeys(ctx, localeFiles, false)
	overwriteResult := files.IdentifyMissingKeys(ctx, localeFiles, true)

	if len(normalResult) != len(overwriteResult) {
		t.Errorf("Expected same results for empty target, got normal=%d, overwrite=%d",
			len(normalResult), len(overwriteResult))
	}

	assertMissingTranslations(t, overwriteResult, normalResult)
}

// TestIdentifyMissingKeysWithStubs_OverwriteMode tests overwrite mode with stubs included.
func TestIdentifyMissingKeysWithStubs_OverwriteMode(t *testing.T) {
	localeFiles := map[types.Locale]*types.LocaleFile{
		"en-US": {
			Locale:     "en-US",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"existing": "Existing",
				"stubbed":  "Content",
			},
		},
		"fr-FR": {
			Locale:     "fr-FR",
			LineEnding: utils.LineEndingLF,
			Contents: map[string]any{
				"existing": "Présent", // Real translation
				"stubbed":  "",        // Empty stub
			},
		},
	}

	ctx := context.Background()

	// Overwrite mode with stubs - should find both existing and stubs (bidirectional)
	overwriteResult := files.IdentifyMissingKeysWithStubs(ctx, localeFiles, true)
	expected := []types.MissingTranslation{
		{
			Path:         []string{"existing"},
			SourceLocale: "en-US",
			SourceValue:  "Existing",
			TargetLocale: "fr-FR",
		},
		{
			Path:         []string{"existing"},
			SourceLocale: "fr-FR",
			SourceValue:  "Présent",
			TargetLocale: "en-US",
		},
		{
			Path:         []string{"stubbed"},
			SourceLocale: "en-US",
			SourceValue:  "Content",
			TargetLocale: "fr-FR",
		},
		{
			Path:         []string{"stubbed"},
			SourceLocale: "fr-FR",
			SourceValue:  "",
			TargetLocale: "en-US",
		},
	}
	assertMissingTranslations(t, overwriteResult, expected)
}
