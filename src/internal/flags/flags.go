// Package flags contains runtime CLI flag state for the application.
package flags

import (
	"strings"

	"github.com/juliandreas/dire/internal/logger"
	"github.com/spf13/cobra"
)

// Flags holds runtime state for CLI flags during a single execution.
type Flags struct {
	// File processing flags
	TrimStrings bool // Whether to trim whitespace from translation values

	// Translation behavior flags
	StubMode     bool   // Create stub translations with empty strings
	IncludeStubs bool   // Include empty strings as missing translations
	SourcedOnly  bool   // Only apply glossary/memory translations (skip external services)
	GlossaryOnly bool   // Only apply glossary translations (skip external services)
	MemoryOnly   bool   // Only apply memory translations (skip external services)
	Overwrite    bool   // Overwrite existing translations with glossary values (requires --glossary)
	Rephrase     bool   // Rephrase existing translations instead of creating new ones
	Check        bool   // Lint translation completeness (CI/CD mode)
	Format       string // Output format for lint results (use with --check)
	Context      string // Additional context for translations
	Prune        bool   // Remove keys from non-reference locales that don't exist in reference locale

	// Filtering flags
	Keys []string // Specific keys to translate (comma-separated, single key for rephrase mode)

	// Rephrase-specific flags
	Locale string // Single locale for rephrase generation

	// Config override flags
	Directory   string            // Override config files.directory setting
	Locales     map[string]string // Override config files.locales mapping
	Indent      int               // Override config files.indent setting
	Formality   string            // Override config styleGuide.formality setting
	ProjectName string            // Override config project.name setting
	ProjectDesc string            // Override config project.description setting
	ProjectDom  string            // Override config project.domain setting

	// Provider override flags
	ProviderActive      string  // Override config providers.active setting
	ProviderModel       string  // Override model of active provider
	ProviderTemperature float64 // Override temperature of active provider
	ProviderMaxTokens   int     // Override maxTokens of active provider

	// Derived flags (computed from other flags)
	RequiresTranslationService bool // Whether a translation service (LLM or translator) is needed
	IsLogging                  bool // Whether logging is enabled (verbose or debug mode)
}

// New creates a new flags instance with default values.
func New() *Flags {
	return &Flags{
		TrimStrings:                true,  // Default behavior is to trim
		StubMode:                   false, // Default is not stub mode
		IncludeStubs:               false, // Default is not to include stubs
		SourcedOnly:                false, // Default is to use external services (LLM/translator) when needed
		GlossaryOnly:               false, // Default is not glossary-only mode
		MemoryOnly:                 false, // Default is not memory-only mode
		Overwrite:                  false, // Default is not to overwrite existing translations
		Rephrase:                   false, // Default is not to rephrase existing translations
		Check:                      false, // Default is not lint mode
		Format:                     "",    // Default is human-readable output
		Context:                    "",    // Default is no additional context
		Prune:                      false, // Default is not to prune orphaned keys
		Keys:                       nil,   // Default is no key filtering
		Locale:                     "",    // Default is no single locale
		Directory:                  "",    // Default is no directory override
		Locales:                    nil,   // Default is no locale overrides
		Indent:                     -1,    // Default is no indent override (-1 means use config value)
		Formality:                  "",    // Default is no formality override
		ProjectName:                "",    // Default is no project name override
		ProjectDesc:                "",    // Default is no project description override
		ProjectDom:                 "",    // Default is no project domain override
		ProviderActive:             "",    // Default is no provider active override
		ProviderModel:              "",    // Default is no provider model override
		ProviderTemperature:        -1,    // Default is no provider temperature override (-1 means use config value)
		ProviderMaxTokens:          -1,    // Default is no provider max tokens override (-1 means use config value)
		RequiresTranslationService: true,  // Default assumes translation service will be needed
		IsLogging:                  false, // Default is not in logging mode
	}
}

// UpdateDerivedFlags computes derived flags based on the current flag values.
func (f *Flags) UpdateDerivedFlags() {
	f.RequiresTranslationService = !f.StubMode && !f.SourcedOnly && !f.GlossaryOnly && !f.MemoryOnly && !f.Check
}

// AddToCommand adds all translation-related flags to a cobra command.
func AddToCommand(cmd *cobra.Command) {
	// Translation behavior flags
	cmd.Flags().StringSlice("keys", nil, "specific keys to translate")
	cmd.Flags().String("locale", "", "single locale for rephrase generation")
	cmd.Flags().Bool("stub", false, "create stub translations with empty strings")
	cmd.Flags().Bool("include-stubs", false, "include stub translations (empty strings) in translation process")
	cmd.Flags().Bool("sourced", false, "only apply sourced translations (glossary and memory); skip external services")
	cmd.Flags().Bool("glossary", false, "only apply glossary translations; skip external services")
	cmd.Flags().Bool("memory", false, "only apply memory translations; skip external services")
	cmd.Flags().Bool("overwrite", false, "overwrite existing translations with glossary values (requires --glossary)")
	cmd.Flags().Bool("rephrase", false, "generate rephrase options (requires single --keys and --locale)")
	cmd.Flags().Bool("check", false, "lint translation completeness and exit with status code")
	cmd.Flags().String("format", "", "output format for lint results: default, json (use with --check)")
	cmd.Flags().String("context", "", "additional context for translations")
	cmd.Flags().Bool("prune", false, "remove keys from non-reference locales that don't exist in reference locale")

	// Config override flags
	cmd.Flags().String("directory", "", "override files.directory from config file")
	cmd.Flags().StringToString("locales", nil, "override locale mapping (filename=language-code)")
	cmd.Flags().Int("indent", -1, "override files.indent from config file (number of spaces, 0 uses default of 2)")
	cmd.Flags().String("formality", "", "override styleGuide.formality from config file (automatic, formal, informal)")
	cmd.Flags().String("project-name", "", "override project.name from config file")
	cmd.Flags().String("project-description", "", "override project.description from config file")
	cmd.Flags().String("project-domain", "", "override project.domain from config file")
	cmd.Flags().String("provider-active", "", "override providers.active from config file")
	cmd.Flags().String("provider-model", "", "override model of active provider")
	cmd.Flags().Float64("provider-temperature", -1, "override temperature of active provider")
	cmd.Flags().Int("provider-max-tokens", -1, "override max tokens of active provider")

	// File processing flags
	cmd.Flags().Bool("no-trim", false, "preserve leading/trailing whitespace in translation values")

	// Logging flags
	cmd.PersistentFlags().Bool("verbose", false, "show detailed progress information")
	cmd.PersistentFlags().Bool("debug", false, "enable debug logging (includes all verbose output plus debug details)")
}

