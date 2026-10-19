// Package version holds the gocpp version and how far the plan has come.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Version is the gocpp version. It is "dev" for a normal build in a checkout. A release build
// (GoReleaser, .goreleaser.yaml) sets it with
//
//	go build -ldflags "-X github.com/huuhait/gocpp/internal/version.Version=v0.1.0"
//
// and a binary made by go install github.com/huuhait/gocpp/cmd/gocpp@v0.1.0 gets the version
// that go install recorded (init, fromBuildInfo). It is a plain string with a constant value,
// because the linker's -X flag does not change a variable that a function initializes.
var Version = "dev"

func init() {
	if Version == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			Version = fromBuildInfo(bi)
		}
	}
}

// fromBuildInfo returns the version that go install recorded in a binary: the version of the
// main module, such as "v1.2.3", or a pseudo-version for @latest on a branch without tags. It
// returns "dev" for a binary built in a checkout (go build or go test there: the module version
// is "(devel)", or, when Go stamped the repository (a vcs.revision setting), derived from it and
// maybe ending in "+dirty"), for a binary of another main module, and for a version with a space.
func fromBuildInfo(bi *debug.BuildInfo) string {
	if bi == nil || bi.Main.Path != "github.com/huuhait/gocpp" {
		return "dev"
	}
	v := bi.Main.Version
	if v == "" || v == "(devel)" || strings.ContainsAny(v, " \t\n") {
		return "dev"
	}
	for _, s := range bi.Settings {
		if s.Key == "vcs.revision" {
			return "dev"
		}
	}
	return v
}

// Phase is the last finished phase of the plan in
// plans/2026-10-13-gocpp (00 to 15). The test harness uses it to skip
// tests of phases that are not reached yet.
const Phase = 5

// String returns the line that "gocpp version" prints, for example
// "gocpp dev (phase 00) go1.25.14 linux/amd64".
func String() string {
	return fmt.Sprintf("gocpp %s (phase %02d) %s %s/%s",
		Version, Phase, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
