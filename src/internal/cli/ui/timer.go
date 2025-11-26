package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/juliandreas/dire/internal/constants"
)

// Timer shows elapsed time.
type Timer struct {
	message           string
	startTime         time.Time
	processStartTime  time.Time
	cancel            context.CancelFunc
	mu                sync.Mutex
	done              chan struct{}
	thinkingIndex     int
	thinkingChars     []string
	processingTimings []time.Duration
	isLogging         bool
}

const (
	clearLine     = "\r\033[K"
	processingMsg = "Tinkering…"
)

// NewTimer creates a new timer.
// When isLogging is true (verbose or debug mode), the spinner is hidden to avoid interfering with logs.
func NewTimer(isLogging bool) *Timer {
	now := time.Now()
	return &Timer{
		message:          processingMsg,
		startTime:        now,
		processStartTime: now,
		cancel:           nil,
		mu:               sync.Mutex{},
		done:             nil,
		thinkingIndex:    0,
		thinkingChars:    []string{"·", "*", "✻", "✽", "*", "✶"},
		processingTimings: []time.Duration{
			180 * time.Millisecond, // · (index 0)
			90 * time.Millisecond,  // * (index 1) - burst
			90 * time.Millisecond,  // ✻ (index 2) - burst
			400 * time.Millisecond, // ✽ (index 3) - special long pause
			90 * time.Millisecond,  // * (index 4) - burst
			350 * time.Millisecond, // ✶ (index 5) - pause
		},
		isLogging: isLogging,
	}
}

// Start begins the timer.
func (t *Timer) Start(ctx context.Context) {
	t.mu.Lock()
	// Cancel existing timer if running
	if t.cancel != nil {
		t.cancel()
	}

	// Capture the previous done channel and immediately replace it
	// This ensures only one goroutine will wait on this specific channel
	prevDone := t.done

	// Create new context and start timer
	timerCtx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	t.startTime = time.Now()
	t.thinkingIndex = 0

	// Create a per-run completion channel
	done := make(chan struct{})
	t.done = done
	t.mu.Unlock()

	// Wait for prior run to exit to avoid interleaved prints
	// This happens outside the lock to prevent blocking other operations
	if prevDone != nil {
		<-prevDone
	}

	// Start the new goroutine after ensuring the previous one has finished
	go t.run(timerCtx, done)
}

// Stop stops the timer and clears the line.
func (t *Timer) Stop() {
	t.mu.Lock()
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
	done := t.done
	t.mu.Unlock()

	// Wait for the current run to fully exit (if any)
	if done != nil {
		<-done
	}

	// Clear line if not in logging mode
	t.mu.Lock()
	isLogging := t.isLogging
	t.mu.Unlock()
	if !isLogging {
		fmt.Print(clearLine)
		_ = os.Stdout.Sync() // Explicit flush for macOS terminals
	}
}

// ClearLine clears the current line without stopping.
func (t *Timer) ClearLine() {
	t.mu.Lock()
	isLogging := t.isLogging
	t.mu.Unlock()
	if !isLogging {
		fmt.Print(clearLine)
		_ = os.Stdout.Sync() // Explicit flush for macOS terminals
	}
}

// Restart stops and starts with the default message.
func (t *Timer) Restart(ctx context.Context) {
	t.mu.Lock()
	t.message = processingMsg
	// Preserve the original process start time
	originalProcessStartTime := t.processStartTime
	t.mu.Unlock()

	t.Start(ctx)

	t.mu.Lock()
	t.processStartTime = originalProcessStartTime
	t.mu.Unlock()
}

// Complete stops the timer and shows a completion message.
func (t *Timer) Complete(message string, showElapsed bool) {
	// Get elapsed time before stopping
	elapsed := time.Since(t.processStartTime)

	// Stop the timer (this will clear the line)
	t.Stop()

	// Now print the completion message
	if showElapsed {
		timing := constants.Gray().Render(fmt.Sprintf("(%s)", formatElapsed(elapsed)))
		fmt.Printf("%s %s\n", message, timing)
	} else {
		fmt.Println(message)
	}
}

// Elapsed returns the time elapsed since creation.
func (t *Timer) Elapsed() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return time.Since(t.startTime)
}

// run is the main timer loop that runs in a goroutine.
func (t *Timer) run(ctx context.Context, done chan struct{}) {
	defer close(done)

	// When in logging mode (verbose/debug), skip all spinner updates.
	t.mu.Lock()
	isLogging := t.isLogging
	t.mu.Unlock()
	if isLogging {
		<-ctx.Done()
		return
	}

	t.mu.Lock()
	initialInterval := t.getNextInterval()
	t.mu.Unlock()

	ticker := time.NewTicker(initialInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.mu.Lock()
			// Check cancellation after acquiring lock
			if ctx.Err() != nil {
				t.mu.Unlock()
				return
			}
			idx := t.thinkingIndex
			start := t.startTime
			msg := t.message
			t.thinkingIndex++
			nextInterval := t.getNextInterval()
			t.mu.Unlock()

			thinking := constants.Yellow().Render(t.thinkingChars[idx%len(t.thinkingChars)])
			elapsed := time.Since(start)
			styledMessage := constants.Yellow().Render(msg)
			styledElapsed := buildElapsedDisplay(elapsed)

			fmt.Printf("%s%s  %s %s", clearLine, thinking, styledMessage, styledElapsed)
			_ = os.Stdout.Sync() // Explicit flush for macOS terminals
			ticker.Reset(nextInterval)
		}
	}
}

// getNextInterval returns the next timing interval based on current symbol.
// Must be called with mutex held.
func (t *Timer) getNextInterval() time.Duration {
	return t.processingTimings[t.thinkingIndex%len(t.processingTimings)]
}

// formatElapsed formats duration for display.
func formatElapsed(d time.Duration) string {
	const secondsPerMinute = 60
	seconds := float64(d) / float64(time.Second)

	// If less than 60 seconds, show with 2 decimal places
	if seconds < secondsPerMinute {
		return fmt.Sprintf("%.2fs", seconds)
	}

	// For 60 seconds or more, show in minutes and seconds format
	minutes := int(seconds) / secondsPerMinute
	remainingSeconds := int(seconds) % secondsPerMinute
	return fmt.Sprintf("%dm%ds", minutes, remainingSeconds)
}

// formatElapsedRounded formats duration for display with second precision.
func formatElapsedRounded(d time.Duration) string {
	return d.Truncate(time.Second).String()
}

// buildElapsedDisplay creates the styled elapsed time display with bold "esc".
func buildElapsedDisplay(elapsed time.Duration) string {
	gray := constants.Gray()
	parts := []string{
		gray.Render("("),
		gray.Render(formatElapsedRounded(elapsed)),
		gray.Render(" · "),
		gray.Bold(true).Render("esc"),
		gray.Render(" to interrupt"),
		gray.Render(")"),
	}
	return strings.Join(parts, "")
}
