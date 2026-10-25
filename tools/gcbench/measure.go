package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// runResult is one program run at one thread count in one flavor.
type runResult struct {
	Threads int      `json:"threads"`        // a goroutine count for the goroutines program
	Mode    string   `json:"mode,omitempty"` // blocked, running, ... (empty: the program has no modes)
	Status  string   `json:"status"`         // ok, mismatch, run-failed
	Note    string   `json:"note,omitempty"`
	Output  string   `json:"output,omitempty"`  // the deterministic output lines, identical in every flavor
	Samples []sample `json:"samples,omitempty"` // the measured runs, warm-up runs dropped
	Wall    stat     `json:"wall"`              // ns
	CPU     stat     `json:"cpu"`               // ns
	RSS     stat     `json:"rss"`               // KiB
}

// programResult is one program, built for every flavor and run.
type programResult struct {
	Name  string `json:"name"`
	About string `json:"about"`
	Path  string `json:"path"`
	// Goroutines is true when the Threads of this program's runs are goroutine
	// counts and its runs have modes.
	Goroutines bool           `json:"goroutines,omitempty"`
	Flavors    []flavorResult `json:"flavors"`
}

// report is the JSON file of a run (schema 2).
type report struct {
	Schema           int             `json:"schema"`
	Tool             string          `json:"tool"`
	Generated        string          `json:"generated"`
	Smoke            bool            `json:"smoke"`
	Reps             int             `json:"reps"`
	Warmup           int             `json:"warmup"`
	Depth            int             `json:"depth"`                       // binarytrees depth
	Threads          []int           `json:"threads"`                     // the scaling programs' thread counts
	GoroutineThreads []int           `json:"goroutine_threads,omitempty"` // the goroutines program's goroutine counts
	GoroutineDepth   int             `json:"goroutine_depth,omitempty"`   // the goroutines program's recursion depth
	Command          string          `json:"command"`
	Commit           string          `json:"commit,omitempty"`
	Dirty            bool            `json:"dirty,omitempty"`
	CXX              string          `json:"cxx,omitempty"`      // GOCPP_CXX of the ARC/GC builds ("" = gocpp's choice)
	CXXFlags         string          `json:"cxxflags,omitempty"` // GOCPP_CXXFLAGS of the ARC/GC builds ("" = gocpp's default)
	GatePhase        string          `json:"gate_phase"`         // the GOCPP_GATE_PHASE the gocpp builds ran with
	TimeoutMS        int64           `json:"timeout_ms"`         // the limit of one run
	Machine          machine         `json:"machine"`
	Programs         []programResult `json:"programs"`
}

// schemaVersion is the version of the JSON file. It changes when a field
// changes its meaning.
//
// 2 added the goroutines program: a run's Threads can be a goroutine count,
// runs have a Mode, and the report records the goroutine settings.
const schemaVersion = 2

