package alerting

import (
	"testing"
	"time"

	"github.com/metraction/pharos/pkg/model"
)

func TestGetPrometheusAlert(t *testing.T) {
	alert := &model.Alert{
		Status: "firing",
		Labels: []model.AlertLabel{
			{Name: "alertname", Value: "HighCPU"},
			{Name: "instance", Value: "localhost:9090"},
		},
		Annotations: []model.AlertAnnotation{
			{Name: "summary", Value: "CPU usage is high"},
		},
		StartsAt:     time.Now(),
		EndsAt:       time.Now().Add(1 * time.Hour),
		GeneratorURL: "http://localhost:9090/graph",
		Fingerprint:  "abc123",
	}

	promAlert := GetPrometheusAlert(alert)
	if promAlert == nil {
		t.Fatal("Expected non-nil PrometheusAlert")
	}
	if promAlert.Status != alert.Status {
		t.Errorf("Expected status %s, got %s", alert.Status, promAlert.Status)
	}
	if promAlert.Labels["alertname"] != "HighCPU" {
		t.Errorf("Expected label alertname=HighCPU, got %v", promAlert.Labels["alertname"])
	}
	if promAlert.Annotations["summary"] != "CPU usage is high" {
		t.Errorf("Expected annotation summary=CPU usage is high, got %v", promAlert.Annotations["summary"])
	}
	if promAlert.Fingerprint != alert.Fingerprint {
		t.Errorf("Expected fingerprint %s, got %s", alert.Fingerprint, promAlert.Fingerprint)
	}
}

// TODO: some more tests with database interactions
