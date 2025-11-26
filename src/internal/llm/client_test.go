package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/llm"
	"github.com/juliandreas/dire/internal/testutil"
	"github.com/juliandreas/dire/internal/types"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// parsePromptAndCreateMatchingResponse parses a translation prompt and creates a response with matching keys.
func parsePromptAndCreateMatchingResponse(promptText string) map[string]any {
	// Look for the JSON section in the prompt (after "# JSON TO TRANSLATE")
	jsonMarker := "# JSON TO TRANSLATE"
	markerIndex := strings.Index(promptText, jsonMarker)
	if markerIndex == -1 {
		return nil
	}

	// Extract the JSON part (everything after the marker)
	jsonSection := promptText[markerIndex+len(jsonMarker):]
	jsonSection = strings.TrimSpace(jsonSection)

	// Find the JSON object boundaries
	start := strings.Index(jsonSection, "{")
	if start == -1 {
		return nil
	}

	// Find the matching closing brace
	braceCount := 0
	end := -1
	for charIndex := start; charIndex < len(jsonSection); charIndex++ {
		if jsonSection[charIndex] == '{' {
			braceCount++
		} else if jsonSection[charIndex] == '}' {
			braceCount--
			if braceCount == 0 {
				end = charIndex
				break
			}
		}
	}

	if end == -1 {
		return nil
	}

	jsonStr := jsonSection[start : end+1]
	var original map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &original); err != nil {
		return nil
	}

	// Create a response with the same structure but with "translated" values
	return createTranslatedResponse(original)
}

// createTranslatedResponse creates a translated version of the input maintaining key structure.
func createTranslatedResponse(original map[string]any) map[string]any {
	result := make(map[string]any)

	for key, value := range original {
		switch valueType := value.(type) {
		case string:
			result[key] = "translated_" + valueType
		case map[string]any:
			result[key] = createTranslatedResponse(valueType)
		case []any:
			translatedArray := make([]any, len(valueType))
			for itemIndex, item := range valueType {
				switch v := item.(type) {
				case map[string]any:
					translatedArray[itemIndex] = createTranslatedResponse(v)
				case string:
					translatedArray[itemIndex] = "translated_" + v
				default:
					translatedArray[itemIndex] = item
				}
			}
			result[key] = translatedArray
		default:
			result[key] = value
		}
	}

	return result
}

// Mock provider function that returns exactly what was sent (simulating translation).
func mockProviderExact(shouldError bool) llm.ProviderFunc {
	return func(
		ctx context.Context,
		httpClient *http.Client,
		provider config.Provider,
		apiKey string,
		promptText string,
	) (map[string]any, error) {
		if shouldError {
			return nil, http.ErrServerClosed
		}

		// Parse the prompt to extract the structure and return matching keys
		// This prevents key structure mismatch validation errors
		if result := parsePromptAndCreateMatchingResponse(promptText); result != nil {
			return result, nil
		}

		// Fallback to empty structure for unparseable prompts
		return map[string]any{}, nil
	}
}

// Simplified mock that just returns empty result to test basic functionality.
func mockProvider(statusCode int) llm.ProviderFunc {
	return func(
		ctx context.Context,
		httpClient *http.Client,
		provider config.Provider,
		apiKey string,
		promptText string,
	) (map[string]any, error) {
		if statusCode != http.StatusOK {
			return nil, http.ErrServerClosed
		}

		// Return empty map to avoid key mismatch issues for basic tests
		return map[string]any{}, nil
	}
}

// Mock provider that returns a response with type mismatch (string becomes number).
func mockProviderTypeMismatch() llm.ProviderFunc {
	return func(
		ctx context.Context,
		httpClient *http.Client,
		provider config.Provider,
		apiKey string,
		promptText string,
	) (map[string]any, error) {
		// Return number instead of string to test type changes are allowed
		return map[string]any{
			"message": 123, // Original was string "Hello World", this is number
		}, nil
	}
}

