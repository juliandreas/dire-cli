package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/juliandreas/dire/internal/config"
)

type localeTestCase struct {
	name     string
	locales  map[string]string
	localeID string
	expected string
}

type applyFlagOverridesTestCase struct {
	name              string
	initialDirectory  string
	overrideDirectory string
	expectedDirectory string
}

func TestConfig_GetFileNameForLocale_ExactMatch(t *testing.T) {
	tests := []localeTestCase{
		{
			name: "exact filename match",
			locales: map[string]string{
				"en.json": "en-US",
				"es.json": "es-ES",
			},
			localeID: "en-US",
			expected: "en.json",
		},
		{
			name: "fallback when locale not found",
			locales: map[string]string{
				"en.json": "en-US",
			},
			localeID: "fr-FR",
			expected: "fr-FR.json", // Fallback
		},
	}

	runGetFileNameForLocaleTests(t, tests)
}

func TestConfig_GetFileNameForLocale_GlobIgnoring(t *testing.T) {
	tests := []localeTestCase{
		{
			name: "ignores glob patterns",
			locales: map[string]string{
				"messages.*.json": "en-US", // Should be ignored
				"en.json":         "en-US", // Should be used
			},
			localeID: "en-US",
			expected: "en.json",
		},
		{
			name: "ignores question mark patterns",
			locales: map[string]string{
				"locale-?.json": "fr-FR", // Should be ignored
				"fr.json":       "fr-FR", // Should be used
			},
			localeID: "fr-FR",
			expected: "fr.json",
		},
		{
			name: "ignores bracket patterns",
			locales: map[string]string{
				"lang-[abc].json": "de-DE", // Should be ignored
				"de.json":         "de-DE", // Should be used
			},
			localeID: "de-DE",
			expected: "de.json",
		},
		{
			name: "fallback when no exact match",
			locales: map[string]string{
				"messages.*.json": "en-US", // Should be ignored
			},
			localeID: "en-US",
			expected: "en-US.json", // Fallback
		},
	}

	runGetFileNameForLocaleTests(t, tests)
}

