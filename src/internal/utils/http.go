package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/juliandreas/dire/internal/errs"
)

// HTTPProviderError represents an HTTP error from a provider with status code preserved.
type HTTPProviderError struct {
	StatusCode int
	Provider   string
	Message    string
}

const (
	// maxErrorResponseSize is the maximum size for reading error response bodies.
	maxErrorResponseSize = 1024
)

// Error implements the error interface.
func (e *HTTPProviderError) Error() string {
	return fmt.Sprintf("provider error: %s: %s", e.Provider, e.Message)
}

// Is implements error checking for ErrProviderError.
func (e *HTTPProviderError) Is(target error) bool {
	return errors.Is(target, errs.ErrProviderError)
}

// NewHTTPProviderError creates a provider error that preserves the HTTP status code.
func NewHTTPProviderError(statusCode int, provider, message string) error {
	return &HTTPProviderError{
		StatusCode: statusCode,
		Provider:   provider,
		Message:    message,
	}
}

// CreateRequest creates an HTTP request with JSON body and headers.
func CreateRequest(
	ctx context.Context,
	method, url string,
	headers map[string]string,
	body any,
) (*http.Request, error) {
	var bodyReader *bytes.Reader

	if body != nil {
		jsonBytes, err := MarshalJSON(body, "")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	var req *http.Request
	var err error

	if bodyReader != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, bodyReader)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, http.NoBody)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers for JSON requests
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Always request JSON responses
	req.Header.Set("Accept", "application/json")

	// Set additional headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

// HandleError processes HTTP error responses and returns appropriate errors.
func HandleError(resp *http.Response, providerName string) error {
	// Try to read the response body for error details
	var errMsg string
	if resp.Body != nil {
		limitedReader := io.LimitReader(resp.Body, maxErrorResponseSize)
		bodyBytes, err := io.ReadAll(limitedReader)
		if err != nil {
			// Log the read error but continue with error handling
			errMsg = fmt.Sprintf("(failed to read error response: %v)", err)
		} else {
			errMsg = strings.TrimSpace(string(bodyBytes))
		}
	}

	// Create error message
	baseMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)
	if errMsg != "" {
		baseMsg = fmt.Sprintf("%s: %s", baseMsg, errMsg)
	}

	// Determine error type based on status code
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		// Try to get retry-after header
		retryAfter := resp.Header.Get("Retry-After")
		baseErr := NewHTTPProviderError(resp.StatusCode, providerName, baseMsg+" (rate limited)")
		return errs.NewRetryableError(baseErr, retryAfter)

	case http.StatusUnauthorized:
		return NewHTTPProviderError(resp.StatusCode, providerName, "authentication failed: "+baseMsg)

	case http.StatusForbidden:
		return NewHTTPProviderError(resp.StatusCode, providerName, "forbidden: "+baseMsg)

	case http.StatusBadRequest:
		return NewHTTPProviderError(resp.StatusCode, providerName, "bad request: "+baseMsg)

	case http.StatusRequestTimeout, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		baseErr := NewHTTPProviderError(resp.StatusCode, providerName, baseMsg+" (server error)")
		return errs.NewRetryableError(baseErr, "")

	default:
		return NewHTTPProviderError(resp.StatusCode, providerName, baseMsg)
	}
}
