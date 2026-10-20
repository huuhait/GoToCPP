package probe

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
)

// Incremental probing (step 12.7, ADR-088).
//
// A probe file is a preamble (the includes and the helpers) and one unit per reference. The
// answer for a unit depends on the preamble, the compiler, the flags and the contents of the
// headers that the preamble reads, and on nothing else: not on the other units, not on where
// its lines are in the file. So the results are cached per unit, under
//
//	(the key of the preamble, the hash of the contents of its headers, the unit's hash)
//
// and a program with 200 references of which one changed asks the compiler for one. The units
// that are not cached go to the compiler in batches (a batch is a probe file with the same
// preamble and only those units; the batches run in parallel, bounded by Jobs), and every
// result goes through the cache's encoding (encodeUnit, decodeUnit) before it reaches the
// table, so a table made from a cold cache and one made from a warm cache are the same.
//
// The list of headers of a preamble is not known before the first run: the context entry holds
// it (the variants of it: a branch switch brings back an older one), and is valid when the hash
// of the contents of its files is still what it was.

// A variant is one list of the headers that a compiler run read, with a hash of their
// contents at the time, and what the run made that belongs to the context as a whole: the
// errors of the headers (expression probes) or the part (declaration runs).
type variant struct {
	Created      int64           `json:"created"` // UnixNano, for maxVariantAge
	Deps         []string        `json:"deps"`
	DepsHash     string          `json:"depsHash"`
	HeaderErrors []Diag          `json:"headerErrors,omitempty"`
	Part         json.RawMessage `json:"part,omitempty"`
}

// maxVariants is the number of header lists kept per context.
const maxVariants = 4

// maxVariantAge is how old a header list can be and still be used. The list cannot show a header
// that would hide one of its files without being included by the package itself (a file of the
// same name earlier on the search path of a header that a header includes), nor one that
// __has_include asks for: those results are used for a week at most, then asked again.
const maxVariantAge = 7 * 24 * time.Hour

type variantFile struct {
	Format   int       `json:"format"`
	Kind     string    `json:"kind"`
	Desc     string    `json:"desc,omitempty"`
	Created  int64     `json:"created"`
	Variants []variant `json:"variants"`
}

// variantFresh reports whether a header list made at created (UnixNano) is young enough; one from the future (the
// clock was set back) is not.
func variantFresh(created int64) bool {
	age := time.Since(time.Unix(0, created))
	return age >= 0 && age < maxVariantAge
}

// findVariant returns the variant of the context entry name whose headers have the contents
// that they had, or nil.
func (rn *Runner) findVariant(dir, name, kind string) *variant {
	body, ok := rn.get(dir, name)
	if !ok {
		cacheCount.misses.Add(1)
		return nil
	}
	var vf variantFile
	if json.Unmarshal(body, &vf) != nil || vf.Format != cacheFormat || vf.Kind != kind {
		cacheCount.corrupt.Add(1)
		cacheCount.misses.Add(1)
		return nil
	}
	for i := range vf.Variants {
		v := &vf.Variants[i]
		if v.DepsHash != "" && variantFresh(v.Created) && DepsHash(v.Deps) == v.DepsHash {
			return v
		}
	}
	cacheCount.misses.Add(1)
	return nil
}

// addVariant puts v first in the context entry name.
func (rn *Runner) addVariant(dir, name, kind, desc string, v variant, toDisk bool) {
	vf := variantFile{Format: cacheFormat, Kind: kind, Desc: desc, Created: time.Now().UnixNano()}
	if v.Created == 0 {
		v.Created = vf.Created
	}
	if body, ok := rn.get(dir, name); ok {
		var old variantFile
		if json.Unmarshal(body, &old) == nil && old.Format == cacheFormat && old.Kind == kind {
			for _, o := range old.Variants {
				if o.DepsHash != v.DepsHash && len(vf.Variants) < maxVariants-1 {
					vf.Variants = append(vf.Variants, o)
				}
			}
		}
	}
	vf.Variants = append([]variant{v}, vf.Variants...)
	body, err := json.Marshal(&vf)
	if err != nil {
		return
	}
	rn.put(dir, name, body, toDisk)
}

// ---------------------------------------------------------------- batches