func runGetFileNameForLocaleTests(t *testing.T, tests []localeTestCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//nolint:exhaustruct // Only need Files field for this test
			cfg := &config.Config{}
			cfg.Files.Locales = tt.locales

			result := cfg.GetFileNameForLocale(tt.localeID)
			if result != tt.expected {
				t.Errorf("GetFileNameForLocale() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestContainsGlobChars(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"normal json file", "en.json", false},
		{"normal file with underscore", "messages.json", false},
		{"file with underscore", "locale_en.json", false},
		{"asterisk glob pattern", "messages.*.json", true},
		{"question mark glob pattern", "locale?.json", true},
		{"bracket glob pattern", "lang[abc].json", true},
		{"file with slash", "path/to/file.json", false},
		{"asterisk in filename", "file*name.json", true},
		{"question mark in filename", "file?name.json", true},
		{"brackets in filename", "file[name].json", true},
	}

	// Test glob patterns are ignored - only provide glob pattern
	for _, tt := range tests {
		if tt.expected { // Only test glob patterns
			t.Run(tt.name+" (glob ignored)", func(t *testing.T) {
				//nolint:exhaustruct // Only need Files field for this test
				cfg := &config.Config{}
				cfg.Files.Locales = map[string]string{
					tt.filename: "test-locale", // Only glob pattern maps to this locale
				}

				result := cfg.GetFileNameForLocale("test-locale")
				// Should fallback since glob pattern is ignored
				if result != "test-locale.json" {
					t.Errorf("Expected glob pattern %q to be ignored and fallback to %q, but got %q",
						tt.filename, "test-locale.json", result)
				}
			})
		} else {
			// Test non-glob patterns are used
			t.Run(tt.name+" (non-glob used)", func(t *testing.T) {
				//nolint:exhaustruct // Only need Files field for this test
				cfg := &config.Config{}
				cfg.Files.Locales = map[string]string{
					tt.filename: "test-locale", // Only this non-glob pattern maps to this locale
				}

				result := cfg.GetFileNameForLocale("test-locale")
				// Should return the exact filename
				if result != tt.filename {
					t.Errorf("Expected non-glob filename %q to be returned, but got %q", tt.filename, result)
				}
			})
		}
	}
}

func getBasicApplyFlagOverridesTestCases() []applyFlagOverridesTestCase {
	return []applyFlagOverridesTestCase{
		{
			name:              "empty override leaves directory unchanged",
			initialDirectory:  "demo",
			overrideDirectory: "",
			expectedDirectory: "demo",
		},
		{
			name:              "whitespace-only override leaves directory unchanged",
			initialDirectory:  "demo",
			overrideDirectory: "   ",
			expectedDirectory: "demo",
		},
		{
			name:              "simple directory override",
			initialDirectory:  "demo",
			overrideDirectory: "./i18n",
			expectedDirectory: "i18n",
		},
		{
			name:              "directory with extra slashes gets cleaned",
			initialDirectory:  "demo",
			overrideDirectory: "./locales//subdir/",
			expectedDirectory: filepath.Clean("./locales//subdir/"),
		},
		{
			name:              "directory with parent references gets cleaned",
			initialDirectory:  "demo",
			overrideDirectory: "./locales/../translations",
			expectedDirectory: filepath.Clean("./locales/../translations"),
		},
		{
			name:              "absolute path gets cleaned",
			initialDirectory:  "demo",
			overrideDirectory: "/usr//local//share/locales/",
			expectedDirectory: filepath.Clean("/usr//local//share/locales/"),
		},
	}
}

func getEdgeCaseApplyFlagOverridesTestCases() []applyFlagOverridesTestCase {
	return []applyFlagOverridesTestCase{
		{
			name:              "directory with whitespace gets trimmed and cleaned",
			initialDirectory:  "demo",
			overrideDirectory: "  ./custom-locales  ",
			expectedDirectory: "custom-locales",
		},
		{
			name:              "dot override is accepted as current dir",
			initialDirectory:  "demo",
			overrideDirectory: ".",
			expectedDirectory: ".",
		},
		{
			name:              "tabs/newlines trimmed then cleaned",
			initialDirectory:  "demo",
			overrideDirectory: "\t ./i18n \n",
			expectedDirectory: "i18n",
		},
		{
			name:              "empty initial directory overridden",
			initialDirectory:  "",
			overrideDirectory: "./locales",
			expectedDirectory: "locales",
		},
	}
}

func getPathExpansionTestCases(t *testing.T) []applyFlagOverridesTestCase {
	t.Helper()
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		t.Skipf("skipping path expansion tests: cannot resolve home dir: %v", err)
	}

	return buildPathExpansionTestCases(homeDir)
}

func TestConfig_ApplyFlagOverrides(t *testing.T) {
	t.Parallel()

	basicTests := getBasicApplyFlagOverridesTestCases()
	edgeCaseTests := getEdgeCaseApplyFlagOverridesTestCases()
	basicTests = append(basicTests, edgeCaseTests...)

	// Run basic tests in parallel
	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			//nolint:exhaustruct // Only need Files field for this test
			cfg := &config.Config{}
			cfg.Files.Directory = tt.initialDirectory

			cfg.ApplyDirectoryOverride(tt.overrideDirectory)

			if cfg.Files.Directory != tt.expectedDirectory {
				t.Errorf("ApplyFlagOverrides() = %q, want %q", cfg.Files.Directory, tt.expectedDirectory)
			}
		})
	}
}

func TestConfig_ApplyFlagOverrides_PathExpansion(t *testing.T) {
	pathTests := getPathExpansionTestCases(t)

	for _, tt := range pathTests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables for tests that need them
			if tt.name == "environment variable expansion" || tt.name == "combined home and env var expansion" {
				t.Setenv("TEST_DIR", "/tmp/test")
				t.Setenv("PROJECT_NAME", "myapp")
			}

			//nolint:exhaustruct // Only need Files field for this test
			cfg := &config.Config{}
			cfg.Files.Directory = tt.initialDirectory

			cfg.ApplyDirectoryOverride(tt.overrideDirectory)

			if cfg.Files.Directory != tt.expectedDirectory {
				t.Errorf("ApplyFlagOverrides() = %q, want %q", cfg.Files.Directory, tt.expectedDirectory)
			}
		})
	}
}

