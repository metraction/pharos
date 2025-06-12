/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/metraction/pharos/internal/scanner"
	"github.com/metraction/pharos/internal/scanner/grype"
	"github.com/metraction/pharos/internal/scanner/syft"
	"github.com/metraction/pharos/internal/scanner/trivy"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of root command
type ScanArgsType = struct {
	ScanEngine  string // scan engine to use
	ScanTimeout string // sbom & scan execution timeout
	Image       string
	Platform    string
}

var ScanArgs = ScanArgsType{}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringVar(&ScanArgs.Image, "image", EnvOrDefault("image", ""), "Image to scan, e.g. docker.io/alpine:3.16")
	scanCmd.Flags().StringVar(&ScanArgs.Platform, "platform", EnvOrDefault("platform", "linux/amd64"), "Image platform")

	scanCmd.Flags().StringVar(&ScanArgs.ScanEngine, "engine", EnvOrDefault("engine", ""), "Scan engine to use [grype,trivy]")
	scanCmd.Flags().StringVar(&ScanArgs.ScanTimeout, "scan_timeout", EnvOrDefault("scan_timeout", "180s"), "Scan timeout")

	scanCmd.MarkFlagRequired("engine")
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

		ExecuteScan(ScanArgs.ScanEngine, ScanArgs.Image, ScanArgs.Platform, scanTimeout, logger)
	},
}

func ExecuteScan(engine, imageRef, platform string, scanTimeout time.Duration, logger *zerolog.Logger) {

	logger.Info().Msg("-----< Scanner Scan >-----")
	logger.Info().
		Str("engine", engine).
		Str("image", imageRef).
		Str("platform", platform).
		Any("scan_timeout", scanTimeout.String()).
		Msg("")

	var err error
	var pharosScanResult model.PharosImageScanResult

	var sbomData *[]byte
	var scanData *[]byte

	variant := ""
	for _, platform := range []string{"linux/arm64", "linux/386", "linux/mips64le"} {
		digest, err := scanner.GetImageDigests(imageRef, platform, variant, scanner.RepoAuthType{})
		if err != nil {
			logger.Error().Err(err).Msg("GetImageDigest()")
		}
		logger.Info().Str("digest", digest).Str("platform", platform).Msg("")
	}
	//os.Exit(1)

	// scan sbom with chosen scanner engine
	if engine == "grype" {
		var grypeResult *grype.GrypeScanType
		var syftSbom *syft.SyftSbomType
		var vulnScanner *grype.GrypeScanner
		var syftSbomGenerator *syft.SyftSbomCreator

		// create sbom generator
		if syftSbomGenerator, err = syft.NewSyftSbomCreator(scanTimeout, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewSyftSbomCreator()")
		}
		// create scanner
		if vulnScanner, err = grype.NewGrypeScanner(scanTimeout, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewGrypeScanner()")
		}
		// get image and create sbom
		if syftSbom, sbomData, err = syftSbomGenerator.CreateSbom(imageRef, platform, "syft-json"); err != nil {
			logger.Fatal().Err(err).Msg("CreateSbom()")
		}
		// check/update scanner database
		if err = vulnScanner.UpdateDatabase(); err != nil {
			logger.Fatal().Err(err).Msg("UpdateDatabase()")
		}
		// scan sbom
		if grypeResult, scanData, err = vulnScanner.VulnScanSbom(sbomData); err != nil {
			logger.Fatal().Err(err).Msg("VulnScanSbom()")
		}
		if err = pharosScanResult.LoadGrypeImageScan(syftSbom, grypeResult); err != nil {
			logger.Fatal().Err(err).Msg("scanResult.LoadGrypeScan()")
		}
		//logger.Info().Any("model", pharosScanResult).Msg("")

		os.WriteFile("grype-sbom.json", *sbomData, 0644)
		os.WriteFile("grype-sbom-model.json", syftSbom.ToBytes(), 0644)
		os.WriteFile("grype-scan.json", *scanData, 0644)

	} else if engine == "trivy" {
		var sbom *cdx.BOM
		var trivyResult *trivy.TrivyScanType
		var vulnScanner *trivy.TrivyScanner
		var trivySbomGenerator *trivy.TrivySbomCreator

		// create sbom generator
		if trivySbomGenerator, err = trivy.NewTrivySbomCreator(scanTimeout, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewTrivySbomCreator()")
		}
		// create scanner
		if vulnScanner, err = trivy.NewTrivyScanner(scanTimeout, logger); err != nil {
			logger.Fatal().Err(err).Msg("NewTrivyScanner()")
		}

		// get image and create sbom
		if sbom, sbomData, err = trivySbomGenerator.CreateSbom(imageRef, platform, "cyclonedx"); err != nil {
			logger.Fatal().Err(err).Msg("CreateSbom()")
		}

		// ensure initial update of vuln database
		if err = vulnScanner.UpdateDatabase(); err != nil {
			logger.Fatal().Err(err).Msg("UpdateDatabase()")
		}
		if trivyResult, scanData, err = vulnScanner.VulnScanSbom(sbomData); err != nil {
			logger.Fatal().Err(err).Msg("VulnScanSbom()")
		}

		// map into model
		if err = pharosScanResult.LoadTrivyImageScan(sbom, trivyResult); err != nil {
			logger.Fatal().Err(err).Msg("scanResult.LoadGrypeScan()")
		}
		//logger.Info().Any("model", pharosScanResult).Msg("")

		os.WriteFile("trivy-sbom.json", *sbomData, 0644)
		os.WriteFile("trivy-scan.json", *scanData, 0644)
	} else {

		logger.Fatal().Str("engine", engine).Msg("unknown engine")
	}
	logger.Info().
		Str("engine", engine).
		Str("image", imageRef).
		Str("platform", platform).
		Any("x.findings", len(pharosScanResult.Findings)).
		Any("x.vulns", len(pharosScanResult.Vulnerabilities)).
		Any("x.packages", len(pharosScanResult.Packages)).
		Msg("success")

	os.WriteFile(engine+"-model.json", pharosScanResult.ToBytes(), 0644)

	logger.Info().Msg("done")

}
