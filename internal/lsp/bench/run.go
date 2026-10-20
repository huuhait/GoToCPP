package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// A Config says how a benchmark runs.
type Config struct {
	Runs    int           // how many times the whole session is run (the medians of the runs count); 5 by default
	Scale   float64       // multiplies the repetitions of each scenario; 1 by default
	Timeout time.Duration // the longest wait for an answer or diagnostics; one minute by default
	ColdMax time.Duration // the same for the cold open of a project; five minutes by default
	WorkDir string        // where the projects and their caches are written (the caller removes it)
	Log     func(format string, args ...any)
	// Only, if not empty, runs only the session scenarios called so (the
	// opens always run).
	Only []string
}

func (c *Config) defaults() {
	if c.Runs <= 0 {
		c.Runs = 5
	}
	if c.Scale <= 0 {
		c.Scale = 1
	}
	if c.Timeout <= 0 {
		c.Timeout = time.Minute
	}
	if c.ColdMax <= 0 {
		c.ColdMax = 5 * time.Minute
	}
	if c.Log == nil {
		c.Log = func(string, ...any) {}
	}
}

// A ScenarioResult is one scenario of one project on one server.
type ScenarioResult struct {
	Name string
	Runs []Summary // one per run
	// Samples are the measured repetitions of each run in milliseconds, in
	// the order they were taken (the warm-up ones are not in them).
	Samples [][]float64
	Agg     Aggregate
	Err     string // the first error, if any repetition failed (a timeout, an empty answer)
	Fail    int    // the repetitions that failed (they have no sample)
}

// A ProjectResult is the measurements of one server on one project.
type ProjectResult struct {
	Project              string
	Target               string
	Files                int
	Lines                int
	Scenarios            []ScenarioResult
	RSS                  float64         // the resident set at the end of a run, MiB: median of the runs
	PeakRSS              float64         // largest observed process peak across all runs, MiB
	Stats                json.RawMessage // gocpp/stats at the end of the last run (gocpp only)
	Notes                []string
	Identity             *ResultIdentity   `json:",omitempty"`
	RSSRuns, PeakRSSRuns []float64         `json:",omitempty"`
	StatsRuns            []json.RawMessage `json:",omitempty"`
}

// Scenario returns the result of the scenario called name, or nil.
func (r *ProjectResult) Scenario(name string) *ScenarioResult {
	for i := range r.Scenarios {
		if r.Scenarios[i].Name == name {
			return &r.Scenarios[i]
		}
	}
	return nil
}

// samples collects the samples of one run by series name.
type samples struct {
	by    map[string][]float64 // the measured repetitions
	first map[string]float64   // the first repetition of a series, a warm-up one included
	errs  map[string]string
	fails map[string]int
	notes []string
}

func newSamples() *samples {
	return &samples{by: map[string][]float64{}, first: map[string]float64{}, errs: map[string]string{}, fails: map[string]int{}}
}

func (s *samples) add(name string, d time.Duration) { s.by[name] = append(s.by[name], Millis(d)) }
func (s *samples) fail(name string, err error) {
	s.fails[name]++
	if s.errs[name] == "" {
		s.errs[name] = err.Error()
	}
}

// RunProject measures the server of t on the project: cfg.Runs times, a
// cold open (empty caches), a warm open (the caches of the cold one) and
// the scenarios in the warm session. The tree of the project is written to
// cfg.WorkDir.
func RunProject(ctx context.Context, cfg Config, proj *Project, t Target) (*ProjectResult, error) {
	cfg.defaults()
	dir := filepath.Join(cfg.WorkDir, proj.Spec.Name+"-"+t.Name)
	tree := filepath.Join(dir, "tree")
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := proj.Write(tree, t.Ext); err != nil {
		return nil, err
	}
	identity, err := identifyResult(proj, t)
	if err != nil {
		return nil, err
	}
	res := &ProjectResult{Project: proj.Spec.Name, Target: t.Name, Lines: proj.Lines, Identity: identity}
	for _, f := range proj.Files {
		if f.Kind == Source && !(t.Ext == ".go" && isCppPath(f.Rel)) {
			res.Files++
		}
	}
	var runs []*samples
	var rss, peak []float64
	for r := 0; r < cfg.Runs; r++ {
		cfg.Log("%s on %s: run %d of %d", t.Name, proj.Spec.Name, r+1, cfg.Runs)
		cache := filepath.Join(dir, fmt.Sprintf("cache%d", r))
		smp, mem, stats, err := runOnce(ctx, cfg, proj, t, tree, cache)
		os.RemoveAll(cache) // the cold run's caches are big; the next run is cold again
		if err != nil {
			return res, fmt.Errorf("%s on %s, run %d: %w", t.Name, proj.Spec.Name, r+1, err)
		}
		runs = append(runs, smp)
		for _, n := range smp.notes {
			if !slices.Contains(res.Notes, n) {
				res.Notes = append(res.Notes, n)
			}
		}
		rss = append(rss, float64(mem[0])/(1<<20))
		peak = append(peak, float64(mem[1])/(1<<20))
		res.Stats = stats
		res.StatsRuns = append(res.StatsRuns, stats)
	}
	res.RSSRuns, res.PeakRSSRuns = rss, peak
	res.RSS, res.PeakRSS = Median(rss), slices.Max(peak)
	names := map[string]bool{}
	for _, s := range runs {
		for n := range s.by {
			names[n] = true
		}
		for n := range s.fails {
			names[n] = true
		}
	}
	for _, name := range orderedNames(names) {
		sr := ScenarioResult{Name: name}
		for _, s := range runs {
			sum := Summarize(s.by[name])
			if f, ok := s.first[name]; ok {
				sum.First = f
			}
			sr.Runs = append(sr.Runs, sum)
			sr.Samples = append(sr.Samples, s.by[name])
			sr.Fail += s.fails[name]
			if sr.Err == "" {
				sr.Err = s.errs[name]
			}
		}
		sr.Agg = Aggregates(sr.Runs)
		res.Scenarios = append(res.Scenarios, sr)
	}
	return res, nil
}

