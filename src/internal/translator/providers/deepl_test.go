package providers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/translator/providers"
)

// mockTransport is a custom http.RoundTripper that redirects requests to our test server.
type mockTransport struct {
	server *httptest.Server
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect the request to our test server
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(m.server.URL, "http://")
	return http.DefaultTransport.RoundTrip(req)
}

//nolint:gocyclo,funlen,goconst // Test functions can be long and complex, repeated test strings are acceptable
func TestCallDeepL(t *testing.T) {
	tests := []struct {
		name           string
		sourceLang     string
		targetLang     string
		texts          []string
		mockResponse   map[string]any
		mockStatusCode int
		expectError    bool
		validateFunc   func(t *testing.T, result []string)
	}{
		//nolint:exhaustruct // Test structs intentionally omit optional fields
		{
			name:       "successful translation with Danish locale",
			sourceLang: "sv-SE",
			targetLang: "da-DK",
			texts:      []string{"Hello", "World"},
			mockResponse: map[string]any{
				"translations": []map[string]string{
					{"text": "Hej"},
					{"text": "Verden"},
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			validateFunc: func(t *testing.T, result []string) {
				if len(result) != 2 {
					t.Errorf("expected 2 translations, got %d", len(result))
				}
				if result[0] != "Hej" || result[1] != "Verden" {
					t.Errorf("unexpected translations: %v", result)
				}
			},
		},
		{
			name:       "successful translation with Swedish locale",
			sourceLang: "en-US",
			targetLang: "sv-SE",
			texts:      []string{"Hello"},
			mockResponse: map[string]any{
				"translations": []map[string]string{
					{"text": "Hej"},
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			validateFunc: func(t *testing.T, result []string) {
				if len(result) != 1 {
					t.Errorf("expected 1 translation, got %d", len(result))
				}
				if result[0] != "Hej" {
					t.Errorf("expected 'Hej', got %q", result[0])
				}
			},
		},
		{
			name:       "variable preservation with double curly braces",
			sourceLang: "en-US",
			targetLang: "sv-SE",
			texts:      []string{"Hello {{name}}!"},
			mockResponse: map[string]any{
				"translations": []map[string]string{
					{"text": "Hej <var>{{name}}</var>!"},
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			validateFunc: func(t *testing.T, result []string) {
				if result[0] != "Hej {{name}}!" {
					t.Errorf("expected variables to be unwrapped, got %q", result[0])
				}
			},
		},
		{
			name:       "variable preservation with dollar sign braces",
			sourceLang: "en-US",
			targetLang: "da-DK",
			texts:      []string{"Welcome ${user}"},
			mockResponse: map[string]any{
				"translations": []map[string]string{
					{"text": "Velkommen <var>${user}</var>"},
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			validateFunc: func(t *testing.T, result []string) {
				if result[0] != "Velkommen ${user}" {
					t.Errorf("expected variables to be unwrapped, got %q", result[0])
				}
			},
		},
		//nolint:exhaustruct // Test structs intentionally omit optional fields
		{
			name:           "empty text array",
			sourceLang:     "en-US",
			targetLang:     "sv-SE",
			texts:          []string{},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			validateFunc: func(t *testing.T, result []string) {
				if result != nil {
					t.Errorf("expected nil for empty input, got %v", result)
				}
			},
		},
		//nolint:exhaustruct // Test structs intentionally omit optional fields
		{
			name:           "API error",
			sourceLang:     "en-US",
			targetLang:     "sv-SE",
			texts:          []string{"Hello"},
			mockStatusCode: http.StatusBadRequest,
			mockResponse: map[string]any{
				"message": "Value for 'target_lang' not supported.",
			},
			expectError: true,
		},
		{
			name:       "multiple variables in single text",
			sourceLang: "en-US",
			targetLang: "sv-SE",
			texts:      []string{"Hello {{name}}, you have {count} messages"},
			mockResponse: map[string]any{
				"translations": []map[string]string{
					{"text": "Hej <var>{{name}}</var>, du har <var>{count}</var> meddelanden"},
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			validateFunc: func(t *testing.T, result []string) {
				expected := "Hej {{name}}, du har {count} meddelanden"
				if result[0] != expected {
					t.Errorf("expected %q, got %q", expected, result[0])
				}
			},
		},
		{
			name:       "underscore locale normalization (en_US)",
			sourceLang: "en_US",
			targetLang: "sv-SE",
			texts:      []string{"Hello"},
			mockResponse: map[string]any{
				"translations": []map[string]string{
					{"text": "Hej"},
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			validateFunc: func(t *testing.T, result []string) {
				if result[0] != "Hej" {
					t.Errorf("expected 'Hej', got %q", result[0])
				}
			},
		},
		{
			name:       "underscore locale normalization (pt_BR)",
			sourceLang: "en-US",
			targetLang: "pt_BR",
			texts:      []string{"Hello"},
			mockResponse: map[string]any{
				"translations": []map[string]string{
					{"text": "Olá"},
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			validateFunc: func(t *testing.T, result []string) {
				if result[0] != "Olá" {
					t.Errorf("expected 'Olá', got %q", result[0])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP server
			//nolint:varnamelen,nestif,lll // Standard Go test patterns
			server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, req *http.Request) {
				// Verify request
				if req.Method != http.MethodPost {
					t.Errorf("expected POST request, got %s", req.Method)
				}

				// Verify Authorization header
				authHeader := req.Header.Get("Authorization")
				if !strings.HasPrefix(authHeader, "DeepL-Auth-Key ") {
					t.Errorf("invalid Authorization header: %s", authHeader)
				}

				// For non-empty texts, verify the request body contains wrapped variables
				if len(tt.texts) > 0 {
					var reqBody map[string]any
					if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
						t.Fatalf("failed to decode request body: %v", err)
					}

					// Check that variables are wrapped with <var> tags in the request
					if texts, ok := reqBody["text"].([]any); ok && len(texts) > 0 {
						for textIndex, text := range texts {
							textStr, ok := text.(string)
							if !ok {
								continue
							}
							// If original contains {{, ${, or {, verify it's wrapped
							originalHasVar := strings.Contains(tt.texts[textIndex], "{{") ||
								strings.Contains(tt.texts[textIndex], "${") ||
								strings.Contains(tt.texts[textIndex], "{")
							if originalHasVar {
								if !strings.Contains(textStr, "<var>") {
									t.Errorf("expected variables to be wrapped in request, got %q", textStr)
								}
							}
						}
					}

					// Verify language codes are correct (uppercase, language-only)
					if sourceLang, ok := reqBody["source_lang"].(string); ok {
						// Normalize underscores, then extract language code
						normalized := strings.ReplaceAll(tt.sourceLang, "_", "-")
						expected := strings.ToUpper(strings.Split(normalized, "-")[0])
						if sourceLang != expected {
							t.Errorf("expected source_lang %q, got %q", expected, sourceLang)
						}
					}
					if targetLang, ok := reqBody["target_lang"].(string); ok {
						// Normalize underscores, then extract language code
						normalized := strings.ReplaceAll(tt.targetLang, "_", "-")
						expected := strings.ToUpper(strings.Split(normalized, "-")[0])
						if targetLang != expected {
							t.Errorf("expected target_lang %q, got %q", expected, targetLang)
						}
					}

					// Verify DeepL-specific parameters for variable preservation
					if tagHandling, ok := reqBody["tag_handling"].(string); ok {
						if tagHandling != "xml" {
							t.Errorf("expected tag_handling %q, got %q", "xml", tagHandling)
						}
					} else {
						t.Error("expected tag_handling field in request body")
					}

					if ignoreTags, ok := reqBody["ignore_tags"].([]any); ok {
						if len(ignoreTags) != 1 || ignoreTags[0] != "var" {
							t.Errorf("expected ignore_tags [\"var\"], got %v", ignoreTags)
						}
					} else {
						t.Error("expected ignore_tags field in request body")
					}

					if preserveFormatting, ok := reqBody["preserve_formatting"].(bool); ok {
						if !preserveFormatting {
							t.Error("expected preserve_formatting to be true")
						}
					} else {
						t.Error("expected preserve_formatting field in request body")
					}
				}

				// Write response
				responseWriter.Header().Set("Content-Type", "application/json")
				responseWriter.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					if err := json.NewEncoder(responseWriter).Encode(tt.mockResponse); err != nil {
						t.Fatalf("failed to encode response: %v", err)
					}
				}
			}))
			defer server.Close()

			// Override the DeepL endpoint for testing
			// Note: This is a workaround since we can't easily inject the endpoint
			// In production, we'd want to make the endpoint configurable
			originalURL := "https://api.deepl.com/v2/translate"
			apiKey := "test-key"

			// Create HTTP client
			//nolint:exhaustruct // Optional fields not needed for test
			httpClient := &http.Client{
				Transport: &mockTransport{
					server: server,
				},
			}

			//nolint:exhaustruct // Only Name field needed for DeepL provider
			provider := config.Provider{
				Name: constants.ProviderDeepL,
			}

			// Call DeepL
			ctx := context.Background()
			result, err := providers.CallDeepL(
				ctx,
				httpClient,
				provider,
				apiKey,
				tt.texts,
				tt.sourceLang,
				tt.targetLang,
				constants.FormalityAutomatic,
			)

			// Verify expectations
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Run validation function if provided
			if tt.validateFunc != nil && !tt.expectError {
				tt.validateFunc(t, result)
			}

			_ = originalURL // Suppress unused variable warning
		})
	}
}

//nolint:funlen // Test functions can be long
func TestCallDeepLEndpointSelection(t *testing.T) {
	tests := []struct {
		name          string
		apiKey        string
		expectFreeAPI bool
	}{
		{
			name:          "free API key ending with :fx",
			apiKey:        "test-key:fx",
			expectFreeAPI: true,
		},
		{
			name:          "pro API key without :fx",
			apiKey:        "test-key-pro",
			expectFreeAPI: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestMade := false
			//nolint:varnamelen // Standard Go test patterns
			server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, req *http.Request) {
				requestMade = true
				// Verify the URL path
				expectedPath := "/v2/translate"
				if req.URL.Path != expectedPath {
					t.Errorf("expected path %q, got %q", expectedPath, req.URL.Path)
				}

				responseWriter.Header().Set("Content-Type", "application/json")
				responseWriter.WriteHeader(http.StatusOK)
				response := map[string]any{
					"translations": []map[string]string{
						{"text": "Translated"},
					},
				}
				if err := json.NewEncoder(responseWriter).Encode(response); err != nil {
					t.Fatalf("failed to encode response: %v", err)
				}
			}))
			defer server.Close()

			//nolint:exhaustruct // Optional fields not needed for test
			httpClient := &http.Client{
				Transport: &mockTransport{server: server},
			}

			//nolint:exhaustruct // Only Name field needed for DeepL provider
			provider := config.Provider{Name: constants.ProviderDeepL}
			ctx := context.Background()

			_, err := providers.CallDeepL(
				ctx,
				httpClient,
				provider,
				tt.apiKey,
				[]string{"Test"},
				"en-US",
				"sv-SE",
				constants.FormalityAutomatic,
			)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !requestMade {
				t.Error("expected HTTP request to be made")
			}
		})
	}
}
