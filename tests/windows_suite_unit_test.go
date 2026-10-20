package tests

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/winssh"
	"github.com/huuhait/gocpp/internal/wintest"
)

// The parts of the Windows suite that need neither the PC nor a build: the environment, the selection,
// the skip list and the smoke list. (The driver itself is tested in internal/wintest with a fake machine.)

func envMap(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestWindowsSuiteEnv(t *testing.T) {
	read := func(m map[string]string) (suiteEnv, error) {
		return readSuiteEnv(envMap(m), func(p string) ([]byte, error) {
			if p == "list.txt" {
				return []byte("# comment\nrun/a\n\npkgs/b\n"), nil
			}
			return nil, errors.New("no such file")
		})
	}
	e, err := read(nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(e.dirs, ",") != "run,pkgs" || e.jobs != 8 || e.buildJobs != 12 || e.batch != 150 || e.runTimeout != 2*time.Minute ||
		e.skips != "check" || e.resume || e.list || e.wine || e.only != nil || e.lock != "/tmp/gocpp-windows-pc.lock" || e.staleAge != 12*time.Hour {
		t.Errorf("defaults: %+v", e)
	}
	e, err = read(map[string]string{"GOCPP_WINDOWS_ONLY": "^run/(a|b)$", "GOCPP_WINDOWS_RESUME": "1", "GOCPP_WINDOWS_LIST": "1",
		"GOCPP_WINDOWS_JOBS": "3", "GOCPP_WINDOWS_TIMEOUT": "90s", "GOCPP_WINDOWS_DIRS": "run, interop", "GOCPP_WINDOWS_SKIPS": "ignore", "GOCPP_WINDOWS_STALE_AGE": "3h"})
	if err != nil || !e.only("run/a") || e.only("run/c") || !e.resume || !e.list || e.jobs != 3 || e.runTimeout != 90*time.Second ||
		strings.Join(e.dirs, ",") != "run,interop" || e.skips != "ignore" || e.staleAge != 3*time.Hour {
		t.Errorf("set: %+v %v", e, err)
	}
	if e, err = read(map[string]string{"GOCPP_WINDOWS_RESUME": "failed"}); err != nil || !e.resume || !e.anyGen {
		t.Errorf("resume=failed: %+v %v", e, err)
	}
	if e, err = read(map[string]string{"GOCPP_WINDOWS_RESUME": "1"}); err != nil || !e.resume || e.anyGen {
		t.Errorf("resume=1: %+v %v", e, err)
	}
	e, err = read(map[string]string{"GOCPP_WINDOWS_ONLY": "@list.txt"})
	if err != nil || !e.only("run/a") || !e.only("pkgs/b") || e.only("run/b") {
		t.Errorf("a list file: %v", err)
	}
	for name, m := range map[string]map[string]string{
		"GOCPP_WINDOWS_ONLY":      {"GOCPP_WINDOWS_ONLY": "(["},
		"@missing":                {"GOCPP_WINDOWS_ONLY": "@missing.txt"},
		"GOCPP_WINDOWS_DIRS":      {"GOCPP_WINDOWS_DIRS": "run,errors"},
		"GOCPP_WINDOWS_JOBS":      {"GOCPP_WINDOWS_JOBS": "0"},
		"GOCPP_WINDOWS_TIMEOUT":   {"GOCPP_WINDOWS_TIMEOUT": "soon"},
		"GOCPP_WINDOWS_RESUME":    {"GOCPP_WINDOWS_RESUME": "yes"},
		"GOCPP_WINDOWS_SKIPS":     {"GOCPP_WINDOWS_SKIPS": "maybe"},
		"GOCPP_WINDOWS_BATCH":     {"GOCPP_WINDOWS_BATCH": "x"},
		"GOCPP_WINDOWS_LOCK_WAIT": {"GOCPP_WINDOWS_LOCK_WAIT": "-1s"},
	} {
		if _, err := read(m); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// The suite sees every run test and every pkgs test, decides each, and the lists agree with the tests.
func TestWindowsSuiteSelection(t *testing.T) {
	c := *setup(t)
	c.phase, c.phaseFrom = lastPhase, "every phase"
	env, err := readSuiteEnv(envMap(nil), os.ReadFile)
	if err != nil {
		t.Fatal(err)
	}
	items, progs := c.suiteItems(t, env) // reports a skip-list name that is not a test
	if t.Failed() {
		return
	}
	byName := make(map[string]wintest.Item)
	groups := map[string]int{}
	for _, it := range items {
		byName[it.Name] = it
		groups[it.Group]++
		if it.Key == "" || progs[it.Name] == nil {
			t.Errorf("%s: no key or program", it.Name)
		}
	}
	if groups["run"] < 1500 || groups["pkgs"] < 40 {
		t.Errorf("found %v", groups)
	}
	skips, err := suiteSkips()
	if err != nil {
		t.Fatal(err)
	}
	for name, reason := range skips {
		it, ok := byName[name]
		if !ok {
			t.Errorf("%s is in %s but not a test", name, windowsSkipFile)
			continue
		}
		if it.Skip == "" && it.Xfail != reason {
			t.Errorf("%s: expected failure %q, want %q", name, it.Xfail, reason)
		}
	}
	// The tests that TestRunWindows runs are not expected failures, and run here.
	_, sample, err := readNameList(windowsSampleFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range sample {
		it, ok := byName["run/"+n]
		if !ok || it.Xfail != "" || it.Skip != "" {
			t.Errorf("run/%s of %s: %+v", n, windowsSampleFile, it)
		}
	}
	// The smoke list names tests that run (or two expected failures).
	data, err := os.ReadFile(filepath.Join(repoRoot, "tests", windowsSmokeFile))
	if err != nil {
		t.Fatal(err)
	}
	var smoke, xfail int
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		smoke++
		it, ok := byName[line]
		switch {
		case !ok:
			t.Errorf("%s names %s, which is not a test", windowsSmokeFile, line)
		case it.Skip != "":
			t.Errorf("%s: %s is skipped (%s)", windowsSmokeFile, line, it.Skip)
		case it.Xfail != "":
			xfail++
		}
	}
	if smoke < 40 || smoke > 50 || xfail < 1 {
		t.Errorf("the smoke list has %d tests and %d expected failures", smoke, xfail)
	}
	// Every skip kind has a reason, and a test with a build line for Linux is skipped.
	for _, it := range items {
		if (it.Skip != "") != (it.SkipKind != "") {
			t.Errorf("%s: skip %q kind %q", it.Name, it.Skip, it.SkipKind)
		}
	}
	if it := byName["run/os_win_dir_remove"]; it.Skip != "" {
		t.Errorf("a //go:build windows test is skipped: %+v", it)
	}
}

func TestWindowsSuiteOnlyAndIgnore(t *testing.T) {
	c := *setup(t)
	c.phase, c.phaseFrom = lastPhase, "every phase"
	env, err := readSuiteEnv(envMap(map[string]string{"GOCPP_WINDOWS_ONLY": "^run/(os_path_long_names|chan_pingpong)$"}), os.ReadFile)
	if err != nil {
		t.Fatal(err)
	}
	items, _ := c.suiteItems(t, env)
	if len(items) != 2 {
		t.Fatalf("items %+v", items)
	}
	if items[0].Name != "run/chan_pingpong" && items[1].Name != "run/chan_pingpong" {
		t.Errorf("items %+v", items)
	}
	for _, it := range items {
		if it.Name == "run/os_path_long_names" && it.Xfail == "" {
			t.Error("a listed test is not an expected failure in the check mode")
		}
	}
	env.skips = "ignore"
	items, _ = c.suiteItems(t, env)
	for _, it := range items {
		if it.Name == "run/os_path_long_names" && (it.Xfail != "" || it.Skip == "") {
			t.Errorf("ignore mode: %+v", it)
		}
	}
}

func TestWindowsSuiteKeyFollowsTheSources(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.gp"), []byte("one"), 0o644)
	p := &program{dir: dir, arg: "a.gp"}
	k1, err := suiteKey(p)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "a.gp"), []byte("two"), 0o644)
	k2, _ := suiteKey(p)
	if k1 == k2 {
		t.Error("the key does not change with the file")
	}
	os.MkdirAll(filepath.Join(dir, "pkg"), 0o755)
	os.WriteFile(filepath.Join(dir, "main.gp"), []byte("m"), 0o644)
	os.WriteFile(filepath.Join(dir, "pkg", "x.gp"), []byte("x"), 0o644)
	folder := &program{dir: dir, arg: "."}
	f1, _ := suiteKey(folder)
	os.WriteFile(filepath.Join(dir, "pkg", "x.gp"), []byte("y"), 0o644)
	f2, _ := suiteKey(folder)
	if f1 == f2 {
		t.Error("the key of a folder test does not change with a file below it")
	}
}

func TestWindowsSuiteListing(t *testing.T) {
	items := []wintest.Item{
		{Name: "run/a"}, {Name: "run/b", Skip: "needs Phase 99"}, {Name: "run/c", Xfail: "gap"}, {Name: "run/d"},
	}
	todo := []wintest.Item{items[0], items[2]}
	got := suiteListing(items, todo, 1, 150)
	want := "run run/a\nskip run/b: needs Phase 99\nxfail run/c: gap\ndone run/d\n" +
		"4 tests: run 1 (in 1 batches of up to 150), xfail 1 (run, and must fail), done 1 (kept from the results file), skip 1\n"
	if got != want {
		t.Errorf("listing:\n%s\nwant:\n%s", got, want)
	}
}

// git runs git in dir for a test.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// The generation changes with a commit, a tracked edit and a new untracked file, and not with an ignored file.
func TestWindowsSuiteGeneration(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored/\n"), 0o644)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "first")
	g0 := generationOf(dir)
	if g0 == "no-git" || g0 != generationOf(dir) {
		t.Fatalf("generation %q is not stable", g0)
	}
	os.MkdirAll(filepath.Join(dir, "ignored"), 0o755)
	os.WriteFile(filepath.Join(dir, "ignored", "x"), []byte("x"), 0o644)
	if generationOf(dir) != g0 {
		t.Error("an ignored file changed the generation")
	}
	os.WriteFile(filepath.Join(dir, "new.gp"), []byte("package main\n"), 0o644)
	g1 := generationOf(dir)
	if g1 == g0 {
		t.Error("a new untracked file did not change the generation")
	}
	os.WriteFile(filepath.Join(dir, "new.gp"), []byte("package main // edited\n"), 0o644)
	g2 := generationOf(dir)
	if g2 == g1 {
		t.Error("an edit of an untracked file did not change the generation")
	}
	os.Remove(filepath.Join(dir, "new.gp"))
	if generationOf(dir) != g0 {
		t.Error("removing the untracked file did not bring the generation back")
	}
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("two\n"), 0o644)
	g3 := generationOf(dir)
	if g3 == g0 {
		t.Error("an edit of a tracked file did not change the generation")
	}
	git(t, dir, "commit", "-q", "-am", "second")
	if g4 := generationOf(dir); g4 == g3 || g4 == g0 {
		t.Error("a commit did not change the generation")
	}
	if got := generationOf(t.TempDir()); got != "no-git" {
		t.Errorf("a folder without git: %q", got)
	}
}

func TestCleanStaleWork(t *testing.T) {
	base := t.TempDir()
	now := time.Now()
	mk := func(name string, marker bool, age time.Duration) string {
		d := filepath.Join(base, name)
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "big.tgz"), []byte("x"), 0o644)
		if marker {
			m := filepath.Join(d, workMarker)
			os.WriteFile(m, nil, 0o644)
			os.Chtimes(m, now.Add(-age), now.Add(-age))
		}
		return d
	}
	old := mk("gocpp-winsuite-123456", true, 30*time.Hour)
	recent := mk("gocpp-winsuite-654321", true, time.Hour)
	noMarker := mk("gocpp-winsuite-111", false, 0)
	otherName := mk("gocpp-winsuite-abc", true, 30*time.Hour)
	otherTool := mk("gocpp-winsuite", true, 30*time.Hour)
	longer := mk("x-gocpp-winsuite-1", true, 30*time.Hour)
	file := filepath.Join(base, "gocpp-winsuite-999")
	os.WriteFile(file, []byte("x"), 0o644)
	got := cleanStaleWork([]string{base}, 12*time.Hour, now)
	if len(got) != 1 || got[0] != old {
		t.Errorf("removed %v, want only %s", got, old)
	}
	for _, d := range []string{recent, noMarker, otherName, otherTool, longer, file} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("%s was removed", d)
		}
	}
	if _, err := os.Stat(old); err == nil {
		t.Error("the old work folder is still there")
	}
}

