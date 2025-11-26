package batching

import (
	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
)

// TranslationBatch represents a batch of texts ready for translation.
type TranslationBatch struct {
	Values         []types.TranslationValue // The values to translate
	EstimatedBytes int                      // Estimated request size in bytes
}

const (
	// maxBytesPerUTF8Char is the maximum bytes per code point in UTF-8 encoding (4)
	// Used for conservative size estimation of international text.
	maxBytesPerUTF8Char = 4
)

// CreateBatches splits translation values into batches that respect provider limits.
func CreateBatches(values []types.TranslationValue, limits BatchLimits) []TranslationBatch {
	if len(values) == 0 {
		return nil
	}

	var batches []TranslationBatch
	currentBatch := TranslationBatch{
		Values:         make([]types.TranslationValue, 0),
		EstimatedBytes: limits.BufferSizeBytes, // Start with buffer overhead
	}

	for _, value := range values {
		// Estimate the size this value would add to the request
		valueBytes := estimateValueSize(value)
		potentialSize := currentBatch.EstimatedBytes + valueBytes

		// Check if single value exceeds max size (overflow case)
		if valueBytes > limits.MaxRequestSizeBytes-limits.BufferSizeBytes {
			logger.Warn("Translation value exceeds maximum request size, skipping",
				"path", value.Path,
				"valueSize", valueBytes,
				"maxSize", limits.MaxRequestSizeBytes)
			continue // Skip this value as it's too large to process
		}

		// Check if adding this value would exceed limits
		shouldStartNewBatch := potentialSize > limits.MaxRequestSizeBytes ||
			len(currentBatch.Values) >= limits.MaxTextsPerRequest

		if shouldStartNewBatch && len(currentBatch.Values) > 0 {
			// Finish current batch and start a new one
			batches = append(batches, currentBatch)
			currentBatch = TranslationBatch{
				Values:         make([]types.TranslationValue, 0),
				EstimatedBytes: limits.BufferSizeBytes + valueBytes,
			}
			currentBatch.Values = append(currentBatch.Values, value)
		} else {
			// Add to current batch
			currentBatch.Values = append(currentBatch.Values, value)
			currentBatch.EstimatedBytes = potentialSize
		}
	}

	// Don't forget the last batch
	if len(currentBatch.Values) > 0 {
		batches = append(batches, currentBatch)
	}

	return batches
}

// estimateValueSize estimates the number of bytes a TranslationValue will add to the JSON request.
func estimateValueSize(value types.TranslationValue) int {
	// Estimate JSON overhead: quotes, commas, brackets, etc.
	// This is a conservative estimate for typical translation API JSON structures
	jsonOverhead := 20 // Bytes for JSON structure per text

	// UTF-8 encoding: most translation text is 1-3 bytes per character
	// Use 3 bytes per character as conservative estimate for international text
	textBytes := len(value.Value) * maxBytesPerUTF8Char

	return textBytes + jsonOverhead
}

// ExtractTexts extracts just the text values from a batch for API calls.
func (b *TranslationBatch) ExtractTexts() []string {
	texts := make([]string, len(b.Values))
	for i, value := range b.Values {
		texts[i] = value.Value
	}
	return texts
}
