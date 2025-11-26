package ui_test

import (
	"context"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/juliandreas/dire/internal/cli/ui"
)

func TestKeyboardConstants(t *testing.T) {
	// Test that the keyboard package can be used
	// This is a simple smoke test to verify the package works

	// The constants are not exported, but we can verify the package loads correctly
	// by creating a context and ensuring no immediate panic
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// This should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("StartKeyboardListener panicked: %v", r)
		}
	}()

	ui.StartKeyboardListener(ctx, func() {})

	// Cancel immediately to clean up
	cancel()
}

func TestStartKeyboardListener_DoesNotPanic(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Test that StartKeyboardListener doesn't panic when called
	// Even in test environment (where stdin is not a real terminal)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// This should work without panicking, even if it can't detect keys
	ui.StartKeyboardListener(ctx, func() {
		// Cancel function - may not be called in test env
	})

	// Wait for timeout to ensure no goroutine leak
	<-ctx.Done()

	// Test passes if no panic occurs
}

func TestStartKeyboardListener_ContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Test that the keyboard listener respects context cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Start the listener
	ui.StartKeyboardListener(ctx, func() {
		// This cancel function might not be called in test environment
	})

	// Cancel the context immediately
	cancel()

	// Give it a moment for cleanup
	time.Sleep(20 * time.Millisecond)

	// Test passes if no goroutine leak occurs (verified by goleak)
}

func TestStartKeyboardListener_MultipleInvocations(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Test that multiple invocations don't cause issues
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Start multiple listeners
	for range 3 {
		ui.StartKeyboardListener(ctx, func() {
			// Cancel functions
		})
	}

	// Wait for timeout
	<-ctx.Done()

	// Test passes if no panic or goroutine leak occurs
}

func TestStartKeyboardListener_ImmediateCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Test behavior when context is already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Start listener with already-canceled context
	ui.StartKeyboardListener(ctx, func() {
		t.Error("Cancel function should not be called with pre-canceled context")
	})

	// Give it a moment for cleanup
	time.Sleep(10 * time.Millisecond)

	// Test passes if cancel function is not called and no goroutine leak occurs
}
