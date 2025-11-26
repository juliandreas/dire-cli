package translator

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

var (
	errCountMismatch    = errors.New("mismatch between values count and translations count")
	errEmptyTranslation = errors.New("empty translation")

	// VariablePatternRe matches common variable placeholder patterns used in i18n.
	// Compile regex pattern once at package initialization for better performance.
	VariablePatternRe = regexp.MustCompile(strings.Join([]string{
		`\{\{[^}]+\}\}`,                   // {{variable}}
		`\$\{[^}]+\}`,                     // ${variable}
		`\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}`, // {variable} or ICU patterns with nested braces
		`%(?:\[[0-9]+\]|[0-9]+\$)?[-+#0 ]*[0-9]*(?:\.[0-9]+)?[bcdeEfFgGopqstTvxXOU%]`, // printf-style: %s, %d, %v, %O, etc.
	}, "|"))

	// spanUnwrapPatternRe matches span tags with notranslate class for unwrapping.
	// Used by translation providers (Google, Azure) that support HTML preservation.
	spanUnwrapPatternRe = regexp.MustCompile(`<span class="notranslate">([^<]+)</span>`)
)

// extractTranslatableValues extracts all translatable string values from missing translations.
// Arrays are handled by extracting strings with position metadata for direct reconstruction.
func extractTranslatableValues(missing []types.MissingTranslation) []types.TranslationValue {
	var values []types.TranslationValue

	for _, item := range missing {
		extractedValues := extractStringValuesRecursively(item.SourceValue, item.Path)
		values = append(values, extractedValues...)
	}

	return values
}

// extractStringValuesRecursively walks through nested objects and extracts all string values.
func extractStringValuesRecursively(value any, basePath []string) []types.TranslationValue {
	var values []types.TranslationValue

	switch val := value.(type) {
	case string:
		if strings.TrimSpace(val) != "" {
			values = append(values, types.TranslationValue{
				Path:  basePath,
				Value: val,
			})
		}

	case map[string]any:
		// Collect and sort keys for deterministic traversal
		keys := slices.Collect(maps.Keys(val))
		slices.Sort(keys)

		// Process map values in sorted key order
		for _, key := range keys {
			subValue := val[key]
			subPath := make([]string, len(basePath)+1)
			copy(subPath, basePath)
			subPath[len(basePath)] = key
			subValues := extractStringValuesRecursively(subValue, subPath)
			values = append(values, subValues...)
		}

	case []any:
		// Extract strings from arrays using standard bracket notation
		for i, element := range val {
			subPath := make([]string, len(basePath)+1)
			copy(subPath, basePath)
			// Use standard bracket notation for array indices
			subPath[len(basePath)] = fmt.Sprintf("[%d]", i)
			subValues := extractStringValuesRecursively(element, subPath)
			values = append(values, subValues...)
		}
	}

	return values
}

// reassembleTranslations converts translated values back into LocaleContent format.
// Returns an error if the count of values and translations don't match.
// Handles array reconstruction by grouping array elements and reconstructing complete structures.
func reassembleTranslations(
	values []types.TranslationValue,
	translations []string,
	locale types.Locale,
) ([]types.LocaleContent, error) {
	if len(values) != len(translations) {
		return nil, fmt.Errorf("%w: values=%d, translations=%d",
			errCountMismatch, len(values), len(translations))
	}

	translationMap := createTranslationMap(values, translations)
	arrayGroups, regularItems := groupTranslationValues(values, translations, locale)
	arrayItems := processArrayGroups(arrayGroups, translationMap, locale)

	results := make([]types.LocaleContent, 0, len(regularItems)+len(arrayItems))
	results = append(results, regularItems...)
	results = append(results, arrayItems...)

	return results, nil
}

// createTranslationMap creates a map for translations for quick lookup using collision-free keys.
func createTranslationMap(values []types.TranslationValue, translations []string) map[string]string {
	translationMap := make(map[string]string)
	for i := range values {
		pathKey := utils.PathToKey(values[i].Path)
		translationMap[pathKey] = translations[i]
	}
	return translationMap
}

