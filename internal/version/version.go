// Package version embeds the project version from the VERSION file.
package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var raw string

// String returns the current version, e.g. "0.3.1".
func String() string {
	return strings.TrimSpace(raw)
}
