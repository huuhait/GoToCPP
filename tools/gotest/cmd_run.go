package main

import (
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// parseShard reads "i/n" (0 <= i < n).
func parseShard(s string) (i, n int, err error) {
	if s == "" {
		return 0, 1, nil
	}
	a, b, ok := strings.Cut(s, "/")
	if ok {
		i, err = strconv.Atoi(a)
		if err == nil {
			n, err = strconv.Atoi(b)
		}
	}
	if !ok || err != nil || n < 1 || i < 0 || i >= n {
		return 0, 0, fmt.Errorf("-shard %q: want i/n with 0 <= i < n", s)
	}
	return i, n, nil
}

// inShard reports whether the path belongs to shard i of n (a hash of the
// path, like Go's runner, so that a shard does not change when tests are
// added).
func inShard(path string, i, n int) bool {
	if n <= 1 {
		return true
	}
	h := fnv.New32()
	io.WriteString(h, path)
	return int(h.Sum32()%uint32(n)) == i
}

// selectTests applies the filters of the run command to the applicable
// tests of the manifest.
type selection struct {
	Filter    *regexp.Regexp
	Dir       string // a folder prefix: "fixedbugs", "." for the top level only
	Directive string
	Shard     string
	Limit     int
	Paths     map[string]bool // only these (the gate)
}

func (s selection) pick(ts []Test) ([]Test, error) {
	si, sn, err := parseShard(s.Shard)
	if err != nil {
		return nil, err
	}
	var out []Test
	for _, t := range ts {
		switch {
		case t.Skipped():
			continue
		case s.Paths != nil && !s.Paths[t.Path]:
			continue
		case s.Filter != nil && !s.Filter.MatchString(t.Path):
			continue
		case s.Directive != "" && t.Directive != s.Directive:
			continue
		case s.Dir == ".":
			if strings.Contains(t.Path, "/") {
				continue
			}
		case s.Dir != "" && !strings.HasPrefix(t.Path, strings.TrimSuffix(s.Dir, "/")+"/"):
			continue
		}
		if !inShard(t.Path, si, sn) {
			continue
		}
		out = append(out, t)
	}
	if s.Limit > 0 && len(out) > s.Limit {
		out = out[:s.Limit]
	}
	return out, nil
}

// runFlags are the flags of the commands that run tests (run, gate).
type runFlags struct {
	commonFlags
	gocpp   string
	goTool  string
	cxx     string
	asan    bool
	jobs    int
	timeout time.Duration
	runTime time.Duration
	work    string
	gcache  string
	verbose bool
	sel     selection
	filter  string
}

func (rf *runFlags) register(fs *flag.FlagSet) {
	rf.commonFlags.register(fs)
	fs.StringVar(&rf.gocpp, "gocpp", "", "the gocpp program (default: built from the repository into the work folder)")
	fs.StringVar(&rf.goTool, "go", "go", "real Go, for runoutput and errorcheckoutput tests")
	fs.StringVar(&rf.cxx, "cxx", "g++", "the C++ compiler (GOCPP_CXX); empty keeps the environment's")
	fs.BoolVar(&rf.asan, "asan", false, "build the programs with -asan (slow)")
	fs.IntVar(&rf.jobs, "j", 2, "tests that run at the same time (at most 3: campaigns run through heavy.sh)")
	fs.DurationVar(&rf.timeout, "build-timeout", 5*time.Minute, "time limit of one gocpp build or check")
	fs.DurationVar(&rf.runTime, "run-timeout", 20*time.Second, "time limit of one test program (a test's -t flag may raise it)")
	fs.StringVar(&rf.work, "work", "", "folder for temporary files (default: a new folder under the system's, removed at the end)")
	fs.StringVar(&rf.gcache, "gocpp-cache", "", "XDG_CACHE_HOME of the gocpp commands (default: inside the work folder; keep one across runs to reuse the compiled runtime and objects)")
	fs.BoolVar(&rf.verbose, "v", false, "print every result")
	fs.StringVar(&rf.filter, "filter", "", "only paths matching this regexp")
	fs.StringVar(&rf.sel.Dir, "dir", "", "only this folder of test/ (\".\" is the top level)")
	fs.StringVar(&rf.sel.Directive, "directive", "", "only this directive")
	fs.StringVar(&rf.sel.Shard, "shard", "", "i/n: only the tests of shard i of n (0 <= i < n)")
	fs.IntVar(&rf.sel.Limit, "limit", 0, "at most this many tests (after the other filters)")
}

// compileFilter compiles -filter into the selection; call it before pick.
func (rf *runFlags) compileFilter() error {
	if rf.filter == "" {
		return nil
	}
	re, err := regexp.Compile(rf.filter)
	if err != nil {
		return usageError("-filter: %v", err)
	}
	rf.sel.Filter = re
	return nil
}

// setup prepares the runner: the gocpp program, the folders.
func (rf *runFlags) setup(pin Pin, stderr io.Writer) (r *Runner, cleanup func(), err error) {
	if rf.jobs < 1 || rf.jobs > 3 {
		return nil, nil, usageError("-j %d: want 1 to 3", rf.jobs)
	}
	root, err := rf.sourceRoot(pin, stderr)
	if err != nil {
		return nil, nil, err
	}
	work := rf.work
	var cleanups []func()
	cleanup = func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}
	if work == "" {
		if work, err = os.MkdirTemp("", "gotest-run-"); err != nil {
			return nil, nil, err
		}
		cleanups = append(cleanups, func() { os.RemoveAll(work) })
	} else if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, nil, err
	}
	gocpp := rf.gocpp
	if gocpp == "" {
		repo, err := repoRoot()
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		gocpp = filepath.Join(work, "gocpp")
		fmt.Fprintln(stderr, "building gocpp")
		cmd := exec.Command("go", "build", "-o", gocpp, "./cmd/gocpp")
		cmd.Dir = repo
		cmd.Stdout, cmd.Stderr = stderr, stderr
		if err := cmd.Run(); err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("go build ./cmd/gocpp: %v", err)
		}
	}
	gcache := rf.gcache
	if gcache == "" {
		gcache = filepath.Join(work, "xdg")
	}
	tmp := filepath.Join(work, "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		cleanup()
		return nil, nil, err
	}
	r = NewRunner(RunConfig{Root: root, Gocpp: gocpp, Go: rf.goTool, Work: tmp, GocppCache: gcache, CXX: rf.cxx,
		ASan: rf.asan, BuildTimeout: rf.timeout, RunTimeout: rf.runTime, Env: []string{"TMPDIR=" + tmp}})
	return r, cleanup, nil
}

