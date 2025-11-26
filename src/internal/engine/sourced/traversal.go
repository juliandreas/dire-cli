package sourced

import (
	"context"
	"sort"
	"strconv"

	"github.com/juliandreas/dire/internal/types"
)

// TranslationLookup is a function that looks up a translation for a source value.
// It returns the translation and a boolean indicating if a translation was found.
type TranslationLookup func(sourceValue string) (translation string, found bool)

// findMatchesInArray recursively traverses an array to find translation matches.
// It uses the provided lookup function to find translations for string values.
func findMatchesInArray(
	ctx context.Context,
	arr []any,
	basePath []string,
	targetLocale types.Locale,
	lookup TranslationLookup,
) ([]types.LocaleContent, bool) {
	matches := make([]types.LocaleContent, 0)
	fullyMatched := true

	for idx, elem := range arr {
		// Check for cancellation
		if ctx.Err() != nil {
			return matches, false
		}

		currentPath := make([]string, len(basePath)+1)
		copy(currentPath, basePath)
		currentPath[len(basePath)] = strconv.Itoa(idx)
		switch value := elem.(type) {
		case string:
			if translation, found := lookup(value); found {
				matches = append(matches, types.LocaleContent{
					Locale: targetLocale,
					Path:   currentPath,
					Value:  translation,
				})
			} else {
				fullyMatched = false
			}
		case map[string]any:
			nestedMatches, nestedFullyMatched := findMatchesInObject(
				ctx, value, currentPath, targetLocale, lookup)
			matches = append(matches, nestedMatches...)
			if !nestedFullyMatched {
				fullyMatched = false
			}
		case []any:
			nestedMatches, nestedFullyMatched := findMatchesInArray(
				ctx, value, currentPath, targetLocale, lookup)
			matches = append(matches, nestedMatches...)
			if !nestedFullyMatched {
				fullyMatched = false
			}
		default:
			// Non-string values (numbers, bools, etc.) are treated as already satisfied
			// since they don't need translation
		}
	}
	return matches, fullyMatched
}

// findMatchesInObject recursively traverses an object to find translation matches.
// It uses the provided lookup function to find translations for string values.
func findMatchesInObject(
	ctx context.Context,
	sourceObj map[string]any,
	basePath []string,
	targetLocale types.Locale,
	lookup TranslationLookup,
) ([]types.LocaleContent, bool) {
	matches := make([]types.LocaleContent, 0)
	fullyMatched := true

	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(sourceObj))
	for k := range sourceObj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		// Check for cancellation
		if ctx.Err() != nil {
			return matches, false
		}

		value := sourceObj[key]
		currentPath := make([]string, len(basePath)+1)
		copy(currentPath, basePath)
		currentPath[len(basePath)] = key

		switch value := value.(type) {
		case string:
			if translation, found := lookup(value); found {
				matches = append(matches, types.LocaleContent{
					Locale: targetLocale,
					Path:   currentPath,
					Value:  translation,
				})
			} else {
				fullyMatched = false
			}
		case map[string]any:
			nestedMatches, nestedFullyMatched := findMatchesInObject(
				ctx, value, currentPath, targetLocale, lookup)
			matches = append(matches, nestedMatches...)
			if !nestedFullyMatched {
				fullyMatched = false
			}
		case []any:
			nestedMatches, nestedFullyMatched := findMatchesInArray(
				ctx, value, currentPath, targetLocale, lookup)
			matches = append(matches, nestedMatches...)
			if !nestedFullyMatched {
				fullyMatched = false
			}
		default:
			// Non-string values (numbers, bools, etc.) are treated as already satisfied
			// since they don't need translation
		}
	}

	return matches, fullyMatched
}
