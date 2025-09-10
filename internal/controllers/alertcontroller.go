// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/alerting"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
	"github.com/theory/jsonpath"
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

type AlertLabels struct {
	Body []string `json:"body"`
}

type AlertValues struct {
	Body []string `json:"body"`
}

type AlertValuesSearchInput struct {
	LabelName string `query:"labelName"`
}

type AlertPayloadSearchInput struct {
	Body struct {
		Pagination
		Detail   bool   `query:"detail" default:"true" doc:"If true, returns detailed information about the alert payload"`
		GroupKey string `query:"groupKey" doc:"GroupKey of the to retrieve, can be a glob pattern, exclusive with search"`
		Receiver string `query:"receiver" doc:"Receiver of the alert payload to retrieve, can be a glob pattern, exclusive with search"`
		JSONPath string `query:"jsonPath" doc:"Use RFC9535 jsonPath to filter alert payloads, exclusive with groupKey and receiver"`
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
	{
		op, handler := ac.V1AlertLabelNamesGet()
		huma.Register(*ac.Api, op, handler)
	}
	{
		op, handler := ac.V1AlertLabelValuesGet()
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
			result := db.Scopes(Paginate(&input.Body.Pagination)).Find(&values)
			if result.Error != nil {
				return nil, huma.Error500InternalServerError("Failed to retrieve alerts: " + result.Error.Error())
			}
			if input.Body.JSONPath == "" {
				input.Body.JSONPath = "$[*]"
			}
			parser := jsonpath.NewParser()
			p, err := parser.Parse(input.Body.JSONPath)
			if err != nil {
				return nil, huma.Error400BadRequest("Invalid jsonPath: " + err.Error())
			}

			jsonBytes, err := json.Marshal(&values)
			if err != nil {
				return nil, huma.Error500InternalServerError("Failed to marshal alert payloads: " + err.Error())
			}
			var data interface{}
			if err := json.Unmarshal(jsonBytes, &data); err != nil {
				return nil, huma.Error500InternalServerError("Failed to unmarshal alert payloads: " + err.Error())
			}
			selected := p.Select(data)
			if len(selected) == 0 {
				return &AlertPayloads{
					Body: []model.AlertPayload{},
				}, nil
			}
			jsonBytes, err = json.Marshal(selected)
			// var filteredValues []model.AlertPayload
			// for _, v := range selected {
			// 	if ap, ok := v.(*model.AlertPayload); ok {
			// 		filteredValues = append(filteredValues, *ap)
			// 	} else {
			// 		ac.Logger.Warn().Msgf("Failed to convert selected value to AlertPayload: %v", v)
			// 		return nil, huma.Error500InternalServerError("Failed to convert selected value to AlertPayload")
			// 	}
			// }
			var filteredValues []model.AlertPayload
			if err := json.Unmarshal(jsonBytes, &filteredValues); err != nil {
				return nil, huma.Error500InternalServerError("Failed to unmarshal filtered alert payloads: " + err.Error())
			}
			return &AlertPayloads{
				Body: filteredValues,
			}, nil
		}
}

func (ac *AlertController) getAllAlertLabels(ctx context.Context) ([]model.AlertLabel, error) {
	databaseContext, err := getDatabaseContext(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("Database context not found in request context")
	}
	var alertLabels []model.AlertLabel
	result := databaseContext.DB.Find(&alertLabels)
	if result.Error != nil {
		return nil, huma.Error500InternalServerError("Failed to retrieve alert labels: " + result.Error.Error())
	}
	labels := []model.AlertLabel{}
	for _, label := range alertLabels {
		exists := false
		for _, existing := range labels {
			if existing.Name == label.Name && existing.Value == label.Value {
				exists = true
				break
			}
		}
		if !exists {
			labels = append(labels, model.AlertLabel{
				Name:  label.Name,
				Value: label.Value,
			})
		}
	}
	var AlertPayloads []model.AlertPayload
	result = databaseContext.DB.Find(&AlertPayloads)
	if result.Error != nil {
		return nil, huma.Error500InternalServerError("Failed to retrieve alert payloads: " + result.Error.Error())
	}
	for _, alertPayload := range AlertPayloads {
		for label := range alertPayload.ExtraLabels {
			exists := false
			for _, existing := range labels {
				if existing.Name == label && existing.Value == alertPayload.ExtraLabels[label] {
					exists = true
					break
				}
			}
			if !exists {
				labels = append(labels, model.AlertLabel{
					Name:  label,
					Value: alertPayload.ExtraLabels[label],
				})
			}
		}
	}
	return labels, nil
}

func (ac *AlertController) V1AlertLabelNamesGet() (huma.Operation, func(ctx context.Context, input *struct{}) (*AlertLabels, error)) {
	return huma.Operation{
			OperationID: "V1GetAlertLabelNames",
			Method:      "GET",
			Path:        ac.Path + "/labelnames",
			Summary:     "Get all alert label names",
			Description: "Returns all unique label key-value pairs that exist in all alerts",
			Tags:        []string{"V1/Alert"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A map of all alert labels",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *struct{}) (*AlertLabels, error) {
			labels, err := ac.getAllAlertLabels(ctx)
			if err != nil {
				return nil, err
			}
			labelNames := []string{}
			seen := make(map[string]struct{})
			for _, label := range labels {
				if _, exists := seen[label.Name]; !exists {
					labelNames = append(labelNames, label.Name)
					seen[label.Name] = struct{}{}
				}
			}
			sort.Strings(labelNames)
			return &AlertLabels{
				Body: labelNames,
			}, nil
		}
}

func (ac *AlertController) V1AlertLabelValuesGet() (huma.Operation, func(ctx context.Context, input *AlertValuesSearchInput) (*AlertValues, error)) {
	return huma.Operation{
			OperationID: "V1GetAlertLabelValues",
			Method:      "GET",
			Path:        ac.Path + "/labelvalues",
			Summary:     "Get all values for a given alert label name",
			Description: "Returns all unique values for a given label name across all alerts and alert payloads",
			Tags:        []string{"V1/Alert"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "A list of label values",
				},
				"500": {
					Description: "Internal server error",
				},
				"400": {
					Description: "Bad request",
				},
			},
		}, func(ctx context.Context, input *AlertValuesSearchInput) (*AlertValues, error) {
			if input.LabelName == "" {
				return nil, huma.Error400BadRequest("labelName query parameter is required")
			}
			labels, err := ac.getAllAlertLabels(ctx)
			if err != nil {
				return nil, err
			}
			valueSet := make(map[string]struct{})
			for _, label := range labels {
				if label.Name == input.LabelName {
					valueSet[label.Value] = struct{}{}
				}
			}
			values := make([]string, 0, len(valueSet))
			for v := range valueSet {
				values = append(values, v)
			}
			sort.Strings(values)
			return &AlertValues{
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
