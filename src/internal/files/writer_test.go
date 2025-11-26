package files_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/files"
	"github.com/juliandreas/dire/internal/flags"
	"github.com/juliandreas/dire/internal/testutil"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
	"go.uber.org/goleak"
)

var (
	errFilenameContainsPathSeparators = errors.New("filename contains path separators")
	errFilePathOutsideDirectory       = errors.New("file path is outside expected directory")
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// writeTestFile is a helper function to write test files with secure permissions.
func writeTestFile(path string, data []byte) error {
	return os.WriteFile(path, data, constants.SecureFilePermissions)
}

func TestUnicodeCharacterPreservation(t *testing.T) {
	defer goleak.VerifyNone(t)

	testTranslations := getUnicodeTestTranslations()

	for _, tc := range testTranslations {
		t.Run(string(tc.Locale), func(t *testing.T) {
			tempDir, localeFile := setupUnicodeTestFile(t, tc)

			ctx := context.Background()
			err := files.UpdateLocaleFile(ctx, localeFile, []types.LocaleContent{tc}, "  ")
			if err != nil {
				t.Fatalf("UpdateLocaleFile failed: %v", err)
			}

			fileName := strings.ToLower(string(tc.Locale)[:2]) + ".json"
			verifyUnicodeCharacterPreservation(t, tempDir, fileName, tc)
		})
	}
}

func TestUnicodeCharacterPreservationWithCRLF(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Test one representative Unicode case with CRLF line endings
	tc := types.LocaleContent{
		Locale: "da-DK",
		Path:   []string{"security", "title"},
		Value:  "Sikkerhed & Privatliv", // Contains '&'; no æ here
	}

	tempDir, localeFile := setupUnicodeTestFileWithCRLF(t, tc)

	ctx := context.Background()
	err := files.UpdateLocaleFile(ctx, localeFile, []types.LocaleContent{tc}, "  ")
	if err != nil {
		t.Fatalf("UpdateLocaleFile failed: %v", err)
	}

	fileName := strings.ToLower(string(tc.Locale)[:2]) + ".json"
	verifyUnicodeCharacterPreservation(t, tempDir, fileName, tc)

	// Also verify CRLF line endings were preserved
	fileContent, err := readFileInDir(tempDir, fileName)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}
	fileStr := string(fileContent)
	tail := fileStr
	if len(fileStr) > 2 {
		tail = fileStr[len(fileStr)-2:]
	}
	if !strings.HasSuffix(fileStr, utils.LineEndingCRLF) {
		t.Errorf("Expected CRLF line ending to be preserved; got tail %q", tail)
	}
	verifyOnlyCRLF(t, fileStr, tc.Locale)
}

func TestUnicodeCharacterPreservationWithCR(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Test one representative Unicode case with CR-only line endings (classic Mac)
	tc := types.LocaleContent{
		Locale: "da-DK",
		Path:   []string{"security", "title"},
		Value:  "Sikkerhed & Privatliv", // Contains '&'; no æ here
	}

	tempDir, localeFile := setupUnicodeTestFileWithCR(t, tc)

	ctx := context.Background()
	err := files.UpdateLocaleFile(ctx, localeFile, []types.LocaleContent{tc}, "  ")
	if err != nil {
		t.Fatalf("UpdateLocaleFile failed: %v", err)
	}

	fileName := strings.ToLower(string(tc.Locale)[:2]) + ".json"
	verifyUnicodeCharacterPreservation(t, tempDir, fileName, tc)

	// Also verify CR line endings were preserved
	fileContent, err := readFileInDir(tempDir, fileName)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}
	fileStr := string(fileContent)
	tail := fileStr
	if len(fileStr) > 1 {
		tail = fileStr[len(fileStr)-1:]
	}
	if !strings.HasSuffix(fileStr, utils.LineEndingCR) {
		t.Errorf("Expected CR line ending to be preserved; got tail %q", tail)
	}
	// Detect any LF (CR-only files must not contain '\n', including CRLF)
	for pos := range fileContent {
		if fileContent[pos] == utils.LineEndingLF[0] {
			t.Errorf("Found LF at offset %d in CR-only file", pos)
			break
		}
	}
}

