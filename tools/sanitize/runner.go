package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The statuses of a Result.
const (
	StatusOK           = "ok"
	StatusReport       = "report"        // a sanitizer report in the program's stderr
	StatusBuildFailed  = "build-failed"  // gocpp build failed (also a failure of the run: nothing was checked)
	StatusBadExit      = "bad-exit"      // no report, but not the exit status the header says
	StatusTimeout      = "timeout"       // the program did not end (build or run)
	StatusCanarySilent = "canary-silent" // a canary that must report its kind, and did not: the sanitizer is blind
	StatusNotRun       = "not-run"       // a canary of a kind this mode cannot make
)

// A Config says how a Runner builds and runs programs.
type Config struct {
	Root         string // the repository root
	Gocpp        string // the gocpp program
	Work         string // the folder of the temporary folders
	Cache        string // XDG_CACHE_HOME of the gocpp commands
	CXX          string // GOCPP_CXX: g++ or clang++ ("" keeps the environment's)
	Mode         Mode
	BuildTimeout time.Duration
	RunTimeout   time.Duration
	OutLimit     int      // bytes of output kept per command: the first half and the last half of it
	Env          []string // the environment of the commands (default: this process's, without the sanitizer variables)
	Extra        []string // more KEY=value for every program run (for example ASAN_OPTIONS=quarantine_size_mb=64)
}

// A Result is what happened to one program in one mode with one compiler.
type Result struct {
	Program string   `json:"program"`
	Mode    string   `json:"mode"`
	CXX     string   `json:"cxx"`
	Status  string   `json:"status"`
	Reports []Report `json:"reports,omitempty"`
	Leak    *Leak    `json:"leak,omitempty"` // LeakSanitizer's totals, when it reported
	Exit    int      `json:"exit"`
	Millis  int64    `json:"millis"`
	Detail  string   `json:"detail,omitempty"` // the command and the start of its output, for a failure
	Cycle   bool     `json:"cycle,omitempty"`  // the program is allowed to leak a cycle (// lsan: cycle): LeakSanitizer was off

	Output string `json:"-"` // all of the output of a failing command, for the log file
}

// Failed reports whether the result fails the run.
func (r Result) Failed() bool { return r.Status != StatusOK && r.Status != StatusNotRun }

// A Runner builds and runs programs.
type Runner struct {
	Config
	procs
}

// NewRunner returns a runner with the defaults filled in.
func NewRunner(c Config) *Runner {
	if c.BuildTimeout == 0 {
		c.BuildTimeout = 10 * time.Minute
	}
	if c.RunTimeout == 0 {
		c.RunTimeout = 2 * time.Minute
	}
	if c.OutLimit == 0 {
		c.OutLimit = 4 << 20
	}
	if c.Env == nil {
		c.Env = cleanEnv(os.Environ())
	}
	if c.Work == "" {
		c.Work = os.TempDir()
	}
	noCoreFiles()
	return &Runner{Config: c}
}

// gocppEnv is the environment of the gocpp builds: its own cache, no downloads, every language
// feature of the later phases open (an unfinished phase's gate would hide what the program is
// about), and the build variables of the mode.
func (r *Runner) gocppEnv(p *Program, tmp string) []string {
	env := []string{"GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1", "GOCPP_WORKLOST=fail", "GOCPP_GATE_PHASE=15", "GOFLAGS=", "GOENV=off", "TMPDIR=" + tmp}
	if r.Cache != "" {
		env = append(env, "XDG_CACHE_HOME="+r.Cache)
	}
	if r.CXX != "" {
		env = append(env, "GOCPP_CXX="+r.CXX)
	}
	if p.Header.StdInternal {
		env = append(env, "GOCPP_ALLOW_INTERNAL=1")
	}
	return append(env, r.Mode.BuildEnv...)
}

// runEnv is the environment of the program: the mode's sanitizer options, with LeakSanitizer off
// for a program that leaks a cycle on purpose (the same rule as the tests harness; the audit of
// leaks.txt bounds what such a program may leak).
func (r *Runner) runEnv(p *Program) []string {
	env := append([]string(nil), r.Mode.RunEnv...)
	if p.Header.Cycle && p.Header.Canary == "" {
		for i, kv := range env {
			if strings.HasPrefix(kv, "ASAN_OPTIONS=") {
				env[i] = kv + ":detect_leaks=0"
			}
		}
	}
	return append(env, r.Extra...)
}

