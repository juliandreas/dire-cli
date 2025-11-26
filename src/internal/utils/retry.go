package utils

import (
	"crypto/rand"
	"errors"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/juliandreas/dire/internal/errs"
)

const (
	// Retry configuration constants.
	retryLargeBatchThreshold  = 20 // Threshold for considering a batch "large" for retry purposes
	largeBatchDelayMultiplier = 2  // Multiplier for base delay on large batches
	defaultMaxBatchRetries    = 2
	baseRetryDelay            = time.Second
	maxRetryAttempt           = 30  // Maximum retry attempts to prevent overflow
	maxJitterMs               = 500 // Maximum jitter in milliseconds for retry delays
)

// GetRetryParameters returns maxRetries and baseDelay based on batch size.
func GetRetryParameters(batchSize int) (maxRetries int, baseDelay time.Duration) {
	maxRetries = defaultMaxBatchRetries
	baseDelay = baseRetryDelay

	// Only adjust for very large or very small batches
	if batchSize == 1 {
		// Single translation failures are often transient - worth one extra try
		maxRetries = 3
	} else if batchSize > retryLargeBatchThreshold {
		// Large batches more likely to have network issues - but don't over-retry
		maxRetries = 3
		baseDelay = time.Duration(largeBatchDelayMultiplier) * time.Second
	}

	return maxRetries, baseDelay
}

// CalculateRetryDelay calculates exponential backoff delay with jitter and retry-after header support.
func CalculateRetryDelay(attempt int, lastErr error) time.Duration {
	const maxRetryDelay = 30 * time.Second

	// Check if we have a retry duration from the error
	if retryAfter := parseRetryDelay(lastErr); retryAfter > 0 {
		if retryAfter > maxRetryDelay {
			return maxRetryDelay
		}
		return retryAfter
	}

	// Fall back to exponential backoff with jitter
	if attempt > maxRetryAttempt { // cap to avoid undefined shifts & giant delays
		attempt = maxRetryAttempt
	}
	if attempt < 0 { // ensure non-negative to avoid overflow
		attempt = 0
	}
	factor := 1 << attempt // 2^attempt
	baseDelay := min(time.Duration(factor)*baseRetryDelay, maxRetryDelay)

	// Add some jitter using a cryptographically secure random number
	var jitter time.Duration
	n, err := rand.Int(rand.Reader, big.NewInt(maxJitterMs))
	if err == nil {
		jitter = time.Duration(n.Int64()) * time.Millisecond
	}

	return baseDelay + jitter
}

// parseRetryDelay extracts retry timing from a retryable error's message.
func parseRetryDelay(err error) time.Duration {
	if !errors.Is(err, errs.ErrRetryable) {
		return 0
	}

	errMsg := err.Error()

	// Look for "(retry-after: X)" in error message
	start := strings.Index(errMsg, "(retry-after: ")
	if start == -1 {
		return 0
	}
	start += len("(retry-after: ")

	end := strings.Index(errMsg[start:], ")")
	if end == -1 {
		return 0
	}

	retryAfterStr := errMsg[start : start+end]
	return parseRetryAfter(retryAfterStr)
}

// parseRetryAfter parses the Retry-After header value.
func parseRetryAfter(retryAfter string) time.Duration {
	if retryAfter == "" {
		return 0
	}

	// Try parsing as seconds first
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 && seconds <= 600 {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP date
	if retryTime, err := http.ParseTime(retryAfter); err == nil {
		if delay := time.Until(retryTime); delay > 0 {
			return delay
		}
	}

	return 0
}
