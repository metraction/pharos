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
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/scanner/config"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

// command line arguments of command
type SendArgsType = struct {
	MqEndpoint string // redis://user:pwd@localhost:6379/1
	Tasks      string // file with scan tasks to send
	Auths      string // file with auth DSNs to user
	OutDir     string // results dump dir
}

var SendArgs = SendArgsType{}

func init() {
	rootCmd.AddCommand(sendCmd)

	sendCmd.Flags().StringVar(&SendArgs.MqEndpoint, "mq_endpoint", EnvOrDefault("mq_endpoint", ""), "Redis message queue, e.g. redis://user:pwd@localhost:6379/1")
	sendCmd.Flags().StringVar(&SendArgs.Tasks, "tasks", EnvOrDefault("tasks", ""), "file with images for scantasks")
	sendCmd.Flags().StringVar(&SendArgs.Auths, "auths", EnvOrDefault("auths", ""), "file with list of auth DNS for scantasks")

	sendCmd.Flags().StringVar(&SendArgs.OutDir, "outdir", EnvOrDefault("outdir", "_output"), "Output directory for results")
}

// runCmd represents the run command
var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send scan tasks via message queue",
	Long:  `Send scan tasks via message queue`,
	Run: func(cmd *cobra.Command, args []string) {

		ExecuteSend(SendArgs.Tasks, SendArgs.Auths, SendArgs.MqEndpoint, SendArgs.OutDir, logger)

	},
}

// helper: return lines of file, ignore comments
func ReadLines(infile string, unique bool) []string {

	data, err := os.ReadFile(infile)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")

	// trim and remove empty lines
	lines = lo.Map(lines, func(x string, k int) string { return strings.TrimSpace(x) })
	lines = lo.Filter(lines, func(x string, k int) bool { return x != "" })
	if unique {
		lines = lo.Uniq(lines)
	}
	return lines
}

func ExecuteSend(tasksFile, authsFile, mqEndpoint, outDir string, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scan sender >-----")
	logger.Info().
		Str("mq_endpoint", utils.MaskDsn(mqEndpoint)).
		Str("tasks", tasksFile).
		Str("auths", authsFile).
		Str("outdir", outDir).
		Msg("")

	// check
	if outDir != "" && !utils.DirExists(outDir) {
		logger.Fatal().Str("outdir", outDir).Msg("dir not found")
	}
	// initialize
	ctx := context.Background()

	var err error
	var taskMq *mq.RedisWorkerGroup[model.PharosScanTask]     // send scan tasks
	var resultMq *mq.RedisWorkerGroup[model.PharosScanResult] // send scan results

	if taskMq, err = mq.NewRedisWorkerGroup[model.PharosScanTask](ctx, mqEndpoint, "$", config.RedisTaskStream, "task-group", config.RedisTaskStreamMaxLen); err != nil {
		logger.Fatal().Err(err).Msg("NewRedisWorkerGroup")
	}
	if resultMq, err = mq.NewRedisWorkerGroup[model.PharosScanResult](ctx, mqEndpoint, "$", config.RedisResultStream, "result-group", config.RedisTaskStreamMaxLen); err != nil {
		logger.Fatal().Err(err).Msg("NewRedisWorkerGroup")
	}
	defer taskMq.Close()
	defer resultMq.Close()

	// try connect 3x with 3 sec sleep to account for startup delays of required pods/services
	if err := integrations.TryConnectServices(ctx, 3, 3*time.Second, []integrations.ServiceInterface{taskMq, resultMq}, logger); err != nil {
		logger.Fatal().Err(err).Msg("services connect")
	}
	logger.Info().Msg("services connect OK")

	// ensure stream groups are present
	taskMq.CreateGroup(ctx)
	resultMq.CreateGroup(ctx)

	// -----< prepare sending scan jobs >-----

	// load authentications
	var lines = []string{}
	auths := []model.PharosRepoAuth{}
	lines = ReadLines(authsFile, true)
	logger.Info().Str("authsFile", authsFile).Any("lines", len(lines)).Msg("load auths")
	for _, line := range lines {
		auth, err := model.NewPharosRepoAuth(line)
		if err != nil {
			logger.Error().Err(err).Str("auth", line).Msg("new-auth")
			continue
		}
		auths = append(auths, auth)
	}

	// send scan tasks
	scanCacheExpiry := 30 * time.Minute
	scanTimeout := 5 * time.Minute

	lines = ReadLines(tasksFile, true)
	logger.Info().
		Str("tasksFile", tasksFile).
		Str("stream:group", taskMq.StreamName+":"+taskMq.GroupName).
		Any("lines", len(lines)).Msg("load tasks")

	for k, image := range lines {
		jobid := fmt.Sprintf("JOB-%v", k)
		auth := model.GetMatchingAuth(image, auths)
		task, _ := model.NewPharosScanTask(jobid, image, "", auth, scanCacheExpiry, scanTimeout)

		// TODO: wait on backpressure
		// send scan task
		id, _ := taskMq.Publish(ctx, 1, task)
		logger.Info().Str("id", id).Any("image", task.ImageSpec.Image).Msg("send scan task")
	}

	// -----< subscribe to scan results >-----

	logger.Info().
		Str("stream:group", resultMq.StreamName+":"+resultMq.GroupName).
		Msg("wait for results ..")

	// process incoming scan results
	resultHandler := func(x mq.TaskMessage[model.PharosScanResult]) error {

		if x.RetryCount > 2 {
			logger.Error().Err(fmt.Errorf("%v: ack & forget", x.Id)) // ensure message is evicted after to many tries
			return nil
		}
		r := x.Data
		logger.Info().
			Str("id", x.Id).
			Str("image", r.Image.ImageSpec).
			Str("distro", r.Image.DistroName+" "+r.Image.DistroVersion).
			Any("findings", len(r.Findings)).
			Any("packages", len(r.Packages)).
			Msg("new scan result")

		// save result
		if utils.DirExists(outDir) {
			os.WriteFile(filepath.Join(outDir, fmt.Sprintf("%v-%s-model.json", x.Id, "grype")), r.ToBytes(), 0644)
		}
		return nil
	}

	claimBlock := int64(5)
	claimMinIdle := 30 * time.Second // min idle time to reclaim Non-ACKed messages
	blockTime := 30 * time.Second    // max block time of xreadgroup
	runTimeout := 5 * time.Minute    // terminate subscribe

	resultMq.Subscribe(ctx, "alfa", claimBlock, claimMinIdle, blockTime, runTimeout, resultHandler)

	logger.Info().Msg("done")

}
