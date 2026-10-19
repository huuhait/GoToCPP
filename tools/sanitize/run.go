package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// commonFlags are the flags of the commands that build and run programs (run, leaks).
type commonFlags struct {
	root      string
	gocpp     string
	cxx       string
	jobs      int
	buildTO   time.Duration
	runTO     time.Duration
	work      string
	cache     string
	out       string
	verbose   bool
	asanOpts  string
	ubsanOpts string
	tsanOpts  string
}

func (c *commonFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&c.root, "root", "", "the repository root (default: found from the working folder)")
	fs.StringVar(&c.gocpp, "gocpp", "", "the gocpp program (default: built from the repository into the work folder)")
	fs.StringVar(&c.cxx, "cxx", "g++", "the C++ compilers (GOCPP_CXX), comma separated: g++,clang++; empty keeps the environment's")
	fs.IntVar(&c.jobs, "j", 4, "programs that are built and run at the same time (run through heavy.sh on a shared machine)")
	fs.DurationVar(&c.buildTO, "build-timeout", 10*time.Minute, "time limit of one gocpp build")
	fs.DurationVar(&c.runTO, "run-timeout", 2*time.Minute, "time limit of one program run")
	fs.StringVar(&c.work, "work", "", "folder for temporary files (default: a new folder under the system's, removed at the end)")
	fs.StringVar(&c.cache, "gocpp-cache", "", "XDG_CACHE_HOME of the gocpp commands (default: inside the work folder; keep one across runs to reuse the compiled runtime and objects)")
	fs.StringVar(&c.out, "out", "sanitize-results", "folder for the results files and the logs of failures")
	fs.BoolVar(&c.verbose, "v", false, "print every result")
	fs.StringVar(&c.asanOpts, "asan-options", "", "more ASAN_OPTIONS (colon separated), after the mode's")
	fs.StringVar(&c.ubsanOpts, "ubsan-options", "", "more UBSAN_OPTIONS (colon separated), after the mode's")
	fs.StringVar(&c.tsanOpts, "tsan-options", "", "more TSAN_OPTIONS (space separated), read after gocpp's -race defaults (detect_deadlocks=0 report_thread_leaks=0 exitcode=66...), for example \"detect_deadlocks=1\"")
}

// A session holds what a command needs: the folders and the gocpp program.
type session struct {
	commonFlags
	root, work, gocpp, cache string
	stage                    string // the nightly stage, for the names of the results files ("" for run)
	cleanups                 []func()
	stderr                   io.Writer

	mu      sync.Mutex
	runners []*Runner
	deep    procs     // the make commands of the deep stages
	exit    func(int) // os.Exit; a test replaces it
	unsig   func()
}

func (c *commonFlags) open(stderr io.Writer) (*session, error) {
	s := &session{commonFlags: *c, stderr: stderr, exit: os.Exit}
	var err error
	if s.root = c.root; s.root == "" {
		if s.root, err = repoRoot(); err != nil {
			return nil, err
		}
	}
	if s.root, err = filepath.Abs(s.root); err != nil {
		return nil, err
	}
	if s.work = c.work; s.work == "" {
		if s.work, err = os.MkdirTemp("", "sanitize-"); err != nil {
			return nil, err
		}
		s.cleanups = append(s.cleanups, func() { os.RemoveAll(s.work) })
	} else if s.work, err = filepath.Abs(s.work); err != nil {
		return nil, err
	} else if err = os.MkdirAll(s.work, 0o755); err != nil {
		return nil, err
	}
	tmp := filepath.Join(s.work, "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		s.close()
		return nil, err
	}
	if s.cache = c.cache; s.cache == "" {
		s.cache = filepath.Join(s.work, "xdg")
	} else if s.cache, err = filepath.Abs(s.cache); err != nil {
		s.close()
		return nil, err
	}
	if s.gocpp = c.gocpp; s.gocpp == "" {
		s.gocpp = filepath.Join(s.work, "gocpp")
		fmt.Fprintln(stderr, "sanitize: building gocpp")
		cmd := exec.Command("go", "build", "-o", s.gocpp, "./cmd/gocpp")
		cmd.Dir = s.root
		cmd.Stdout, cmd.Stderr = stderr, stderr
		if err := cmd.Run(); err != nil {
			s.close()
			return nil, fmt.Errorf("go build ./cmd/gocpp: %v", err)
		}
	} else if s.gocpp, err = filepath.Abs(s.gocpp); err != nil {
		s.close()
		return nil, err
	}
	s.unsig = s.handleSignals()
	return s, nil
}

// handleSignals stops the session when the tool gets SIGINT or SIGTERM; the returned function stops
// listening.
func (s *session) handleSignals() func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-ch:
			s.interrupt()
		case <-done:
		}
	}()
	return func() { signal.Stop(ch); close(done) }
}

// interrupt kills every command of every runner, removes the work folder (the deferred close of
// the command never runs when the process exits here) and exits with 130.
func (s *session) interrupt() {
	s.mu.Lock()
	rs := slices.Clone(s.runners)
	s.mu.Unlock()
	for _, r := range rs {
		r.StopAll()
	}
	s.deep.StopAll()
	s.close()
	s.exit(130)
}

