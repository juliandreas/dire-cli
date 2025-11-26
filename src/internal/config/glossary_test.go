package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
)

// TestGlossaryBasicSorting tests basic alphabetical sorting functionality.
func TestGlossaryBasicSorting(t *testing.T) {
	configContent := `[files]
directory = "test"

[files.locales]
"en.json" = "en-US"

[glossary]
autoSort = true
entries = [
    { "en-US" = "zebra" },
    { "en-US" = "apple" },
    { "en-US" = "banana" },
]`

	configFile := createTempConfigFile(t, configContent)
	cfg, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if !cfg.Glossary.AutoSort {
		t.Error("Expected AutoSort to be true")
	}

	if len(cfg.Glossary.Entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(cfg.Glossary.Entries))
		return
	}

	// Verify alphabetical sorting: apple, banana, zebra
	expectedOrder := []string{"apple", "banana", "zebra"}
	for i, expected := range expectedOrder {
		actual := cfg.Glossary.Entries[i]["en-US"]
		if actual != expected {
			t.Errorf("Entry %d: expected %s, got %s", i, expected, actual)
		}
	}
}

// TestGlossaryLocaleOrderPreservation tests that locale order is preserved.
func TestGlossaryLocaleOrderPreservation(t *testing.T) {
	configContent := `[files]
directory = "test"

[files.locales]
"da.json" = "da-DK"
"en.json" = "en-US"
"sv.json" = "sv-SE"

[glossary]
autoSort = true
entries = [
    { "da-DK" = "sebra", "en-US" = "zebra", "sv-SE" = "zebra" },
    { "da-DK" = "æble", "en-US" = "apple", "sv-SE" = "äpple" },
]`

	configFile := createTempConfigFile(t, configContent)
	cfg, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify sorting by first locale (da-DK): sebra should come before æble (Unicode collation)
	if len(cfg.Glossary.Entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(cfg.Glossary.Entries))
	}

	firstEntry := cfg.Glossary.Entries[0]["da-DK"]
	secondEntry := cfg.Glossary.Entries[1]["da-DK"]

	if firstEntry != "sebra" {
		t.Errorf("Expected first entry to be 'sebra', got '%s'", firstEntry)
	}
	if secondEntry != "æble" {
		t.Errorf("Expected second entry to be 'æble', got '%s'", secondEntry)
	}

	// Verify locale order is preserved in file
	content, err := os.ReadFile(configFile) // #nosec G304 -- configFile is a test temp file
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)
	// Check that the first entry in the file still has da-DK first
	if !strings.Contains(contentStr, `"da-DK" = "sebra", "en-US" = "zebra", "sv-SE" = "zebra"`) {
		t.Error("Locale order not preserved in file output")
	}
}

// TestGlossaryDisabledSorting tests that sorting can be disabled.
func TestGlossaryDisabledSorting(t *testing.T) {
	configContent := `[files]
directory = "test"

[files.locales]
"en.json" = "en-US"

[glossary]
autoSort = false
entries = [
    { "en-US" = "zebra" },
    { "en-US" = "apple" },
]`

	configFile := createTempConfigFile(t, configContent)
	cfg, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Glossary.AutoSort {
		t.Error("Expected AutoSort to be false")
	}

	// Order should be preserved (zebra, apple)
	if len(cfg.Glossary.Entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(cfg.Glossary.Entries))
	}

	firstEntry := cfg.Glossary.Entries[0]["en-US"]
	secondEntry := cfg.Glossary.Entries[1]["en-US"]

	if firstEntry != "zebra" {
		t.Errorf("Expected first entry to be 'zebra', got '%s'", firstEntry)
	}
	if secondEntry != "apple" {
		t.Errorf("Expected second entry to be 'apple', got '%s'", secondEntry)
	}
}

// TestGlossaryEmptyEntries tests empty glossary handling.
func TestGlossaryEmptyEntries(t *testing.T) {
	configContent := `[files]
directory = "test"

[files.locales]
"en.json" = "en-US"

[glossary]
autoSort = true
entries = []`

	configFile := createTempConfigFile(t, configContent)
	cfg, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(cfg.Glossary.Entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(cfg.Glossary.Entries))
	}
}

