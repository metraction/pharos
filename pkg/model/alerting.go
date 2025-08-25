package model

import (
	"time"
)

type WebHookPayload struct {
	Version           string             `json:"version"`
	GroupKey          string             `json:"groupKey"`
	TruncatedAlerts   int                `json:"truncatedAlerts"`
	Status            string             `json:"status"`
	Receiver          string             `json:"receiver"`
	GroupLabels       map[string]string  `json:"groupLabels"`
	CommonLabels      map[string]string  `json:"commonLabels"`
	CommonAnnotations map[string]string  `json:"commonAnnotations"`
	ExternalURL       string             `json:"externalURL"`
	Alerts            []*PrometheusAlert `json:"alerts"`
}

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
	StartsAt     time.Time         `json:"startsAt" gorm:"index"`
	EndsAt       time.Time         `json:"endsAt" gorm:"index"`
	GeneratorURL string            `json:"generatorURL" gorm:"index"`
	Fingerprint  string            `json:"fingerprint" gorm:"primaryKey"` // see how this is defined: https://stackoverflow.com/questions/59066569/is-the-fingerprint-field-in-alertmanager-unique
}

type AlertLabel struct {
	AlertFingerprint string `json:"alert_fingerprint" gorm:"primaryKey"`
	Name             string `json:"name" gorm:"primaryKey"`
	Value            string `json:"value" gorm:"primaryKey"`
}

type AlertAnnotation struct {
	AlertFingerprint string `json:"alert_fingerprint" gorm:"primaryKey"`
	Name             string `json:"name" gorm:"primaryKey"`
	Value            string `json:"value" gorm:"primaryKey"`
}
