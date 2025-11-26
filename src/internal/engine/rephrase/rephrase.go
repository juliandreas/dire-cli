package rephrase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/llm"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
)

// Service handles rephrase generation.
type Service struct {
	llmClient   *llm.Client
	config      *config.Config
	repository  files.Repository
	userContext string
}

// Result contains the results of a rephrase generation request.
type Result struct {
	Key          string
	Locale       types.Locale
	CurrentValue string
	Rephrases    []string
}

var (
	ErrNoRephraseOptions   = errors.New("no rephrase options found in response")
	ErrKeyNotFound         = errors.New("key not found")
	ErrNoContentToRephrase = errors.New("key has no content to rephrase")
)

// NewService creates a new rephrase service.
func NewService(
	llmClient *llm.Client,
	cfg *config.Config,
	repository files.Repository,
	userContext string,
) *Service {
	logger.Debug("Created rephrase service")
	return &Service{
		llmClient:   llmClient,
		config:      cfg,
		repository:  repository,
		userContext: userContext,
	}
}

// Generate generates rephrase options for a single key in a single locale.
func (s *Service) Generate(
	ctx context.Context,
	key string,
	locale string,
) (*Result, error) {
	logger.Info("Generating rephrases", "key", key, "locale", locale)

	// Validate requirements
	if err := validateRequirements(s.config, locale); err != nil {
		return nil, err
	}

	// Convert locale string to types.Locale
	targetLocale := s.config.ResolveLocaleCode(locale)

	// Get the current value for the key
	currentValue, err := s.getCurrentValue(ctx, key, targetLocale)
	if err != nil {
		return nil, fmt.Errorf("failed to get current value: %w", err)
	}

	// Validate that there's actual content in the reference locale to rephrase
	if strings.TrimSpace(currentValue) == "" {
		return nil, ErrNoContentToRephrase
	}

	// Generate rephrases using LLM directly
	rephrases, err := s.generateOptions(ctx, key, currentValue, targetLocale)
	if err != nil {
		return nil, err
	}

	logger.Info("Generated rephrase options",
		"key", key,
		"locale", locale,
		"rephraseCount", len(rephrases))

	return &Result{
		Key:          key,
		Locale:       targetLocale,
		CurrentValue: currentValue,
		Rephrases:    rephrases,
	}, nil
}

// validateRequirements validates basic rephrase requirements.
func validateRequirements(appConfig *config.Config, locale string) error {
	// Only validate that the locale exists if specified
	if locale != "" {
		// Check if file locale exists
		if !appConfig.HasFileLocale(locale) {
			// Locale not found - create helpful error message with available file locales
			availableLocales := appConfig.GetFileLocales()
			return errs.NewLocaleNotFoundError(locale, availableLocales)
		}
	}

	return nil
}

// getCurrentValue retrieves the current translation value for a key in a locale.
func (s *Service) getCurrentValue(ctx context.Context, key string, locale types.Locale) (string, error) {
	// Get locale content for the specific key
	content, err := s.repository.GetFilteredContentForLocale(ctx, locale)
	if err != nil {
		return "", err
	}

	// Find the specific key in the content
	for _, localeContent := range content {
		// Convert path array to dot-separated key
		currentKey := strings.Join(localeContent.Path, ".")
		if currentKey == key {
			// Convert value to string
			if str, ok := localeContent.Value.(string); ok {
				return str, nil
			}
			return fmt.Sprintf("%v", localeContent.Value), nil
		}
	}

	return "", ErrKeyNotFound
}

// generateOptions generates rephrase options.
func (s *Service) generateOptions(
	ctx context.Context,
	key string,
	currentValue string,
	locale types.Locale,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("rephrase cancelled: %w", err)
	}

	logger.Debug("Generating rephrase options",
		"key", key,
		"locale", locale,
		"currentValue", currentValue)

	// Create rephrase prompt
	resolvedLocale := s.config.ResolveLocaleCode(string(locale))
	promptText, err := llm.CreateRephrasePrompt(
		key,
		currentValue,
		resolvedLocale,
		s.config.BuildProjectInfo(),
		s.userContext,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create rephrase prompt: %w", err)
	}

	// Execute API call using LLM client
	translated, err := s.llmClient.CallProvider(ctx, promptText)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("rephrase cancelled: %w", ctx.Err())
		}
		logger.Error("API call failed for rephrase options",
			"key", key,
			"locale", locale,
			"error", err)
		return nil, fmt.Errorf("failed to generate rephrase options: %w", err)
	}

	// Parse options from response
	options, err := s.parseOptions(translated)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rephrase options: %w", err)
	}

	logger.Info("Generated rephrase options successfully",
		"key", key,
		"locale", locale,
		"optionCount", len(options))

	return options, nil
}

// parseOptions converts standard translation response to string slice.
func (s *Service) parseOptions(translated map[string]any) ([]string, error) {
	var options []string
	var keys []string

	// Collect keys first, only for string values
	for key, value := range translated {
		if _, ok := value.(string); ok {
			keys = append(keys, key)
		}
	}

	// Sort keys to preserve intended order (suggestion1, suggestion2, etc.)
	sort.Strings(keys)

	// Extract values in sorted order
	for _, key := range keys {
		if str, ok := translated[key].(string); ok {
			options = append(options, str)
		}
	}

	if len(options) == 0 {
		return nil, ErrNoRephraseOptions
	}

	return options, nil
}
