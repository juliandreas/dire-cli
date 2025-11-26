package translator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/events"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/translator/batching"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

// ProviderFunc defines the signature for translator implementations.
type ProviderFunc func(
	ctx context.Context,
	httpClient *http.Client,
	provider config.Provider,
	apiKey string,
	texts []string,
	sourceLang, targetLang string,
	formality constants.Formality,
) ([]string, error)

// Client handles translation operations using pure translation services (not LLMs).
type Client struct {
	provider   config.Provider
	apiKey     string
	httpClient *http.Client
	providerFn ProviderFunc
	formality  constants.Formality
}

const (
	translationTimeout  = 30 * time.Second
	idleConnTimeout     = 30 * time.Second
	responseTimeout     = 15 * time.Second
	maxIdleConns        = 100 // Increase connection pool for throughput
	maxIdleConnsPerHost = 100 // Allow more connections per host
)

var (
	errTranslationCountMismatch = errors.New("translation count mismatch")
	errProviderTranslationCount = errors.New("provider returned wrong number of translations")
	errProviderFnRequired       = errors.New("providerFn is required")
)

// NewClient creates a new translation services client.
func NewClient(
	appConfig *config.Config,
	provider config.Provider,
	apiKey string,
	providerFn ProviderFunc,
) (*Client, error) {
	if providerFn == nil {
		return nil, errProviderFnRequired
	}

	// Extract formality from config defaults
	defaults := config.ResolveDefaults(appConfig)
	formality := defaults.Formality

	// Create HTTP client optimized for translation services
	transport := &http.Transport{ //nolint:exhaustruct // only configure fields needed for translation services
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		IdleConnTimeout:       idleConnTimeout,
		ResponseHeaderTimeout: responseTimeout,
		DisableCompression:    false, // Translation services benefit from compression
	}

	httpClient := &http.Client{ //nolint:exhaustruct // only configure fields needed for translation services
		Transport: transport,
		Timeout:   translationTimeout,
	}

	logger.Debug("Created translation services client",
		"provider", provider.Name,
		"timeout", translationTimeout)

	return &Client{
		provider:   provider,
		apiKey:     apiKey,
		httpClient: httpClient,
		providerFn: providerFn,
		formality:  formality,
	}, nil
}

// ProcessTranslations translates missing translations using the configured translation service.
func (c *Client) ProcessTranslations(
	ctx context.Context,
	missing []types.MissingTranslation,
	progress events.Callback,
) ([]types.LocaleContent, error) {
	if len(missing) == 0 {
		return nil, nil
	}

	logger.Debug("Processing translations with translation service",
		"provider", c.provider.Name,
		"totalKeys", len(missing))

	// Group by locale pairs
	byPair := utils.GroupByLocalePair(missing)
	var allResults []types.LocaleContent

	// Process each locale pair
	for pair, pairMissing := range byPair {
		logger.Debug("Processing locale pair",
			"source", pair.Source,
			"target", pair.Target,
			"keys", len(pairMissing))

		// Emit translation pair started event
		if progress != nil {
			progress(events.NewTranslationPairStarted(pair.Source, pair.Target))
		}

		pairStartTime := time.Now()
		results, err := c.processLocalePair(ctx, pairMissing)
		pairDuration := time.Since(pairStartTime)

		if err != nil {
			logger.Error("Translation service failed for locale pair",
				"source", pair.Source,
				"target", pair.Target,
				"provider", c.provider.Name,
				"error", err)
			return allResults, err
		}

		// Emit translation pair completed event
		if progress != nil {
			progress(events.NewTranslationPairCompleted(pair.Source, pair.Target, pairDuration))
		}

		allResults = append(allResults, results...)
	}

	logger.Info("Translation service completed",
		"provider", c.provider.Name,
		"totalResults", len(allResults),
		"localePairs", len(byPair))

	return allResults, nil
}

