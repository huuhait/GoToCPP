//go:build unix

package wintest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// The fake machine is a folder of this machine and the agent runs in this process: the "programs" are
// shell scripts. Everything the driver does (batches, resume, expected failures, retries, the lock,
// cleanup) is the real code.

type rig struct {
	t      *testing.T
	dir    string
	remote *LocalRemote
	builds atomic.Int64
	logs   []string
	mu     sync.Mutex
	cfg    Config
}

// script is a program: the body of a shell script.
func newRig(t *testing.T, scripts map[string]string) *rig {
	t.Helper()
	dir := t.TempDir()
	r := &rig{t: t, dir: dir}
	r.remote = &LocalRemote{Root: filepath.Join(dir, "machine")}
	if err := os.MkdirAll(r.remote.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	agentExe := filepath.Join(dir, "agent")
	os.WriteFile(agentExe, []byte("not run: the fake runs the agent in this process"), 0o755)
	work := filepath.Join(dir, "work")
	os.MkdirAll(work, 0o755)
	build := filepath.Join(dir, "build")
	os.MkdirAll(build, 0o755)
	r.cfg = Config{
		Remote: r.remote, AgentExe: agentExe, Work: work, Jobs: 4, BuildJobs: 4, BatchSize: 3,
		Timeout: 20 * time.Second, MaxOutput: 1 << 20, ResultsPath: filepath.Join(dir, "results.json"),
		Generation: "g1", Target: "fake",
		Build: func(ctx context.Context, it Item) (string, error) {
			r.builds.Add(1)
			body, ok := scripts[it.Name]
			if !ok {
				return "", fmt.Errorf("no script for %s", it.Name)
			}
			if body == "BUILD-FAIL" {
				return "", errors.New("g++: error: boom\nsecond line")
			}
			p := filepath.Join(build, strings.ReplaceAll(it.Name, "/", "_"))
			return p, os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755)
		},
		Check: func(it Item, o Outcome) []string {
			var problems []string
			if o.Exit != 0 {
				problems = append(problems, fmt.Sprintf("exit status %d, want 0", o.Exit))
			}
			if want := "ok " + it.Name + "\n"; string(o.Stdout) != want {
				problems = append(problems, fmt.Sprintf("stdout %q, want %q", o.Stdout, want))
			}
			return problems
		},
		Log: func(s string) { r.mu.Lock(); r.logs = append(r.logs, s); r.mu.Unlock() },
	}
	return r
}

func (r *rig) run(items ...Item) *Report {
	r.t.Helper()
	rep, err := New(r.cfg).Run(context.Background(), items)
	if err != nil {
		r.t.Fatalf("Run: %v", err)
	}
	return rep
}

func okScript(name string) string { return "echo ok " + name }

func item(name string) Item {
	group, _, _ := strings.Cut(name, "/")
	return Item{Name: name, Group: group, Phase: 6, Key: "k-" + name}
}

func status(t *testing.T, rep *Report, name string, want Status) *Entry {
	t.Helper()
	e := rep.Results.Tests[name]
	if e == nil {
		t.Fatalf("%s has no result", name)
	}
	if e.Status != want {
		t.Fatalf("%s: status %s (%s), want %s\n%s", name, e.Status, e.Reason, want, e.Detail)
	}
	return e
}

