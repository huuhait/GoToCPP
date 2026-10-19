// Package diff runs generated programs with real Go and with gocpp and compares what they
// do: the backends, the comparison, the campaign runner, the reducer and the classifier of
// known differences.
package diff

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Result is what one backend did with one program.
type Result struct {
	BuildOK   bool   // the program compiled
	BuildErr  string // the compiler's output when it did not
	ICE       bool   // gocpp: "internal error: generated C++ did not compile" (exit status 2)
	Stdout    string
	Stderr    string
	Exit      int
	TimedOut  bool
	Truncated bool // the output exceeded the limit
	BuildTime time.Duration
	RunTime   time.Duration
}

// Backend builds and runs a program (the source of one Go package main).
type Backend interface {
	Name() string
	// Run builds src and runs it. A returned error is a failure of the harness itself (cannot
	// write a file, cannot start the tool), not of the program.
	Run(ctx context.Context, src string) (Result, error)
}

const maxOutput = 16 << 20

type limitBuf struct {
	bytes.Buffer
	over bool
}

func (b *limitBuf) Write(p []byte) (int, error) {
	if b.Len()+len(p) > maxOutput {
		b.over = true
		p = p[:max(0, maxOutput-b.Len())]
	}
	b.Buffer.Write(p)
	return len(p), nil
}

type cmdResult struct {
	stdout, stderr string
	exit           int
	timedOut       bool
	truncated      bool
	interrupted    bool  // the campaign was stopped (the context ended) while it ran
	err            error // the command could not be started
}

// stopGrace is how long a build gets to stop after SIGTERM before it is killed.
const stopGrace = 20 * time.Second

// runCmd runs a command in its own process group with a time limit.
func runCmd(parent context.Context, dir string, env []string, timeout time.Duration, name string, args ...string) cmdResult {
	return runCmdG(parent, dir, env, timeout, false, name, args...)
}

// runCmdG is runCmd; graceful (a build of gocpp) stops the command with SIGTERM first.
func runCmdG(parent context.Context, dir string, env []string, timeout time.Duration, graceful bool, name string, args ...string) cmdResult {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	inGroup(cmd)
	if graceful {
		cmd.Cancel = func() error { return terminate(cmd) }
		cmd.WaitDelay = stopGrace
	}
	var so, se limitBuf
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	killGroup(cmd)
	r := cmdResult{stdout: so.String(), stderr: se.String(), truncated: so.over || se.over}
	if parent.Err() != nil {
		r.interrupted = true
		return r
	}
	if ctx.Err() == context.DeadlineExceeded {
		r.timedOut = true
	}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			r.exit = ee.ExitCode()
			if r.exit < 0 { // killed by a signal
				r.exit = 128 + 9
			}
		} else if !r.timedOut {
			r.err = err
		}
	}
	return r
}

func envWith(base []string, kv ...string) []string {
	env := append([]string(nil), base...)
	out := env[:0]
	skip := map[string]bool{}
	for i := 0; i+1 < len(kv); i += 2 {
		skip[kv[i]] = true
	}
	for _, e := range env {
		k, _, _ := strings.Cut(e, "=")
		if !skip[k] {
			out = append(out, e)
		}
	}
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, kv[i]+"="+kv[i+1])
	}
	return out
}

// GoBackend builds with real Go, the way tests/diff does (a go.mod of its own, GOAMD64=v1,
// no local toolchain switching) and runs the binary.
type GoBackend struct {
	Go      string // the go command ("go")
	Work    string // a folder for the temporary modules
	Cache   string // GOCACHE
	Timeout time.Duration
	Env     []string
}

func (b *GoBackend) Name() string { return "go" }

func (b *GoBackend) Run(ctx context.Context, src string) (Result, error) {
	dir, err := os.MkdirTemp(b.Work, "go-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module difftest\n\ngo 1.25\n"), 0o644); err != nil {
		return Result{}, err
	}
	for _, f := range SplitFiles(src) {
		if err := os.WriteFile(filepath.Join(dir, f.Name+".go"), []byte(f.Text), 0o644); err != nil {
			return Result{}, err
		}
	}
	env := envWith(append(os.Environ(), b.Env...), "GOTOOLCHAIN", "local", "GOWORK", "off", "GOFLAGS", "", "GOAMD64", "v1", "GOCACHE", b.Cache, "CGO_ENABLED", "0")
	goCmd := b.Go
	if goCmd == "" {
		goCmd = "go"
	}
	t0 := time.Now()
	bin := filepath.Join(dir, "prog")
	br := runCmd(ctx, dir, env, 5*time.Minute, goCmd, "build", "-o", bin, ".")
	res := Result{BuildTime: time.Since(t0)}
	if br.interrupted {
		return res, ctx.Err()
	}
	if br.err != nil {
		return res, fmt.Errorf("go build: %v", br.err)
	}
	if br.exit != 0 || br.timedOut {
		res.BuildErr = br.stderr + br.stdout
		return res, nil
	}
	res.BuildOK = true
	return runBinary(ctx, res, dir, bin, nil, b.Timeout)
}

