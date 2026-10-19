package diff

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huuhait/gocpp/tools/difftest/gen"
)

// Config is a campaign: Programs numbered First to First+N-1 of the seed, in batches of Batch
// programs, run with the reference and each subject.
type Config struct {
	Seed           uint64
	First, N       int
	Batch          int
	Jobs           int // batches in flight (each is one build at a time)
	Gen            gen.Config
	Ref            Backend
	Subjects       []Backend
	Redo           bool           // run again the bundles that had findings or generator bugs
	TmpDir         string         // the folder of the backends' temporary folders (swept)
	CacheDir       string         // gocpp's cache folder (gocpp/ in XDG_CACHE_HOME; swept)
	SweepEvery     time.Duration  // 0: 3 minutes
	StopOnMismatch bool           // stop at the first mismatch (the mutation experiments)
	Files          int            // files per bundle: gocpp compiles them in parallel
	Every          map[string]int // subject name -> run it on every K-th batch only
	StateDir       string         // resumable state: one file per finished batch
	FindingsDir    string         // minimized repros of mismatches that are not known
	Known          *Known
	FatalSample    int // percent of the programs with a fatal tail that also run alone
	ReduceBudget   int // gocpp builds per reduction
	MaxReduce      int // reductions per cluster of equal mismatches
	VerifyRef      bool
	Log            io.Writer
}

// Stats are the counters of a campaign.
type Stats struct {
	Programs   int            `json:"programs"`
	Lines      int            `json:"lines"`
	Batches    int            `json:"batches"`
	Standalone int            `json:"standalone"`
	GenBugs    int            `json:"gen_bugs"`
	Retries    int            `json:"build_retries"`
	Flaky      int            `json:"flaky_reference"`
	Mismatches int            `json:"mismatches"`
	ByKind     map[string]int `json:"by_kind"`
	Known      map[string]int `json:"known"`
	Unknown    int            `json:"unknown"`
	Clusters   int            `json:"clusters"`
	Builds     int            `json:"builds"`
	Used       map[string]int `json:"scenarios"`
}

func newStats() *Stats {
	return &Stats{ByKind: map[string]int{}, Known: map[string]int{}, Used: map[string]int{}}
}

func (s *Stats) add(o *Stats) {
	s.Programs += o.Programs
	s.Lines += o.Lines
	s.Batches += o.Batches
	s.Standalone += o.Standalone
	s.GenBugs += o.GenBugs
	s.Retries += o.Retries
	s.Flaky += o.Flaky
	s.Mismatches += o.Mismatches
	s.Unknown += o.Unknown
	s.Builds += o.Builds
	for k, v := range o.ByKind {
		s.ByKind[k] += v
	}
	for k, v := range o.Known {
		s.Known[k] += v
	}
	for k, v := range o.Used {
		s.Used[k] += v
	}
}

// Campaign runs a Config.
type Campaign struct {
	cfg Config

	cancel   context.CancelFunc
	found    atomic.Bool
	foundMm  *Mismatch
	mu       sync.Mutex
	clusters map[string]int // cluster key -> mismatches seen (reduced or not)
	verdict  map[string]string
	logMu    sync.Mutex
	builds   atomic.Int64
}

// NewCampaign prepares a campaign.
func NewCampaign(cfg Config) *Campaign {
	if cfg.Batch <= 0 {
		cfg.Batch = 200
	}
	if cfg.Jobs <= 0 {
		cfg.Jobs = 1
	}
	if cfg.ReduceBudget <= 0 {
		cfg.ReduceBudget = 100
	}
	if cfg.MaxReduce <= 0 {
		cfg.MaxReduce = 2
	}
	if cfg.Known == nil {
		cfg.Known = &Known{}
	}
	if cfg.Log == nil {
		cfg.Log = io.Discard
	}
	cfg.Gen.Seed = cfg.Seed
	return &Campaign{cfg: cfg, clusters: map[string]int{}, verdict: map[string]string{}}
}

func (c *Campaign) logf(format string, args ...any) {
	c.logMu.Lock()
	defer c.logMu.Unlock()
	fmt.Fprintf(c.cfg.Log, format+"\n", args...)
}

type batchFile struct {
	Batch int    `json:"batch"`
	Stats *Stats `json:"stats"`
}

func (c *Campaign) batchPath(b int) string {
	return filepath.Join(c.cfg.StateDir, fmt.Sprintf("batch-%07d.json", b))
}

