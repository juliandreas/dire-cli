package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/translator"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

const (
	azureTranslateEndpoint = "https://api.cognitive.microsofttranslator.com/translate"
	azureAPIVersion        = "3.0"
)

// CallAzureTranslate translates texts using Azure AI Translator.
func CallAzureTranslate(
	ctx context.Context,
	httpClient *http.Client,
	provider config.Provider,
	apiKey string,
	texts []string,
	sourceLang, targetLang string,
	formality constants.Formality,
) ([]string, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Build request body
	reqBody := buildAzureRequest(texts)

	// Build endpoint with query parameters
	sourceAzure := localeToAzureLanguage(types.Locale(sourceLang))
	targetAzure := localeToAzureLanguage(types.Locale(targetLang))
	endpoint := fmt.Sprintf("%s?api-version=%s&from=%s&to=%s&textType=html",
		azureTranslateEndpoint, azureAPIVersion, sourceAzure, targetAzure)

	// Create request with Azure headers
	req, err := utils.CreateRequest(ctx, "POST", endpoint, map[string]string{
		"Ocp-Apim-Subscription-Key":    apiKey,
		"Ocp-Apim-Subscription-Region": provider.Region,
	}, reqBody)
	if err != nil {
		return nil, err
	}

	// Handle context cancellation
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, errs.ErrContextCanceled
	}

	// Execute request
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errs.NewAPIError(constants.ProviderAzure, err)
	}

	// Process response and close body
	result, processErr := processAzureResponse(resp, texts)
	if resp.Body != nil {
		// Drain the body to enable connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	return result, processErr
}

// buildAzureRequest creates the request body for the Azure Translator API call.
func buildAzureRequest(texts []string) []map[string]string {
	// Wrap texts with span tags to protect variables from translation
	wrappedTexts := make([]map[string]string, len(texts))
	for idx, text := range texts {
		wrappedTexts[idx] = map[string]string{
			"Text": translator.WrapVariablesWithSpan(text),
		}
	}

	return wrappedTexts
}

// processAzureResponse processes the response and returns translations or an error.
func processAzureResponse(resp *http.Response, texts []string) ([]string, error) {
	// Handle non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, utils.HandleError(resp, constants.ProviderAzure)
	}

	// Parse response and validate
	result, parseErr := parseAzureResponse(resp)
	if parseErr != nil {
		return nil, parseErr
	}

	// Validate response
	if validationErr := translator.ValidateTranslations(result, texts); validationErr != nil {
		return nil, utils.NewHTTPProviderError(
			http.StatusUnprocessableEntity, constants.ProviderAzure, validationErr.Error())
	}

	return result, nil
}

// parseAzureResponse parses Azure Translator API response and extracts translated texts.
func parseAzureResponse(resp *http.Response) ([]string, error) {
	var apiResp []struct {
		Translations []struct {
			Text string `json:"text"`
			To   string `json:"to"`
		} `json:"translations"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	if len(apiResp) == 0 {
		return nil, utils.NewHTTPProviderError(
			http.StatusUnprocessableEntity,
			constants.ProviderAzure,
			"empty response from API")
	}

	// Extract translated texts and unwrap span tags
	translations := make([]string, len(apiResp))
	for idx, item := range apiResp {
		if len(item.Translations) == 0 {
			return nil, utils.NewHTTPProviderError(
				http.StatusUnprocessableEntity,
				constants.ProviderAzure,
				"empty translations in API response")
		}
		translations[idx] = translator.UnwrapSpanTags(item.Translations[0].Text)
	}

	return translations, nil
}

// localeToAzureLanguage converts locale format to Azure language code.
// Azure accepts standard language codes like "en", "es", "zh-Hans".
func localeToAzureLanguage(locale types.Locale) string {
	return translator.NormalizeLocale(locale)
}
