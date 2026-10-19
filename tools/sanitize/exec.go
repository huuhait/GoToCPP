package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// limitBuf keeps the first n/2 and the last n/2 bytes written and drops the middle: a program that
// prints a lot has its sanitizer report at the end (a leak check runs at exit, a race is reported when
// the program ends), and a report must not be cut off by the limit.
type limitBuf struct {
	mu   sync.Mutex
	head bytes.Buffer
	tail []byte // the last bytes (at most n/2), in order
	n    int
	cut  int // bytes dropped between head and tail
}

func (l *limitBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	rest := p
	if room := l.n/2 - l.head.Len(); room > 0 {
		k := min(room, len(rest))
		l.head.Write(rest[:k])
		rest = rest[k:]
	}
	if len(rest) > 0 {
		l.tail = append(l.tail, rest...)
		if over := len(l.tail) - l.n/2; over > 0 {
			l.cut += over
			l.tail = append(l.tail[:0], l.tail[over:]...)
		}
	}
	return len(p), nil
}

func (l *limitBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cut == 0 {
		return l.head.String() + string(l.tail)
	}
	return fmt.Sprintf("%s\n[%d bytes of output cut]\n%s", l.head.String(), l.cut, l.tail)
}

// cmdResult is what a command did.
type cmdResult struct {
	Stdout, Stderr string
	Code           int // exit status; -1 when killed by a signal or not started
	TimedOut       bool
	Err            error // the command could not start
}

// A procs tracks the running commands, so that a signal can stop them all.
type procs struct {
	mu      sync.Mutex
	running map[*exec.Cmd]bool
	stopped bool
}

func (p *procs) track(cmd *exec.Cmd, runs bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running == nil {
		p.running = map[*exec.Cmd]bool{}
	}
	switch {
	case !runs:
		delete(p.running, cmd)
	case p.stopped:
		killGroup(cmd)
	default:
		p.running[cmd] = true
	}
}

// StopAll kills the process groups of the commands that run now and of the ones that start later.
func (p *procs) StopAll() {
	p.mu.Lock()
	p.stopped = true
	cmds := make([]*exec.Cmd, 0, len(p.running))
	for c := range p.running {
		cmds = append(cmds, c)
	}
	p.mu.Unlock()
	for _, c := range cmds {
		killGroup(c)
	}
}

// Stopped reports whether StopAll was called.
func (p *procs) Stopped() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopped
}

// exec runs a command in a process group of its own, in dir, with the environment base plus env (a
// later KEY=value wins), and kills it and what it started when the time limit passes.
func (p *procs) exec(limit int, dir string, base, env []string, timeout time.Duration, name string, args ...string) cmdResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	inGroup(cmd)
	cmd.Dir = dir
	cmd.Env = append(append([]string(nil), base...), env...)
	stdout, stderr := &limitBuf{n: limit}, &limitBuf{n: limit}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Start()
	if err == nil {
		p.track(cmd, true)
		err = cmd.Wait()
		p.track(cmd, false)
	}
	killGroup(cmd)
	res := cmdResult{Stdout: stdout.String(), Stderr: stderr.String()}
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		res.TimedOut = true
		res.Code = -1
	case err == nil:
	default:
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.Code = ee.ExitCode()
		} else {
			res.Err = err
			res.Code = -1
		}
	}
	return res
}

// cleanEnv returns the environment of this process without the variables that change what a
// sanitizer does or what gocpp builds: a run must not depend on the developer's shell. Dropped are
// the sanitizers' options, GORACE, every GOCPP_ variable (the runner sets the ones it needs), and
// GOMAXPROCS and GODEBUG (GOMAXPROCS=1 would weaken the interleavings that ThreadSanitizer sees).
func cleanEnv(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if droppedEnv(name) {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func droppedEnv(name string) bool {
	switch name {
	case "ASAN_OPTIONS", "UBSAN_OPTIONS", "TSAN_OPTIONS", "LSAN_OPTIONS", "MSAN_OPTIONS", "GORACE", "GOMAXPROCS", "GODEBUG":
		return true
	}
	return strings.HasPrefix(name, "GOCPP_")
}

// repoRoot finds the repository root: the folder with go.mod and cmd/gocpp above the working folder.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		_, e1 := os.Stat(filepath.Join(dir, "go.mod"))
		_, e2 := os.Stat(filepath.Join(dir, "cmd", "gocpp"))
		if e1 == nil && e2 == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("can't find the repository root (run from inside the gocpp repository, or give -root)")
		}
		dir = parent
	}
}
