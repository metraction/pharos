/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/internal/utils"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of root command
type RootArgsType = struct {
	LogType  string
	LogLevel string
}

var EnvOrDefault = utils.EnvOrDefaultFunc("PHAROS") // return function that return envvar <PREFIX>_name or given default value
var RootArgs = RootArgsType{}
var logger *zerolog.Logger

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "scanner",
	Short: "Pharos scanner",
	Long:  `Pharos scanner (using grype)`,

	// Uncomment the following line if your bare application has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.

func Execute() {

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	logger = logging.NewLogger("info")

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.

	// Persistent flags

	rootCmd.PersistentFlags().StringVar(&RootArgs.LogType, "logtype", EnvOrDefault("logtype", "console"), "Log output format [console,json]")
	rootCmd.PersistentFlags().StringVar(&RootArgs.LogLevel, "loglevel", EnvOrDefault("loglevel", "info"), "Loglevel [debug,info,warn,error]")

	// Local flags, which will only run when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
