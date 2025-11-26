package batching

import (
	"fmt"
	"strings"

	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

const (
	maxKeysPerBatch      = 30  // Quality limit for translation coherence
	promptOverheadTokens = 800 // Estimated tokens for prompt template
	outputMultiplier     = 2.5 // Conservative ratio of output:input tokens for translations
	safetyBufferRatio    = 0.3 // Use only 30% of available tokens to handle large nested objects and estimation errors
)

// CreateBatches groups translations into batches respecting token and quality limits.
func CreateBatches(missing []types.MissingTranslation, maxTokens int) ([][]types.MissingTranslation, error) {
	if len(missing) == 0 {
		logger.Debug("No missing translations to batch")
		return nil, nil
	}

	logger.Debug("Starting batch creation",
		"totalTranslations", len(missing),
		"maxTokens", maxTokens)

	// Preprocess translations and prepare for batching
	processedMissing, inputBudget, estimator := preprocessTranslations(missing, maxTokens)

	// Group by locale pairs (can't mix locale pairs)
	byPair := utils.GroupByLocalePair(processedMissing)

	var allBatches [][]types.MissingTranslation

	// Process each locale pair separately
	for pair, pairTranslations := range byPair {
		pairBatches, err := createPairBatches(pairTranslations, inputBudget, estimator)
		if err != nil {
			logger.Error("Failed to create batches for locale pair",
				"source", pair.Source,
				"target", pair.Target,
				"error", err)
			return nil, err
		}

		logger.Debug("Created batches for locale pair",
			"source", pair.Source,
			"target", pair.Target,
			"translations", len(pairTranslations),
			"batches", len(pairBatches))

		allBatches = append(allBatches, pairBatches...)
	}

	logger.Info("Batch creation completed",
		"totalBatches", len(allBatches),
		"totalTranslations", len(processedMissing),
		"avgBatchSize", float64(len(processedMissing))/float64(max(len(allBatches), 1)))

	return allBatches, nil
}

// preprocessTranslations handles setup, budget calculation, and oversized translation breakdown.
func preprocessTranslations(
	missing []types.MissingTranslation, maxTokens int,
) ([]types.MissingTranslation, int, TokenEstimator) {
	// Create token estimator
	estimator := NewTokenEstimator()

	// Calculate safe input token budget
	inputBudget := calculateInputBudget(maxTokens)

	// Break down any oversized translations
	processedMissing := NewTranslationBreakdown().BreakdownOversized(missing, inputBudget, estimator)

	if len(processedMissing) != len(missing) {
		logger.Info("Broke down oversized translations",
			"originalCount", len(missing),
			"processedCount", len(processedMissing),
			"breakdown", len(processedMissing)-len(missing))
	}

	return processedMissing, inputBudget, estimator
}

// calculateInputBudget calculates how many tokens we can safely use for input.
func calculateInputBudget(maxTokens int) int {
	// Reserve tokens for prompt overhead and output expansion
	available := maxTokens - promptOverheadTokens
	if available <= 0 {
		return 0 // Clamp to prevent negative budgets
	}

	// Account for output
	inputSpace := int(float64(available) / (1.0 + outputMultiplier))

	// Apply safety buffer
	budget := int(float64(inputSpace) * safetyBufferRatio)

	return budget
}

// createPairBatches creates batches for a single locale pair, trying smaller batch sizes if needed.
func createPairBatches(
	translations []types.MissingTranslation,
	inputBudget int,
	estimator TokenEstimator,
) ([][]types.MissingTranslation, error) {
	// Try different batch sizes with graceful degradation
	for _, maxKeys := range []int{maxKeysPerBatch, 15, 10, 5, 1} {
		batches, err := createLimitedBatches(translations, maxKeys, inputBudget, estimator)
		if err == nil {
			return batches, nil
		}

		// Only continue if it was a batch size issue, not an individual translation issue
		if maxKeys == 1 {
			logger.Error("Individual translation too large",
				"error", err)
			return nil, err // Individual translation too large
		}
	}

	// Should never reach here
	return nil, fmt.Errorf("%w: even with single-item batches", errs.ErrBatching)
}

// createLimitedBatches groups translations into batches with specific key and token limits.
func createLimitedBatches(
	translations []types.MissingTranslation,
	maxKeys int,
	inputBudget int,
	estimator TokenEstimator,
) ([][]types.MissingTranslation, error) {
	var batches [][]types.MissingTranslation
	currentBatch := make([]types.MissingTranslation, 0, maxKeys)
	var currentTokens int

	for _, translation := range translations {
		tokens := estimator.EstimateTranslationTokens(translation)

		// Check if individual translation is too large
		if tokens > inputBudget {
			logger.Debug("Individual translation exceeds budget",
				"path", strings.Join(translation.Path, "."),
				"tokens", tokens,
				"budget", inputBudget)
			return nil, errs.NewOversizedError(
				strings.Join(translation.Path, "."),
				tokens,
				inputBudget,
			)
		}

		// Check if adding this would exceed either limit
		wouldExceedKeys := len(currentBatch) >= maxKeys
		wouldExceedTokens := currentTokens+tokens > inputBudget

		if (wouldExceedKeys || wouldExceedTokens) && len(currentBatch) > 0 {
			// Close current batch
			batches = append(batches, currentBatch)
			currentBatch = make([]types.MissingTranslation, 0, maxKeys)
			currentTokens = 0
		}

		currentBatch = append(currentBatch, translation)
		currentTokens += tokens
	}

	// Add final batch
	if len(currentBatch) > 0 {
		batches = append(batches, currentBatch)
	}

	return batches, nil
}
