package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	codeBlockMarkerLen = 3 // Length of markdown code block markers ("```")
)

var (
	ErrEmptyResponse       = errors.New("empty response text provided")
	ErrNoJSONFound         = errors.New("no JSON object found in response")
	ErrJSONArrayNotObject  = errors.New("received JSON array instead of expected object")
	ErrEmptyJSONObject     = errors.New("parsed JSON object is empty")
	ErrNoValidJSONInBlocks = errors.New("no valid JSON in code blocks")
)

// ExtractJSON finds and parses JSON from response text, checking code blocks first.
func ExtractJSON(responseText string) (map[string]any, error) {
	// Input validation
	if strings.TrimSpace(responseText) == "" {
		return nil, ErrEmptyResponse
	}

	// Limit processing to reasonable response sizes (1MB max)
	const maxResponseSize = 1024 * 1024
	if len(responseText) > maxResponseSize {
		responseText = responseText[:maxResponseSize]
	}

	// Look for JSON in code blocks first
	if result, err := extractFromCodeBlock(responseText); err == nil {
		return result, nil
	}

	// Find the first valid JSON object
	startPos := strings.Index(responseText, "{")
	if startPos == -1 {
		return nil, ErrNoJSONFound
	}

	// Use streaming decoder
	decoder := json.NewDecoder(strings.NewReader(responseText[startPos:]))

	var result map[string]any
	if err := decoder.Decode(&result); err != nil {
		if strings.Contains(err.Error(), "cannot unmarshal array") {
			return nil, ErrJSONArrayNotObject
		}
		return nil, fmt.Errorf("failed to parse JSON from response: %w", err)
	}

	if len(result) == 0 {
		return nil, ErrEmptyJSONObject
	}

	return result, nil
}

// extractFromCodeBlock attempts to extract JSON from markdown code blocks.
func extractFromCodeBlock(text string) (map[string]any, error) {
	// Convert to lowercase once, outside the loop
	lcText := strings.ToLower(text)

	// Look for code blocks with optional language specifier
	searchText := lcText
	offset := 0

	for {
		// Find the start of a code block
		start := strings.Index(searchText, "```")
		if start == -1 {
			break
		}

		actualStart := offset + start + codeBlockMarkerLen
		// Skip whitespace after ```
		for actualStart < len(text) && (text[actualStart] == '\n' || text[actualStart] == ' ') {
			actualStart++
		}

		// Check if there's a language specifier (case-insensitive)
		remaining := text[actualStart:]
		if strings.HasPrefix(strings.ToLower(remaining), "json") {
			actualStart += 4 // Skip "json"
			// Skip any whitespace after the language specifier
			for actualStart < len(text) && (text[actualStart] == '\n' || text[actualStart] == ' ') {
				actualStart++
			}
		}

		end := strings.Index(text[actualStart:], "```")
		if end == -1 {
			break
		}

		jsonText := strings.TrimSpace(text[actualStart : actualStart+end])

		decoder := json.NewDecoder(strings.NewReader(jsonText))
		var result map[string]any
		if err := decoder.Decode(&result); err == nil {
			return result, nil
		}

		offset = actualStart + end + codeBlockMarkerLen
		if offset >= len(text) {
			break
		}
		searchText = lcText[offset:]
	}

	return nil, ErrNoValidJSONInBlocks
}
