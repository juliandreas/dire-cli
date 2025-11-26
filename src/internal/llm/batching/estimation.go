package batching

import (
	"math"
	"strings"

	"github.com/juliandreas/dire/internal/logger"
	"github.com/juliandreas/dire/internal/types"
	"github.com/juliandreas/dire/internal/utils"
)

// TokenEstimator estimates token usage for translations and values.
type TokenEstimator interface {
	EstimateTranslationTokens(translation types.MissingTranslation) int
	EstimateValueTokens(value any) int
}

// DefaultTokenEstimator implements TokenEstimator with standard estimation logic.
type DefaultTokenEstimator struct{}

const (
	charactersPerToken        = 2.5  // Conservative chars per token (varies by language, worst case: Chinese/Arabic)
	tokensPerPathSegment      = 4    // Estimated tokens per JSON path segment (e.g. "auth.login")
	jsonStructureOverhead     = 0.2  // Additional 20% for JSON formatting (braces, quotes, commas)
	jsonComplexityMultiplier  = 1.8  // JSON structure tokenization is more expensive than plain text
	minObjectTokens           = 20   // Minimum tokens for object values
	fallbackObjectTokens      = 100  // Conservative fallback for object estimation
	minArrayTokens            = 10   // Minimum tokens for array values
	fallbackArrayTokens       = 10   // Fallback tokens per array element
	nullTokens                = 2    // Tokens for null values
	booleanTokens             = 2    // Tokens for boolean values
	numberTokens              = 3    // Conservative estimate for numbers
	unknownTypeTokens         = 10   // Conservative fallback for unknown types
	largeTranslationThreshold = 1000 // Threshold for logging large translation estimates
)

// NewTokenEstimator creates a token estimator for translation size calculations.
func NewTokenEstimator() TokenEstimator {
	return &DefaultTokenEstimator{}
}

// EstimateTranslationTokens estimates total tokens needed for a single translation.
func (e *DefaultTokenEstimator) EstimateTranslationTokens(translation types.MissingTranslation) int {
	// Path tokens (each segment uses ~4 tokens)
	pathTokens := len(translation.Path) * tokensPerPathSegment

	// Value tokens
	valueTokens := e.EstimateValueTokens(translation.SourceValue)

	// JSON structure overhead
	overhead := int(float64(valueTokens) * jsonStructureOverhead)

	total := pathTokens + valueTokens + overhead

	// Only log for very large individual translations to avoid spam
	if total > largeTranslationThreshold {
		logger.Debug("Large translation token estimate",
			"path", strings.Join(translation.Path, "."),
			"pathTokens", pathTokens,
			"valueTokens", valueTokens,
			"overhead", overhead,
			"total", total)
	}

	return total
}

// EstimateValueTokens estimates tokens for any value type.
func (e *DefaultTokenEstimator) EstimateValueTokens(value any) int {
	switch val := value.(type) {
	case string:
		// Conservative character-to-token ratio
		tokens := int(math.Ceil(float64(len(val)) / charactersPerToken))
		return max(1, tokens)
	case map[string]any:
		// For nested objects, serialize and estimate
		if jsonBytes, err := utils.MarshalJSON(val, ""); err == nil {
			// Apply complexity multiplier for JSON structure tokenization
			tokens := int(math.Ceil(float64(len(jsonBytes)) / charactersPerToken * jsonComplexityMultiplier))
			return max(minObjectTokens, tokens) // Minimum tokens for objects
		}
		return fallbackObjectTokens // Conservative fallback
	case []any:
		// Arrays - estimate based on serialized size
		if jsonBytes, err := utils.MarshalJSON(val, ""); err == nil {
			// Apply complexity multiplier for JSON array tokenization
			tokens := int(math.Ceil(float64(len(jsonBytes)) / charactersPerToken * jsonComplexityMultiplier))
			return max(minArrayTokens, tokens)
		}
		return len(val) * fallbackArrayTokens // Fallback estimate
	case nil:
		return nullTokens // "null"
	case bool:
		return booleanTokens // "true" or "false"
	case float64, int, int64:
		return numberTokens // Conservative estimate for numbers
	default:
		return unknownTypeTokens // Conservative fallback for unknown types
	}
}
