package version

// build-time variables such as BuildTime, Version, ..
// these variables are set upon build time using Go linker flags (-ldflags)

var (
	BuildTimestamp = "n/a"
	Version        = "0.0.0"
	GoVersion      = "go n/a"
)
