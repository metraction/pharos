// WIP - Not used yet. Will show how to imiplement CRUD actions

package controllers

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

type ConfigController struct {
	Path    string
	Api     *huma.API
	Version string
	Config  *model.Config
	Logger  *zerolog.Logger
}

func NewConfigController(api *huma.API, config *model.Config) *ConfigController {
	cc := &ConfigController{
		Path:    "/config",
		Api:     api,
		Config:  config,
		Logger:  logging.NewLogger("info", "component", "ConfigController"),
		Version: "v1",
	}
	return cc
}

type Config struct {
	Body model.Config `json:"body"`
}

func (cc *ConfigController) V1AddRoutes() {
	{
		op, handler := cc.V1GetConfig()
		huma.Register(*cc.Api, op, handler)
	}
}

// SyncScan handles the creation or update of a Docker image and initiates a scan.

func (cc *ConfigController) V1GetConfig() (huma.Operation, func(ctx context.Context, input *struct{}) (*Config, error)) {
	return huma.Operation{
			OperationID: "V1GetConfig",
			Method:      "GET",
			Path:        cc.Path,
			Summary:     "Get the current configuration of Pharos",
			Description: "Get the current configuration of Pharos",
			Tags:        []string{"V1/Config"},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "The current configuration of Pharos",
				},
				"500": {
					Description: "Internal server error",
				},
			},
		}, func(ctx context.Context, input *struct{}) (*Config, error) {
			return &Config{
				Body: *cc.Config.ObfuscateSensitiveData(),
			}, nil
		}
}
