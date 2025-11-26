package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/juliandreas/dire/internal/check"
	"github.com/juliandreas/dire/internal/cli/ui"
	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/engine"
	"github.com/juliandreas/dire/internal/engine/rephrase"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/events"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/flags"
	"github.com/juliandreas/dire/internal/llm/registry"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/wire"
	"github.com/spf13/cobra"
)

const (
	translationTimeoutMinutes = 10 // Timeout for translation operations
	checkTimeoutSeconds       = 30 // Timeout for lint checks (file I/O only)
)

var (
	// ErrMissingTranslations indicates missing translations were detected during check.
	ErrMissingTranslations = errors.New("missing translations detected")
	// ErrCheckFailed indicates the check command failed for other reasons.
	ErrCheckFailed = errors.New("check failed")
	// ErrRephraseInvalidArgs indicates invalid arguments for rephrase command.
	ErrRephraseInvalidArgs = errors.New("rephrase requires exactly one --keys entry and a non-empty --locale")
)

// RunCommand is the main entry point for the commands.
func RunCommand(cmd *cobra.Command, args []string) error {
	// Setup application state
	flagsInstance, appConfig, err := setupApplicationState(cmd)
	if err != nil {
		return err
	}

	// Show banner unless we're producing machine-readable JSON in check mode
	isJSONCheck := flagsInstance.Check && strings.EqualFold(flagsInstance.Format, constants.FormatJSON)
	if !isJSONCheck {
		ui.DisplayAppVersion()
	}

	// Handle check mode
	if flagsInstance.Check {
		ctx, cleanup := setupContext(time.Duration(checkTimeoutSeconds)*time.Second, flagsInstance)
		exitCode := executeCheck(ctx, appConfig, flagsInstance)
		cleanup()

		// Return different errors based on exit code for main() to handle
		switch exitCode {
		case constants.ExitCodeSuccess:
			return nil
		case constants.ExitCodeWarning:
			return ErrMissingTranslations
		default:
			return fmt.Errorf("check failed with exit code %d: %w", exitCode, ErrCheckFailed)
		}
	}

	// Set up context with timeout and signal handling for long-running operations
	timeout := time.Duration(translationTimeoutMinutes) * time.Minute
	ctx, cleanup := setupContext(timeout, flagsInstance)
	defer cleanup()

	// Handle prune mode
	if flagsInstance.Prune {
		return executePrune(ctx, appConfig, flagsInstance)
	}

	// Handle rephrase mode
	if flagsInstance.Rephrase {
		return executeRephrase(ctx, appConfig, flagsInstance)
	}

	// Execute normal translation
	metrics, timer, err := executeTranslation(ctx, appConfig, flagsInstance)
	if err != nil {
		return err
	}

	// Display completion
	isSourcedMode := flagsInstance.SourcedOnly || flagsInstance.GlossaryOnly || flagsInstance.MemoryOnly
	ui.DisplayTranslationCompletion(
		ctx,
		metrics,
		timer,
		appConfig,
		isSourcedMode,
		flagsInstance.RequiresTranslationService,
	)
	return nil
}

// setupApplicationState handles parsing flags, loading config, applying overrides, and validation.
func setupApplicationState(cmd *cobra.Command) (*flags.Flags, *config.Config, error) {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Parse flags
	flagsInstance, err := flags.ParseFromCobraCommand(cmd)
	if err != nil {
		return nil, nil, err
	}

	// Load configuration
	appConfig, err := loadConfigurationFile(workDir)
	if err != nil {
		return nil, nil, err
	}

	// Apply flag overrides to config
	applyFlagOverrides(appConfig, flagsInstance)

	// Validate flag combinations
	if validationErr := flags.ValidateFlagCombinations(flagsInstance, appConfig); validationErr != nil {
		return nil, nil, validationErr
	}

	// Validate configuration
	if validationErr := appConfig.Validate(flagsInstance.RequiresTranslationService); validationErr != nil {
		return nil, nil, config.FormatValidationError(validationErr)
	}

	return flagsInstance, appConfig, nil
}

