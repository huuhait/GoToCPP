// Command gcbench measures the numbers of Phase 16 step 16.1: the same small
// programs on real Go, on gocpp's reference-counting engine (ARC) and, once
// steps 16.1d/16.1f land, on the GC engine. It builds each program for every
// flavor, runs it at several thread counts, repeats every run, compares the
// deterministic output of the flavors (the checksum) and writes stable JSON and
// a markdown table.
//
//	gcbench [run] -bench binarytrees|parks|syscalls|all \
//	    -flavor go|arc|gc|all -threads 1,2,4,8,16,32,64 \
//	    -out FILE -md FILE [-smoke] [-reps 5] [-warmup 1]
//	gcbench report -in FILE [-md FILE]   write the markdown of a run's JSON
//
// The programs are bench/gc/<name>/main.gp: one file that is valid Go and gp.
// Go builds it as main.go, gocpp builds it as main.gp, so both engines run the
// same algorithm. The deterministic output ("checksum N" or "ops N") must be
// identical in every flavor; a difference is a MISMATCH and fails the run.
//
// The GC flavor runs `gocpp build -gc-engine`, which does not exist yet: the
// tool records it as "not available yet (16.1d/16.1f)" and writes that into the
// GC rows instead of numbers. See docs/benchmarks/gc-engine-spike.md.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	// The machine has many cores and other jobs run: gcbench itself needs few
	// threads, and the measured programs get their own GOMAXPROCS (their
	// thread count). The go commands it starts get GOMAXPROCS=16.
	runtime.GOMAXPROCS(16)
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without os.Exit, so the tests can call it.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "run":
			args = args[1:]
		case "report":
			return cmdReport(args[1:], stdout, stderr)
		default:
			fmt.Fprintf(stderr, "gcbench: unknown command %q (run, report)\n", args[0])
			return 2
		}
	}
	cfg, code := parseFlags(args, stderr)
	if cfg == nil {
		return code
	}
	if cfg.tempWork {
		if cfg.keep {
			fmt.Fprintf(stderr, "gcbench: kept the work folder %s\n", cfg.work)
		} else {
			defer os.RemoveAll(cfg.work)
		}
	}
	rep, err := measure(cfg)
	if err != nil {
		fmt.Fprintln(stderr, "gcbench:", err)
		return 2
	}
	if cfg.out != "" {
		if err := writeJSON(cfg.out, rep); err != nil {
			fmt.Fprintln(stderr, "gcbench:", err)
			return 2
		}
	}
	if cfg.md != "" {
		if err := os.WriteFile(cfg.md, []byte(markdown(rep)), 0o644); err != nil {
			fmt.Fprintln(stderr, "gcbench:", err)
			return 2
		}
	}
	if cfg.out == "" && cfg.md == "" {
		fmt.Fprint(stdout, markdown(rep))
	}
	return rep.exitCode()
}

type config struct {
	root     string
	work     string
	gocpp    string
	bench    []program
	flavors  []string
	threads  []int
	reps     int
	warmup   int
	depth    int
	smoke    bool
	keep     bool
	tempWork bool // the work folder was made by the tool and is removed at the end unless -keep
	cxx      string
	cxxflags string
	timeout  time.Duration
	maxLoad  float64
	out, md  string
	command  string
	log      func(format string, args ...any)
}

