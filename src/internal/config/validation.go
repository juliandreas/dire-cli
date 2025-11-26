package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/types"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

// Validate checks if the configuration is valid.
// When requiresTranslationService is true, provider configuration is validated.
func (c *Config) Validate(requiresTranslationService bool) error {
	var validationErrors []error

	if err := c.validateFiles(); err != nil {
		validationErrors = append(validationErrors, err)
	}

	if requiresTranslationService {
		if err := c.validateProviders(); err != nil {
			validationErrors = append(validationErrors, err)
		}
	}

	if err := c.validateStyleGuide(); err != nil {
		validationErrors = append(validationErrors, err)
	}

	if err := c.validateLocales(); err != nil {
		validationErrors = append(validationErrors, err)
	}

	if err := c.validateGlossary(); err != nil {
		validationErrors = append(validationErrors, err)
	}

	return errors.Join(validationErrors...)
}

// NormalizeGlossaryTerm normalizes a glossary term for comparison and indexing.
// Uses Unicode NFKC normalization, case-insensitive folding, and trimming.
// This normalization is used consistently in both validation and runtime.
func NormalizeGlossaryTerm(term string) string {
	caseFold := cases.Fold()
	return caseFold.String(norm.NFKC.String(strings.TrimSpace(term)))
}

// validateGlossary validates the glossary configuration.
func (c *Config) validateGlossary() error {
	var glossaryErrors []error

	// First validate individual entries
	for i, entry := range c.Glossary.Entries {
		if err := validateGlossaryEntry(entry, i); err != nil {
			glossaryErrors = append(glossaryErrors, err)
		}
	}

	// Then check for duplicates across all entries
	if err := validateGlossaryDuplicates(c.Glossary.Entries); err != nil {
		glossaryErrors = append(glossaryErrors, err)
	}

	return errors.Join(glossaryErrors...)
}

// validateGlossaryEntry validates a single glossary entry.
func validateGlossaryEntry(entry types.GlossaryEntry, index int) error {
	if len(entry) == 0 {
		return fmt.Errorf("glossary entry at index %d: %w", index, ErrGlossaryEntryEmpty)
	}

	// Validate all locales and terms
	for locale, term := range entry {
		// Validate locale format (BCP 47 with region)
		if err := validateLocaleString(locale); err != nil {
			return fmt.Errorf("glossary entry at index %d has invalid locale '%s': %w", index, locale, err)
		}

		// Validate term is not empty
		if strings.TrimSpace(term) == "" {
			return fmt.Errorf(
				"glossary entry at index %d has empty term for locale '%s': %w",
				index, locale, ErrGlossaryTermEmpty)
		}
	}

	return nil
}

// validateGlossaryDuplicates checks for duplicate terms across all glossary entries.
// Since the glossary works bi-directionally, a duplicate in ANY locale is a problem.
func validateGlossaryDuplicates(entries []types.GlossaryEntry) error {
	termIndex := buildTermIndex(entries)
	duplicates := findDuplicateTerms(entries, termIndex)

	if len(duplicates) > 0 {
		return fmt.Errorf("%w:\n%s", ErrGlossaryDuplicateTerms, strings.Join(duplicates, "\n\n"))
	}

	return nil
}

// buildTermIndex creates a map of locale->normalized_term->entry_indices for duplicate detection.
func buildTermIndex(entries []types.GlossaryEntry) map[string]map[string][]int {
	termIndex := make(map[string]map[string][]int)

	// Build index of all term-locale pairs
	for entryIndex, entry := range entries {
		for locale, term := range entry {
			// Normalize the term for comparison
			normalized := NormalizeGlossaryTerm(term)

			// Initialize maps if needed
			if termIndex[locale] == nil {
				termIndex[locale] = make(map[string][]int)
			}

			// Add this entry index to the list for this term-locale pair
			termIndex[locale][normalized] = append(termIndex[locale][normalized], entryIndex)
		}
	}

	return termIndex
}

// findDuplicateTerms identifies and formats duplicate terms for error reporting.
func findDuplicateTerms(entries []types.GlossaryEntry, termIndex map[string]map[string][]int) []string {
	var duplicates []string

	// Sort locales to ensure deterministic error message ordering
	locales := make([]string, 0, len(termIndex))
	for locale := range termIndex {
		locales = append(locales, locale)
	}
	sort.Strings(locales)

	for _, locale := range locales {
		for _, entryIndices := range termIndex[locale] {
			if len(entryIndices) <= 1 {
				continue
			}

			// Found a duplicate! Get the original term from the first entry
			originalTerm := getOriginalTerm(entries[entryIndices[0]], locale)
			conflictingEntries := formatConflictingEntries(entries, entryIndices)

			duplicates = append(duplicates, fmt.Sprintf(
				"%q in %s:\n%s",
				originalTerm, locale,
				strings.Join(conflictingEntries, "\n")))
		}
	}

	return duplicates
}