func TestBufferedWriterUnicodePreservation(t *testing.T) {
	defer goleak.VerifyNone(t)

	// This test specifically tests the buffering path to ensure it doesn't reintroduce Unicode escaping
	tempDir := t.TempDir()

	// Create a proper config struct
	cfg := testutil.MockConfig()
	cfg.Files.Directory = tempDir
	cfg.Files.Locales = map[string]string{
		"da.json": "da-DK",
	}

	setupInitialFile(t, tempDir)

	// Create buffered writer and send translations
	ctx := context.Background()
	bufferedWriter := files.NewBufferedWriter(ctx, cfg, 10)
	bufferedWriter.Start(ctx)

	// Send Unicode translations through the buffer
	translations := []types.LocaleContent{
		{
			Locale: "da-DK",
			Path:   []string{"security", "title"},
			Value:  "Sikkerhed & Privatliv", // Contains & but no æ
		},
		{
			Locale: "da-DK",
			Path:   []string{"settings", "privacy"},
			Value:  "Privatlivspolitik & Søgning", // Contains ø in Søgning
		},
		{
			Locale: "da-DK",
			Path:   []string{"help", "contact"},
			Value:  "Hjælp & Understøttelse", // Contains æ in Hjælp and ø in Understøttelse
		},
	}

	if !bufferedWriter.Send(translations) {
		t.Fatal("Failed to send translations to buffer")
	}

	// Check for any errors during buffering
	if getErr := bufferedWriter.GetError(); getErr != nil {
		t.Fatalf("Buffered writer encountered error: %v", getErr)
	}

	// Close and flush
	if closeErr := bufferedWriter.Close(); closeErr != nil {
		t.Fatalf("Failed to close buffered writer: %v", closeErr)
	}

	// Verify Unicode preservation
	verifyUnicodePreservation(t, tempDir)
}

func setupInitialFile(t *testing.T, tempDir string) {
	filePath := filepath.Join(tempDir, "da.json")
	initialContent := map[string]any{"existing": "content"}
	initialData, err := utils.MarshalJSON(initialContent, "  ")
	if err != nil {
		t.Fatalf("Failed to marshal initial content: %v", err)
	}

	if writeErr := writeTestFile(filePath, initialData); writeErr != nil {
		t.Fatalf("Failed to write initial file: %v", writeErr)
	}
}

func verifyUnicodePreservation(t *testing.T, tempDir string) {
	fileContent, err := readFileInDir(tempDir, "da.json")
	if err != nil {
		t.Fatalf("Failed to read file after buffered write: %v", err)
	}

	fileStr := string(fileContent)

	// Debug: Print the actual file content
	t.Logf("File content: %q", fileStr)
	t.Logf("File content (raw): %s", fileStr)

	// Check for Unicode escaping
	if strings.Contains(fileStr, "\\u") {
		t.Errorf("Found Unicode escape sequences in buffered write output")
		t.Logf("File content: %s", fileStr)
	}

	// Check that actual Unicode characters are present
	expectedChars := []string{"æ", "ø", "&"}
	for _, char := range expectedChars {
		if !strings.Contains(fileStr, char) {
			t.Errorf("Expected Unicode character '%s' not found in buffered output", char)
			t.Logf("Searching for '%s' in content", char)
		}
	}
}

