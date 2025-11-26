package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/utils"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/text/cases"
)

// entryBlock represents a block of text containing comments and one entry line.
type entryBlock struct {
	Lines   []string // All lines in this block (comments + entry line)
	SortKey string   // The value to sort by (first locale's value)
}

const (
	entriesArrayRegexGroups = 4 // Regex groups: [full_match, "entries = [", content, "]"]

	// TOML syntax constants.
	entriesArrayRegex  = `(?s)(entries\s*=\s*\[)(.*?)(\])`
	inlineTableStart   = "{"
	inlineTableEnd     = "}"
	keyValueSeparator  = "="
	listSeparator      = ","
	commentPrefix      = "#"
	quoteChars         = `"'`
	parentDirTraversal = ".."
)

var ErrEntryCountMismatch = errors.New("glossary sorting changed entry count")

// sortGlossaryInFile sorts glossary entries while preserving all file formatting.
func sortGlossaryInFile(filePath string) (*Config, error) {
	// Clean the path and validate for security
	cleanPath := filepath.Clean(filePath)
	// Disallow parent directory traversal beyond the working directory
	if cleanPath == parentDirTraversal || strings.HasPrefix(cleanPath, parentDirTraversal+string(os.PathSeparator)) {
		return nil, ErrInvalidConfigPath
	}

	// Read the original file content as text
	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	originalContent := string(content)

	// Parse the TOML config to get entries for sorting
	var cfg Config
	if err = toml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse TOML config: %w", err)
	}

	// Check if we need to sort
	if !cfg.Glossary.AutoSort || len(cfg.Glossary.Entries) == 0 {
		return &cfg, nil // Nothing to sort
	}

	// Store original count for validation
	originalCount := len(cfg.Glossary.Entries)

	// Detect line ending style from original content
	lineEnding := utils.DetectLineEnding([]byte(originalContent))

	// Sort entries using block-based approach
	newContent := sortEntries(originalContent)

	// Only write if content changed
	if newContent != originalContent {
		// Ensure the final content uses the same line endings as the original file
		finalContent := normalizeLineEndings(newContent, lineEnding)
		if writeErr := os.WriteFile(cleanPath, []byte(finalContent), constants.SecureFilePermissions); writeErr != nil {
			return nil, fmt.Errorf("failed to write sorted config to %s: %w", cleanPath, writeErr)
		}
	}

	// Re-parse the TOML after sorting to get the updated config structure
	if err = toml.Unmarshal([]byte(newContent), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse sorted TOML config: %w", err)
	}

	// Validate that entry count hasn't changed
	if len(cfg.Glossary.Entries) != originalCount {
		return nil, fmt.Errorf("%w: expected %d, got %d",
			ErrEntryCountMismatch, originalCount, len(cfg.Glossary.Entries))
	}

	return &cfg, nil
}

// sortEntries sorts glossary entries using a block-based approach
// that preserves all formatting, comments, indentation, and line endings.
func sortEntries(content string) string {
	// Find the glossary entries array using regex
	entriesPattern := regexp.MustCompile(entriesArrayRegex)
	matches := entriesPattern.FindStringSubmatch(content)

	if len(matches) != entriesArrayRegexGroups {
		// No entries array found, return original content
		return content
	}

	prefix := matches[1]          // "entries = ["
	originalEntries := matches[2] // the entries content
	suffix := matches[3]          // "]"

	// Extract the locale order from the first entry
	localeOrder := extractLocaleOrder(originalEntries)

	// Determine the first locale from the first entry
	firstLocale := ""
	if len(localeOrder) > 0 {
		firstLocale = localeOrder[0]
	}

	// Parse entries into blocks
	blocks := parseBlocks(originalEntries, firstLocale, localeOrder)

	// Sort blocks by their sort keys
	sortBlocks(blocks)

	// Reconstruct the entries section using LF (will be normalized later)
	newEntries := reconstructBlocks(blocks, utils.LineEndingLF)

	// Replace the entries section with proper formatting (using LF, will be normalized later)
	var replacement string
	if strings.TrimSpace(newEntries) == "" {
		// Empty entries
		replacement = prefix + suffix
	} else {
		// Ensure proper line breaks around entries
		replacement = prefix + utils.LineEndingLF + newEntries + utils.LineEndingLF + suffix
	}

	newContent := entriesPattern.ReplaceAllString(content, replacement)

	return newContent
}

// extractLocaleOrder extracts the order of locales from the entry with the most locales.
func extractLocaleOrder(entriesContent string) []string {
	lines := strings.SplitSeq(entriesContent, utils.LineEndingLF)

	var bestLocales []string
	bestCount := 0

	for line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, commentPrefix) {
			continue
		}

		// Check if this is an entry line
		if strings.Contains(trimmed, inlineTableStart) && strings.Contains(trimmed, inlineTableEnd) {
			locales := getLocales(trimmed)
			// Use the entry with the most locales to establish the order
			if len(locales) > bestCount {
				bestLocales = locales
				bestCount = len(locales)
			}
		}
	}

	return bestLocales
}

