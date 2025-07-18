package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/metraction/pharos/pkg/enricher"
	"github.com/metraction/pharos/pkg/mappers"
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

		var enrichers *model.Enrichers
		var err error

		var enrichersPath string
		if len(args) == 0 {
			fmt.Println("No arguments provided. Using default enrichers.")
			enrichersPath = config.EnricherPath
		} else {
			enrichersPath = args[0]
		}

		enrichers, err = loadEnrichersFromDirOrFile(enrichersPath)
		if err != nil {
			fmt.Printf("Error loading enrichers from %s: %v\n", enrichersPath, err)
			return
		}
		if len(enrichers.Sources) == 0 {
			fmt.Println("No sources found")
			return
		}

		// Load test result
		fmt.Printf("Using data file: %s\n", dataFilePath)
		testResult, err := model.LoadResultFromFile(dataFilePath)
		if err != nil {
			fmt.Printf("Error loading test result from %s: %v\n", dataFilePath, err)
			return
		}

		inputChannel := make(chan any, 1)
		inputChannel <- *testResult
		close(inputChannel)

		plugin := createEnrichersFlow(extension.NewChanSource(inputChannel), enrichers)

		result := (<-(plugin.(streams.Flow)).Out()).(model.PharosScanResult)

		out, err := yaml.Marshal(result.Image.ContextRoots)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println(string(out))

	},
}

func loadEnrichersFromDirOrFile(enrichersPath string) (*model.Enrichers, error) {
	var enrichers *model.Enrichers
	var err error
	// Check if args[0] points to enrichers.yaml file
	if filepath.Base(enrichersPath) == "enrichers.yaml" {
		logger.Info().Msgf("Loading Enrichers from file: %s\n", enrichersPath)
		enrichers, err = model.LoadEnrichersFromFile(enrichersPath)
		if err != nil {
			logger.Error().Msgf("Error loading Enrichers from %s: %v\n", enrichersPath, err)
			return nil, err
		}
		logger.Info().Msgf("Successfully loaded Enrichers with %d order items and %d sources\n",
			len(enrichers.Order), len(enrichers.Sources))
	} else {
		logger.Info().Msgf("Loading Enricher from directory: %s\n", enrichersPath)
		enrichers = &model.Enrichers{
			Order: []string{"result"},
			Sources: []model.EnricherSource{
				{
					Name: "results",
					Path: enrichersPath,
				},
			},
		}
	}
	return enrichers, nil
}

func createEnrichersFlow(plugin streams.Source, enrichers *model.Enrichers) streams.Flow {
	for _, source := range enrichers.Sources {
		// Load the plugin
		var enricherPath string
		if source.Git != "" {
			tempDir, err := os.MkdirTemp("", "pharos-enricher-*")
			if err != nil {
				logger.Error().Msgf("Error creating temporary directory: %v\n", err)
				return nil
			}

			enricherPath, err = enricher.FetchEnricherFromGit(source.Git, tempDir)
			if err != nil {
				logger.Error().Msgf("Error loading enricher from Git: %v\n", err)
				return nil
			}
		} else if source.Path != "" {
			enricherPath = source.Path
		}
		enricherPath = addBasePathToRelative(config, enricherPath)

		enricherConfig := enricher.LoadEnricher(enricherPath, source.Name)
		plugin = mappers.NewResultEnricherStream(plugin, source.Name, enricherConfig)
	}
	return plugin.(streams.Flow)
}

