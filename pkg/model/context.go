package model

import (
	"time"
)

type ContextRoot struct {
	Key       string `gorm:"primaryKey"`
	ImageId   string `gorm:"primaryKey"`
	UpdatedAt time.Time
	TTL       time.Duration
	Contexts  []Context `gorm:"foreignKey:ContextRootKey,ImageId;references:Key,ImageId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (cr *ContextRoot) IsExpired() bool {
	// Check if the context root is expired based on the TTL
	return time.Since(cr.UpdatedAt) > cr.TTL
}

type Context struct {
	ID             uint   `gorm:"primaryKey"` // Auto-incrementing primary key
	ContextRootKey string // Composite Foreign Key to the ContextRoot Table
	ImageId        string // Composite Foreign Key to the ContextRoot Table
	Owner          string // The owner of the Context, this is the plugin that has created / changed it. Will be a Foreign Key to the Plugins Table
	UpdatedAt      time.Time
	Data           map[string]any `gorm:"serializer:json"` // Context data
}

type ContextEntry struct {
	ContextRootKey string    `json:"ContextRootKey"` // Composite Foreign Key to the ContextRoot Table
	Owner          string    `json:"Owner"`
	Key            string    `json:"Key"`       // Composite Key to the ContextRoot Table
	Value          any       `json:"Value"`     // Value of the context entry
	UpdatedAt      time.Time `json:"UpdatedAt"` // Last update timestamp
}
