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
		RootRoute:       NewRoute(&config.Route, config, "root", databaseContext),
	}
	al.Run() // Start the periodic run
	return al
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