// groupTranslationValues groups values by base path (everything before array indices).
func groupTranslationValues(
	values []types.TranslationValue,
	translations []string,
	locale types.Locale,
) (map[string][]types.TranslationValue, []types.LocaleContent) {
	arrayGroups := make(map[string][]types.TranslationValue)
	regularItems := []types.LocaleContent{}

	for valueIndex := range values {
		path := values[valueIndex].Path
		hasArrayIndex := false
		basePathLen := 0

		// Find the first array index in the path
		for segmentIndex, segment := range path {
			if files.IsArrayIndex(segment) {
				hasArrayIndex = true
				basePathLen = segmentIndex
				break
			}
		}

		if hasArrayIndex {
			basePath := utils.PathToKey(path[:basePathLen])
			arrayGroups[basePath] = append(arrayGroups[basePath], values[valueIndex])
		} else {
			// Regular non-array translation
			regularItems = append(regularItems, types.LocaleContent{
				Locale: locale,
				Path:   path,
				Value:  translations[valueIndex],
			})
		}
	}

	return arrayGroups, regularItems
}

// processArrayGroups processes array groups and returns LocaleContent items.
func processArrayGroups(
	arrayGroups map[string][]types.TranslationValue,
	translationMap map[string]string,
	locale types.Locale,
) []types.LocaleContent {
	results := make([]types.LocaleContent, 0, len(arrayGroups))

	for basePath, groupValues := range arrayGroups {
		indexGroups, maxIndex := groupValuesByArrayIndex(groupValues)
		arrayData := buildArrayFromIndexGroups(indexGroups, maxIndex, groupValues, translationMap)

		var basePathSlice []string
		if basePath == "" {
			basePathSlice = []string{}
		} else {
			basePathSlice = strings.Split(basePath, "\x00")
		}

		results = append(results, types.LocaleContent{
			Locale: locale,
			Path:   basePathSlice,
			Value:  arrayData,
		})
	}

	return results
}

// groupValuesByArrayIndex groups values by their array index and finds the maximum index.
func groupValuesByArrayIndex(values []types.TranslationValue) (map[int][]types.TranslationValue, int) {
	indexGroups := make(map[int][]types.TranslationValue)
	maxIndex := -1

	for _, value := range values {
		index, segmentIndex := findArrayIndexInPath(value.Path)
		if index == -1 {
			continue
		}

		if index > maxIndex {
			maxIndex = index
		}

		// Create a new value with the path after the array index
		newValue := types.TranslationValue{
			Path:  value.Path[segmentIndex+1:], // Everything after the array index
			Value: value.Value,
		}
		indexGroups[index] = append(indexGroups[index], newValue)
	}

	return indexGroups, maxIndex
}

// findArrayIndexInPath finds the first array index in a path and returns the index and its position.
func findArrayIndexInPath(path []string) (int, int) {
	for segmentIndex, segment := range path {
		if index, ok := files.ParseArrayIndex(segment); ok {
			return index, segmentIndex
		}
	}
	return -1, -1
}

// buildArrayFromIndexGroups builds the final array from grouped values.
func buildArrayFromIndexGroups(
	indexGroups map[int][]types.TranslationValue,
	maxIndex int,
	originalValues []types.TranslationValue,
	translationMap map[string]string,
) []any {
	result := make([]any, maxIndex+1)

	for arrayIndex := 0; arrayIndex <= maxIndex; arrayIndex++ {
		group, exists := indexGroups[arrayIndex]
		if !exists {
			result[arrayIndex] = nil
			continue
		}

		if isDirectArrayElement(group) {
			result[arrayIndex] = getDirectElementValue(arrayIndex, originalValues, translationMap)
		} else {
			result[arrayIndex] = buildNestedObject(group, originalValues, arrayIndex, translationMap)
		}
	}

	return result
}

// isDirectArrayElement checks if a group represents a direct array element.
func isDirectArrayElement(group []types.TranslationValue) bool {
	return len(group) == 1 && len(group[0].Path) == 0
}

// getDirectElementValue gets the translation value for a direct array element.
func getDirectElementValue(
	arrayIndex int,
	originalValues []types.TranslationValue,
	translationMap map[string]string,
) any {
	for _, origVal := range originalValues {
		if segIdx, ok := findMatchingArrayIndex(origVal.Path, arrayIndex); ok {
			// Direct array element means nothing after the matched index
			if segIdx+1 == len(origVal.Path) {
				fullPathKey := utils.PathToKey(origVal.Path)
				return translationMap[fullPathKey]
			}
		}
	}
	return nil
}

