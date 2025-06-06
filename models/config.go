package models

type Config struct {
	// also known as task manager
	Application ApplicationConfig `mapstructure:"application" validate:"required"`
	Scanner     ScannerConfig     `mapstructure:"scanner" validate:"required"`
}

type ApplicationConfig struct {
	// Port is the port on which the application will
	Listen string `mapstructure:"listen" validate:"required"`
}

type ScannerConfig struct {
}
