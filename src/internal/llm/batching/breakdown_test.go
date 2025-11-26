package batching_test

import (
	"testing"

	"github.com/juliandreas/dire/internal/llm/batching"
	"github.com/juliandreas/dire/internal/types"
)

// MockEstimator is a test estimator that returns fixed token counts.
type MockEstimator struct {
	tokenCount int
}

func (m *MockEstimator) EstimateTranslationTokens(translation types.MissingTranslation) int {
	return m.tokenCount
}

func (m *MockEstimator) EstimateValueTokens(value any) int {
	return m.tokenCount
}

func TestBreakdownOversized_WithArrays(t *testing.T) {
	t.Run("array breakdown scenarios", func(t *testing.T) {
		testArrayBreakdownScenarios(t)
	})

	t.Run("budget behavior with arrays", func(t *testing.T) {
		testArrayBudgetBehavior(t)
	})
}

func testArrayBreakdownScenarios(t *testing.T) {
	tests := createArrayBreakdownTestCases()
	runBreakdownTests(t, tests)
}

//nolint:funlen // Test data creation function with multiple test cases
func createArrayBreakdownTestCases() []struct {
	name        string
	missing     []types.MissingTranslation
	inputBudget int
	tokenCount  int
	wantCount   int
} {
	return []struct {
		name        string
		missing     []types.MissingTranslation
		inputBudget int
		tokenCount  int
		wantCount   int
	}{
		{
			name: "array of objects that needs breakdown",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"list"},
					SourceLocale: "en",
					TargetLocale: "fr",
					SourceValue: []any{
						map[string]any{
							"title": "Title 1",
							"text":  "Text 1",
						},
						map[string]any{
							"title": "Title 2",
							"text":  "Text 2",
						},
					},
				},
			},
			inputBudget: 50,
			tokenCount:  100, // Over budget, should break down
			wantCount:   4,   // 2 objects * 2 fields each
		},
		{
			name: "array of strings stays together",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"items"},
					SourceLocale: "en",
					TargetLocale: "fr",
					SourceValue:  []any{"string1", "string2", "string3"},
				},
			},
			inputBudget: 50,
			tokenCount:  100, // Over budget but can't break down
			wantCount:   1,   // Should stay as single unit
		},
		{
			name: "nested map with array of objects",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"section"},
					SourceLocale: "en",
					TargetLocale: "fr",
					SourceValue: map[string]any{
						"title": "Section Title",
						"items": []any{
							map[string]any{
								"name": "Item 1",
								"desc": "Description 1",
							},
							map[string]any{
								"name": "Item 2",
								"desc": "Description 2",
							},
						},
					},
				},
			},
			inputBudget: 50,
			tokenCount:  150, // Over budget, should break down
			wantCount:   5,   // 1 title + 2 objects * 2 fields
		},
	}
}

func testArrayBudgetBehavior(t *testing.T) {
	tests := []struct {
		name        string
		missing     []types.MissingTranslation
		inputBudget int
		tokenCount  int
		wantCount   int
	}{
		{
			name: "array under budget stays intact",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"list"},
					SourceLocale: "en",
					TargetLocale: "fr",
					SourceValue: []any{
						map[string]any{
							"title": "Title 1",
							"text":  "Text 1",
						},
					},
				},
			},
			inputBudget: 100,
			tokenCount:  50, // Under budget
			wantCount:   1,  // Should stay intact
		},
	}

	runBreakdownTests(t, tests)
}

func TestBreakdownOversized_ComplexStructures(t *testing.T) {
	t.Run("deeply nested structures", func(t *testing.T) {
		testDeeplyNestedStructures(t)
	})

	t.Run("mixed content types", func(t *testing.T) {
		testMixedContentTypes(t)
	})
}

func testDeeplyNestedStructures(t *testing.T) {
	tests := []struct {
		name        string
		missing     []types.MissingTranslation
		inputBudget int
		tokenCount  int
		wantCount   int
		description string
	}{
		{
			name: "deeply nested structure breaks down correctly",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"app"},
					SourceLocale: "en",
					TargetLocale: "fr",
					SourceValue: map[string]any{
						"navigation": map[string]any{
							"menu": map[string]any{
								"home":    "Home",
								"about":   "About",
								"contact": "Contact",
							},
						},
						"content": map[string]any{
							"title": "Welcome",
							"body":  "This is the main content",
						},
					},
				},
			},
			inputBudget: 50,
			tokenCount:  100, // Over budget
			wantCount:   5,   // Should break into individual leaf values
			description: "Complex nested map should break down to leaf values",
		},
	}

	runComplexBreakdownTests(t, tests)
}