func runBinary(ctx context.Context, res Result, dir, bin string, env []string, timeout time.Duration) (Result, error) {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	t0 := time.Now()
	rr := runCmd(ctx, dir, env, timeout, bin)
	res.RunTime = time.Since(t0)
	if rr.interrupted {
		return res, ctx.Err()
	}
	if rr.err != nil {
		return res, fmt.Errorf("run %s: %v", bin, rr.err)
	}
	res.Stdout, res.Stderr, res.Exit, res.TimedOut, res.Truncated = rr.stdout, rr.stderr, rr.exit, rr.timedOut, rr.truncated
	return res, nil
}

// GocppBackend builds with "gocpp build" and runs the binary.
type GocppBackend struct {
	Gocpp   string // path of the gocpp binary
	Cxx     string // g++ or clang++
	Phase   string // GOCPP_GATE_PHASE, two digits ("" for none)
	Asan    bool
	Release bool   // gocpp build -release (the optimized build)
	Flags   string // trusted C++ flags (-cxxflags): "-g0" makes the builds faster; not with Asan
	P       int    // gocpp build -p: compiles at the same time (0: gocpp's default)
	ObjSize string // GOCPP_OBJCACHE_SIZE (default 3G)
	Work    string
	XDG     string // XDG_CACHE_HOME (the runtime and object cache of the campaign)
	Timeout time.Duration
	Env     []string
}

func (b *GocppBackend) Name() string {
	n := "gocpp/" + b.Cxx
	if b.Asan {
		n += "+asan"
	}
	if b.Release {
		n += "+release"
	}
	return n
}

// IsICE reports whether a gocpp build output is the "generated C++ did not compile" bug.
func IsICE(out string) bool { return strings.Contains(out, "internal error") }

func (b *GocppBackend) Run(ctx context.Context, src string) (Result, error) {
	dir, err := os.MkdirTemp(b.Work, "cpp-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(dir)
	var gps []string
	for _, f := range SplitFiles(src) {
		if err := os.WriteFile(filepath.Join(dir, f.Name+".gp"), []byte(f.Text), 0o644); err != nil {
			return Result{}, err
		}
		gps = append(gps, f.Name+".gp")
	}
	objSize := b.ObjSize
	if objSize == "" {
		objSize = "3G"
	}
	kv := []string{"XDG_CACHE_HOME", b.XDG, "GOCPP_CXX", b.Cxx, "GOCPP_MODCACHE", "off", "GOCPP_OFFLINE", "1", "GOCPP_WORKLOST", "fail", "GOCPP_OBJCACHE_SIZE", objSize}
	if b.Phase != "" {
		kv = append(kv, "GOCPP_GATE_PHASE", b.Phase)
	}
	env := envWith(append(os.Environ(), b.Env...), kv...)
	bin := filepath.Join(dir, "prog")
	args := []string{"build"}
	if b.Asan {
		args = append(args, "-asan")
	}
	if b.Release {
		args = append(args, "-release")
	}
	if b.Flags != "" && !b.Asan && !b.Release {
		args = append(args, "-cxxflags="+b.Flags)
	}
	if b.P > 0 {
		args = append(args, "-p", strconv.Itoa(b.P))
	}
	args = append(args, "-o", bin)
	args = append(args, gps...)
	t0 := time.Now()
	br := runCmdG(ctx, dir, env, 15*time.Minute, true, b.Gocpp, args...)
	res := Result{BuildTime: time.Since(t0)}
	if br.interrupted {
		return res, ctx.Err()
	}
	if br.err != nil {
		return res, fmt.Errorf("gocpp build: %v", br.err)
	}
	if br.exit != 0 || br.timedOut {
		res.BuildErr = br.stderr + br.stdout
		if br.timedOut {
			res.BuildErr = "build timed out\n" + res.BuildErr
		}
		res.ICE = IsICE(res.BuildErr) || br.exit == 2
		return res, nil
	}
	res.BuildOK = true
	return runBinary(ctx, res, dir, bin, env, b.Timeout)
}
