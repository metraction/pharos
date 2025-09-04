package alerting

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func StartMockServer(t *testing.T) {
	log := logging.NewLogger("info", "component", "MockServer")
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/alerts", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		expectedToken := "Bearer test-token"
		if authHeader != expectedToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			require.NoError(t, fmt.Errorf("unauthorized: invalid bearer token"))
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			require.NoError(t, fmt.Errorf("method not allowed"))
		}
		var alert model.WebHookPayload
		if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			require.NoError(t, err)
		}
		w.WriteHeader(http.StatusOK)
		log.Info().Int("number of alerts", len(alert.Alerts)).Msg("Received alerts")
	})

	server := &http.Server{
		Addr:    ":8085", // TODO pick port randomly as it clashes with integration tests
		Handler: mux,
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			panic(fmt.Sprintf("Failed to start mock server: %v", err))
		}
	}()
}

func TestDatabase(t *testing.T) {
	t.Logf("setup suite") //
	driver := os.Getenv("DATABASE_DRIVER")
	if driver == "" {
		driver = "sqlite"
	}
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = ":memory:"
	}
	StartMockServer(t)
	var err error
	fmt.Printf("Using database driver: %s, DSN: %s\n", driver, dsn)
	configData, err := os.ReadFile("../../testdata/config/alerting_test.yaml")
	require.NoError(t, err)
	var config *model.Config
	config = &model.Config{}
	err = yaml.Unmarshal(configData, config)
	require.NoError(t, err)
	var db *gorm.DB

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
	alertSize := 0
	// Base Router

	t.Run("01 AddDataTo Database", func(t *testing.T) {
		t.Logf("Adding data to database")
		data, err := os.ReadFile("../../testdata/alerts.json")
		require.NoError(t, err)

		var alerts []model.Alert
		err = json.Unmarshal(data, &alerts)
		require.NoError(t, err)
		t.Logf("Unmarshaled %d alerts", len(alerts))

		// Add alerts to the database
		for _, alert := range alerts {
			tx := databaseContext.DB.Create(&alert)
			require.NoError(t, tx.Error)
		}
		t.Logf("Added %d alerts to the database", len(alerts))
		for _, alert := range alerts {
			tx := databaseContext.DB.Save(&alert)
			require.NoError(t, tx.Error)
		}
		t.Logf("Updated %d alerts in the database", len(alerts))
		// Wait for the server to start
		alertSize = len(alerts)
	})

	t.Run("02 Retrieve Data from the database and create GroupedAlert", func(t *testing.T) {
		t.Logf("Retrieving data from database")
		var alerts []*model.Alert
		tx := databaseContext.DB.Preload("Labels").Preload("Annotations").Find(&alerts)
		require.NoError(t, tx.Error)
		t.Logf("Retrieved %d alerts from the database", len(alerts))
		require.Equal(t, alertSize, len(alerts), "expected %d alerts in the database", alertSize)
		rootRoute := NewRoute(&config.Alerting.Route, &config.Alerting, "root", &databaseContext)
		rootRoute.SendAlerts(alerts)
		require.NotNil(t, rootRoute, "AlertGroup should not be nil")
		// we have 2 alerts in the first child
		require.Len(t, rootRoute.FirstChild.Alerts, 2, "expected 2 alerts in the first child")
		require.Len(t, rootRoute.
			FirstChild.
			NextSibling.
			Alerts, 21, "expected 21 alerts for the first sibling of the first child")
		require.Len(t, rootRoute.
			FirstChild.
			NextSibling.
			NextSibling.
			Alerts, 15, "expected 15 alerts for the second sibling of the first child")
		require.Len(t, rootRoute.
			FirstChild.
			NextSibling.
			NextSibling.
			NextSibling.
			Alerts, 8, "expected 8 alerts for the third sibling of the first child")
	})
}
