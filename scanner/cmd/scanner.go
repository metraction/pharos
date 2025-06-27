/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/internal/integrations/cache"
	"github.com/metraction/pharos/internal/integrations/mq"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/grype"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/scanner/config"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of command
type ScannerArgsType = struct {
	OutDir string // results dump dir
	Engine string // scan engine to use
	Worker string // scanner consumer name (MQ)

	MqEndpoint    string // redis://user:pwd@localhost:6379/1
	CacheEndpoint string
}

var ScannerArgs = ScannerArgsType{}

func init() {
	rootCmd.AddCommand(scannerCmd)

	scannerCmd.Flags().StringVar(&ScannerArgs.OutDir, "outdir", EnvOrDefault("outdir", ""), "Output directory for results")
	scannerCmd.Flags().StringVar(&ScannerArgs.Engine, "engine", EnvOrDefault("engine", ""), "Scan engine [grype,trivy]")
	scannerCmd.Flags().StringVar(&ScannerArgs.Worker, "worker", EnvOrDefault("worker", ""), "scanner worker name (consumer)")

	scannerCmd.Flags().StringVar(&ScannerArgs.MqEndpoint, "mq_endpoint", EnvOrDefault("mq_endpoint", ""), "Redis message queue, e.g. redis://:pwd@localhost:6379/1")
	scannerCmd.Flags().StringVar(&ScannerArgs.CacheEndpoint, "cache_endpoint", EnvOrDefault("cache_endpoint", ""), "Redis message queue, e.g. redis://:pwd@localhost:6379/1")

}

// runCmd represents the run command
var scannerCmd = &cobra.Command{
	Use:   "scanner",
	Short: "Execute scan tasks from MQ",
	Long:  `Execute scan tasks from MQ`,
	Run: func(cmd *cobra.Command, args []string) {

		ExecuteScanner(ScannerArgs.Engine, ScannerArgs.Worker, ScannerArgs.MqEndpoint, ScannerArgs.CacheEndpoint, ScannerArgs.OutDir, logger)

	},
}

