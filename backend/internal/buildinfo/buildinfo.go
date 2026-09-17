// Package buildinfo reports reproducible build identity. Release images set
// these variables with linker flags; local builds use Go's VCS metadata.
package buildinfo

import "runtime/debug"

var Version = "0.4.2-dev"
var Commit = "unknown"
var BuildTime = "unknown"
var Dirty bool

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if Commit == "unknown" {
					Commit = setting.Value
				}
			case "vcs.modified":
				Dirty = setting.Value == "true"
			}
		}
	}
}