// Mock provider that returns a response with modified placeholder.
func mockProviderPlaceholderModification() llm.ProviderFunc {
	return func(
		ctx context.Context,
		httpClient *http.Client,
		provider config.Provider,
		apiKey string,
		promptText string,
	) (map[string]any, error) {
		// Return modified placeholder to trigger placeholder validation error
		return map[string]any{
			"greeting": "Bonjour nom!", // Original was "Hello {name}!", placeholder was translated
		}, nil
	}
}

// Mock provider that returns a response with numeric values (regression test for numeric bug).
func mockProviderNumericValues() llm.ProviderFunc {
	return func(
		ctx context.Context,
		httpClient *http.Client,
		provider config.Provider,
		apiKey string,
		promptText string,
	) (map[string]any, error) {
		// Return the numeric value as-is to test that validation accepts it
		return map[string]any{
			"dashboard": map[string]any{
				"version": 2.1, // This numeric value previously caused failures
			},
		}, nil
	}
}

// Create a test provider config.
func testProvider() config.Provider {
	return config.Provider{
		Name:        constants.ProviderOpenAI,
		Model:       "gpt-4",
		MaxTokens:   1000,
		Temperature: 0.2,
		Region:      "", // Not used for LLMs
	}
}

// setupTestEnvironment sets up the test environment with a dummy API key.
func setupTestEnvironment(t *testing.T) func() {
	provider := testProvider()
	envKey := constants.GetEnvKeyForProvider(provider.Name)
	if err := os.Setenv(envKey, "dummy-test-key"); err != nil {
		t.Fatalf("failed to set environment variable: %v", err)
	}

	return func() {
		if err := os.Unsetenv(envKey); err != nil {
			t.Errorf("failed to unset environment variable: %v", err)
		}
	}
}

// createTestClient creates a new client for testing.
func createTestClient(t *testing.T, providerFn llm.ProviderFunc) *llm.Client {
	cfg := testutil.MockConfig()
	provider := testProvider()

	client, err := llm.NewClient(cfg, constants.FormalityAutomatic, provider, providerFn, "")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	return client
}

// runTranslationTest runs a single translation test case.
func runTranslationTest(t *testing.T, tc struct {
	name           string
	missing        []types.MissingTranslation
	mockStatusCode int
	expectError    bool
	expectedCount  int
},
) {
	t.Helper()
	var mockProviderFn llm.ProviderFunc

	// Use specific mock providers for validation tests
	switch {
	case strings.Contains(tc.name, "type changes"):
		mockProviderFn = mockProviderTypeMismatch() // Still returns number but now should be accepted
	case strings.Contains(tc.name, "numeric values"):
		mockProviderFn = mockProviderNumericValues() // Returns numeric values that should be accepted
	case strings.Contains(tc.name, "placeholder modification"):
		mockProviderFn = mockProviderPlaceholderModification()
	default:
		// Default behavior for existing tests
		shouldError := tc.mockStatusCode != http.StatusOK
		mockProviderFn = mockProviderExact(shouldError)
	}

	client := createTestClient(t, mockProviderFn)

	ctx := context.Background()
	results, err := client.ProcessTranslations(ctx, tc.missing)

	if tc.expectError && err == nil {
		t.Error("expected error, got nil")
	}
	if !tc.expectError && err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(results) != tc.expectedCount {
		t.Errorf("expected %d results, got %d", tc.expectedCount, len(results))
	}
}

// getTranslationTestCases returns the test cases for translation testing.
func getTranslationTestCases() []struct {
	name           string
	missing        []types.MissingTranslation
	mockStatusCode int
	expectError    bool
	expectedCount  int
} {
	tests := getEmptyTranslationTestCase()
	tests = append(tests, getSingleTranslationTestCase()...)
	tests = append(tests, getMultipleTranslationTestCase()...)
	tests = append(tests, getAPIErrorTranslationTestCase()...)
	tests = append(tests, getValidationTestCases()...)
	return tests
}

func getEmptyTranslationTestCase() []struct {
	name           string
	missing        []types.MissingTranslation
	mockStatusCode int
	expectError    bool
	expectedCount  int
} {
	return []struct {
		name           string
		missing        []types.MissingTranslation
		mockStatusCode int
		expectError    bool
		expectedCount  int
	}{
		{
			name:           "should handle empty translations",
			missing:        []types.MissingTranslation{},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			expectedCount:  0,
		},
	}
}