// A linePos says what a line of a batch is.
type linePos struct {
	ref   int
	own   int    // index in the unit's own lines
	alias string // the alias, for an alias line
}

// A batch is a probe file with the preamble and some units.
type batch struct {
	src   *Source
	units []int
	info  map[int]linePos
	first int // the first line after the preamble
	uses  map[int]map[string]bool
}

func newBatch(full *Source, units []int) *batch {
	b := &batch{units: units, info: make(map[int]linePos), uses: make(map[int]map[string]bool)}
	var text strings.Builder
	text.WriteString(full.Preamble)
	line := strings.Count(full.Preamble, "\n")
	b.first = line + 1
	lines := make(map[int]Line)
	emitted := make(map[string]bool)
	emit := func(l Line, p linePos) {
		text.WriteString(l.Text)
		text.WriteByte('\n')
		line++
		lines[line] = l
		b.info[line] = p
	}
	for _, ui := range units {
		u := &full.Units[ui]
		set := make(map[string]bool, len(u.Aliases))
		for _, a := range u.Aliases {
			name := aliasNameOf(a.Text)
			set[name] = true
			if !emitted[name] {
				emitted[name] = true
				emit(a, linePos{ref: -1, own: -1, alias: name})
			}
		}
		b.uses[ui] = set
		for i, l := range u.Lines {
			emit(l, linePos{ref: u.Ref, own: i})
		}
	}
	b.src = &Source{Text: text.String(), Lines: lines, Skipped: map[int]string{}, NRefs: full.NRefs, Keys: full.Keys,
		HelperFrom: full.HelperFrom, HelperTo: full.HelperTo, Includes: full.Includes, Preamble: full.Preamble}
	return b
}

// ---------------------------------------------------------------- the encoding of a unit

// A cpos is a place in the probe file in terms that do not depend on the file: a line of the
// preamble, a line of the unit, a line of an alias the unit uses.
type cpos struct {
	K string `json:"k"` // "p" preamble, "o" own, "a" alias, "x" somewhere else
	N int    `json:"n,omitempty"`
	A string `json:"a,omitempty"`
}

type cconst struct {
	Kind string `json:"kind"`
	Int  string `json:"int,omitempty"`
	Bits uint64 `json:"bits,omitempty"`
	Str  string `json:"str,omitempty"`
}

type cachedResult struct {
	R        Result  `json:"r"`
	Value    *cconst `json:"v,omitempty"`
	ErrAt    *cpos   `json:"ea,omitempty"`
	Notes    []*cpos `json:"n,omitempty"`
	RecAt    *cpos   `json:"ra,omitempty"`
	RecNotes []*cpos `json:"rn,omitempty"`
}

type unitFile struct {
	Format int          `json:"format"`
	Kind   string       `json:"kind"`
	Result cachedResult `json:"result"`
}

// encodeUnit returns the cache's form of the result of the unit ui of the full file, which a
// batch made: with the places in the probe file that its messages name in file-independent
// terms. A batch's own Source has no units, so the reference comes from full.
func encodeUnit(b *batch, full *Source, ui int, r *Result) ([]byte, error) {
	cl := func(d *Diag) *cpos {
		if d == nil {
			return nil
		}
		return b.classify(full, ui, d.File, d.Line)
	}
	cr := cachedResult{R: *r}
	cr.R.ID, cr.R.Key = 0, ""
	if r.Value != nil {
		c := &cconst{Kind: r.Value.Kind}
		switch r.Value.Kind {
		case "int", "bool":
			c.Int = r.Value.Int.String()
		case "float":
			c.Bits = math.Float64bits(r.Value.Float)
		default:
			c.Str = r.Value.Str
		}
		cr.Value = c
		cr.R.Value = nil
	}
	cr.ErrAt = cl(r.ErrorAt)
	cr.RecAt = cl(r.RecoveredAt)
	for i := range r.Notes {
		cr.Notes = append(cr.Notes, cl(&r.Notes[i]))
	}
	for i := range r.RecoveredNotes {
		cr.RecNotes = append(cr.RecNotes, cl(&r.RecoveredNotes[i]))
	}
	// The lines are the cache's places now: clear them from the copies.
	if cr.R.ErrorAt != nil {
		d := *cr.R.ErrorAt
		if cr.ErrAt != nil {
			d.Line = 0
		}
		cr.R.ErrorAt = &d
	}
	if cr.R.RecoveredAt != nil {
		d := *cr.R.RecoveredAt
		if cr.RecAt != nil {
			d.Line = 0
		}
		cr.R.RecoveredAt = &d
	}
	cr.R.Notes = slices.Clone(r.Notes)
	for i, p := range cr.Notes {
		if p != nil {
			cr.R.Notes[i].Line = 0
		}
	}
	cr.R.RecoveredNotes = slices.Clone(r.RecoveredNotes)
	for i, p := range cr.RecNotes {
		if p != nil {
			cr.R.RecoveredNotes[i].Line = 0
		}
	}
	return json.Marshal(unitFile{Format: cacheFormat, Kind: "u", Result: cr})
}

