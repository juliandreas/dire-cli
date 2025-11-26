package files

import (
	"context"
	"slices"
	"sort"

	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

// IdentifyMissingKeys finds missing translations by comparing all locale pairs bi-directionally.
// When overwrite is true, existing translations are also reported as "missing".
func IdentifyMissingKeys(
	ctx context.Context,
	files map[types.Locale]*types.LocaleFile,
	overwrite bool,
) []types.MissingTranslation {
	return identifyMissingKeysInternal(ctx, files, false, overwrite)
}

// IdentifyMissingKeysWithStubs is like IdentifyMissingKeys but also treats empty strings as missing.
// Supports bi-directional comparison and overwrite mode (for glossary translations).
func IdentifyMissingKeysWithStubs(
	ctx context.Context,
	files map[types.Locale]*types.LocaleFile,
	overwrite bool,
) []types.MissingTranslation {
	return identifyMissingKeysInternal(ctx, files, true, overwrite)
}

// identifyMissingKeysInternal is the internal implementation.
// When includeStubs is true, empty strings are also considered missing translations.
// When overwrite is true, existing translations are also treated as missing for glossary translation application.
func identifyMissingKeysInternal(
	ctx context.Context,
	files map[types.Locale]*types.LocaleFile,
	includeStubs bool,
	overwrite bool,
) []types.MissingTranslation {
	// Check if context is done before proceeding
	if ctx.Err() != nil {
		return nil
	}

	// No files to process
	if len(files) == 0 {
		return nil
	}

	// Compare every locale against every other locale to find all missing translations
	// This ensures comprehensive bi-directional coverage of translation gaps
	return findAllMissingTranslations(ctx, files, includeStubs, overwrite)
}

// findAllMissingTranslations performs bi-directional comparison with intelligent deduplication.
// It compares every locale against every other locale to find comprehensive translation gaps,
// then removes duplicate entries for the same target key while preserving the best source.
func findAllMissingTranslations(
	ctx context.Context,
	files map[types.Locale]*types.LocaleFile,
	includeStubs bool,
	overwrite bool,
) []types.MissingTranslation {
	var missing []types.MissingTranslation

	// Compare every locale pair to identify all possible missing translations
	for sourceLocale, sourceFile := range files {
		if ctx.Err() != nil {
			return missing
		}

		for targetLocale, targetFile := range files {
			if sourceLocale == targetLocale {
				continue // Skip self-comparison
			}

			localMissing := compareStructure(
				ctx,
				sourceFile.Contents,
				targetFile.Contents,
				[]string{},
				sourceLocale,
				targetLocale,
				includeStubs,
				overwrite,
			)

			missing = append(missing, localMissing...)
		}
	}

	// Deduplicate missing translations by target key, preferring best source
	return deduplicateMissingTranslations(missing, files)
}

// compareStructure finds missing translations by comparing reference and target structures recursively.
func compareStructure(
	ctx context.Context,
	reference map[string]any,
	target map[string]any,
	path []string,
	referenceLocale types.Locale,
	targetLocale types.Locale,
	includeStubs bool,
	overwrite bool,
) []types.MissingTranslation {
	// Check for context cancellation
	if ctx.Err() != nil {
		return nil
	}

	var missing []types.MissingTranslation

	// Get keys and sort for deterministic output
	keys := make([]string, 0, len(reference))
	for key := range reference {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	// Check each key in reference that should exist in target
	const contextCheckInterval = 50 // Check context every 50 keys instead of every key
	for i, key := range keys {
		// Periodically check for context cancellation during iteration, not on every iteration
		// Skip i=0 since context was already checked at function entry
		if i > 0 && i%contextCheckInterval == 0 && ctx.Err() != nil {
			return missing
		}

		keyMissing := compareKey(
			ctx,
			key,
			reference[key],
			target,
			path,
			referenceLocale,
			targetLocale,
			includeStubs,
			overwrite,
		)
		missing = append(missing, keyMissing...)
	}

	return missing
}

func compareKey(
	ctx context.Context,
	key string,
	refValue any,
	target map[string]any,
	path []string,
	referenceLocale types.Locale,
	targetLocale types.Locale,
	includeStubs bool,
	overwrite bool,
) []types.MissingTranslation {
	currentPath := slices.Clone(path)
	currentPath = append(currentPath, key)

	// Check if key exists in target
	targetValue, exists := target[key]
	if !exists {
		// Key doesn't exist in target at all - create single missing translation
		return []types.MissingTranslation{{
			Path:         currentPath,
			SourceLocale: referenceLocale,
			SourceValue:  refValue,
			TargetLocale: targetLocale,
		}}
	}

	// If includeStubs is true, check if target value is an empty string (stub)
	if includeStubs && isEmptyStub(targetValue) {
		return []types.MissingTranslation{{
			Path:         currentPath,
			SourceLocale: referenceLocale,
			SourceValue:  refValue,
			TargetLocale: targetLocale,
		}}
	}

	// Overwrite mode applies after key existence and stub checks, meaning it only affects
	// keys that exist and aren't empty stubs (when includeStubs is true).
	// This is correct for glossary overwrites: we want to replace existing translations,
	// but still treat missing keys and stubs as regular missing translations.
	if overwrite {
		return handleOverwriteMode(ctx, currentPath, refValue, targetValue, referenceLocale, targetLocale, includeStubs)
	}

	// Handle type comparisons and recursion (normal mode without overwrite)
	return handleValueComparison(
		ctx, refValue, targetValue, currentPath, referenceLocale, targetLocale, includeStubs, overwrite)
}

// handleValueComparison handles type checking and recursion for value comparison.
func handleValueComparison(
	ctx context.Context,
	refValue, targetValue any,
	currentPath []string,
	referenceLocale, targetLocale types.Locale,
	includeStubs bool,
	overwrite bool,
) []types.MissingTranslation {
	refMap, refIsMap := refValue.(map[string]any)
	targetMap, targetIsMap := targetValue.(map[string]any)

	// Type mismatch cases - both return missing translation
	if refIsMap != targetIsMap {
		return []types.MissingTranslation{{
			Path:         currentPath,
			SourceLocale: referenceLocale,
			SourceValue:  refValue,
			TargetLocale: targetLocale,
		}}
	}

	// Both are maps - recursively check nested structure
	if refIsMap && targetIsMap {
		return compareStructure(ctx, refMap, targetMap, currentPath, referenceLocale, targetLocale, includeStubs, overwrite)
	}

	// Both are leaf values - no structural issue
	return nil
}

// handleOverwriteMode handles the overwrite mode logic where existing translations are treated as missing.
// This allows glossary translations to be applied to existing translations.
func handleOverwriteMode(
	ctx context.Context,
	currentPath []string,
	refValue, targetValue any,
	referenceLocale, targetLocale types.Locale,
	includeStubs bool,
) []types.MissingTranslation {
	// Check if this is a leaf value (string) that can be overwritten
	refMap, refIsMap := refValue.(map[string]any)
	targetMap, targetIsMap := targetValue.(map[string]any)

	// If both are leaf values (not maps), check if they're strings before treating as overwritable
	if !refIsMap && !targetIsMap {
		// Only treat strings as overwritable leaves; other types (numbers, booleans, arrays)
		// are not glossary-driven and should not generate overwrite entries
		if _, refIsString := refValue.(string); refIsString {
			if _, targetIsString := targetValue.(string); targetIsString {
				return []types.MissingTranslation{{
					Path:         currentPath,
					SourceLocale: referenceLocale,
					SourceValue:  refValue,
					TargetLocale: targetLocale,
				}}
			}
		}
		// Non-string leaf values: no overwrite entry needed
		return nil
	}

	// If both are maps, continue recursion with overwrite enabled
	if refIsMap && targetIsMap {
		// Keep overwrite=true through recursion to maintain overwrite mode for all nested keys
		return compareStructure(ctx, refMap, targetMap, currentPath, referenceLocale, targetLocale, includeStubs, true)
	}

	// Type mismatch - treat as missing
	return []types.MissingTranslation{{
		Path:         currentPath,
		SourceLocale: referenceLocale,
		SourceValue:  refValue,
		TargetLocale: targetLocale,
	}}
}

// isEmptyStub checks if a value is considered an empty stub (empty string or nested structure with all empty strings).
func isEmptyStub(value any) bool {
	switch val := value.(type) {
	case string:
		return val == ""
	case map[string]any:
		// For nested objects, check if all values are empty stubs
		for _, nestedValue := range val {
			if !isEmptyStub(nestedValue) {
				return false
			}
		}
		return true
	case []any:
		// For arrays, check if all elements are empty stubs
		for _, item := range val {
			if !isEmptyStub(item) {
				return false
			}
		}
		return true
	default:
		// Other types (numbers, booleans, etc.) are not considered empty stubs
		return false
	}
}

// deduplicateMissingTranslations removes duplicate missing translations for the same target key.
// When multiple sources provide the same target key, it selects the best source based on locale completeness.
func deduplicateMissingTranslations(
	missing []types.MissingTranslation,
	files map[types.Locale]*types.LocaleFile,
) []types.MissingTranslation {
	if len(missing) == 0 {
		return missing
	}

	// Create a map of target key -> best translation
	keyMap := make(map[string]types.MissingTranslation)

	// Calculate locale completeness scores for tie-breaking
	localeScores := make(map[types.Locale]int)
	for locale, file := range files {
		if file != nil {
			localeScores[locale] = utils.CountKeysRecursive(file.Contents)
		}
	}

	for _, translation := range missing {
		// Create unique key for this target path + target locale using collision-free path key
		keyPath := utils.PathToKey(translation.Path)
		targetKey := keyPath + "|" + string(translation.TargetLocale)

		existing, exists := keyMap[targetKey]
		if !exists || isBetterSource(translation.SourceLocale, existing.SourceLocale, localeScores) {
			// First time seeing this target key or we found a better source
			keyMap[targetKey] = translation
		}
	}

	// Convert back to slice
	result := make([]types.MissingTranslation, 0, len(keyMap))
	for _, translation := range keyMap {
		result = append(result, translation)
	}

	// Sort for deterministic output across runs
	sort.Slice(result, func(index1, index2 int) bool {
		// Primary sort: path (using collision-free path key)
		pathI := utils.PathToKey(result[index1].Path)
		pathJ := utils.PathToKey(result[index2].Path)
		if pathI != pathJ {
			return pathI < pathJ
		}

		// Secondary sort: target locale
		if result[index1].TargetLocale != result[index2].TargetLocale {
			return result[index1].TargetLocale < result[index2].TargetLocale
		}

		// Tertiary sort: source locale (for tie-breaking)
		return result[index1].SourceLocale < result[index2].SourceLocale
	})

	return result
}

// isBetterSource determines if source A is better than source B for translation purposes.
// Prefers more complete locales, with lexicographic tie-breaking for determinism.
func isBetterSource(sourceA, sourceB types.Locale, localeScores map[types.Locale]int) bool {
	scoreA := localeScores[sourceA]
	scoreB := localeScores[sourceB]

	if scoreA != scoreB {
		return scoreA > scoreB // Higher score is better
	}

	// Tie-breaking: prefer lexicographically first locale for determinism
	return string(sourceA) < string(sourceB)
}
