package tests

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/pclock"
	"github.com/huuhait/gocpp/internal/wine"
	"github.com/huuhait/gocpp/internal/winssh"
	"github.com/huuhait/gocpp/internal/wintest"
)

// TestWindowsSuite is the whole suite for windows/amd64 (Phase 14 step 14.7, ADR-103): every test of
// tests/run and tests/pkgs that applies to Windows is built here with the cross compiler (gocpp build
// -os windows), packed in batches into ONE archive each, run on the real PC over ssh by a small agent
// (internal/wintest/agent: at most GOCPP_WINDOWS_JOBS programs at once, a time limit and a job object
// for each, outputs back in ONE archive), and judged with the expectations that TestRun uses (the
// // Output: block, // ExitCode:, // Stderr:). The real PC is the way; Wine is an option for a
// developer who has no PC (GOCPP_WINDOWS_WINE=1, never the default).
//
// It is not part of make ci. `make test-windows` runs it, `make test-windows-smoke` runs the selection of
// windows_suite_smoke.txt, `make test-windows-failed` runs only the tests that failed or were not done
// before; the variables are in tests/README.md ("The Windows suite") and docs/guides/windows-testing.md.
//
//   - GOCPP_WINDOWS_SUITE=1 turns it on; GOCPP_WINDOWS_SSH=user@host names the PC (only the environment
//     names it, never a file); GOCPP_WINDOWS_WINE=1 uses Wine instead.
//   - GOCPP_WINDOWS_LIST=1 is the dry run: it prints what would run, what is skipped (with the reason)
//     and what is an expected failure, and builds and copies nothing.
//   - windows_skip.txt lists the tests that are expected to FAIL on Windows, each with its reason. They
//     run too, and one that passes is an error (a stale entry). GOCPP_WINDOWS_SKIPS=ignore leaves them out.
//   - The results are in GOCPP_WINDOWS_RESULTS (default .windows-suite/results.json): one JSON document,
//     written after every batch. GOCPP_WINDOWS_RESUME=1 keeps the passes of an earlier run of the same
//     checkout state and runs the rest; =failed keeps them across changes of the checkout (run only what failed or is new).
//   - A lock file (GOCPP_WINDOWS_LOCK) serialises the runs that use the PC.

const windowsSmokeFile = "windows_suite_smoke.txt"

// suiteEnv is the configuration of the suite that comes from the environment.
type suiteEnv struct {
	dirs       []string
	only       func(name string) bool
	list       bool
	resume     bool
	anyGen     bool   // GOCPP_WINDOWS_RESUME=failed: also keep passes of another state of the checkout
	skips      string // "check" (run the listed tests, expect failure) or "ignore"
	results    string
	jobs       int
	buildJobs  int
	batch      int
	runTimeout time.Duration
	lock       string
	lockWait   time.Duration
	staleAge   time.Duration
	wine       bool
}

var suiteDirs = map[string]format{"run": runFormat, "pkgs": pkgsFormat, "interop": interopFormat}

func envInt(get func(string) string, name string, def int) (int, error) {
	s := get(name)
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s=%s: want a positive number", name, s)
	}
	return n, nil
}

func envDuration(get func(string) string, name string, def time.Duration) (time.Duration, error) {
	s := get(name)
	if s == "" {
		return def, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s=%s: want a duration like 90s or 3h", name, s)
	}
	return d, nil
}

func envFlag(get func(string) string, name string) (bool, error) {
	switch s := get(name); s {
	case "", "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("%s=%s: want 1 or 0", name, s)
	}
}

