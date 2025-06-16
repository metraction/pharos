package model

// DockerImage represents the structure for a Docker image submission.
type DockerImage struct {
	Name   *string `json:"name" gorm:"not null"`     // Name is the name of the Docker image, e.g., "ubuntu:latest"
	Digest *string `json:"digest" gorm:"primaryKey"` // SHA is the unique identifier for the image
}

// GetId returns the unique identifier for the DockerImage, which is its SHA.
func (d DockerImage) GetId() string {
	return *d.Digest
}
