package tests

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/version"
)

var update = flag.Bool("update", false, "rewrite the snapshots in tests/golden (*.cpp.golden) with what gocpp emit prints")

const (
	// defaultTimeout bounds each command a test runs; GOCPP_TEST_TIMEOUT
	// changes it. It is long because a test compiles C++.
	defaultTimeout = 2 * time.Minute

	// maxOutput is how much of the stdout and of the stderr of a command the
	// harness keeps. A command that prints more is stopped, so that a test
	// program that prints forever can't fill the memory.
	maxOutput = 8 << 20

	// deadlineMargin is how long before go test's -timeout runs out a
	// command is stopped, so that its test can fail with a message and
	// TestMain can clean up before the test binary panics. It is longer than
	// waitDelay.
	deadlineMargin = 5 * time.Second

	// waitDelay is how long a stopped command has to exit before it is
	// killed, and how long its output pipes may stay open after it exits,
	// held by a process that it started.
	waitDelay = 2 * time.Second
)

// exe ends the file names of programs: ".exe" on Windows.
var exe = func() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}()

// What TestMain sets up for the tests.
var (
	repoRoot string // the absolute path of the repository root
	gocppBin string // the gocpp binary built from ./cmd/gocpp
)

// TestMain builds ./cmd/gocpp once into a temporary folder; the tests run
// that binary as a subprocess. It removes the folder at the end, and also
// when the test binary is stopped by a signal or is about to panic because
// go test's -timeout runs out.
func TestMain(m *testing.M) {
	// First, so that a bad flag (or -h) exits before there is anything to
	// clean up.
	flag.Parse()
	root, err := filepath.Abs("..") // go test runs the tests in the package folder, tests/
	if err == nil {
		_, err = os.Stat(filepath.Join(root, "go.mod"))
	}
	if err != nil {
		fatalf("tests: can't find the repository root: %v", err)
	}
	tmp, err := os.MkdirTemp("", "gocpp-tests-")
	if err != nil {
		fatalf("tests: %v", err)
	}
	bin := filepath.Join(tmp, "gocpp"+exe)
	build := exec.Command("go", "build", "-o", bin, "./cmd/gocpp")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		os.RemoveAll(tmp)
		fatalf("tests: go build ./cmd/gocpp: %v\n%s", err, out)
	}
	repoRoot, gocppBin = root, bin
	// The gocpp runs get a cache folder of their own for the runtime's
	// object files, so that the tests never write the user's
	// ~/.cache/gocpp (the runtime is compiled once per compiler and flags,
	// about a second). The Go of the diff tests finds its build cache
	// through XDG_CACHE_HOME too, so GOCACHE keeps it first.
	if out, err := exec.Command("go", "env", "GOCACHE").Output(); err == nil && len(bytes.TrimSpace(out)) > 0 {
		os.Setenv("GOCACHE", string(bytes.TrimSpace(out)))
	}
	os.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))
	killOnSignal(tmp)
	// run stops each command before go test's -timeout runs out. If a test
	// hangs somewhere else, the test binary panics at the timeout without
	// returning here: stop the commands and remove tmp just before.
	var alarm *time.Timer
	if d := testTimeout(); d > 0 {
		alarm = time.AfterFunc(d-min(time.Second, d/10), func() {
			stopRunning()
			os.RemoveAll(tmp)
		})
	}
	code := m.Run()
	if alarm != nil {
		alarm.Stop()
	}
	os.RemoveAll(tmp)
	os.Exit(code)
}

