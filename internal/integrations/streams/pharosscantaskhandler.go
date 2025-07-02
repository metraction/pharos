// Handler functions related to Pharos scan tasks.

package streams

import (
	"fmt"
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
	fmt.Println("Creating root context for image", item.Image.ContextRoots)

	return item
}

func (ph *PharosScanTaskHandler) NotifyReceiver(item model.PharosScanResult) model.PharosScanResult {
	if item.ScanTask.GetReceiver() != nil {
		ph.Logger.Info().Str("ImageId", item.Image.ImageId).Msg("Notifying receiver of scan result")
		*item.ScanTask.GetReceiver() <- item
	}
	return item
}