// getLocales extracts all locale keys from an entry line in order.
func getLocales(line string) []string {
	start := strings.Index(line, inlineTableStart)
	end := strings.LastIndex(line, inlineTableEnd)
	if start == -1 || end == -1 || start >= end {
		return nil
	}

	content := line[start+1 : end]
	parts := strings.Split(content, listSeparator)

	locales := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		eqIndex := strings.Index(part, keyValueSeparator)
		if eqIndex <= 0 {
			continue
		}

		key := strings.TrimSpace(part[:eqIndex])
		key = strings.Trim(key, quoteChars)
		locales = append(locales, key)
	}

	return locales
}

// parseEntryMap parses the entry content into a map of locale to value.
func parseEntryMap(entryContent string) map[string]string {
	parts := strings.Split(entryContent, listSeparator)
	entryMap := make(map[string]string, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		eqIndex := strings.Index(part, keyValueSeparator)
		if eqIndex <= 0 {
			continue
		}

		key := strings.TrimSpace(part[:eqIndex])
		key = strings.Trim(key, quoteChars)

		value := strings.TrimSpace(part[eqIndex+1:])
		value = strings.Trim(value, quoteChars)

		entryMap[key] = value
	}

	return entryMap
}

// buildReorderedPairs builds the reordered locale-value pairs.
func buildReorderedPairs(entryMap map[string]string, localeOrder []string) []string {
	pairs := make([]string, 0, len(entryMap))

	// Add locales in the specified order
	for _, locale := range localeOrder {
		if value, exists := entryMap[locale]; exists {
			pairs = append(pairs, fmt.Sprintf(`%q = %q`, locale, value))
		}
	}

	// Add any locales that weren't in the order (shouldn't happen normally)
	for locale, value := range entryMap {
		if !slices.Contains(localeOrder, locale) {
			pairs = append(pairs, fmt.Sprintf(`%q = %q`, locale, value))
		}
	}

	return pairs
}

// reorderEntry reorders the locales in an entry line according to the specified order.
func reorderEntry(line string, localeOrder []string) string {
	if len(localeOrder) == 0 {
		return line
	}

	// If the line is fully commented out, don't reorder it
	if strings.HasPrefix(strings.TrimSpace(line), commentPrefix) {
		return line
	}

	// Find the entry boundaries
	start := strings.Index(line, inlineTableStart)
	end := strings.LastIndex(line, inlineTableEnd)
	if start == -1 || end == -1 {
		return line
	}

	// Extract parts
	beforeEntry := line[:start]
	entryContent := line[start+1 : end]
	afterEntry := line[end+1:]

	// Extract comments and indentation
	commentBeforeIndex := strings.LastIndex(beforeEntry, commentPrefix)
	prefixComment := ""
	indent := beforeEntry
	if commentBeforeIndex != -1 {
		prefixComment = beforeEntry[commentBeforeIndex:]
		indent = beforeEntry[:commentBeforeIndex]
	}

	commentAfterIndex := strings.Index(afterEntry, commentPrefix)
	suffixComment := ""
	trailingPart := afterEntry
	if commentAfterIndex != -1 {
		suffixComment = afterEntry[commentAfterIndex:]
		trailingPart = afterEntry[:commentAfterIndex]
	}

	// Parse and reorder
	entryMap := parseEntryMap(entryContent)
	if len(entryMap) == 0 {
		return line
	}

	pairs := buildReorderedPairs(entryMap, localeOrder)
	if len(pairs) == 0 {
		return line
	}

	// Rebuild the line
	result := ""
	if prefixComment != "" {
		result = prefixComment + " "
	} else {
		result = indent
	}
	result += inlineTableStart + " " + strings.Join(pairs, listSeparator+" ") + " " + inlineTableEnd
	if strings.Contains(trailingPart, listSeparator) {
		result += listSeparator
	}
	if suffixComment != "" {
		result += " " + suffixComment
	}

	return result
}

