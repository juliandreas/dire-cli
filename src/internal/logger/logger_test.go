package logger_test

import (
	"errors"
	"testing"

	"github.com/juliandreas/dire/internal/logger"
)

// Static errors.
var (
	errTestMessage = errors.New("test error message")
	errTest        = errors.New("test error")
	errAuth        = errors.New("authentication failed with API key sk-123")
)

func TestPublicLoggingFunctions(t *testing.T) {
	// Test that all public logging functions can be called without errors
	// This tests the public API contract without testing internal implementation

	t.Run("Debug function", func(t *testing.T) {
		// Should not panic or error
		logger.Debug("test debug message", "key", "value", "apiKey", "secret123")
	})

	t.Run("Info function", func(t *testing.T) {
		// Should not panic or error
		logger.Info("test info message", "count", 42, "password", "secretpass")
	})

	t.Run("Warn function", func(t *testing.T) {
		// Should not panic or error
		logger.Warn("test warning message", "token", "abc123", "enabled", true)
	})

	t.Run("Error function", func(t *testing.T) {
		// Should not panic or error
		logger.Error("test error", "err", errTestMessage, "path", "/very/long/sensitive/path/demo/.dire.toml")
	})

	t.Run("Fatal function", func(t *testing.T) {
		// Note: We don't actually call Fatal in tests as it would exit the process
		// This is just to document that the function exists and can be called
		t.Skip("Fatal function would exit the process, skipping in tests")
	})
}

func TestInitializeFunction(t *testing.T) {
	// Test that Initialize can be called without errors
	t.Run("Initialize with debug true", func(t *testing.T) {
		logger.Initialize(false, true)
	})

	t.Run("Initialize with debug false", func(t *testing.T) {
		logger.Initialize(false, false)
	})

	t.Run("Initialize with verbose true", func(t *testing.T) {
		logger.Initialize(true, false)
	})

	t.Run("Initialize with both verbose and debug", func(t *testing.T) {
		logger.Initialize(true, true)
	})
}

func TestLoggingWithVariousDataTypes(t *testing.T) {
	// Test that logging functions handle various data types correctly
	t.Run("mixed data types", func(t *testing.T) {
		logger.Info("mixed types test",
			"string", "hello",
			"int", 42,
			"bool", true,
			"float", 3.14,
			"error", errTest,
		)
	})

	t.Run("empty keyvals", func(t *testing.T) {
		logger.Info("no keyvals")
	})

	t.Run("odd number of keyvals", func(t *testing.T) {
		logger.Info("odd keyvals", "key1", "value1", "key2")
	})
}

func TestLoggingWithSensitiveData(t *testing.T) {
	// Test that logging functions can handle sensitive data without errors
	// The actual sanitization is internal, but we can verify the functions work
	t.Run("API keys", func(t *testing.T) {
		logger.Info("api test", "apiKey", "sk-1234567890abcdefghijklmnop")
		logger.Info("api test", "api_key", "secret123")
	})

	t.Run("passwords and tokens", func(t *testing.T) {
		logger.Info("auth test", "password", "mypassword", "token", "abc123def456")
	})

	t.Run("file paths", func(t *testing.T) {
		logger.Info("path test", "path", "/very/long/sensitive/path/demo/.dire.toml")
		logger.Info("path test", "configPath", "C:\\Users\\Someone\\Projects\\myapp\\config\\app.toml")
	})

	t.Run("error messages", func(t *testing.T) {
		logger.Error("error test", "err", errAuth)
	})
}