// testTimeout returns go test's -timeout (the -test.timeout flag of the
// test binary), or 0 for none.
func testTimeout() time.Duration {
	if f := flag.Lookup("test.timeout"); f != nil {
		if g, ok := f.Value.(flag.Getter); ok {
			d, _ := g.Get().(time.Duration)
			return d
		}
	}
	return 0
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// A compiler is one C++ compiler of the matrix.
type compiler struct {
	name    string // the subtest name: "g++", "clang++"
	cxx     string // the GOCPP_CXX value that selects it
	missing string // why it can't be used here, or ""
}

// A config is the harness setup that comes from the environment.
type config struct {
	compilers  []compiler    // GOCPP_CXX, or g++ and clang++
	flags      []string      // GOCPP_FLAGS: more flags for gocpp run
	timeout    time.Duration // GOCPP_TEST_TIMEOUT: the limit for one command
	roots      []string      // the paths of the repository root, for trimRoot
	phase      int           // GOCPP_TEST_PHASE, or version.Phase: tests of later phases are skipped
	phaseFrom  string        // where phase comes from, for skip messages: "now: Phase 00"
	requireGDB bool          // GOCPP_REQUIRE_GDB=1: the debug tests fail instead of skipping gdb
	runVsGo    bool          // GOCPP_RUN_VS_GO=1: TestRunVsGo checks the run tests' expectations with real Go
}

var (
	configOnce sync.Once
	theConfig  config
	configErr  error
)

// setup returns the harness setup. It reads the environment inside a test
// and not in TestMain: go test caches test results, and it only notices the
// environment variables and files that the tests read while they run.
func setup(t *testing.T) *config {
	t.Helper()
	configOnce.Do(func() { theConfig, configErr = loadConfig() })
	if configErr != nil {
		t.Fatal(configErr)
	}
	return &theConfig
}

func loadConfig() (config, error) {
	c := config{
		flags:   strings.Fields(os.Getenv("GOCPP_FLAGS")),
		timeout: defaultTimeout,
	}
	if s := os.Getenv("GOCPP_TEST_TIMEOUT"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil || d <= 0 {
			return c, fmt.Errorf("GOCPP_TEST_TIMEOUT=%s: want a duration like 5m", s)
		}
		c.timeout = d
	}
	var err error
	if c.phase, c.phaseFrom, err = phaseLimit(os.Getenv("GOCPP_TEST_PHASE")); err != nil {
		return c, err
	}
	switch s := os.Getenv("GOCPP_REQUIRE_GDB"); s {
	case "", "0":
	case "1":
		// Checked here like GOCPP_REQUIRE_CXX, so that a missing gdb fails
		// make ci also while the debug tests are skipped by their phase.
		if _, err := exec.LookPath("gdb"); err != nil {
			return c, fmt.Errorf("GOCPP_REQUIRE_GDB=1 needs gdb, but it is not installed: %v", err)
		}
		c.requireGDB = true
	default:
		return c, fmt.Errorf("GOCPP_REQUIRE_GDB=%s: want 1 (gdb must run) or 0", s)
	}
	switch s := os.Getenv("GOCPP_RUN_VS_GO"); s {
	case "", "0":
	case "1":
		c.runVsGo = true
	default:
		return c, fmt.Errorf("GOCPP_RUN_VS_GO=%s: want 1 (check the run tests with real Go) or 0", s)
	}
	roots := []string{repoRoot}
	if real, err := filepath.EvalSymlinks(repoRoot); err == nil && real != repoRoot {
		roots = append(roots, real)
	}
	for _, r := range roots {
		c.roots = append(c.roots, r)
		if s := filepath.ToSlash(r); s != r {
			c.roots = append(c.roots, s) // Windows: C:/... as well as C:\...
		}
	}
	names := []string{"g++", "clang++"}
	if cxx := os.Getenv("GOCPP_CXX"); cxx != "" {
		names = []string{cxx}
	}
	for _, cxx := range names {
		cc := compiler{name: filepath.Base(cxx), cxx: cxx}
		if _, err := exec.LookPath(cxx); err != nil {
			cc.missing = fmt.Sprintf("%s is not installed: %v", cxx, err)
		}
		c.compilers = append(c.compilers, cc)
	}
	trackSources()
	// Checked here, before any test runs: a test that is skipped (by its
	// phase, today all of them) never reaches its compilers.
	if err := checkRequired(c.compilers, os.Getenv("GOCPP_REQUIRE_CXX"), os.Getenv("GOCPP_CXX")); err != nil {
		return c, err
	}
	return c, nil
}

// gateEnv returns GOCPP_GATE_PHASE=NN for a test of phase NN that comes
// after version.Phase, else "". While phase NN is built, version.Phase is
// still an earlier phase, so gocpp's own phase gates would reject the
// features that the tests of phase NN use: GOCPP_GATE_PHASE opens them up to
// NN (cmd/gocpp/gatephase.go), and only for that test. A test of a finished
// phase keeps the gates of version.Phase, so an error test can still expect
// the gate of a later phase.
func gateEnv(phase int) string {
	if phase <= version.Phase {
		return ""
	}
	return fmt.Sprintf("GOCPP_GATE_PHASE=%02d", phase)
}

// phaseLimitRx matches a GOCPP_TEST_PHASE value: one or two digits.
var phaseLimitRx = regexp.MustCompile(`^\d{1,2}$`)

// phaseLimit reads GOCPP_TEST_PHASE (s): the phase to run the tests up to,
// 00 to 15, so that a coder building a phase can run its tests before
// version.Phase says it is finished. Without it, the limit is
// version.Phase. It also returns where the limit comes from, for the skip
// messages.
func phaseLimit(s string) (phase int, from string, err error) {
	if s == "" {
		return version.Phase, fmt.Sprintf("now: Phase %02d", version.Phase), nil
	}
	n, err := strconv.Atoi(s)
	if !phaseLimitRx.MatchString(s) || err != nil || n > lastPhase {
		return 0, "", fmt.Errorf("GOCPP_TEST_PHASE=%s: want a phase from 00 to %02d, for example GOCPP_TEST_PHASE=01", s, lastPhase)
	}
	return n, "GOCPP_TEST_PHASE=" + s, nil
}

// checkRequired checks the compilers of the matrix against
// GOCPP_REQUIRE_CXX (required, a comma list such as "g++,clang++"): each
// required compiler must be in the matrix and installed. cxx is the value
// of GOCPP_CXX, for messages. Outside CI a missing compiler is only a skip;
// "make ci" sets GOCPP_REQUIRE_CXX, so there it is a failure.
func checkRequired(compilers []compiler, required, cxx string) error {
	for _, want := range strings.Split(required, ",") {
		want = strings.TrimSpace(want)
		if want == "" {
			continue
		}
		i := slices.IndexFunc(compilers, func(cc compiler) bool { return cc.name == want || cc.cxx == want })
		switch {
		case i < 0 && cxx != "":
			return fmt.Errorf("GOCPP_REQUIRE_CXX=%s needs %s, but GOCPP_CXX=%s tests only that compiler", required, want, cxx)
		case i < 0:
			return fmt.Errorf("GOCPP_REQUIRE_CXX=%s needs %s, which the tests don't use: they use g++ and clang++, or the compiler in GOCPP_CXX",
				required, want)
		case compilers[i].missing != "":
			return fmt.Errorf("GOCPP_REQUIRE_CXX=%s needs %s, but %s", required, want, compilers[i].missing)
		}
	}
	return nil
}

// trackSources reads the folders that go into the gocpp binary. go test
// builds the tests package, but not the gocpp binary that the tests run,
// so without this a cached result would hide a change to gocpp. Reading a
// folder is enough: go test then checks the size and time of every file in
// it.
func trackSources() {
	for _, dir := range []string{"cmd", "internal", "runtime", "stdlib"} {
		trackFiles(filepath.Join(repoRoot, dir))
	}
	for _, file := range []string{"go.mod", "go.sum"} {
		os.Stat(filepath.Join(repoRoot, file))
	}
}

// trackFiles reads every folder under dir, for go test's result cache (see
// trackSources). A missing folder is fine.
func trackFiles(dir string) {
	filepath.WalkDir(dir, func(string, fs.DirEntry, error) error { return nil })
}

// TestRun runs the programs in tests/run with gocpp run: stdout must match
// the // Output: block at the end of the file. See tests/run/README.md.
func TestRun(t *testing.T) {
	c := setup(t)
	c.runPrograms(t, "tests/run", loadTests(t, runFormat), c.runTest)
}

// TestRunVsGo checks the expectations of the tests in tests/run (the
// // Output: block, // ExitCode: and the // Stderr: regexps) against real
// Go, so that an expected output pasted from Go can't drift. It is opt-in:
// GOCPP_RUN_VS_GO=1 turns it on. Real Go doesn't depend on gocpp's
// progress, so it checks the tests of every phase. A test whose code go
// build rejects (gocpp-only code, such as a header import) is skipped; any
// other failure of go build fails the test, and so does a run that compared
// no test at all. It runs no C++ compiler.
func TestRunVsGo(t *testing.T) {
	c := setup(t)
	if !c.runVsGo {
		t.Skip("GOCPP_RUN_VS_GO=1 turns it on")
	}
	var n vsGoCount
	// The cleanup runs after the parallel subtests.
	t.Cleanup(func() {
		compared, rejected, diverges := n.compared.Load(), n.rejected.Load(), n.diverges.Load()
		t.Logf("tests/run: compared %d with real Go, skipped %d (go build rejects them) and %d (// divergence:)",
			compared, rejected, diverges)
		if compared == 0 {
			t.Error("no test was compared with real Go")
		}
	})
	all := *c
	all.phase, all.phaseFrom = lastPhase, "every phase"
	all.runPrograms(t, "tests/run", loadTests(t, runFormat), func(t *testing.T, p *program) {
		c.runVsGoTest(t, p, &n)
	})
}

// vsGoCount counts the tests of TestRunVsGo: those compared with real Go,
// those skipped because go build rejects their code, and those skipped
// because they show a divergence (// divergence:).
type vsGoCount struct {
	compared, rejected, diverges atomic.Int64
}

// TestErrors checks the files in tests/errors with gocpp check: it must
// report the errors of the // ERROR comments and the warnings of the
// // WARNING comments, and no others. See
// tests/errors/README.md.
func TestErrors(t *testing.T) {
	c := setup(t)
	c.runPrograms(t, "tests/errors", loadTests(t, errorsFormat), c.errorsTest)
}

// TestGolden compares what gocpp emit prints for the files in tests/golden
// with their snapshots; -update rewrites them. See tests/golden/README.md.
func TestGolden(t *testing.T) {
	c := setup(t)
	orphans, err := orphanSnapshots(filepath.Join(repoRoot, "tests", "golden"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range orphans {
		t.Errorf("tests/golden/%s: a snapshot without its test (there is no %s.gp); delete it",
			name, strings.TrimSuffix(name, snapshotSuffix))
	}
	c.runPrograms(t, "tests/golden", loadTests(t, goldenFormat), c.goldenTest)
}

// TestDiff runs the programs in tests/diff with real Go and with gocpp run:
// both must print the same stdout and exit with the same code. See
// tests/diff/README.md.
func TestDiff(t *testing.T) {
	c := setup(t)
	c.runPrograms(t, "tests/diff", loadTests(t, diffFormat), c.diffTest)
}

// TestInterop runs the programs in tests/interop like TestRun; a test is
// skipped when a library of its // pkg-config: line is missing. A test with
// // ERROR comments is built instead, and the build must fail with those
// errors (interopErrorsTest). See tests/interop/README.md.
func TestInterop(t *testing.T) {
	c := setup(t)
	c.runPrograms(t, "tests/interop", loadTests(t, interopFormat), func(t *testing.T, p *program) {
		if p.wants != nil {
			c.interopErrorsTest(t, p)
			return
		}
		c.runTest(t, p)
	})
}

// TestExamples runs the target programs in examples/: each folder with a
// main.gp file is one program, and its stdout must equal output.txt. See
// examples/README.md.
func TestExamples(t *testing.T) {
	c := setup(t)
	dir := filepath.Join(repoRoot, "examples")
	names, err := listExamples(dir)
	if err != nil {
		t.Fatal(err)
	}
	progs := make([]*program, 0, len(names))
	for _, name := range names {
		rel := "examples/" + name
		exDir := filepath.Join(dir, name)
		// The program runs in its own folder, so that the file names in its
		// args.txt (testdata/poem.txt) work: gocpp run . <args>.
		p := &program{name: name, path: rel + "/main.gp", dir: exDir, arg: "."}
		if p.ex, p.err = loadExample(exDir, rel); p.err == nil {
			p.header = p.ex.header
		}
		trackFiles(exDir) // an example can have more files than the harness reads
		progs = append(progs, p)
	}
	c.runPrograms(t, "examples", progs, c.exampleTest)
}

// A program is one test that the harness found.
type program struct {
	name string // the subtest name: the file name without .gp, or the example's folder
	path string // its .gp file, relative to the repository root, for messages
	dir  string // the absolute folder that gocpp runs in
	arg  string // the program as gocpp gets it: "x.gp", or "." for an example
	err  error  // why the test is malformed, or nil
	skip string // why the test can't run here, or ""

	header *header
	wants  []wantError   // an error test's // ERROR comments
	ex     *example      // an example's output.txt, args.txt and stdin.txt
	debug  *debugProgram // a debug test's funcs and // gdb: comments
}

// loadTests reads the tests in tests/<kind>: each .gp file directly in the
// folder is one test.
func loadTests(t *testing.T, f format) []*program {
	t.Helper()
	dir := filepath.Join(repoRoot, "tests", f.kind)
	names, err := listPrograms(dir)
	if err != nil {
		t.Fatal(err)
	}
	progs := make([]*program, 0, len(names))
	for _, name := range names {
		p := &program{name: strings.TrimSuffix(name, ".gp"), path: "tests/" + f.kind + "/" + name, dir: dir, arg: name}
		src, err := os.ReadFile(filepath.Join(dir, name))
		pf := f
		if err == nil && f.kind == "interop" {
			// An interop test with // ERROR comments is an error test of
			// the build.
			if p.wants, err = parseErrors(p.path, src); err == nil && len(p.wants) > 0 {
				pf = interopErrorsFormat
				if !wantsErrors(p.wants) {
					err = fmt.Errorf("%s: an interop error test needs a // ERROR comment (// WARNING alone is not checked here)", p.path)
				}
			}
		}
		if err == nil {
			p.header, err = pf.parse(p.path, src)
		}
		if err == nil && f.kind == "errors" {
			p.wants, err = parseErrors(p.path, src)
		}
		if err == nil && f.kind == "debug" {
			p.debug, err = parseDebug(p.path, filepath.Join(dir, name), src)
		}
		p.err = err
		progs = append(progs, p)
	}
	return progs
}

// runPrograms runs fn for each program, as a parallel subtest, after the
// checks that every test gets: a malformed test fails, and a test that
// can't run here (skipReason) is skipped. It logs how many tests it found
// and skipped.
func (c *config) runPrograms(t *testing.T, where string, progs []*program, fn func(*testing.T, *program)) {
	skipped := make(map[string]int)
	var nskipped, bad int
	for _, p := range progs {
		if p.err != nil {
			bad++
			continue
		}
		var kind string
		if p.skip, kind = c.skipReason(p.header); p.skip != "" {
			skipped[kind]++
			nskipped++
		}
	}
	summary := fmt.Sprintf("%s: found %d, skipped %d", where, len(progs), nskipped)
	var kinds []string
	for _, kind := range skipKinds {
		if n := skipped[kind]; n > 0 {
			kinds = append(kinds, fmt.Sprintf("%s: %d", kind, n))
		}
	}
	if len(kinds) > 0 {
		summary += " (" + strings.Join(kinds, ", ") + ")"
	}
	t.Logf("%s, malformed %d", summary, bad)
	for _, p := range progs {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			if p.err != nil {
				t.Fatal(p.err)
			}
			if p.skip != "" {
				t.Skip(p.skip)
			}
			fn(t, p)
		})
	}
}

// skipKinds are the kinds of reasons to skip a test, in the order that
// skipReason checks them.
var skipKinds = []string{"needs a later phase", "builds for another target", "needs a display", "needs a missing library"}

// skipReason returns why the test with header h can't run here, with the
// kind of the reason (one of skipKinds), or "" and "".
func (c *config) skipReason(h *header) (why, kind string) {
	if why = phaseGate(h.phase, c.phase, c.phaseFrom); why != "" {
		return why, skipKinds[0]
	}
	if why = targetGate(h); why != "" {
		return why, skipKinds[1]
	}
	if why = displayGate(h.display, os.Getenv); why != "" {
		return why, skipKinds[2]
	}
	if why = missingLibraries(h.pkgConfig); why != "" {
		return why, skipKinds[3]
	}
	return "", ""
}

// phaseGate returns why a test for phase can't run when the tests run up
// to the phase limit, or "". from says where limit comes from.
func phaseGate(phase, limit int, from string) string {
	if phase > limit {
		return fmt.Sprintf("needs Phase %02d (%s)", phase, from)
	}
	return ""
}

// missingLibraries returns why a test that needs the pkg-config packages
// libs can't run here, or "" when pkg-config finds them all.
func missingLibraries(libs []string) string {
	if len(libs) == 0 {
		return ""
	}
	if _, err := exec.LookPath("pkg-config"); err != nil {
		return fmt.Sprintf("needs %s, but pkg-config is not installed", strings.Join(libs, " "))
	}
	var missing []string
	for _, lib := range libs {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		if exec.CommandContext(ctx, "pkg-config", "--exists", lib).Run() != nil {
			missing = append(missing, lib)
		}
		cancel()
	}
	if len(missing) == 0 {
		return ""
	}
	return fmt.Sprintf("needs %s (pkg-config --exists %s failed)", strings.Join(missing, " "), strings.Join(missing, " "))
}

// forEachCompiler runs fn once per C++ compiler of the matrix, as
// subtests named after the compilers. A compiler that isn't installed is a
// skip.
func (c *config) forEachCompiler(t *testing.T, fn func(*testing.T, compiler)) {
	for _, cc := range c.compilers {
		t.Run(cc.name, func(t *testing.T) {
			if cc.missing != "" {
				t.Skip(cc.missing)
			}
			fn(t, cc)
		})
	}
}

// runArgs returns the arguments of "gocpp run" for the program prog with
// the arguments args, with the GOCPP_FLAGS flags.
func (c *config) runArgs(prog string, args ...string) []string {
	return append(append(append([]string{"run"}, c.flags...), prog), args...)
}

// runTest runs a run or interop test with each compiler.
func (c *config) runTest(t *testing.T, p *program) {
	want := expect{
		stdout:   p.header.output,
		from:     "the // Output: block",
		exitCode: p.header.exitCode,
		stderr:   p.header.stderr,
	}
	c.forEachCompiler(t, func(t *testing.T, cc compiler) {
		checkRun(t, c.run(t, gocppJob(p.dir, runEnv(p.header), cc.cxx, p.header.phase, c.runArgs(p.arg)...)), want)
	})
}

// runEnv returns the environment that gocpp run gets for the test with the header h, on top of
// the harness's own: for a test that leaks a reference cycle on purpose (// lsan: cycle), the
// ASAN_OPTIONS that turn LeakSanitizer off (lsanEnv), which the program inherits.
func runEnv(h *header) []string {
	if !h.cycle {
		return nil
	}
	return []string{lsanEnv(os.Getenv("ASAN_OPTIONS"))}
}

// runVsGoTest builds a run test with real Go, runs it like gocpp run runs
// it, and checks the result against the test's expectations.
func (c *config) runVsGoTest(t *testing.T, p *program, n *vsGoCount) {
	if d := p.header.diverges; d != "" {
		n.diverges.Add(1)
		t.Skipf("%s shows divergence %s of docs/spec/language.md (its // divergence: line), so real Go does something else", p.path, d)
	}
	prog, build := c.goBuild(t, p)
	if build.problem != "" {
		t.Fatalf("%s: %s\n%s", build.cmd, build.problem, indent(string(build.stdout)))
	}
	if build.exit != 0 {
		if goRejects(string(build.stdout), goFileName(p.arg)) {
			n.rejected.Add(1)
			t.Skipf("go build rejects the code of %s, so it is not a Go program:\n%s", p.path, indent(string(build.stdout)))
		}
		t.Fatalf("%s: %s, and not because of an error in the program:\n%s", build.cmd, build.status, indent(string(build.stdout)))
	}
	n.compared.Add(1)
	r := c.run(t, job{dir: p.dir, name: prog, show: "the Go build of " + p.path + " (run in tests/run)"})
	checkRun(t, r, expect{
		stdout:   p.header.output,
		from:     "the // Output: block",
		exitCode: p.header.exitCode,
		stderr:   p.header.stderr,
	})
}

// exampleTest runs an example in its folder with each compiler, with the
// arguments of args.txt, the input of stdin.txt and the environment of
// env.txt.
func (c *config) exampleTest(t *testing.T, p *program) {
	want := expect{
		stdout:    string(p.ex.output),
		exact:     true,
		from:      strings.TrimSuffix(p.path, "main.gp") + "output.txt",
		exitCode:  p.header.exitCode,
		anyStderr: true,
	}
	c.forEachCompiler(t, func(t *testing.T, cc compiler) {
		j := gocppJob(p.dir, p.ex.env, cc.cxx, p.header.phase, c.runArgs(p.arg, p.ex.args...)...)
		if p.ex.stdin != nil {
			j.stdin = p.ex.stdin
			j.show += " < stdin.txt"
		}
		checkRun(t, c.run(t, j), want)
	})
}

// errorsTest checks an error test with gocpp check (checkJob). It must exit
// with 1 if the test wants errors, and with 0 if it wants none (warnings
// don't change the exit status).
func (c *config) errorsTest(t *testing.T, p *program) {
	r := c.run(t, checkJob(p.dir, p.arg, p.header.phase))
	if r.problem != "" {
		t.Fatalf("%s: %s\noutput:\n%s", r.cmd, r.problem, indent(string(r.stdout)))
	}
	diags, other := parseDiagnostics(string(r.stdout))
	problems := matchErrors(p.arg, p.wants, diags, other)
	wantExit := 0
	if wantsErrors(p.wants) {
		wantExit = 1
	}
	if r.exit != wantExit {
		problems = append([]string{fmt.Sprintf("%s, want exit code %d", r.status, wantExit)}, problems...)
	}
	if len(problems) > 0 {
		t.Errorf("%s:\n\t%s\noutput:\n%s", r.cmd, strings.Join(problems, "\n\t"), indent(string(r.stdout)))
	}
}

// interopErrorsTest builds an interop error test with each compiler
// (gocpp build, with the flags of GOCPP_FLAGS): the build must fail with
// exit code 1 and report the errors of the // ERROR comments, at their .gp
// lines, and nothing else: the C++ compiler's and the linker's errors of
// the user's code, which gocpp check can't see.
func (c *config) interopErrorsTest(t *testing.T, p *program) {
	c.forEachCompiler(t, func(t *testing.T, cc compiler) {
		args := append(append([]string{"build"}, c.flags...), "-o", filepath.Join(t.TempDir(), "prog"), p.arg)
		j := gocppJob(p.dir, nil, cc.cxx, p.header.phase, args...)
		j.combined = true
		r := c.run(t, j)
		if r.problem != "" {
			t.Fatalf("%s: %s\noutput:\n%s", r.cmd, r.problem, indent(string(r.stdout)))
		}
		diags, other := parseDiagnostics(string(r.stdout))
		// gocpp's count of the errors it leaves out ("(and 2 more errors
		// in gocpp's code: ...)") is no error.
		other = slices.DeleteFunc(other, othersLineRx.MatchString)
		problems := matchErrors(p.arg, p.wants, diags, other)
		if r.exit != 1 {
			problems = append([]string{fmt.Sprintf("%s, want exit code 1", r.status)}, problems...)
		}
		if len(problems) > 0 {
			t.Errorf("%s:\n\t%s\noutput:\n%s", r.cmd, strings.Join(problems, "\n\t"), indent(string(r.stdout)))
		}
	})
}

// othersLineRx matches the line after the errors of an interop error test
// that counts the compiler's errors that gocpp left out.
var othersLineRx = regexp.MustCompile(`^\(and \d+ more errors? in gocpp's code: -cpperrors=full shows (it|them)\)$`)

// checkJob returns the job that checks the error test file in dir:
// gocpp check -e file. -e reports every error, not only the first 10 (Go's
// own test/ runner passes -e too). The errors may go to stdout or to
// stderr, so both go to one buffer. phase is the test's (gocppJob).
func checkJob(dir, file string, phase int) job {
	j := gocppJob(dir, nil, "", phase, "check", "-e", file)
	j.combined = true
	return j
}

// goldenTest compares what gocpp emit prints with each compiler with the
// snapshot. With -update, the first compiler writes the snapshot, and the
// others must print the same.
func (c *config) goldenTest(t *testing.T, p *program) {
	snapshot := filepath.Join(p.dir, strings.TrimSuffix(p.arg, ".gp")+snapshotSuffix)
	rel := strings.TrimSuffix(p.path, ".gp") + snapshotSuffix
	written := false
	c.forEachCompiler(t, func(t *testing.T, cc compiler) {
		r := c.run(t, gocppJob(p.dir, nil, cc.cxx, p.header.phase, "emit", p.arg))
		if r.problem != "" || r.exit != 0 {
			t.Fatalf("%s: %s\nstderr:\n%s", r.cmd, r.why(), indent(string(r.stderr)))
		}
		got := trimRoot(string(r.stdout), c.roots)
		want, err := os.ReadFile(snapshot)
		if *update && !written {
			written = true
			if err == nil && string(want) == got {
				return
			}
			if err := os.WriteFile(snapshot, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Logf("wrote %s", rel)
			return
		}
		if errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%s: no snapshot %s; write it with: go test ./tests/... -run 'Golden/^%s$' -update",
				r.cmd, rel, regexp.QuoteMeta(p.name))
		}
		if err != nil {
			t.Fatal(err)
		}
		if string(want) != got {
			t.Errorf("%s: the C++ differs from the snapshot %s (if the change is on purpose, rewrite it with -update and look at git diff):\n%s",
				r.cmd, rel, explainMismatch(got, string(want)))
		}
	})
}

