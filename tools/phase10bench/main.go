// Command phase10bench measures what the features of Phase 10 cost: a program that uses `!` or macros against the
// same program written by hand, built by gocpp with each C++ compiler at -release (step 10.13).
//
//	go run ./tools/phase10bench [-reps 10] [-cxx g++,clang++] [-cases REGEXP] [-jobs 6] [-keep DIR]
//
// bench/phase10/CASE/VARIANT/ is a module (gocpp.mod, main.gp, and the packages it imports). The variants of a
// case print the same thing (checked) and differ in how they are written: the first variant in the order
// bang, macro, hand, func is the feature, "hand" is the code a Go programmer writes without the feature, and
// "func" (when there is one) is the version with the abstraction written as functions that take func values.
// Run it from the repository or any folder below it. The tool builds every variant (gocpp build -release), then runs rounds: one round runs every binary once, in a fixed
// order, so a change of the machine's load hits all variants alike; the first round is dropped. The table has the
// median CPU time (and wall time) with the MAD in percent, and the ratio of each variant to "hand". See docs/benchmarks/must-macros.md.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

// funcRounds is the number of measured rounds of a variant "func" (the first round is dropped as for the others).
const funcRounds = 3

type variant struct {
	name, dir string
}

type benchCase struct {
	name     string
	variants []variant
}

func main() {
	root := flag.String("root", "", "the folder of the cases (default: bench/phase10 of the repository, found from the working folder upwards)")
	reps := flag.Int("reps", 10, "measured rounds (one more is run first and dropped)")
	cxxList := flag.String("cxx", "g++,clang++", "C++ compilers")
	match := flag.String("cases", "", "regexp: only the cases whose name matches")
	jobs := flag.Int("jobs", 6, "builds at a time")
	keep := flag.String("keep", "", "work folder to keep (default: a temporary one that is removed)")
	timeout := flag.Duration("timeout", 5*time.Minute, "limit for a build or a run")
	flag.Parse()

	// Pdeathsig (proc_linux.go) is per OS thread: the thread that starts a child must live as long as it does.
	runtime.LockOSThread()
	repo, err := findRepo()
	if err != nil {
		fatal(err)
	}
	if *root == "" {
		*root = filepath.Join(repo, "bench", "phase10")
	}
	cases, err := findCases(*root, *match)
	if err != nil {
		fatal(err)
	}
	work := *keep
	if work == "" {
		if work, err = os.MkdirTemp("", "phase10bench"); err != nil {
			fatal(err)
		}
		defer os.RemoveAll(work)
	} else if err := os.MkdirAll(work, 0o755); err != nil {
		fatal(err)
	}
	gocpp := filepath.Join(work, "gocpp")
	goBuild := exec.Command("go", "build", "-o", gocpp, "./cmd/gocpp")
	goBuild.Dir = repo
	if out, err := goBuild.CombinedOutput(); err != nil {
		fatal(fmt.Errorf("go build ./cmd/gocpp: %v\n%s", err, out))
	}
	cache := filepath.Join(work, "cache")
	env := append(os.Environ(), "XDG_CACHE_HOME="+cache, "GOCPP_GATE_PHASE=09", "GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1")

	for _, cxx := range strings.Split(*cxxList, ",") {
		fmt.Printf("\n## %s\n\n", cxx)
		type built struct {
			c   *benchCase
			v   variant
			bin string
		}
		var all []built
		for i := range cases {
			for _, v := range cases[i].variants {
				all = append(all, built{&cases[i], v, filepath.Join(work, fmt.Sprintf("%s_%s_%s", cases[i].name, v.name, strings.ReplaceAll(cxx, "+", "p")))})
			}
		}
		// Build: the first build fills the runtime cache alone, the others share it.
		var wg sync.WaitGroup
		sem := make(chan struct{}, *jobs)
		errs := make([]error, len(all))
		for i, b := range all {
			if i == 1 {
				wg.Wait()
			}
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				runtime.LockOSThread() // the thread ends with this goroutine, after its child has
				defer wg.Done()
				defer func() { <-sem }()
				errs[i] = build(gocpp, env, cxx, b.v.dir, b.bin, *timeout)
			}()
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				fatal(fmt.Errorf("%s/%s: %v", all[i].c.name, all[i].v.name, err))
			}
		}
		// Run: rounds over every binary.
		times := make([][]time.Duration, len(all))
		cpus := make([][]time.Duration, len(all))
		outputs := make([]string, len(all))
		for round := 0; round <= *reps; round++ {
			for i, b := range all {
				if b.v.name == "func" && round > funcRounds {
					continue // 10 to 150 times slower than the others: a few rounds are enough
				}
				out, d, cpu, err := runOnce(b.bin, *timeout)
				if err != nil {
					fatal(fmt.Errorf("%s/%s: %v", b.c.name, b.v.name, err))
				}
				if round == 0 {
					outputs[i] = out
					continue
				}
				if out != outputs[i] {
					fatal(fmt.Errorf("%s/%s prints something different from one run to the next", b.c.name, b.v.name))
				}
				times[i] = append(times[i], d)
				cpus[i] = append(cpus[i], cpu)
			}
		}
		report(os.Stdout, all2rows(all, times, cpus, outputs, func(b built) (string, string) { return b.c.name, b.v.name }))
	}
	if load, err := os.ReadFile("/proc/loadavg"); err == nil {
		fmt.Printf("\nload average at the end: %s", load)
	}
}

