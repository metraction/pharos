package cmd

import (
	"context"
	"net/http"
	"time"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/internal/routing"
	"github.com/metraction/pharos/pkg/model"
	"github.com/spf13/cobra"
)

func init() {

}

var prometheusReporterCmd = &cobra.Command{
	Use:   "prometheus-reporter",
	Short: "Report images from prometheus to pharos",
	Long:  `Pulls information about images from prometheus and posts scantasks to pharos.`, // You can customize this more
	Run: func(cmd *cobra.Command, args []string) {
		logger := logging.NewLogger("info", "component", "cmd.prometheus-reporter")
		config := cmd.Context().Value("config").(*model.Config)
		if config.Init {
			checkUrl := config.Prometheus.PharosURL + "/api/v1/metrics/readiness"
			logger.Info().Str("url", checkUrl).Msg("Checking Pharos readiness...")
			ok := false
			for !ok {
				resp, err := http.Get(checkUrl)
				if err != nil {
					logger.Error().Err(err).Str("url", checkUrl).Msg("Readiness check request failed")
				} else {
					defer resp.Body.Close()
					if resp.StatusCode != 200 {
						logger.Warn().Int("status", resp.StatusCode).Str("url", checkUrl).Msg("Pharos not ready")
					} else {
						logger.Info().Int("status", resp.StatusCode).Str("url", checkUrl).Msg("Pharos ready")
						ok = true
					}
				}
				if !ok {
					logger.Info().Msg("Pharos readiness check failed, retrying in 5 seconds...")
					<-time.After(5 * time.Second)
				}
			}

			logger.Info().Msg("Init flag set, exiting after checks.")
			return
		}
		// Create a new context that can be cancelled.
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel() // Ensure cancel is called on exit to clean up resources
		logger.Info().Msg("Starting Prometheus reporter...")
		reporter := routing.NewPrometheusReporter(&ctx, config)
		logger.Info().Str("reporter", reporter.Logger.GetLevel().String()).Msg("Starting Prometheus reporter...")
		reporter.RunAsServer()
		logger.Info().Msg("Shutting Prometheus reporter down...")
		// cancel() will be called by defer, signaling NewScannerFlow to stop.
	},
}

func init() {
	rootCmd.AddCommand(prometheusReporterCmd)

	// You might want to add other flags specific to the HTTP server here.
}