// diffTest runs a diff test with real Go, then with gocpp run and each
// compiler, and compares them (compareDiff). In the "combined" mode both
// run with stdout and stderr in one pipe.
func (c *config) diffTest(t *testing.T, p *program) {
	mode := p.header.diff
	ref := c.goReference(t, p, mode == diffCombined)
	c.forEachCompiler(t, func(t *testing.T, cc compiler) {
		j := gocppJob(p.dir, runEnv(p.header), cc.cxx, p.header.phase, c.runArgs(p.arg)...)
		j.combined = mode == diffCombined
		r := c.run(t, j)
		if r.problem != "" {
			label, out := streamOf(r, j.combined)
			t.Fatalf("%s: %s\n%s:\n%s", r.cmd, r.problem, label, indent(string(out)))
		}
		if problems := compareDiff(mode, r, ref); len(problems) > 0 {
			t.Errorf("%s:\n%s", r.cmd, strings.Join(problems, "\n"))
			// Show what the problems don't show yet.
			switch {
			case mode == diffCombined && bytes.Equal(r.stdout, ref.stdout):
				t.Logf("the output of both (stdout and stderr in one pipe):\n%s", indent(string(r.stdout)))
			case mode == diffStdout || mode == diffStderr && bytes.Equal(r.stderr, ref.stderr):
				t.Logf("gocpp stderr:\n%sGo stderr:\n%s", indent(string(r.stderr)), indent(string(ref.stderr)))
			}
		}
	})
}

