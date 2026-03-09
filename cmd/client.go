package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	_ "github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/model"
	"github.com/metraction/pharos/pkg/syfttype"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var clientLogger = logging.NewLogger("info", "component", "client")

// clientCmd represents the client command
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Pharos client allows to send scan requests and receive results",
	Long:  "Pharos client allows to send scan requests and receive results. This can be used for testing or as a lightweight client to interact with the Pharos API.",
	Run: func(cmd *cobra.Command, args []string) {
		action, _ := cmd.Flags().GetString("action")
		outputFormat, _ := cmd.Flags().GetString("outputformat")
		if action == "pharosscantask/syncscan" {
			clientLogger.Info().Msg("Performing synchronous scan...")
			source, _ := cmd.Flags().GetString("source")
			syftBin, err := utils.OsWhich("syft")
			ctx := cmd.Context()
			if err != nil {
				clientLogger.Fatal().Msgf("Syft not installed: %v", err)
			}
			if source == "" {
				clientLogger.Fatal().Msgf("Source is required for synchronous scan.")
			}
			syftCmd := exec.CommandContext(ctx, syftBin, "scan", source, "-o", "syft-json")
			var stdout, stderr bytes.Buffer
			syftCmd.Stdout = &stdout
			syftCmd.Stderr = &stderr
			err = syftCmd.Run()

			if ctx.Err() == context.DeadlineExceeded {
				clientLogger.Fatal().Msg("Syft scan timed out")
			} else if err != nil {
				clientLogger.Fatal().Msgf("Syft scan failed: %s", utils.NoColorCodes(stderr.String()))
			}

			// get and parse sbom
			sbomData := stdout.Bytes()
			sbomString := stdout.String()
			var sbom syfttype.SyftSbomType
			if err := json.Unmarshal(sbomData, &sbom); err != nil {
				clientLogger.Fatal().Msgf("Failed to parse SBOM: %v", err)
			}
			namespace, _ := cmd.Flags().GetString("namespace")
			clientLogger.Debug().Str("result", stdout.String()).Msg("Scan successful")
			scanTask := model.PharosScanTask{
				ImageSpec:      source,
				ContextRootKey: namespace,
				Context: map[string]any{
					"namespace": namespace,
				},
				Sbom: &sbomString,
			}
			scanTaskJSON, err := json.Marshal(scanTask)
			if err != nil {
				clientLogger.Fatal().Msgf("Failed to marshal scan task: %v", err)
			}
			URL, _ := cmd.Flags().GetString("server-url")
			reqBody := strings.NewReader(string(scanTaskJSON))
			clientLogger.Info().Str("server_url", URL+"/"+action).Msg("Sending scan request to server...")
			resp, err := http.Post(URL+"/"+action, "application/json", reqBody)
			if err != nil {
				clientLogger.Fatal().Msgf("Failed to send request: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				clientLogger.Fatal().Msgf("Unexpected status code: %d", resp.StatusCode)
			}
			var got model.PharosScanResult
			err = json.NewDecoder(resp.Body).Decode(&got)
			if err != nil {
				clientLogger.Fatal().Msgf("Failed to decode response: %v", err)
			}
			got.ScanTask.Sbom = nil
			result := ""
			if outputFormat == "json" {
				resultBytes, err := json.MarshalIndent(got, "", "  ")
				result = string(resultBytes)
				if err != nil {
					clientLogger.Fatal().Msgf("Failed to marshal result to YAML: %v", err)
				}
			}
			if outputFormat == "yaml" {
				resultBytes, err := yaml.Marshal(got)
				result = string(resultBytes)
				if err != nil {
					clientLogger.Fatal().Msgf("Failed to marshal result to YAML: %v", err)
				}
			}
			if outputFormat == "human-readable" {
				fmt.Println(result)
			}
			fmt.Println(result)
		} else {
			clientLogger.Fatal().Msgf("Unknown action: %s", action)
		}
	},
}

func init() {
	rootCmd.AddCommand(clientCmd)
	// clientCmd.Flags().IntVarP(&httpPort, "port", "p", 8080, "Port for the HTTP server")
	clientCmd.Flags().String("server-url", "http://localhost:8080/api/v1", "URL of the Pharos API server")
	clientCmd.Flags().String("action", "pharosscantask/syncscan", "Action to perform: 'syncscan' to do a synchronous scan.")
	clientCmd.Flags().String("source", "", "Image or binary scan (e.g. 'docker:nginx:latest' or 'flile:/path/to/binary')")
	clientCmd.Flags().String("namespace", "client-scans", "Namespace for the scan (e.g. 'myproject')")
	clientCmd.Flags().StringP("outputformat", "o", "json", "output format, can be json or yaml")
	clientCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("outputformat")
		if format != "json" && format != "yaml" {
			return fmt.Errorf("invalid output format: %s (must be 'json' or 'yaml')", format)
		}
		return nil
	}
}
