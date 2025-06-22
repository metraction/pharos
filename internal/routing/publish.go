package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/pkg/model"
)

func NewPublisher(ctx context.Context, cfg *model.Config) (*integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult], error) {
	client, err := integrations.NewRedisGtrsClient[model.PharosScanTask, model.PharosScanResult](ctx, cfg, cfg.Publisher.RequestQueue, cfg.Publisher.ResponseQueue)
	return client, err
}

func NewPriorityPublisher(ctx context.Context, cfg *model.Config) (*integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult], error) {
	client, err := integrations.NewRedisGtrsClient[model.PharosScanTask, model.PharosScanResult](ctx, cfg, cfg.Publisher.PriorityRequestQueue, cfg.Publisher.PriorityResponseQueue)
	return client, err
}

// SubmitImageHandler handles HTTP requests for submitting Docker image information.
func SubmitImageHandler(client *integrations.RedisGtrsClient[model.PharosScanTask, model.PharosScanResult], cfg *model.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		// Try to parse as a simple image request first
		var simpleRequest struct {
			Image string `json:"image"`
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		// First try to parse as a simple image request
		var request model.PharosScanTask

		if err := json.Unmarshal(bodyBytes, &simpleRequest); err == nil && simpleRequest.Image != "" {
			// Successfully parsed simple request with image field
			timeout, err := time.ParseDuration(cfg.Publisher.Timeout)
			if err != nil {
				timeout = 30 * time.Second
			}

			// Create a full scan task from the simple image name
			request, err = model.NewPharosScanTask(
				uuid.New().String(),    // jobId
				simpleRequest.Image,    // imageRef
				"linux/amd64",          // platform
				model.PharosRepoAuth{}, // auth
				24*time.Hour,           // cacheExpiry
				timeout,                // scanTimeout
			)
			if err != nil {
				log.Printf("Error creating scan task: %v\n", err)
				http.Error(w, "Error creating scan task", http.StatusInternalServerError)
				return
			}
		} else {
			// Try parsing as a full PharosScanTask
			if err := json.Unmarshal(bodyBytes, &request); err != nil {
				http.Error(w, "Invalid request format", http.StatusBadRequest)
				return
			}
		}

		// Make sure we have an image to scan
		if request.ImageSpec.Image == "" {
			http.Error(w, "Missing image specification", http.StatusBadRequest)
			return
		}

		fmt.Println("Sending image scan request:", request, " to ", cfg.Publisher.RequestQueue)
		response, err := client.RequestReply(r.Context(), request)
		if err != nil {
			log.Printf("Failed to get result for %s: %v\n", request.ImageSpec.Image, err)
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
