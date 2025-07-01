package cache

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/metraction/pharos/internal/logging"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

var logger = logging.NewLogger("info")
var loremIpsum string = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."

// Helper: setup redis test endpoint (miniredis or external instance)
func setupRedis(t *testing.T) (string, bool) {

	// prepare local redis
	mr, err := miniredis.Run()
	assert.NoError(t, err)

	// select redis instance for tests
	redisEndpoint := os.Getenv("TEST_REDIS_ENDPOINT")
	useMiniRedis := lo.Ternary(redisEndpoint == "", true, false) // some results differ
	redisEndpoint = lo.Ternary(redisEndpoint == "", "redis://"+mr.Addr(), redisEndpoint)

	return redisEndpoint, useMiniRedis
}

func TestCache(t *testing.T) {

	// get redis or miniredis endpoint
	redisEndpoint, useMiniRedis := setupRedis(t)

	fmt.Println("-----< TEST SETUP >-----")
	fmt.Printf("redisEndpoint: %s (use miniredis:%v)\n", redisEndpoint, useMiniRedis)

	ctx := context.Background()

	var err error
	var data []byte
	var loremData []byte = []byte(loremIpsum)

	var kvc *PharosCache

	kvc, err = NewPharosCache(redisEndpoint, logger)
	assert.NoError(t, err)
	assert.Equal(t, "cache", kvc.ServiceName())

	defer kvc.Close()
	err = kvc.Connect(ctx)
	assert.NoError(t, err)

	// check memory
	used, peak, system := kvc.UsedMemory(ctx)
	assert.NotEqual(t, "", used)
	assert.NotEqual(t, "", peak)
	assert.NotEqual(t, "", system)

	// pack/unpack
	packed := kvc.Pack(loremData)
	unpacked, err := kvc.UnPack(packed)

	assert.NoError(t, err)
	assert.Greater(t, len(loremData), len(packed))
	assert.Greater(t, len(unpacked), len(packed))
	assert.Equal(t, string(unpacked), loremIpsum)

	// set/get expired
	cacheTTL := 1 * time.Second
	names := []string{"alfa", "bravo", "charlie"}
	for _, key := range names {

		// set
		err = kvc.SetExpire(ctx, key+".uno", loremData, cacheTTL)
		assert.NoError(t, err)
		err = kvc.SetExpirePack(ctx, key+".due", loremData, cacheTTL)
		assert.NoError(t, err)

		// get
		data, err = kvc.GetExpire(ctx, key+".uno", cacheTTL)
		assert.NoError(t, err)
		assert.Equal(t, loremData, data)

		data, err = kvc.GetExpireUnpack(ctx, key+".due", cacheTTL)
		assert.NoError(t, err)
		assert.Equal(t, loremData, data)

		// nokey
		data, err = kvc.GetExpireUnpack(ctx, key+".nope", cacheTTL)
		assert.Equal(t, ErrKeyNotFound, err)
		assert.Empty(t, data)
	}

}