// Run runs every batch that has no state file yet and returns the counters of all batches
// (also of the earlier runs of a resumed campaign).
func (c *Campaign) Run(ctx context.Context) (*Stats, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c.cancel = cancel
	go c.janitor(ctx)
	if c.cfg.StateDir != "" {
		if err := os.MkdirAll(c.cfg.StateDir, 0o755); err != nil {
			return nil, err
		}
	}
	if c.cfg.FindingsDir != "" {
		if err := os.MkdirAll(c.cfg.FindingsDir, 0o755); err != nil {
			return nil, err
		}
	}
	if c.cfg.Redo {
		c.redo()
	}
	nb := (c.cfg.N + c.cfg.Batch - 1) / c.cfg.Batch
	first := c.cfg.First / c.cfg.Batch
	var todo []int
	for b := first; b < first+nb; b++ {
		if c.cfg.StateDir != "" {
			if _, err := os.Stat(c.batchPath(b)); err == nil {
				continue
			}
		}
		todo = append(todo, b)
	}
	work := make(chan int)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex
	for w := 0; w < c.cfg.Jobs; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for b := range work {
				if ctx.Err() != nil {
					continue
				}
				t0 := time.Now()
				st, err := c.runBatch(ctx, b)
				if err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
					continue
				}
				if c.cfg.StateDir != "" {
					data, _ := json.Marshal(batchFile{Batch: b, Stats: st})
					tmp := c.batchPath(b) + ".tmp"
					if os.WriteFile(tmp, data, 0o644) == nil {
						os.Rename(tmp, c.batchPath(b))
					}
				}
				c.logf("batch %d done: %d programs, %d mismatches, %s", b, st.Programs, st.Mismatches, time.Since(t0).Round(time.Second))
			}
		}()
	}
	for _, b := range todo {
		work <- b
	}
	close(work)
	wg.Wait()
	total, err := c.Totals()
	if err != nil {
		return nil, err
	}
	if c.cfg.StopOnMismatch && c.found.Load() {
		return total, nil
	}
	if firstErr != nil {
		return total, firstErr
	}
	return total, ctx.Err()
}

// redo deletes the state files of the bundles that had findings or generator bugs, so that a
// campaign run again runs them again (after a fix of gocpp or of the generator).
func (c *Campaign) redo() {
	files, _ := filepath.Glob(filepath.Join(c.cfg.StateDir, "batch-*.json"))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var bf batchFile
		if json.Unmarshal(data, &bf) == nil && bf.Stats != nil && (bf.Stats.Unknown > 0 || bf.Stats.GenBugs > 0) {
			os.Remove(f)
		}
	}
}

// Found returns the first mismatch of a campaign with StopOnMismatch, or nil.
func (c *Campaign) Found() *Mismatch {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.foundMm
}

// Totals adds up the state files.
func (c *Campaign) Totals() (*Stats, error) {
	total := newStats()
	if c.cfg.StateDir == "" {
		return total, nil
	}
	files, _ := filepath.Glob(filepath.Join(c.cfg.StateDir, "batch-*.json"))
	sort.Strings(files)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var bf batchFile
		if err := json.Unmarshal(data, &bf); err != nil {
			return nil, fmt.Errorf("%s: %v", f, err)
		}
		total.add(bf.Stats)
	}
	total.Clusters = c.countClusters()
	return total, nil
}

func (c *Campaign) countClusters() int {
	if c.cfg.FindingsDir == "" {
		return 0
	}
	files, _ := filepath.Glob(filepath.Join(c.cfg.FindingsDir, "*", "finding.md"))
	return len(files)
}

// generate makes the programs of a batch.
func (c *Campaign) generate(b int) ([]*gen.Program, error) {
	var progs []*gen.Program
	for id := b * c.cfg.Batch; id < (b+1)*c.cfg.Batch; id++ {
		if id < c.cfg.First || id >= c.cfg.First+c.cfg.N {
			continue
		}
		p, err := gen.Generate(c.cfg.Gen, id)
		if err != nil {
			return nil, err
		}
		progs = append(progs, p)
	}
	return progs, nil
}

func (c *Campaign) bundle(progs []*gen.Program) (string, error) {
	return gen.BundleFiles(progs, gen.Recovered, c.cfg.Files)
}

func bundleOf(progs []*gen.Program, mode gen.Mode) (string, error) {
	return gen.Bundle(progs, mode)
}

