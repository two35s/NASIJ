// Package version holds build-time version metadata for NASIJ.
// Variables are set via -ldflags at compile time:
//
//	go build -ldflags "-X github.com/nasij/nasij/pkg/version.Version=1.0.0" ./cmd/nasij
package version

import "runtime"

// Version is the semantic version string, e.g. "0.1.0".
var Version = "0.1.0"

// GitCommit is the short git commit hash at build time.
var GitCommit = "dev"

// BuildDate is the RFC3339 timestamp of the build.
var BuildDate = "unknown"

// GoVersion returns the Go runtime version string.
func GoVersion() string {
	return runtime.Version()
}

// Platform returns the OS/architecture string (e.g. "darwin/amd64").
func Platform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}
