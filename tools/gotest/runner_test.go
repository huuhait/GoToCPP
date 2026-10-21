//go:build unix

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// fakeGocpp is a stand-in for the gocpp program: build writes a shell script
// that prints the //OUT: lines of the source (and exits 3 on EXIT3, sleeps
// on SLEEP, and records its pid in a file named by PIDFILE); a source with
// BUILDFAIL does not build. check prints a diagnostic for every //DIAG:
// line of the source and exits 1 when there is one; like the real checker, it also reports a
// function without a body ("func f()") unless GOCPP_ALLOW_BODYLESS=1.
const fakeGocpp = `#!/bin/sh
cmd=$1; shift
case "$cmd" in
version) echo "gocpp fake"; exit 0;;
build)
  out=; src=
  while [ $# -gt 0 ]; do
    case "$1" in
      -o) out=$2; shift 2;;
      -asan) shift;;
      *) src=$1; shift;;
    esac
  done
  all() { if [ -d "$src" ]; then cat "$src"/*.gp; else cat "$src"; fi; }
  if all | grep -q BUILDFAIL; then echo "./$(basename "$src"):3:1: boom" >&2; exit 1; fi
  if [ -d "$src" ] && all | grep -q '^package main' && ! all | grep -q 'func main()'; then echo "function main is undeclared in the main package" >&2; exit 1; fi
  if all | grep -q '^func [a-z]*()$'; then echo "undefined symbol: a function without a body" >&2; exit 1; fi
  if all | grep -q BUILDSLEEP; then sleep 60; fi
  {
    echo '#!/bin/sh'
    all | grep '^//OUT:' | sed 's|^//OUT:||; s|.*|echo "&"|'
    all | grep -q ARGS && echo 'echo "args=$*"'
    all | grep -q EXIT3 && echo 'exit 3'
    all | grep -q SLEEPFORK && echo '(sleep 60 & echo $! > "$PIDFILE") ; sleep 60'
    all | grep -q SLEEP && ! all | grep -q SLEEPFORK && echo 'sleep 60'
  } > "$out"
  chmod +x "$out"
  exit 0;;
check)
  src=
  while [ $# -gt 0 ]; do
    case "$1" in
      -e) shift;;
      *) src=$1; shift;;
    esac
  done
  n=0; any=0
  all() { if [ -d "$src" ]; then cat "$src"/*.gp; else cat "$src"; fi; }
  all | while IFS= read -r line; do
    n=$((n+1))
    case "$line" in
      *//DIAG:*) any=1; echo "./$(basename "$src"):$n:1: ${line##*//DIAG:}";;
      'func '[a-z]'()') if [ "$GOCPP_ALLOW_BODYLESS" != 1 ]; then echo "./$(basename "$src"):$n:6: missing function body"; fi;;
    esac
  done > "${TMPDIR:-/tmp}/fake-check.$$"
  cat "${TMPDIR:-/tmp}/fake-check.$$"
  if [ -s "${TMPDIR:-/tmp}/fake-check.$$" ]; then rm -f "${TMPDIR:-/tmp}/fake-check.$$"; exit 1; fi
  rm -f "${TMPDIR:-/tmp}/fake-check.$$"
  exit 0;;
esac
echo "fake gocpp: unknown command $cmd" >&2
exit 2
`

// fakeGo plays real Go for runoutput: it prints the program the test file
// asks for in its //GEN: lines.
const fakeGo = `#!/bin/sh
shift # run
file=$1
case "$file" in /*) echo "go run with an absolute path: an argument that is a .go file would not be in the same folder" >&2; exit 1;; esac
grep '^//GEN:' "$file" | sed 's|^//GEN:||'
`

func newTestRunner(t *testing.T, files map[string]string) *Runner {
	t.Helper()
	root := writeTree(t, files)
	bin := t.TempDir()
	for name, content := range map[string]string{"gocpp": fakeGocpp, "go": fakeGo} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r := NewRunner(RunConfig{Root: root, Gocpp: filepath.Join(bin, "gocpp"), Go: filepath.Join(bin, "go"),
		Work: t.TempDir(), BuildTimeout: 20 * time.Second, RunTimeout: 20 * time.Second})
	t.Cleanup(r.StopAll)
	return r
}

