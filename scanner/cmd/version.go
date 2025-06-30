/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/scanner/version"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Scanner version",
	Long:  `Scanner display version.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger = logging.NewLogger(RootArgs.LogLevel)

		fmt.Printf("Pharos scanner %s (%s, %s)\n", version.Version, version.BuildTimestamp, version.GoVersion)
	},
}
