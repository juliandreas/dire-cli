package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

// ProviderFunc defines the signature for provider implementations.
type ProviderFunc func(
	ctx context.Context,
	httpClient *http.Client,
	provider config.Provider,
	apiKey string,
	promptText string,
) (map[string]any, error)

// Client handles all LLM API calls.
type Client struct {
	provider    config.Provider
	apiKey      string
	httpClient  *http.Client
	providerFn  ProviderFunc
	projectInfo string
	formality   constants.Formality
	glossary    []types.GlossaryEntry
	config      *config.Config
	userContext string
}

const (
	maxAPIRetries         = 2                // Maximum number of API retries for failed requests
	idleConnTimeout       = 30 * time.Second // Close idle connections after this duration
	responseHeaderTimeout = 45 * time.Second // Fail fast on slow response headers
	clientTimeout         = 2 * time.Minute  // Overall request timeout for LLM API calls
	maxDebugKeys          = 50               // Maximum number of keys to show in debug logs
)

// NewClient creates a new LLM client.
func NewClient(
	appConfig *config.Config,
	formality constants.Formality,
	provider config.Provider,
	providerFn ProviderFunc,
	userContext string,
) (*Client, error) {
	// Get API key from environment
	envKey := constants.GetEnvKeyForProvider(provider.Name)
	apiKey := os.Getenv(envKey)
	if apiKey == "" {
		logger.Error("API key not found",
			"provider", provider.Name,
			"envKey", envKey)
		return nil, errs.NewMissingAPIKeyError(envKey, provider.Name)
	}

	// Build project info
	projectInfo := appConfig.BuildProjectInfo()

	// Create HTTP client with proper transport configuration
	transport := http.DefaultTransport.(*http.Transport).Clone()

	// Configure HTTP transport for optimal LLM API performance and reliability
	transport.MaxIdleConns = 100                            // Maintain connection pool for reuse
	transport.MaxIdleConnsPerHost = 10                      // Prevent too many idle connections per host
	transport.MaxConnsPerHost = 15                          // Limit total connections per host
	transport.IdleConnTimeout = idleConnTimeout             // Close idle connections
	transport.ResponseHeaderTimeout = responseHeaderTimeout // Fail fast on slow responses, skip batch for retry
	transport.ExpectContinueTimeout = 1 * time.Second       // Quick timeout for 100-continue responses

	logger.Debug("Created LLM client",
		"provider", provider.Name,
		"model", provider.Model,
		"maxTokens", provider.MaxTokens)

	return &Client{
		provider:    provider,
		apiKey:      apiKey,
		projectInfo: projectInfo,
		formality:   formality,
		glossary:    appConfig.Glossary.Entries,
		config:      appConfig,
		providerFn:  providerFn,
		userContext: userContext,
		httpClient: &http.Client{
			Transport:     transport,
			CheckRedirect: nil,
			Jar:           nil,
			Timeout:       clientTimeout,
		},
	}, nil
}

// CallProvider makes a raw API call using the configured provider.
// This is used by domain services that need direct access to LLM providers.
func (c *Client) CallProvider(ctx context.Context, promptText string) (map[string]any, error) {
	return c.providerFn(ctx, c.httpClient, c.provider, c.apiKey, promptText)
}

// ProcessTranslations translates missing keys, grouped by locale pairs.
func (c *Client) ProcessTranslations(
	ctx context.Context,
	missing []types.MissingTranslation,
) ([]types.LocaleContent, error) {
	if err := ctx.Err(); err != nil {
		logger.Debug("Context error before translation", "error", err)
		return nil, fmt.Errorf("context error before translation: %w", err)
	}

	if len(missing) == 0 {
		logger.Debug("No missing translations to process")
		return []types.LocaleContent{}, nil
	}

	byPair := utils.GroupByLocalePair(missing)
	allResults, lastError := c.processLocalePairs(ctx, byPair)

	if lastError != nil {
		logger.Warn("Some translations failed but continuing with partial results",
			"successfulTranslations", len(allResults),
			"lastError", lastError)
		return allResults, lastError
	}

	logger.Info("All translations completed successfully",
		"totalResults", len(allResults),
		"localePairs", len(byPair))

	return allResults, nil
}

// processLocalePairs processes translation for each locale pair.
func (c *Client) processLocalePairs(
	ctx context.Context,
	byPair map[types.LocalePair][]types.MissingTranslation,
) ([]types.LocaleContent, error) {
	var allResults []types.LocaleContent
	var lastError error

	for pair, items := range byPair {
		if err := ctx.Err(); err != nil {
			logger.Debug("Context cancelled during pair processing",
				"pair", fmt.Sprintf("%s→%s", pair.Source, pair.Target),
				"error", err)
			return allResults, fmt.Errorf("context canceled during translation: %w", err)
		}

		results, err := c.translateLocalePair(ctx, pair, items)
		if err != nil {
			if !errors.Is(err, errs.ErrRetryable) {
				return allResults, err
			}
			lastError = err
			continue
		}

		allResults = append(allResults, results...)
	}

	return allResults, lastError
}

