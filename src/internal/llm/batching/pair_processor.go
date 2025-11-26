package batching

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

// LLMClient defines the interface for LLM client operations needed by batching.
type LLMClient interface {
	ProcessTranslations(ctx context.Context, missing []types.MissingTranslation) ([]types.LocaleContent, error)
}

// PairProcessor handles processing all batches for a single locale pair.
type PairProcessor struct {
	llmClient LLMClient
	config    *config.Config
}

// errorClassification represents the classification of a batch error.
type errorClassification int

const (
	errorStopAllBatches errorClassification = iota // Stops processing all remaining batches
	errorSkipBatch                                 // Skip this batch only, continue others
	errorRetryBatch                                // Will be automatically retried
)

const (
	batchTimeoutMinutes = 1  // Fail fast timeout - skip slow batches for retry later
	maxChannelBuffer    = 50 // Maximum buffer size for channel operations

)

var (
	ErrBatchWriteFailed = errors.New("batch write failed")
	ErrBatchTimeout     = errors.New("batch timeout")
)

// String returns a human-readable string representation of the error classification.
func (c errorClassification) String() string {
	switch c {
	case errorStopAllBatches:
		return "stop-all"
	case errorSkipBatch:
		return "skip"
	case errorRetryBatch:
		return "retry"
	default:
		return "unknown"
	}
}

// NewPairProcessor creates a new pair processor.
func NewPairProcessor(llmClient LLMClient, cfg *config.Config) *PairProcessor {
	return &PairProcessor{
		llmClient: llmClient,
		config:    cfg,
	}
}

// ProcessPair processes all batches for a single locale pair concurrently.
func (pp *PairProcessor) ProcessPair(
	ctx context.Context,
	allBatches [][]types.MissingTranslation,
	batchIndices []int,
	semaphore chan struct{},
) (int, error) {
	// Create cancelable context for early termination on global fatal errors
	pairCtx, pairCancel := context.WithCancel(ctx)
	defer pairCancel()

	var wg sync.WaitGroup
	var translatedCount int
	var countMutex sync.Mutex
	var closeErr error

	// Error handling for batch-level failures
	var batchErrors []error
	var batchErrorsMutex sync.Mutex

	// Create separate buffered writer for this locale pair
	channelBufferSize := min(len(batchIndices), maxChannelBuffer)
	bufferedWriter := files.NewBufferedWriter(pairCtx, pp.config, channelBufferSize)
	bufferedWriter.Start(pairCtx)

	defer func() {
		if err := bufferedWriter.Close(); err != nil {
			logger.Error("Failed to close buffered writer", "error", err)
			if closeErr == nil {
				closeErr = err
			} else {
				closeErr = fmt.Errorf("close error: %w (also: %v)", err, closeErr)
			}
		}
	}()

	// Process each batch concurrently within this pair
	for _, idx := range batchIndices {
		wg.Add(1)
		go pp.processBatch(
			pairCtx, idx, allBatches, semaphore, bufferedWriter,
			&wg, &translatedCount, &countMutex, &batchErrors, &batchErrorsMutex,
			pairCancel,
		)
	}

	// Wait for all batches in this pair to complete
	wg.Wait()

	// Check for non-retryable errors in batch processing
	if nonRetryableErr := pp.checkBatchErrors(batchErrors, translatedCount); nonRetryableErr != nil {
		return translatedCount, nonRetryableErr
	}

	// Check for buffer errors
	bufferErr := bufferedWriter.GetError()
	if bufferErr != nil {
		logger.Error("Buffered writer error", "error", bufferErr)
		return translatedCount, bufferErr
	}

	return translatedCount, closeErr
}

