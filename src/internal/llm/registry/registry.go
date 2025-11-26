package registry

import (
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/llm"
	"github.com/juliandreas/dire/internal/llm/providers"
)

// GetProviderFunction returns the appropriate provider function for the given provider name.
func GetProviderFunction(providerName string) (llm.ProviderFunc, error) {
	providerFunctions := map[string]llm.ProviderFunc{
		constants.ProviderClaude:   providers.CallClaude,
		constants.ProviderOpenAI:   providers.CallOpenAI,
		constants.ProviderGemini:   providers.CallGemini,
		constants.ProviderMistral:  providers.CallMistral,
		constants.ProviderDeepSeek: providers.CallDeepSeek,
	}
	providerFn, exists := providerFunctions[providerName]
	if !exists {
		return nil, errs.NewUnsupportedProviderError(providerName, constants.SupportedProviders())
	}
	return providerFn, nil
}