func TestBufferedWriterNewFileCreationBug(t *testing.T) {
	defer goleak.VerifyNone(t)

	// This test reproduces the exact bug scenario: buffering tries to create a new file
	// for a locale that doesn't exist, which was causing Unicode escaping
	tempDir := t.TempDir()

	// Create proper config struct
	cfg := testutil.MockConfig()
	cfg.Files.Directory = tempDir
	cfg.Files.Locales = map[string]string{
		"en.json": "en-US",
		"da.json": "da-DK", // This file won't exist, triggering the bug scenario
	}

	// Only create en.json, not da.json (simulating the bug condition)
	enFilePath := filepath.Join(tempDir, "en.json")
	initialContent := map[string]any{"existing": "content"}
	initialData, err := utils.MarshalJSON(initialContent, "  ")
	if err != nil {
		t.Fatalf("Failed to marshal initial content: %v", err)
	}

	if writeErr := writeTestFile(enFilePath, initialData); writeErr != nil {
		t.Fatalf("Failed to write initial file: %v", writeErr)
	}

	// Create buffered writer
	ctx := context.Background()
	bufferedWriter := files.NewBufferedWriter(ctx, cfg, 10)
	bufferedWriter.Start(ctx)

	// Send translations for the non-existent locale (this was the bug scenario)
	translations := []types.LocaleContent{
		{
			Locale: "da-DK", // This locale file doesn't exist!
			Path:   []string{"security", "title"},
			Value:  "Sikkerhed & Privatliv", // Unicode content
		},
	}

	// Send translations through buffered writer
	if !bufferedWriter.Send(translations) {
		t.Fatal("Failed to send translations to buffer")
	}

	// Check for any errors during buffering
	if getErr := bufferedWriter.GetError(); getErr != nil {
		t.Fatalf("Buffered writer encountered error: %v", getErr)
	}

	// Close and flush - this should now return an error instead of creating the file
	err = bufferedWriter.Close()
	if err == nil {
		t.Fatal("Expected error when trying to write to non-existent locale file, but got nil")
	}

	// Verify the error message mentions the missing locale
	if !strings.Contains(err.Error(), "da-DK") {
		t.Errorf("Expected error to mention missing locale 'da-DK', got: %v", err)
	}

	// Verify that no da.json file was created
	daFilePath := filepath.Join(tempDir, "da.json")
	if _, err := os.Stat(daFilePath); err == nil {
		t.Error("da.json file was created when it should not have been")
	}
}

func TestCleanupFiles(t *testing.T) {
	defer goleak.VerifyNone(t)

	tempDir := createTempDirForTest(t)
	localeFiles := setupTestFilesForCleanup(t, tempDir)

	// Run cleanup
	ctx := context.Background()
	if err := files.CleanupFiles(ctx, localeFiles, true, "  "); err != nil {
		t.Fatalf("CleanupFiles failed: %v", err)
	}

	// Verify cleanup worked for all files
	for locale, expectedFile := range localeFiles {
		verifyFileCleanedUp(t, tempDir, locale, expectedFile)
	}
}

func TestCleanupStringTrimming(t *testing.T) {
	defer goleak.VerifyNone(t)

	tempDir := createTempDirForTest(t)
	localeFiles := setupTestFilesForCleanup(t, tempDir)

	ctx := context.Background()
	if err := files.CleanupFiles(ctx, localeFiles, true, "  "); err != nil {
		t.Fatalf("CleanupFiles failed: %v", err)
	}

	// Verify string trimming specifically
	for locale, expectedFile := range localeFiles {
		verifyStringsTrimmed(t, tempDir, locale, expectedFile)
	}
}

func TestCleanupPreservesNonStringValues(t *testing.T) {
	defer goleak.VerifyNone(t)

	tempDir := createTempDirForTest(t)
	localeFiles := setupTestFilesForCleanup(t, tempDir)

	ctx := context.Background()
	if err := files.CleanupFiles(ctx, localeFiles, true, "  "); err != nil {
		t.Fatalf("CleanupFiles failed: %v", err)
	}

	// Verify non-string values preserved
	for locale, expectedFile := range localeFiles {
		verifyNonStringValuesPreserved(t, tempDir, locale, expectedFile)
	}
}

func TestCleanupFileFormat(t *testing.T) {
	defer goleak.VerifyNone(t)

	tempDir := createTempDirForTest(t)
	localeFiles := setupTestFilesForCleanup(t, tempDir)

	ctx := context.Background()
	if err := files.CleanupFiles(ctx, localeFiles, true, "  "); err != nil {
		t.Fatalf("CleanupFiles failed: %v", err)
	}

	// Verify file format
	for locale, expectedFile := range localeFiles {
		verifyFileFormat(t, tempDir, locale, expectedFile)
	}
}

