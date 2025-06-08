/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"time"

	"github.com/metraction/pharos/internal/scanner/grype"
	"github.com/metraction/pharos/internal/scanner/syft"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of root command
type ScanArgsType = struct {
	Image       string
	Platform    string
	ScanTimeout string // sbom & scan execution timeout
}

var ScanArgs = ScanArgsType{}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringVar(&ScanArgs.Image, "image", EnvOrDefault("image", ""), "Image to scan, e.g. docker.io/alpine:3.16")
	scanCmd.Flags().StringVar(&ScanArgs.Platform, "platform", EnvOrDefault("platform", "linux/amd64"), "Image platform")
	scanCmd.Flags().StringVar(&ScanArgs.ScanTimeout, "scan_timeout", EnvOrDefault("scan_timeout", "180s"), "Scan timeout")
}

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Run scanner and exit",
	Long:  `Scan one asset then exit`,
	Run: func(cmd *cobra.Command, args []string) {

		scanTimeout, err := time.ParseDuration(ScanArgs.ScanTimeout)
		if err != nil {
			logger.Fatal().Err(err).Msg("invalid argument")
		}

		ExecuteScan(ScanArgs.Image, ScanArgs.Platform, scanTimeout, logger)
	},
}

func ExecuteScan(imageUri, platform string, scanTimeout time.Duration, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scanner Scan >-----")
	logger.Info().
		Str("image", imageUri).
		Str("platform", platform).
		Any("scan_timeout", scanTimeout.String()).
		Msg("")

	var err error
	var sbomGenerator *syft.SyftSbomCreator
	var vulnScanner *grype.GrypeScanner
	//var sbomCydx *cyclonedx.BOM
	var sbomData *[]byte
	var scanData *[]byte

	// create sbom and scanner generators
	if sbomGenerator, err = syft.NewSyftSbomCreator(scanTimeout, logger); err != nil {
		logger.Fatal().Err(err).Msg("NewSyftSbomCreator()")
	}
	if vulnScanner, err = grype.NewGrypeScanner(scanTimeout, logger); err != nil {
		logger.Fatal().Err(err).Msg("NewGrypeScanner()")
	}

	// ensure initial update of vuln database
	if err = vulnScanner.UpdateDatabase(); err != nil {
		logger.Fatal().Err(err).Msg("UpdateDatabase()")
	}

	// get image and create sbom
	_, sbomData, err = sbomGenerator.CreateSbom(imageUri, platform)
	if err != nil {
		logger.Fatal().Err(err).Msg("CreateSbom()")
	}

	scanData, err = vulnScanner.VulnScanSbom(sbomData)
	if err != nil {
		logger.Fatal().Err(err).Msg("VulnScanSbom()")
	}

	os.WriteFile("scan.json", *scanData, 0644)
	logger.Info().Msg("done")
}