func TestRunnerStatuses(t *testing.T) {
	scripts := map[string]string{
		"run/a_pass":    okScript("run/a_pass"),
		"run/b_wrong":   "echo different",
		"run/c_exit":    "echo ok run/c_exit; exit 3",
		"run/d_hang":    "echo started; sleep 300",
		"run/e_build":   "BUILD-FAIL",
		"run/f_flood":   "yes 0123456789abcdef",
		"run/g_xfail":   "echo nope",
		"run/h_xpass":   okScript("run/h_xpass"),
		"pkgs/i_pass":   okScript("pkgs/i_pass"),
		"pkgs/j_stderr": "echo ok pkgs/j_stderr; echo oops >&2",
	}
	r := newRig(t, scripts)
	r.cfg.Timeout = 1500 * time.Millisecond
	r.cfg.MaxOutput = 64 << 10
	var items []Item
	for name := range scripts {
		it := item(name)
		switch name {
		case "run/g_xfail":
			it.Xfail = "known gap: prints nope"
		case "run/h_xpass":
			it.Xfail = "was a gap once"
		}
		items = append(items, it)
	}
	skipped := item("run/z_skip")
	skipped.Skip, skipped.SkipKind = "needs Phase 99", "needs a later phase"
	items = append(items, skipped)
	rep := r.run(items...)

	status(t, rep, "run/a_pass", Pass)
	if e := status(t, rep, "run/b_wrong", Fail); !strings.Contains(e.Detail, `stdout "different\n"`) {
		t.Errorf("detail %q", e.Detail)
	}
	status(t, rep, "run/c_exit", Fail)
	if e := status(t, rep, "run/d_hang", Fail); !strings.Contains(e.Reason, "timed out after 1.5s") || !strings.Contains(e.Detail, "started") {
		t.Errorf("hang: %q / %q", e.Reason, e.Detail)
	}
	if e := status(t, rep, "run/e_build", Fail); e.Reason != "it does not build for Windows: g++: error: boom" {
		t.Errorf("build failure reason %q", e.Reason)
	}
	if e := status(t, rep, "run/f_flood", Fail); !strings.Contains(e.Reason, "printed more than") {
		t.Errorf("flood: %q", e.Reason)
	}
	e := status(t, rep, "run/g_xfail", XFail)
	if e.Reason != "known gap: prints nope" {
		t.Errorf("xfail reason %q", e.Reason)
	}
	status(t, rep, "run/h_xpass", XPass)
	status(t, rep, "pkgs/i_pass", Pass)
	status(t, rep, "pkgs/j_stderr", Pass) // Check ignores stderr
	status(t, rep, "run/z_skip", Skip)
	if rep.OK() {
		t.Error("the report is OK, but there are failures and a stale entry")
	}
	bad := strings.Join(rep.Bad(), " ")
	if want := "pkgs/none"; bad == want {
		t.Fatal("unreachable")
	}
	if bad != "run/b_wrong run/c_exit run/d_hang run/e_build run/f_flood run/h_xpass" {
		t.Errorf("bad = %s", bad)
	}
	sum := rep.Summary()
	for _, want := range []string{
		"windows suite (fake): 11 tests: pass 3, fail 5, xfail 1, xpass 1, skip 1, error 0",
		"  run      pass 1, fail 5, xfail 1, xpass 1, skip 1, error 0",
		"  pkgs     pass 2, fail 0",
		"  skipped (needs a later phase): 1",
		"STALE run/h_xpass: listed as an expected failure (was a gap once), but it passes now: remove the entry",
		"FAIL run/e_build: it does not build for Windows: g++: error: boom",
	} {
		if !strings.Contains(sum, want) {
			t.Errorf("the summary lacks %q:\n%s", want, sum)
		}
	}
	// The results file is whole and the same document.
	saved, ok, err := LoadResults(r.cfg.ResultsPath)
	if err != nil || !ok || len(saved.Tests) != 11 {
		t.Fatalf("results file: %v %v %d", err, ok, len(saved.Tests))
	}
	// Nothing is left on the machine but the agent, and no batch archive is left here.
	entries, _ := os.ReadDir(r.remote.Root)
	var left []string
	for _, e := range entries {
		left = append(left, e.Name())
	}
	if strings.Join(left, " ") != AgentName {
		t.Errorf("left on the machine: %v", left)
	}
	if es, _ := os.ReadDir(r.cfg.Work); len(es) != 0 {
		t.Errorf("left in the work folder: %v", es)
	}
}

func TestRunnerSummaryIsStable(t *testing.T) {
	r := newRig(t, map[string]string{"run/a": okScript("run/a"), "run/b": "echo no"})
	rep1 := r.run(item("run/a"), item("run/b"))
	rep2 := r.run(item("run/b"), item("run/a"))
	if rep1.Summary() != rep2.Summary() {
		t.Errorf("the summary depends on the order of the items:\n%s\n%s", rep1.Summary(), rep2.Summary())
	}
}