// loadConfigurationFile loads the application configuration file without validation.
func loadConfigurationFile(workDir string) (*config.Config, error) {
	// Find config file
	configPath, err := config.LocateConfigFile(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to locate config: %w", err)
	}

	// If no config file exists, return empty config to be populated by CLI flags
	if configPath == "" {
		logger.Debug("No configuration file found, using CLI flag overrides only")
		//nolint:exhaustruct // Empty config intentionally returned to be populated by CLI flag overrides
		cfg := &config.Config{}
		// Initialize map to avoid nil pointer issues
		cfg.Files.Locales = make(map[string]string)
		return cfg, nil
	}

	// Load configuration from file
	appConfig, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	logger.Debug("Configuration loaded successfully",
		"activeProvider", appConfig.Providers.Active,
		"localesCount", len(appConfig.Files.Locales))

	return appConfig, nil
}

// applyFlagOverrides applies command-line flag overrides to the config.
func applyFlagOverrides(appConfig *config.Config, flagsInstance *flags.Flags) {
	appConfig.ApplyDirectoryOverride(flagsInstance.Directory)
	appConfig.ApplyLocaleOverrides(flagsInstance.Locales)
	appConfig.ApplyIndentOverride(flagsInstance.Indent)
	appConfig.ApplyFormalityOverride(flagsInstance.Formality)
	appConfig.ApplyProjectNameOverride(flagsInstance.ProjectName)
	appConfig.ApplyProjectDescriptionOverride(flagsInstance.ProjectDesc)
	appConfig.ApplyProjectDomainOverride(flagsInstance.ProjectDom)
	appConfig.ApplyProviderActiveOverride(flagsInstance.ProviderActive)
	appConfig.ApplyProviderModelOverride(flagsInstance.ProviderModel)
	appConfig.ApplyProviderTemperatureOverride(flagsInstance.ProviderTemperature)
	appConfig.ApplyProviderMaxTokensOverride(flagsInstance.ProviderMaxTokens)
}

// setupContext creates a context with timeout and signal handling.
func setupContext(
	timeout time.Duration, flagsInstance *flags.Flags,
) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)

	// Set up keyboard listener only when not logging and translation service is needed
	// Raw terminal mode interferes with log output and is only relevant for interactive translation mode
	keyboardCtx, keyboardCancel := context.WithCancel(sigCtx)
	if !flagsInstance.IsLogging && flagsInstance.RequiresTranslationService {
		ui.StartKeyboardListener(keyboardCtx, keyboardCancel)
	}

	// Return a cleanup function that calls all cancel functions
	cleanup := func() {
		keyboardCancel()
		stop()
		cancel()
	}

	return keyboardCtx, cleanup
}

// executeTranslation runs the translation process with progress tracking.
func executeTranslation(
	ctx context.Context,
	appConfig *config.Config,
	flagsInstance *flags.Flags,
) (*engine.Metrics, *ui.Timer, error) {
	// Create base repository
	baseRepository := files.NewRepository(appConfig, flagsInstance)

	// Apply filtering wrapper if keys provided
	fileRepository := files.NewFilteredRepository(baseRepository, flagsInstance.Keys)

	// Create translation service
	translationService, _, err := wire.Dependencies(
		appConfig, flagsInstance, flagsInstance.RequiresTranslationService, fileRepository)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize application (requiresTranslationService=%t): %w",
			flagsInstance.RequiresTranslationService, err)
	}

	// Initialize file scanning
	localeFiles, err := fileRepository.Scan(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize files: %w", err)
	}

	// Determine missing translations and reference locale
	missing, referenceLocale, err := fileRepository.DetermineMissing(ctx, localeFiles, flagsInstance.IncludeStubs)
	if err != nil {
		if errors.Is(err, errs.ErrInvalidKeys) {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("failed to determine missing translations: %w", err)
	}

	// Create timer and progress callback
	timer := ui.NewTimer(flagsInstance.IsLogging)
	timer.Start(ctx)

	progressCallback := func(event *events.Event) {
		ui.DisplayTranslationProgress(ctx, appConfig, event, timer)
	}

	// Execute translation with pre-initialized data
	metrics, err := translationService.Run(
		ctx, localeFiles, missing, referenceLocale, progressCallback)
	if err != nil {
		timer.Stop()
		logger.Error("Translation failed", "error", err)
		return nil, timer, err
	}

	return metrics, timer, nil
}

