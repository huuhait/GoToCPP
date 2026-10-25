package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Status values of a flavor.
const (
	statusOK           = "ok"
	statusBuildFailed  = "build-failed"
	statusNotAvailable = "not-available"
)

// Status values of one (program, flavor, thread count) run.
const (
	runOK       = "ok"
	runMismatch = "mismatch"
	runFailed   = "run-failed"
)

// flavorResult is one program built for one flavor, with its runs.
type flavorResult struct {
	Flavor       string      `json:"flavor"` // go, arc, gc
	Status       string      `json:"status"` // ok, build-failed, not-available
	Note         string      `json:"note,omitempty"`
	BuildCommand string      `json:"build_command"`
	BuildMS      int64       `json:"build_ms"`
	BinSize      int64       `json:"bin_size"`
	Runs         []runResult `json:"runs,omitempty"`
}

// cleanEnv returns the process environment without the variables that would
// change a build behind the harness's back.
func cleanEnv(base []string) []string {
	var out []string
	for _, e := range base {
		k, _, _ := strings.Cut(e, "=")
		switch {
		case k == "GOMAXPROCS", k == "GOFLAGS", k == "GOGC", k == "GODEBUG", k == "GOMEMLIMIT",
			k == "GOEXPERIMENT", k == "GO111MODULE", k == "GOWORK", k == "GOTOOLCHAIN",
			k == "CGO_ENABLED", k == "CXX", k == "CC", k == "GOCPP_CXX", k == "GOCPP_CXXFLAGS",
			k == "GOCPP_CFLAGS", k == "GOCPP_LDFLAGS", k == "XDG_CACHE_HOME",
			strings.HasPrefix(k, "GOCPP_"), strings.HasPrefix(k, "MALLOC_"):
			continue
		}
		out = append(out, e)
	}
	return out
}

// envOver returns base with the given KEY=VALUE entries replacing the ones
// with the same keys.
func envOver(base []string, extra ...string) []string {
	drop := map[string]bool{}
	for _, e := range extra {
		k, _, _ := strings.Cut(e, "=")
		drop[k] = true
	}
	var out []string
	for _, e := range base {
		k, _, _ := strings.Cut(e, "=")
		if !drop[k] {
			out = append(out, e)
		}
	}
	return append(out, extra...)
}

// goEnv is the environment of a Go build: 16 threads (the machine is shared),
// no network, no toolchain download, no workspace and no flags from the shell.
func (c *config) goEnv(extra ...string) []string {
	return envOver(cleanEnv(os.Environ()), append([]string{
		"GOMAXPROCS=16", "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=", "GO111MODULE=on", "CGO_ENABLED=0", "GOPROXY=off",
	}, extra...)...)
}

// gatePhase returns the GOCPP_GATE_PHASE the gocpp builds run with: the
// shell's when it sets one, else 15 (every feature gate open).
func (c *config) gatePhase() string {
	if v := os.Getenv("GOCPP_GATE_PHASE"); v != "" {
		return v
	}
	return "15"
}

// gocppEnv is the environment of a gocpp build: 16 threads, its own cache in
// the work folder, offline, and the compiler and flags the run records.
func (c *config) gocppEnv(extra ...string) []string {
	env := []string{
		"GOMAXPROCS=16",
		"XDG_CACHE_HOME=" + filepath.Join(c.work, "cache"),
		"GOCPP_MODCACHE=off",
		"GOCPP_OFFLINE=1",
		"GOCPP_GATE_PHASE=" + c.gatePhase(),
	}
	if c.cxx != "" {
		env = append(env, "GOCPP_CXX="+c.cxx)
	}
	if c.cxxflags != "" {
		env = append(env, "GOCPP_CXXFLAGS="+c.cxxflags)
	}
	return envOver(cleanEnv(os.Environ()), append(env, extra...)...)
}

// gocppBinary returns the gocpp binary to use, building it from the
// repository when none was given.
func (c *config) gocppBinary() (string, error) {
	if c.gocpp != "" {
		return filepath.Abs(c.gocpp)
	}
	bin := filepath.Join(c.work, "bin", "gocpp")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		return "", err
	}
	c.log("building gocpp from %s", c.root)
	out, err := capture(c.root, c.goEnv(), 20*time.Minute, "go", "build", "-o", bin, "./cmd/gocpp")
	if err != nil {
		return "", fmt.Errorf("building gocpp: %v: %s", err, firstLine(out))
	}
	return bin, nil
}

