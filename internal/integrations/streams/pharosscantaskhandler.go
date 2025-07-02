// Handler functions related to Pharos scan tasks.

package streams

import (
	"time"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

type PharosScanTaskHandler struct {
	Logger *zerolog.Logger
}

func NewPharosScanTaskHandler() *PharosScanTaskHandler {
	return &PharosScanTaskHandler{
		Logger: logging.NewLogger("info", "component", "PharosScanTaskHandler"),
	}
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

func (ph *PharosScanTaskHandler) UpdateScanTime(item model.PharosScanResult) model.PharosScanResult {
	ph.Logger.Info().Str("ImageId", item.Image.ImageId).Msg("Updating scan time for image")
	item.Image.LastSuccessfulScan = time.Now()
	return item
}

func (ph *PharosScanTaskHandler) CreateRootContext(item model.PharosScanResult) model.PharosScanResult {
	contextRoot := item.GetContextRoot("pharos controller", time.Minute*30) // TODO: Need to make this configurable
	item.Image.ContextRoots = []model.ContextRoot{contextRoot}
	return item
}

func (ph *PharosScanTaskHandler) AddSampleData(item model.PharosScanResult) model.PharosScanResult {
	ph.Logger.Info().Str("ImageId", item.Image.ImageId).Msg("Adding sample data to scan result")
	if len(item.Image.ContextRoots) == 0 {
		ph.Logger.Warn().Msg("No context roots found in scan result, I cannot add anything.")
		return item
	}
	if len(item.Image.ContextRoots) != 1 {
		ph.Logger.Warn().Msg("Wow, this should not happen either, only one context root is expected, but found multiple.")
		return item
	}
	item.Image.ContextRoots[0].Contexts = append(item.Image.ContextRoots[0].Contexts, model.Context{
		ContextRootKey: item.Image.ContextRoots[0].Key,
		ImageId:        item.Image.ImageId,
		Owner:          "my sample enricher",
		UpdatedAt:      time.Now(),
		Data: map[string]any{
			"sampleData": "This is some sample data added by a sample enricher",
			"sampleTime": time.Now(), // we can also add a time here, whatever.
		},
	})
	ph.Logger.Info().Str("ImageId", item.Image.ImageId).Str("urltocheck", "http://localhost//api/pharosimagemeta/contexts/"+item.Image.ImageId).Msg("Sample data added to scan result")
	return item
}
