package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/pkg/model"
)

func NewPublisher(ctx context.Context, cfg *model.Config) (*integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult], error) {
	client, err := integrations.NewRedisGtrsClient[model.PharosScanTask, model.PharosScanResult](ctx, cfg, cfg.Publisher.RequestQueue, cfg.Publisher.ResponseQueue)
	return client, err
}

// SubmitImageHandler handles HTTP requests for submitting Docker image information.
func SubmitImageHandler(client *integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult], cfg *model.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		var request model.PharosScanTask
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		fmt.Println("Sending image scan request:", request, " to ", cfg.Publisher.RequestQueue)
		response, err := client.RequestReply(r.Context(), request)
		if err != nil {
			log.Printf("Failed to get result for %s %v\n", request.ImageSpec, err)
			http.Error(w, "Failed to get result", http.StatusInternalServerError)
			return
		}

		log.Printf("Successfully sent image %s to stream %s\n", request.ImageSpec, cfg.Publisher.RequestQueue)

		// Set content type to JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)

		// Create response object
		jsonResponse := map[string]interface{}{
			"status":      "accepted",
			"message":     fmt.Sprintf("Image %s accepted for scanning", request.ImageSpec),
			"scan_result": response,
		}

		// Encode and send JSON response
		if err := json.NewEncoder(w).Encode(jsonResponse); err != nil {
			log.Printf("Error encoding JSON response: %v\n", err)
			http.Error(w, "Error encoding response", http.StatusInternalServerError)
			return
		}

		// Note: AwaitCompletion is not called here as this is a fire-and-forget handler.
		// The sink will process messages asynchronously.
	}
}