// getOriginalTerm retrieves the original term from an entry for a specific locale.
func getOriginalTerm(entry types.GlossaryEntry, locale string) string {
	for loc, term := range entry {
		if loc == locale {
			return term
		}
	}
	return ""
}

// formatConflictingEntries formats entry information for duplicate error messages.
func formatConflictingEntries(entries []types.GlossaryEntry, entryIndices []int) []string {
	conflictingEntries := make([]string, len(entryIndices))
	for entryNum, idx := range entryIndices {
		// Show the full entry for context with stable locale ordering
		entryParts := make([]string, 0, len(entries[idx]))

		// Sort locales within each entry for deterministic output
		entryLocales := make([]string, 0, len(entries[idx]))
		for loc := range entries[idx] {
			entryLocales = append(entryLocales, loc)
		}
		sort.Strings(entryLocales)

		for _, loc := range entryLocales {
			val := entries[idx][loc]
			entryParts = append(entryParts, fmt.Sprintf("%s: %q", loc, val))
		}
		conflictingEntries[entryNum] = fmt.Sprintf("  Entry %d: { %s }", idx+1, strings.Join(entryParts, ", "))
	}
	return conflictingEntries
}

// validateLocaleString validates a single locale string using BCP 47 format.
func validateLocaleString(localeStr string) error {
	// Handle empty string
	if localeStr == "" {
		return ErrLocaleEmpty
	}

	// Require hyphenated format - simple language codes are not valid
	if !strings.Contains(localeStr, "-") {
		return ErrLocaleBCP47Required
	}

	// BCP 47 validation
	tag, err := language.Parse(localeStr)
	if err != nil {
		return fmt.Errorf("invalid BCP 47 format: %w", err)
	}

	// Ensure the tag has a region component
	_, confidence := tag.Region()
	if confidence == language.No {
		return ErrLocaleRegionRequired
	}

	return nil
}

// validateStyleGuide validates the style guide configuration section.
func (c *Config) validateStyleGuide() error {
	isValidFormality := func(formality constants.Formality) bool {
		formalityLower := strings.ToLower(string(formality))
		switch formalityLower {
		case strings.ToLower(string(constants.FormalityAutomatic)),
			strings.ToLower(string(constants.FormalityFormal)),
			strings.ToLower(string(constants.FormalityInformal)):
			return true
		default:
			return false
		}
	}

	if c.StyleGuide.Formality != "" && !isValidFormality(c.StyleGuide.Formality) {
		return fmt.Errorf("invalid formality %q. Valid options: %q | %q | %q: %w",
			c.StyleGuide.Formality,
			constants.FormalityAutomatic, constants.FormalityFormal, constants.FormalityInformal, ErrInvalidFormality)
	}

	return nil
}

// validateFiles validates the files configuration section.
func (c *Config) validateFiles() error {
	const minRequiredLocales = 2 // Minimum number of locales required for translation
	var fileErrors []error

	if c.Files.Directory == "" {
		fileErrors = append(fileErrors, ErrFilesDirRequired)
	}

	if len(c.Files.Locales) < minRequiredLocales {
		fileErrors = append(fileErrors, ErrInsufficientLocales)
	}

	// Validate indent if specified (0 means use default, which is valid)
	if c.Files.Indent < 0 {
		fileErrors = append(fileErrors, fmt.Errorf("%w: %d", ErrInvalidIndent, c.Files.Indent))
	}
	// Note: Large indents (>8) are allowed but unusual

	// Check that all configured locale files actually exist
	for pattern, locale := range c.Files.Locales {
		filePath := filepath.Join(c.Files.Directory, pattern)
		if !fileExists(filePath) {
			fileErrors = append(fileErrors, fmt.Errorf("locale file %s (for %s): %w", pattern, locale, ErrLocaleFileNotFound))
		}
	}

	return errors.Join(fileErrors...)
}