// compareDiff compares what gocpp run did (gp) with what real Go did (ref)
// in a diff test with the given mode, and returns one message per
// difference. The exit code always counts, and stdout byte for byte. In the
// "stderr" mode, stderr counts byte for byte too. In the "combined" mode,
// stdout holds both streams, merged in the order they were written.
func compareDiff(mode string, gp, ref result) []string {
	var problems []string
	if gp.exit != ref.exit {
		problems = append(problems, fmt.Sprintf("%s, but real Go gives %s", gp.status, ref.status))
	}
	what := "stdout"
	if mode == diffCombined {
		what = "the output (stdout and stderr in one pipe)"
	}
	if !bytes.Equal(gp.stdout, ref.stdout) {
		problems = append(problems, fmt.Sprintf("%s differs from real Go's:\n%s", what, explainMismatch(string(gp.stdout), string(ref.stdout))))
	}
	if mode == diffStderr && !bytes.Equal(gp.stderr, ref.stderr) {
		problems = append(problems, fmt.Sprintf("stderr differs from real Go's:\n%s", explainMismatch(string(gp.stderr), string(ref.stderr))))
	}
	return problems
}

// goReference builds the diff test p with real Go and runs it, once for
// all compilers; with combined, its stdout and stderr go into one pipe. It
// copies the program to a temporary module as a .go file, then uses go
// build and runs the binary: go run would turn every failing exit code
// into 1.
func (c *config) goReference(t *testing.T, p *program, combined bool) result {
	t.Helper()
	prog, build := c.goBuild(t, p)
	if build.problem != "" || build.exit != 0 {
		t.Fatalf("real Go can't build %s, and a diff test must be a valid Go 1.25 program: %s\n%s",
			p.path, build.why(), indent(string(build.stdout)))
	}
	// The binary runs where gocpp run runs the program, in tests/diff, so
	// that relative file names mean the same files for both.
	r := c.run(t, job{dir: p.dir, name: prog, show: "the Go build of " + p.path + " (run in tests/diff)", combined: combined})
	if r.problem != "" {
		label, out := streamOf(r, combined)
		t.Fatalf("%s: %s\n%s:\n%s", r.cmd, r.problem, label, indent(string(out)))
	}
	return r
}

