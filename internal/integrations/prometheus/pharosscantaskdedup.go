package prometheus

import (
	"sync"
	"time"

	"github.com/metraction/pharos/pkg/model"
)

// PharosScanTaskDedup holds state for filtering duplicate images in-memory.
type PharosScanTaskDedup struct {
	seen map[string]model.PharosScanTask
	mu   sync.Mutex
}

// NewDedup constructs a Dedup instance.
func NewPharosScanTaskDeduplicator() *PharosScanTaskDedup {
	return &PharosScanTaskDedup{
		seen: make(map[string]model.PharosScanTask),
	}
}

// FilterDuplicates is a predicate for flow.NewFilter to filter out already seen images.
func (d *PharosScanTaskDedup) FilterDuplicates(task model.PharosScanTask) bool {
	key := task.ImageSpec.Image
	d.mu.Lock()
	defer d.mu.Unlock()
	random := time.Now().UnixNano() % 31
	if _, exists := d.seen[key]; exists {
		oldTask := d.seen[key]
		if oldTask.Created.Before(time.Now().Add(-1 * time.Minute).Add(time.Duration(time.Duration(random) * time.Second))) {
			delete(d.seen, key)
		}
	}
	if _, exists := d.seen[key]; exists {
		return false
	}
	d.seen[key] = task
	return true
}
