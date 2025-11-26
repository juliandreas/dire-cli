// Package wire provides manual dependency wiring for the application.
// This file contains functions to create and wire together all
// application dependencies without using a DI framework
package wire

import (
	"fmt"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/engine"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/flags"
	"github.com/juliandreas/dire/internal/llm"
	llmRegistry "github.com/juliandreas/dire/internal/llm/registry"
	"github.com/juliandreas/dire/internal/translator"
	translatorRegistry "github.com/juliandreas/dire/internal/translator/registry"
)

// Dependencies creates and wires all application dependencies.
func Dependencies(
	appConfig *config.Config,
	flagsInstance *flags.Flags,
	requiresTranslationService bool,
	customRepository ...files.Repository,
) (*engine.Service, files.Repository, error) {
	var llmClient *llm.Client
	var translatorClient *translator.Client
	var maxTokens int

	if requiresTranslationService {
		var err error
		llmClient, translatorClient, maxTokens, err = createTranslationClients(appConfig, flagsInstance)
		if err != nil {
			return nil, nil, err
		}
	}

	// Use provided repository or create a new one
	var fileRepository files.Repository
	if len(customRepository) > 0 && customRepository[0] != nil {
		fileRepository = customRepository[0]
	} else {
		fileRepository = files.NewRepository(appConfig, flagsInstance)
	}

	// Create translation service
	translationService := engine.NewService(
		llmClient,
		translatorClient,
		appConfig,
		flagsInstance,
		maxTokens,
		fileRepository,
	)

	return translationService, fileRepository, nil
}

// createTranslationClients creates the appropriate translation clients based on provider type.
// Returns either an LLM client or translator client, but never both.
func createTranslationClients(
	appConfig *config.Config,
	flagsInstance *flags.Flags,
) (*llm.Client, *translator.Client, int, error) {
	// Get the active provider configuration
	provider, err := appConfig.GetActiveProviderConfig()
	if err != nil {
		return nil, nil, 0, fmt.Errorf("active provider config error: %w", err)
	}

	maxTokens := provider.MaxTokens

	// Route to appropriate client type based on provider
	if constants.IsTranslatorProvider(provider.Name) {
		// Create translator service client (e.g., DeepL, Google Translate, etc.)
		translatorClient, clientErr := translatorRegistry.NewClientFromConfig(appConfig)
		if clientErr != nil {
			return nil, nil, 0, clientErr
		}
		return nil, translatorClient, maxTokens, nil
	}

	// Create LLM client (e.g., Claude, OpenAI, Gemini)
	llmClient, clientErr := llmRegistry.NewClientFromConfig(appConfig, flagsInstance)
	if clientErr != nil {
		return nil, nil, 0, clientErr
	}
	return llmClient, nil, maxTokens, nil
}
