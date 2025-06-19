package cmd

import (
	"fmt"
	"log"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/metraction/pharos/internal/controllers"
	"github.com/metraction/pharos/internal/routing"
	"github.com/metraction/pharos/pkg/model"
	"github.com/spf13/cobra"
)

var httpPort int

// httpCmd represents the http command
var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "Starts an HTTP server to accept image scan requests.",
	Long: `Starts an HTTP server that listens for Docker image submissions (name and SHA) via a POST request.
These submissions are then published to a Redis stream for further processing by the scanner.`,
	Run: func(cmd *cobra.Command, args []string) {
		currentConfig := cmd.Context().Value("config").(*model.Config)
		if currentConfig == nil {
			log.Fatal("Configuration not found in context. Ensure rootCmd PersistentPreRun is setting it.")
			return
		}
		databaseContext := model.NewDatabaseContext(&currentConfig.Database)
		databaseContext.Migrate()
		router := http.NewServeMux()
		apiConfig := huma.DefaultConfig("Pharos API", "1.0.0")
		apiConfig.Servers = []*huma.Server{
			{URL: "/api", Description: "Pharos API server"},
		}
		apiConfig.OpenAPIPath = "/openapi"
		api := humago.NewWithPrefix(router, "/api", apiConfig)
		api.UseMiddleware(databaseContext.DatabaseMiddleware())
		client, err := routing.NewPublisher(cmd.Context(), currentConfig)
		if err != nil {
			log.Fatal("Failed to create publisher flow:", err)
			return
		}
		controllers.NewDockerImageController(&api, currentConfig).WithPublisher(client).AddRoutes()

		router.HandleFunc("/submit/image", routing.SubmitImageHandler(client, currentConfig))
		serverAddr := fmt.Sprintf(":%d", httpPort)
		log.Printf("Starting HTTP server on %s\n", serverAddr)
		if err := http.ListenAndServe(serverAddr, router); err != nil {
			log.Fatalf("Failed to start HTTP server: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(httpCmd)
	httpCmd.Flags().IntVarP(&httpPort, "port", "p", 8080, "Port for the HTTP server")

	// You might want to add other flags specific to the HTTP server here.
}