func (c *Campaign) runBatch(ctx context.Context, b int) (*Stats, error) {
	st := newStats()
	progs, err := c.generate(b)
	if err != nil {
		return nil, err
	}
	st.Batches = 1
	// the reference: its output defines what each program must print
	progs, refOut, err := c.reference(ctx, b, progs, st)
	if err != nil {
		return nil, err
	}
	for _, p := range progs {
		st.Programs++
		st.Lines += p.Lines
		for _, u := range p.Used {
			st.Used[u]++
		}
	}
	for _, sub := range c.cfg.Subjects {
		if k := c.cfg.Every[sub.Name()]; k > 1 && b%k != 0 {
			continue
		}
		mms, err := c.checkBundle(ctx, sub, progs, refOut, st)
		if err != nil {
			return nil, err
		}
		byProg := map[int]*gen.Program{}
		for _, p := range progs {
			byProg[p.ID] = p
		}
		for _, m := range mms {
			if err := c.handle(ctx, sub, byProg[m.Prog], m, st); err != nil {
				return nil, err
			}
		}
		// programs that end with a fatal panic also run alone, for the exit code and stderr
		for _, p := range progs {
			if p.Tail == "" || !sample(c.cfg.Seed, p.ID, c.cfg.FatalSample) {
				continue
			}
			src, err := bundleOf([]*gen.Program{p}, gen.Standalone)
			if err != nil {
				return nil, err
			}
			ref, err := c.cfg.Ref.Run(ctx, src)
			if err != nil {
				return nil, err
			}
			if !ref.BuildOK {
				continue
			}
			st.Standalone++
			c.builds.Add(1)
			got, err := c.runRetry(ctx, sub, src, st)
			if err != nil {
				return nil, err
			}
			st.Builds++
			var mm []Mismatch
			if !got.BuildOK {
				mm = []Mismatch{c.buildMismatch(p.ID, got)}
			} else {
				mm = CompareStandalone(p.ID, ref, got)
			}
			for _, m := range mm {
				m.Detail = "standalone program\n" + m.Detail
				if err := c.handleStandalone(ctx, sub, p, m, st); err != nil {
					return nil, err
				}
			}
		}
	}
	return st, nil
}

func sample(seed uint64, id, pct int) bool {
	if pct <= 0 {
		return false
	}
	if pct >= 100 {
		return true
	}
	h := sha256.Sum256([]byte(fmt.Sprintf("%d/%d", seed, id)))
	return int(h[0])*100/256 < pct
}

// retryable reports whether a failed build looks like the loss of the build's temporary folder
// (generated C++ that did not compile, or object files that are gone at the link).
func retryable(r Result) bool {
	lower := strings.ToLower(r.BuildErr)
	return r.ICE || (strings.Contains(lower, "no such file or directory") && strings.Contains(lower, "gocpp-build"))
}

// runRetry builds and runs src with the subject; a build that fails with "generated C++ did not
// compile" or loses its object files is done once more before it counts: the temporary folder of a build can be removed by
// whoever cleans the machine's /tmp (this happened on a shared machine), which no program
// causes. A real compiler error comes again.
func (c *Campaign) runRetry(ctx context.Context, sub Backend, src string, st *Stats) (Result, error) {
	res, err := sub.Run(ctx, src)
	if err != nil || res.BuildOK || !retryable(res) {
		return res, err
	}
	c.logf("%s: the generated C++ did not compile, building the bundle again: %s", sub.Name(), firstLine(res.BuildErr))
	st.Retries++
	return sub.Run(ctx, src)
}

func (c *Campaign) buildMismatch(id int, r Result) Mismatch {
	k := KindBuild
	if r.ICE {
		k = KindICE
	}
	sig := FirstError(r.BuildErr)
	if r.ICE {
		sig = iceSig(r.BuildErr)
	}
	return Mismatch{Prog: id, Kind: k, Sig: sig, Detail: clip(r.BuildErr, 3000)}
}

