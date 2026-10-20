package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/winssh"
)

// gocpp test on Windows (Phase 14, step 14.9): the natives of package testing for Windows
// (runtime/src/os/windows/testing_os.cpp) and the test binary that `gocpp test` makes, run on the real PC
// over ssh (GOCPP_WINDOWS_SSH=user@host). Before this step a test binary did not link there
// (undefined reference to gp::testing::cpu_name, make_temp_dir, set_env).
//
// The module of the cases (gotestModule) has passing, failing and skipping tests, t.TempDir (also a
// read-only file in it, which the cleanup must remove), t.Setenv (new and existing variables, a
// subtest, the case of the name), t.Cleanup order, t.Chdir, a benchmark and an example. The tests
// check the effects themselves (the folders are gone and the environment is back in the tests
// that run after); the harness checks the output of `go test` and the exit status, and that the
// run left no TempDir folder in TEMP.
//
//   - TestWindowsNativeGoTest: gocpp.exe on the PC runs `gocpp test` with the PC's MinGW-w64.
//   - TestWindowsCrossGoTest: `gocpp test -c -os windows` here (the cross build), and the test
//     binary runs on the PC.
//
// Both skip without GOCPP_WINDOWS_SSH (the cross one also without a MinGW-w64 cross compiler).

var gotestModule = map[string]string{
	"gocpp.mod": "module example.com/wt\n\ngo 1.25\n",
	"lib.gp": `package lib

func Add(a, b int) int { return a + b }

func Greet(s string) string { return "hello, " + s }
`,
	"lib_test.gp": `package lib

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

var startWD, _ = os.Getwd()

// (package path/filepath is not available yet: the helpers are here)
func base(p string) string { return p[strings.LastIndexByte(p, '\\')+1:] }

func dir(p string) string { return p[:strings.LastIndexByte(p, '\\')] }

var tempDirs []string

func TestPass(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Error("bad add")
	}
}

func TestFailing(t *testing.T) {
	t.Errorf("expected failure %d", Add(2, 2))
}

func TestSkip(t *testing.T) {
	t.Skip("skipping on purpose")
}

func TestTempDir(t *testing.T) {
	d, e := t.TempDir(), t.TempDir()
	if d == e || d == "" {
		t.Fatalf("bad temp dirs %q %q", d, e)
	}
	if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
		t.Fatalf("TempDir %q is not a folder: %v", d, err)
	}
	if strings.Contains(d, "/") || len(d) < 3 || d[1] != ':' || d[2] != '\\' {
		t.Errorf("TempDir %q is not a Windows path", d)
	}
	if !strings.Contains(base(dir(d)), "TestTempDir") {
		t.Errorf("TempDir %q has not the name of the test", d)
	}
	// a file and a read-only file: the cleanup must remove both
	if err := os.WriteFile(d+"\\a.txt", []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	ro := e+"\\ro.txt"
	if err := os.WriteFile(ro, []byte("r"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ro, 0o444); err != nil {
		t.Fatal(err)
	}
	tempDirs = append(tempDirs, d, e)
}

func TestTempDirRemoved(t *testing.T) {
	if len(tempDirs) != 2 {
		t.Fatalf("TestTempDir did not run: %q", tempDirs)
	}
	for _, d := range tempDirs {
		if _, err := os.Stat(d); err == nil {
			t.Errorf("%s still exists after the test", d)
		}
	}
}

func TestSetenv(t *testing.T) {
	t.Setenv("GOCPP_WT_NEW", "one")
	t.Setenv("GOCPP_WT_PRE", "changed")
	if got := os.Getenv("GOCPP_WT_NEW"); got != "one" {
		t.Errorf("GOCPP_WT_NEW = %q", got)
	}
	if got := os.Getenv("gocpp_wt_new"); got != "one" { // the names are not case sensitive on Windows
		t.Errorf("gocpp_wt_new = %q", got)
	}
	t.Run("sub", func(t *testing.T) {
		t.Setenv("GOCPP_WT_NEW", "two")
		if got := os.Getenv("GOCPP_WT_NEW"); got != "two" {
			t.Errorf("in sub: GOCPP_WT_NEW = %q", got)
		}
	})
	if got := os.Getenv("GOCPP_WT_NEW"); got != "one" {
		t.Errorf("after sub: GOCPP_WT_NEW = %q", got)
	}
}

func TestSetenvRestored(t *testing.T) {
	if v, ok := os.LookupEnv("GOCPP_WT_NEW"); ok {
		t.Errorf("GOCPP_WT_NEW is still set: %q", v)
	}
	if got := os.Getenv("GOCPP_WT_PRE"); got != "orig" {
		t.Errorf("GOCPP_WT_PRE = %q, want the value it had (orig)", got)
	}
}

func TestCleanupOrder(t *testing.T) {
	t.Cleanup(func() { fmt.Println("cleanup 1") })
	t.Cleanup(func() { fmt.Println("cleanup 2") })
}

func TestChdir(t *testing.T) {
	pwd := os.Getenv("PWD")
	d := t.TempDir()
	t.Chdir(d)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(base(wd), base(d)) {
		t.Errorf("Getwd = %q after Chdir(%q)", wd, d)
	}
	if v := os.Getenv("PWD"); v != pwd { // Windows does not use PWD
		t.Errorf("PWD was %q and is %q after Chdir", pwd, v)
	}
}

func TestChdirRestored(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil || wd != startWD {
		t.Errorf("Getwd = %q, %v; want %q", wd, err, startWD)
	}
}

var sink []byte

// The counting of allocations (runtime/src/testing_alloc.cpp replaces operator new): a cross-built
// binary once counted none, because the linker took libstdc++'s operator new.
func TestAllocs(t *testing.T) {
	n := testing.AllocsPerRun(100, func() { sink = make([]byte, 1024) })
	if n != 1 {
		t.Errorf("AllocsPerRun(make([]byte, 1024)) = %v, want 1", n)
	}
	if z := testing.AllocsPerRun(100, func() {}); z != 0 {
		t.Errorf("AllocsPerRun(empty) = %v, want 0", z)
	}
}

func BenchmarkAlloc(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sink = make([]byte, 1024)
	}
}

func BenchmarkAdd(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Add(i, i)
	}
}

func ExampleGreet() {
	fmt.Println(Greet("win"))
	fmt.Println(Greet("dows"))
	// Output:
	// hello, win
	// hello, dows
}

// Go's example.go compares "\r\n" as "\n" on Windows.
func ExampleCRLF() {
	fmt.Print("a\r\nb\n")
	// Output:
	// a
	// b
}
`,
}