// pluginRunCmd represents the run subcommand of the plugin command
var pluginRunCmd = &cobra.Command{
	Use:   "run [enrichers.yaml]",
	Short: "Run a plugin with specified enrichers",
	Long:  `Run a plugin with enrichers specified in the enrichers.yaml file.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running plugin...")
		// Implementation goes here
	},
}

var configMapName string

var pluginConfigMapCmd = &cobra.Command{
	Use:   "configmap [enrichers.yaml]",
	Short: "Create a configmap from an enrichers.yaml file",
	Long:  `Create a configmap from an enrichers.yaml file and include all referenced files.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Load enrichers from file
		enrichers, err := model.LoadEnrichersFromFile(args[0])
		if err != nil {
			fmt.Printf("Error loading Enrichers from %s: %v\n", args[0], err)
			return
		}

		// Create configmap with custom name if provided
		configMap, err := createConfigMap(enrichers, configMapName)
		if err != nil {
			fmt.Printf("Error creating ConfigMap: %v\n", err)
			return
		}

		// Print configmap
		out, err := yaml.Marshal(configMap)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println(string(out))
	},
}

// createConfigMap generates a Kubernetes ConfigMap from the enrichers configuration
// It collects all files referenced by the enrichers and includes them in the ConfigMap
func createConfigMap(enrichers *model.Enrichers, name string) (map[string]interface{}, error) {
	// Use default name if not provided
	configMapName := "pharos-enrichers"
	if name != "" {
		configMapName = name
	}

	// Define the ConfigMap structure
	configMap := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":   configMapName,
			"labels": map[string]string{"app": "pharos"},
		},
		"data": map[string]string{},
	}

	// Add enrichers.yaml to the ConfigMap
	enrichersYaml, err := yaml.Marshal(enrichers)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal enrichers: %w", err)
	}
	configMap["data"].(map[string]string)["enrichers.yaml"] = string(enrichersYaml)

	// Process each enricher source to collect files
	for _, source := range enrichers.Sources {
		// If it's a Git repo, clone it to a temp directory
		var enricherPath string
		if source.Git != "" {
			tempDir, err := os.MkdirTemp("", "pharos-enricher-*")
			if err != nil {
				return nil, fmt.Errorf("error creating temporary directory: %w", err)
			}

			enricherPath, err = enricher.FetchEnricherFromGit(source.Git, tempDir)
			if err != nil {
				return nil, fmt.Errorf("error fetching enricher from Git: %w", err)
			}
		} else if source.Path != "" {
			enricherPath = source.Path
		} else {
			continue // Skip if no path or Git URL is provided
		}

		// Walk through the enricher directory and add all files to the ConfigMap
		err = filepath.Walk(enricherPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				// Read file content
				content, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("failed to read file %s: %w", path, err)
				}

				// Get relative path for ConfigMap key
				relPath, err := filepath.Rel(enricherPath, path)
				if err != nil {
					return fmt.Errorf("failed to get relative path for %s: %w", path, err)
				}

				// Use the source name as prefix in the ConfigMap to avoid conflicts
				configMapKey := filepath.Join(source.Name, relPath)
				
				// Check if the file has an extension that might contain template expressions
				ext := strings.ToLower(filepath.Ext(path))
				if ext == ".hbs" || ext == ".tmpl" || ext == ".tpl" || strings.Contains(string(content), "{{"){ 
					// Base64 encode content to avoid Helm template processing
					encodedContent := base64.StdEncoding.EncodeToString(content)
					configMap["data"].(map[string]string)[configMapKey] = "b64:" + encodedContent
				} else {
					// Store regular content as is
					configMap["data"].(map[string]string)[configMapKey] = string(content)
				}
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error processing enricher files: %w", err)
		}
	}

	return configMap, nil
}

func init() {
	rootCmd.AddCommand(pluginCmd)
	pluginCmd.AddCommand(pluginRunCmd)
	pluginCmd.AddCommand(pluginConfigMapCmd)
	pluginCmd.AddCommand(testCmd)

	// Add flags for the configmap command
	pluginConfigMapCmd.Flags().StringVarP(&configMapName, "name", "n", "pharos-enrichers", "Name for the ConfigMap (default: pharos-enrichers)")

	// Add flags specific to the test subcommand
	testCmd.Flags().StringVar(&dataFilePath, "data", "test-data.yaml", "Path to test data file to use instead of test-data.yaml")
}
