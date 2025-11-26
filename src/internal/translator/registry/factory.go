package registry

import (
	"os"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/translator"
)

// NewClientFromConfig creates a translator services client.
func NewClientFromConfig(appConfig *config.Config) (*translator.Client, error) {
	// Get provider configuration
	provider, err := appConfig.GetActiveProviderConfig()
	if err != nil {
		return nil, err
	}

	// Get API key from environment
	envKey := constants.GetEnvKeyForProvider(provider.Name)
	apiKey := os.Getenv(envKey)
	if apiKey == "" {
		logger.Error("API key not found",
			"provider", provider.Name,
			"envKey", envKey)
		return nil, errs.NewMissingAPIKeyError(envKey, provider.Name)
	}

	// Get provider function
	providerFn, err := GetProviderFunction(provider.Name)
	if err != nil {
		return nil, err
	}

	// Create translator services client
	client, err := translator.NewClient(appConfig, provider, apiKey, providerFn)
	if err != nil {
		return nil, err
	}

	return client, nil
}
