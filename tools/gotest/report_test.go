package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func sampleManifest() []Test {
	return []Test{
		{Path: "a.go", Directive: "run", Status: "applicable"},
		{Path: "b.go", Directive: "run", Status: "applicable", Needs: "needs 9.4"},
		{Path: "c.go", Directive: "errorcheck", Status: "applicable"},
		{Path: "d.go", Directive: "run", Status: "applicable"},
		{Path: "e.go", Directive: "run", Status: "skip:cgo"},
		{Path: "fixedbugs/f.go", Directive: "compile", Status: "applicable", Needs: "needs 7.3"},
	}
}

func sampleResults() *ResultSet {
	return &ResultSet{GoVersion: "go1.25.14", Compiler: "g++", Results: []Result{
		{Path: "a.go", Outcome: Pass},
		{Path: "b.go", Outcome: Pass}, // passes although it waits for 9.4: stale
		{Path: "c.go", Outcome: Fail, Cluster: "package not available: reflect"},
		{Path: "d.go", Outcome: Fail, Cluster: "package not available: reflect"},
		{Path: "fixedbugs/f.go", Outcome: Timeout, Cluster: "timeout: build"},
	}}
}

func TestBuildReport(t *testing.T) {
	rules, err := ParseClusterRules("# c\n9.4\t^package not available: reflect$  # why\n")
	if err != nil {
		t.Fatal(err)
	}
	rep := BuildReport(sampleManifest(), sampleResults(), rules)
	if rep.Overall.Applicable != 5 || rep.Overall.Pass != 2 || rep.Overall.Run != 5 {
		t.Errorf("overall %+v", rep.Overall)
	}
	if got := pct(rep.Overall); got != "40.0%" {
		t.Errorf("rate %s", got)
	}
	if rep.SkipCounts["skip:cgo"] != 1 {
		t.Errorf("skips %v", rep.SkipCounts)
	}
	if len(rep.Clusters) != 2 || rep.Clusters[0].Count != 2 || rep.Clusters[0].Owner != "9.4" {
		t.Errorf("clusters %+v", rep.Clusters)
	}
	// a cluster without a rule falls back to the test's own needs column
	if rep.Clusters[1].Owner != "7.3" {
		t.Errorf("fallback owner %q", rep.Clusters[1].Owner)
	}
	if rep.ByOwner["9.4"] != 2 || rep.ByOwner["7.3"] != 1 {
		t.Errorf("by owner %v", rep.ByOwner)
	}
	if len(rep.Stale) != 1 || !strings.HasPrefix(rep.Stale[0], "b.go") {
		t.Errorf("stale %v", rep.Stale)
	}
}

func TestReportNotRunCountsAsNotPassing(t *testing.T) {
	rs := &ResultSet{Results: []Result{{Path: "a.go", Outcome: Pass}}}
	rep := BuildReport(sampleManifest(), rs, nil)
	if rep.Overall.Run != 1 || rep.Overall.Pass != 1 || rep.NotRun != 4 || rep.Overall.Applicable != 5 {
		t.Errorf("%+v notrun=%d", rep.Overall, rep.NotRun)
	}
	if got := pct(rep.Overall); got != "20.0%" {
		t.Errorf("rate %s", got)
	}
}

func TestReportRendering(t *testing.T) {
	rep := BuildReport(sampleManifest(), sampleResults(), nil)
	var txt, md1, md2 bytes.Buffer
	rep.WriteText(&txt, 5)
	rep.WriteMarkdown(&md1, 5)
	rep.WriteMarkdown(&md2, 5)
	if md1.String() != md2.String() {
		t.Error("markdown is not deterministic")
	}
	for _, want := range []string{"BY DIRECTIVE", "FAILURE CLUSTERS", "package not available: reflect", "40.0%"} {
		if !strings.Contains(txt.String(), want) {
			t.Errorf("text report lacks %q:\n%s", want, txt.String())
		}
	}
	for _, want := range []string{"# Go `test/` conformance", "| `run` |", "unowned", "go1.25.14"} {
		if !strings.Contains(md1.String(), want) {
			t.Errorf("markdown lacks %q", want)
		}
	}
}