// measure builds every program for every selected flavor and measures it at
// every thread count.
func measure(c *config) (*report, error) {
	if c.reps < 5 && !c.smoke {
		c.log("warning: %d measured runs is fewer than the 5 a baseline needs", c.reps)
	}
	gocppBin := ""
	needGocpp := false
	for _, f := range c.flavors {
		if f == "arc" || f == "gc" {
			needGocpp = true
		}
	}
	if needGocpp {
		var err error
		gocppBin, err = c.gocppBinary()
		if err != nil {
			return nil, err
		}
	}
	m := machineInfo(gocppBin)
	m.LoadBefore = loadAvg()
	m.MaxLoad = c.maxLoad
	logMachine(c.log, m)

	// The goroutine settings are recorded only when the program is measured, so
	// that a report of another subset does not claim a goroutines run.
	goroutineThreads, goroutineDepth := []int(nil), 0
	if hasBenchProgram(c.bench, "goroutines") {
		goroutineThreads, goroutineDepth = c.goroutineThreads, c.goroutineDepth
	}
	rep := &report{
		Schema: schemaVersion, Tool: "gcbench", Generated: time.Now().UTC().Format(time.RFC3339),
		Smoke: c.smoke, Reps: c.reps, Warmup: c.warmup, Depth: c.depth, Threads: c.threads,
		GoroutineThreads: goroutineThreads, GoroutineDepth: goroutineDepth,
		Command: c.command, Machine: m, CXX: c.cxx, CXXFlags: c.cxxflags,
		GatePhase: c.gatePhase(), TimeoutMS: c.timeout.Milliseconds(),
	}
	rep.Commit, rep.Dirty = gitState(c.root)

	// Build first, measure afterwards: a build in the middle of the runs would
	// load the machine and show up in the numbers.
	type built struct {
		program program
		flavors []flavorResult
	}
	all := make([]built, 0, len(c.bench))
	for _, p := range c.bench {
		var fs []flavorResult
		for _, fl := range c.flavors {
			t := time.Now()
			fr := c.build(gocppBin, p, fl)
			if fr.Status == statusOK {
				c.log("built %-12s %-3s %-10s %6.1fs %8d bytes", p.name, fl, fr.Status, time.Since(t).Seconds(), fr.BinSize)
			} else {
				c.log("built %-12s %-3s %-13s %s", p.name, fl, fr.Status, firstLine(fr.Note))
			}
			fs = append(fs, fr)
		}
		all = append(all, built{p, fs})
	}

	for _, b := range all {
		pr := programResult{Name: b.program.name, About: b.program.about, Path: filepath.Join("bench", "gc", b.program.name),
			Goroutines: b.program.goroutines, Flavors: b.flavors}
		c.runProgram(b.program, &pr)
		compareOutputs(&pr, b.program.sameAcrossThreads, c.log)
		rep.Programs = append(rep.Programs, pr)
	}

	m.LoadAfter = loadAvg()
	m.Busy = m.LoadBefore[0] > m.MaxLoad || m.LoadAfter[0] > m.MaxLoad
	rep.Machine = m
	if m.Busy {
		c.log("noisy run: %s", m.busyNote())
	}
	return rep, nil
}

// pendingRun is one (program, flavor, thread count) cell while it is measured.
type pendingRun struct {
	flavor *flavorResult
	result runResult
	argv   []string
	env    []string
}

// runProgram measures one program at every thread count, in each of its modes.
// Within a cell the flavors alternate in every round (Go, ARC, GC, then Go,
// ARC, GC, ...), as tools/gobench does, so that a change of the machine's load
// during the runs hits all flavors alike; the thread counts themselves follow
// one after the other.
func (c *config) runProgram(p program, pr *programResult) {
	rounds := c.warmup + c.reps
	for _, mode := range modesOf(p) {
		for _, threads := range c.threadsFor(p) {
			var cells []*pendingRun
			for fi := range pr.Flavors {
				fr := &pr.Flavors[fi]
				if fr.Status != statusOK {
					continue
				}
				bin := filepath.Join(c.work, "build", p.name, fr.Flavor, "prog")
				cells = append(cells, &pendingRun{
					flavor: fr,
					result: runResult{Threads: threads, Mode: mode},
					argv:   append([]string{bin}, c.programArgs(p, mode, threads)...),
					env:    c.runEnv(p, fr.Flavor, threads),
				})
			}
			for r := 0; r < rounds; r++ {
				for _, cell := range cells {
					c.runOnce(p, cell, r < c.warmup)
				}
			}
			for _, cell := range cells {
				c.finishRun(p, cell)
				cell.flavor.Runs = append(cell.flavor.Runs, cell.result)
			}
		}
	}
}