func runOne(t *testing.T, r *Runner, path, directive string) Result {
	t.Helper()
	return r.RunTest(Test{Path: path, Directive: directive, Status: "applicable"})
}

func TestRunnerRun(t *testing.T) {
	r := newTestRunner(t, map[string]string{
		"ok.go":        "// run\n\npackage main\n\n//OUT:hello\n",
		"ok.out":       "hello\n",
		"empty.go":     "// run\n\npackage main\n",
		"diff.go":      "// run\n\npackage main\n\n//OUT:bye\n",
		"diff.out":     "hello\n",
		"exit.go":      "// run\n\npackage main\n\n//OUT:hello\n//EXIT3\n",
		"exit.out":     "hello\n",
		"build.go":     "// run\n\npackage main\n\n//BUILDFAIL\n",
		"args.go":      "// run arg1 'a b'\n\npackage main\n\n//ARGS\n",
		"args.out":     "args=arg1 a b\n",
		"sleep.go":     "// run\n\npackage main\n\n//SLEEP\n",
		"buildrun.go":  "// buildrun\n\npackage main\n\n//OUT:x\n",
		"buildrun.out": "x\n",
	})
	r.RunTimeout = 400 * time.Millisecond
	for _, c := range []struct {
		path, outcome, stage, cluster string
	}{
		{"ok.go", Pass, "", ""},
		{"empty.go", Pass, "", ""},
		{"diff.go", Fail, "compare", "output differs"},
		{"exit.go", Fail, "run", "run: exit status N"},
		{"build.go", Fail, "build", "gocpp rejects: boom"},
		{"args.go", Pass, "", ""},
		{"sleep.go", Timeout, "run", "timeout: run"},
		{"buildrun.go", Pass, "", ""},
	} {
		res := runOne(t, r, c.path, "run")
		if res.Outcome != c.outcome || res.Stage != c.stage || res.Cluster != c.cluster {
			t.Errorf("%s: outcome %s stage %s cluster %q (%s), want %s %s %q", c.path, res.Outcome, res.Stage, res.Cluster, res.Detail, c.outcome, c.stage, c.cluster)
		}
	}
}

func TestRunnerBuildTimeout(t *testing.T) {
	r := newTestRunner(t, map[string]string{"slow.go": "// run\n\npackage main\n\n//BUILDSLEEP\n"})
	r.BuildTimeout = 300 * time.Millisecond
	res := runOne(t, r, "slow.go", "run")
	if res.Outcome != Timeout || res.Stage != "build" {
		t.Errorf("%+v", res)
	}
}

func TestRunnerCompile(t *testing.T) {
	r := newTestRunner(t, map[string]string{
		"c1.go": "// compile\n\npackage main\n\nfunc main() {}\n",
		"c2.go": "// compile\n\npackage p\n",
		"c3.go": "// compile\n\npackage main\n\nfunc main() {}\n//BUILDFAIL\n",
		"c4.go": "// compile\n\npackage main\n\n//DIAG:bad\n",
		"c5.go": "// compile\n\npackage main\n",
	})
	for _, c := range []struct{ path, outcome, stage string }{
		{"c1.go", Pass, ""}, {"c2.go", Pass, ""}, {"c3.go", Fail, "build"}, {"c4.go", Fail, "check"}, {"c5.go", Pass, ""},
	} {
		if res := runOne(t, r, c.path, "compile"); res.Outcome != c.outcome || res.Stage != c.stage {
			t.Errorf("%s: %+v", c.path, res)
		}
	}
}

