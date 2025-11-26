// Package logger provides application-wide logging configuration and functions.
package logger

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/log"
)

const (
	maxSafePathComponents = 2 // max path components to keep when sanitizing file paths
)

// Initialize sets up the logger based on verbose and debug flags.
// Log levels:
// - Default (neither flag): FatalLevel (silent, only fmt.Printf output visible)
// - Verbose only: InfoLevel (shows Info, Warn, Error)
// - Debug: DebugLevel (shows everything, implies verbose).
func Initialize(verbose, debug bool) {
	log.SetOutput(os.Stderr)

	switch {
	case debug:
		log.SetLevel(log.DebugLevel) // Show everything
	case verbose:
		log.SetLevel(log.InfoLevel) // Show Info, Warn, Error
	default:
		log.SetLevel(log.FatalLevel) // Effectively silent for normal operation
	}
}

// Debug logs a debug message with optional key-value pairs.
func Debug(msg string, keyvals ...any) {
	sanitized := sanitizeKeyvals(keyvals...)
	log.Debug(msg, sanitized...)
}

// Info logs an info message with optional key-value pairs.
func Info(msg string, keyvals ...any) {
	sanitized := sanitizeKeyvals(keyvals...)
	log.Info(msg, sanitized...)
}

// Warn logs a warning message with optional key-value pairs.
func Warn(msg string, keyvals ...any) {
	sanitized := sanitizeKeyvals(keyvals...)
	log.Warn(msg, sanitized...)
}

// Error logs an error message with optional key-value pairs.
func Error(msg string, keyvals ...any) {
	sanitized := sanitizeKeyvals(keyvals...)
	log.Error(msg, sanitized...)
}

// Fatal logs a fatal message with optional key-value pairs and exits.
func Fatal(msg string, keyvals ...any) {
	sanitized := sanitizeKeyvals(keyvals...)
	log.Fatal(msg, sanitized...)
}

// sanitizeKeyvals removes or masks sensitive information from log key-value pairs.
func sanitizeKeyvals(keyvals ...any) []any {
	if len(keyvals) == 0 {
		return keyvals
	}

	// Create a copy to avoid modifying the original
	sanitized := make([]any, len(keyvals))
	copy(sanitized, keyvals)

	// Process pairs (key, value, key, value, ...)
	for idx := 0; idx < len(sanitized)-1; idx += 2 {
		key, ok := sanitized[idx].(string)
		if !ok {
			continue
		}

		sanitized[idx+1] = sanitizeValue(key, sanitized[idx+1])
	}

	return sanitized
}

// sanitizeValue applies appropriate sanitization based on the key type.
func sanitizeValue(key string, value any) any {
	keyLower := strings.ToLower(key)

	if isSecretKey(keyLower) {
		return "[REDACTED]"
	}

	if isPathKey(keyLower) {
		return sanitizePathValue(value)
	}

	if isErrorKey(keyLower) {
		return sanitizeErrorValue(value)
	}

	return value
}

// isSecretKey checks if the key represents a secret/sensitive value.
func isSecretKey(keyLower string) bool {
	secretKeys := []string{
		"apikey", "api_key", "key", "token", "password", "secret",
		"project", "projectname", "domain",
	}
	return slices.Contains(secretKeys, keyLower)
}

// isPathKey checks if the key represents a file path.
func isPathKey(keyLower string) bool {
	pathKeys := []string{"path", "filepath", "configpath"}
	return slices.Contains(pathKeys, keyLower)
}

// isErrorKey checks if the key represents an error.
func isErrorKey(keyLower string) bool {
	return strings.Contains(keyLower, "error") || strings.Contains(keyLower, "err")
}

// sanitizePathValue sanitizes path values.
func sanitizePathValue(value any) any {
	if path, ok := value.(string); ok {
		return sanitizePath(path)
	}
	return value
}

// sanitizeErrorValue sanitizes error values.
func sanitizeErrorValue(value any) any {
	switch v := value.(type) {
	case error:
		return sanitizeError(v.Error())
	case string:
		return sanitizeError(v)
	default:
		return value
	}
}

// sanitizePath removes potentially sensitive directory information from file paths.
func sanitizePath(path string) string {
	// Convert to forward slashes for consistency
	path = filepath.ToSlash(path)

	// Get just the last few components to avoid exposing full directory structure
	parts := strings.Split(path, "/")
	if len(parts) <= maxSafePathComponents {
		return path // Short path, probably safe
	}

	// Keep last 2 components (e.g., "demo/.dire.toml" instead of "/full/path/to/demo/.dire.toml")
	return strings.Join(parts[len(parts)-maxSafePathComponents:], "/")
}

// sanitizeError removes potentially sensitive information from error messages.
func sanitizeError(errMsg string) string {
	// Only redact errors that contain actual credential values, not just the words
	// API error messages are safe to show - they describe validation errors, not leak secrets
	sensitivePatterns := []string{
		"password",   // Actual password values
		"bearer sk-", // Actual bearer tokens (common prefix)
		"api_key=",   // API key in URL parameter
	}

	lowerMsg := strings.ToLower(errMsg)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerMsg, pattern) {
			return "[ERROR MESSAGE REDACTED - contains sensitive information]"
		}
	}

	// Return error message as-is - API error responses contain validation messages, not credentials
	return errMsg
}
