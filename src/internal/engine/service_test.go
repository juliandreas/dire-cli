package engine_test

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/engine"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/flags"
	"github.com/juliandreas/dire/internal/translator"
	"github.com/juliandreas/dire/internal/types"
)

// translatorSpy tracks whether the translator was called.
type translatorSpy struct {
	called bool
}

// newSpyTranslatorClient creates a translator client with a spy provider function.
func newSpyTranslatorClient(t *testing.T, cfg *config.Config, spy *translatorSpy) *translator.Client {
	t.Helper()

	// Create a fake provider function that tracks calls
	providerFn := func(
		ctx context.Context,
		httpClient *http.Client,
		provider config.Provider,
		apiKey string,
		texts []string,
		sourceLang, targetLang string,
		formality constants.Formality,
	) ([]string, error) {
		spy.called = true
		// Return fake translations (same as input)
		return texts, nil
	}

	provider := config.Provider{Name: "test-spy"} //nolint:exhaustruct // Minimal provider for testing
	client, err := translator.NewClient(cfg, provider, "fake-key", providerFn)
	if err != nil {
		t.Fatalf("translator.NewClient failed: %v", err)
	}
	return client
}

// TestService_PreSourcingInNormalMode verifies that glossary and memory are applied by default
// in normal translation mode (no flags). This test would have caught the bug where pre-sourcing
// was only enabled when explicit flags were set.
//
//nolint:funlen // Integration test is clearer kept together
func TestService_PreSourcingInNormalMode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tempDir := t.TempDir()

	// Setup: Config with glossary entry
	cfg := &config.Config{} //nolint:exhaustruct // Minimal config for testing
	cfg.Glossary.AutoSort = false
	cfg.Glossary.Entries = []types.GlossaryEntry{
		{"en": "Hello", "fr": "Salut"},
	}
	cfg.Files.Directory = tempDir
	cfg.Files.Locales = map[string]string{
		"en.json": "en",
		"fr.json": "fr",
	}
	cfg.Files.Indent = 2

	// Setup: Flags with NO sourced-only modes (normal translation mode)
	flagsInstance := &flags.Flags{ //nolint:exhaustruct // Minimal flags for testing
		TrimStrings:  true,
		StubMode:     false,
		SourcedOnly:  false,
		GlossaryOnly: false,
		MemoryOnly:   false,
	}

	// Setup: Locale files with missing translation that matches glossary
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

	// Create repository
	repository := files.NewRepository(cfg, flagsInstance)

	// Create service (no LLM/translator clients needed for pre-sourcing test)
	service := engine.NewService(nil, nil, cfg, flagsInstance, 4096, repository)

	// Run service
	metrics, err := service.Run(ctx, localeFiles, missing, "en", nil)
	if err != nil {
		t.Fatalf("Service.Run failed: %v", err)
	}

	// CRITICAL ASSERTION: Pre-sourcing should have applied the glossary match
	if metrics.SourcedTranslatedKeys == 0 {
		t.Error("BUG: Pre-sourcing did not apply in normal mode! Expected glossary match to be applied.")
	}

	// Verify the translation was actually written
	if val, ok := localeFiles["fr"].Contents["greeting"]; !ok || val != "Salut" {
		t.Errorf("Expected 'Salut' from glossary, got %v (exists=%v)", val, ok)
	}
}

