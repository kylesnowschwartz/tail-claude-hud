// Package version reports the module version recorded in the executable's Go build info.
// Local builds without a module version report "dev".
package version

import (
	"runtime/debug"
	"strings"
)

// String returns the current version, e.g. "0.3.1".
func String() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return resolve(info.Main.Version)
	}
	return "dev"
}

func resolve(buildVersion string) string {
	if buildVersion == "" || buildVersion == "(devel)" {
		return "dev"
	}
	return strings.TrimPrefix(buildVersion, "v")
}
