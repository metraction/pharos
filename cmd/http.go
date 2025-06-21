package cmd

import (
	"fmt"
	"log"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/metraction/pharos/internal/controllers"
	"github.com/metraction/pharos/internal/logging"
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
		logger := logging.NewLogger("info")
		currentConfig := cmd.Context().Value("config").(*model.Config)
		if currentConfig == nil {
			logger.Fatal().Msg("Configuration not found in context. Ensure rootCmd PersistentPreRun is setting it.")
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
			logger.Fatal().Err(err).Msg("Failed to create publisher flow")
			return
		}
		// Add routes for the API
		controllers.NewimageController(&api, currentConfig).WithPublisher(client).AddRoutes()
		controllers.NewPharosScanTaskController(&api, currentConfig).WithPublisher(client).AddRoutes()

		// I love swagger

		router.HandleFunc("/api/swagger", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <meta name="description" content="SwaggerUI" />
  <title>SwaggerUI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui.css" />
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-bundle.js" crossorigin></script>
<script>
  window.onload = () => {
    window.ui = SwaggerUIBundle({
      url: '/api/openapi.json',
      dom_id: '#swagger-ui',
    });
  };
</script>
</body>
</html>`))
		})

		router.HandleFunc("/submit/image", routing.SubmitImageHandler(client, currentConfig))
		serverAddr := fmt.Sprintf(":%d", httpPort)
		logger.Info().Str("address", serverAddr).Msg("Starting HTTP server")
		if err := http.ListenAndServe(serverAddr, router); err != nil {
			logger.Fatal().Err(err).Msg("Failed to start HTTP server")
		}
	},
}

func init() {
	rootCmd.AddCommand(httpCmd)
	httpCmd.Flags().IntVarP(&httpPort, "port", "p", 8080, "Port for the HTTP server")

	// You might want to add other flags specific to the HTTP server here.
}