// readSuiteEnv reads the variables with get (os.Getenv; a map in the unit test); readFile reads the file
// of a GOCPP_WINDOWS_ONLY=@file selection.
func readSuiteEnv(get func(string) string, readFile func(string) ([]byte, error)) (suiteEnv, error) {
	var e suiteEnv
	var err error
	dirs := get("GOCPP_WINDOWS_DIRS")
	if dirs == "" {
		dirs = "run,pkgs"
	}
	for _, d := range strings.Split(dirs, ",") {
		d = strings.TrimSpace(d)
		if _, ok := suiteDirs[d]; !ok {
			return e, fmt.Errorf("GOCPP_WINDOWS_DIRS=%s: %q is not one of run, pkgs, interop", dirs, d)
		}
		e.dirs = append(e.dirs, d)
	}
	if only := get("GOCPP_WINDOWS_ONLY"); only != "" {
		if file, ok := strings.CutPrefix(only, "@"); ok {
			data, err := readFile(file)
			if err != nil {
				return e, fmt.Errorf("GOCPP_WINDOWS_ONLY=%s: %v", only, err)
			}
			names := make(map[string]bool)
			sc := bufio.NewScanner(strings.NewReader(string(data)))
			for sc.Scan() {
				if line := strings.TrimSpace(sc.Text()); line != "" && !strings.HasPrefix(line, "#") {
					names[line] = true
				}
			}
			e.only = func(n string) bool { return names[n] }
		} else {
			rx, err := regexp.Compile(only)
			if err != nil {
				return e, fmt.Errorf("GOCPP_WINDOWS_ONLY=%s: %v", only, err)
			}
			e.only = rx.MatchString
		}
	}
	if e.list, err = envFlag(get, "GOCPP_WINDOWS_LIST"); err != nil {
		return e, err
	}
	if get("GOCPP_WINDOWS_RESUME") == "failed" {
		e.resume, e.anyGen = true, true
	} else if e.resume, err = envFlag(get, "GOCPP_WINDOWS_RESUME"); err != nil {
		return e, fmt.Errorf("%v (or failed)", err)
	}
	if e.wine, err = envFlag(get, "GOCPP_WINDOWS_WINE"); err != nil {
		return e, err
	}
	switch e.skips = get("GOCPP_WINDOWS_SKIPS"); e.skips {
	case "":
		e.skips = "check"
	case "check", "ignore":
	default:
		return e, fmt.Errorf("GOCPP_WINDOWS_SKIPS=%s: want check or ignore", e.skips)
	}
	e.results = get("GOCPP_WINDOWS_RESULTS")
	if e.jobs, err = envInt(get, "GOCPP_WINDOWS_JOBS", 8); err != nil {
		return e, err
	}
	if e.buildJobs, err = envInt(get, "GOCPP_WINDOWS_BUILD_JOBS", 12); err != nil {
		return e, err
	}
	if e.batch, err = envInt(get, "GOCPP_WINDOWS_BATCH", 150); err != nil {
		return e, err
	}
	if e.runTimeout, err = envDuration(get, "GOCPP_WINDOWS_TIMEOUT", 2*time.Minute); err != nil {
		return e, err
	}
	if e.lockWait, err = envDuration(get, "GOCPP_WINDOWS_LOCK_WAIT", 3*time.Hour); err != nil {
		return e, err
	}
	if e.staleAge, err = envDuration(get, "GOCPP_WINDOWS_STALE_AGE", 12*time.Hour); err != nil {
		return e, err
	}
	e.lock = windowsLock(get)
	return e, nil
}

// windowsLock is the lock file of the PC: GOCPP_WINDOWS_LOCK, else a file of /tmp that every run on this
// machine shares.
func windowsLock(get func(string) string) string {
	if l := get("GOCPP_WINDOWS_LOCK"); l != "" {
		return l
	}
	return pclock.DefaultPath
}

func windowsLockPath() string { return windowsLock(os.Getenv) }

// suiteSkips resolves windows_skip.txt: "name" is a test of tests/run, "dir/name" one of another folder.
func suiteSkips() (map[string]string, error) {
	m, _, err := readNameList(windowsSkipFile)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if !strings.Contains(k, "/") {
			k = "run/" + k
		}
		out[k] = v
	}
	return out, nil
}

