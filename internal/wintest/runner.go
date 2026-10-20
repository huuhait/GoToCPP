package wintest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/wintest/agent"
)

// An Item is one test of the suite.
type Item struct {
	Name     string // unique: "run/algo_bfs"
	Group    string // "run", "pkgs": the folder of tests/ it comes from
	Phase    int
	Key      string // what a result depends on (the sources and the expectation); resume keeps a pass only for an equal key
	Args     []string
	Env      []string // KEY=value
	Stdin    []byte
	Timeout  time.Duration // zero: Config.Timeout
	Skip     string        // not run at all: why
	SkipKind string        // the kind of the reason, for the summary
	Xfail    string        // expected to fail: why. It runs, and a pass is an error (stale entry)
}

// An Outcome is what a program did on the machine.
type Outcome struct {
	Stdout, Stderr []byte
	Exit           int
	TimedOut       bool
	Limit          bool   // it printed more than Config.MaxOutput bytes and was ended
	Err            string // it did not start
	Elapsed        time.Duration
}

// Config is what a Runner needs.
type Config struct {
	Remote   Remote
	Lock     Locker // nil: no lock
	AgentExe string // the agent built for the machine (local path)
	Work     string // a local folder for the archives (removed by the caller)

	Jobs      int           // programs that run at once on the machine (default 8)
	BuildJobs int           // programs built at once here (default 8)
	BatchSize int           // programs per archive (default 150)
	Timeout   time.Duration // for a program without its own (default 2m)
	MaxOutput int64         // bytes of stdout or stderr kept (default 8 MiB)
	Retries   int           // new tries of a batch after a failure of the machine or the connection (default 1; -1 none)
	StaleAge  time.Duration // CleanStale's age (default 12h)
	Transfer  time.Duration // the longest a copy of an archive may take (default 20m)
	// WorkFree returns the free bytes of the folder Work; nil means unknown. The run stops when there is
	// less than twice the size of the last batch (or of a first batch of BatchSize 12 MiB programs).
	WorkFree func() (int64, error)

	ResultsPath string
	Generation  string // the checkout's state: a resume keeps results only of the same
	Target      string // for the summary: "real PC", "Wine"
	Resume      bool   // keep the passes of the results file and run the rest
	// AnyGeneration makes Resume keep the passes of a results file from another state of the checkout
	// ("run only what failed or is new" after a fix: the passes are assumed to still hold).
	AnyGeneration bool

	// Build builds the program of the item and returns the path of its .exe. An error is a build failure
	// (a test failure, not an error of the run).
	Build func(ctx context.Context, it Item) (exe string, err error)
	// Check returns the ways the outcome differs from the expectation; none means the test passed. It
	// is not called for a program that timed out, printed too much or did not start.
	Check func(it Item, o Outcome) []string
	Log   func(string)
}

// A Runner runs a list of items.
type Runner struct{ c Config }

// New returns a Runner; it fills in the defaults.
func New(c Config) *Runner {
	if c.Jobs < 1 {
		c.Jobs = 8
	}
	if c.BuildJobs < 1 {
		c.BuildJobs = 8
	}
	if c.BatchSize < 1 {
		c.BatchSize = 150
	}
	if c.Timeout <= 0 {
		c.Timeout = 2 * time.Minute
	}
	if c.MaxOutput <= 0 {
		c.MaxOutput = 8 << 20
	}
	if c.Retries == 0 {
		c.Retries = 1
	} else if c.Retries < 0 {
		c.Retries = 0
	}
	if c.StaleAge <= 0 {
		c.StaleAge = 12 * time.Hour
	}
	if c.Transfer <= 0 {
		c.Transfer = 20 * time.Minute
	}
	if c.Log == nil {
		c.Log = func(string) {}
	}
	return &Runner{c: c}
}

func (r *Runner) logf(format string, args ...any) { r.c.Log(fmt.Sprintf(format, args...)) }

