package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// runTypedProbe is the hidden developer command "gocpp __typed-probe" (steps
// 12.1 and 12.2 of typed interop): it checks the packages of the patterns,
// collects the C/C++ references of each root package, probes them with clang
// and prints the reference table and the result table as JSON to standard
// output (-refs: only the references, nothing is probed). It exists for
// developing the typed pipeline and for its tests; it is not in the help.
//
// The answers are cached on disk (ADR-088): -cache-stats prints what the cache folder holds and
// -cache-clean empties it (neither checks anything); -stats prints to standard error, after each
// package, how many compiler runs the probes needed and where the answers came from;
// GOCPP_PROBE_CACHE=off turns the cache off, GOCPP_PROBE_JOBS bounds the compiler runs.
func runTypedProbe(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	c := &checkCommand{stdout: io.Discard, stderr: stderr, kinds: true, typedMembers: true, noTyped: true}
	flags := flag.NewFlagSet("gocpp __typed-probe", flag.ContinueOnError)
	flags.SetOutput(stderr)
	refsOnly := flags.Bool("refs", false, "only collect the references (no clang run)")
	decls := flags.Bool("decls", false, "also extract the declarations the results mention and map every result to the foreign view (steps 12.3 and 12.4)")
	cacheStats := flags.Bool("cache-stats", false, "print what the probe cache folder holds, as JSON, and exit")
	cacheClean := flags.Bool("cache-clean", false, "remove the typed probe cache folder and exit")
	stable := flags.Bool("stable", false, "leave out timing, statistics and cache flags from the JSON, so that a cold and a warm run print the same bytes")
	stats := flags.Bool("stats", false, "print the cache statistics of each package to standard error")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *cacheStats || *cacheClean {
		return typedProbeCache(*cacheClean, stdout, stderr)
	}
	var err error
	if c.goos, c.goarch, err = target(); err != nil {
		fmt.Fprintf(stderr, "gocpp: %v\n", err)
		return 2
	}
	if c.dir, err = getwd(); err != nil {
		fmt.Fprintf(stderr, "gocpp __typed-probe: %v\n", err)
		return 1
	}
	var stop func()
	c.ctx, c.stopped, stop = signalContext()
	defer stop()
	prog, r, code := c.check(flags.Args())
	if r == nil || code != 0 {
		return code
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	runner := new(probe.Runner)
	for _, root := range prog.Roots {
		p := r.Package(root)
		if p == nil || !p.Checked() {
			continue
		}
		opts := typedinterop.Options{Decls: *decls, Target: ftype.LP64, Runner: runner, GOOS: c.goos, GOARCH: c.goarch}
		if c.goos == "windows" {
			opts.Target = ftype.LLP64
		}
		if *refsOnly {
			rep := typedinterop.Collect(p, prog.Fset)
			if err := enc.Encode(rep); err != nil {
				fmt.Fprintf(stderr, "gocpp __typed-probe: %v\n", err)
				return 1
			}
			continue
		}
		rep, err := typedinterop.Analyze(c.ctx, p, prog.Fset, opts)
		if err != nil && rep == nil {
			if errors.Is(err, typedinterop.ErrNoClang) {
				fmt.Fprintf(stderr, "gocpp __typed-probe: %v\n", err)
				return 3
			}
			fmt.Fprintf(stderr, "gocpp __typed-probe: %v\n", err)
			return 1
		}
		if err != nil {
			fmt.Fprintf(stderr, "gocpp __typed-probe: %v\n", err)
		}
		rep.Stable = *stable
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintf(stderr, "gocpp __typed-probe: %v\n", err)
			return 1
		}
		if *stats {
			printProbeStats(stderr, rep)
		}
	}
	return 0
}

// typedProbeCache is -cache-stats and -cache-clean.
func typedProbeCache(clean bool, stdout, stderr io.Writer) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if probe.CacheDir() == "" {
		fmt.Fprintln(stderr, "gocpp __typed-probe: the probe cache on disk is off (GOCPP_PROBE_CACHE, GOCPP_NOPROBECACHE)")
	}
	if clean {
		u, err := probe.CleanCache("")
		if err != nil {
			fmt.Fprintf(stderr, "gocpp __typed-probe: %v\n", err)
			return 1
		}
		fmt.Fprintf(stderr, "gocpp __typed-probe: removed %d entries, %d bytes from %s\n", u.Entries, u.Bytes, u.Dir)
		return 0
	}
	if err := enc.Encode(probe.CacheUsage("")); err != nil {
		fmt.Fprintf(stderr, "gocpp __typed-probe: %v\n", err)
		return 1
	}
	return 0
}

// printProbeStats writes the line that -stats prints for a package.
func printProbeStats(w io.Writer, rep *typedinterop.Report) {
	st := struct {
		Units, Hits, Ran, Batches int
		ProbeMs, KindMs           float64
		DeclRuns                  int `json:",omitempty"`
		Process                   probe.Counters
	}{Process: probe.CacheCounters()}
	if rep.Table != nil {
		c := rep.Table.Cache
		st.Units, st.Hits, st.Ran, st.Batches = c.Units, c.Hits, c.Ran, c.Batches
		st.ProbeMs = float64(rep.Table.Timing.Total.Microseconds()) / 1000
		st.KindMs = float64(rep.KindTime.Microseconds()) / 1000
	}
	if rep.Decls != nil {
		st.DeclRuns = rep.Decls.Runs
	}
	b, _ := json.Marshal(st)
	fmt.Fprintf(w, "gocpp __typed-probe: stats %s\n", b)
}
