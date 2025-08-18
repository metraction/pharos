package alerting

import (
	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

type Receiver struct {
	Config   *model.ReceiverConfig
	WebHooks []*WebHook
	Logging  *zerolog.Logger
}

func NewReceiver(config *model.ReceiverConfig) *Receiver {
	webhooks := []*WebHook{}
	for _, webHookConfig := range config.WebhookConfigs {
		webhooks = append(webhooks, NewWebHook(&webHookConfig))
	}
	return &Receiver{
		Config:   config,
		WebHooks: webhooks,
		Logging:  logging.NewLogger("info", "component", "Receiver "+config.Name),
	}
}

func (r *Receiver) SendAlerts(alertGroup *AlertGroup, route *Route) error {
	// Send alerts to the configured receiver
	for _, webhook := range r.WebHooks {
		err := alertGroup.SendWebhookAlerts(webhook, route)
		if err != nil {
			r.Logging.Error().Err(err).Str("webhook", webhook.WebHookConfig.URL).Msg("Failed to send alerts")
			return err
		}
	}
	return nil
}

type WebHook struct {
	WebHookConfig *model.WebhookConfig
	Logger        *zerolog.Logger
}

func NewWebHook(webHookConfig *model.WebhookConfig) *WebHook {
	return &WebHook{
		WebHookConfig: webHookConfig,
		Logger:        logging.NewLogger("info", "component", "Webhook "+webHookConfig.URL),
	}
}

func (wh *WebHook) String() string {
	return "webhook " + wh.WebHookConfig.URL
}
