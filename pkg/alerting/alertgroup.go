package alerting

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

type GroupInfo struct {
	LastSentAt     time.Time
	LastSentHash   string // Hash of the last sent alert payload, so we know it has changed.
	ResolutionSent bool
}

// A grouped alert is dependend on a route.
type AlertGroup struct {
	GroupLabels     map[string]string
	Logger          *zerolog.Logger
	Alerts          []*model.Alert
	GroupInfo       map[string]*GroupInfo
	DatabaseContext *model.DatabaseContext
	AlertsUpdated   bool
}

func NewAlertGroup(routeConfig *model.RouteConfig, groupLabels map[string]string, databaseContext *model.DatabaseContext) *AlertGroup {
	return &AlertGroup{
		GroupLabels:     groupLabels,
		GroupInfo:       make(map[string]*GroupInfo),
		Logger:          logging.NewLogger("info", "component", "AlertGroup "+getGroupKey(groupLabels)),
		DatabaseContext: databaseContext,
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
	ag.Logger.Info().Str("webhook", webhook.String()).Any("grouplabels", payload.GroupLabels).Msg("Sending alert now.")
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
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			ag.Logger.Error().Err(err).Msg("Failed to read webhook response body")
		}
		content := string(bodyBytes)
		ag.Logger.Error().Str("response content", content).Msg("Webhook returned non-2xx status")
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	ag.GroupInfo[indentifier].LastSentAt = time.Now()
	ag.GroupInfo[indentifier].LastSentHash = ag.GetHash()
	if payload.Status == "resolved" {
		ag.GroupInfo[indentifier].ResolutionSent = true
	}
	return nil
}

func (ag *AlertGroup) GetAlertPayload(receiverName string) *model.AlertPayload {
	// First we need to try to get the payload from the database
	var alertPayload model.AlertPayload
	newPayload := false
	tx := ag.DatabaseContext.DB.Where("receiver = ? AND group_key = ?", receiverName, ag.String()).First(&alertPayload)
	if tx.Error != nil {
		ag.Logger.Info().Msg("This is a new alertpayload")
		newPayload = true
		var keys []string
		for k := range ag.GroupLabels {
			keys = append(keys, k)
		}
		alertPayload = model.AlertPayload{
			GroupKey:  ag.String(),
			Receiver:  receiverName,
			GroupedBy: keys,
		}
	}
	// Set the Status of the alert payload
	status := "resolved"
	for _, alert := range ag.Alerts {
		if alert.Status == "firing" {
			status = "firing"
		}
	}
	alertPayload.Status = status
	alertPayload.Alerts = ag.Alerts
	if newPayload {
		tx = ag.DatabaseContext.DB.Create(alertPayload)
		if tx.Error != nil {
			ag.Logger.Error().Err(tx.Error).Msg("Failed to create alert payload in database")
		}
	} else {
		tx = ag.DatabaseContext.DB.Save(alertPayload)
		if tx.Error != nil {
			ag.Logger.Error().Err(tx.Error).Msg("Failed to update alert payload in database")
		}
	}
	return &alertPayload
}

func (ag *AlertGroup) GetWebhookPayload(webhook *WebHook, receiverName string) *model.WebHookPayload {
	alertPayload := ag.GetAlertPayload(receiverName)
	prometheusAlerts := make([]*model.PrometheusAlert, len(ag.Alerts))
	commonLabels := ag.GroupLabels
	// Add extra labels to commonLabels
	for key, value := range alertPayload.ExtraLabels {
		commonLabels[key] = value
	}
	// TODO: crashes if someone else changes alerts while we're processing them
	for i, alert := range ag.Alerts {
		if webhook.WebHookConfig.SendResolved || alert.Status == "firing" {
			prometheusAlerts[i] = GetPrometheusAlert(alert)
			summary := fmt.Sprintf("Image %s has vulnerabilities in namespace %s", prometheusAlerts[i].Labels["imagespec"], prometheusAlerts[i].Labels["namespace"])
			prometheusAlerts[i].Annotations = map[string]string{
				"summary":     summary,
				"description": summary,
			}
		}
		// Add extra labels to each alert
		for key, value := range alertPayload.ExtraLabels {
			prometheusAlerts[i].Labels[key] = value
		}
	}
	return &model.WebHookPayload{
		Version:           "4",
		GroupKey:          ag.String(),
		TruncatedAlerts:   0,
		Status:            alertPayload.Status,
		Receiver:          receiverName,
		GroupLabels:       ag.GroupLabels,
		CommonLabels:      commonLabels,
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