// The gates of the selection: a //go:build line that excludes windows, and the phase.
func TestWindowsSuiteGates(t *testing.T) {
	env, err := readSuiteEnv(envMap(map[string]string{"GOCPP_WINDOWS_ONLY": `^run/(deadlock_after_pipe_close|os_win_dir_remove|chan_pingpong)$`}), os.ReadFile)
	if err != nil {
		t.Fatal(err)
	}
	kinds := func(phase int) map[string]string {
		c := *setup(t)
		c.phase, c.phaseFrom = phase, "the test"
		items, _ := c.suiteItems(t, env)
		m := map[string]string{}
		for _, it := range items {
			m[it.Name] = it.SkipKind
		}
		return m
	}
	all := kinds(lastPhase)
	if all["run/deadlock_after_pipe_close"] != "builds for another target" {
		t.Errorf("a //go:build unix test: kind %q", all["run/deadlock_after_pipe_close"])
	}
	if k, ok := all["run/os_win_dir_remove"]; !ok || k != "" {
		t.Errorf("a //go:build windows test: %q %v", k, ok)
	}
	if k := all["run/chan_pingpong"]; k != "" {
		t.Errorf("a plain test is skipped: %q", k)
	}
	low := kinds(1)
	if low["run/chan_pingpong"] != "needs a later phase" || low["run/os_win_dir_remove"] != "needs a later phase" {
		t.Errorf("phase gate at phase 1: %v", low)
	}
	// The gates are checked in this order: the phase first, so the reason a reader sees is the earliest one.
	if low["run/deadlock_after_pipe_close"] != "needs a later phase" {
		t.Errorf("phase gate before the build gate: %q", low["run/deadlock_after_pipe_close"])
	}
}

