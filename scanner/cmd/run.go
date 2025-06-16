/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"time"

	"github.com/metraction/pharos/internal/utils"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of root command
// implemented as type to facilitate testing of command main routine
type RunArgsType = struct {
	ScanEngine  string // scan engine to use
	RepoAuth    string // Registry authority dsn
	ScanTimeout string // sbom & scan execution timeout
	TlsCheck    string // Skip TLS cert check when pulling images
	TasksFile   string // File with images to scan
	//
	CacheExpiry   string // how log to cache sboms in redis
	CacheEndpoint string // redis://user:pwd@localhost:6379/0

}

var RunArgs = RunArgsType{}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&RunArgs.ScanEngine, "engine", EnvOrDefault("engine", ""), "Scan engine to use [grype,trivy]")
	runCmd.Flags().StringVar(&RunArgs.TasksFile, "tasks", EnvOrDefault("tasks", ""), "File with images to scan")
	runCmd.Flags().StringVar(&RunArgs.RepoAuth, "repo_auth", EnvOrDefault("repo_auth", ""), "Registry auth, e.g. registry://user:pwd@docker.io/?type=password")
	runCmd.Flags().StringVar(&RunArgs.TlsCheck, "tlscheck", EnvOrDefault("tlscheck", "on"), "Check TLS cert (on), skip check (off)")

	runCmd.Flags().StringVar(&RunArgs.ScanTimeout, "scan_timeout", EnvOrDefault("scan_timeout", "180s"), "Scan timeout")
	runCmd.Flags().StringVar(&RunArgs.CacheExpiry, "cache_expiry", EnvOrDefault("cache_expiry", "90s"), "Redis sbom cache expiry")
	runCmd.Flags().StringVar(&RunArgs.CacheEndpoint, "cache_endpoint", EnvOrDefault("cache_endpoint", ""), "Redis cache, e.g. redis://user:pwd@localhost:6379/0")

	runCmd.MarkFlagRequired("engine")
	runCmd.MarkFlagRequired("image")

}

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run scanner as server",
	Long:  `Run the scanner as service listening for scan jobs to execute`,
	Run: func(cmd *cobra.Command, args []string) {

		tlsCheck := utils.ToBool(ScanArgs.TlsCheck)
		scanTimeout := utils.DurationOr(ScanArgs.ScanTimeout, 90*time.Second)
		cacheExpiry := utils.DurationOr(ScanArgs.CacheExpiry, 90*time.Second)

		ExecuteRunScan(RunArgs.ScanEngine, RunArgs.TasksFile, RunArgs.RepoAuth, tlsCheck, scanTimeout, cacheExpiry, RunArgs.CacheEndpoint, logger)

	},
}

func ExecuteRunScan(engine, tasksFile, repoAuth string, tlsCheck bool, scanTimeout, cacheExpiry time.Duration, cacheEndpoint string, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scanner Run/queue processing >-----")
	logger.Info().
		Str("engine", engine).
		Str("tasks", tasksFile).
		Str("repo_auth", utils.MaskDsn(repoAuth)).
		Bool("tlscheck", tlsCheck).
		Any("scan_timeout", scanTimeout.String()).
		Any("cache_expiry", cacheExpiry.String()).
		Str("cache_endpoint", utils.MaskDsn(cacheEndpoint)).
		Msg("")

	_, err := os.Open(tasksFile)
	if err != nil {
		logger.Fatal().Err(err).Str("tasks", tasksFile).Msg("no tasks")
	}

	logger.Info().Msg("done")

}
