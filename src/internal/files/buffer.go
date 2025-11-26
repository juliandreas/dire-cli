package files

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/types"
)

// WriteBuffer buffers translations before writing to files.
type WriteBuffer struct {
	translations []types.LocaleContent
	lastWrite    time.Time
	mutex        sync.Mutex
	config       *config.Config
}

// BufferedWriter manages concurrent translation writing with buffering.
type BufferedWriter struct {
	buffer           *WriteBuffer
	translationsChan chan []types.LocaleContent
	ctx              context.Context
	wg               sync.WaitGroup
	errorMutex       sync.Mutex
	firstError       error
	closeOnce        sync.Once
}

const (
	writeBufferSize      = 50               // Write after 50 translations
	writeBufferTimeout   = 30 * time.Second // Or after 30 seconds
	flushCheckInterval   = 5 * time.Second  // Check for timeout flushes every 5 seconds
	gracefulFlushTimeout = 5 * time.Second  // Timeout for final flush on context cancellation
)

var errLocaleFileNotFound = errors.New("no file found for locale")

// NewWriteBuffer creates a new write buffer.
func NewWriteBuffer(cfg *config.Config) *WriteBuffer {
	return &WriteBuffer{
		translations: make([]types.LocaleContent, 0),
		lastWrite:    time.Now(),
		mutex:        sync.Mutex{},
		config:       cfg,
	}
}

// Add adds translations to the buffer and returns true if it should flush.
func (wb *WriteBuffer) Add(translations []types.LocaleContent) bool {
	wb.mutex.Lock()
	defer wb.mutex.Unlock()

	if translations != nil {
		wb.translations = append(wb.translations, translations...)
	}

	// No data buffered: never flush due to timeouts
	if len(wb.translations) == 0 {
		return false
	}

	// Flush if buffer is full OR it's been too long since last write
	return len(wb.translations) >= writeBufferSize ||
		time.Since(wb.lastWrite) > writeBufferTimeout
}

// Flush writes all buffered translations to files.
func (wb *WriteBuffer) Flush(ctx context.Context) error {
	wb.mutex.Lock()
	defer wb.mutex.Unlock()

	if len(wb.translations) == 0 {
		return nil
	}

	// Get current file state
	localeFiles, err := ScanLocaleFiles(ctx, wb.config)
	if err != nil {
		return fmt.Errorf("failed to scan for buffered update: %w", err)
	}

	// Group translations by locale
	byLocale := make(map[types.Locale][]types.LocaleContent)
	for _, t := range wb.translations {
		byLocale[t.Locale] = append(byLocale[t.Locale], t)
	}

	// Update each existing file
	for locale, file := range localeFiles {
		if translations, ok := byLocale[locale]; ok && len(translations) > 0 {
			if err := UpdateLocaleFile(ctx, file, translations, wb.config.GetIndent()); err != nil {
				return fmt.Errorf("failed to update %s: %w", file.Path, err)
			}
			delete(byLocale, locale)
		}
	}

	// Check if there are any translations left for locales that don't have files
	// This should never happen if the config is correct, so log it as an error
	for locale, translations := range byLocale {
		if len(translations) > 0 {
			return fmt.Errorf("%w: attempted to write %d translations for locale %s",
				errLocaleFileNotFound, len(translations), locale)
		}
	}

	// Clear buffer and update timestamp
	wb.translations = wb.translations[:0] // Keep capacity, reset length
	wb.lastWrite = time.Now()

	return nil
}

// ForceFlush flushes any remaining translations.
func (wb *WriteBuffer) ForceFlush(ctx context.Context) error {
	wb.mutex.Lock()
	hasData := len(wb.translations) > 0
	wb.mutex.Unlock()

	if hasData {
		return wb.Flush(ctx)
	}
	return nil
}

// NewBufferedWriter creates a new buffered writer for a locale pair.
func NewBufferedWriter(ctx context.Context, cfg *config.Config, channelBufferSize int) *BufferedWriter {
	if channelBufferSize <= 0 {
		channelBufferSize = 1
	}

	return &BufferedWriter{
		buffer:           NewWriteBuffer(cfg),
		translationsChan: make(chan []types.LocaleContent, channelBufferSize),
		ctx:              ctx,
		wg:               sync.WaitGroup{},
		errorMutex:       sync.Mutex{},
		firstError:       nil,
		closeOnce:        sync.Once{},
	}
}

// Start begins the background writing goroutine.
func (bw *BufferedWriter) Start(ctx context.Context) {
	bw.wg.Go(func() {
		ticker := time.NewTicker(flushCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case translations, ok := <-bw.translationsChan:
				if !ok {
					// Channel closed, force flush any remaining and exit
					flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), gracefulFlushTimeout)
					if err := bw.buffer.ForceFlush(flushCtx); err != nil {
						bw.setError(fmt.Errorf("failed to flush final buffer: %w", err))
					}
					cancel()
					return
				}

				// Add to buffer and flush if needed
				if bw.buffer.Add(translations) {
					if err := bw.buffer.Flush(ctx); err != nil {
						bw.setError(fmt.Errorf("failed to flush buffer: %w", err))
					}
				}

			case <-ticker.C:
				// Periodic flush check for timeout
				if bw.buffer.Add(nil) { // Trigger timeout check
					if err := bw.buffer.Flush(ctx); err != nil {
						bw.setError(fmt.Errorf("failed to flush buffer on timeout: %w", err))
					}
				}

			case <-ctx.Done():
				// Context canceled - attempt bounded final flush to prevent data loss
				// Use a short timeout to avoid hanging on context cancellation
				flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), gracefulFlushTimeout)
				if err := bw.buffer.ForceFlush(flushCtx); err != nil {
					bw.setError(fmt.Errorf("failed to flush buffer on context cancellation: %w", err))
				}
				cancel()
				return
			}
		}
	})
}

// Send sends translations to the buffered writer (non-blocking).
func (bw *BufferedWriter) Send(translations []types.LocaleContent) bool {
	select {
	case bw.translationsChan <- translations:
		return true
	case <-bw.ctx.Done():
		return false
	default:
		// Channel is full and context not canceled - this shouldn't block
		// In a high-load scenario, we need to handle this gracefully
		return false
	}
}

// Close closes the writer and waits for final flush.
func (bw *BufferedWriter) Close() error {
	bw.closeOnce.Do(func() {
		close(bw.translationsChan)
		bw.wg.Wait()
	})
	return bw.getError()
}

// GetError safely gets the first error.
func (bw *BufferedWriter) GetError() error {
	return bw.getError()
}

// setError safely sets the first error.
func (bw *BufferedWriter) setError(err error) {
	bw.errorMutex.Lock()
	defer bw.errorMutex.Unlock()

	if bw.firstError == nil {
		bw.firstError = err
	}
}

// getError safely gets the first error.
func (bw *BufferedWriter) getError() error {
	bw.errorMutex.Lock()
	defer bw.errorMutex.Unlock()

	return bw.firstError
}
