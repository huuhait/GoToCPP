package stdcount

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	// maxRounds bounds the runs of one test binary: each round after the first skips the tests that
	// have a result, and a round ends early only when it found a test that took the binary down.
	maxRounds = 40

	// hardTimeoutGrace is how long Run waits, after the -test.timeout of the test binary, for the
	// binary to panic with its list of running tests before it kills the binary and its process
	// group. A binary stuck in a loop that does not return to the scheduler, or in a lock held
	// across the timer, never reports its own timeout.
	hardTimeoutGrace = 30 * time.Second

	// processExitDelay is how long Wait waits for the output pipes to close after the test binary
	// has been killed: a process of its group may still hold them.
	processExitDelay = 5 * time.Second
)

// RunOptions tells Run what to build and where.
type RunOptions struct {
	Gocpp   string        // the gocpp binary
	StdGo   string        // tests/stdgo
	Work    string        // a folder for the test binary and the caches
	Timeout time.Duration // of one run of the test binary (default 5m)
	Grace   time.Duration // more time before the binary is killed (default 30s)
	Env     []string      // more environment
	Stderr  io.Writer
}

// Run builds the tests of the package and runs them, again with the test that crashed or hung
// skipped, until every test function has a result or the binary ends normally. It returns the
// results and the log of the runs.
func Run(ctx context.Context, opt RunOptions, m *Manifest) (map[string]*Result, string, error) {
	if opt.Timeout == 0 {
		opt.Timeout = 5 * time.Minute
	}
	if opt.Grace == 0 {
		opt.Grace = hardTimeoutGrace
	}
	stdgo, err := filepath.Abs(opt.StdGo)
	if err != nil {
		return nil, "", err
	}
	env := runEnvironment(opt, stdgo)
	if err := os.MkdirAll(opt.Work, 0o755); err != nil {
		return nil, "", err
	}

	bin := filepath.Join(opt.Work, strings.ReplaceAll(m.Package, "/", "_")+".test")
	if out, err := buildTests(ctx, opt, env, m.Package, bin); err != nil {
		return nil, out, err
	}
	defer os.Remove(bin)

	dir := filepath.Join(stdgo, filepath.FromSlash(m.Package))
	round := func(skip []string) roundResult {
		args := testArguments(opt.Timeout, m.Args, skip)
		return execRound(ctx, bin, args, dir, env, opt.Timeout+opt.Grace)
	}
	results, log := runRounds(ctx, round)
	return results, log, nil
}

// runEnvironment returns the environment of the build and of the test binary: the converted tests
// are the overlay of the standard library, and nothing reaches for the network or the user's
// caches.
func runEnvironment(opt RunOptions, stdgo string) []string {
	env := append(os.Environ(),
		"GOCPP_TEST_STDLIB="+stdgo,
		"GOCPP_MODCACHE=off",
		"GOCPP_OFFLINE=1",
	)
	if os.Getenv("XDG_CACHE_HOME") == "" {
		env = append(env, "XDG_CACHE_HOME="+filepath.Join(opt.Work, "cache"))
	}
	return append(env, opt.Env...)
}

// buildTests runs gocpp test -c for the package.
func buildTests(
	ctx context.Context, opt RunOptions, env []string, pkg, bin string,
) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	build := commandInGroup(ctx, opt.Gocpp, "test", "-c", "-o", bin, pkg)
	build.Env = env
	build.Dir = opt.Work
	out, err := build.CombinedOutput()
	killGroup(build)
	if err != nil {
		return string(out), fmt.Errorf("gocpp test -c %s: %v\n%s", pkg, err, out)
	}
	return "", nil
}

// testArguments returns the command line of one run: the tests that have a result do not run again.
func testArguments(timeout time.Duration, extra, skip []string) []string {
	args := []string{"-test.v", "-test.paniconexit0", "-test.timeout=" + timeout.String()}
	args = append(args, extra...)
	if len(skip) > 0 {
		args = append(args, "-test.skip=^("+strings.Join(quoteAll(skip), "|")+")$")
	}
	return args
}

// A roundResult is what one run of the test binary left behind.
type roundResult struct {
	Out         []byte
	Err         error
	HardTimeout bool // Run killed the binary because it did not end by itself in time
}