// classify returns the place of the line of file in file-independent terms, or nil for a
// place in a header.
func (b *batch) classify(full *Source, ui int, file string, line int) *cpos {
	if !isProbeFile(file, "") {
		return nil
	}
	if line < b.first {
		return &cpos{K: "p", N: line}
	}
	p, ok := b.info[line]
	switch {
	case !ok:
		return &cpos{K: "x"}
	case p.alias != "":
		if b.uses[ui][p.alias] {
			return &cpos{K: "a", A: p.alias}
		}
	case p.ref == full.Units[ui].Ref:
		return &cpos{K: "o", N: p.own}
	}
	return &cpos{K: "x"}
}

// decodeUnit reads a unit file for the unit ui of the file full, and puts the places of its
// messages where the file has the lines. It reports false for anything that is not a result.
func decodeUnit(body []byte, full *Source, ui int) (Result, bool) {
	var uf unitFile
	if err := json.Unmarshal(body, &uf); err != nil || uf.Format != cacheFormat || uf.Kind != "u" {
		return Result{}, false
	}
	u := &full.Units[ui]
	cr := &uf.Result
	r := cr.R
	r.ID, r.Key = u.Ref, full.Keys[u.Ref]
	place := func(d *Diag, p *cpos) bool {
		if p == nil {
			return true
		}
		switch p.K {
		case "p":
			d.Line = p.N
		case "o":
			if p.N < 0 || p.N >= len(u.At) {
				return false
			}
			d.Line = u.At[p.N]
		case "a":
			at, ok := full.aliasAt[p.A]
			if !ok {
				return false
			}
			d.Line = at
		default:
			d.Line = 0
		}
		return true
	}
	if r.ErrorAt != nil {
		d := *r.ErrorAt
		if !place(&d, cr.ErrAt) {
			return Result{}, false
		}
		r.ErrorAt = &d
	}
	if r.RecoveredAt != nil {
		d := *r.RecoveredAt
		if !place(&d, cr.RecAt) {
			return Result{}, false
		}
		r.RecoveredAt = &d
	}
	if len(cr.Notes) != len(r.Notes) || len(cr.RecNotes) != len(r.RecoveredNotes) {
		return Result{}, false
	}
	r.Notes = slices.Clone(r.Notes)
	for i, p := range cr.Notes {
		if i >= len(r.Notes) || !place(&r.Notes[i], p) {
			return Result{}, false
		}
	}
	r.RecoveredNotes = slices.Clone(r.RecoveredNotes)
	for i, p := range cr.RecNotes {
		if i >= len(r.RecoveredNotes) || !place(&r.RecoveredNotes[i], p) {
			return Result{}, false
		}
	}
	if c := cr.Value; c != nil {
		v := &Const{Kind: c.Kind}
		switch c.Kind {
		case "int", "bool":
			n, ok := new(big.Int).SetString(c.Int, 10)
			if !ok {
				return Result{}, false
			}
			v.Int = n
		case "float":
			v.Float = math.Float64frombits(c.Bits)
		case "string":
			v.Str = c.Str
		default:
			return Result{}, false
		}
		r.Value = v
	}
	return r, true
}

// ---------------------------------------------------------------- the run of a table

// CacheInfo says where the answers of a table came from.
type CacheInfo struct {
	Units   int // units with a probe
	Hits    int // answered from the cache (memory or disk)
	Ran     int // answered by a compiler run
	Batches int // compiler runs
}