var gotestTimeRx = regexp.MustCompile(`\d+\.\d+s`)

// gotestNorm normalizes the output of a test run: line ends and times.
func gotestNorm(b []byte) string {
	return gotestTimeRx.ReplaceAllString(normNL(b), "T")
}

// inOrder checks that the lines of out match the regular expressions wants (each one the whole
// line), in this order, with other lines allowed between them.
func inOrder(t *testing.T, what, out string, wants ...string) {
	t.Helper()
	lines := strings.Split(out, "\n")
	i := 0
	for _, w := range wants {
		rx := regexp.MustCompile(`^(?:` + w + `)$`)
		for i < len(lines) && !rx.MatchString(lines[i]) {
			i++
		}
		if i == len(lines) {
			t.Errorf("%s: no line %q after the previous one, in:\n%s", what, w, out)
			return
		}
		i++
	}
}

// checkGotestRuns checks the three runs of the module: the passing and skipping tests (without the
// failing one), all of them (the failing one makes the exit status 1), and a benchmark run.
func checkGotestRuns(t *testing.T, pass, fail, bench winssh.Result) {
	t.Helper()
	p := gotestNorm(pass.Stdout)
	if pass.Code != 0 {
		t.Errorf("the passing run exited with %d:\n%s\n%s", pass.Code, pass.Stdout, pass.Stderr)
	}
	inOrder(t, "the passing run", p,
		`=== RUN   TestPass`, `--- PASS: TestPass \(T\)`,
		`=== RUN   TestSkip`, `    lib_test\.gp:\d+: skipping on purpose`, `--- SKIP: TestSkip \(T\)`,
		`=== RUN   TestTempDir`, `--- PASS: TestTempDir \(T\)`,
		`=== RUN   TestTempDirRemoved`, `--- PASS: TestTempDirRemoved \(T\)`,
		`=== RUN   TestSetenv`, `=== RUN   TestSetenv/sub`, `--- PASS: TestSetenv \(T\)`, `    --- PASS: TestSetenv/sub \(T\)`,
		`=== RUN   TestSetenvRestored`, `--- PASS: TestSetenvRestored \(T\)`,
		`=== RUN   TestCleanupOrder`, `cleanup 2`, `cleanup 1`, `--- PASS: TestCleanupOrder \(T\)`,
		`=== RUN   TestChdir`, `--- PASS: TestChdir \(T\)`,
		`=== RUN   TestChdirRestored`, `--- PASS: TestChdirRestored \(T\)`,
		`=== RUN   TestAllocs`, `--- PASS: TestAllocs \(T\)`,
		`=== RUN   ExampleGreet`, `--- PASS: ExampleGreet \(T\)`,
		`=== RUN   ExampleCRLF`, `--- PASS: ExampleCRLF \(T\)`,
		`PASS`, `ok  \texample\.com/wt\tT`)
	if strings.Contains(p, "FAIL") {
		t.Errorf("the passing run says FAIL:\n%s", p)
	}

	f := gotestNorm(fail.Stdout)
	if fail.Code != 1 {
		t.Errorf("the run with a failing test exited with %d, want 1:\n%s\n%s", fail.Code, fail.Stdout, fail.Stderr)
	}
	inOrder(t, "the failing run", f,
		`=== RUN   TestPass`, `--- PASS: TestPass \(T\)`,
		`=== RUN   TestFailing`, `    lib_test\.gp:\d+: expected failure 4`, `--- FAIL: TestFailing \(T\)`,
		`=== RUN   TestSkip`, `--- SKIP: TestSkip \(T\)`,
		`=== RUN   ExampleGreet`, `--- PASS: ExampleGreet \(T\)`,
		`FAIL`, `FAIL\texample\.com/wt\tT`)

	b := gotestNorm(bench.Stdout)
	if bench.Code != 0 {
		t.Errorf("the benchmark run exited with %d:\n%s\n%s", bench.Code, bench.Stdout, bench.Stderr)
	}
	// (ns/op is left out when it is 0, as in Go: 100 runs of Add are below the clock's step.)
	inOrder(t, "the benchmark run", b,
		`goos: windows`, `goarch: amd64`, `pkg: example\.com/wt`, `cpu: \S.*\S`,
		`BenchmarkAlloc-\d+\s+100(?:\s+[0-9.]+ ns/op)?\s+[1-9]\d{3,} B/op\s+[1-9]\d* allocs/op`,
		`BenchmarkAdd-\d+\s+100(?:\s+[0-9.]+ ns/op)?\s+\d+ B/op\s+\d+ allocs/op`, `PASS`, `ok  \texample\.com/wt\tT`)
}

