package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/engine/sourced"
	"github.com/juliandreas/dire/internal/engine/stub"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/events"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/flags"
	"github.com/juliandreas/dire/internal/llm"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/translator"
	"github.com/juliandreas/dire/internal/types"
)

// Metrics contains information about the translation process.
type Metrics struct {
	SourcedTranslatedKeys int
	TranslatedKeys        int // Keys translated by LLM or translation service (e.g., DeepL)
	StubTranslatedKeys    int
	RemainingKeys         int
}

// Service handles translation operations.
type Service struct {
	llmClient        *llm.Client
	translatorClient *translator.Client
	config           *config.Config
	flags            *flags.Flags
	maxTokens        int
	repository       files.Repository
}

const (
	warningThreshold = 200 // Maximum number of missing translations to warn about
)

// NewService creates a new translation service.
func NewService(
	llmClient *llm.Client,
	translatorClient *translator.Client,
	cfg *config.Config,
	flagsInstance *flags.Flags,
	maxTokens int,
	repository files.Repository,
) *Service {
	logger.Debug("Created translation service",
		"maxTokens", maxTokens,
		"provider", cfg.Providers.Active)
	if flagsInstance == nil {
		flagsInstance = flags.New()
	}
	return &Service{
		llmClient:        llmClient,
		translatorClient: translatorClient,
		config:           cfg,
		flags:            flagsInstance,
		maxTokens:        maxTokens,
		repository:       repository,
	}
}

// Run handles the main translation logic with pre-initialized files.
func (s *Service) Run(
	ctx context.Context,
	localeFiles map[types.Locale]*types.LocaleFile,
	missing []types.MissingTranslation,
	referenceLocale types.Locale,
	progress events.Callback,
) (*Metrics, error) {
	logger.Info("Starting translation process")
	startTime := time.Now()

	metrics := &Metrics{
		SourcedTranslatedKeys: 0,
		TranslatedKeys:        0,
		StubTranslatedKeys:    0,
		RemainingKeys:         0,
	}

	// Early exit if no translations needed
	if len(missing) == 0 {
		logger.Info("No missing translations found")
		// Still format all locale files for consistency
		if err := s.repository.FinalCleanup(ctx); err != nil {
			logger.Error("Failed to format locale files", "error", err)
			return metrics, fmt.Errorf("format locale files: %w", err)
		}
		return metrics, nil
	}

	// Handle translations based on mode
	if err := s.executeTranslationMode(
		ctx, missing, localeFiles, referenceLocale, progress, metrics); err != nil {
		return metrics, err
	}

	// Finalize translation process
	if err := s.finalizeTranslation(ctx, metrics); err != nil {
		return metrics, err
	}

	totalDuration := time.Since(startTime)
	logger.Info("Translation process completed",
		"totalDuration", totalDuration,
		"sourcedKeys", metrics.SourcedTranslatedKeys,
		"translatedKeys", metrics.TranslatedKeys,
		"stubKeys", metrics.StubTranslatedKeys,
		"remainingKeys", metrics.RemainingKeys)

	return metrics, nil
}

// executeTranslationMode handles translations based on the mode.
func (s *Service) executeTranslationMode(
	ctx context.Context,
	missing []types.MissingTranslation,
	localeFiles map[types.Locale]*types.LocaleFile,
	referenceLocale types.Locale,
	progress events.Callback,
	metrics *Metrics,
) error {
	// Handle stub mode (skips all other modes)
	if s.flags.StubMode {
		return s.handleStubTranslations(ctx, missing, localeFiles, referenceLocale, metrics)
	}

	// Handle sourced translations
	if err := s.handleSourcedTranslations(ctx, missing, localeFiles, referenceLocale, metrics); err != nil {
		return err
	}

	// If any sourced-only mode is active, skip translation services
	if s.flags.SourcedOnly || s.flags.GlossaryOnly || s.flags.MemoryOnly {
		logger.Info("Skipping external translation services due to sourced-only mode",
			"sourcedOnly", s.flags.SourcedOnly,
			"glossaryOnly", s.flags.GlossaryOnly,
			"memoryOnly", s.flags.MemoryOnly)
		return nil
	}

	// Skip only when no client is configured at all
	if s.llmClient == nil && s.translatorClient == nil {
		logger.Info("Skipping translation services: no client configured")
		return nil
	}

	// Get remaining translations after sourced translations (including stubs if requested)
	var remaining []types.MissingTranslation
	var err error

	if s.flags.IncludeStubs {
		remaining, err = s.repository.GetMissingWithStubs(ctx)
	} else {
		remaining, err = s.repository.GetMissing(ctx)
	}
	if err != nil {
		logger.Error("Failed to get remaining translations", "error", err)
		return fmt.Errorf("get remaining translations: %w", err)
	}

	// Handle translation services
	return s.handleTranslationServices(ctx, remaining, progress, metrics)
}

