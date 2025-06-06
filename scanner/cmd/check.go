/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"os/exec"

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
		Bool("grype", isInstalled("grype")).
		Bool("syft", isInstalled("syft")).
		Bool("trivy", isInstalled("trivy")).
		Msg("check dependencies")

	errors := 0
	for _, name := range requiredPrograms {
		if !isInstalled(name) {
			errors += 1
			logger.Error().Msg("Not found: " + name)
		}
	}
	if errors > 0 {
		os.Exit(1)
	}
	logger.Info().Msg("Success")

}

func isInstalled(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
