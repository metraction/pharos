/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"time"

	"github.com/metraction/pharos/internal/scanner/grype"
	"github.com/metraction/pharos/internal/scanner/syft"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of root command
type ScanArgsType = struct {
	Image    string
	Platform string
	Timeout  string // sbom and scan timeout
}

var ScanArgs = ScanArgsType{}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringVar(&ScanArgs.Image, "image", EnvOrDefault("image", ""), "Image to scan, e.g. docker.io/alpine:3.16")
	scanCmd.Flags().StringVar(&ScanArgs.Platform, "platform", EnvOrDefault("platform", "linux/amd64"), "Image platform")
	scanCmd.Flags().StringVar(&ScanArgs.Timeout, "timeout", EnvOrDefault("timeout", "90s"), "Scan timeout")
}

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Run scanner and exit",
	Long:  `Scan one asset then exit`,
	Run: func(cmd *cobra.Command, args []string) {

		timeout, err := time.ParseDuration(ScanArgs.Timeout)
		if err != nil {
			logger.Fatal().Err(err).Msg("invalid argument")
		}

		ExecuteScan(ScanArgs.Image, ScanArgs.Platform, timeout, logger)
	},
}

func ExecuteScan(imageUri, platform string, timeout time.Duration, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scanner Scan >-----")
	logger.Info().
		Str("image", imageUri).
		Str("platform", platform).
		Any("timeout", timeout.String()).
		Msg("")

	sbomGenerator, err := syft.NewSyftSbomCreator(timeout, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("NewSyftSbomCreator()")
	}
	vulnScanner, err := grype.NewGrypeScanner(timeout, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("NewGrypeScanner()")
	}

	// check vulnerability database
	if err := vulnScanner.CheckUpdate(); err != nil {
		logger.Fatal().Err(err).Msg("CheckUpdate()")
	}

	_, _, err = sbomGenerator.CreateSbom(imageUri, platform)
	if err != nil {
		logger.Fatal().Err(err).Msg("CreateSbom()")
	}

}
