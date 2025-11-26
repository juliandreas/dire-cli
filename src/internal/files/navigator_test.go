package files_test

import (
	"maps"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/types"
)

func TestLocaleData_SetValue(t *testing.T) {
	t.Run("valid operations", func(t *testing.T) {
		testSetValueValidOperations(t)
	})

	t.Run("error cases", func(t *testing.T) {
		testSetValueErrorCases(t)
	})
}

func testSetValueValidOperations(t *testing.T) {
	tests := []struct {
		name      string
		initial   map[string]any
		path      []string
		value     any
		expected  map[string]any
		wantError bool
	}{
		{
			name:    "set top-level value",
			initial: map[string]any{},
			path:    []string{"key"},
			value:   "value",
			expected: map[string]any{
				"key": "value",
			},
			wantError: false,
		},
		{
			name:    "set nested value",
			initial: map[string]any{},
			path:    []string{"parent", "child"},
			value:   "nested value",
			expected: map[string]any{
				"parent": map[string]any{
					"child": "nested value",
				},
			},
			wantError: false,
		},
		{
			name: "override existing value",
			initial: map[string]any{
				"key": "old",
			},
			path:  []string{"key"},
			value: "new",
			expected: map[string]any{
				"key": "new",
			},
			wantError: false,
		},
		{
			name: "set value in array",
			initial: map[string]any{
				"items": []any{"first", "second", "third"},
			},
			path:  []string{"items", "[1]"},
			value: "updated",
			expected: map[string]any{
				"items": []any{"first", "updated", "third"},
			},
			wantError: false,
		},
	}

	runSetValueTests(t, tests)
}

func testSetValueErrorCases(t *testing.T) {
	tests := []struct {
		name      string
		initial   map[string]any
		path      []string
		value     any
		expected  map[string]any
		wantError bool
	}{
		{
			name:      "empty path error",
			initial:   map[string]any{},
			path:      []string{},
			value:     "value",
			expected:  map[string]any{},
			wantError: true,
		},
		{
			name: "array index out of bounds",
			initial: map[string]any{
				"items": []any{"one", "two"},
			},
			path:      []string{"items", "[5]"},
			value:     "new",
			expected:  map[string]any{},
			wantError: true,
		},
	}

	runSetValueTests(t, tests)
}

func runSetValueTests(t *testing.T, tests []struct {
	name      string
	initial   map[string]any
	path      []string
	value     any
	expected  map[string]any
	wantError bool
},
) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := maps.Clone(tt.initial) // Make a copy
			err := files.SetValue(data, tt.path, tt.value)

			if (err != nil) != tt.wantError {
				t.Errorf("SetValue() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && !reflect.DeepEqual(data, tt.expected) {
				t.Errorf("SetValue() result = %+v, want %+v", data, tt.expected)
			}
		})
	}
}

func TestLocaleData_SetValueStrict(t *testing.T) {
	tests := []struct {
		name      string
		initial   map[string]any
		path      []string
		value     any
		expected  map[string]any
		wantError bool
	}{
		{
			name:    "set nested value without conflicts",
			initial: map[string]any{},
			path:    []string{"parent", "child"},
			value:   "value",
			expected: map[string]any{
				"parent": map[string]any{
					"child": "value",
				},
			},
			wantError: false,
		},
		{
			name: "error on path conflict",
			initial: map[string]any{
				"parent": "not a map",
			},
			path:      []string{"parent", "child"},
			value:     "value",
			expected:  map[string]any{},
			wantError: true,
		},
		{
			name: "error on type mismatch",
			initial: map[string]any{
				"key": map[string]any{"nested": "value"},
			},
			path:      []string{"key"},
			value:     "string value",
			expected:  map[string]any{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := maps.Clone(tt.initial)
			err := files.SetValueStrict(data, tt.path, tt.value)

			if (err != nil) != tt.wantError {
				t.Errorf("SetValueStrict() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && !reflect.DeepEqual(data, tt.expected) {
				t.Errorf("SetValueStrict() result = %+v, want %+v", data, tt.expected)
			}
		})
	}
}

func TestLocaleData_ExtractTranslations(t *testing.T) {
	t.Run("flat and nested structures", func(t *testing.T) {
		testExtractTranslationsStructures(t)
	})

	t.Run("array handling", func(t *testing.T) {
		testExtractTranslationsArrays(t)
	})
}

func testExtractTranslationsStructures(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		path     []string
		locale   types.Locale
		expected []types.LocaleContent
	}{
		{
			name: "extract from flat structure",
			data: map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
			path:   []string{},
			locale: "en",
			expected: []types.LocaleContent{
				{
					Locale: "en",
					Path:   []string{"key1"},
					Value:  "value1",
				},
				{
					Locale: "en",
					Path:   []string{"key2"},
					Value:  "value2",
				},
			},
		},
		{
			name: "extract from nested structure",
			data: map[string]any{
				"section": map[string]any{
					"title": "Section Title",
					"text":  "Section Text",
				},
			},
			path:   []string{},
			locale: "fr",
			expected: []types.LocaleContent{
				{
					Locale: "fr",
					Path:   []string{"section", "text"},
					Value:  "Section Text",
				},
				{
					Locale: "fr",
					Path:   []string{"section", "title"},
					Value:  "Section Title",
				},
			},
		},
	}

	runExtractTranslationsTests(t, tests)
}

