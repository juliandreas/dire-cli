package translator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/events"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

// Processor handles the complete translation workflow including retries for type conflict resolution.
type Processor struct {
	translatorClient *Client
	config           *config.Config
	repository       files.Repository
	progress         events.Callback
}

const (
	maxAttempts = 5 // Maximum number of translation attempts (1 initial + 4 retries)
)

// NewProcessor creates a new translation processor with retry logic.
func NewProcessor(
	translatorClient *Client,
	cfg *config.Config,
	repository files.Repository,
	progress events.Callback,
) *Processor {
	logger.Debug("Created translator retry processor",
		"provider", cfg.Providers.Active)
	return &Processor{
		translatorClient: translatorClient,
		config:           cfg,
		repository:       repository,
		progress:         progress,
	}
}

// ApplyTranslations handles the complete translation workflow including retries for type conflict resolution.
func (p *Processor) ApplyTranslations(
	ctx context.Context,
	missing []types.MissingTranslation,
) (int, error) {
	logger.Info("Applying translator translations with retry logic",
		"totalKeys", len(missing),
		"provider", p.config.Providers.Active)

	var totalTranslated int

	for attempt := range maxAttempts {
		if ctx.Err() != nil {
			logger.Debug("Translation cancelled", "attempt", attempt, "error", ctx.Err())
			return totalTranslated, fmt.Errorf("translation cancelled: %w", ctx.Err())
		}

		// Execute single translation attempt
		translated, complete, remainingCount, err := p.executeAttempt(ctx, missing, attempt)
		totalTranslated += translated

		// Handle errors
		if err != nil {
			if errors.Is(err, errs.ErrRetryable) && attempt < maxAttempts-1 {
				// Use shared backoff utility with jitter
				delay := utils.CalculateRetryDelay(attempt, err)
				logger.Warn("Retryable translator error; backing off",
					"attempt", attempt,
					"delay", delay,
					"error", err)

				select {
				case <-ctx.Done():
					return totalTranslated, fmt.Errorf("translation cancelled during backoff: %w", ctx.Err())
				case <-time.After(delay):
					// Continue to next attempt
				}
				continue
			}
			logger.Error("Translator failed",
				"attempt", attempt+1,
				"error", err)
			return totalTranslated, fmt.Errorf("translator attempt %d failed: %w", attempt+1, err)
		}

		// If translation is complete, we're done
		if complete {
			logger.Info("Translator completed successfully",
				"totalAttempts", attempt+1,
				"totalTranslated", totalTranslated)
			return totalTranslated, nil
		}

		// Log retry information when there are more attempts to make
		const oneBasedOffset = 1
		nextAttempt := attempt + oneBasedOffset + 1 // Convert to 1-based and add 1 for next
		if nextAttempt <= maxAttempts {
			logger.Info("Continuing translator for remaining keys",
				"completedAttempt", attempt+1,
				"nextAttempt", nextAttempt,
				"maxAttempts", maxAttempts,
				"remainingKeys", remainingCount)
		}
	}

	// Check final state after max attempts
	return p.handleIncompleteTranslation(ctx, totalTranslated)
}

// executeAttempt performs a single translation attempt and returns results.
func (p *Processor) executeAttempt(
	ctx context.Context,
	originalMissing []types.MissingTranslation,
	attempt int,
) (int, bool, int, error) {
	logger.Debug("Executing translator attempt",
		"attempt", attempt,
		"originalMissing", len(originalMissing))

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
			return 0, false, 0, fmt.Errorf("retry scan failed: %w", scanErr)
		}

		if len(currentMissing) == 0 {
			logger.Info("All translations completed during retry", "attempt", attempt)
			return 0, true, 0, nil // Complete!
		}
	}

	// Create progress wrapper to add attempt info to events
	progressWrapper := func(event *events.Event) {
		if attempt > 0 {
			event.Attempt = attempt
		}
		if p.progress != nil {
			p.progress(event)
		}
	}

	// Process translations using the translator client
	results, err := p.translatorClient.ProcessTranslations(ctx, currentMissing, progressWrapper)
	if err != nil {
		// Error will be logged at the caller level to avoid duplication
		return 0, false, 0, fmt.Errorf("translator client failed: %w", err)
	}

	logger.Debug("Translator attempt completed",
		"attempt", attempt,
		"inputKeys", len(currentMissing),
		"outputResults", len(results))

	// Apply translations and check completion
	return p.applyAndCheckCompletion(ctx, results)
}

// applyAndCheckCompletion applies translations using buffered writer and checks if translation is complete.
func (p *Processor) applyAndCheckCompletion(
	ctx context.Context,
	results []types.LocaleContent,
) (int, bool, int, error) {
	// Apply translations using buffered writer
	bufferedWriter := files.NewBufferedWriter(ctx, p.config, len(results))
	bufferedWriter.Start(ctx)

	// Send translations to buffered writer
	if !bufferedWriter.Send(results) {
		logger.Error("Failed to send translations to buffered writer")
		return 0, false, 0, fmt.Errorf("%w: failed to send translations to buffered writer", errs.ErrProviderError)
	}

	// Ensure all writes are flushed before scanning
	if err := bufferedWriter.Close(); err != nil {
		logger.Error("Failed to close buffered writer", "error", err)
		return 0, false, 0, fmt.Errorf("failed to flush translations: %w", err)
	}

	// Check if translation is complete
	remaining, scanErr := p.repository.GetMissing(ctx)
	if scanErr != nil {
		logger.Debug("Failed to get remaining translations", "error", scanErr)
		return len(results), false, 0, fmt.Errorf("completion scan failed: %w", scanErr)
	}
	complete := len(remaining) == 0
	remainingCount := len(remaining)

	return len(results), complete, remainingCount, nil
}

// handleIncompleteTranslation handles the case where translation didn't complete within max attempts.
func (p *Processor) handleIncompleteTranslation(ctx context.Context, totalTranslated int) (int, error) {
	remaining, err := p.repository.GetMissing(ctx)
	if err != nil {
		logger.Error("Failed to get final remaining count", "error", err)
		return totalTranslated, err
	}

	if len(remaining) > 0 {
		logger.Warn("Translation incomplete after maximum attempts",
			"remainingKeys", len(remaining),
			"totalTranslated", totalTranslated,
			"maxAttempts", maxAttempts)
		// Don't return an error - partial translation is still progress
		return totalTranslated, nil
	}

	logger.Info("All translations completed", "totalTranslated", totalTranslated)
	return totalTranslated, nil
}
