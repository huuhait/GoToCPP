//go:build unix

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cmdFixture is a configuration folder and a Go tree for the commands, with a
// fake gocpp.
type cmdFixture struct {
	t      *testing.T
	root   string // has test/
	config string
	gocpp  string
	work   string
}

func newCmdFixture(t *testing.T) *cmdFixture {
	t.Helper()
	files := map[string]string{
		"a.go":      "// run\n\npackage main\n\n//OUT:a\n",
		"a.out":     "a\n",
		"b.go":      "// run\n\npackage main\n\n//OUT:b\n",
		"b.out":     "WRONG\n",
		"c.go":      "// run\n\npackage main\n\n//BUILDFAIL\n",
		"e.go":      "// errorcheck\n\npackage p\n\nvar a = b // ERROR \"undefined\" //DIAG:undefined: b\n",
		"skip.go":   "// run\n\npackage main\n\nimport \"runtime\"\n\nfunc main() { runtime.SetFinalizer(nil, nil) }\n",
		"ken/k.go":  "// run\n\npackage main\n\n//OUT:k\n",
		"ken/k.out": "k\n",
	}
	f := &cmdFixture{t: t, root: writeTree(t, files), config: t.TempDir(), work: t.TempDir()}
	bin := t.TempDir()
	f.gocpp = filepath.Join(bin, "gocpp")
	if err := os.WriteFile(f.gocpp, []byte(fakeGocpp), 0o755); err != nil {
		t.Fatal(err)
	}
	f.write("PINS", "version go1.25.14\nurl https://example.invalid/go.tar.gz\nsha256 "+strings.Repeat("ab", 32)+"\n")
	return f
}

func (f *cmdFixture) write(name, content string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.config, name), []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *cmdFixture) read(name string) string {
	f.t.Helper()
	b, err := os.ReadFile(filepath.Join(f.config, name))
	if err != nil {
		return ""
	}
	return string(b)
}

// do runs the tool with the common flags.
func (f *cmdFixture) do(args ...string) (code int, out, errOut string) {
	f.t.Helper()
	var so, se bytes.Buffer
	pre := []string{args[0], "-config", f.config, "-root", f.root}
	switch args[0] {
	case "run", "gate":
		pre = append(pre, "-gocpp", f.gocpp, "-work", f.work, "-j", "2", "-run-timeout", "10s")
	}
	args = append(pre, args[1:]...)
	code = run(args, &so, &se)
	return code, so.String(), se.String()
}

