package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/metraction/pharos/pkg/enricher"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams"
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

var dataFilePath string

// testCmd represents the test subcommand of the plugin command
var testCmd = &cobra.Command{
	Use:   "test [uri]",
	Short: "Test plugin functionality",
	// TODO check is config.EnricherPath is still needed
	Long: `
	pharos plugin test [uri]
	   Where uri could point to enrichers.yaml file or directory.
	   In case of enrichers.yaml and relative path values - it will be resolved relarive to enrichers.yaml file.
	   If it is directory it expectes to find only one enricher in it.
	`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running plugin test...")
		// Get config from context
		config := cmd.Context().Value("config").(*model.Config)

		// Get the URI from args or use default path
		var enricherPath string
		if len(args) > 0 && args[0] != "" {
			enricherPath = args[0]
		} else {
			enricherPath = config.EnricherPath
		}

		var enrichers *model.Enrichers
		var err error
		// Check if args[0] points to enrichers.yaml file
		if filepath.Base(args[0]) == "enrichers.yaml" {
			fmt.Printf("Loading Enrichers from file: %s\n", args[0])
			enrichers, err = model.LoadEnrichersFromFile(args[0])
			if err != nil {
				fmt.Printf("Error loading Enrichers from %s: %v\n", args[0], err)
				return
			}
			fmt.Printf("Successfully loaded Enrichers with %d order items and %d sources\n",
				len(enrichers.Order), len(enrichers.Sources))
		} else {
			fmt.Printf("Loading Enricher from directory: %s\n", args[0])
			enrichers = &model.Enrichers{
				Order: []string{"result"},
				Sources: []model.EnricherSource{
					{
						Name: "result",
						Path: args[0],
					},
				},
			}
		}

		// Get the data file path from flag or use default
		if !filepath.IsAbs(dataFilePath) {
			dataFilePath = filepath.Join(enricherPath, dataFilePath)
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

		var plugin streams.Source = extension.NewChanSource(inputChannel)
		for _, source := range enrichers.Sources {
			// Load the plugin
			var enricherPath string
			if source.Git != "" {
				enricherPath, err = enricher.FetchEnricherFromGit(source.Git, enricherPath)
				if err != nil {
					fmt.Printf("Error loading enricher from Git: %v\n", err)
					return
				}
			} else if source.Path != "" {
				enricherPath = source.Path
			}
			enricherPath = refineEnricherPath(config, enricherPath)

			plugin = enricher.LoadEnricher(enricherPath, source.Name, plugin)
		}
		if len(enrichers.Sources) == 0 {
			fmt.Println("No sources found")
			return
		}
		result := (<-(plugin.(streams.Flow)).Out()).(model.PharosScanResult)

		out, err := yaml.Marshal(result.Image.ContextRoots)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println(string(out))

	},
}

func init() {
	rootCmd.AddCommand(pluginCmd)

	// Add subcommands to the plugin command
	pluginCmd.AddCommand(testCmd)

	// Add flags specific to the test subcommand
	testCmd.Flags().StringVar(&dataFilePath, "data", "test-data.yaml", "Path to test data file to use instead of test-data.yaml")
}

func refineEnricherPath(config *model.Config, enricherPath string) string {
	// If enricherPath is absolute, return it as is
	if filepath.IsAbs(enricherPath) {
		return enricherPath
	}

	return filepath.Join(config.BasePath, enricherPath)
}