// suiteKey is what a test's result depends on: the files of its program.
func suiteKey(p *program) (string, error) {
	h := sha256.New()
	add := func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(p.dir, path)
		fmt.Fprintf(h, "%s\x00%d\x00", filepath.ToSlash(rel), len(data))
		h.Write(data)
		return nil
	}
	if p.arg != "." {
		if err := add(filepath.Join(p.dir, p.arg)); err != nil {
			return "", err
		}
	} else {
		err := filepath.WalkDir(p.dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			return add(path)
		})
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// suiteItems finds the tests of the folders and decides, for each, whether it runs, is skipped (and why) or is an
// expected failure. programs maps the item names to the harness's programs.
func (c *config) suiteItems(t *testing.T, env suiteEnv) ([]wintest.Item, map[string]*program) {
	t.Helper()
	skips, err := suiteSkips()
	if err != nil {
		t.Fatal(err)
	}
	var items []wintest.Item
	progs := make(map[string]*program)
	known := make(map[string]bool)
	for _, dir := range env.dirs {
		for _, p := range loadTests(t, suiteDirs[dir]) {
			name := dir + "/" + p.name
			known[name] = true
			if env.only != nil && !env.only(name) {
				continue
			}
			if p.err == nil && p.header != nil {
				p.err = checkPending(p.path, p.header)
			}
			if p.err != nil {
				t.Errorf("%s: %v", name, p.err)
				continue
			}
			key, err := suiteKey(p)
			if err != nil {
				t.Errorf("%s: %v", name, err)
				continue
			}
			it := wintest.Item{Name: name, Group: dir, Phase: p.header.phase, Key: key}
			switch {
			case phaseGate(p.header.phase, c.phase, c.phaseFrom) != "":
				it.Skip, it.SkipKind = phaseGate(p.header.phase, c.phase, c.phaseFrom), "needs a later phase"
			case p.header.build != nil && !p.header.build.Eval(windowsTag):
				it.Skip, it.SkipKind = fmt.Sprintf("its %q line excludes windows/amd64", p.header.buildLine), "builds for another target"
			case p.header.pending != "" && !c.pending:
				it.Skip, it.SkipKind = "pending: "+p.header.pending, "pending"
			case p.header.display:
				it.Skip, it.SkipKind = "needs a display", "needs a display"
			case len(p.header.pkgConfig) > 0:
				it.Skip, it.SkipKind = "needs pkg-config libraries ("+strings.Join(p.header.pkgConfig, " ")+"), which are Linux's", "needs a library"
			case p.header.zoneinfo:
				it.Skip, it.SkipKind = "needs the system's time zone database (Linux's path)", "needs a time zone database"
			case p.wants != nil:
				it.Skip, it.SkipKind = "an error test of the build, not a program", "not a program"
			}
			if reason, listed := skips[name]; listed && it.Skip == "" && env.skips == "check" {
				it.Xfail = reason
			} else if listed && it.Skip == "" {
				it.Skip, it.SkipKind = reason, "listed in windows_skip.txt"
			}
			items = append(items, it)
			progs[name] = p
		}
	}
	// A stale name in the skip list is an error too: only a full selection can say so.
	if env.only == nil && len(env.dirs) == len(suiteDirs) || env.only == nil && containsAll(env.dirs, "run", "pkgs") {
		for name := range skips {
			dir, _, _ := strings.Cut(name, "/")
			if !known[name] && contains(env.dirs, dir) {
				t.Errorf("%s lists %s, which is not a test", windowsSkipFile, name)
			}
		}
	}
	return items, progs
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func containsAll(list []string, want ...string) bool {
	for _, w := range want {
		if !contains(list, w) {
			return false
		}
	}
	return true
}

// suiteGeneration says which state of the checkout made the results: the commit, a hash of the
// uncommitted changes of tracked files and a hash of the untracked files that git does not ignore (a new
// source file changes what the tests do). Any edit of a tracked file therefore invalidates every pass of a
// RESUME=1 run; RESUME=failed ignores the generation.
func suiteGeneration() string { return generationOf(repoRoot) }

func generationOf(dir string) string {
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return string(out)
	}
	head := strings.TrimSpace(run("rev-parse", "HEAD"))
	if head == "" {
		return "no-git"
	}
	h := sha256.New()
	h.Write([]byte(run("diff", "HEAD")))
	h.Write([]byte{0})
	untracked := strings.Split(strings.TrimSpace(run("ls-files", "-o", "--exclude-standard")), "\n")
	sort.Strings(untracked)
	for _, f := range untracked {
		if f == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, f))
		fmt.Fprintf(h, "%s\x00%d\x00%v\x00", f, len(data), err != nil)
		h.Write(data)
	}
	return head[:12] + "+" + hex.EncodeToString(h.Sum(nil))[:8]
}

