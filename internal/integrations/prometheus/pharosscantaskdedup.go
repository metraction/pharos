package prometheus

import (
	"regexp"
	"sync"
	"time"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

// PharosScanTaskDedup holds state for filtering duplicate images in-memory.
type PharosScanTaskDedup struct {
	seen   map[string]model.PharosScanTask
	mu     sync.Mutex
	Logger *zerolog.Logger
}

// NewDedup constructs a Dedup instance.
func NewPharosScanTaskDeduplicator() *PharosScanTaskDedup {
	return &PharosScanTaskDedup{
		seen:   make(map[string]model.PharosScanTask),
		Logger: logging.NewLogger("info"),
	}
}

// FilterDuplicates is a predicate for flow.NewFilter to filter out already seen images.
func (d *PharosScanTaskDedup) FilterDuplicates(task model.PharosScanTask) bool {
	key := task.ImageSpec.Image
	d.mu.Lock()
	defer d.mu.Unlock()
	if matched, _ := regexp.MatchString(`^ghcr.io`, key); !matched {
		return false
	}
	random := time.Now().UnixNano() % 15
	if _, exists := d.seen[key]; exists {
		oldTask := d.seen[key]
		if oldTask.Created.Before(time.Now().Add(-30 * time.Minute).Add(time.Duration(time.Duration(random) * time.Minute))) {
			delete(d.seen, key)
		}
	}
	if _, exists := d.seen[key]; exists {
		return false
	}
	d.seen[key] = task
	return true
}