// classifyBatchError determines the classification of a batch error.
func (pp *PairProcessor) classifyBatchError(batchErr error) errorClassification {
	// Check if this is a retryable error
	// Timeout errors are always retryable
	if errors.Is(batchErr, ErrBatchTimeout) || errors.Is(batchErr, errs.ErrRetryable) {
		return errorRetryBatch
	}

	// Check if this is an HTTP provider error with status code
	var httpErr *utils.HTTPProviderError
	if errors.As(batchErr, &httpErr) {
		switch httpErr.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
			// 401/403/404 - Auth, permission, or endpoint/model misconfiguration affect all batches globally
			return errorStopAllBatches
		case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
			// 400/413/422 - Content-specific issues
			return errorSkipBatch
		case http.StatusRequestTimeout, http.StatusTooManyRequests: // 408/429 - Transient issues
			return errorRetryBatch
		default:
			// Check if it's a 5xx server error (transient)
			if httpErr.StatusCode >= 500 && httpErr.StatusCode <= 599 {
				return errorRetryBatch // 5xx server errors are transient
			}
			// Unknown HTTP error - treat as batch-specific to avoid infinite retries
			return errorSkipBatch
		}
	}

	// Non-HTTP provider error - treat as batch-specific to be conservative
	return errorSkipBatch
}

// checkBatchErrors examines batch errors and returns any non-retryable errors.
func (pp *PairProcessor) checkBatchErrors(batchErrors []error, translatedCount int) error {
	if len(batchErrors) == 0 {
		return nil
	}

	// Classify all batch errors
	batchSpecificErrors, retryableCount, globalFatalErr := pp.classifyAllBatchErrors(batchErrors)

	// Handle global fatal errors (e.g., authentication failures)
	if globalFatalErr != nil {
		logger.Error("Global fatal error encountered - stopping all batch processing",
			"error", globalFatalErr,
			"totalBatchErrors", len(batchErrors),
			"successfulTranslations", translatedCount)
		return globalFatalErr
	}

	// Handle case where all batches failed with configuration errors (not translation quality issues)
	batchSpecificCount := len(batchSpecificErrors)
	if pp.shouldFailImmediately(translatedCount, batchSpecificCount, retryableCount, batchSpecificErrors) {
		return pp.handleAllBatchesFailedError(batchSpecificErrors)
	}

	// Log skipped batches for debugging
	if batchSpecificCount > 0 || retryableCount > 0 {
		pp.logSkippedBatches(batchErrors, batchSpecificCount, retryableCount, translatedCount)
	}

	return nil
}

// classifyAllBatchErrors classifies batch errors and returns categorized results.
func (pp *PairProcessor) classifyAllBatchErrors(
	batchErrors []error,
) (batchSpecificErrors []error, retryableCount int, globalFatalErr error) {
	batchSpecificErrors = make([]error, 0, len(batchErrors))

	for _, batchErr := range batchErrors {
		classification := pp.classifyBatchError(batchErr)

		switch classification {
		case errorStopAllBatches:
			return nil, 0, batchErr // Return immediately on fatal error
		case errorSkipBatch:
			batchSpecificErrors = append(batchSpecificErrors, batchErr)
		case errorRetryBatch:
			retryableCount++
		}
	}

	return batchSpecificErrors, retryableCount, nil
}

// shouldFailImmediately determines if processing should stop due to configuration errors.
// Returns true only for HTTP 400 errors (bad request), which indicate configuration issues
// like invalid max_tokens. Translation validation errors should be skipped and retried later.
func (pp *PairProcessor) shouldFailImmediately(
	translatedCount, batchSpecificCount, retryableCount int,
	batchSpecificErrors []error,
) bool {
	// Don't fail immediately if we have retryable errors or some translations succeeded
	if translatedCount > 0 || retryableCount > 0 || batchSpecificCount == 0 {
		return false
	}

	// Check if all batch-specific errors are HTTP 400 (configuration errors)
	// Translation validation errors won't have HTTP status codes
	http400Count := 0
	for _, err := range batchSpecificErrors {
		var httpErr *utils.HTTPProviderError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusBadRequest {
			http400Count++
		}
	}

	// Only fail immediately if ALL errors are HTTP 400 (configuration issues)
	return http400Count == batchSpecificCount
}

// handleAllBatchesFailedError logs detailed error information and returns a combined error.
func (pp *PairProcessor) handleAllBatchesFailedError(batchSpecificErrors []error) error {
	logger.Error("All batches failed with non-retryable errors",
		"failedBatches", len(batchSpecificErrors),
		"note", "These errors require configuration or input changes - retrying won't help")

	// Log details of all failed batches to help user diagnose the issue
	for _, batchErr := range batchSpecificErrors {
		pp.logBatchErrorDetails(batchErr)
	}

	// Return a combined error that includes all batch errors
	joined := errors.Join(batchSpecificErrors...)
	return fmt.Errorf("all batches failed with non-retryable errors (%d total): %w",
		len(batchSpecificErrors), joined)
}

