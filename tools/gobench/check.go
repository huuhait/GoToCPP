package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/huuhait/gocpp/tools/gobench/benchdata"
)

// runCheck compares the measurements rep with the baseline base and decides
// the exit status: 0 when nothing regressed or the figures cannot tell, 1 for
// a regression that reproduced, 2 for an error (code, from the run, when a
// program failed).
func runCheck(cfg *Config, suite *Suite, base, rep *benchdata.Report, f *runFlags, maxLoad float64, code int) int {
	out := os.Stdout
	if code != 0 {
		fmt.Fprintln(out, "bench-check: FAILED: the run itself failed (see above)")
		return code
	}
	if f.out != "" {
		_ = benchdata.WriteReport(f.out, rep)
	}
	inconclusive := func(format string, a ...any) int {
		msg := fmt.Sprintf(format, a...)
		if f.requireQuiet {
			fmt.Fprintln(out, "bench-check: FAILED (-require-quiet): "+msg)
			return 1
		}
		fmt.Fprintln(out, "bench-check: INCONCLUSIVE: "+msg+" (not a failure; rerun on a quiet machine)")
		return 0
	}
	if base.Machine.CPU != rep.Machine.CPU || base.Machine.Cores != rep.Machine.Cores {
		return inconclusive("the baseline was measured on %q (%d cores), this is %q (%d cores)", base.Machine.CPU, base.Machine.Cores, rep.Machine.CPU, rep.Machine.Cores)
	}
	if (base.Machine.Pinned != "") != (rep.Machine.Pinned != "") {
		return inconclusive("the baseline was measured %s, this run %s", pinWord(base.Machine.Pinned), pinWord(rep.Machine.Pinned))
	}
	if base.Machine.GoMaxProcs != 0 && rep.Machine.GoMaxProcs != 0 && base.Machine.GoMaxProcs != rep.Machine.GoMaxProcs {
		return inconclusive("the programs had %d cpus' worth of parallelism (GOMAXPROCS, Go's garbage collector) in the baseline and have %d now", base.Machine.GoMaxProcs, rep.Machine.GoMaxProcs)
	}
	if l := base.Machine.LoadBefore[0]; l > maxLoad && !f.trustBaseline {
		return inconclusive("the baseline was measured at a 1-minute load average of %.1f (limit %.1f); write it again on a quiet machine, or -trust-baseline", l, maxLoad)
	}
	if l := rep.Machine.LoadBefore[0]; l > maxLoad {
		return inconclusive("the 1-minute load average was %.1f before the run (limit %.1f)", l, maxLoad)
	}
	// Only the programs and implementations of this run are compared (-match measures a part of the suite, -flavors a part of the flavors).
	{
		have := map[string]bool{}
		impls := map[string]bool{}
		for _, r := range rep.Results {
			have[r.Program] = true
			impls[r.Impl] = true
		}
		trimmed := *base
		trimmed.Results = nil
		for _, r := range base.Results {
			if have[r.Program] && impls[r.Impl] {
				trimmed.Results = append(trimmed.Results, r)
			}
		}
		base = &trimmed
	}
	cc := benchdata.GateConfig{Threshold: f.threshold}
	cmp := benchdata.Compare(base, rep, cc, !f.noDrift, nil)
	fmt.Fprintf(out, "bench-check: %d benchmarks compared, machine drift %.3f (from %d reference pairs), threshold %.0f%%\n", len(cmp.Rows), cmp.Drift, cmp.DriftPairs, f.threshold*100)
	if len(cmp.Rows) == 0 {
		return inconclusive("nothing to compare (the baseline has no ok gocpp results for these programs)")
	}
	// Suspected regressions are measured again, with the binaries already built.
	decisions := map[string][]benchdata.Decision{}
	key := func(p, i string) string { return p + "\x00" + i }
	pending := map[string][]string{}
	for _, r := range cmp.Rows {
		decisions[key(r.Program, r.Impl)] = []benchdata.Decision{r.Decision}
		if r.Decision.Verdict == benchdata.Regression || (r.Decision.Verdict == benchdata.Inconclusive && r.Decision.Ratio > 1+f.threshold) {
			pending[r.Program] = append(pending[r.Program], r.Impl)
		}
	}
	for try := 1; try <= f.retries && len(pending) > 0; try++ {
		n := 0
		for _, v := range pending {
			n += len(v)
		}
		fmt.Fprintf(out, "bench-check: measuring %d suspected regression(s) again (%d of %d)\n", n, try, f.retries)
		again := suite.Remeasure(pending)
		next := map[string][]string{}
		for _, r := range again {
			b := base.Find(r.Program, r.Impl)
			if b == nil {
				continue
			}
			cfgd := cc
			cfgd.Drift = cmp.Drift
			d := benchdata.Decide(b.WallStat(), r.WallStat(), cfgd)
			k := key(r.Program, r.Impl)
			decisions[k] = append(decisions[k], d)
			if d.Verdict != benchdata.OK && d.Verdict != benchdata.Improved {
				next[r.Program] = append(next[r.Program], r.Impl)
			}
		}
		pending = next
	}
	var regs, incs, imps []string
	for _, r := range cmp.Rows {
		ds := decisions[key(r.Program, r.Impl)]
		v := benchdata.Confirm(ds)
		if len(ds) == 1 {
			v = ds[0].Verdict
		}
		line := fmt.Sprintf("%-22s %-14s %s (%s)", r.Program, r.Impl, ds[len(ds)-1].Reason, fmt.Sprintf("x%.3f", ds[len(ds)-1].Ratio))
		switch v {
		case benchdata.Regression:
			regs = append(regs, line)
		case benchdata.Inconclusive:
			incs = append(incs, line)
		case benchdata.Improved:
			imps = append(imps, line)
		}
	}
	for _, m := range cmp.Missing {
		fmt.Fprintln(out, "bench-check: not measured now (or not ok): "+m)
	}
	sort.Strings(regs)
	print := func(title string, ls []string) {
		if len(ls) > 0 {
			fmt.Fprintf(out, "%s (%d):\n  %s\n", title, len(ls), strings.Join(ls, "\n  "))
		}
	}
	print("improved", imps)
	print("inconclusive", incs)
	print("REGRESSIONS", regs)
	if len(regs) > 0 {
		fmt.Fprintf(out, "bench-check: FAILED: %d regression(s) of more than %.0f%% that reproduced in %d measurements\n", len(regs), f.threshold*100, 1+f.retries)
		return 1
	}
	fmt.Fprintf(out, "bench-check: passed (%d inconclusive, %d improved)\n", len(incs), len(imps))
	return 0
}

func pinWord(pin string) string {
	if pin == "" {
		return "unpinned"
	}
	return "pinned to cpus " + pin
}