func TestRunnerResume(t *testing.T) {
	scripts := map[string]string{"run/a": okScript("run/a"), "run/b": "echo no", "run/c": okScript("run/c"), "run/d": "BUILD-FAIL", "run/e": "echo gap"}
	r := newRig(t, scripts)
	r.cfg.BatchSize = 2
	e := item("run/e")
	e.Xfail = "gap"
	items := []Item{item("run/a"), item("run/b"), item("run/c"), item("run/d"), e}
	r.run(items...)
	if n := r.builds.Load(); n != 5 {
		t.Fatalf("first run built %d programs, want 5", n)
	}
	// A resume reruns the failures only: a, c and the expected failure e are kept.
	r.builds.Store(0)
	r.cfg.Resume = true
	rep := r.run(items...)
	if n := r.builds.Load(); n != 2 {
		t.Errorf("the resume built %d programs, want 2 (the two failures)", n)
	}
	if rep.Resumed != 3 {
		t.Errorf("resumed %d, want 3", rep.Resumed)
	}
	status(t, rep, "run/a", Pass)
	status(t, rep, "run/b", Fail)
	status(t, rep, "run/e", XFail)

	// The program b is fixed: the resume runs it (a failure is never kept) and keeps the rest.
	scripts["run/b"] = okScript("run/b")
	r.builds.Store(0)
	rep = r.run(items...)
	status(t, rep, "run/b", Pass)
	if n := r.builds.Load(); n != 2 {
		t.Errorf("built %d, want 2", n)
	}

	// A changed key reruns the test; a changed generation reruns everything.
	a := item("run/a")
	a.Key = "changed"
	r.builds.Store(0)
	r.run(a)
	if n := r.builds.Load(); n != 1 {
		t.Errorf("a changed key built %d, want 1", n)
	}
	r.run(items[:3]...) // all three have passes of generation g1 now
	r.builds.Store(0)
	r.run(items[:3]...)
	if n := r.builds.Load(); n != 0 {
		t.Errorf("a resume of three passes built %d, want 0", n)
	}
	r.cfg.Generation = "g2"
	r.builds.Store(0)
	r.run(items[:3]...)
	if n := r.builds.Load(); n != 3 {
		t.Errorf("a changed generation built %d, want 3", n)
	}
	// "Only what failed or is new" keeps the passes across generations.
	r.cfg.AnyGeneration = true
	r.cfg.Generation = "g3"
	r.builds.Store(0)
	r.run(items[:3]...)
	if n := r.builds.Load(); n != 0 {
		t.Errorf("AnyGeneration built %d, want 0", n)
	}
	r.cfg.AnyGeneration = false
	// An entry that stops being an expected failure is run again, not kept.
	r.builds.Store(0)
	e2 := item("run/e")
	r.run(e2)
	if n := r.builds.Load(); n != 1 {
		t.Errorf("a test that is no longer an expected failure built %d, want 1", n)
	}
}

func TestRunnerResumeWithoutFlagRunsAll(t *testing.T) {
	r := newRig(t, map[string]string{"run/a": okScript("run/a")})
	r.run(item("run/a"))
	r.builds.Store(0)
	r.run(item("run/a"))
	if n := r.builds.Load(); n != 1 {
		t.Errorf("built %d, want 1: only Resume keeps results", n)
	}
}

func TestRunnerDuplicateNames(t *testing.T) {
	r := newRig(t, nil)
	if _, err := New(r.cfg).Run(context.Background(), []Item{item("run/a"), item("run/a")}); err == nil || !strings.Contains(err.Error(), "listed twice") {
		t.Errorf("err = %v", err)
	}
}

func TestRunnerListsWithoutTouchingTheMachine(t *testing.T) {
	r := newRig(t, nil)
	r.cfg.Resume = true
	s := item("run/s")
	s.Skip, s.SkipKind = "no", "k"
	todo, res, resumed, err := New(r.cfg).Select([]Item{item("run/b"), item("run/a"), s})
	if err != nil || resumed != 0 || len(todo) != 2 || todo[0].Name != "run/a" || res.Tests["run/s"].Status != Skip {
		t.Errorf("Select: %v %d %v %v", err, resumed, todo, res.Tests)
	}
	if r.builds.Load() != 0 {
		t.Error("Select built something")
	}
	if _, err := os.Stat(r.cfg.ResultsPath); err == nil {
		t.Error("Select wrote the results file")
	}
}

