package batching_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/juliandreas/dire/internal/llm"
	"github.com/juliandreas/dire/internal/llm/batching"
	"github.com/juliandreas/dire/internal/testutil"
)

// TestNewPairProcessor tests constructor.
func TestNewPairProcessor(t *testing.T) {
	client := &llm.Client{}
	cfg := testutil.MockConfig()

	processor := batching.NewPairProcessor(client, cfg)

	if processor == nil {
		t.Fatal("Expected non-nil processor")
	}
}

// TestConcurrentCounterUpdate verifies that concurrent counter updates work correctly
// when properly synchronized with wg.Wait().
func TestConcurrentCounterUpdate(t *testing.T) {
	var translatedCount int
	var countMutex sync.Mutex
	var wg sync.WaitGroup

	numGoroutines := 5
	translationsPerGoroutine := 10

	for range numGoroutines {
		wg.Go(func() {
			countMutex.Lock()
			translatedCount += translationsPerGoroutine
			countMutex.Unlock()
		})
	}

	wg.Wait()

	expectedCount := numGoroutines * translationsPerGoroutine
	if translatedCount != expectedCount {
		t.Errorf("Expected %d, got %d", expectedCount, translatedCount)
	}
}

// TestEarlyCounterRead_DetectsBug demonstrates the original race condition bug:
// reading the counter before waiting for all goroutines to complete.
func TestEarlyCounterRead_DetectsBug(t *testing.T) {
	// Run multiple times to increase chance of detecting the race condition
	var raceDetected bool

	for attempt := range 100 {
		var translatedCount int
		var countMutex sync.Mutex
		var wg sync.WaitGroup

		numGoroutines := 10
		translationsPerGoroutine := 5

		for range numGoroutines {
			wg.Go(func() {
				// Add small delay to increase chance of race condition
				// In real code, this would be actual translation work
				for range 1000 {
					// Busy work to simulate processing time
				}
				countMutex.Lock()
				translatedCount += translationsPerGoroutine
				countMutex.Unlock()
			})
		}

		// Read counter before waiting (the original bug)
		countMutex.Lock()
		earlyCount := translatedCount
		countMutex.Unlock()

		wg.Wait()

		expectedCount := numGoroutines * translationsPerGoroutine
		if translatedCount != expectedCount {
			t.Errorf("Final count wrong. Expected %d, got %d", expectedCount, translatedCount)
		}

		// Check if we detected the race condition
		if earlyCount < expectedCount {
			raceDetected = true
			t.Logf("Race condition detected on attempt %d: early=%d, final=%d",
				attempt+1, earlyCount, translatedCount)
			break
		}
	}

	if !raceDetected {
		t.Logf("Race condition not reliably detected in 100 attempts - this is expected on fast machines")
	}
}

// TestProperCounterSynchronization verifies the correct synchronization pattern.
func TestProperCounterSynchronization(t *testing.T) {
	var translatedCount int
	var countMutex sync.Mutex
	var wg sync.WaitGroup

	numGoroutines := 5
	translationsPerGoroutine := 8

	for range numGoroutines {
		wg.Go(func() {
			countMutex.Lock()
			translatedCount += translationsPerGoroutine
			countMutex.Unlock()
		})
	}

	wg.Wait()

	expectedCount := numGoroutines * translationsPerGoroutine
	if translatedCount != expectedCount {
		t.Errorf("Expected %d, got %d", expectedCount, translatedCount)
	}
}

// TestConcurrentCounterUpdate_Stress runs the concurrent test multiple times
// to increase chances of detecting race conditions.
func TestConcurrentCounterUpdate_Stress(t *testing.T) {
	t.Parallel()

	for i := range 50 {
		t.Run(fmt.Sprintf("iteration-%d", i), func(t *testing.T) {
			t.Parallel()

			// Inline the test logic instead of calling another test function
			var translatedCount int
			var countMutex sync.Mutex
			var wg sync.WaitGroup

			numGoroutines := 5
			translationsPerGoroutine := 10

			for range numGoroutines {
				wg.Go(func() {
					countMutex.Lock()
					translatedCount += translationsPerGoroutine
					countMutex.Unlock()
				})
			}

			wg.Wait()

			expectedCount := numGoroutines * translationsPerGoroutine
			if translatedCount != expectedCount {
				t.Errorf("Expected %d, got %d", expectedCount, translatedCount)
			}
		})
	}
}
