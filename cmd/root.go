/*
Copyright 2025 NAME HERE EMAIL ADDRESS
*/
package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/metraction/pharos/internal/routing"
	"github.com/metraction/pharos/pkg/model"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var cfgFile string
var config *model.Config = &model.Config{}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "scanner",
	Short: "Pharos scanner",
	Long:  `Pharos scanner (using grype)`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		ctx := context.WithValue(cmd.Context(), "config", config)
		cmd.SetContext(ctx)
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("This is the root command that does nothing.\n  Run go run . scanner")
		// TODO: this is duplicate code of scanner.go
		// Create a new context that can be cancelled.
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel() // Ensure cancel is called on exit to clean up resources

		err := routing.NewScannerFlow(ctx, config)
		if err != nil {
			fmt.Printf("Error creating scanner flow: %v\n", err)
			return
		}
		fmt.Println("Scanner started successfully. Press Ctrl+C to exit.")

		// Wait for interrupt signal to gracefully shut down.
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		fmt.Println("Shutting down scanner...")
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func initConfig() {
	// Setup environment variable handling
	// Viper will look for environment variables like PHAROS_REDIS_PORT, PHAROS_CONFIG
	viper.SetEnvPrefix("PHAROS")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	// Iterate over all flags to bind them to Viper and set Viper defaults from flag defaults
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		// Set Viper's default for this key using the flag's default value.
		// This has the lowest precedence in Viper's hierarchy.
		// Need to handle type conversion for f.DefValue if it's not a string.
		var typedDefaultValue any
		switch f.Value.Type() {
		case "int", "int8", "int16", "int32", "int64":
			typedDefaultValue, _ = strconv.Atoi(f.DefValue)
		case "bool":
			typedDefaultValue, _ = strconv.ParseBool(f.DefValue)
		// Add other types like float, stringSlice if needed
		default: // Assumes string for others (like the 'config' flag)
			typedDefaultValue = f.DefValue
		}
		if typedDefaultValue != nil { // Only set if conversion was meaningful or it's a string default
			viper.SetDefault(f.Name, typedDefaultValue)
		}

		// Bind the pflag to Viper. If the flag is set on the command line,
		// its value will take precedence over environment variables, config files, and Viper defaults.
		viper.BindPFlag(f.Name, f)
	})

	var cfgFilePath string

	if cfgFile != "" {
		// Use config file from the flag.
		cfgFilePath = cfgFile
	} else {
		// Set path to default config file
		cfgFilePath = "./.pharos.yaml"
	}
	viper.SetConfigType("yaml")

	// Attempt to read config file for ENV variables substitution
	file, err := os.Open(cfgFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Config file not found. Using defaults, flags, and environment variables.")
		} else {
			// For other errors, like permission issues, we should fail.
			log.Fatalf("Error opening config file: %v", err)
		}
	} else {
		// Config file was found, let's process it.
		defer file.Close()
		content, err := io.ReadAll(file)
		if err != nil {
			log.Fatalf("Error reading config file: %v", err)
		}
		expandedContent := os.ExpandEnv(string(content))
		myReader := strings.NewReader(expandedContent)

		// If a config file is found, read it in.
		if err := viper.ReadConfig(myReader); err == nil {
			fmt.Println("Using config file:", cfgFilePath)
		} else {
			// This could happen if the file is malformed, for example.
			fmt.Printf("Error parsing config file: %v\n", err)
		}
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		log.Fatal("Unable to decode config into struct", err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pharos.yaml)") // cfgFile is handled specially for file loading, so direct binding is fine.
	rootCmd.PersistentFlags().String("redis.dsn", "localhost:6379", "Redis address")                           // Use dot-notation for Viper key compatibility with nested structs.

	rootCmd.PersistentFlags().String("publisher.requestQueue", "scantasks", "Redis stream for async requests")
	rootCmd.PersistentFlags().String("publisher.priorityRequestQueue", "priorityScantasks", "Redis stream for syncrequests")
	rootCmd.PersistentFlags().String("publisher.responseQueue", "scanresult", "Redis stream for async responses")
	rootCmd.PersistentFlags().String("publisher.priorityResponseQueue", "priorityScanresult", "Redis stream for sync responses")
	rootCmd.PersistentFlags().String("publisher.timeout", "300s", "Publisher timeout")

	rootCmd.PersistentFlags().String("scanner.requestQueue", "scantasks", "Redis stream for requests")
	rootCmd.PersistentFlags().String("scanner.responseQueue", "scanresult", "Redis stream for responses")
	rootCmd.PersistentFlags().String("scanner.timeout", "300s", "Scanner timeout")
	rootCmd.PersistentFlags().String("scanner.cacheEndpoint", "redis://localhost:6379", "Scanner cache endpoint")

	rootCmd.PersistentFlags().String("prometheus.url", "http://prometheus.prometheus.svc.cluster.local:9090", "URL of the Prometheus server")
	rootCmd.PersistentFlags().String("prometheus.interval", "30s", "Interval for scraping Prometheus metrics")

	rootCmd.PersistentFlags().String("database.driver", "postgres", "Database driver for the scanner, righ now, only 'postgres' is implemented.")
	defaultDSN := fmt.Sprintf("postgres://postgres:postgres@localhost:5432/pharos?sslmode=disable") // run `brew install db-browser-for-sqlite` to view the database.
	rootCmd.PersistentFlags().String("database.dsn", defaultDSN, "Database DSN for the scanner, for postgres it is the connection string.")
	rootCmd.AddCommand(scannerCmd)
}
