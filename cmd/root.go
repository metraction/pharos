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
	"strconv"
	"strings"

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

		// Debug output (optional)
		// envVarKey := "PHAROS_" + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		// fmt.Printf("Viper key '%s': CLI (via BindPFlag), Env ('%s'), Default ('%v')\n", f.Name, envVarKey, f.DefValue)
	})
	// Note: viper.AutomaticEnv() with SetEnvPrefix and SetEnvKeyReplacer handles binding environment variables.
	// Explicit viper.BindEnv calls are not strictly necessary if keys align.

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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pharos.yaml)")  // cfgFile is handled specially for file loading, so direct binding is fine.
	rootCmd.PersistentFlags().String("redis.host", "localhost", "Redis host")                                   // Use dot-notation for Viper key compatibility with nested structs.
	rootCmd.PersistentFlags().Int("redis.port", 6379, "Redis port")                                             // Use dot-notation for Viper key compatibility with nested structs.
	rootCmd.PersistentFlags().String("publisher.stream-name", "scanner", "Redis stream name for the publisher") // Publisher specific config
	rootCmd.PersistentFlags().String("database.driver", "sqlite", "Database driver for the scanner, can be 'sqlite' or 'postgres', `sqlite` is default.")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Unable to determine home directory: %v", err)
	}
	defaultDSN := fmt.Sprintf("%s/%s", homeDir, "pharos.db") // run `brew install db-browser-for-sqlite` to view the database.
	rootCmd.PersistentFlags().String("database.dsn", defaultDSN, "Database DSN for the scanner, for sqlite it is the file name (default is $HOME/.pharos.db, can be 'file::memory:?cache=shared'), for postgres it is the connection string.")
	rootCmd.AddCommand(scannerCmd)
}