// reference runs the whole batch with the reference, drops the programs that the reference
// does not accept (generator bugs: recorded) or that it does not run the same twice, and
// returns the per-program output of the others.
func (c *Campaign) reference(ctx context.Context, b int, progs []*gen.Program, st *Stats) ([]*gen.Program, RefOut, error) {
	for attempt := 0; attempt < 12; attempt++ {
		src, err := c.bundle(progs)
		if err != nil {
			return nil, RefOut{}, err
		}
		res, err := c.cfg.Ref.Run(ctx, src)
		if err != nil {
			return nil, RefOut{}, err
		}
		if !res.BuildOK {
			bad := ProgsOfErrors(src, res.BuildErr)
			if len(bad) == 0 {
				// cannot attribute: bisect
				bad = c.bisectBuild(ctx, c.cfg.Ref, progs)
			}
			if len(bad) == 0 {
				return nil, RefOut{}, fmt.Errorf("batch %d: the reference does not build and no program is to blame:\n%s", b, clip(res.BuildErr, 2000))
			}
			c.recordGenBug(progs, bad, res.BuildErr, st)
			progs = without(progs, bad)
			continue
		}
		if res.Exit != 0 || res.TimedOut {
			// a program the reference cannot run in a bundle: find it by the last marker
			ids, _ := SplitOutput(res.Stdout)
			if len(ids) == 0 {
				return nil, RefOut{}, fmt.Errorf("batch %d: the reference fails at start: exit %d\n%s", b, res.Exit, clip(res.Stderr, 1000))
			}
			c.recordGenBug(progs, []int{ids[len(ids)-1]}, fmt.Sprintf("reference run failed (exit %d, timeout %v):\n%s", res.Exit, res.TimedOut, clip(res.Stderr, 1500)), st)
			progs = without(progs, []int{ids[len(ids)-1]})
			continue
		}
		ref := RefOf(res)
		parts := ref.Out
		if c.cfg.VerifyRef {
			res2, err := c.cfg.Ref.Run(ctx, src)
			if err != nil {
				return nil, RefOut{}, err
			}
			ref2 := RefOf(res2)
			var flaky []int
			for _, p := range progs {
				if parts[p.ID] != ref2.Out[p.ID] || ref.Err[p.ID] != ref2.Err[p.ID] {
					flaky = append(flaky, p.ID)
				}
			}
			if len(flaky) > 0 {
				st.Flaky += len(flaky)
				c.recordGenBug(progs, flaky, "the reference prints different output in two runs", st)
				progs = without(progs, flaky)
				continue
			}
		}
		// a panic that no scenario meant to make: the generator's program is not total
		var unexpected []int
		for _, p := range progs {
			if strings.Contains(parts[p.ID], "UNEXPECTED PANIC:") {
				unexpected = append(unexpected, p.ID)
			}
		}
		if len(unexpected) > 0 {
			why := "unexpected panic of the program: " + firstLineWith(parts[unexpected[0]], "UNEXPECTED PANIC:")
			c.recordGenBug(progs, unexpected, why, st)
			progs = without(progs, unexpected)
		}
		return progs, ref, nil
	}
	return nil, RefOut{}, fmt.Errorf("batch %d: the reference keeps failing", b)
}

// What is masked as an address: Go prints the pointers of its heap as 0xc0 and ten hexadecimal
// digits, and the addresses of its code and data (a static binary that is not position
// independent) as 0x4 to 0x9 and five digits; gocpp's programs are position independent: 0x55,
// 0x56 or 0x7f and ten digits. Any other hexadecimal number is a value, which must be equal:
// 0xdeadbeef and 0xdeadbeee are a difference.
var (
	goAddrRx  = regexp.MustCompile(`0x(?:c0[0-9a-f]{8}|[4-9][0-9a-f]{5})\b`)
	cppAddrRx = regexp.MustCompile(`0x(?:55|56|7f)[0-9a-f]{10}\b`)
)

// sameModuloAddresses reports whether the output of the reference and the output of gocpp (both
// whole) are equal once the addresses in them are masked. It is the safety net for a generated
// program that prints a pointer (the lint should have refused it): such a program is a
// generator bug, not a difference.
func sameModuloAddresses(want, got string) bool {
	if want == got {
		return false
	}
	return goAddrRx.ReplaceAllString(want, "0xADDR") == cppAddrRx.ReplaceAllString(got, "0xADDR")
}

func firstLineWith(s, sub string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			return l
		}
	}
	return ""
}

func without(progs []*gen.Program, ids []int) []*gen.Program {
	drop := map[int]bool{}
	for _, i := range ids {
		drop[i] = true
	}
	var out []*gen.Program
	for _, p := range progs {
		if !drop[p.ID] {
			out = append(out, p)
		}
	}
	return out
}

