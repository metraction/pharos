package controllers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestServer(t *testing.T) {
	t.Logf("setup suite") //
	config := &model.Config{
		Database: model.Database{
			Driver: "sqlite",
			Dsn:    ":memory:", // Use in-memory SQLite for testing
		},
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	require.NoError(t, err)
	databaseContext := model.DatabaseContext{
		DB:     db,
		Logger: logging.NewLogger("info", "component", "DatabaseContext"),
	}
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
	t.Run("AddDataTo Database", func(t *testing.T) {
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

	t.Run("GetDataViaAPI", func(t *testing.T) {
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
}
