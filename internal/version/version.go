package version

import (
	"runtime"
)

// Build-time variables set by ldflags during build
var (
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

// Get returns the current version
func Get() string {
	return Version
}

// GetBuildDate returns the build date
func GetBuildDate() string {
	return BuildDate
}

// GetCommit returns the git commit hash
func GetCommit() string {
	return GitCommit
}

// GetFullVersion returns version with build info
func GetFullVersion() string {
	return Version + " (" + GitCommit + ") built on " + BuildDate
}

// GetGoVersion returns the Go version used to build
func GetGoVersion() string {
	return runtime.Version()
}

// GetPlatform returns the platform info
func GetPlatform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
} 