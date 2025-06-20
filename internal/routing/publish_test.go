package routing

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/metraction/pharos/internal/integrations"
	"github.com/metraction/pharos/pkg/model"
)

// loadDockerImages loads Docker image names from a file
func loadDockerImages(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var images []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		image := scanner.Text()
		if image != "" {
			images = append(images, image)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return images, nil
}

// BenchmarkSubmit1000Images benchmarks the submission of Docker images
// by sending tasks to Redis streams.
// This benchmark requires a Redis instance to be running and accessible.
func BenchmarkSubmit1000Images(b *testing.B) {
	// Load Docker images from file
	images, err := loadDockerImages("../../testdata/docker_images.txt")
	if err != nil {
		b.Fatalf("Failed to load Docker images: %v", err)
	}

	b.Logf("Loaded %d Docker images from file", len(images))

	// Configure Redis client and streams
	cfg := &model.Config{
		Redis: model.Redis{
			DSN: "localhost:6379",
		},
		Publisher: model.PublisherConfig{
			RequestQueue:  "scantasks",
			ResponseQueue: "scanresult",
			Timeout:       "300s",
		},
	}

	ctx := context.Background()

	// Create Redis client for request-reply
	client, err := integrations.NewRedisGtrsClient[model.PharosScanTask, model.PharosScanResult](ctx, cfg, cfg.Publisher.RequestQueue, cfg.Publisher.ResponseQueue)
	if err != nil {
		b.Fatalf("Failed to create Redis client: %v", err)
	}

	tasks := make([]model.PharosScanTask, 0, len(images))

	for i, img := range images {
		// Create a proper PharosScanTask using the constructor
		task, err := model.NewPharosScanTask(
			fmt.Sprintf("task-%d-%s", i, img), // jobId
			img,                               // imageRef
			"linux/amd64",                     // platform
			model.PharosRepoAuth{},            // auth
			24*time.Hour,                      // cacheExpiry
			30*time.Second,                    // scanTimeout
		)
		if err != nil {
			b.Fatalf("Failed to create scan task: %v", err)
		}
		tasks = append(tasks, task)
	}

	b.ResetTimer() // Start timing after all setup

	for n := 0; n < b.N; n++ {
		b.StopTimer() // Stop timer during setup

		b.StartTimer() // Resume timing for the actual test

		for _, task := range tasks {
			r, err := client.RequestReply(ctx, task)
			if err != nil {
				b.Fatalf("Error sending request: %v", err)
			}
			if r.ScanTask.Error != "" {
				b.Fatalf("Error in scan task result: %v", r.ScanTask.Error)
			}

			b.Log("Recieved: ", r.Image.ImageSpec, r.ScanTask.Error)
		}
	}
}