// capture runs argv in dir with env and returns the combined output (at most
// 64 KiB) and the error.
func capture(dir string, env []string, timeout time.Duration, argv ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = dir, env
	w := &limitedBuffer{max: 64 << 10}
	cmd.Stdout, cmd.Stderr = w, w
	setupChild(cmd)
	err := cmd.Run()
	if ctx.Err() != nil {
		err = fmt.Errorf("timeout after %v", timeout)
	}
	return w.String(), err
}

// build builds program p for flavor into the work folder. It never returns an
// error: a failure is the flavor's status.
func (c *config) build(gocppBin string, p program, flavor string) flavorResult {
	fr := flavorResult{Flavor: flavor}
	fail := func(format string, args ...any) flavorResult {
		fr.Status, fr.Note = statusBuildFailed, fmt.Sprintf(format, args...)
		return fr
	}
	dir := filepath.Join(c.work, "build", p.name, flavor)
	if err := os.RemoveAll(dir); err != nil {
		return fail("%v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fail("%v", err)
	}
	src := filepath.Join(c.programDir(p), "main.gp")
	if _, err := os.Stat(src); err != nil {
		return fail("%v", err)
	}
	bin := filepath.Join(dir, "prog")
	start := time.Now()
	var out string
	var err error
	switch flavor {
	case "go":
		if err := copyFile(src, filepath.Join(dir, "main.go")); err != nil {
			return fail("%v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gcbench\n\ngo 1.25\n"), 0o644); err != nil {
			return fail("%v", err)
		}
		fr.BuildCommand = "go build -o prog ."
		out, err = capture(dir, c.goEnv(), 10*time.Minute, "go", "build", "-o", bin, ".")
	case "arc", "gc":
		if err := copyFile(src, filepath.Join(dir, "main.gp")); err != nil {
			return fail("%v", err)
		}
		args := []string{"build"}
		if flavor == "gc" {
			args = append(args, "-gc-engine")
			fr.BuildCommand = "gocpp build -gc-engine -o prog ."
		} else {
			fr.BuildCommand = "gocpp build -o prog ."
		}
		args = append(args, "-o", bin, ".")
		out, err = capture(dir, c.gocppEnv(), 20*time.Minute, append([]string{gocppBin}, args...)...)
		if err != nil && flavor == "gc" {
			if note := notAvailableNote(out); note != "" {
				fr.Status = statusNotAvailable
				fr.Note = note
				return fr
			}
		}
	default:
		return fail("unknown flavor %q", flavor)
	}
	fr.BuildMS = time.Since(start).Milliseconds()
	if err != nil {
		return fail("%v: %s", err, errorLine(out))
	}
	st, err := os.Stat(bin)
	if err != nil {
		return fail("the build wrote no binary")
	}
	fr.BinSize = st.Size()
	fr.Status = statusOK
	return fr
}

// notAvailableNote returns the "not available yet" note of a GC build's output,
// or "". Until steps 16.1d/16.1f, `gocpp build -gc-engine` does not exist: the
// flag parser says so, and that is the same "not available yet" for this
// harness. Once the flag exists, only a message that names the GC engine (or
// the flag) counts: a gated feature of the program that the build reports as
// "not available yet" is a real build failure, not the GC stub.
func notAvailableNote(out string) string {
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if strings.Contains(l, "flag provided but not defined: -gc-engine") {
			return "not available yet (16.1d/16.1f): " + firstLine(l)
		}
		low := strings.ToLower(l)
		if (strings.Contains(low, "not available yet") || strings.Contains(low, "not supported yet")) && namesGCEngine(low) {
			return "not available yet (16.1d/16.1f): " + firstLine(l)
		}
	}
	return ""
}

// namesGCEngine reports whether a lower-case message names the GC engine (or
// its flag), so that notAvailableNote does not swallow a gated feature of the
// program.
func namesGCEngine(low string) bool {
	return strings.Contains(low, "gc engine") || strings.Contains(low, "gc-engine") ||
		strings.Contains(low, "gc mode") || strings.Contains(low, "garbage collector")
}

// errorLine picks the line of a build's output that tells the most: the first
// with "error", else the first line that is not empty.
func errorLine(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, l := range lines {
		if strings.Contains(l, "error") {
			return firstLine(l)
		}
	}
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			return firstLine(l)
		}
	}
	return ""
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
