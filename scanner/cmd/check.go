/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"

	"github.com/metraction/pharos/internal/integrations/cache"
	"github.com/metraction/pharos/internal/integrations/mq"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/scanner/config"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

type CheckArgsType = struct {
	MqEndpoint    string // redis://user:pwd@localhost:6379/1
	CacheEndpoint string
}

var CheckArgs = CheckArgsType{}

// define command line arguments
func init() {
	rootCmd.AddCommand(checkCmd)

	checkCmd.Flags().StringVar(&CheckArgs.MqEndpoint, "mq_endpoint", EnvOrDefault("mq_endpoint", ""), "Redis message queue, e.g. redis://:pwd@localhost:6379/1")
	checkCmd.Flags().StringVar(&CheckArgs.CacheEndpoint, "cache_endpoint", EnvOrDefault("cache_endpoint", ""), "Redis cache, e.g. redis://:pwd@localhost:6379/0")
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check config and dependencies",
	Long:  `Check local envronment config and dependencies`,

	Run: func(cmd *cobra.Command, args []string) {
		logger = logging.NewLogger(RootArgs.LogLevel)

		ExecuteCheck(CheckArgs.MqEndpoint, CheckArgs.CacheEndpoint, logger)
	},
}

func foundIt(found bool) string {
	return lo.Ternary(found, "found", "n/a")
}

// execute command
func ExecuteCheck(mqEndpoint, cacheEndpoint string, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scanner Check >-----")
	logger.Info().
		Str("mq_endpoint", utils.MaskDsn(mqEndpoint)).
		Str("cache_endpoint", utils.MaskDsn(cacheEndpoint)).
		Msg("")

	logger.Info().Msg("---< Scanner Dependencies >---")

	errors := 0
	requiredPrograms := []string{"syft", "grype", "trivy"}
	for _, name := range requiredPrograms {
		if !utils.IsInstalled(name) {
			errors += 1
			logger.Error().Msg("Not found: " + name)
		}
		if path, err := utils.OsWhich(name); err == nil {
			logger.Info().Str("name", name).Str("path", path).Msg("")
		}
	}

	// check queue if requested
	ctx := context.Background()

	if cacheEndpoint != "" {
		logger.Info().Msg("---< Redis Cache Stats >---")

		kvCache, _ := cache.NewPharosCache(cacheEndpoint, logger)
		memUsed, memPeak, memSystem := kvCache.UsedMemory(ctx)
		logger.Info().
			Str("used", memUsed).
			Str("peak", memPeak).
			Str("system", memSystem).
			Str("endpoint", utils.MaskDsn(kvCache.Endpoint)).
			Msg("redis cache memory")
	}
	if mqEndpoint != "" {
		logger.Info().Msg("---< Redis Queue Stats >---")
		taskMq, _ := mq.NewRedisWorkerGroup[model.PharosScanTask](ctx, mqEndpoint, "$", config.RedisTaskStream, "task-group", 0)
		resultMq, _ := mq.NewRedisWorkerGroup[model.PharosScanResult](ctx, mqEndpoint, "$", config.RedisResultStream, "result-group", 0)

		memUsed, memPeak, memSystem := taskMq.UsedMemory(ctx)
		logger.Info().
			Str("used", memUsed).
			Str("peak", memPeak).
			Str("system", memSystem).
			Str("endpoint", utils.MaskDsn(taskMq.Endpoint)).
			Msg("redis mq memory")

		logger.Info().Msg("---< Redis Task Queue Stats >---")
		if stats, err := taskMq.GroupStats(ctx, "*"); err == nil {
			ShowQueueStats("task-queue", stats, logger)
		}
		if stats, err := resultMq.GroupStats(ctx, "*"); err == nil {
			ShowQueueStats("result-queue", stats, logger)
		}
	}

	logger.Info().Msg("done")
}
