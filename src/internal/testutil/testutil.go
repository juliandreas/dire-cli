// Package testutil provides shared testing utilities across the application.
package testutil

import (
	"context"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/types"
)

// MockRepository is a configurable mock implementation of files.Repository for testing.
// It can be customized to return specific values or errors for different test scenarios.
type MockRepository struct {
	LocaleFiles      map[types.Locale]*types.LocaleFile
	Missing          []types.MissingTranslation
	MissingWithStubs []types.MissingTranslation
	ReferenceLocale  types.Locale
	ScanErr          error
	IdentifyErr      error
}

const (
	// TestMaxTokens is the default max tokens for test configurations.
	TestMaxTokens = 1000
	// TestTemperature is the default temperature for test configurations.
	TestTemperature = 0.2
)

// MockConfig creates a complete, valid test configuration.
// Returns a fresh instance each time to prevent test pollution.
func MockConfig() *config.Config {
	return &config.Config{
		Project: struct {
			Name        string `toml:"name"`
			Description string `toml:"description"`
			Domain      string `toml:"domain,omitempty"`
		}{
			Name:        "test-project",
			Description: "Test project for unit tests",
			Domain:      "",
		},
		Files: struct {
			Directory string            `toml:"directory"`
			Locales   map[string]string `toml:"locales"`
			Indent    int               `toml:"indent,omitempty"`
		}{
			Directory: "test",
			Locales: map[string]string{
				"en.json": "en-US",
				"fr.json": "fr-FR",
			},
			Indent: 0, // Use default
		},
		Providers: struct {
			Configuration []config.Provider `toml:"configuration,omitempty"`
			Active        string            `toml:"active,omitempty"`
		}{
			Configuration: []config.Provider{
				{
					Name:        constants.ProviderOpenAI,
					Model:       "gpt-4",
					MaxTokens:   TestMaxTokens,
					Temperature: TestTemperature,
					Region:      "", // Not used for LLMs
				},
			},
			Active: constants.ProviderOpenAI,
		},
		StyleGuide: struct {
			Formality constants.Formality `toml:"formality,omitempty"`
		}{
			Formality: constants.FormalityAutomatic,
		},
		Glossary: struct {
			AutoSort bool                  `toml:"autoSort,omitempty"`
			Entries  []types.GlossaryEntry `toml:"entries,omitempty"`
		}{
			AutoSort: false,
			Entries:  []types.GlossaryEntry{},
		},
	}
}

// Scan returns the configured locale files or error.
func (m *MockRepository) Scan(ctx context.Context) (map[types.Locale]*types.LocaleFile, error) {
	if m.ScanErr != nil {
		return nil, m.ScanErr
	}
	return m.LocaleFiles, nil
}

// DetermineMissing returns the configured missing translations and reference locale or error.
func (m *MockRepository) DetermineMissing(
	ctx context.Context,
	files map[types.Locale]*types.LocaleFile,
	includeStubs bool,
) ([]types.MissingTranslation, types.Locale, error) {
	if m.IdentifyErr != nil {
		return nil, "", m.IdentifyErr
	}
	if includeStubs && m.MissingWithStubs != nil {
		return m.MissingWithStubs, m.ReferenceLocale, nil
	}
	return m.Missing, m.ReferenceLocale, nil
}

// GetMissing returns the configured missing translations.
func (m *MockRepository) GetMissing(ctx context.Context) ([]types.MissingTranslation, error) {
	return m.Missing, nil
}

// GetMissingWithStubs returns the configured missing translations including stubs.
func (m *MockRepository) GetMissingWithStubs(ctx context.Context) ([]types.MissingTranslation, error) {
	if m.MissingWithStubs != nil {
		return m.MissingWithStubs, nil
	}
	return m.Missing, nil
}

// GetFilteredContentForLocale returns nil (not used in most tests).
func (m *MockRepository) GetFilteredContentForLocale(
	ctx context.Context,
	locale types.Locale,
) ([]types.LocaleContent, error) {
	return nil, nil
}

// FinalCleanup does nothing (not used in most tests).
func (m *MockRepository) FinalCleanup(ctx context.Context) error {
	return nil
}
