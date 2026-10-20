package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/gotest"
	"github.com/huuhait/gocpp/internal/load"
)

const testUsage = `usage: gocpp test [build flags] [packages] [flags for the test binary]

gocpp test builds and runs the tests of the packages that the arguments name
(the package in the current folder without arguments), like go test: each
package gets one test binary, made of the package with its _test.gp files,
its external test package (package x_test) and a main package that gocpp
generates. A test is a function TestXxx(t *testing.T), a benchmark is
BenchmarkXxx(b *testing.B), and an example is ExampleXxx() with an "// Output:"
comment (docs/guides/testing.md). The output is go test's:

	ok  	example.com/app/geom	0.003s
	FAIL	example.com/app/geom	0.002s
	?   	example.com/app/cmd	[no test files]

It exits with status 1 when a test, a build or a package fails.

The flags of go test that gocpp test has: -v, -run regexp, -skip regexp,
-count n, -short, -failfast, -timeout d (default 10m, 0: none), -parallel n,
-cpu list, -list regexp, -bench regexp, -benchtime t, -benchmem, -fullpath
(full file names in t.Log), -c (write pkg.test, do not run it), -o file (write
the test binary to the file or folder; the test runs unless -c), and -args
(the rest goes to the test binary as it is). The build flags are those of
gocpp build. A flag that gocpp test does not know goes to the test binary.
Fuzz targets run their seed corpus; -fuzz, -race, -cover and -json are not
supported yet.

` + interopHelp + `The flags:
`

// testRun is what one run of gocpp test knows about the package it is working on.
type testRun struct {
	importPath  string // the package under test
	noTestFiles bool
	setupFailed bool // the test files can't be used (a wrong signature)
}

// testFlag is a flag of go test that gocpp test passes to the test binary as -test.name.
type testFlag struct {
	value bool // takes a value (else it is a bool)
}

var testFlags = map[string]testFlag{
	"v": {}, "short": {}, "failfast": {}, "benchmem": {}, "fullpath": {},
	"run": {true}, "skip": {true}, "count": {true}, "timeout": {true}, "parallel": {true},
	"cpu": {true}, "list": {true}, "bench": {true}, "benchtime": {true}, "shuffle": {true},
}

// testOptions are the arguments of gocpp test.
type testOptions struct {
	build     buildFlags
	pkgs      []string
	binArgs   []string // the flags for the test binary, as "-test.v=true"
	timeout   time.Duration
	hasTimout bool
	v         bool
	list      bool
	bench     bool
	c         bool   // -c
	out       string // -o
	args      []string
}

