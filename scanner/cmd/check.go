/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/metraction/pharos/internal/utils"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// define command line arguments
func init() {
	rootCmd.AddCommand(checkCmd)
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check config and dependencies",
	Long:  `Check local envronment config and dependencies`,

	Run: func(cmd *cobra.Command, args []string) {
		ExecuteCheck(logger)
	},
}

// execute command
func ExecuteCheck(logger *zerolog.Logger) {

	// required executables
	requiredPrograms := []string{"syft", "grype"}

	logger.Info().Msg("-----< Scanner Check >-----")

	logger.Info().
		Bool("grype", utils.IsInstalled("grype")).
		Bool("syft", utils.IsInstalled("syft")).
		Bool("trivy", utils.IsInstalled("trivy")).
		Msg("check dependencies")

	errors := 0
	for _, name := range requiredPrograms {
		if !utils.IsInstalled(name) {
			errors += 1
			logger.Error().Msg("Not found: " + name)
		}
		if path, err := utils.OsWhich(name); err == nil {
			logger.Info().Str("name", name).Str("path", path).Msg("")
		}
	}
	if errors > 0 {
		os.Exit(1)
	}
	logger.Info().Msg("Success")
}
