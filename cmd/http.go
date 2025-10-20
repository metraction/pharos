package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	_ "github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/metraction/pharos/internal/controllers"
	"github.com/metraction/pharos/internal/integrations/cache"
	"github.com/metraction/pharos/internal/integrations/db"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/internal/metriccollectors"
	"github.com/metraction/pharos/internal/routing"
	"github.com/metraction/pharos/pkg/enricher"
	"github.com/metraction/pharos/pkg/grype"
	"github.com/metraction/pharos/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/reugn/go-streams/extension"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("info", "component", "cmd.http")

// httpCmd represents the http command
var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "Starts an HTTP server to accept image scan requests.",
	Long: `Starts an HTTP server that listens for Docker image submissions (name and SHA) via a POST request.
These submissions are then published to a Redis stream for further processing by the scanner.`,
	Run: func(cmd *cobra.Command, args []string) {
		configValue := cmd.Context().Value("config")
		if configValue == nil {
			logger.Fatal().Msg("Configuration not found in context. Ensure rootCmd PersistentPreRun is setting it.")
		}
		config, ok := configValue.(*model.Config)
		if !ok || config == nil {
			logger.Fatal().Msg("Invalid configuration type in context.")
		}
		databaseContext := model.NewDatabaseContext(&config.Database, config.Init)
		databaseContext.Migrate()
		if config.Init {
			ctx := cmd.Context()
			redisOk := false
			logger.Info().Msg("Checking Redis cache...")
			for !redisOk {
				kvc, err := cache.NewPharosCache(config.Scanner.CacheEndpoint, logger)
				if err != nil {
					logger.Err(err).Msg("Redis cache create")
				}
				if err == nil {
					if err = kvc.Connect(ctx); err != nil {
						logger.Err(err).Msg("Redis cache connect")
					} else {
						redisOk = true
						logger.Info().Str("redis_version", kvc.Version(ctx)).Msg("PharosCache.Connect() OK")
					}
				}
				if !redisOk {
					logger.Info().Msg("Retrying Redis cache connection in 5 seconds...")
					<-time.After(5 * time.Second)
				}
			}
			logger.Info().Msg("Updating Grype scanner...")
			if _, err := grype.NewGrypeScanner(60, true, "", logger); err != nil {
				dbCacheDir := os.Getenv("GRYPE_DB_CACHE_DIR")
				logger.Debug().Str("GRYPE_DB_CACHE_DIR", dbCacheDir).Msg("Grype settings: ")
				logger.Fatal().Err(err).Msg("NewGrypeScanner()")
			}
			logger.Info().Msg("Init flag set, exiting.")
			os.Exit(0)
		}

		// For scan tasks
		taskChannel := make(chan any, config.Publisher.QueueSize)
		// For scanning bypass
		resultChannel := make(chan any, config.ResultCollector.QueueSize)

		// TODO other commands expect current path, but for http default file are in kodata...
		enricherPath := config.EnricherCommon.EnricherPath
		koDataPath := os.Getenv("KO_DATA_PATH")
		if koDataPath == "" && !filepath.IsAbs(enricherPath) {
			// So append kodada if command is executed with go run .
			enricherPath = filepath.Join("kodata", enricherPath)
		}
		enrichersPath := addBasePathToRelative(config, enricherPath)
		//		enricherConfig := enricher.LoadEnricher(enricherPath, "results")
		enrichers, err := enricher.LoadEnrichersConfig(enrichersPath)
		if err != nil {
			fmt.Printf("Error loading enrichers from %s: %v\n", enrichersPath, err)
			return
		}
		if len(enrichers.Sources) == 0 {
			fmt.Println("No sources found")
			return
		}

		// Results processing stream reading from redis
		collectorFlow := routing.NewScanResultCollectorFlow(
			cmd.Context(),
			config,
			extension.NewChanSource(taskChannel),
			databaseContext,
			logger,
		)

		// Create results flow without redis
		internalFlow := routing.NewScanResultsInternalFlow(extension.NewChanSource(resultChannel), databaseContext)

		// go CreateEnrichersFlow(internalFlow, enrichers, databaseContext, &config.EnricherCommon).
		// 	To(db.NewImageDbSink(databaseContext))
		// go CreateEnrichersFlow(collectorFlow, enrichers, databaseContext, &config.EnricherCommon).
		// 	To(db.NewImageDbSink(databaseContext))

		enricherFlowInternal := NewEnricherFlow(enrichers, databaseContext, &config.EnricherCommon)
		enricherFlowCollector := NewEnricherFlow(enrichers, databaseContext, &config.EnricherCommon)

		go collectorFlow.Via(enricherFlowCollector).
			To(db.NewImageDbSink(databaseContext))

		go internalFlow.Via(enricherFlowInternal).
			To(db.NewImageDbSink(databaseContext))

		// Base Router
		baseRouter := chi.NewRouter()
		commonController := controllers.NewCommonController()
		baseRouter.Use(commonController.RedirectToV1)
		// Define the v1 api
		v1ApiRouter := chi.NewMux()
		v1ApiConfig := huma.DefaultConfig("Pharos API", "1.0.0")
		v1ApiConfig.Servers = []*huma.Server{
			{URL: "/api/v1", Description: "Pharos API server"},
		}

		v1ApiConfig.OpenAPIPath = "/openapi"
		v1Api := humachi.New(v1ApiRouter, v1ApiConfig)

		metricsController := controllers.NewMetricsController(&v1Api, config, taskChannel)
		v1Api.UseMiddleware(metricsController.MetricsMiddleware())

		v1Api.UseMiddleware(databaseContext.DatabaseMiddleware())
		controllers.NewimageController(&v1Api, config).V1AddRoutes()
		controllers.NewPharosScanTaskController(&v1Api, config, taskChannel, resultChannel).V1AddRoutes()
		controllers.NewConfigController(&v1Api, config).V1AddRoutes()
		controllers.NewAlertController(&v1Api, config).V1AddRoutes()
		controllers.NewEnricherController(&v1Api, config).V1AddRoutes()
		metricsController.V1AddRoutes()
		//router.Use(metricsController.MetricsMiddleware)
		// Add go streams routes

		// Register collectors for metrics
		prometheus.MustRegister(
			metriccollectors.NewChannelCollector().
				WithChannel("task_channel", taskChannel).
				WithChannel("result_channel", resultChannel),
		)
		v1ApiRouter.HandleFunc("/swagger", func(w http.ResponseWriter, r *http.Request) {
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
      url: '/api/v1/openapi.json',
      dom_id: '#swagger-ui',
    });
  };
</script>
</body>
</html>`))
		})

		serverAddr := fmt.Sprintf(":%d", httpPort)
		baseRouter.Mount("/api/v1", v1ApiRouter)
		logger.Info().Str("address", serverAddr).Msg("Starting HTTP server")
		if err := http.ListenAndServe(serverAddr, baseRouter); err != nil {
			logger.Fatal().Err(err).Msg("Failed to start HTTP server")
		}
	},
}

func init() {
	rootCmd.AddCommand(httpCmd)
	httpCmd.Flags().IntVarP(&httpPort, "port", "p", 8080, "Port for the HTTP server")
}
