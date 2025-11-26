//nolint:lll // Prompts should remain as single lines to avoid interfering with AI processing
package llm

import (
	"fmt"
	"strings"

	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

// CreateTranslationPrompt builds the complete prompt for the translation request.
func CreateTranslationPrompt(
	sourceLocale types.Locale,
	targetLocale types.Locale,
	projectInfo string,
	formality constants.Formality,
	toTranslate map[string]any,
	glossary []types.GlossaryEntry,
	userContext string,
) (string, error) {
	jsonData, err := toJSON(toTranslate)
	if err != nil {
		return "", fmt.Errorf("failed to marshal translation data: %w", err)
	}

	// Filter and format glossary for this language pair and batch content
	glossarySection := buildGlossarySection(sourceLocale, targetLocale, glossary, toTranslate)

	// Build context section with user-provided context if available
	contextSection := buildContextSection(projectInfo, userContext)

	return fmt.Sprintf(`You are an expert translator specializing in software localization from %s to %s.

# PROJECT CONTEXT
%s

This context helps you understand the application domain and choose appropriate terminology, tone, and register for the translations.

%s

%s

# TRANSLATION REQUIREMENTS
- Translate only the values, not the JSON keys
- Maintain the exact same nested structure and data types (strings stay strings; numbers/booleans/null remain untouched)
- For arrays: preserve exact structure, length, and item order (no reordering). Do not add or remove elements; indices must remain unchanged. Translate only string values; keep numbers/booleans/null unchanged. If an array contains objects, translate only string fields inside those objects while preserving object keys and structure.
- **CRITICAL: Preserve placeholders exactly - do not translate words inside them.** Placeholders are code, not text to translate. Copy them character-for-character, preserving exact spelling, capitalization, and spacing. Examples: {date}→{date}, {userId}→{userId}, {{count}}→{{count}}, { name }→{ name }. Supported formats: {x}, {{x}}, ${x}, { x } (with spaces), printf-style tokens like %%s, %%d, %%v, and ICU patterns like {count, plural, ...}. This applies to ALL placeholder content regardless of language. Never translate, modify, add, or remove any part of a placeholder. For ICU patterns, translate only literal text outside the control syntax.
- Keep brand names, product names, special characters, and formatting unchanged; punctuation may follow target language conventions
- Follow %s regional language conventions and cultural norms
- Ensure translations feel natural to native speakers
- Handle empty strings by keeping them empty, and preserve relative length/style of UI elements (buttons concise, descriptions longer)
- For short strings (like "OK", "Yes", "No"), use standard target language conventions
- If a value is already in the target language or contains only non-translatable content (IDs, URLs, hashes, file paths, emails), return it unchanged

# TRANSLATION PRINCIPLES
- Preserve the intent and meaning while adapting punctuation and spacing to target language conventions
- Maintain numerical values, counts, formatting patterns, and use terminology consistent with the project domain

# JSON TO TRANSLATE
%s

Return ONLY the translated JSON object without any additional text, explanations, markdown, backticks, or code block formatting. The response must be strict, valid JSON (UTF-8, no comments, no trailing commas) that can be parsed directly.`,
		sourceLocale,
		targetLocale,
		contextSection,
		formatFormalityInstructions(formality),
		glossarySection,
		targetLocale,
		jsonData), nil
}

// buildContextSection combines project info with user-provided context.
func buildContextSection(projectInfo, userContext string) string {
	if userContext == "" {
		return projectInfo
	}

	if projectInfo == "" {
		return userContext
	}

	return fmt.Sprintf("%s\n\nAdditional Context: %s", projectInfo, userContext)
}

// buildGlossarySection creates the glossary section for the prompt.
func buildGlossarySection(sourceLocale, targetLocale types.Locale, glossary []types.GlossaryEntry, toTranslate map[string]any) string {
	if len(glossary) == 0 {
		return ""
	}

	// Filter glossary entries that have both source and target locales
	languageFiltered := filterGlossaryByLanguage(sourceLocale, targetLocale, glossary)

	if len(languageFiltered) == 0 {
		return ""
	}

	// Further filter by relevance to this specific batch
	relevantEntries := filterGlossaryByRelevance(languageFiltered, sourceLocale, toTranslate)

	if len(relevantEntries) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("# GLOSSARY TERMS\n")
	builder.WriteString("Use these specific translations for the following terms. Adapt them for proper grammar, inflection, and context as needed:\n\n")

	for _, entry := range relevantEntries {
		sourceValue := entry[string(sourceLocale)]
		targetValue := entry[string(targetLocale)]
		builder.WriteString(fmt.Sprintf("- %s → %s\n", sourceValue, targetValue))
	}

	return builder.String()
}

// filterGlossaryByLanguage returns only glossary entries that have both source and target locales.
func filterGlossaryByLanguage(sourceLocale, targetLocale types.Locale, glossary []types.GlossaryEntry) []types.GlossaryEntry {
	var filtered []types.GlossaryEntry

	for _, entry := range glossary {
		sourceValue, hasSource := entry[string(sourceLocale)]
		targetValue, hasTarget := entry[string(targetLocale)]

		// Only include entries that have both locales and non-empty values
		if hasSource && hasTarget && strings.TrimSpace(sourceValue) != "" && strings.TrimSpace(targetValue) != "" {
			filtered = append(filtered, entry)
		}
	}

	return filtered
}

// filterGlossaryByRelevance returns only glossary entries that appear in the batch content.
func filterGlossaryByRelevance(glossary []types.GlossaryEntry, sourceLocale types.Locale, toTranslate map[string]any) []types.GlossaryEntry {
	// Extract all text content from the batch
	batchText := extractAllTextFromBatch(toTranslate)
	if batchText == "" {
		return nil
	}

	lowerBatchText := strings.ToLower(batchText)
	var relevant []types.GlossaryEntry

	for _, entry := range glossary {
		sourceValue := entry[string(sourceLocale)]
		searchTerm := strings.ToLower(sourceValue)

		// Inclusive substring matching
		if strings.Contains(lowerBatchText, searchTerm) {
			relevant = append(relevant, entry)
		}
	}

	return relevant
}

// extractAllTextFromBatch recursively extracts all string values from the nested structure.
func extractAllTextFromBatch(data map[string]any) string {
	var textParts []string
	extractTextRecursive(data, &textParts)
	return strings.Join(textParts, " ")
}

// extractTextRecursive recursively walks through the nested structure and collects all string values.
func extractTextRecursive(data any, textParts *[]string) {
	switch val := data.(type) {
	case map[string]any:
		for _, value := range val {
			extractTextRecursive(value, textParts)
		}
	case []any:
		for _, item := range val {
			extractTextRecursive(item, textParts)
		}
	case string:
		if strings.TrimSpace(val) != "" {
			*textParts = append(*textParts, val)
		}
	}
}

// formatFormalityInstructions formats formality-specific instructions.
func formatFormalityInstructions(formality constants.Formality) string {
	const automaticInstructions = `# FORMALITY INSTRUCTIONS
- Detect the formality level of each source text string and match it in the translated text
- If the source uses formal language (professional, respectful), use formal pronouns and constructions in the target language
- If the source uses informal language (casual, friendly), use informal pronouns and constructions in the target language
- If source strings have different formality levels within this batch, preserve each string's individual formality level
- When formality is ambiguous, choose formal for system messages and errors, informal for user-facing interactions and greetings
- Maintain consistency in terminology and terminology usage across the batch. Preserve each string's individual formality; do not unify formality across strings that differ`

	switch formality {
	case constants.FormalityFormal:
		return `# FORMALITY INSTRUCTIONS
- Use formal language and respectful approach throughout all translations
- Use formal pronouns when addressing users (appropriate to target language)
- Use polite, professional constructions and vocabulary
- Maintain dignity and respect in all translated content
- Choose the more formal variant when multiple options exist`

	case constants.FormalityInformal:
		return `# FORMALITY INSTRUCTIONS
- Use casual, friendly language throughout all translations
- Use informal pronouns when addressing users (appropriate to target language)
- Use conversational style and approachable vocabulary
- Make the content feel warm and accessible
- Choose the more casual variant when multiple options exist`

	default: // constants.FormalityAutomatic or any other value
		return automaticInstructions
	}
}

// CreateRephrasePrompt creates a prompt for generating rephrases.
func CreateRephrasePrompt(
	key string,
	currentValue string,
	locale types.Locale,
	projectInfo string,
	userContext string,
) (string, error) {
	languageCode := string(locale)

	// Build context section with user-provided context if available
	contextSection := buildContextSection(projectInfo, userContext)

	return fmt.Sprintf(`You are an expert language editor specializing in %s text for software applications.

# PROJECT CONTEXT
%s

This context helps you understand the application domain and choose appropriate terminology and style for the rephrasing suggestions.

# TASK
Generate 5 alternative phrasings for the given translation. Each suggestion should:
- Maintain the same meaning and intent
- Be natural and fluent in %s with proper %s regional conventions
- Preserve the original's formality level unless explicitly creating a formality variation
- Be appropriate for software UI text and the specific UI context indicated by the key
- Consider the UI element type: buttons/labels stay concise, descriptions/help text can be longer
- Preserve any variables or placeholders (like {{name}}, {count}, ${variable}, etc.)
- Keep brand names and product names unchanged; do not translate or inflect them
- Vary in approach (word choice, sentence structure, emphasis) while maintaining appropriateness
- Be consistent with standard software terminology for this domain
- Provide at least one succinct option suitable for tight UI (e.g., buttons or compact labels)

# QUALITY RANKING CRITERIA
Order suggestions by naturalness and UI appropriateness:
- suggestion1: Most natural and idiomatic for native speakers
- suggestion2-4: Natural alternatives with different approaches or emphasis
- suggestion5: Acceptable but less preferred option

# KEY INFORMATION
Key: %s
Current translation: "%s"

# RESPONSE FORMAT
Return ONLY a JSON object with numbered suggestion keys, ordered by quality.
Example format: {"suggestion1": "Best alternative", "suggestion2": "Second best", "suggestion3": "Third option"}

Do not include explanations, markdown, backticks, or code block formatting. The response must be strict, valid JSON (UTF-8, no comments, no trailing commas) that can be parsed directly.`,
		languageCode,
		contextSection,
		languageCode,
		locale,
		key,
		currentValue), nil
}

// toJSON converts a map to a JSON string with proper formatting.
func toJSON(data any) (string, error) {
	bytes, err := utils.MarshalJSON(data, "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(bytes), nil
}
