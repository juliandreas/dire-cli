package ui_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/juliandreas/dire/internal/cli/ui"
	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/engine"
	"github.com/juliandreas/dire/internal/engine/rephrase"
	"github.com/juliandreas/dire/internal/testutil"
	"github.com/juliandreas/dire/internal/types"
)

// displayCompletionTestCase represents a test case for display completion behavior.
type displayCompletionTestCase struct {
	name               string
	metrics            *engine.Metrics
	isStubMode         bool
	isSourcedMode      bool
	isCheckMode        bool
	expectRemainingMsg bool
}

// displayRephrasesTestCase represents a test case for DisplayRephrases function.
type displayRephrasesTestCase struct {
	name           string
	result         *rephrase.Result
	expectedOutput []string // Strings that should be present in output
	description    string
}

// getDisplayCompletionTestCases returns test cases for display completion behavior.
func getDisplayCompletionTestCases() []displayCompletionTestCase {
	return []displayCompletionTestCase{
		{
			name: "sourced mode hides remaining keys",
			metrics: &engine.Metrics{
				SourcedTranslatedKeys: 5, TranslatedKeys: 0, StubTranslatedKeys: 0, RemainingKeys: 10,
			},
			isStubMode: false, isSourcedMode: true, isCheckMode: false, expectRemainingMsg: false,
		},
		{
			name: "normal mode shows remaining keys",
			metrics: &engine.Metrics{
				SourcedTranslatedKeys: 5, TranslatedKeys: 15, StubTranslatedKeys: 0, RemainingKeys: 10,
			},
			isStubMode: false, isSourcedMode: false, isCheckMode: false, expectRemainingMsg: true,
		},
		{
			name: "sourced mode with no remaining keys",
			metrics: &engine.Metrics{
				SourcedTranslatedKeys: 5, TranslatedKeys: 0, StubTranslatedKeys: 0, RemainingKeys: 0,
			},
			isStubMode: false, isSourcedMode: true, isCheckMode: false, expectRemainingMsg: false,
		},
		{
			name: "stub mode behavior",
			metrics: &engine.Metrics{
				SourcedTranslatedKeys: 0, TranslatedKeys: 0, StubTranslatedKeys: 10, RemainingKeys: 5,
			},
			isStubMode: true, isSourcedMode: false, isCheckMode: false, expectRemainingMsg: true,
		},
		{
			name: "check mode disables translation service",
			metrics: &engine.Metrics{
				SourcedTranslatedKeys: 0, TranslatedKeys: 0, StubTranslatedKeys: 0, RemainingKeys: 3,
			},
			isStubMode: false, isSourcedMode: false, isCheckMode: true, expectRemainingMsg: true,
		},
	}
}

func TestDisplayTranslationCompletion_SourcedMode(t *testing.T) {
	tests := getDisplayCompletionTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureCompletionOutput(t, tt.metrics, tt.isStubMode, tt.isSourcedMode, tt.isCheckMode)

			hasRemainingMsg := strings.Contains(output, "remain untranslated")
			if hasRemainingMsg != tt.expectRemainingMsg {
				t.Errorf("Expected remaining message = %t, got %t", tt.expectRemainingMsg, hasRemainingMsg)
			}

			if !strings.Contains(output, "Done!") && !strings.Contains(output, "All good!") {
				t.Errorf("Expected completion message in output")
			}

			// Assert spacing behavior: leading blank line should appear only when requiresTranslationService is true
			requiresTranslationService := !tt.isStubMode && !tt.isSourcedMode && !tt.isCheckMode
			hasTranslatedKeys := tt.metrics.SourcedTranslatedKeys > 0 ||
				tt.metrics.TranslatedKeys > 0 ||
				tt.metrics.StubTranslatedKeys > 0

			if hasTranslatedKeys && requiresTranslationService {
				// Should have leading blank line before "Done!" message
				if !strings.Contains(output, "\n\n") {
					t.Errorf("Expected leading blank line when requiresTranslationService=true and keys translated")
				}
			}
		})
	}
}

func captureCompletionOutput(
	t *testing.T, metrics *engine.Metrics, isStubMode, isSourcedMode, isCheckMode bool,
) string {
	t.Helper()
	// NOTE: Do not use t.Parallel() - this test captures os.Stdout and would interfere with parallel tests

	oldStdout := os.Stdout
	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = pipeWriter

	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = pipeReader.Close()
	})

	outputChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, pipeReader)
		outputChan <- buf.String()
	}()

	ctx := context.Background()
	cfg := createTestConfig()
	timer := ui.NewTimer(false)
	timer.Start(ctx)

	requiresTranslationService := !isStubMode && !isSourcedMode && !isCheckMode
	ui.DisplayTranslationCompletion(ctx, metrics, timer, cfg, isSourcedMode, requiresTranslationService)

	_ = pipeWriter.Close()
	output := <-outputChan
	timer.Stop()

	return output
}