// executeRephrase handles the rephrase mode.
func executeRephrase(
	ctx context.Context,
	appConfig *config.Config,
	flagsInstance *flags.Flags,
) error {
	if len(flagsInstance.Keys) != 1 || strings.TrimSpace(flagsInstance.Locale) == "" {
		return fmt.Errorf("rephrase validation failed: %w", ErrRephraseInvalidArgs)
	}

	logger.Info("Starting rephrase process",
		"key", flagsInstance.Keys[0],
		"locale", flagsInstance.Locale)

	// Create LLM client for rephrase
	llmClient, err := registry.NewClientFromConfig(appConfig, flagsInstance)
	if err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Create file repository for rephrase service
	repository := files.NewRepository(appConfig, flagsInstance)

	// Create rephrase service directly
	rephraseService := rephrase.NewService(llmClient, appConfig, repository, flagsInstance.Context)

	// Create timer for rephrase processing
	timer := ui.NewTimer(flagsInstance.IsLogging)
	timer.Start(ctx)

	// Generate rephrase options
	result, err := rephraseService.Generate(ctx, flagsInstance.Keys[0], flagsInstance.Locale)
	if err != nil {
		timer.Stop()
		return err
	}

	// Stop timer after successful generation
	timer.Stop()

	// Display rephrase options
	ui.DisplayRephrases(result)

	logger.Info("Rephrase process completed successfully",
		"key", flagsInstance.Keys[0],
		"locale", flagsInstance.Locale,
		"rephraseCount", len(result.Rephrases))

	return nil
}

// executePrune handles the prune mode.
func executePrune(
	ctx context.Context,
	appConfig *config.Config,
	flagsInstance *flags.Flags,
) error {
	// Create file repository for prune
	repository := files.NewRepository(appConfig, flagsInstance)

	// Initialize file scanning
	localeFiles, err := repository.Scan(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize files: %w", err)
	}

	// Prune orphaned keys
	pruneResults, err := files.Prune(
		ctx, repository, localeFiles, flagsInstance.TrimStrings, appConfig.GetIndent(),
	)
	if err != nil {
		return err
	}

	// Display results
	ui.DisplayPruneResults(pruneResults)

	return nil
}

// executeCheck handles the lint check mode and returns the exit code.
func executeCheck(
	ctx context.Context,
	appConfig *config.Config,
	flagsInstance *flags.Flags,
) int {
	logger.Info("Starting lint check",
		"includeStubs", flagsInstance.IncludeStubs,
		"format", flagsInstance.Format,
		"keysCount", len(flagsInstance.Keys))

	// Create file repository for check service
	repository := files.NewRepository(appConfig, flagsInstance)

	// Apply filtering wrapper if keys provided
	fileRepository := files.NewFilteredRepository(repository, flagsInstance.Keys)

	// Create check service
	checkService := check.NewService(appConfig, flagsInstance, fileRepository)

	// Run linting
	result, err := checkService.Run(ctx)
	if err != nil {
		return constants.ExitCodeError // Configuration or system errors
	}

	// Output result in requested format and get exit code
	isJSONFormat := flagsInstance.Format == constants.FormatJSON
	exitCode := checkService.OutputResult(result, isJSONFormat)

	return exitCode
}