func TestRunnerErrorcheck(t *testing.T) {
	// the diagnostics of the fake gocpp come from //DIAG: and the
	// expectations from // ERROR, as in Go's tests
	r := newTestRunner(t, map[string]string{
		"ok.go":      "// errorcheck\n\npackage p\n\nvar a = b // ERROR \"undefined: b\" //DIAG:undefined: b\n",
		"text.go":    "// errorcheck\n\npackage p\n\nvar a = b // ERROR \"expected\" //DIAG:something else\n",
		"extra.go":   "// errorcheck\n\npackage p\n\nvar a = b //DIAG:oops\n",
		"valid.go":   "// errorcheck -0\n\npackage p\n\nvar a = b //DIAG:oops\n",
		"clean.go":   "// errorcheck -0\n\npackage p\n\nvar a int\n",
		"missing.go": "// errorcheck\n\npackage p\n\nvar b int // ERROR \"expected\"\n",
	})
	for _, c := range []struct{ path, outcome, cluster string }{
		{"ok.go", Pass, ""},
		{"text.go", Fail, "errorcheck: message text differs: got something else"},
		// errorcheck expects at least one error: gocpp reports one that has no comment
		{"extra.go", Fail, "errorcheck: extra error: oops"},
		// errorcheck -0: gocpp must report nothing, it reports one
		{"valid.go", Fail, "errorcheck: gocpp rejects a valid program: oops"},
		{"clean.go", Pass, ""},
		// Go would report an error, gocpp accepts the program
		{"missing.go", Fail, "errorcheck: gocpp accepts what Go rejects"},
	} {
		res := runOne(t, r, c.path, "errorcheck")
		if res.Outcome != c.outcome || res.Cluster != c.cluster {
			t.Errorf("%s: outcome %s cluster %q (%s), want %s %q", c.path, res.Outcome, res.Cluster, res.Detail, c.outcome, c.cluster)
		}
	}
}

func TestRunnerRunoutput(t *testing.T) {
	r := newTestRunner(t, map[string]string{
		"gen.go":   "// runoutput\n\npackage main\n\n//GEN:package main\n//GEN://OUT:generated\n",
		"gen.out":  "generated\n",
		"gen2.go":  "// runoutput ./gen.go\n\npackage main\n\n//GEN:package main\n//GEN://OUT:generated\n",
		"gen2.out": "generated\n",
		"bad.go":   "// runoutput\n\npackage main\n\n//GEN:package main\n//GEN://OUT:generated\n",
		"bad.out":  "other\n",
	})
	if res := runOne(t, r, "gen.go", "runoutput"); res.Outcome != Pass {
		t.Errorf("gen: %+v", res)
	}
	if res := runOne(t, r, "gen2.go", "runoutput"); res.Outcome != Pass {
		t.Errorf("gen2: %+v", res)
	}
	if res := runOne(t, r, "bad.go", "runoutput"); res.Outcome != Fail || res.Stage != "compare" {
		t.Errorf("bad: %+v", res)
	}
}

func TestRunnerRunoutputKeepsALongProgram(t *testing.T) {
	// the generated program is longer than the limit of a test's own output
	var gen strings.Builder
	gen.WriteString("//GEN:package main\n")
	for i := 0; i < 20000; i++ {
		gen.WriteString("//GEN://OUT:line\n")
	}
	r := newTestRunner(t, map[string]string{"long.go": "// runoutput\n\npackage main\n\n" + gen.String(), "long.out": strings.Repeat("line\n", 20000)})
	r.OutLimit = 200 << 10
	if res := runOne(t, r, "long.go", "runoutput"); res.Outcome != Pass {
		t.Errorf("%+v", res)
	}
}

func TestRunnerUnknownDirective(t *testing.T) {
	r := newTestRunner(t, map[string]string{"a.go": "// asmcheck\n\npackage main\n"})
	if res := runOne(t, r, "a.go", "asmcheck"); res.Outcome != Fail || res.Stage != "runner" {
		t.Errorf("%+v", res)
	}
	if res := runOne(t, r, "missing.go", "run"); res.Outcome != Fail || res.Stage != "runner" {
		t.Errorf("missing file: %+v", res)
	}
}

