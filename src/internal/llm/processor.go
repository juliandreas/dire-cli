package llm

import (
	"context"
	"errors"
	"fmt"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/events"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/llm/batching"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
)

// Processor handles the complete LLM translation workflow including retries.
type Processor struct {
	llmClient  *Client
	config     *config.Config
	maxTokens  int
	repository files.Repository
	progress   events.Callback
}

const (
	maxAttempts = 5 // Maximum number of translation attempts (1 initial + 4 retries)
)

// NewProcessor creates a new LLM translation processor with retry logic.
func NewProcessor(
	llmClient *Client,
	cfg *config.Config,
	maxTokens int,
	repository files.Repository,
	progress events.Callback,
) *Processor {
	logger.Debug("Created LLM translation processor",
		"maxTokens", maxTokens,
		"provider", cfg.Providers.Active)
	return &Processor{
		llmClient:  llmClient,
		config:     cfg,
		maxTokens:  maxTokens,
		repository: repository,
		progress:   progress,
	}
}

// ApplyTranslations handles the complete LLM translation workflow including batching and retries.
func (p *Processor) ApplyTranslations(
	ctx context.Context,
	missing []types.MissingTranslation,
) (int, error) {
	logger.Info("Applying LLM translations",
		"totalKeys", len(missing),
		"maxTokens", p.maxTokens)

	var totalTranslated int

	for attempt := range maxAttempts {
		if ctx.Err() != nil {
			logger.Debug("Translation cancelled", "attempt", attempt, "error", ctx.Err())
			return totalTranslated, fmt.Errorf("translation cancelled: %w", ctx.Err())
		}

		// Execute single translation attempt
		translated, complete, err := p.executeAttempt(ctx, missing, attempt)
		totalTranslated += translated

		// Handle errors
		if err != nil {
			// Check if retryable and not final attempt
			if !errors.Is(err, errs.ErrRetryable) || attempt == maxAttempts-1 {
				logger.Error("LLM translation failed",
					"attempt", attempt,
					"retryable", errors.Is(err, errs.ErrRetryable),
					"error", err)
				return totalTranslated, fmt.Errorf("LLM translation failed: %w", err)
			}
			logger.Warn("Retryable error encountered",
				"attempt", attempt,
				"error", err)
			continue // Retry
		}

		// Check if all translations are complete
		if complete {
			logger.Info("LLM translations completed successfully", "attempt", attempt)
			return totalTranslated, nil
		}

		// Log retry info if we're going to retry
		if attempt < maxAttempts-1 {
			remainingCount := p.getRemainingCount(ctx)
			logger.Info("LLM translation incomplete, retrying",
				"attempt", attempt+1,
				"maxAttempts", maxAttempts,
				"remainingKeys", remainingCount)
		}
	}

	// Final validation after all attempts
	return p.checkFinalCompletion(ctx, totalTranslated, maxAttempts)
}

// executeAttempt executes a single LLM translation attempt with rescanning, batching, and processing.
func (p *Processor) executeAttempt(
	ctx context.Context,
	originalMissing []types.MissingTranslation,
	attempt int,
) (translated int, complete bool, err error) {
	// Determine current missing translations
	var currentMissing []types.MissingTranslation
	if attempt == 0 {
		// First attempt: use provided missing translations
		currentMissing = originalMissing
	} else {
		// Retry attempts: re-scan to get current missing translations
		var scanErr error
		currentMissing, scanErr = p.repository.GetMissing(ctx)
		if scanErr != nil {
			logger.Error("Failed to get missing translations for retry", "error", scanErr)
			return 0, false, scanErr
		}

		if len(currentMissing) == 0 {
			logger.Info("All translations completed during retry", "attempt", attempt)
			return 0, true, nil // Complete!
		}
	}

	// Create token-aware batches for current attempt
	batches, err := batching.CreateBatches(currentMissing, p.maxTokens)
	if err != nil {
		logger.Error("Failed to create batches",
			"attempt", attempt,
			"missing", len(currentMissing),
			"maxTokens", p.maxTokens,
			"error", err)
		return 0, false, fmt.Errorf("failed to create batches: %w", err)
	}

	logger.Info("Created LLM translation batches",
		"attempt", attempt,
		"batchCount", len(batches),
		"totalKeys", len(currentMissing),
		"maxTokens", p.maxTokens)

	// Process batches for this attempt using batch processor
	batchProcessor := batching.NewBatchProcessor(p.llmClient, p.config, p.maxTokens, p.progress)
	translated, processErr := batchProcessor.ProcessBatches(ctx, batches)

	logger.Debug("LLM batch processing completed",
		"attempt", attempt,
		"translated", translated,
		"error", processErr)

	if processErr != nil {
		return translated, false, processErr
	}

	// Check if all translations are now complete
	remaining, scanErr := p.repository.GetMissing(ctx)
	if scanErr != nil {
		logger.Error("Failed to get missing translations for completion check", "attempt", attempt, "error", scanErr)
		return translated, false, nil // Continue to retry
	}
	complete = len(remaining) == 0

	return translated, complete, nil
}

// getRemainingCount gets the count of remaining missing translations (for logging).
func (p *Processor) getRemainingCount(ctx context.Context) int {
	remaining, err := p.repository.GetMissing(ctx)
	if err != nil {
		return -1 // Unknown count
	}
	return len(remaining)
}

// checkFinalCompletion performs final validation after all retry attempts.
func (p *Processor) checkFinalCompletion(ctx context.Context, totalTranslated, maxAttempts int) (int, error) {
	// Final check - if we still have missing translations after all attempts
	remaining, err := p.repository.GetMissing(ctx)
	if err != nil {
		logger.Error("Failed to get missing translations for final completion check", "error", err)
		return totalTranslated, fmt.Errorf("failed to check final completion: %w", err)
	}

	if len(remaining) > 0 {
		logger.Error("LLM translation incomplete after maximum attempts",
			"remainingKeys", len(remaining),
			"maxAttempts", maxAttempts)
		return totalTranslated, errs.NewTranslationIncompleteError(maxAttempts-1, len(remaining))
	}

	return totalTranslated, nil
}