// parseTestArgs reads the arguments of gocpp test, like go test does: the flags that it knows may
// be anywhere among the packages, the others go to the test binary, and -args ends them.
func parseTestArgs(args []string, stderr io.Writer) (*testOptions, int) {
	o := &testOptions{timeout: 10 * time.Minute}
	fail := func(format string, a ...any) (*testOptions, int) {
		fmt.Fprintf(stderr, "gocpp test: "+format+"\n", a...)
		return nil, 2
	}
	// The build flags: parsed with the flag package, one at a time.
	buildFS := flag.NewFlagSet("gocpp test", flag.ContinueOnError)
	buildFS.SetOutput(io.Discard)
	o.build.register(buildFS, false)
	o.build.extra.registerCompdb(buildFS)
	buildNames := map[string]bool{}
	buildFS.VisitAll(func(f *flag.Flag) { buildNames[f.Name] = true })
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-args" || arg == "--args" {
			o.args = append(o.args, args[i+1:]...)
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			o.pkgs = append(o.pkgs, arg)
			continue
		}
		name := strings.TrimLeft(arg, "-")
		value, hasValue := "", false
		if j := strings.IndexByte(name, '='); j >= 0 {
			name, value, hasValue = name[:j], name[j+1:], true
		}
		name = strings.TrimPrefix(name, "test.")
		switch {
		case name == "h" || name == "help":
			fmt.Fprint(stderr, testUsage)
			buildFS.SetOutput(stderr)
			buildFS.PrintDefaults()
			return nil, 0
		case name == "c":
			o.c = true
		case name == "o":
			if !hasValue {
				if i+1 >= len(args) {
					return fail("flag needs an argument: -o")
				}
				i++
				value = args[i]
			}
			o.out = value
		case name == "vet":
			// go test runs vet; gocpp vet is a later phase: accepted and ignored.
		case name == "json":
			return fail("-json is not supported yet")
		case name == "race" || name == "cover" || name == "coverprofile" || name == "covermode" || name == "coverpkg" ||
			name == "fuzz" || name == "cpuprofile" || name == "memprofile" || name == "trace":
			return fail("-%s is not supported yet", name)
		case buildNames[name] && !isTestFlag(name):
			// Hand the flag to the flag package, with its value if it has one.
			bf := buildFS.Lookup(name)
			isBool := false
			if b, ok := bf.Value.(interface{ IsBoolFlag() bool }); ok {
				isBool = b.IsBoolFlag()
			}
			one := []string{arg}
			if !hasValue && !isBool {
				if i+1 >= len(args) {
					return fail("flag needs an argument: -%s", name)
				}
				i++
				one = append(one, args[i])
			}
			if err := buildFS.Parse(one); err != nil {
				return fail("%v", err)
			}
		default:
			tf, known := testFlags[name]
			if !known {
				o.binArgs = append(o.binArgs, arg) // for the test binary: it reports what it does not know
				continue
			}
			if tf.value && !hasValue {
				if i+1 >= len(args) {
					return fail("flag needs an argument: -%s", name)
				}
				i++
				value = args[i]
				hasValue = true
			}
			if !tf.value && !hasValue {
				value = "true"
			}
			switch name {
			case "v":
				o.v = value == "true"
			case "list":
				o.list = value != ""
			case "bench":
				o.bench = value != ""
			case "timeout":
				d, err := time.ParseDuration(value)
				if err != nil {
					return fail("invalid value %q for flag -timeout: parse error", value)
				}
				o.timeout, o.hasTimout = d, true
			}
			o.binArgs = append(o.binArgs, "-test."+name+"="+value)
		}
	}
	return o, 0
}

// runTest is "gocpp test".
func runTest(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	o, code := parseTestArgs(args, stderr)
	if o == nil {
		return code
	}
	d, code := newDriver("test", &o.build, stdout, stderr)
	if d == nil {
		return code
	}
	defer d.stopSignals()
	c := d.check
	c.tests = true

	patterns := o.pkgs
	local := len(patterns) == 0 // the package of the current folder: go test's "local directory mode"
	if local {
		patterns = []string{"."}
	}
	m := matchPatterns(c.dir, patterns)
	for _, w := range m.warnings {
		fmt.Fprintln(stderr, w)
	}
	if len(m.errors) > 0 {
		for _, e := range m.errors {
			fmt.Fprintln(stderr, e)
		}
		return exitErrors
	}
	if m.files {
		fmt.Fprintln(stderr, "gocpp test: a list of .gp files is not supported yet; name the package")
		return exitErrors
	}
	c.warnNoMatch(m)
	failed := false
	for _, pat := range m.patterns {
		if d.interrupted() != nil {
			break
		}
		if !d.testPackage(pat, o, local, len(m.patterns) == 1) {
			failed = true
		}
	}
	if sig := d.interrupted(); sig != nil {
		return stopStatus(sig)
	}
	if failed {
		if !local {
			fmt.Fprintln(stdout, "FAIL")
		}
		return exitErrors
	}
	return 0
}

