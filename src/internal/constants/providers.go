package constants

import (
	"slices"
	"sort"
	"strings"
)

const (
	ProviderClaude   = "claude"
	ProviderOpenAI   = "openai"
	ProviderGemini   = "gemini"
	ProviderMistral  = "mistral"
	ProviderDeepSeek = "deepseek"
	ProviderDeepL    = "deepl"
	ProviderGoogle   = "google"
	ProviderAzure    = "azure"
)

// getLLMProviders returns AI language models that may hallucinate or modify structure.
func getLLMProviders() []string {
	return []string{
		ProviderClaude,
		ProviderOpenAI,
		ProviderGemini,
		ProviderMistral,
		ProviderDeepSeek,
	}
}

// getTranslatorProviders returns pure translation APIs that maintain exact structure.
func getTranslatorProviders() []string {
	return []string{
		ProviderDeepL,
		ProviderGoogle,
		ProviderAzure,
	}
}

// getProviderEnvKeyMap returns a map of provider names to their environment variable keys.
func getProviderEnvKeyMap() map[string]string {
	return map[string]string{
		ProviderClaude:   "DIRE_CLAUDE_API_KEY",
		ProviderOpenAI:   "DIRE_OPENAI_API_KEY",
		ProviderGemini:   "DIRE_GEMINI_API_KEY",
		ProviderMistral:  "DIRE_MISTRAL_API_KEY",
		ProviderDeepSeek: "DIRE_DEEPSEEK_API_KEY",
		ProviderDeepL:    "DIRE_DEEPL_API_KEY",
		ProviderGoogle:   "DIRE_GOOGLE_API_KEY",
		ProviderAzure:    "DIRE_AZURE_API_KEY",
	}
}

// SupportedProviders returns all supported provider names.
func SupportedProviders() []string {
	providerEnvKeys := getProviderEnvKeyMap()
	providers := make([]string, 0, len(providerEnvKeys))
	for name := range providerEnvKeys {
		providers = append(providers, name)
	}
	sort.Strings(providers)
	return providers
}

// IsAzureProvider checks if user input matches Azure provider pattern (azure-<region>).
// Only used during config validation before normalization to "azure".
// Returns true for "azure-global", "azure-swedencentral", "azure-eastus", etc.
func IsAzureProvider(name string) bool {
	return strings.HasPrefix(name, "azure-")
}

// IsValidProvider checks if a provider name is supported.
// Accepts both normalized names ("azure") and user input format ("azure-<region>").
func IsValidProvider(name string) bool {
	if IsAzureProvider(name) {
		return true
	}

	providerEnvKeys := getProviderEnvKeyMap()
	_, exists := providerEnvKeys[name]
	return exists
}

// GetEnvKeyForProvider returns the environment variable key for a provider.
func GetEnvKeyForProvider(providerName string) string {
	if IsAzureProvider(providerName) {
		providerName = ProviderAzure
	}

	providerEnvKeys := getProviderEnvKeyMap()
	return providerEnvKeys[providerName]
}

// IsLLMProvider checks if a provider is an LLM that may need validation.
func IsLLMProvider(providerName string) bool {
	return slices.Contains(getLLMProviders(), providerName)
}

// IsTranslatorProvider checks if a provider is a pure translation service.
func IsTranslatorProvider(providerName string) bool {
	return slices.Contains(getTranslatorProviders(), providerName)
}

func IsTranslationService(providerName string) bool {
	return IsLLMProvider(providerName) || IsTranslatorProvider(providerName)
}