// A hung program is ended with the processes it started.
func TestRunnerKillsTheProcessTree(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	r := newRig(t, map[string]string{
		"run/tree": "sleep 300 &\necho $! > " + pidFile + "\nsleep 300",
		"run/leak": "sleep 300 &\necho $! > " + pidFile + ".leak\necho ok run/leak",
	})
	r.cfg.Timeout = 1500 * time.Millisecond
	rep := r.run(item("run/tree"), item("run/leak"))
	status(t, rep, "run/tree", Fail)
	status(t, rep, "run/leak", Pass)
	for _, f := range []string{pidFile, pidFile + ".leak"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		deadline := time.Now().Add(10 * time.Second)
		for syscall.Kill(pid, 0) == nil && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
		if syscall.Kill(pid, 0) == nil {
			syscall.Kill(pid, syscall.SIGKILL)
			t.Errorf("%s: the child %d still runs after its program ended", filepath.Base(f), pid)
		}
	}
}

// flaky wraps a Remote and fails the first calls of one kind.
type flaky struct {
	Remote
	failPut, failAgent, failGet atomic.Int64
	killed                      atomic.Int64
	agentCalls                  atomic.Int64
}

func (f *flaky) Put(ctx context.Context, local, name string) error {
	if strings.HasSuffix(name, ".tgz") && f.failPut.Add(-1) >= 0 {
		return errors.New("ssh: connection dropped")
	}
	return f.Remote.Put(ctx, local, name)
}

func (f *flaky) Agent(ctx context.Context, d time.Duration, args ...string) (AgentRun, error) {
	f.agentCalls.Add(1)
	if f.failAgent.Add(-1) >= 0 {
		return AgentRun{}, errors.New("ssh: connection reset")
	}
	return f.Remote.Agent(ctx, d, args...)
}

func (f *flaky) Get(ctx context.Context, name, local string) error {
	if strings.HasSuffix(name, "-res.tgz") && f.failGet.Add(-1) >= 0 {
		return errors.New("scp: connection dropped")
	}
	return f.Remote.Get(ctx, name, local)
}

func (f *flaky) KillAll(ctx context.Context) { f.killed.Add(1); f.Remote.KillAll(ctx) }

func TestRunnerRetriesAfterConnectionDrops(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "runs")
	scripts := map[string]string{
		"run/a": "echo x >> " + counter + ".a\necho ok run/a",
		"run/b": "echo x >> " + counter + ".b\necho ok run/b",
	}
	for _, c := range []struct {
		name string
		set  func(*flaky)
	}{
		{"put", func(f *flaky) { f.failPut.Store(1) }},
		{"agent", func(f *flaky) { f.failAgent.Store(1) }},
		{"get", func(f *flaky) { f.failGet.Store(1) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, scripts)
			os.Remove(counter + ".a")
			os.Remove(counter + ".b")
			f := &flaky{Remote: r.remote}
			c.set(f)
			r.cfg.Remote = f
			rep := r.run(item("run/a"), item("run/b"))
			status(t, rep, "run/a", Pass)
			status(t, rep, "run/b", Pass)
			if f.killed.Load() != 1 {
				t.Errorf("KillAll called %d times, want 1 (before the retry)", f.killed.Load())
			}
			if c.name == "get" {
				// The agent ran the batch once; the retry found every result and ran no program again.
				for _, n := range []string{".a", ".b"} {
					data, _ := os.ReadFile(counter + n)
					if got := strings.Count(string(data), "x"); got != 1 {
						t.Errorf("program %s ran %d times, want 1 (the retry resumes)", n, got)
					}
				}
			}
		})
	}
}

