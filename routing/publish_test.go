package routing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/metraction/pharos/model"
)

// BenchmarkSubmit1000Images benchmarks the submission of 1000 DockerImage objects
// into the NewPublisherFlow.
// It assumes that the `imageSubmissionStream` constant is defined and accessible
// within the routing package (e.g., from publish.go or another shared file).
// This benchmark also requires a Redis instance to be running and accessible as configured.
func BenchmarkSubmit1000Images(b *testing.B) {
	cfg := &model.Config{
		Redis: model.Redis{
			Port: 6379,
		},
	}

	ctx := context.Background()

	// Set up the publisher flow once, outside the main benchmark loop.
	publishChan, err := NewPublisherFlow(ctx, cfg)
	if err != nil {
		b.Fatalf("Failed to create publisher flow: %v", err)
	}

	const imagesToSubmit = 1000
	dockerImages := make([]model.DockerImage, imagesToSubmit)
	for i := 0; i < imagesToSubmit; i++ {
		dockerImages[i] = model.DockerImage{
			Name: fmt.Sprintf("benchmark-image-%d", i),
			SHA:  fmt.Sprintf("sha256-benchmark-%d-%d", i, time.Now().UnixNano()),
		}
	}

	b.ResetTimer() // Start timing after all setup

	for n := 0; n < b.N; n++ { // b.N is the number of iterations the benchmark runs
		for i := 0; i < imagesToSubmit; i++ {
			publishChan <- dockerImages[i] // Send the pre-generated image
		}
	}

	b.StopTimer() // Stop timing before any potential cleanup
	// Note: The goroutine started by NewPublisherFlow will continue to run.
	// For benchmark purposes, this is generally acceptable as we're measuring submission throughput.
	// In a long-running test suite, you might want to manage the lifecycle of such goroutines more explicitly (e.g., via context cancellation).
}