// runTests runs the tests with jobs workers and returns the results in the
// order of ts.
func runTests(r *Runner, ts []Test, jobs int, progress func(Result, int, int)) []Result {
	results := make([]Result, len(ts))
	next := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for w := 0; w < jobs; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				res := r.RunTest(ts[i])
				mu.Lock()
				results[i] = res
				done++
				if progress != nil {
					progress(res, done, len(ts))
				}
				mu.Unlock()
			}
		}()
	}
	for i := range ts {
		if r.Stopped() {
			results[i] = Result{Path: ts[i].Path, Directive: ts[i].Directive, Outcome: Skipped, Detail: "interrupted"}
			continue
		}
		next <- i
	}
	close(next)
	wg.Wait()
	return results
}

// stopOnInterrupt kills the commands of r (and those that start later) when
// the tool gets SIGINT, so that no test program outlives the tool. The
// returned function removes the handler.
func stopOnInterrupt(r *Runner, stderr io.Writer) (stop func()) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	done := make(chan struct{})
	go func() {
		select {
		case <-sig:
			fmt.Fprintln(stderr, "interrupted: stopping the running commands")
			r.StopAll()
		case <-done:
		}
	}()
	return func() {
		signal.Stop(sig)
		close(done)
	}
}

func gocppVersion(gocpp string) string {
	out, err := exec.Command(gocpp, "version").CombinedOutput()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func cmdRun(args []string, stdout, stderr io.Writer) error {
	var rf runFlags
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rf.register(fs)
	out := fs.String("out", "", "write the results JSON here (default: print counts only)")
	rerun := fs.String("failing", "", "only the tests that failed in this results file")
	if err := fs.Parse(args); err != nil {
		return exitError{2, ""}
	}
	pin, err := rf.loadPin()
	if err != nil {
		return err
	}
	ts, err := rf.loadManifest()
	if err != nil {
		return err
	}
	if *rerun != "" {
		prev, err := ReadResults(*rerun)
		if err != nil {
			return err
		}
		rf.sel.Paths = map[string]bool{}
		for _, r := range prev.Results {
			if r.Outcome == Fail || r.Outcome == Timeout {
				rf.sel.Paths[r.Path] = true
			}
		}
	}
	if err := rf.compileFilter(); err != nil {
		return err
	}
	picked, err := rf.sel.pick(ts)
	if err != nil {
		return usageError("%v", err)
	}
	r, cleanup, err := rf.setup(pin, stderr)
	if err != nil {
		return err
	}
	defer cleanup()
	stop := stopOnInterrupt(r, stderr)
	defer stop()
	fmt.Fprintf(stderr, "running %d tests with %d workers\n", len(picked), rf.jobs)
	results := runTests(r, picked, rf.jobs, func(res Result, n, total int) {
		if rf.verbose || res.Outcome != Pass {
			fmt.Fprintf(stderr, "[%d/%d] %s %s %s\n", n, total, res.Outcome, res.Path, res.Cluster)
		} else if n%25 == 0 {
			fmt.Fprintf(stderr, "[%d/%d]\n", n, total)
		}
	})
	rs := &ResultSet{GoVersion: pin.Version, Gocpp: gocppVersion(r.Gocpp), Compiler: rf.cxx, Shard: rf.sel.Shard, Results: results}
	if *out != "" {
		if err := rs.WriteJSON(*out); err != nil {
			return err
		}
	}
	counts := map[string]int{}
	for _, x := range results {
		counts[x.Outcome]++
	}
	fmt.Fprintf(stdout, "%d tests: %d pass, %d fail, %d timeout, %d not run\n", len(results), counts[Pass], counts[Fail], counts[Timeout], counts[Skipped])
	return nil
}
