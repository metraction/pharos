package integrations

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

// service / integrations interface
type ServiceInterface interface {
	Connect(context.Context) error
	ServiceName() string
}

// try to connect services of ServiceInterface interface type.
// this accounts for delays in statup of dependencies
func TryConnectServices(ctx context.Context, retryMax int, retrySleep time.Duration, services []ServiceInterface, logger *zerolog.Logger) error {

	for k := 1; k < retryMax+1; k++ {
		logger.Info().Any("attempt", k).Any("max", retryMax).Msg("services connect ..")
		ready := true
		for _, service := range services {
			if err := service.Connect(ctx); err != nil {
				logger.Error().Err(err).Str("service", service.ServiceName()).Msg("connect failed")
				ready = false
			}
		}
		if ready {
			break
		}
		if k >= retryMax {
			return fmt.Errorf("service connect failed after %v attempts", k)

		}
		time.Sleep(retrySleep)
	}
	return nil
}