// runEnv is the environment of one measured run: the shell's without the
// variables that would change the program, with GOMAXPROCS set to the thread
// count for Go (so its scheduler sees the processors the row names) and the
// gocpp variables removed for the native programs. The goroutines program's
// "thread" count is a goroutine count, not a processor count: its Go runs keep
// GOMAXPROCS at 16, the harness's thread cap, at every count.
func (c *config) runEnv(p program, flavor string, threads int) []string {
	env := cleanEnv(os.Environ())
	if flavor == "go" {
		procs := threads
		if p.goroutines {
			procs = 16
		}
		env = envOver(env, fmt.Sprintf("GOMAXPROCS=%d", procs))
	}
	return env
}

// runOnce runs one round of a cell. A warm-up round is dropped after it ran;
// a failure or a change of the output between the measured rounds stops the
// cell (the remaining rounds are skipped).
func (c *config) runOnce(p program, cell *pendingRun, warm bool) {
	if cell.result.Status != "" && cell.result.Status != runOK {
		return
	}
	s, out, err := runTimed(cell.argv, cell.env, c.timeout)
	if err != nil {
		cell.result.Status, cell.result.Note = runFailed, err.Error()
		return
	}
	if warm {
		return
	}
	if out == "" {
		cell.result.Status, cell.result.Note = runMismatch, "the program printed nothing with -quiet"
		return
	}
	if cell.result.Output == "" {
		cell.result.Output = out
	} else if out != cell.result.Output {
		cell.result.Status = runMismatch
		cell.result.Note = fmt.Sprintf("the output changed between runs: %q then %q", firstLine(cell.result.Output), firstLine(out))
		return
	}
	cell.result.Samples = append(cell.result.Samples, s)
}

// finishRun summarizes a measured cell and appends it to its flavor.
func (c *config) finishRun(p program, cell *pendingRun) {
	if cell.result.Status == "" {
		cell.result.Status = runOK
	}
	cell.result.Wall, cell.result.CPU, cell.result.RSS = statOf(cell.result.Samples)
	mode := ""
	if cell.result.Mode != "" {
		mode = cell.result.Mode + " "
	}
	c.log("ran %-12s t=%-5d %s%-3s %s", p.name, cell.result.Threads, mode, cell.flavor.Flavor, runCell(&cell.result))
}

// statOf computes the wall, CPU and resident-set stats of samples.
func statOf(samples []sample) (wall, cpu, rss stat) {
	var w, c, r []int64
	for _, s := range samples {
		w = append(w, s.WallNS)
		c = append(c, s.CPUNS)
		if s.RSSKB > 0 {
			r = append(r, s.RSSKB)
		}
	}
	return summarize(w), summarize(c), summarize(r)
}

// compareOutputs checks the correctness rules: every flavor that ran a cell
// (a mode and thread count) must have printed the same deterministic output
// there, and a program whose output must not depend on the thread count
// (binarytrees' checksum) must have printed the same output at every thread
// count. A difference marks the rows as a mismatch and names the outputs.
func compareOutputs(pr *programResult, sameAcross bool, log func(string, ...any)) {
	for _, key := range cellKeys(pr) {
		var ok []*flavorResult
		var outputs []string
		for fi := range pr.Flavors {
			fr := &pr.Flavors[fi]
			if fr.Status != statusOK {
				continue
			}
			rr := findRun(fr, key.mode, key.threads)
			if rr == nil || rr.Status != runOK {
				continue
			}
			ok = append(ok, fr)
			outputs = append(outputs, rr.Output)
		}
		if len(ok) < 2 || allEqual(outputs) {
			continue
		}
		detail := make([]string, len(ok))
		for i, fr := range ok {
			detail[i] = fr.Flavor + " " + firstLine(outputs[i])
		}
		for _, fr := range ok {
			rr := findRun(fr, key.mode, key.threads)
			rr.Status = runMismatch
			rr.Note = "the flavors printed different output: " + strings.Join(detail, ", ")
		}
		log("MISMATCH %s %s: %s", pr.Name, key.label(), strings.Join(detail, ", "))
	}
	if !sameAcross {
		return
	}
	// The checksum must not depend on the thread count: a partition bug that
	// dropped or double-counted work would otherwise pass, because Go and ARC
	// run the same wrong program.
	var detail []string
	var runs []*runResult
	for fi := range pr.Flavors {
		fr := &pr.Flavors[fi]
		if fr.Status != statusOK {
			continue
		}
		for ri := range fr.Runs {
			rr := &fr.Runs[ri]
			if rr.Status != runOK {
				continue
			}
			runs = append(runs, rr)
			detail = append(detail, fmt.Sprintf("%s t=%d %s", fr.Flavor, rr.Threads, firstLine(rr.Output)))
		}
	}
	if len(runs) < 2 || allEqual(outputsOf(runs)) {
		return
	}
	note := "the output must be the same at every thread count: " + strings.Join(detail, ", ")
	for _, rr := range runs {
		rr.Status, rr.Note = runMismatch, note
	}
	log("MISMATCH %s: %s", pr.Name, note)
}

