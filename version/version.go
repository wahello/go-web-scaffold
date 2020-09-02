package version

import (
	"fmt"
	"time"
)

const (
	// Name Program name
	Name = "telescope"
)

// The following value is injected by build script
var (
	// Version git version
	Version = "delve"
	// BuildDate build date
	BuildDate = "unknown-build-date"
)

// The following value is initialized when program starts
//noinspection GoUnusedGlobalVariable
var (
	// FullName full name
	FullName string
	// FullNameWithBuildDate full name with build date
	FullNameWithBuildDate string
	// StartedAt time when this instance starts
	StartedAt time.Time
)

func init() {
	FullName = fmt.Sprintf("%s %s", Name, Version)
	FullNameWithBuildDate = fmt.Sprintf("%s %s (%s)", Name, Version, BuildDate)
	StartedAt = time.Now()
}
