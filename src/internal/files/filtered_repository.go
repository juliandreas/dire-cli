package files

import (
	"context"
	"slices"
	"strings"

	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
)

// FilteredRepository wraps a Repository to apply key filtering as a cross-cutting concern.
type FilteredRepository struct {
	Repository Repository
	KeyFilters []string
}

// NewFilteredRepository creates a filtered repository wrapper.
func NewFilteredRepository(repo Repository, keyFilters []string) Repository {
	if len(keyFilters) == 0 {
		// No filtering needed, return original repository
		return repo
	}

	logger.Debug("Created filtered repository wrapper", "keyFilters", keyFilters)
	return &FilteredRepository{
		Repository: repo,
		KeyFilters: keyFilters,
	}
}

// Scan delegates to the underlying repository.
func (fr *FilteredRepository) Scan(ctx context.Context) (map[types.Locale]*types.LocaleFile, error) {
	return fr.Repository.Scan(ctx)
}

// DetermineMissing analyzes locale files, determines the reference locale, and applies filtering to results.
func (fr *FilteredRepository) DetermineMissing(
	ctx context.Context,
	files map[types.Locale]*types.LocaleFile,
	includeStubs bool,
) ([]types.MissingTranslation, types.Locale, error) {
	// Get unfiltered results from wrapped repository
	missing, referenceLocale, err := fr.Repository.DetermineMissing(ctx, files, includeStubs)
	if err != nil {
		return nil, "", err
	}

	// Apply filtering if filters are configured
	if len(fr.KeyFilters) == 0 {
		return missing, referenceLocale, nil
	}

	// Validate key paths exist in reference locale
	if referenceFile, exists := files[referenceLocale]; exists {
		if err := fr.validateFilterKeys(referenceFile); err != nil {
			return nil, "", err
		}
	}

	logger.Debug("Applying key filters", "keyFilters", fr.KeyFilters, "beforeFilter", len(missing))

	// Expand object-level missing translations to leaf-level for granular filtering
	missing = expandObjectsToLeaves(missing)

	// Apply filtering to the expanded leaf-level translations
	missing = filterMissingByKeys(missing, fr.KeyFilters)
	logger.Debug("Applied key filters", "afterFilter", len(missing))

	return missing, referenceLocale, nil
}

// GetMissing rescans files and returns filtered missing translations.
func (fr *FilteredRepository) GetMissing(ctx context.Context) ([]types.MissingTranslation, error) {
	return fr.getMissingInternal(ctx, false)
}

// GetMissingWithStubs rescans files and returns filtered missing translations, including empty strings as missing.
func (fr *FilteredRepository) GetMissingWithStubs(ctx context.Context) ([]types.MissingTranslation, error) {
	return fr.getMissingInternal(ctx, true)
}

// GetFilteredContentForLocale rescans files and returns filtered content for a specific locale.
func (fr *FilteredRepository) GetFilteredContentForLocale(
	ctx context.Context,
	locale types.Locale,
) ([]types.LocaleContent, error) {
	// Get existing translations for the specific locale from wrapped repository
	existing, err := fr.Repository.GetFilteredContentForLocale(ctx, locale)
	if err != nil {
		return nil, err
	}

	logger.Debug(
		"Applying key filters to existing translations for locale",
		"locale", locale,
		"keyFilters", fr.KeyFilters,
		"beforeFilter", len(existing))

	// Filter existing translations to only include those matching the specified key paths
	var filtered []types.LocaleContent
	for _, existingItem := range existing {
		if shouldIncludeKey(existingItem.Path, fr.KeyFilters) {
			filtered = append(filtered, existingItem)
		}
	}
	existing = filtered

	logger.Debug("Applied key filters to existing translations for locale",
		"locale", locale,
		"afterFilter", len(existing))

	return existing, nil
}

// FinalCleanup delegates to the wrapped repository (no filtering needed for cleanup).
func (fr *FilteredRepository) FinalCleanup(ctx context.Context) error {
	return fr.Repository.FinalCleanup(ctx)
}