// handleSourcedTranslations processes pre-determined translations from memory and glossary.
func (s *Service) handleSourcedTranslations(
	ctx context.Context,
	missing []types.MissingTranslation,
	localeFiles map[types.Locale]*types.LocaleFile,
	referenceLocale types.Locale,
	metrics *Metrics,
) error {
	// Enable both by default; "only" flags turn the other source off; --sourced forces both on
	enableGlossary := s.flags.SourcedOnly || s.flags.GlossaryOnly || !s.flags.MemoryOnly
	enableMemory := s.flags.SourcedOnly || s.flags.MemoryOnly || !s.flags.GlossaryOnly

	logger.Debug("Sourced enablement decided",
		"glossary", enableGlossary,
		"memory", enableMemory,
		"flags", map[string]bool{
			"sourcedOnly":  s.flags.SourcedOnly,
			"glossaryOnly": s.flags.GlossaryOnly,
			"memoryOnly":   s.flags.MemoryOnly,
		})

	// Skip if neither is enabled
	if !enableGlossary && !enableMemory {
		return nil
	}

	sourcedCount, err := sourced.ApplySourced(
		ctx,
		missing,
		localeFiles,
		referenceLocale,
		s.config,
		enableGlossary,
		enableMemory,
	)
	if err != nil {
		logger.Error("Failed to apply sourced translations", "error", err)
		return err
	}
	metrics.SourcedTranslatedKeys = sourcedCount

	return nil
}

// handleTranslationServices processes remaining translations using AI providers or translation services.
func (s *Service) handleTranslationServices(
	ctx context.Context,
	remaining []types.MissingTranslation,
	progress events.Callback,
	metrics *Metrics,
) error {
	if len(remaining) == 0 {
		logger.Info("No translation-service work remaining")
		return nil
	}

	// Route strictly by configured provider; allow Active==""
	active := s.config.Providers.Active
	if constants.IsTranslatorProvider(active) {
		if s.translatorClient == nil {
			logger.Error("Active translator provider configured but translator client not initialized", "provider", active)
			return errs.NewMissingClientError("translator", active)
		}
		return s.handleTranslator(ctx, remaining, progress, metrics)
	}
	if constants.IsLLMProvider(active) {
		if s.llmClient == nil {
			logger.Error("Active LLM provider configured but LLM client not initialized", "provider", active)
			return errs.NewMissingClientError("LLM", active)
		}
		return s.handleLLM(ctx, remaining, progress, metrics)
	}
	// Active unset: choose deterministically by wired client (prefer translator)
	if active == "" {
		if s.translatorClient != nil {
			return s.handleTranslator(ctx, remaining, progress, metrics)
		}
		if s.llmClient != nil {
			return s.handleLLM(ctx, remaining, progress, metrics)
		}
		logger.Info("Skipping translation services: no client configured")
		return nil
	}
	// Active unknown/invalid: configuration error
	logger.Error("Unknown or unsupported provider", "provider", active)
	return errs.NewUnsupportedProviderError(active, constants.SupportedProviders())
}

