package cmd

import (
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/reugn/go-streams/extension"

	"github.com/metraction/pharos/internal/controllers"
	"github.com/metraction/pharos/internal/integrations/db"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/internal/routing"
	"github.com/metraction/pharos/pkg/mappers"
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
		logger := logging.NewLogger("info", "component", "cmd.http")
		config := cmd.Context().Value("config").(*model.Config)
		if config == nil {
			logger.Fatal().Msg("Configuration not found in context. Ensure rootCmd PersistentPreRun is setting it.")
		}
		databaseContext := model.NewDatabaseContext(&config.Database)
		databaseContext.Migrate()

		// TODO move huma config at the end of this file: first dependencies (db, redis), then flows and then huma
		router := http.NewServeMux()
		apiConfig := huma.DefaultConfig("Pharos API", "1.0.0")
		apiConfig.Servers = []*huma.Server{
			{URL: "/api", Description: "Pharos API server"},
		}
		apiConfig.OpenAPIPath = "/openapi"
		api := humago.NewWithPrefix(router, "/api", apiConfig)
		metricsController := controllers.NewMetricsController(&api, config)
		api.UseMiddleware(metricsController.MetricsMiddleware())

		api.UseMiddleware(databaseContext.DatabaseMiddleware())

		enricherConfig, err := mappers.LoadMappersConfig("results", config.EnricherPath+"/config.yaml")
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to load mappers config")
		}
		enricher := model.EnricherConfig{
			BasePath: config.EnricherPath,
			Configs:  enricherConfig,
		}

		// For scan tasks
		taskChannel := make(chan any, config.Publisher.QueueSize)
		// For scanning bypass
		resultChannel := make(chan any, config.ResultCollector.QueueSize)

		// Results processing stream reading from redis
		go routing.NewScanResultCollectorFlow(
			cmd.Context(),
			config,
			enricher,
			extension.NewChanSource(taskChannel),
			logger,
		).
			//Via(flow.NewMap(routing.NewNotifier(), 1)).
			To(db.NewImageDbSink(databaseContext))

		// Create results flow without redis
		go routing.NewScanResultsInternalFlow(extension.NewChanSource(resultChannel), enricher).
			To(db.NewImageDbSink(databaseContext))

		// Add routes for the API
		controllers.NewimageController(&api, config).AddRoutes()
		controllers.NewPharosScanTaskController(&api, config, taskChannel, resultChannel).AddRoutes()
		metricsController.AddRoutes()
		// Add go streams routes
		go routing.NewImageCleanupFlow(databaseContext, config)

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
}
