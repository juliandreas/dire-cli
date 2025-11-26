package files_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/testutil"
	"github.com/juliandreas/dire/internal/types"
)

//nolint:funlen,gocyclo // Test length and complexity acceptable due to multiple comprehensive test cases
func TestPrune(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("prunes keys not in reference locale", func(t *testing.T) {
		t.Parallel()

		// Create temp dir for file operations
		tempDir := t.TempDir()

		localeFiles := map[types.Locale]*types.LocaleFile{
			"en": {
				Path: filepath.Join(tempDir, "en.json"),
				Contents: map[string]any{
					"active": "Active value",
					"nested": map[string]any{
						"key": "Nested value",
					},
					"another": "Another value", // Make en the most complete
				},
			},
			"sv": {
				Path: filepath.Join(tempDir, "sv.json"),
				Contents: map[string]any{
					"active":   "Aktivt värde",
					"orphaned": "Old value", // This should be pruned (not in en)
					"nested": map[string]any{
						"key": "Nestad värde",
					},
				},
			},
		}

		//nolint:exhaustruct // Test mock only needs ReferenceLocale
		mock := &testutil.MockRepository{ReferenceLocale: "en"}
		results, err := files.Prune(ctx, mock, localeFiles, false, "  ")
		if err != nil {
			t.Fatalf("Prune returned error: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}

		result := results[0]
		if result.Locale != "sv" {
			t.Errorf("Expected locale 'sv', got '%s'", result.Locale)
		}

		if result.PrunedCount != 1 {
			t.Errorf("Expected 1 pruned key, got %d", result.PrunedCount)
		}

		// Verify orphaned key was removed
		if _, exists := localeFiles["sv"].Contents["orphaned"]; exists {
			t.Error("Expected 'orphaned' key to be removed")
		}

		// Verify files were written
		if _, err := os.Stat(filepath.Join(tempDir, "sv.json")); os.IsNotExist(err) {
			t.Error("Expected sv.json to be written")
		}
		if _, err := os.Stat(filepath.Join(tempDir, "en.json")); os.IsNotExist(err) {
			t.Error("Expected en.json to be written")
		}
	})

	t.Run("does not prune reference locale", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		localeFiles := map[types.Locale]*types.LocaleFile{
			"en": {
				Path: filepath.Join(tempDir, "en.json"),
				Contents: map[string]any{
					"key1": "Value 1",
					"key2": "Value 2",
				},
			},
			"sv": {
				Path: filepath.Join(tempDir, "sv.json"),
				Contents: map[string]any{
					"key1": "Värde 1",
				},
			},
		}

		//nolint:exhaustruct // Test mock only needs ReferenceLocale
		mock := &testutil.MockRepository{ReferenceLocale: "en"}
		results, err := files.Prune(ctx, mock, localeFiles, false, "  ")
		if err != nil {
			t.Fatalf("Prune returned error: %v", err)
		}

		// Should only prune sv, not en (which is the reference)
		if len(results) != 0 {
			t.Errorf("Expected 0 results (sv has no extra keys), got %d", len(results))
		}

		// Verify en still has both keys
		if len(localeFiles["en"].Contents) != 2 {
			t.Errorf("Expected en to still have 2 keys, got %d", len(localeFiles["en"].Contents))
		}
	})

	t.Run("returns empty when no orphaned keys", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		localeFiles := map[types.Locale]*types.LocaleFile{
			"en": {
				Path: filepath.Join(tempDir, "en.json"),
				Contents: map[string]any{
					"key1": "Value 1",
					"key2": "Value 2",
				},
			},
			"sv": {
				Path: filepath.Join(tempDir, "sv.json"),
				Contents: map[string]any{
					"key1": "Värde 1",
					"key2": "Värde 2",
				},
			},
		}

		//nolint:exhaustruct // Test mock only needs ReferenceLocale
		mock := &testutil.MockRepository{ReferenceLocale: "en"}
		results, err := files.Prune(ctx, mock, localeFiles, false, "  ")
		if err != nil {
			t.Fatalf("Prune returned error: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected no results when no orphaned keys, got %d", len(results))
		}
	})

	t.Run("prunes keys and cleans up empty objects", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		localeFiles := map[types.Locale]*types.LocaleFile{
			"en": {
				Path: filepath.Join(tempDir, "en.json"),
				Contents: map[string]any{
					"active":  "Active value",
					"another": "Another",
				},
			},
			"sv": {
				Path: filepath.Join(tempDir, "sv.json"),
				Contents: map[string]any{
					"active": "Aktivt värde",
					"deprecated": map[string]any{
						"old_feature": "Gammal funktion",
					},
				},
			},
		}

		//nolint:exhaustruct // Test mock only needs ReferenceLocale
		mock := &testutil.MockRepository{ReferenceLocale: "en"}
		results, err := files.Prune(ctx, mock, localeFiles, false, "  ")
		if err != nil {
			t.Fatalf("Prune returned error: %v", err)
		}

		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}

		// Verify the entire deprecated object was removed
		if _, exists := localeFiles["sv"].Contents["deprecated"]; exists {
			t.Error("Expected 'deprecated' object to be removed after all children pruned")
		}
	})
}
