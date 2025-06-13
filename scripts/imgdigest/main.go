package main

// Util to list various image digest for a set of platforms
// Helps identify which digest we need for caching

import (
	"flag"
	"os"

	"github.com/joho/godotenv"
	"github.com/metraction/pharos/internal/scanner/repo"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

var err error
var logger zerolog.Logger
var auth repo.RepoAuth

func init() {

	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
	logger = zerolog.New(consoleWriter).With().Timestamp().Logger()

	logger.Info().Msg("-----< Image/Platform Digest Test >-----")

	err = godotenv.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("")
	}
}

// return left of digest, e.g. "sha256:f85340bf132ae1"
func Left(input string) string {
	return lo.Substring(input, 0, 19)
}
func main() {

	imageRef := flag.String("image", "docker.io/busybox:1.37.0", "Image reference")
	regAuth := flag.String("auth", "", "Authority, e.g. registry://user:pwd@docker.io/?type=password")

	flag.Parse()

	logger.Info().Msg(*regAuth)
	// init
	err = auth.FromDsn(*regAuth)
	if err != nil {
		logger.Fatal().Err(err).Msg("")
	}

	// Code
	platforms := []string{"linux/386", "linux/amd64"} //, "linux/arm/v6", "linux/arm/v7", "linux/arm64/v8"}

	logger.Info().
		Str("authority", auth.ToMaskedDsn("***")).
		Str("image", *imageRef).
		Msg("")

	for _, platform := range platforms {
		d1, d2, err := repo.GetImageDigests(*imageRef, platform, auth)
		if err != nil {
			logger.Error().Err(err).Str("platform", platform).Msg("")
			continue
		}
		logger.Info().
			Str("platform", platform).
			Str("manifest.digest", Left(d1)).
			Str("index.digest", Left(d2)).
			Msg("")
	}
}
