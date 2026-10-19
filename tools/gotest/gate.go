package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The regression gate: tools/gotest/passing.txt lists the tests that
// passed when it was last updated, one path per line. The set must not
// shrink: a test that is in the list and no longer passes fails the gate
// (make conformance-check). Progress is recorded by `gate -update`.

const passingHeader = `# Tests of Go's test/ folder that pass with gocpp (tools/gotest, step 15.10).
# The set only grows: make conformance-check runs these tests and fails when one
# of them does not pass any more. Update it after a full run with:
#   go run ./tools/gotest gate -update -results results.json
`

// ParsePassing reads passing.txt.
func ParsePassing(text string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return out
}

// FormatPassing writes passing.txt.
func FormatPassing(paths []string) string {
	p := append([]string(nil), paths...)
	sort.Strings(p)
	return passingHeader + strings.Join(p, "\n") + "\n"
}

// GateVerdict compares the committed passing set with results.
type GateVerdict struct {
	Regressions []string // in passing.txt, not passing now
	Missing     []string // in passing.txt, no result (partial run)
	NewPasses   []string // passing now, not in passing.txt
}

// CompareGate computes the verdict.
func CompareGate(passing []string, rs *ResultSet) GateVerdict {
	by := map[string]Result{}
	for _, r := range rs.Results {
		by[r.Path] = r
	}
	var v GateVerdict
	old := map[string]bool{}
	for _, p := range passing {
		old[p] = true
		r, ok := by[p]
		switch {
		case !ok || r.Outcome == Skipped:
			v.Missing = append(v.Missing, p)
		case r.Outcome != Pass:
			v.Regressions = append(v.Regressions, p)
		}
	}
	for _, p := range rs.Passing() {
		if !old[p] {
			v.NewPasses = append(v.NewPasses, p)
		}
	}
	return v
}

func cmdGate(args []string, stdout, stderr io.Writer) error {
	var rf runFlags
	fl := newFlagSet("gate", stderr)
	rf.register(fl)
	results := fl.String("results", "", "check this results file (comma-separated list) instead of running the tests")
	update := fl.Bool("update", false, "rewrite passing.txt from the results (needs -results): the set may not shrink unless -allow-shrink")
	shrink := fl.Bool("allow-shrink", false, "with -update: let tests leave passing.txt (say why in the commit)")
	partial := fl.Bool("partial", false, "tests of passing.txt without a result are not an error (a shard or a folder was run)")
	if err := fl.Parse(args); err != nil {
		return exitError{2, ""}
	}
	if *update && *results == "" {
		return usageError("gate -update needs -results (run the whole suite first: docs/guides/conformance.md)")
	}
	text, err := rf.readConfig("passing.txt", true)
	if err != nil {
		return err
	}
	passing := ParsePassing(text)
	ts, err := rf.loadManifest()
	if err != nil {
		return err
	}
	app := map[string]bool{}
	for _, t := range ts {
		if !t.Skipped() {
			app[t.Path] = true
		}
	}
	for _, p := range passing {
		if !app[p] {
			return exitError{1, fmt.Sprintf("passing.txt lists %s, which is not an applicable test of the manifest", p)}
		}
	}
	var rs *ResultSet
	if *results != "" {
		if rs, err = ReadResults(strings.Split(*results, ",")...); err != nil {
			return err
		}
	} else {
		if len(passing) == 0 {
			fmt.Fprintln(stdout, "passing.txt is empty: nothing to check")
			return nil
		}
		pin, err := rf.loadPin()
		if err != nil {
			return err
		}
		rf.sel.Paths = map[string]bool{}
		for _, p := range passing {
			rf.sel.Paths[p] = true
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
		fmt.Fprintf(stderr, "running %d tests of passing.txt\n", len(picked))
		res := runTests(r, picked, rf.jobs, func(res Result, n, total int) {
			if res.Outcome != Pass {
				fmt.Fprintf(stderr, "[%d/%d] %s %s %s\n", n, total, res.Outcome, res.Path, res.Cluster)
			}
		})
		rs = &ResultSet{GoVersion: pin.Version, Results: res}
		if rf.sel.Shard != "" || rf.sel.Limit > 0 || rf.filter != "" || rf.sel.Dir != "" || rf.sel.Directive != "" {
			*partial = true
		}
	}
	v := CompareGate(passing, rs)
	if *update {
		if len(v.Regressions) > 0 && !*shrink {
			return exitError{1, fmt.Sprintf("%d tests of passing.txt no longer pass (first: %s): fix them, or -allow-shrink", len(v.Regressions), v.Regressions[0])}
		}
		if len(v.Missing) > 0 && !*shrink {
			return exitError{1, fmt.Sprintf("%d tests of passing.txt have no result (first: %s): give the results of a full run", len(v.Missing), v.Missing[0])}
		}
		dir, err := rf.configDir()
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "passing.txt"), []byte(FormatPassing(rs.Passing())), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "passing.txt: %d tests (%d new, %d removed)\n", len(rs.Passing()), len(v.NewPasses), len(v.Regressions)+len(v.Missing))
		return nil
	}
	for _, p := range v.Regressions {
		fmt.Fprintf(stdout, "REGRESSION %s\n", p)
	}
	if !*partial {
		for _, p := range v.Missing {
			fmt.Fprintf(stdout, "NO RESULT %s\n", p)
		}
	}
	fmt.Fprintf(stdout, "conformance-check: %d in passing.txt, %d regressions, %d new passes (run gate -update to record them)\n",
		len(passing), len(v.Regressions), len(v.NewPasses))
	if len(v.Regressions) > 0 || (!*partial && len(v.Missing) > 0) {
		return exitError{1, "the set of passing tests shrank"}
	}
	return nil
}
