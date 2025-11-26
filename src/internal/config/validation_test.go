package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/types"
)

// createValidConfig creates a minimal valid configuration for testing.
// It creates temporary locale files to satisfy file existence validation.
func createValidConfig(t *testing.T) *config.Config {
	t.Helper()

	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Create the locale files
	enFile := filepath.Join(tempDir, "en.json")
	svFile := filepath.Join(tempDir, "sv.json")

	// Create minimal valid JSON locale files
	enContent := `{"key": "value"}`
	svContent := `{"key": "värde"}`

	if err := os.WriteFile(enFile, []byte(enContent), 0o600); err != nil {
		t.Fatalf("Failed to create test file %s: %v", enFile, err)
	}
	if err := os.WriteFile(svFile, []byte(svContent), 0o600); err != nil {
		t.Fatalf("Failed to create test file %s: %v", svFile, err)
	}

	return &config.Config{
		Project: struct {
			Name        string `toml:"name"`
			Description string `toml:"description"`
			Domain      string `toml:"domain,omitempty"`
		}{
			Name:        "Test Project",
			Description: "A test project",
			Domain:      "",
		},
		Files: struct {
			Directory string            `toml:"directory"`
			Locales   map[string]string `toml:"locales"`
			Indent    int               `toml:"indent,omitempty"`
		}{
			Directory: tempDir,
			Locales: map[string]string{
				"en.json": "en-US",
				"sv.json": "sv-SE",
			},
			Indent: 2,
		},
		Providers: struct {
			Configuration []config.Provider `toml:"configuration,omitempty"`
			Active        string            `toml:"active,omitempty"`
		}{
			Configuration: []config.Provider{},
			Active:        "",
		},
		StyleGuide: struct {
			Formality constants.Formality `toml:"formality,omitempty"`
		}{
			Formality: "",
		},
		Glossary: struct {
			AutoSort bool                  `toml:"autoSort,omitempty"`
			Entries  []types.GlossaryEntry `toml:"entries,omitempty"`
		}{
			AutoSort: true,
			Entries:  []types.GlossaryEntry{},
		},
	}
}

//nolint:funlen // Test table is long by nature
func TestConfigValidateGlossaryValidCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		glossary    []types.GlossaryEntry
		wantErr     bool
		expectedErr string
	}{
		{
			name:        "empty glossary is valid",
			glossary:    []types.GlossaryEntry{},
			wantErr:     false,
			expectedErr: "",
		},
		{
			name: "single valid entry",
			glossary: []types.GlossaryEntry{
				{"en-US": "hello", "sv-SE": "hej"},
			},
			wantErr:     false,
			expectedErr: "",
		},
		{
			name: "multiple valid entries",
			glossary: []types.GlossaryEntry{
				{"en-US": "hello", "sv-SE": "hej", "da-DK": "hej"},
				{"en-US": "goodbye", "sv-SE": "hej då", "da-DK": "farvel"},
				{"en-US": "apple", "sv-SE": "äpple", "da-DK": "æble"},
			},
			wantErr:     false,
			expectedErr: "",
		},
		{
			name: "valid entry with special characters",
			glossary: []types.GlossaryEntry{
				{"en-US": "Say \"hello, world!\"", "sv-SE": "Säg \"hej, världen!\""},
			},
			wantErr:     false,
			expectedErr: "",
		},
		{
			name: "valid entry with whitespace in values",
			glossary: []types.GlossaryEntry{
				{"en-US": "  valid with spaces  ", "sv-SE": "giltigt med mellanslag"},
			},
			wantErr:     false,
			expectedErr: "",
		},
		{
			name: "valid entry with non-standard but valid BCP47 locale",
			glossary: []types.GlossaryEntry{
				{"zh-Hans-CN": "hello in Chinese", "en-US": "hello"},
			},
			wantErr:     false,
			expectedErr: "",
		},
		{
			name: "accented characters are not duplicates",
			glossary: []types.GlossaryEntry{
				{"fr-FR": "café", "en-US": "coffee"},
				{"fr-FR": "cafe", "en-US": "restaurant"}, // Different words in French
			},
			wantErr:     false,
			expectedErr: "",
		},
		{
			name: "special characters and punctuation differences",
			glossary: []types.GlossaryEntry{
				{"en-US": "Say \"hello!\"", "sv-SE": "Säg \"hej!\""},
				{"en-US": "Say 'hello!'", "sv-SE": "Säg 'hej!'"}, // Different punctuation
			},
			wantErr:     false,
			expectedErr: "",
		},
		{
			name: "same terms in different locales are valid",
			glossary: []types.GlossaryEntry{
				{"en-US": "bank", "sv-SE": "bank"},     // Same word, different meaning
				{"en-US": "spring", "da-DK": "spring"}, // Same word, different meaning
			},
			wantErr:     false,
			expectedErr: "",
		},
		{
			name: "different terms in same locale are valid",
			glossary: []types.GlossaryEntry{
				{"en-US": "hello"},
				{"en-US": "goodbye"},
				{"en-US": "apple"},
			},
			wantErr:     false,
			expectedErr: "",
		},
	}

	runGlossaryTests(t, tests)
}

