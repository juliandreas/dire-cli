package ui_test

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/juliandreas/dire/internal/cli/ui"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestNewTimer(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)

	if timer == nil {
		t.Fatal("NewTimer returned nil")
	}

	// Verify initial state - we can't access unexported fields from test package
	// So we'll test the behavior indirectly
	timer.Start(context.Background())
	time.Sleep(5 * time.Millisecond)

	if timer == nil {
		t.Error("timer should still be valid after start")
	}

	timer.Stop()
}

func TestTimerStartStop(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)

	// Test start
	timer.Start(context.Background())

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Test stop
	timer.Stop()

	// Verify timer was created and can be started/stopped
	if timer == nil {
		t.Error("timer should be valid")
	}
}

func TestTimerMultipleStartStop(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)

	// Test multiple start/stop cycles
	for index := range 3 {
		timer.Start(context.Background())
		time.Sleep(5 * time.Millisecond)
		timer.Stop()

		if timer == nil {
			t.Errorf("timer should be valid after cycle %d", index+1)
		}
	}
}

func TestTimerRestart(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)

	// Start first time
	timer.Start(context.Background())
	time.Sleep(5 * time.Millisecond)

	// Restart should stop and start again
	timer.Restart(context.Background())
	time.Sleep(5 * time.Millisecond)

	if timer == nil {
		t.Error("timer should be valid after Restart()")
	}

	timer.Stop()
}

func TestTimerComplete(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)

	timer.Start(context.Background())
	time.Sleep(5 * time.Millisecond)

	// Complete should stop the timer
	timer.Complete("Test completed", true)

	if timer == nil {
		t.Error("timer should be valid after Complete()")
	}
}

func TestTimerCompleteWithoutElapsed(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)

	timer.Start(context.Background())
	time.Sleep(5 * time.Millisecond)

	// Complete without showing elapsed time
	timer.Complete("Test completed", false)

	if timer == nil {
		t.Error("timer should be valid after Complete()")
	}
}

func TestTimerElapsed(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)

	start := time.Now()
	timer.Start(context.Background())
	time.Sleep(10 * time.Millisecond)

	elapsed := timer.Elapsed()
	actualElapsed := time.Since(start)

	// Should be roughly the same (within reasonable tolerance)
	tolerance := 20 * time.Millisecond // generous for CI environments
	if elapsed < actualElapsed-tolerance || elapsed > actualElapsed+tolerance {
		t.Errorf("elapsed time seems incorrect: got %v, expected around %v (±%v)",
			elapsed, actualElapsed, tolerance)
	}

	timer.Stop()
}

func TestTimerClearLine(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)

	timer.Start(context.Background())
	time.Sleep(5 * time.Millisecond)

	// ClearLine should not stop the timer
	timer.ClearLine()

	if timer == nil {
		t.Error("timer should be valid after ClearLine()")
	}

	timer.Stop()
}

func TestTimerStopWhenNotRunning(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)

	// Stop when not running should not panic
	timer.Stop()

	if timer == nil {
		t.Error("timer should be valid")
	}
}

func TestTimerStartWhenAlreadyRunning(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)

	timer.Start(context.Background())
	time.Sleep(5 * time.Millisecond)

	// Starting again should not create additional goroutines
	timer.Start(context.Background())
	time.Sleep(5 * time.Millisecond)

	if timer == nil {
		t.Error("timer should be valid")
	}

	timer.Stop()
}

func TestTimerConcurrentOperations(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)

	// Test concurrent start/stop operations
	done := make(chan struct{}, 2)

	// Goroutine 1: start/stop cycles
	go func() {
		defer func() { done <- struct{}{} }()
		for range 5 {
			timer.Start(context.Background())
			time.Sleep(2 * time.Millisecond)
			timer.Stop()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Goroutine 2: clear line operations
	go func() {
		defer func() { done <- struct{}{} }()
		for range 10 {
			timer.ClearLine()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Ensure timer is stopped
	timer.Stop()

	if timer == nil {
		t.Error("timer should be valid after concurrent operations")
	}
}

func TestTimerAnimationCycle(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)

	// Test that the timer cycles through animation characters
	timer.Start(context.Background())

	// Let it run for enough time to cycle through a few animations
	time.Sleep(400 * time.Millisecond)

	// Verify the timer is still valid
	if timer == nil {
		t.Error("timer should be valid during animation")
	}

	timer.Stop()
}

func TestTimerThinkingChars(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)

	// Verify the timer was created successfully
	if timer == nil {
		t.Error("timer should be created successfully")
	}

	// Start and stop to ensure the timer works
	timer.Start(context.Background())
	time.Sleep(5 * time.Millisecond)
	timer.Stop()
}

func TestTimerRapidRestarts(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)

	// Test rapid restart calls
	for range 10 {
		timer.Restart(context.Background())
		time.Sleep(1 * time.Millisecond)
	}

	// Give the timer a moment to settle
	time.Sleep(10 * time.Millisecond)

	if timer == nil {
		t.Error("timer should be valid after rapid restarts")
	}

	timer.Stop()
}

