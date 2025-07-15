package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/metraction/pharos/pkg/enricher"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams/extension"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
		config.EnricherPath = enricherPath

		// Check if the enricher file exists in the specified path
		enricherFilePath := filepath.Join(enricherPath, "enricher.yaml")
		if _, err := os.Stat(enricherFilePath); os.IsNotExist(err) {
			fmt.Printf("Error: Enricher file not found at %s\n", enricherFilePath)
			return
		}

		// Get the data file path from flag or use default
		dataFile, _ := cmd.Flags().GetString("data")
		dataFilePath := dataFile
		// If the data file is not an absolute path, join it with the enricher path
		if !filepath.IsAbs(dataFile) {
			dataFilePath = filepath.Join(enricherPath, dataFile)
		}
		fmt.Printf("Using data file: %s\n", dataFilePath)

		// Load test result
		testResult, err := model.LoadResultFromFile(dataFilePath)
		if err != nil {
			fmt.Printf("Error loading test result from %s: %v\n", dataFilePath, err)
			return
		}

		inputChannel := make(chan any, 1)
		inputChannel <- *testResult
		close(inputChannel)

		// Load the plugin
		plugin := enricher.LoadPlugin(config, extension.NewChanSource(inputChannel)).Out()
		result := (<-plugin).(model.PharosScanResult)

		out, err := yaml.Marshal(result.Image.ContextRoots)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Result:\n%v", string(out))
	},
}

func init() {
	rootCmd.AddCommand(pluginCmd)

	// Add subcommands to the plugin command
	pluginCmd.AddCommand(testCmd)

	// Add flags specific to the test subcommand
	testCmd.Flags().String("data", "test-data.yaml", "Path to test data file to use instead of test-data.yaml")
}
