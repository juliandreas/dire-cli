package check

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/flags"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

// LintResult represents the result of a translation lint check.
type LintResult struct {
	Complete        bool                    `json:"complete"`
	MissingByLocale map[string][]MissingKey `json:"missingByLocale"`
}

// MissingKey represents a missing translation key.
type MissingKey struct {
	Key     string   `json:"key"`
	KeyPath []string `json:"keyPath"`
}

// Service handles translation linting operations.
type Service struct {
	config     *config.Config
	flags      *flags.Flags
	repository files.Repository
}

const (
	maxDisplayedKeys = 5 // Maximum number of missing keys to display per locale
)

// NewService creates a new lint service.
func NewService(
	cfg *config.Config,
	flagsInstance *flags.Flags,
	repository files.Repository,
) *Service {
	if flagsInstance == nil {
		flagsInstance = flags.New()
	}
	return &Service{
		config:     cfg,
		flags:      flagsInstance,
		repository: repository,
	}
}

// Run executes the lint check and returns the result.
func (s *Service) Run(ctx context.Context) (*LintResult, error) {
	logger.Info("Starting translation linting")

	// Scan locale files
	localeFiles, err := s.repository.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to scan locale files: %w", err)
	}

	// Determine missing translations
	missing, _, err := s.repository.DetermineMissing(ctx, localeFiles, s.flags.IncludeStubs)
	if err != nil {
		return nil, fmt.Errorf("failed to determine missing translations: %w", err)
	}

	logger.Debug("Determined missing translations", "count", len(missing))

	// Expand object-level missing translations to leaf keys for accurate check reporting
	expandedMissing := s.expandMissingForCheck(ctx, missing)

	logger.Debug("Expanded missing translations", "count", len(expandedMissing))

	// Build lint result
	result := &LintResult{
		Complete:        len(expandedMissing) == 0,
		MissingByLocale: s.groupMissingByLocale(expandedMissing),
	}

	logger.Info("Translation linting completed",
		"complete", result.Complete,
	)

	return result, nil
}

// OutputResult outputs the lint result in the requested format and returns appropriate exit code.
func (s *Service) OutputResult(result *LintResult, jsonFormat bool) int {
	// Guard against nil result
	if result == nil {
		logger.Error("OutputResult called with nil result", "context", "check service")
		return constants.ExitCodeError
	}

	var err error
	if jsonFormat {
		err = s.outputJSON(result)
	} else {
		s.outputHumanReadable(result)
	}

	// Handle JSON marshaling errors
	if err != nil {
		logger.Error("Failed to output lint result", "error", err)
		return constants.ExitCodeError
	}

	// Return appropriate code for CI/CD
	if result.Complete {
		return constants.ExitCodeSuccess // Success: all translations complete
	}
	return constants.ExitCodeWarning // Failure: missing translations detected
}

// outputJSON outputs the lint result as JSON.
func (s *Service) outputJSON(result *LintResult) error {
	jsonOutput, err := utils.MarshalJSON(result, "")
	if err != nil {
		return fmt.Errorf("failed to marshal lint result to JSON: %w", err)
	}
	fmt.Println(string(jsonOutput))
	return nil
}

// outputHumanReadable outputs the lint result in a human-readable format.
func (s *Service) outputHumanReadable(result *LintResult) {
	if result.Complete {
		fmt.Printf("✓ All translations complete\r\n")
	} else {
		fmt.Printf("✖ Missing translations:\r\n")
		// Sort locales for stable output
		locales := make([]string, 0, len(result.MissingByLocale))
		for locale := range result.MissingByLocale {
			locales = append(locales, locale)
		}
		sort.Strings(locales)

		for _, locale := range locales {
			s.displayLocaleKeys(locale, result.MissingByLocale[locale])
		}
	}
}

// displayLocaleKeys displays missing keys for a specific locale with truncation.
func (s *Service) displayLocaleKeys(locale string, missing []MissingKey) {
	keyWord := "key"
	if len(missing) != 1 {
		keyWord = "keys"
	}
	fileName := s.config.GetFileNameForLocale(locale)
	fmt.Printf("  %s: %d %s (in %s)\r\n", locale, len(missing), keyWord, fileName)

	// Sort missing keys for stable output
	sort.Slice(missing, func(i, j int) bool { return missing[i].Key < missing[j].Key })

	for i, key := range missing {
		if i < maxDisplayedKeys {
			fmt.Printf("    - %s\r\n", key.Key)
		} else if i == maxDisplayedKeys {
			fmt.Printf("    ... and %d more\r\n", len(missing)-maxDisplayedKeys)
			break
		}
	}
}

// groupMissingByLocale groups missing translations by locale and deduplicates by key path.
func (s *Service) groupMissingByLocale(missing []types.MissingTranslation) map[string][]MissingKey {
	byLocale := make(map[string][]MissingKey)
	seen := make(map[string]bool) // "locale:keyPath"

	for _, missingTranslation := range missing {
		locale := string(missingTranslation.TargetLocale)
		keyPath := strings.Join(missingTranslation.Path, ".")
		compositeKey := locale + ":" + keyPath

		// Skip if we've already seen this locale:key combination
		if seen[compositeKey] {
			continue
		}
		seen[compositeKey] = true

		byLocale[locale] = append(byLocale[locale], MissingKey{
			Key:     keyPath,
			KeyPath: missingTranslation.Path,
		})
	}

	return byLocale
}

// expandMissingForCheck expands object-level missing translations to individual leaf keys
// for accurate and consistent check reporting.
func (s *Service) expandMissingForCheck(
	ctx context.Context,
	missing []types.MissingTranslation,
) []types.MissingTranslation {
	var expanded []types.MissingTranslation

	for _, missingItem := range missing {
		// Check for context cancellation
		if ctx.Err() != nil {
			return expanded
		}

		// If the source value is an object, expand it to leaf keys
		if sourceMap, isMap := missingItem.SourceValue.(map[string]any); isMap {
			leafTranslations := s.expandObjectToLeaves(
				ctx, sourceMap, missingItem.Path, missingItem.SourceLocale, missingItem.TargetLocale)
			expanded = append(expanded, leafTranslations...)
		} else {
			// Leaf value - keep as is
			expanded = append(expanded, missingItem)
		}
	}

	return expanded
}

// expandObjectToLeaves recursively expands a missing object into individual leaf key missing translations.
func (s *Service) expandObjectToLeaves(
	ctx context.Context,
	missingObj map[string]any,
	basePath []string,
	sourceLocale, targetLocale types.Locale,
) []types.MissingTranslation {
	var leafMissing []types.MissingTranslation

	for key, value := range missingObj {
		// Check for context cancellation
		if ctx.Err() != nil {
			return leafMissing
		}

		currentPath := make([]string, len(basePath), len(basePath)+1)
		copy(currentPath, basePath)
		currentPath = append(currentPath, key)

		// If this value is also an object, recurse
		if nestedMap, isMap := value.(map[string]any); isMap {
			nestedMissing := s.expandObjectToLeaves(ctx, nestedMap, currentPath, sourceLocale, targetLocale)
			leafMissing = append(leafMissing, nestedMissing...)
		} else {
			// Leaf value - add as missing translation
			leafMissing = append(leafMissing, types.MissingTranslation{
				Path:         currentPath,
				SourceLocale: sourceLocale,
				SourceValue:  value,
				TargetLocale: targetLocale,
			})
		}
	}

	return leafMissing
}
