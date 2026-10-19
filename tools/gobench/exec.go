package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Sample is one run of a program.
type Sample struct {
	Wall   time.Duration
	CPU    time.Duration
	RSSKB  int64
	Hash   string // the first 16 hex digits of the SHA-256 of stdout
	Head   string // the first bytes of stdout
	Stderr string // the first bytes of stderr
}

// headWriter keeps the first max bytes written to it.
type headWriter struct {
	buf bytes.Buffer
	max int
}

func (w *headWriter) Write(p []byte) (int, error) {
	if room := w.max - w.buf.Len(); room > 0 {
		w.buf.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

// hashWriter hashes everything written to it and keeps the first bytes.
type hashWriter struct {
	h    hash.Hash
	head headWriter
}

func newHashWriter() *hashWriter { return &hashWriter{h: sha256.New(), head: headWriter{max: 256}} }

func (w *hashWriter) Write(p []byte) (int, error) {
	w.h.Write(p)
	return w.head.Write(p)
}

func (w *hashWriter) Sum() string { return hex.EncodeToString(w.h.Sum(nil))[:16] }

// runTimed runs argv with the environment env (nil: the runner's) and measures
// it: wall time, CPU time and peak resident set from the kernel's accounting
// of the child. A non-zero exit status or a timeout is an error (the Sample is
// still filled). Stdout is hashed, not kept: a program may print a lot.
func runTimed(argv []string, env []string, timeout time.Duration) (Sample, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = env
	out := newHashWriter()
	errw := &headWriter{max: 512}
	cmd.Stdout, cmd.Stderr = out, errw
	setupChild(cmd)
	start := time.Now()
	err := cmd.Start()
	var peak atomic.Int64
	done := make(chan struct{})
	sampled := make(chan struct{})
	if err == nil {
		go func() { // the peak resident set of the child, sampled from /proc (see samplePeakRSS)
			defer close(sampled)
			samplePeakRSS(cmd.Process.Pid, done, &peak)
		}()
		err = cmd.Wait()
	} else {
		close(sampled)
	}
	wall := time.Since(start)
	close(done)
	<-sampled
	s := Sample{Wall: wall, Hash: out.Sum(), Head: out.head.buf.String(), Stderr: strings.TrimSpace(errw.buf.String())}
	if cmd.ProcessState != nil {
		var ru int64
		s.CPU, ru = usage(cmd.ProcessState)
		s.RSSKB = chooseRSS(ru, readVmHWM(os.Getpid()), peak.Load(), wall)
	}
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return s, fmt.Errorf("timeout after %v", timeout)
		}
		return s, fmt.Errorf("%w: %s", err, firstLine(s.Stderr))
	}
	return s, nil
}

// minRSSWall is the shortest run whose sampled resident set is believed.
const minRSSWall = 20 * time.Millisecond

// chooseRSS picks the peak resident set (KiB) of a finished child from what is known:
//
//   - ru is getrusage's ru_maxrss of the child, which is the larger of the child's
//     own peak and the runner's own high-water mark at the exec (the child shares
//     the runner's address space until then). When it is above the runner's
//     high-water mark (runnerHWM, read after the run, so at least as large as at
//     the exec), it is the child's peak, exactly.
//   - otherwise the child's peak is below that floor and ru says nothing: the
//     sampled VmHWM (sampled, see samplePeakRSS) is used, but a run shorter than
//     minRSSWall ends before the sampler sees the real peak (the process is gone
//     from /proc the moment it exits), so a tiny run has no figure (0).
//
// Without /proc (runnerHWM 0) ru is all there is.
func chooseRSS(ru, runnerHWM, sampled int64, wall time.Duration) int64 {
	switch {
	case ru > runnerHWM:
		return ru
	case sampled > 0 && wall >= minRSSWall:
		return sampled
	}
	return 0
}

// samplePeakRSS stores into peak the largest VmHWM (the kernel's high-water
// mark of the resident set, in KiB) it reads from /proc/<pid>/status until done
// is closed. getrusage's ru_maxrss of a child cannot be used for this: it starts
// at the high-water mark of the runner's own address space (the child shares it
// until exec), so every small program would report the runner's 5 MB. VmHWM
// belongs to the new image alone and only grows, so the last reading before the
// process ends is the peak (a program that peaks in its last 2 ms is missed by
// a little; see chooseRSS). Platforms without /proc report nothing.
func samplePeakRSS(pid int, done <-chan struct{}, peak *atomic.Int64) {
	for {
		if v := readVmHWM(pid); v > peak.Load() {
			peak.Store(v)
		}
		select {
		case <-done:
			return
		case <-time.After(2 * time.Millisecond):
		}
	}
}

// parseVmHWM returns the VmHWM line of the text of a /proc/<pid>/status file in KiB, 0 without one.
func parseVmHWM(status string) int64 {
	for _, line := range strings.Split(status, "\n") {
		if rest, ok := strings.CutPrefix(line, "VmHWM:"); ok {
			f := strings.Fields(rest)
			if len(f) >= 1 {
				v, _ := strconv.ParseInt(f[0], 10, 64)
				return v
			}
		}
	}
	return 0
}

// capture runs argv in dir with env (nil: the runner's), and returns the
// combined output (at most 64 KiB) and the error.
func capture(dir string, env []string, timeout time.Duration, argv ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = dir, env
	w := &headWriter{max: 64 << 10}
	cmd.Stdout, cmd.Stderr = w, w
	setupChild(cmd)
	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		err = fmt.Errorf("timeout after %v", timeout)
	}
	return w.buf.String(), err
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// envWith returns the process environment with the given KEY=VALUE entries
// replacing the ones with the same keys.
func envWith(extra ...string) []string { return envOver(os.Environ(), extra...) }

// envOver returns base with the given KEY=VALUE entries replacing the ones with the same keys.
func envOver(base []string, extra ...string) []string {
	drop := map[string]bool{}
	for _, e := range extra {
		k, _, _ := strings.Cut(e, "=")
		drop[k] = true
	}
	var env []string
	for _, e := range base {
		k, _, _ := strings.Cut(e, "=")
		if !drop[k] {
			env = append(env, e)
		}
	}
	return append(env, extra...)
}

// benchEnv is the environment of a benchmarked program: the runner's, without
// the variables that change how Go's runtime or the C library behave (GC target,
// parallelism, debug settings, allocator tuning) and without gocpp's own, so
// that every implementation runs with its defaults and the figures do not
// depend on the shell the runner was started from.
func benchEnv() []string { return cleanEnv(os.Environ(), false) }

// cleanEnv drops the variables of benchEnv from env. With keepGate, the gocpp
// variables are dropped except GOCPP_GATE_PHASE (for the build environment,
// where the caller adds its own).
func cleanEnv(env []string, keepGate bool) []string {
	var out []string
	for _, e := range env {
		k, _, _ := strings.Cut(e, "=")
		drop := false
		switch k {
		case "GOGC", "GOMAXPROCS", "GODEBUG", "GOMEMLIMIT", "GOEXPERIMENT", "GOTRACEBACK", "GORACE",
			"GLIBC_TUNABLES", "LD_PRELOAD", "MALLOC_ARENA_MAX", "MALLOC_CONF", "GOCPP_ASAN":
			drop = true
		default:
			drop = strings.HasPrefix(k, "MALLOC_") || (strings.HasPrefix(k, "GOCPP_") && !(keepGate && k == "GOCPP_GATE_PHASE"))
		}
		if !drop {
			out = append(out, e)
		}
	}
	return out
}
