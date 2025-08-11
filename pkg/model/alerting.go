package model

import (
	"time"
)

type PrometheusAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"` // see how this is defined: https://stackoverflow.com/questions/59066569/is-the-fingerprint-field-in-alertmanager-unique
}

type Alert struct {
	Status       string            `json:"status"`
	Labels       []AlertLabel      `json:"labels" gorm:"foreignKey:AlertFingerprint;references:Fingerprint;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Annotations  []AlertAnnotation `json:"annotations" gorm:"foreignKey:AlertFingerprint;references:Fingerprint;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint" gorm:"primaryKey"` // see how this is defined: https://stackoverflow.com/questions/59066569/is-the-fingerprint-field-in-alertmanager-unique
}

func (a *Alert) GetPrometheusAlert() *PrometheusAlert {
	if a == nil {
		return nil
	}
	labels := make(map[string]string, len(a.Labels))
	for _, l := range a.Labels {
		labels[l.Name] = l.Value
	}
	annotations := make(map[string]string, len(a.Annotations))
	for _, a := range a.Annotations {
		annotations[a.Name] = a.Value
	}
	return &PrometheusAlert{
		Status:       a.Status,
		Labels:       labels,
		Annotations:  annotations,
		StartsAt:     a.StartsAt,
		EndsAt:       a.EndsAt,
		GeneratorURL: a.GeneratorURL,
		Fingerprint:  a.Fingerprint,
	}
}

type AlertLabel struct {
	ID               uint   `gorm:"primaryKey"`
	AlertFingerprint string `json:"alert_fingerprint"`
	Name             string `json:"name"`
	Value            string `json:"value"`
}

type AlertAnnotation struct {
	ID               uint   `gorm:"primaryKey"`
	AlertFingerprint string `json:"alert_fingerprint"`
	Name             string `json:"name" `
	Value            string `json:"value"`
}