//nolint:funlen // Test data structure is inherently long
func getLocaleOverrideTestCases() []struct {
	name            string
	initialLocales  map[string]string
	overrideLocales map[string]string
	expectedLocales map[string]string
} {
	return []struct {
		name            string
		initialLocales  map[string]string
		overrideLocales map[string]string
		expectedLocales map[string]string
	}{
		{
			name: "no locale override preserves config",
			initialLocales: map[string]string{
				"en.json": "en-US",
				"sv.json": "sv-SE",
			},
			overrideLocales: nil,
			expectedLocales: map[string]string{
				"en.json": "en-US",
				"sv.json": "sv-SE",
			},
		},
		{
			name: "empty locale override preserves config",
			initialLocales: map[string]string{
				"en.json": "en-US",
				"sv.json": "sv-SE",
			},
			overrideLocales: map[string]string{},
			expectedLocales: map[string]string{
				"en.json": "en-US",
				"sv.json": "sv-SE",
			},
		},
		{
			name: "locale override completely replaces config",
			initialLocales: map[string]string{
				"en.json": "en-US",
				"sv.json": "sv-SE",
			},
			overrideLocales: map[string]string{
				"fr.json": "fr-FR",
				"de.json": "de-DE",
			},
			expectedLocales: map[string]string{
				"fr.json": "fr-FR",
				"de.json": "de-DE",
			},
		},
		{
			name: "single locale override replaces all",
			initialLocales: map[string]string{
				"en.json": "en-US",
				"sv.json": "sv-SE",
				"fr.json": "fr-FR",
			},
			overrideLocales: map[string]string{
				"en.json": "en-GB",
			},
			expectedLocales: map[string]string{
				"en.json": "en-GB",
			},
		},
	}
}

func TestConfig_ApplyFlagOverrides_LocaleOverrides(t *testing.T) {
	t.Parallel()
	tests := getLocaleOverrideTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			//nolint:exhaustruct // Only need Files field for this test
			cfg := &config.Config{}
			cfg.Files.Locales = tt.initialLocales

			cfg.ApplyLocaleOverrides(tt.overrideLocales)

			// Compare the maps
			if len(cfg.Files.Locales) != len(tt.expectedLocales) {
				t.Errorf("Expected %d locales, got %d", len(tt.expectedLocales), len(cfg.Files.Locales))
				return
			}

			for key, expectedValue := range tt.expectedLocales {
				if actualValue, exists := cfg.Files.Locales[key]; !exists {
					t.Errorf("Expected locale key %q not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("For locale key %q: expected %q, got %q", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func buildPathExpansionTestCases(homeDir string) []applyFlagOverridesTestCase {
	return []applyFlagOverridesTestCase{
		{
			name:              "home directory expansion (~)",
			initialDirectory:  "config-dir",
			overrideDirectory: "~/locales",
			expectedDirectory: filepath.Join(homeDir, "locales"),
		},
		{
			name:              "home directory with subdirs (~/path/to/locales)",
			initialDirectory:  "config-dir",
			overrideDirectory: "~/path/to/locales",
			expectedDirectory: filepath.Join(homeDir, "path", "to", "locales"),
		},
		{
			name:              "environment variable expansion",
			initialDirectory:  "config-dir",
			overrideDirectory: "$TEST_DIR/locales",
			expectedDirectory: filepath.Clean("/tmp/test/locales"),
		},
		{
			name:              "combined home and env var expansion",
			initialDirectory:  "config-dir",
			overrideDirectory: "~/projects/$PROJECT_NAME/i18n",
			expectedDirectory: filepath.Join(homeDir, "projects", "myapp", "i18n"),
		},
		{
			name:              "home expansion with whitespace trimming",
			initialDirectory:  "config-dir",
			overrideDirectory: "  ~/locales  ",
			expectedDirectory: filepath.Join(homeDir, "locales"),
		},
		{
			name:              "plain tilde expands to home directory",
			initialDirectory:  "config-dir",
			overrideDirectory: "~",
			expectedDirectory: homeDir,
		},
		{
			name:              "tilde with slash expands correctly",
			initialDirectory:  "config-dir",
			overrideDirectory: "~/subdir",
			expectedDirectory: filepath.Join(homeDir, "subdir"),
		},
		{
			name:              "tilde without slash does not expand",
			initialDirectory:  "config-dir",
			overrideDirectory: "~subdir",
			expectedDirectory: "~subdir",
		},
		{
			name:              "tilde with username does not expand",
			initialDirectory:  "config-dir",
			overrideDirectory: "~otheruser/locales",
			expectedDirectory: filepath.Clean("~otheruser/locales"),
		},
	}
}
