package batching

import (
	"github.com/juliandreas/dire/internal/constants"
)

// BatchLimits defines the batching constraints for a translation service.
type BatchLimits struct {
	MaxRequestSizeBytes int // Maximum total request size in bytes
	MaxTextsPerRequest  int // Maximum number of texts in a single request
	BufferSizeBytes     int // Buffer to account for JSON overhead
}

const (
	// DeepL limits (per official API documentation).
	deeplMaxRequestSizeKiB  = 128  // 128 KiB maximum request body size (official limit)
	deeplMaxTextsPerRequest = 1000 // No official limit - conservative high number for batching
	deeplBufferSizeKiB      = 8    // 8 KiB buffer for JSON overhead

	// Google Cloud Translation limits (per official API documentation).
	googleMaxRequestSizeKiB  = 10  // 5K characters recommended per request (~10 KiB with buffer)
	googleMaxTextsPerRequest = 128 // 128 text segments per request (official hard limit)
	googleBufferSizeKiB      = 2   // 2 KiB buffer for JSON overhead

	// Azure AI Translator limits (per official API documentation).
	azureMaxRequestSizeKiB  = 48   // 50,000 characters maximum per request (~48 KiB, official limit)
	azureMaxTextsPerRequest = 1000 // 1,000 array elements per request (official limit)
	azureBufferSizeKiB      = 10   // 10 KiB buffer for JSON overhead

	// Default limits for unknown providers.
	defaultMaxRequestSizeKiB  = 50  // Conservative default
	defaultMaxTextsPerRequest = 100 // Conservative default
	defaultBufferSizeKiB      = 10  // Conservative default

	// Conversion factor.
	bytesPerKiB = 1024 // Bytes per kibibyte
)

// GetBatchLimits returns the batching limits for a specific provider.
func GetBatchLimits(providerName string) BatchLimits {
	switch providerName {
	case constants.ProviderDeepL:
		return BatchLimits{
			MaxRequestSizeBytes: deeplMaxRequestSizeKiB * bytesPerKiB,
			MaxTextsPerRequest:  deeplMaxTextsPerRequest,
			BufferSizeBytes:     deeplBufferSizeKiB * bytesPerKiB,
		}
	case constants.ProviderGoogle:
		return BatchLimits{
			MaxRequestSizeBytes: googleMaxRequestSizeKiB * bytesPerKiB,
			MaxTextsPerRequest:  googleMaxTextsPerRequest,
			BufferSizeBytes:     googleBufferSizeKiB * bytesPerKiB,
		}
	case constants.ProviderAzure:
		return BatchLimits{
			MaxRequestSizeBytes: azureMaxRequestSizeKiB * bytesPerKiB,
			MaxTextsPerRequest:  azureMaxTextsPerRequest,
			BufferSizeBytes:     azureBufferSizeKiB * bytesPerKiB,
		}
	default:
		// Conservative defaults for unknown providers
		return BatchLimits{
			MaxRequestSizeBytes: defaultMaxRequestSizeKiB * bytesPerKiB,
			MaxTextsPerRequest:  defaultMaxTextsPerRequest,
			BufferSizeBytes:     defaultBufferSizeKiB * bytesPerKiB,
		}
	}
}
