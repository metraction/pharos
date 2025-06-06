package cmd

import (
	"io"
	"os"
	"strings"

	"github.com/metraction/pharos/globals"
	"github.com/metraction/pharos/logging"
	"github.com/metraction/pharos/models"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var mode string // application or scanner
var log = logging.NewLogger("cmd")

var rootCmd = &cobra.Command{
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var applicationCmd = &cobra.Command{
	Use:   "application",
	Short: "Run the application mode",
	Long:  `Run the application mode of Pharos, which includes the task manager and other application functionalities.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Running in application mode")
		// Here you would typically start the application server,
	},
}

var scannerCmd = &cobra.Command{
	Use:   "scanner",
	Short: "Run the scanner mode",
	Long:  `Run the scanner mode of Pharos, which includes scanning functionalities.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Running in scanner mode")
		// Here you would typically start the scanner functionalities,
	},
}

// Add more commands here as you like

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func initConfig() {
	viper.SetConfigFile(cfgFile)
	viper.AutomaticEnv() // read in environment variables that match

	// Open config file for ENV variables substitution
	file, err := os.Open(viper.ConfigFileUsed())
	if err != nil {
		log.Fatal("No config file found ", err)
		globals.Config = &models.Config{}
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
		log.Info("Using config file: %v", viper.ConfigFileUsed())
	} else {
		log.Fatal("Error loading config", err)
		globals.Config = &models.Config{}
	}

	err = viper.Unmarshal(&globals.Config)
	if err != nil {
		log.Fatal("Unable to decode config into struct", err)
	}
	// TODO: validate the config and add defaults

}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "./config.yaml", "config file (default is ./config.yaml)")
	rootCmd.AddCommand(applicationCmd)
}
