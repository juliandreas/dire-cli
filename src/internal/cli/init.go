package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/juliandreas/dire/internal/config"
	"github.com/juliandreas/dire/internal/constants"
	"github.com/juliandreas/dire/internal/flags"
	"github.com/spf13/cobra"
)

// NewInitCommand creates and returns the init command.
func NewInitCommand() *cobra.Command {
	var cmd cobra.Command
	cmd.Use = "init"
	cmd.Short = "Create configuration file"
	cmd.Long = "Create a new .dire.toml configuration file in the current directory."
	cmd.RunE = runInit

	// Add flags
	flags.AddInitFlags(&cmd)

	return &cmd
}

func runInit(cmd *cobra.Command, _ []string) error {
	// Get the force flag value from the command.
	force, err := flags.GetForceFlag(cmd)
	if err != nil {
		return fmt.Errorf("failed to get force flag: %w", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	configPath := config.ConfigPath(workDir)

	// Ensure the directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, constants.SecureDirPermissions); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", configDir, err)
	}

	// Create config file
	if err := config.CreateConfigFile(configPath, force); err != nil {
		return err
	}

	file := filepath.Base(configPath)
	fmt.Printf("%s Config file created: %s\n", constants.Yellow().Render("✦"), file)
	fmt.Println("Now edit it to match your project and you're good to go!")
	return nil
}
