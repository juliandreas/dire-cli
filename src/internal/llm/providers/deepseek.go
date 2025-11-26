//nolint:dupl // Providers are intentionally kept separate for independent evolution
package providers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/llm"
	"github.com/juliandreas/dire/internal/utils"
)

// CallDeepSeek makes a request to the DeepSeek API with the given prompt and returns the parsed JSON response.
func CallDeepSeek(
	ctx context.Context,
	httpClient *http.Client,
	provider config.Provider,
	apiKey string,
	promptText string,
) (map[string]any, error) {
	// Build request body (OpenAI-compatible format)
	reqBody := buildDeepSeekRequest(provider, promptText)

	// Create request - using DeepSeek's OpenAI-compatible endpoint
	req, err := utils.CreateRequest(ctx, "POST", "https://api.deepseek.com/v1/chat/completions", map[string]string{
		"Authorization": "Bearer " + apiKey,
	}, reqBody)
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
		return nil, errs.NewAPIError(constants.ProviderDeepSeek, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Handle errors
	if resp.StatusCode != http.StatusOK {
		return nil, utils.HandleError(resp, constants.ProviderDeepSeek)
	}

	// Parse response and extract content
	responseText, err := parseDeepSeekResponse(resp)
	if err != nil {
		return nil, err
	}

	// Extract and return the JSON from the response
	return llm.ExtractJSON(responseText)
}

// buildDeepSeekRequest creates the request body for the DeepSeek API call.
func buildDeepSeekRequest(provider config.Provider, promptText string) map[string]any {
	return map[string]any{
		"model":       provider.Model,
		"max_tokens":  provider.MaxTokens,
		"temperature": provider.Temperature,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": promptText,
			},
		},
		"response_format": map[string]string{
			"type": "json_object",
		},
	}
}

// parseDeepSeekResponse parses the DeepSeek API response and extracts the content.
func parseDeepSeekResponse(resp *http.Response) (string, error) {
	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", errs.NewAPIError(constants.ProviderDeepSeek, err)
	}

	if len(apiResp.Choices) == 0 {
		return "", errs.ErrEmptyChoices
	}

	responseText := apiResp.Choices[0].Message.Content
	if responseText == "" {
		return "", errs.ErrEmptyContent
	}

	return responseText, nil
}