// Select returns the items to run now and the results document that holds the rest: the skipped tests,
// and with Resume the passes (and expected failures that failed) of an earlier run of the same
// checkout. It is Run's first step and the whole of a dry run.
func (r *Runner) Select(items []Item) (todo []Item, res *Results, resumed int, err error) {
	seen := make(map[string]bool)
	for _, it := range items {
		if seen[it.Name] {
			return nil, nil, 0, fmt.Errorf("the test %s is listed twice", it.Name)
		}
		seen[it.Name] = true
	}
	res = NewResults(r.c.Generation, r.c.Target)
	var prev *Results
	if r.c.Resume {
		old, ok, err := LoadResults(r.c.ResultsPath)
		if err != nil {
			return nil, nil, 0, err
		}
		switch {
		case !ok:
		case old.Generation != r.c.Generation && !r.c.AnyGeneration:
			r.logf("resume: %s is from another state of the checkout (%s, now %s): running everything", r.c.ResultsPath, old.Generation, r.c.Generation)
		default:
			prev = old
		}
	}
	sorted := append([]Item(nil), items...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	for _, it := range sorted {
		if it.Skip != "" {
			res.Tests[it.Name] = &Entry{Status: Skip, Group: it.Group, Phase: it.Phase, Kind: it.SkipKind, Reason: it.Skip}
			continue
		}
		if prev != nil {
			if e := prev.Tests[it.Name]; e != nil && e.Key == it.Key && (e.Status == Pass || e.Status == XFail) && (e.Status == XFail) == (it.Xfail != "") {
				res.Tests[it.Name] = e
				resumed++
				continue
			}
		}
		todo = append(todo, it)
	}
	return todo, res, resumed, nil
}

type built struct {
	it  Item
	id  int
	exe string
}

// Run builds, runs and judges the items. The error is a failure of the run itself (it could not start
// the machine, the context ended); a test that fails is in the report. The results file is written
// after every batch.
func (r *Runner) Run(ctx context.Context, items []Item) (*Report, error) {
	rep, err := r.run(ctx, items)
	if rep != nil && err != nil && rep.Incomplete == "" {
		rep.Incomplete = err.Error()
	}
	return rep, err
}

func (r *Runner) run(ctx context.Context, items []Item) (*Report, error) {
	todo, res, resumed, err := r.Select(items)
	if err != nil {
		return nil, err
	}
	rep := &Report{Results: res, Resumed: resumed, Planned: len(items)}
	save := func() error { return res.Save(r.c.ResultsPath) }
	if err := save(); err != nil {
		return rep, err
	}
	if len(todo) == 0 {
		return rep, nil
	}
	if err := r.start(ctx); err != nil {
		rep.Incomplete = "the run did not start: " + err.Error()
		return rep, err
	}
	var batches [][]Item
	for i := 0; i < len(todo); i += r.c.BatchSize {
		batches = append(batches, todo[i:min(i+r.c.BatchSize, len(todo))])
	}
	ctx, cancel := context.WithCancel(ctx)
	builderDone := make(chan struct{})
	// Run returns only when the builder has stopped: nothing writes into Work afterwards.
	defer func() { cancel(); <-builderDone }()
	// The builder runs one batch ahead of the machine: it builds the next batch while this one runs.
	type prepared struct {
		bs  []built
		err error
	}
	ready := make(chan prepared, 1)
	var mu sync.Mutex // res
	go func() {
		defer close(builderDone)
		defer close(ready)
		var lastBytes int64
		for i, b := range batches {
			if err := r.checkWork(lastBytes, len(b)); err != nil {
				select {
				case ready <- prepared{err: err}:
				case <-ctx.Done():
				}
				return
			}
			t0 := time.Now()
			bs := r.buildBatch(ctx, b, res, &mu)
			lastBytes = 0
			for _, x := range bs {
				if fi, err := os.Stat(x.exe); err == nil {
					lastBytes += fi.Size()
				}
			}
			r.logf("batch %d/%d: built %d of %d programs in %v", i+1, len(batches), len(bs), len(b), time.Since(t0).Round(time.Millisecond))
			select {
			case ready <- prepared{bs: bs}:
			case <-ctx.Done():
				return
			}
		}
	}()
	n := 0
	for pb := range ready {
		if pb.err != nil {
			cancel()
			return rep, pb.err
		}
		bs := pb.bs
		n++
		if len(bs) > 0 {
			t0 := time.Now()
			outs, errs, fatal := r.execBatch(ctx, n, bs)
			if fatal != nil {
				cancel()
				return rep, fatal
			}
			r.logf("batch %d/%d: %d programs packed, copied, run and fetched in %v", n, len(batches), len(bs), time.Since(t0).Round(time.Millisecond))
			mu.Lock()
			for _, b := range bs {
				r.judge(res, b, outs[b.id], errs[b.id])
			}
			mu.Unlock()
		}
		mu.Lock()
		err := save()
		mu.Unlock()
		if err != nil {
			return rep, err
		}
		r.logf("batch %d/%d done", n, len(batches))
	}
	if ctx.Err() != nil {
		return rep, ctx.Err()
	}
	return rep, nil
}

// start cleans old folders and uploads the agent.
func (r *Runner) start(ctx context.Context) error {
	unlock, err := r.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	if err := r.c.Remote.CleanStale(ctx, r.c.StaleAge); err != nil {
		r.logf("cleaning old run folders failed (ignored): %v", err)
	}
	for try := 0; ; try++ {
		pctx, cancel := context.WithTimeout(ctx, r.c.Transfer)
		err := r.c.Remote.Put(pctx, r.c.AgentExe, AgentName)
		cancel()
		if err == nil {
			return nil
		}
		if try >= r.c.Retries || ctx.Err() != nil {
			return fmt.Errorf("copying the agent to the machine: %v", err)
		}
		r.logf("copying the agent failed, trying again: %v", err)
	}
}

func (r *Runner) lock(ctx context.Context) (func(), error) {
	if r.c.Lock == nil {
		return func() {}, nil
	}
	return r.c.Lock.Lock(ctx)
}

// buildBatch builds the items in parallel. A build that fails is a result at once.
func (r *Runner) buildBatch(ctx context.Context, b []Item, res *Results, mu *sync.Mutex) []built {
	out := make([]built, len(b))
	ok := make([]bool, len(b))
	sem := make(chan struct{}, r.c.BuildJobs)
	var wg sync.WaitGroup
	for i, it := range b {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			exe, err := r.c.Build(ctx, it)
			if err != nil {
				mu.Lock()
				defer mu.Unlock()
				r.finish(res, it, false, "it does not build for Windows: "+firstLine(err.Error()), err.Error(), 0)
				return
			}
			out[i], ok[i] = built{it: it, id: i + 1, exe: exe}, true
		}()
	}
	wg.Wait()
	var bs []built
	for i := range out {
		if ok[i] {
			bs = append(bs, out[i])
		}
	}
	// Ids are the positions in this list, from 1.
	for i := range bs {
		bs[i].id = i + 1
	}
	return bs
}

