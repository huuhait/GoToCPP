package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Runner runs a program with real Go and with gocpp.
type Runner struct {
	Gocpp   string        // the gocpp binary
	Work    string        // scratch folder
	XDG     string        // XDG_CACHE_HOME of gocpp
	Timeout time.Duration // one run
	Flags   []string      // extra arguments of `gocpp run` (e.g. -asan)
	Cxx     string        // GOCPP_CXX ("": gocpp's default compiler)
	seq     atomic.Int64
}

// Result of one run.
type Result struct {
	Out    string
	Stderr string
	Code   int
	Err    error // the process did not start or timed out
}

func (r Result) Failed() bool { return r.Err != nil || r.Code != 0 }

func (r *Runner) dir(tag string) (string, error) {
	d := filepath.Join(r.Work, fmt.Sprintf("%s-%d", tag, r.seq.Add(1)))
	return d, os.MkdirAll(d, 0o755)
}

func (r *Runner) exec(dir string, env []string, name string, args ...string) Result {
	to := r.Timeout
	if to == 0 {
		to = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), to)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	inGroup(cmd)
	cmd.WaitDelay = 5 * time.Second
	err := cmd.Run()
	res := Result{Out: so.String(), Stderr: se.String()}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			res.Code = ee.ExitCode()
			if ctx.Err() != nil {
				res.Err = fmt.Errorf("timeout after %v", to)
			}
		} else {
			res.Err = err
		}
	}
	return res
}

// RunGo runs the source with real Go.
func (r *Runner) RunGo(src string) Result {
	d, err := r.dir("go")
	if err != nil {
		return Result{Err: err}
	}
	defer os.RemoveAll(d)
	os.WriteFile(filepath.Join(d, "main.go"), []byte(src), 0o644)
	os.WriteFile(filepath.Join(d, "go.mod"), []byte("module fmtdiff\n\ngo 1.25\n"), 0o644)
	return r.exec(d, []string{"GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=", "CGO_ENABLED=0"}, "go", "run", ".")
}

// RunGocpp runs the source with gocpp.
func (r *Runner) RunGocpp(src string) Result {
	d, err := r.dir("cpp")
	if err != nil {
		return Result{Err: err}
	}
	defer os.RemoveAll(d)
	os.WriteFile(filepath.Join(d, "main.gp"), []byte(src), 0o644)
	env := []string{"TMPDIR=" + d, "GOCPP_GATE_PHASE=15", "GOCPP_TEST_PHASE=15", "GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1"}
	if r.XDG != "" {
		env = append(env, "XDG_CACHE_HOME="+r.XDG)
	}
	if r.Cxx != "" {
		env = append(env, "GOCPP_CXX="+r.Cxx)
	}
	args := append([]string{"run"}, r.Flags...)
	args = append(args, "main.gp")
	return r.exec(d, env, r.Gocpp, args...)
}

// parseOut reads the `ID "quoted"` lines.
func parseOut(s string) map[int]string {
	m := map[int]string{}
	for _, line := range strings.Split(s, "\n") {
		id, rest, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(id)
		if err != nil {
			continue
		}
		m[n] = rest
	}
	return m
}

// Mismatch is one case where gocpp printed something else than Go.
type Mismatch struct {
	Seed  uint64
	Prog  int
	Case  Case
	Vals  []Val  // the values of the case
	Go    string // the quoted result of Go ("<missing>")
	Gocpp string
	Note  string // "crash", "build", ...
}

// Outcome of one program.
type Outcome struct {
	Mismatches []Mismatch
	GenBugs    []Mismatch // the program does not run with real Go (the generator's fault)
	Runs       int        // builds spent
	Cases      int
	Notes      []string
}