// AddInitFlags adds flags specific to the init command.
func AddInitFlags(cmd *cobra.Command) {
	cmd.Flags().BoolP("force", "f", false, "overwrite existing config")
}

// HandleLoggingFlags extracts and processes the verbose and debug flags to initialize logging.
func HandleLoggingFlags(cmd *cobra.Command) {
	verbose, _ := GetVerboseFlag(cmd)
	debug, _ := GetDebugFlag(cmd)
	logger.Initialize(verbose, debug)
}

// GetVerboseFlag extracts the verbose flag value from a command.
func GetVerboseFlag(cmd *cobra.Command) (bool, error) {
	return cmd.PersistentFlags().GetBool("verbose")
}

// GetDebugFlag extracts the debug flag value from a command.
func GetDebugFlag(cmd *cobra.Command) (bool, error) {
	return cmd.PersistentFlags().GetBool("debug")
}

// GetForceFlag extracts the force flag value from a command.
func GetForceFlag(cmd *cobra.Command) (bool, error) {
	return cmd.Flags().GetBool("force")
}

// GetSourcedFlag extracts the sourced flag value from a command.
func GetSourcedFlag(cmd *cobra.Command) (bool, error) {
	return cmd.Flags().GetBool("sourced")
}

// ParseFromCobraCommand creates a Flags instance from cobra command flags.
// It extracts all translation-related flags, populates a Flags struct.
//
//nolint:funlen // Length is acceptable due to large number of flags to parse
func ParseFromCobraCommand(cmd *cobra.Command) (*Flags, error) {
	// Extract persistent flags (verbose/debug for logging configuration)
	verbose, _ := GetVerboseFlag(cmd)
	debug, _ := GetDebugFlag(cmd)

	// Extract flags from cobra command
	keys, _ := cmd.Flags().GetStringSlice("keys")
	locale, _ := cmd.Flags().GetString("locale")
	stub, _ := cmd.Flags().GetBool("stub")
	includeStubs, _ := cmd.Flags().GetBool("include-stubs")
	sourced, _ := cmd.Flags().GetBool("sourced")
	glossary, _ := cmd.Flags().GetBool("glossary")
	memory, _ := cmd.Flags().GetBool("memory")
	overwrite, _ := cmd.Flags().GetBool("overwrite")
	rephrase, _ := cmd.Flags().GetBool("rephrase")
	check, _ := cmd.Flags().GetBool("check")
	rawFormat, _ := cmd.Flags().GetString("format")
	noTrim, _ := cmd.Flags().GetBool("no-trim")
	context, _ := cmd.Flags().GetString("context")
	prune, _ := cmd.Flags().GetBool("prune")
	directory, _ := cmd.Flags().GetString("directory")
	locales, _ := cmd.Flags().GetStringToString("locales")
	indent, _ := cmd.Flags().GetInt("indent")
	formality, _ := cmd.Flags().GetString("formality")
	projectName, _ := cmd.Flags().GetString("project-name")
	projectDesc, _ := cmd.Flags().GetString("project-description")
	projectDom, _ := cmd.Flags().GetString("project-domain")
	providerActive, _ := cmd.Flags().GetString("provider-active")
	providerModel, _ := cmd.Flags().GetString("provider-model")
	providerTemperature, _ := cmd.Flags().GetFloat64("provider-temperature")
	providerMaxTokens, _ := cmd.Flags().GetInt("provider-max-tokens")

	// Create and populate flags instance
	flagSet := New()
	flagSet.TrimStrings = !noTrim
	flagSet.StubMode = stub
	flagSet.IncludeStubs = includeStubs
	flagSet.SourcedOnly = sourced
	flagSet.GlossaryOnly = glossary
	flagSet.MemoryOnly = memory
	flagSet.Overwrite = overwrite
	flagSet.Rephrase = rephrase
	flagSet.Check = check
	flagSet.Format = strings.ToLower(strings.TrimSpace(rawFormat))
	flagSet.Context = context
	flagSet.Prune = prune
	flagSet.Keys = keys
	flagSet.Locale = locale
	flagSet.Directory = directory
	flagSet.Locales = locales
	flagSet.Indent = indent
	flagSet.Formality = formality
	flagSet.ProjectName = projectName
	flagSet.ProjectDesc = projectDesc
	flagSet.ProjectDom = projectDom
	flagSet.ProviderActive = providerActive
	flagSet.ProviderModel = providerModel
	flagSet.ProviderTemperature = providerTemperature
	flagSet.ProviderMaxTokens = providerMaxTokens

	// Compute derived flags
	flagSet.IsLogging = verbose || debug
	flagSet.UpdateDerivedFlags()

	return flagSet, nil
}
