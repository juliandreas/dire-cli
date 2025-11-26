package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/translator"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

const (
	defaultFormality = "default"
	preferMore       = "prefer_more"
	preferLess       = "prefer_less"
)

// Unwrap pattern for var tags.
var deeplUnwrapPatternRe = regexp.MustCompile(`<var>([^<]+)</var>`)

// CallDeepL translates texts using DeepL API.
func CallDeepL(
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
	reqBody := buildDeepLRequest(texts, sourceLang, targetLang, formality)
	endpoint := getDeepLEndpoint(apiKey)

	// Create request
	req, err := utils.CreateRequest(ctx, "POST", endpoint, map[string]string{
		"Authorization": "DeepL-Auth-Key " + apiKey,
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
		return nil, errs.NewAPIError(constants.ProviderDeepL, err)
	}

	// Process response and close body
	result, processErr := processResponse(resp, texts)
	if resp.Body != nil {
		// Drain the body to enable connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	return result, processErr
}

// buildDeepLRequest creates the request body for the DeepL API call.
func buildDeepLRequest(texts []string, sourceLang, targetLang string, formality constants.Formality) map[string]any {
	// Convert locale codes to DeepL language codes
	sourceDeepL := localeToDeepLLanguage(types.Locale(sourceLang))
	targetDeepL := localeToDeepLLanguage(types.Locale(targetLang))

	// Wrap texts with XML tags to protect variables
	wrappedTexts := make([]string, len(texts))
	for i, text := range texts {
		wrappedTexts[i] = deeplWrapVariables(text)
	}

	// Build request body
	reqBody := map[string]any{
		"text":                wrappedTexts,
		"source_lang":         sourceDeepL,
		"target_lang":         targetDeepL,
		"tag_handling":        "xml",
		"ignore_tags":         []string{"var"},
		"preserve_formatting": true,
	}

	// Add formality if not automatic (default)
	if formality != constants.FormalityAutomatic {
		formalityParam := mapFormalityToDeepL(formality)
		if formalityParam != defaultFormality {
			reqBody["formality"] = formalityParam
		}
	}

	return reqBody
}

// mapFormalityToDeepL converts our formality constants to DeepL API parameters.
// Uses prefer_ variants to ensure compatibility with all languages.
func mapFormalityToDeepL(formality constants.Formality) string {
	switch formality {
	case constants.FormalityFormal:
		return preferMore
	case constants.FormalityInformal:
		return preferLess
	default: // constants.FormalityAutomatic or any other value
		return defaultFormality
	}
}

// processResponse processes the response and returns translations or an error.
func processResponse(resp *http.Response, texts []string) ([]string, error) {
	// Handle non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, utils.HandleError(resp, constants.ProviderDeepL)
	}

	// Parse response and validate
	result, parseErr := parseDeepLResponse(resp)
	if parseErr != nil {
		return nil, parseErr
	}

	// Validate response
	if validationErr := translator.ValidateTranslations(result, texts); validationErr != nil {
		return nil, utils.NewHTTPProviderError(
			http.StatusUnprocessableEntity, constants.ProviderDeepL, validationErr.Error())
	}

	return result, nil
}

// parseDeepLResponse parses DeepL API response and extracts translated texts.
func parseDeepLResponse(resp *http.Response) ([]string, error) {
	var apiResp struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	if len(apiResp.Translations) == 0 {
		return nil, utils.NewHTTPProviderError(
			http.StatusUnprocessableEntity,
			constants.ProviderDeepL,
			"empty translations in API response")
	}

	// Extract translated texts and unwrap var tags
	translations := make([]string, len(apiResp.Translations))
	for i, t := range apiResp.Translations {
		translations[i] = deeplUnwrapTags(t.Text)
	}

	return translations, nil
}

// deeplWrapVariables wraps variables and placeholders with var tags to protect them.
func deeplWrapVariables(text string) string {
	return translator.VariablePatternRe.ReplaceAllStringFunc(text, func(match string) string {
		return "<var>" + match + "</var>"
	})
}

// deeplUnwrapTags removes the var tags we added to protect variables.
func deeplUnwrapTags(text string) string {
	return deeplUnwrapPatternRe.ReplaceAllString(text, "$1")
}

// localeToDeepLLanguage converts locale format to DeepL language code.
// DeepL uses simple uppercase language codes (e.g., "en-US" → "EN", "da-DK" → "DA").
// Handles both hyphen and underscore separators.
func localeToDeepLLanguage(locale types.Locale) string {
	// Normalize locale first (handles underscores: "en_US" → "en-us")
	normalized := translator.NormalizeLocale(locale)
	// Extract language code and uppercase (e.g., "en-us" → "en" → "EN")
	return strings.ToUpper(translator.ExtractLanguageCode(types.Locale(normalized)))
}

// getDeepLEndpoint determines the correct DeepL API endpoint based on API key.
func getDeepLEndpoint(apiKey string) string {
	if strings.HasSuffix(apiKey, ":fx") {
		return "https://api-free.deepl.com/v2/translate"
	}
	return "https://api.deepl.com/v2/translate"
}
