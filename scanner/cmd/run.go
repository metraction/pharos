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

	"github.com/metraction/pharos/internal/integrations/cache"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/grype"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/pkg/trivy"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

// command line arguments of root command
// implemented as type to facilitate testing of command main routine
type RunArgsType = struct {
	ScanEngine  string // scan engine to use
	RepoAuth    string // Registry authority dsn
	ScanTimeout string // sbom & scan execution timeout
	TasksFile   string // File with images to scan
	OutDir      string // Output directory to store results
	//
	CacheExpiry   string // how log to cache sboms in redis
	CacheEndpoint string // redis://user:pwd@localhost:6379/0
	MqEndpoint    string // redis://user:pwd@localhost:6379/1

}

var RunArgs = RunArgsType{}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&RunArgs.ScanEngine, "engine", EnvOrDefault("engine", ""), "Scan engine to use [grype,trivy]")
	runCmd.Flags().StringVar(&RunArgs.TasksFile, "tasks", EnvOrDefault("tasks", ""), "File with images to scan")
	runCmd.Flags().StringVar(&RunArgs.OutDir, "outdir", EnvOrDefault("outdir", ""), "Output directory for results")
	runCmd.Flags().StringVar(&RunArgs.RepoAuth, "repo_auth", EnvOrDefault("repo_auth", ""), "Registry auth, e.g. registry://user:pwd@docker.io/?type=password")

	runCmd.Flags().StringVar(&RunArgs.ScanTimeout, "scan_timeout", EnvOrDefault("scan_timeout", "180s"), "Scan timeout")
	runCmd.Flags().StringVar(&RunArgs.CacheExpiry, "cache_expiry", EnvOrDefault("cache_expiry", "90s"), "Redis sbom cache expiry")
	runCmd.Flags().StringVar(&RunArgs.CacheEndpoint, "cache_endpoint", EnvOrDefault("cache_endpoint", ""), "Redis cache, e.g. redis://user:pwd@localhost:6379/0")
	runCmd.Flags().StringVar(&RunArgs.MqEndpoint, "mq_endpoint", EnvOrDefault("mq_endpoint", ""), "Redis message queue, e.g. redis://user:pwd@localhost:6379/1")

	runCmd.MarkFlagRequired("engine")
	runCmd.MarkFlagRequired("image")

}

