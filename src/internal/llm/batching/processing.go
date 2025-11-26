package batching

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/events"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
)

// BatchProcessor handles concurrent batch translation with progress reporting.
type BatchProcessor struct {
	llmClient      LLMClient
	config         *config.Config
	progress       events.Callback
	maxTokens      int
	completedPairs map[types.LocalePair]bool
	mu             sync.Mutex
}

// coord groups concurrent processing coordination variables.
type coord struct {
	wg              sync.WaitGroup
	translatedCount int64
	errorsMutex     sync.Mutex
	collectedErrors []error
	safeProgress    func(*events.Event)
}

const (
	maxConcurrencyBase  = 3  // Base number of concurrent API calls per locale pair
	maxConcurrencyLarge = 5  // Maximum concurrent API calls for large batch jobs
	largeBatchThreshold = 30 // Threshold for considering a batch job "large" (affects concurrency)
)

// NewBatchProcessor creates a new batch processor for concurrent translation operations.
func NewBatchProcessor(
	llmClient LLMClient,
	cfg *config.Config,
	maxTokens int,
	progress events.Callback,
) *BatchProcessor {
	logger.Debug("Created batch processor")
	return &BatchProcessor{
		llmClient:      llmClient,
		config:         cfg,
		progress:       progress,
		maxTokens:      maxTokens,
		completedPairs: make(map[types.LocalePair]bool),
		mu:             sync.Mutex{},
	}
}

// ProcessBatches handles batch processing of translations.
func (p *BatchProcessor) ProcessBatches(
	ctx context.Context,
	batches [][]types.MissingTranslation,
) (int, error) {
	logger.Debug("Starting batch processing",
		"batches", len(batches))

	// Wrap progress callback to prevent duplicate completion events
	progressCallback := func(event *events.Event) {
		if event.Type == events.TranslationPairCompleted {
			p.mu.Lock()
			pair := types.LocalePair{Source: event.Source, Target: event.Target}
			if p.completedPairs[pair] {
				p.mu.Unlock()
				return // Skip duplicate completion event
			}
			p.completedPairs[pair] = true
			p.mu.Unlock()
		}
		if p.progress != nil {
			p.progress(event)
		}
	}

	// Process batches
	return p.processAllBatches(ctx, batches, progressCallback)
}

// processAllBatches handles concurrent batch translation with progress reporting.
func (p *BatchProcessor) processAllBatches(
	ctx context.Context, batches [][]types.MissingTranslation, progressFn events.Callback,
) (int, error) {
	if len(batches) == 0 {
		logger.Debug("No batches to process")
		return 0, nil
	}

	logger.Info("Starting batch processing",
		"totalBatches", len(batches))

	// Group by locale pairs for progress reporting
	batchesByPair := p.groupBatchIndicesByLocalePair(batches)

	// Initialize concurrent processing coordination
	coordination := p.initializeConcurrentProcessing(progressFn)

	// Create pair processor
	pairProcessor := NewPairProcessor(p.llmClient, p.config)

	// Process each locale pair concurrently
	for pair, batchIndices := range batchesByPair {
		coordination.wg.Add(1)
		go p.processSinglePair(
			ctx, pair, batchIndices, batches, pairProcessor, coordination,
		)
	}

	// Finalize processing and return results
	return p.finalizeProcessing(coordination)
}

// calculateOptimalConcurrency determines concurrent API calls based on batch count.
func (p *BatchProcessor) calculateOptimalConcurrency(totalBatches int) int {
	// Start with 3 concurrent requests - safe for all providers
	baseConcurrency := maxConcurrencyBase

	// For very large jobs, allow slightly more parallelism
	if totalBatches > largeBatchThreshold {
		return maxConcurrencyLarge
	}

	return baseConcurrency
}

// initializeConcurrentProcessing sets up coordination variables and thread-safe progress reporting.
func (p *BatchProcessor) initializeConcurrentProcessing(
	progressFn events.Callback,
) *coord {
	coordination := &coord{
		wg:              sync.WaitGroup{},
		translatedCount: 0,
		errorsMutex:     sync.Mutex{},
		collectedErrors: []error{},
		safeProgress:    nil,
	}

	// Progress reporting needs to be thread-safe
	var progressMutex sync.Mutex
	coordination.safeProgress = func(event *events.Event) {
		if progressFn == nil {
			return
		}
		progressMutex.Lock()
		defer progressMutex.Unlock()
		progressFn(event)
	}

	return coordination
}