func ExecuteScanner(engine, worker, mqEndpoint, cacheEndpoint, outDir string, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scanner sender >-----")
	logger.Info().
		Str("engine", engine).
		Str("worker", worker).
		Str("mq_endpoint", utils.MaskDsn(mqEndpoint)).
		Str("cache_endpoint", utils.MaskDsn(cacheEndpoint)).
		Str("outdir", outDir).
		Msg("")

	// check
	if worker == "" {
		logger.Fatal().Str("worker", worker).Msg("empty worker")
	}
	// check
	if outDir != "" && !utils.DirExists(outDir) {
		logger.Fatal().Str("outdir", outDir).Msg("dir not found")
	}
	// initialize
	ctx := context.Background()

	var err error
	var taskMq *mq.RedisWorkerGroup[model.PharosScanTask]     // send scan tasks
	var resultMq *mq.RedisWorkerGroup[model.PharosScanResult] // send scan results
	var kvCache *cache.PharosCache                            // sbom cache

	if taskMq, err = mq.NewRedisWorkerGroup[model.PharosScanTask](ctx, mqEndpoint, "$", config.RedisTaskStream, "task-group", config.RedisTaskStreamMaxLen); err != nil {
		logger.Fatal().Err(err).Msg("NewRedisWorkerGroup")
	}
	if resultMq, err = mq.NewRedisWorkerGroup[model.PharosScanResult](ctx, mqEndpoint, "$", config.RedisResultStream, "result-group", config.RedisTaskStreamMaxLen); err != nil {
		logger.Fatal().Err(err).Msg("NewRedisWorkerGroup")
	}
	if kvCache, err = cache.NewPharosCache(cacheEndpoint, logger); err != nil {
		logger.Fatal().Err(err).Msg("NewPharosCache")
	}
	defer taskMq.Close()
	defer resultMq.Close()
	defer kvCache.Close()

	// try connect 3x with 3 sec sleep to account for startup delays of required pods/services
	if err := integrations.TryConnectServices(ctx, 3, 3*time.Second, []integrations.ServiceInterface{taskMq, resultMq, kvCache}, logger); err != nil {
		logger.Fatal().Err(err).Msg("services connect")
	}
	logger.Info().Msg("services connect OK")

	// ensure stream groups are present
	taskMq.CreateGroup(ctx)
	resultMq.CreateGroup(ctx)

	scanTimeout := 180 * time.Second // default timeout
	if engine == "grype" {
		var scanEngine *grype.GrypeScanner

		if scanEngine, err = grype.NewGrypeScanner(scanTimeout, true, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewGrypeScanner()")
		}

		// scanHandler
		grypeScanHandler := func(x mq.TaskMessage[model.PharosScanTask]) error {

			task := x.Data
			image := task.ImageSpec.Image

			// ensure message is evicted after 2 tries
			if x.RetryCount > 2 {
				logger.Error().
					Err(fmt.Errorf("%v ack & forget", x.Id)).
					Str("_id", x.Id).Str("_job", task.JobId).Any("retry", x.RetryCount).Any("image", image).
					Msg("max retry exceeded")
				return nil
			}

			logger.Info().
				Str("_id", x.Id).Str("_job", task.JobId).Any("retry", x.RetryCount).Any("image", image).
				Msg("scan task ")

			// scan image, use cache
			result, _, _, err := grype.ScanImage(task, scanEngine, kvCache, logger)
			if err != nil {
				logger.Error().Err(err).
					Str("_id", x.Id).Str("_job", task.JobId).Any("retry", x.RetryCount).Any("image", image).
					Msg("scan error")
				return err
			}

			logger.Info().
				Str("_id", x.Id).Str("_job", task.JobId).Any("retry", x.RetryCount).Any("image", image).
				Str("os", result.Image.DistroName+" "+result.Image.DistroVersion).
				Any("findings", len(result.Findings)).
				Any("packages", len(result.Packages)).
				Msg("scan OK")

			// submit scan results
			id, _ := resultMq.Publish(ctx, 1, result)
			logger.Info().Str("id", id).Str("job", task.JobId).Any("image", task.ImageSpec.Image).Msg("send result")

			saveResults(outDir, utils.ShortDigest(result.Image.ImageId), "grype", result)
			// success
			return err
		}

		claimBlock := int64(5)
		claimMinIdle := 30 * time.Second // min idle time to reclaim Non-ACKed messages
		blockTime := 30 * time.Second    // max block time of xreadgroup
		runTimeout := 15 * time.Minute   // terminate subscribe

		// event loop: scubscribe to scan tasks, run scanner update every 30 min
		run := 0
		elapsedTotal := utils.ElapsedFunc()
		for {
			run++
			elapsed := utils.ElapsedFunc()
			stats, err := taskMq.GroupStats(ctx, "*")
			if err != nil {
				logger.Fatal().Err(err).Msg("taskMq.GroupStats")
			}

			logger.Info().
				Any("pending", stats.Pending).
				Any("lag", stats.Lag).
				Any("stream.len", stats.StreamLen).
				Any("stream.max", stats.StreamMax).
				Any("run.id", run).
				Any("run.timeout", runTimeout.String()).
				Any("elapsed.tot", elapsedTotal().String()).
				Any("elapsed.run", elapsed().String()).
				Msg("even loop")

			taskMq.Subscribe(ctx, worker, claimBlock, claimMinIdle, blockTime, runTimeout, grypeScanHandler)

			if err := scanEngine.UpdateDatabase(); err != nil {
				logger.Fatal().Err(err).Msg("vulndb update failed")
			}
		}
	} else {
		logger.Fatal().Str("engine", engine).Msg("unknon engine")
	}
	logger.Info().Msg("done")
}

// saveResults(outDir, utils.ShortDigest(result.Image.ImageId), "grype", result)
func saveResults(outDir, id, engine string, result model.PharosScanResult) {
	outFile := filepath.Join(outDir, fmt.Sprintf("%s-%s-model.json", id, engine))
	os.WriteFile(outFile, result.ToBytes(), 0644)

}
