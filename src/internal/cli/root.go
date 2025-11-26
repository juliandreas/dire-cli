// Package cli contains CLI command implementations for the application.
package cli

import (
	"fmt"

	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/flags"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	var rootCmd cobra.Command
	rootCmd.Use = constants.AppName
	rootCmd.Short = "AI-powered translation tool for i18n files"
	rootCmd.Version = constants.AppVersion
	rootCmd.Long = "Go-based CLI that automatically processes missing translations in your i18n JSON files " +
		"using translation and AI providers, glossary definitions, and translation memory"
	rootCmd.Example = `  dire init                               # Create configuration file
  dire                                    # Translate all missing keys
  dire --keys auth.login                  # Translate specific key value or object
  dire --sourced                          # Only apply glossary and memory translations
  dire --glossary                         # Only apply glossary translations
  dire --memory                           # Only apply memory translations
  dire --stub                             # Create placeholder translations with empty strings
  dire --no-trim                          # Preserve whitespace in translations
  dire --context "Dashboard UI"           # Add context for better AI translations`
	rootCmd.SilenceUsage = true  // Don't show usage on errors
	rootCmd.SilenceErrors = true // We handle errors in main
	rootCmd.RunE = RunCommand

	// Add all flags
	flags.AddToCommand(&rootCmd)

	// Initialize logging
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		flags.HandleLoggingFlags(cmd)
	}

	rootCmd.AddCommand(NewInitCommand())
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetVersionTemplate(fmt.Sprintf("%s %s\n", constants.AppName, constants.AppVersion))

	return &rootCmd
}

// Execute is the main entry point for the CLI.
func Execute() error {
	loadEnvFiles()

	return NewRootCommand().Execute()
}
