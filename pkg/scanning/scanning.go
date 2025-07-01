package scanning

import (
	"time"

	"github.com/metraction/pharos/internal/integrations/cache"
	"github.com/metraction/pharos/pkg/grype"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/pkg/trivy"
	"github.com/rs/zerolog"
)

// scanner interface
type ScanEngineInterface interface {
	ScannerName() string
	ScanImage(model.PharosScanTask2) (model.PharosScanResult, []byte, []byte, error)
	UpdateDatabase() error
}

// grype engine interface
type GrypeScannerEngine struct {
	ScanEngine *grype.GrypeScanner

	logger *zerolog.Logger
	cache  *cache.PharosCache
}

func NewGrypeScannerEngine(scanTimeout time.Duration, doUpdate bool, vulnDbDir string, kvCache *cache.PharosCache, logger *zerolog.Logger) (*GrypeScannerEngine, error) {
	var err error
	scanner := GrypeScannerEngine{
		cache:  kvCache,
		logger: logger,
	}
	if scanner.ScanEngine, err = grype.NewGrypeScanner(scanTimeout, doUpdate, vulnDbDir, logger); err != nil {
		return nil, err
	}
	return &scanner, nil
}
func (rx *GrypeScannerEngine) ScannerName() string {
	return rx.ScanEngine.Engine
}
func (rx *GrypeScannerEngine) ScanImage(task model.PharosScanTask2) (model.PharosScanResult, []byte, []byte, error) {
	return grype.ScanImage(task, rx.ScanEngine, rx.cache, rx.logger)
}

func (rx *GrypeScannerEngine) UpdateDatabase() error {
	return rx.ScanEngine.UpdateDatabase()
}

// trivy engine interface
type TrivyScannerEngine struct {
	ScanEngine *trivy.TrivyScanner

	logger *zerolog.Logger
	cache  *cache.PharosCache
}

func NewTrivyScannerEngine(scanTimeout time.Duration, doUpdate bool, vulnDbDir string, kvCache *cache.PharosCache, logger *zerolog.Logger) (*TrivyScannerEngine, error) {
	var err error
	scanner := TrivyScannerEngine{
		cache:  kvCache,
		logger: logger,
	}
	if scanner.ScanEngine, err = trivy.NewTrivyScanner(scanTimeout, doUpdate, vulnDbDir, logger); err != nil {
		return nil, err
	}
	return &scanner, nil
}
func (rx *TrivyScannerEngine) ScannerName() string {
	return rx.ScanEngine.Engine
}

func (rx *TrivyScannerEngine) ScanImage(task model.PharosScanTask2) (model.PharosScanResult, []byte, []byte, error) {
	return trivy.ScanImage(task, rx.ScanEngine, rx.cache, rx.logger)
}

func (rx *TrivyScannerEngine) UpdateDatabase() error {
	return rx.ScanEngine.UpdateDatabase()
}