// TestGlossaryFormatPreservation tests that formatting is preserved.
func TestGlossaryFormatPreservation(t *testing.T) {
	configContent := `# Configuration file
[files]
directory = "test"

[files.locales]
"en.json" = "en-US"

# Glossary section
[glossary]
autoSort = true
entries = [
    { "en-US" = "zebra" },
    { "en-US" = "apple" },
]
# End comment`

	configFile := createTempConfigFile(t, configContent)

	// Load config to trigger sorting
	_, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Read file back and verify comments are preserved
	content, err := os.ReadFile(configFile) // #nosec G304 -- configFile is a test temp file
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)

	// Check comments are preserved
	if !strings.Contains(contentStr, "# Configuration file") {
		t.Error("Top comment not preserved")
	}
	if !strings.Contains(contentStr, "# Glossary section") {
		t.Error("Glossary section comment not preserved")
	}
	if !strings.Contains(contentStr, "# End comment") {
		t.Error("End comment not preserved")
	}

	// Check entries are sorted (apple before zebra)
	appleIndex := strings.Index(contentStr, `"apple"`)
	zebraIndex := strings.Index(contentStr, `"zebra"`)

	if appleIndex == -1 || zebraIndex == -1 {
		t.Fatal("Could not find apple or zebra entries in result")
	}

	if appleIndex >= zebraIndex {
		t.Error("Entries not sorted: apple should come before zebra")
	}
}

// TestGlossarySwedishFirstLocaleOrder tests sorting when Swedish is the first locale.
func TestGlossarySwedishFirstLocaleOrder(t *testing.T) {
	configContent := `[files]
directory = "test"

[files.locales]
"en.json" = "en-US"
"sv.json" = "sv-SE"
"da.json" = "da-DK"

[glossary]
autoSort = true
entries = [
    { "sv-SE" = "varukorg", "en-US" = "cart", "da-DK" = "kurv" },
    { "sv-SE" = "API", "en-US" = "API", "da-DK" = "API" },
    { "sv-SE" = "betala", "en-US" = "checkout", "da-DK" = "betale" },
]`

	configFile := createTempConfigFile(t, configContent)
	cfg, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify sorting by first locale (sv-SE): API < betala < varukorg (Swedish alphabetical order)
	if len(cfg.Glossary.Entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(cfg.Glossary.Entries))
	}

	expectedSwedishOrder := []string{"API", "betala", "varukorg"}
	for i, expected := range expectedSwedishOrder {
		actual := cfg.Glossary.Entries[i]["sv-SE"]
		if actual != expected {
			t.Errorf("Entry %d: expected Swedish '%s', got '%s'", i, expected, actual)
		}
	}

	// Verify locale order is preserved in file (sv-SE first)
	content, err := os.ReadFile(configFile) // #nosec G304 -- configFile is a test temp file
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)

	// Check that the first entry in the file still has sv-SE first
	if !strings.Contains(contentStr, `"sv-SE" = "API", "en-US" = "API", "da-DK" = "API"`) {
		t.Error("Locale order not preserved in file output, expected sv-SE first")
	}
}

// TestGlossaryEmptyLineHandling tests that empty lines between entries are handled correctly.
func TestGlossaryEmptyLineHandling(t *testing.T) {
	configContent := `[files]
directory = "test"

[files.locales]
"en.json" = "en-US"

[glossary]
autoSort = true
entries = [
    { "en-US" = "zebra" },

    { "en-US" = "apple" },
    { "en-US" = "banana" },
]`

	configFile := createTempConfigFile(t, configContent)
	_, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Read file back and check that no stray empty lines exist
	content, err := os.ReadFile(configFile) // #nosec G304 -- configFile is a test temp file
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)

	// Check that entries are properly sorted without stray empty lines
	lines := strings.Split(contentStr, "\n")
	var entriesSection []string
	inEntries := false
	for _, line := range lines {
		if strings.Contains(line, "entries = [") {
			inEntries = true
		}
		if inEntries {
			entriesSection = append(entriesSection, line)
			if strings.Contains(line, "]") {
				break
			}
		}
	}

	entriesText := strings.Join(entriesSection, "\n")

	// Should not have multiple consecutive empty lines within entries
	if strings.Contains(entriesText, "\n\n\n") {
		t.Error("Found multiple consecutive empty lines in entries section")
	}
}

