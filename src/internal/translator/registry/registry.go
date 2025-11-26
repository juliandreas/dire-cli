package registry

import (
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/translator"
	"github.com/juliandreas/dire/internal/translator/providers"
)

// GetProviderFunction returns the appropriate translator function for the given provider name.
func GetProviderFunction(providerName string) (translator.ProviderFunc, error) {
	translatorFunctions := map[string]translator.ProviderFunc{
		constants.ProviderDeepL:  providers.CallDeepL,
		constants.ProviderGoogle: providers.CallGoogleTranslate,
		constants.ProviderAzure:  providers.CallAzureTranslate,
	}

	translatorFn, exists := translatorFunctions[providerName]
	if !exists {
		return nil, errs.NewUnsupportedProviderError(providerName, constants.SupportedProviders())
	}
	return translatorFn, nil
}
