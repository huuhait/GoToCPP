// Command gostddryrun lists, for a pinned Go source tree, everything that the
// GC engine's standard library must replace under the build constraints of
// Phase 16 README 2.7: bodyless functions, //go:linkname of each kind, the
// compiler intrinsics, assembly-only paths, the files that the gocpp tag drops
// or adds, and cgo files.
//
// Usage:
//
//	go run ./tools/gostddryrun [-goroot DIR] [-goos linux|windows]
//	    [-goarch amd64|arm64] [-tags t1,t2] [-packages p1,p2] [-closure]
//	    [-typecheck] [-out file.json] [-md README.md]
//
// With -typecheck the same selection is type-checked with go/types and a
// source importer over the tool's own file selection (typecheck.go); the JSON
// then holds the type-check result and splits real type errors from the gaps
// the constraints themselves cause.
//
// The default GOROOT is GOCPP_GOROOT, else the GOROOT of the Go that built the
// tool (runtime.GOROOT()). That path is the only thing taken from the running
// toolchain: every build constraint, and every tool tag, comes from the tree
// at that path (see tags.go). No go command and no network are used.
//
// The default package set is the list of step 16.1 plus the import closure of
// net/http and crypto/tls. With -packages, the closure (when -closure is on,
// the default) is walked from the named packages instead.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// defaultPackages is the package list of Phase 16 step 16.1 (README 3, step
// 16.1) without its closure: the closure of net/http and crypto/tls is added
// by -closure.
var defaultPackages = []string{
	"runtime",
	"reflect",
	"internal/reflectlite",
	"internal/runtime/maps",
	"sync",
	"internal/sync",
	"time",
	"os",
	"syscall",
	"net",
	"net/http",
	"crypto/tls",
	"testing",
}

// defaultClosureRoots are the packages whose import closure the default set
// adds. They are not the whole default list: testing's closure (the test main,
// internal/fuzz, os/signal, runtime/pprof) is not part of the 16.1 dry run.
var defaultClosureRoots = []string{"net/http", "crypto/tls"}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseFlags(args, stderr)
	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintln(stderr, "gostddryrun:", err)
		return 2
	}
	res, err := dryRun(opts)
	if err != nil {
		fmt.Fprintln(stderr, "gostddryrun:", err)
		return 1
	}
	var tc *TypeCheckResult
	if opts.typecheck {
		tc, err = typeCheck(opts.goroot, res)
		if err != nil {
			fmt.Fprintln(stderr, "gostddryrun:", err)
			return 1
		}
	}
	if opts.out != "" {
		if tc != nil {
			err = writeTypeCheckJSON(opts.out, tc)
		} else {
			err = writeJSON(opts.out, res)
		}
		if err != nil {
			fmt.Fprintln(stderr, "gostddryrun:", err)
			return 1
		}
	}
	if opts.md != "" {
		siblings, err := readSiblings(opts.md, res)
		if err != nil {
			fmt.Fprintln(stderr, "gostddryrun:", err)
			return 1
		}
		tcs, err := readTypeCheckSiblings(opts.md, res, tc)
		if err != nil {
			fmt.Fprintln(stderr, "gostddryrun:", err)
			return 1
		}
		if err := os.MkdirAll(filepath.Dir(opts.md), 0o755); err != nil {
			fmt.Fprintln(stderr, "gostddryrun:", err)
			return 1
		}
		f, err := os.Create(opts.md)
		if err != nil {
			fmt.Fprintln(stderr, "gostddryrun:", err)
			return 1
		}
		if err := writeMarkdown(f, res, siblings, tcs); err != nil {
			f.Close()
			fmt.Fprintln(stderr, "gostddryrun:", err)
			return 1
		}
		if err := f.Close(); err != nil {
			fmt.Fprintln(stderr, "gostddryrun:", err)
			return 1
		}
	}
	if opts.out == "" && opts.md == "" {
		printSummary(stdout, res)
		if tc != nil {
			printTypeCheckSummary(stdout, tc)
		}
	}
	return 0
}

