// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testing provides support for automated testing of Go packages.
// It is intended to be used in concert with the "go test" command, which automates
// execution of any function of the form
//
//	func TestXxx(*testing.T)
//
// where Xxx does not start with a lowercase letter. The function name
// serves to identify the test routine.
//
// Within these functions, use [T.Error], [T.Fail] or related methods to signal failure.
//
// To write a new test suite, create a file that
// contains the TestXxx functions as described here,
// and give that file a name ending in "_test.gp".
// The file will be excluded from regular
// package builds but will be included when the "gocpp test" command is run.
//
// The test file can be in the same package as the one being tested,
// or in a corresponding package with the suffix "_test".
//
// This is gocpp's testing package, written for gocpp: it follows Go 1.25's testing package
// (the rules for the output, the names of subtests, -run and -skip, parallel tests, cleanup
// functions, examples and benchmarks) and is driven by "gocpp test", which generates the main
// package of each test binary. What it leaves out for now is listed in stdlib/testing/PORT.md
// (the methods that need package time, context or io, fuzzing, coverage, profiles, -json).
package testing

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"time"
)

// ---------------------------------------------------------------------------------------------
// The flags of the test binary. "gocpp test" passes them like "go test" does (-test.v, -test.run,
// ...). As in Go, Init registers them with package flag's command line, and flag.Parse (called
// by m.Run when the TestMain has not called it) reads them next to the flags of the test program.

var (
	initRan bool // Init registered the flags
	derived bool // the values below that are computed from the flags are set (parseFlags)

	short        bool   // -test.short
	failFast     bool   // -test.failfast
	verbose      bool   // -test.v (chattyF.on, after the flags are parsed)
	matchPattern string // -test.run
	skipPattern  string // -test.skip
	matchList    string // -test.list
	matchBench   string // -test.bench
	benchMem     bool   // -test.benchmem
	fullPath     bool   // -test.fullpath
	benchTimeN   int    // -test.benchtime=Nx
	benchTimeD   int64  // -test.benchtime=1s, in nanoseconds
	testCount    = 1    // -test.count
	parallelism  int    // -test.parallel
	timeoutNs    int64  // -test.timeout, in nanoseconds; 0: none
	cpuList      []int  // -test.cpu
	chatty       *chattyPrinter
	haveExamples bool

	numFailed int // the number of tests that failed (guarded by the lock), for -failfast

	// What the flags are read into.
	chattyF    chattyFlag
	countU     uint
	timeoutD   time.Duration
	cpuListStr string
	shuffleStr string
	benchTime  = durationOrCountFlag{d: time.Second}

	// Flags that are accepted and ignored: no profiles, coverage, trace or fuzzing yet.
	ignoredString                                   string
	ignoredInt                                      int
	ignoredBool                                     bool
	fuzzDuration                                    durationOrCountFlag
	minimizeDuration                                = durationOrCountFlag{d: 60 * time.Second, allowZero: true}
	blockProfileRate, mutexProfileFraction, memRate int
)

// Init registers testing flags. These flags are automatically registered by
// the "gocpp test" command before running test functions, so Init is only needed
// when calling functions such as Benchmark without using "gocpp test".
//
// Init has no effect if it was already called.
func Init() {
	if initRan {
		return
	}
	initRan = true
	flag.BoolVar(&short, "test.short", false, "run smaller test suite to save time")
	flag.BoolVar(&failFast, "test.failfast", false, "do not start new tests after the first test failure")
	flag.StringVar(&ignoredString, "test.outputdir", "", "write profiles to `dir`")
	flag.Var(&chattyF, "test.v", "verbose: print additional output")
	flag.UintVar(&countU, "test.count", 1, "run tests and benchmarks `n` times")
	flag.StringVar(&ignoredString, "test.coverprofile", "", "write a coverage profile to `file`")
	flag.StringVar(&ignoredString, "test.gocoverdir", "", "write coverage intermediate files to this directory")
	flag.StringVar(&matchList, "test.list", "", "list tests, examples, and benchmarks matching `regexp` then exit")
	flag.StringVar(&matchPattern, "test.run", "", "run only tests and examples matching `regexp`")
	flag.StringVar(&skipPattern, "test.skip", "", "do not list or run tests matching `regexp`")
	flag.StringVar(&ignoredString, "test.memprofile", "", "write an allocation profile to `file`")
	flag.IntVar(&memRate, "test.memprofilerate", 0, "set memory allocation profiling `rate` (see runtime.MemProfileRate)")
	flag.StringVar(&ignoredString, "test.cpuprofile", "", "write a cpu profile to `file`")
	flag.StringVar(&ignoredString, "test.blockprofile", "", "write a goroutine blocking profile to `file`")
	flag.IntVar(&blockProfileRate, "test.blockprofilerate", 1, "set blocking profile `rate` (see runtime.SetBlockProfileRate)")
	flag.StringVar(&ignoredString, "test.mutexprofile", "", "write a mutex contention profile to the named file after execution")
	flag.IntVar(&mutexProfileFraction, "test.mutexprofilefraction", 1, "if >= 0, calls runtime.SetMutexProfileFraction()")
	flag.BoolVar(&ignoredBool, "test.paniconexit0", false, "panic on call to os.Exit(0)")
	flag.StringVar(&ignoredString, "test.trace", "", "write an execution trace to `file`")
	flag.DurationVar(&timeoutD, "test.timeout", 0, "panic test binary after duration `d` (default 0, timeout disabled)")
	flag.StringVar(&cpuListStr, "test.cpu", "", "comma-separated `list` of cpu counts to run each test with")
	flag.IntVar(&parallelism, "test.parallel", libNCPU(), "run at most `n` tests in parallel")
	flag.StringVar(&ignoredString, "test.testlogfile", "", "write test action log to `file` (for use only by cmd/go)")
	flag.StringVar(&shuffleStr, "test.shuffle", "off", "randomize the execution order of tests and benchmarks")
	flag.BoolVar(&fullPath, "test.fullpath", false, "show full file names in error messages")

	// The benchmark and fuzzing flags.
	flag.StringVar(&matchBench, "test.bench", "", "run only benchmarks matching `regexp`")
	flag.BoolVar(&benchMem, "test.benchmem", false, "print memory allocations for benchmarks")
	flag.Var(&benchTime, "test.benchtime", "run each benchmark for duration `d` or N times if `d` is of the form Nx")
	flag.StringVar(&ignoredString, "test.fuzz", "", "run the fuzz test matching `regexp`")
	flag.Var(&fuzzDuration, "test.fuzztime", "time to spend fuzzing; default is to run indefinitely")
	flag.Var(&minimizeDuration, "test.fuzzminimizetime", "time to spend minimizing a value after finding a failing input")
	flag.StringVar(&ignoredString, "test.fuzzcachedir", "", "directory where interesting fuzzing inputs are stored (for use only by cmd/go)")
	flag.BoolVar(&ignoredBool, "test.fuzzworker", false, "coordinate with the parent process to fuzz random values (for use only by cmd/go)")

	parallelism = libNCPU()
	countU = 1
	benchTimeD = int64(benchTime.d)
}

