package ui

import (
	"context"
	"os"

	"github.com/juliandreas/dire/internal/logger"
	"golang.org/x/term"
)

// ASCII control characters for keyboard interrupts.
const (
	ctrlC  = 3  // Ctrl+C
	escKey = 27 // ESC key
)

// StartKeyboardListener starts a keyboard listener that cancels the context when ESC or Ctrl+C is pressed.
func StartKeyboardListener(ctx context.Context, cancel context.CancelFunc) {
	go listenForInterruptKeys(ctx, cancel)
}

// listenForInterruptKeys listens for ESC key press and Ctrl+C, cancels context when detected.
func listenForInterruptKeys(ctx context.Context, cancel context.CancelFunc) {
	// Check if stdin is a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		logger.Debug("stdin is not a terminal, skipping keyboard listener")
		return
	}

	// Save original terminal state
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		logger.Debug("Failed to set terminal to raw mode", "error", err)
		return
	}

	// Ensure terminal state is restored
	defer func() {
		if err := term.Restore(int(os.Stdin.Fd()), oldState); err != nil {
			logger.Debug("Failed to restore terminal state", "error", err)
		}
	}()

	// Buffer for reading input
	buf := make([]byte, 1)

	// Use a goroutine to handle stdin reading
	inputChan := make(chan byte, 1)
	go func() {
		defer close(inputChan)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				select {
				case inputChan <- buf[0]:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case key, ok := <-inputChan:
			if !ok {
				return
			}
			if key == escKey {
				logger.Debug("ESC key detected, cancelling context")
				cancel()
				return
			}
			if key == ctrlC {
				logger.Debug("Ctrl+C detected, cancelling context")
				cancel()
				return
			}
		}
	}
}
