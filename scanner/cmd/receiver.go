/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/internal/integrations/mq"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/scanner/config"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of command
type ReceiverArgsType = struct {
	OutDir string // results dump dir
	Worker string // scanner consumer name (MQ)

	MqEndpoint string // redis://user:pwd@localhost:6379/1
}

var ReceiverArgs = ReceiverArgsType{}

func init() {
	rootCmd.AddCommand(receiverCmd)
	logger = logging.NewLogger(RootArgs.LogLevel)

	receiverCmd.Flags().StringVar(&ReceiverArgs.OutDir, "outdir", EnvOrDefault("outdir", ""), "Output directory for results")
	receiverCmd.Flags().StringVar(&ReceiverArgs.Worker, "worker", EnvOrDefault("worker", ""), "receiver worker name (consumer)")
	receiverCmd.Flags().StringVar(&ReceiverArgs.MqEndpoint, "mq_endpoint", EnvOrDefault("mq_endpoint", ""), "Redis message queue, e.g. redis://:pwd@localhost:6379/1")

}

// runCmd represents the run command
var receiverCmd = &cobra.Command{
	Use:   "receiver",
	Short: "Receive and process scan results",
	Long:  `Receive and process scan results`,
	Run: func(cmd *cobra.Command, args []string) {

		ExecuteReceiver(ReceiverArgs.Worker, ReceiverArgs.MqEndpoint, ReceiverArgs.OutDir, logger)

	},
}

func ExecuteReceiver(worker, mqEndpoint, outDir string, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Receiver  >-----")
	logger.Info().
		Str("worker", worker).
		Str("mq_endpoint", utils.MaskDsn(mqEndpoint)).
		Str("outdir", outDir).
		Msg("")

	// check
	if outDir != "" && !utils.DirExists(outDir) {
		logger.Fatal().Str("outdir", outDir).Msg("dir not found")
	}
	// initialize
	ctx := context.Background()

	var err error
	var resultMq *mq.RedisWorkerGroup[model.PharosScanResult] // send scan results

	if resultMq, err = mq.NewRedisWorkerGroup[model.PharosScanResult](ctx, mqEndpoint, "$", config.RedisResultStream, "result-group", config.RedisTaskStreamMaxLen); err != nil {
		logger.Fatal().Err(err).Msg("NewRedisWorkerGroup")
	}
	defer resultMq.Close()

	// try connect 3x with 3 sec sleep to account for startup delays of required pods/services
	if err := integrations.TryConnectServices(ctx, 3, 3*time.Second, []integrations.ServiceInterface{resultMq}, logger); err != nil {
		logger.Fatal().Err(err).Msg("services connect")
	}
	logger.Info().Msg("services connect OK")

	// ensure stream groups are present
	resultMq.CreateGroup(ctx)

	// scan result handler
	scanResultHandler := func(x mq.TaskMessage[model.PharosScanResult]) error {

		result := x.Data
		task := result.ScanTask
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
			Str("os", result.Image.DistroName+" "+result.Image.DistroVersion).
			Any("findings", len(result.Findings)).
			Any("packages", len(result.Packages)).
			Msg("result OK")

		// process result
		saveResults(outDir, utils.ShortDigest(result.Image.ImageId), result.ScanEngine.Name, result)

		return nil
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
		stats, err := resultMq.GroupStats(ctx, "*")
		if err != nil {
			logger.Fatal().Err(err).Msg("resultMq.GroupStats")
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

		resultMq.Subscribe(ctx, worker, claimBlock, claimMinIdle, blockTime, runTimeout, scanResultHandler)

	}
	logger.Info().Msg("done")
}

// saveResults(outDir, utils.ShortDigest(result.Image.ImageId), "grype", result)
func saveResults(outDir, id, engine string, result model.PharosScanResult) {
	filename := strings.Replace(fmt.Sprintf("%s-%s-model.json", id, engine), ":", "-", -1)
	outFile := filepath.Join(outDir, filename)
	os.WriteFile(outFile, result.ToBytes(), 0644)

}
