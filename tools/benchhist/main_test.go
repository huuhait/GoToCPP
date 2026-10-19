package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/tools/gobench/benchdata"
)

func entry(commit string, gocpp, goT float64, machine string) benchdata.HistEntry {
	return benchdata.HistEntry{
		Commit: commit, Subject: "subject " + commit, Date: "2026-10-18T10:00:00Z", Size: "full", Reps: 10, Machine: machine, Load: 1.5,
		Wall: map[string]float64{
			"cpu/nbody/go": goT, "cpu/nbody/gocpp-g++:O2": gocpp, "cpu/nbody/cpp-g++": goT * 0.9,
			"data/map_int/go": goT, "data/map_int/gocpp-g++:O2": gocpp,
		},
	}
}

func write(t *testing.T, es ...benchdata.HistEntry) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "h.jsonl")
	for _, e := range es {
		if err := benchdata.AppendHistory(name, e); err != nil {
			t.Fatal(err)
		}
	}
	return name
}

func runHist(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestSummaryMarksChanges(t *testing.T) {
	f := write(t,
		entry("aaa", 1.0e9, 1.0e9, "cpu x, 8 cores"),
		entry("bbb", 1.02e9, 1.0e9, "cpu x, 8 cores"),
		entry("ccc", 1.3e9, 1.0e9, "cpu x, 8 cores"),
		entry("ddd", 0.9e9, 1.0e9, "cpu x, 8 cores"))
	code, out, errs := runHist("-file", f)
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 6 {
		t.Fatalf("%d lines:\n%s", len(lines), out)
	}
	if strings.Contains(lines[3], "SLOWER") || !strings.Contains(lines[4], "+27% SLOWER") || !strings.Contains(lines[5], "faster") {
		t.Errorf("marks wrong:\n%s", out)
	}
	if !strings.Contains(lines[3], "1.02") || !strings.Contains(lines[3], "subject bbb") {
		t.Errorf("row: %s", lines[3])
	}
	// a group restricts the geometric mean
	if code, out, _ := runHist("-file", f, "-group", "cpu", "-last", "1"); code != 0 || strings.Count(out, "\n") != 3 || !strings.Contains(out, "0.90") {
		t.Errorf("group/last: %d\n%s", code, out)
	}
}

func TestMachineChangeIsMarkedNotCompared(t *testing.T) {
	f := write(t, entry("aaa", 1e9, 1e9, "cpu x, 8 cores"), entry("bbb", 5e9, 1e9, "cpu y, 64 cores"))
	_, out, _ := runHist("-file", f)
	if !strings.Contains(out, "[other machine]") || strings.Contains(out, "SLOWER") {
		t.Errorf("machines were compared:\n%s", out)
	}
}

func TestBenchTable(t *testing.T) {
	f := write(t, entry("aaa", 2e6, 1e6, "m"), entry("bbb", 3e6, 1e6, "m"))
	code, out, _ := runHist("-file", f, "-bench", "nbody")
	if code != 0 || !strings.Contains(out, "nbody") || !strings.Contains(out, "gocpp-g++:O2 (+50% SLOWER)") {
		t.Errorf("%d\n%s", code, out)
	}
	// Go first, as in the tables
	if strings.Index(out, "go") > strings.Index(out, "gocpp-g++:O2") {
		t.Errorf("column order:\n%s", out)
	}
	if code, _, errs := runHist("-file", f, "-bench", "nothing"); code != 1 || !strings.Contains(errs, "no benchmark") {
		t.Errorf("unknown benchmark: %d %s", code, errs)
	}
}

func TestSizeFilterAndErrors(t *testing.T) {
	smoke := entry("sss", 1, 1, "m")
	smoke.Size = "smoke"
	f := write(t, smoke)
	if code, _, errs := runHist("-file", f); code != 1 || !strings.Contains(errs, "no history") {
		t.Errorf("only smoke rows: %d %s", code, errs)
	}
	if code, _, _ := runHist("-file", f, "-size", ""); code != 0 {
		t.Errorf("-size '': %d", code)
	}
	if code, _, errs := runHist("-file", filepath.Join(t.TempDir(), "none.jsonl")); code != 1 {
		t.Errorf("a missing file: %d %s", code, errs)
	}
	if code, _, _ := runHist("-bogus"); code != 2 {
		t.Errorf("bad flag: %d", code)
	}
}
