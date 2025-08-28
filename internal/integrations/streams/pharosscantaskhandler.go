// Handler functions related to Pharos scan tasks.

package streams

import (
	"time"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type PharosScanTaskHandler struct {
	Logger            *zerolog.Logger
	ScanTaskStatus    *prometheus.GaugeVec   // Metric to track scan task status
	scanTaskProcessed *prometheus.CounterVec // Metric to track processed scan tasks
	DatabaseContext   *model.DatabaseContext // needed to update CreatedAt value
}

func NewPharosScanTaskHandler(databaseContext *model.DatabaseContext) *PharosScanTaskHandler {
	logger := logging.NewLogger("info", "component", "PharosScanTaskHandler")
	scanTaskStatus := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pharos_scantask_status",
		Help: "Status of Pharos scan task",
	}, []string{"image", "platform", "status"})
	err := prometheus.Register(scanTaskStatus)
	if err != nil {
		logger.Warn().Msg("Failed to register pharos_scantask_status status metric duplicate registration?")
	}
	scanTaskProcessed := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pharos_scantask_processed_count",
		Help: "Number of Pharos scan tasks processed",
	}, []string{"status"})
	err = prometheus.Register(scanTaskProcessed)
	if err != nil {
		logger.Warn().Msg("Failed to register pharos_scantask_processed metric duplicate registration?")
	}
	return &PharosScanTaskHandler{
		Logger:            logger,
		ScanTaskStatus:    scanTaskStatus,
		scanTaskProcessed: scanTaskProcessed,
		DatabaseContext:   databaseContext,
	}
}

func (ph *PharosScanTaskHandler) UpdateScanTaskMetrics(item model.PharosScanResult) model.PharosScanResult {
	ph.Logger.Info().Str("ImageSpec", item.ScanTask.ImageSpec).Msg("Updating scan task status")
	errorValue := 0
	status := "success"
	if item.ScanTask.Error != "" {
		errorValue = 1
		status = "error"
	}
	sucessvalue := (errorValue + 1) % 2 // Toggle
	ph.ScanTaskStatus.WithLabelValues(item.ScanTask.ImageSpec, item.ScanTask.Platform, "error").Set(float64(errorValue))
	ph.ScanTaskStatus.WithLabelValues(item.ScanTask.ImageSpec, item.ScanTask.Platform, "success").Set(float64(sucessvalue))
	ph.scanTaskProcessed.WithLabelValues(status).Inc()
	return item
}

func (ph *PharosScanTaskHandler) FilterFailedTasks(item model.PharosScanResult) bool {
	ph.Logger.Info().Str("ImageId", item.Image.ImageId).Msg("Receiving scan result for image")
	if item.ScanTask.Error != "" {
		ph.Logger.Warn().Str("JobId", item.ScanTask.JobId).Str("error", item.ScanTask.Error).Msg("Scan task failed during async scan")
		return false
	} else {
		return true
	}
}

func (ph *PharosScanTaskHandler) SetFirstSeen(item model.PharosScanResult) model.PharosScanResult {
	ph.Logger.Info().Str("ImageId", item.Image.ImageId).Msg("Updating vulnerabilities CreatedAt fields")
	if ph.DatabaseContext == nil {
		ph.Logger.Warn().Msg("No database context available to update CreatedAt fields")
		return item
	}
	for _, vulnerability := range item.Image.Vulnerabilities {
		var dbVulnerability model.PharosVulnerability
		if vulnerability.CreatedAt.IsZero() { // it is not zero if we do the internalflow, it is zero if scanner returns value.
			search := model.PharosVulnerability{
				AdvId:     vulnerability.AdvId,
				AdvSource: vulnerability.AdvSource,
			}
			tx := ph.DatabaseContext.DB.First(&dbVulnerability, search)
			if tx.Error != nil {
				ph.Logger.Info().Str("AdvId", vulnerability.AdvId).Msg("Failed to find vulnerability in database, this is new.")
				vulnerability.CreatedAt = time.Now()
			} else {
				if dbVulnerability.CreatedAt.IsZero() {
					ph.Logger.Info().Str("AdvId", vulnerability.AdvId).Msg("Setting CreatedAt for vulnerability to now")
					vulnerability.CreatedAt = time.Now()
				} else {
					vulnerability.CreatedAt = dbVulnerability.CreatedAt
				}
			}
		}
		// now update item.Image.FirstSeen we find the Finding that has the same AdvId as the vulnerability
		// We always run this, if we import vulnerabilites.CreatedAt from another installation, we have to set it here.
		for i, finding := range item.Image.Findings {
			if finding.AdvId == vulnerability.AdvId && finding.AdvSource == vulnerability.AdvSource {
				item.Image.Findings[i].FirstSeen = vulnerability.CreatedAt
				break
			}
		}
	}
	item.Image.LastSuccessfulScan = time.Now()
	return item
}

func (ph *PharosScanTaskHandler) UpdateScanTime(item model.PharosScanResult) model.PharosScanResult {
	ph.Logger.Info().Str("ImageId", item.Image.ImageId).Msg("Updating scan time for image")
	item.Image.LastSuccessfulScan = time.Now()
	return item
}

func (ph *PharosScanTaskHandler) CreateRootContext(item model.PharosScanResult) model.PharosScanResult {
	contextRoot := item.GetContextRoot("pharos-controller", time.Minute*30) // TODO: Need to make this configurable
	item.Image.ContextRoots = []model.ContextRoot{contextRoot}
	ph.Logger.Info().Interface("ContextRoots", item.Image.ContextRoots).Msg("Creating root context for image")

	return item
}

func (ph *PharosScanTaskHandler) NotifyReceiver(item model.PharosScanResult) model.PharosScanResult {
	if item.ScanTask.GetReceiver() != nil {
		ph.Logger.Info().Str("ImageId", item.Image.ImageId).Msg("Notifying receiver of scan result")
		*item.ScanTask.GetReceiver() <- item
	}
	return item
}
