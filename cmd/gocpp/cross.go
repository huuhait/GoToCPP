package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/build"
)

// Cross-compilation (Phase 06 step 6.13, ADR-055, docs/guides/windows-cross.md): the target of a
// build is a GOOS/GOARCH pair, and it comes from, in this order, the -os and -arch flags, the
// GOCPP_OS and GOCPP_ARCH environment variables, GOOS and GOARCH (like the go command), and the
// system that runs gocpp. This file holds the flags and the rules; target.go (the check's target)
// and internal/build/crosstarget.go (the compiler, the flags and the file names of a target) do
// the rest.

// crossFlags are the flags that choose the target: -os and -arch of check, build, run and emit.
type crossFlags struct {
	os, arch string
}

func (f *crossFlags) register(flags *flag.FlagSet) {
	flags.StringVar(&f.os, "os", "", "the target operating system (GOOS): linux or windows; default GOCPP_OS, GOOS or this system")
	flags.StringVar(&f.arch, "arch", "", "the target architecture (GOARCH): amd64 or arm64; default GOCPP_ARCH, GOARCH or this system")
}

// resolve returns the target: the flags, else GOCPP_OS and GOCPP_ARCH, else GOOS and GOARCH, else the
// system that runs gocpp. A pair that the go command doesn't know is an error with go's message.
func (f crossFlags) resolve() (goos, goarch string, err error) {
	pick := func(flag string, envs ...string) string {
		if flag != "" {
			return flag
		}
		for _, e := range envs {
			if v := os.Getenv(e); v != "" {
				return v
			}
		}
		return ""
	}
	goos = pick(f.os, "GOCPP_OS", "GOOS")
	goarch = pick(f.arch, "GOCPP_ARCH", "GOARCH")
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

// buildTarget returns an error when gocpp can't build programs for goos/goarch: the targets are
// build.Targets (linux/amd64, linux/arm64, windows/amd64). gocpp check and gocpp emit work for any
// target that the go command knows.
func buildTarget(goos, goarch string) error {
	if build.SupportedTarget(goos, goarch) {
		return nil
	}
	return fmt.Errorf("building for %s/%s is not supported (targets: %s)", goos, goarch, strings.Join(build.Targets(), ", "))
}

// isHostTarget reports whether goos/goarch is the system that runs gocpp.
func isHostTarget(goos, goarch string) bool {
	return goos == runtime.GOOS && goarch == runtime.GOARCH
}

// exeSuffixFor ends the names of programs for the operating system goos.
func exeSuffixFor(goos string) string {
	return build.ExeSuffix(goos)
}

// crossRunner returns the words that start the program of gocpp run: GOCPP_EXEC ("wine",
// "qemu-aarch64"; split into words like GOCPP_CXX), like the go command's -exec, when it is set,
// for any target. Without it, a program for another target can't run, and the error says so.
func crossRunner(goos, goarch string) ([]string, error) {
	if v := os.Getenv("GOCPP_EXEC"); v != "" {
		words, err := build.SplitCommand(v)
		if err != nil {
			return nil, fmt.Errorf("GOCPP_EXEC=%s: %w", v, err)
		}
		return words, nil
	}
	if isHostTarget(goos, goarch) {
		return nil, nil
	}
	return nil, fmt.Errorf("a program for %s/%s does not run on %s/%s: set GOCPP_EXEC to a program that runs it (GOCPP_EXEC=wine for windows), or use gocpp build",
		goos, goarch, runtime.GOOS, runtime.GOARCH)
}
