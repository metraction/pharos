// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/alerting"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AlertController struct {
	Path    string
	Api     *huma.API
	Config  *model.Config
	Logger  *zerolog.Logger
	Version string
}

type PrometheusAlerts struct {
	Body []model.PrometheusAlert `json:"body"`
}

type Alerts struct {
	Body []model.Alert `json:"body"`
}

type AlertSearchInput struct {
	Pagination
	Detail bool `query:"detail" default:"true" doc:"If true, returns detailed information about the alert"`
}

func NewAlertController(api *huma.API, config *model.Config) *AlertController {
	ac := &AlertController{
		Path:    "/alert",
		Api:     api,
		Config:  config,
		Logger:  logging.NewLogger("info", "component", "AlertController"),
		Version: "v1",
	}
	return ac
}

func (ac *AlertController) V1AddRoutes() {
	{
		op, handler := ac.V1AlertsGetBySearch()
		huma.Register(*ac.Api, op, handler)
	}
	{
		op, handler := ac.V1PrometheusAlertsGetBySearch()
		huma.Register(*ac.Api, op, handler)
	}
}

func (ac *AlertController) V1PrometheusAlertsGetBySearch() (huma.Operation, func(ctx context.Context, input *AlertSearchInput) (*PrometheusAlerts, error)) {
	return huma.Operation{
			OperationID: "V1SearchPrometheusAlerts",
			Method:      "GET",
			Path:        ac.Path + "/prometheus",
			Summary:     "Search for prometheus alerts",
			Description: "Retrieves alerts stored in the database as prometheus alerts",
			Tags:        []string{"V1/Alert"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A list of prometheus alerts",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *AlertSearchInput) (*PrometheusAlerts, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var db *gorm.DB
			if input.Detail {
				db = databaseContext.DB.
					Preload("Labels").
					Preload("Annotations")
			} else {
				db = databaseContext.DB.Omit(clause.Associations)
			}
			db = db.Order("fingerprint ASC")

			var values []model.Alert
			result := db.Scopes(Paginate(&input.Pagination)).Find(&values)
			if result.Error != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve alerts: " + result.Error.Error())
			}
			prometheusAlerts := make([]model.PrometheusAlert, len(values))
			for i, alert := range values {
				prometheusAlerts[i] = *alerting.GetPrometheusAlert(&alert)
			}

			return &PrometheusAlerts{
				Body: prometheusAlerts,
			}, nil
		}
}

func (ac *AlertController) V1AlertsGetBySearch() (huma.Operation, func(ctx context.Context, input *AlertSearchInput) (*Alerts, error)) {
	return huma.Operation{
			OperationID: "V1SearchAlerts",
			Method:      "GET",
			Path:        ac.Path,
			Summary:     "Search for alerts",
			Description: "Retrieves alerts stored in the database as internal representation",
			Tags:        []string{"V1/Alert"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A list of alerts",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *AlertSearchInput) (*Alerts, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var db *gorm.DB
			if input.Detail {
				db = databaseContext.DB.
					Preload("Labels").
					Preload("Annotations")
			} else {
				db = databaseContext.DB.Omit(clause.Associations)
			}
			db = db.Order("fingerprint ASC")

			var values []model.Alert
			result := db.Scopes(Paginate(&input.Pagination)).Find(&values)
			if result.Error != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve alerts: " + result.Error.Error())
			}
			return &Alerts{
				Body: values,
			}, nil
		}
}
