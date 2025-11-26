package sourced

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

// MemoryResult contains the results of memory matches.
type MemoryResult struct {
	Matched   []types.LocaleContent
	Remaining []types.MissingTranslation
}

// applyMemory splits missing translations into memory matches vs AI-needed categories.
func applyMemory(
	ctx context.Context,
	glossaryResult glossaryResult,
	files map[types.Locale]*types.LocaleFile,
	referenceLocale types.Locale,
) MemoryResult {
	result := MemoryResult{
		Matched:   make([]types.LocaleContent, 0, len(glossaryResult.Remaining)),
		Remaining: make([]types.MissingTranslation, 0, len(glossaryResult.Remaining)),
	}

	// Check if context is done before proceeding
	if ctx.Err() != nil {
		result.Remaining = glossaryResult.Remaining
		return result
	}

	// Nothing left after glossary — short-circuit
	if len(glossaryResult.Remaining) == 0 {
		return result
	}

	// Build a set of glossary-matched paths to skip
	glossaryPaths := make(map[string]bool, len(glossaryResult.Matched))
	for _, match := range glossaryResult.Matched {
		pathKey := createPathKey(match.Locale, match.Path)
		glossaryPaths[pathKey] = true
	}

	// Build TM only for locales that actually have remaining work
	filesForBuild := make(map[types.Locale]*types.LocaleFile, len(glossaryResult.Remaining)+1)
	filesForBuild[referenceLocale] = files[referenceLocale]
	for _, m := range glossaryResult.Remaining {
		if m.TargetLocale != referenceLocale {
			filesForBuild[m.TargetLocale] = files[m.TargetLocale]
		}
	}
	existingTranslations := buildTranslationMaps(filesForBuild, referenceLocale)

	// Process each missing translation from glossary's remaining items
	for i, missingItem := range glossaryResult.Remaining {
		// Check for context cancellation periodically
		if ctx.Err() != nil {
			// Add all remaining items to needs translation and exit
			result.Remaining = append(result.Remaining, glossaryResult.Remaining[i:]...)
			break
		}

		matched := processMemoryItem(ctx, missingItem, existingTranslations, &result, glossaryPaths)
		if !matched {
			result.Remaining = append(result.Remaining, missingItem)
		}
	}

	return result
}

// processMemoryItem processes a single missing translation item for memory matches.
func processMemoryItem(
	ctx context.Context,
	missingItem types.MissingTranslation,
	existingTranslations map[types.Locale]map[string]string,
	result *MemoryResult,
	glossaryPaths map[string]bool,
) bool {
	// Early context check to short-circuit deep traversals on cancellation
	if ctx.Err() != nil {
		return false
	}

	targetLocale := missingItem.TargetLocale
	sourceValue := missingItem.SourceValue

	// Handle different types of source values
	switch value := sourceValue.(type) {
	case string:
		return processStringMemoryMatch(value, missingItem, existingTranslations, result, glossaryPaths)
	case []any, map[string]any:
		// Handle array and object values - find memory matches within the collection
		memoryMatches := collectCollectionMatches(
			ctx, value, missingItem.Path, targetLocale, existingTranslations, glossaryPaths)
		if len(memoryMatches) > 0 {
			result.Matched = append(result.Matched, memoryMatches...)
		}
		// Check if fully covered by combination of glossary and memory (even if no memory exists)
		fullyMatched := isFullyCoveredByGlossaryOrMemory(
			value, missingItem.Path, targetLocale, glossaryPaths, memoryMatches)
		return fullyMatched
	default:
		// Non-string values don't need translation: copy through.
		result.Matched = append(result.Matched, types.LocaleContent{
			Locale: missingItem.TargetLocale,
			Path:   missingItem.Path,
			Value:  missingItem.SourceValue,
		})
		return true
	}
}

// processStringMemoryMatch handles memory matching for string values.
func processStringMemoryMatch(
	sourceValue string,
	missingItem types.MissingTranslation,
	existingTranslations map[types.Locale]map[string]string,
	result *MemoryResult,
	glossaryPaths map[string]bool,
) bool {
	targetLocale := missingItem.TargetLocale

	// If glossary already handled this path, consider it satisfied early.
	pathKey := createPathKey(targetLocale, missingItem.Path)
	if glossaryPaths[pathKey] {
		return true
	}

	translationMap, exists := existingTranslations[targetLocale]
	if !exists {
		return false
	}

	translationValue, found := translationMap[sourceValue]
	if !found {
		return false
	}

	// We found a translation memory match!
	result.Matched = append(result.Matched, types.LocaleContent{
		Locale: targetLocale,
		Path:   missingItem.Path,
		Value:  translationValue,
	})
	return true
}