// handleLLM processes translations using LLM providers.
func (s *Service) handleLLM(
	ctx context.Context,
	remaining []types.MissingTranslation,
	progress events.Callback,
	metrics *Metrics,
) error {
	if s.llmClient == nil {
		if constants.IsLLMProvider(s.config.Providers.Active) {
			return errs.NewMissingClientError("LLM", s.config.Providers.Active)
		}
		logger.Info("Skipping LLM translations: no LLM client configured")
		return nil
	}

	// Warning for large LLM translation jobs
	if len(remaining) > warningThreshold {
		logger.Warn("Large translation job detected",
			"remainingKeys", len(remaining),
			"threshold", warningThreshold)
		if progress != nil {
			progress(events.NewWarning("Large translation job detected. This might take a while, grab a coffee!"))
		}
	}

	logger.Info("Starting LLM translations", "remainingForLLM", len(remaining))

	// Apply LLM translations using LLM processor
	processor := llm.NewProcessor(s.llmClient, s.config, s.maxTokens, s.repository, progress)
	translatedCount, err := processor.ApplyTranslations(ctx, remaining)
	metrics.TranslatedKeys = translatedCount

	if err != nil {
		logger.Error("LLM translation failed", "translatedKeys", translatedCount, "error", err)
		return fmt.Errorf("llm translations failed after %d keys: %w", translatedCount, err)
	}

	logger.Info("LLM translations completed", "translatedKeys", translatedCount)
	return nil
}

// handleTranslator processes translations using translator services with retry logic.
func (s *Service) handleTranslator(
	ctx context.Context,
	remaining []types.MissingTranslation,
	progress events.Callback,
	metrics *Metrics,
) error {
	if s.translatorClient == nil {
		if constants.IsTranslatorProvider(s.config.Providers.Active) {
			return errs.NewMissingClientError("translator", s.config.Providers.Active)
		}
		logger.Info("Skipping translator: no client configured")
		return nil
	}

	logger.Info("Starting translator with processor", "remainingForTranslator", len(remaining))

	// Create processor for translation with type conflict resolution
	processor := translator.NewProcessor(s.translatorClient, s.config, s.repository, progress)

	// Apply translations using the processor
	translatedCount, err := processor.ApplyTranslations(ctx, remaining)
	metrics.TranslatedKeys = translatedCount

	if err != nil {
		logger.Error("Translator processor failed", "translatedKeys", translatedCount, "error", err)
		return fmt.Errorf("translator translations failed after %d keys: %w", translatedCount, err)
	}
	logger.Info("Translator completed", "translatedKeys", translatedCount)
	return nil
}

// handleStubTranslations processes stub translations.
func (s *Service) handleStubTranslations(
	ctx context.Context,
	initialMissing []types.MissingTranslation,
	localeFiles map[types.Locale]*types.LocaleFile,
	referenceLocale types.Locale,
	metrics *Metrics,
) error {
	currentMissing := initialMissing
	totalStubCount := 0

	// Keep processing until no more missing translations are found
	for len(currentMissing) > 0 {
		// Process current batch of missing translations
		stubCount, _, err := stub.ApplyStubs(
			ctx,
			currentMissing,
			localeFiles,
			referenceLocale,
			s.config,
		)
		if err != nil {
			logger.Error("Failed to apply stub translations", "error", err)
			return err
		}
		totalStubCount += stubCount

		logger.Info("Stub translations batch applied", "stubCount", stubCount)

		// Rescan for any remaining missing translations (bidirectional)
		remaining, err := s.repository.GetMissing(ctx)
		if err != nil {
			logger.Error("Failed to rescan remaining translations after stubs", "error", err)
			return fmt.Errorf("rescan after stubs: %w", err)
		}

		// Update for next iteration
		currentMissing = remaining
	}

	metrics.StubTranslatedKeys = totalStubCount
	logger.Info("All stub translations applied", "totalStubCount", totalStubCount)

	return nil
}

// finalizeTranslation performs final cleanup and calculates remaining translations.
func (s *Service) finalizeTranslation(ctx context.Context, metrics *Metrics) error {
	// Final cleanup (trims strings and ensures consistent formatting)
	if err := s.repository.FinalCleanup(ctx); err != nil {
		return err
	}

	// Check for any remaining untranslated keys
	finalRemaining, err := s.repository.GetMissing(ctx)
	if err != nil {
		logger.Warn("Failed to get final remaining count", "error", err)
	} else {
		metrics.RemainingKeys = len(finalRemaining)
	}

	return nil
}
