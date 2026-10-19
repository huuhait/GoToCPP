// Package gotest runs gocpp test on the fixture modules in testdata/ and compares what it prints
// with what real go test prints for the same sources (the .gp files renamed to .go): the text on
// standard output after the normalization below, and the exit status. See README.md.
package gotest

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	repoRoot   string
	gocppBin   string // the gocpp program, built once
	cacheDir   string // the object cache of every gocpp run (XDG_CACHE_HOME)
	goPath     string // the go command, or ""
	jobs       chan struct{}
	goBuildErr error
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	flag.Parse()
	_, file, _, _ := runtime.Caller(0)
	repoRoot = filepath.Join(filepath.Dir(file), "..", "..")
	tmp, err := os.MkdirTemp("", "gocpp-gotest")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(tmp)
	// The tests never use the real caches or the network (downloads exist).
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	os.Setenv("GOCPP_WORKLOST", "fail") // (docs/spec/build.md: a lost work folder is an error here)
	cacheDir = filepath.Join(tmp, "cache")
	if testing.Short() {
		return m.Run()
	}
	gocppBin = filepath.Join(tmp, "gocpp"+exeSuffix())
	build := exec.Command("go", "build", "-o", gocppBin, "./cmd/gocpp")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		goBuildErr = fmt.Errorf("go build ./cmd/gocpp: %v\n%s", err, out)
	}
	goPath, _ = exec.LookPath("go")
	n := 2
	if v, err := strconv.Atoi(os.Getenv("GOCPP_TEST_JOBS")); err == nil && v > 0 {
		n = v
	}
	jobs = make(chan struct{}, n)
	return m.Run()
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// A fixture is a folder of testdata: a module (gocpp.mod and .gp files), the file args with the
// arguments of gocpp test (one on each line) and the optional file meta.
type fixture struct {
	name string
	dir  string
	args []string
	meta meta
}

// meta is the file meta of a fixture, one directive on each line:
//
//	cut-panic     the goroutine traces after a panic are not compared (their frames differ)
//	bench         the numbers of benchmark lines are not compared
//	stdout-only   the build errors on standard error are not compared (the compilers' words differ)
//	timeout       the run is expected to time out: the traces are not compared
//	multi         several packages: the order of their lines is the order of the packages
//	diverge: why  the output differs from go test's for a reason: it is not compared
//	want: text    a line that gocpp test must print (for fixtures that diverge)
//	phase: NN     the gate phase of gocpp test (default 09), for a fixture that uses Phase 10 syntax
//
// A fixture that uses Phase 10 syntax (!, macros) has a folder twin/: a Go module (go.mod and .go files) with the same
// package written as plain Go. go test builds the twin in place of the renamed .gp files.
type meta struct {
	phase                       string
	cutPanic, bench, stdoutOnly bool
	diverge                     string
	wants                       []string
}

func loadFixtures(t *testing.T) []fixture {
	t.Helper()
	root := filepath.Join(repoRoot, "tests", "gotest", "testdata")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []fixture
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		f := fixture{name: e.Name(), dir: filepath.Join(root, e.Name())}
		args, err := os.ReadFile(filepath.Join(f.dir, "args"))
		if err != nil {
			t.Fatalf("%s: %v", f.name, err)
		}
		f.args = strings.Fields(strings.ReplaceAll(string(args), "\n", " "))
		if b, err := os.ReadFile(filepath.Join(f.dir, "meta")); err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				line = strings.TrimSpace(line)
				switch {
				case line == "":
				case line == "cut-panic":
					f.meta.cutPanic = true
				case line == "bench":
					f.meta.bench = true
				case line == "stdout-only":
					f.meta.stdoutOnly = true
				case line == "timeout", line == "multi":
				case strings.HasPrefix(line, "diverge:"):
					f.meta.diverge = strings.TrimSpace(strings.TrimPrefix(line, "diverge:"))
				case strings.HasPrefix(line, "phase:"):
					f.meta.phase = strings.TrimSpace(strings.TrimPrefix(line, "phase:"))
				case strings.HasPrefix(line, "want:"):
					f.meta.wants = append(f.meta.wants, strings.TrimSpace(strings.TrimPrefix(line, "want:")))
				default:
					t.Fatalf("%s: bad line in meta: %q", f.name, line)
				}
			}
		}
		fixtures = append(fixtures, f)
	}
	return fixtures
}