func parseFlags(args []string, stderr io.Writer) (*config, int) {
	fs := flag.NewFlagSet("gcbench", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "the repository (a folder with go.mod and bench/gc/ at or above it)")
	work := fs.String("work", "", "work folder for sources, binaries and caches (default: a new temporary folder, removed at the end)")
	keep := fs.Bool("keep", false, "keep the work folder")
	gocpp := fs.String("gocpp", "", "the gocpp binary (default: built from the repository)")
	bench := fs.String("bench", "all", "the programs, comma separated: binarytrees, parks, syscalls, or all")
	flavor := fs.String("flavor", "all", "the flavors, comma separated: go, arc, gc, or all")
	threads := fs.String("threads", "", "thread counts, comma separated (default: 1,2,4,8,16,32,64; -smoke: 1,8)")
	reps := fs.Int("reps", 5, "measured runs per program, flavor and thread count (at least 5 for a baseline; -smoke: 1)")
	warmup := fs.Int("warmup", 1, "warm-up runs before them, dropped (-smoke: 0)")
	depth := fs.Int("depth", 0, "binarytrees depth (default: 18; -smoke: 10)")
	smoke := fs.Bool("smoke", false, "tiny sizes, one run: builds and runs, no timing meaning")
	cxx := fs.String("cxx", "", "the C++ compiler of the gocpp build (GOCPP_CXX; default: gocpp's choice)")
	cxxflags := fs.String("cxxflags", "", "GOCPP_CXXFLAGS of the gocpp build (default: gocpp's own, -O0 -g today)")
	timeout := fs.Duration("timeout", 5*time.Minute, "limit of one run of one program")
	maxLoad := fs.Float64("max-load", 0, "1-minute load average above which the machine is called busy (0: an eighth of the cores, at least 4)")
	out := fs.String("out", "", "write the JSON of the measurements here")
	md := fs.String("md", "", "write the markdown report here (empty: print it to stdout when -out is empty too)")
	if err := fs.Parse(args); err != nil {
		return nil, 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "gcbench: unexpected arguments %v\n", fs.Args())
		return nil, 2
	}
	rootDir, err := findRoot(*root)
	if err != nil {
		fmt.Fprintln(stderr, "gcbench:", err)
		return nil, 2
	}
	ps, err := parsePrograms(*bench)
	if err != nil {
		fmt.Fprintln(stderr, "gcbench:", err)
		return nil, 2
	}
	fl, err := parseFlavors(*flavor)
	if err != nil {
		fmt.Fprintln(stderr, "gcbench:", err)
		return nil, 2
	}
	ths, err := parseThreads(*threads, *smoke)
	if err != nil {
		fmt.Fprintln(stderr, "gcbench:", err)
		return nil, 2
	}
	r, w := *reps, *warmup
	d := *depth
	if *smoke {
		r, w = 1, 0
		if d == 0 {
			d = 10
		}
	}
	if d == 0 {
		d = 18
	}
	if r < 1 {
		fmt.Fprintln(stderr, "gcbench: -reps must be at least 1")
		return nil, 2
	}
	if w < 0 {
		fmt.Fprintln(stderr, "gcbench: -warmup must not be negative")
		return nil, 2
	}
	if *maxLoad <= 0 {
		*maxLoad = float64(max(4, runtime.NumCPU()/8))
	}
	workDir := *work
	tempWork := false
	if workDir == "" {
		workDir, err = os.MkdirTemp("", "gcbench-")
		if err != nil {
			fmt.Fprintln(stderr, "gcbench:", err)
			return nil, 2
		}
		tempWork = true
	} else if workDir, err = filepath.Abs(workDir); err != nil {
		fmt.Fprintln(stderr, "gcbench:", err)
		return nil, 2
	}
	return &config{
		root: rootDir, work: workDir, gocpp: *gocpp, bench: ps, flavors: fl, threads: ths,
		reps: r, warmup: w, depth: d, smoke: *smoke, keep: *keep, tempWork: tempWork,
		cxx: *cxx, cxxflags: *cxxflags,
		timeout: *timeout, maxLoad: *maxLoad, out: *out, md: *md,
		command: "gcbench run " + shellJoin(args),
		log:     func(format string, a ...any) { fmt.Fprintf(stderr, format+"\n", a...) },
	}, 0
}

// shellJoin formats command line arguments so that the recorded command can be
// copied back into a shell.
func shellJoin(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		if a == "" || strings.ContainsAny(a, " \t'\"") {
			parts[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		} else {
			parts[i] = a
		}
	}
	return strings.Join(parts, " ")
}

// findRoot returns the repository root: the folder with go.mod and bench/gc
// above dir.
func findRoot(dir string) (string, error) {
	d, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(d, "bench", "gc")); err == nil {
				return d, nil
			}
		}
		p := filepath.Dir(d)
		if p == d {
			return "", fmt.Errorf("no folder with go.mod and bench/gc above %s (use -root)", dir)
		}
		d = p
	}
}