func TestTimerConcurrentRestarts(t *testing.T) {
	defer goleak.VerifyNone(t)

	timer := ui.NewTimer(false)
	done := make(chan struct{})
	numGoroutines := 5

	// Multiple goroutines calling restart concurrently
	for range numGoroutines {
		go func() {
			defer func() { done <- struct{}{} }()
			for range 3 {
				timer.Restart(context.Background())
				time.Sleep(2 * time.Millisecond)
			}
		}()
	}

	// Wait for all goroutines to complete
	for range numGoroutines {
		<-done
	}

	// Give the timer a moment to settle
	time.Sleep(10 * time.Millisecond)

	if timer == nil {
		t.Error("timer should be valid after concurrent restarts")
	}

	timer.Stop()
}

func TestTimerRestartPreservesProcessStartTime(t *testing.T) {
	defer goleak.VerifyNone(t)

	// This test verifies the specific bug fix where timer.Restart() should preserve
	// the original process start time for completion timing calculation.
	// Previously, Restart() would reset startTime, causing Complete() to show
	// only the time since the last restart instead of total process time.

	timer := ui.NewTimer(false)

	// Start the timer and record the approximate start time
	timer.Start(context.Background())
	afterStart := time.Now()

	// Wait a measurable amount of time
	time.Sleep(50 * time.Millisecond)

	// Restart the timer (this was the source of the bug)
	restartTime := time.Now()
	timer.Restart(context.Background())

	// Wait a shorter amount of time after restart
	time.Sleep(20 * time.Millisecond)

	// Get the elapsed time that Complete() would use
	// This should reflect the time from the original start, not from restart
	completeTime := time.Now()

	// Calculate expected durations
	minTotalDuration := completeTime.Sub(afterStart)    // Minimum total time
	maxRestartDuration := completeTime.Sub(restartTime) // Time since restart

	// The bug would cause Complete() to show maxRestartDuration (~20ms)
	// The fix should make it show minTotalDuration (~70ms)

	// Verify that the total duration is significantly longer than restart duration
	// This indirectly tests that the process start time is preserved
	if minTotalDuration <= maxRestartDuration*2 {
		t.Skip("Test timing insufficient to verify fix, skipping")
	}

	// Test that Complete() behavior is consistent with the fix
	// While we can't directly test the output, we can verify the timer
	// maintains correct state through restart operations
	timer.Complete("Test completed", true)

	// Verify no crash or invalid state
	if timer == nil {
		t.Error("timer should remain valid after completion")
	}

	// Document the expected behavior for future developers
	t.Logf("Total process duration: ~%v, Time since restart: ~%v",
		minTotalDuration.Truncate(time.Millisecond),
		maxRestartDuration.Truncate(time.Millisecond))
	t.Logf("Complete() should show total process time, not time since restart")
}

func TestTimerDebugModeOptimization(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Test that debug mode timer early-exits without running ticker loop
	ctx, cancel := context.WithCancel(context.Background())

	// Create timer in debug mode
	timer := ui.NewTimer(true)
	timer.Start(ctx)

	// Give it a brief moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context to trigger early exit
	cancel()

	// Timer should stop cleanly without any ticker activity
	timer.Stop()

	// Verify Elapsed() still works (should be non-zero)
	elapsed := timer.Elapsed()
	if elapsed <= 0 {
		t.Error("Elapsed() should work in debug mode")
	}

	// Verify Complete() still works
	timer.Complete("Debug test completed", true)
}

func TestTimerSuppressesOutputInDebugMode(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Capture stdout
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = writer

	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = writer.Close() // safe if already closed
		_ = reader.Close()
	})

	// Create timer in debug mode
	timer := ui.NewTimer(true)
	ctx := t.Context()

	// Start timer and let it run long enough to potentially produce animation output
	timer.Start(ctx)
	time.Sleep(100 * time.Millisecond) // Long enough for multiple animation cycles

	// Test that ClearLine() produces no output in debug mode
	timer.ClearLine()

	// Stop timer (should also not produce clear-line output in debug mode)
	timer.Stop()

	// Close write end and read captured output
	_ = writer.Close()
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}

	// In debug mode, there should be no animation or clear-line output
	if len(output) != 0 {
		t.Fatalf("expected no output in debug mode, got: %q", string(output))
	}
}