// goBuild copies the test program p to a temporary module as a .go file
// and builds it there with go build. It returns the binary's path and the
// result of go build, whose stdout holds go build's stdout and stderr.
func (c *config) goBuild(t *testing.T, p *program) (prog string, build result) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(p.dir, p.arg))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	goFile := goFileName(p.arg)
	if err := os.WriteFile(filepath.Join(dir, goFile), src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module difftest\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prog = filepath.Join(dir, "prog"+exe)
	// Only the Go that runs the tests, no workspace; GOAMD64=v1, because gc fuses a*b + c from
	// v3 on and gocpp gives the results without fusing (docs/spec/cpp-mapping.md).
	return prog, c.run(t, job{
		dir:      dir,
		env:      []string{"GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=", "GOAMD64=v1"},
		name:     "go",
		args:     []string{"build", "-o", prog, "."},
		show:     "go build (" + p.path + " copied to " + goFile + ")",
		combined: true,
	})
}

// goFileName returns the name of the .go file that goBuild copies the test
// file name (x.gp) to.
func goFileName(name string) string {
	return strings.TrimSuffix(name, ".gp") + ".go"
}

// goRejects reports whether the output of a failed go build (out) shows an
// error in the program's own code: a message about a line of goFile
// ("./x.go:8:2: undefined: foo"). Any other failure (a go.mod that the Go
// toolchain refuses, a missing toolchain) is a problem of the setup, not of
// the program.
func goRejects(out, goFile string) bool {
	rx := regexp.MustCompile(`(?m)^(\./)?` + regexp.QuoteMeta(goFile) + `:\d+(:\d+)?: `)
	return rx.MatchString(out)
}

