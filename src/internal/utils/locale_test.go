package utils_test

import (
	"testing"

	"github.com/juliandreas/dire/internal/utils"
)

// Test HasKeyMismatch function which validates AI response structure.
func TestHasKeyMismatch(t *testing.T) {
	t.Run("valid matches", func(t *testing.T) {
		testValidMatches(t)
	})

	t.Run("key translation errors", func(t *testing.T) {
		testKeyTranslationErrors(t)
	})

	t.Run("type mismatches", func(t *testing.T) {
		testTypeMismatches(t)
	})

	t.Run("edge cases", func(t *testing.T) {
		testEdgeCases(t)
	})
}

func testValidMatches(t *testing.T) {
	tests := []struct {
		name           string
		original       map[string]any
		aiResponse     map[string]any
		expectMismatch bool
	}{
		{
			name: "should pass when structures match exactly",
			original: map[string]any{
				"auth": map[string]any{
					"login": "Login",
				},
			},
			aiResponse: map[string]any{
				"auth": map[string]any{
					"login": "Connexion",
				},
			},
			expectMismatch: false,
		},
		{
			name: "should detect when AI returns partial translations",
			original: map[string]any{
				"auth": map[string]any{
					"login":  "Login",
					"logout": "Logout",
				},
			},
			aiResponse: map[string]any{
				"auth": map[string]any{
					"login": "Connexion",
				},
			},
			expectMismatch: true,
		},
		{
			name:           "should detect empty AI response",
			original:       map[string]any{"key": "value"},
			aiResponse:     map[string]any{},
			expectMismatch: true,
		},
		{
			name:           "should handle both empty",
			original:       map[string]any{},
			aiResponse:     map[string]any{},
			expectMismatch: false,
		},
	}

	runHasKeyMismatchTests(t, tests)
}

func testKeyTranslationErrors(t *testing.T) {
	tests := []struct {
		name           string
		original       map[string]any
		aiResponse     map[string]any
		expectMismatch bool
	}{
		{
			name: "should detect when top-level key is translated",
			original: map[string]any{
				"auth": map[string]any{
					"login": "Login",
				},
			},
			aiResponse: map[string]any{
				"authentification": map[string]any{
					"login": "Connexion",
				},
			},
			expectMismatch: true,
		},
		{
			name: "should detect when nested key is translated",
			original: map[string]any{
				"auth": map[string]any{
					"login": "Login",
				},
			},
			aiResponse: map[string]any{
				"auth": map[string]any{
					"connexion": "Connexion",
				},
			},
			expectMismatch: true,
		},
		{
			name: "should detect when all keys are translated",
			original: map[string]any{
				"auth": map[string]any{
					"login": "Login",
				},
			},
			aiResponse: map[string]any{
				"authentification": map[string]any{
					"connexion": "Connexion",
				},
			},
			expectMismatch: true,
		},
	}

	runHasKeyMismatchTests(t, tests)
}

func testTypeMismatches(t *testing.T) {
	tests := []struct {
		name           string
		original       map[string]any
		aiResponse     map[string]any
		expectMismatch bool
	}{
		{
			name: "should detect type mismatch (object vs string)",
			original: map[string]any{
				"auth": map[string]any{
					"login": "Login",
				},
			},
			aiResponse: map[string]any{
				"auth": "Authentication",
			},
			expectMismatch: true,
		},
		{
			name: "should detect type mismatch (string vs object)",
			original: map[string]any{
				"message": "Hello",
			},
			aiResponse: map[string]any{
				"message": map[string]any{
					"text": "Bonjour",
				},
			},
			expectMismatch: true,
		},
		{
			name: "should handle deeply nested structures",
			original: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": map[string]any{
							"level4": "value",
						},
					},
				},
			},
			aiResponse: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": map[string]any{
							"level4": "valeur",
						},
					},
				},
			},
			expectMismatch: false,
		},
	}

	runHasKeyMismatchTests(t, tests)
}

func testEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		original       map[string]any
		aiResponse     map[string]any
		expectMismatch bool
	}{
		{
			name:           "should detect when AI adds new keys to empty original",
			original:       map[string]any{},
			aiResponse:     map[string]any{"key": "value"},
			expectMismatch: true,
		},
		{
			name: "should handle case sensitivity",
			original: map[string]any{
				"Auth": map[string]any{
					"Login": "Login",
				},
			},
			aiResponse: map[string]any{
				"auth": map[string]any{
					"login": "Connexion",
				},
			},
			expectMismatch: true,
		},
	}

	runHasKeyMismatchTests(t, tests)
}

func runHasKeyMismatchTests(t *testing.T, tests []struct {
	name           string
	original       map[string]any
	aiResponse     map[string]any
	expectMismatch bool
},
) {
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := utils.HasKeyMismatch(tc.original, tc.aiResponse)

			if result != tc.expectMismatch {
				t.Errorf("expected mismatch=%v, got mismatch=%v", tc.expectMismatch, result)
				t.Logf("Original: %+v", tc.original)
				t.Logf("AI Response: %+v", tc.aiResponse)
			}
		})
	}
}

// Benchmark for HasKeyMismatch.
func BenchmarkHasKeyMismatch(b *testing.B) {
	smallOriginal, smallResponse := createSmallTestData()
	largeOriginal, largeResponse := createLargeTestData()

	b.Run("small_structure_match", func(b *testing.B) {
		benchmarkHasKeyMismatch(b, smallOriginal, smallResponse)
	})

	b.Run("small_structure_mismatch", func(b *testing.B) {
		mismatchResponse := map[string]any{
			"authentification": map[string]any{
				"connexion": "Connexion",
			},
		}
		benchmarkHasKeyMismatch(b, smallOriginal, mismatchResponse)
	})

	b.Run("large_structure_match", func(b *testing.B) {
		benchmarkHasKeyMismatch(b, largeOriginal, largeResponse)
	})

	b.Run("large_structure_mismatch", func(b *testing.B) {
		mismatchResponse := createMismatchResponse(largeResponse)
		benchmarkHasKeyMismatch(b, largeOriginal, mismatchResponse)
	})
}

func createSmallTestData() (map[string]any, map[string]any) {
	original := map[string]any{
		"auth": map[string]any{
			"login":  "Login",
			"logout": "Logout",
		},
	}
	response := map[string]any{
		"auth": map[string]any{
			"login":  "Connexion",
			"logout": "Déconnexion",
		},
	}
	return original, response
}

func createLargeTestData() (map[string]any, map[string]any) {
	largeOriginal := make(map[string]any)
	largeResponse := make(map[string]any)

	for i := range 10 {
		sectionKey := string(rune('A' + i))
		section := make(map[string]any)
		for j := range 10 {
			subsection := make(map[string]any)
			for k := range 5 {
				subsection[string(rune('a'+k))] = "value"
			}
			section[string(rune('a'+j))] = subsection
		}
		largeOriginal[sectionKey] = section
		largeResponse[sectionKey] = section
	}

	return largeOriginal, largeResponse
}

func createMismatchResponse(original map[string]any) map[string]any {
	mismatchResponse := make(map[string]any)
	for k, v := range original {
		if k == "A" {
			mismatchResponse["A_translated"] = v
		} else {
			mismatchResponse[k] = v
		}
	}
	return mismatchResponse
}

func benchmarkHasKeyMismatch(b *testing.B, original, response map[string]any) {
	b.ReportAllocs()
	for b.Loop() {
		utils.HasKeyMismatch(original, response)
	}
}