// logBatchErrorDetails logs detailed information about a batch error.
func (pp *PairProcessor) logBatchErrorDetails(batchErr error) {
	cls := pp.classifyBatchError(batchErr)
	var httpErr *utils.HTTPProviderError
	if errors.As(batchErr, &httpErr) {
		logger.Error("Batch error details",
			"classification", cls.String(),
			"status", httpErr.StatusCode,
			"provider", httpErr.Provider,
			"error", batchErr)
	} else {
		logger.Error("Batch error details",
			"classification", cls.String(),
			"error", batchErr)
	}
}

// logSkippedBatches logs information about batches that were skipped during processing.
func (pp *PairProcessor) logSkippedBatches(
	batchErrors []error,
	batchSpecificCount, retryableCount, translatedCount int,
) {
	logger.Debug("Some batches were skipped",
		"note", "retryable errors will be retried on next run; batch-specific errors require input changes",
		"skippedBatches", batchSpecificCount+retryableCount,
		"batchSpecific", batchSpecificCount,
		"retryable", retryableCount,
		"successfulTranslations", translatedCount)

	// Log details of failed batches for debugging
	for _, batchErr := range batchErrors {
		cls := pp.classifyBatchError(batchErr)
		var httpErr *utils.HTTPProviderError
		if errors.As(batchErr, &httpErr) {
			logger.Debug("Batch failed",
				"classification", cls.String(), "status", httpErr.StatusCode, "provider", httpErr.Provider, "error", batchErr)
		} else {
			logger.Debug("Batch failed", "classification", cls.String(), "error", batchErr)
		}
	}
}

// translateBatch handles translation of a single batch with retry logic.
func (pp *PairProcessor) translateBatch(
	ctx context.Context,
	batch []types.MissingTranslation,
) ([]types.LocaleContent, error) {
	if len(batch) == 0 {
		return nil, nil
	}

	sourceLoc := batch[0].SourceLocale
	targetLoc := batch[0].TargetLocale

	maxRetries, baseDelay := utils.GetRetryParameters(len(batch))

	startTime := time.Now()
	result, err := pp.retryBatch(ctx, batch, maxRetries, baseDelay)
	duration := time.Since(startTime)

	if err != nil {
		logger.Error("Batch translation failed",
			"source", sourceLoc,
			"target", targetLoc,
			"size", len(batch),
			"duration", duration,
			"error", err)
		return nil, err
	}

	logger.Debug("Batch translation completed",
		"source", sourceLoc,
		"target", targetLoc,
		"inputSize", len(batch),
		"outputSize", len(result),
		"duration", duration)

	return result, nil
}

// retryBatch handles single batch retry logic.
func (pp *PairProcessor) retryBatch(
	ctx context.Context,
	batch []types.MissingTranslation,
	maxRetries int,
	baseDelay time.Duration,
) ([]types.LocaleContent, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("batch translation cancelled: %w", ctx.Err())
		}

		if attempt > 0 {
			delay := max(utils.CalculateRetryDelay(attempt, lastErr), baseDelay)
			logger.Debug("Retrying batch after delay",
				"attempt", attempt,
				"maxRetries", maxRetries,
				"delay", delay)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, fmt.Errorf("cancelled during backoff: %w", ctx.Err())
			}
		}

		translated, err := pp.llmClient.ProcessTranslations(ctx, batch)
		if err == nil {
			if attempt > 0 {
				logger.Info("Batch retry succeeded", "attempt", attempt, "translated", len(translated))
			}
			return translated, nil
		}

		if !errors.Is(err, errs.ErrRetryable) {
			if errors.Is(err, errs.ErrTranslation) {
				logger.Error("Translation error (non-retryable)",
					"attempt", attempt,
					"error", err)
				return nil, err
			}
			logger.Error("Non-retryable batch error",
				"attempt", attempt,
				"error", err)
			return nil, err
		}

		logger.Warn("Retryable batch error",
			"attempt", attempt,
			"maxRetries", maxRetries,
			"error", err)
		lastErr = err
	}

	logger.Error("Batch failed after all retries",
		"maxRetries", maxRetries,
		"lastError", lastErr)
	return nil, fmt.Errorf("batch failed after %d retries, last error: %w", maxRetries, lastErr)
}