//nolint:funlen // Test table is long by nature
func TestConfigValidateGlossaryErrorCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		glossary    []types.GlossaryEntry
		wantErr     bool
		expectedErr string
	}{
		{
			name: "empty entry",
			glossary: []types.GlossaryEntry{
				{"en-US": "hello", "sv-SE": "hej"},
				{}, // Empty entry
			},
			wantErr:     true,
			expectedErr: "glossary entry is empty",
		},
		{
			name: "invalid locale format - simple language code",
			glossary: []types.GlossaryEntry{
				{"en": "hello"}, // Should be en-US
			},
			wantErr:     true,
			expectedErr: "must use BCP 47 format",
		},
		{
			name: "invalid locale format - empty locale",
			glossary: []types.GlossaryEntry{
				{"": "hello"},
			},
			wantErr:     true,
			expectedErr: "locale cannot be empty",
		},
		{
			name: "invalid locale format - malformed BCP47",
			glossary: []types.GlossaryEntry{
				{"en_US": "hello"}, // Should use hyphen, not underscore
			},
			wantErr:     true,
			expectedErr: "must use BCP 47 format",
		},
		{
			name: "empty term",
			glossary: []types.GlossaryEntry{
				{"en-US": ""},
			},
			wantErr:     true,
			expectedErr: "empty term",
		},
		{
			name: "whitespace-only term",
			glossary: []types.GlossaryEntry{
				{"en-US": "   \t\n   "},
			},
			wantErr:     true,
			expectedErr: "empty term",
		},
		{
			name: "mixed valid and invalid locales",
			glossary: []types.GlossaryEntry{
				{"en-US": "hello", "sv": "hej"}, // Invalid: missing region
			},
			wantErr:     true,
			expectedErr: "must use BCP 47 format",
		},
		{
			name: "mixed valid and empty terms",
			glossary: []types.GlossaryEntry{
				{"en-US": "hello", "sv-SE": ""}, // Invalid: empty term
			},
			wantErr:     true,
			expectedErr: "empty term",
		},
	}

	runGlossaryTests(t, tests)
}

