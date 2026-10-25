//go:build unix

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSmokeRun runs the tool end to end with tiny sizes: it builds gocpp from
// this repository, builds binarytrees with Go and with gocpp, runs both and
// checks that the two engines printed the same checksum. It also checks the GC
// row is recorded as not available yet and that the JSON and markdown are
// written. It takes a build of gocpp and a build of the program: a minute on a
// quiet machine.
func TestSmokeRun(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "cmd", "gocpp")); err != nil {
		t.Fatalf("not in a worktree: %v", err)
	}
	dir := t.TempDir()
	outJSON := filepath.Join(dir, "out.json")
	outMD := filepath.Join(dir, "out.md")
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"run", "-root", root,
		"-bench", "all",
		"-flavor", "go,arc,gc",
		"-smoke",
		"-work", filepath.Join(dir, "work"),
		"-out", outJSON,
		"-md", outMD,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("gcbench -smoke: exit %d\nstderr:\n%s", code, stderr.String())
	}
	data, err := os.ReadFile(outJSON)
	if err != nil {
		t.Fatal(err)
	}
	var r report
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if r.Smoke != true || r.Reps != 1 || r.Warmup != 0 || r.Depth != 10 {
		t.Errorf("smoke settings: %+v", r)
	}
	if len(r.Programs) != 3 {
		t.Fatalf("programs: %d", len(r.Programs))
	}
	for _, pr := range r.Programs {
		fl := map[string]flavorResult{}
		for _, f := range pr.Flavors {
			fl[f.Flavor] = f
		}
		if fl["go"].Status != statusOK || fl["arc"].Status != statusOK {
			t.Fatalf("%s: go/arc did not build and run: go %q (%s), arc %q (%s)", pr.Name, fl["go"].Status, fl["go"].Note, fl["arc"].Status, fl["arc"].Note)
		}
		if fl["gc"].Status != statusNotAvailable {
			t.Errorf("%s: gc status %q, want %q (%s)", pr.Name, fl["gc"].Status, statusNotAvailable, fl["gc"].Note)
		}
		if len(fl["go"].Runs) != 2 || len(fl["arc"].Runs) != 2 {
			t.Fatalf("%s: runs: go %d, arc %d", pr.Name, len(fl["go"].Runs), len(fl["arc"].Runs))
		}
		for i := range fl["go"].Runs {
			g, a := fl["go"].Runs[i], fl["arc"].Runs[i]
			if g.Threads != a.Threads || g.Status != runOK || a.Status != runOK {
				t.Errorf("%s thread %d: go %q arc %q", pr.Name, g.Threads, g.Status, a.Status)
				continue
			}
			if g.Output == "" || g.Output != a.Output {
				t.Errorf("%s thread %d: outputs differ: go %q, arc %q", pr.Name, g.Threads, g.Output, a.Output)
			}
			if len(g.Samples) != 1 || g.Samples[0].WallNS <= 0 {
				t.Errorf("%s thread %d: samples %+v", pr.Name, g.Threads, g.Samples)
			}
		}
		// binarytrees must print the same checksum at every thread count (the
		// partition must not depend on it); the operation counts of parks and
		// syscalls follow the thread count by design.
		if pr.Name == "binarytrees" && fl["go"].Runs[0].Output != fl["go"].Runs[1].Output {
			t.Errorf("binarytrees checksum depends on the thread count: %q vs %q", fl["go"].Runs[0].Output, fl["go"].Runs[1].Output)
		}
	}
	md, err := os.ReadFile(outMD)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"not yet (16.1d/16.1f)", "the flavors agree", "### binarytrees", "### parks", "### syscalls"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("markdown does not contain %q", want)
		}
	}
}