// A groupOut is what a batch made.
type groupOut struct {
	units    []int
	bodies   [][]byte // the unit files, by position in units
	deps     []string // the headers, with the places where others would hide them
	depsHash string
	racy     bool // not stored: see racyDeps
	headers  []Diag
	stats    Stats
	stderr   string
	cpu      time.Duration
	compile  time.Duration
	parse    time.Duration
}

// defaultMinBatch is the number of units below which a second batch is not worth its compiler
// start-up and its headers. Measured (clang 21, <vector>, <map>, <string>, <algorithm>): the
// preamble costs 0.3 s of CPU and a probe of a standard container member 0.4 ms, so a run is
// 85% headers at 237 probes and two batches cost twice the CPU for no gain in wall time;
// they pay off from about a thousand probes, or for probes that instantiate heavy
// templates. EnvBatch changes it.
const defaultMinBatch = 400

// EnvBatch names the environment variable that sets the least number of units per batch.
const EnvBatch = "GOCPP_PROBE_BATCH"

func minBatch() int {
	if n, err := strconv.Atoi(os.Getenv(EnvBatch)); err == nil && n > 0 {
		return n
	}
	return defaultMinBatch
}

// split divides the units that need a run into at most Jobs() batches of at least minBatch
// units (one run for few), keeping their order.
func split(need []int) [][]int {
	n := len(need)
	g := max(1, min(Jobs(), n/minBatch()))
	out := make([][]int, 0, g)
	for i := 0; i < g; i++ {
		lo, hi := i*n/g, (i+1)*n/g
		out = append(out, need[lo:hi])
	}
	return out
}

// diskFor returns the cache folder for cfg and whether results may be written to it: the
// results that depend on the host's CPU stay in memory (two machines can share a folder).
func (rn *Runner) diskFor(cfg Config) (dir string, write bool) {
	dir = cfg.CacheDir
	switch dir {
	case "":
		dir = CacheDir()
	case "off":
		dir = ""
	}
	if dir == "" || selfID() == "" {
		return "", false
	}
	if build.NativeArch(cfg.Flags...) || build.NativeArch(cfg.CXX.Args...) || build.NativeArch(build.KeyEnv()...) {
		return dir, false
	}
	return dir, true
}

// selfID identifies the running gocpp for the keys (interop.SelfID).
var selfID = interop.SelfID

