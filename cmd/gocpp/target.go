package main

import (
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
)

// goTargets are the GOOS/GOARCH pairs that the go command accepts: what
// "go tool dist list" of Go 1.25.14 prints. gocpp's own targets are
// fewer (Linux and Windows, ADR-021), but the build constraints of a
// package can name any of these, and gocpp check checks a package for any
// of them, like go vet.
var goTargets = strings.Fields(`aix/ppc64 android/386 android/amd64 android/arm android/arm64
darwin/amd64 darwin/arm64 dragonfly/amd64 freebsd/386 freebsd/amd64 freebsd/arm freebsd/arm64
freebsd/riscv64 illumos/amd64 ios/amd64 ios/arm64 js/wasm linux/386 linux/amd64 linux/arm
linux/arm64 linux/loong64 linux/mips linux/mips64 linux/mips64le linux/mipsle linux/ppc64
linux/ppc64le linux/riscv64 linux/s390x netbsd/386 netbsd/amd64 netbsd/arm netbsd/arm64
openbsd/386 openbsd/amd64 openbsd/arm openbsd/arm64 openbsd/ppc64 openbsd/riscv64 plan9/386
plan9/amd64 plan9/arm solaris/amd64 wasip1/wasm windows/386 windows/amd64 windows/arm64`)

// target returns the GOOS and GOARCH that the build constraints use: the
// GOOS and GOARCH environment variables, like the go command, or the
// system that runs gocpp. A pair that the go command doesn't know is an
// error with go's message.
func target() (goos, goarch string, err error) {
	goos, goarch = os.Getenv("GOOS"), os.Getenv("GOARCH")
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	if !slices.Contains(goTargets, goos+"/"+goarch) {
		return "", "", fmt.Errorf("unsupported GOOS/GOARCH pair %s/%s", goos, goarch)
	}
	return goos, goarch, nil
}
