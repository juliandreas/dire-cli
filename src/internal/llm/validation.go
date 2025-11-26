package llm

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/juliandreas/dire/internal/utils"
)

// Static error definitions.
var (
	ErrPlaceholderCountMismatch = errors.New("placeholder count mismatch")
	ErrPlaceholderModified      = errors.New("placeholder was translated or modified")
	ErrEmptyStringOverwrite     = errors.New("translated empty string would overwrite existing value")
	ErrKeyStructureMismatch     = errors.New("LLM response has key structure mismatch")
	ErrArrayLengthMismatch      = errors.New("array length mismatch")
	ErrStructuralTypeMismatch   = errors.New("structural type mismatch")
)

// Precompiled regex patterns for placeholder validation.
var (
	doubleBracePattern = regexp.MustCompile(`\{\{[^{}]+\}\}`) // {{name}}
	dollarBracePattern = regexp.MustCompile(`\$\{[^}]+\}`)    // ${variable}
	singleBracePattern = regexp.MustCompile(`\{[^{}]+\}`)     // {count}
	printfPattern      = regexp.MustCompile(
		`%(?:\[[0-9]+\]|[0-9]+\$)?[-+#0]*[0-9]*(?:\.[0-9]+)?[bcdeEfFgGopqstTvxXU%]`,
	)
	icuPattern = regexp.MustCompile(`\{[^{}]+,\s*(plural|select|selectordinal|number|date|time)\b`)
)

// validateLLMResponse performs basic validation on LLM translation responses.
func validateLLMResponse(original, translated map[string]any) error {
	if err := validateKeyStructure(original, translated); err != nil {
		return fmt.Errorf("key structure validation failed: %w", err)
	}

	if err := validatePlaceholders(original, translated); err != nil {
		return fmt.Errorf("placeholder validation failed: %w", err)
	}

	// Note: Data type validation removed for maintainability
	// JSON marshaling/unmarshaling naturally handles type conversions (int->float64, etc.)
	// and the other validations catch the important structural issues

	if err := validateNoEmptyStrings(original, translated); err != nil {
		return fmt.Errorf("empty string validation failed: %w", err)
	}

	return nil
}

// validatePlaceholders checks that placeholder counts match between original and translated.
func validatePlaceholders(original, translated map[string]any) error {
	return validateRecursive(original, translated, []string{}, func(path []string, origVal, transVal any) error {
		origStr, origOk := origVal.(string)
		transStr, transOk := transVal.(string)
		if !origOk || !transOk {
			return nil // Skip non-string values for placeholder validation
		}

		// Validate each placeholder type separately to avoid overlaps
		placeholderTypes := []struct {
			name    string
			pattern *regexp.Regexp
		}{
			{"double-brace", doubleBracePattern},
			{"dollar-brace", dollarBracePattern},
			{"printf", printfPattern},
		}

		for _, pType := range placeholderTypes {
			if err := validatePlaceholderType(origStr, transStr, pType.name, pType.pattern, path); err != nil {
				return err
			}
		}

		// Validate single braces last, after removing overlapping patterns
		return validateSingleBraces(origStr, transStr, path)
	})
}

// validatePlaceholderType validates a specific type of placeholder.
func validatePlaceholderType(origStr, transStr, typeName string, pattern *regexp.Regexp, path []string) error {
	origPlaceholders := pattern.FindAllString(origStr, -1)
	transPlaceholders := pattern.FindAllString(transStr, -1)

	if len(origPlaceholders) != len(transPlaceholders) {
		return fmt.Errorf(
			"%w: %s placeholders at %s (original: %d, translated: %d)\n  Original: %q\n  Translated: %q",
			ErrPlaceholderCountMismatch, typeName, pathString(path),
			len(origPlaceholders), len(transPlaceholders),
			truncateForLog(origStr), truncateForLog(transStr))
	}

	// Check that exact placeholder names and counts are preserved
	origCounts := make(map[string]int)
	for _, placeholder := range origPlaceholders {
		origCounts[placeholder]++
	}

	transCounts := make(map[string]int)
	for _, placeholder := range transPlaceholders {
		transCounts[placeholder]++
	}

	// Check each original placeholder has same count in translation
	for placeholder, origCount := range origCounts {
		transCount := transCounts[placeholder]
		if transCount != origCount {
			return fmt.Errorf("%w: %s placeholder '%s' at %s (original: %d, translated: %d)\n  Original: %q\n  Translated: %q",
				ErrPlaceholderModified, typeName, placeholder, pathString(path), origCount, transCount,
				truncateForLog(origStr), truncateForLog(transStr))
		}
	}

	// Check no new placeholders were added in translation
	for placeholder := range transCounts {
		if _, exists := origCounts[placeholder]; !exists {
			return fmt.Errorf("%w: %s placeholder '%s' added at %s\n  Original: %q\n  Translated: %q",
				ErrPlaceholderModified, typeName, placeholder, pathString(path),
				truncateForLog(origStr), truncateForLog(transStr))
		}
	}

	return nil
}

