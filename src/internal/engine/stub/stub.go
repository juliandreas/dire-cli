package stub

import (
	"context"
	"strings"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

// ApplyStubs fills missing translations with empty strings.
func ApplyStubs(
	ctx context.Context,
	missing []types.MissingTranslation,
	localeFiles map[types.Locale]*types.LocaleFile,
	referenceLocale types.Locale,
	cfg *config.Config,
) (int, []types.LocaleContent, error) {
	if len(missing) == 0 {
		logger.Debug("No missing translations to stub")
		return 0, nil, nil
	}

	logger.Info("Starting stub translation process", "missingKeys", len(missing))

	translated := make([]types.LocaleContent, 0, len(missing))
	stubCount := 0

	for _, miss := range missing {
		// Check for context cancellation
		if ctx.Err() != nil {
			return stubCount, translated, ctx.Err()
		}

		// Convert the source value to empty string, preserving structure
		stubValue := convertToEmptyString(miss.SourceValue)

		// Create the translated content
		translatedContent := types.LocaleContent{
			Locale: miss.TargetLocale,
			Path:   miss.Path,
			Value:  stubValue,
		}

		translated = append(translated, translatedContent)

		// Count the actual number of leaf keys being stubbed
		if sourceMap, ok := miss.SourceValue.(map[string]any); ok {
			// For nested objects, count leaf keys recursively
			stubCount += utils.CountKeysRecursive(sourceMap)
		} else {
			// For simple values and arrays, count as 1 stub
			// Note: Arrays are rare in translation files and treated as single translatable units
			stubCount++
		}

		// Update the locale file directly
		targetFile := localeFiles[miss.TargetLocale]
		if targetFile != nil {
			if err := files.SetValueStrict(targetFile.Contents, miss.Path, stubValue); err != nil {
				logger.Error("Failed to set stub value",
					"path", strings.Join(miss.Path, "."),
					"locale", miss.TargetLocale,
					"error", err)
				continue
			}
		}
	}

	logger.Info("Stub translation completed", "stubCount", stubCount)

	// Write updated files to disk
	if err := writeStubFilesToDisk(ctx, localeFiles, translated, cfg); err != nil {
		logger.Error("Failed to write stub files to disk", "error", err)
		return stubCount, translated, err
	}

	return stubCount, translated, nil
}

// writeStubFilesToDisk writes the modified locale files to disk.
func writeStubFilesToDisk(
	ctx context.Context,
	localeFiles map[types.Locale]*types.LocaleFile,
	translated []types.LocaleContent,
	cfg *config.Config,
) error {
	// Group translated content by locale to determine which files need updating
	localesUpdated := make(map[types.Locale]bool)
	for _, content := range translated {
		localesUpdated[content.Locale] = true
	}

	// Write each updated locale file to disk
	for locale := range localesUpdated {
		file := localeFiles[locale]
		if file == nil {
			continue
		}

		// Use the same file writing logic as the regular translation process
		if err := files.UpdateLocaleFile(ctx, file, []types.LocaleContent{}, cfg.GetIndent()); err != nil {
			logger.Error("Failed to write stub file",
				"locale", locale,
				"path", file.Path,
				"error", err)
			return err
		}
	}

	return nil
}

// convertToEmptyString converts any value to an empty string, handling nested structures.
func convertToEmptyString(value any) any {
	switch val := value.(type) {
	case map[string]any:
		// For nested objects, create structure with empty strings
		result := make(map[string]any)
		for key, nestedValue := range val {
			result[key] = convertToEmptyString(nestedValue)
		}
		return result
	case []any:
		// For arrays, convert each element
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = convertToEmptyString(item)
		}
		return result
	case string:
		// Only strings are stubbed to empty
		return ""
	default:
		// Preserve non-string leaf types as-is
		return val
	}
}