// orderedNames returns the names in the order of the report, and the ones
// it does not know last.
func orderedNames(names map[string]bool) []string {
	var out []string
	for _, n := range order {
		if names[n] {
			out = append(out, n)
			delete(names, n)
		}
	}
	for n := range names {
		out = append(out, n)
	}
	return out
}

// runOnce is one run: a cold process, a warm process with the scenarios.
func runOnce(ctx context.Context, cfg Config, proj *Project, t Target, tree, cache string) (smp *samples, mem [2]int64, stats json.RawMessage, err error) {
	smp = newSamples()
	// The cold process: empty caches; its work fills them.
	if err := coldRun(ctx, cfg, proj, t, tree, cache, smp); err != nil {
		return nil, mem, nil, fmt.Errorf("cold: %w", err)
	}
	// The warm process: the caches the cold one left.
	s, err := startSession(ctx, cfg, proj, t, tree, cache)
	if err != nil {
		return nil, mem, nil, fmt.Errorf("warm: %w", err)
	}
	defer s.P.Kill()
	if err := s.firstOpen(ctx, cfg, smp, false); err != nil {
		return nil, mem, nil, fmt.Errorf("warm: %w", err)
	}
	if err := s.openAll(ctx); err != nil {
		return nil, mem, nil, fmt.Errorf("warm: %w", err)
	}
	for rel, d := range s.docs {
		if p := s.C.LastPublish(d.URI); p != nil && p.Errors() > 0 {
			smp.notes = append(smp.notes, fmt.Sprintf("%s: %d errors at baseline in %s, the first: %s", t.Name, p.Errors(), rel, oneLine(p.firstError())))
		}
	}
	slices.Sort(smp.notes)
	// The editor has the project open, and the user reads before the first request.
	sleep(ctx, afterOpen)
	for _, sc := range scenarios(proj) {
		if sc.gocppOnly && t.Name != "gocpp" || len(cfg.Only) > 0 && !slices.Contains(cfg.Only, sc.name) {
			continue
		}
		runScenario(ctx, cfg, s, sc, smp)
		if ctx.Err() != nil {
			return nil, mem, nil, ctx.Err()
		}
	}
	for _, f := range s.Facts {
		smp.notes = append(smp.notes, t.Name+" "+f)
	}
	slices.Sort(smp.notes)
	mem[0], mem[1] = s.P.Memory()
	if t.Name == "gocpp" {
		var raw json.RawMessage
		sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if _, err := s.C.Call(sctx, "gocpp/stats", nil, &raw); err == nil {
			stats = raw
		}
		cancel()
	}
	s.P.Stop()
	return smp, mem, stats, nil
}

// startSession starts the server and initializes it (the time goes to
// smp as ScInit when smp is not nil).
func startSession(ctx context.Context, cfg Config, proj *Project, t Target, tree, cache string) (*Session, error) {
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return nil, err
	}
	p, err := StartProc(ctx, t, tree, cache)
	if err != nil {
		return nil, err
	}
	s := &Session{P: p, C: p.Client, Proj: proj, Root: tree, Timeout: cfg.Timeout, docs: map[string]*Doc{}, Facts: map[string]string{}}
	ictx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	d, err := s.Initialize(ictx)
	if err != nil {
		p.Kill()
		return nil, fmt.Errorf("initialize: %w (stderr: %.500s)", err, p.Stderr())
	}
	s.initTime = d
	return s, nil
}

