// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"
	"strings"

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

type AlertPayloads struct {
	Body []model.AlertPayload `json:"body"`
}

type AlertPayloadSearchInput struct {
	Body struct {
		Pagination
		Detail   bool              `query:"detail" default:"true" doc:"If true, returns detailed information about the alert payload"`
		GroupKey string            `query:"groupKey" doc:"GroupKey of the to retrieve, can be a glob pattern, exclusive with search"`
		Receiver string            `query:"receiver" doc:"Receiver of the alert payload to retrieve, can be a glob pattern, exclusive with search"`
		Labels   map[string]string `query:"labels" doc:"Labels to filter the alert payloads, key=value pairs. If set, then pagination is disabled and detail is enabled."`
	}
}

type AlertPayloadsUpdateInput struct {
	Body struct {
		GroupKey    string            `json:"groupKey" query:"groupKey" doc:"GroupKey of the to retrieve, can be a glob pattern, exclusive with search"`
		Receiver    string            `json:"receiver" query:"receiver" doc:"Receiver of the alert payload to retrieve, can be a glob pattern, exclusive with search"`
		ExtraLabels map[string]string `json:"extraLabels" doc:"Extra labels to add to the alert payload"`
	}
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
	{
		op, handler := ac.V1AlertPayloadsGetBySearch()
		huma.Register(*ac.Api, op, handler)
	}
	{
		op, handler := ac.V1AlertPayloadsUpdateExtraLabels()
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

func (ac *AlertController) V1AlertPayloadsGetBySearch() (huma.Operation, func(ctx context.Context, input *AlertPayloadSearchInput) (*AlertPayloads, error)) {
	return huma.Operation{
			OperationID: "V1SearchAlertPayloads",
			Method:      "POST",
			Path:        ac.Path + "/payloads",
			Summary:     "Search for alert payloads",
			Description: "Retrieves alert payloads stored in the database as internal representation",
			Tags:        []string{"V1/Alert"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A list of alert payloads",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *AlertPayloadSearchInput) (*AlertPayloads, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var db *gorm.DB
			// we have to disable pagination if we filter by alerts, because the logic is implemente
			// in controller code and not in the database query.
			// also if we search by labels, details have to be enabled.
			if len(input.Body.Labels) > 0 {
				input.Body.Pagination.PageSize = -1
				input.Body.Detail = true
			}
			if input.Body.Detail {
				db = databaseContext.DB.
					Preload("Alerts").
					Preload("Alerts.Labels").
					Preload("Alerts.Annotations")
			} else {
				db = databaseContext.DB.Omit(clause.Associations)
			}

			if input.Body.GroupKey != "" {
				db = db.Where("group_key LIKE ?", strings.ReplaceAll(input.Body.GroupKey, "*", "%"))
			}
			if input.Body.Receiver != "" {
				db = db.Where("receiver LIKE ?", strings.ReplaceAll(input.Body.Receiver, "*", "%"))

			}
			db = db.Order("group_key,receiver ASC")

			var values []model.AlertPayload
			var filteredValues []model.AlertPayload
			result := db.Scopes(Paginate(&input.Body.Pagination)).Find(&values)
			if len(input.Body.Labels) > 0 {
				for i, payload := range values {
					matched := false
					for _, alert := range payload.Alerts {
						for _, payloadLabel := range alert.Labels {
							for inputKey, inputValue := range input.Body.Labels {
								if payloadLabel.Name == inputKey && payloadLabel.Value == inputValue {
									matched = true
								}
							}
						}
					}
					if matched {
						filteredValues = append(filteredValues, values[i])
					}
				}
				values = filteredValues
			}

			if result.Error != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve alerts: " + result.Error.Error())
			}
			return &AlertPayloads{
				Body: values,
			}, nil
		}
}

func (ac *AlertController) V1AlertPayloadsUpdateExtraLabels() (huma.Operation, func(ctx context.Context, input *AlertPayloadsUpdateInput) (*AlertPayloads, error)) {
	return huma.Operation{
			OperationID: "V1SearchAlertPayloadsUpdateExtraLabels",
			Method:      "POST",
			Path:        ac.Path + "/updatepayloads",
			Summary:     "Update alert payloads",
			Description: "Updates alert payloads",
			Tags:        []string{"V1/Alert"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A list of alert payloads",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *AlertPayloadsUpdateInput) (*AlertPayloads, error) {
			databaseContext, err := getDatabaseContext(ctx)
			if err != nil {
				return nil, huma.Error500InternalServerError("Database context not found in request context")
			}
			var db *gorm.DB

			db = databaseContext.DB

			if input.Body.GroupKey != "" {
				db = db.Where("group_key LIKE ?", strings.ReplaceAll(input.Body.GroupKey, "*", "%"))
			}
			if input.Body.Receiver != "" {
				db = db.Where("receiver LIKE ?", strings.ReplaceAll(input.Body.Receiver, "*", "%"))

			}
			db = db.Order("group_key,receiver ASC")

			var values []model.AlertPayload
			result := db.Find(&values)
			if result.Error != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve alerts: " + result.Error.Error())
			}
			for i, _ := range values {
				values[i].ExtraLabels = input.Body.ExtraLabels
			}
			tx := db.Save(&values)
			if tx.Error != nil {
				return nil, huma.Error500InternalServerError("Failed to update alert payloads: " + tx.Error.Error())
			}
			return &AlertPayloads{
				Body: values,
			}, nil
		}
}
