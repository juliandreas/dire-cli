package files

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

// UpdateLocaleFile updates the locale file with the given updates.
func UpdateLocaleFile(ctx context.Context, file *types.LocaleFile, updates []types.LocaleContent, indent string) error {
	// Check if the context has been canceled
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Apply updates to the file contents
	contents := file.Contents

	for _, update := range updates {
		if update.Locale != file.Locale {
			continue
		}

		// Handle nested paths - use strict variant to prevent data corruption
		if err := SetValueStrict(contents, update.Path, update.Value); err != nil {
			return fmt.Errorf("failed to set value at path %s: %w", strings.Join(update.Path, "."), err)
		}
	}

	// Marshal with sorting and write to file
	return writeFileSorted(ctx, file.Path, contents, indent, file.LineEnding)
}

// TrimFiles performs in-place string trimming on file contents without writing to disk.
func TrimFiles(ctx context.Context, files map[types.Locale]*types.LocaleFile, trimStrings bool) error {
	if !trimStrings {
		return nil // Nothing to do
	}

	for _, file := range files {
		// Check if the context has been canceled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Apply trimming operations in-place
		file.Contents = trimAllStrings(file.Contents).(map[string]any)
	}

	return nil
}

// CleanupFiles performs file cleanup operations like trimming and sorting.
func CleanupFiles(
	ctx context.Context, files map[types.Locale]*types.LocaleFile, trimStrings bool, indent string,
) error {
	// Single loop for both trimming and writing for optimal performance
	for _, file := range files {
		// Check if the context has been canceled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Apply trimming if requested
		if trimStrings {
			file.Contents = trimAllStrings(file.Contents).(map[string]any)
		}

		// Sort and write the file
		if err := writeFileSorted(ctx, file.Path, file.Contents, indent, file.LineEnding); err != nil {
			return fmt.Errorf("failed to cleanup file %s: %w", file.Path, err)
		}
	}

	return nil
}

// trimAllStrings recursively trims all string values in a nested structure.
func trimAllStrings(data any) any {
	switch value := data.(type) {
	case map[string]any:
		for key, item := range value {
			value[key] = trimAllStrings(item)
		}
		return value
	case []any:
		for i, item := range value {
			value[i] = trimAllStrings(item)
		}
		return value
	case string:
		return strings.TrimSpace(value)
	default:
		// For other types (numbers, booleans, etc.), return as-is
		return value
	}
}

// writeFileSorted marshals JSON with sorted keys and writes to file.
func writeFileSorted(ctx context.Context, filePath string, contents map[string]any, indent, lineEnding string) error {
	// Marshal with sorting
	data, err := utils.MarshalJSON(contents, indent)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Abort if the operation was canceled while marshaling
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Default empty/unknown to LF to avoid removing newlines
	if lineEnding == "" {
		lineEnding = utils.LineEndingLF
	}
	// Convert line endings if needed (JSON encoder always uses LF)
	if lineEnding != utils.LineEndingLF {
		data = bytes.ReplaceAll(data, []byte(utils.LineEndingLF), []byte(lineEnding))
	}

	// Add trailing line ending if not present
	if len(data) > 0 && !bytes.HasSuffix(data, []byte(lineEnding)) {
		data = append(data, []byte(lineEnding)...)
	}

	// Write back to file anchored to a secure root to avoid symlink races
	root, err := os.OpenRoot(filepath.Dir(filePath))
	if err != nil {
		return fmt.Errorf("failed to open secure root for %s: %w", filePath, err)
	}

	// Perform the atomic write
	writeErr := writeFileAtomically(ctx, root, filePath, data)

	// Close the root and surface any close errors
	if closeErr := root.Close(); closeErr != nil {
		if writeErr != nil {
			return errors.Join(writeErr, fmt.Errorf("failed to close secure root for %s: %w", filePath, closeErr))
		}
		// If only close failed, return the close error
		return fmt.Errorf("failed to close secure root for %s: %w", filePath, closeErr)
	}

	return writeErr
}

// writeFileAtomically performs an atomic write using temp file + rename to avoid partial states on crash.
func writeFileAtomically(ctx context.Context, root *os.Root, filePath string, data []byte) error {
	finalName := filepath.Base(filePath)

	// Generate unique temp filename to avoid collisions in concurrent scenarios
	var tempName string
	const randomBytesLength = 8 // 64-bit random identifier
	randomBytes := make([]byte, randomBytesLength)
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback to time-based if crypto/rand fails
		tempName = fmt.Sprintf("%s.tmp.%d.%d", finalName, os.Getpid(), time.Now().UnixNano())
	} else {
		tempName = fmt.Sprintf("%s.tmp.%s", finalName, hex.EncodeToString(randomBytes))
	}

	// Create temp file with exclusive access
	tempFile, err := root.OpenFile(
		tempName,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		constants.SecureFilePermissions,
	)
	if err != nil {
		return fmt.Errorf("failed to create temp file for %s: %w", filePath, err)
	}

	// Helper to cleanup temp file on failure
	cleanup := func() {
		_ = tempFile.Close()
		_ = root.Remove(tempName)
	}

	// Write, sync, and close in sequence
	if written, err := tempFile.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("failed to write temp file for %s: %w", filePath, err)
	} else if written != len(data) {
		cleanup()
		return fmt.Errorf("%w: %s (wrote %d bytes, expected %d)", io.ErrShortWrite, filePath, written, len(data))
	}

	if err := tempFile.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("failed to sync temp file for %s: %w", filePath, err)
	}

	if err := tempFile.Close(); err != nil {
		_ = root.Remove(tempName)
		return fmt.Errorf("failed to close temp file for %s: %w", filePath, err)
	}

	// Check cancellation before making the write visible
	if ctx.Err() != nil {
		_ = root.Remove(tempName)
		return ctx.Err()
	}

	// Atomic rename (on Unix)
	if err := root.Rename(tempName, finalName); err != nil {
		_ = root.Remove(tempName)
		return fmt.Errorf("failed to rename temp file to %s: %w", filePath, err)
	}

	// Sync directory for crash durability
	syncDirectoryForDurability(root)

	return nil
}

// syncDirectoryForDurability syncs the directory to ensure rename operations are durable across power loss.
func syncDirectoryForDurability(root *os.Root) {
	// Skip on Windows since directory sync isn't supported
	if runtime.GOOS == "windows" {
		return
	}

	// Sync directory for crash durability
	if dirFile, err := root.Open("."); err == nil {
		_ = dirFile.Sync() // Ignore errors - not all filesystems support this
		_ = dirFile.Close()
	}
}
