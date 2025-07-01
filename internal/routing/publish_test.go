package routing

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
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
// It supports registry authentication via DOCKER_REGISTRIES_AUTH environment variable
// in the format: "registry://user:password@docker.io,registry://token@ghcr.io"
// using standard DSN format for registry authentication
func BenchmarkSubmit1000Images(b *testing.B) {
	// Load Docker images from file
	images, err := loadDockerImages("../../testdata/docker_images.txt")
	if err != nil {
		b.Fatalf("Failed to load Docker images: %v", err)
	}

	b.Logf("Loaded %d Docker images from file", len(images))

	// Parse registry authentication from environment variable
	authList := make([]model.PharosRepoAuth, 0)
	if authEnv := os.Getenv("DOCKER_REGISTRIES_AUTH"); authEnv != "" {
		dsns := strings.Split(authEnv, ",")
		for _, dsn := range dsns {
			dsn = strings.TrimSpace(dsn)
			if dsn != "" {
				auth, err := model.NewPharosRepoAuth(dsn)
				if err != nil {
					b.Logf("Warning: Invalid auth DSN: %s - %v", dsn, err)
					continue
				}
				if auth.Authority != "" {
					authList = append(authList, auth)
					b.Logf("Configured auth for registry: %s", auth.Authority)
				}
			}
		}
	}

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
	client, err := integrations.NewRedisGtrsClient[model.PharosScanTask2, model.PharosScanResult](ctx, cfg, cfg.Publisher.RequestQueue, cfg.Publisher.ResponseQueue)
	if err != nil {
		b.Fatalf("Failed to create Redis client: %v", err)
	}

	tasks := make([]model.PharosScanTask2, 0, len(images))

	for i, img := range images {
		// Check if we have auth for this image's registry
		var auth model.PharosRepoAuth
		for _, repoAuth := range authList {
			if strings.HasPrefix(img, repoAuth.Authority) {
				auth = repoAuth
				if auth.Username != "" && auth.Password != "" {
					b.Logf("Using username/password auth for image %s with registry %s", img, auth.Authority)
				} else if auth.Token != "" {
					b.Logf("Using token auth for image %s with registry %s", img, auth.Authority)
				}
				break
			}
		}

		// Create a proper PharosScanTask using the constructor
		// task, err := model.NewPharosScanTask(
		// 	fmt.Sprintf("task-%d-%s", i, img), // jobId
		// 	img,                               // imageRef
		// 	"linux/amd64",                     // platform
		// 	auth,                              // auth (with credentials if matched)
		// 	24*time.Hour,                      // cacheExpiry
		// 	300*time.Second,                   // scanTimeout
		// )
		context := make(map[string]any)
		context["namespace"] = "default" // Example context, can be customized
		task := model.PharosScanTask2{
			JobId:          fmt.Sprintf("task-%d-%s", i, img), // jobId
			ImageSpec:      img,                               // imageRef
			Platform:       "linux/amd64",                     // platform
			AuthDsn:        auth.ToDsn(),                      //auth,                              // auth (with credentials if matched)
			CacheTTL:       24 * time.Hour,                    // cacheExpiry
			ScanTTL:        300 * time.Second,                 // scanTimeout
			Context:        context,                           // context
			ContextRootKey: "namespace=default",               // Example context
		}
		// if err != nil {
		// 	b.Fatalf("Failed to create scan task: %v", err)
		// }
		tasks = append(tasks, task)
	}

	b.ResetTimer() // Start timing after all setup

	for n := 0; n < b.N; n++ {
		b.StopTimer() // Stop timer during setup

		// Create channels for results and errors
		results := make(chan struct {
			result model.PharosScanResult
			err    error
			task   model.PharosScanTask2
		}, len(tasks))

		// Track errors to report at the end
		var errors []string
		var errorsMu sync.Mutex

		b.StartTimer() // Resume timing for the actual test

		// Launch goroutines for parallel task submission
		var wg sync.WaitGroup
		for _, task := range tasks {
			wg.Add(1)
			go func(t model.PharosScanTask2) {
				defer wg.Done()

				// Create a context with timeout for this specific request
				reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()

				r, err := client.RequestReply(reqCtx, t)
				if err != nil {
					errorsMu.Lock()
					errors = append(errors, fmt.Sprintf("Error sending request for %s: %v", t.ImageSpec, err))
					errorsMu.Unlock()
					return
				}

				if r.ScanTask.Error != "" {
					errorsMu.Lock()
					errors = append(errors, fmt.Sprintf("Error in scan task result for %s: %v", t.ImageSpec, r.ScanTask.Error))
					errorsMu.Unlock()
				}

				results <- struct {
					result model.PharosScanResult
					err    error
					task   model.PharosScanTask2
				}{r, err, t}
			}(task)
		}

		// Close results channel when all goroutines are done
		go func() {
			wg.Wait()
			close(results)
		}()

		// Collect and log results
		successCount := 0
		for result := range results {
			if result.err == nil && result.result.ScanTask.Error == "" {
				successCount++
				b.Logf("Success: %s", result.task.ImageSpec)
			}
		}

		// Report summary and errors at the end
		b.Logf("Completed %d/%d tasks successfully", successCount, len(tasks))
		if len(errors) > 0 {
			b.Logf("Encountered %d errors:", len(errors))
			for i, err := range errors {
				b.Logf("Error %d: %s", i+1, err)
			}
		}
	}
}
