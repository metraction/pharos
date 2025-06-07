/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// define command line arguments
func init() {
	rootCmd.AddCommand(updateCmd)
}

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update scanner vulnerability database",
	Long:  `Check external scanner vulnerability database, update local database if required`,
	Run: func(cmd *cobra.Command, args []string) {
		ExecuteUpdate(logger)
	},
}

// execute command
func ExecuteUpdate(logger *zerolog.Logger) {

	// required executables

	logger.Info().Msg("-----< Scanner Update >-----")
}
