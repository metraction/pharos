package model

// DockerImage represents the structure for a Docker image submission.
type DockerImage struct {
	Name string `json:"name"`
<<<<<<< HEAD
<<<<<<< HEAD
	SHA  string `json:"sha" gorm:"primaryKey"` // SHA is the unique identifier for the image
=======
	SHA  string `json:"sha"`
>>>>>>> c457fd0 (Subscriber implemented)
=======
	SHA  string `json:"sha" gorm:"primaryKey"` // SHA is the unique identifier for the image
>>>>>>> 539dff5 (add database)
}

// GetId returns the unique identifier for the DockerImage, which is its SHA.
func (d DockerImage) GetId() string {
	return d.SHA
}