// testPackage builds and runs the test of the package that the pattern names, and prints its
// result like go test. It reports whether the package passed.
func (d *driver) testPackage(pattern string, o *testOptions, local, single bool) bool {
	c := d.check
	ti := &testRun{}
	d.test = ti
	c.testMain = func(fset *token.FileSet, p, xt *load.Package) ([]byte, []diag.Diagnostic) {
		ti.importPath = p.ImportPath
		if !gotest.HasTestFiles(p, xt) {
			ti.noTestFiles = true
			return nil, nil
		}
		found, errs := gotest.Scan(fset, p, xt)
		if len(errs) > 0 {
			ti.setupFailed = true
			return nil, errs
		}
		return gotest.Main(p.ImportPath, found), nil
	}
	c.typeChecked, c.testSetupFailed = false, false
	prog, code := d.compile([]string{pattern})
	if prog == nil {
		path := ti.importPath
		if path == "" && c.lastProg != nil && len(c.lastProg.Roots) > 0 {
			path = c.lastProg.Roots[0].ImportPath
		}
		switch {
		case code == exitNoTestFiles:
			fmt.Fprintf(d.stdout, "?   \t%s\t[no test files]\n", path)
			return true
		case d.interrupted() != nil:
			return false
		case code == exitInternal:
			fmt.Fprintf(d.stdout, "FAIL\t%s [internal error]\n", path)
		case c.typeChecked && !ti.setupFailed && !c.testSetupFailed:
			fmt.Fprintf(d.stdout, "FAIL\t%s [build failed]\n", path)
		default:
			fmt.Fprintf(d.stdout, "FAIL\t%s [setup failed]\n", path)
		}
		return false
	}
	defer d.removeWork()
	if o.c || o.out != "" {
		// -c and -o write the test binary (pkg.test, or the name or folder that -o gives); with -o and
		// not -c the test runs as well.
		out := o.out
		name := filepath.Base(ti.importPath) + ".test" + exeSuffix
		if out == "" {
			out = name
		}
		if !filepath.IsAbs(out) {
			out = filepath.Join(c.dir, out)
		}
		if isDir(out) || strings.HasSuffix(o.out, "/") || strings.HasSuffix(o.out, string(filepath.Separator)) {
			out = filepath.Join(out, name)
		}
		var err error
		if o.c {
			err = build.MoveFile(prog.exe, out)
		} else {
			err = copyExecutable(prog.exe, out)
		}
		if err != nil {
			fmt.Fprintf(d.stderr, "gocpp test: %v\n", err)
			return false
		}
		if o.c {
			return true
		}
	}

	// Run the test binary as go test does (cmd/go/internal/test).
	stream := local || o.bench
	showPass := o.v || o.list
	var buf bytes.Buffer
	var out io.Writer = d.stdout
	switch {
	case stream:
	case showPass && single:
		out = io.MultiWriter(d.stdout, &buf)
	default:
		out = &buf
	}
	args := []string{"-test.paniconexit0"}
	if !o.hasTimout {
		args = append(args, "-test.timeout="+o.timeout.String())
	}
	args = append(args, o.binArgs...)
	args = append(args, o.args...)
	if d.flags.x {
		fmt.Fprintf(d.stderr, "%s\n", strings.Join(append([]string{build.ShellQuoteWork(prog.exe, prog.work)}, args...), " "))
	}
	killAfter := time.Duration(0)
	if o.timeout > 0 && !o.bench {
		killAfter = o.timeout + max(o.timeout/10, 5*time.Second)
		if killAfter < o.timeout+time.Minute {
			killAfter = o.timeout + time.Minute
		}
	}
	killedFor := ""
	t0 := time.Now()
	err := d.execTest(prog, args, out, killAfter, &killedFor)
	elapsed := fmt.Sprintf("%.3fs", time.Since(t0).Seconds())
	data := buf.Bytes()
	if err == nil && !showPass {
		buf.Reset()
	}
	writeTestResult(out, ti.importPath, elapsed, err, data, killedFor, killAfter)
	if out == io.Writer(&buf) {
		d.stdout.Write(buf.Bytes())
	}
	return err == nil
}

// writeTestResult prints the line that go test prints after the test binary of the package
// importPath ran (cmd/go/internal/test): "ok  \tpkg\t0.003s" with " [no tests to run]" when the binary
// said so, or what the failure was and "FAIL\tpkg\t0.003s". data is the output of the program that
// was buffered (empty in stream mode, where it went to out already, so that a failure prints
// "exit status 1" after it, as go test does in the local directory mode), err what Wait returned,
// killedFor the signal of the backup alarm ("" if it did not fire).
func writeTestResult(out io.Writer, importPath, elapsed string, err error, data []byte, killedFor string, killAfter time.Duration) {
	if err == nil {
		norun := ""
		if bytes.HasPrefix(data, []byte(noTestsToRun[1:])) || bytes.Contains(data, []byte(noTestsToRun)) {
			norun = " [no tests to run]"
		}
		if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
			out.Write([]byte("\n"))
		}
		fmt.Fprintf(out, "ok  \t%s\t%s%s\n", importPath, elapsed, norun)
		return
	}
	if killedFor != "" {
		fmt.Fprintf(out, "*** Test killed with %s: ran too long (%v).\n", killedFor, killAfter)
	}
	var ee *exec.ExitError
	if len(data) == 0 || !errors.As(err, &ee) || !ee.Exited() {
		fmt.Fprintf(out, "%s\n", err)
	} else if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		out.Write([]byte("\n"))
	}
	fmt.Fprintf(out, "FAIL\t%s\t%s\n", importPath, elapsed)
}

