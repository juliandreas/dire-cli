package batching_test

import (
	"fmt"
	"testing"

	"github.com/juliandreas/dire/internal/llm/batching"
	"github.com/juliandreas/dire/internal/types"
)

const (
	testMaxTokens        = 32000
	testPromptOverhead   = 800
	testOutputMultiplier = 2.5
	testSafetyBuffer     = 0.6
)

func TestTokenEstimationAccuracy(t *testing.T) {
	estimator := batching.NewTokenEstimator()

	// Simple string test
	simple := types.MissingTranslation{
		Path: []string{"simple"}, SourceValue: "Hello world", SourceLocale: "en-US", TargetLocale: "da-DK",
	}
	estimated := estimator.EstimateTranslationTokens(simple)
	if estimated < 6 || estimated > 12 {
		t.Errorf("Simple string estimate out of range: got %d, expected 6-12", estimated)
	}

	// Large nested object test - this is the critical test that would have caught the original bug
	large := types.MissingTranslation{
		Path: []string{"module"}, SourceValue: createLargeNestedObject(), SourceLocale: "en-US", TargetLocale: "da-DK",
	}
	estimated = estimator.EstimateTranslationTokens(large)
	if estimated < 2500 {
		t.Errorf("Large nested object underestimated: got %d, expected 2500+. This would cause timeout issues.", estimated)
	}

	// Deep nesting test
	deep := types.MissingTranslation{
		Path: []string{"userWeb"}, SourceValue: createDeeplyNestedObject(), SourceLocale: "en-US", TargetLocale: "da-DK",
	}
	estimated = estimator.EstimateTranslationTokens(deep)
	if estimated < 100 || estimated > 200 {
		t.Errorf("Deep nesting estimate out of range: got %d, expected 100-200", estimated)
	}

	// Array with objects test
	array := types.MissingTranslation{
		Path: []string{"systemLog"}, SourceValue: createArrayWithObjects(), SourceLocale: "en-US", TargetLocale: "da-DK",
	}
	estimated = estimator.EstimateTranslationTokens(array)
	if estimated < 300 || estimated > 600 {
		t.Errorf("Array with objects estimate out of range: got %d, expected 300-600", estimated)
	}
}

func TestTokenEstimationForBreakdownThreshold(t *testing.T) {
	// This test verifies that large objects will be broken down by testing
	// if they exceed realistic breakdown thresholds

	largeObject := createLargeNestedObject()
	translation := types.MissingTranslation{
		Path:         []string{"module"},
		SourceValue:  largeObject,
		SourceLocale: "en-US",
		TargetLocale: "da-DK",
	}

	estimator := batching.NewTokenEstimator()
	estimated := estimator.EstimateTranslationTokens(translation)

	// Calculate actual input budget dynamically using the same logic as the real code
	maxTokens := testMaxTokens
	promptOverhead := testPromptOverhead
	outputMultiplier := testOutputMultiplier
	safetyBuffer := testSafetyBuffer

	available := maxTokens - promptOverhead
	inputSpace := int(float64(available) / (1.0 + outputMultiplier))
	actualBudget := int(float64(inputSpace) * safetyBuffer)

	// Test that our improvements work: large objects should be broken down to prevent timeouts
	if estimated <= actualBudget {
		t.Logf("Large object fits in budget: %d tokens (budget: %d) - won't be broken down",
			estimated, actualBudget)
	} else {
		t.Logf("Large object exceeds budget: %d tokens (budget: %d) - will be broken down ✓",
			estimated, actualBudget)
	}

	// Large objects that caused timeouts should be broken down into multiple batches/pieces
	batches, err := batching.CreateBatches([]types.MissingTranslation{translation}, testMaxTokens)
	if err != nil {
		t.Fatalf("CreateBatches failed: %v", err)
	}

	// If this fails, it means the safety settings are too permissive and timeouts could happen again
	if len(batches) <= 1 {
		t.Errorf("Large object was not broken down! Got %d batch(es), expected multiple batches. "+
			"This configuration may allow timeout issues. Object tokens: %d", len(batches), estimated)
	} else {
		t.Logf("Large object breakdown: %d tokens → %d batches ✓", estimated, len(batches))
	}
}

// TestJSONComplexityMultiplier tests that JSON structures get the complexity multiplier applied.
func TestJSONComplexityMultiplier(t *testing.T) {
	// Same content as string vs JSON object
	content := "This is a long piece of text that would be in a translation key with " +
		"multiple sentences and complex punctuation."

	stringTranslation := types.MissingTranslation{
		Path:         []string{"simple"},
		SourceValue:  content,
		SourceLocale: "en-US",
		TargetLocale: "da-DK",
	}

	objectTranslation := types.MissingTranslation{
		Path: []string{"object"},
		SourceValue: map[string]any{
			"text": content,
			"metadata": map[string]any{
				"author": "test",
				"date":   "2024-01-01",
			},
		},
		SourceLocale: "en-US",
		TargetLocale: "da-DK",
	}

	estimator := batching.NewTokenEstimator()
	stringTokens := estimator.EstimateTranslationTokens(stringTranslation)
	objectTokens := estimator.EstimateTranslationTokens(objectTranslation)

	// Object should have significantly more tokens due to JSON complexity multiplier
	if objectTokens <= stringTokens {
		t.Errorf("JSON object should have more estimated tokens than plain string. "+
			"String: %d, Object: %d", stringTokens, objectTokens)
	}

	// Should be at least 1.5x more (accounting for the 1.8x multiplier minus some variance)
	multiplier := float64(objectTokens) / float64(stringTokens)
	if multiplier < 1.5 {
		t.Errorf("JSON complexity multiplier not working. Expected at least 1.5x, got %.2fx", multiplier)
	}

	t.Logf("JSON complexity verified: string=%d tokens, object=%d tokens (%.2fx multiplier)",
		stringTokens, objectTokens, multiplier)
}

// createLargeNestedObject creates a large nested object to test token estimation accuracy.
func createLargeNestedObject() map[string]any {
	// Simulate a realistic i18n module with nested structure similar to what caused the timeout
	auth := make(map[string]any)
	for i := range 20 {
		auth[fmt.Sprintf("key%d", i)] = fmt.Sprintf("This is a longer translation string for authentication item %d "+
			"with detailed content", i)
	}

	dashboard := make(map[string]any)
	for i := range 15 {
		dashboard[fmt.Sprintf("item%d", i)] = fmt.Sprintf("Dashboard translation %d with comprehensive user "+
			"interface text", i)
	}

	return map[string]any{
		"authentication": auth,
		"dashboard":      dashboard,
		"settings": map[string]any{
			"general":  "General settings with detailed configuration options and help text",
			"privacy":  "Privacy settings with comprehensive policy information and user controls",
			"security": "Security settings with advanced authentication and authorization options",
		},
	}
}

// createDeeplyNestedObject creates deep nesting to test structural token costs.
func createDeeplyNestedObject() map[string]any {
	return map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"level3": map[string]any{
					"level4": map[string]any{
						"level5": map[string]any{
							"data": "Deep content with high tokenization cost due to nested JSON structure",
						},
					},
				},
			},
		},
	}
}

// createArrayWithObjects creates an array with objects to test array tokenization costs.
func createArrayWithObjects() []any {
	logs := make([]any, 5)
	for i := range 5 {
		logs[i] = map[string]any{
			"id":      i + 1,
			"message": fmt.Sprintf("Log entry %d with detailed system information and context", i+1),
			"level":   "INFO",
		}
	}
	return logs
}
