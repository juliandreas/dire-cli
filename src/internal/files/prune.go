package files

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
)

// PruneResult holds the results of a pruning operation for a single locale.
type PruneResult struct {
	Locale      types.Locale
	PrunedKeys  []string // Dot-separated paths of pruned keys
	PrunedCount int
}

// Prune removes orphaned keys from locale files and saves the changes.
// It determines the reference locale and removes keys from all other locales that don't exist in the reference.
// Returns a slice of per-locale prune results and any error encountered.
func Prune(
	ctx context.Context,
	repo Repository,
	localeFiles map[types.Locale]*types.LocaleFile,
	trimStrings bool,
	indent string,
) ([]PruneResult, error) {
	// Determine reference locale (the one with the most keys)
	_, referenceLocale, err := repo.DetermineMissing(ctx, localeFiles, false)
	if err != nil {
		return nil, fmt.Errorf("failed to determine reference locale: %w", err)
	}
	logger.Debug("Using reference locale for pruning", "locale", referenceLocale)

	// Prune orphaned keys
	results := pruneOrphanedKeys(ctx, localeFiles, referenceLocale)

	// Check for context cancellation before proceeding
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Log results
	if len(results) > 0 {
		for _, result := range results {
			logger.Info("Pruned orphaned keys",
				"locale", result.Locale,
				"count", result.PrunedCount)
		}
	} else {
		logger.Info("No orphaned keys found to prune")
	}

	// Save the pruned files
	if len(results) == 0 && !trimStrings {
		return results, nil
	}
	if err := CleanupFiles(ctx, localeFiles, trimStrings, indent); err != nil {
		return nil, fmt.Errorf("failed to save pruned files: %w", err)
	}

	return results, nil
}

// pruneOrphanedKeys removes keys from target locales that don't exist in the reference locale.
// It returns a slice of PruneResult containing information about what was pruned from each locale.
// The referenceLocale parameter specifies which locale to use as the source of truth.
func pruneOrphanedKeys(
	ctx context.Context,
	localeFiles map[types.Locale]*types.LocaleFile,
	referenceLocale types.Locale,
) []PruneResult {
	if referenceLocale == "" {
		logger.Debug("No reference locale provided for pruning")
		return nil
	}

	referenceFile := localeFiles[referenceLocale]
	if referenceFile == nil {
		logger.Debug("Reference locale file is nil", "locale", referenceLocale)
		return nil
	}

	referenceKeySet := buildReferenceKeySet(referenceFile)
	results := make([]PruneResult, 0, len(localeFiles))

	for locale, file := range localeFiles {
		if ctx.Err() != nil {
			return results
		}

		// Skip the reference locale itself or nil files
		if locale == referenceLocale || file == nil {
			continue
		}

		if result := pruneLocaleFile(file, locale, referenceKeySet); result != nil {
			results = append(results, *result)
		}
	}

	// Ensure deterministic ordering by locale
	slices.SortFunc(results, func(a, b PruneResult) int {
		return strings.Compare(string(a.Locale), string(b.Locale))
	})

	return results
}

// buildReferenceKeySet creates a set of all keys from the reference locale.
func buildReferenceKeySet(referenceFile *types.LocaleFile) map[string]bool {
	referenceKeys := getAllKeys(referenceFile.Contents, []string{})
	referenceKeySet := make(map[string]bool, len(referenceKeys))
	for _, key := range referenceKeys {
		referenceKeySet[key] = true
	}
	return referenceKeySet
}

// pruneLocaleFile prunes orphaned keys from a single locale file.
func pruneLocaleFile(
	file *types.LocaleFile,
	locale types.Locale,
	referenceKeySet map[string]bool,
) *PruneResult {
	orphanedKeys := findOrphanedKeys(file, referenceKeySet)

	if len(orphanedKeys) == 0 {
		logger.Debug("No orphaned keys found", "locale", locale)
		return nil
	}

	slices.Sort(orphanedKeys)
	removeOrphanedKeys(file, orphanedKeys)

	return &PruneResult{
		Locale:      locale,
		PrunedKeys:  orphanedKeys,
		PrunedCount: len(orphanedKeys),
	}
}

// findOrphanedKeys identifies keys in the target file that don't exist in the reference.
func findOrphanedKeys(file *types.LocaleFile, referenceKeySet map[string]bool) []string {
	targetKeys := getAllKeys(file.Contents, []string{})
	var orphanedKeys []string

	for _, key := range targetKeys {
		if !referenceKeySet[key] {
			orphanedKeys = append(orphanedKeys, key)
		}
	}

	return orphanedKeys
}

// removeOrphanedKeys removes the specified keys from the file and cleans up empty objects.
func removeOrphanedKeys(file *types.LocaleFile, orphanedKeys []string) {
	for _, keyPath := range orphanedKeys {
		removeKeyByPath(file.Contents, keyPath)
	}
	cleanupEmptyObjects(file.Contents)
}

// getAllKeys recursively extracts all key paths from a nested structure.
// Returns dot-separated key paths (e.g., "user.name", "app.settings.theme").
func getAllKeys(data map[string]any, currentPath []string) []string {
	var keys []string

	for key, value := range data {
		newPath := slices.Clone(currentPath)
		newPath = append(newPath, key)

		if nested, ok := value.(map[string]any); ok {
			// Recurse into nested objects
			nestedKeys := getAllKeys(nested, newPath)
			keys = append(keys, nestedKeys...)
		} else {
			// Leaf node - add the full path
			keys = append(keys, strings.Join(newPath, "."))
		}
	}

	return keys
}

// removeKeyByPath removes a key from a nested structure using a dot-separated path.
func removeKeyByPath(data map[string]any, keyPath string) {
	parts := splitPath(keyPath)
	if len(parts) == 0 {
		return
	}

	// Navigate to the parent of the target key
	current := data
	for i := range len(parts) - 1 {
		next, ok := current[parts[i]]
		if !ok {
			return // Path doesn't exist
		}

		nested, ok := next.(map[string]any)
		if !ok {
			return // Path is not a nested object
		}

		current = nested
	}

	// Remove the final key
	delete(current, parts[len(parts)-1])
}

// cleanupEmptyObjects recursively removes empty objects from a nested structure.
// An object is considered empty if it has no keys or all its nested values are empty.
func cleanupEmptyObjects(data map[string]any) {
	for key, value := range data {
		if nested, ok := value.(map[string]any); ok {
			// Recursively clean up nested objects first
			cleanupEmptyObjects(nested)

			// Remove the nested object if it's now empty
			if len(nested) == 0 {
				delete(data, key)
			}
		}
	}
}

// splitPath splits a dot-separated path into components.
// Note: Keys containing literal dots are not supported (dots are path separators).
func splitPath(path string) []string {
	if path == "" {
		return nil
	}

	segments := strings.Split(path, ".")
	var parts []string
	for _, segment := range segments {
		if segment != "" {
			parts = append(parts, segment)
		}
	}

	return parts
}
