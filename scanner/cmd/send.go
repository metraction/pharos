/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/metraction/pharos/internal/integrations/mq"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

// command line arguments of root command
// implemented as type to facilitate testing of command main routine
type SendArgsType = struct {
	MqEndpoint string // redis://user:pwd@localhost:6379/1
	Tasks      string // file with scan tasks to send
	Auths      string // file with auth DSNs to user
	Queue      string // taskqueue definition "queue://mx:scantask/?maxlen=1000&maxretry=2&maxttl=1h"

}

var SendArgs = SendArgsType{}

func init() {
	rootCmd.AddCommand(sendCmd)

	sendCmd.Flags().StringVar(&SendArgs.MqEndpoint, "mq_endpoint", EnvOrDefault("mq_endpoint", ""), "Redis message queue, e.g. redis://user:pwd@localhost:6379/1")
	sendCmd.Flags().StringVar(&SendArgs.Queue, "queue", EnvOrDefault("queue", "queue://mx:scantask/?maxlen=1000&maxretry=2&maxttl=1h"), "taskqueue definition")
	sendCmd.Flags().StringVar(&SendArgs.Tasks, "tasks", EnvOrDefault("tasks", ""), "file with images for scantasks")
	sendCmd.Flags().StringVar(&SendArgs.Auths, "auths", EnvOrDefault("auths", ""), "file with list of auth DNS for scantasks")

}

// runCmd represents the run command
var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send scan tasks via message queue",
	Long:  `Send scan tasks via message queue`,
	Run: func(cmd *cobra.Command, args []string) {

		//cacheExpiry := utils.DurationOr(SendArgs.CacheExpiry, 90*time.Second)

		ExecuteSend(SendArgs.Tasks, SendArgs.Auths, SendArgs.Queue, SendArgs.MqEndpoint, logger)

	},
}

// read file, return lines, ignore comments
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

func ExecuteSend(tasksFile, authsFile, queue, mqEndpoint string, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scan sender >-----")
	logger.Info().
		Str("mq_endpoint", utils.MaskDsn(mqEndpoint)).
		Str("queue", queue).
		Str("tasks", tasksFile).
		Str("auths", authsFile).
		Msg("")

	// initialize
	var err error
	ctx := context.Background()

	streamName, groupName, maxStreamLen, maxMsgRetry, maxMsgTTL, err := mq.ParseTaskQueueDsn(queue)
	if err != nil {
		logger.Fatal().Err(err).Msg("queue")
	}

	var tmq *mq.RedisTaskQueue[model.PharosScanTask]
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
		err1 = nil
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
	cacheExpiry := 30 * time.Minute
	scanTimeout := 5 * time.Minute

	lines = ReadLines(tasksFile, true)
	logger.Info().Str("tasksFile", tasksFile).Any("lines", len(lines)).Msg("load tasks")

	for k, image := range lines {
		auth := model.GetMatchingAuth(image, auths)
		task, _ := model.NewPharosScanTask(string(k), image, "", auth, cacheExpiry, scanTimeout)

		logger.Info().Any("image", task.ImageSpec.Image).Msg("")
		id, err := tmq.AddMessage(ctx, 1, task)

		logger.Info().Err(err).Str("id", id).Msg("send")

	}
}