// buildNestedObject builds a nested object from grouped values.
func buildNestedObject(
	group []types.TranslationValue,
	originalValues []types.TranslationValue,
	arrayIndex int,
	translationMap map[string]string,
) map[string]any {
	obj := make(map[string]any)
	for _, val := range group {
		if len(val.Path) > 0 {
			// Reconstruct the full path for translation lookup
			var fullPath string
			for _, origVal := range originalValues {
				if path := tryMatchOriginalValue(origVal, val, arrayIndex); path != "" {
					fullPath = path
					break
				}
			}
			if fullPath != "" {
				setNestedValue(obj, utils.PathToKey(val.Path), translationMap[fullPath])
			}
		}
	}
	return obj
}

// tryMatchOriginalValue attempts to match an original value with a group value.
func tryMatchOriginalValue(origVal, groupVal types.TranslationValue, arrayIndex int) string {
	if len(origVal.Path) <= len(groupVal.Path) {
		return ""
	}

	segmentIndex, hasMatch := findMatchingArrayIndex(origVal.Path, arrayIndex)
	if !hasMatch {
		return ""
	}

	if pathsMatch(origVal.Path[segmentIndex+1:], groupVal.Path) {
		return utils.PathToKey(origVal.Path)
	}

	return ""
}

// findMatchingArrayIndex finds the array index position that matches the target index.
func findMatchingArrayIndex(path []string, targetIndex int) (int, bool) {
	for segmentIndex, segment := range path {
		if idx, ok := files.ParseArrayIndex(segment); ok && idx == targetIndex {
			return segmentIndex, true
		}
	}
	return -1, false
}

// pathsMatch checks if two path slices are identical.
func pathsMatch(path1, path2 []string) bool {
	if len(path1) != len(path2) {
		return false
	}

	for i, segment := range path1 {
		if segment != path2[i] {
			return false
		}
	}
	return true
}

// setNestedValue sets a value in a nested map structure using a null byte-separated path.
// NOTE: This function currently only supports map-based paths. Array segments like "[0]"
// are treated as literal map keys rather than array indices. This is acceptable for
// translation reconstruction since nested arrays in locale files are rare.
// If array support is needed in the future, consider using a navigator-based approach.
func setNestedValue(obj map[string]any, path, value string) {
	if path == "" {
		return
	}

	parts := strings.Split(path, "\x00")
	current := obj

	for _, part := range parts[:len(parts)-1] {
		if current[part] == nil {
			current[part] = make(map[string]any)
		}
		if nextMap, ok := current[part].(map[string]any); ok {
			current = nextMap
		} else {
			// Path conflict - create new map
			newMap := make(map[string]any)
			current[part] = newMap
			current = newMap
		}
	}

	finalKey := parts[len(parts)-1]
	current[finalKey] = value
}

// ValidateTranslations ensures the translated results match expected count and are non-empty.
func ValidateTranslations(translations, originalTexts []string) error {
	// Check count matches
	if len(translations) != len(originalTexts) {
		return fmt.Errorf("%w: expected %d, got %d",
			errCountMismatch, len(originalTexts), len(translations))
	}

	// Check for empty translations
	for i, translation := range translations {
		if translation == "" {
			return fmt.Errorf("%w at index %d", errEmptyTranslation, i)
		}
	}

	return nil
}

// NormalizeLocale normalizes locale format for translation APIs.
// Preserves region variants (e.g., "pt-BR", "zh-TW") and normalizes separators.
func NormalizeLocale(locale types.Locale) string {
	localeStr := string(locale)
	// Replace underscore with hyphen for normalization (e.g., "pt_BR" → "pt-BR")
	localeStr = strings.ReplaceAll(localeStr, "_", "-")

	return strings.ToLower(localeStr)
}

// ExtractLanguageCode extracts the language code from a locale identifier.
// For example, "en-US" -> "en", "sv-SE" -> "sv".
func ExtractLanguageCode(locale types.Locale) string {
	localeStr := string(locale)
	// Extract language part before dash (e.g., "en-US" -> "en")
	if dashIndex := strings.Index(localeStr, "-"); dashIndex != -1 {
		return localeStr[:dashIndex]
	}
	return localeStr
}

// WrapVariablesWithSpan wraps variables with span tags to protect them from translation.
// Uses <span class="notranslate"> which is supported by Google and Azure translation APIs.
func WrapVariablesWithSpan(text string) string {
	return VariablePatternRe.ReplaceAllStringFunc(text, func(match string) string {
		return `<span class="notranslate">` + match + `</span>`
	})
}

// UnwrapSpanTags removes span tags added by WrapVariablesWithSpan.
func UnwrapSpanTags(text string) string {
	return spanUnwrapPatternRe.ReplaceAllString(text, "$1")
}
