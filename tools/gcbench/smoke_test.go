//go:build unix

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSmokeRun runs the tool end to end with tiny sizes: it builds gocpp from
// this repository, builds all four programs with Go and with gocpp, runs them
// and checks that the engines printed the same deterministic output. It also
// checks that the goroutines program's -mode and -depth reach it, that the GC
// rows are recorded as not available yet, and that the JSON and markdown are
// written. It takes a build of gocpp and of every program: about a minute on a
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
	if len(r.Programs) != 4 {
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
		wantRuns := 2 // the smoke run's two thread counts
		if pr.Name == "goroutines" {
			wantRuns = 4 // two modes, two goroutine counts
		}
		if len(fl["go"].Runs) != wantRuns || len(fl["arc"].Runs) != wantRuns {
			t.Fatalf("%s: runs: go %d, arc %d, want %d", pr.Name, len(fl["go"].Runs), len(fl["arc"].Runs), wantRuns)
		}
		for i := range fl["go"].Runs {
			g, a := fl["go"].Runs[i], fl["arc"].Runs[i]
			if g.Threads != a.Threads || g.Mode != a.Mode || g.Status != runOK || a.Status != runOK {
				t.Errorf("%s: go %s/%d %q, arc %s/%d %q: the cells must pair", pr.Name, g.Mode, g.Threads, g.Status, a.Mode, a.Threads, a.Status)
				continue
			}
			if g.Output == "" || g.Output != a.Output {
				t.Errorf("%s %s at %d: outputs differ: go %q, arc %q", pr.Name, g.Mode, g.Threads, g.Output, a.Output)
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
		// The goroutines program prints the observed goroutine count, and it
		// must be the count the row names. The blocked and running cells at one
		// count must differ and one checksum is pinned, so that a -mode that
		// never reaches the program (both flavors would agree on blocked) or a
		// changed program is caught; the mode itself must reach it through
		// programArgs, which the fallback of a missing mode cannot hide.
		if pr.Name == "goroutines" {
			byCell := map[string]string{}
			for _, rr := range fl["go"].Runs {
				if rr.Mode == "" {
					t.Errorf("goroutines run at %d has no mode", rr.Threads)
				}
				if !strings.Contains(rr.Output, "goroutines "+fmt.Sprint(rr.Threads)) {
					t.Errorf("goroutines %s t=%d: output %q does not report the count", rr.Mode, rr.Threads, rr.Output)
				}
				byCell[fmt.Sprintf("%s/%d", rr.Mode, rr.Threads)] = rr.Output
			}
			for cell, want := range map[string]string{
				"blocked/1":  "checksum 1\ngoroutines 1",
				"blocked/10": "checksum 325\ngoroutines 10",
			} {
				if got := byCell[cell]; got != want {
					t.Errorf("goroutines %s: output %q, want %q", cell, got, want)
				}
			}
			if byCell["blocked/10"] == byCell["running/10"] || byCell["blocked/1"] == byCell["running/1"] {
				t.Errorf("goroutines: blocked and running printed the same output: blocked %q, running %q",
					byCell["blocked/10"], byCell["running/10"])
			}
			// The depth must reach the program too: the same running cell at
			// depth 8 must print a different checksum than depth 32 did.
			bin := filepath.Join(dir, "work", "build", "goroutines", "go", "prog")
			env := envOver(cleanEnv(os.Environ()), "GOMAXPROCS=16")
			_, out, err := runTimed([]string{bin, "-n", "10", "-mode", "running", "-depth", "8", "-quiet"}, env, time.Minute)
			if err != nil {
				t.Errorf("goroutines at depth 8: %v", err)
			} else if out = strings.TrimSpace(out); out == byCell["running/10"] {
				t.Errorf("the depth did not reach the program: depth 8 and 32 both printed %q", out)
			} else if !strings.Contains(out, "goroutines 10") {
				t.Errorf("goroutines at depth 8: output %q does not report the count", out)
			}
		}
	}
	md, err := os.ReadFile(outMD)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"not yet (16.1d/16.1f)", "the flavors agree", "### binarytrees", "### parks", "### syscalls", "### goroutines", "Peak resident set per goroutine"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("markdown does not contain %q", want)
		}
	}
}