// validateSingleBraces validates {count} style placeholders, excluding {{name}} and ${var} to prevent overlap.
func validateSingleBraces(origStr, transStr string, path []string) error {
	// If ICU-style detected, skip strict single-brace validation to avoid false positives
	if isICUMessage(origStr) || isICUMessage(transStr) {
		return nil
	}

	// Remove overlapping patterns first to get accurate single-brace count
	origCleaned := doubleBracePattern.ReplaceAllString(origStr, "")
	origCleaned = dollarBracePattern.ReplaceAllString(origCleaned, "")

	transCleaned := doubleBracePattern.ReplaceAllString(transStr, "")
	transCleaned = dollarBracePattern.ReplaceAllString(transCleaned, "")

	return validatePlaceholderType(origCleaned, transCleaned, "single-brace", singleBracePattern, path)
}

// validateNoEmptyStrings prevents empty strings from overwriting existing translations.
func validateNoEmptyStrings(original, translated map[string]any) error {
	return validateRecursive(original, translated, []string{}, func(path []string, origVal, transVal any) error {
		origStr, origOk := origVal.(string)
		transStr, transOk := transVal.(string)
		if !origOk || !transOk {
			return nil // Skip non-string values
		}

		if strings.TrimSpace(transStr) == "" && strings.TrimSpace(origStr) != "" {
			return fmt.Errorf("%w at %s", ErrEmptyStringOverwrite, pathString(path))
		}
		return nil
	})
}

// validateRecursive walks through nested maps and slices and applies validation function.
func validateRecursive(
	original,
	translated map[string]any,
	path []string,
	validator func([]string, any, any) error,
) error {
	for key, origVal := range original {
		transVal, exists := translated[key]
		if !exists {
			// Skip missing keys intentionally. Missing keys are handled by the service layer's
			// retry/aggregation. This validator focuses on blocking structural shape corruption (object/array mismatches).
			// Primitive type changes (e.g., string <-> number) are allowed by design.
			continue
		}

		currentPath := append([]string{}, path...)
		currentPath = append(currentPath, key)

		switch origVal := origVal.(type) {
		case map[string]any:
			if transMap, ok := transVal.(map[string]any); ok {
				if err := validateRecursive(origVal, transMap, currentPath, validator); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%w at %s: original is object, translated is %T",
					ErrStructuralTypeMismatch, pathString(currentPath), transVal)
			}
		case []any:
			if transSlice, ok := transVal.([]any); ok {
				if err := validateSlice(origVal, transSlice, currentPath, validator); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%w at %s: original is array, translated is %T",
					ErrStructuralTypeMismatch, pathString(currentPath), transVal)
			}
		default:
			if err := validator(currentPath, origVal, transVal); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateSlice validates array/slice elements recursively.
func validateSlice(original, translated []any, path []string, validator func([]string, any, any) error) error {
	// Check if slice lengths match first
	if len(original) != len(translated) {
		return fmt.Errorf("%w at %s: original has %d elements, translated has %d",
			ErrArrayLengthMismatch, pathString(path), len(original), len(translated))
	}

	// Validate each element
	for index, origVal := range original {
		transVal := translated[index]
		currentPath := append([]string{}, path...)
		currentPath = append(currentPath, fmt.Sprintf("[%d]", index))

		switch origVal := origVal.(type) {
		case map[string]any:
			if transMap, ok := transVal.(map[string]any); ok {
				if err := validateRecursive(origVal, transMap, currentPath, validator); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%w at %s: original is object, translated is %T",
					ErrStructuralTypeMismatch, pathString(currentPath), transVal)
			}
		case []any:
			if transSlice, ok := transVal.([]any); ok {
				if err := validateSlice(origVal, transSlice, currentPath, validator); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%w at %s: original is array, translated is %T",
					ErrStructuralTypeMismatch, pathString(currentPath), transVal)
			}
		default:
			if err := validator(currentPath, origVal, transVal); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateKeyStructure detects if AI translated JSON keys instead of just values.
func validateKeyStructure(original, translated map[string]any) error {
	if utils.HasKeyMismatch(original, translated) {
		return ErrKeyStructureMismatch
	}
	return nil
}

// pathString formats a path slice for error messages, showing "<root>" for empty paths.
func pathString(path []string) string {
	if len(path) == 0 {
		return "<root>"
	}
	return strings.Join(path, ".")
}

// truncateForLog truncates long strings to prevent log bloat while preserving diagnostic value.
func truncateForLog(text string) string {
	const maxRunes = 500
	runes := []rune(text)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "…"
	}
	return text
}

// isICUMessage detects ICU MessageFormat patterns to avoid false positives in single-brace validation.
func isICUMessage(s string) bool {
	return icuPattern.MatchString(s)
}