// collectCollectionMatches handles memory matching for both arrays and objects.
// It creates a memory lookup, finds matches using the appropriate function,
// and filters out matches already handled by glossary.
func collectCollectionMatches(
	ctx context.Context,
	value any,
	path []string,
	targetLocale types.Locale,
	existingTranslations map[types.Locale]map[string]string,
	glossaryPaths map[string]bool,
) []types.LocaleContent {
	lookup := createMemoryLookup(existingTranslations, targetLocale)
	if lookup == nil {
		return nil
	}

	var memoryMatches []types.LocaleContent
	switch val := value.(type) {
	case []any:
		if ctx.Err() != nil {
			return nil
		}
		arrayMatches, _ := findMatchesInArray(ctx, val, path, targetLocale, lookup)
		memoryMatches = arrayMatches
	case map[string]any:
		if ctx.Err() != nil {
			return nil
		}
		objectMatches, _ := findMatchesInObject(ctx, val, path, targetLocale, lookup)
		memoryMatches = objectMatches
	}

	return filterGlossaryMatches(memoryMatches, glossaryPaths)
}

// createMemoryLookup creates a lookup function for memory translation.
// Returns nil if no translation map exists for the target locale.
func createMemoryLookup(
	existingTranslations map[types.Locale]map[string]string,
	targetLocale types.Locale,
) TranslationLookup {
	translationMap := existingTranslations[targetLocale]
	if translationMap == nil {
		return nil
	}
	return func(value string) (string, bool) {
		s, ok := translationMap[value]
		return s, ok
	}
}

// buildTranslationMaps creates lookup maps from reference locale values to target locale values.
func buildTranslationMaps(
	files map[types.Locale]*types.LocaleFile,
	referenceLocale types.Locale,
) map[types.Locale]map[string]string {
	translationMaps := make(map[types.Locale]map[string]string)

	// Use the provided reference locale (already determined once in service.go)
	if referenceLocale == "" {
		return translationMaps
	}

	// Build a map of paths to values for the reference locale
	referenceFile := files[referenceLocale]
	if referenceFile == nil {
		return translationMaps
	}

	referencePaths := make(map[string]any)
	collectPathValuePairs(referenceFile.Contents, []string{}, referencePaths)

	// Now for each target locale, build translation maps
	for targetLocale, targetFile := range files {
		if targetLocale == referenceLocale || targetFile == nil {
			continue
		}

		translationMap := buildSingleTranslationMap(referencePaths, targetFile)
		translationMaps[targetLocale] = translationMap
	}

	return translationMaps
}

// buildSingleTranslationMap creates a translation map for a single target locale file.
func buildSingleTranslationMap(referencePaths map[string]any, targetFile *types.LocaleFile) map[string]string {
	translationMap := make(map[string]string)
	targetPaths := make(map[string]any)
	collectPathValuePairs(targetFile.Contents, []string{}, targetPaths)

	// For each path that exists in both reference and target (deterministic order)
	refKeys := make([]string, 0, len(referencePaths))
	for k := range referencePaths {
		refKeys = append(refKeys, k)
	}
	sort.Strings(refKeys)
	for _, pathStr := range refKeys {
		referenceValue := referencePaths[pathStr]
		targetValue, exists := targetPaths[pathStr]
		if !exists {
			continue
		}

		if !shouldAddTranslationMapping(referenceValue, targetValue, translationMap) {
			continue
		}
		translationMap[referenceValue.(string)] = targetValue.(string)
	}

	return translationMap
}

// shouldAddTranslationMapping determines if a reference->target mapping should be added.
func shouldAddTranslationMapping(referenceValue, targetValue any, translationMap map[string]string) bool {
	// Reference must be a non-empty string (lookup is string-based)
	refStr, ok := referenceValue.(string)
	if !ok || refStr == "" {
		return false
	}

	// Optional: treat whitespace-only reference as empty
	if strings.TrimSpace(refStr) == "" {
		return false
	}

	// Check if target value exists in the map
	if targetValue == nil {
		return false
	}

	// First-win policy to keep mappings stable across runs
	if _, alreadyMapped := translationMap[refStr]; alreadyMapped {
		return false
	}

	// Skip non-string targets and empty strings (empty translations intentionally ignored to avoid TM hits)
	targetStr, ok := targetValue.(string)
	if !ok || targetStr == "" {
		return false
	}

	// Optional: treat whitespace-only target as empty
	if strings.TrimSpace(targetStr) == "" {
		return false
	}

	// Optional: skip identity mappings (no-op TM)
	if targetStr == refStr {
		return false
	}

	return true
}

