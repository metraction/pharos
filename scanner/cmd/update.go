/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"time"

	"github.com/metraction/pharos/internal/scanner/grype"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of root command
type UpdateArgsType = struct {
	ScanEngine  string // scan engine to use
	ScanTimeout string // database update timeout

}

var UpdateArgs = UpdateArgsType{}

// define command line arguments
func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().StringVar(&UpdateArgs.ScanEngine, "engine", EnvOrDefault("engine", "trivy"), "Scan engine to use [grype,trivy]")
	updateCmd.Flags().StringVar(&UpdateArgs.ScanTimeout, "scan_timeout", EnvOrDefault("scan_timeout", "180s"), "Scan timeout")

}

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update scanner vulnerability database",
	Long:  `Check and update scanner vulnerability database`,
	Run: func(cmd *cobra.Command, args []string) {

		ExecuteUpdate(logger)
	},
}

// execute command
func ExecuteUpdate(logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scanner Update >-----")

	vulnScanner, err := grype.NewGrypeScanner(1*time.Second, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("NewGrypeScanner()")
	}

	err = vulnScanner.UpdateDatabase()
	if err != nil {
		logger.Fatal().Err(err).Msg("UpdateDatabase()")
	}

}
