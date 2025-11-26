//nolint:dupl // Providers are intentionally kept separate for independent evolution
package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/llm"
	"github.com/juliandreas/dire/internal/utils"
)

// CallGemini makes a request to the Gemini API with the given prompt and returns the parsed JSON response.
func CallGemini(
	ctx context.Context,
	httpClient *http.Client,
	provider config.Provider,
	apiKey string,
	promptText string,
) (map[string]any, error) {
	// Build request body
	reqBody := buildGeminiRequest(provider, promptText)

	// Construct the URL with the API key as a query parameter (Gemini-specific)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		provider.Model, apiKey)

	// Create request (no Authorization header needed since API key is in URL)
	req, err := utils.CreateRequest(ctx, "POST", url, nil, reqBody)
	if err != nil {
		return nil, err
	}

	// Handle context cancellation
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, errs.ErrContextCanceled
	}

	// Make request
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errs.NewAPIError(constants.ProviderGemini, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Handle errors
	if resp.StatusCode != http.StatusOK {
		return nil, utils.HandleError(resp, constants.ProviderGemini)
	}

	// Parse response and extract content
	responseText, err := parseGeminiResponse(resp)
	if err != nil {
		return nil, err
	}

	// Extract and return the JSON from the response
	return llm.ExtractJSON(responseText)
}

// buildGeminiRequest creates the request body for the Gemini API call.
func buildGeminiRequest(provider config.Provider, promptText string) map[string]any {
	return map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{
						"text": promptText,
					},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":     provider.Temperature,
			"maxOutputTokens": provider.MaxTokens,
		},
	}
}

// parseGeminiResponse parses the Gemini API response and extracts the content.
func parseGeminiResponse(resp *http.Response) (string, error) {
	var apiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", errs.NewAPIError(constants.ProviderGemini, err)
	}

	// Guard against various edge cases in Gemini responses
	if len(apiResp.Candidates) == 0 {
		return "", errs.ErrEmptyChoices
	}

	if len(apiResp.Candidates[0].Content.Parts) == 0 {
		return "", errs.ErrEmptyContent
	}

	// Extract the text from the first candidate's first part
	responseText := apiResp.Candidates[0].Content.Parts[0].Text

	// Check if we actually got usable text content
	if responseText == "" {
		return "", errs.ErrEmptyContent
	}

	return responseText, nil
}
