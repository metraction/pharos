package model

import (
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/metraction/pharos/internal/logging"
	"github.com/rs/zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DatabaseContext struct {
	DB     *gorm.DB
	Config *Database
	Logger *zerolog.Logger
}

var Models = []interface{}{
	PharosImageMeta{},
	PharosVulnerability{},
	PharosScanFinding{},
	PharosPackage{},
	ContextRoot{},
	Context{},
	Alert{},
	AlertLabel{},
	AlertAnnotation{},
	AlertPayload{},
	Enricher{},
}

// DefaultGormModel provides a base model with common fields for GORM models, removing the DeletedAt field.
type DefaultGormModel struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewDatabaseContext(config *Database) *DatabaseContext {
	dsn := config.Dsn
	logger := logging.NewLogger("info", "component", "DatabaseContext")
	var dialector gorm.Dialector
	switch config.Driver {
	// case DatabaseDriverSqlite:
	// 	dialector = sqlite.Open(dsn)
	case DatabaseDriverPostgres:
		dialector = postgres.Open(dsn)
	default:
		logger.Panic().Any("driver", config.Driver).Msg("Unsupported database driver")
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to connect to database")
	}
	return &DatabaseContext{
		DB:     db,
		Config: config,
		Logger: logger,
	}
}

func (dc *DatabaseContext) Migrate() error {
	if dc.DB.Migrator().HasColumn(AlertLabel{}, "id") {
		dc.Logger.Warn().Msg("Dropping AlertLabel table, it has been changed to use different primary keys")
		err := dc.DB.Migrator().DropTable(AlertLabel{})
		if err != nil {
			dc.Logger.Error().Err(err).Msg("Failed to drop AlertLabel table")
		}
	}
	if dc.DB.Migrator().HasColumn(AlertAnnotation{}, "id") {
		dc.Logger.Warn().Msg("Dropping AlertAnnotation table, it has been changed to use different primary keys")
		err := dc.DB.Migrator().DropTable(AlertAnnotation{})
		if err != nil {
			dc.Logger.Error().Err(err).Msg("Failed to drop AlertAnnotation table")
		}
	}

	for _, model := range Models {
		err := dc.DB.AutoMigrate(&model)
		if err != nil {
			return err
		}
		dc.Logger.Info().Str("model", fmt.Sprintf("%T", model)).Msg("Migrated model")
	}
	return nil
}

func (databaseContext *DatabaseContext) DatabaseMiddleware() func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {

		ctx = huma.WithValue(ctx, "databaseContext", databaseContext)
		next(ctx)
	}
}