func TestRunnerGivesUpAfterTheRetry(t *testing.T) {
	r := newRig(t, map[string]string{"run/a": okScript("run/a")})
	f := &flaky{Remote: r.remote}
	f.failAgent.Store(100)
	r.cfg.Remote = f
	rep := r.run(item("run/a"))
	e := status(t, rep, "run/a", Error)
	if !strings.Contains(e.Reason, "the machine or the connection failed: running the batch") {
		t.Errorf("reason %q", e.Reason)
	}
	if got := f.agentCalls.Load(); got != 2 {
		t.Errorf("the agent was started %d times, want 2 (one retry)", got)
	}
	if rep.OK() {
		t.Error("an error is not OK")
	}
	// An error is not kept by a resume.
	r.cfg.Remote = r.remote
	r.cfg.Resume = true
	rep = r.run(item("run/a"))
	status(t, rep, "run/a", Pass)
}

func TestRunnerRetriesZero(t *testing.T) {
	r := newRig(t, map[string]string{"run/a": okScript("run/a")})
	f := &flaky{Remote: r.remote}
	f.failAgent.Store(1)
	r.cfg.Remote = f
	r.cfg.Retries = -1
	status(t, r.run(item("run/a")), "run/a", Error)
	if f.agentCalls.Load() != 1 {
		t.Errorf("agent calls %d", f.agentCalls.Load())
	}
}

func TestRunnerCleansStaleAndKillsOnlyWhenNeeded(t *testing.T) {
	r := newRig(t, map[string]string{"run/a": okScript("run/a")})
	var stale []time.Duration
	r.remote.Stale = func(d time.Duration) { stale = append(stale, d) }
	killed := 0
	r.remote.Kill = func() { killed++ }
	r.cfg.StaleAge = 7 * time.Hour
	status(t, r.run(item("run/a")), "run/a", Pass)
	if len(stale) != 1 || stale[0] != 7*time.Hour {
		t.Errorf("CleanStale calls %v", stale)
	}
	if killed != 0 {
		t.Errorf("KillAll called %d times in a run without failures", killed)
	}
	// Nothing to run (everything resumed): the machine is not touched at all.
	stale = nil
	r.cfg.Resume = true
	r.run(item("run/a"))
	if len(stale) != 0 {
		t.Errorf("a run with nothing to do cleaned the machine: %v", stale)
	}
}

// lockRemote records the order of calls to check that the lock covers the machine's work.
type lockRecorder struct {
	mu     sync.Mutex
	events []string
	held   bool
}

func (l *lockRecorder) Lock(ctx context.Context) (func(), error) {
	l.mu.Lock()
	l.events = append(l.events, "lock")
	l.held = true
	l.mu.Unlock()
	return func() { l.mu.Lock(); l.events = append(l.events, "unlock"); l.held = false; l.mu.Unlock() }, nil
}

type heldRemote struct {
	Remote
	l *lockRecorder
	t *testing.T
}

func (h *heldRemote) Agent(ctx context.Context, d time.Duration, args ...string) (AgentRun, error) {
	h.l.mu.Lock()
	held := h.l.held
	h.l.mu.Unlock()
	if !held {
		h.t.Error("the agent runs without the lock")
	}
	return h.Remote.Agent(ctx, d, args...)
}

func TestRunnerHoldsTheLockWhileTheMachineWorks(t *testing.T) {
	r := newRig(t, map[string]string{"run/a": okScript("run/a"), "run/b": okScript("run/b"), "run/c": okScript("run/c")})
	r.cfg.BatchSize = 2
	l := &lockRecorder{}
	r.cfg.Lock = l
	r.cfg.Remote = &heldRemote{Remote: r.remote, l: l, t: t}
	r.run(item("run/a"), item("run/b"), item("run/c"))
	// start (lock, unlock), then one lock for each of the two batches.
	if got := strings.Join(l.events, ","); got != "lock,unlock,lock,unlock,lock,unlock" {
		t.Errorf("lock events %s", got)
	}
}

func TestResultsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.json")
	if _, ok, err := LoadResults(path); ok || err != nil {
		t.Fatalf("missing file: %v %v", ok, err)
	}
	os.WriteFile(path, []byte("{broken"), 0o644)
	if _, _, err := LoadResults(path); err == nil || !strings.Contains(err.Error(), "delete it") {
		t.Errorf("broken file: %v", err)
	}
	r := NewResults("g", "t")
	r.Tests["run/x"] = &Entry{Status: Pass, Group: "run"}
	if err := r.Save(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Error("the temporary file is left")
	}
	got, ok, err := LoadResults(path)
	if err != nil || !ok || got.Tests["run/x"].Status != Pass || got.Generation != "g" {
		t.Errorf("round trip: %v %v %+v", err, ok, got)
	}
}