// finish records the result of a test: passed or not, with the expected-failure rule.
func (r *Runner) finish(res *Results, it Item, passed bool, reason, detail string, ms int64) {
	e := &Entry{Group: it.Group, Phase: it.Phase, Key: it.Key, Ms: ms}
	switch {
	case it.Xfail != "" && passed:
		e.Status, e.Reason = XPass, it.Xfail
	case it.Xfail != "":
		e.Status, e.Reason, e.Detail = XFail, it.Xfail, shorten(detail)
	case passed:
		e.Status = Pass
	default:
		e.Status, e.Reason, e.Detail = Fail, reason, shorten(detail)
	}
	res.Tests[it.Name] = e
}

// judge turns an outcome into a result.
func (r *Runner) judge(res *Results, b built, o *Outcome, err error) {
	it := b.it
	if o == nil {
		reason := "no result came back from the machine"
		if err != nil {
			reason = "the machine or the connection failed: " + firstLine(err.Error())
		}
		res.Tests[it.Name] = &Entry{Status: Error, Group: it.Group, Phase: it.Phase, Key: it.Key, Reason: reason, Detail: shorten(fmt.Sprint(err))}
		return
	}
	ms := o.Elapsed.Milliseconds()
	switch {
	case o.Err != "":
		r.finish(res, it, false, o.Err, o.Err, ms)
	case o.TimedOut:
		msg := fmt.Sprintf("it timed out after %v", r.timeout(it))
		r.finish(res, it, false, msg, msg+"\nstdout so far:\n"+head(o.Stdout)+"\nstderr so far:\n"+head(o.Stderr), ms)
	case o.Limit:
		msg := fmt.Sprintf("it printed more than %d MiB and was ended", r.c.MaxOutput>>20)
		r.finish(res, it, false, msg, msg, ms)
	default:
		problems := r.c.Check(it, *o)
		if len(problems) == 0 {
			r.finish(res, it, true, "", "", ms)
		} else {
			r.finish(res, it, false, firstLine(problems[0]), strings.Join(problems, "\n"), ms)
		}
	}
}