// noTestTempDirs fails if the TEMP folder of the run (a Windows path below the run folder) holds a
// folder that t.TempDir made and the cleanup did not remove.
func noTestTempDirs(t *testing.T, listing string) {
	t.Helper()
	for _, name := range strings.Fields(listing) {
		if strings.HasPrefix(name, "Test") {
			t.Errorf("TEMP holds %q after the run: t.TempDir's cleanup left it\n%s", name, listing)
		}
	}
}

func TestWindowsNativeGoTest(t *testing.T) {
	n := nativeRemote(t)
	n.put(t, "wt", gotestModule)
	env := []string{"GOCPP_WT_PRE=orig"}
	pass := n.gocpp(t, "wt", env, "test", "-v", "-skip", "TestFailing")
	fail := n.gocpp(t, "wt", env, "test", "-v")
	bench := n.gocpp(t, "wt", env, "test", "-run", "NoSuchTest", "-bench", ".", "-benchtime", "100x", "-benchmem")
	checkGotestRuns(t, pass, fail, bench)
	noTestTempDirs(t, normNL(n.cmd(t, "", nil, `dir /b tmp`).Stdout))
}

func TestWindowsCrossGoTest(t *testing.T) {
	host := windowsHost()
	if host == "" {
		t.Skip("GOCPP_WINDOWS_SSH=user@host turns it on: it runs a test binary on a real Windows PC")
	}
	if why := windowsUnavailable(); why != "" {
		t.Skip(why)
	}
	// gocpp for this machine, and the module on disk.
	tmp := t.TempDir()
	gocpp := filepath.Join(tmp, "gocpp")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", gocpp, "./cmd/gocpp")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/gocpp: %v\n%s", err, out)
	}
	mod := filepath.Join(tmp, "wt")
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range gotestModule {
		if err := os.WriteFile(filepath.Join(mod, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	exe := filepath.Join(tmp, "wt.test.exe")
	cmd := exec.CommandContext(t.Context(), gocpp, "test", "-c", "-os", "windows", "-o", exe)
	cmd.Dir = mod
	cmd.Env = append(os.Environ(), "XDG_CACHE_HOME="+filepath.Join(tmp, "cache"), "GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1", "GOCPP_GATE_PHASE=15")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gocpp test -c -os windows: %v\n%s", err, out)
	}

	r, err := winssh.New(host)
	if err != nil {
		t.Fatalf("the Windows PC is not reachable: %v", err)
	}
	t.Cleanup(r.Close)
	home, err := r.Home(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	runDir := home + `\` + r.Dir()
	line := func(l string) {
		t.Helper()
		res, err := r.Exec(t.Context(), 2*time.Minute, nil, l)
		if err != nil || res.Code != 0 {
			t.Fatalf("%s: %v, exit %d\n%s%s", l, err, res.Code, res.Stdout, res.Stderr)
		}
	}
	line(`mkdir tmp`) // (Exec starts in the run's folder)
	// TEMP and TMP inside the run's folder: the TempDir folders are there, and nothing else is touched.
	env := []string{"TEMP=" + runDir + `\tmp`, "TMP=" + runDir + `\tmp`, "GOCPP_WT_PRE=orig"}
	run := func(args ...string) winssh.Result {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 6*time.Minute)
		defer cancel()
		res, err := r.Run(ctx, 5*time.Minute, exe, env, args, false)
		if err != nil {
			t.Fatal(err)
		}
		if res.TimedOut {
			t.Fatalf("%v: it timed out\n%s\n%s", args, res.Stdout, res.Stderr)
		}
		return res
	}
	pass := run("-test.v", "-test.skip=TestFailing")
	fail := run("-test.v")
	bench := run("-test.run=NoSuchTest", "-test.bench=.", "-test.benchtime=100x", "-test.benchmem")
	// The test binary prints the same lines as `gocpp test` (the "ok" line is gocpp's): its own
	// PASS/FAIL lines are checked, and a "ok" line is added for the comparison.
	addOK := func(res winssh.Result) winssh.Result {
		if res.Code == 0 {
			res.Stdout = append(res.Stdout, []byte("ok  \texample.com/wt\t0.001s\n")...)
		} else {
			res.Stdout = append(res.Stdout, []byte("FAIL\texample.com/wt\t0.001s\n")...)
		}
		return res
	}
	checkGotestRuns(t, addOK(pass), addOK(fail), addOK(bench))
	res, err := r.Exec(t.Context(), time.Minute, nil, `dir /b tmp`)
	if err != nil {
		t.Fatal(err)
	}
	noTestTempDirs(t, normNL(res.Stdout))
}