// TestService_SourcedFlagCombinations tests how different flag combinations affect
// which translation sources are enabled.
//
//nolint:funlen,gocyclo // Table-driven test with comprehensive assertions
func TestService_SourcedFlagCombinations(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                  string
		sourcedOnly           bool
		glossaryOnly          bool
		memoryOnly            bool
		expectGlossaryEnabled bool
		expectMemoryEnabled   bool
		expectSkipAI          bool
		description           string
	}{
		{
			name:                  "normal mode - both sources enabled",
			sourcedOnly:           false,
			glossaryOnly:          false,
			memoryOnly:            false,
			expectGlossaryEnabled: true,
			expectMemoryEnabled:   true,
			expectSkipAI:          false,
			description:           "In normal mode, both glossary and memory should be used for pre-sourcing",
		},
		{
			name:                  "sourced flag - both sources enabled, skip AI",
			sourcedOnly:           true,
			glossaryOnly:          false,
			memoryOnly:            false,
			expectGlossaryEnabled: true,
			expectMemoryEnabled:   true,
			expectSkipAI:          true,
			description:           "With --sourced, both sources enabled and AI skipped",
		},
		{
			name:                  "glossary flag - only glossary enabled, skip AI",
			sourcedOnly:           false,
			glossaryOnly:          true,
			memoryOnly:            false,
			expectGlossaryEnabled: true,
			expectMemoryEnabled:   false,
			expectSkipAI:          true,
			description:           "With --glossary, only glossary enabled and AI skipped",
		},
		{
			name:                  "memory flag - only memory enabled, skip AI",
			sourcedOnly:           false,
			glossaryOnly:          false,
			memoryOnly:            true,
			expectGlossaryEnabled: false,
			expectMemoryEnabled:   true,
			expectSkipAI:          true,
			description:           "With --memory, only memory enabled and AI skipped",
		},
		{
			name:                  "glossary and memory flags - both enabled, skip AI",
			sourcedOnly:           false,
			glossaryOnly:          true,
			memoryOnly:            true,
			expectGlossaryEnabled: true,
			expectMemoryEnabled:   true,
			expectSkipAI:          true,
			description:           "With --glossary --memory, both enabled and AI skipped (equivalent to --sourced)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			tempDir := t.TempDir()

			// Setup config with both glossary and memory available
			cfg := &config.Config{} //nolint:exhaustruct // Minimal config for testing
			cfg.Glossary.AutoSort = false
			cfg.Glossary.Entries = []types.GlossaryEntry{
				{"en": "Hello", "fr": "Salut"},
			}
			cfg.Files.Directory = tempDir
			cfg.Files.Locales = map[string]string{
				"en.json": "en",
				"fr.json": "fr",
			}
			cfg.Files.Indent = 2

			// Setup flags
			flagsInstance := &flags.Flags{ //nolint:exhaustruct // Minimal flags for testing
				TrimStrings:  true,
				StubMode:     false,
				SourcedOnly:  tc.sourcedOnly,
				GlossaryOnly: tc.glossaryOnly,
				MemoryOnly:   tc.memoryOnly,
			}

			// Setup locale files with:
			// 1. Missing translation that matches glossary: "Hello"
			// 2. Missing translation that matches memory: "Goodbye"
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
						"other":    "Goodbye", // Memory source for "Goodbye"
					},
				},
				"fr": {
					Path:   filepath.Join(tempDir, "fr.json"),
					Locale: "fr",
					Contents: map[string]any{
						"other": "Au revoir", // Memory match for "Goodbye"
					},
				},
			}

			// Create spy to track if translator was called
			spy := &translatorSpy{} //nolint:exhaustruct // Zero value is desired
			translatorClient := newSpyTranslatorClient(t, cfg, spy)

			// Create repository and service with spy client
			repository := files.NewRepository(cfg, flagsInstance)
			service := engine.NewService(nil, translatorClient, cfg, flagsInstance, 4096, repository)

			// Run service
			metrics, err := service.Run(ctx, localeFiles, missing, "en", nil)
			if err != nil {
				t.Fatalf("Service.Run failed: %v", err)
			}

			// Calculate expected sourced count
			expectedSourcedCount := 0
			if tc.expectGlossaryEnabled {
				expectedSourcedCount++ // "Hello" from glossary
			}
			if tc.expectMemoryEnabled {
				expectedSourcedCount++ // "Goodbye" from memory
			}

			// Assert sourced count matches expectations
			if metrics.SourcedTranslatedKeys != expectedSourcedCount {
				t.Errorf("%s: Expected %d sourced translations, got %d",
					tc.description, expectedSourcedCount, metrics.SourcedTranslatedKeys)
			}

			// Verify glossary was applied (or not)
			if tc.expectGlossaryEnabled {
				if val := localeFiles["fr"].Contents["greeting"]; val != "Salut" {
					t.Errorf("Expected glossary to be applied, got: %v", val)
				}
			}

			// Verify memory was applied (or not)
			if tc.expectMemoryEnabled {
				if val := localeFiles["fr"].Contents["farewell"]; val != "Au revoir" {
					t.Errorf("Expected memory to be applied, got: %v", val)
				}
			}

			// Verify AI skip behavior using spy
			//nolint:nestif // Comprehensive assertion logic is clearer kept together
			if tc.expectSkipAI {
				// When AI should be skipped, translator should not be called
				if spy.called {
					t.Errorf("Expected translator to be skipped, but it was called")
				}
				if metrics.TranslatedKeys != 0 {
					t.Errorf("Expected AI to be skipped, but got %d AI translations", metrics.TranslatedKeys)
				}
			} else {
				// In normal mode, translator is called only if there are remaining translations
				// Check if pre-sourcing handled everything
				remainingTranslations := len(missing) - metrics.SourcedTranslatedKeys
				if remainingTranslations > 0 {
					// There were remaining translations, so translator should have been called
					if !spy.called {
						t.Errorf("Expected translator to be called for %d remaining translations, but it was not",
							remainingTranslations)
					}
				} else {
					// All translations were sourced, translator should not be called
					if spy.called {
						t.Errorf("Expected translator NOT to be called (all translations sourced), but it was")
					}
				}
			}
		})
	}
}

// TestService_PreSourcingWithMemory tests that translation memory works in normal mode.
func TestService_PreSourcingWithMemory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tempDir := t.TempDir()

	// Setup: Config without glossary (to test memory alone)
	cfg := &config.Config{} //nolint:exhaustruct // Minimal config for testing
	cfg.Files.Directory = tempDir
	cfg.Files.Locales = map[string]string{
		"en.json": "en",
		"fr.json": "fr",
	}
	cfg.Files.Indent = 2

	// Setup: Normal mode flags
	flagsInstance := &flags.Flags{ //nolint:exhaustruct // Minimal flags for testing
		TrimStrings: true,
	}

	// Setup: Locale files where "Hello" appears twice with one translated
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
				"other":    "Hello", // Same text in different location
			},
		},
		"fr": {
			Path:   filepath.Join(tempDir, "fr.json"),
			Locale: "fr",
			Contents: map[string]any{
				"other": "Bonjour", // Memory source for "Hello"
			},
		},
	}

	// Create repository and service
	repository := files.NewRepository(cfg, flagsInstance)
	service := engine.NewService(nil, nil, cfg, flagsInstance, 4096, repository)

	// Run service
	metrics, err := service.Run(ctx, localeFiles, missing, "en", nil)
	if err != nil {
		t.Fatalf("Service.Run failed: %v", err)
	}

	// CRITICAL ASSERTION: Memory should have applied in normal mode
	if metrics.SourcedTranslatedKeys == 0 {
		t.Error("BUG: Translation memory did not apply in normal mode!")
	}

	// Verify the translation was actually written via memory
	if val := localeFiles["fr"].Contents["greeting"]; val != "Bonjour" {
		t.Errorf("Expected 'Bonjour' from memory, got: %v", val)
	}
}
