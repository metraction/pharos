package model

import (
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DatabaseContext struct {
	DB     *gorm.DB
	Config *Database
}

var Models = []interface{}{
	DockerImage{},
}

// DefaultGormModel provides a base model with common fields for GORM models, removing the DeletedAt field.
type DefaultGormModel struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewDatabaseContext(config *Database) *DatabaseContext {
	dsn := config.Dsn
	var dialector gorm.Dialector
	switch config.Driver {
	// case DatabaseDriverSqlite:
	// 	dialector = sqlite.Open(dsn)
	case DatabaseDriverPostgres:
		dialector = postgres.Open(dsn)
	default:
		panic(fmt.Sprintf("Unsupported database driver: %s", config.Driver))
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to database: %v", err))
	}
	return &DatabaseContext{
		DB:     db,
		Config: config,
	}
}

func (dc *DatabaseContext) Migrate() error {
	for _, model := range Models {
		err := dc.DB.AutoMigrate(&model)
		if err != nil {
			return err
		}
		fmt.Printf("Migrated model: %T\n", model)
	}
	return nil
}

func (databaseContext *DatabaseContext) DatabaseMiddleware() func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		ctx = huma.WithValue(ctx, "databaseContext", databaseContext)
		next(ctx)
	}
}