// dump scan results to files (for debugging)
func saveResults(outdir string, id int, prefix string, sbomData []byte, scanData []byte, result model.PharosScanResult) {

	if utils.DirExists(outdir) {
		os.WriteFile(filepath.Join(outdir, fmt.Sprintf("%v-%s-sbom.json", id, prefix)), sbomData, 0644)
		os.WriteFile(filepath.Join(outdir, fmt.Sprintf("%v-%s-scan.json", id, prefix)), scanData, 0644)
		os.WriteFile(filepath.Join(outdir, fmt.Sprintf("%v-%s-model.json", id, prefix)), result.ToBytes(), 0644)

		// logging
		delta := result.ScanTask.Updated.Sub(result.ScanTask.Created)
		msg := fmt.Sprintf(
			"%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
			id,
			result.ScanTask.Created.Format("2006-01-02 15:04:05"),
			result.ScanTask.SbomEngine,
			result.ScanTask.ScanEngine,
			result.ScanTask.Status,
			delta.Seconds(),
			len(result.Findings),
			len(result.Vulnerabilities),
			len(result.Packages),
			result.ScanTask.ImageSpec.Image)

		flog, err := os.OpenFile(filepath.Join(outdir, "scanlog.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer flog.Close()
		flog.WriteString(msg)
	}

	return
}

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run scanner as server",
	Long:  `Run the scanner as service listening for scan jobs to execute`,
	Run: func(cmd *cobra.Command, args []string) {

		scanTimeout := utils.DurationOr(ScanArgs.ScanTimeout, 90*time.Second)
		cacheExpiry := utils.DurationOr(ScanArgs.CacheExpiry, 90*time.Second)

		ExecuteRunScan(RunArgs.ScanEngine, RunArgs.TasksFile, RunArgs.RepoAuth, scanTimeout, cacheExpiry, RunArgs.OutDir, RunArgs.CacheEndpoint, RunArgs.MqEndpoint, logger)

	},
}

func ExecuteRunScan(engine, tasksFile, repoAuth string, scanTimeout, cacheExpiry time.Duration, outDir string, cacheEndpoint, mqEndpoint string, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scanner Run/queue processing >-----")
	logger.Info().
		Str("engine", engine).
		Str("tasks", tasksFile).
		Str("outdir", outDir).
		Str("repo_auth", utils.MaskDsn(repoAuth)).
		Any("scan_timeout", scanTimeout.String()).
		Any("cache_expiry", cacheExpiry.String()).
		Str("cache_endpoint", utils.MaskDsn(cacheEndpoint)).
		Str("mq_endpoint", utils.MaskDsn(mqEndpoint)).
		Msg("")

	// initialize
	var err error
	var sbomData []byte // sbom raw result
	var scanData []byte // scan raw result
	var task model.PharosScanTask
	var auth model.PharosRepoAuth
	var result model.PharosScanResult // Pharos scan result type

	ctx := context.Background()

	var kvc *cache.PharosCache // redis cache

	// create redis KV cache
	if kvc, err = cache.NewPharosCache(cacheEndpoint, logger); err != nil {
		logger.Fatal().Err(err).Msg("Redis cache create")
	}
	defer kvc.Close()

	// try connect: account for starupt delay of required pods/services
	// TODO Stefan: refactor with loop over servies (with interface for Connect(), CheckConnect(), .. )
	maxAttempts := 3
	var err1 error
	var err2 error

	for connectCount := 1; connectCount < maxAttempts+1; connectCount++ {
		logger.Info().Any("attempt", connectCount).Any("max", maxAttempts).Msg("service connect ..")
		err1 = kvc.Connect(ctx)
		err2 = nil
		if err1 == nil && err2 == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err1 != nil || err2 != nil {
		logger.Fatal().Err(err1).Err(err2).Msg("service connect errors")
	}
	logger.Info().Msg("service connect OK")

	// prepare auth (same for all images for testing)
	if auth, err = model.NewPharosRepoAuth(repoAuth); err != nil {
		logger.Fatal().Err(err).Msg("invalid repo auth definition")
	}

	// get images to scan as []
	data, err := os.ReadFile(tasksFile)
	if err != nil {
		logger.Fatal().Err(err).Str("tasks", tasksFile).Msg("no tasks")
	}
	images := strings.Split(string(data), "\n")
	images = lo.Map(images, func(x string, k int) string { return strings.TrimSpace(x) })
	images = lo.Filter(images, func(x string, k int) bool { return x != "" })
	images = lo.Uniq(images)

	logger.Info().Any("unique images", len(images)).Msg("scan queue")

	// prepare output directiry (if exists)
	if outDir != "" && !utils.DirExists(outDir) {
		logger.Fatal().Err(err).Str("outdir", outDir).Msg("outdir must exist")
	}

	// ------< scanning >-----

	if engine == "grype" {
		var scanEngine *grype.GrypeScanner

		// create scanner & update database (once)
		if scanEngine, err = grype.NewGrypeScanner(scanTimeout, true, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewGrypeScanner()")
		}

		// loop simulates arrival of tasks from worker queue
		for k, imageRef := range images {

			logger.Info().Any("#", k).Str("image", imageRef).Msg("-----< new task >-----")

			// make scantask (scantasks would be received from worker queue, here we build it)
			if task, err = model.NewPharosScanTask("", imageRef, "", auth, cacheExpiry, scanTimeout); err != nil {
				logger.Fatal().Err(err).Msg("invalid scan task definition")
			}

			// BEGIN WORKER
			// scan image, use cache
			result, sbomData, scanData, err = grype.ScanImage(task, scanEngine, kvc, logger)
			if err != nil {
				logger.Error().Err(err).Msg("grype.ScanImage()")
			}
			saveResults(outDir, k, "grype", sbomData, scanData, result)

			// call scanEngine.UpdateDatabase() every hour
			// END WORKER
		}

	} else if engine == "trivy" {
		var scanEngine *trivy.TrivyScanner

		// create scanner & update database
		if scanEngine, err = trivy.NewTrivyScanner(scanTimeout, true, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewTrivyScanner()")
		}

		// loop simulates arrival of tasks from worker queue
		for k, imageRef := range images {
			logger.Info().Any("#", k).Str("image", imageRef).Msg("-----< new task >-----")

			// make scantask (scantasks would be received from worker queue, here we build it)
			if task, err = model.NewPharosScanTask("", imageRef, "", auth, cacheExpiry, scanTimeout); err != nil {
				logger.Fatal().Err(err).Msg("invalid scan task definition")
			}

			// scan image, use cache
			result, sbomData, scanData, err = trivy.ScanImage(task, scanEngine, kvc, logger)
			if err != nil {
				logger.Error().Err(err).Msg("trivy.ScanImage()")
			}
			saveResults(outDir, k, "trivy", sbomData, scanData, result)
		}
	} else {
		logger.Fatal().Str("engine", engine).Msg("unknown engine")
	}

	logger.Info().Any("Data", images).Msg("done")

}
