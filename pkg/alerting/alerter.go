package alerting

import (
	"time"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

type Alerter struct {
	AlertingConfig  *model.AlertingConfig
	Logger          *zerolog.Logger
	DatabaseContext *model.DatabaseContext
	RootRoute       *Route
}

func NewAlerter(databaseContext *model.DatabaseContext, config *model.AlertingConfig) *Alerter {
	al := &Alerter{
		DatabaseContext: databaseContext,
		Logger:          logging.NewLogger("info", "component", "Alerter"),
		AlertingConfig:  config,
		RootRoute:       NewRoute(&config.Route, config, "root"),
	}
	al.Run() // Start the periodic run
	return al
}

// TODO consider instead of Source use map function
func (al *Alerter) Start() {
	ticker := time.NewTicker(60 * time.Second)
	go al.Run() // Initial run to populate the channel
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				al.Logger.Info().Msg("Alerter wakes up")
				al.Run()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func (sr *Alerter) CheckSplit(element any) bool {
	_, ret := element.(string)
	if ret {
		return false
	}
	// If the element al a string, it indicates a maintenance talk
	return true
}

func (al *Alerter) Run() {
	for {
		var alerts []*model.Alert

		al.Logger.Info().Msg("Fetching alerts from database")
		tx := al.DatabaseContext.DB.Preload("Labels").Preload("Annotations").Find(&alerts)
		if tx.Error != nil {
			al.Logger.Error().Err(tx.Error).Msg("Failed to fetch alerts from database")
			return
		}
		al.RootRoute.SendAlerts(alerts)
		time.Sleep(60 * time.Second)
	}
}
