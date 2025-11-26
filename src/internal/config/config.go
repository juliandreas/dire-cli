package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/errs"
	"github.com/juliandreas/dire/internal/types"
	"github.com/pelletier/go-toml/v2"
)

// Provider represents a single AI provider configuration.
type Provider struct {
	Name        string  `toml:"name"`
	Model       string  `toml:"model,omitempty"`
	MaxTokens   int     `toml:"maxTokens,omitempty"`
	Temperature float64 `toml:"temperature,omitempty"`
	Region      string  `toml:"-"` // Runtime-only: populated from azure-<region> during validation
}

// Config represents the complete application configuration.
type Config struct {
	Project struct {
		Name        string `toml:"name"`
		Description string `toml:"description"`
		Domain      string `toml:"domain,omitempty"`
	} `toml:"project"`

	Files struct {
		Directory string            `toml:"directory"`
		Locales   map[string]string `toml:"locales"`
		Indent    int               `toml:"indent,omitempty"` // Number of spaces for indentation (0 uses default of 2)
	} `toml:"files"`

	Providers struct {
		Configuration []Provider `toml:"configuration,omitempty"`
		Active        string     `toml:"active,omitempty"`
	} `toml:"providers"`

	StyleGuide struct {
		Formality constants.Formality `toml:"formality,omitempty"`
	} `toml:"styleGuide,omitempty"`

	Glossary struct {
		AutoSort bool                  `toml:"autoSort,omitempty"`
		Entries  []types.GlossaryEntry `toml:"entries,omitempty"`
	} `toml:"glossary,omitempty"`

	// Private fields set during validation
	azureRegion string // Extracted Azure region from provider name (azure-<region>)
}

var (
	ErrNoActiveProvider   = errors.New("no active provider specified")
	ErrConfigFileExists   = errors.New("configuration file already exists")
	ErrInvalidConfigPath  = errors.New("invalid config path: contains directory traversal")
	ErrConfigFileNotFound = errors.New("configuration file not found")
)

// DefaultConfigContent is embedded from the template file
//
//go:embed config.template.toml
var DefaultConfigContent string

// GetIndent returns the indentation string based on the configured number of spaces.
// If indent is 0 or not set, returns the default of 2 spaces.
func (c *Config) GetIndent() string {
	spaces := c.Files.Indent
	if spaces <= 0 {
		// Default to 2 spaces
		spaces = 2
	}

	// Build string with the specified number of spaces
	return strings.Repeat(" ", spaces)
}

// GetActiveProviderConfig returns the configuration for the active provider.
func (c *Config) GetActiveProviderConfig() (Provider, error) {
	if c.Providers.Active == "" {
		return Provider{}, ErrNoActiveProvider
	}

	// First, try to find the provider in the configuration array (for LLMs with config)
	for _, provider := range c.Providers.Configuration {
		if provider.Name == c.Providers.Active {
			return provider, nil
		}
	}

	// If not found in configuration array, check if it's a known translator provider
	if constants.IsTranslatorProvider(c.Providers.Active) {
		// Return minimal config for translator providers
		provider := Provider{
			Name:        c.Providers.Active,
			Model:       "", // Not used for translators
			MaxTokens:   0,  // Not used for translators
			Temperature: 0,  // Not used for translators
			Region:      "", // Set below for Azure
		}

		// For Azure, populate the Region field from validation
		if c.Providers.Active == constants.ProviderAzure {
			provider.Region = c.azureRegion
		}

		return provider, nil
	}

	// Provider not found and not a known translator
	return Provider{}, fmt.Errorf("active provider '%s': %w", c.Providers.Active, errs.ErrActiveProviderNotFound)
}

// ResolveLocaleCode converts a file-based locale identifier (e.g., "sv") to the full language code (e.g., "sv-SE")
// used as the locale key in the system.
func (c *Config) ResolveLocaleCode(fileLocale string) types.Locale {
	// Look up the language code for this file-based locale
	filename := fileLocale + ".json"
	if languageCode, exists := c.Files.Locales[filename]; exists {
		return types.Locale(languageCode)
	}

	// Fallback: return the file locale as-is if not found in config
	return types.Locale(fileLocale)
}