// coldRun runs a process with empty caches through the open of the first
// files and stops it (it fills the caches that the warm run uses).
func coldRun(ctx context.Context, cfg Config, proj *Project, t Target, tree, cache string, smp *samples) error {
	s, err := startSession(ctx, cfg, proj, t, tree, cache)
	if err != nil {
		return err
	}
	defer s.P.Kill()
	if err := s.firstOpen(ctx, cfg, smp, true); err != nil {
		return err
	}
	s.P.Stop()
	return nil
}

// firstOpen records the initialize time and the open of the hot file (and,
// with C++ headers, a file that imports them and the file with a typo in a
// std name).
func (s *Session) firstOpen(ctx context.Context, cfg Config, smp *samples, cold bool) error {
	smp.add(ScInit, s.initTime)
	name, cppName := ScOpenWarm, ScCppOpenWarm
	typoFirst, typoError := ScTypoFirstWarm, ScTypoErrorWarm
	wait := cfg.Timeout
	if cold {
		name, cppName = ScOpenCold, ScCppOpenCold
		typoFirst, typoError = ScTypoFirstCold, ScTypoErrorCold
		wait = cfg.ColdMax
	}
	saved := s.Timeout
	s.Timeout = wait
	defer func() { s.Timeout = saved }()
	d, err := s.openAndWait(ctx, s.Proj.Hot)
	if err != nil {
		return err
	}
	smp.add(name, d)
	if s.Proj.CppFile != "" && s.P.Target.Name == "gocpp" {
		if d, err := s.openAndWait(ctx, s.Proj.CppFile); err != nil {
			smp.fail(cppName, err)
		} else {
			smp.add(cppName, d)
		}
		seq := s.C.Seq()
		t0, err := s.Open(s.Proj.Typo)
		if err != nil {
			return err
		}
		first, _, err := s.Wait(ctx, s.Proj.Typo, seq, t0, nil)
		if err != nil {
			smp.fail(typoFirst, err)
		} else {
			smp.add(typoFirst, first)
		}
		// The namespace probe runs in the background; its error comes when
		// it is done.
		d, _, err := s.Wait(ctx, s.Proj.Typo, seq, t0, func(p *Publish) bool {
			for _, x := range p.Diags {
				if strings.Contains(x.Message, "undefined: stdd") {
					return true
				}
			}
			return false
		})
		if err != nil {
			smp.fail(typoError, err)
		} else {
			smp.add(typoError, d)
		}
		if err := s.Close(s.Proj.Typo); err != nil {
			return err
		}
	}
	return nil
}

// openAndWait opens the file rel and waits for its first diagnostics.
func (s *Session) openAndWait(ctx context.Context, rel string) (time.Duration, error) {
	seq := s.C.Seq()
	t0, err := s.Open(rel)
	if err != nil {
		return 0, err
	}
	d, _, err := s.Wait(ctx, rel, seq, t0, nil)
	return d, err
}

// openAll opens the other documents of the scenarios and waits for their
// diagnostics, so that the first repetition of a scenario meets a server
// that has done its work for the open files.
func (s *Session) openAll(ctx context.Context) error {
	p := s.Proj
	for _, rel := range []string{p.Dep, p.Dependent, p.Core, p.Big, p.APIFile, p.APIUser} {
		if s.docs[rel] != nil && s.docs[rel].Version > 0 {
			continue
		}
		if _, err := s.openAndWait(ctx, rel); err != nil {
			return err
		}
	}
	sleep(ctx, 300*time.Millisecond) // the background work of the server
	return nil
}

// runScenario runs sc in the session and adds its samples to smp.
func runScenario(ctx context.Context, cfg Config, s *Session, sc scenario, smp *samples) {
	if sc.prep != nil {
		if err := sc.prep(ctx, s); err != nil {
			smp.fail(sc.name, fmt.Errorf("prep: %w", err))
			return
		}
	}
	iters := max(int(float64(sc.iters)*cfg.Scale+0.5), 2)
	for i := 0; i < sc.warm+iters; i++ {
		s.extra = s.extra[:0]
		series, d, err := sc.run(ctx, s, i)
		if series == "" {
			series = sc.name
		}
		if err != nil {
			smp.fail(series, err)
			cfg.Log("  %s: %v", series, err)
			if log := s.P.Stderr(); log != "" && smp.fails[series] == 1 {
				cfg.Log("  the server's log:\n%s", tail(log, 3000))
			}
			if ctx.Err() != nil || s.C.Done() {
				return
			}
			continue
		}
		if _, ok := smp.first[series]; !ok {
			smp.first[series] = Millis(d)
		}
		if i >= sc.warm { // the warm-up repetitions are not counted
			smp.add(series, d)
			for _, x := range s.extra {
				smp.add(x.series, x.d)
			}
		}
	}
	if sc.done != nil {
		if err := sc.done(ctx, s); err != nil {
			cfg.Log("  %s: putting back: %v", sc.name, err)
		}
	}
}

// tail returns the last n bytes of s.
func tail(s string, n int) string {
	if len(s) > n {
		return "..." + s[len(s)-n:]
	}
	return s
}