//nolint:funlen // Test table is long by nature
func TestConfigValidateGlossaryDuplicates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		glossary    []types.GlossaryEntry
		wantErr     bool
		expectedErr string
	}{
		{
			name: "exact duplicate in same locale",
			glossary: []types.GlossaryEntry{
				{"en-US": "hello", "sv-SE": "hej"},
				{"en-US": "hello", "sv-SE": "hejsan"}, // Duplicate "hello" in en-US
			},
			wantErr:     true,
			expectedErr: "duplicate terms",
		},
		{
			name: "case-insensitive duplicate in same locale",
			glossary: []types.GlossaryEntry{
				{"en-US": "Hello", "sv-SE": "hej"},
				{"en-US": "hello", "sv-SE": "hejsan"}, // Case-insensitive duplicate
			},
			wantErr:     true,
			expectedErr: "duplicate terms",
		},
		{
			name: "duplicate with leading/trailing whitespace",
			glossary: []types.GlossaryEntry{
				{"en-US": "hello", "sv-SE": "hej"},
				{"en-US": "  hello  ", "sv-SE": "hejsan"}, // Whitespace duplicate
			},
			wantErr:     true,
			expectedErr: "duplicate terms",
		},
		{
			name: "unicode case folding duplicate",
			glossary: []types.GlossaryEntry{
				{"de-DE": "Straße", "en-US": "street"},
				{"de-DE": "STRASSE", "en-US": "road"}, // Unicode case folding duplicate
			},
			wantErr:     true,
			expectedErr: "duplicate terms",
		},
		{
			name: "cross-locale duplicate - Swedish term appears twice",
			glossary: []types.GlossaryEntry{
				{"en-US": "log in", "sv-SE": "logga in"},
				{"en-US": "sign in", "sv-SE": "logga in"}, // Same Swedish term for different English terms
			},
			wantErr:     true,
			expectedErr: "duplicate terms",
		},
		{
			name: "three-way duplicate",
			glossary: []types.GlossaryEntry{
				{"en-US": "apple", "sv-SE": "äpple"},
				{"en-US": "Apple", "sv-SE": "frukt"}, // Case-insensitive duplicate
				{"en-US": "tree", "sv-SE": "äpple"},  // Different duplicate
			},
			wantErr:     true,
			expectedErr: "duplicate terms",
		},
		{
			name: "complex multi-locale entries with duplicates",
			glossary: []types.GlossaryEntry{
				{"en-US": "home", "sv-SE": "hem", "da-DK": "hjem", "no-NO": "hjem"},
				{"en-US": "house", "sv-SE": "hus", "da-DK": "hus", "no-NO": "hjem"}, // "hjem" duplicate in no-NO
			},
			wantErr:     true,
			expectedErr: "duplicate terms",
		},
		{
			name: "multiple duplicates across different locales",
			glossary: []types.GlossaryEntry{
				{"en-US": "hello", "sv-SE": "hej"},
				{"en-US": "hello", "sv-SE": "tjena"}, // Duplicate in en-US
				{"en-US": "goodbye", "sv-SE": "hej"}, // Duplicate in sv-SE
			},
			wantErr:     true,
			expectedErr: "duplicate terms",
		},
		{
			name: "unicode normalization duplicate - decomposed vs composed",
			glossary: []types.GlossaryEntry{
				{"fr-FR": "café", "en-US": "coffee"},     // "é" as composed character (U+00E9)
				{"fr-FR": "cafe\u0301", "en-US": "shop"}, // "é" as decomposed "e" + combining acute accent (U+0065 + U+0301)
			},
			wantErr:     true,
			expectedErr: "duplicate terms",
		},
	}

	runGlossaryTests(t, tests)
}

func TestConfigValidateGlossaryDeterministicErrorMessages(t *testing.T) {
	t.Parallel()

	// Test that error messages are deterministic by running the same validation multiple times
	cfg := createValidConfig(t)
	cfg.Glossary.Entries = []types.GlossaryEntry{
		{"zh-CN": "hello", "sv-SE": "hej", "en-US": "hi"},
		{"zh-CN": "hello", "sv-SE": "tjena", "en-US": "hey"}, // Duplicate "hello" in zh-CN
	}

	// Run validation multiple times to ensure message is stable
	errorMessages := make([]string, 0, 5)
	for i := range 5 {
		err := cfg.Validate(false)
		if err == nil {
			t.Fatalf("expected error due to duplicate terms on iteration %d", i)
		}
		errorMessages = append(errorMessages, err.Error())
	}

	// All error messages should be identical (deterministic ordering)
	firstMessage := errorMessages[0]
	for i, msg := range errorMessages {
		if msg != firstMessage {
			t.Errorf("Error message %d differs from first message:\nFirst: %s\nDiffer: %s", i, firstMessage, msg)
		}
	}

	// Verify the message contains sorted locale information
	if !strings.Contains(firstMessage, "zh-CN") {
		t.Errorf("Error message should contain locale information: %s", firstMessage)
	}
}