// readTypeCheckSiblings reads the type-check JSON files next to the README,
// so that the type-check section compares linux/amd64 and windows/amd64. As
// readSiblings does, it ignores a file for another Go version; the fresh run,
// when there is one, wins over a stale file for the same target.
func readTypeCheckSiblings(md string, res *Result, fresh *TypeCheckResult) ([]*TypeCheckResult, error) {
	dir := filepath.Dir(md)
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var out []*TypeCheckResult
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "-typecheck.json") {
			continue
		}
		tc, err := readTypeCheckJSON(filepath.Join(dir, e.Name()))
		if err != nil || tc.GoVersion != res.GoVersion {
			continue
		}
		if fresh != nil && sameTarget(tc.Tags, fresh.Tags) {
			continue
		}
		out = append(out, tc)
	}
	if fresh != nil {
		out = append(out, fresh)
	}
	return out, nil
}

// sameTarget reports whether two tag sets describe the same GOOS/GOARCH.
func sameTarget(a, b *TagSet) bool {
	if a == nil || b == nil {
		return false
	}
	return a.GOOS == b.GOOS && a.GOARCH == b.GOARCH
}

// printTypeCheckSummary prints the counts when no output file was asked for.
func printTypeCheckSummary(w io.Writer, tc *TypeCheckResult) {
	s := tc.Summary
	fmt.Fprintf(w, "type check: %d packages (%d in the set), %d checked, %d real errors in %d packages, %d expected gaps, %d import failures\n",
		s.Packages, s.PackagesInSet, s.Checked, s.RealErrors, s.FailedPackages, s.ExpectedGaps, s.ImportFailures)
}

type options struct {
	goroot      string
	goos        string
	goarch      string
	tags        []string
	packages    []string
	packagesSet bool // -packages was given: walk the closure from those, not from net/http + crypto/tls
	out         string
	md          string
	closure     bool
	typecheck   bool
}

func parseFlags(args []string, stderr io.Writer) (*options, error) {
	fs := flag.NewFlagSet("gostddryrun", flag.ContinueOnError)
	fs.SetOutput(stderr)
	goroot := fs.String("goroot", "", "the Go source tree (default: GOCPP_GOROOT, else the GOROOT of the Go that built the tool)")
	goos := fs.String("goos", "linux", "target GOOS: linux or windows")
	goarch := fs.String("goarch", "amd64", "target GOARCH: amd64 or arm64")
	tags := fs.String("tags", "", "extra build tags, comma-separated")
	packages := fs.String("packages", "", "packages to analyze, comma-separated (default: the step 16.1 list)")
	out := fs.String("out", "", "write the JSON result here")
	md := fs.String("md", "", "write the markdown summary/README here")
	closure := fs.Bool("closure", true, "walk the imports of the named packages (or of net/http and crypto/tls by default)")
	typecheck := fs.Bool("typecheck", false, "type-check the selected packages with go/types and a source importer (writes a type-check JSON)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: gostddryrun [-goroot DIR] [-goos linux|windows] [-goarch amd64|arm64]")
		fmt.Fprintln(stderr, "                   [-tags t1,t2] [-packages p1,p2] [-closure] [-typecheck] [-out file.json] [-md README.md]")
	}
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	o := &options{
		goroot:      *goroot,
		goos:        *goos,
		goarch:      *goarch,
		tags:        splitList(*tags),
		packages:    splitList(*packages),
		packagesSet: *packages != "",
		out:         *out,
		md:          *md,
		closure:     *closure,
		typecheck:   *typecheck,
	}
	if o.goroot == "" {
		o.goroot = os.Getenv("GOCPP_GOROOT")
	}
	if o.goroot == "" {
		o.goroot = runtime.GOROOT()
	}
	if o.goroot == "" {
		return nil, fmt.Errorf("no GOROOT: set GOCPP_GOROOT or -goroot")
	}
	if len(o.packages) == 0 {
		o.packages = append([]string{}, defaultPackages...)
	}
	return o, nil
}

// splitList splits a comma or space separated flag value.
func splitList(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
}

