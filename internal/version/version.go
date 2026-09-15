package version

import "runtime/debug"

const Version = "0.7.0"

// Commit is set by the release build and identifies the packaged source.
var Commit = "development"

// BuildCommit falls back to Go build metadata for local builds, including whether
// their checkout contained uncommitted changes. Release builds inject Commit.
func BuildCommit() string {
	if Commit != "development" {
		return Commit
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		revision := ""
		modified := false
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				revision = setting.Value
			}
			if setting.Key == "vcs.modified" {
				modified = setting.Value == "true"
			}
		}
		if revision != "" {
			if modified {
				revision += "+dirty"
			}
			return revision
		}
	}
	return Commit
}