func (r *Runner) timeout(it Item) time.Duration {
	if it.Timeout > 0 {
		return it.Timeout
	}
	return r.c.Timeout
}

// execBatch packs, copies, runs and fetches one batch, trying again after a failure of the machine or
// the connection (the agent resumes the batch: it runs only the programs that have no result yet).
func (r *Runner) execBatch(ctx context.Context, n int, bs []built) (outs map[int]*Outcome, errs map[int]error, fatal error) {
	name := fmt.Sprintf("b%d", n)
	outs = make(map[int]*Outcome)
	errs = make(map[int]error)
	tgz := filepath.Join(r.c.Work, name+".tgz")
	size, err := r.pack(tgz, bs)
	if err != nil {
		for _, b := range bs {
			errs[b.id] = fmt.Errorf("packing the batch: %v", err)
		}
		return outs, errs, nil
	}
	defer os.Remove(tgz)
	if err := r.checkMachine(ctx, size, tgz); err != nil {
		return nil, nil, err
	}
	for _, b := range bs {
		os.Remove(b.exe) // the archive has it: the disk is for the next batch
	}
	var maxTimeout time.Duration
	for _, b := range bs {
		maxTimeout = max(maxTimeout, r.timeout(b.it))
	}
	waves := (len(bs) + r.c.Jobs - 1) / r.c.Jobs
	budget := time.Duration(waves)*maxTimeout + 3*time.Minute
	var lastErr error
	for try := 0; try <= r.c.Retries; try++ {
		if ctx.Err() != nil {
			lastErr = ctx.Err()
			break
		}
		if try > 0 {
			r.logf("batch %s: trying again after: %v", name, lastErr)
			r.c.Remote.KillAll(ctx)
		}
		got, err := r.once(ctx, name, tgz, budget)
		if err == nil {
			for id, o := range got {
				outs[id] = o
			}
			lastErr = nil
			break
		}
		lastErr = err
	}
	if lastErr != nil {
		r.c.Remote.KillAll(ctx)
		for _, b := range bs {
			errs[b.id] = lastErr
		}
	}
	rm, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
	defer cancel()
	r.c.Remote.Remove(rm, name+".tgz", name, name+"-res.tgz")
	os.Remove(filepath.Join(r.c.Work, name+"-res.tgz"))
	return outs, errs, nil
}

// checkWork stops the run when the folder for the archives and the built programs has less than twice
// the room that the last batch (or a first batch of 12 MiB programs) needed.
func (r *Runner) checkWork(lastBytes int64, n int) error {
	if r.c.WorkFree == nil {
		return nil
	}
	need := lastBytes
	if need == 0 {
		need = int64(n) * (12 << 20)
	}
	need *= 2
	free, err := r.c.WorkFree()
	if err != nil || free < 0 {
		return nil
	}
	if free < need {
		return fmt.Errorf("not enough free disk space for the builds here (%s): %s free, %s needed (twice the size of a batch)", r.c.Work, size(free), size(need))
	}
	return nil
}

// checkMachine stops the run when the machine has less free room than twice the batch's programs plus its archive.
func (r *Runner) checkMachine(ctx context.Context, programs int64, tgz string) error {
	fctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	free, err := r.c.Remote.FreeBytes(fctx)
	if err != nil {
		r.logf("the free disk space of the machine could not be read (ignored): %v", err)
		return nil
	}
	if free < 0 {
		return nil
	}
	var archive int64
	if fi, err := os.Stat(tgz); err == nil {
		archive = fi.Size()
	}
	need := 2*programs + archive
	if free < need {
		return fmt.Errorf("not enough free disk space on the machine: %s free, %s needed (twice the programs of a batch, %s, plus its archive, %s)", size(free), size(need), size(programs), size(archive))
	}
	return nil
}