// runGlossaryTests is a helper function to run glossary validation tests.
func runGlossaryTests(t *testing.T, tests []struct {
	name        string
	glossary    []types.GlossaryEntry
	wantErr     bool
	expectedErr string
},
) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := createValidConfig(t)
			cfg.Glossary.Entries = tt.glossary

			// Call the public Validate method with requiresTranslationService=false
			// to skip provider validation and focus on glossary validation
			err := cfg.Validate(false)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Config.Validate() expected error, got nil")
					return
				}

				if tt.expectedErr != "" && !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("Config.Validate() error = %q, expected to contain %q", err.Error(), tt.expectedErr)
				}
			} else if err != nil {
				t.Errorf("Config.Validate() unexpected error = %v", err)
			}
		})
	}
}

//nolint:funlen // Test table is long by nature
func TestConfigValidateGlossaryErrorMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		glossary      []types.GlossaryEntry
		expectedInMsg []string
	}{
		{
			name: "empty entry includes index",
			glossary: []types.GlossaryEntry{
				{"en-US": "valid", "sv-SE": "giltigt"},
				{"en-US": "another", "sv-SE": "en annan"},
				{}, // Empty entry at index 2
			},
			expectedInMsg: []string{
				"glossary entry at index 2",
				"empty",
			},
		},
		{
			name: "invalid locale includes index and locale",
			glossary: []types.GlossaryEntry{
				{"en-US": "valid", "sv-SE": "giltigt"},
				{"en": "hello"}, // Invalid locale at index 1
			},
			expectedInMsg: []string{
				"glossary entry at index 1",
				"invalid locale 'en'",
			},
		},
		{
			name: "empty term includes index and locale",
			glossary: []types.GlossaryEntry{
				{"en-US": "valid", "sv-SE": "giltigt"},
				{"en-US": "hello", "sv-SE": ""}, // Empty term at index 1
			},
			expectedInMsg: []string{
				"glossary entry at index 1",
				"empty term for locale 'sv-SE'",
			},
		},
		{
			name: "duplicate error message format",
			glossary: []types.GlossaryEntry{
				{"en-US": "hello", "sv-SE": "hej"},
				{"en-US": "hello", "sv-SE": "hejsan"}, // Duplicate "hello" in en-US
			},
			expectedInMsg: []string{
				"duplicate terms",
				"hello",
				"en-US",
				"Entry 1",
				"Entry 2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := createValidConfig(t)
			cfg.Glossary.Entries = tt.glossary

			err := cfg.Validate(false)
			if err == nil {
				t.Fatal("expected error")
			}

			errorMsg := err.Error()
			for _, expected := range tt.expectedInMsg {
				if !strings.Contains(errorMsg, expected) {
					t.Errorf("error message should contain %q, got: %v", expected, errorMsg)
				}
			}
		})
	}
}

func TestConfigValidateGlossaryValidationOrder(t *testing.T) {
	t.Parallel()

	t.Run("validation fails on first invalid entry", func(t *testing.T) {
		t.Parallel()

		cfg := createValidConfig(t)
		cfg.Glossary.Entries = []types.GlossaryEntry{
			{}, // This should fail first
			{"en-US": "hello", "sv-SE": "hej"},
			{"en-US": "hello", "sv-SE": "hejsan"}, // This would also fail but shouldn't reach it
		}

		err := cfg.Validate(false)
		if err == nil {
			t.Fatal("expected error")
		}

		// Should fail on the empty entry, not the duplicate
		if !strings.Contains(err.Error(), "empty") {
			t.Errorf("expected error about empty entry, got: %v", err)
		}
	})
}

func TestConfigValidateGlossaryWithComplexValidConfig(t *testing.T) {
	t.Parallel()

	cfg := createValidConfig(t)
	cfg.Glossary.Entries = []types.GlossaryEntry{
		{
			"en-US": "home",
			"sv-SE": "hem",
			"da-DK": "hjem",
			"no-NO": "hjemme", // Different from Danish to avoid duplicate
			"de-DE": "Zuhause",
		},
		{
			"en-US": "house",
			"sv-SE": "hus",
			"da-DK": "hus",
			"no-NO": "hus",
			"de-DE": "Haus",
		},
		{
			"en-US": "Say \"hello, world!\"",
			"sv-SE": "Säg \"hej, världen!\"",
			"da-DK": "Sig \"hej, verden!\"",
		},
	}

	err := cfg.Validate(false)
	if err != nil {
		t.Errorf("Config.Validate() unexpected error with complex valid config: %v", err)
	}
}