// noTestsToRun is what the test binary prints when no test matched (go test's noTestsToRun).
const noTestsToRun = "\ntesting: warning: no tests to run\n"

// testHardKillDelay is how long after the backup alarm's SIGQUIT the test binary is killed (a variable
// for the tests of execTest).
var testHardKillDelay = 5 * time.Second

// execTest runs the test binary and waits for it. The output and the errors of the program both
// go to out, in the order they came. After killAfter (if > 0) it gets SIGQUIT and, testHardKillDelay
// (5 seconds) later, SIGKILL: go test's backup alarm for a test binary that its own -test.timeout
// does not stop. killed names the signal when that happened. The test binary runs in a process
// group of its own (not on Windows), and the group is killed with the test binary at the end of the
// alarm, or when a signal that gocpp got was passed on to it (watchSignals), so that the processes
// that the tests started do not outlive it (one that ignores SIGINT, such as a background job of a
// shell, would).
func (d *driver) execTest(p *program, args []string, out io.Writer, killAfter time.Duration, killed *string) error {
	newCmd := func() *exec.Cmd {
		cmd := exec.Command(p.exe, args...)
		cmd.Dir = p.dir
		cmd.Env = append(programEnv(), "PWD="+p.dir) // see programEnv: the user's TEMP and LOCALAPPDATA
		if d.flags.asan && os.Getenv("ASAN_OPTIONS") == "" {
			// Memory is reference counted (ADR-004): a cycle of values that the program drops is a
			// leak, which LeakSanitizer reports at the exit of the test binary and fails the run
			// with. The test harness of gocpp turns the check off too. ASAN_OPTIONS set by the
			// caller is left as it is.
			cmd.Env = append(cmd.Env, "ASAN_OPTIONS=detect_leaks=0")
		}
		cmd.Stdout, cmd.Stderr = out, out
		setTestGroup(cmd)
		if killAfter > 0 {
			// A process of the test that holds the output open does not keep us waiting for ever.
			cmd.WaitDelay = max(killAfter/20, testHardKillDelay)
		}
		return cmd
	}
	notify(d.sigs, syscall.SIGQUIT)
	if sig := d.interrupted(); sig != nil {
		return fmt.Errorf("interrupted by %v", sig)
	}
	cmd, err := build.Start(d.ctx, newCmd)
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.running, d.runningGroup = cmd, hasTestGroup
	if d.stopped != nil {
		signalTest(cmd, d.stopped.(syscall.Signal), true)
	}
	d.mu.Unlock()
	var timer *time.Timer
	var hard atomic.Pointer[time.Timer]
	if killAfter > 0 {
		timer = time.AfterFunc(killAfter, func() {
			d.mu.Lock()
			*killed = "quit"
			d.mu.Unlock()
			signalTest(cmd, syscall.SIGQUIT, false)
			hard.Store(time.AfterFunc(testHardKillDelay, func() { signalTest(cmd, syscall.SIGKILL, false) }))
		})
	}
	err = cmd.Wait()
	if timer != nil {
		timer.Stop()
	}
	d.mu.Lock()
	d.running, d.runningGroup = nil, false
	fired := *killed != "" || d.groupSignaled
	d.groupSignaled = false
	d.mu.Unlock()
	if h := hard.Load(); h != nil {
		h.Stop()
	}
	if fired {
		signalTest(cmd, syscall.SIGKILL, true) // the group: what the tests left running
	}
	return err
}

// copyExecutable copies the program file src to dst.
func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o777); err != nil {
		return err
	}
	os.Remove(dst) // not written through a link or a running program
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o777)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// isTestFlag reports whether name is a flag of the test binary that gocpp test knows. It comes before the
// build flags: -v is the test's (verbose output), though gocpp build has a -v of its own (it prints the
// packages that are built).
func isTestFlag(name string) bool {
	_, ok := testFlags[name]
	return ok
}