// processLocalePair processes a single locale pair using the translation service with batching.
func (c *Client) processLocalePair(
	ctx context.Context,
	missing []types.MissingTranslation,
) ([]types.LocaleContent, error) {
	if len(missing) == 0 {
		return nil, nil
	}

	// All items in a batch have the same source/target locales
	sourceLocale := missing[0].SourceLocale
	targetLocale := missing[0].TargetLocale

	// Extract translatable values recursively
	values := extractTranslatableValues(missing)

	if len(values) == 0 {
		logger.Debug("No translatable values found",
			"source", sourceLocale,
			"target", targetLocale)
		return nil, nil
	}

	// Process batches and get translations
	allTranslations, allValues, err := c.processBatches(ctx, values, sourceLocale, targetLocale)
	if err != nil {
		return nil, err
	}

	// Verify we got all expected translations
	if len(allTranslations) != len(allValues) {
		return nil, fmt.Errorf("expected %d, got %d: %w",
			len(allValues), len(allTranslations), errTranslationCountMismatch)
	}

	logger.Debug("Completed batched translation",
		"source", sourceLocale,
		"target", targetLocale,
		"totalTranslations", len(allTranslations))

	// Reassemble into locale content
	return reassembleTranslations(allValues, allTranslations, targetLocale)
}

// processBatches handles the batching logic and processes all batches for a locale pair.
func (c *Client) processBatches(
	ctx context.Context,
	values []types.TranslationValue,
	sourceLocale, targetLocale types.Locale,
) ([]string, []types.TranslationValue, error) {
	// Get batching limits for this provider
	limits := batching.GetBatchLimits(c.provider.Name)

	// Create batches based on provider limits
	batches := batching.CreateBatches(values, limits)
	stats := batching.CalculateBatchingStats(values, batches)

	logger.Debug("Batching translation request",
		"source", sourceLocale,
		"target", targetLocale,
		"originalTexts", stats.OriginalCount,
		"batches", stats.BatchCount,
		"skippedTexts", stats.SkippedCount,
		"estimatedBytes", stats.TotalBytes)

	if stats.SkippedCount > 0 {
		logger.Warn("Skipped oversized texts",
			"provider", c.provider.Name,
			"skippedCount", stats.SkippedCount)
	}

	// Process each batch and collect all translations
	var allTranslations []string
	var allValues []types.TranslationValue

	for index, batch := range batches {
		logger.Debug("Processing batch",
			"batchIndex", index+1,
			"totalBatches", len(batches),
			"textsInBatch", len(batch.Values),
			"estimatedBytes", batch.EstimatedBytes)

		// Translate this batch
		batchTranslations, err := c.translateBatch(ctx, batch, sourceLocale, targetLocale)
		if err != nil {
			return nil, nil, fmt.Errorf("batch %d/%d failed: %w", index+1, len(batches), err)
		}

		// Collect results
		allTranslations = append(allTranslations, batchTranslations...)
		allValues = append(allValues, batch.Values...)
	}

	return allTranslations, allValues, nil
}

// translateBatch translates a single batch of texts.
func (c *Client) translateBatch(
	ctx context.Context,
	batch batching.TranslationBatch,
	sourceLocale, targetLocale types.Locale,
) ([]string, error) {
	// Extract texts from batch
	texts := batch.ExtractTexts()

	// Get retry parameters based on batch size
	maxRetries, _ := utils.GetRetryParameters(len(texts))

	// Translate using the registry function with retry logic
	var translations []string
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check if context is done before attempting
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		translations, lastErr = c.providerFn(
			ctx, c.httpClient, c.provider, c.apiKey, texts,
			string(sourceLocale), string(targetLocale), c.formality,
		)
		if lastErr == nil {
			// Success - validate translation count
			if len(translations) != len(texts) {
				return nil, fmt.Errorf("expected %d, got %d: %w",
					len(texts), len(translations), errProviderTranslationCount)
			}
			return translations, nil
		}

		// Check if error is retryable
		if !errors.Is(lastErr, errs.ErrRetryable) {
			// Non-retryable error, fail immediately
			return nil, fmt.Errorf("translation failed (%s→%s): %w", sourceLocale, targetLocale, lastErr)
		}

		// Don't wait after the last attempt
		if attempt == maxRetries {
			break
		}

		// Calculate delay and wait with context awareness
		delay := utils.CalculateRetryDelay(attempt, lastErr)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// Continue to next retry attempt
		}
	}

	// All retries exhausted, return the last error
	return nil, fmt.Errorf(
		"translation failed (%s→%s) after %d attempts: %w", sourceLocale, targetLocale, maxRetries+1, lastErr)
}
