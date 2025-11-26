package sourced

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

var errLocaleFileNotFound = errors.New("locale file not found")

// ApplySourced applies glossary and/or memory translations based on the enabled flags.
func ApplySourced(
	ctx context.Context,
	missing []types.MissingTranslation,
	localeFiles map[types.Locale]*types.LocaleFile,
	referenceLocale types.Locale,
	cfg *config.Config,
	enableGlossary bool,
	enableMemory bool,
) (sourcedCount int, err error) {
	var glossaryRes glossaryResult

	// Apply glossary if enabled, otherwise create empty result with all items in remaining
	if enableGlossary {
		glossaryRes = applyGlossary(ctx, missing, cfg)
		logger.Debug("Glossary applied",
			"matches", len(glossaryRes.Matched),
			"remaining", len(glossaryRes.Remaining))
	} else {
		// Create empty glossary result with all items remaining for memory to process
		glossaryRes.Matched = make([]types.LocaleContent, 0)
		glossaryRes.Remaining = missing
	}

	// Apply memory if enabled
	var memoryRes MemoryResult
	if enableMemory {
		memoryRes = applyMemory(ctx, glossaryRes, localeFiles, referenceLocale)
		logger.Debug("Memory applied",
			"matches", len(memoryRes.Matched),
			"remaining", len(memoryRes.Remaining))
	} else {
		// Create empty memory result
		memoryRes = MemoryResult{
			Matched:   make([]types.LocaleContent, 0),
			Remaining: glossaryRes.Remaining,
		}
	}

	// Filter and save all sourced translations
	appliedCount, err := saveSourced(
		ctx, glossaryRes.Matched, memoryRes.Matched, localeFiles, cfg)
	if err != nil {
		return 0, err
	}

	return appliedCount, nil
}

// saveSourced combines glossary and memory matches, filters duplicates, and saves to files.
func saveSourced(
	ctx context.Context,
	glossaryMatches []types.LocaleContent,
	memoryMatches []types.LocaleContent,
	localeFiles map[types.Locale]*types.LocaleFile,
	cfg *config.Config,
) (int, error) {
	totalMatched := len(glossaryMatches) + len(memoryMatches)
	if totalMatched == 0 {
		logger.Info("Sourced translations summary",
			"glossaryMatches", 0,
			"memoryMatches", 0,
			"totalMatched", 0,
			"actuallyApplied", 0)
		return 0, nil
	}

	// Combine all sourced translations (glossary first) and deduplicate by locale+path
	allSourcedTranslations := make([]types.LocaleContent, 0, totalMatched)
	allSourcedTranslations = append(allSourcedTranslations, glossaryMatches...)
	allSourcedTranslations = append(allSourcedTranslations, memoryMatches...)

	// Deduplicate by (locale, path) - keep first occurrence (glossary wins)
	seen := make(map[string]struct{}, len(allSourcedTranslations))
	unique := make([]types.LocaleContent, 0, len(allSourcedTranslations))
	for _, trans := range allSourcedTranslations {
		// Use collision-free path key construction
		key := string(trans.Locale) + "\x00" + utils.PathToKey(trans.Path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, trans)
	}

	// Filter out translations that are already present with the same value
	filteredTranslations := filterExistingTranslations(unique, localeFiles)
	appliedCount := len(filteredTranslations)
	skippedCount := totalMatched - appliedCount

	if skippedCount > 0 {
		logger.Debug("Skipped sourced translations that already exist with same value",
			"skipped", skippedCount)
	}

	// Save filtered translations to files
	if appliedCount > 0 {
		if err := writeToFiles(ctx, localeFiles, filteredTranslations, cfg); err != nil {
			logger.Error("Failed to update files with sourced translations", "error", err)
			return 0, err
		}
	}

	logger.Info("Sourced translations summary",
		"glossaryMatches", len(glossaryMatches),
		"memoryMatches", len(memoryMatches),
		"totalMatched", totalMatched,
		"actuallyApplied", appliedCount)

	return appliedCount, nil
}

// writeToFiles writes translations to locale files.
func writeToFiles(
	ctx context.Context,
	localeFiles map[types.Locale]*types.LocaleFile,
	translated []types.LocaleContent,
	cfg *config.Config,
) error {
	// Group by locale
	byLocale := make(map[types.Locale][]types.LocaleContent)
	for _, t := range translated {
		byLocale[t.Locale] = append(byLocale[t.Locale], t)
	}

	// Update each file for which we actually have translations
	for locale, translations := range byLocale {
		if err := ctx.Err(); err != nil {
			return err
		}
		file, ok := localeFiles[locale]
		if !ok {
			return fmt.Errorf("%w for locale %s", errLocaleFileNotFound, locale)
		}
		if len(translations) == 0 {
			continue
		}
		if fileUpdateErr := files.UpdateLocaleFile(ctx, file, translations, cfg.GetIndent()); fileUpdateErr != nil {
			logger.Error("Failed to update locale file",
				"locale", locale,
				"path", file.Path,
				"error", fileUpdateErr)
			return fmt.Errorf("failed to update %s: %w", file.Path, fileUpdateErr)
		}
	}

	return nil
}

// filterExistingTranslations filters out translations that already exist with the same value.
func filterExistingTranslations(
	translations []types.LocaleContent,
	localeFiles map[types.Locale]*types.LocaleFile,
) []types.LocaleContent {
	filtered := make([]types.LocaleContent, 0, len(translations))

	for _, trans := range translations {
		// Get the locale file for this translation
		localeFile, exists := localeFiles[trans.Locale]
		if !exists {
			// Locale file doesn't exist, include this translation
			filtered = append(filtered, trans)
			continue
		}

		// Get the current value at this path
		currentValue := getValueAtPath(localeFile.Contents, trans.Path)

		// If the current value doesn't exist or is different from the sourced value, include it
		if currentValue == nil || !valuesEqual(currentValue, trans.Value) {
			filtered = append(filtered, trans)
		}
		// Otherwise skip it (already has the same value)
	}

	return filtered
}

// getValueAtPath retrieves the value at a given path in a nested map structure.
func getValueAtPath(contents map[string]any, path []string) any {
	if len(path) == 0 {
		return nil
	}

	current := contents
	for index, key := range path {
		value, exists := current[key]
		if !exists {
			return nil
		}

		// If this is the last key in the path, return the value
		if index == len(path)-1 {
			return value
		}

		// Otherwise, continue navigating if it's a nested map
		if nested, ok := value.(map[string]any); ok {
			current = nested
		} else {
			// Path doesn't lead to the target, value doesn't exist
			return nil
		}
	}

	return nil
}

// valuesEqual checks if two values are equal for translation purposes.
// For strings, we use case-insensitive comparison to avoid overriding translations
// that only differ in casing (e.g., "Cost-Benefit Analysis" vs "Cost-benefit analysis").
func valuesEqual(valueA, valueB any) bool {
	// Handle string comparisons (the most common case)
	aStr, aIsString := valueA.(string)
	bStr, bIsString := valueB.(string)

	if aIsString && bIsString {
		// Case-insensitive comparison to preserve existing casing preferences
		return strings.EqualFold(aStr, bStr)
	}

	// For other types, use simple equality
	return valueA == valueB
}
