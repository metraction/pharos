package cmd

import (
	"fmt"
	"net/http"

	_ "github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/metraction/pharos/internal/routing"
	"github.com/metraction/pharos/pkg/model"
	"github.com/spf13/cobra"
)

// schedulerCmd represents the scheduler command
var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Starts the Pharos scheduler.",
	Long:  `Starts the Pharos scheduler, which manages the scheduling of image scans.`,
	Run: func(cmd *cobra.Command, args []string) {
		configValue := cmd.Context().Value("config")
		if configValue == nil {
			logger.Fatal().Msg("Configuration not found in context. Ensure rootCmd PersistentPreRun is setting it.")
		}
		config, ok := configValue.(*model.Config)
		if !ok || config == nil {
			logger.Fatal().Msg("Invalid configuration type in context.")
		}
		databaseContext := model.NewDatabaseContext(&config.Database)
		databaseContext.Migrate()

		go routing.NewImageSchedulerFlow(databaseContext, config)

		serverAddr := fmt.Sprintf(":%d", httpPort)
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok\n"))
		})
		_ = http.ListenAndServe(serverAddr, nil)
	},
}

func init() {
	rootCmd.AddCommand(schedulerCmd)
	schedulerCmd.Flags().IntVarP(&httpPort, "port", "p", 8080, "Port for the HTTP server")
}
