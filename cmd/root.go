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
	"strings"

	"github.com/metraction/pharos/model"
	"github.com/spf13/cobra"
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
	viper.AutomaticEnv() // read in environment variables that match

	var cfgFilePath string

	if cfgFile != "" {
		// Use config file from the flag.
		cfgFilePath = cfgFile
	} else {
		// Set path to default config file
		cfgFilePath = "./.pharos.yaml"
	}
	viper.SetConfigType("yaml")

	// Open config file for ENV variables substitution
	file, err := os.Open(cfgFilePath)
	if err != nil {
		log.Fatal("No config file found ", err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		log.Fatal("Error reading config file", err)
	}
	expandedContent := os.ExpandEnv(string(content))
	myReader := strings.NewReader(expandedContent)
	// If a config file is found, read it in.
	if err := viper.ReadConfig(myReader); err == nil {
		fmt.Println("Using config file:", cfgFilePath)
	} else {
		fmt.Println("Error loading config", err)
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		log.Fatal("Unable to decode config into struct", err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pharos.yaml)")
	rootCmd.PersistentFlags().IntVar(&config.Redis.Port, "redis-port", 6379, "Redis port")
	rootCmd.AddCommand(scannerCmd)
}