func getSingleTranslationTestCase() []struct {
	name           string
	missing        []types.MissingTranslation
	mockStatusCode int
	expectError    bool
	expectedCount  int
} {
	return []struct {
		name           string
		missing        []types.MissingTranslation
		mockStatusCode int
		expectError    bool
		expectedCount  int
	}{
		{
			name: "should process single translation without error (testing for leaks)",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"key1"},
					SourceLocale: "en-US",
					SourceValue:  "Hello",
					TargetLocale: "fr-FR",
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false, // Our mock returns matching structure, no key mismatch
			expectedCount:  1,     // Now provides actual translation
		},
	}
}

func getMultipleTranslationTestCase() []struct {
	name           string
	missing        []types.MissingTranslation
	mockStatusCode int
	expectError    bool
	expectedCount  int
} {
	return []struct {
		name           string
		missing        []types.MissingTranslation
		mockStatusCode int
		expectError    bool
		expectedCount  int
	}{
		{
			name: "should process multiple translations without error (testing for leaks)",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"key1"},
					SourceLocale: "en-US",
					SourceValue:  "Hello",
					TargetLocale: "fr-FR",
				},
				{
					Path:         []string{"key2"},
					SourceLocale: "en-US",
					SourceValue:  "World",
					TargetLocale: "fr-FR",
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false, // Our mock returns matching structure, no key mismatch
			expectedCount:  2,     // Now provides actual translations for both keys
		},
	}
}

func getAPIErrorTranslationTestCase() []struct {
	name           string
	missing        []types.MissingTranslation
	mockStatusCode int
	expectError    bool
	expectedCount  int
} {
	return []struct {
		name           string
		missing        []types.MissingTranslation
		mockStatusCode int
		expectError    bool
		expectedCount  int
	}{
		{
			name: "should handle API errors",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"key1"},
					SourceLocale: "en-US",
					SourceValue:  "Hello",
					TargetLocale: "fr-FR",
				},
			},
			mockStatusCode: http.StatusInternalServerError,
			expectError:    true,
			expectedCount:  0,
		},
	}
}

func getValidationTestCases() []struct {
	name           string
	missing        []types.MissingTranslation
	mockStatusCode int
	expectError    bool
	expectedCount  int
} {
	return []struct {
		name           string
		missing        []types.MissingTranslation
		mockStatusCode int
		expectError    bool
		expectedCount  int
	}{
		{
			name: "should allow response with type changes (type validation removed)",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"message"},
					SourceLocale: "en-US",
					SourceValue:  "Hello World",
					TargetLocale: "fr-FR",
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false, // Type changes are now allowed
			expectedCount:  1,
		},
		{
			name: "should allow JSON with numeric values (regression test for numeric bug)",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"dashboard", "version"},
					SourceLocale: "en-US",
					SourceValue:  2.1, // Numeric value that previously caused failures
					TargetLocale: "fr-FR",
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false, // Numeric values should be accepted
			expectedCount:  1,
		},
		{
			name: "should reject response with placeholder modification",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"greeting"},
					SourceLocale: "en-US",
					SourceValue:  "Hello {name}!",
					TargetLocale: "fr-FR",
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    true, // Placeholder modification should cause validation error
			expectedCount:  0,
		},
	}
}

func TestProcessTranslations(t *testing.T) {
	defer goleak.VerifyNone(t)
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := getTranslationTestCases()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runTranslationTest(t, tc)
		})
	}
}