// filterMissingByKeys filters missing translations to only include those matching the specified key paths.
// If keyPaths is empty, returns all missing translations (no filtering).
func filterMissingByKeys(
	missing []types.MissingTranslation, keyPaths []string,
) []types.MissingTranslation {
	// If no key paths specified, return all missing translations
	if len(keyPaths) == 0 {
		return missing
	}

	var filtered []types.MissingTranslation

	for _, missingItem := range missing {
		if shouldIncludeKey(missingItem.Path, keyPaths) {
			filtered = append(filtered, missingItem)
		}
	}

	return filtered
}

// shouldIncludeKey determines if a key path should be included based on the filter key paths.
// Returns true if the key matches exactly or is a child of any filter key path.
func shouldIncludeKey(keyPath, filterPaths []string) bool {
	keyStr := strings.Join(keyPath, ".")

	for _, filterPath := range filterPaths {
		// Direct match
		if keyStr == filterPath {
			return true
		}

		// Check if key is a child of the filter path
		if strings.HasPrefix(keyStr, filterPath+".") {
			return true
		}
	}

	return false
}

// validateFilterKeys validates that the specified key filters exist in the reference locale file.
func (fr *FilteredRepository) validateFilterKeys(referenceFile *types.LocaleFile) error {
	var invalidKeys []string

	for _, keyPath := range fr.KeyFilters {
		if !keyExists(strings.Split(keyPath, "."), referenceFile.Contents) {
			invalidKeys = append(invalidKeys, keyPath)
		}
	}

	if len(invalidKeys) > 0 {
		return errs.NewInvalidKeysError(invalidKeys)
	}

	return nil
}

// keyExists checks if a key path exists in the given contents map.
func keyExists(keyPath []string, contents map[string]any) bool {
	if len(keyPath) == 0 {
		return false
	}

	current := contents
	for index, key := range keyPath {
		value, exists := current[key]
		if !exists {
			return false
		}

		// If this is not the last key in the path, ensure it's a map
		if index < len(keyPath)-1 {
			if nested, ok := value.(map[string]any); ok {
				current = nested
			} else {
				return false // Path doesn't lead to a nested object
			}
		}
	}

	return true
}

// expandObjectsToLeaves expands object-level missing translations to individual leaf translations
// for granular filtering. This preserves the original IdentifyMissingKeys behavior while enabling
// fine-grained filtering when needed.
func expandObjectsToLeaves(missing []types.MissingTranslation) []types.MissingTranslation {
	var expanded []types.MissingTranslation

	for _, missingItem := range missing {
		if objValue, isObject := missingItem.SourceValue.(map[string]any); isObject {
			// This is an object-level missing translation - expand it to leaf translations
			leafTranslations := expandObjectToLeafTranslations(
				objValue,
				missingItem.Path,
				missingItem.SourceLocale,
				missingItem.TargetLocale,
			)
			expanded = append(expanded, leafTranslations...)
		} else {
			// This is already a leaf translation - keep it as is
			expanded = append(expanded, missingItem)
		}
	}

	return expanded
}

// expandObjectToLeafTranslations recursively expands an object to individual leaf translations.
func expandObjectToLeafTranslations(
	obj map[string]any,
	basePath []string,
	sourceLocale types.Locale,
	targetLocale types.Locale,
) []types.MissingTranslation {
	var translations []types.MissingTranslation

	for key, value := range obj {
		currentPath := slices.Clone(basePath)
		currentPath = append(currentPath, key)

		if nestedObj, isObject := value.(map[string]any); isObject {
			// Recursively expand nested objects
			nestedTranslations := expandObjectToLeafTranslations(
				nestedObj,
				currentPath,
				sourceLocale,
				targetLocale,
			)
			translations = append(translations, nestedTranslations...)
		} else {
			// This is a leaf value - create a missing translation
			translations = append(translations, types.MissingTranslation{
				Path:         currentPath,
				SourceLocale: sourceLocale,
				SourceValue:  value,
				TargetLocale: targetLocale,
			})
		}
	}

	return translations
}

// getMissingInternal is the internal implementation for both GetMissing methods.
func (fr *FilteredRepository) getMissingInternal(
	ctx context.Context, includeStubs bool,
) ([]types.MissingTranslation, error) {
	// Scan files to get current state
	files, err := fr.Repository.Scan(ctx)
	if err != nil {
		return nil, err
	}

	// Identify missing translations using the same logic as DetermineMissing
	missing, _, err := fr.DetermineMissing(ctx, files, includeStubs)
	if err != nil {
		return nil, err
	}

	return missing, nil
}
