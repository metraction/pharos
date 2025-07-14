package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/metraction/pharos/pkg/enricher"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams/extension"
	"github.com/spf13/cobra"
)

// pluginCmd represents the plugin command
var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Plugin management commands",
	Long:  `Commands for managing and interacting with Pharos plugins.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, print help
		cmd.Help()
	},
}

// testCmd represents the test subcommand of the plugin command
var testCmd = &cobra.Command{
	Use:   "test [uri]",
	Short: "Test plugin functionality",
	Long: `Test the functionality of installed plugins or plugin configurations.

The test command accepts a URI parameter that points to a directory where it expects to find an enricher file.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running plugin test...")
		// Get config from context
		config := cmd.Context().Value("config").(*model.Config)

		// Get the URI from args or use default path
		enricherPath := config.EnricherPath
		if len(args) > 0 && args[0] != "" {
			enricherPath = args[0]
			fmt.Printf("Using custom enricher path: %s\n", enricherPath)
		} else {
			fmt.Printf("Using default enricher path: %s\n", enricherPath)
		}

		// Check if the enricher file exists in the specified path
		enricherFilePath := filepath.Join(enricherPath, "enricher.yaml")
		if _, err := os.Stat(enricherFilePath); os.IsNotExist(err) {
			fmt.Printf("Error: Enricher file not found at %s\n", enricherFilePath)
			return
		}

		resultChannel := make(chan any, 1)

		// Load the plugin
		plugin := enricher.LoadPlugin(config, extension.NewChanSource(resultChannel)).Out()
		result := <-plugin
		fmt.Printf("Result: %v\n", result)

	},
}

func init() {
	rootCmd.AddCommand(pluginCmd)

	// Add subcommands to the plugin command
	pluginCmd.AddCommand(testCmd)

	// Add flags specific to the test subcommand if needed
	// testCmd.Flags().StringVar(&someVar, "flag-name", "default", "Description")
}
