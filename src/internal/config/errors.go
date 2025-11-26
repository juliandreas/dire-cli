package config

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrInvalidConfig indicates configuration validation failed.
	ErrInvalidConfig = errors.New("invalid configuration")

	// Locale validation errors.
	ErrLocaleBCP47Required = errors.New(
		"must use BCP 47 format like 'en-US', simple language codes like 'en' are not allowed")
	ErrLocaleEmpty          = errors.New("locale cannot be empty")
	ErrLocaleRegionRequired = errors.New("must include region component (e.g., 'en-US' not 'en')")
	ErrInvalidLocaleCodes   = errors.New("invalid locale code(s)")

	// Glossary validation errors.
	ErrGlossaryEntryEmpty     = errors.New("glossary entry is empty")
	ErrGlossaryTermEmpty      = errors.New("glossary entry has empty term")
	ErrGlossaryDuplicateTerms = errors.New("glossary has duplicate terms")

	// Style guide validation errors.
	ErrInvalidFormality = errors.New("invalid formality")

	// Files validation errors.
	ErrInvalidIndent       = errors.New("indent must be a positive number")
	ErrFilesDirRequired    = errors.New("files directory is required")
	ErrInsufficientLocales = errors.New("at least two locales are required for translation")
	ErrLocaleFileNotFound  = errors.New("locale file does not exist")

	// Provider validation errors.
	ErrNoProvidersConfigured    = errors.New("at least one provider must be configured")
	ErrActiveProviderRequired   = errors.New("active provider must be specified")
	ErrDuplicateProviderName    = errors.New("duplicate provider name")
	ErrProviderNameEmpty        = errors.New("provider has empty name")
	ErrProviderModelEmpty       = errors.New("provider has empty model")
	ErrProviderMaxTokensInvalid = errors.New("provider max tokens must be specified")
	ErrProviderTempInvalid      = errors.New("provider must specify temperature")
	ErrProviderInvalid          = errors.New("invalid provider")
	ErrAzureInvalidFormat       = errors.New(
		"azure provider requires region: use format 'azure-<region>' " +
			"(e.g., 'azure-global', 'azure-swedencentral', 'azure-westeurope')")
)

// FormatValidationError formats configuration validation errors with bullet points and blank lines.
func FormatValidationError(err error) error {
	if err == nil {
		return nil
	}

	// Try to unwrap multiple errors (from errors.Join)
	type unwrapper interface {
		Unwrap() []error
	}

	var individualErrors []error
	if u, ok := err.(unwrapper); ok {
		individualErrors = u.Unwrap()
	} else {
		// Single error, wrap it in a slice
		individualErrors = []error{err}
	}

	// Format each error with a bullet point
	var formattedErrors []string
	for _, e := range individualErrors {
		if e != nil {
			// Indent continuation lines but keep the bullet on the first line
			lines := strings.Split(e.Error(), "\n")
			if len(lines) > 0 {
				formatted := "  • " + lines[0]
				for i := 1; i < len(lines); i++ {
					formatted += "\n    " + lines[i]
				}
				formattedErrors = append(formattedErrors, formatted)
			}
		}
	}

	if len(formattedErrors) == 0 {
		return err
	}

	formattedMessage := strings.Join(formattedErrors, "\n\n")
	return fmt.Errorf("%w:\n%s", ErrInvalidConfig, formattedMessage)
}