// streamOf returns the stream of r to show when a command fails: its
// stderr, or with combined its stdout, which holds both streams.
func streamOf(r result, combined bool) (label string, out []byte) {
	if combined {
		return "output (stdout and stderr in one pipe)", r.stdout
	}
	return "stderr", r.stderr
}

// expect is what a run, interop or examples test wants from gocpp run.
type expect struct {
	stdout    string           // the expected stdout
	exact     bool             // stdout must be the same byte for byte; otherwise it is compared like a Go example
	from      string           // where the expected stdout comes from, for messages
	exitCode  int              // the expected exit code
	stderr    []*regexp.Regexp // each must match stderr; with none, stderr must be empty...
	anyStderr bool             // ...unless anyStderr is set: then stderr is not checked
}

// checkRun checks what gocpp run (or, in TestRunVsGo, a Go build) did
// against want.
func checkRun(t *testing.T, r result, want expect) {
	t.Helper()
	if r.problem != "" {
		t.Fatalf("%s: %s\nstderr:\n%s", r.cmd, r.problem, indent(string(r.stderr)))
	}
	problems, shown := runMismatches(r, want)
	for _, p := range problems {
		t.Errorf("%s: %s", r.cmd, p)
	}
	// Whatever failed, stderr may say why (a C++ compile error, a panic).
	if len(problems) > 0 && !shown {
		t.Logf("stderr:\n%s", indent(string(r.stderr)))
	}
}

