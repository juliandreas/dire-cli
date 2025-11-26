package utils

import (
	"reflect"
	"strings"

	"github.com/juliandreas/dire/internal/types"
)

// GroupByLocalePair groups missing translations by their source-target locale pairs.
func GroupByLocalePair(missing []types.MissingTranslation) map[types.LocalePair][]types.MissingTranslation {
	result := make(map[types.LocalePair][]types.MissingTranslation)
	for _, m := range missing {
		pair := types.LocalePair{Source: m.SourceLocale, Target: m.TargetLocale}
		result[pair] = append(result[pair], m)
	}
	return result
}

// DetermineReferenceLocale determines the most complete locale in a set of files.
func DetermineReferenceLocale(localeFiles map[types.Locale]*types.LocaleFile) types.Locale {
	var referenceLocale types.Locale
	var maxScore int

	for locale, file := range localeFiles {
		if file == nil {
			continue
		}

		score := CountKeysRecursive(file.Contents)
		// Prefer more complete locales, with lexicographic tie-breaking for determinism
		if score > maxScore || (score == maxScore && (referenceLocale == "" || locale < referenceLocale)) {
			maxScore = score
			referenceLocale = locale
		}
	}

	return referenceLocale
}

// CountKeysRecursive counts leaf keys, excluding only empty-string values;
// all other leaf values (nil, numbers, bools) are counted.
// Arrays are recursed; count non-empty strings and all non-string leaves (including nil).
func CountKeysRecursive(data map[string]any) int {
	total := 0
	for _, v := range data {
		total += countLeaves(v)
	}
	return total
}

// countLeaves walks maps/arrays and counts leaves per CountKeysRecursive semantics.
func countLeaves(value any) int {
	switch typedValue := value.(type) {
	case map[string]any:
		sum := 0
		for _, mapValue := range typedValue {
			sum += countLeaves(mapValue)
		}
		return sum
	case []any:
		sum := 0
		for _, element := range typedValue {
			sum += countLeaves(element)
		}
		return sum
	case string:
		if strings.TrimSpace(typedValue) != "" {
			return 1
		}
		return 0
	default:
		// Recurse over any slice/array kind (e.g., []string, []map[string]any)
		rv := reflect.ValueOf(value)
		if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
			sum := 0
			for i := range rv.Len() {
				sum += countLeaves(rv.Index(i).Interface())
			}
			return sum
		}
		return 1
	}
}

// HasKeyMismatch detects structural drift between original and AI response:
// - Added/dropped keys are flagged
// - Map-vs-non-map and shallow array-vs-non-array mismatches are flagged
// Array contents are intentionally not recursed to keep validation lightweight.
func HasKeyMismatch(original, aiResponse map[string]any) bool {
	// Check each key in AI response exists in original with same type
	for key, responseValue := range aiResponse {
		originalValue, exists := original[key]
		if !exists {
			return true // AI added a key that doesn't exist in original - likely translated
		}

		// Check if both are maps or both are not maps
		originalMap, originalIsMap := originalValue.(map[string]any)
		responseMap, responseIsMap := responseValue.(map[string]any)

		if originalIsMap != responseIsMap {
			return true // Type mismatch
		}

		// Check if both are arrays or both are not arrays
		originalIsArray := isArrayLike(originalValue)
		responseIsArray := isArrayLike(responseValue)

		if originalIsArray != responseIsArray {
			return true // Array type mismatch
		}

		// If both are maps, recurse
		if originalIsMap && responseIsMap {
			if HasKeyMismatch(originalMap, responseMap) {
				return true // Nested structure mismatch
			}
		}
	}

	// Symmetric check: ensure AI didn't drop original keys
	for key, originalValue := range original {
		responseValue, exists := aiResponse[key]
		if !exists {
			return true
		}
		_, oIsMap := originalValue.(map[string]any)
		_, rIsMap := responseValue.(map[string]any)
		if oIsMap != rIsMap {
			return true
		}

		// Check array type consistency in symmetric direction
		oIsArray := isArrayLike(originalValue)
		rIsArray := isArrayLike(responseValue)
		if oIsArray != rIsArray {
			return true
		}
	}
	return false
}

// isArrayLike returns true for any slice/array kind (e.g., []any, []string, []map[string]any).
func isArrayLike(v any) bool {
	if v == nil {
		return false
	}
	k := reflect.TypeOf(v).Kind()
	return k == reflect.Slice || k == reflect.Array
}
