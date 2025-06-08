/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// command line arguments of root command
type ScanArgsType = struct {
	Image       string
	Platform    string
	ScanTimeout string // sbom generation and scan execution timeout
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

	// var err error
	// var sbomGenerator *syft.SyftSbomCreator
	// var vulnScanner *grype.GrypeScanner

	// // create sbom and scanner generators
	// if sbomGenerator, err = syft.NewSyftSbomCreator(scanTimeout, logger); err != nil {
	// 	logger.Fatal().Err(err).Msg("NewSyftSbomCreator()")
	// }
	// if vulnScanner, err = grype.NewGrypeScanner(1*time.Second, logger); err != nil {
	// 	logger.Fatal().Err(err).Msg("NewGrypeScanner()")
	// }

	// //
	// _, _, err = sbomGenerator.CreateSbom(imageUri, platform)
	// if err != nil {
	// 	logger.Fatal().Err(err).Msg("CreateSbom()")
	// }

	// err = vulnScanner.UpdateDatabase()
	// if err != nil {
	// 	logger.Fatal().Err(err).Msg("UpdateDatabase()")
	// }

}