func (rn *Runner) probeSource(ctx context.Context, cfg Config, src *Source) (*Table, error) {
	defer rn.WaitPCH(ctx) // (no compiler of this call outlives it: pch.go)
	t0 := time.Now()
	t := &Table{Source: src, Results: make([]Result, src.NRefs), Compiler: compilerID(cfg.CXX)}
	for id := range t.Results {
		r := &t.Results[id]
		r.ID, r.Key = id, src.Keys[id]
		r.Skipped = src.Skipped[id]
	}
	dir, write := rn.diskFor(cfg)
	tc := rn.toolchain(ctx, cfg, dir, write)
	pre := withToolchain(PreKey(cfg, &Source{Text: src.Preamble}), tc)
	cname := entryName("c", selfID(), pre)
	info := &t.Cache
	info.Units = len(src.Units)
	var (
		hdrErrs []Diag
		stats   Stats
		stderr  strings.Builder
		batches int
		ran     = make(map[int]bool) // units answered by a run of this call
		ranHash string               // the hash of the headers those results belong to
	)
	for round := 0; ; round++ {
		if round > 16 {
			return nil, errors.New("the headers of the probe file kept changing while it was probed")
		}
		v := rn.findVariant(dir, cname, "c")
		if len(ran) > 0 && (v == nil || v.DepsHash != ranHash) {
			ran = make(map[int]bool) // the headers changed since: those results are old
		}
		placed := make([]bool, len(src.Units))
		if v != nil {
			for i := range src.Units {
				if ran[i] {
					continue
				}
				if body, ok := rn.get(dir, entryName("u", selfID(), pre, v.DepsHash, src.Units[i].Hash)); ok {
					if r, ok := decodeUnit(body, src, i); ok {
						t.Results[src.Units[i].Ref] = r
						placed[i] = true
						continue
					}
					cacheCount.corrupt.Add(1)
				}
				cacheCount.misses.Add(1)
			}
		}
		var need []int
		for i := range placed {
			if !placed[i] && !ran[i] {
				need = append(need, i)
			}
		}
		if v != nil && len(need) == 0 {
			// Everything is known without running the compiler (or the run of an earlier round
			// made the rest).
			info.Ran = len(ran)
			info.Hits = len(src.Units) - len(ran)
			t.Deps = v.Deps
			t.HeaderErrors = mergeDiags(hdrErrs, v.HeaderErrors)
			return rn.finish(t, cfg, src, v.DepsHash, t0, stats, stderr.String(), batches, info), nil
		}
		groups := split(need)
		outs := make([]*groupOut, len(groups))
		errs := make([]error, len(groups))
		retry := make([]bool, len(groups))
		gctx, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		for gi := range groups {
			wg.Add(1)
			go func() {
				defer wg.Done()
				outs[gi], retry[gi], errs[gi] = rn.runGroup(gctx, cfg, src, pre, dir, tc, groups[gi])
				if errs[gi] != nil {
					cancel()
				}
			}()
		}
		wg.Wait()
		cancel()
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("%s was stopped: %w", cfg.CXX.Name(), err)
		}
		// The error of the batch that failed, not the "canceled" of the ones that were stopped for it.
		var firstErr error
		for _, err := range errs {
			if err != nil && (firstErr == nil || errors.Is(firstErr, context.Canceled) && !errors.Is(err, context.Canceled)) {
				firstErr = err
			}
		}
		if firstErr != nil {
			return nil, firstErr
		}
		again := false
		var done []*groupOut
		hashes := make(map[string]bool)
		for gi, o := range outs {
			if retry[gi] || o == nil {
				again = true
				continue
			}
			done = append(done, o)
			hashes[o.depsHash] = true
			hdrErrs = mergeDiags(hdrErrs, o.headers)
			stats = addStats(stats, o.stats)
			stderr.WriteString(o.stderr)
			t.Timing.CPU += o.cpu
			t.Timing.Parse += o.parse
			t.Timing.Compile += o.compile
		}
		if len(done) == 0 {
			continue // another caller ran these: look at the cache again
		}
		batches += len(done)
		if len(hashes) > 1 {
			// A header changed while the batches ran: they saw different contents. Their results
			// are in the cache under their own hashes; look again with the contents of now.
			continue
		}
		var deps []string
		for _, o := range done {
			deps = unionDeps(deps, o.deps)
			for k, ui := range o.units {
				r, ok := decodeUnit(o.bodies[k], src, ui)
				if !ok {
					return nil, fmt.Errorf("probe cache: cannot read back the result of %s", src.Keys[src.Units[ui].Ref])
				}
				t.Results[src.Units[ui].Ref] = r
				ran[ui] = true
			}
		}
		depsHash := done[0].depsHash
		racy := false
		for _, o := range done {
			racy = racy || o.racy
		}
		changed := v != nil && v.DepsHash != depsHash
		if changed || again {
			// The headers changed between the look at the cache and the run (the results that
			// came from the cache belong to the old contents), or another caller made the rest:
			// look again. The new variant is there now, with the units that ran.
			ranHash = depsHash
			if v != nil && v.DepsHash != depsHash {
				ran = make(map[int]bool)
			}
			continue
		}
		info.Ran = len(ran)
		info.Hits = len(src.Units) - len(ran)
		t.Deps = deps
		t.HeaderErrors = hdrErrs
		return rn.finish(t, cfg, src, depsHash, t0, stats, stderr.String(), batches, info), nil
	}
}

// finish completes a table: the skipped references, the key, the statistics.
func (rn *Runner) finish(t *Table, cfg Config, src *Source, depsHash string, t0 time.Time, stats Stats, stderr string, batches int, info *CacheInfo) *Table {
	slices.SortFunc(t.HeaderErrors, compareDiag)
	t.Key = keyFrom(PreKey(cfg, src), depsHash)
	t.Stats = stats
	t.Stderr = stderr
	info.Batches = batches
	t.Cached = batches == 0
	t.Timing.Total = time.Since(t0)
	return t
}

