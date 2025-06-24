/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"time"

	"github.com/metraction/pharos/internal/integrations/cache"
	"github.com/metraction/pharos/internal/integrations/mq"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/grype"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of root command
// implemented as type to facilitate testing of command main routine
type ServiceArgsType = struct {
	ScanEngine string // scan engine to use
	//
	CacheExpiry   string // how log to cache sboms in redis
	CacheEndpoint string // redis://user:pwd@localhost:6379/0
	MqEndpoint    string // redis://user:pwd@localhost:6379/1
	//
	OutDir string // Output directory to store results

}

var ServiceArgs = RunArgsType{}

func init() {
	rootCmd.AddCommand(serviceCmd)

	serviceCmd.Flags().StringVar(&ServiceArgs.ScanEngine, "engine", EnvOrDefault("engine", ""), "Scan engine to use [grype,trivy]")

	serviceCmd.Flags().StringVar(&ServiceArgs.CacheExpiry, "cache_expiry", EnvOrDefault("cache_expiry", "90s"), "Redis sbom cache expiry")
	serviceCmd.Flags().StringVar(&ServiceArgs.CacheEndpoint, "cache_endpoint", EnvOrDefault("cache_endpoint", ""), "Redis cache, e.g. redis://user:pwd@localhost:6379/0")
	serviceCmd.Flags().StringVar(&ServiceArgs.MqEndpoint, "mq_endpoint", EnvOrDefault("mq_endpoint", ""), "Redis message queue, e.g. redis://user:pwd@localhost:6379/1")

	serviceCmd.Flags().StringVar(&ServiceArgs.OutDir, "outdir", EnvOrDefault("_outdir", ""), "Output directory for results")

}

// runCmd represents the run command
var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Run scanner as worker",
	Long:  `Run the scanner as service listening for scan jobs to execute`,
	Run: func(cmd *cobra.Command, args []string) {

		cacheExpiry := utils.DurationOr(ScanArgs.CacheExpiry, 90*time.Second)

		ExecuteService(ServiceArgs.ScanEngine, cacheExpiry, ServiceArgs.OutDir, ServiceArgs.CacheEndpoint, ServiceArgs.MqEndpoint, logger)

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
	var err error
	ctx := context.Background()

	// TODO: From arguments, mq_options="stream=mx&maxlen=1000&maxretry=2&maxttl=1h"
	streamName := "mx"
	groupName := "scantask"
	maxStreamLen := int64(1000)
	maxMsgRetry := int64(2)
	maxMsgTTL := 60 * time.Minute
	// TODO: From arguments / scantask
	scanTimeout := 5 * time.Minute

	var kvc *cache.PharosCache // redis cache
	var tmq *mq.RedisTaskQueue[model.PharosScanTask]

	// create redis KV cache
	if kvc, err = cache.NewPharosCache(cacheEndpoint, logger); err != nil {
		logger.Fatal().Err(err).Msg("Redis cache create")
	}
	defer kvc.Close()

	if tmq, err = mq.NewRedisTaskQueue[model.PharosScanTask](ctx, mqEndpoint, streamName, maxStreamLen, maxMsgRetry, maxMsgTTL); err != nil {
		logger.Fatal().Err(err).Msg("Redis mq create")
	}
	defer tmq.Close()

	// try connect: account for starupt delay of required pods/services
	// TODO Stefan: refactor with loop over servies (with interface for Connect(), CheckConnect(), .. )
	maxAttempts := 3
	var err1 error
	var err2 error

	for connectCount := 1; connectCount < maxAttempts+1; connectCount++ {
		logger.Info().Any("attempt", connectCount).Any("max", maxAttempts).Msg("service connect ..")
		err1 = kvc.Connect(ctx)
		err2 = tmq.Connect(ctx)
		if err1 == nil && err2 == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err1 != nil || err2 != nil {
		logger.Fatal().Err(err1).Err(err2).Msg("service connect errors")
	}

	// register group
	if err := tmq.CreateGroup(ctx, groupName, "$"); err != nil {
		logger.Fatal().Err(err1).Err(err2).Msg("tmq.CreateGroup()")
	}

	if engine == "grype" {
		var scanEngine *grype.GrypeScanner

		// create scanner & update database (once)
		if scanEngine, err = grype.NewGrypeScanner(scanTimeout, true, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewGrypeScanner()")
		}

		// scanner invocation handler
		grypeHandler := func(x mq.TaskMessage[model.PharosScanTask]) error {

			logger.Info().Any("retry", x.RetryCount).Any("image", x.Data.ImageSpec.Image).Msg("scan-grype")

			// scan image, use cache
			result, sbomData, scanData, err := grype.ScanImage(x.Data, scanEngine, kvc, logger)
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

			saveResults(outDir, x.Id, "grype", sbomData, scanData, result)
			// success
			return err
		}

		// event loop
		logger.Info().Str("stream", streamName).Str("group", groupName).Str("consumer", utils.Hostname()).Msg("GroupSubscribe() ..")

		err := tmq.GroupSubscribe(ctx, ">", groupName, utils.Hostname(), 0*time.Second, grypeHandler)
		if err != nil {
			logger.Error().Err(err).Msg("GroupSubscribe(grype)")
		}

	}

	logger.Info().Msg("service connect OK")

}