func testExtractTranslationsArrays(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		path     []string
		locale   types.Locale
		expected []types.LocaleContent
	}{
		{
			name: "handle arrays with translatable content",
			data: map[string]any{
				"items": []any{"first", "second", "third"},
			},
			path:   []string{},
			locale: "de",
			expected: []types.LocaleContent{
				{
					Locale: "de",
					Path:   []string{"items"},
					Value:  []any{"first", "second", "third"},
				},
			},
		},
		{
			name: "skip empty arrays",
			data: map[string]any{
				"empty":    []any{},
				"nonEmpty": "value",
			},
			path:   []string{},
			locale: "es",
			expected: []types.LocaleContent{
				{
					Locale: "es",
					Path:   []string{"nonEmpty"},
					Value:  "value",
				},
			},
		},
	}

	runExtractTranslationsTests(t, tests)
}

func runExtractTranslationsTests(t *testing.T, tests []struct {
	name     string
	data     map[string]any
	path     []string
	locale   types.Locale
	expected []types.LocaleContent
},
) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := maps.Clone(tt.data)
			result := files.ExtractTranslations(data, tt.path, tt.locale)

			// Sort results for consistent comparison
			sortLocaleContent(result)
			sortLocaleContent(tt.expected)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ExtractTranslations() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestLocaleData_ArrayHandling(t *testing.T) {
	t.Run("set value in nested array", func(t *testing.T) {
		data := map[string]any{
			"list": []any{
				map[string]any{"name": "Item 1"},
				map[string]any{"name": "Item 2"},
			},
		}
		err := files.SetValue(data, []string{"list", "[1]", "name"}, "Updated Item 2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := map[string]any{
			"list": []any{
				map[string]any{"name": "Item 1"},
				map[string]any{"name": "Updated Item 2"},
			},
		}

		if !reflect.DeepEqual(data, expected) {
			t.Errorf("got %+v, want %+v", data, expected)
		}
	})

	t.Run("extract translations from array of objects", func(t *testing.T) {
		data := map[string]any{
			"menu": []any{
				map[string]any{
					"title": "Home",
					"link":  "/home",
				},
				map[string]any{
					"title": "About",
					"link":  "/about",
				},
			},
		}
		result := files.ExtractTranslations(data, []string{}, "en")

		// Should extract the whole array as a unit
		if len(result) != 1 {
			t.Fatalf("expected 1 translation unit, got %d", len(result))
		}

		if !reflect.DeepEqual(result[0].Path, []string{"menu"}) {
			t.Errorf("expected path [menu], got %v", result[0].Path)
		}
	})
}

// Helper function to sort LocaleContent slices for comparison.
func sortLocaleContent(content []types.LocaleContent) {
	sort.Slice(content, func(i, j int) bool {
		return strings.Join(content[i].Path, ".") < strings.Join(content[j].Path, ".")
	})
}
