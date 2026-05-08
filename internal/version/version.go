package version

import (
	"fmt"
	"runtime/debug"
)

// Set by ldflags. Default values used in dev builds without -ldflags.
var (
	Version = "0.0.0-dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// String returns a human-readable build descriptor. Pure function — no
// package-state mutation. When the binary was built without ldflags
// injection (typical `go build` with no Makefile), it falls back to
// VCS metadata embedded by the Go toolchain.
func String() string {
	v := Version
	commit := Commit
	date := Date

	if v == "0.0.0-dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, s := range info.Settings {
				switch s.Key {
				case "vcs.revision":
					if commit == "unknown" && s.Value != "" {
						commit = s.Value
						if len(commit) > 12 {
							commit = commit[:12]
						}
					}
				case "vcs.time":
					if date == "unknown" && s.Value != "" {
						date = s.Value
					}
				}
			}
		}
	}

	return fmt.Sprintf("%s (commit %s, built %s)", v, commit, date)
}