// parseBlocks parses the entries content into blocks and reorders locales.
func parseBlocks(entriesContent, firstLocale string, localeOrder []string) []entryBlock {
	lines := strings.Split(entriesContent, utils.LineEndingLF)
	var blocks []entryBlock
	var currentBlock []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if this is an entry line (contains inline table)
		if strings.Contains(trimmed, inlineTableStart) && strings.Contains(trimmed, inlineTableEnd) {
			// Reorder the entry line according to locale order
			reorderedLine := reorderEntry(line, localeOrder)

			// Extract sort key and create block
			sortKey := getSortKey(trimmed, firstLocale)
			if sortKey != "" {
				currentBlock = append(currentBlock, reorderedLine)
				blocks = append(blocks, entryBlock{
					Lines:   cleanBlock(currentBlock),
					SortKey: sortKey,
				})
				currentBlock = nil // Reset for next block
			}
		} else if trimmed != "" {
			// This is a non-empty comment or other line - add to current block
			currentBlock = append(currentBlock, line)
		}
		// Skip empty lines between entries to prevent spacing issues
	}

	// Handle any remaining lines as a final block (trailing comments)
	if len(currentBlock) > 0 {
		cleanedBlock := cleanBlock(currentBlock)
		if len(cleanedBlock) > 0 {
			blocks = append(blocks, entryBlock{
				Lines:   cleanedBlock,
				SortKey: "", // No sort key for trailing comments
			})
		}
	}

	return blocks
}

// cleanBlock removes leading and trailing empty lines from a block while preserving internal structure.
func cleanBlock(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}

	// Remove leading empty lines
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	// Remove trailing empty lines
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	if start >= end {
		return nil
	}

	return lines[start:end]
}

// getSortKey extracts the sort key for a specific locale from an entry line.
func getSortKey(line, targetLocale string) string {
	if targetLocale == "" {
		// Fallback to first key-value pair if no target locale
		return getFirstValue(line)
	}

	// Find content between { and }
	start := strings.Index(line, inlineTableStart)
	end := strings.LastIndex(line, inlineTableEnd)
	if start == -1 || end == -1 || start >= end {
		return ""
	}

	content := line[start+1 : end]

	// Split by comma and look for the target locale
	parts := strings.SplitSeq(content, listSeparator)
	for part := range parts {
		part = strings.TrimSpace(part)
		eqIndex := strings.Index(part, keyValueSeparator)
		if eqIndex <= 0 {
			continue
		}

		key := strings.TrimSpace(part[:eqIndex])
		key = strings.Trim(key, quoteChars)

		if key == targetLocale {
			value := strings.TrimSpace(part[eqIndex+1:])
			value = strings.Trim(value, quoteChars)
			return value
		}
	}

	// If target locale not found, fallback to first value
	return getFirstValue(line)
}

// getFirstValue extracts the first value from an entry line as fallback.
func getFirstValue(line string) string {
	// Find content between { and }
	start := strings.Index(line, inlineTableStart)
	end := strings.LastIndex(line, inlineTableEnd)
	if start == -1 || end == -1 || start >= end {
		return ""
	}

	content := line[start+1 : end]

	// Find the first key-value pair
	parts := strings.Split(content, listSeparator)
	if len(parts) == 0 {
		return ""
	}

	firstPart := strings.TrimSpace(parts[0])
	eqIndex := strings.Index(firstPart, keyValueSeparator)
	if eqIndex <= 0 {
		return ""
	}

	value := strings.TrimSpace(firstPart[eqIndex+1:])
	// Remove quotes
	value = strings.Trim(value, quoteChars)

	return value
}

// sortBlocks sorts the blocks by their sort keys (case-insensitive).
func sortBlocks(blocks []entryBlock) {
	fold := cases.Fold()

	sort.SliceStable(blocks, func(left, right int) bool {
		// Blocks without sort keys (comments) stay at the end
		if blocks[left].SortKey == "" && blocks[right].SortKey == "" {
			return false // Preserve relative order
		}
		if blocks[left].SortKey == "" {
			return false // Empty sort key goes after non-empty
		}
		if blocks[right].SortKey == "" {
			return true // Non-empty sort key goes before empty
		}

		// Sort by sort key (case-insensitive)
		return fold.String(blocks[left].SortKey) < fold.String(blocks[right].SortKey)
	})
}

// reconstructBlocks reconstructs the entries content from sorted blocks.
func reconstructBlocks(blocks []entryBlock, lineEnding string) string {
	var allLines []string

	for _, block := range blocks {
		allLines = append(allLines, block.Lines...)
	}

	return strings.Join(allLines, lineEnding)
}

// normalizeLineEndings ensures the content uses the specified line ending consistently.
func normalizeLineEndings(content, targetLineEnding string) string {
	// First normalize all line endings to LF
	content = strings.ReplaceAll(content, utils.LineEndingCRLF, utils.LineEndingLF)
	content = strings.ReplaceAll(content, utils.LineEndingCR, utils.LineEndingLF)

	// Then convert to target line ending if it's not LF
	if targetLineEnding != utils.LineEndingLF {
		content = strings.ReplaceAll(content, utils.LineEndingLF, targetLineEnding)
	}

	return content
}