// bisectBuild finds programs that make the build fail when no error line names one.
func (c *Campaign) bisectBuild(ctx context.Context, be Backend, progs []*gen.Program) []int {
	if len(progs) == 0 {
		return nil
	}
	src, err := c.bundle(progs)
	if err != nil {
		return nil
	}
	res, err := be.Run(ctx, src)
	if err != nil || res.BuildOK {
		return nil
	}
	if len(progs) == 1 {
		return []int{progs[0].ID}
	}
	h := len(progs) / 2
	l := c.bisectBuild(ctx, be, progs[:h])
	r := c.bisectBuild(ctx, be, progs[h:])
	if len(l)+len(r) == 0 {
		return []int{progs[0].ID} // the failure needs both halves: blame the first program
	}
	return append(l, r...)
}

func (c *Campaign) recordGenBug(progs []*gen.Program, ids []int, why string, st *Stats) {
	st.GenBugs += len(ids)
	if c.cfg.FindingsDir == "" {
		return
	}
	dir := filepath.Join(c.cfg.FindingsDir, "genbugs")
	os.MkdirAll(dir, 0o755)
	for _, id := range ids {
		for _, p := range progs {
			if p.ID == id {
				name := filepath.Join(dir, fmt.Sprintf("seed%d-prog%d.go.txt", c.cfg.Seed, id))
				os.WriteFile(name, []byte("// "+strings.ReplaceAll(clip(why, 1500), "\n", "\n// ")+"\n\n"+p.Src), 0o644)
			}
		}
	}
	c.logf("generator bug: programs %v: %s", ids, firstLine(why))
}

// checkBundle runs the programs with the subject and returns the mismatches, running again
// the programs that a crash cut off and leaving out the programs that do not build.
func (c *Campaign) checkBundle(ctx context.Context, sub Backend, progs []*gen.Program, refOut RefOut, st *Stats) ([]Mismatch, error) {
	var all []Mismatch
	pending := progs
	for round := 0; len(pending) > 0 && round < 40; round++ {
		src, err := c.bundle(pending)
		if err != nil {
			return nil, err
		}
		res, err := c.runRetry(ctx, sub, src, st)
		if err != nil {
			return nil, err
		}
		c.builds.Add(1)
		st.Builds++
		ids := make([]int, len(pending))
		for i, p := range pending {
			ids[i] = p.ID
		}
		if !res.BuildOK {
			bad := ProgsOfErrors(src, res.BuildErr)
			if len(bad) == 0 {
				bad = c.bisectBuild(ctx, sub, pending)
			}
			if len(bad) == 0 {
				bad = []int{pending[0].ID}
			}
			for _, id := range bad {
				// the error text of the whole bundle names several programs: keep the
				// lines of this one
				r := res
				r.BuildErr = ErrorOfProg(src, res.BuildErr, id)
				all = append(all, c.buildMismatch(id, r))
			}
			pending = without(pending, bad)
			continue
		}
		mm, unrun := CompareBundle(ids, refOut, res)
		all = append(all, mm...)
		if len(unrun) == 0 {
			break
		}
		// the subject died; run the programs after the dead one again
		var next []*gen.Program
		un := map[int]bool{}
		for _, id := range unrun {
			un[id] = true
		}
		for _, p := range pending {
			if un[p.ID] {
				next = append(next, p)
			}
		}
		pending = next
	}
	return all, nil
}

// ---- mismatches: cluster, reduce, classify ----

func clusterKey(m Mismatch) string {
	// the scenario tells numeric-only differences ("- 5 / + 6") apart
	h := sha256.Sum256([]byte(string(m.Kind) + "\x00" + MaskSig(m.Sig) + "\x00" + m.Scenario))
	return string(m.Kind) + "-" + hex.EncodeToString(h[:5])
}

func (c *Campaign) handle(ctx context.Context, sub Backend, p *gen.Program, m Mismatch, st *Stats) error {
	return c.handleWith(ctx, sub, p, m, st, gen.Recovered)
}

func (c *Campaign) handleStandalone(ctx context.Context, sub Backend, p *gen.Program, m Mismatch, st *Stats) error {
	return c.handleWith(ctx, sub, p, m, st, gen.Standalone)
}

