package mappers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/metraction/pharos/pkg/model"
	"github.com/reugn/go-streams/flow"
)

func NewVisual(enricher *model.Enricher, enricherCommon *model.EnricherCommonConfig) flow.MapFunction[map[string]interface{}, map[string]interface{}] {
	logger.Info().Str("enricher", enricher.Name).Msg("Creating visual enricher")
	return func(data map[string]interface{}) map[string]interface{} {
		if enricherCommon == nil || enricherCommon.UiUrl == "" {
			logger.Error().Msg("UiURL is not configured in EnricherCommonConfig please check your configuration")
			return map[string]interface{}{}
		}
		payload, ok := data["payload"].(map[string]interface{})
		if !ok {
			logger.Error().Msg("Invalid data format, expected map with 'payload' key")
			return map[string]interface{}{}
		}
		if payload["Image"] == nil {
			logger.Error().Msg("Missing image metadata in payload")
			return map[string]interface{}{}
		}
		image, ok := payload["Image"].(map[string]interface{})
		if !ok {
			logger.Error().Msg("Invalid payload type, expected model.PharosImageMeta")
			return map[string]interface{}{}
		}
		logger.Info().Str("image_id", image["ImageId"].(string)).Str("enricher", enricher.Name).Uint("id", enricher.ID).Msg("Processing visual enricher")
		// Make GET request to enricherCommon.UIURL with imageid and id as query parameters
		imageID := image["ImageId"].(string)
		url := enricherCommon.UiUrl + "/api/execute?imageid=" + imageID + "&enricherid=" + fmt.Sprintf("%d", enricher.ID)
		resp, err := http.Get(url)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to make GET request to UIURL")
			return map[string]interface{}{}
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to read response body from UIURL")
			return map[string]interface{}{}
		}
		// Optionally, you can unmarshal the response if it's JSON
		var uiData map[string]interface{}
		if err := json.Unmarshal(body, &uiData); err != nil {
			logger.Error().Err(err).Msg("Failed to unmarshal UIURL response")
			return map[string]interface{}{}
		}
		return uiData
	}
}
