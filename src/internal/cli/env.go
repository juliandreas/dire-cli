package cli

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/joho/godotenv"
	"github.com/juliandreas/dire/internal/logger"
)

// loadEnvFiles automatically discovers and loads all .env* files in priority order.
// Files are loaded from highest to lowest precedence using godotenv.Load,
// so OS env vars are never overridden and higher-precedence files set values first.
// Precedence (high → low): .env.{name}.local > .env.local > .env.{name} > .env.
func loadEnvFiles() {
	const baseEnvFile = ".env"
	envFiles, err := filepath.Glob(".env*")
	if err != nil {
		// Non-fatal, but useful when debugging
		logger.Debug("Failed to glob .env files", "error", err)
		return
	}

	// Filter to only accept exact ".env" or files with prefix ".env." and exclude any ".example" files
	var filteredFiles []string
	for _, file := range envFiles {
		if (file == baseEnvFile || strings.HasPrefix(file, baseEnvFile+".")) && !strings.HasSuffix(file, ".example") {
			filteredFiles = append(filteredFiles, file)
		}
	}
	envFiles = filteredFiles

	// Sort files by priority: highest precedence first
	sort.Slice(envFiles, func(idx1, idx2 int) bool {
		weight1 := getEnvFileWeight(envFiles[idx1], baseEnvFile)
		weight2 := getEnvFileWeight(envFiles[idx2], baseEnvFile)

		// Sort by weight first (higher weight = higher priority, should come first)
		if weight1 != weight2 {
			return weight1 > weight2
		}

		// For same weight, sort alphabetically for deterministic ordering
		return envFiles[idx1] < envFiles[idx2]
	})

	// Load all discovered .env files in precedence order (high → low)
	for _, file := range envFiles {
		if err := godotenv.Load(file); err != nil {
			// The file exists (we discovered it), so errors here are likely parse/permission issues.
			// Warn rather than fail hard.
			logger.Warn("Failed to load .env file", "file", file, "error", err)
		}
	}
}

// getEnvFileWeight assigns priority weights to .env files for deterministic sorting.
// Higher weight = higher priority. Weights: .env=0, .env.<name>=1, .env.local=2, .env.<name>.local=3.
func getEnvFileWeight(filename, baseEnvFile string) int {
	const (
		baseEnvWeight    = 0 // .env has lowest priority
		namedEnvWeight   = 1 // .env.<name> files
		localEnvWeight   = 2 // .env.local
		namedLocalWeight = 3 // .env.<name>.local has highest priority
	)

	if filename == baseEnvFile {
		return baseEnvWeight
	}

	if strings.HasSuffix(filename, ".local") {
		if filename == baseEnvFile+".local" {
			return localEnvWeight
		}
		return namedLocalWeight
	}

	return namedEnvWeight
}