// parseFlags parses the command line (unless the program did: a TestMain that calls flag.Parse),
// and sets the values that are computed from the flags. It is what m.Run does before it runs a test.
func parseFlags() {
	Init()
	if !flag.Parsed() {
		flag.Parse()
	}
	if derived {
		return
	}
	derived = true
	verbose = chattyF.on
	testCount = int(countU)
	timeoutNs = int64(timeoutD)
	benchTimeN, benchTimeD = benchTime.n, int64(benchTime.d)
	cpuList = nil
	for _, val := range splitByte(cpuListStr, ',') {
		val = trimSpace(val)
		if val == "" {
			continue
		}
		n, ok := atoi(val)
		if !ok || n <= 0 {
			fmt.Fprintf(os.Stderr, "testing: invalid value %q for -test.cpu\n", val)
			libExit(1)
		}
		cpuList = append(cpuList, n)
	}
	if len(cpuList) == 0 {
		cpuList = append(cpuList, libNCPU())
	}
	if shuffleStr != "off" {
		fmt.Fprintln(os.Stderr, "testing: -test.shuffle is not supported yet")
		libExit(2)
	}
	if verbose {
		chatty = &chattyPrinter{}
	}
}

// chattyFlag is the value of -test.v: a boolean flag that also takes "test2json".
type chattyFlag struct {
	on   bool // -v is set in some form
	json bool // -v=test2json is set, to make output better for test2json
}

func (*chattyFlag) IsBoolFlag() bool { return true }

func (f *chattyFlag) Set(arg string) error {
	switch arg {
	default:
		return fmt.Errorf("invalid flag -test.v=%s", arg)
	case "true", "test2json":
		f.on = true
		f.json = arg == "test2json"
	case "false":
		f.on = false
		f.json = false
	}
	return nil
}

func (f *chattyFlag) String() string {
	if f.json {
		return "test2json"
	}
	if f.on {
		return "true"
	}
	return "false"
}

func (f *chattyFlag) Get() any {
	if f.json {
		return "test2json"
	}
	return f.on
}

// durationOrCountFlag is the value of -test.benchtime and -test.fuzztime: a duration or a count "Nx".
type durationOrCountFlag struct {
	d         time.Duration
	n         int
	allowZero bool
}

func (f *durationOrCountFlag) String() string {
	if f.n > 0 {
		return fmt.Sprintf("%dx", f.n)
	}
	return f.d.String()
}