// collectPathValuePairsFromArray recursively collects all leaf path -> value mappings from an array.
func collectPathValuePairsFromArray(arr []any, basePath []string, result map[string]any) {
	for i, elem := range arr {
		idxPath := make([]string, len(basePath)+1)
		copy(idxPath, basePath)
		idxPath[len(basePath)] = strconv.Itoa(i)
		switch child := elem.(type) {
		case map[string]any:
			collectPathValuePairs(child, idxPath, result)
		case []any:
			// Recursively process nested arrays
			collectPathValuePairsFromArray(child, idxPath, result)
		default:
			// Leaf element inside array
			result[utils.PathToKey(idxPath)] = child
		}
	}
}

// collectPathValuePairs recursively collects all leaf path -> value mappings from a nested structure.
func collectPathValuePairs(data map[string]any, basePath []string, result map[string]any) {
	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := data[key]
		currentPath := append([]string(nil), basePath...)
		currentPath = append(currentPath, key)

		switch typedValue := value.(type) {
		case map[string]any:
			// Recurse into nested objects
			collectPathValuePairs(typedValue, currentPath, result)
		case []any:
			// Recurse into arrays using deterministic index-based paths
			collectPathValuePairsFromArray(typedValue, currentPath, result)
		default:
			// Leaf value - store the path -> value mapping
			result[utils.PathToKey(currentPath)] = value
		}
	}
}

// createPathKey creates a unique collision-free key for a locale-path combination.
func createPathKey(locale types.Locale, path []string) string {
	return string(locale) + "\x00" + utils.PathToKey(path)
}

// filterGlossaryMatches removes any matches that glossary already handled.
func filterGlossaryMatches(matches []types.LocaleContent, glossaryPaths map[string]bool) []types.LocaleContent {
	filtered := make([]types.LocaleContent, 0, len(matches))
	for _, match := range matches {
		pathKey := createPathKey(match.Locale, match.Path)
		if !glossaryPaths[pathKey] {
			filtered = append(filtered, match)
		}
	}
	return filtered
}

// collectStringLeafPathKeys collects pathKeys (locale + path) for string leaves.
func collectStringLeafPathKeys(value any, basePath []string, locale types.Locale, result map[string]bool) {
	switch typedValue := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typedValue))
		for k := range typedValue {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			childValue := typedValue[key]
			childPath := make([]string, len(basePath)+1)
			copy(childPath, basePath)
			childPath[len(basePath)] = key
			collectStringLeafPathKeys(childValue, childPath, locale, result)
		}
	case []any:
		for i, elem := range typedValue {
			childPath := make([]string, len(basePath)+1)
			copy(childPath, basePath)
			childPath[len(basePath)] = strconv.Itoa(i)
			collectStringLeafPathKeys(elem, childPath, locale, result)
		}
	case string:
		// This is a string leaf - record its pathKey
		result[createPathKey(locale, basePath)] = true
		// Non-string scalars (numbers, booleans) are not translatable, so we ignore them
	}
}

// isFullyCoveredByGlossaryOrMemory checks if all string leaves are covered by glossary or memory matches.
func isFullyCoveredByGlossaryOrMemory(
	value any,
	basePath []string,
	targetLocale types.Locale,
	glossaryPaths map[string]bool,
	memoryMatches []types.LocaleContent,
) bool {
	// Collect all string leaf pathKeys (locale+path) in the structure
	stringLeafPathKeys := make(map[string]bool)
	collectStringLeafPathKeys(value, basePath, targetLocale, stringLeafPathKeys)

	// Create a set of memory match pathKeys
	memoryPathKeys := make(map[string]bool)
	for _, match := range memoryMatches {
		if match.Locale == targetLocale {
			memoryPathKeys[createPathKey(match.Locale, match.Path)] = true
		}
	}

	// Check if every leaf pathKey is covered by either glossary or memory
	for pathKey := range stringLeafPathKeys {
		if !glossaryPaths[pathKey] && !memoryPathKeys[pathKey] {
			return false // Found uncovered path
		}
	}

	return true // All string leaves are covered
}
