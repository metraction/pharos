package controllers_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/1set/starlet/lib/file"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/metraction/pharos/cmd"
	"github.com/metraction/pharos/internal/controllers"
	"github.com/metraction/pharos/internal/integrations/db"
	pharosstreams "github.com/metraction/pharos/internal/integrations/streams"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/internal/routing"
	"github.com/metraction/pharos/pkg/enricher"
	"github.com/metraction/pharos/pkg/grype"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestServer(t *testing.T) {
	t.Logf("setup suite") //
	driver := os.Getenv("DATABASE_DRIVER")
	if driver == "" {
		driver = "sqlite"
	}
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = ":memory:"
	}
	fmt.Printf("Using database driver: %s, DSN: %s\n", driver, dsn)
	config := &model.Config{}
	var database *gorm.DB
	var err error
	if driver == "sqlite" {
		database, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: false,
		})
	} else if driver == "postgres" {
		database, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: false,
		})
	} else {
		t.Fatalf("Unsupported database driver: %s", driver)
	}
	databaseContext := model.DatabaseContext{
		DB:     database,
		Logger: logging.NewLogger("info", "component", "DatabaseContext"),
	}
	logger := logging.NewLogger("info", "component", "cmd.http")
	err = databaseContext.Migrate()
	require.NoError(t, err)
	// ctx := t.Context()
	// config.Redis.DSN = "localhost:6379"
	config.Scanner.RequestQueue = "scantasks"
	config.Scanner.ResponseQueue = "scanresult"
	config.Scanner.Timeout = (300 * time.Second).String()
	config.Scanner.CacheEndpoint = "redis://localhost:6379"

	// logger.Info().Msg("Checking Redis cache...")
	// kvc, err := cache.NewPharosCache(config.Scanner.CacheEndpoint, logger)
	// require.NoError(t, err, "Failed to create Redis cache")
	// logger.Info().Str("redis_version", kvc.Version(ctx)).Msg("PharosCache.Connect() OK")
	// err = kvc.Connect(ctx)
	// require.NoError(t, err, "Failed to connect to Redis cache")

	logger.Info().Msg("Updating Grype scanner...")
	if _, err := grype.NewGrypeScanner(60, true, "", logger); err != nil {
		dbCacheDir := os.Getenv("GRYPE_DB_CACHE_DIR")
		logger.Debug().Str("GRYPE_DB_CACHE_DIR", dbCacheDir).Msg("Grype settings: ")
	}
	require.NoError(t, err)
	yamlPath := filepath.Join("..", "..", "testdata", "enrichers", "enrichers.yaml")
	enrichers, err := enricher.LoadEnrichersConfig(yamlPath)

	require.NoError(t, err)
	// For scan tasks
	taskChannel := make(chan any, config.Publisher.QueueSize)
	// For scanning bypass
	resultChannel := make(chan any, config.ResultCollector.QueueSize)
	// Results processing stream reading from redis
	collectorFlow := routing.NewScanResultCollectorFlow(
		t.Context(),
		config,
		extension.NewChanSource(taskChannel),
		&databaseContext,
		logger,
		false,
	)

	// Create results flow without redis
	internalFlow := routing.NewScanResultsInternalFlow(extension.NewChanSource(resultChannel), &databaseContext)

	// go CreateEnrichersFlow(internalFlow, enrichers, databaseContext, &config.EnricherCommon).
	// 	To(db.NewImageDbSink(databaseContext))
	// go CreateEnrichersFlow(collectorFlow, enrichers, databaseContext, &config.EnricherCommon).
	// 	To(db.NewImageDbSink(databaseContext))

	enricherFlowInternal := cmd.NewEnricherFlow(enrichers, &databaseContext, &config.EnricherCommon)
	enricherFlowCollector := cmd.NewEnricherFlow(enrichers, &databaseContext, &config.EnricherCommon)
	pharosScanTaskHandler := pharosstreams.NewPharosScanTaskHandler(&databaseContext)

	go collectorFlow.Via(enricherFlowCollector).
		Via(flow.NewMap(pharosScanTaskHandler.NotifyReceiver, 1)).
		To(db.NewImageDbSink(&databaseContext))

	go internalFlow.Via(enricherFlowInternal).
		Via(flow.NewMap(pharosScanTaskHandler.NotifyReceiver, 1)).
		To(db.NewImageDbSink(&databaseContext))
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
	metricsController := controllers.NewMetricsController(&v1Api, config, make(chan any, 1000))
	v1Api.UseMiddleware(metricsController.MetricsMiddleware())
	v1Api.UseMiddleware(databaseContext.DatabaseMiddleware())
	controllers.NewImageController(&v1Api, config).V1AddRoutes()
	controllers.NewPharosScanTaskController(&v1Api, config, taskChannel, resultChannel).V1AddRoutes()
	metricsController.V1AddRoutes()

	baseRouter.Mount("/api/v1", v1ApiRouter)

	go func() {
		err := http.ListenAndServe(":8081", baseRouter)
		require.NoError(t, err, "Failed to start server")
	}()

	t.Run("01 Create sbom with syft", func(t *testing.T) {
		sbom, err := file.ReadFileString("../../testdata/sbom-k9s.json")
		require.NoError(t, err)
		URL := "http://localhost:8081/api/v1/pharosscantask/syncscan"
		scanTask := model.PharosScanTask{
			ImageSpec: "/bin/k9s",
			Context: map[string]any{
				"namespace": "temporary",
			},
			ContextRootKey: "test",
			Platform:       "darwin/arm64",
			Sbom:           &sbom,
		}
		scanTaskJSON, err := json.Marshal(scanTask)
		require.NoError(t, err)
		// // Wait for the server to start before making the request
		// logger.Info().Msg("Waiting for server to start...")
		// time.Sleep(120 * time.Second)
		reqBody := strings.NewReader(string(scanTaskJSON))
		resp, err := http.Post(URL, "application/json", reqBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var got model.PharosScanResult
		err = json.NewDecoder(resp.Body).Decode(&got)
		require.NoError(t, err)
		defer resp.Body.Close()
	})

	t.Run("03 AddDataTo Database", func(t *testing.T) {
		pharosImageMeta := model.PharosImageMeta{
			ImageId:     "test-image-id",
			IndexDigest: "test-digest",
			ArchName:    "amd64",
			ArchOS:      "linux",
			Vulnerabilities: []model.PharosVulnerability{
				{
					AdvId:       "CVE-2023-12345",
					AdvSource:   "NVD",
					Severity:    "High",
					Description: "Test vulnerability",
				},
			},
			Findings: []model.PharosScanFinding{
				{
					AdvId:     "CVE-2023-12345",
					AdvSource: "NVD",
					Severity:  "High",
				},
			},
			ContextRoots: []model.ContextRoot{
				{
					Key:     "test-context",
					ImageId: "test-image-id",
					TTL:     time.Hour * 24,
				},
			},
			Size: 123456,
		}
		tx := databaseContext.DB.Create(&pharosImageMeta)
		require.NoError(t, tx.Error)
		// Wait for the server to start
		time.Sleep(5 * time.Second)
	})

	t.Run("04 GetDataViaAPI", func(t *testing.T) {
		resp, err := http.Get("http://localhost:8081/api/pharosimagemeta/test-image-id")
		require.NoError(t, err)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status OK, got %s", resp.Status)
		}
		var got model.PharosImageMeta
		err = json.NewDecoder(resp.Body).Decode(&got)
		require.NoError(t, err)
		require.NoError(t, err)
		require.Equal(t, "test-image-id", got.ImageId)
		require.Equal(t, "test-digest", got.IndexDigest)
		require.Equal(t, "amd64", got.ArchName)
		require.Equal(t, "linux", got.ArchOS)
		require.Equal(t, uint64(123456), got.Size)
		require.Len(t, got.Vulnerabilities, 1)
		require.Equal(t, "CVE-2023-12345", got.Vulnerabilities[0].AdvId)
		require.Len(t, got.Findings, 1)
		require.Equal(t, "CVE-2023-12345", got.Findings[0].AdvId)
	})
	// Uncomment the following test once the metrics endpoint is implemented
	t.Run("05 GetMetrics", func(t *testing.T) {
		resp, err := http.Get("http://localhost:8081/api/metrics/gauge")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status OK, got %s", resp.Status)
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		bodyStr := string(bodyBytes)
		lines := 0
		vulnerabilites := 0
		for _, line := range strings.Split(bodyStr, "\n") {
			if regexp.MustCompile("^pharos_vulnerabilities").MatchString(line) {
				t.Logf("Found Pharos vulnerabilites metric: %s", line)
				vulnerabilites++
			}
			lines++
		}
		//require.Greater(t, lines, 0)
		//require.Greater(t, vulnerabilites, 0, "Expected at least one pharos_vulnerabilities metric")
	})
	t.Run("06 Test enricher controller", func(t *testing.T) {
		// Setup: Add a test image meta to the database
		enricher := model.Enricher{
			Name:        "Test Enricher",
			Type:        model.EnricherTypeYaegi,
			Description: "A test enricher that adds dummy data",
		}

		// Setup: Create EnricherController and add routes
		enricherController := controllers.NewEnricherController(&v1Api, config)
		enricherController.V1AddRoutes()

		// Simulate enrichment via API (assuming POST /api/v1/enricher)
		enrichURL := "http://localhost:8081/api/v1/enricher"
		enricherJSON, err := json.Marshal(enricher)
		require.NoError(t, err)
		reqBody := strings.NewReader(string(enricherJSON))
		resp, err := http.Post(enrichURL, "application/json", reqBody)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status OK from enrichment endpoint")

		// Query the enricher endpoint to verify the enricher was added
		getURL := "http://localhost:8081/api/v1/enricher/1"
		resp, err = http.Get(getURL)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status OK from GET enricher endpoint")

		err = json.NewDecoder(resp.Body).Decode(&enricher)
		require.NoError(t, err)
		require.Equal(t, "Test Enricher", enricher.Name)
		require.Equal(t, model.EnricherTypeYaegi, enricher.Type)
		require.Equal(t, "A test enricher that adds dummy data", enricher.Description)
		// Test deletion of enricher via API (assuming DELETE /api/v1/enricher/{id})
		deleteURL := "http://localhost:8081/api/v1/enricher/1"
		req, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
		require.NoError(t, err)
		client := &http.Client{}
		resp, err = client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusNoContent, resp.StatusCode, "Expected status NoContent from DELETE enricher endpoint")

		// Verify enricher is deleted by trying to GET it again
		resp, err = http.Get(deleteURL)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusNotFound, resp.StatusCode, "Expected status NotFound after deletion")

		// Test listing enrichers (should be empty after deletion)
		listURL := "http://localhost:8081/api/v1/enricher"
		resp, err = http.Get(listURL)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "Expected status OK from GET enricher list endpoint")
		var enrichers []model.Enricher
		err = json.NewDecoder(resp.Body).Decode(&enrichers)
		require.NoError(t, err)
		require.Len(t, enrichers, 0, "Expected no enrichers after deletion")

		// Test creating multiple enrichers and listing them
		for i := 2; i <= 3; i++ {
			enricher := model.Enricher{
				Name:        fmt.Sprintf("Enricher %d", i),
				Type:        model.EnricherTypeYaegi,
				Description: fmt.Sprintf("Description %d", i),
			}
			enricherJSON, err := json.Marshal(enricher)
			require.NoError(t, err)
			reqBody := strings.NewReader(string(enricherJSON))
			resp, err := http.Post(enrichURL, "application/json", reqBody)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)
		}
		resp, err = http.Get(listURL)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		err = json.NewDecoder(resp.Body).Decode(&enrichers)
		require.NoError(t, err)
		require.Len(t, enrichers, 2, "Expected two enrichers after adding")
		require.Equal(t, "Enricher 2", enrichers[0].Name)
		require.Equal(t, "Enricher 3", enrichers[1].Name)
	})
	t.Run("07 Cleanup", func(t *testing.T) {
		go routing.NewImageSchedulerFlow(&databaseContext, config)
		// Allow cleanup to run
		time.Sleep(10 * time.Second)
	})
}