// runMismatches compares the exit code, stdout and stderr of a finished
// program (r) with want, and returns one message per difference. shown
// says whether a message already holds the whole stderr.
func runMismatches(r result, want expect) (problems []string, shown bool) {
	if r.exit != want.exitCode {
		problems = append(problems, fmt.Sprintf("%s, want exit code %d", r.status, want.exitCode))
	}
	got := string(r.stdout)
	switch {
	case want.exact && got != want.stdout:
		problems = append(problems, fmt.Sprintf("stdout differs from %s:\n%s", want.from, explainMismatch(got, want.stdout)))
	case !want.exact && !sameOutput(got, want.stdout):
		problems = append(problems, fmt.Sprintf("stdout differs from %s (space at the start and at the end doesn't count):\n%s",
			want.from, explainMismatch(strings.TrimSpace(got), strings.TrimSpace(want.stdout))))
	}
	switch {
	case want.anyStderr:
	case len(want.stderr) == 0:
		if len(r.stderr) > 0 {
			problems = append(problems, fmt.Sprintf("stderr must be empty (the test has no // Stderr: line), but it is:\n%s", indent(string(r.stderr))))
			shown = true
		}
	default:
		for _, rx := range want.stderr {
			if !rx.Match(r.stderr) {
				problems = append(problems, fmt.Sprintf("stderr doesn't match the // Stderr: regexp %q", rx))
			}
		}
	}
	return problems, shown
}

