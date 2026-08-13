// Package version reports the app version or, for development/test builds,
// the build date.
package version

import (
	"os"
	"runtime/debug"
	"time"
)

// Version is the release version, injected at build time with
//
//	go build -ldflags "-X github.com/jonathanhecl/vWriter/internal/version.Version=v0.5.0"
//
// Empty means a development or test build, in which case the build date is
// reported instead.
var Version = ""

// buildSetting reads one setting from the embedded build info.
func buildSetting(key string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, setting := range info.Settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	return ""
}

// String returns the release version when one is injected, otherwise the
// build date derived from the VCS commit time (or the executable's
// modification time as a fallback).
func String() string {
	if Version != "" {
		return Version
	}
	if vcsTime := buildSetting("vcs.time"); vcsTime != "" {
		if t, err := time.Parse(time.RFC3339, vcsTime); err == nil {
			return t.Format("2006-01-02")
		}
		return vcsTime
	}
	if exe, err := os.Executable(); err == nil {
		if info, err := os.Stat(exe); err == nil {
			return info.ModTime().Format("2006-01-02")
		}
	}
	return "dev"
}