func TestClusterRuleErrors(t *testing.T) {
	for _, bad := range []string{"9.4 no tab\n", "9.4\t(\n", "\tre\n"} {
		if _, err := ParseClusterRules(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	rules, _ := ParseClusterRules("1.1\t^foo$\n2.2\tfoo.*\n")
	if ownerOf(rules, "foobar") != "2.2" || ownerOf(rules, "foo") != "1.1" || ownerOf(rules, "zzz") != "" {
		t.Error("first match wins")
	}
}

func TestResultsMergeAndJSON(t *testing.T) {
	dir := t.TempDir()
	a := &ResultSet{GoVersion: "go1.25.14", Results: []Result{{Path: "a.go", Outcome: Fail, Cluster: "x"}, {Path: "b.go", Outcome: Pass}}}
	b := &ResultSet{GoVersion: "go1.25.14", Results: []Result{{Path: "a.go", Outcome: Pass}}}
	fa, fb := filepath.Join(dir, "a.json"), filepath.Join(dir, "b.json")
	if err := a.WriteJSON(fa); err != nil {
		t.Fatal(err)
	}
	if err := b.WriteJSON(fb); err != nil {
		t.Fatal(err)
	}
	m, err := ReadResults(fa, fb)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Passing(); len(got) != 2 || got[0] != "a.go" {
		t.Errorf("a later file replaces an earlier result: %v", got)
	}
}

func TestClusterOf(t *testing.T) {
	for _, c := range []struct{ stage, detail, want string }{
		{"build", "x.gp:12:8: package runtime is not available yet (Phase 09)\n", "package not available: runtime"},
		{"build", "./x.gp:3:1: for range over a channel not supported yet (Phase 08)\n", "not supported yet (Phase 08): for range over a channel"},
		{"build", "internal error: generated C++ did not compile\nfoo.cpp:3:4: error: 'x' was not declared\n", "C++ does not compile: Q was not declared"},
		{"build", "# command-line-arguments\n./fifo.gp:23:4: internal error: not supported yet: the call of os.Exit (gocpp lowers only x)\n", "internal error: not supported yet: the call of os.Exit (gocpp lowers only x)"},
		{"build", "./x.gp:3:5: undefined: foo\n", "gocpp rejects: undefined: foo"},
		{"timeout-run", "x", "timeout: run"},
		{"run", "exit status 2\npanic: runtime error: index out of range [5] with length 3\n", "run: panic: runtime error: index out of range [N] with length N"},
		{"compare", "x", "output differs"},
		{"errorcheck", "missing\nx", "errorcheck: error not reported"},
		{"errorcheck", "succeeded\nx", "errorcheck: gocpp accepts what Go rejects"},
		{"errorcheck", "nomatch\nx", "errorcheck: message text differs"},
		{"errorcheck", "unmatched\nx", "errorcheck: extra error"},
		{"errorcheck", "nomatch\nx.go:3: no match for `un.*` in:\n\tx.go:3: expected ';', found ')'\n", "errorcheck: message text differs: got expected Q, found Q"},
		{"errorcheck", "unmatched\nunmatched error: x.go:12: other declaration of x\n", "errorcheck: extra error: other declaration of x"},
	} {
		if got := clusterOf(c.stage, c.detail); got != c.want {
			t.Errorf("clusterOf(%s, %q) = %q, want %q", c.stage, c.detail, got, c.want)
		}
	}
}

func TestReadResultsReclusters(t *testing.T) {
	dir := t.TempDir()
	rs := &ResultSet{GoVersion: "go1.25.14", Results: []Result{
		{Path: "a.go", Outcome: Fail, Stage: "build", Cluster: "old key", Detail: "x.gp:12:8: package runtime is not available yet (Phase 09)\n"},
		{Path: "b.go", Outcome: Timeout, Stage: "run", Cluster: "old", Detail: "ran out of time"},
		{Path: "c.go", Outcome: Fail, Stage: "build", Cluster: "kept"},
	}}
	f := filepath.Join(dir, "r.json")
	if err := rs.WriteJSON(f); err != nil {
		t.Fatal(err)
	}
	got, err := ReadResults(f)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"a.go": "package not available: runtime", "b.go": "timeout: run", "c.go": "kept"}
	for _, r := range got.Results {
		if r.Cluster != want[r.Path] {
			t.Errorf("%s: cluster %q, want %q", r.Path, r.Cluster, want[r.Path])
		}
	}
}