func createTempDirForTest(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "dire-cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	t.Cleanup(func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			t.Logf("Failed to remove temp directory: %v", removeErr)
		}
	})
	return tempDir
}

func setupTestFilesForCleanup(t *testing.T, tempDir string) map[types.Locale]*types.LocaleFile {
	// Test data with strings that need trimming
	testContent := map[string]any{
		"auth": map[string]any{
			"login":    " Login ",
			"password": "\tPassword\n",
		},
		"items": []any{
			" item1 ",
			"  item2  ",
			map[string]any{
				"nested": " nested value ",
			},
		},
		"number":  42,
		"boolean": true,
	}

	// Create test files
	enFile := filepath.Join(tempDir, "en.json")
	daFile := filepath.Join(tempDir, "da.json")

	// Write initial files with unsorted, untrimmed content
	initialData, err := utils.MarshalJSON(testContent, "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	if err := writeTestFile(enFile, initialData); err != nil {
		t.Fatalf("Failed to write en.json: %v", err)
	}
	if err := writeTestFile(daFile, initialData); err != nil {
		t.Fatalf("Failed to write da.json: %v", err)
	}

	// Create locale files map
	return map[types.Locale]*types.LocaleFile{
		"en": {
			Path:       enFile,
			Locale:     "en",
			Contents:   testContent,
			LineEnding: utils.LineEndingLF, // Default Unix line ending for tests
		},
		"da": {
			Path:       daFile,
			Locale:     "da",
			Contents:   testContent,
			LineEnding: utils.LineEndingLF, // Default Unix line ending for tests
		},
	}
}

func verifyFileCleanedUp(t *testing.T, tempDir string, locale types.Locale, expectedFile *types.LocaleFile) {
	// Read the updated file
	updatedData, err := readFileInDir(tempDir, filepath.Base(expectedFile.Path))
	if err != nil {
		t.Fatalf("Failed to read updated %s file: %v", locale, err)
	}

	// Parse the updated content
	var updatedContent map[string]any
	if err := json.Unmarshal(updatedData, &updatedContent); err != nil {
		t.Fatalf("Failed to parse updated %s file: %v", locale, err)
	}
}

func verifyStringsTrimmed(t *testing.T, tempDir string, locale types.Locale, expectedFile *types.LocaleFile) {
	updatedData, err := readFileInDir(tempDir, filepath.Base(expectedFile.Path))
	if err != nil {
		t.Fatalf("Failed to read updated %s file: %v", locale, err)
	}

	var updatedContent map[string]any
	if err := json.Unmarshal(updatedData, &updatedContent); err != nil {
		t.Fatalf("Failed to parse updated %s file: %v", locale, err)
	}

	// Verify strings were trimmed
	authMap := updatedContent["auth"].(map[string]any)
	if authMap["login"] != "Login" {
		t.Errorf("Expected 'Login', got '%v'", authMap["login"])
	}
	if authMap["password"] != "Password" {
		t.Errorf("Expected 'Password', got '%v'", authMap["password"])
	}

	// Verify nested array strings were trimmed
	items := updatedContent["items"].([]any)
	if items[0] != "item1" {
		t.Errorf("Expected 'item1', got '%v'", items[0])
	}
	if items[1] != "item2" {
		t.Errorf("Expected 'item2', got '%v'", items[1])
	}

	// Verify nested map in array was trimmed
	nestedMap := items[2].(map[string]any)
	if nestedMap["nested"] != "nested value" {
		t.Errorf("Expected 'nested value', got '%v'", nestedMap["nested"])
	}
}

func verifyNonStringValuesPreserved(t *testing.T, tempDir string, locale types.Locale, expectedFile *types.LocaleFile) {
	updatedData, err := readFileInDir(tempDir, filepath.Base(expectedFile.Path))
	if err != nil {
		t.Fatalf("Failed to read updated %s file: %v", locale, err)
	}

	var updatedContent map[string]any
	if err := json.Unmarshal(updatedData, &updatedContent); err != nil {
		t.Fatalf("Failed to parse updated %s file: %v", locale, err)
	}

	// Verify non-string values were preserved
	if updatedContent["number"] != float64(42) { // JSON unmarshals numbers as float64
		t.Errorf("Expected 42, got '%v'", updatedContent["number"])
	}
	if updatedContent["boolean"] != true {
		t.Errorf("Expected true, got '%v'", updatedContent["boolean"])
	}
}