// TestGlossaryLineEndingPreservation tests that line endings are preserved.
func TestGlossaryLineEndingPreservation(t *testing.T) {
	// Create content with CRLF line endings
	configContent := "[files]\r\n" +
		"directory = \"test\"\r\n\r\n" +
		"[files.locales]\r\n" +
		"\"en.json\" = \"en-US\"\r\n\r\n" +
		"[glossary]\r\n" +
		"autoSort = true\r\n" +
		"entries = [\r\n" +
		"    { \"en-US\" = \"zebra\" },\r\n" +
		"    { \"en-US\" = \"apple\" },\r\n" +
		"]\r\n"

	configFile := createTempConfigFile(t, configContent)

	// Load config to trigger sorting
	_, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Read file back as bytes to check line endings
	content, err := os.ReadFile(configFile) // #nosec G304 -- configFile is a test temp file
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Check that CRLF line endings are preserved
	if !strings.Contains(string(content), "\r\n") {
		t.Error("CRLF line endings not preserved")
	}

	// Ensure entries are sorted
	contentStr := string(content)
	appleIndex := strings.Index(contentStr, `"apple"`)
	zebraIndex := strings.Index(contentStr, `"zebra"`)

	if appleIndex >= zebraIndex {
		t.Error("Entries not sorted correctly")
	}
}

// TestGlossaryInlineComments tests that inline comments are preserved.
func TestGlossaryInlineComments(t *testing.T) {
	configContent := `[files]
directory = "test"

[files.locales]
"en.json" = "en-US"

[glossary]
autoSort = true
entries = [
    { "en-US" = "zebra" }, # Animal
    # Comment before { "en-US" = "apple" }, # Fruit
    { "en-US" = "apple" }, # Real fruit entry
    { "en-US" = "banana" },
]`

	configFile := createTempConfigFile(t, configContent)
	_, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Read file back and verify comments are preserved
	content, err := os.ReadFile(configFile) // #nosec G304 -- configFile is a test temp file
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)

	// Check inline comments are preserved
	if !strings.Contains(contentStr, "# Animal") {
		t.Error("Inline comment '# Animal' not preserved")
	}
	if !strings.Contains(contentStr, "# Fruit") {
		t.Error("Inline comment '# Fruit' not preserved")
	}
	if !strings.Contains(contentStr, "# Comment before") {
		t.Error("Comment before entry not preserved")
	}

	// Verify sorting worked (apple should be first)
	verifyEntrySorting(t, contentStr)
}

// TestGlossaryCommentedOutEntries tests that commented-out entries preserve indentation.
func TestGlossaryCommentedOutEntries(t *testing.T) {
	configContent := `[files]
directory = "test"

[files.locales]
"en.json" = "en-US"

[glossary]
autoSort = true
entries = [
    { "en-US" = "zebra" },
#  { "en-US" = "disabled" },
    { "en-US" = "apple" },
]`

	configFile := createTempConfigFile(t, configContent)
	_, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Read file back and verify commented entry formatting is preserved
	content, err := os.ReadFile(configFile) // #nosec G304 -- configFile is a test temp file
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)

	// Check that commented-out entry preserves indentation
	if !strings.Contains(contentStr, "#  { \"en-US\" = \"disabled\" },") {
		t.Error("Commented-out entry indentation not preserved")
	}

	// Verify active entries are still sorted
	appleIndex := strings.Index(contentStr, `"apple"`)
	zebraIndex := strings.Index(contentStr, `"zebra"`)
	if appleIndex >= zebraIndex {
		t.Error("Active entries not sorted correctly with commented entries present")
	}
}

// createTempConfigFile creates a temporary config file for testing.
func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, ".dire.toml")

	err := os.WriteFile(configFile, []byte(content), constants.SecureFilePermissions)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	return configFile
}