func TestCommandsEndToEnd(t *testing.T) {
	f := newCmdFixture(t)
	if code, out, errOut := f.do("manifest"); code != 0 {
		t.Fatalf("manifest: %d %s %s", code, out, errOut)
	}
	if code, _, _ := f.do("manifest", "-check"); code != 0 {
		t.Error("a fresh manifest is not up to date")
	}
	m := f.read("manifest.txt")
	if !strings.Contains(m, "skip.go") || !strings.Contains(m, "skip:gc-finalizer") {
		t.Errorf("manifest lacks the skipped test:\n%s", m)
	}
	results := filepath.Join(t.TempDir(), "r.json")
	code, out, errOut := f.do("run", "-out", results)
	if code != 0 {
		t.Fatalf("run: %d %s %s", code, out, errOut)
	}
	if !strings.Contains(out, "5 tests: 3 pass, 2 fail") {
		t.Errorf("run summary: %q", out)
	}
	rs, err := ReadResults(results)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(rs.Passing(), " "); got != "a.go e.go ken/k.go" {
		t.Errorf("passing: %s", got)
	}

	// report
	md := filepath.Join(t.TempDir(), "go-test.md")
	code, out, errOut = f.do("report", "-md", md, results)
	if code != 0 || !strings.Contains(out, "60.0%") {
		t.Errorf("report: %d %s %s", code, out, errOut)
	}
	if b, _ := os.ReadFile(md); !strings.Contains(string(b), "Go `test/` conformance") {
		t.Errorf("markdown: %s", b)
	}

	// triage: c.go fails with "gocpp rejects: boom"; b.go with "output differs"
	f.write("clusters.txt", "7.3\t^gocpp rejects: boom$\n")
	if code, out, errOut := f.do("triage", results); code != 0 {
		t.Fatalf("triage: %d %s %s", code, out, errOut)
	}
	if got := f.read("expect.txt"); !strings.Contains(got, "c.go needs 7.3") || strings.Contains(got, "b.go") {
		t.Errorf("expect.txt:\n%s", got)
	}
	if got := f.read("manifest.txt"); !strings.Contains(got, "needs 7.3") {
		t.Errorf("manifest after triage lacks the owner:\n%s", got)
	}

	// c.go gets fixed: the next triage drops it, and the manifest's needs
	// column with it
	if err := os.WriteFile(filepath.Join(f.root, "test", "c.go"), []byte("// run\n\npackage main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixed := filepath.Join(t.TempDir(), "fixed.json")
	if code, out, errOut := f.do("run", "-filter", "^c\\.go$", "-out", fixed); code != 0 || !strings.Contains(out, "1 tests: 1 pass") {
		t.Fatalf("rerun of c.go: %d %s %s", code, out, errOut)
	}
	if code, out, errOut := f.do("triage", fixed); code != 0 {
		t.Fatalf("triage 2: %d %s %s", code, out, errOut)
	}
	if got := f.read("expect.txt"); strings.Contains(got, "c.go") {
		t.Errorf("a passing test stays in expect.txt:\n%s", got)
	}
	if got := f.read("manifest.txt"); strings.Contains(got, "needs 7.3") {
		t.Errorf("a passing test keeps its needs column:\n%s", got)
	}
	// a test that was not run keeps its entry
	f.write("expect.txt", "b.go needs 9.9\n")
	if code, _, _ := f.do("triage", fixed); code != 0 {
		t.Fatal("triage 3")
	}
	if got := f.read("expect.txt"); !strings.Contains(got, "b.go needs 9.9") {
		t.Errorf("an entry of a test that was not run vanished:\n%s", got)
	}

	// the gate: record the passing set, check it
	if code, out, errOut := f.do("gate", "-update", "-results", results); code != 0 {
		t.Fatalf("gate -update: %d %s %s", code, out, errOut)
	}
	if got := f.read("passing.txt"); !strings.Contains(got, "a.go\n") || !strings.Contains(got, "ken/k.go\n") || strings.Contains(got, "b.go") {
		t.Errorf("passing.txt:\n%s", got)
	}
	code, out, errOut = f.do("gate")
	if code != 0 || !strings.Contains(out, "3 in passing.txt, 0 regressions") {
		t.Errorf("gate: %d %s %s", code, out, errOut)
	}
	// a regression: the .out file of a.go changes
	if err := os.WriteFile(filepath.Join(f.root, "test", "a.out"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ = f.do("gate")
	if code != 1 || !strings.Contains(out, "REGRESSION a.go") {
		t.Errorf("gate after regression: %d %s", code, out)
	}
	// the set may not shrink on update
	rs2 := filepath.Join(t.TempDir(), "r2.json")
	if code, _, _ := f.do("run", "-out", rs2); code != 0 {
		t.Fatal("rerun")
	}
	if code, _, errOut := f.do("gate", "-update", "-results", rs2); code != 1 || !strings.Contains(errOut, "no longer pass") {
		t.Errorf("update with a regression: %d %s", code, errOut)
	}
	if code, _, _ := f.do("gate", "-update", "-allow-shrink", "-results", rs2); code != 0 {
		t.Error("-allow-shrink")
	}
	if got := f.read("passing.txt"); strings.Contains(got, "a.go") {
		t.Errorf("a.go stays in passing.txt:\n%s", got)
	}
}

func TestRunSelectionFlags(t *testing.T) {
	f := newCmdFixture(t)
	f.do("manifest")
	_, out, _ := f.do("run", "-dir", "ken")
	if !strings.Contains(out, "1 tests: 1 pass") {
		t.Errorf("-dir: %q", out)
	}
	_, out, _ = f.do("run", "-directive", "errorcheck")
	if !strings.Contains(out, "1 tests: 1 pass") {
		t.Errorf("-directive: %q", out)
	}
	_, out, _ = f.do("run", "-filter", "^[ab]\\.go$", "-limit", "1")
	if !strings.Contains(out, "1 tests: 1 pass") {
		t.Errorf("-filter: %q", out)
	}
	if code, _, _ := f.do("run", "-shard", "3/3"); code == 0 {
		t.Error("bad shard accepted")
	}
	if code, _, _ := f.do("run", "-j", "4"); code == 0 {
		t.Error("-j 4 accepted: campaigns are limited to 3 jobs")
	}
}

func TestManifestCheckDetectsChange(t *testing.T) {
	f := newCmdFixture(t)
	f.do("manifest")
	if err := os.WriteFile(filepath.Join(f.root, "test", "new.go"), []byte("// run\n\npackage main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := f.do("manifest", "-check"); code != 1 {
		t.Errorf("a new test did not make the manifest stale: %d", code)
	}
}

func TestUnknownCommand(t *testing.T) {
	var o, e bytes.Buffer
	if code := run([]string{"frob"}, &o, &e); code != 2 {
		t.Errorf("code %d", code)
	}
	if code := run(nil, &o, &e); code != 2 {
		t.Errorf("no args: %d", code)
	}
}