func verifyFileFormat(t *testing.T, tempDir string, locale types.Locale, expectedFile *types.LocaleFile) {
	updatedData, err := readFileInDir(tempDir, filepath.Base(expectedFile.Path))
	if err != nil {
		t.Fatalf("Failed to read updated %s file: %v", locale, err)
	}

	if len(updatedData) == 0 {
		t.Errorf("File %s should not be empty", locale)
		return
	}
	content := string(updatedData)
	if !strings.HasSuffix(content, expectedFile.LineEnding) {
		t.Errorf("File %s should end with %q", locale, expectedFile.LineEnding)
	}

	verifyLineEndingPurity(t, content, locale, expectedFile.LineEnding)
}

func verifyLineEndingPurity(t *testing.T, content string, locale types.Locale, expectedLineEnding string) {
	switch expectedLineEnding {
	case utils.LineEndingCRLF:
		verifyOnlyCRLF(t, content, locale)
	case utils.LineEndingLF:
		verifyOnlyLF(t, content, locale)
	case utils.LineEndingCR:
		verifyOnlyCR(t, content, locale)
	}
}

func verifyOnlyCRLF(t *testing.T, content string, locale types.Locale) {
	for i := range content {
		if content[i] == utils.LineEndingLF[0] && (i == 0 || content[i-1] != utils.LineEndingCR[0]) {
			t.Errorf("File %s contains bare LF at offset %d", locale, i)
			break
		}
	}
	for pos := 0; pos < len(content); pos++ {
		if content[pos] == utils.LineEndingCR[0] {
			if pos+1 >= len(content) || content[pos+1] != utils.LineEndingLF[0] {
				t.Errorf("File %s contains stray CR at offset %d", locale, pos)
				break
			}
			pos++ // skip matched '\n'
		}
	}
}

func verifyOnlyLF(t *testing.T, content string, locale types.Locale) {
	if strings.Contains(content, utils.LineEndingCRLF) {
		t.Errorf("File %s contains CRLF sequences but expected LF", locale)
	}
	if strings.ContainsRune(content, rune(utils.LineEndingCR[0])) {
		t.Errorf("File %s contains CR bytes but expected LF-only", locale)
	}
}

func verifyOnlyCR(t *testing.T, content string, locale types.Locale) {
	if strings.Contains(content, utils.LineEndingLF) || strings.Contains(content, utils.LineEndingCRLF) {
		t.Errorf("File %s contains LF/CRLF but expected CR-only", locale)
	}
}

// readFileInDir reads a file that must be within the specified directory.
func getUnicodeTestTranslations() []types.LocaleContent {
	return []types.LocaleContent{
		{
			Locale: "da-DK",
			Path:   []string{"security", "title"},
			Value:  "Sikkerhed & Privatliv", // Danish æ character
		},
		{
			Locale: "sv-SE",
			Path:   []string{"settings", "title"},
			Value:  "Inställningar", // Swedish ä character
		},
		{
			Locale: "no-NO",
			Path:   []string{"help", "title"},
			Value:  "Hjelp & Støtte", // Norwegian ø character
		},
		{
			Locale: "de-DE",
			Path:   []string{"common", "title"},
			Value:  "Größe & Qualität", // German ö and ß characters
		},
		{
			Locale: "fr-FR",
			Path:   []string{"auth", "title"},
			Value:  "Authentification & Sécurité", // French é character
		},
	}
}