// findRepo returns the repository root: the folder with go.mod and bench/phase10 above the working folder.
func findRepo() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(d, "bench", "phase10")); err == nil {
				return d, nil
			}
		}
		if p := filepath.Dir(d); p != d {
			d = p
			continue
		}
		return "", fmt.Errorf("no folder with go.mod and bench/phase10 above the working folder (use -root for the cases)")
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "phase10bench:", err)
	os.Exit(1)
}

// findCases lists the cases of root (folders with variant folders), whose names match rx.
func findCases(root, rx string) ([]benchCase, error) {
	var re *regexp.Regexp
	if rx != "" {
		var err error
		if re, err = regexp.Compile(rx); err != nil {
			return nil, err
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var cases []benchCase
	for _, e := range entries {
		if !e.IsDir() || (re != nil && !re.MatchString(e.Name())) {
			continue
		}
		c := benchCase{name: e.Name()}
		sub, err := os.ReadDir(filepath.Join(root, e.Name()))
		if err != nil {
			return nil, err
		}
		for _, s := range sub {
			if s.IsDir() {
				c.variants = append(c.variants, variant{s.Name(), filepath.Join(root, e.Name(), s.Name())})
			}
		}
		order := []string{"bang", "macro", "hand", "func"}
		slices.SortFunc(c.variants, func(a, b variant) int { return slices.Index(order, a.name) - slices.Index(order, b.name) })
		if len(c.variants) < 2 {
			return nil, fmt.Errorf("case %s has fewer than two variants", c.name)
		}
		cases = append(cases, c)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("no case in %s", root)
	}
	return cases, nil
}

func build(gocpp string, env []string, cxx, dir, bin string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	abs, err := filepath.Abs(bin)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, gocpp, "build", "-release", "-o", abs, ".")
	cmd.Dir = dir
	cmd.Env = append(env[:len(env):len(env)], "GOCPP_CXX="+cxx)
	setupChild(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gocpp build: %v\n%s", err, out)
	}
	return nil
}

// runOnce runs a binary and returns its standard output, its wall time and its CPU time (user plus system, from the
// kernel's accounting: a busy machine moves the wall time of a run but much less its CPU time).
func runOnce(bin string, timeout time.Duration) (string, time.Duration, time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	setupChild(cmd)
	start := time.Now()
	if err := cmd.Run(); err != nil {
		return "", 0, 0, err
	}
	return stdout.String(), time.Since(start), cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime(), nil
}
