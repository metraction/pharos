package alerting

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

type GroupInfo struct {
	LastSentAt     time.Time
	LastSentHash   string
	ResolutionSent bool
}

// A grouped alert is dependend on a route.
type AlertGroup struct {
	GroupLabels map[string]string
	Logger      *zerolog.Logger
	Alerts      []model.Alert
	GroupInfo   map[string]*GroupInfo
}

func NewAlertGroup(routeConfig *model.RouteConfig, groupLabels map[string]string) *AlertGroup {
	return &AlertGroup{
		GroupLabels: groupLabels,
		GroupInfo:   make(map[string]*GroupInfo),
		Logger:      logging.NewLogger("info", "component", "AlertGroup "+getGroupKey(groupLabels)),
	}
}

func (ag *AlertGroup) String() string {
	return getGroupKey(ag.GroupLabels)
}

func (ag *AlertGroup) SendWebhookAlerts(webhook *WebHook, route *Route) error {
	shouldSend := false
	indentifier := route.String() + webhook.String()
	if ag.GroupInfo[indentifier] == nil {
		ag.GroupInfo[indentifier] = &GroupInfo{}
		shouldSend = true
		ag.Logger.Info().Str("webhook", webhook.String()).Str("route", route.String()).Msg("Sending alert because this is the first time.")
	}
	payload := ag.GetWebhookPayload(webhook, route.Receiver.Config.Name)
	if !shouldSend && ag.GetHash() != ag.GroupInfo[indentifier].LastSentHash {
		shouldSend = true
		ag.Logger.Info().Str("webhook", webhook.String()).Msg("Sending alert because the alertgroup has changed.")
	}
	if route.String() == "root[3]" {
		ag.Logger.Info().Str("webhook", webhook.String()).Msg("Debugging here because this is root[3].")
	}
	if !shouldSend && ag.GroupInfo[indentifier].LastSentAt.Add(route.RepeatInterval).Before(time.Now()) {
		shouldSend = true
		ag.Logger.Info().Str("webhook", webhook.String()).Msg("Sending alert because the repeat interval has passed.")
	}
	if ag.GroupInfo[indentifier].ResolutionSent && payload.Status == "resolved" {
		ag.Logger.Info().Str("webhook", webhook.String()).Msg("Not sending alert because resolution was already sent.")
		shouldSend = false
	}
	if len(ag.Alerts) == 0 && !webhook.WebHookConfig.SendResolved {
		ag.Logger.Info().Str("webhook", webhook.String()).Msg("Not sending alert because there are no alerts and SendResolved is false.")
		shouldSend = false
	}
	if !shouldSend {
		return nil
	}

	url := webhook.WebHookConfig.URL
	client := &http.Client{Timeout: 10 * time.Second}
	reqBody, err := json.Marshal(payload)
	if err != nil {
		ag.Logger.Error().Err(err).Msg("Failed to marshal webhook payload")
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if webhook.WebHookConfig.HTTPConfig != nil {
		if webhook.WebHookConfig.HTTPConfig.Authorization != nil {
			authType := webhook.WebHookConfig.HTTPConfig.Authorization.Type
			if authType == "" {
				authType = "Bearer"
			}
			req.Header.Set("Authorization", authType+" "+webhook.WebHookConfig.HTTPConfig.Authorization.Credentials)
		}
		if webhook.WebHookConfig.HTTPConfig.BasicAuth != nil {
			req.SetBasicAuth(webhook.WebHookConfig.HTTPConfig.BasicAuth.Username, webhook.WebHookConfig.HTTPConfig.BasicAuth.Password)
		}
	}
	if err != nil {
		ag.Logger.Error().Err(err).Msg("Failed to create HTTP request")
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		ag.Logger.Error().Err(err).Msg("Failed to send webhook")
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		ag.Logger.Error().Int("status", resp.StatusCode).Msg("Webhook returned non-2xx status")
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	ag.GroupInfo[indentifier].LastSentAt = time.Now()
	ag.GroupInfo[indentifier].LastSentHash = ag.GetHash()
	if payload.Status == "resolved" {
		ag.GroupInfo[indentifier].ResolutionSent = true
	}
	return nil
}

func (ag *AlertGroup) GetWebhookPayload(webhook *WebHook, receiverName string) *model.WebHookPayload {
	prometheusAlerts := make([]*model.PrometheusAlert, len(ag.Alerts))
	status := "resolved"
	// TODO: crashes if someone else changes alerts while we're processing them
	for i, alert := range ag.Alerts {
		if webhook.WebHookConfig.SendResolved || alert.Status == "firing" {
			prometheusAlerts[i] = GetPrometheusAlert(&alert)
			summary := fmt.Sprintf("Image %s has vulnerabilities in namespace %s", prometheusAlerts[i].Labels["imagespec"], prometheusAlerts[i].Labels["namespace"])
			prometheusAlerts[i].Annotations = map[string]string{
				"summary":     summary,
				"description": summary,
			}
			if alert.Status == "firing" {
				status = "firing"
			}
		}
	}
	return &model.WebHookPayload{
		Version:           "4",
		GroupKey:          ag.String(),
		TruncatedAlerts:   0,
		Status:            status,
		Receiver:          receiverName,
		GroupLabels:       ag.GroupLabels,
		CommonLabels:      ag.GroupLabels,
		CommonAnnotations: make(map[string]string),
		ExternalURL:       "todo",
		Alerts:            prometheusAlerts,
	}
}

// return a hash of this AlertGroup, to see if it has changed
func (ag *AlertGroup) GetHash() string {
	// make a sha256 hash value of all alerts
	hash := sha256.New()
	hash.Write([]byte(fmt.Sprintf("%v", ag.GroupLabels)))
	return hex.EncodeToString(hash.Sum(nil))
}

func getGroupKey(groupLabels map[string]string) string {
	keys := make([]string, 0, len(groupLabels))
	for k := range groupLabels {
		keys = append(keys, k)
	}
	// Sort keys to ensure deterministic groupKey
	sort.Strings(keys)
	groupKey := ""
	for _, k := range keys {
		groupKey += fmt.Sprintf("%s=%s;", k, groupLabels[k])
	}
	return groupKey
}