func (s *session) close() {
	if s.unsig != nil {
		s.unsig()
		s.unsig = nil
	}
	for i := len(s.cleanups) - 1; i >= 0; i-- {
		s.cleanups[i]()
	}
	s.cleanups = nil
}

// runner returns a runner for the mode and the compiler, and registers it for interrupt.
func (s *session) runner(mode Mode, cxx string) *Runner {
	var extra []string
	add := func(name, base, more, sep string) {
		if more != "" {
			extra = append(extra, name+"="+joinOptions(base, more, sep))
		}
	}
	add("ASAN_OPTIONS", modeEnvOption(mode, "ASAN_OPTIONS"), s.asanOpts, ":")
	add("UBSAN_OPTIONS", modeEnvOption(mode, "UBSAN_OPTIONS"), s.ubsanOpts, ":")
	add("TSAN_OPTIONS", "", s.tsanOpts, " ") // TSan's options are separated by spaces (or colons, or commas); gocpp's defaults are read before
	r := NewRunner(Config{
		Root: s.root, Gocpp: s.gocpp, Work: filepath.Join(s.work, "tmp"), Cache: s.cache, CXX: cxx, Mode: mode,
		BuildTimeout: s.buildTO, RunTimeout: s.runTO, Extra: extra,
	})
	s.mu.Lock()
	s.runners = append(s.runners, r)
	s.mu.Unlock()
	return r
}

// joinOptions puts the options more after those of base, with the separator of the sanitizer's variable
// (a later option wins).
func joinOptions(base, more, sep string) string {
	if base == "" {
		return more
	}
	return base + sep + more
}

// modeEnvOption returns the value of the variable in the mode's run environment, or "".
func modeEnvOption(m Mode, name string) string {
	for _, kv := range m.RunEnv {
		if v, ok := strings.CutPrefix(kv, name+"="); ok {
			return v
		}
	}
	return ""
}

// runAll runs the programs with jobs workers and returns the results in the order of ps. progress is
// called after each result.
func runAll(r *Runner, ps []*Program, jobs int, progress func(Result, int)) []Result {
	results := make([]Result, len(ps))
	work := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for range max(jobs, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				res := r.RunProgram(ps[i])
				results[i] = res
				mu.Lock()
				done++
				n := done
				if progress != nil {
					progress(res, n)
				}
				mu.Unlock()
			}
		}()
	}
	for i := range ps {
		work <- i
	}
	close(work)
	wg.Wait()
	return results
}

// A Summary is the result of one mode with one compiler.
type Summary struct {
	Stage   string         `json:"stage,omitempty"`
	Mode    string         `json:"mode"`
	CXX     string         `json:"cxx"`
	Corpus  string         `json:"corpus"`
	Skipped Skipped        `json:"skipped,omitempty"`
	Counts  map[string]int `json:"counts"`
	Results []Result       `json:"results"`
}

// Failures returns the failed results.
func (s *Summary) Failures() []Result {
	var out []Result
	for _, r := range s.Results {
		if r.Failed() {
			out = append(out, r)
		}
	}
	return out
}

func newSummary(stage string, mode Mode, cxx, corpus string, skipped Skipped, results []Result) *Summary {
	s := &Summary{Stage: stage, Mode: mode.Name, CXX: cxx, Corpus: corpus, Skipped: skipped, Counts: map[string]int{}, Results: results}
	for _, r := range results {
		s.Counts[r.Status]++
	}
	return s
}

// Line returns the one-line summary: "asan g++: 12 programs, ok 11, report 1".
func (s *Summary) Line() string {
	keys := make([]string, 0, len(s.Counts))
	n := 0
	for k, c := range s.Counts {
		keys = append(keys, k)
		n += c
	}
	sort.Strings(keys)
	parts := []string{fmt.Sprintf("%d programs", n)}
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, s.Counts[k]))
	}
	return fmt.Sprintf("%s %s: %s", s.Mode, nameOr(s.CXX, "default compiler"), strings.Join(parts, ", "))
}

func nameOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// base is the name of the summary's files: "asan-g++" for the run command and the asan stage, and
// "canary-asan-g++" for a stage that runs a mode under another name, so that the stages of one night do
// not overwrite each other.
func (s *Summary) base() string {
	name := s.Mode
	if s.Stage != "" && s.Stage != s.Mode {
		name = s.Stage + "-" + s.Mode
	}
	return name + "-" + nameOr(s.CXX, "default")
}

// write saves the summary as JSON in out, and the output of each failure as a log file.
func (s *Summary) write(out string) error {
	dir := filepath.Join(out, "logs", s.base())
	for _, r := range s.Failures() {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		name := strings.ReplaceAll(strings.TrimSuffix(r.Program, ".gp"), "/", "__") + ".log"
		text := fmt.Sprintf("%s\nstatus: %s\n\n%s\n\n%s", r.Program, r.Status, r.Detail, r.Output)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, s.base()+".json"), append(b, '\n'), 0o644)
}

