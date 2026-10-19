// Difftest generates random Go programs whose behaviour the language defines, runs them with
// real Go and with gocpp, and compares what they print and how they end. See
// docs/guides/difftest.md.
//
//	go run ./tools/difftest -n 10000 -seed 1 -batch 200 -j 3
//	go run ./tools/difftest -mode gc -n 1000        # check the generator against real Go only
//	go run ./tools/difftest -print 17 -seed 1       # print program 17
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/huuhait/gocpp/tools/difftest/diff"
	"github.com/huuhait/gocpp/tools/difftest/gen"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "difftest:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		n          = flag.Int("n", 1000, "number of programs")
		first      = flag.Int("first", 0, "id of the first program (a multiple of -batch to resume a campaign)")
		seed       = flag.Uint64("seed", 1, "seed")
		batch      = flag.Int("batch", 200, "programs per bundle")
		jobs       = flag.Int("j", 3, "bundles in flight (builds at once)")
		phase      = flag.Int("phase", 9, "the phase of the plan the compiler has reached: the generator uses its features")
		features   = flag.String("features", "", "feature changes: +name,-name,all,none")
		scenarios  = flag.Int("scenarios", 0, "scenarios per program (0: 2 to 5)")
		fatal      = flag.Int("fatal", 8, "percent of the programs that end with a panic")
		fatalSamp  = flag.Int("fatal-sample", 25, "percent of those that also run alone for exit code and stderr")
		mode       = flag.String("mode", "gocpp", "gocpp: compare with gocpp; gc: only check the programs with real Go")
		compilers  = flag.String("compilers", "g++,clang++", "C++ compilers")
		files      = flag.Int("files", 8, "files per bundle (gocpp compiles them in parallel)")
		clangEvery = flag.Int("clang-every", 4, "run the second and later compilers on every K-th batch only")
		releaseEv  = flag.Int("release-every", 5, "also build every K-th bundle with gocpp build -release, with the first compiler (0: never)")
		buildP     = flag.Int("p", 8, "gocpp build -p: compiles at the same time in one build (-j builds are in flight)")
		cxxflags   = flag.String("cxxflags", "-g0", "trusted C++ flags of the gocpp builds, not -asan and -release ones (-g0 makes a bundle about 18% faster)")
		objSize    = flag.String("objcache", "3G", "GOCPP_OBJCACHE_SIZE of the gocpp builds")
		redo       = flag.Bool("redo", false, "run again the bundles that had findings or generator bugs (after a fix)")
		niceness   = flag.Int("nice", 10, "run at this nice level (children too; Unix)")
		asan       = flag.Bool("asan", false, "build gocpp programs with -asan: any sanitizer report fails")
		gocppBin   = flag.String("gocpp", "", "the gocpp binary (default: built from this repository)")
		work       = flag.String("work", "", "work folder (default: a folder in the scratch directory of the campaign)")
		state      = flag.String("state", "", "state folder of a resumable campaign (default: <work>/state-<seed>)")
		findings   = flag.String("findings", "", "folder for findings (default: tools/difftest/findings)")
		knownPath  = flag.String("known", "", "known differences (default: tools/difftest/known.txt)")
		budget     = flag.Int("reduce-budget", 100, "gocpp builds per reduction")
		maxReduce  = flag.Int("max-reduce", 2, "reductions per cluster of equal mismatches")
		jsonOut    = flag.String("json", "", "write the counters as JSON to this file")
		printID    = flag.Int("print", -1, "print the program with this id and exit")
		bundleN    = flag.Int("bundle", 0, "print a bundle of this many programs starting at -first and exit")
		verifyRef  = flag.Bool("verify-ref", true, "run the reference twice and drop programs whose output differs")
		timeout    = flag.Duration("timeout", 60*time.Second, "time limit of one program run")
		noRecover  = flag.Bool("norecover", false, "debugging: the bundle does not recover, a panic shows its stack trace")
		gateNN     = flag.String("gate-phase", "", "GOCPP_GATE_PHASE for gocpp (default: -phase, two digits)")
	)
	flag.Parse()
	fs := gen.FeaturesForPhase(*phase)
	if err := fs.Parse(*features); err != nil {
		return err
	}
	gen.NoRecover = *noRecover
	gcfg := gen.Config{Seed: *seed, Features: fs, Scenarios: *scenarios, FatalPercent: *fatal}
	if *printID >= 0 {
		p, err := gen.Generate(gcfg, *printID)
		if err != nil {
			return err
		}
		fmt.Print(p.Src)
		return nil
	}
	if *bundleN > 0 {
		var ps []*gen.Program
		for i := *first; i < *first+*bundleN; i++ {
			p, err := gen.Generate(gcfg, i)
			if err != nil {
				return err
			}
			ps = append(ps, p)
		}
		src, err := gen.Bundle(ps, gen.Recovered)
		if err != nil {
			return err
		}
		fmt.Print(src)
		return nil
	}
	root, err := repoRoot()
	if err != nil {
		return err
	}
	if *work == "" {
		*work = filepath.Join(os.TempDir(), fmt.Sprintf("difftest-%d", os.Getuid()))
	}
	if err := os.MkdirAll(*work, 0o755); err != nil {
		return err
	}
	if *state == "" {
		// per generator version and feature set: programs of an older generator are not the same
		h := sha256.Sum256([]byte(fmt.Sprint(fs.Names(), *scenarios, *fatal)))
		*state = filepath.Join(*work, fmt.Sprintf("state-seed%d-p%d-v%d-%x", *seed, *phase, gen.Version, h[:3]))
	}
	if *findings == "" {
		*findings = filepath.Join(root, "tools", "difftest", "findings")
	}
	if *knownPath == "" {
		*knownPath = filepath.Join(root, "tools", "difftest", "known.txt")
	}
	known, err := diff.LoadKnown(*knownPath)
	if err != nil {
		return err
	}
	setNice(*niceness)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tmp := filepath.Join(*work, "tmp")
	os.MkdirAll(tmp, 0o755)
	ref := &diff.GoBackend{Work: tmp, Cache: filepath.Join(*work, "gocache"), Timeout: *timeout}
	xdg := filepath.Join(*work, "xdg")
	cfg := diff.Config{
		Redo: *redo, TmpDir: tmp, CacheDir: filepath.Join(xdg, "gocpp"),
		Seed: *seed, First: *first, N: *n, Batch: *batch, Jobs: *jobs, Gen: gcfg,
		Ref: ref, StateDir: *state, FindingsDir: *findings, Known: known,
		FatalSample: *fatalSamp, ReduceBudget: *budget, MaxReduce: *maxReduce,
		VerifyRef: *verifyRef, Files: *files, Log: os.Stderr, Every: map[string]int{},
	}
	if *mode == "gocpp" {
		bin := *gocppBin
		if bin == "" {
			bin = filepath.Join(*work, "gocpp")
			c := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/gocpp")
			c.Dir, c.Stdout, c.Stderr = root, os.Stderr, os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("building gocpp: %v", err)
			}
		}
		gate := *gateNN
		if gate == "" {
			gate = fmt.Sprintf("%02d", *phase)
		}
		for i, cxx := range strings.Split(*compilers, ",") {
			cxx = strings.TrimSpace(cxx)
			if cxx == "" {
				continue
			}
			flags := *cxxflags
			if *asan {
				flags = "" // the sanitizers' reports need the debug information
			}
			b := &diff.GocppBackend{Gocpp: bin, Cxx: cxx, Phase: gate, Asan: *asan, Work: tmp, Flags: flags, P: *buildP,
				ObjSize: *objSize, XDG: xdg, Timeout: *timeout}
			cfg.Subjects = append(cfg.Subjects, b)
			if i > 0 && *clangEvery > 1 {
				cfg.Every[b.Name()] = *clangEvery
			}
			if i == 0 && *releaseEv > 0 && !*asan {
				rb := *b
				rb.Release, rb.Flags = true, ""
				cfg.Subjects = append(cfg.Subjects, &rb)
				cfg.Every[rb.Name()] = *releaseEv
			}
		}
	} else if *mode != "gc" {
		return fmt.Errorf("unknown -mode %q", *mode)
	}
	c := diff.NewCampaign(cfg)
	t0 := time.Now()
	st, err := c.Run(ctx)
	if st != nil {
		data, _ := json.MarshalIndent(st, "", "  ")
		if *jsonOut != "" {
			os.WriteFile(*jsonOut, data, 0o644)
		}
		fmt.Printf("difftest seed %d: %d programs (%d lines) in %d batches, %s\n", *seed, st.Programs, st.Lines, st.Batches, time.Since(t0).Round(time.Second))
		fmt.Printf("  generator bugs %d (flaky reference %d); standalone fatal programs %d; gocpp builds %d\n", st.GenBugs, st.Flaky, st.Standalone, st.Builds)
		fmt.Printf("  mismatches %d: %v; known %v; unknown %d; findings %d\n", st.Mismatches, st.ByKind, st.Known, st.Unknown, st.Clusters)
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("interrupted: the state is saved, run the same command again to go on")
	}
	if err != nil {
		return err
	}
	if st != nil && (st.Unknown > 0 || st.GenBugs > 0) {
		os.Exit(3)
	}
	return nil
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		if d == filepath.Dir(d) {
			return "", fmt.Errorf("no go.mod above %s", dir)
		}
	}
}
