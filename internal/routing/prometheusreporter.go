package routing

import (
	"context"
	"time"

	hwintegrations "github.com/metraction/handwheel/integrations"
	hwmodel "github.com/metraction/handwheel/model"
	"github.com/metraction/pharos/internal/integrations/prometheus"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	ext "github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
	"github.com/rs/zerolog"
	v1 "k8s.io/api/core/v1"
)

type PrometheusReporter struct {
	Context               *context.Context
	Config                *model.Config
	Logger                *zerolog.Logger
	Secrets               *[]v1.Secret
	PrometheusIntegration *hwintegrations.PrometheusIntegration
}

func NewPrometheusReporter(ctx *context.Context, config *model.Config) *PrometheusReporter {
	logger := logging.NewLogger("info", "component", "PrometheusReporter")
	hwModelConfig := hwmodel.Config{
		Prometheus: hwmodel.PrometheusConfig{
			Interval: config.Prometheus.Interval,
			URL:      config.Prometheus.URL,
			Query:    config.Prometheus.Query,
			Auth: hwmodel.PrometheusAuth{
				Username: config.Prometheus.Auth.Username,
				Password: config.Prometheus.Auth.Password,
				Token:    config.Prometheus.Auth.Token,
			},
		},
	}
	return &PrometheusReporter{
		Logger:                logger,
		Context:               ctx,
		Config:                config,
		PrometheusIntegration: hwintegrations.NewPrometheusIntegration(&hwModelConfig),
	}
}

func (pr *PrometheusReporter) NewTicker() chan any {
	outChan := make(chan any)
	period, err := time.ParseDuration(pr.Config.Prometheus.Interval)
	if err != nil {
		pr.Logger.Warn().Err(err).Msg("Invalid prometheus.interval in config, defaulting to 1m")
		period = time.Minute
	}
	ticker := time.NewTicker(period)
	go func() {
		for {
			outChan <- ""
			<-ticker.C
		}
	}()
	return outChan
}

func (pr *PrometheusReporter) RunAsServer() error {
	pr.Logger.Info().Msg("PrometheusReporter is running as a server")
	source := ext.NewChanSource(pr.NewTicker())
	pharosScanTaskCreator := prometheus.NewPharosScanTaskCreator(pr.Config).WithImagePullSecrets()
	pharosTaskSink := prometheus.NewPharosTaskSink(pr.Config)
	source.
		Via(flow.NewMap(pr.PrometheusIntegration.FetchImageMetrics, 1)).
		Via(flow.NewFlatMap(hwintegrations.PrometheusResult, 1)).
		Via(flow.NewFlatMap(pharosScanTaskCreator.Result, 1)).
		To(pharosTaskSink)
	return nil
}
