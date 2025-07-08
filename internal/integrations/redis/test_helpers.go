package redis

import (
	"context"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/metraction/pharos/pkg/model"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// We'll use the actual model types for testing
// PharosScanTask and PharosScanResult are defined in pkg/model

func SetupTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client, *model.Config) {
	// Check if REDIS_DSN environment variable is set
	redisDSN := os.Getenv("REDIS_DSN")
	if redisDSN != "" {
		// Use real Redis instance
		t.Logf("Using real Redis instance at %s", redisDSN)
		config := &model.Config{
			Redis:     model.Redis{DSN: redisDSN},
			Publisher: model.PublisherConfig{Timeout: "5s"},
		}

		// Create Redis client
		rdb := redis.NewClient(&redis.Options{
			Addr: redisDSN,
		})

		// Test the connection
		err := rdb.Ping(context.Background()).Err()
		require.NoError(t, err, "Failed to connect to Redis")

		return nil, rdb, config
	}

	// Start a mini Redis server for testing
	t.Log("Using miniredis for testing")
	mr, err := miniredis.Run()
	require.NoError(t, err)

	// Create Redis config that points to the mini Redis
	redisCfg := model.Redis{
		DSN: mr.Addr(),
	}

	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	config := &model.Config{
		Redis:     redisCfg,
		Publisher: model.PublisherConfig{Timeout: "5s"},
	}
	return mr, rdb, config
}
