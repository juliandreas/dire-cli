package registry

import (
	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/flags"
	"github.com/juliandreas/dire/internal/llm"
)

// NewClientFromConfig creates just the LLM client for use by standalone services.
func NewClientFromConfig(
	appConfig *config.Config,
	flagsInstance *flags.Flags,
) (*llm.Client, error) {
	// Resolve defaults
	defaults := config.ResolveDefaults(appConfig)

	// Get provider configuration
	provider, err := appConfig.GetActiveProviderConfig()
	if err != nil {
		return nil, err
	}

	// Get provider function
	providerFn, err := GetProviderFunction(provider.Name)
	if err != nil {
		return nil, err
	}

	// Create LLM client with safe context handling
	var userContext string
	if flagsInstance != nil {
		userContext = flagsInstance.Context
	}
	llmClient, err := llm.NewClient(appConfig, defaults.Formality, provider, providerFn, userContext)
	if err != nil {
		return nil, err
	}

	return llmClient, nil
}
