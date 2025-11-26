package errs

import (
	"errors"
	"fmt"
	"strings"
)

// Application-wide error types.
var (
	ErrRetryable              = errors.New("retryable error")
	ErrTranslation            = errors.New("translation failed")
	ErrTranslationIncomplete  = errors.New("translation incomplete after maximum retries")
	ErrOversized              = errors.New("translation too large")
	ErrBatching               = errors.New("unable to create batches")
	ErrActiveProviderNotFound = errors.New("active provider not found in providers list")
	ErrProviderError          = errors.New("provider error")
	ErrInvalidKeys            = errors.New("invalid keys")
	ErrLocaleNotFound         = errors.New("locale not found in project configuration")
	ErrInvalidType            = errors.New("invalid type")
	ErrUnsupportedProvider    = errors.New("unsupported provider")
	ErrMissingClient          = errors.New("client not configured")

	// API errors.
	ErrEmptyChoices    = errors.New("empty choices array in API response")
	ErrEmptyContent    = errors.New("empty content in API response")
	ErrAPIRequest      = errors.New("API request failed")
	ErrContextCanceled = errors.New("context canceled before request")
	ErrMissingAPIKey   = errors.New("API key not found")
)

// NewRetryableError creates an error for temporary failures that should be retried.
// The retryAfterHeader should contain the raw value from the Retry-After HTTP header.
func NewRetryableError(err error, retryAfterHeader string) error {
	if retryAfterHeader != "" {
		return fmt.Errorf("%w (retry-after: %s): %w", ErrRetryable, retryAfterHeader, err)
	}
	return fmt.Errorf("%w: %w", ErrRetryable, err)
}

// NewTranslationError creates an error when the AI provider fails to follow instructions.
// If cause is provided, it will be wrapped to preserve the error chain.
func NewTranslationError(provider, reason string, cause error) error {
	reason = strings.TrimSpace(reason)

	if cause == nil {
		if reason == "" {
			return fmt.Errorf("%w: %s unable to complete translation - try again or use a different provider",
				ErrTranslation, provider)
		}
		return fmt.Errorf("%w: %s failed: %s", ErrTranslation, provider, reason)
	}

	if reason == "" {
		return fmt.Errorf("%w: provider %s unable to complete translation: %w",
			ErrTranslation, provider, cause)
	}
	return fmt.Errorf("%w: provider %s failed: %s: %w", ErrTranslation, provider, reason, cause)
}

// NewOversizedError creates an error when a translation batch exceeds the provider's token limits.
func NewOversizedError(path string, estimated, limit int) error {
	return fmt.Errorf("%w: path '%s' requires ~%d tokens but only %d available",
		ErrOversized, path, estimated, limit)
}

// NewAPIError creates an error for API request failures with provider context.
func NewAPIError(provider string, err error) error {
	return fmt.Errorf("%w: %s: %v", ErrAPIRequest, provider, err)
}

// NewMissingAPIKeyError creates an error when a required API key is not found in environment variables.
func NewMissingAPIKeyError(envKey, provider string) error {
	return fmt.Errorf("%w: %s API key not found in environment variable %s",
		ErrMissingAPIKey, provider, envKey)
}

// NewTranslationIncompleteError creates an error when translation fails to complete after retries.
func NewTranslationIncompleteError(retries, remainingKeys int) error {
	return fmt.Errorf("%w: %d keys remain after %d retries", ErrTranslationIncomplete, remainingKeys, retries)
}

// NewInvalidKeysError creates an error for invalid key paths.
func NewInvalidKeysError(invalidKeys []string) error {
	if len(invalidKeys) == 1 {
		return fmt.Errorf("%w: key '%s' not found", ErrInvalidKeys, invalidKeys[0])
	}
	return fmt.Errorf("%w: keys not found: %s", ErrInvalidKeys, fmt.Sprintf("'%s'", strings.Join(invalidKeys, "', '")))
}

// NewLocaleNotFoundError creates an error when a locale is not found in the project configuration.
func NewLocaleNotFoundError(locale string, availableLocales []string) error {
	return fmt.Errorf("%w: %q. Available locales: %s", ErrLocaleNotFound, locale, strings.Join(availableLocales, ", "))
}

// NewUnsupportedProviderError returns a standardized unsupported-provider error.
func NewUnsupportedProviderError(provider string, supported []string) error {
	return fmt.Errorf("%w: %q. Valid options: %s", ErrUnsupportedProvider, provider, strings.Join(supported, ", "))
}

// NewMissingClientError creates an error when a required client is not configured.
func NewMissingClientError(clientType, provider string) error {
	return fmt.Errorf("%w: %s provider %q is active but %s client is not configured",
		ErrMissingClient, clientType, provider, clientType)
}