// createTestServer creates a test HTTP server for testing.
func createTestServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"key1": "translated value"}`)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
}

// createHTTPTestProvider creates a provider function that makes real HTTP requests.
func createHTTPTestProvider(server *httptest.Server) llm.ProviderFunc {
	return func(
		ctx context.Context,
		httpClient *http.Client,
		provider config.Provider,
		apiKey string,
		promptText string,
	) (map[string]any, error) {
		// Make a real HTTP request to test server
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, http.NoBody)
		if err != nil {
			return nil, err
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		errClose := resp.Body.Close()
		if errClose != nil {
			return nil, errClose
		}

		// Return mock response with matching structure
		return map[string]any{
			"key1": "translated value",
		}, nil
	}
}

func TestClientHTTPTransport(t *testing.T) {
	defer goleak.VerifyNone(t)
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	server := createTestServer(t)
	defer server.Close()

	testProviderFn := createHTTPTestProvider(server)
	client := createTestClient(t, testProviderFn)

	// Test translation with real HTTP calls
	missing := []types.MissingTranslation{
		{
			Path:         []string{"key1"},
			SourceLocale: "en-US",
			SourceValue:  "Hello",
			TargetLocale: "fr-FR",
		},
	}

	ctx := context.Background()
	results, err := client.ProcessTranslations(ctx, missing)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// createSlowProvider creates a provider function that simulates slow operations.
func createSlowProvider() llm.ProviderFunc {
	return func(
		ctx context.Context,
		httpClient *http.Client,
		provider config.Provider,
		apiKey string,
		promptText string,
	) (map[string]any, error) {
		// Simulate slow operation
		select {
		case <-time.After(500 * time.Millisecond):
			return map[string]any{"key": "value"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func TestClientContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	slowProviderFn := createSlowProvider()
	client := createTestClient(t, slowProviderFn)

	missing := []types.MissingTranslation{
		{
			Path:         []string{"key1"},
			SourceLocale: "en-US",
			SourceValue:  "Hello",
			TargetLocale: "fr-FR",
		},
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.ProcessTranslations(ctx, missing)

	// Should get context deadline exceeded error
	if err == nil {
		t.Error("expected context cancellation error, got nil")
	}

	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded error, got: %v", err)
	}
}

func TestClientConnectionPooling(t *testing.T) {
	defer goleak.VerifyNone(t)
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create multiple clients to test connection pooling
	mockProviderFn := mockProviderExact(false)
	clients := make([]*llm.Client, 5)

	for i := range 5 {
		client, err := llm.NewClient(testutil.MockConfig(), constants.FormalityAutomatic, testProvider(), mockProviderFn, "")
		if err != nil {
			t.Fatalf("failed to create client %d: %v", i, err)
		}
		clients[i] = client
	}

	// Test that multiple clients can be created without issues
	if len(clients) != 5 {
		t.Errorf("expected 5 clients, got %d", len(clients))
	}

	// Quick verification that they work
	ctx := context.Background()
	missing := []types.MissingTranslation{
		{
			Path:         []string{"test"},
			SourceLocale: "en-US",
			SourceValue:  "test",
			TargetLocale: "fr-FR",
		},
	}

	for i, client := range clients {
		_, err := client.ProcessTranslations(ctx, missing)
		if err != nil {
			t.Errorf("client %d failed: %v", i, err)
		}
	}
}

func TestNewClient(t *testing.T) {
	defer goleak.VerifyNone(t)

	provider := testProvider()
	envKey := constants.GetEnvKeyForProvider(provider.Name)

	tests := []struct {
		name        string
		setupEnv    func() error
		expectError bool
	}{
		{
			name: "should create client with valid API key",
			setupEnv: func() error {
				return os.Setenv(envKey, "dummy-test-key")
			},
			expectError: false,
		},
		{
			name: "should fail with missing API key",
			setupEnv: func() error {
				return os.Unsetenv(envKey)
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.setupEnv(); err != nil {
				t.Fatalf("failed to setup environment: %v", err)
			}
			defer func() {
				if err := os.Unsetenv(envKey); err != nil {
					t.Errorf("failed to cleanup environment: %v", err)
				}
			}()

			cfg := testutil.MockConfig()
			mockProviderFn := mockProvider(http.StatusOK)

			client, err := llm.NewClient(cfg, constants.FormalityAutomatic, provider, mockProviderFn, "")

			if tc.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tc.expectError && client == nil {
				t.Error("expected client, got nil")
			}
		})
	}
}