func testMixedContentTypes(t *testing.T) {
	tests := []struct {
		name        string
		missing     []types.MissingTranslation
		inputBudget int
		tokenCount  int
		wantCount   int
		description string
	}{
		{
			name: "mixed content types handled correctly",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"data"},
					SourceLocale: "en",
					TargetLocale: "es",
					SourceValue: map[string]any{
						"text":    "Some text",
						"number":  42,
						"boolean": true,
						"array":   []any{"item1", "item2"},
						"object": map[string]any{
							"nested": "value",
						},
					},
				},
			},
			inputBudget: 30,
			tokenCount:  80, // Over budget
			wantCount:   5,  // All leaf values should be extracted
			description: "Mixed content types should all be extracted as separate translations",
		},
		{
			name: "mixed array with strings and objects",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"mixed_array"},
					SourceLocale: "en",
					TargetLocale: "es",
					SourceValue: []any{
						"First string",
						map[string]any{
							"title": "Object title",
							"count": 42,
						},
						"Second string",
						map[string]any{
							"description": "Another object",
							"active":      true,
						},
					},
				},
			},
			inputBudget: 50,
			tokenCount:  80, // Over budget
			wantCount:   1,  // Mixed arrays are treated as single units to preserve structure
			description: "Mixed arrays with strings and objects should be kept as single units",
		},
	}

	runComplexBreakdownTests(t, tests)
}

func TestBreakdownOversized_BudgetBehavior(t *testing.T) {
	tests := createBudgetBehaviorTestCases()
	runBudgetBehaviorTests(t, tests)
}

//nolint:funlen // Test data creation function with multiple test cases
func createBudgetBehaviorTestCases() []struct {
	name        string
	missing     []types.MissingTranslation
	inputBudget int
	tokenCount  int
	wantCount   int
	description string
} {
	return []struct {
		name        string
		missing     []types.MissingTranslation
		inputBudget int
		tokenCount  int
		wantCount   int
		description string
	}{
		{
			name: "under budget items remain intact",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"small"},
					SourceLocale: "en",
					TargetLocale: "fr",
					SourceValue: map[string]any{
						"title": "Title",
						"text":  "Text",
					},
				},
			},
			inputBudget: 100,
			tokenCount:  50, // Under budget
			wantCount:   1,  // Should stay as one item
			description: "Items under budget should not be broken down",
		},
		{
			name: "exactly at budget remains intact",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"exact"},
					SourceLocale: "en",
					TargetLocale: "fr",
					SourceValue: map[string]any{
						"key": "value",
					},
				},
			},
			inputBudget: 50,
			tokenCount:  50, // Exactly at budget
			wantCount:   1,  // Should stay as one item
			description: "Items exactly at budget should not be broken down",
		},
		{
			name: "over budget gets broken down",
			missing: []types.MissingTranslation{
				{
					Path:         []string{"large"},
					SourceLocale: "en",
					TargetLocale: "fr",
					SourceValue: map[string]any{
						"key1": "value1",
						"key2": "value2",
					},
				},
			},
			inputBudget: 50,
			tokenCount:  100, // Over budget
			wantCount:   2,   // Should break into parts
			description: "Items over budget should be broken down",
		},
	}
}

func runBudgetBehaviorTests(t *testing.T, tests []struct {
	name        string
	missing     []types.MissingTranslation
	inputBudget int
	tokenCount  int
	wantCount   int
	description string
},
) {
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			breakdown := batching.NewTranslationBreakdown()
			estimator := &MockEstimator{tokenCount: testCase.tokenCount}

			result := breakdown.BreakdownOversized(testCase.missing, testCase.inputBudget, estimator)

			if len(result) != testCase.wantCount {
				t.Errorf("BreakdownOversized() returned %d items, want %d. %s",
					len(result), testCase.wantCount, testCase.description)
			}
		})
	}
}

func runBreakdownTests(t *testing.T, tests []struct {
	name        string
	missing     []types.MissingTranslation
	inputBudget int
	tokenCount  int
	wantCount   int
},
) {
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			breakdown := batching.NewTranslationBreakdown()
			estimator := &MockEstimator{tokenCount: testCase.tokenCount}

			result := breakdown.BreakdownOversized(testCase.missing, testCase.inputBudget, estimator)

			if len(result) != testCase.wantCount {
				t.Errorf("BreakdownOversized() returned %d items, want %d", len(result), testCase.wantCount)
				// Print details for debugging
				for index, item := range result {
					t.Logf("Item %d: path=%v, value=%v", index, item.Path, item.SourceValue)
				}
			}
		})
	}
}

func runComplexBreakdownTests(t *testing.T, tests []struct {
	name        string
	missing     []types.MissingTranslation
	inputBudget int
	tokenCount  int
	wantCount   int
	description string
},
) {
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			breakdown := batching.NewTranslationBreakdown()
			estimator := &MockEstimator{tokenCount: testCase.tokenCount}

			result := breakdown.BreakdownOversized(testCase.missing, testCase.inputBudget, estimator)

			if len(result) != testCase.wantCount {
				t.Errorf("BreakdownOversized() returned %d items, want %d. %s",
					len(result), testCase.wantCount, testCase.description)
				// Print details for debugging
				for index, item := range result {
					t.Logf("Item %d: path=%v, value=%v", index, item.Path, item.SourceValue)
				}
			}

			// Verify all results have the correct source and target locales
			for index, item := range result {
				if item.SourceLocale != testCase.missing[0].SourceLocale {
					t.Errorf("Item %d has wrong source locale: got %s, want %s",
						index, item.SourceLocale, testCase.missing[0].SourceLocale)
				}
				if item.TargetLocale != testCase.missing[0].TargetLocale {
					t.Errorf("Item %d has wrong target locale: got %s, want %s",
						index, item.TargetLocale, testCase.missing[0].TargetLocale)
				}
			}
		})
	}
}