func addStats(a, b Stats) Stats {
	a.Decls += b.Decls
	a.JSONBytes += b.JSONBytes
	a.LargestDecl = max(a.LargestDecl, b.LargestDecl)
	a.TooLarge += b.TooLarge
	a.StderrBytes += b.StderrBytes
	a.StderrTruncated = a.StderrTruncated || b.StderrTruncated
	a.ExitCode = max(a.ExitCode, b.ExitCode)
	return a
}

func unionDeps(a, b []string) []string {
	if len(a) == 0 {
		return slices.Clone(b)
	}
	out := slices.Concat(a, b)
	slices.Sort(out)
	return slices.Compact(out)
}

// mergeDiags returns the diagnostics of a and b without repeats.
func mergeDiags(a, b []Diag) []Diag {
	var out []Diag
	seen := make(map[Diag]bool)
	for _, d := range slices.Concat(a, b) {
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out
}

// racyDeps reports whether one of the headers was modified (or changed) shortly before start, or after it:
// the compiler may have read it before the modification, and the contents that are hashed
// after the run are not the ones that it saw.
func racyDeps(deps []string, start time.Time) bool {
	limit := start.Add(-racyWindow)
	for _, d := range deps {
		fi, err := os.Stat(d)
		if err != nil {
			continue
		}
		if fi.ModTime().After(limit) {
			return true
		}
		// The change time moves at every write and cannot be set back (touch -r, rsync -t, a checkout of an
		// old blob keep the modification time).
		if ct, ok := changeTime(fi); ok && time.Unix(0, ct).After(limit) {
			return true
		}
	}
	return false
}

// hookRunGroup is called when a batch is about to be run, before anything is looked up again
// (tests make two callers meet there).
var hookRunGroup func(keys []string)

// hookGroupStored is called when a batch has stored its results (tests order batches with it).
var hookGroupStored func()

// runGroup runs one batch for the units of a table that the cache did not have. retry is true
// when it did not run: another caller (in this process or in another) made the same results
// meanwhile, and the caller looks at the cache again.
func (rn *Runner) runGroup(ctx context.Context, cfg Config, src *Source, pre, dir string, tc toolchain, units []int) (out *groupOut, retry bool, err error) {
	if hookRunGroup != nil {
		keys := make([]string, len(units))
		for i, ui := range units {
			keys[i] = src.Keys[src.Units[ui].Ref]
		}
		hookRunGroup(keys)
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00", pre)
	hashes := make([]string, len(units))
	for i, ui := range units {
		hashes[i] = src.Units[ui].Hash
	}
	slices.Sort(hashes)
	for _, x := range hashes {
		fmt.Fprintf(h, "%s\x00", x)
	}
	key := hex.EncodeToString(h.Sum(nil))

	// One run per key in this process.
	rn.mu.Lock()
	if other := rn.flying[key]; other != nil {
		rn.mu.Unlock()
		select {
		case <-other.done:
		case <-ctx.Done():
			return nil, false, fmt.Errorf("%s was stopped: %w", cfg.CXX.Name(), ctx.Err())
		}
		if other.err != nil && !errors.Is(other.err, context.Canceled) {
			return nil, false, other.err
		}
		return nil, true, nil
	}
	fl := &flight{done: make(chan struct{})}
	if rn.flying == nil {
		rn.flying = make(map[string]*flight)
	}
	rn.flying[key] = fl
	rn.mu.Unlock()
	defer func() {
		rn.mu.Lock()
		delete(rn.flying, key)
		rn.mu.Unlock()
		fl.err = err
		close(fl.done)
	}()

	// A caller that came just after another one finished finds the results here.
	if rn.allCached(cfg, src, pre, dir, units) {
		return nil, true, nil
	}
	// One run per key on this machine.
	if dir != "" {
		release, waited := acquireLock(ctx, dir, "g"+key, effTimeout(cfg))
		if release != nil {
			defer release()
		}
		if waited && rn.allCached(cfg, src, pre, dir, units) {
			return nil, true, nil
		}
	}
	// The question of finalDeps about the includes of the preamble does not wait for the compile.
	ictx, endInfo := ctx, func() {}
	if len(src.Includes) > 0 { // (a text with no header of the package is not asked: includeInfo)
		ictx, endInfo = rn.prefetchInfo(ctx, cfg, src.Preamble)
	}
	defer endInfo()
	start := time.Now()
	out, _, err = rn.compileGroup(ctx, cfg, src, units)
	if err != nil {
		return nil, false, err
	}
	// Store before the flight and the lock are let go: whoever waited for them finds the results.
	deps := rn.finalDeps(ictx, cfg, src.Preamble, src.Includes, out.deps, out.headers, tc)
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	out.deps, out.depsHash = deps, DepsHash(deps)
	// A header that changed while the compiler ran (or just before it) may be read in the old
	// contents and hashed in the new: such a result is not kept anywhere.
	if out.racy = racyDeps(deps, start); out.racy {
		return out, false, nil
	}
	_, write := rn.diskFor(cfg)
	toDisk := write && len(deps) > 0
	for k, ui := range units {
		rn.put(dir, entryName("u", selfID(), pre, out.depsHash, src.Units[ui].Hash), out.bodies[k], toDisk)
	}
	rn.addVariant(dir, entryName("c", selfID(), pre), "c", fmt.Sprintf("%s %s %d headers", cfg.CXX.Name(), canonPath(cfg.Dir), len(src.Includes)),
		variant{Deps: deps, DepsHash: out.depsHash, HeaderErrors: out.headers}, toDisk)
	if hookGroupStored != nil {
		hookGroupStored()
	}
	return out, false, nil
}

// allCached reports whether the cache has a result for each of the units now.
func (rn *Runner) allCached(cfg Config, src *Source, pre, dir string, units []int) bool {
	v := rn.findVariant(dir, entryName("c", selfID(), pre), "c")
	if v == nil {
		return false
	}
	for _, ui := range units {
		body, ok := rn.get(dir, entryName("u", selfID(), pre, v.DepsHash, src.Units[ui].Hash))
		if !ok {
			return false
		}
		if _, ok := decodeUnit(body, src, ui); !ok {
			return false
		}
	}
	return true
}

// compileGroup runs the compiler on a batch of the units and encodes their results.
func (rn *Runner) compileGroup(ctx context.Context, cfg Config, full *Source, units []int) (*groupOut, bool, error) {
	b := newBatch(full, units)
	founds := make(map[probeName]*found)
	var parse time.Duration
	tab := &Table{Source: b.src, Results: make([]Result, full.NRefs)}
	d, err := runDump(ctx, rn.withPCH(ctx, cfg, b.src.Text), b.src.Text, probeDeclPrefix, func(rd RawDecl) error {
		pn, ok := parseProbeName(rd.Name)
		if !ok {
			return nil
		}
		if rd.TooLarge {
			tab.Stats.TooLarge++
			founds[pn] = &found{tooBig: true}
			return nil
		}
		p0 := time.Now()
		f, err := decodeDecl(rd.Raw)
		parse += time.Since(p0)
		if err != nil {
			return fmt.Errorf("%s: %w", rd.Name, err)
		}
		founds[pn] = f
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	rn.mu.Lock()
	rn.runs++
	rn.mu.Unlock()
	cacheCount.runs.Add(1)
	tooLarge := tab.Stats.TooLarge
	st := d.stats
	st.TooLarge = tooLarge
	ids := make([]int, len(units))
	for i, ui := range units {
		ids[i] = full.Units[ui].Ref
	}
	if err := assemble(tab, b.src, founds, ParseDiags(d.stderr, d.srcPath), ids); err != nil {
		return nil, false, err
	}
	out := &groupOut{units: units, deps: d.deps, headers: tab.HeaderErrors, stats: st, stderr: string(d.stderr), cpu: d.cpu, compile: d.compile, parse: parse}
	for _, ui := range units {
		body, err := encodeUnit(b, full, ui, &tab.Results[full.Units[ui].Ref])
		if err != nil {
			return nil, false, err
		}
		out.bodies = append(out.bodies, body)
	}
	return out, false, nil
}

// compareDiag orders diagnostics by every field, so that the order does not depend on the sort.
func compareDiag(a, b Diag) int {
	return cmp.Or(strings.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col),
		strings.Compare(a.Severity, b.Severity), strings.Compare(a.Msg, b.Msg))
}