// A job is a command for a test to run.
type job struct {
	dir      string   // the folder it runs in
	env      []string // KEY=value pairs to add to the environment
	stdin    []byte   // its standard input; nil for none
	name     string   // the program
	args     []string
	show     string // the command as messages show it, so that it can be run by hand
	combined bool   // stdout and stderr go to the same buffer, result.stdout
}

// A result is what a finished job did.
type result struct {
	cmd     string // the job's show string
	stdout  []byte
	stderr  []byte
	exit    int    // the exit code; -1 when the command was killed or didn't start
	status  string // "exit status 3", "signal: killed", for messages
	problem string // why the result can't be trusted (timeout, too much output, no start), or ""
}

// why returns the problem, or else the status, for messages.
func (r result) why() string {
	if r.problem != "" {
		return r.problem
	}
	return r.status
}

// gocppJob returns a job that runs the gocpp binary in dir with args, for a
// test of the phase (its // phase: line). env holds KEY=value pairs to add
// to the environment; cxx, if it isn't "", goes to gocpp in GOCPP_CXX, after
// them, and so does the GOCPP_GATE_PHASE of gateEnv(phase).
func gocppJob(dir string, env []string, cxx string, phase int, args ...string) job {
	if cxx != "" {
		env = append(env[:len(env):len(env)], "GOCPP_CXX="+cxx)
	}
	if gate := gateEnv(phase); gate != "" {
		env = append(env[:len(env):len(env)], gate)
	}
	var show strings.Builder
	if rel, err := filepath.Rel(repoRoot, dir); err == nil && rel != "." {
		show.WriteString("cd " + shellQuote(filepath.ToSlash(rel)) + " && ")
	}
	for _, kv := range env {
		key, value, _ := strings.Cut(kv, "=")
		show.WriteString(key + "=" + shellQuote(value) + " ")
	}
	show.WriteString("gocpp")
	for _, arg := range args {
		show.WriteString(" " + shellQuote(arg))
	}
	return job{dir: dir, env: env, name: gocppBin, args: args, show: show.String()}
}

// run runs a job with the timeout of the config, or until shortly before
// go test's own -timeout runs out, whichever comes first: then the test
// fails with a message instead of the test binary panicking with commands
// left running. It also stops the job when the test ends, and when the job
// prints more than maxOutput bytes. On Unix the job runs in a process group
// of its own (see proc_unix_test.go). To stop the job, the group gets
// SIGTERM, so that gocpp can remove its work folder, and what still runs
// waitDelay later is killed; when the job ends, whatever it left running is
// killed.
func (c *config) run(t *testing.T, j job) result {
	t.Helper()
	if isStopping() {
		return result{cmd: j.show, exit: -1, problem: "it didn't start: the test binary is stopping (a signal, or go test's -timeout)"}
	}
	stop := time.Now().Add(c.timeout)
	timedOut := fmt.Sprintf("it timed out after %v (GOCPP_TEST_TIMEOUT changes the limit)", c.timeout)
	if d, ok := t.Deadline(); ok && d.Add(-deadlineMargin).Before(stop) {
		stop = d.Add(-deadlineMargin)
		timedOut = fmt.Sprintf("it was stopped %v before the -timeout of go test runs out (go test -timeout changes that limit)", deadlineMargin)
	}
	ctx, cancel := context.WithDeadline(t.Context(), stop)
	defer cancel()
	cmd := exec.CommandContext(ctx, j.name, j.args...)
	cmd.Dir = j.dir
	cmd.Env = append(os.Environ(), j.env...) // a later KEY=value wins
	if j.stdin != nil {
		cmd.Stdin = bytes.NewReader(j.stdin)
	}
	stdout := &capBuffer{max: maxOutput, stop: cancel}
	stderr := stdout
	if !j.combined {
		stderr = &capBuffer{max: maxOutput, stop: cancel}
	}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	// A stopped command (SIGTERM, see inGroup) is killed if it still runs
	// waitDelay later; and when a process that the command started keeps the
	// output pipes open after the command exits, run stops waiting then.
	cmd.WaitDelay = waitDelay
	inGroup(cmd)
	err := cmd.Start()
	if err == nil {
		track(cmd.Process.Pid, true)
		err = cmd.Wait()
		killGroup(cmd.Process.Pid) // whatever the command left running
		track(cmd.Process.Pid, false)
	}

	r := result{cmd: j.show, stdout: stdout.buf.Bytes(), exit: -1}
	if !j.combined {
		r.stderr = stderr.buf.Bytes()
	}
	if cmd.ProcessState != nil {
		r.exit = cmd.ProcessState.ExitCode()
		r.status = cmd.ProcessState.String()
	}
	switch {
	case stdout.full || stderr.full:
		r.problem = fmt.Sprintf("it printed more than %d MiB, so it was stopped", maxOutput>>20)
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		r.problem = timedOut
	case cmd.ProcessState == nil:
		r.problem = fmt.Sprintf("it didn't start: %v", err)
	case errors.Is(err, exec.ErrWaitDelay):
		r.problem = "it exited, but a process that it started kept its output open"
	}
	return r
}

// capBuffer keeps the first max bytes written to it. When more come, it
// calls stop and drops the rest.
type capBuffer struct {
	buf  bytes.Buffer
	max  int
	full bool
	stop func()
}

func (b *capBuffer) Write(p []byte) (int, error) {
	if b.full {
		return len(p), nil
	}
	if room := b.max - b.buf.Len(); len(p) > room {
		b.buf.Write(p[:room])
		b.full = true
		b.stop()
		return len(p), nil
	}
	return b.buf.Write(p)
}
