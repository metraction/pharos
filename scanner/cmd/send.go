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
	Tasks  string // file with scan tasks to send
	Auths  string // file with auth DSNs to user
	OutDir string // results dump dir

	MqEndpoint  string // redis://user:pwd@localhost:6379/1
	ScanTimeout string // max scan duration
	CacheExpiry string // sbom cache expiry

}

var SendArgs = SendArgsType{}

func init() {
	rootCmd.AddCommand(sendCmd)

	sendCmd.Flags().StringVar(&SendArgs.Tasks, "tasks", EnvOrDefault("tasks", ""), "file with images for scantasks")
	sendCmd.Flags().StringVar(&SendArgs.Auths, "auths", EnvOrDefault("auths", ""), "repo auth DSNs (registry://usr:pwd@docker.io registry://usr:pwd@google.com)")
	sendCmd.Flags().StringVar(&SendArgs.OutDir, "outdir", EnvOrDefault("outdir", ""), "Output directory for results")

	sendCmd.Flags().StringVar(&SendArgs.ScanTimeout, "scan_timeout", EnvOrDefault("scan_timeout", "3m"), "Scanner timeout")
	sendCmd.Flags().StringVar(&SendArgs.CacheExpiry, "cache_expiry", EnvOrDefault("cache_expiry", "1h"), "Redis sbom cache expiry")
	sendCmd.Flags().StringVar(&SendArgs.MqEndpoint, "mq_endpoint", EnvOrDefault("mq_endpoint", ""), "Redis message queue, e.g. redis://:pwd@localhost:6379/1")

}

// runCmd represents the run command
var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send scan tasks",
	Long:  `Send scan tasks`,
	Run: func(cmd *cobra.Command, args []string) {

		cacheExpiry := utils.DurationOr(SendArgs.CacheExpiry, 90*time.Second)
		scanTimeout := utils.DurationOr(SendArgs.ScanTimeout, 180*time.Second)

		ExecuteSend(SendArgs.Tasks, SendArgs.Auths, SendArgs.MqEndpoint, SendArgs.OutDir, scanTimeout, cacheExpiry, logger)

	},
}

func ExecuteSend(tasksFile, authsDsn, mqEndpoint, outDir string, scanTimeout, scanCacheExpiry time.Duration, logger *zerolog.Logger) {

	auths := ParseAuths(authsDsn)

	logger.Info().Msg("-----< Scan sender >-----")
	logger.Info().
		Str("mq_endpoint", utils.MaskDsn(mqEndpoint)).
		Str("tasks", tasksFile).
		Any("auths", len(auths)).
		Str("cache_expiry", scanCacheExpiry.String()).
		Str("scan_timeout", scanTimeout.String()).
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

	images := ReadLines(tasksFile, true)

	logger.Info().
		Str("tasks(file)", tasksFile).
		Any("images", len(images)).
		Str("redis_mq", taskMq.StreamName+":"+taskMq.GroupName).
		Any("lines", len(images)).Msg("load tasks")

	if len(images) == 0 {
		logger.Fatal().Msg("no images to scan")
	}

	stats1, err := taskMq.GroupStats(ctx, "*")
	if err != nil {
		logger.Fatal().Err(err).Msg("taskMq.GroupStats")
	}

	for k, image := range images {
		jobid := fmt.Sprintf("JOB-%v-%v", utils.Hostname(), k)
		auth := model.GetMatchingAuth(image, auths)
		task, _ := model.NewPharosScanTask(jobid, image, "", auth, scanCacheExpiry, scanTimeout)

		// TODO: wait on backpressure
		// send scan task
		id, _ := taskMq.Publish(ctx, 1, task)
		logger.Info().Str("id", id).Str("job", jobid).Any("image", task.ImageSpec.Image).Msg("send scan task")
	}

	stats2, err := taskMq.GroupStats(ctx, "*")
	if err != nil {
		logger.Fatal().Err(err).Msg("taskMq.GroupStats")
	}

	for k, stats := range []mq.GroupStats{stats1, stats2} {
		logger.Info().
			Any("sent", len(images)).
			Any("pending", stats.Pending).
			Any("lag", stats.Lag).
			Any("stream.len", stats.StreamLen).
			Any("stream.max", stats.StreamMax).
			Msg("tasmMQ stats " + lo.Ternary(k == 0, "before", "after "))
	}
	os.Exit(0)
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

// parse auth DNS from lie
func ParseAuths(input string) []model.PharosRepoAuth {

	lines := strings.Split(input, " ")
	lines = lo.Map(lines, func(x string, k int) string { return strings.TrimSpace(x) })
	lines = lo.Filter(lines, func(x string, k int) bool { return x != "" })

	auths := []model.PharosRepoAuth{}
	for _, line := range lines {
		auth, err := model.NewPharosRepoAuth(line)
		if err != nil {
			continue
		}
		auths = append(auths, auth)
	}
	return auths
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
