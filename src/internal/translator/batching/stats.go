package batching

import "github.com/juliandreas/dire/internal/types"

// BatchingStats provides information about the batching process.
type BatchingStats struct {
	OriginalCount int // Number of original values
	BatchCount    int // Number of batches created
	SkippedCount  int // Number of values skipped due to size limits
	TotalBytes    int // Estimated total bytes across all batches
}

// CalculateBatchingStats analyzes the batching results.
func CalculateBatchingStats(originalValues []types.TranslationValue, batches []TranslationBatch) BatchingStats {
	totalProcessed := 0
	totalBytes := 0

	for _, batch := range batches {
		totalProcessed += len(batch.Values)
		totalBytes += batch.EstimatedBytes
	}

	return BatchingStats{
		OriginalCount: len(originalValues),
		BatchCount:    len(batches),
		SkippedCount:  max(len(originalValues)-totalProcessed, 0),
		TotalBytes:    totalBytes,
	}
}
