package model

import (
	"time"
)

// AlertPayload is something we store in the database. An Alert Payload
// is identified by its GroupKey and Receiver. It is exposed by the API and we can manually silence it
// or add more commonlabels/annotations to it.
type AlertPayload struct {
	Receiver    string            `json:"receiver" gorm:"primary_key"`
	GroupKey    string            `json:"groupKey" gorm:"primary_key"`
	GroupedBy   StringSlice       `json:"groupedBy" gorm:"type:VARCHAR"`
	Status      string            `json:"status"` // "firing" or "resolved"
	Alerts      []*Alert          `json:"alerts" gorm:"many2many:join_alert_payload_with_alert;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	ExtraLabels map[string]string `json:"extraLabels" yaml:"extraLabels" gorm:"serializer:json"` // Context data
}

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
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
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
