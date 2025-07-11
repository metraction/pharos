/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/metraction/pharos/internal/integrations/mq"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of command
type SendApiArgsType = struct {
	Tasks  string // file with scan tasks to send
	OutDir string // results dump dir
}

var SendApiArgs = SendArgsType{}

func init() {
	rootCmd.AddCommand(sendApiCmd)

	sendApiCmd.Flags().StringVar(&SendApiArgs.Tasks, "tasks", EnvOrDefault("tasks", ""), "file with images for scantasks")
	sendApiCmd.Flags().StringVar(&SendApiArgs.OutDir, "outdir", EnvOrDefault("outdir", ""), "Output directory for results")

}

// runCmd represents the run command
var sendApiCmd = &cobra.Command{
	Use:   "sendapi",
	Short: "Send scan tasks to API",
	Long:  `Send scan tasks to API`,
	Run: func(cmd *cobra.Command, args []string) {

		logger = logging.NewLogger(RootArgs.LogLevel)

		ExecuteSendApi(SendApiArgs.Tasks, SendApiArgs.OutDir, logger)

	},
}

func ExecuteSendApi(tasksFile, outDir string, logger *zerolog.Logger) {

	//auths := ParseAuths(authsDsn)

	logger.Info().Msg("-----< Scan sender (API) >-----")
	logger.Info().
		Str("tasks", tasksFile).
		Str("outdir", outDir).
		Msg("")

	// check
	if outDir != "" && !utils.DirExists(outDir) {
		logger.Fatal().Str("outdir", outDir).Msg("dir not found")
	}

	// -----< prepare sending scan jobs >-----

	images := readLines(tasksFile, true)
	logger.Info().
		Str("tasks(file)", tasksFile).
		Any("lines", len(images)).
		Any("lines", len(images)).Msg("load tasks")

	if len(images) == 0 {
		logger.Fatal().Msg("no images to scan")
	}

	// submit scan requests for all images in list
	// set <auth> and <platform> for all images following settings like
	// # auth: registry://<user>:<pwd>@repo.host.lan
	// # platform: linux/amd64
	// # cachettl: 60m
	// # scanttl: 3m
	// # backpressure: 0.1
	auth := ""
	platform := "linux/amd64"
	cacheTTL := "60m"
	scanTTL := "3m"
	maxpressure := "0.1"
	priority := "2"
	count := 0
	targetapi := "" // where to send tasks to https://api.pharos/...

	for k, line := range images {
		logger.Info().Msg(line)

		// read task commands/settings
		if strings.HasPrefix(line, "# exit") {
			logger.Info().Any("line", k+1).Msg("exit")
			break
		}
		if strings.HasPrefix(line, "#") {
			auth = os.ExpandEnv(utils.RightOfPrefixOr(line, "# auth:", auth))
			scanTTL = os.ExpandEnv(utils.RightOfPrefixOr(line, "# scanttl:", scanTTL))
			cacheTTL = os.ExpandEnv(utils.RightOfPrefixOr(line, "# cachettl:", cacheTTL))
			platform = os.ExpandEnv(utils.RightOfPrefixOr(line, "# platform:", platform))
			maxpressure = os.ExpandEnv(utils.RightOfPrefixOr(line, "# maxpressure:", maxpressure))
			priority = os.ExpandEnv(utils.RightOfPrefixOr(line, "# priority:", priority))
			targetapi = os.ExpandEnv(utils.RightOfPrefixOr(line, "# targetapi:", targetapi))

			continue
		}

		count++
		assetContext := mq.ContextGenerator()
		task := model.PharosScanTask2{
			JobId:          fmt.Sprintf("job-%v-%v", utils.Hostname(), count),
			Status:         "new",
			Error:          "",
			AuthDsn:        auth,
			ImageSpec:      line,
			Platform:       platform,
			ScanTTL:        utils.DurationOr(scanTTL, 3*time.Minute),
			CacheTTL:       utils.DurationOr(cacheTTL, 15*time.Minute),
			ContextRootKey: utils.ResolveMap("{{.cluster}}/{{.namespace}}", assetContext),
			Context:        assetContext,
		}

		result, err := ScanAsync(targetapi, task)
		if err != nil {
			logger.Error().Err(err).Any("result", result).Msg("ScanAsync")
		}

		logger.Info().
			// Str(" auth", utils.MaskDsn(task.AuthDsn)).
			Any("err", err).
			Str(" platform", task.Platform).
			// Str(" cacheTTL", task.CacheTTL.String()).
			// Str(" scanTTL", task.ScanTTL.String()).
			// Any("result", string(result)).
			Str("image", task.ImageSpec).
			Any(".ns", utils.PropOr(task.Context, "namespace", "none")).
			Any(".cluster", utils.PropOr(task.Context, "cluster", "none")).
			Msg(task.JobId)
	}

	logger.Info().Msg("done")

}

// send async scan task
func ScanAsync(url string, task model.PharosScanTask2) ([]byte, error) {

	var err error
	var result []byte
	var jsonData []byte

	if jsonData, err = json.Marshal(task); err != nil {
		return result, err
	}

	fmt.Println("POST url", url)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return result, err
	}
	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	if result, err = io.ReadAll(resp.Body); err != nil {
		return result, err
	}
	return result, err
}