// outputsOf returns the outputs of runs.
func outputsOf(runs []*runResult) []string {
	out := make([]string, len(runs))
	for i, rr := range runs {
		out[i] = rr.Output
	}
	return out
}

// cellKey names one measured cell: a mode (empty when the program has none)
// and a thread count (a goroutine count for the goroutines program).
type cellKey struct {
	mode    string
	threads int
}

// label formats a cell for a log line.
func (k cellKey) label() string {
	if k.mode == "" {
		return fmt.Sprintf("t=%d", k.threads)
	}
	return fmt.Sprintf("%s t=%d", k.mode, k.threads)
}

// cellKeys lists the cells of a program's runs: every mode in run order, and
// the thread counts ascending within it.
func cellKeys(pr *programResult) []cellKey {
	var modes []string
	seenMode := map[string]bool{}
	for _, fr := range pr.Flavors {
		for _, rr := range fr.Runs {
			if !seenMode[rr.Mode] {
				seenMode[rr.Mode] = true
				modes = append(modes, rr.Mode)
			}
		}
	}
	var out []cellKey
	for _, mode := range modes {
		for _, threads := range threadCounts(pr, mode) {
			out = append(out, cellKey{mode, threads})
		}
	}
	return out
}

// modeList lists the modes of a program's runs, in run order. A program with
// one unnamed run has the single empty mode.
func modeList(pr *programResult) []string {
	var out []string
	seen := map[string]bool{}
	for _, fr := range pr.Flavors {
		for _, rr := range fr.Runs {
			if !seen[rr.Mode] {
				seen[rr.Mode] = true
				out = append(out, rr.Mode)
			}
		}
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}

// threadCounts lists the thread counts of a program's runs in mode, ascending.
func threadCounts(pr *programResult, mode string) []int {
	seen := map[int]bool{}
	var out []int
	for _, fr := range pr.Flavors {
		for _, rr := range fr.Runs {
			if rr.Mode == mode && !seen[rr.Threads] {
				seen[rr.Threads] = true
				out = append(out, rr.Threads)
			}
		}
	}
	sort.Ints(out)
	return out
}

// findRun returns the run of fr at (mode, threads), or nil.
func findRun(fr *flavorResult, mode string, threads int) *runResult {
	for i := range fr.Runs {
		if fr.Runs[i].Mode == mode && fr.Runs[i].Threads == threads {
			return &fr.Runs[i]
		}
	}
	return nil
}

// allEqual reports whether every string of xs is the same.
func allEqual(xs []string) bool {
	for _, x := range xs[1:] {
		if x != xs[0] {
			return false
		}
	}
	return true
}

// exitCode returns 1 when a flavor failed to build or run, or the flavors
// printed different output; 0 otherwise. A GC flavor that is not available yet
// is not a failure.
func (r *report) exitCode() int {
	for _, pr := range r.Programs {
		for _, fr := range pr.Flavors {
			if fr.Status == statusBuildFailed {
				return 1
			}
			for _, rr := range fr.Runs {
				if rr.Status != runOK {
					return 1
				}
			}
		}
	}
	return 0
}
