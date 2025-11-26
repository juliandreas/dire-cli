//nolint:dupl // Providers are intentionally kept separate for independent evolution
package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/llm"
	"github.com/juliandreas/dire/internal/utils"
)

// CallClaude makes a request to the Claude API with the given prompt and returns the parsed JSON response.
func CallClaude(
	ctx context.Context,
	httpClient *http.Client,
	provider config.Provider,
	apiKey string,
	promptText string,
) (map[string]any, error) {
	// Build request body
	reqBody := buildClaudeRequest(provider, promptText)

	// Create request
	req, err := utils.CreateRequest(ctx, "POST", "https://api.anthropic.com/v1/messages", map[string]string{
		"x-api-key":         apiKey,
		"anthropic-version": "2023-06-01",
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
		return nil, errs.NewAPIError(constants.ProviderClaude, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Handle errors
	if resp.StatusCode != http.StatusOK {
		return nil, utils.HandleError(resp, constants.ProviderClaude)
	}

	// Parse response and extract content
	responseText, err := parseClaudeResponse(resp)
	if err != nil {
		return nil, err
	}

	// Extract and return the JSON from the response
	return llm.ExtractJSON(responseText)
}

// buildClaudeRequest creates the request body for the Claude API call.
func buildClaudeRequest(provider config.Provider, promptText string) map[string]any {
	return map[string]any{
		"model":      provider.Model,
		"max_tokens": provider.MaxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": promptText},
		},
		"temperature": provider.Temperature,
	}
}

// parseClaudeResponse parses the Claude API response and extracts the content.
func parseClaudeResponse(resp *http.Response) (string, error) {
	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", errs.NewAPIError(constants.ProviderClaude, err)
	}

	if len(apiResp.Content) == 0 {
		return "", errs.ErrEmptyContent
	}

	// Extract text content - Claude can return multiple content blocks
	var fullText strings.Builder
	for _, part := range apiResp.Content {
		if part.Type == "text" {
			fullText.WriteString(part.Text)
		}
	}

	responseText := fullText.String()
	if responseText == "" {
		return "", errs.ErrEmptyContent
	}

	return responseText, nil
}
