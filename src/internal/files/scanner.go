package files

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

const maxLocaleFileBytes = 5 << 20 // 5 MiB cap per locale file

var (
	ErrFileTooLarge    = errors.New("locale file exceeds maximum allowed size")
	ErrNotRegularFile  = errors.New("not a regular file")
	ErrTrailingContent = errors.New("unexpected trailing content in JSON file")
)

// ScanLocaleFiles finds and reads all locale files based on the configuration.
func ScanLocaleFiles(ctx context.Context, cfg *config.Config) (map[types.Locale]*types.LocaleFile, error) {
	// Check if the context has been canceled
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Create a secure root filesystem for the configured directory
	root, err := os.OpenRoot(cfg.Files.Directory)
	if err != nil {
		return nil, fmt.Errorf("failed to open root directory %s: %w", cfg.Files.Directory, err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			logger.Debug("Failed to close root filesystem", "error", closeErr)
		}
	}()

	// Process files using the secure root
	return processLocaleFilesFromRoot(ctx, root, cfg)
}

// processLocaleFilesFromRoot processes locale files using a secure root filesystem.
func processLocaleFilesFromRoot(
	ctx context.Context, root *os.Root, cfg *config.Config,
) (map[types.Locale]*types.LocaleFile, error) {
	localeFiles := make(map[types.Locale]*types.LocaleFile)

	logger.Info("Scanning locale files", "directory", cfg.Files.Directory)

	// List all files in the directory using the same root for consistent trust boundary
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", cfg.Files.Directory, err)
	}

	// Ensure deterministic processing order by sorting entries by name (case-insensitive)
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip directories
		}

		// Only process JSON files
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}

		// Find locale using the patterns in config
		localeStr := getLocaleFromPatterns(entry.Name(), cfg.Files.Locales)
		if localeStr == "" {
			continue // Skip files where we can't determine locale
		}

		// Convert to Locale type
		locale := types.Locale(localeStr)

		// Read and parse JSON file using secure root filesystem
		contents, lineEnding, err := readJSONFileFromRoot(ctx, root, entry.Name())
		if err != nil {
			return nil, err
		}

		// Check for duplicate locale mapping before assignment
		if _, exists := localeFiles[locale]; exists {
			logger.Debug("Duplicate locale mapping; later filename overwrites earlier (lexicographic by name)",
				"locale", locale, "filename", entry.Name())
		}

		// Construct the full path for the LocaleFile
		fullPath := filepath.Clean(filepath.Join(cfg.Files.Directory, entry.Name()))

		localeFiles[locale] = &types.LocaleFile{
			Path:       fullPath,
			Locale:     locale,
			Contents:   contents,
			LineEnding: lineEnding,
		}
	}

	logger.Info("Scanned locale files", "count", len(localeFiles))

	return localeFiles, nil
}

func getLocaleFromPatterns(filename string, patterns map[string]string) string {
	// First try exact matches
	if locale, ok := patterns[filename]; ok {
		return locale
	}

	// Then try glob patterns in a deterministic order
	keys := make([]string, 0, len(patterns))
	for k := range patterns {
		// Only include glob-like patterns for the second pass
		if strings.ContainsAny(k, "*?[") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, pattern := range keys {
		if match, err := filepath.Match(pattern, filename); err != nil {
			logger.Debug("Invalid glob pattern in config; skipping", "pattern", pattern, "error", err)
			continue
		} else if match {
			return patterns[pattern]
		}
	}

	return ""
}

// readJSONFileFromRoot reads and parses a JSON file using os.Root for secure access.
// See: https://pkg.go.dev/os#OpenRoot
func readJSONFileFromRoot(ctx context.Context, root *os.Root, filename string) (map[string]any, string, error) {
	// Check if the context has been canceled before starting
	if ctx.Err() != nil {
		return nil, "", ctx.Err()
	}
	// Open the file securely through the root filesystem
	// os.Root automatically prevents directory traversal and access outside the root
	file, err := root.Open(filename)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Debug("Failed to close file", "filename", filename, "error", closeErr)
		}
	}()

	// Get file info for validation (fail fast for new application)
	info, err := file.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("failed to stat file %s: %w", filename, err)
	}

	// Ensure it's a regular file to avoid blocking on FIFOs/sockets/devices
	if !info.Mode().IsRegular() {
		return nil, "", fmt.Errorf("%w: %s (mode: %v)", ErrNotRegularFile, filename, info.Mode())
	}

	// Fail fast on oversized files
	if info.Size() > maxLocaleFileBytes {
		return nil, "", fmt.Errorf("%w: %s (%d bytes > %d bytes)",
			ErrFileTooLarge, filename, info.Size(), maxLocaleFileBytes)
	}

	// Read the file contents with a sane upper bound (+1 to detect oversize as a fallback)
	data, err := io.ReadAll(io.LimitReader(file, maxLocaleFileBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	// Check for cancellation after reading
	if ctx.Err() != nil {
		return nil, "", ctx.Err()
	}

	// If we read more than the limit, the file exceeded the cap after the Stat check.
	// We don't know the exact size here; report limit breach.
	if len(data) > maxLocaleFileBytes {
		return nil, "", fmt.Errorf("%w: %s (> %d bytes)", ErrFileTooLarge, filename, maxLocaleFileBytes)
	}

	// Detect line ending style before parsing
	lineEnding := utils.DetectLineEnding(data)

	// Short-circuit on empty/whitespace files to avoid parsing overhead
	if len(bytes.TrimSpace(data)) == 0 {
		return make(map[string]any), lineEnding, nil // Return empty object for empty files
	}

	// Parse JSON content with numeric fidelity preservation
	var contents map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // Preserve exact numeric values instead of converting to float64
	if err := dec.Decode(&contents); err != nil {
		return nil, "", fmt.Errorf("failed to parse JSON file %s: %w", filename, err)
	}

	// Ensure no trailing non-whitespace content after the JSON object
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, "", fmt.Errorf("failed to parse JSON file %s: %w", filename, ErrTrailingContent)
	}

	return contents, lineEnding, nil
}
