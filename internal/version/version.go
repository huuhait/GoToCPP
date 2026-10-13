// Package version holds the gocpp version and how far the plan has come.
package version

import (
	"fmt"
	"runtime"
)

// Version is the gocpp version. It is "dev" for a normal build. A release
// build can set it with
//
//	go build -ldflags "-X github.com/huuhait/gocpp/internal/version.Version=v0.1.0"
var Version = "dev"

// Phase is the last finished phase of the plan in
// plans/2026-10-13-gocpp (00 to 15). The test harness uses it to skip
// tests of phases that are not reached yet.
const Phase = 0

// String returns the line that "gocpp version" prints, for example
// "gocpp dev (phase 00) go1.25.7 linux/amd64".
func String() string {
	return fmt.Sprintf("gocpp %s (phase %02d) %s %s/%s",
		Version, Phase, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