// What an Item carries reaches the program, and what the program does comes back whole.
func TestRunnerPlumbing(t *testing.T) {
	r := newRig(t, map[string]string{
		"run/plumb": `read line; echo "stdin=$line"; echo "args=$1|$2"; echo "env=$WIN_X"; echo "err=oops" >&2; exit 4`,
		"run/junk":  "unused",
	})
	var got Outcome
	r.cfg.Check = func(it Item, o Outcome) []string {
		if it.Name == "run/plumb" {
			got = o
		}
		return nil
	}
	plumb := item("run/plumb")
	plumb.Args, plumb.Env, plumb.Stdin = []string{"a b", "c"}, []string{"WIN_X=1 2"}, []byte("typed\n")
	// A file that is no program: the agent cannot start it, and that is a failure with the reason, not a pass.
	junk := item("run/junk")
	r.cfg.Build = func(ctx context.Context, it Item) (string, error) {
		p := filepath.Join(r.dir, "build-"+strings.ReplaceAll(it.Name, "/", "_"))
		body := "#!/bin/sh\n" + `read line; echo "stdin=$line"; echo "args=$1|$2"; echo "env=$WIN_X"; echo "err=oops" >&2; exit 4` + "\n"
		if it.Name == "run/junk" {
			body = "\x00\x01 not a program"
		}
		return p, os.WriteFile(p, []byte(body), 0o755)
	}
	rep := r.run(plumb, junk)
	status(t, rep, "run/plumb", Pass)
	if want := "stdin=typed\nargs=a b|c\nenv=1 2\n"; string(got.Stdout) != want || string(got.Stderr) != "err=oops\n" || got.Exit != 4 {
		t.Errorf("outcome: %q %q exit %d", got.Stdout, got.Stderr, got.Exit)
	}
	e := status(t, rep, "run/junk", Fail)
	if !strings.Contains(e.Reason, "did not start") {
		t.Errorf("a program that does not start: %q", e.Reason)
	}
}

func TestSummaryGroupsAndPhasesAreSorted(t *testing.T) {
	res := NewResults("g", "t")
	for i := 0; i < 12; i++ {
		g := fmt.Sprintf("group%02d", (i*7)%12)
		res.Tests[g+"/x"] = &Entry{Status: Pass, Group: g, Phase: (i * 5) % 12}
	}
	want := (&Report{Results: res}).Summary()
	for i := 0; i < 20; i++ {
		if got := (&Report{Results: res}).Summary(); got != want {
			t.Fatalf("the summary changes between calls:\n%s\n%s", got, want)
		}
	}
	var groups []string
	for _, l := range strings.Split(want, "\n") {
		if strings.HasPrefix(l, "  group") {
			groups = append(groups, strings.Fields(l)[0])
		}
	}
	for i := 1; i < len(groups); i++ {
		if groups[i-1] > groups[i] {
			t.Errorf("groups are not sorted: %v", groups)
		}
	}
	var phases []string
	for _, l := range strings.Split(want, "\n") {
		if strings.HasPrefix(l, "  phase") {
			phases = append(phases, strings.Fields(l)[1])
		}
	}
	for i := 1; i < len(phases); i++ {
		if phases[i-1] > phases[i] {
			t.Errorf("phases are not sorted: %v", phases)
		}
	}
}

type failingLock struct{}

func (failingLock) Lock(context.Context) (func(), error) {
	return nil, errors.New("the Windows machine is in use by another run")
}