// RunProgram builds the program with the mode and runs it, and judges the run.
func (r *Runner) RunProgram(p *Program) Result {
	start := time.Now()
	res := Result{Program: p.Rel, Mode: r.Mode.Name, CXX: r.CXX, Cycle: p.Header.Cycle}
	finish := func() Result {
		res.Millis = time.Since(start).Milliseconds()
		return res
	}
	if k := p.Header.Canary; k != "" && !r.Mode.Has(k) {
		res.Status = StatusNotRun
		return finish()
	}
	if r.Stopped() {
		res.Status, res.Detail = StatusBuildFailed, "the tool is stopping"
		return finish()
	}
	work, err := os.MkdirTemp(r.Work, "sanitize-")
	if err != nil {
		res.Status, res.Detail = StatusBuildFailed, err.Error()
		return finish()
	}
	defer os.RemoveAll(work)
	exe := filepath.Join(work, "prog")
	dir, base := filepath.Dir(p.Path), filepath.Base(p.Path)

	args := append(append([]string{"build", "-compdb=false"}, r.Mode.BuildArgs...), "-o", exe, base)
	b := r.exec(r.OutLimit, dir, r.Env, r.gocppEnv(p, work), r.BuildTimeout, r.Gocpp, args...)
	buildCmd := "gocpp " + strings.Join(args, " ")
	switch {
	case b.TimedOut:
		res.Status = StatusTimeout
		res.Detail = fmt.Sprintf("%s: timed out after %v", buildCmd, r.BuildTimeout)
		res.Output = b.Stdout + b.Stderr
		return finish()
	case b.Err != nil || b.Code != 0:
		res.Status, res.Exit = StatusBuildFailed, b.Code
		res.Detail = fmt.Sprintf("%s: exit %d %v\n%s", buildCmd, b.Code, b.Err, head(b.Stdout+b.Stderr, 2000))
		res.Output = b.Stdout + b.Stderr
		return finish()
	}

	run := r.exec(r.OutLimit, dir, r.Env, r.runEnv(p), r.RunTimeout, exe)
	res.Exit = run.Code
	res.Reports = Scan(run.Stderr + "\n" + run.Stdout) // a report is on stderr; stdout is read too, in case a program joins them
	if l, ok := ParseLeak(run.Stderr); ok {
		res.Leak = &l
	}
	res.Output = "stdout:\n" + run.Stdout + "\nstderr:\n" + run.Stderr
	runCmd := fmt.Sprintf("%s (%s, %s)", filepath.Base(p.Path), r.Mode.Name, r.CXX)
	switch {
	case run.TimedOut:
		res.Status = StatusTimeout
		res.Detail = fmt.Sprintf("%s: timed out after %v", runCmd, r.RunTimeout)
	case run.Err != nil:
		res.Status, res.Detail = StatusBadExit, fmt.Sprintf("%s: %v", runCmd, run.Err)
	case p.Header.Canary != "":
		res.Status = StatusOK
		if !hasKind(res.Reports, p.Header.Canary) {
			res.Status = StatusCanarySilent
			res.Detail = fmt.Sprintf("%s: a canary of %s, and the run made no %s report (exit %d); the sanitizer is blind:\n%s",
				runCmd, p.Header.Canary, p.Header.Canary, run.Code, head(run.Stderr, 2000))
		}
	case len(res.Reports) > 0:
		res.Status = StatusReport
		res.Detail = fmt.Sprintf("%s: %s\n%s", runCmd, joinReports(res.Reports), head(run.Stderr, 3000))
	case run.Code != p.Header.ExitCode:
		res.Status = StatusBadExit
		res.Detail = fmt.Sprintf("%s: exit %d, the header says %d\n%s", runCmd, run.Code, p.Header.ExitCode, head(run.Stderr, 2000))
	default:
		res.Status = StatusOK
	}
	return finish()
}

func hasKind(rs []Report, k Kind) bool {
	for _, r := range rs {
		if r.Kind == k {
			return true
		}
	}
	return false
}

func joinReports(rs []Report) string {
	parts := make([]string, len(rs))
	for i, r := range rs {
		parts[i] = r.String()
	}
	return strings.Join(parts, "; ")
}

// head returns the first n bytes of s.
func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n[...]"
}