// copyFixture copies the module of f to dst. asGo renames the .gp files to .go and makes a go.mod
// of gocpp.mod.
func copyFixture(f fixture, dst string, asGo bool) error {
	return filepath.WalkDir(f.dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(f.dir, p)
		if rel == "twin" && e.IsDir() {
			return fs.SkipDir // the twin is a module of its own, for go test only
		}
		if e.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if rel == "args" || rel == "meta" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if asGo {
			switch {
			case filepath.Base(rel) == "gocpp.mod":
				rel = filepath.Join(filepath.Dir(rel), "go.mod")
			case strings.HasSuffix(rel, ".gp"):
				rel = strings.TrimSuffix(rel, ".gp") + ".go"
			}
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
}

// result is what one command printed.
type result struct {
	stdout, stderr string
	exit           int
}

// runCmd runs name with args in dir, in a process group that dies with the test.
func runCmd(t *testing.T, dir string, env []string, name string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	setGroup(cmd)
	cmd.Cancel = func() error { return killGroup(cmd) }
	cmd.WaitDelay = 5 * time.Second
	err := cmd.Run()
	r := result{stdout: stdout.String(), stderr: stderr.String()}
	if ee, ok := err.(*exec.ExitError); ok {
		r.exit = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%s %s: %v", name, strings.Join(args, " "), err)
	}
	return r
}

var (
	reDuration  = regexp.MustCompile(`\((\d+\.\d\d)s\)`)
	reTotal     = regexp.MustCompile(`\t\d+\.\d{3}s`)
	reGoFile    = regexp.MustCompile(`(\w)\.go:(\d+)`)
	reGoroutine = regexp.MustCompile(`(?s)\ngoroutine \d+ \[.*?\n((?:exit status \d+\n)?(?:FAIL|ok)\t)`)
	reBenchName = regexp.MustCompile(`(?m)^(Benchmark\S*?)-\d+`)
	reBenchNum  = regexp.MustCompile(`\d+(\.\d+)?( ns/op| B/op| allocs/op| MB/s)`)
	reBenchN    = regexp.MustCompile(`(?m)^(Benchmark\S*)\s+\d+\t`)
	reSpaces    = regexp.MustCompile(`[ \t]+`)
	reTmp       = regexp.MustCompile(`/tmp/\S+`)
	reCPU       = regexp.MustCompile(`(?m)^cpu: .*$`)
)

// normalize makes the text of go test and of gocpp test comparable: the durations, the file
// extension of the positions, the goroutine traces after a panic (cutPanic), the numbers of the
// benchmark lines (bench).
func normalize(s string, f fixture, goSide bool) string {
	s = reDuration.ReplaceAllString(s, "(0.00s)")
	s = reTotal.ReplaceAllString(s, "\t0.000s")
	if goSide {
		s = reGoFile.ReplaceAllString(s, "$1.gp:$2")
	}
	s = reTmp.ReplaceAllString(s, "/TMP")
	if f.meta.cutPanic {
		s = reGoroutine.ReplaceAllString(s, "\n$1")
	}
	if f.meta.bench {
		s = reCPU.ReplaceAllString(s, "cpu: X") // go test's name has the padding of cpuid
		s = reBenchName.ReplaceAllString(s, "$1-N")
		s = reBenchN.ReplaceAllString(s, "$1 N\t")
		s = reBenchNum.ReplaceAllString(s, "N$2")
		var lines []string
		for _, l := range strings.Split(s, "\n") {
			if strings.HasPrefix(l, "Benchmark") {
				l = reSpaces.ReplaceAllString(l, " ")
			}
			lines = append(lines, l)
		}
		s = strings.Join(lines, "\n")
	}
	return s
}

func acquire() func() {
	jobs <- struct{}{}
	return func() { <-jobs }
}

func TestGoTest(t *testing.T) {
	if testing.Short() {
		t.Skip("builds gocpp and the test binaries: skipped with -short")
	}
	if goBuildErr != nil {
		t.Fatal(goBuildErr)
	}
	fixtures := loadFixtures(t)
	if len(fixtures) < 30 {
		t.Fatalf("only %d fixtures", len(fixtures))
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			t.Parallel()
			ours := runGocpp(t, f)
			if len(f.meta.wants) > 0 || f.meta.diverge != "" {
				for _, w := range f.meta.wants {
					if !strings.Contains(ours.stdout, w) {
						t.Errorf("gocpp test lacks %q:\n%s", w, ours.stdout)
					}
				}
				if f.meta.diverge != "" {
					return
				}
			}
			if goPath == "" {
				t.Skip("go is not in the PATH")
			}
			theirs := runGo(t, f)
			a, b := normalize(ours.stdout, f, false), normalize(theirs.stdout, f, true)
			if a != b {
				t.Errorf("standard output differs from go test's:\n--- gocpp test\n%s\n--- go test\n%s", ours.stdout, theirs.stdout)
			}
			if ours.exit != theirs.exit {
				t.Errorf("gocpp test exited with %d, go test with %d\nstderr:\n%s", ours.exit, theirs.exit, ours.stderr)
			}
			if !f.meta.stdoutOnly {
				if a, b := normalize(ours.stderr, f, false), normalize(theirs.stderr, f, true); a != b {
					t.Errorf("standard error differs:\n--- gocpp test\n%s\n--- go test\n%s", ours.stderr, theirs.stderr)
				}
			}
		})
	}
}