func (c *Campaign) handleWith(ctx context.Context, sub Backend, p *gen.Program, m Mismatch, st *Stats, mode gen.Mode) error {
	if m.Kind == KindOutput && p != nil && m.WantFull != "" && sameModuloAddresses(m.WantFull, m.GotFull) {
		// the program printed an address (a func, a pointer): a generator bug, not gocpp's
		c.recordGenBug([]*gen.Program{p}, []int{p.ID}, "the output differs only in addresses:\n"+m.Sig, st)
		return nil
	}
	if c.cfg.StopOnMismatch {
		c.mu.Lock()
		if c.foundMm == nil {
			c.foundMm = &m
		}
		c.mu.Unlock()
		c.found.Store(true)
		c.cancel()
		st.Mismatches++
		st.ByKind[string(m.Kind)]++
		st.Unknown++
		return nil
	}
	st.Mismatches++
	st.ByKind[string(m.Kind)]++
	key := clusterKey(m)
	c.mu.Lock()
	n := c.clusters[key]
	c.clusters[key] = n + 1
	verdict, decided := c.verdict[key]
	c.mu.Unlock()
	if decided && n >= c.cfg.MaxReduce {
		// The cluster is already understood. A member of a known cluster still has to match
		// the known entry itself (its source: a leak is a leak, but only the closure cycle
		// of the D1 entry is known); one that does not is reduced like a new cluster.
		if !strings.HasPrefix(verdict, "known:") {
			st.Unknown++
			return nil
		}
		if p != nil {
			if e := c.cfg.Known.Match(m, p.Src); e != nil {
				st.Known[e.Name]++
				return nil
			}
		}
	}
	if p == nil {
		return fmt.Errorf("mismatch for unknown program %d", m.Prog)
	}
	r := &Reducer{Ref: c.cfg.Ref, Sub: sub, Budget: c.cfg.ReduceBudget, Mode: mode, Log: c.cfg.Log}
	min, mm, builds, err := r.Reduce(ctx, p, m)
	if err != nil {
		return err
	}
	st.Builds += builds
	c.builds.Add(int64(builds))
	if min == "" { // does not reproduce alone: keep the batch interaction
		min = p.Src
		mm.Detail += "\n(does not reproduce when the program runs alone)"
	}
	if k := c.cfg.Known.Match(mm, min); k != nil {
		c.mu.Lock()
		c.verdict[key] = "known:" + k.Name
		c.mu.Unlock()
		st.Known[k.Name]++
		c.logf("known %s: %s", k.Name, mm)
		return nil
	}
	c.mu.Lock()
	c.verdict[key] = "unknown"
	c.mu.Unlock()
	st.Unknown++
	c.logf("FINDING %s: %s", key, mm)
	return c.writeFinding(sub, key, p, mm, min, mode)
}

func (c *Campaign) writeFinding(sub Backend, key string, p *gen.Program, m Mismatch, min string, mode gen.Mode) error {
	if c.cfg.FindingsDir == "" {
		return nil
	}
	dir := filepath.Join(c.cfg.FindingsDir, fmt.Sprintf("%s-seed%d-prog%d", key, c.cfg.Seed, p.ID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	bundle, err := bundleOf([]*gen.Program{reducedProgram(p, min)}, mode)
	if err != nil {
		bundle = min
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(bundle), 0o644); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Finding %s\n\n", key)
	fmt.Fprintf(&b, "- subject: %s\n- program: seed %d, id %d (scenarios: %s)\n- kind: %s\n- signature: `%s`\n\n", sub.Name(), c.cfg.Seed, p.ID, strings.Join(p.Used, ", "), m.Kind, strings.ReplaceAll(m.Sig, "\n", " | "))
	fmt.Fprintf(&b, "Run `main.go` with real Go and with `gocpp run main.gp` (copy it) and compare.\n\n")
	if m.Want != "" || m.Got != "" {
		fmt.Fprintf(&b, "## Go prints\n\n```\n%s\n```\n\n## gocpp prints\n\n```\n%s\n```\n\n", m.Want, m.Got)
	}
	if m.Detail != "" {
		fmt.Fprintf(&b, "## Details\n\n```\n%s\n```\n", m.Detail)
	}
	return os.WriteFile(filepath.Join(dir, "finding.md"), []byte(b.String()), 0o644)
}

// reducedProgram wraps the reduced source of a program into a Program, to bundle it.
func reducedProgram(p *gen.Program, src string) *gen.Program {
	q := *p
	q.Src = src
	return &q
}