func setupUnicodeTestFile(t *testing.T, tc types.LocaleContent) (string, *types.LocaleFile) {
	tempDir := t.TempDir()
	fileName := strings.ToLower(string(tc.Locale)[:2]) + ".json"
	filePath := filepath.Join(tempDir, fileName)

	initialContent := map[string]any{"existing": "content"}
	initialData, err := utils.MarshalJSON(initialContent, "  ")
	if err != nil {
		t.Fatalf("Failed to marshal initial content: %v", err)
	}

	if writeErr := writeTestFile(filePath, initialData); writeErr != nil {
		t.Fatalf("Failed to write initial file: %v", writeErr)
	}

	return tempDir, &types.LocaleFile{
		Path:       filePath,
		Locale:     tc.Locale,
		Contents:   initialContent,
		LineEnding: utils.LineEndingLF, // Default Unix line ending for tests
	}
}

func setupUnicodeTestFileWithCRLF(t *testing.T, tc types.LocaleContent) (string, *types.LocaleFile) {
	tempDir := t.TempDir()
	fileName := strings.ToLower(string(tc.Locale)[:2]) + ".json"
	filePath := filepath.Join(tempDir, fileName)

	initialContent := map[string]any{"existing": "content"}
	initialData, err := utils.MarshalJSON(initialContent, "  ")
	if err != nil {
		t.Fatalf("Failed to marshal initial content: %v", err)
	}

	// Convert to CRLF for initial file (normalize first to avoid CRCRLF)
	s := strings.ReplaceAll(string(initialData), utils.LineEndingCRLF, utils.LineEndingLF)
	initialDataCRLF := []byte(strings.ReplaceAll(s, utils.LineEndingLF, utils.LineEndingCRLF))
	if writeErr := writeTestFile(filePath, initialDataCRLF); writeErr != nil {
		t.Fatalf("Failed to write initial file: %v", writeErr)
	}

	return tempDir, &types.LocaleFile{
		Path:       filePath,
		Locale:     tc.Locale,
		Contents:   initialContent,
		LineEnding: utils.LineEndingCRLF, // CRLF line ending for Windows test
	}
}

func setupUnicodeTestFileWithCR(t *testing.T, tc types.LocaleContent) (string, *types.LocaleFile) {
	tempDir := t.TempDir()
	fileName := strings.ToLower(string(tc.Locale)[:2]) + ".json"
	filePath := filepath.Join(tempDir, fileName)

	initialContent := map[string]any{"existing": "content"}
	initialData, err := utils.MarshalJSON(initialContent, "  ")
	if err != nil {
		t.Fatalf("Failed to marshal initial content: %v", err)
	}

	// Convert to CR for initial file (normalize first to avoid mixed endings)
	s := strings.ReplaceAll(string(initialData), utils.LineEndingCRLF, utils.LineEndingLF)
	s = strings.ReplaceAll(s, utils.LineEndingCR, utils.LineEndingLF) // Remove any existing CR
	initialDataCR := []byte(strings.ReplaceAll(s, utils.LineEndingLF, utils.LineEndingCR))
	if writeErr := writeTestFile(filePath, initialDataCR); writeErr != nil {
		t.Fatalf("Failed to write initial file: %v", writeErr)
	}

	return tempDir, &types.LocaleFile{
		Path:       filePath,
		Locale:     tc.Locale,
		Contents:   initialContent,
		LineEnding: utils.LineEndingCR, // CR line ending for classic Mac test
	}
}

func verifyUnicodeCharacterPreservation(t *testing.T, tempDir, fileName string, tc types.LocaleContent) {
	fileContent, err := readFileInDir(tempDir, fileName)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	fileStr := string(fileContent)
	unicodeChars := []string{"æ", "ø", "å", "ä", "ö", "ß", "é", "&"}

	for _, char := range unicodeChars {
		if strings.Contains(tc.Value.(string), char) {
			if !strings.Contains(fileStr, char) {
				t.Errorf("Unicode character '%s' was not preserved in file", char)
			}

			if strings.Contains(fileStr, "\\u") {
				t.Errorf("Found Unicode escape sequence in file content, but Unicode should be preserved literally")
				t.Logf("File content: %s", fileStr)
			}
		}
	}

	if !strings.Contains(fileStr, tc.Value.(string)) {
		t.Errorf("Expected translation '%s' not found in file", tc.Value.(string))
		t.Logf("File content: %s", fileStr)
	}

	var parsedContent map[string]any
	if err := json.Unmarshal(fileContent, &parsedContent); err != nil {
		t.Errorf("File content is not valid JSON: %v", err)
	}
}