func runGocpp(t *testing.T, f fixture) result {
	t.Helper()
	dir := t.TempDir()
	if err := copyFixture(f, dir, false); err != nil {
		t.Fatal(err)
	}
	defer acquire()()
	args := append([]string{"test"}, f.args...)
	phase := "09"
	if f.meta.phase != "" {
		phase = f.meta.phase
	}
	env := []string{"XDG_CACHE_HOME=" + cacheDir, "GOCPP_GATE_PHASE=" + phase, "GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1", "GOCPP_WORKLOST=fail"}
	return runCmd(t, dir, env, gocppBin, args...)
}

func runGo(t *testing.T, f fixture) result {
	t.Helper()
	dir := t.TempDir()
	if twin := filepath.Join(f.dir, "twin"); isDir(twin) {
		// Phase 10 syntax: the same package written as plain Go.
		if err := copyFixture(fixture{dir: twin}, dir, false); err != nil {
			t.Fatal(err)
		}
	} else if err := copyFixture(f, dir, true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer acquire()()
	args := append([]string{"test"}, f.args...)
	if !hasCount(f.args) {
		args = append(args, "-count=1") // no cached results
	}
	env := []string{"GOTOOLCHAIN=local", "GOFLAGS=", "GOWORK=off", "GO111MODULE=on", "GOPROXY=off"}
	return runCmd(t, dir, env, goPath, args...)
}

func hasCount(args []string) bool {
	for _, a := range args {
		if a == "-count" || strings.HasPrefix(a, "-count=") {
			return true
		}
	}
	return false
}

// TestRegexpDialect checks the patterns of -test.list (the matcher of -test.run, -skip and -bench) against
// the test binary of go test: the same names are listed, and the patterns that Go rejects are
// rejected with Go's words. The matcher is package regexp (testing/internal/testdeps, as in Go).
func TestRegexpDialect(t *testing.T) {
	if testing.Short() {
		t.Skip("builds gocpp and the test binaries: skipped with -short")
	}
	if goBuildErr != nil {
		t.Fatal(goBuildErr)
	}
	if goPath == "" {
		t.Skip("go is not in the PATH")
	}
	f := fixture{name: "regexp_names", dir: filepath.Join(repoRoot, "tests", "gotest", "regexp_names")}
	ours, theirs := t.TempDir(), t.TempDir()
	if err := copyFixture(f, ours, false); err != nil {
		t.Fatal(err)
	}
	if err := copyFixture(f, theirs, true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(theirs, "go.mod"), []byte("module example.com/m\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ourBin, goBin := filepath.Join(ours, "m.test"), filepath.Join(theirs, "m.test")
	func() {
		defer acquire()()
		env := []string{"XDG_CACHE_HOME=" + cacheDir, "GOCPP_GATE_PHASE=09", "GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1", "GOCPP_WORKLOST=fail"}
		if r := runCmd(t, ours, env, gocppBin, "test", "-c", "-o", ourBin); r.exit != 0 {
			t.Fatalf("gocpp test -c: %d\n%s%s", r.exit, r.stdout, r.stderr)
		}
		genv := []string{"GOTOOLCHAIN=local", "GOFLAGS=", "GOWORK=off", "GO111MODULE=on", "GOPROXY=off"}
		if r := runCmd(t, theirs, genv, goPath, "test", "-c", "-o", goBin); r.exit != 0 {
			t.Fatalf("go test -c: %d\n%s%s", r.exit, r.stdout, r.stderr)
		}
	}()

	list := func(bin, dir, pat string) result {
		return runCmd(t, dir, nil, bin, "-test.list="+pat)
	}
	// Patterns that both accept: the same names are listed.
	same := []string{
		`Test`, `^TestA$`, `TestA|TestAB`, `TestA|`, `^$`, `TestA+B`, `TestA*B`, `TestAB?`, `TestA{2}`, `TestAB{1,2}$`, `TestAB{2,}$`,
		`Test.ber`, `Test..ber`, `^Test.$`, `^Test..$`, `Test[^A]`, `Test[^A-Z]`, `Test[A-B]B`, `Test[[:upper:]]B`, `Test[\d]`, `Test\d`, `Test\w\w`,
		`(?i)testa`, `(?i)^testab$`, `(?s)Test.ber`, `(?m)^TestA$`, `(?is)TEST.ber`,
		`(?P<x>A)B`, `(?<x>A)B`, `Test(?:A|X)`, `Test(A|X)(B|y)`, `(?P<a>Test)(?P<b>A)$`,
		`\QTest_x\E`, `\QTest.\E`, `Test\QA\E`, `TestA\z`, `\ATest`, `\ATestA\z`, `Test\x41`, `Test\x{41}B`, `Test\101B`, `Test\x{DC}ber`,
		`TestÜ`, `TestÜber`, `Test[Ü]ber`, `Test(Ü)ber`, `TestÜ+ber`, `^TestÀB$`, `Test\_x`, `Test\-`, `Test_x$`,
		`Test[a-zA-Z]+$`, `Test[^_]`, `Test[\w]B`, `^(Test)(A|X)`, `a{,3}`, `^*TestA`, `$*`, `^+TestA`, `\\b*TestA`, `\\b+Test`, `\\A*Test`, `(^)*Test`, `(?)Test`, `Test[[:^digit:]]`, `Test[[:^alpha:]]`, `Test[^[:^alpha:]]`, `Test[\\D]B`, `Test[\\Wa]`, `Test\\SB`, `Test\\W`, `Test[[:^word:]]`, `Test[[:^space:]]B`, `Test{`, `Test}`, `Test]`, `[]a]`,
		`TestA/sub`, `TestA/`, `/`,
		// Unicode classes, flags inside a pattern and non-ASCII ranges: package regexp has them (the std::regex translation of steps 9.9 had not).
		`Test\pL`, `Test\p{Lu}ber`, `TestA(?i)b`, `(?i:testa)`, `(?U)TestA+`, `Test[Ü-Ý]`, `(?i)über`, `(?i)É`, `Test\PL`, `Test[\p{Lu}]B`, `Test[^\pL]`, `(?i)TESTÜBER`, `(?i:TestA)B`, `(?-i)TestA`,
		`Test\p{Greek}`, `\pN`, `Test[\pL\d]`, `Test\p{^Lu}`, `Test\P{^Lu}B`, `(?i)\x{DC}BER`, `Test[ü-ÿ]ber`,
	}
	for _, pat := range same {
		a, b := list(ourBin, ours, pat), list(goBin, theirs, pat)
		if a.stdout != b.stdout || a.stderr != b.stderr || a.exit != b.exit {
			t.Errorf("-test.list=%q\ngocpp: exit %d\n%s%s\ngo:    exit %d\n%s%s", pat, a.exit, a.stdout, a.stderr, b.exit, b.stdout, b.stderr)
		}
	}
	// Patterns that Go rejects: gocpp rejects them with the same message.
	rejected := []string{
		`(`, `)`, `A)`, `(A`, `[a`, `*`, `?`, `+`, `A**`, `A++`, `A?+`, `A+*`, `x{2}{3}`, `A|*`, `(*)`, `{2}`, "A\\",
		`Test(?=A)`, `Test(?!A)`, `Test(?<=A)`, `Test(?<!A)`, `^Test(A)\1`, `\1`, `\8`, `\q`, `\C`, `\Z`, `(?P=n)`, `(?P<n>`, `(?P<>A)`, `(?<n`,
		`(?`, `(?i`, `(?x)A`, `(?i-)`, `(?-)`, `Test[a-\\d]`, `[[:^foo:]]`, `[z-a]`, `[[:foo:]]`, `A{2,1}`, `A{1001}`, `\x{110000}`, `\xZZ`,
	}
	for _, pat := range rejected {
		a, b := list(ourBin, ours, pat), list(goBin, theirs, pat)
		if b.exit == 0 {
			t.Errorf("-test.list=%q: go accepts it; move it to the patterns that both accept", pat)
		}
		if a.stdout != b.stdout || a.stderr != b.stderr || a.exit != b.exit {
			t.Errorf("-test.list=%q\ngocpp: exit %d\n%s%s\ngo:    exit %d\n%s%s", pat, a.exit, a.stdout, a.stderr, b.exit, b.stdout, b.stderr)
		}
	}
}

// gocppTest runs gocpp test with args in dir.
func gocppTest(t *testing.T, dir string, args ...string) result {
	t.Helper()
	defer acquire()()
	env := []string{"XDG_CACHE_HOME=" + cacheDir, "GOCPP_GATE_PHASE=09", "GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1", "GOCPP_WORKLOST=fail"}
	return runCmd(t, dir, env, gocppBin, append([]string{"test"}, args...)...)
}

// TestRerunAfterEdit checks that a second gocpp test sees what changed since the first: a test file,
// then the flags. (The up-to-date check of gocpp build, step 6.5, does not apply to test binaries:
// gocpp test goes through the object cache, which keys the objects by the text of the C++.)
func TestRerunAfterEdit(t *testing.T) {
	if testing.Short() {
		t.Skip("builds gocpp and the test binaries: skipped with -short")
	}
	if goBuildErr != nil {
		t.Fatal(goBuildErr)
	}
	dir := t.TempDir()
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("gocpp.mod", "module example.com/m\n\ngo 1.25\n")
	write("lib.gp", "package lib\n\nfunc Add(a, b int) int { return a + b }\n")
	testFile := func(want int) string {
		return fmt.Sprintf("package lib\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 2) != %d {\n\t\tt.Error(\"bad\")\n\t}\n}\n", want)
	}
	write("lib_test.gp", testFile(4))
	if r := gocppTest(t, dir); r.exit != 0 || !strings.HasPrefix(r.stdout, "PASS\nok  \texample.com/m") {
		t.Fatalf("first run: exit %d\n%s%s", r.exit, r.stdout, r.stderr)
	}
	// The test file changes: the second run fails.
	write("lib_test.gp", testFile(5))
	if r := gocppTest(t, dir); r.exit != 1 || !strings.Contains(r.stdout, "--- FAIL: TestAdd") || !strings.Contains(r.stdout, "lib_test.gp:7: bad") {
		t.Fatalf("after editing the test: exit %d\n%s%s", r.exit, r.stdout, r.stderr)
	}
	// The package changes: the test passes again.
	write("lib.gp", "package lib\n\nfunc Add(a, b int) int { return a + b + 1 }\n")
	if r := gocppTest(t, dir); r.exit != 0 {
		t.Fatalf("after editing the package: exit %d\n%s%s", r.exit, r.stdout, r.stderr)
	}
	// The same binary, other flags.
	if r := gocppTest(t, dir, "-v"); r.exit != 0 || !strings.Contains(r.stdout, "=== RUN   TestAdd") {
		t.Fatalf("with -v: exit %d\n%s%s", r.exit, r.stdout, r.stderr)
	}
	if r := gocppTest(t, dir, "-run", "NoSuchTest"); r.exit != 0 || !strings.Contains(r.stdout, "testing: warning: no tests to run") {
		t.Fatalf("with -run: exit %d\n%s%s", r.exit, r.stdout, r.stderr)
	}
	// A file that makes the test not build is reported, and fixed again.
	write("lib_test.gp", "package lib\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tvar x int = \"s\"\n}\n")
	if r := gocppTest(t, dir); r.exit != 1 || !strings.Contains(r.stdout, "[build failed]") {
		t.Fatalf("with a type error: exit %d\n%s%s", r.exit, r.stdout, r.stderr)
	}
	write("lib_test.gp", testFile(5))
	if r := gocppTest(t, dir); r.exit != 0 {
		t.Fatalf("after the fix: exit %d\n%s%s", r.exit, r.stdout, r.stderr)
	}
}

// TestOutputFlag checks -c and -o: -c writes the test binary and runs nothing, -o writes it and the test
// runs as well, and the binary can be run by hand.
func TestOutputFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("builds gocpp and the test binaries: skipped with -short")
	}
	if goBuildErr != nil {
		t.Fatal(goBuildErr)
	}
	dir := t.TempDir()
	for name, text := range map[string]string{
		"gocpp.mod":   "module example.com/m\n\ngo 1.25\n",
		"lib.gp":      "package lib\n",
		"lib_test.gp": "package lib\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { t.Log(\"ran\") }\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// -c: the file is made, nothing runs.
	if r := gocppTest(t, dir, "-c", "-o", "c.test", "-v"); r.exit != 0 || r.stdout != "" {
		t.Fatalf("-c: exit %d\n%s%s", r.exit, r.stdout, r.stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "c.test"+exeSuffix())); err != nil {
		t.Fatalf("-c -o c.test: %v", err)
	}
	// -o without -c: the file is made and the test runs.
	r := gocppTest(t, dir, "-o", "o.test", "-v")
	if r.exit != 0 || !strings.Contains(r.stdout, "--- PASS: TestA") {
		t.Fatalf("-o: exit %d\n%s%s", r.exit, r.stdout, r.stderr)
	}
	// The binary runs by itself (in the folder of the package, as go test runs it).
	r = runCmd(t, dir, nil, filepath.Join(dir, "o.test"+exeSuffix()), "-test.v")
	if r.exit != 0 || !strings.Contains(r.stdout, "lib_test.gp:5: ran") {
		t.Fatalf("o.test: exit %d\n%s%s", r.exit, r.stdout, r.stderr)
	}
	// -o with a folder: pkg.test in it.
	if err := os.Mkdir(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if r := gocppTest(t, dir, "-c", "-o", "bin/"); r.exit != 0 {
		t.Fatalf("-o bin/: exit %d\n%s%s", r.exit, r.stdout, r.stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "bin", "m.test"+exeSuffix())); err != nil {
		t.Fatalf("-c -o bin/: %v", err)
	}
}

// TestParallelOrder checks the order in which parallel subtests are released: with -parallel 1 they wait for
// the one slot, and they get it in the order they paused (Go's scheduler does something of its own;
// gocpp's goroutines are threads, which would take them in any order). The fixture has 40 subtests, so
// that an order by chance is not the order of the tests.
func TestParallelOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("builds gocpp and the test binaries: skipped with -short")
	}
	if goBuildErr != nil {
		t.Fatal(goBuildErr)
	}
	dir := t.TempDir()
	var src strings.Builder
	// Each test takes longer than the moment that the parent waits for a test it starts, so that the
	// ones behind the first wait in the queue of testState.
	src.WriteString("package lib\n\nimport (\n\t\"testing\"\n\t\"time\"\n)\n\nfunc TestOrder(t *testing.T) {\n\tfor i := 0; i < 40; i++ {\n\t\tt.Run(\"p\", func(t *testing.T) {\n\t\t\tt.Parallel()\n\t\t\ttime.Sleep(2 * time.Millisecond)\n\t\t\tt.Log(\"ran\")\n\t\t})\n\t}\n}\n")
	for name, text := range map[string]string{
		"gocpp.mod":   "module example.com/m\n\ngo 1.25\n",
		"lib.gp":      "package lib\n",
		"lib_test.gp": src.String(),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := gocppTest(t, dir, "-v", "-parallel", "1")
	if r.exit != 0 {
		t.Fatalf("exit %d\n%s%s", r.exit, r.stdout, r.stderr)
	}
	var cont, ran []string
	for _, line := range strings.Split(r.stdout, "\n") {
		if name, ok := strings.CutPrefix(line, "=== CONT  "); ok {
			cont = append(cont, name)
		}
		if strings.Contains(line, ": ran") {
			ran = append(ran, line)
		}
	}
	if len(cont) != 40 {
		t.Fatalf("%d lines === CONT, want 40:\n%s", len(cont), r.stdout)
	}
	for i, name := range cont {
		want := "TestOrder/p"
		if i > 0 {
			want = fmt.Sprintf("TestOrder/p#%02d", i)
		}
		if name != want {
			t.Fatalf("=== CONT line %d is %s, want %s:\n%s", i, name, want, r.stdout)
		}
	}
	if len(ran) != 40 {
		t.Errorf("%d tests ran, want 40", len(ran))
	}
}

// TestAsanLeakFree runs parallel subtests, a fuzz target and cleanups in a test binary built with -asan and
// LeakSanitizer turned on: memory is reference counted, so a cycle in the testing package (a test and
// its writer, a test and its parallel subtests) would be reported as a leak at exit. (Without
// ASAN_OPTIONS, gocpp test turns the check off for the binary, as the fixture asan_parallel runs.)
func TestAsanLeakFree(t *testing.T) {
	if testing.Short() {
		t.Skip("builds gocpp and the test binaries: skipped with -short")
	}
	if goBuildErr != nil {
		t.Fatal(goBuildErr)
	}
	dir := t.TempDir()
	for name, text := range map[string]string{
		"gocpp.mod": "module example.com/m\n\ngo 1.25\n",
		"lib.gp":    "package lib\n",
		"lib_test.gp": `package lib

import "testing"

func TestPar(t *testing.T) {
	t.Cleanup(func() { t.Log("cleanup") })
	for i := 0; i < 3; i++ {
		t.Run("p", func(t *testing.T) {
			t.Parallel()
			t.Run("inner", func(t *testing.T) { t.Parallel() })
		})
	}
}

func FuzzF(f *testing.F) {
	f.Add(1)
	f.Fuzz(func(t *testing.T, n int) { t.Parallel() })
}
`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	defer acquire()()
	env := []string{"XDG_CACHE_HOME=" + cacheDir, "GOCPP_GATE_PHASE=09", "GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1", "GOCPP_WORKLOST=fail", "ASAN_OPTIONS=detect_leaks=1"}
	r := runCmd(t, dir, env, gocppBin, "test", "-asan", "-v")
	if r.exit != 0 || strings.Contains(r.stdout+r.stderr, "Sanitizer") {
		t.Fatalf("exit %d\n%s%s", r.exit, r.stdout, r.stderr)
	}
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
