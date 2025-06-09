package model

// DockerImage represents the structure for a Docker image submission.
type DockerImage struct {
	Name string `json:"name"`
	SHA  string `json:"sha"`
}

// GetId returns the unique identifier for the DockerImage, which is its SHA.
func (d DockerImage) GetId() string {
	return d.SHA
}
