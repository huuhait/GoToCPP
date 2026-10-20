// Command stdgocount runs Go's own tests of the standard packages with gocpp test and counts the
// test functions that pass, by the counting rule of the Phase 09 README (2.8).
//
// usage: go run ./tools/stdgocount [-gocpp bin] [-stdgo dir] [-j n] [-detail] [-check] [pkg...]
//
// Without packages it takes every manifest in tests/stdgo/manifest. The tests were made by
// tools/portstd/gotests/convert_std.py. With -check the exit status is 1 when a test fails or skips
// without an expectation in its specification, or an expectation is stale (the test passes), or a
// package is under 90 %, or a package could not be built or run.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/stdcount"
)

func main() {
	gocpp := flag.String("gocpp", "", "the gocpp binary (default: built from ./cmd/gocpp)")
	stdgo := flag.String("stdgo", "tests/stdgo", "the folder of the converted tests")
	work := flag.String("work", "", "a folder for the test binaries and caches (default: temp)")
	jobs := flag.Int("j", 4, "packages to build and run at once")
	detail := flag.Bool("detail", false, "list every function that does not pass, with its reason")
	check := flag.Bool("check", false, "exit with status 1 on a problem (see above)")
	logs := flag.String("logs", "", "write the log of each package's runs to this folder")
	timeout := flag.Duration("timeout", 5*time.Minute, "limit of one run of a test binary")
	flag.Parse()

	pkgs := flag.Args()
	if len(pkgs) == 0 {
		pkgs = allPackages(*stdgo)
	}
	if *work == "" {
		dir, err := os.MkdirTemp("", "gocpp-stdgocount")
		if err != nil {
			fatal(err)
		}
		defer os.RemoveAll(dir)
		*work = dir
	}
	if *gocpp == "" {
		*gocpp = filepath.Join(*work, "gocpp")
		if err := buildGocpp(*gocpp); err != nil {
			fatal(err)
		}
	}
	absGocpp, err := filepath.Abs(*gocpp)
	if err != nil {
		fatal(err)
	}

	runner := packageRunner{
		options: stdcount.RunOptions{Gocpp: absGocpp, StdGo: *stdgo, Timeout: *timeout},
		work:    *work,
		logs:    *logs,
	}
	counts, errs := runner.runAll(pkgs, *jobs)

	if report(os.Stdout, os.Stderr, pkgs, counts, errs, *detail) && *check {
		os.Exit(1)
	}
}

// allPackages returns the import paths of every manifest in the folder, in order.
func allPackages(stdgo string) []string {
	manifests, _ := filepath.Glob(filepath.Join(stdgo, "manifest", "*.json"))
	var pkgs []string
	for _, manifest := range manifests {
		name := strings.TrimSuffix(filepath.Base(manifest), ".json")
		pkgs = append(pkgs, strings.ReplaceAll(name, "__", "/"))
	}
	sort.Strings(pkgs)
	return pkgs
}

func buildGocpp(path string) error {
	build := exec.Command("go", "build", "-o", path, "./cmd/gocpp")
	build.Stderr = os.Stderr
	return build.Run()
}

// A packageRunner builds and runs the tests of packages, several at a time.
type packageRunner struct {
	options stdcount.RunOptions
	work    string // the parent of the work folder of each package
	logs    string // where the log of each package's runs goes, if not empty
}

// runAll runs the packages with the given number of jobs; counts[i] or errs[i] is the outcome of
// pkgs[i].
func (r *packageRunner) runAll(pkgs []string, jobs int) ([]*stdcount.Count, []error) {
	counts := make([]*stdcount.Count, len(pkgs))
	errs := make([]error, len(pkgs))
	slots := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	for i, pkg := range pkgs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			counts[i], errs[i] = r.run(pkg)
		}()
	}
	wg.Wait()
	return counts, errs
}

// run builds and runs the tests of one package and counts them.
func (r *packageRunner) run(pkg string) (*stdcount.Count, error) {
	m, err := stdcount.LoadManifest(r.options.StdGo, pkg)
	if err != nil {
		return nil, err
	}
	name := strings.ReplaceAll(pkg, "/", "_")
	options := r.options
	options.Work = filepath.Join(r.work, name)

	results, log, err := stdcount.Run(context.Background(), options, m)
	if r.logs != "" {
		os.MkdirAll(r.logs, 0o755)
		os.WriteFile(filepath.Join(r.logs, name+".log"), []byte(log), 0o644)
	}
	if err != nil {
		return nil, err
	}
	return stdcount.Tally(m, results), nil
}

// report prints the table (and the detail of each package), prints what a check rejects to errOut,
// and returns whether there is anything to reject: a package that did not run, an unexplained
// failure or skip, a stale expectation, or a package under the budget.
func report(
	out, errOut io.Writer, pkgs []string, counts []*stdcount.Count, errs []error, detail bool,
) bool {
	rejected := false
	var done []*stdcount.Count
	for i, count := range counts {
		if errs[i] != nil {
			fmt.Fprintf(errOut, "%s: %v\n", pkgs[i], errs[i])
			rejected = true
			continue
		}
		done = append(done, count)
	}

	fmt.Fprint(out, stdcount.Table(done))
	for _, count := range done {
		if detail {
			fmt.Fprintln(out)
			fmt.Fprint(out, stdcount.Detail(count))
		}
		for _, reject := range count.Rejects() {
			fmt.Fprintf(errOut, "%s: %s\n", count.Package, reject)
			rejected = true
		}
	}
	return rejected
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}
