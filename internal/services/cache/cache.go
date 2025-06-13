package cache

import (
	"github.com/metraction/pharos/internal/utils"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// Pharos redis cache

type PharosCache struct {
	Endpoint string

	client *redis.Client
	packer *utils.ZStd
	logger *zerolog.Logger
}