// The work folder of a run that was killed under `nix develop` is in a sibling TMPDIR; it is found, and nothing
// else in that TMPDIR is touched.
func TestCleanStaleWorkInNixShellTmp(t *testing.T) {
	root := t.TempDir()
	mine := filepath.Join(root, "nix-shell.mine")
	other := filepath.Join(root, "nix-shell.other")
	os.MkdirAll(mine, 0o755)
	os.MkdirAll(other, 0o755)
	old := time.Now().Add(-30 * time.Hour)
	mk := func(dir string, marker bool) string {
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "big.tgz"), []byte("x"), 0o644)
		if marker {
			m := filepath.Join(dir, workMarker)
			os.WriteFile(m, nil, 0o644)
			os.Chtimes(m, old, old)
		}
		return dir
	}
	stale := mk(filepath.Join(other, "gocpp-winsuite-77"), true)
	deeper := mk(filepath.Join(other, "sub", "gocpp-winsuite-78"), true)
	noMarker := mk(filepath.Join(other, "gocpp-winsuite-79"), false)
	foreign := mk(filepath.Join(other, "gocpp-build-1"), true)
	t.Setenv("TMPDIR", mine)
	bases := workBases()
	if len(bases) != 2 || bases[0] != mine || bases[1] != other {
		t.Fatalf("bases %v", bases)
	}
	// the bases are the siblings of this run's TMPDIR in root only if TMPDIR's parent is root: it is
	removed := cleanStaleWork(bases, 12*time.Hour, time.Now())
	if len(removed) != 1 || removed[0] != stale {
		t.Errorf("removed %v, want only %s", removed, stale)
	}
	for _, d := range []string{deeper, noMarker, foreign} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("%s was removed", d)
		}
	}
}

