/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/internal/integrations/cache"
	"github.com/metraction/pharos/internal/integrations/mq"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/pkg/scanning"
	"github.com/metraction/pharos/scanner/config"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of command
type ScannerArgsType = struct {
	OutDir string // results dump dir
	Engine string // scan engine to use
	Worker string // scanner consumer name (MQ)

	QueueMax      string // task queue max size
	MqEndpoint    string // redis://user:pwd@localhost:6379/1
	CacheEndpoint string
}

var ScannerArgs = ScannerArgsType{}

func init() {
	rootCmd.AddCommand(scannerCmd)

	scannerCmd.Flags().StringVar(&ScannerArgs.OutDir, "outdir", EnvOrDefault("outdir", ""), "Output directory for results")
	scannerCmd.Flags().StringVar(&ScannerArgs.Engine, "engine", EnvOrDefault("engine", ""), "Scan engine [grype,trivy]")
	scannerCmd.Flags().StringVar(&ScannerArgs.Worker, "worker", EnvOrDefault("worker", ""), "scanner worker name (consumer)")
	scannerCmd.Flags().StringVar(&ScannerArgs.QueueMax, "queue_max", EnvOrDefault("queue_max", "1000"), "redis max queue stream size")

	scannerCmd.Flags().StringVar(&ScannerArgs.MqEndpoint, "mq_endpoint", EnvOrDefault("mq_endpoint", ""), "Redis message queue, e.g. redis://:pwd@localhost:6379/1")
	scannerCmd.Flags().StringVar(&ScannerArgs.CacheEndpoint, "cache_endpoint", EnvOrDefault("cache_endpoint", ""), "Redis cache, e.g. redis://:pwd@localhost:6379/0")

}

// runCmd represents the run command
var scannerCmd = &cobra.Command{
	Use:   "scanner",
	Short: "Execute scan tasks from MQ",
	Long:  `Execute scan tasks from MQ`,
	Run: func(cmd *cobra.Command, args []string) {

		logger = logging.NewLogger(RootArgs.LogLevel)

		queueMaxLen := utils.ToNumOr[int64](ScannerArgs.QueueMax, 1000)

		ExecuteScanner(ScannerArgs.Engine, ScannerArgs.Worker, ScannerArgs.MqEndpoint, ScannerArgs.CacheEndpoint, ScannerArgs.OutDir, queueMaxLen, logger)

	},
}

func ExecuteScanner(engine, worker, mqEndpoint, cacheEndpoint, outDir string, queueMaxLen int64, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scanner sender >-----")
	logger.Info().
		Str("engine", engine).
		Str("worker", worker).
		Str("mq_endpoint", utils.MaskDsn(mqEndpoint)).
		Any("queue_max", queueMaxLen).
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

	if taskMq, err = mq.NewRedisWorkerGroup[model.PharosScanTask](ctx, mqEndpoint, "$", config.RedisTaskStream, "task-group", queueMaxLen); err != nil {
		logger.Fatal().Err(err).Msg("NewRedisWorkerGroup")
	}
	if resultMq, err = mq.NewRedisWorkerGroup[model.PharosScanResult](ctx, mqEndpoint, "$", config.RedisResultStream, "result-group", queueMaxLen); err != nil {
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

	// prepare scan engine and scanning worker function
	scanTimeout := 180 * time.Second // default timeout
	var scanner scanning.ScanEngineInterface
	if engine == "grype" {
		if scanner, err = scanning.NewGrypeScannerEngine(scanTimeout, true, kvCache, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewGrypeScannerEngine()")
		}
	} else if engine == "trivy" {
		if scanner, err = scanning.NewTrivyScannerEngine(scanTimeout, true, kvCache, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewTrivyScannerEngine()")
		}
	} else {
		logger.Fatal().Str("engine", engine).Msg("Unknown scanner")
	}

	// scanning worker function:
	// execute scan task, report status (errors) and result via MQ
	scanHandler := func(x mq.TaskMessage[model.PharosScanTask]) error {

		task := x.Data
		image := task.ImageSpec.Image
		var err error
		var sbom []byte
		var scan []byte

		var result = model.PharosScanResult{
			ScanTask: task,
		}
		// ensure message is evicted after 2 tries (err=nil will ACK message)
		if x.RetryCount > 2 {
			logger.Error().
				Any(" image", image).Str("job", task.JobId).Any("retry", x.RetryCount).Str("id", x.Id).
				Msg("max retry exceeded")

			resultMq.Publish(ctx, 1, result.SetError(fmt.Errorf("max retry exceeded %v", x.RetryCount)))
			return nil
		}

		logger.Info().
			Any(" image", image).Str("job", task.JobId).Any("retry", x.RetryCount).
			Msg("ScanTask() ..")

		// scan image, use cache
		if result, sbom, scan, err = scanner.ScanImage(task); err != nil {
			logger.Error().Err(err).
				Any(" image", image).Str("job", task.JobId).Any("retry", x.RetryCount).
				Msg("ScanImage()")
			resultMq.Publish(ctx, 1, result.SetError(err))
			return err
		}

		logger.Info().
			Any(" image", image).Str("job", task.JobId).Any("retry", x.RetryCount).
			Str("os", result.Image.DistroName+" "+result.Image.DistroVersion).
			Any("size", humanize.Bytes(result.Image.Size)).
			Any("findings", len(result.Findings)).
			Any("vulns", len(result.Vulnerabilities)).
			Any("packages", len(result.Packages)).
			Msg("ScanTask() OK")

		// submit scan results
		resultMq.Publish(ctx, 1, result)

		// save locally
		saveResults(outDir, utils.ShortDigest(result.Image.ImageId), result.ScanEngine.Name, "sbom", sbom)
		saveResults(outDir, utils.ShortDigest(result.Image.ImageId), result.ScanEngine.Name, "scan", scan)

		return err
	}

	// subscribe to scan tasks, every 1m check and process max 5 pending tasks idle for more than 5m
	// after 30 check for vulndb updates
	runTimeout := 30 * time.Minute  // terminate subscribe
	blockTime := 1 * time.Minute    // max block time of xreadgroup
	claimMinIdle := 5 * time.Minute // min idle time to reclaim Non-ACKed messages
	claimBlock := int64(5)

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

		taskMq.Subscribe(ctx, worker, claimBlock, claimMinIdle, blockTime, runTimeout, scanHandler)

		if err := scanner.UpdateDatabase(); err != nil {
			logger.Fatal().Err(err).Msg("vulndb update failed")
		}
	}

	logger.Info().Msg("done")
}