func readFileInDir(dir, filename string) ([]byte, error) {
	// Clean and validate the path
	cleanDir := filepath.Clean(dir)
	cleanFile := filepath.Clean(filename)

	// Ensure filename doesn't contain path separators
	if strings.ContainsAny(cleanFile, "/\\") {
		return nil, fmt.Errorf("%w: %s", errFilenameContainsPathSeparators, filename)
	}

	// Construct the full path
	fullPath := filepath.Join(cleanDir, cleanFile)

	// Verify the path is within the expected directory
	rel, err := filepath.Rel(cleanDir, filepath.Clean(fullPath))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errFilePathOutsideDirectory
	}

	return os.ReadFile(fullPath)
}

// TestCleanupPreservesCRLFEndings verifies that CleanupFiles preserves CRLF line endings
// when processing files that originally had Windows-style line endings.
func TestCleanupPreservesCRLFEndings(t *testing.T) {
	defer goleak.VerifyNone(t)
	tempDir := createTempDirForTest(t)

	testContent := map[string]any{"auth": map[string]any{"login": " Login "}}
	dataLF, err := utils.MarshalJSON(testContent, "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Convert to CRLF for initial file (normalize first to avoid CRCRLF)
	s := strings.ReplaceAll(string(dataLF), utils.LineEndingCRLF, utils.LineEndingLF)
	dataCRLF := []byte(strings.ReplaceAll(s, utils.LineEndingLF, utils.LineEndingCRLF))
	enFile := filepath.Join(tempDir, "en.json")
	if writeErr := writeTestFile(enFile, dataCRLF); writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}

	localeFiles := map[types.Locale]*types.LocaleFile{
		"en": {
			Path:       enFile,
			Locale:     "en",
			Contents:   testContent,
			LineEnding: utils.LineEndingCRLF,
		},
	}

	ctx := context.Background()
	if cleanupErr := files.CleanupFiles(ctx, localeFiles, true, "  "); cleanupErr != nil {
		t.Fatalf("CleanupFiles failed: %v", cleanupErr)
	}

	// Assert CRLF preserved
	b, err := readFileInDir(tempDir, "en.json")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(b)
	if !strings.HasSuffix(content, utils.LineEndingCRLF) {
		start := max(len(content)-2, 0)
		t.Errorf("expected CRLF line ending, got tail %q", content[start:])
	}

	verifyOnlyCRLF(t, content, "en")
}

func TestNoTrimFunctionality(t *testing.T) {
	defer goleak.VerifyNone(t)

	testCases := []struct {
		name        string
		trimStrings bool
		expected    string
	}{
		{"with trimming", true, "Login"},
		{"without trimming", false, " Login "},
	}

	tempDir := t.TempDir()
	testContent := map[string]any{"auth": map[string]any{"login": " Login "}}

	cfg := testutil.MockConfig()
	cfg.Files.Directory = tempDir
	cfg.Files.Locales = map[string]string{"en.json": "en-US"}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Write test file
			enFile := filepath.Join(tempDir, "en.json")
			initialData, _ := utils.MarshalJSON(testContent, "  ")
			if err := writeTestFile(enFile, initialData); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Test repository behavior
			flagsInstance := flags.New()
			flagsInstance.TrimStrings = tc.trimStrings
			repo := files.NewRepository(cfg, flagsInstance)

			// Scan and apply trimming to test trimming behavior
			localeFiles, err := repo.Scan(context.Background())
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}

			// Apply trimming manually to test trimming behavior
			if trimErr := files.TrimFiles(context.Background(), localeFiles, flagsInstance.TrimStrings); trimErr != nil {
				t.Fatalf("TrimFiles failed: %v", trimErr)
			}

			// Verify result
			authMap := localeFiles["en-US"].Contents["auth"].(map[string]any)
			if authMap["login"] != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, authMap["login"])
			}
		})
	}
}
