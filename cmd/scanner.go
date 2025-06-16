package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/routing"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	// Define 'stream-name' as a local flag for the scanner command.
	// The default value here is for the flag's help message and initial value if not overridden.
	scannerCmd.Flags().String("stream-name", "scanner", "Redis stream name for the scanner input")

	// Bind this local flag to the Viper key "scanner.stream-name".
	// This allows Viper to pick up the flag's value if provided.
	if err := viper.BindPFlag("scanner.stream-name", scannerCmd.Flags().Lookup("stream-name")); err != nil {
		panic(fmt.Errorf("failed to bind scanner.stream-name flag: %w", err))
	}

	// Set a default value in Viper for "scanner.stream-name".
	// This default is used if the flag is not set, no environment variable is set,
	// and the key is not in the config file.
	viper.SetDefault("scanner.stream-name", "scanner")
}

var scannerCmd = &cobra.Command{
	Use:   "scanner",
	Short: "Run the pharos scanner",
	Long:  `Run the vulnerability scanner against specified targets or configurations.`, // You can customize this more
	Run: func(cmd *cobra.Command, args []string) {
		config := cmd.Context().Value("config").(*model.Config)
		fmt.Println("Using config:", config)
		// Create a new context that can be cancelled.
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel() // Ensure cancel is called on exit to clean up resources

		err := routing.NewScannerFlow(ctx, config, config.Scanner.StreamName)
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
		// cancel() will be called by defer, signaling NewScannerFlow to stop.
	},
}