// filterActiveEntryLines extracts only the active entry lines from content, skipping comments and empty lines.
func filterActiveEntryLines(content string) []string {
	lines := strings.Split(content, "\n")
	var activeEntryLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip empty lines and comment lines
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Include lines that contain inline tables (actual entries)
		if strings.Contains(trimmed, "{") && strings.Contains(trimmed, "}") {
			activeEntryLines = append(activeEntryLines, line)
		}
	}
	return activeEntryLines
}

// findEntryPositions finds the positions of apple and zebra entries in the active entry lines.
func findEntryPositions(activeEntryLines []string) (appleIndex, zebraIndex int) {
	appleIndex = -1
	zebraIndex = -1
	for i, line := range activeEntryLines {
		if strings.Contains(line, `"apple"`) {
			appleIndex = i
		}
		if strings.Contains(line, `"zebra"`) {
			zebraIndex = i
		}
	}
	return appleIndex, zebraIndex
}

// verifyEntrySorting verifies that apple appears before zebra in the sorted entries.
func verifyEntrySorting(t *testing.T, contentStr string) {
	t.Helper()
	activeEntryLines := filterActiveEntryLines(contentStr)
	appleLineIndex, zebraLineIndex := findEntryPositions(activeEntryLines)

	if appleLineIndex == -1 || zebraLineIndex == -1 {
		t.Error("Could not find apple or zebra entries in active lines")
	} else if appleLineIndex >= zebraLineIndex {
		t.Errorf("Entries not sorted correctly: apple at line %d, zebra at line %d", appleLineIndex, zebraLineIndex)
	}
}

// TestGlossaryIncompleteFirstEntryLocaleOrder tests that locale ordering works correctly
// even when the first entry has incomplete locale coverage.
func TestGlossaryIncompleteFirstEntryLocaleOrder(t *testing.T) {
	configContent := `[files]
directory = "test"

[files.locales]
"da.json" = "da-DK"
"en.json" = "en-US"
"sv.json" = "sv-SE"

[glossary]
autoSort = true
entries = [
    { "en-US" = "API" },
    { "en-US" = "zebra", "sv-SE" = "zebra", "da-DK" = "sebra" },
    { "en-US" = "apple", "sv-SE" = "äpple", "da-DK" = "æble" },
]`

	configFile := createTempConfigFile(t, configContent)
	cfg, err := config.Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify that sorting worked correctly
	if len(cfg.Glossary.Entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(cfg.Glossary.Entries))
	}

	// Read file back to check locale ordering
	content, err := os.ReadFile(configFile) // #nosec G304 -- configFile is a test temp file
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)

	// Verify that the multi-locale entries have consistent locale order
	// Even though the first entry only has "en-US", the other entries should
	// use the locale order from the entry with the most locales
	if !strings.Contains(contentStr, `{ "en-US" = "apple", "sv-SE" = "äpple", "da-DK" = "æble" }`) {
		t.Error("Apple entry should have consistent locale ordering: en-US, sv-SE, da-DK")
	}
	if !strings.Contains(contentStr, `{ "en-US" = "zebra", "sv-SE" = "zebra", "da-DK" = "sebra" }`) {
		t.Error("Zebra entry should have consistent locale ordering: en-US, sv-SE, da-DK")
	}

	// Verify that the incomplete first entry is preserved as-is
	if !strings.Contains(contentStr, `{ "en-US" = "API" }`) {
		t.Error("Single-locale entry should be preserved")
	}

	// Verify entries are sorted alphabetically
	apiIndex := strings.Index(contentStr, `"API"`)
	appleIndex := strings.Index(contentStr, `"apple"`)
	zebraIndex := strings.Index(contentStr, `"zebra"`)

	if apiIndex == -1 || appleIndex == -1 || zebraIndex == -1 {
		t.Fatal("Could not find all expected entries in the config")
	}

	if apiIndex >= appleIndex || appleIndex >= zebraIndex {
		t.Error("Entries are not sorted alphabetically: API should come before apple, apple should come before zebra")
	}
}
