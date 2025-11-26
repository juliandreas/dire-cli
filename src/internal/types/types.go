// Package types provides core data structures for the application.
package types

// Locale represents a language/region identifier (e.g., "en-US", "fr-CA").
type Locale string

// LocaleFile represents a parsed locale file.
type LocaleFile struct {
	Path       string
	Locale     Locale
	Contents   map[string]any
	LineEnding string // "", "\n", "\r\n", or "\r" - preserves original file's line endings; empty is treated as "\n"
}

// LocalePair represents a source-target locale pair.
type LocalePair struct {
	Source Locale
	Target Locale
}

// MissingTranslation represents a translation that exists in source but not in target.
type MissingTranslation struct {
	Path         []string
	SourceLocale Locale
	SourceValue  any
	TargetLocale Locale
}

// LocaleContent represents content with locale, path, and value.
// Used for translation output, rephrase input, and any localized content.
type LocaleContent struct {
	Locale Locale
	Path   []string
	Value  any
}

// TranslationValue represents a translatable value with its JSON path.
type TranslationValue struct {
	Path  []string // JSON path to the value (e.g., ["products", "sort_by"])
	Value string   // The actual text to translate
}

// GlossaryEntry represents a single glossary term with locale-specific translations.
type GlossaryEntry map[string]string
