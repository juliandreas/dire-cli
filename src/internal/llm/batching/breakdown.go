package batching

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
)

// TranslationBreakdown splits oversized translations into manageable pieces.
type TranslationBreakdown interface {
	BreakdownOversized(
		missing []types.MissingTranslation,
		inputBudget int,
		estimator TokenEstimator,
	) []types.MissingTranslation
}

// DefaultTranslationBreakdown implements TranslationBreakdown with standard breakdown logic.
type DefaultTranslationBreakdown struct{}

// NewTranslationBreakdown creates a handler for splitting oversized translations.
func NewTranslationBreakdown() TranslationBreakdown {
	return &DefaultTranslationBreakdown{}
}

// BreakdownOversized splits any translations that are too large into smaller pieces.
func (b *DefaultTranslationBreakdown) BreakdownOversized(
	missing []types.MissingTranslation,
	inputBudget int,
	estimator TokenEstimator,
) []types.MissingTranslation {
	var result []types.MissingTranslation
	var oversizedCount int
	var brokenDownCount int

	for _, translation := range missing {
		var processed []types.MissingTranslation

		// Handle a single translation, breaking it down if needed
		sourceMap, ok := translation.SourceValue.(map[string]any)
		if !ok {
			processed = b.breakdownNonMap(translation, inputBudget, estimator)
		} else {
			processed = b.breakdownMap(translation, sourceMap, inputBudget, estimator)
		}

		if len(processed) > 1 {
			oversizedCount++
			brokenDownCount += len(processed)
			logger.Debug("Broke down oversized translation",
				"path", strings.Join(translation.Path, "."),
				"resultingLeaves", len(processed))
		}
		result = append(result, processed...)
	}

	if oversizedCount > 0 {
		logger.Info("Oversized translation breakdown summary",
			"oversizedObjects", oversizedCount,
			"resultingLeaves", brokenDownCount,
			"avgLeavesPerOversizedObject", float64(brokenDownCount)/float64(oversizedCount))
	}

	return result
}

// breakdownMap handles map-type translations, breaking them down if oversized.
func (b *DefaultTranslationBreakdown) breakdownMap(
	translation types.MissingTranslation, sourceMap map[string]any, inputBudget int, estimator TokenEstimator,
) []types.MissingTranslation {
	// Estimate total tokens using the estimator
	totalTokens := estimator.EstimateTranslationTokens(translation)

	if totalTokens <= inputBudget {
		// Normal size - keep as is
		return []types.MissingTranslation{translation}
	}

	// Too large - break down into leaf translations
	logger.Debug("Breaking down oversized map translation",
		"path", strings.Join(translation.Path, "."),
		"estimatedTokens", totalTokens,
		"budget", inputBudget,
	)

	leaves := b.extractLeavesFromMap(
		sourceMap, translation.Path, translation.SourceLocale, translation.TargetLocale)

	// If no leaves were extracted, fall back to the original translation
	// to prevent oversized-but-empty maps from being dropped entirely
	if len(leaves) == 0 {
		logger.Debug("No leaves extracted from oversized map, preserving original",
			"path", strings.Join(translation.Path, "."))
		return []types.MissingTranslation{translation}
	}

	return leaves
}

// breakdownNonMap validates non-map translations for size limits.
func (b *DefaultTranslationBreakdown) breakdownNonMap(
	translation types.MissingTranslation,
	inputBudget int,
	estimator TokenEstimator,
) []types.MissingTranslation {
	// Compute token estimation once and reuse across branches
	tokens := estimator.EstimateTranslationTokens(translation)

	// Check if it's an array that might need breakdown
	if sourceArray, ok := translation.SourceValue.([]any); ok {
		if tokens <= inputBudget {
			return []types.MissingTranslation{translation}
		}

		// Array is too large - check if we can break it down
		if canBreakdownArray(sourceArray) {
			logger.Debug("Breaking down oversized array translation",
				"path", strings.Join(translation.Path, "."),
				"estimatedTokens", tokens,
				"budget", inputBudget,
				"arrayLength", len(sourceArray))
			return b.extractLeavesFromArray(
				sourceArray, translation.Path, translation.SourceLocale, translation.TargetLocale)
		}
	}

	// Non-map/non-array value or unbreakable array - reuse tokens
	if tokens <= inputBudget {
		return []types.MissingTranslation{translation}
	}

	// Oversized value - keep as is and let batching handle the error
	logger.Warn("Translation exceeds token budget",
		"path", strings.Join(translation.Path, "."),
		"estimatedTokens", tokens,
		"budget", inputBudget,
		"valueType", fmt.Sprintf("%T", translation.SourceValue))
	return []types.MissingTranslation{translation}
}

// extractLeavesFromMap extracts individual leaf values from a map.
func (b *DefaultTranslationBreakdown) extractLeavesFromMap(
	sourceMap map[string]any,
	basePath []string,
	sourceLocale, targetLocale types.Locale,
) []types.MissingTranslation {
	var result []types.MissingTranslation
	keys := slices.Collect(maps.Keys(sourceMap))
	slices.Sort(keys)
	for _, key := range keys {
		value := sourceMap[key]
		currentPath := append(slices.Clone(basePath), key)

		switch val := value.(type) {
		case map[string]any:
			// Recursively extract from nested maps
			nested := b.extractLeavesFromMap(val, currentPath, sourceLocale, targetLocale)
			result = append(result, nested...)
		case []any:
			// Handle arrays - keep them as whole units unless they can be broken down
			if canBreakdownArray(val) {
				nested := b.extractLeavesFromArray(val, currentPath, sourceLocale, targetLocale)
				result = append(result, nested...)
			} else {
				result = append(result, types.MissingTranslation{
					Path:         currentPath,
					SourceLocale: sourceLocale,
					SourceValue:  value,
					TargetLocale: targetLocale,
				})
			}
		default:
			// Leaf value - create individual translation
			result = append(result, types.MissingTranslation{
				Path:         currentPath,
				SourceLocale: sourceLocale,
				SourceValue:  value,
				TargetLocale: targetLocale,
			})
		}
	}

	logger.Debug("Extracted leaf translations from map",
		"basePath", strings.Join(basePath, "."),
		"leafCount", len(result))

	return result
}

// extractLeavesFromArray extracts values from an array that contains only objects.
func (b *DefaultTranslationBreakdown) extractLeavesFromArray(
	sourceArray []any,
	basePath []string,
	sourceLocale, targetLocale types.Locale,
) []types.MissingTranslation {
	var result []types.MissingTranslation

	// If array contains objects, we can break it down
	for i, item := range sourceArray {
		if itemMap, ok := item.(map[string]any); ok {
			// Create a path with array index
			indexPath := append(slices.Clone(basePath), fmt.Sprintf("[%d]", i))

			// Extract from the nested object
			nested := b.extractLeavesFromMap(itemMap, indexPath, sourceLocale, targetLocale)
			result = append(result, nested...)
		}
	}

	logger.Debug("Extracted leaf translations from array",
		"basePath", strings.Join(basePath, "."),
		"arrayLength", len(sourceArray),
		"leafCount", len(result))

	return result
}

// canBreakdownArray checks if an array contains only objects and thus can be broken down.
func canBreakdownArray(arr []any) bool {
	if len(arr) == 0 {
		return false
	}

	// Check if all elements are objects
	for _, item := range arr {
		if _, ok := item.(map[string]any); !ok {
			return false // Found non-object element
		}
	}

	return true
}
