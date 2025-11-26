package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/juliandreas/dire/internal/cli"
	"github.com/juliandreas/dire/internal/constants"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\r\n", err)

		// Map specific errors to distinct exit codes
		switch {
		case errors.Is(err, cli.ErrMissingTranslations):
			os.Exit(constants.ExitCodeWarning)
		case errors.Is(err, cli.ErrCheckFailed):
			os.Exit(constants.ExitCodeError)
		case errors.Is(err, cli.ErrRephraseInvalidArgs):
			os.Exit(constants.ExitCodeError)
		default:
			os.Exit(constants.ExitCodeError)
		}
	}
}