// translateLocalePair handles translation for a single locale pair.
func (c *Client) translateLocalePair(
	ctx context.Context,
	pair types.LocalePair,
	items []types.MissingTranslation,
) ([]types.LocaleContent, error) {
	debugKeys := extractDebugKeys(items)

	logger.Debug("Translating batch",
		"source", pair.Source,
		"target", pair.Target,
		"keys", len(items),
		"provider", c.provider.Name,
		"maxRetries", maxAPIRetries,
		"keyList", debugKeys)

	toTranslate := make(map[string]any)
	for _, item := range items {
		if err := SetValue(toTranslate, item.Path, item.SourceValue); err != nil {
			logger.Error("Failed to build translation structure",
				"path", strings.Join(item.Path, "."),
				"error", err)
			return nil, fmt.Errorf("failed to build translation structure: %w", err)
		}
	}

	startTime := time.Now()
	translated, err := c.translateWithRetry(ctx, toTranslate, pair.Source, pair.Target)
	duration := time.Since(startTime)

	if err != nil {
		logger.Error("Translation failed for locale pair",
			"source", pair.Source,
			"target", pair.Target,
			"duration", duration,
			"retryable", errors.Is(err, errs.ErrRetryable),
			"error", err,
			"failedKeys", debugKeys)
		return nil, err
	}

	results := files.ExtractTranslations(translated, []string{}, pair.Target)
	logger.Debug("Translation completed for locale pair",
		"source", pair.Source,
		"target", pair.Target,
		"inputKeys", len(items),
		"outputKeys", len(results),
		"duration", duration)

	return results, nil
}

func (c *Client) createTranslationPrompt(
	sourceLocale types.Locale,
	targetLocale types.Locale,
	toTranslate map[string]any,
) (string, error) {
	promptText, err := CreateTranslationPrompt(
		sourceLocale,
		targetLocale,
		c.projectInfo,
		c.formality,
		toTranslate,
		c.glossary,
		c.userContext,
	)
	if err != nil {
		logger.Error("Failed to create prompt",
			"source", sourceLocale,
			"target", targetLocale,
			"error", err)
		return "", fmt.Errorf("failed to create prompt: %w", err)
	}
	return promptText, nil
}

func (c *Client) executeAPICall(
	ctx context.Context,
	promptText string,
	toTranslate map[string]any,
	sourceLocale types.Locale,
	targetLocale types.Locale,
	attempt int,
) (map[string]any, error) {
	logger.Debug("Making API call",
		"attempt", attempt,
		"provider", c.provider.Name,
		"source", sourceLocale,
		"target", targetLocale)

	callStart := time.Now()
	translated, err := c.providerFn(ctx, c.httpClient, c.provider, c.apiKey, promptText)
	callDuration := time.Since(callStart)

	if err != nil {
		logger.Warn("API call failed",
			"attempt", attempt,
			"provider", c.provider.Name,
			"model", c.provider.Model,
			"source", sourceLocale,
			"target", targetLocale,
			"duration", callDuration,
			"retryable", errors.Is(err, errs.ErrRetryable),
			"error", err)
		return nil, err
	}

	// Only validate LLM providers, since translators don't have LLM-style hallucination issues
	if constants.IsLLMProvider(c.provider.Name) {
		if err := validateLLMResponse(toTranslate, translated); err != nil {
			logger.Error("API response failed validation",
				"attempt", attempt,
				"provider", c.provider.Name,
				"source", sourceLocale,
				"target", targetLocale,
				"duration", callDuration,
				"error", err)
			return nil, errs.NewTranslationError(c.provider.Name, "", err)
		}
	}

	logger.Debug("API call successful",
		"attempt", attempt,
		"source", sourceLocale,
		"target", targetLocale,
		"duration", callDuration)

	if attempt > 0 {
		logger.Info("API retry succeeded",
			"finalAttempt", attempt,
			"provider", c.provider.Name,
			"duration", callDuration)
	}

	return translated, nil
}

func (c *Client) handleRetryDelay(
	ctx context.Context,
	attempt int,
	lastErr error,
	sourceLocale types.Locale,
	targetLocale types.Locale,
) error {
	delay := utils.CalculateRetryDelay(attempt, lastErr)
	logger.Info("Retrying API call",
		"attempt", attempt,
		"maxRetries", maxAPIRetries,
		"delay", delay,
		"source", sourceLocale,
		"target", targetLocale,
		"reason", lastErr)

	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context canceled during retry backoff: %w", ctx.Err())
	}
}

func (c *Client) translateWithRetry(
	ctx context.Context,
	toTranslate map[string]any,
	sourceLocale types.Locale,
	targetLocale types.Locale,
) (map[string]any, error) {
	var lastErr error

	promptText, err := c.createTranslationPrompt(sourceLocale, targetLocale, toTranslate)
	if err != nil {
		return nil, err
	}

	for attempt := 0; attempt <= maxAPIRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			logger.Debug("Context cancelled during retry",
				"attempt", attempt,
				"error", err)
			return nil, fmt.Errorf("context canceled during retry: %w", err)
		}

		if attempt > 0 {
			if err := c.handleRetryDelay(ctx, attempt, lastErr, sourceLocale, targetLocale); err != nil {
				return nil, err
			}
		}

		translated, err := c.executeAPICall(ctx, promptText, toTranslate, sourceLocale, targetLocale, attempt)
		if err == nil {
			return translated, nil
		}

		if !errors.Is(err, errs.ErrRetryable) {
			logger.Error("Non-retryable API error",
				"provider", c.provider.Name,
				"source", sourceLocale,
				"target", targetLocale,
				"finalAttempt", attempt,
				"error", err)
			return nil, err
		}

		lastErr = err
	}

	logger.Error("API call failed after all retries",
		"provider", c.provider.Name,
		"model", c.provider.Model,
		"source", sourceLocale,
		"target", targetLocale,
		"maxRetries", maxAPIRetries,
		"lastError", lastErr)

	return nil, lastErr
}

// extractDebugKeys extracts path strings from missing translations for logging.
func extractDebugKeys(items []types.MissingTranslation) []string {
	keys := make([]string, len(items))
	for i, item := range items {
		keys[i] = strings.Join(item.Path, ".")
	}
	if len(keys) > maxDebugKeys {
		keys = append(keys[:maxDebugKeys], "…")
	}
	return keys
}
