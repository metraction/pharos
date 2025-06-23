// Converts prometheus metrics to a PharosScanTask struct

package prometheus

import (
	"time"

	hwmodel "github.com/metraction/handwheel/model"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

type PharosScanTaskCreator struct {
	Logger *zerolog.Logger
}

func NewPharosScanTaskCreator() *PharosScanTaskCreator {
	return &PharosScanTaskCreator{
		Logger: logging.NewLogger("info"),
	}
}

func (pst *PharosScanTaskCreator) Result(metric hwmodel.ImageMetric) []model.PharosScanTask {
	now := time.Now()
	pharosScanTask := model.PharosScanTask{
		ImageSpec: model.PharosImageSpec{
			Image: metric.Image_spec,
		},
		Created: now,
		Updated: now,
	}
	return []model.PharosScanTask{pharosScanTask}
}