// createTestConfig creates a minimal config for testing.
func createTestConfig() *config.Config {
	cfg := testutil.MockConfig()
	cfg.Files.Locales = map[string]string{
		"en.json": "en-US",
		"da.json": "da-DK",
		"sv.json": "sv-SE",
	}
	cfg.Providers.Active = "openai"
	return cfg
}

// getDisplayRephrasesTestCases returns test cases for DisplayRephrases function.
func getDisplayRephrasesTestCases() []displayRephrasesTestCase {
	var testCases []displayRephrasesTestCase
	testCases = append(testCases, getBasicRephraseTestCases()...)
	testCases = append(testCases, getComplexRephraseTestCases()...)
	return testCases
}

// getBasicRephraseTestCases returns basic test cases for DisplayRephrases function.
func getBasicRephraseTestCases() []displayRephrasesTestCase {
	return []displayRephrasesTestCase{
		{
			name:   "nil result",
			result: nil,
			expectedOutput: []string{
				"No rephrase options generated",
			},
			description: "Should handle nil result gracefully",
		},
		{
			name: "empty rephrases list",
			result: &rephrase.Result{
				Key:          "common.save",
				Locale:       types.Locale("en-US"),
				CurrentValue: "Save changes",
				Rephrases:    []string{},
			},
			expectedOutput: []string{
				"Rephrase options for \"common.save\" in en-US:",
				"Current: \"Save changes\"",
				"No rephrase options available",
			},
			description: "Should display header and current value, then show no options message",
		},
		{
			name: "single rephrase option",
			result: &rephrase.Result{
				Key:          "auth.login.title",
				Locale:       types.Locale("sv-SE"),
				CurrentValue: "Logga in",
				Rephrases:    []string{"Logga in på ditt konto"},
			},
			expectedOutput: []string{
				"Rephrase options for \"auth.login.title\" in sv-SE:",
				"Current: \"Logga in\"",
				"1. \"Logga in på ditt konto\"",
			},
			description: "Should display single rephrase option with proper numbering",
		},
	}
}

// getComplexRephraseTestCases returns complex test cases for DisplayRephrases function.
func getComplexRephraseTestCases() []displayRephrasesTestCase {
	return []displayRephrasesTestCase{
		{
			name: "multiple rephrase options",
			result: &rephrase.Result{
				Key:          "button.submit",
				Locale:       types.Locale("da-DK"),
				CurrentValue: "Send",
				Rephrases:    []string{"Indsend", "Gem", "Bekræft", "Fortsæt"},
			},
			expectedOutput: []string{
				"Rephrase options for \"button.submit\" in da-DK:",
				"Current: \"Send\"",
				"1. \"Indsend\"",
				"2. \"Gem\"",
				"3. \"Bekræft\"",
				"4. \"Fortsæt\"",
			},
			description: "Should display multiple rephrase options with sequential numbering",
		},
		{
			name: "special characters in key and value",
			result: &rephrase.Result{
				Key:          "error.validation.required",
				Locale:       types.Locale("en-US"),
				CurrentValue: "This field is \"required\"",
				Rephrases:    []string{"Field cannot be empty", "Required field"},
			},
			expectedOutput: []string{
				"Rephrase options for \"error.validation.required\" in en-US:",
				"Current: \"This field is \\\"required\\\"\"",
				"1. \"Field cannot be empty\"",
				"2. \"Required field\"",
			},
			description: "Should properly escape quotes in displayed values",
		},
	}
}

func TestDisplayRephrases(t *testing.T) {
	tests := getDisplayRephrasesTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureRephraseOutput(t, tt.result)

			// Check that all expected output strings are present
			for _, expected := range tt.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain: %q\nActual output:\n%s", expected, output)
				}
			}

			// Verify output structure for valid results
			if tt.result != nil && len(tt.result.Rephrases) > 0 {
				// Check that numbering is sequential
				for i, rephrase := range tt.result.Rephrases {
					expectedNumber := fmt.Sprintf("%d.", i+1)
					if !strings.Contains(output, expectedNumber) {
						t.Errorf("Expected output to contain number: %q", expectedNumber)
					}
					// Check that the rephrase text is present
					if !strings.Contains(output, rephrase) {
						t.Errorf("Expected output to contain rephrase: %q", rephrase)
					}
				}
			}
		})
	}
}

func captureRephraseOutput(t *testing.T, result *rephrase.Result) string {
	t.Helper()
	// NOTE: Do not use t.Parallel() - this test captures os.Stdout and would interfere with parallel tests

	oldStdout := os.Stdout
	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = pipeWriter

	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = pipeReader.Close()
	})

	outputChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, pipeReader)
		outputChan <- buf.String()
	}()

	// Call the function we're testing
	ui.DisplayRephrases(result)

	_ = pipeWriter.Close()
	output := <-outputChan

	return output
}