func TestWatchParent(t *testing.T) {
	stop := make(chan struct{})
	pid := 100
	var mu sync.Mutex
	get := func() int { mu.Lock(); defer mu.Unlock(); return pid }
	gone := make(chan struct{})
	go watchParent(get, 5*time.Millisecond, stop, func() { close(gone) })
	select {
	case <-gone:
		t.Fatal("reported a parent that is still the same")
	case <-time.After(60 * time.Millisecond):
	}
	mu.Lock()
	pid = 1
	mu.Unlock()
	select {
	case <-gone:
	case <-time.After(5 * time.Second):
		t.Fatal("an orphaned binary was not noticed")
	}
	// stopped before the parent changes: never reports
	stop2 := make(chan struct{})
	called := make(chan struct{}, 1)
	pid2 := 5
	go watchParent(func() int { return pid2 }, 5*time.Millisecond, stop2, func() { called <- struct{}{} })
	close(stop2)
	time.Sleep(20 * time.Millisecond)
	pid2 = 1
	select {
	case <-called:
		t.Error("reported after stop")
	case <-time.After(60 * time.Millisecond):
	}
}

// The stop hook of the harness (a signal) ends a wait for the PC lock: after windowsStop a connection is refused at once.
func TestWindowsStopInterruptsTheLockWait(t *testing.T) {
	defer winssh.Resume()
	windowsStop()
	start := time.Now()
	_, err := winssh.New("user@example.invalid")
	if err == nil || !strings.Contains(err.Error(), "stopped while waiting for the Windows machine") {
		t.Errorf("err = %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("it took too long")
	}
}