// Check compares the program's cases (ids) on both sides, isolating cases that make a side fail.
func (r *Runner) Check(g *Gen, p *Program, budget int) *Outcome {
	oc := &Outcome{Cases: len(p.Cases)}
	live := map[int]bool{}
	for _, c := range p.Cases {
		live[c.ID] = true
	}
	byID := map[int]Case{}
	for _, c := range p.Cases {
		byID[c.ID] = c
	}
	for round := 0; round < 12 && len(live) > 0 && budget > 0; round++ {
		goRes, cppRes := r.runBoth(g, p, live)
		oc.Runs++
		budget--
		if goRes.Failed() {
			// The generator's fault: find the cases that make real Go fail.
			bad := r.isolate(&budget, oc, ids(live), func(sub map[int]bool) bool {
				return r.RunGo(g.Source(p, sub)).Failed()
			})
			oc.Notes = append(oc.Notes, fmt.Sprintf("go failed (round %d): %s", round, firstLine(goRes.Stderr)))
			for _, id := range bad {
				oc.GenBugs = append(oc.GenBugs, Mismatch{Seed: p.Seed, Prog: p.Index, Vals: p.caseVals(byID[id]), Case: byID[id], Note: firstLine(goRes.Stderr)})
				delete(live, id)
			}
			if len(bad) == 0 {
				break
			}
			continue
		}
		goOut := parseOut(goRes.Out)
		cppOut := parseOut(cppRes.Out)
		complete := !cppRes.Failed()
		for id := range live {
			if _, ok := cppOut[id]; !ok {
				complete = false
			}
		}
		if complete {
			for _, id := range ids(live) {
				if goOut[id] != cppOut[id] {
					oc.Mismatches = append(oc.Mismatches, Mismatch{Seed: p.Seed, Prog: p.Index, Vals: p.caseVals(byID[id]), Case: byID[id], Go: goOut[id], Gocpp: cppOut[id]})
				}
			}
			return oc
		}
		// gocpp did not build or died: find the cases that make it fail (the cases that printed
		// before the crash and matched are fine).
		bad := r.isolate(&budget, oc, ids(live), func(sub map[int]bool) bool {
			cr := r.RunGocpp(g.Source(p, sub))
			if cr.Failed() {
				return true
			}
			co := parseOut(cr.Out)
			for id := range sub {
				if _, ok := co[id]; !ok {
					return true
				}
			}
			return false
		})
		note := "gocpp: " + firstLine(cppRes.Stderr)
		if cppRes.Err != nil {
			note = "gocpp: " + cppRes.Err.Error()
		}
		oc.Notes = append(oc.Notes, note)
		if len(bad) == 0 {
			// not reproducible alone: report the missing ones
			for _, id := range ids(live) {
				if _, ok := cppOut[id]; !ok {
					oc.Mismatches = append(oc.Mismatches, Mismatch{Seed: p.Seed, Prog: p.Index, Vals: p.caseVals(byID[id]), Case: byID[id], Go: goOut[id], Gocpp: "<missing>", Note: note})
				}
			}
			return oc
		}
		for _, id := range bad {
			oc.Mismatches = append(oc.Mismatches, Mismatch{Seed: p.Seed, Prog: p.Index, Vals: p.caseVals(byID[id]), Case: byID[id], Go: goOut[id], Gocpp: "<crash>", Note: note})
			delete(live, id)
		}
	}
	return oc
}

func (r *Runner) runBoth(g *Gen, p *Program, live map[int]bool) (Result, Result) {
	src := g.Source(p, live)
	var cppRes Result
	done := make(chan struct{})
	go func() {
		cppRes = r.RunGocpp(src)
		close(done)
	}()
	goRes := r.RunGo(src)
	<-done
	return goRes, cppRes
}

func ids(m map[int]bool) []int {
	var s []int
	for id := range m {
		s = append(s, id)
	}
	sort.Ints(s)
	return s
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	lines := strings.Split(s, "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "./main.") || strings.Contains(l, "error") || strings.Contains(l, "panic") || strings.Contains(l, "fatal") {
			return strings.TrimSpace(l)
		}
	}
	if len(lines) > 0 {
		return lines[0]
	}
	return ""
}

// isolate returns the ids of single cases that fail alone, by halving. fails(sub) is true when the
// program with the cases of sub fails. When no single case fails but a set does, the set is returned.
func (r *Runner) isolate(budget *int, oc *Outcome, set []int, fails func(map[int]bool) bool) []int {
	var out []int
	var rec func(s []int, known bool)
	rec = func(s []int, known bool) {
		if len(s) == 0 || *budget <= 0 {
			return
		}
		if !known {
			*budget--
			oc.Runs++
			if !fails(toSet(s)) {
				return
			}
		}
		if len(s) == 1 {
			out = append(out, s[0])
			return
		}
		h := len(s) / 2
		a, b := s[:h], s[h:]
		before := len(out)
		rec(a, false)
		rec(b, false)
		if len(out) == before {
			// neither half fails alone: the failure needs both (or is flaky): report the set
			out = append(out, s...)
		}
	}
	rec(set, true)
	return out
}

func toSet(s []int) map[int]bool {
	m := map[int]bool{}
	for _, x := range s {
		m[x] = true
	}
	return m
}
