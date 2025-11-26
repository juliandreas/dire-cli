package files

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/flags"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

// Repository handles all file operations for translation processing.
type Repository interface {
	// Scan scans locale files without any modifications
	Scan(ctx context.Context) (map[types.Locale]*types.LocaleFile, error)

	// DetermineMissing analyzes locale files to find missing translations and determine the reference locale
	// When includeStubs is true, empty strings are also considered missing translations
	DetermineMissing(
		ctx context.Context,
		files map[types.Locale]*types.LocaleFile,
		includeStubs bool,
	) ([]types.MissingTranslation, types.Locale, error)

	// GetMissing rescans files and returns missing translations
	GetMissing(ctx context.Context) ([]types.MissingTranslation, error)

	// GetMissingWithStubs rescans files and returns missing translations, including empty strings as missing
	GetMissingWithStubs(ctx context.Context) ([]types.MissingTranslation, error)

	// GetFilteredContentForLocale rescans files and returns filtered content for a specific locale
	GetFilteredContentForLocale(ctx context.Context, locale types.Locale) ([]types.LocaleContent, error)

	// FinalCleanup performs final cleanup on all files
	FinalCleanup(ctx context.Context) error
}

type repository struct {
	config *config.Config
	flags  *flags.Flags
}

// NewRepository creates a new file repository.
func NewRepository(cfg *config.Config, flagsInstance *flags.Flags) Repository {
	return &repository{
		config: cfg,
		flags:  flagsInstance,
	}
}

// Scan scans locale files without performing any modifications.
func (r *repository) Scan(ctx context.Context) (map[types.Locale]*types.LocaleFile, error) {
	// Scan locale files
	localeFiles, err := ScanLocaleFiles(ctx, r.config)
	if err != nil {
		logger.Error("Failed to scan locale files", "error", err)
		return nil, fmt.Errorf("failed to scan locale files: %w", err)
	}

	logger.Debug("Scanned locale files",
		"totalFiles", len(localeFiles),
		"directory", r.config.Files.Directory)

	return localeFiles, nil
}

// DetermineMissing analyzes locale files to find missing translations and determine the reference locale.
func (r *repository) DetermineMissing(
	ctx context.Context,
	files map[types.Locale]*types.LocaleFile,
	includeStubs bool,
) ([]types.MissingTranslation, types.Locale, error) {
	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, "", ctx.Err()
	}

	referenceLocale := utils.DetermineReferenceLocale(files)
	var missing []types.MissingTranslation
	if includeStubs {
		missing = IdentifyMissingKeysWithStubs(ctx, files, r.flags.Overwrite)
	} else {
		missing = IdentifyMissingKeys(ctx, files, r.flags.Overwrite)
	}

	// Check again after potentially long operation
	if ctx.Err() != nil {
		return nil, "", ctx.Err()
	}

	logger.Info("Translation analysis completed",
		"referenceLocale", referenceLocale,
		"missingKeys", len(missing),
		"includeStubs", includeStubs,
		"totalFiles", len(files),
	)

	return missing, referenceLocale, nil
}

// GetMissing rescans files and returns missing translations.
func (r *repository) GetMissing(ctx context.Context) ([]types.MissingTranslation, error) {
	return r.getMissingInternal(ctx, false)
}

// GetMissingWithStubs rescans files and returns missing translations, including empty strings as missing.
func (r *repository) GetMissingWithStubs(ctx context.Context) ([]types.MissingTranslation, error) {
	return r.getMissingInternal(ctx, true)
}

// GetFilteredContentForLocale rescans files and returns filtered content for a specific locale.
func (r *repository) GetFilteredContentForLocale(
	ctx context.Context,
	locale types.Locale,
) ([]types.LocaleContent, error) {
	// Check for context cancellation early
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Scan files to get current state
	currentFiles, err := r.Scan(ctx)
	if err != nil {
		logger.Debug("Failed to scan files for existing translations", "error", err)
		return nil, err
	}

	// Check for context cancellation after scan
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Get translations for the specific locale only
	var existing []types.LocaleContent
	if file, exists := currentFiles[locale]; exists {
		existing = r.extractExistingTranslations(file.Contents, nil, locale)
		logger.Debug("Extracted existing translations from file", "locale", locale, "extractedCount", len(existing))
	} else {
		logger.Debug("Locale file not found in scanned files",
			"locale", locale,
			"availableLocales", maps.Keys(currentFiles))
	}

	logger.Debug("Existing translations identified for locale", "locale", locale, "existingCount", len(existing))
	return existing, nil
}

// FinalCleanup performs final cleanup on all files.
func (r *repository) FinalCleanup(ctx context.Context) error {
	// Scan files for final processing
	finalFiles, err := r.Scan(ctx)
	if err != nil {
		return fmt.Errorf("failed to scan files for final cleanup: %w", err)
	}

	// Cleanup all files to ensure consistent formatting
	finalCleanupErr := CleanupFiles(ctx, finalFiles, r.flags.TrimStrings, r.config.GetIndent())
	if finalCleanupErr != nil {
		logger.Error("Final cleanup failed", "error", finalCleanupErr)
		return fmt.Errorf("final cleanup failed: %w", finalCleanupErr)
	}

	return nil
}

// getMissingInternal is the internal implementation for both GetMissing methods.
func (r *repository) getMissingInternal(ctx context.Context, includeStubs bool) ([]types.MissingTranslation, error) {
	// Check for context cancellation early
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Scan files to get current state
	currentFiles, err := r.Scan(ctx)
	if err != nil {
		logger.Debug("Failed to rescan files for missing translations", "error", err)
		return nil, fmt.Errorf("failed to rescan files: %w", err)
	}

	// Check for context cancellation after scan
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Identify missing translations
	var missing []types.MissingTranslation
	if includeStubs {
		missing = IdentifyMissingKeysWithStubs(ctx, currentFiles, r.flags.Overwrite)
	} else {
		missing = IdentifyMissingKeys(ctx, currentFiles, r.flags.Overwrite)
	}

	// Final context cancellation check
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	logger.Debug("Missing translations identified", "missingKeys", len(missing), "includeStubs", includeStubs)
	return missing, nil
}

// extractExistingTranslations recursively extracts existing translations from a content map.
func (r *repository) extractExistingTranslations(
	content map[string]any,
	currentPath []string,
	locale types.Locale,
) []types.LocaleContent {
	var translations []types.LocaleContent

	for key, value := range content {
		// Build the path for this key
		path := append(slices.Clone(currentPath), key)

		switch val := value.(type) {
		case map[string]any:
			// Recursively process nested objects
			nestedTranslations := r.extractExistingTranslations(val, path, locale)
			translations = append(translations, nestedTranslations...)
		case string:
			// Only include non-empty strings
			if val != "" {
				translations = append(translations, types.LocaleContent{
					Path:   path,
					Locale: locale,
					Value:  val,
				})
			}
		default:
			// Include other types (numbers, booleans, etc.) as they might be valid content
			translations = append(translations, types.LocaleContent{
				Path:   path,
				Locale: locale,
				Value:  val,
			})
		}
	}

	return translations
}