//nolint:funlen // Test scenarios require detailed setup
func TestConfigValidateGlossaryIntegrationWithOtherValidation(t *testing.T) {
	t.Parallel()

	t.Run("glossary validation runs after other validations", func(t *testing.T) {
		t.Parallel()

		// Create config with invalid files section but valid glossary
		cfg := &config.Config{
			Project: struct {
				Name        string `toml:"name"`
				Description string `toml:"description"`
				Domain      string `toml:"domain,omitempty"`
			}{
				Name:        "Test Project",
				Description: "A test project",
				Domain:      "",
			},
			Files: struct {
				Directory string            `toml:"directory"`
				Locales   map[string]string `toml:"locales"`
				Indent    int               `toml:"indent,omitempty"`
			}{
				Directory: "", // Invalid: empty directory
				Locales: map[string]string{
					"en.json": "en-US",
					"sv.json": "sv-SE",
				},
				Indent: 0,
			},
			Providers: struct {
				Configuration []config.Provider `toml:"configuration,omitempty"`
				Active        string            `toml:"active,omitempty"`
			}{
				Configuration: []config.Provider{},
				Active:        "",
			},
			StyleGuide: struct {
				Formality constants.Formality `toml:"formality,omitempty"`
			}{
				Formality: "",
			},
			Glossary: struct {
				AutoSort bool                  `toml:"autoSort,omitempty"`
				Entries  []types.GlossaryEntry `toml:"entries,omitempty"`
			}{
				AutoSort: false,
				Entries: []types.GlossaryEntry{
					{"en-US": "hello", "sv-SE": "hej"},
				},
			},
		}

		err := cfg.Validate(false)
		if err == nil {
			t.Fatal("expected error due to invalid files configuration")
		}

		// Should fail on files validation, not glossary
		if !strings.Contains(err.Error(), "directory") {
			t.Errorf("expected error about files directory, got: %v", err)
		}
	})

	t.Run("glossary validation happens when other sections are valid", func(t *testing.T) {
		t.Parallel()

		cfg := createValidConfig(t)
		cfg.Glossary.Entries = []types.GlossaryEntry{
			{"en-US": ""}, // Invalid glossary
		}

		err := cfg.Validate(false)
		if err == nil {
			t.Fatal("expected error due to invalid glossary")
		}

		// Should fail on glossary validation
		if !strings.Contains(err.Error(), "empty term") {
			t.Errorf("expected error about empty glossary term, got: %v", err)
		}
	})
}

//nolint:funlen // Test table is long by nature
func TestConfigValidateGlossaryErrorTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		glossary     []types.GlossaryEntry
		expectedType error
	}{
		{
			name: "empty entry error",
			glossary: []types.GlossaryEntry{
				{},
			},
			expectedType: config.ErrGlossaryEntryEmpty,
		},
		{
			name: "empty term error",
			glossary: []types.GlossaryEntry{
				{"en-US": ""},
			},
			expectedType: config.ErrGlossaryTermEmpty,
		},
		{
			name: "invalid locale error",
			glossary: []types.GlossaryEntry{
				{"en": "hello"},
			},
			expectedType: config.ErrLocaleBCP47Required,
		},
		{
			name: "empty locale error",
			glossary: []types.GlossaryEntry{
				{"": "hello"},
			},
			expectedType: config.ErrLocaleEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := createValidConfig(t)
			cfg.Glossary.Entries = tt.glossary

			err := cfg.Validate(false)
			if err == nil {
				t.Fatal("expected error")
			}

			if !errors.Is(err, tt.expectedType) {
				t.Errorf("Config.Validate() error = %v, expected to wrap %v", err, tt.expectedType)
			}
		})
	}
}