// A test program that forked must not outlive the test: the whole process
// group dies after the time limit.
func TestRunnerKillsProcessGroup(t *testing.T) {
	pidfile := filepath.Join(t.TempDir(), "pid")
	r := newTestRunner(t, map[string]string{"fork.go": "// run\n\npackage main\n\n//SLEEPFORK\n"})
	t.Setenv("PIDFILE", pidfile)
	r.RunTimeout = 300 * time.Millisecond
	res := runOne(t, r, "fork.go", "run")
	if res.Outcome != Timeout {
		t.Fatalf("%+v", res)
	}
	b, err := os.ReadFile(pidfile)
	if err != nil {
		t.Fatalf("the program did not start its child: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	// the kill is asynchronous for a reparented process: poll a bounded
	// number of times
	for i := 0; i < 200; i++ {
		if err := syscall.Kill(pid, 0); err != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	syscall.Kill(pid, syscall.SIGKILL)
	t.Errorf("the child %d of the test program is still alive", pid)
}

func TestRunTestsOrderAndStop(t *testing.T) {
	files := map[string]string{}
	var ts []Test
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		files[n+".go"] = "// run\n\npackage main\n\n//OUT:" + n + "\n"
		files[n+".out"] = n + "\n"
		ts = append(ts, Test{Path: n + ".go", Directive: "run", Status: "applicable"})
	}
	files["c.out"] = "wrong\n"
	r := newTestRunner(t, files)
	var seen int
	res := runTests(r, ts, 3, func(Result, int, int) { seen++ })
	if seen != 5 || len(res) != 5 {
		t.Fatalf("seen %d, results %d", seen, len(res))
	}
	for i, x := range res {
		if x.Path != ts[i].Path {
			t.Errorf("order: result %d is %s", i, x.Path)
		}
		if want := map[bool]string{true: Fail, false: Pass}[x.Path == "c.go"]; x.Outcome != want {
			t.Errorf("%s: %s", x.Path, x.Outcome)
		}
	}
	// after StopAll nothing runs any more
	r.StopAll()
	for _, x := range runTests(r, ts, 2, nil) {
		if x.Outcome != Skipped || x.Detail != "interrupted" {
			t.Errorf("after stop: %+v", x)
		}
	}
}

func TestSelectionFilters(t *testing.T) {
	var ts []Test
	for _, p := range []string{"a.go", "b.go", "c.go", "fixedbugs/x.go", "fixedbugs/y.go", "ken/z.go"} {
		ts = append(ts, Test{Path: p, Directive: "run", Status: "applicable"})
	}
	ts = append(ts, Test{Path: "s.go", Directive: "run", Status: "skip:gc-flags"}, Test{Path: "e.go", Directive: "errorcheck", Status: "applicable"})
	names := func(x []Test, err error) string {
		if err != nil {
			return "error: " + err.Error()
		}
		var n []string
		for _, t := range x {
			n = append(n, t.Path)
		}
		return strings.Join(n, " ")
	}
	if got := names(selection{}.pick(ts)); got != "a.go b.go c.go fixedbugs/x.go fixedbugs/y.go ken/z.go e.go" {
		t.Errorf("all: %s", got)
	}
	if got := names(selection{Dir: "."}.pick(ts)); got != "a.go b.go c.go e.go" {
		t.Errorf("top: %s", got)
	}
	if got := names(selection{Dir: "fixedbugs"}.pick(ts)); got != "fixedbugs/x.go fixedbugs/y.go" {
		t.Errorf("dir: %s", got)
	}
	if got := names(selection{Directive: "errorcheck"}.pick(ts)); got != "e.go" {
		t.Errorf("directive: %s", got)
	}
	if got := names(selection{Filter: regexp.MustCompile(`^k`)}.pick(ts)); got != "ken/z.go" {
		t.Errorf("filter: %s", got)
	}
	if got := names(selection{Limit: 2}.pick(ts)); got != "a.go b.go" {
		t.Errorf("limit: %s", got)
	}
	if got := names(selection{Paths: map[string]bool{"b.go": true, "s.go": true}}.pick(ts)); got != "b.go" {
		t.Errorf("paths: %s", got)
	}
	// shards partition the tests
	total := 0
	for i := 0; i < 3; i++ {
		x, err := selection{Shard: strings.Replace("I/3", "I", string(rune('0'+i)), 1)}.pick(ts)
		if err != nil {
			t.Fatal(err)
		}
		total += len(x)
	}
	if total != 7 {
		t.Errorf("three shards hold %d tests, want 7", total)
	}
	for _, bad := range []string{"3/3", "x", "1/0", "-1/2", "1"} {
		if _, err := (selection{Shard: bad}).pick(ts); err == nil {
			t.Errorf("shard %q accepted", bad)
		}
	}
}

func TestCompileWithBodylessFunc(t *testing.T) {
	// Go compiles and does not link: a function without a body is fine; gocpp checks only
	r := newTestRunner(t, map[string]string{
		"nb.go":        "// compile\n\npackage p\n\nfunc f()\n",
		"nbd.go":       "// compiledir\n\npackage ignored\n",
		"nbd.dir/a.go": "package a\n\nfunc g()\n",
		"bad.go":       "// compile\n\npackage p\n\nfunc f()\n\n//DIAG:no good\n",
		"ec.go":        "// errorcheck\n\npackage p\n\nfunc f()\n\nvar a = b // ERROR \"undefined: b\" //DIAG:undefined: b\n",
	})
	if res := runOne(t, r, "nb.go", "compile"); res.Outcome != Pass {
		t.Errorf("compile: %+v", res)
	}
	if res := runOne(t, r, "nbd.go", "compiledir"); res.Outcome != Pass {
		t.Errorf("compiledir: %+v", res)
	}
	if res := runOne(t, r, "bad.go", "compile"); res.Outcome != Fail || res.Stage != "check" {
		t.Errorf("a bodyless function does not hide errors: %+v", res)
	}
	if res := runOne(t, r, "ec.go", "errorcheck"); res.Outcome != Pass {
		t.Errorf("errorcheck: %+v", res)
	}
}

// TestRunnerAllowsBodylessFunctions: gocpp check refuses a function without a body, and Go's test
// files declare them for assembly, so every gocpp command of the runner (also the ones for the
// packages of a .dir test and for the output of a generator) must ask for them to be accepted.
func TestRunnerAllowsBodylessFunctions(t *testing.T) {
	r := newTestRunner(t, nil)
	env := r.gocppEnv()
	if !slices.Contains(env, "GOCPP_ALLOW_BODYLESS=1") {
		t.Errorf("the environment of gocpp lacks GOCPP_ALLOW_BODYLESS=1: %v", env)
	}
}

func TestCompiledirWithoutMain(t *testing.T) {
	// a package main without func main is only checked; the other package is built
	r := newTestRunner(t, map[string]string{
		"cd.go":       "// compiledir\n\npackage ignored\n",
		"cd.dir/a.go": "package a\n\nfunc F() {}\n",
		"cd.dir/b.go": "package main\n\nimport \"./a\"\n\nvar _ = a.F\n",
	})
	if res := runOne(t, r, "cd.go", "compiledir"); res.Outcome != Pass {
		t.Errorf("%+v", res)
	}
}

func TestDirTestFake(t *testing.T) {
	// a rundir test: two packages, the main one last
	r := newTestRunner(t, map[string]string{
		"d.go":       "// rundir\n\npackage ignored\n",
		"d.dir/a.go": "package a\n\nfunc F() {}\n",
		"d.dir/b.go": "package main\n\nimport \"./a\"\n\nfunc main() { a.F() }\n//OUT:dir\n",
		"d.out":      "dir\n",
	})
	res := runOne(t, r, "d.go", "rundir")
	if res.Outcome != Pass {
		t.Errorf("%+v", res)
	}
}