// once is one try: lock the machine, copy, run, fetch.
func (r *Runner) once(ctx context.Context, name, tgz string, budget time.Duration) (map[int]*Outcome, error) {
	unlock, err := r.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	pctx, cancel := context.WithTimeout(ctx, r.c.Transfer)
	err = r.c.Remote.Put(pctx, tgz, name+".tgz")
	cancel()
	if err != nil {
		return nil, fmt.Errorf("copying the batch: %v", err)
	}
	t0 := time.Now()
	run, err := r.c.Remote.Agent(ctx, budget, "run", name)
	if err == nil {
		r.logf("batch %s: the agent took %v on the machine: %s", name, time.Since(t0).Round(time.Millisecond), strings.TrimSpace(run.Stdout))
	}
	if err != nil {
		return nil, fmt.Errorf("running the batch: %v", err)
	}
	if run.TimedOut {
		return nil, fmt.Errorf("the batch did not end in %v (the agent was stopped)", budget)
	}
	if run.Code != 0 {
		return nil, fmt.Errorf("the agent failed (exit %d): %s%s", run.Code, run.Stdout, run.Stderr)
	}
	local := filepath.Join(r.c.Work, name+"-res.tgz")
	gctx, cancel := context.WithTimeout(ctx, r.c.Transfer)
	err = r.c.Remote.Get(gctx, name+"-res.tgz", local)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("fetching the results: %v", err)
	}
	return r.readResults(local)
}

// pack writes the archive of a batch: the manifest and the programs.
func (r *Runner) pack(path string, bs []built) (programs int64, err error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	zw, _ := gzip.NewWriterLevel(f, gzip.BestSpeed)
	tw := tar.NewWriter(zw)
	m := agent.Manifest{Jobs: r.c.Jobs, MaxOutput: r.c.MaxOutput}
	for _, b := range bs {
		m.Tests = append(m.Tests, agent.Spec{ID: b.id, Name: b.it.Name, Args: b.it.Args, Env: b.it.Env,
			Stdin: b.it.Stdin != nil, TimeoutMs: r.timeout(b.it).Milliseconds()})
	}
	data, err := json.Marshal(m)
	if err != nil {
		return 0, err
	}
	add := func(name string, size int64, src io.Reader) error {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: size}); err != nil {
			return err
		}
		_, err := io.Copy(tw, src)
		return err
	}
	if err := add(agent.ManifestName, int64(len(data)), bytes.NewReader(data)); err != nil {
		return 0, err
	}
	for _, b := range bs {
		fi, err := os.Stat(b.exe)
		if err != nil {
			return 0, err
		}
		programs += fi.Size()
		src, err := os.Open(b.exe)
		if err != nil {
			return 0, err
		}
		err = add(agent.ExeDir+"/"+strconv.Itoa(b.id)+".exe", fi.Size(), src)
		src.Close()
		if err != nil {
			return 0, err
		}
		if b.it.Stdin != nil {
			if err := add(agent.StdinDir+"/"+strconv.Itoa(b.id), int64(len(b.it.Stdin)), bytes.NewReader(b.it.Stdin)); err != nil {
				return 0, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return 0, err
	}
	if err := zw.Close(); err != nil {
		return 0, err
	}
	return programs, f.Close()
}

// readResults reads the result archive of a batch.
func (r *Runner) readResults(path string) (map[int]*Outcome, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("the result archive: %v", err)
	}
	tr := tar.NewReader(zr)
	files := make(map[string][]byte)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("the result archive: %v", err)
		}
		data, err := io.ReadAll(io.LimitReader(tr, r.c.MaxOutput+1))
		if err != nil {
			return nil, fmt.Errorf("the result archive: %v", err)
		}
		files[h.Name] = data
	}
	outs := make(map[int]*Outcome)
	for _, line := range bytes.Split(files[agent.ResultsName], []byte{'\n'}) {
		var l agent.Line
		if len(line) == 0 || json.Unmarshal(line, &l) != nil {
			continue
		}
		o := &Outcome{Exit: l.Exit, TimedOut: l.TimedOut, Limit: l.Limit, Err: l.Err, Elapsed: time.Duration(l.Ms) * time.Millisecond}
		o.Stdout = files[strconv.Itoa(l.ID)+".out"]
		o.Stderr = files[strconv.Itoa(l.ID)+".err"]
		outs[l.ID] = o
	}
	if len(outs) == 0 && len(files) == 0 {
		return nil, errors.New("the result archive is empty")
	}
	return outs, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}

func shorten(s string) string {
	const max = 4000
	if len(s) > max {
		return s[:max] + "\n... (cut)"
	}
	return s
}

func head(b []byte) string { return shorten(string(b)) }

// size writes a number of bytes for a message.
func size(n int64) string {
	switch {
	case n >= 10<<20:
		return fmt.Sprintf("%d MiB", n>>20)
	case n >= 10<<10:
		return fmt.Sprintf("%d KiB", n>>10)
	}
	return fmt.Sprintf("%d B", n)
}
