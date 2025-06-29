/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/internal/integrations/mq"
	"github.com/metraction/pharos/internal/logging"
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
	OutDir string // results dump dir

	MqEndpoint string // redis://user:pwd@localhost:6379/1

}

var SendArgs = SendArgsType{}

func init() {
	rootCmd.AddCommand(sendCmd)

	sendCmd.Flags().StringVar(&SendArgs.Tasks, "tasks", EnvOrDefault("tasks", ""), "file with images for scantasks")
	sendCmd.Flags().StringVar(&SendArgs.OutDir, "outdir", EnvOrDefault("outdir", ""), "Output directory for results")
	sendCmd.Flags().StringVar(&SendArgs.MqEndpoint, "mq_endpoint", EnvOrDefault("mq_endpoint", ""), "Redis message queue, e.g. redis://:pwd@localhost:6379/1")

}

// runCmd represents the run command
var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send scan tasks",
	Long:  `Send scan tasks`,
	Run: func(cmd *cobra.Command, args []string) {

		logger = logging.NewLogger(RootArgs.LogLevel)

		ExecuteSend(SendArgs.Tasks, SendArgs.MqEndpoint, SendArgs.OutDir, logger)

	},
}

func ExecuteSend(tasksFile, mqEndpoint, outDir string, logger *zerolog.Logger) {

	//auths := ParseAuths(authsDsn)

	logger.Info().Msg("-----< Scan sender >-----")
	logger.Info().
		Str("mq_endpoint", utils.MaskDsn(mqEndpoint)).
		Str("tasks", tasksFile).
		Str("outdir", outDir).
		Msg("")

	// check
	if outDir != "" && !utils.DirExists(outDir) {
		logger.Fatal().Str("outdir", outDir).Msg("dir not found")
	}
	// initialize
	ctx := context.Background()
	var err error
	var taskMq *mq.RedisWorkerGroup[model.PharosScanTask2] // send scan tasks

	if taskMq, err = mq.NewRedisWorkerGroup[model.PharosScanTask2](ctx, mqEndpoint, "$", config.RedisTaskStream, "task-group", config.RedisTaskStreamMaxLen); err != nil {
		logger.Fatal().Err(err).Msg("NewRedisWorkerGroup")
	}
	defer taskMq.Close()

	// try connect 3x with 3 sec sleep to account for startup delays of required pods/services
	if err := integrations.TryConnectServices(ctx, 3, 3*time.Second, []integrations.ServiceInterface{taskMq}, logger); err != nil {
		logger.Fatal().Err(err).Msg("services connect")
	}
	logger.Info().Msg("services connect OK")

	taskMq.CreateGroup(ctx) // ensure stream groups are present

	// -----< prepare sending scan jobs >-----

	images := readLines(tasksFile, true)

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
	count := 0
	var pressure float64

	for _, line := range images {
		if strings.HasPrefix(line, "#") {
			auth = os.ExpandEnv(utils.RightOfPrefixOr(line, "# auth:", auth))
			platform = os.ExpandEnv(utils.RightOfPrefixOr(line, "# platform:", platform))
			cacheTTL = os.ExpandEnv(utils.RightOfPrefixOr(line, "# cachettl:", cacheTTL))
			scanTTL = os.ExpandEnv(utils.RightOfPrefixOr(line, "# scanttl:", scanTTL))
			maxpressure = os.ExpandEnv(utils.RightOfPrefixOr(line, "# maxpressure:", maxpressure))
			continue
		}
		count++
		task := model.PharosScanTask2{
			JobId:     fmt.Sprintf("job-%v-%v", utils.Hostname(), count),
			Status:    "new",
			Error:     "",
			AuthDsn:   auth,
			ImageSpec: line,
			Platform:  platform,
			ScanTTL:   utils.DurationOr(scanTTL, 3*time.Minute),
			CacheTTL:  utils.DurationOr(cacheTTL, 15*time.Minute),
			Context:   contextGenerator(),
		}
		utils.SetPath(task.Context, "scan/jobid", task.JobId)

		// wait on queue backpressure
		for {
			pressure = taskMq.PressureOr(ctx, 0)
			if pressure < utils.ToNumOr[float64](maxpressure, 0) {
				break
			}
			sleep := 10 * time.Second
			logger.Error().
				Any("pressure", pressure).
				Any("maxpressure", maxpressure).
				Any("sleep", sleep.String()).
				Msg("queue backpressure")
			time.Sleep(sleep)
		}
		// send
		id, err := taskMq.Publish(ctx, 1, task)

		logger.Info().
			// Str(" auth", utils.MaskDsn(task.AuthDsn)).
			Any("err", err).
			Str(" platform", task.Platform).
			Str(" cacheTTL", task.CacheTTL.String()).
			Str(" scanTTL", task.ScanTTL.String()).
			Str("image", task.ImageSpec).
			Any(".ns", utils.PropOr(task.Context, "namespace", "none")).
			Any(".cluster", utils.PropOr(task.Context, "cluster", "none")).
			Any("preassure", pressure).
			Any("id", id).
			Msg(task.JobId)

	}

	stats2, err := taskMq.GroupStats(ctx, "*")
	if err != nil {
		logger.Fatal().Err(err).Msg("taskMq.GroupStats")
	}

	for k, stats := range []mq.GroupStats{stats1, stats2} {
		ShowQueueStats(lo.Ternary(k == 0, "before", "after "), stats, logger)
	}
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
func readLines(infile string, unique bool) []string {

	data, err := os.ReadFile(infile)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")

	// trim and remove empty lines
	lines = lo.Map(lines, func(x string, k int) string { return strings.TrimSpace(x) })
	lines = lo.Filter(lines, func(x string, k int) bool { return x != "" })

	return lines
}

// generate related contexts
func contextGenerator() map[string]any {

	ctxt := map[string]any{}
	clusters := []string{"dev", "txt", "pre", "mte", "prd"}
	owners := []string{"Darth Vader", "Luke Skywalker", "Han Solo", "Princess Leia", "Yoda", "Obi-Wan Kenobi", "R2-D2", "C-3PO", "Chewbacca", "Din Djarin"}
	cities := map[string][]string{
		"Abruzzo":        []string{"Pescara", "Aquila", "Teramo"},
		"AostaValley":    []string{"Aosta", "Saint-Vincent", "Sarre"},
		"Apulia":         []string{"Bari", "Taranto", "Foggia"},
		"Basilicata":     []string{"Potenza", "Matera", "Melfi"},
		"Calabria":       []string{"ReggioCalabria", "Catanzaro", "Lamezia Terme"},
		"Campania":       []string{"Naples", "Salerno", "Giugliano in Campania"},
		"Emilia-Romagna": []string{"Bologna", "Parma", "Modena"},
		"Friuli-Venezia": []string{"Trieste", "Udine", "Pordenone"},
		"Lazio":          []string{"Rome", "Latina", "Guidonia Montecelio"},
		"Liguria":        []string{"Genoa", "LaSpezia", "Savona"},
		"Lombardy":       []string{"Milan", "Brescia", "Monza"},
		"Marche":         []string{"Ancona", "Pesaro", "Fano"},
		"Molise":         []string{"Campobasso", "Termoli", "Isernia"},
		"Piedmont":       []string{"Turin", "Novara", "Alessandria"},
		"Sardinia":       []string{"Cagliari", "Sassari", "Quartu SantElena"},
		"Sicily":         []string{"Palermo", "Catania", "Messina"},
		"Trentino-Alto":  []string{"Bolzano", "Trento", "Merano"},
		"Tuscany":        []string{"Florence", "Prato", "Livorno"},
		"Umbria":         []string{"Perugia", "Terni", "Foligno"},
		"Veneto":         []string{"Venice", "Verona", "Padua"}}

	namespaces := lo.Keys(cities)
	namespace := lo.Sample(namespaces)

	ctxt["cluster"] = lo.Sample(clusters)
	ctxt["namespace"] = namespace
	ctxt["apps"] = lo.Samples(cities[namespace], rand.Intn(3)) // add N apps related to namespace
	ctxt["owner"] = lo.Sample(owners)
	return ctxt
}
