// Command benchhist prints the benchmark history of bench/history/*.jsonl as a
// table: one row per run (commit), the geometric mean of gocpp's time over
// Go's and over the C++ twins' per group, and the change from the row before.
//
//	go run ./tools/benchhist                       summary table of the history of this machine
//	go run ./tools/benchhist -bench nbody          one benchmark's median time per commit
//	go run ./tools/benchhist -file bench/history/x.jsonl -group cpu -last 20
//
// The runs of different machines are never mixed: the table shows one history
// file (default: the one of this machine's name), and a row whose machine or size differs
// from the row before is marked.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/huuhait/gocpp/tools/gobench/benchdata"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("benchhist", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("file", "", "history file or folder (default: bench/history/<this machine>.jsonl, else every file of bench/history)")
	group := fs.String("group", "all", "group for the summary: all, or a category (cpu, data, dispatch, runtime, generics)")
	bench := fs.String("bench", "", "show one benchmark (name, or category/name) instead of the summary")
	last := fs.Int("last", 0, "only the last n rows (0: all)")
	size := fs.String("size", "full", "only runs of this size (full or smoke; empty: both)")
	flavor := fs.String("flavor", "O2", "the flavor of the gocpp builds in the summary: default, O2 or release")
	thr := fs.Float64("threshold", 0.05, "a change of this fraction or more is marked")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	pattern := *file
	if pattern == "" {
		pattern = "bench/history"
	}
	entries, err := benchdata.ReadHistory(pattern)
	if err != nil {
		fmt.Fprintln(stderr, "benchhist:", err)
		return 2
	}
	var rows []benchdata.HistEntry
	for _, e := range entries {
		if *size == "" || e.Size == *size {
			rows = append(rows, e)
		}
	}
	if len(rows) == 0 {
		fmt.Fprintf(stderr, "benchhist: no history in %s (make bench appends a line)\n", pattern)
		return 1
	}
	if *last > 0 && len(rows) > *last {
		rows = rows[len(rows)-*last:]
	}
	if *bench != "" {
		return benchTable(stdout, stderr, rows, *bench, *thr)
	}
	summaryTable(stdout, rows, *group, *flavor, *thr)
	return 0
}

func short(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func mark(prev, cur, thr float64) string {
	if prev <= 0 || cur <= 0 {
		return ""
	}
	d := cur/prev - 1
	switch {
	case d >= thr:
		return fmt.Sprintf(" (+%.0f%% SLOWER)", d*100)
	case d <= -thr:
		return fmt.Sprintf(" (%.0f%% faster)", d*100)
	}
	return ""
}

func machineNote(prev *benchdata.HistEntry, cur benchdata.HistEntry) string {
	if prev == nil {
		return ""
	}
	if prev.Machine != cur.Machine {
		return " [other machine]"
	}
	if prev.Pinned != cur.Pinned {
		return " [pinning changed]"
	}
	return ""
}

func summaryTable(w io.Writer, rows []benchdata.HistEntry, group, flavor string, thr float64) {
	gxx, clang := benchdata.GocppImpl("g++", flavor), benchdata.GocppImpl("clang++", flavor)
	fmt.Fprintf(w, "geometric mean of time ratios, group %q, gocpp flavor %s (below 1.00: gocpp is faster)\n", group, flavor)
	fmt.Fprintf(w, "%-12s %-20s %6s %5s %9s %11s %9s %11s   %s\n", "commit", "date", "load", "n", "g++/Go", "clang++/Go", "g++/C++", "clang++/C++", "subject")
	var prev *benchdata.HistEntry
	var prevG float64
	for i := range rows {
		e := rows[i]
		a, n := e.GroupRatio(group, gxx, benchdata.ImplGo)
		b, _ := e.GroupRatio(group, clang, benchdata.ImplGo)
		c, _ := e.GroupRatio(group, gxx, benchdata.ImplCppGxx)
		d, _ := e.GroupRatio(group, clang, benchdata.ImplCppClang)
		cell := func(x float64) string {
			if x == 0 {
				return "-"
			}
			return fmt.Sprintf("%.2f", x)
		}
		note := machineNote(prev, e)
		if note == "" {
			note = mark(prevG, a, thr)
		}
		dirty := ""
		if e.Dirty {
			dirty = "+"
		}
		fmt.Fprintf(w, "%-12s %-20s %6.1f %5d %9s %11s %9s %11s   %s%s\n", e.Commit+dirty, short(e.Date, 19), e.Load, n, cell(a), cell(b), cell(c), cell(d), short(e.Subject, 50), note)
		prev, prevG = &rows[i], a
	}
}

func benchTable(w, stderr io.Writer, rows []benchdata.HistEntry, name string, thr float64) int {
	var keys []string
	seen := map[string]bool{}
	for _, e := range rows {
		for k := range e.Wall {
			cat, prog, _ := benchdata.SplitHistKey(k)
			if (prog == name || cat+"/"+prog == name) && !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	if len(keys) == 0 {
		fmt.Fprintf(stderr, "benchhist: no benchmark %q in the history\n", name)
		return 1
	}
	sort.Slice(keys, func(i, j int) bool {
		_, _, a := benchdata.SplitHistKey(keys[i])
		_, _, b := benchdata.SplitHistKey(keys[j])
		return rank(a) < rank(b)
	})
	fmt.Fprintf(w, "median wall time in ms of %s\n", strings.Split(keys[0], "/")[1])
	fmt.Fprintf(w, "%-12s %-20s %6s", "commit", "date", "load")
	for _, k := range keys {
		_, _, impl := benchdata.SplitHistKey(k)
		fmt.Fprintf(w, " %14s", impl)
	}
	fmt.Fprintln(w)
	prev := map[string]float64{}
	for _, e := range rows {
		fmt.Fprintf(w, "%-12s %-20s %6.1f", e.Commit, short(e.Date, 19), e.Load)
		var notes []string
		for _, k := range keys {
			v, ok := e.Wall[k]
			if !ok {
				fmt.Fprintf(w, " %14s", "-")
				continue
			}
			fmt.Fprintf(w, " %14.2f", v/1e6)
			if m := mark(prev[k], v, thr); m != "" {
				_, _, impl := benchdata.SplitHistKey(k)
				notes = append(notes, impl+m)
			}
			prev[k] = v
		}
		if len(notes) > 0 {
			fmt.Fprintf(w, "   %s", strings.Join(notes, "; "))
		}
		fmt.Fprintln(w)
	}
	return 0
}

func rank(impl string) int {
	return benchdata.ImplRank(impl)
}