// GetFileLocales returns all file-based locale identifiers (without .json extension).
func (c *Config) GetFileLocales() []string {
	locales := make([]string, 0, len(c.Files.Locales))
	for filename := range c.Files.Locales {
		fileLocale := strings.TrimSuffix(filename, ".json")
		locales = append(locales, fileLocale)
	}
	return locales
}

// HasFileLocale checks if a file-based locale identifier exists in the configuration.
func (c *Config) HasFileLocale(fileLocale string) bool {
	filename := fileLocale + ".json"
	_, exists := c.Files.Locales[filename]
	return exists
}

// GetFileNameForLocale returns the file name for a given locale identifier.
func (c *Config) GetFileNameForLocale(localeID string) string {
	// Search through the locale mapping to find the file name
	// Only consider exact filenames, ignore glob patterns (callers expect concrete filenames)
	for fileName, configLocaleID := range c.Files.Locales {
		if configLocaleID == localeID && !strings.ContainsAny(fileName, "*?[") {
			return fileName
		}
	}
	// Fallback if not found in config
	return localeID + ".json"
}

// BuildProjectInfo creates enhanced project context for AI prompts.
func (c *Config) BuildProjectInfo() string {
	var info strings.Builder

	if c.Project.Name != "" {
		info.WriteString(fmt.Sprintf("Project: %s\n", c.Project.Name))
	}
	if c.Project.Description != "" {
		info.WriteString(fmt.Sprintf("Description: %s\n", c.Project.Description))
	}
	if c.Project.Domain != "" {
		info.WriteString(fmt.Sprintf("Domain: %s\n", c.Project.Domain))
	}

	return info.String()
}

// getConfigDirectory returns the appropriate config directory based on environment.
func getConfigDirectory() string {
	env := os.Getenv("DIRE_MODE")
	switch env {
	// Uses "demo" for development environment
	case "development":
		return "demo"
	default:
		return "." // Default to current directory for production
	}
}

// ConfigPath returns the path to the config file.
func ConfigPath(workDir string) string {
	return filepath.Join(workDir, getConfigDirectory(), constants.ConfigFileNameTOML)
}

// CreateConfigFile creates a new configuration file at the specified path.
func CreateConfigFile(path string, force bool) error {
	// Check if file already exists and force flag
	if !force {
		if fileExists(path) {
			return fmt.Errorf("configuration file already exists at %s: %w", path, ErrConfigFileExists)
		}
	}

	// Write the file
	err := os.WriteFile(path, []byte(DefaultConfigContent), constants.SecureFilePermissions)
	if err != nil {
		return fmt.Errorf("failed to create configuration file: %w", err)
	}

	return nil
}

// Load reads the configuration from file.
func Load(configPath string) (*Config, error) {
	// Clean the path and check for directory traversal attacks
	cleanPath := filepath.Clean(configPath)

	// Ensure the cleaned path doesn't escape the intended directory
	if strings.Contains(configPath, "..") || cleanPath != configPath {
		return nil, ErrInvalidConfigPath
	}

	// Read config file
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse TOML
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply glossary sorting if enabled
	if cfg.Glossary.AutoSort {
		if sortedCfg, sortErr := sortGlossaryInFile(cleanPath); sortErr != nil {
			// Don't fail config loading if sorting fails, just log it
			// Note: We return the original config to avoid breaking the app
			_ = sortErr // Acknowledge the error but don't fail
		} else if sortedCfg != nil {
			// Use the sorted config data
			cfg = *sortedCfg
		}
	}

	return &cfg, nil
}

// LocateConfigFile tries to find the TOML config file.
func LocateConfigFile(workDir string) (string, error) {
	configDir := filepath.Join(workDir, getConfigDirectory())
	tomlPath := filepath.Join(configDir, constants.ConfigFileNameTOML)

	if fileExists(tomlPath) {
		return tomlPath, nil
	}

	return "", nil
}

// ApplyDirectoryOverride handles directory path override with expansion and normalization.
func (c *Config) ApplyDirectoryOverride(directory string) {
	dir := strings.TrimSpace(directory)
	if dir == "" {
		return
	}

	// Expand home directory (~) if present
	// Only expand plain "~" or "~/..." patterns, not "~user" or "~subdir"
	if dir == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			dir = home
		}
	} else if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, dir[2:])
		}
	}

	// Expand environment variables
	dir = os.ExpandEnv(dir)

	// Normalize path; allow relative or absolute paths
	c.Files.Directory = filepath.Clean(dir)
}