// processSinglePair handles the translation of a single locale pair in a goroutine.
func (p *BatchProcessor) processSinglePair(
	ctx context.Context,
	pair types.LocalePair,
	indices []int,
	batches [][]types.MissingTranslation,
	pairProcessor *PairProcessor,
	coordination *coord,
) {
	defer coordination.wg.Done()

	startTime := time.Now()

	logger.Debug("Starting translation",
		"source", pair.Source,
		"target", pair.Target,
		"batches", len(indices))

	coordination.safeProgress(events.NewTranslationPairStarted(pair.Source, pair.Target))

	if ctx.Err() != nil {
		return
	}

	// Calculate optimal concurrency and process batches
	pairConcurrency := p.calculateOptimalConcurrency(len(indices))
	pairSemaphore := make(chan struct{}, pairConcurrency)
	pairTranslated, err := pairProcessor.ProcessPair(ctx, batches, indices, pairSemaphore)

	duration := time.Since(startTime)
	atomic.AddInt64(&coordination.translatedCount, int64(pairTranslated))

	if err != nil {
		p.handlePairError(pair, duration, pairTranslated, err, coordination)
		return
	}

	if ctx.Err() != nil {
		logger.Debug("Translation pair cancelled during completion",
			"source", pair.Source,
			"target", pair.Target)
		coordination.safeProgress(events.NewTranslationPairCancelled(pair.Source, pair.Target))
		return
	}

	coordination.safeProgress(events.NewTranslationPairCompleted(pair.Source, pair.Target, duration))
}

// handlePairError handles error logging and collection for a failed pair.
func (p *BatchProcessor) handlePairError(
	pair types.LocalePair,
	duration time.Duration,
	translatedCount int,
	err error,
	coordination *coord,
) {
	logger.Error("Locale pair processing failed",
		"source", pair.Source,
		"target", pair.Target,
		"duration", duration,
		"translatedCount", translatedCount,
		"error", err)

	coordination.errorsMutex.Lock()
	pairErr := fmt.Errorf("pair %s→%s: %w", pair.Source, pair.Target, err)
	coordination.collectedErrors = append(coordination.collectedErrors, pairErr)
	coordination.errorsMutex.Unlock()
}

// finalizeProcessing waits for completion and handles error collection.
func (p *BatchProcessor) finalizeProcessing(coordination *coord) (int, error) {
	// Wait for all locale pairs to complete
	coordination.wg.Wait()

	finalCount := int(coordination.translatedCount)

	// Return the first error if any occurred
	coordination.errorsMutex.Lock()
	defer coordination.errorsMutex.Unlock()
	if len(coordination.collectedErrors) > 0 {
		logger.Error("Batch processing completed with errors",
			"successfulTranslations", finalCount,
			"errorCount", len(coordination.collectedErrors))
		return finalCount, errors.Join(coordination.collectedErrors...)
	}

	return finalCount, nil
}

// groupBatchIndicesByLocalePair organizes batch indices by their source-target locale pairs.
func (p *BatchProcessor) groupBatchIndicesByLocalePair(
	batches [][]types.MissingTranslation,
) map[types.LocalePair][]int {
	batchesByPair := make(map[types.LocalePair][]int)
	for batchIndex, batch := range batches {
		if len(batch) > 0 {
			pair := types.LocalePair{Source: batch[0].SourceLocale, Target: batch[0].TargetLocale}

			// Defensive check: ensure all items in batch have the same locale pair
			for itemIndex, mt := range batch[1:] {
				if mt.SourceLocale != pair.Source || mt.TargetLocale != pair.Target {
					logger.Warn("Mixed locale pair detected in batch",
						"batchIndex", batchIndex,
						"itemIndex", itemIndex+1,
						"expected", pair,
						"found", types.LocalePair{Source: mt.SourceLocale, Target: mt.TargetLocale})
					break
				}
			}

			batchesByPair[pair] = append(batchesByPair[pair], batchIndex)
		}
	}
	return batchesByPair
}
