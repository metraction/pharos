/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
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

// command line arguments of root command
// implemented as type to facilitate testing of command main routine
type ReceiveArgsType = struct {
	ScanEngine string // scan engine to use
	//
	CacheExpiry   string // how log to cache sboms in redis
	CacheEndpoint string // redis://user:pwd@localhost:6379/0
	MqEndpoint    string // redis://user:pwd@localhost:6379/1
	//
	OutDir string // Output directory to store results

}

var ReceiveArgs = ReceiveArgsType{}

func init() {
	rootCmd.AddCommand(receiveCmd)

	receiveCmd.Flags().StringVar(&ReceiveArgs.MqEndpoint, "mq_endpoint", EnvOrDefault("mq_endpoint", ""), "Redis message queue, e.g. redis://user:pwd@localhost:6379/1")

	receiveCmd.Flags().StringVar(&ReceiveArgs.OutDir, "outdir", EnvOrDefault("_outdir", ""), "Output directory for results")

}

// runCmd represents the run command
var receiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Run scan results receiver",
	Long:  `Run scan results receiver`,
	Run: func(cmd *cobra.Command, args []string) {

		cacheExpiry := utils.DurationOr(ScanArgs.CacheExpiry, 90*time.Second)

		ExecuteService(ReceiveArgs.ScanEngine, cacheExpiry, ReceiveArgs.OutDir, ReceiveArgs.CacheEndpoint, ReceiveArgs.MqEndpoint, logger)

	},
}

func ExecuteService(engine string, cacheExpiry time.Duration, outDir, cacheEndpoint, mqEndpoint string, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scanner service >-----")
	logger.Info().
		Str("engine", engine).
		Any("cache_expiry", cacheExpiry.String()).
		Str("mq_endpoint", mqEndpoint). //utils.MaskDsn(
		Str("cache_endpoint", cacheEndpoint).
		Str("outdir", outDir).
		Msg("")

	// initialize
	ctx := context.Background()

	var err error
	var kvc *cache.PharosCache                                // redis cache
	var taskMq *mq.RedisWorkerGroup[model.PharosScanTask]     // send scan tasks
	var resultMq *mq.RedisWorkerGroup[model.PharosScanResult] // send scan results

	if kvc, err = cache.NewPharosCache(cacheEndpoint, logger); err != nil {
		logger.Fatal().Err(err).Msg("NewPharosCache")
	}
	defer kvc.Close()

	if taskMq, err = mq.NewRedisWorkerGroup[model.PharosScanTask](ctx, mqEndpoint, "$", config.RedisTaskStream, "task-group", config.RedisTaskStreamMaxLen); err != nil {
		logger.Fatal().Err(err).Msg("NewRedisWorkerGroup")
	}
	if resultMq, err = mq.NewRedisWorkerGroup[model.PharosScanResult](ctx, mqEndpoint, "$", config.RedisResultStream, "result-group", config.RedisTaskStreamMaxLen); err != nil {
		logger.Fatal().Err(err).Msg("NewRedisWorkerGroup")
	}
	defer taskMq.Close()
	defer resultMq.Close()

	// try connect 3x with 3 sec sleep to account for startup delays of required pods/services
	services := []integrations.ServiceInterface{kvc, taskMq, resultMq}
	if err := integrations.TryConnectServices(ctx, 3, 3*time.Second, services, logger); err != nil {
		logger.Fatal().Err(err).Msg("services connect")
	}
	logger.Info().Msg("services connect OK")

	// ensure stream groups are present
	resultMq.CreateGroup(ctx)

	// -----< subscribe to scan tasks >-----
	scanTimeout := 180 * time.Second

	logger.Info().
		Str("stream:group", taskMq.StreamName+":"+taskMq.GroupName).
		Msg("wait for tasks ..")

	if engine == "grype" {
		var scanEngine *grype.GrypeScanner

		if scanEngine, err = grype.NewGrypeScanner(scanTimeout, true, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewGrypeScanner()")
		}

		// scanHandler
		grypeHandler := func(x mq.TaskMessage[model.PharosScanTask]) error {

			if x.RetryCount > 2 {
				logger.Error().Err(fmt.Errorf("%v: ack & forget", x.Id)) // ensure message is evicted after to many tries
				return nil
			}
			task := x.Data

			logger.Info().Str("id", x.Id).Any("retry", x.RetryCount).Any("image", task.ImageSpec.Image).Msg("new scan task")

			// scan image, use cache
			result, _, _, err := grype.ScanImage(task, scanEngine, kvc, logger)
			if err != nil {
				logger.Error().Err(err).Str("image", x.Data.ImageSpec.Image).Msg("grype.ScanImage()")
				return err
			}
			// save result
			// TODO: submit result to controller
			logger.Info().
				Str("image", x.Data.ImageSpec.Image).
				Str("os", result.Image.DistroName+" "+result.Image.DistroVersion).
				Any("findings", len(result.Findings)).
				Any("packages", len(result.Packages)).
				Msg("grype.ScanImage()")

			// submit scan results
			id, _ := resultMq.Publish(ctx, 1, result)
			logger.Info().Str("id", id).Any("image", task.ImageSpec.Image).Msg("send scan result")

			// saveResults(outDir, x.Id, "grype", sbomData, scanData, result)
			// success
			return err
		}

		claimBlock := int64(5)
		claimMinIdle := 30 * time.Second // min idle time to reclaim Non-ACKed messages
		blockTime := 30 * time.Second    // max block time of xreadgroup
		runTimeout := 5 * time.Minute    // terminate subscribe

		taskMq.Subscribe(ctx, "bravo", claimBlock, claimMinIdle, blockTime, runTimeout, grypeHandler)

	}

	logger.Info().Msg("service connect OK")

}