// dryRun analyzes the tree and returns the result.
func dryRun(opts *options) (*Result, error) {
	defaults, err := readToolDefaults(opts.goroot)
	if err != nil {
		return nil, err
	}
	tags, err := defaults.tagSet(opts.goos, opts.goarch, opts.tags)
	if err != nil {
		return nil, err
	}
	ctxt := newContext(opts.goroot, tags, "gocpp")

	named := append([]string{}, opts.packages...)
	sort.Strings(named)
	named = dedupe(named)

	roots := defaultClosureRoots
	if opts.packagesSet {
		roots = named
	}
	var closurePkgs []string
	var closureSet map[string]bool
	if opts.closure {
		closurePkgs, closureSet, err = closure(ctxt, opts.goroot, roots)
		if err != nil {
			return nil, err
		}
	} else {
		// Without the walk the "closure" subset is the named set itself, so the
		// summary and the README still describe what was analyzed.
		closureSet = make(map[string]bool, len(named))
		for _, p := range named {
			closureSet[p] = true
		}
	}

	set := map[string]bool{}
	for _, p := range named {
		set[p] = true
	}
	for _, p := range closurePkgs {
		set[p] = true
	}
	var all []string
	for p := range set {
		all = append(all, p)
	}
	sort.Strings(all)

	res := &Result{
		Schema:        schemaVersion,
		Tool:          "gostddryrun",
		GoVersion:     defaults.Version,
		GoMinor:       defaults.Minor,
		GOROOT:        opts.goroot,
		Tags:          tags,
		ToolDefaults:  defaults,
		PackageSet:    named,
		ClosureRoots:  roots,
		ClosureWalked: opts.closure,
	}
	for _, pkg := range all {
		dir := filepath.Join(opts.goroot, "src", filepath.FromSlash(pkg))
		files, err := selectDir(ctxt, dir)
		if err != nil {
			return nil, fmt.Errorf("package %s: %v", pkg, err)
		}
		// A package can have no file under the gocpp constraints (internal/goroot
		// is gc.go and gccgo.go only): that is a finding, not an error. The
		// dropped_by_gocpp list says why.
		scan, err := scanPackage(pkg, dir, files.GoFiles)
		if err != nil {
			return nil, fmt.Errorf("package %s: %v", pkg, err)
		}
		dropped, added, err := droppedAdded(opts.goroot, tags, dir)
		if err != nil {
			return nil, fmt.Errorf("package %s: %v", pkg, err)
		}
		cgo, err := cgoSelected(opts.goroot, tags, dir)
		if err != nil {
			return nil, fmt.Errorf("package %s: %v", pkg, err)
		}
		res.Packages = append(res.Packages, Package{
			Path:             pkg,
			Name:             scan.Name,
			Dir:              "src/" + pkg,
			InClosure:        closureSet[pkg],
			Files:            files.GoFiles,
			Excluded:         files.Excluded,
			DroppedByGocpp:   dropped,
			AddedByGocpp:     added,
			SFiles:           files.SFiles,
			CgoFiles:         cgo,
			TestFiles:        files.Tests,
			NoPureGoFallback: len(files.SFiles) > 0 && len(scan.Bodyless) > 0,
			Bodyless:         scan.Bodyless,
			Linknames:        scan.Linknames,
			Intrinsics:       scan.Intrinsics,
		})
	}
	res.Summary.All = summarize(res.Packages)
	var inClosure []Package
	for _, p := range res.Packages {
		if p.InClosure {
			inClosure = append(inClosure, p)
		}
	}
	res.Summary.Closure = summarize(inClosure)
	normalize(res)
	return res, nil
}

// readSiblings reads the other dry runs of the same Go version next to the
// README, so that it can compare linux/amd64 and windows/amd64. The README's
// directory is the one place that matters; the current run is not excluded
// here because writeMarkdown puts res over any sibling of its target.
func readSiblings(md string, res *Result) ([]*Result, error) {
	if md == "" {
		return nil, nil
	}
	dir := filepath.Dir(md)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var siblings []*Result
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		r, err := readJSON(filepath.Join(dir, e.Name()))
		if err != nil || r.GoVersion != res.GoVersion || r.Tool != "gostddryrun" {
			continue
		}
		siblings = append(siblings, r)
	}
	return siblings, nil
}

// printSummary prints the counts when no output file was asked for.
func printSummary(w io.Writer, res *Result) {
	fmt.Fprintf(w, "%s, %s/%s: %d packages, %d bodyless functions, %d linknames, %d distinct targets\n",
		res.GoVersion, res.Tags.GOOS, res.Tags.GOARCH, res.Summary.All.Packages,
		res.Summary.All.Bodyless, res.Summary.All.Linknames, res.Summary.All.DistinctTargets)
	fmt.Fprintf(w, "closure: %d packages, %d runtime hook targets, %d synctest-only, %d bodyless outside the runtime\n",
		res.Summary.Closure.Packages, res.Summary.Closure.RuntimeHookTargets,
		res.Summary.Closure.SynctestOnlyTargets, res.Summary.Closure.BodylessTargetsOutsideRuntime)
}
