package flags

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
)

// Flag Categories
//
// When adding a new flag, categorize it here and update the helpers in ValidateFlagCombinations.
// This serves as a reference for understanding flag relationships and conflicts.
//
// TRANSLATION MODES (mutually exclusive with each other and read-only modes):
//   • StubMode - creates stub translations with empty strings
//   • SourcedOnly - uses glossary + memory, skips external services
//   • GlossaryOnly - uses glossary only, skips external services
//   • MemoryOnly - uses memory only, skips external services
//   • Rephrase - generates rephrase options for existing translations
//
// MODIFICATION MODES:
//   • Overwrite - overwrites existing translations (requires GlossaryOnly)
//
// READ-ONLY MODES (standalone, cannot combine with translation/modification modes):
//   • Check - lints translation completeness
//   • Prune - removes orphaned keys
//
// MODIFIERS (work with translation modes, not read-only modes):
//   • Keys - filters to specific translation keys
//   • Locale - specifies single locale (rephrase only)
//   • Context - provides additional context (LLM providers only)
//   • IncludeStubs - includes empty strings as missing translations
//   • Format - output format (check mode only)
//
// CONFIG OVERRIDES (work with any mode):
//   • Directory, Locales, Indent, Formality
//   • ProjectName, ProjectDesc, ProjectDom
//   • ProviderActive, ProviderModel, ProviderTemperature, ProviderMaxTokens
//   • TrimStrings

const (
	MaxContextLength = 500 // MaxContextLength defines the maximum allowed length for the context flag.
)

var (
	errStubWithOtherModes             = errors.New("--stub flag cannot be combined with other translation mode flags")
	errSourcedWithGlossaryOrMemory    = errors.New("--sourced flag cannot be used with --glossary or --memory")
	errGlossaryWithMemory             = errors.New("--glossary and --memory cannot be combined (use --sourced for both)")
	errRephraseRequiresKeys           = errors.New("--rephrase flag requires exactly one key in --keys")
	errRephraseRequiresLocale         = errors.New("--rephrase flag requires --locale to be specified")
	errRephraseMultipleKeys           = errors.New("--rephrase mode accepts only one key, got multiple keys")
	errRephraseWithOtherModes         = errors.New("--rephrase flag cannot be combined with other mode flags")
	errLocaleRequiresRephrase         = errors.New("--locale flag can only be used with --rephrase")
	errContextTooLong                 = errors.New("--context flag exceeds maximum length")
	errContextOnlyWithLLMs            = errors.New("--context flag only works with LLM providers")
	errCheckWithOtherModesConflict    = errors.New("--check flag cannot be used with other modes")
	errFormatWithoutCheckConflict     = errors.New("--format flag can only be used with --check")
	errInvalidFormatValue             = errors.New("--format must be one of: default, json")
	errRephraseWithTranslatorConflict = errors.New("--rephrase flag is not supported with translators")
	errOverwriteOnlyWithGlossary      = errors.New(
		"--overwrite flag requires --glossary and cannot be combined with other translation modes",
	)
	errPruneWithOtherFlags = errors.New("--prune flag cannot be used with other flags")
)

// ValidateFlagCombinations checks for conflicting flag combinations and returns an error if found.
func ValidateFlagCombinations(fs *Flags, appConfig *config.Config) error {
	// Semantic flag groupings - update these when adding new flags
	// These helpers make validation logic more readable and maintainable
	hasSourcedMode := fs.SourcedOnly || fs.GlossaryOnly || fs.MemoryOnly
	hasTranslationMode := fs.StubMode || hasSourcedMode || fs.Rephrase
	hasModificationMode := fs.Overwrite
	hasReadOnlyMode := fs.Check || fs.Prune

	hasKeyFilters := len(fs.Keys) > 0 || fs.Locale != ""
	hasContentModifiers := fs.Context != "" || fs.IncludeStubs
	hasOutputModifiers := fs.Format != ""

	if err := validateGeneralConflicts(fs, hasSourcedMode, hasTranslationMode, hasModificationMode,
		hasReadOnlyMode, hasKeyFilters, hasContentModifiers); err != nil {
		return err
	}

	if err := validateCheck(fs, hasTranslationMode, hasModificationMode, hasOutputModifiers); err != nil {
		return err
	}

	if err := validateRephrase(fs, appConfig); err != nil {
		return err
	}

	if err := validateContext(fs, appConfig); err != nil {
		return err
	}

	return nil
}

// validateGeneralConflicts checks for general flag conflicts.
func validateGeneralConflicts(fs *Flags, hasSourcedMode, hasTranslationMode, hasModificationMode,
	hasReadOnlyMode, hasKeyFilters, hasContentModifiers bool,
) error {
	if err := validatePrune(fs, hasTranslationMode, hasModificationMode,
		hasKeyFilters, hasContentModifiers); err != nil {
		return err
	}

	if err := validateOverwrite(fs, hasReadOnlyMode); err != nil {
		return err
	}

	if err := validateStub(fs, hasSourcedMode); err != nil {
		return err
	}

	if err := validateSourcedFlags(fs); err != nil {
		return err
	}

	return nil
}

