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
	googleTranslateEndpoint = "https://translation.googleapis.com/language/translate/v2"
)

// CallGoogleTranslate translates texts using Google Cloud Translation API.
func CallGoogleTranslate(
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
	reqBody := buildGoogleTranslateRequest(texts, sourceLang, targetLang)

	// Create request with API key as query parameter
	endpoint := fmt.Sprintf("%s?key=%s", googleTranslateEndpoint, apiKey)
	req, err := utils.CreateRequest(ctx, "POST", endpoint, map[string]string{}, reqBody)
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
		return nil, errs.NewAPIError(constants.ProviderGoogle, err)
	}

	// Process response and close body
	result, processErr := processGoogleResponse(resp, texts)
	if resp.Body != nil {
		// Drain the body to enable connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	return result, processErr
}

// buildGoogleTranslateRequest creates the request body for the Google Cloud Translation API call.
func buildGoogleTranslateRequest(texts []string, sourceLang, targetLang string) map[string]any {
	// Convert locale codes to Google language codes
	sourceGoogle := localeToGoogleLanguage(types.Locale(sourceLang))
	targetGoogle := localeToGoogleLanguage(types.Locale(targetLang))

	// Wrap texts with span tags to protect variables from translation
	wrappedTexts := make([]string, len(texts))
	for i, text := range texts {
		wrappedTexts[i] = translator.WrapVariablesWithSpan(text)
	}

	// Build request body
	// Google Translate API accepts an array of strings in the "q" parameter
	reqBody := map[string]any{
		"q":      wrappedTexts,
		"source": sourceGoogle,
		"target": targetGoogle,
		"format": "html", // Use HTML format to preserve span tags
	}

	return reqBody
}

// processGoogleResponse processes the response and returns translations or an error.
func processGoogleResponse(resp *http.Response, texts []string) ([]string, error) {
	// Handle non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, utils.HandleError(resp, constants.ProviderGoogle)
	}

	// Parse response and validate
	result, parseErr := parseGoogleTranslateResponse(resp)
	if parseErr != nil {
		return nil, parseErr
	}

	// Validate response
	if validationErr := translator.ValidateTranslations(result, texts); validationErr != nil {
		return nil, utils.NewHTTPProviderError(
			http.StatusUnprocessableEntity, constants.ProviderGoogle, validationErr.Error())
	}

	return result, nil
}

// parseGoogleTranslateResponse parses Google Cloud Translation API response and extracts translated texts.
func parseGoogleTranslateResponse(resp *http.Response) ([]string, error) {
	var apiResp struct {
		Data struct {
			Translations []struct {
				TranslatedText string `json:"translatedText"`
			} `json:"translations"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	if len(apiResp.Data.Translations) == 0 {
		return nil, utils.NewHTTPProviderError(
			http.StatusUnprocessableEntity,
			constants.ProviderGoogle,
			"empty translations in API response")
	}

	// Extract translated texts and unwrap span tags
	translations := make([]string, len(apiResp.Data.Translations))
	for i, t := range apiResp.Data.Translations {
		translations[i] = translator.UnwrapSpanTags(t.TranslatedText)
	}

	return translations, nil
}

// localeToGoogleLanguage converts locale format to Google language code.
// Google's API is case-insensitive, normalized to lowercase (e.g., "en-US" → "en-us").
func localeToGoogleLanguage(locale types.Locale) string {
	return translator.NormalizeLocale(locale)
}
