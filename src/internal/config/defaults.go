package config

import (
	"github.com/juliandreas/dire/internal/constants"
)

// ResolvedDefaults contains all resolved default values.
type ResolvedDefaults struct {
	Formality constants.Formality
}

// ResolveDefaults takes a config and returns all resolved default values.
func ResolveDefaults(cfg *Config) ResolvedDefaults {
	return ResolvedDefaults{
		Formality: getFormality(cfg),
	}
}

// getFormality returns the formality setting, using default if not specified.
func getFormality(cfg *Config) constants.Formality {
	if cfg != nil && cfg.StyleGuide.Formality != "" {
		return cfg.StyleGuide.Formality
	}
	return constants.FormalityAutomatic
}