// validateProviders validates the providers configuration section.
func (c *Config) validateProviders() error {
	var providerErrors []error

	if len(c.Providers.Configuration) == 0 && c.Providers.Active == "" {
		providerErrors = append(providerErrors, ErrNoProvidersConfigured)
	}

	if c.Providers.Active == "" {
		providerErrors = append(providerErrors, ErrActiveProviderRequired)
	}

	// Validate each provider configuration
	providerNames := make(map[string]bool)
	for i, provider := range c.Providers.Configuration {
		if err := c.validateProvider(provider, i); err != nil {
			providerErrors = append(providerErrors, err)
		}

		name := strings.ToLower(provider.Name)
		if providerNames[name] {
			providerErrors = append(providerErrors,
				fmt.Errorf("duplicate provider name '%s': %w", provider.Name, ErrDuplicateProviderName))
		}
		providerNames[name] = true
	}

	// Ensure active provider exists in configuration list OR is a known translator provider
	// Only check if active provider is specified (not empty)
	if c.Providers.Active != "" {
		active := strings.ToLower(c.Providers.Active)

		// Validate and normalize Azure provider if applicable (azure-<region> -> azure + extract region)
		if err := c.validateAzure(&active); err != nil {
			providerErrors = append(providerErrors, err)
		}

		if !providerNames[active] {
			// Allow translator providers that don't need to be in the configuration array
			if !constants.IsTranslatorProvider(active) {
				providerErrors = append(providerErrors,
					fmt.Errorf("active provider '%s': %w", c.Providers.Active, errs.ErrActiveProviderNotFound))
			}
		}
	}

	return errors.Join(providerErrors...)
}

// validateAzure validates Azure provider format and normalizes it.
// Checks for invalid "azure" without region, and parses "azure-<region>" format.
// Updates the active provider name in-place to "azure" after extracting the region.
func (c *Config) validateAzure(active *string) error {
	// Check for invalid "azure" without region
	if *active == constants.ProviderAzure {
		return ErrAzureInvalidFormat
	}

	// Parse and normalize Azure provider (azure-<region> -> azure + extract region)
	if constants.IsAzureProvider(*active) {
		// Extract region from "azure-<region>" format
		region, ok := strings.CutPrefix(*active, "azure-")
		if !ok || region == "" {
			return fmt.Errorf("%w: provided '%s'", ErrAzureInvalidFormat, *active)
		}

		// Store region for later use, normalize active provider name to just "azure"
		c.azureRegion = region
		c.Providers.Active = constants.ProviderAzure
		*active = constants.ProviderAzure
	}

	return nil
}

// validateProvider validates a single provider configuration.
func (c *Config) validateProvider(provider Provider, index int) error {
	var singleProviderErrors []error

	if provider.Name == "" {
		singleProviderErrors = append(singleProviderErrors,
			fmt.Errorf("provider at index %d: %w", index, ErrProviderNameEmpty))
	}

	if provider.Model == "" {
		singleProviderErrors = append(singleProviderErrors,
			fmt.Errorf("provider '%s': %w", provider.Name, ErrProviderModelEmpty))
	}

	if provider.MaxTokens <= 0 {
		singleProviderErrors = append(singleProviderErrors, fmt.Errorf(
			"provider '%s' must specify max_tokens (got %d): %w",
			provider.Name, provider.MaxTokens, ErrProviderMaxTokensInvalid))
	}

	if provider.Temperature < 0 {
		singleProviderErrors = append(singleProviderErrors, fmt.Errorf(
			"provider '%s' must specify temperature (got %f): %w",
			provider.Name, provider.Temperature, ErrProviderTempInvalid))
	}

	if provider.Name != "" && !constants.IsValidProvider(strings.ToLower(provider.Name)) {
		singleProviderErrors = append(singleProviderErrors, fmt.Errorf("invalid provider '%s'. Valid options: %v: %w",
			provider.Name, constants.SupportedProviders(), ErrProviderInvalid))
	}

	return errors.Join(singleProviderErrors...)
}

// validateLocales validates all locale strings in the configuration.
func (c *Config) validateLocales() error {
	var invalidLocales []string

	for _, localeStr := range c.Files.Locales {
		if err := validateLocaleString(localeStr); err != nil {
			invalidLocales = append(invalidLocales, localeStr)
		}
	}

	if len(invalidLocales) > 0 {
		return fmt.Errorf(
			"invalid locale code(s): %v. Must use BCP 47 format like 'en-US', "+
				"simple language codes like 'en' are not allowed: %w",
			invalidLocales, ErrInvalidLocaleCodes)
	}

	return nil
}