// runFlags are the flags of the run command.
type runFlags struct {
	commonFlags
	mode    string
	corpus  string
	phase   int
	pending bool
	shard   string
	filter  string
	limit   int
}

func cmdRun(args []string, stdout, stderr io.Writer) error {
	var rf runFlags
	fs := newFlagSet("run", stderr)
	rf.commonFlags.register(fs)
	fs.StringVar(&rf.mode, "mode", "asan", "the sanitizer build: "+strings.Join(ModeNames(), ", "))
	fs.StringVar(&rf.corpus, "corpus", "", "run, concurrency, race, canary or sample (default: concurrency for tsan, run for the others); program paths as arguments replace it")
	fs.IntVar(&rf.phase, "phase", 99, "the highest phase (// phase: NN) to run")
	fs.BoolVar(&rf.pending, "pending", false, "also run the programs that say // pending:")
	fs.StringVar(&rf.shard, "shard", "", "i/n: only the programs of shard i of n (0 <= i < n)")
	fs.StringVar(&rf.filter, "filter", "", "only paths matching this regexp")
	fs.IntVar(&rf.limit, "limit", 0, "at most this many programs (after the other filters)")
	if err := fs.Parse(args); err != nil {
		return usageError("%v", err)
	}
	mode, err := ModeByName(rf.mode)
	if err != nil {
		return usageError("%v", err)
	}
	corpus := rf.corpus
	if corpus == "" && fs.NArg() == 0 {
		corpus = CorpusRun
		if mode.Name == "tsan" {
			corpus = CorpusConcurrency
		}
	}
	sel := Selection{Corpus: corpus, Paths: fs.Args(), Phase: rf.phase, Pending: rf.pending, Shard: rf.shard, Limit: rf.limit}
	if rf.filter != "" {
		if sel.Filter, err = regexp.Compile(rf.filter); err != nil {
			return usageError("-filter: %v", err)
		}
	}
	if rf.jobs < 1 {
		return usageError("-j %d: want 1 or more", rf.jobs)
	}
	s, err := rf.commonFlags.open(stderr)
	if err != nil {
		return err
	}
	defer s.close()
	return runCorpus(s, mode, sel, stdout, stderr)
}

// runCorpus runs the selection in the mode with each compiler and reports; the error is an
// exitError with code 1 when any program failed.
func runCorpus(s *session, mode Mode, sel Selection, stdout, stderr io.Writer) error {
	cf := &s.commonFlags
	sel.Root = s.root
	ps, skipped, err := sel.Pick()
	if err != nil {
		return err
	}
	if len(ps) == 0 {
		if sel.Shard != "" {
			// a small corpus has shards without a program: not an error. Say which of the two it is, so that a
			// shard whose every program was skipped (by -phase, a build constraint, pending) is not mistaken for
			// an empty one.
			if n := skipped.total(); n > 0 {
				fmt.Fprintf(stderr, "sanitize: %s: shard %s of the corpus: all %d of its programs were skipped (%s): nothing was checked\n", mode.Name, sel.Shard, n, skipped)
			} else {
				fmt.Fprintf(stderr, "sanitize: %s: shard %s of the corpus has no program\n", mode.Name, sel.Shard)
			}
			return nil
		}
		return fmt.Errorf("no programs selected (skipped: %s)", skipped)
	}
	label := sel.Corpus
	if label == "" {
		label = fmt.Sprintf("%d given", len(sel.Paths))
	}
	failed := 0
	for _, cxx := range splitList(cf.cxx) {
		r := s.runner(mode, cxx)
		fmt.Fprintf(stderr, "sanitize: %s with %s: %d programs of the %s corpus (skipped: %s)\n", mode.Name, nameOr(cxx, "default compiler"), len(ps), label, skipped)
		results := runAll(r, ps, cf.jobs, func(res Result, n int) {
			if res.Failed() {
				fmt.Fprintf(stderr, "FAIL %s [%s %s] %s\n%s\n", res.Program, res.Mode, nameOr(res.CXX, "default"), res.Status, indent(res.Detail))
			} else if cf.verbose {
				fmt.Fprintf(stderr, "ok   %s [%s %s] %s\n", res.Program, res.Mode, nameOr(res.CXX, "default"), res.Status)
			} else if n%100 == 0 {
				fmt.Fprintf(stderr, "sanitize: %d of %d\n", n, len(ps))
			}
		})
		sum := newSummary(s.stage, mode, cxx, label, skipped, results)
		if err := sum.write(cf.out); err != nil {
			return err
		}
		fmt.Fprintln(stdout, sum.Line())
		failed += len(sum.Failures())
	}
	if failed > 0 {
		return exitError{1, fmt.Sprintf("%d failures (logs in %s)", failed, cf.out)}
	}
	return nil
}

func splitList(s string) []string {
	if s == "" {
		return []string{""}
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func indent(s string) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	return "    " + strings.ReplaceAll(s, "\n", "\n    ")
}
