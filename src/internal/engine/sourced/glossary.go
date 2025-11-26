package sourced

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
)

// glossaryResult contains the results of glossary matches.
type glossaryResult struct {
	Matched   []types.LocaleContent
	Remaining []types.MissingTranslation
}

// glossaryIndexCache provides thread-safe caching of glossary indices.
type glossaryIndexCache struct {
	mu    sync.RWMutex
	cache map[string]map[string]map[string]types.GlossaryEntry // hash -> index
}

// Package-level cache for glossary indices.
//
//nolint:gochecknoglobals // Performance cache is appropriate use of package-level state
var glossaryCache = &glossaryIndexCache{ //nolint:exhaustruct // mu is zero-initialized
	cache: make(map[string]map[string]map[string]types.GlossaryEntry),
}

// Package-level cached case transformers for performance.
//
//nolint:gochecknoglobals // Cached casers for performance
var (
	caseLowerUnd = cases.Lower(language.Und)
	caseUpperUnd = cases.Upper(language.Und)
)

// applyGlossary applies glossary terms to missing translations.
func applyGlossary(
	ctx context.Context,
	missing []types.MissingTranslation,
	cfg *config.Config,
) glossaryResult {
	result := glossaryResult{
		Matched:   make([]types.LocaleContent, 0, len(missing)),
		Remaining: make([]types.MissingTranslation, 0, len(missing)),
	}

	// Check if context is done before proceeding
	if ctx.Err() != nil {
		result.Remaining = missing
		return result
	}

	// Nothing to do if no missing translations
	if len(missing) == 0 {
		return result
	}

	// Defensive: allow callers to pass nil cfg safely
	if cfg == nil {
		result.Remaining = missing
		return result
	}

	// If no glossary configured, return everything as remaining
	if len(cfg.Glossary.Entries) == 0 {
		result.Remaining = missing
		return result
	}

	// Get or build glossary index with caching for O(1) lookups
	index := glossaryCache.getOrBuildIndex(cfg.Glossary.Entries)

	// Process each missing translation
	for i, missingItem := range missing {
		// Check for context cancellation periodically
		if ctx.Err() != nil {
			// Add all remaining items to needs translation and exit
			result.Remaining = append(result.Remaining, missing[i:]...)
			break
		}

		matched := processGlossaryItem(ctx, missingItem, index, &result)
		if !matched {
			result.Remaining = append(result.Remaining, missingItem)
		}
	}

	return result
}

// processGlossaryItem processes a single missing translation item for glossary matches.
func processGlossaryItem(
	ctx context.Context,
	missingItem types.MissingTranslation,
	index map[string]map[string]types.GlossaryEntry,
	result *glossaryResult,
) bool {
	sourceLocale := missingItem.SourceLocale
	targetLocale := missingItem.TargetLocale
	sourceValue := missingItem.SourceValue

	// Handle both string values and objects
	switch sourceValue := sourceValue.(type) {
	case string:
		// Handle individual string values
		glossaryTranslation := findMatchFromIndex(sourceValue, sourceLocale, targetLocale, index)
		if glossaryTranslation != "" {
			// Found a glossary match!
			result.Matched = append(result.Matched, types.LocaleContent{
				Locale: targetLocale,
				Path:   missingItem.Path,
				Value:  glossaryTranslation,
			})
			return true
		}
	case map[string]any, []any:
		// Handle complex types (objects and arrays) - create lookup once for both
		lookup := createGlossaryLookup(sourceLocale, targetLocale, index)

		switch value := sourceValue.(type) {
		case map[string]any:
			objectMatches, fullyMatched := findMatchesInObject(
				ctx, value, missingItem.Path, targetLocale, lookup)
			if len(objectMatches) > 0 {
				result.Matched = append(result.Matched, objectMatches...)
			}
			return fullyMatched
		case []any:
			arrayMatches, fullyMatched := findMatchesInArray(
				ctx, value, missingItem.Path, targetLocale, lookup)
			if len(arrayMatches) > 0 {
				result.Matched = append(result.Matched, arrayMatches...)
			}
			return fullyMatched
		}
	default:
		// Non-strings don't need translation: copy through here to avoid handing off to memory
		result.Matched = append(result.Matched, types.LocaleContent{
			Locale: missingItem.TargetLocale,
			Path:   missingItem.Path,
			Value:  missingItem.SourceValue,
		})
		return true
	}
	return false
}

