package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/metraction/pharos/integrations"
	"github.com/metraction/pharos/model"
	"github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
)

func NewPublisherFlow(ctx context.Context, cfg *model.Config) (chan<- any, error) {
<<<<<<< HEAD
	redisSink, err := integrations.NewRedisStreamSink(ctx, cfg.Redis, cfg.Publisher.StreamName)
=======
	redisSink, err := integrations.NewRedisStreamSink(ctx, cfg.Redis, imageSubmissionStream)
>>>>>>> c457fd0 (Subscriber implemented)
	if err != nil {
		log.Printf("Error creating Redis sink: %v\n", err)
		return nil, err
	}
	ch := make(chan any)
	go extension.NewChanSource(ch).
		Via(flow.NewMap(func(msg any) map[string]interface{} {
			// Map to redis message format
			image, _ := msg.(model.DockerImage)
			return map[string]interface{}{
				"name": image.Name,
				"sha":  image.SHA,
			}
		}, 1)).
		To(redisSink)
	return ch, nil
}

// SubmitImageHandler handles HTTP requests for submitting Docker image information.
func SubmitImageHandler(ch chan<- any, cfg *model.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		var dockerImage model.DockerImage
		if err := json.NewDecoder(r.Body).Decode(&dockerImage); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if dockerImage.Name == "" || dockerImage.SHA == "" {
			http.Error(w, "Image name and SHA must be provided", http.StatusBadRequest)
			return
		}

<<<<<<< HEAD
<<<<<<< HEAD
=======
>>>>>>> 539dff5 (add database)
		// this db context should be initialized in a middleware later, for now we just create it here
		db := model.NewDatabaseContext(&cfg.Database)
		tx := db.DB.Save(&dockerImage)
		if tx.Error != nil {
			http.Error(w, tx.Error.Error(), http.StatusInternalServerError)
			return
		}
		ch <- dockerImage

		log.Printf("Successfully sent image %s:%s to stream %s\n", dockerImage.Name, dockerImage.SHA, cfg.Publisher.StreamName)
=======
		ch <- dockerImage

		log.Printf("Successfully sent image %s:%s to stream %s\n", dockerImage.Name, dockerImage.SHA, imageSubmissionStream)
>>>>>>> c457fd0 (Subscriber implemented)
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintf(w, "Image %s:%s accepted for scanning\n", dockerImage.Name, dockerImage.SHA)
		return

		// Note: AwaitCompletion is not called here as this is a fire-and-forget handler.
		// The sink will process messages asynchronously.
	}
}