// validatePrune checks that prune is not combined with other flags.
func validatePrune(fs *Flags, hasTranslationMode, hasModificationMode,
	hasKeyFilters, hasContentModifiers bool,
) error {
	if !fs.Prune {
		return nil
	}

	// Prune must be standalone - conflicts with all modes and modifiers
	hasAnyOtherMode := hasTranslationMode || hasModificationMode || fs.Check
	hasAnyModifier := hasKeyFilters || hasContentModifiers

	if hasAnyOtherMode || hasAnyModifier {
		return errPruneWithOtherFlags
	}

	return nil
}

// validateOverwrite checks that overwrite only works with glossary flag.
func validateOverwrite(fs *Flags, hasReadOnlyMode bool) error {
	if !fs.Overwrite {
		return nil
	}

	// Overwrite requires glossary-only mode with no other modes
	// Check for any other translation mode besides glossary (sourced, memory, stub, rephrase)
	hasOtherMode := fs.SourcedOnly || fs.MemoryOnly || fs.StubMode || fs.Rephrase
	if !fs.GlossaryOnly || hasOtherMode || hasReadOnlyMode {
		return errOverwriteOnlyWithGlossary
	}

	return nil
}

// validateStub checks stub mode conflicts.
func validateStub(fs *Flags, hasSourcedMode bool) error {
	if !fs.StubMode {
		return nil
	}

	// Stub cannot be combined with sourced modes, rephrase, or include-stubs modifier
	if hasSourcedMode || fs.Rephrase || fs.IncludeStubs {
		return errStubWithOtherModes
	}

	return nil
}

// validateSourcedFlags checks sourced/glossary/memory flag combinations.
func validateSourcedFlags(fs *Flags) error {
	if fs.SourcedOnly && (fs.GlossaryOnly || fs.MemoryOnly) {
		return errSourcedWithGlossaryOrMemory
	}

	if fs.GlossaryOnly && fs.MemoryOnly {
		return errGlossaryWithMemory
	}

	return nil
}

// validateRephrase checks all rephrase-related validation rules.
func validateRephrase(fs *Flags, appConfig *config.Config) error {
	// Locale flag can only be used with rephrase
	if !fs.Rephrase && fs.Locale != "" {
		return errLocaleRequiresRephrase
	}

	if !fs.Rephrase {
		return nil
	}

	// Rephrase cannot be combined with other translation modes
	if fs.StubMode || fs.SourcedOnly || fs.GlossaryOnly || fs.MemoryOnly || fs.Overwrite {
		return errRephraseWithOtherModes
	}

	// Check if using translator
	if appConfig != nil && constants.IsTranslatorProvider(appConfig.Providers.Active) {
		return errRephraseWithTranslatorConflict
	}

	// Rephrase mode requires exactly one key and a locale
	if len(fs.Keys) == 0 {
		return errRephraseRequiresKeys
	}
	if len(fs.Keys) > 1 {
		return errRephraseMultipleKeys
	}
	if fs.Locale == "" {
		return errRephraseRequiresLocale
	}

	return nil
}

// validateCheck checks check mode validation rules.
func validateCheck(fs *Flags, hasTranslationMode, hasModificationMode, hasOutputModifiers bool) error {
	if fs.Check {
		// Check mode is mutually exclusive with translation and modification modes
		if hasTranslationMode || hasModificationMode {
			return errCheckWithOtherModesConflict
		}
	}

	// Format flag can only be used with check mode
	if hasOutputModifiers && !fs.Check {
		return errFormatWithoutCheckConflict
	}

	// Validate format values (case-insensitive)
	if fs.Format != "" {
		switch strings.ToLower(fs.Format) {
		case constants.FormatDefault, constants.FormatJSON:
			// Valid format values
		default:
			return errInvalidFormatValue
		}
	}

	return nil
}

// validateContext checks context flag validation rules.
func validateContext(fs *Flags, appConfig *config.Config) error {
	// Only validate if context is provided
	if fs.Context == "" {
		return nil
	}

	// Context length validation (count runes, not bytes, for Unicode support)
	runeCount := utf8.RuneCountInString(fs.Context)
	if runeCount > MaxContextLength {
		return fmt.Errorf("%w: cannot exceed %d characters (got %d)", errContextTooLong, MaxContextLength, runeCount)
	}

	// Context only works with LLM providers
	isTranslator := appConfig != nil && constants.IsTranslatorProvider(appConfig.Providers.Active)
	isNonLLMMode := fs.StubMode || fs.SourcedOnly || fs.GlossaryOnly || fs.MemoryOnly || fs.Check

	if isTranslator || isNonLLMMode {
		return errContextOnlyWithLLMs
	}

	return nil
}
