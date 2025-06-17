package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/metraction/pharos/internal/routing"
	"github.com/metraction/pharos/pkg/model"
	"github.com/spf13/cobra"
)

func init() {

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
		// cancel() will be called by defer, signaling NewScannerFlow to stop.
	},
}