// An sshRemote is the real PC: internal/winssh with the few operations that the suite needs. The command
// lines (the clean-up of old run folders, the end of a run's processes, the removal of files) are built in
// internal/winssh and tested there.
type sshRemote struct{ r *winssh.Remote }

func (s sshRemote) Put(ctx context.Context, local, name string) error {
	return s.r.Put(ctx, local, name)
}

func (s sshRemote) Get(ctx context.Context, name, local string) error {
	return s.r.Get(ctx, name, local)
}

func (s sshRemote) Agent(ctx context.Context, timeout time.Duration, args ...string) (wintest.AgentRun, error) {
	for _, a := range args {
		if !remoteName.MatchString(a) {
			return wintest.AgentRun{}, fmt.Errorf("the agent argument %q is not a plain name", a)
		}
	}
	res, err := s.r.Exec(ctx, timeout, nil, `.\`+wintest.AgentName+` `+strings.Join(args, " "))
	return wintest.AgentRun{Stdout: string(res.Stdout), Stderr: string(res.Stderr), Code: res.Code, TimedOut: res.TimedOut}, err
}

var remoteName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func (s sshRemote) Remove(ctx context.Context, names ...string) error {
	return s.r.Remove(ctx, names...)
}
func (s sshRemote) KillAll(context.Context) { s.r.KillRun() }
func (s sshRemote) CleanStale(ctx context.Context, age time.Duration) error {
	return s.r.CleanStale(ctx, age)
}
func (s sshRemote) FreeBytes(ctx context.Context) (int64, error) { return s.r.FreeBytes(ctx) }
func (s sshRemote) Close()                                       { s.r.Close() }

// suiteStop ends the machine of a running suite when the test binary is stopped by a signal or by
// go test's -timeout: the processes of the run on the PC, its folder there, and the work folder here.
var suiteStopFn struct {
	sync.Mutex
	fns []func()
}

func suiteStop() {
	suiteStopFn.Lock()
	fns := suiteStopFn.fns
	suiteStopFn.fns = nil
	suiteStopFn.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// A suiteMachine is the machine of a run with the folders on this side: the agent, the work folder with
// the archives and built programs. close ends and removes everything, once, from the end of the test, from a
// signal and from -timeout.
type suiteMachine struct {
	ctx      context.Context
	cancel   context.CancelFunc
	work     string
	agentExe string
	remote   wintest.Remote
	wine     *wine.Env
	once     sync.Once
	running  sync.WaitGroup // runs in progress: close removes the work folder after they have returned
}

// run is wintest.Runner.Run, counted for close.
func (m *suiteMachine) run(cfg wintest.Config, items []wintest.Item) (*wintest.Report, error) {
	m.running.Add(1)
	defer m.running.Done()
	return wintest.New(cfg).Run(m.ctx, items)
}

// watchParent calls gone when the parent process changes (the binary was orphaned: go test was killed with
// SIGTERM, which it does not pass on), so that the programs do not keep running on the PC.
func watchParent(ppid func() int, every time.Duration, stop <-chan struct{}, gone func()) {
	first := ppid()
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			if ppid() != first {
				gone()
				return
			}
		}
	}
}

const workMarker = ".gocpp-winsuite"

var workDirRx = regexp.MustCompile(`^gocpp-winsuite-[0-9]+$`)

// cleanStaleWork removes the work folders of earlier suite runs that were killed (SIGKILL cannot be handled): the
// folders of base named gocpp-winsuite-<digits> that hold the marker file and are older than age. It
// returns what it removed.
func cleanStaleWork(bases []string, age time.Duration, now time.Time) []string {
	var removed []string
	for _, base := range bases {
		entries, _ := os.ReadDir(base)
		for _, e := range entries {
			if !e.IsDir() || !workDirRx.MatchString(e.Name()) {
				continue
			}
			dir := filepath.Join(base, e.Name())
			fi, err := os.Stat(filepath.Join(dir, workMarker))
			if err != nil || now.Sub(fi.ModTime()) < age || !ownedByMe(fi) {
				continue
			}
			if os.RemoveAll(dir) == nil {
				removed = append(removed, dir)
			}
		}
	}
	return removed
}

// workBases are the folders where an earlier run's work folder can be: the TMPDIR of this run, and the
// sibling TMPDIRs of `nix develop` (/tmp/nix-shell.*), which are new for every invocation, so a run that was
// killed with SIGKILL left its folder in one that this run does not use. Only the direct children named
// gocpp-winsuite-<digits> with the marker file, owned by this user, are removed.
func workBases() []string {
	tmp := os.TempDir()
	bases := []string{tmp}
	pattern := filepath.Join(tmp, "nix-shell*")
	if strings.HasPrefix(filepath.Base(tmp), "nix-shell") {
		pattern = filepath.Join(filepath.Dir(tmp), "nix-shell*")
	}
	m, _ := filepath.Glob(pattern)
	for _, d := range m {
		if d != tmp {
			bases = append(bases, d)
		}
	}
	return bases
}

// newSuiteMachine makes the work folder, builds the agent and connects to the PC (or makes the Wine
// environment). The PC lock is not taken here: the Runner takes it around each use of the PC.
func newSuiteMachine(t *testing.T, host string, useWine bool) *suiteMachine {
	t.Helper()
	for _, d := range cleanStaleWork(workBases(), 12*time.Hour, time.Now()) {
		t.Logf("removed the work folder of an earlier run that was killed: %s", d)
	}
	work, err := os.MkdirTemp("", "gocpp-winsuite-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, workMarker), nil, 0o644); err != nil {
		os.RemoveAll(work)
		t.Fatal(err)
	}
	m := &suiteMachine{work: work}
	m.ctx, m.cancel = context.WithCancel(t.Context())
	suiteStopFn.Lock()
	suiteStopFn.fns = append(suiteStopFn.fns, m.close)
	suiteStopFn.Unlock()
	t.Cleanup(m.close)
	stopWatch := make(chan struct{})
	t.Cleanup(func() { close(stopWatch) })
	go watchParent(os.Getppid, time.Second, stopWatch, func() {
		fmt.Fprintln(os.Stderr, "the parent (go test) is gone: ending the run on the PC")
		suiteStop()
		os.Exit(3)
	})
	if m.agentExe, err = buildAgent(m.ctx, work); err != nil {
		t.Fatal(err)
	}
	if useWine {
		if m.wine, err = wine.New(filepath.Join(work, "wine")); err != nil {
			t.Fatal(err)
		}
		root := filepath.Join(work, "machine")
		os.MkdirAll(root, 0o755)
		wenv := m.wine
		m.remote = &wintest.LocalRemote{Root: root, Command: func(ctx context.Context, args ...string) *exec.Cmd {
			argv := append([]string{"-u", "DISPLAY", "-u", "WAYLAND_DISPLAY", "-u", "WAYLAND_SOCKET", wenv.Wine(), wintest.AgentName}, args...)
			cmd := exec.CommandContext(ctx, "env", argv...)
			cmd.Env = wenv.Environ()
			return cmd
		}, Free: func() (int64, error) { return wintest.FreeBytesOf(root) }}
		return m
	}
	r, err := winssh.NewUnlocked(host)
	if err != nil {
		t.Fatal(err) // an unreachable PC is a failure, not a skip
	}
	m.remote = sshRemote{r}
	return m
}

func (m *suiteMachine) close() {
	m.once.Do(func() {
		m.cancel() // the ssh commands in flight end
		if m.remote != nil {
			m.remote.Close() // the run's processes on the PC, then its folder
		}
		if m.wine != nil {
			m.wine.Close()
		}
		m.running.Wait() // a build in flight must not write into the folder after it is gone
		os.RemoveAll(m.work)
	})
}

// buildAgent builds the agent for windows/amd64 into dir.
func buildAgent(ctx context.Context, dir string) (string, error) {
	out := filepath.Join(dir, "winagent.exe")
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags=-s -w", "-o", out, "./internal/wintest/winagent")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("building the agent: %v\n%s", err, b)
	}
	return out, nil
}

// suiteListing is the text of the dry run.
func suiteListing(items []wintest.Item, todo []wintest.Item, resumed int, batch int) string {
	run := make(map[string]bool)
	for _, it := range todo {
		run[it.Name] = true
	}
	var b strings.Builder
	counts := map[string]int{}
	for _, it := range items {
		kind := "run"
		switch {
		case it.Skip != "":
			kind = "skip"
		case !run[it.Name]:
			kind = "done"
		case it.Xfail != "":
			kind = "xfail"
		}
		counts[kind]++
		line := kind + " " + it.Name
		switch kind {
		case "skip":
			line += ": " + it.Skip
		case "xfail":
			line += ": " + it.Xfail
		}
		b.WriteString(line + "\n")
	}
	batches := (len(todo) + batch - 1) / batch
	fmt.Fprintf(&b, "%d tests: run %d (in %d batches of up to %d), xfail %d (run, and must fail), done %d (kept from the results file), skip %d\n",
		len(items), counts["run"], batches, batch, counts["xfail"], counts["done"], counts["skip"])
	return b.String()
}

func TestWindowsSuite(t *testing.T) {
	if os.Getenv("GOCPP_WINDOWS_SUITE") != "1" {
		t.Skip("GOCPP_WINDOWS_SUITE=1 turns it on (make test-windows)")
	}
	env, err := readSuiteEnv(os.Getenv, func(p string) ([]byte, error) {
		if !filepath.IsAbs(p) {
			p = filepath.Join(repoRoot, p) // the tests run in tests/: a path is the repository's
		}
		return os.ReadFile(p)
	})
	if err != nil {
		t.Fatal(err)
	}
	c := setup(t)
	if env.results == "" {
		env.results = filepath.Join(repoRoot, ".windows-suite", "results.json")
	}
	if env.only != nil || len(env.dirs) != 2 {
		// A selection never overwrites the results of a full run by accident.
		if os.Getenv("GOCPP_WINDOWS_RESULTS") == "" {
			env.results = filepath.Join(repoRoot, ".windows-suite", "selection.json")
		}
	}
	items, progs := c.suiteItems(t, env)
	if t.Failed() {
		return
	}
	if len(items) == 0 {
		t.Fatal("the selection is empty")
	}
	host := windowsHost()
	target := "real PC"
	switch {
	case host != "" && env.wine:
		t.Fatal("GOCPP_WINDOWS_SSH and GOCPP_WINDOWS_WINE=1 are both set: choose one (the real PC is the way; Wine is for machines without one)")
	case host == "" && env.wine:
		target = "Wine"
	}
	gen := suiteGeneration()
	cfg := wintest.Config{
		Jobs: env.jobs, BuildJobs: env.buildJobs, BatchSize: env.batch, Timeout: env.runTimeout,
		StaleAge: env.staleAge, ResultsPath: env.results, Generation: gen, Target: target, Resume: env.resume, AnyGeneration: env.anyGen,
		Log: func(s string) { t.Log(s) },
	}
	if env.list {
		todo, _, resumed, err := wintest.New(cfg).Select(items)
		if err != nil {
			t.Fatal(err)
		}
		text := suiteListing(items, todo, resumed, env.batch)
		fmt.Print(text)
		os.MkdirAll(filepath.Dir(env.results), 0o755)
		os.WriteFile(strings.TrimSuffix(env.results, ".json")+"-list.txt", []byte(text), 0o644)
		return
	}
	if host == "" && !env.wine {
		t.Fatal("no Windows machine: set GOCPP_WINDOWS_SSH=user@host (the real PC, key authentication), or GOCPP_WINDOWS_WINE=1 to use Wine on this machine. Nothing is skipped silently.")
	}
	if why := windowsUnavailable(); why != "" {
		t.Fatalf("the Windows suite can't run here: %s", why)
	}
	m := newSuiteMachine(t, host, env.wine)
	work := m.work
	cfg.Remote, cfg.AgentExe, cfg.Work = m.remote, m.agentExe, work
	cfg.WorkFree = func() (int64, error) { return wintest.FreeBytesOf(work) }
	cfg.Lock = &wintest.FileLock{Path: env.lock, Wait: env.lockWait, Log: func(s string) { t.Log(s) }}
	cfg.Build = func(ctx context.Context, it wintest.Item) (string, error) {
		p := progs[it.Name]
		out := filepath.Join(work, "build", strings.ReplaceAll(it.Name, "/", "__")+".exe")
		b := c.run(t, gocppJob(p.dir, runEnv(p.header), "", p.header.phase, "build", "-os", "windows", "-o", out, p.arg))
		if b.problem != "" || b.exit != 0 {
			return "", fmt.Errorf("%s: %s\n%s", b.cmd, b.why(), indent(string(b.stderr)))
		}
		return out, nil
	}
	cfg.Check = func(it wintest.Item, o wintest.Outcome) []string {
		p := progs[it.Name]
		r := result{cmd: it.Name + ".exe", stdout: o.Stdout, stderr: windowsStderr(o.Stderr), exit: o.Exit, status: fmt.Sprintf("exit status %d", o.Exit)}
		problems, shown := runMismatches(r, expect{stdout: p.header.output, from: "the // Output: block", exitCode: p.header.exitCode, stderr: p.header.stderr})
		if len(problems) > 0 && !shown {
			problems = append(problems, "stderr:\n"+indent(string(r.stderr)))
		}
		return problems
	}
	start := time.Now()
	rep, err := m.run(cfg, items)
	if rep != nil {
		summary := rep.Summary()
		fmt.Print(summary)
		os.WriteFile(filepath.Join(filepath.Dir(env.results), strings.TrimSuffix(filepath.Base(env.results), ".json")+"-summary.txt"), []byte(summary), 0o644)
		var failed strings.Builder
		for _, n := range rep.Bad() {
			failed.WriteString(n + "\n")
		}
		os.WriteFile(filepath.Join(filepath.Dir(env.results), strings.TrimSuffix(filepath.Base(env.results), ".json")+"-failed.txt"), []byte(failed.String()), 0o644)
		t.Logf("results: %s (elapsed %v)", env.results, time.Since(start).Round(time.Second))
		var detail []string
		for _, n := range rep.Bad() {
			e := rep.Results.Tests[n]
			if e.Detail != "" {
				detail = append(detail, fmt.Sprintf("--- %s: %s\n%s", n, e.Reason, e.Detail))
			}
		}
		sort.Strings(detail)
		if len(detail) > 0 {
			t.Logf("details of the failures (also in the results file):\n%s", strings.Join(detail, "\n"))
		}
		if !rep.OK() {
			t.Errorf("%d of %d tests are wrong (%s): the list is in %s", len(rep.Bad()), len(rep.Results.Tests), strings.Join(headNames(rep.Bad(), 10), " "), env.results)
		}
	}
	if err != nil {
		t.Fatalf("the Windows suite did not finish: %v (GOCPP_WINDOWS_RESUME=1 continues)", err)
	}
	if rep != nil && rep.Incomplete != "" {
		t.Errorf("the run did not complete: %s", rep.Incomplete)
	}
}

func headNames(s []string, n int) []string {
	if len(s) > n {
		return append(s[:n:n], "...")
	}
	return s
}
