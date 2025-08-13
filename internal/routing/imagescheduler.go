package routing

import (
	"github.com/metraction/pharos/internal/integrations/db"
	"github.com/metraction/pharos/pkg/alerting"
	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
)

func NewImageSchedulerFlow(databaseContext *model.DatabaseContext, config *model.Config) {
	imageDbSource := db.NewImageDbSource(databaseContext, config)
	imageDbHandler := db.NewImageDbHandler(databaseContext)
	imageDbSource.
		Via(flow.NewMap(imageDbHandler.RemoveImagesWithoutContext, 1)).
		Via(flow.NewMap(alerting.HandleAlerts(databaseContext), 1)).
		Via(flow.NewMap(imageDbHandler.RemoveExpiredContexts, 1)).
		To(extension.NewIgnoreSink())
}
