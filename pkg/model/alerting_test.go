package model

import (
	"testing"
	"time"
)

func TestGetPrometheusAlert(t *testing.T) {
	alert := &Alert{
		Status: "firing",
		Labels: []AlertLabel{
			{Name: "alertname", Value: "HighCPU"},
			{Name: "instance", Value: "localhost:9090"},
		},
		Annotations: []AlertAnnotation{
			{Name: "summary", Value: "CPU usage is high"},
		},
		StartsAt:     time.Now(),
		EndsAt:       time.Now().Add(1 * time.Hour),
		GeneratorURL: "http://localhost:9090/graph",
		Fingerprint:  "abc123",
	}

	promAlert := alert.GetPrometheusAlert()
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

func TestGetPrometheusAlert_Nil(t *testing.T) {
	var alert *Alert
	if alert.GetPrometheusAlert() != nil {
		t.Error("Expected nil PrometheusAlert for nil Alert")
	}
}

func TestGetFingerprint(t *testing.T) {
	alert := &Alert{
		Labels: []AlertLabel{
			{Name: "b", Value: "2"},
			{Name: "a", Value: "1"},
		},
	}
	fp1 := alert.GetFingerprint()
	// Changing order should not change fingerprint
	alert.Labels = []AlertLabel{
		{Name: "a", Value: "1"},
		{Name: "b", Value: "2"},
	}
	fp2 := alert.GetFingerprint()
	if fp1 != fp2 {
		t.Errorf("Expected fingerprint to be order-independent, got %s and %s", fp1, fp2)
	}
}

func TestGetFingerprint_EmptyLabels(t *testing.T) {
	alert := &Alert{}
	fp := alert.GetFingerprint()
	if fp != "none" {
		t.Errorf("Expected fingerprint 'none' for empty labels, got %s", fp)
	}
}

func TestAlertLabelAndAnnotation(t *testing.T) {
	label := AlertLabel{Name: "severity", Value: "critical"}
	if label.Name != "severity" || label.Value != "critical" {
		t.Errorf("AlertLabel fields not set correctly")
	}
	annotation := AlertAnnotation{Name: "description", Value: "something happened"}
	if annotation.Name != "description" || annotation.Value != "something happened" {
		t.Errorf("AlertAnnotation fields not set correctly")
	}
}
