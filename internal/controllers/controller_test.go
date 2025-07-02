package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/internal/routing"
	"github.com/metraction/pharos/pkg/model"
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
	var db *gorm.DB
	var err error
	if driver == "sqlite" {
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: false,
		})
	} else if driver == "postgres" {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: false,
		})
	} else {
		t.Fatalf("Unsupported database driver: %s", driver)
	}
	databaseContext := model.DatabaseContext{
		DB:     db,
		Logger: logging.NewLogger("info", "component", "DatabaseContext"),
	}
	require.NoError(t, err)
	err = databaseContext.Migrate()
	require.NoError(t, err)
	router := http.NewServeMux()
	apiConfig := huma.DefaultConfig("Pharos API", "1.0.0")
	apiConfig.Servers = []*huma.Server{
		{URL: "/api", Description: "Pharos API server"},
	}
	apiConfig.OpenAPIPath = "/openapi"
	api := humago.NewWithPrefix(router, "/api", apiConfig)

	api.UseMiddleware(databaseContext.DatabaseMiddleware())
	NewimageController(&api, config).AddRoutes()

	go func() {
		err := http.ListenAndServe(":8081", router)
		require.NoError(t, err, "Failed to start server")
	}()
	t.Run("01 AddDataTo Database", func(t *testing.T) {
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
			Size: 123456,
		}
		tx := databaseContext.DB.Create(&pharosImageMeta)
		require.NoError(t, tx.Error)
		// Wait for the server to start
		time.Sleep(5 * time.Second)
	})

	t.Run("02 GetDataViaAPI", func(t *testing.T) {
		resp, err := http.Get("http://localhost:8081/api/pharosimagemeta/test-image-id")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
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
	t.Run("03 Cleanup", func(t *testing.T) {
		go routing.NewImageCleanupFlow(&databaseContext, config)
		// Allow cleanup to run
		time.Sleep(10 * time.Second)
	})
}
