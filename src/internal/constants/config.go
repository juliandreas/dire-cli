package constants

// Formality represents the formality level for translations.
type Formality string

const (
	// FormalityAutomatic represents automatic formality detection, matching the source text.
	FormalityAutomatic Formality = "automatic"

	// FormalityFormal represents formal language and respectful tone (Sie/vous/usted/etc.)
	FormalityFormal Formality = "formal"

	// FormalityInformal represents casual, friendly language (du/tu/tú/etc.)
	FormalityInformal Formality = "informal"
)
