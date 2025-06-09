package model

type Config struct {
	Redis Redis
}

type Redis struct {
	Port int
}