// ApplyLocaleOverrides handles locale mapping overrides with complete replacement.
func (c *Config) ApplyLocaleOverrides(locales map[string]string) {
	if len(locales) > 0 {
		c.Files.Locales = locales
	}
}

// ApplyIndentOverride handles indent override.
func (c *Config) ApplyIndentOverride(indent int) {
	if indent >= 0 {
		c.Files.Indent = indent
	}
}

// ApplyFormalityOverride handles formality override.
func (c *Config) ApplyFormalityOverride(formality string) {
	formalityValue := strings.ToLower(strings.TrimSpace(formality))
	if formalityValue != "" {
		c.StyleGuide.Formality = constants.Formality(formalityValue)
	}
}

// ApplyProjectNameOverride handles project name override.
func (c *Config) ApplyProjectNameOverride(name string) {
	nameValue := strings.TrimSpace(name)
	if nameValue != "" {
		c.Project.Name = nameValue
	}
}

// ApplyProjectDescriptionOverride handles project description override.
func (c *Config) ApplyProjectDescriptionOverride(description string) {
	descValue := strings.TrimSpace(description)
	if descValue != "" {
		c.Project.Description = descValue
	}
}

// ApplyProjectDomainOverride handles project domain override.
func (c *Config) ApplyProjectDomainOverride(domain string) {
	domainValue := strings.TrimSpace(domain)
	if domainValue != "" {
		c.Project.Domain = domainValue
	}
}

// ApplyProviderActiveOverride handles provider active override.
// Sets which provider is active and ensures it exists in configuration.
func (c *Config) ApplyProviderActiveOverride(name string) {
	nameValue := strings.TrimSpace(name)
	if nameValue == "" {
		return
	}

	// Set the active provider
	c.Providers.Active = nameValue

	// For translator providers (like DeepL), don't create configuration entry
	// They don't need model/maxTokens/temperature - GetActiveProviderConfig handles them
	if constants.IsTranslatorProvider(nameValue) {
		return
	}

	// Ensure provider exists in configuration (for LLM providers only)
	found := false
	for _, p := range c.Providers.Configuration {
		if p.Name == nameValue {
			found = true
			break
		}
	}

	// Create minimal config if provider doesn't exist
	if !found {
		c.Providers.Configuration = append(c.Providers.Configuration, Provider{
			Name:        nameValue,
			Model:       "",
			MaxTokens:   0,
			Temperature: 0,
			Region:      "", // Not used for LLMs
		})
	}
}

// ApplyProviderModelOverride handles provider model override.
// Overrides the model of the active provider.
func (c *Config) ApplyProviderModelOverride(model string) {
	modelValue := strings.TrimSpace(model)
	if modelValue == "" || c.Providers.Active == "" {
		return
	}

	// Find and update the active provider's model
	for i := range c.Providers.Configuration {
		if c.Providers.Configuration[i].Name == c.Providers.Active {
			c.Providers.Configuration[i].Model = modelValue
			return
		}
	}
}

// ApplyProviderTemperatureOverride handles provider temperature override.
// Overrides the temperature of the active provider.
func (c *Config) ApplyProviderTemperatureOverride(temperature float64) {
	if temperature < 0 || c.Providers.Active == "" {
		return
	}

	// Find and update the active provider's temperature
	for i := range c.Providers.Configuration {
		if c.Providers.Configuration[i].Name == c.Providers.Active {
			c.Providers.Configuration[i].Temperature = temperature
			return
		}
	}
}

// ApplyProviderMaxTokensOverride handles provider max tokens override.
// Overrides the max tokens of the active provider.
func (c *Config) ApplyProviderMaxTokensOverride(maxTokens int) {
	if maxTokens <= 0 || c.Providers.Active == "" {
		return
	}

	// Find and update the active provider's max tokens
	for i := range c.Providers.Configuration {
		if c.Providers.Configuration[i].Name == c.Providers.Active {
			c.Providers.Configuration[i].MaxTokens = maxTokens
			return
		}
	}
}

// fileExists checks if a file exists at the given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
