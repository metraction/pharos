package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/metraction/pharos/internal/utils"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

var ErrKeyNotFound = errors.New("key not found")

// Pharos redis cache

type PharosCache struct {
	Endpoint string

	rdb    *redis.Client
	packer *utils.ZStd
	logger *zerolog.Logger
}

func NewPharosCache(endpoint string, logger *zerolog.Logger) (*PharosCache, error) {

	logger.Debug().
		Msg("NewPharosCache() ..")

	packer, err := utils.NewZStd()
	if err != nil {
		return nil, err
	}

	// prepare client, does not connect
	options, err := redis.ParseURL(endpoint)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(options)
	return &PharosCache{
		Endpoint: endpoint,
		packer:   packer,
		rdb:      client,
		logger:   logger,
	}, nil
}

func (rx PharosCache) ServiceName() string {
	return "cache"
}

func (rx *PharosCache) Connect(ctx context.Context) error {

	if err := rx.rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis connect (ping): %v", err)
	}
	return nil
}

func (rx *PharosCache) CheckConnected(ctx context.Context) error {
	return rx.rdb.Ping(ctx).Err()
}
func (rx *PharosCache) Close() {
	if rx.rdb != nil {
		rx.rdb.Close()
	}
}

// retrieve memory usage
func (rx *PharosCache) UsedMemory(ctx context.Context) (string, string, string) {

	memUsed := "N/A"
	memPeak := "N/A"
	memSystem := "N/A"

	if info, err := rx.rdb.Info(ctx, "memory").Result(); err == nil {
		for _, line := range strings.Split(info, "\n") {
			var parts []string
			line = strings.TrimSpace(line)
			if parts = strings.Split(line, "used_memory_human:"); len(parts) == 2 {
				memUsed = parts[1]
			}
			if parts = strings.Split(line, "used_memory_peak_human:"); len(parts) == 2 {
				memPeak = parts[1]
			}
			if parts = strings.Split(line, "total_system_memory_human:"); len(parts) == 2 {
				memSystem = parts[1]
			}
		}
	}
	return memUsed, memPeak, memSystem
}
func (rx PharosCache) Version(ctx context.Context) string {
	info, err := rx.rdb.Info(ctx, "server").Result()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(info, "\n") {
		if strings.HasPrefix(line, "redis_version:") {
			return strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
	}
	return "redis_version not found"
}

func (rx PharosCache) Pack(data []byte) []byte {
	return rx.packer.Compress(data)
}

func (rx PharosCache) UnPack(data []byte) ([]byte, error) {
	result, err := rx.packer.Decompress(data)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (rx PharosCache) Get(ctx context.Context, key string) ([]byte, error) {
	result, err := rx.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// key not found
			return nil, ErrKeyNotFound
		} else {
			return nil, err
		}
	}
	return []byte(result), nil
}

// set key and expire.
func (rx PharosCache) SetExpire(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	return rx.rdb.Set(ctx, key, data, ttl).Err()
}
func (rx PharosCache) SetExpirePack(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	return rx.rdb.Set(ctx, key, rx.Pack(data), ttl).Err()
}

// get key and expire
func (rx PharosCache) GetExpire(ctx context.Context, key string, ttl time.Duration) ([]byte, error) {
	result, err := rx.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if ttl > 0 {
		if err := rx.rdb.Expire(ctx, key, ttl).Err(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (rx PharosCache) GetExpireUnpack(ctx context.Context, key string, ttl time.Duration) ([]byte, error) {
	result, err := rx.GetExpire(ctx, key, ttl)
	if err != nil {
		return nil, err
	}
	return rx.UnPack(result)
}