// execRound runs the test binary once and kills it, with everything it started, after the hard
// limit.
func execRound(
	ctx context.Context, bin string, args []string, dir string, env []string,
	hardLimit time.Duration,
) roundResult {
	// The parent death signal of the child (Pdeathsig) fires when the thread that started it exits,
	// not when this process does, and the Go runtime may end an idle thread. A locked thread is not
	// ended while the goroutine runs, so the child lives exactly as long as the round it belongs
	// to.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	roundCtx, cancel := context.WithTimeout(ctx, hardLimit)
	defer cancel()

	var out bytes.Buffer
	run := commandInGroup(roundCtx, bin, args...)
	run.Env = env
	run.Dir = dir
	run.Stdout = &out
	run.Stderr = &out
	err := run.Run()
	killGroup(run)

	expired := err != nil && ctx.Err() == nil && roundCtx.Err() == context.DeadlineExceeded
	return roundResult{Out: out.Bytes(), Err: err, HardTimeout: expired}
}

// commandInGroup returns a command in a process group of its own: when the context ends, the whole
// group is killed, and Wait does not wait for pipes that a process of the group still holds.
func commandInGroup(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	setProcessGroup(cmd)
	cmd.Cancel = func() error {
		killGroup(cmd)
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = processExitDelay
	return cmd
}

// runRounds runs the test binary until every test has a result or it ends normally. The run
// function takes the names of the tests that have a result and returns what one more run left
// behind.
func runRounds(
	ctx context.Context, run func(skip []string) roundResult,
) (map[string]*Result, string) {
	var log strings.Builder
	all := map[string]*Result{}
	for round := 0; round < maxRounds && ctx.Err() == nil; round++ {
		res := run(sortedNames(all))
		fmt.Fprintf(&log, "=== round %d (%d tests done): %v\n%s\n",
			round, len(all), res.Err, res.Out)
		if res.HardTimeout {
			fmt.Fprintf(&log, "=== the test binary did not end in time and was killed\n")
		}

		results, running, last := Parse(res.Out)
		for name, r := range results {
			all[name] = r
		}
		if res.Err == nil {
			break
		}

		victims := findVictims(res, running, last)
		if len(victims) == 0 {
			break
		}
		why := "the test binary ended during this test:"
		if res.HardTimeout {
			why = "the test binary hung and was killed during this test:"
		}
		markVictims(all, victims, why, res.Out)
	}
	return all, log.String()
}

// findVictims returns the tests that took the binary down. They are the ones it reports as running
// at a timeout, or the last one that started and has no result (the others were running in parallel
// and run again) when the binary panicked, crashed or was killed after the hard limit, or the one
// whose FAIL line a panic follows (the panic of a test goroutine ends the binary).
func findVictims(res roundResult, running []string, last string) []string {
	victims := timedOut(res.Out)
	if len(victims) == 0 && len(running) > 0 {
		victims = []string{running[len(running)-1]}
	}
	if len(victims) == 0 && last != "" && bytes.Contains(res.Out, []byte("\npanic: ")) {
		victims = []string{last}
	}
	return victims
}

// markVictims gives each victim a failing result that carries the end of the output.
func markVictims(all map[string]*Result, victims []string, why string, out []byte) {
	tail := tailLines(out, 40)
	for _, victim := range victims {
		lines := []string{why}
		if old := all[victim]; old != nil {
			lines = append(lines, old.Lines...)
		}
		all[victim] = &Result{Status: "fail", Lines: append(lines, tail...)}
	}
}

func sortedNames(results map[string]*Result) []string {
	names := make([]string, 0, len(results))
	for name := range results {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// timedOut returns the tests that a timeout panic lists as running ("running tests:" and a line
// "\tTestX (5m0s)" each).
func timedOut(out []byte) []string {
	text := string(out)
	i := strings.Index(text, "panic: test timed out after")
	if i < 0 {
		return nil
	}
	var names []string
	rest := text[i:]
	j := strings.Index(rest, "running tests:\n")
	if j < 0 {
		return nil
	}
	for _, line := range strings.Split(rest[j+len("running tests:\n"):], "\n") {
		if !strings.HasPrefix(line, "\t") {
			break
		}
		fields := strings.Fields(line)
		if len(fields) > 0 && !strings.Contains(fields[0], "/") {
			names = append(names, fields[0])
		}
	}
	return names
}

func quoteAll(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = regexp.QuoteMeta(n)
	}
	return out
}

func tailLines(out []byte, n int) []string {
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}