// handleBatchError handles errors from individual batch processing.
func (pp *PairProcessor) handleBatchError(
	err error,
	batchIndex, batchSize int,
	duration time.Duration,
	batchErrors *[]error,
	mutex *sync.Mutex,
) {
	if errors.Is(err, context.Canceled) {
		return // Silently exit on user cancellation
	}

	if errors.Is(err, context.DeadlineExceeded) {
		logger.Debug("Batch timed out, will retry on next run",
			"batchIndex", batchIndex,
			"batchSize", batchSize,
			"duration", duration)
		// This is expected behavior when API is slow - we skip and retry later

		// Track timeout in batch errors for summary reporting
		mutex.Lock()
		*batchErrors = append(*batchErrors, fmt.Errorf("batch %d: %w (after %s)", batchIndex, ErrBatchTimeout, duration))
		mutex.Unlock()

		return
	}

	logger.Debug("Batch failed; queued for summary classification",
		"batchIndex", batchIndex,
		"batchSize", batchSize,
		"duration", duration,
		"error", err)

	// Track failed batches for summary logging
	mutex.Lock()
	*batchErrors = append(*batchErrors, fmt.Errorf("batch %d: %w", batchIndex, err))
	mutex.Unlock()
}

// processBatch handles the translation of a single batch in a goroutine.
func (pp *PairProcessor) processBatch(
	ctx context.Context,
	batchIndex int,
	allBatches [][]types.MissingTranslation,
	semaphore chan struct{},
	bufferedWriter *files.BufferedWriter,
	wg *sync.WaitGroup,
	translatedCount *int,
	countMutex *sync.Mutex,
	batchErrors *[]error,
	batchErrorsMutex *sync.Mutex,
	cancelPair context.CancelFunc,
) {
	defer wg.Done()

	batch := allBatches[batchIndex]
	batchSize := len(batch)

	// Acquire semaphore to limit concurrent API calls for this pair
	select {
	case semaphore <- struct{}{}:
		defer func() {
			<-semaphore // Release semaphore
		}()
	case <-ctx.Done():
		return
	}

	// Double-check context after acquiring semaphore to handle race conditions
	if ctx.Err() != nil {
		return
	}

	individualBatchStart := time.Now()

	// Create a timeout context for this specific batch to catch hanging batches
	batchCtx, batchCancel := context.WithTimeout(ctx, batchTimeoutMinutes*time.Minute)
	defer batchCancel()

	// Translate single batch with per-batch timeout
	translated, err := pp.translateBatch(batchCtx, batch)

	individualBatchDuration := time.Since(individualBatchStart)

	if err != nil {
		// Check if this is a global fatal error that should cancel all batches
		if pp.classifyBatchError(err) == errorStopAllBatches {
			logger.Error("Global fatal error detected - canceling remaining batches",
				"batchIndex", batchIndex,
				"error", err)
			cancelPair() // Cancel all other batches in this pair
		}

		pp.handleBatchError(err, batchIndex, batchSize, individualBatchDuration, batchErrors, batchErrorsMutex)
		return
	}

	if len(translated) > 0 {
		pp.handleTranslationSuccess(
			batchIndex, translated, bufferedWriter,
			translatedCount, countMutex, batchErrors, batchErrorsMutex,
		)
	}
}

// handleTranslationSuccess processes successful translation results.
func (pp *PairProcessor) handleTranslationSuccess(
	batchIndex int,
	translated []types.LocaleContent,
	bufferedWriter *files.BufferedWriter,
	translatedCount *int,
	countMutex *sync.Mutex,
	batchErrors *[]error,
	batchErrorsMutex *sync.Mutex,
) {
	// Send to this pair's buffered writer
	if bufferedWriter.Send(translated) {
		countMutex.Lock()
		*translatedCount += len(translated)
		countMutex.Unlock()
	} else {
		logger.Debug("Failed to send translations to buffered writer",
			"batchIndex", batchIndex,
			"translationCount", len(translated))
		batchErrorsMutex.Lock()
		*batchErrors = append(*batchErrors, fmt.Errorf(
			"%w: batch %d failed to write %d translations", ErrBatchWriteFailed, batchIndex, len(translated)))
		batchErrorsMutex.Unlock()
	}
}