func (f *durationOrCountFlag) Set(s string) error {
	if hasSuffix(s, "x") {
		n, ok := atoi(s[:len(s)-1])
		if !ok || n < 0 || (!f.allowZero && n == 0) {
			return fmt.Errorf("invalid count")
		}
		*f = durationOrCountFlag{n: n}
		return nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 || (!f.allowZero && d == 0) {
		return fmt.Errorf("invalid duration")
	}
	*f = durationOrCountFlag{d: d}
	return nil
}

// Short reports whether the -test.short flag is set.
func Short() bool {
	if !initRan {
		panic("testing: Short called before Init")
	}
	// Catch code that calls this from TestMain without first calling flag.Parse.
	if !flag.Parsed() {
		panic("testing: Short called before Parse")
	}
	return short
}

// testBinary is set by MainStart: the program was made by "gocpp test".
var testBinary bool

// Testing reports whether the current code is being run in a test.
// This will report true in programs created by "gocpp test",
// false in programs created by "gocpp build".
func Testing() bool {
	return testBinary
}

// CoverMode reports what the test coverage mode is set to. The
// values are "set", "count", or "atomic". The return value will be
// empty if test coverage is not enabled.
func CoverMode() string {
	return ""
}

// Verbose reports whether the -test.v flag is set.
func Verbose() bool {
	if !initRan {
		panic("testing: Verbose called before Init")
	}
	if !flag.Parsed() {
		panic("testing: Verbose called before Parse")
	}
	return chattyF.on
}

// ---------------------------------------------------------------------------------------------
// Output.

// A writer is where the output of a test goes: the standard output for the top-level tests, and
// the output buffer of the parent (with the lines indented) for the subtests.
type writer interface {
	Write(b []byte) (int, error)
}

type stdoutWriter struct{}

func (stdoutWriter) Write(b []byte) (int, error) {
	fmt.Print(string(b))
	return len(b), nil
}

// indenter writes to the output of its common, with every line indented by four spaces.
type indenter struct {
	c *common
}

func (w indenter) Write(b []byte) (n int, err error) {
	n = len(b)
	for len(b) > 0 {
		end := -1
		for i := 0; i < len(b); i++ {
			if b[i] == '\n' {
				end = i
				break
			}
		}
		if end == -1 {
			end = len(b)
		} else {
			end++
		}
		// An indent of 4 spaces will neatly align the dashes with the status
		// indicator of the parent.
		line := b[:end]
		w.c.output = append(w.c.output, "    "...)
		w.c.output = append(w.c.output, line...)
		b = b[end:]
	}
	return
}

// chattyPrinter prints the output of a verbose run (-test.v) as it comes, and says which test a
// line belongs to when two tests print in turn ("=== NAME  TestX"), like Go's.
type chattyPrinter struct {
	lastName string // name of the last testing.T that produced output
}

// Updatef prints a message about the status of the named test and records it as the most
// recently logged test.
func (p *chattyPrinter) Updatef(testName, format string, args ...any) {
	libLock()
	defer libUnlock()
	p.lastName = testName
	fmt.Print(fmt.Sprintf(format, args...))
}

// Printf prints a message, generated by the named test, that does not necessarily mention that
// tests's name itself.
func (p *chattyPrinter) Printf(testName, format string, args ...any) {
	libLock()
	defer libUnlock()
	if p.lastName == "" {
		p.lastName = testName
	} else if p.lastName != testName {
		fmt.Print(fmt.Sprintf("=== NAME  %s\n", testName))
		p.lastName = testName
	}
	fmt.Print(fmt.Sprintf(format, args...))
}

// ---------------------------------------------------------------------------------------------
// common, T, TB.

// common holds the elements common between T and B and
// captures common methods such as Errorf.
type common struct {
	output         []byte          // Output generated by test or benchmark.
	w              writer          // For flushToParent.
	ran            bool            // Test or benchmark (or one of its subtests) was executed.
	failed         bool            // Test or benchmark has failed.
	skipped        bool            // Test or benchmark has been skipped.
	done           bool            // Test is finished and all subtests have completed.
	helperNames    map[string]bool // The functions marked by Helper, by name.
	cleanups       []func()        // The functions to call when the test and subtests complete.
	cleanupStarted bool            // Registered cleanup callbacks have started to execute
	finished       bool            // Test function has completed.
	hasSub         bool            // whether there are sub-benchmarks.
	isParallel     bool            // Whether the test is parallel.
	denyParallel   bool            // Whether the test (or an ancestor) used Setenv or Chdir

	parent   *common
	level    int       // Nesting depth of test or benchmark.
	name     string    // Name of test or benchmark.
	start    int64     // Time test or benchmark started
	duration int64     // The time the test or benchmark took, in nanoseconds
	barrier  chan bool // Not nil when T.Parallel is usable (nil for B, and when fuzzing). Never closed: a parent releases its parallel subtests one at a time, in order (resume).
	resume   chan bool // A parallel test waits here for its parent to release it and for a free slot (testState).
	resumed  chan bool // A parallel test says here that it runs again (its "=== CONT" line is printed).
	ended    chan bool // Closed when a parallel test is done (releaseParallel waits a moment for it).
	signal   chan bool // To signal a test is done.
	sub      []*T      // Queue of subtests to be run in parallel.
	chatty   *chattyPrinter
	bench    bool // Whether the current test is a benchmark.

	tempDir    string
	tempDirErr string
	tempDirSeq int

	// What frameSkip needs to find the line of the test that called Log (the frames are those of
	// callers; Go keeps program counters).
	runner      string  // Name of the function that runs the test (tRunner).
	cleanupName string  // Name of the closure that Cleanup wraps around a cleanup function, while one runs.
	cleanupPc   []frame // The stack at the point where Cleanup was called, while a cleanup function runs.
	creator     []frame // If level > 0, the stack at the point where the parent called t.Run.
}

// isHelperFunc reports whether the function fn was marked by Helper in c (frameSkip goes on
// with the parent of a subtest, and its own marks, when the search leaves the subtest).
func (c *common) isHelperFunc(fn string) bool {
	return c.helperNames[fn]
}

// A frame is one line of libFrames.
type frame struct {
	fn   string
	file string
	line int
}

// callers returns the frames of the calling goroutine that are not in package testing, innermost
// first, or an empty slice when the program has no debug information.
func callers() []frame {
	var out []frame
	for _, ln := range splitByte(libFrames(64), '\n') {
		if ln == "" {
			continue
		}
		parts := splitByte(ln, '\t')
		if len(parts) != 3 {
			continue
		}
		n, _ := atoi(parts[2])
		out = append(out, frame{parts[0], parts[1], n})
	}
	return out
}

// inTesting reports whether the function name is a function of package testing.
func inTesting(fn string) bool {
	return hasPrefix(fn, "testing.")
}

// userFrames drops the frames of package testing that come first in a list of callers (callers
// itself and the methods that were called to get here): what is left starts in the code that called
// into the package.
func userFrames(fs []frame) []frame {
	for len(fs) > 0 && inTesting(fs[0].fn) {
		fs = fs[1:]
	}
	return fs
}

// frameSkip returns the frame of the first function on the stack of the caller that is not marked
// as a helper (Helper), which is where the test called Log, Error, Fatal and so on, and whether
// there is one. The search stops at the tRunner that called the test function; in a subtest it goes
// on in the parent test, from the point of the call to Run. It is Go's frameSkip.
func (c *common) frameSkip() (frame, bool) {
	raw := callers()
	fs := userFrames(raw)
	if len(fs) == 0 {
		// All of it is package testing (it logged by itself, as fRunner does when a fuzz test
		// returns without calling F.Fuzz): the line of testing's code that did, as Go prints one.
		for _, f := range raw {
			if f.file != "" && !logHelper(f.fn) {
				return f, true
			}
		}
		return frame{}, false
	}
	var first, prev, fr frame
	haveFirst := false
	for i := 0; i < len(fs); i, prev = i+1, fr {
		fr = fs[i]
		if fr.fn == "runtime.gopanic" {
			continue
		}
		if c.cleanupName != "" && fr.fn == c.cleanupName {
			fs, i = c.cleanupPc, -1
			continue
		}
		if !haveFirst {
			first, haveFirst = fr, true
		}
		if c.runner != "" && fr.fn == c.runner {
			// We've gone up all the way to the tRunner calling the test function (so the user must
			// have called Helper from inside that test function). If this is a top-level test, only
			// skip up to the test function itself. If we're in a subtest, continue searching in the
			// parent test, starting from the point of the call to Run which created this subtest.
			if c.level > 1 && c.parent != nil {
				fs, i = c.creator, -1
				c = c.parent
				continue
			}
			return prev, prev.fn != ""
		}
		if !c.isHelperFunc(fr.fn) {
			// Found a frame that wasn't inside a helper function.
			return fr, true
		}
	}
	return first, haveFirst
}

// logHelper reports whether fn is one of the functions of package testing between the call of Log,
// Error, Fatal and so on and the code that called it.
func logHelper(fn string) bool {
	if fn == "testing.callers" {
		return true
	}
	for _, name := range []string{"frameSkip", "callSite", "decorate", "log", "Log", "Logf", "Error", "Errorf", "Fatal", "Fatalf", "Skip", "Skipf"} {
		if fn == "testing.(*common)."+name {
			return true
		}
	}
	return false
}

// callSite returns the "file:line: " of the call of Log, Error, Fatal and so on in the test
// (frameSkip).
func (c *common) callSite() string {
	file, line := "???", 1
	if fr, ok := c.frameSkip(); ok {
		if fr.file != "" {
			file = fr.file
			if i := lastIndexByte(file, '/'); i >= 0 && !fullPath {
				file = file[i+1:]
			}
		}
		if fr.line != 0 {
			line = fr.line
		}
	}
	return fmt.Sprintf("%s:%d: ", file, line)
}

// decorate prefixes the string with the file and line of the call site
// and inserts the final newline if needed and indentation spaces for formatting.
// This function must be called with c.mu held.
func (c *common) decorate(s string) string {
	buf := "    " + c.callSite()
	lines := splitByte(s, '\n')
	if l := len(lines); l > 1 && lines[l-1] == "" {
		lines = lines[:l-1]
	}
	for i, line := range lines {
		if i > 0 {
			// Second and subsequent lines are indented an additional 4 spaces.
			buf += "\n        "
		}
		buf += line
	}
	return buf + "\n"
}

// flushToParent writes c.output to the parent after first writing the header
// with the given format and arguments.
func (c *common) flushToParent(testName, format string, args ...any) {
	libLock()
	defer libUnlock()
	p := c.parent
	text := fmt.Sprintf(format, args...)
	if len(c.output) > 0 {
		// Add the current c.output to the print,
		// and then arrange for the print to replace c.output.
		// (This displays the logged output after the --- FAIL line.)
		text += string(c.output)
		c.output = c.output[:0]
	}
	if c.chatty != nil {
		if _, direct := p.w.(stdoutWriter); direct {
			// We're flushing to the actual output, so track that this output is
			// associated with a specific test (and, specifically, that the next output
			// is *not* associated with that test).
			c.chatty.Updatef(testName, "%s", text)
			return
		}
	}
	// We're flushing to the output buffer of the parent test, which will
	// itself follow a test-name header when it is finally flushed to stdout.
	p.w.Write([]byte(text))
}

// flushPartial is Go's check for a partial log line; there is no Output writer here, so
// nothing is ever partial.
func (c *common) flushPartial() {}

// Name returns the name of the running (sub-) test or benchmark.
//
// The name will include the name of the test along with the names of
// any nested sub-tests. If two sibling sub-tests have the same name,
// Name will append a suffix to guarantee the returned name is unique.
func (c *common) Name() string {
	return c.name
}

// Fail marks the function as having failed but continues execution.
func (c *common) Fail() {
	if c.parent != nil {
		c.parent.Fail()
	}
	libLock()
	defer libUnlock()
	// c.done needs to be locked to synchronize checks to c.done in parent tests.
	if c.done {
		panic("Fail in goroutine after " + c.name + " has completed")
	}
	c.failed = true
}

// Failed reports whether the function has failed.
func (c *common) Failed() bool {
	libLock()
	failed := c.failed
	libUnlock()
	return failed
}

// FailNow marks the function as having failed and stops its execution
// by calling [runtime.Goexit] (which then runs all deferred calls in the
// current goroutine).
// Execution will continue at the next test or benchmark.
// FailNow must be called from the goroutine running the
// test or benchmark function, not from other goroutines
// created during the test. Calling FailNow does not stop
// those other goroutines.
func (c *common) FailNow() {
	c.Fail()

	// Calling runtime.Goexit will exit the goroutine, which
	// will run the deferred functions in this goroutine,
	// which will eventually run the deferred lines in tRunner,
	// which will signal to the test loop that this test is done.
	//
	// A previous version of this code said:
	//
	//	c.duration = ...
	//	c.report() // tell parent about test failure
	//	runtime.Goexit()
	//
	// That failed to mark the test as finished and
	// caused tRunner to call report which then double-counted
	// the test's failure.
	libLock()
	c.finished = true
	libUnlock()
	libGoexit()
}

// log generates the output. It is always at the same stack depth. log records the text
// in the error log and, in verbose mode, prints it as it comes.
func (c *common) log(s string) {
	libLock()
	defer libUnlock()
	if c.done {
		// This test has already finished. Try and log this message
		// with our parent. If we don't have a parent, panic.
		for parent := c.parent; parent != nil; parent = parent.parent {
			if !parent.done {
				parent.output = append(parent.output, parent.decorate(s)...)
				return
			}
		}
		panic("Log in goroutine after " + c.name + " has completed: " + s)
	}
	if c.chatty != nil {
		if c.bench {
			// Benchmarks don't print === CONT, so we should skip the test
			// printer and just print straight to stdout.
			fmt.Print(c.decorate(s))
		} else {
			c.chatty.Printf(c.name, "%s", c.decorate(s))
		}
		return
	}
	c.output = append(c.output, c.decorate(s)...)
}

// Log formats its arguments using default formatting, analogous to [fmt.Println],
// and records the text in the error log. For tests, the text will be printed only if
// the test fails or the -test.v flag is set. For benchmarks, the text is always
// printed to avoid having performance depend on the value of the -test.v flag.
// It is an error to call Log after a test or benchmark returns.
func (c *common) Log(args ...any) {
	c.log(fmt.Sprintln(args...))
}

// Logf formats its arguments according to the format, analogous to [fmt.Printf], and
// records the text in the error log. A final newline is added if not provided. For
// tests, the text will be printed only if the test fails or the -test.v flag is
// set. For benchmarks, the text is always printed to avoid having performance
// depend on the value of the -test.v flag.
// It is an error to call Logf after a test or benchmark returns.
func (c *common) Logf(format string, args ...any) {
	c.log(fmt.Sprintf(format, args...))
}

// Error is equivalent to Log followed by Fail.
func (c *common) Error(args ...any) {
	c.log(fmt.Sprintln(args...))
	c.Fail()
}

// Errorf is equivalent to Logf followed by Fail.
func (c *common) Errorf(format string, args ...any) {
	c.log(fmt.Sprintf(format, args...))
	c.Fail()
}

// Fatal is equivalent to Log followed by FailNow.
func (c *common) Fatal(args ...any) {
	c.log(fmt.Sprintln(args...))
	c.FailNow()
}

// Fatalf is equivalent to Logf followed by FailNow.
func (c *common) Fatalf(format string, args ...any) {
	c.log(fmt.Sprintf(format, args...))
	c.FailNow()
}

// Skip is equivalent to Log followed by SkipNow.
func (c *common) Skip(args ...any) {
	c.log(fmt.Sprintln(args...))
	c.SkipNow()
}

// Skipf is equivalent to Logf followed by SkipNow.
func (c *common) Skipf(format string, args ...any) {
	c.log(fmt.Sprintf(format, args...))
	c.SkipNow()
}

// SkipNow marks the test as having been skipped and stops its execution
// by calling [runtime.Goexit].
// If a test fails (see Error, Errorf, Fail) and is then skipped,
// it is still considered to have failed.
// Execution will continue at the next test or benchmark. See also FailNow.
// SkipNow must be called from the goroutine running the test, not from
// other goroutines created during the test. Calling SkipNow does not stop
// those other goroutines.
func (c *common) SkipNow() {
	libLock()
	c.skipped = true
	c.finished = true
	libUnlock()
	libGoexit()
}

// Skipped reports whether the test was skipped.
func (c *common) Skipped() bool {
	libLock()
	skipped := c.skipped
	libUnlock()
	return skipped
}

// Helper marks the calling function as a test helper function.
// When printing file and line information, that function will be skipped.
// Helper may be called simultaneously from multiple goroutines.
func (c *common) Helper() {
	libLock()
	defer libUnlock()
	if c.helperNames == nil {
		c.helperNames = make(map[string]bool)
	}
	// The function that called Helper (or F.Helper, B.Helper, ... which call this one): the first
	// frame that is not one of the Helper methods. If that is a function of package testing (F.Fuzz),
	// it is marked as well, which changes nothing: the call site skips package testing anyway.
	for _, f := range callers() {
		if !inTesting(f.fn) || !(f.fn == "testing.callers" || hasSuffix(f.fn, ".Helper")) {
			c.helperNames[f.fn] = true
			return
		}
	}
}

// Cleanup registers a function to be called when the test (or subtest) and all its
// subtests complete. Cleanup functions will be called in last added,
// first called order.
func (c *common) Cleanup(f func()) {
	// The stack at this point: the cleanup function is a helper when it calls Helper, and its
	// messages are then reported where Cleanup was called (frameSkip).
	cleanupPc := userFrames(callers())

	fn := func() {
		defer func() {
			libLock()
			defer libUnlock()
			c.cleanupName = ""
			c.cleanupPc = nil
		}()

		// The name of this closure, which is on the stack above f.
		name := ""
		if fs := callers(); len(fs) > 1 {
			name = fs[1].fn // fs[0] is callers
		}
		libLock()
		c.cleanupName = name
		c.cleanupPc = cleanupPc
		libUnlock()

		f()
	}

	libLock()
	defer libUnlock()
	c.cleanups = append(c.cleanups, fn)
}

// TempDir returns a temporary directory for the test to use.
// The directory is automatically removed when the test and
// all its subtests complete.
// Each subsequent call to TempDir returns a unique directory;
// if the directory creation fails, TempDir terminates the test by calling Fatal.
func (c *common) TempDir() string {
	// Use a single parent directory for all the temporary directories
	// created by a test, each numbered sequentially.
	libLock()
	if c.tempDir == "" {
		pattern := c.Name()
		// Limit length of file names on disk.
		if len(pattern) > 64 {
			pattern = pattern[:64]
		}
		// Drop unusual characters (such as path separators or
		// characters interacting with globs) from the directory name to
		// avoid surprising os.MkdirTemp behavior.
		var kept []byte
		for i := 0; i < len(pattern); i++ {
			r := pattern[i]
			if '0' <= r && r <= '9' || 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' ||
				indexByte("!#$%&()+,-.=@^_{}~ ", r) >= 0 || r >= 0x80 {
				kept = append(kept, r)
			}
		}
		dir, errText := libMakeTempDir(string(kept))
		c.tempDir, c.tempDirErr = dir, errText
		if errText == "" {
			c.cleanups = append(c.cleanups, func() {
				if err := libRemoveAll(dir); err != "" {
					c.Errorf("TempDir RemoveAll cleanup: %v", err)
				}
			})
		}
	}
	if c.tempDirErr == "" {
		c.tempDirSeq++
	}
	seq := c.tempDirSeq
	errText := c.tempDirErr
	dir := c.tempDir
	libUnlock()

	if errText != "" {
		c.Helper()
		c.Fatalf("TempDir: %v", errText)
	}
	sep := "/"
	if libGOOS() == "windows" {
		sep = "\\"
	}
	sub := fmt.Sprintf("%s%s%03d", dir, sep, seq)
	if err := libMakeDir(sub); err != "" {
		c.Helper()
		c.Fatalf("TempDir: %v", err)
	}
	return sub
}

// Setenv calls [os.Setenv] and uses Cleanup to
// restore the environment variable to its original value
// after the test.
//
// Because Setenv affects the whole process, it cannot be used
// in parallel tests or tests with parallel ancestors.
func (c *common) Setenv(key, value string) {
	// The environment is os's (syscall keeps a copy that os.Getenv reads), so this calls os and
	// not the C++ environment directly: the code under test sees the change.
	prevValue, ok := os.LookupEnv(key)

	if err := os.Setenv(key, value); err != nil {
		c.Helper()
		c.Fatalf("cannot set environment variable: %v", err)
	}

	if ok {
		c.Cleanup(func() {
			os.Setenv(key, prevValue)
		})
	} else {
		c.Cleanup(func() {
			os.Unsetenv(key)
		})
	}
}

// Chdir calls [os.Chdir] and uses Cleanup to restore the current
// working directory to its original value after the test. On Unix, it
// also sets PWD environment variable for the duration of the test.
//
// Because Chdir affects the whole process, it cannot be used
// in parallel tests or tests with parallel ancestors.
func (c *common) Chdir(dir string) {
	oldwd, errText := libGetwd()
	if errText != "" {
		c.Helper()
		c.Fatal(errText)
	}
	if err := libChdir(dir); err != "" {
		c.Helper()
		c.Fatal(err)
	}
	// On POSIX platforms, PWD represents “an absolute pathname of the
	// current working directory.” Since we are changing the working
	// directory, we should also set or update PWD to reflect that.
	// (Windows and Plan 9 do not use the PWD variable.)
	if libGOOS() != "windows" {
		if wd, errText := libGetwd(); errText == "" {
			c.Setenv("PWD", wd)
		}
	}
	c.Cleanup(func() {
		if err := libChdir(oldwd); err != "" {
			// It's not safe to continue with tests if we can't
			// get back to the original working directory.
			panic("testing.Chdir: " + err)
		}
	})
}

// Attr emits a test attribute associated with this test.
//
// The key must not contain whitespace.
// The value must not contain newlines or carriage returns.
//
// The meaning of different attribute keys is left up to
// continuous integration systems and test frameworks.
//
// Test attributes are emitted immediately in the test log,
// but they are intended to be treated as unordered.
func (c *common) Attr(key, value string) {
	for i := 0; i < len(key); i++ {
		if isSpaceByte(key[i]) {
			c.Errorf("disallowed whitespace in attribute key %q", key)
			return
		}
	}
	if indexByte(value, '\r') >= 0 || indexByte(value, '\n') >= 0 {
		c.Errorf("disallowed newline in attribute value %q", value)
		return
	}
	if c.chatty == nil {
		return
	}
	c.chatty.Updatef(c.name, "=== ATTR  %s %v %v\n", c.name, key, value)
}

// panicHandling controls the panic handling used by runCleanup.
type panicHandling int

const (
	normalPanic panicHandling = iota
	recoverAndReturnPanic
)

// runCleanup is called at the end of the test.
// If ph is recoverAndReturnPanic, it will catch panics, and return the
// recovered value if any.
func (c *common) runCleanup(ph panicHandling) (panicVal any) {
	libLock()
	c.cleanupStarted = true
	libUnlock()
	defer func() {
		libLock()
		c.cleanupStarted = false
		libUnlock()
	}()

	if ph == recoverAndReturnPanic {
		defer func() {
			panicVal = recover()
		}()
	}

	// Make sure that if a cleanup function panics,
	// we still run the remaining cleanup functions.
	defer func() {
		libLock()
		recur := len(c.cleanups) > 0
		libUnlock()
		if recur {
			c.runCleanup(normalPanic)
		}
	}()

	for {
		var cleanup func()
		libLock()
		if len(c.cleanups) > 0 {
			last := len(c.cleanups) - 1
			cleanup = c.cleanups[last]
			c.cleanups[last] = nil // the closure usually holds the test: let go of the cycle
			c.cleanups = c.cleanups[:last]
		}
		libUnlock()
		if cleanup == nil {
			return nil
		}
		cleanup()
	}
}

// private marks the types that are a TB: only T, B and F.
func (c *common) private() {}

// TB is the interface common to [T], [B], and [F].
//
// A TB is a T, B or F: the methods of the testing types that tests, benchmarks and fuzz
// targets have in common. Because it has an unexported method, only the types of this package
// implement it.
type TB interface {
	Attr(key, value string)
	Cleanup(func())
	Error(args ...any)
	Errorf(format string, args ...any)
	Fail()
	FailNow()
	Failed() bool
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Helper()
	Log(args ...any)
	Logf(format string, args ...any)
	Name() string
	Setenv(key, value string)
	Chdir(dir string)
	Skip(args ...any)
	SkipNow()
	Skipf(format string, args ...any)
	Skipped() bool
	TempDir() string

	// A private method to prevent users implementing the
	// interface and so future additions to it will not
	// violate Go 1 compatibility.
	private()
}

var (
	_ TB = (*T)(nil)
	_ TB = (*B)(nil)
)

// T is a type passed to Test functions to manage test state and support formatted test logs.
//
// A test ends when its Test function returns or calls any of the methods
// FailNow, Fatal, Fatalf, SkipNow, Skip, or Skipf. Those methods, as well as
// the Parallel method, must be called only from the goroutine running the
// Test function.
//
// The other reporting methods, such as the variations of Log and Error,
// may be called simultaneously from multiple goroutines.
type T struct {
	common
	tstate *testState // For running tests and subtests.
}

const parallelConflict = `testing: test using t.Setenv or t.Chdir can not use t.Parallel`

// Parallel signals that this test is to be run in parallel with (and only with)
// other parallel tests. When a test is run multiple times due to use of
// -test.count or -test.cpu, multiple instances of a single test never run in
// parallel with each other.
func (t *T) Parallel() {
	if t.isParallel {
		panic("testing: t.Parallel called multiple times")
	}
	if t.denyParallel {
		panic(parallelConflict)
	}
	if t.parent.barrier == nil {
		return
	}

	t.isParallel = true

	// We don't want to include the time we spend waiting for serial tests
	// in the test duration. Record the elapsed time thus far and reset the
	// timer afterwards.
	t.duration += libNow() - t.start

	// Add to the list of tests to be released by the parent.
	t.resume, t.resumed, t.ended = make(chan bool, 1), make(chan bool, 1), make(chan bool)
	libLock()
	t.parent.sub = append(t.parent.sub, t)
	libUnlock()

	if t.chatty != nil {
		t.chatty.Updatef(t.name, "=== PAUSE %s\n", t.name)
	}
	libRunningDelete(t.name)

	t.signal <- true // Release calling test.
	// Wait for the parent test to complete and release this one. Go closes one channel for all the
	// parallel subtests and its scheduler starts them in order; here the goroutines are threads,
	// so the parent releases them one by one, in the order they called Parallel (tRunner), and a
	// test that has to wait for a slot waits in the first-come order of testState.
	<-t.resume

	if t.chatty != nil {
		t.chatty.Updatef(t.name, "=== CONT  %s\n", t.name)
	}
	libRunningAdd(t.name, libNow())
	t.start = libNow()
	t.resumed <- true
}

func (t *T) checkParallel() {
	// Non-parallel subtests that have parallel ancestors may still
	// run in parallel with other tests: they are only non-parallel
	// with respect to the other subtests of the same parent.
	// Since calls like SetEnv or Chdir affects the whole process, we need
	// to deny those if the current test or any parent is parallel.
	for c := &t.common; c != nil; c = c.parent {
		if c.isParallel {
			panic(parallelConflict)
		}
	}

	t.denyParallel = true
}

// Setenv calls os.Setenv(key, value) and uses Cleanup to
// restore the environment variable to its original value
// after the test.
//
// Because Setenv affects the whole process, it cannot be used
// in parallel tests or tests with parallel ancestors.
func (t *T) Setenv(key, value string) {
	t.checkParallel()
	t.common.Setenv(key, value)
}

// Chdir calls os.Chdir(dir) and uses Cleanup to restore the current
// working directory to its original value after the test. On Unix, it
// also sets PWD environment variable for the duration of the test.
//
// Because Chdir affects the whole process, it cannot be used
// in parallel tests or tests with parallel ancestors.
func (t *T) Chdir(dir string) {
	t.checkParallel()
	t.common.Chdir(dir)
}

// Deadline reports the time at which the test binary will have exceeded the timeout specified by
// the -timeout flag.
//
// The ok result is false if the -timeout flag indicates “no timeout” (-timeout 0).
func (t *T) Deadline() (deadline time.Time, ok bool) {
	if t.tstate.deadline == 0 {
		return time.Time{}, false
	}
	// The deadline is a reading of the test clock (libNow); time.Now has the wall clock.
	return time.Now().Add(time.Duration(t.tstate.deadline - libNow())), true
}

// InternalTest is an internal type but exported because it is cross-package;
// it is part of the implementation of the "gocpp test" command.
type InternalTest struct {
	Name string
	F    func(*T)
}

var errNilPanicOrGoexit = errors.New("test executed panic(nil) or runtime.Goexit")

func tRunner(t *T, fn func(t *T)) {
	// When this goroutine is done, either because fn(t)
	// returned normally or because a test failure triggered
	// a call to runtime.Goexit, record the duration and send
	// a signal saying that the test is done.
	defer func() {
		if t.Failed() {
			libLock()
			numFailed++
			libUnlock()
		}

		// Check if the test panicked or Goexited inappropriately.
		//
		// If this happens in a normal test, print output but continue panicking.
		// tRunner is called in its own goroutine, so this terminates the process.
		err := recover()
		signal := true

		libLock()
		finished := t.finished
		libUnlock()
		if !finished && err == nil {
			err = errNilPanicOrGoexit
			for p := t.parent; p != nil; p = p.parent {
				libLock()
				finished = p.finished
				libUnlock()
				if finished {
					if !t.isParallel {
						t.Errorf("%v: subtest may have called FailNow on a parent test", err)
						err = nil
					}
					signal = false
					break
				}
			}
		}

		// Use a deferred call to ensure that we report that the test is
		// complete even if a cleanup function calls t.FailNow. See issue 41355.
		didPanic := false
		defer func() {
			// Only report that the test is complete if it doesn't panic,
			// as otherwise the test binary can exit before the panic is
			// reported to the user. See issue 41479.
			if didPanic {
				return
			}
			if err != nil {
				panic(err)
			}
			libRunningDelete(t.name)
			if t.ended != nil {
				close(t.ended)
			}
			t.signal <- signal
		}()

		doPanic := func(err any) {
			t.Fail()
			if r := t.runCleanup(recoverAndReturnPanic); r != nil {
				t.Logf("cleanup panicked with %v", r)
			}
			// Flush the output log up to the root before dying.
			for root := &t.common; root.parent != nil; root = root.parent {
				libLock()
				root.duration += libNow() - root.start
				d := root.duration
				libUnlock()
				root.flushToParent(root.name, "--- FAIL: %s (%s)\n", root.name, fmtDuration(d))
				if r := root.parent.runCleanup(recoverAndReturnPanic); r != nil {
					root.parent.w.Write([]byte(fmt.Sprintf("cleanup panicked with %v", r)))
				}
			}
			didPanic = true
			panic(err)
		}
		if err != nil {
			doPanic(err)
		}

		t.duration += libNow() - t.start

		if len(t.sub) > 0 {
			// Run parallel subtests.

			// Decrease the running count for this test and mark it as no longer running.
			t.tstate.release()
			libRunningDelete(t.name)

			// Release the parallel subtests, one at a time in the order they paused: each
			// one that gets a slot says when it runs again, so that the "=== CONT" lines and the
			// first output of the tests come in that order (Go's order, with its scheduler).
			t.tstate.releaseParallel(t.sub)
			// Wait for subtests to complete.
			for _, sub := range t.sub {
				<-sub.signal
			}

			// Run any cleanup callbacks, marking the test as running
			// in case the cleanup hangs.
			cleanupStart := libNow()
			libRunningAdd(t.name, cleanupStart)
			err := t.runCleanup(recoverAndReturnPanic)
			t.duration += libNow() - cleanupStart
			if err != nil {
				doPanic(err)
			}
			if !t.isParallel {
				// Reacquire the count for sequential tests. See comment in Run.
				t.tstate.waitParallel()
			}
		} else if t.isParallel {
			// Only release the count for this test if it was run as a parallel
			// test. See comment in Run method.
			t.tstate.release()
		}
		// Output buffered logs.
		t.report() // Report after all subtests have finished.

		// Do not lock t.done to allow race detector to detect race in case
		// the user does not appropriately synchronize a goroutine.
		libLock()
		t.done = true
		if t.parent != nil && !t.hasSub {
			t.setRan()
		}
		// Let go of the references that make a cycle of this test with its writer and its parallel
		// subtests (memory is reference counted); a late Log goes to the parent (log).
		t.w = nil
		t.sub = nil
		libUnlock()
	}()
	defer func() {
		if len(t.sub) == 0 {
			t.runCleanup(normalPanic)
		}
	}()

	t.start = libNow()
	t.runner = "testing.tRunner"
	fn(t)

	// code beyond here will not be executed when FailNow is invoked
	libLock()
	t.finished = true
	libUnlock()
}

// setRan marks c and its parents as having run. It must be called with the lock held.
func (c *common) setRan() {
	if c.parent != nil {
		c.parent.setRan()
	}
	c.ran = true
}

// Run runs f as a subtest of t called name. It runs f in a separate goroutine
// and blocks until f returns or calls t.Parallel to become a parallel test.
// Run reports whether f succeeded (or at least did not fail before calling t.Parallel).
//
// Run may be called simultaneously from multiple goroutines, but all such calls
// must return before the outer test function for t returns.
func (t *T) Run(name string, f func(t *T)) bool {
	if t.cleanupStarted {
		panic("testing: t.Run called during t.Cleanup")
	}

	libLock()
	t.hasSub = true
	libUnlock()
	testName, ok, _ := t.tstate.match.fullName(&t.common, name)
	if !ok || shouldFailFast() {
		return true
	}

	sub := &T{
		common: common{
			barrier: make(chan bool),
			signal:  make(chan bool, 1),
			name:    testName,
			parent:  &t.common,
			level:   t.level + 1,
			creator: userFrames(callers()), // the stack at this call of Run, for frameSkip
			chatty:  t.chatty,
		},
		tstate: t.tstate,
	}
	sub.w = indenter{&sub.common}

	if sub.chatty != nil {
		sub.chatty.Updatef(sub.name, "=== RUN   %s\n", sub.name)
	}
	libRunningAdd(sub.name, libNow())

	// Instead of reducing the running count of this test before calling the
	// tRunner and increasing it afterwards, we rely on tRunner keeping the
	// count correct. This ensures that a sequence of sequential tests runs
	// without being preempted, even when their parent is a parallel test. This
	// may especially reduce surprises if *parallel == 1.
	go tRunner(sub, f)

	// The parent goroutine will block until the subtest either finishes or calls
	// Parallel, but in general we don't know whether the parent goroutine is the
	// top-level test function or some other goroutine it has spawned.
	if !<-sub.signal {
		// At this point, it is likely that FailNow was called on one of the
		// parent tests by one of the subtests. Continue aborting up the chain.
		libGoexit()
	}
	return !sub.failed
}

// testState holds all fields that are common to all tests. This includes
// synchronization primitives to run at most *parallel tests.
type testState struct {
	match *matcher

	// deadline is the time (libNow) at which the test binary exceeds its timeout, or 0.
	deadline int64

	// waiters are the tests that are ready to be run in parallel and wait for a slot, first come
	// first served (a channel to send true to when the test gets the slot).
	waiters []chan bool

	// running is the number of tests currently running in parallel.
	// This does not include tests that are waiting for subtests to complete.
	running int

	// maxParallel is a copy of the parallel flag.
	maxParallel int
}

func newTestState(maxParallel int, m *matcher) *testState {
	return &testState{
		match:       m,
		maxParallel: maxParallel,
		running:     1, // Set the count to 1 for the main (sequential) test.
	}
}

// acquire takes a slot for a test and reports true, or, when all the slots are taken, queues ch and
// reports false: the slot is taken for the test when ch gets true (release).
func (s *testState) acquire(ch chan bool) bool {
	libLock()
	defer libUnlock()
	if s.running < s.maxParallel {
		s.running++
		return true
	}
	s.waiters = append(s.waiters, ch)
	return false
}

// parallelGrace is how long (ns) releaseParallel waits for a test that it has just started to end.
const parallelGrace = 300000

// releaseParallel starts the parallel tests subs (the subtests of one test that called Parallel),
// one at a time in the order they paused. Go starts them all at once and its scheduler runs them
// in that order, so that the output of tests that are done in a moment is in the order of the
// tests; here the goroutines are threads, which would finish in any order. A test that takes
// longer than parallelGrace does not hold the others back.
func (s *testState) releaseParallel(subs []*T) {
	for _, sub := range subs {
		if !s.acquire(sub.resume) {
			continue // it starts when a running test ends (release)
		}
		sub.resume <- true
		<-sub.resumed
		for waited := int64(0); waited < parallelGrace; waited += 20000 {
			select {
			case <-sub.ended:
				waited = parallelGrace
			default:
				libSleep(20000)
			}
		}
	}
}

func (s *testState) waitParallel() {
	ch := make(chan bool, 1)
	if s.acquire(ch) {
		return
	}
	<-ch
}

func (s *testState) release() {
	libLock()
	if len(s.waiters) == 0 {
		s.running--
		libUnlock()
		return
	}
	ch := s.waiters[0] // pick the test that waited longest; it takes the slot of this one
	s.waiters = s.waiters[1:]
	libUnlock()
	ch <- true
}

func (t *T) report() {
	if t.parent == nil {
		return
	}
	dstr := fmtDuration(t.duration)
	format := "--- %s: %s (%s)\n"
	if t.Failed() {
		t.flushToParent(t.name, format, "FAIL", t.name, dstr)
	} else if t.chatty != nil {
		if t.Skipped() {
			t.flushToParent(t.name, format, "SKIP", t.name, dstr)
		} else {
			t.flushToParent(t.name, format, "PASS", t.name, dstr)
		}
	}
}

func shouldFailFast() bool {
	libLock()
	defer libUnlock()
	return failFast && numFailed > 0
}

// ---------------------------------------------------------------------------------------------
// M, Main, RunTests.

// matchStringOnly adapts a match function (the argument of Main and RunTests) to testDeps.
type matchStringOnly func(pat, str string) (bool, error)

func (f matchStringOnly) MatchString(pat, str string) (bool, error) { return f(pat, str) }
func (f matchStringOnly) ImportPath() string                        { return "" }
func (f matchStringOnly) SetExitCode(code int)                      {}

// Main is an internal function, part of the implementation of the "gocpp test" command.
// It was exported because it is cross-package and predates "internal" packages.
// It is no longer used by "gocpp test" but preserved for systems that simulate "go test"
// using Main.
func Main(matchString func(pat, str string) (bool, error), tests []InternalTest, benchmarks []InternalBenchmark, examples []InternalExample) {
	libExit(MainStart(matchStringOnly(matchString), tests, benchmarks, nil, examples).Run())
}

// M is a type passed to a TestMain function to run the actual tests.
type M struct {
	deps        testDeps
	tests       []InternalTest
	benchmarks  []InternalBenchmark
	fuzzTargets []InternalFuzzTarget
	examples    []InternalExample

	numRun int

	// value to pass to os.Exit, the outer test func main
	// harness calls os.Exit with this code. See #34129.
	exitCode int
}

// testDeps is an internal interface of functionality that is
// passed into this package by a test's generated main package.
// The canonical implementation of this interface is
// internal/testdeps's TestDeps.
type testDeps interface {
	ImportPath() string
	MatchString(pat, str string) (bool, error)
	SetExitCode(code int)
}

// MainStart is meant for use by tests generated by 'gocpp test'.
// It is not meant to be called directly and is not subject to the Go 1 compatibility document.
// It may change signature from release to release.
func MainStart(deps testDeps, tests []InternalTest, benchmarks []InternalBenchmark, fuzzTargets []InternalFuzzTarget, examples []InternalExample) *M {
	testBinary = true
	Init()
	return &M{
		deps:        deps,
		tests:       tests,
		benchmarks:  benchmarks,
		fuzzTargets: fuzzTargets,
		examples:    examples,
	}
}

// Run runs the tests. It returns an exit code to pass to os.Exit.
// The exit code is zero when all tests pass, and non-zero for any kind
// of failure.
func (m *M) Run() (code int) {
	defer func() {
		code = m.exitCode
		m.deps.SetExitCode(code)
	}()

	// Count the number of calls to m.Run.
	// We only ever expected 1, but we didn't enforce that,
	// and now there are tests in the wild that call m.Run multiple times.
	// Sigh. go.dev/issue/23129.
	m.numRun++

	parseFlags()

	if parallelism < 1 {
		fmt.Fprintln(os.Stderr, "testing: -parallel can only be given a positive integer")
		m.exitCode = 2
		return
	}

	if matchList != "" {
		listTests(m.deps.MatchString, m.tests, m.benchmarks, m.fuzzTargets, m.examples)
		m.exitCode = 0
		return
	}

	deadline := startAlarm()
	haveExamples = len(m.examples) > 0
	testRan, testOk := runTests(m.deps.MatchString, m.tests, deadline)
	fuzzRan, fuzzOk := runFuzzTests(m.deps, m.fuzzTargets, deadline)
	exampleRan, exampleOk := runExamples(m.deps.MatchString, m.examples)
	stopAlarm()
	if !testRan && !exampleRan && !fuzzRan && matchBench == "" {
		fmt.Fprintln(os.Stderr, "testing: warning: no tests to run")
	}
	anyFailed := !testOk || !fuzzOk || !exampleOk || !runBenchmarks(m.deps.ImportPath(), m.deps.MatchString, m.benchmarks)
	if anyFailed {
		fmt.Print("FAIL\n")
		m.exitCode = 1
		return
	}

	m.exitCode = 0
	fmt.Print("PASS\n")
	return
}

func listTests(matchString func(pat, str string) (bool, error), tests []InternalTest, benchmarks []InternalBenchmark, fuzzTargets []InternalFuzzTarget, examples []InternalExample) {
	if _, err := matchString(matchList, "non-empty"); err != nil {
		fmt.Fprintf(os.Stderr, "testing: invalid regexp in -test.list (%q): %s\n", matchList, err)
		libExit(1)
	}

	for _, test := range tests {
		if ok, _ := matchString(matchList, test.Name); ok {
			fmt.Println(test.Name)
		}
	}
	for _, bench := range benchmarks {
		if ok, _ := matchString(matchList, bench.Name); ok {
			fmt.Println(bench.Name)
		}
	}
	for _, fuzzTarget := range fuzzTargets {
		if ok, _ := matchString(matchList, fuzzTarget.Name); ok {
			fmt.Println(fuzzTarget.Name)
		}
	}
	for _, example := range examples {
		if ok, _ := matchString(matchList, example.Name); ok {
			fmt.Println(example.Name)
		}
	}
}

// RunTests is an internal function but exported because it is cross-package;
// it is part of the implementation of the "gocpp test" command.
func RunTests(matchString func(pat, str string) (bool, error), tests []InternalTest) (ok bool) {
	parseFlags()
	var deadline int64
	if timeoutNs > 0 {
		deadline = libNow() + timeoutNs
	}
	ran, ok := runTests(matchString, tests, deadline)
	if !ran && !haveExamples {
		fmt.Fprintln(os.Stderr, "testing: warning: no tests to run")
	}
	return ok
}

func runTests(matchString func(pat, str string) (bool, error), tests []InternalTest, deadline int64) (ran, ok bool) {
	ok = true
	for range cpuList {
		for i := 0; i < testCount; i++ {
			if shouldFailFast() {
				break
			}
			if i > 0 && !ran {
				// There were no tests to run on the first
				// iteration. This won't change, so no reason
				// to keep trying.
				break
			}
			tstate := newTestState(parallelism, newMatcher(matchString, matchPattern, "-test.run", skipPattern))
			tstate.deadline = deadline
			t := &T{
				common: common{
					signal:  make(chan bool, 1),
					barrier: make(chan bool),
					w:       stdoutWriter{},
				},
				tstate: tstate,
			}
			if Verbose() {
				t.chatty = chatty
			}
			tRunner(t, func(t *T) {
				for _, test := range tests {
					t.Run(test.Name, test.F)
				}
			})
			select {
			case <-t.signal:
			default:
				panic("internal error: tRunner exited without sending on t.signal")
			}
			ok = ok && !t.Failed()
			ran = ran || t.ran
		}
	}
	return ran, ok
}

// startAlarm starts the timer of -test.timeout, if there is one, and returns the deadline.
func startAlarm() int64 {
	if timeoutNs <= 0 {
		return 0
	}
	deadline := libNow() + timeoutNs
	timeout := timeoutNs
	go func() {
		if !libAlarmWait(timeout) {
			return
		}

		// As Go's testing does: the panic below prints every goroutine, not only this one.
		debug.SetTraceback("all")
		extra := ""
		var list []string
		for _, ln := range splitByte(libRunningList(), '\n') {
			if ln == "" {
				continue
			}
			parts := splitByte(ln, '\t')
			start, _ := atoi(parts[1])
			elapsed := libNow() - int64(start)
			list = append(list, fmt.Sprintf("%s (%s)", parts[0], durationString((elapsed+500000000)/1000000000*1000000000)))
		}
		if len(list) > 0 {
			extra = "\nrunning tests:"
			for _, name := range list {
				extra += "\n\t" + name
			}
		}
		panic(fmt.Sprintf("test timed out after %s%s", durationString(timeout), extra))
	}()
	return deadline
}

// stopAlarm turns off the alarm.
func stopAlarm() {
	if timeoutNs > 0 {
		libAlarmCancel()
	}
}