func TestRunThatDoesNotStartSaysSo(t *testing.T) {
	r := newRig(t, map[string]string{"run/a": okScript("run/a"), "run/b": okScript("run/b")})
	r.cfg.Lock = failingLock{}
	skipped := item("run/s")
	skipped.Skip, skipped.SkipKind = "no", "k"
	rep, err := New(r.cfg).Run(context.Background(), []Item{item("run/a"), item("run/b"), skipped})
	if err == nil || rep == nil {
		t.Fatalf("Run: %v %v", rep, err)
	}
	sum := rep.Summary()
	for _, want := range []string{"RUN DID NOT COMPLETE: the run did not start: the Windows machine is in use by another run", "1 of 3 tests have a result"} {
		if !strings.Contains(sum, want) {
			t.Errorf("the summary lacks %q:\n%s", want, sum)
		}
	}
	if rep.OK() {
		t.Error("an incomplete run is not OK")
	}
	if r.builds.Load() != 0 {
		t.Error("something was built for a run that did not start")
	}
}

func TestRunStopsWithoutRoom(t *testing.T) {
	scripts := map[string]string{"run/a": okScript("run/a"), "run/b": okScript("run/b")}
	t.Run("machine", func(t *testing.T) {
		r := newRig(t, scripts)
		r.remote.Free = func() (int64, error) { return 5, nil }
		rep, err := New(r.cfg).Run(context.Background(), []Item{item("run/a"), item("run/b")})
		if err == nil || !strings.Contains(err.Error(), "not enough free disk space on the machine: 5 B free") {
			t.Fatalf("err = %v", err)
		}
		if !strings.Contains(rep.Summary(), "RUN DID NOT COMPLETE: not enough free disk space on the machine") {
			t.Errorf("summary:\n%s", rep.Summary())
		}
		if rep.Results.Tests["run/a"] != nil && rep.Results.Tests["run/a"].Status == Error {
			t.Error("a test that never ran must have no result")
		}
	})
	t.Run("machine unknown or enough", func(t *testing.T) {
		r := newRig(t, scripts)
		r.remote.Free = func() (int64, error) { return -1, nil }
		status(t, r.run(item("run/a")), "run/a", Pass)
		r = newRig(t, scripts)
		r.remote.Free = func() (int64, error) { return 1 << 40, nil }
		status(t, r.run(item("run/a")), "run/a", Pass)
		r = newRig(t, scripts)
		r.remote.Free = func() (int64, error) { return 0, errors.New("powershell failed") }
		status(t, r.run(item("run/a")), "run/a", Pass) // a failed question does not stop the run
	})
	t.Run("here", func(t *testing.T) {
		r := newRig(t, scripts)
		r.cfg.BatchSize = 10
		r.cfg.WorkFree = func() (int64, error) { return 100 << 20, nil } // a first batch of 2 programs needs 2 x 2 x 12 MiB
		if _, err := New(r.cfg).Run(context.Background(), []Item{item("run/a"), item("run/b")}); err != nil {
			t.Fatalf("100 MiB is enough for 48: %v", err) // the run goes on
		}
		r = newRig(t, scripts)
		r.cfg.BatchSize = 10
		r.cfg.WorkFree = func() (int64, error) { return 10 << 20, nil }
		_, err := New(r.cfg).Run(context.Background(), []Item{item("run/a"), item("run/b")})
		if err == nil || !strings.Contains(err.Error(), "10 MiB free, 48 MiB needed") {
			t.Fatalf("err = %v", err)
		}
		if r.builds.Load() != 0 {
			t.Error("a build started without room")
		}
	})
}

// Run does not return while a build is still running: after an early end (here: no room on the machine for the
// first batch while the second is being built) nothing may write into the work folder.
func TestRunWaitsForBuildsOnAnEarlyEnd(t *testing.T) {
	r := newRig(t, map[string]string{"run/a": okScript("run/a"), "run/b": okScript("run/b")})
	r.cfg.BatchSize = 1
	r.remote.Free = func() (int64, error) { return 1, nil }
	var finished atomic.Bool
	inner := r.cfg.Build
	r.cfg.Build = func(ctx context.Context, it Item) (string, error) {
		if it.Name == "run/b" {
			time.Sleep(500 * time.Millisecond)
			finished.Store(true)
		}
		return inner(ctx, it)
	}
	if _, err := New(r.cfg).Run(context.Background(), []Item{item("run/a"), item("run/b")}); err == nil {
		t.Fatal("the run should have stopped for lack of room")
	}
	if !finished.Load() {
		t.Error("Run returned while a build was still running")
	}
}
