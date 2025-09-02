package model

import (
	"time"
)

type ContextRoot struct {
	Key       string        `json:"Key" yaml:"Key" gorm:"primaryKey"`
	ImageId   string        `json:"ImageId" yaml:"ImageId" gorm:"primaryKey"`
	UpdatedAt time.Time     `json:"UpdatedAt" yaml:"UpdatedAt"`
	TTL       time.Duration `json:"TTL" yaml:"TTL"`
	Contexts  []Context     `json:"Contexts" yaml:"Contexts" gorm:"foreignKey:ContextRootKey,ImageId;references:Key,ImageId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (cr *ContextRoot) IsExpired() bool {
	// Check if the context root is expired based on the TTL
	return time.Since(cr.UpdatedAt) > cr.TTL
}

type Context struct {
	ID             uint           `json:"ID" yaml:"ID" gorm:"primaryKey"`       // Auto-incrementing primary key
	ContextRootKey string         `json:"ContextRootKey" yaml:"ContextRootKey"` // Composite Foreign Key to the ContextRoot Table
	ImageId        string         `json:"ImageId" yaml:"ImageId"`               // Composite Foreign Key to the ContextRoot Table
	Owner          string         `json:"Owner" yaml:"Owner"`                   // The owner of the Context, this is the plugin that has created / changed it. Will be a Foreign Key to the Plugins Table
	UpdatedAt      time.Time      `json:"UpdatedAt" yaml:"UpdatedAt"`
	Data           map[string]any `json:"Data" yaml:"Data" gorm:"serializer:json"` // Context data
}

type ContextEntry struct {
	ContextRootKey string    `json:"ContextRootKey" yaml:"ContextRootKey"` // Composite Foreign Key to the ContextRoot Table
	Owner          string    `json:"Owner" yaml:"Owner"`
	Key            string    `json:"Key" yaml:"Key"`             // Composite Key to the ContextRoot Table
	Value          any       `json:"Value" yaml:"Value"`         // Value of the context entry
	UpdatedAt      time.Time `json:"UpdatedAt" yaml:"UpdatedAt"` // Last update timestamp
}