// createGlossaryLookup creates a lookup function for glossary translation.
func createGlossaryLookup(
	sourceLocale types.Locale,
	targetLocale types.Locale,
	index map[string]map[string]types.GlossaryEntry,
) TranslationLookup {
	return func(value string) (string, bool) {
		if translation := findMatchFromIndex(value, sourceLocale, targetLocale, index); translation != "" {
			return translation, true
		}
		return "", false
	}
}

// findMatchFromIndex fetches the target translation via the entry map.
func findMatchFromIndex(
	sourceValue string,
	sourceLocale types.Locale,
	targetLocale types.Locale,
	index map[string]map[string]types.GlossaryEntry,
) string {
	sourceKey := config.NormalizeGlossaryTerm(sourceValue)

	// Look up entries for the source locale
	entryMap, ok := index[string(sourceLocale)]
	if !ok {
		return ""
	}

	// Find the entry that contains our source value
	if entry, ok := entryMap[sourceKey]; ok {
		// Get the target value from the same entry
		if targetValue, ok := entry[string(targetLocale)]; ok {
			return applyCasingPattern(sourceValue, targetValue)
		}
	}

	return ""
}

// applyCasingPattern applies the casing pattern from source text to target text.
// Supports three patterns: all-lowercase, all-uppercase, and simple title case.
// Mixed or complex casing patterns return the target value unchanged.
func applyCasingPattern(source, target string) string {
	if source == "" || target == "" {
		return target
	}

	switch {
	case source == caseLowerUnd.String(source):
		return caseLowerUnd.String(target)
	case source == caseUpperUnd.String(source):
		return caseUpperUnd.String(target)
	default:
		// Check for simple title case using proper Unicode handling
		firstSrc, sizeSrc := utf8.DecodeRuneInString(source)
		if unicode.IsUpper(firstSrc) && len(source) > sizeSrc && caseLowerUnd.String(source[sizeSrc:]) == source[sizeSrc:] {
			// Apply title case to target
			firstTgt, sizeTgt := utf8.DecodeRuneInString(target)
			return string(unicode.ToUpper(firstTgt)) + caseLowerUnd.String(target[sizeTgt:])
		}
		return target
	}
}

// getOrBuildIndex returns a cached index or builds a new one if not cached.
func (c *glossaryIndexCache) getOrBuildIndex(entries []types.GlossaryEntry) map[string]map[string]types.GlossaryEntry {
	hash := hashGlossaryEntries(entries)

	// Try to get from cache first (read lock)
	c.mu.RLock()
	if index, exists := c.cache[hash]; exists {
		c.mu.RUnlock()
		return index
	}
	c.mu.RUnlock()

	// Build outside lock to reduce contention
	built := buildIndex(entries)

	// Store if still absent
	c.mu.Lock()
	defer c.mu.Unlock()
	if index, exists := c.cache[hash]; exists {
		return index
	}
	c.cache[hash] = built
	return built
}

// buildIndex maps: sourceLocale -> normalisedSourceValue -> GlossaryEntry.
func buildIndex(entries []types.GlossaryEntry) map[string]map[string]types.GlossaryEntry {
	index := make(map[string]map[string]types.GlossaryEntry)

	for _, entry := range entries {
		for locale, value := range entry {
			normalized := config.NormalizeGlossaryTerm(value)
			if normalized == "" {
				continue // Skip empty normalized terms to avoid indexing under empty key
			}

			if index[locale] == nil {
				index[locale] = make(map[string]types.GlossaryEntry)
			}

			// Store the entire entry indexed by normalized value for this locale.
			// Keep the first definition to avoid silent overrides (first-win policy).
			if existingEntry, exists := index[locale][normalized]; !exists {
				index[locale][normalized] = entry
			} else {
				// Optional debug logging for duplicate normalized terms
				logger.Debug("Duplicate normalized glossary term detected",
					"locale", locale,
					"normalized", normalized,
					"original", value,
					"existing", existingEntry[locale])
			}
		}
	}

	return index
}

// hashGlossaryEntries creates a hash of glossary entries for cache keying.
func hashGlossaryEntries(entries []types.GlossaryEntry) string {
	hasher := sha256.New() //nolint:gosec // Not for cryptographic security, just for cache keying

	// Create canonical string representation of each entry, then sort entries
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys := make([]string, 0, len(entry))
		for locale := range entry {
			keys = append(keys, locale)
		}
		sort.Strings(keys)
		var builder strings.Builder
		for _, locale := range keys {
			builder.WriteString(locale)
			builder.WriteByte(':')
			builder.WriteString(entry[locale])
			builder.WriteByte(';')
		}
		parts = append(parts, builder.String())
	}
	sort.Strings(parts)
	for _, p := range parts {
		hasher.Write([]byte(p))
		hasher.Write([]byte("|"))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
