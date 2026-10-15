package cache

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
)

// The namespace probe off the hot path (README §2.2 item 8, step 11.4).
//
// check.Check asks the C++ compiler, for each include set of a package with
// header imports, which of the names that the loader took for namespaces
// really are namespaces (Phase 01's namespace probe, now the namespace mode
// of the kind probe, interop.ProbeNamespaces),
// and it finds the compiler first (build.FindCXXFor, which runs it). A probe
// takes up to a few seconds (about 2.5 s for <vector>), so the pipeline never
// waits for either: the check gets, at once, what the prober knows, and the
// prober asks the compiler in the background.
//
//   - The compiler: until it is found, the check gets a placeholder, and
//     every probe answers Unknown. An error (no compiler) is kept for
//     cxxRetry, then the compiler is looked for again, in the background
//     (so one that is installed later is found without a restart).
//   - The probe of an include set (compiler, folder, flags, includes): the prober
//     keeps the answers it got, per candidate name. A name it has no answer
//     for is Unknown (its uses stay opaque, without an error: "publish what
//     is there"), and the prober probes it in the background, together with
//     the names it knows, so the answers come from one run. Requests that
//     come while a probe runs wait for the next run, one at a time: typing
//     a new name never queues one probe per keystroke. (A probe asks about
//     every name the checks asked for, up to maxProbeNames; also after
//     include errors, which answer for every name, so that the probe after
//     the fix has them all.) An answer that is
//     older than revalidateAfter is checked again in the background when a
//     check uses it (interop.ProbeNamespaces stats the headers and runs the
//     compiler only when one changed, or when its include errors are older
//     than 5 s), and so is every answer when the editor reports a changed
//     header (Pipeline.HeadersChanged).
//
// Each check records what it got (a probeRecord). A checked package whose
// record would now get other answers is stale (Package.Stale): the caches
// drop it when it is asked for (checked, summaryOf), and the language
// server, told by Config.OnProbe, checks the packages it shows again in a
// new snapshot. A package whose check got an Unknown for want of an answer
// is provisional (Package.Provisional) until the answer comes.
//
// So the diagnostics of a check depend on what the prober knew then, not
// only on the snapshot (a State is meant to depend on its snapshot alone).
// Once the background work is done, they are those of gocpp check.

// cxxRetry is how long an error to find the C++ compiler is kept (a
// variable for the tests).
var cxxRetry = 30 * time.Second

const (
	// revalidateAfter is the age of an answer that a check uses after
	// which it is checked again in the background.
	revalidateAfter = 2 * time.Second
	// maxProbeSets is the number of include sets the prober keeps (the
	// one used least recently goes), and maxProbeNames the number of
	// names an include set keeps answers for: misspelled names pile up
	// while someone types, and after this many only the names of the
	// last checks are probed again.
	maxProbeSets  = 128
	maxProbeNames = 1024
	// maxProbes is the number of probes that run at the same time: a probe
	// of heavy headers takes seconds and hundreds of MB (cc1plus), and
	// HeadersChanged checks every include set again.
	maxProbes = 2
)

// pendingCXX is the placeholder compiler that a check gets while the
// compiler is being looked for.
var pendingCXX = build.CXX{Path: "\x00gocpp lsp: the C++ compiler is being looked for"}

// A prober asks the C++ compiler in the background (see above). Its methods
// are safe for concurrent use.
type prober struct {
	ctx      context.Context // done when the pipeline closes
	cancel   context.CancelFunc
	find     func(ctx context.Context) (build.CXX, error)
	probe    func(ctx context.Context, cxx build.CXX, dir string, flags, includes, candidates []string) (interop.Result, error)
	onChange func()        // Config.OnProbe, or nil
	sem      chan struct{} // a place for each probe that runs: at most maxProbes

	mu      sync.Mutex
	idle    sync.Cond // on mu: signalled when active goes to 0
	active  int       // the goroutines of background work
	closed  bool
	cxx     cxxLookup
	sets    map[string]*probeSet
	clock   uint64 // counts uses, for probeSet.lastUse
	changes int64  // counts the changed answers, for the tests
}

// A cxxLookup is what the prober knows of the C++ compiler.
type cxxLookup struct {
	done, running bool
	cxx           build.CXX
	err           error
	at            time.Time // when it was done
	gen           int64     // one more for each lookup that gave another answer
}

// A probeSet is what the prober knows of one include set. Its fields are
// guarded by prober.mu.
type probeSet struct {
	key      string // in prober.sets
	cxx      build.CXX
	dir      string
	flags    []string
	includes []string
	lastUse  uint64

	has     bool                    // a probe has answered: err, incErrs or kinds hold its answer
	err     error                   // the probe failed
	incErrs []interop.IncludeError  // an include failed
	kinds   map[string]interop.Kind // else: the answer per candidate; replaced, never changed

	names     map[string]bool // the candidates asked for, which a probe asks about
	want      map[string]bool // the names asked for without an answer
	running   bool
	validated time.Time // when the last probe answered
}

func newProber(find func(context.Context) (build.CXX, error), probe func(context.Context, build.CXX, string, []string, []string, []string) (interop.Result, error), onChange func()) *prober {
	ctx, cancel := context.WithCancel(context.Background())
	pr := &prober{ctx: ctx, cancel: cancel, find: find, probe: probe, onChange: onChange,
		sem: make(chan struct{}, maxProbes), sets: make(map[string]*probeSet)}
	pr.idle.L = &pr.mu
	return pr
}

// close stops the background work and waits for it to end.
func (pr *prober) close() {
	pr.mu.Lock()
	pr.closed = true
	pr.mu.Unlock()
	pr.cancel()
	pr.wait()
}

// wait waits until no background work runs (for close and the tests).
func (pr *prober) wait() {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	for pr.active > 0 {
		pr.idle.Wait()
	}
}

// goLocked runs f on a goroutine of background work. pr.mu is held.
func (pr *prober) goLocked(f func()) {
	pr.active++
	go func() {
		defer func() {
			pr.mu.Lock()
			pr.active--
			if pr.active == 0 {
				pr.idle.Broadcast()
			}
			pr.mu.Unlock()
		}()
		f()
	}()
}

// changed tells the pipeline's owner that answers changed.
func (pr *prober) changed() {
	pr.mu.Lock()
	pr.changes++
	pr.mu.Unlock()
	if pr.onChange != nil {
		pr.onChange()
	}
}

// findCXX returns the check.Config.CXX of a check that records in r.
func (pr *prober) findCXX(r *probeRecord) func(context.Context) (build.CXX, error) {
	return func(context.Context) (build.CXX, error) {
		pr.mu.Lock()
		defer pr.mu.Unlock()
		l := &pr.cxx
		if !l.done || l.err != nil && time.Since(l.at) >= cxxRetry {
			pr.lookCXXLocked()
		}
		r.addCXX(l.gen, !l.done, l.cxx, l.err)
		if !l.done {
			return pendingCXX, nil
		}
		return l.cxx, l.err
	}
}

// compiler returns the C++ compiler, when the prober has found it; else it
// looks for it in the background (like findCXX, but without a record).
func (pr *prober) compiler() (build.CXX, bool) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	l := &pr.cxx
	if !l.done || l.err != nil && time.Since(l.at) >= cxxRetry {
		pr.lookCXXLocked()
	}
	return l.cxx, l.done && l.err == nil
}

// lookCXXLocked looks for the compiler in the background, unless that runs
// already. pr.mu is held.
func (pr *prober) lookCXXLocked() {
	l := &pr.cxx
	if l.running || pr.closed {
		return
	}
	l.running = true
	pr.goLocked(func() {
		cxx, err := pr.find(pr.ctx)
		pr.mu.Lock()
		l.running = false
		if pr.ctx.Err() != nil {
			pr.mu.Unlock()
			return
		}
		changed := !l.done || !sameCXX(l.cxx, cxx) || errText(l.err) != errText(err)
		l.done, l.cxx, l.err, l.at = true, cxx, err, time.Now()
		if changed {
			l.gen++
		}
		pr.mu.Unlock()
		if changed {
			pr.changed()
		}
	})
}

func sameCXX(a, b build.CXX) bool {
	return a.Path == b.Path && slices.Equal(a.Args, b.Args) && a.Family == b.Family && a.Version == b.Version
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return "error: " + err.Error()
}

// probeFor returns the check.Config.Probe of a check that records in r.
func (pr *prober) probeFor(r *probeRecord) func(context.Context, build.CXX, string, []string, []string, []string) (interop.Result, error) {
	return func(_ context.Context, cxx build.CXX, dir string, flags, includes, candidates []string) (interop.Result, error) {
		if cxx.Path == pendingCXX.Path {
			return unknownResult(candidates), nil // r has the pending compiler
		}
		pr.mu.Lock()
		defer pr.mu.Unlock()
		set := pr.setLocked(cxx, dir, flags, includes)
		res, err, complete := set.answer(candidates)
		r.add(probeUse{set: set, cands: slices.Clone(candidates), digest: digest(res, err, complete), complete: complete})
		if len(set.names)+len(candidates) > maxProbeNames {
			clear(set.names) // only the names of the checks from now on
		}
		for _, c := range candidates {
			set.names[c] = true
		}
		switch {
		case !complete:
			for _, c := range candidates {
				if _, ok := set.kinds[c]; !ok {
					set.want[c] = true
				}
			}
			pr.startLocked(set)
		case time.Since(set.validated) >= revalidateAfter:
			pr.startLocked(set)
		}
		return res, err
	}
}

// setLocked returns the include set, new if the prober doesn't know it.
// pr.mu is held.
func (pr *prober) setLocked(cxx build.CXX, dir string, flags, includes []string) *probeSet {
	parts := append([]string{cxx.Path, strconv.Itoa(len(cxx.Args))}, cxx.Args...)
	parts = append(parts, cxx.Family.String(), cxx.Version, dir, strconv.Itoa(len(flags)))
	parts = append(parts, flags...)
	key := strings.Join(append(parts, includes...), "\x00")
	pr.clock++
	set := pr.sets[key]
	if set == nil {
		if len(pr.sets) >= maxProbeSets {
			pr.evictLocked()
		}
		set = &probeSet{key: key, cxx: cxx, dir: dir, flags: slices.Clone(flags), includes: slices.Clone(includes), names: make(map[string]bool), want: make(map[string]bool)}
		pr.sets[key] = set
	}
	set.lastUse = pr.clock
	return set
}

// evictLocked drops the include set used least recently that no probe
// runs for. A record that has it finds the new set of its key, once a check
// makes one (probeRecord.stale).
func (pr *prober) evictLocked() {
	var oldest string
	for key, set := range pr.sets {
		if !set.running && (oldest == "" || set.lastUse < pr.sets[oldest].lastUse) {
			oldest = key
		}
	}
	delete(pr.sets, oldest)
}

// startLocked starts a probe of set in the background, unless one runs
// already (then the names it wants wait for the next run). pr.mu is held.
func (pr *prober) startLocked(set *probeSet) {
	if set.running || pr.closed {
		return
	}
	set.running = true
	pr.goLocked(func() { pr.run(set) })
}

// run probes set until no name waits for an answer.
func (pr *prober) run(set *probeSet) {
	for {
		pr.mu.Lock()
		names := make([]string, 0, len(set.names))
		for name := range set.names {
			names = append(names, name)
		}
		clear(set.want)
		pr.mu.Unlock()
		slices.Sort(names)

		var res interop.Result
		var err error
		select {
		case pr.sem <- struct{}{}:
			res, err = pr.probe(pr.ctx, set.cxx, set.dir, set.flags, set.includes, names)
			<-pr.sem
		case <-pr.ctx.Done():
		}

		pr.mu.Lock()
		if pr.ctx.Err() != nil {
			set.running = false
			pr.mu.Unlock()
			return
		}
		changed := set.setAnswer(res, err, names)
		set.validated = time.Now()
		more := len(set.want) > 0
		set.running = more
		pr.mu.Unlock()
		if changed {
			pr.changed()
		}
		if !more {
			return
		}
	}
}

// headersChanged checks every include set again in the background.
func (pr *prober) headersChanged() {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	for _, set := range pr.sets {
		if set.has {
			pr.startLocked(set)
		}
	}
}

// answer returns what set knows for the candidates, and whether that is
// all (complete): a name without an answer is Unknown. pr.mu is held.
func (set *probeSet) answer(candidates []string) (interop.Result, error, bool) {
	switch {
	case !set.has:
		return unknownResult(candidates), nil, false
	case set.err != nil:
		return interop.Result{}, set.err, true
	case set.incErrs != nil:
		r := unknownResult(candidates)
		r.IncludeErrors = slices.Clone(set.incErrs)
		return r, nil, true
	}
	r := interop.Result{Kinds: make(map[string]interop.Kind, len(candidates))}
	complete := true
	for _, c := range candidates {
		k, ok := set.kinds[c]
		complete = complete && ok
		r.Kinds[c] = k
	}
	return r, nil, complete
}

// setAnswer keeps the answer of a probe for the names and reports whether
// it changed what set says about them. pr.mu is held.
func (set *probeSet) setAnswer(res interop.Result, err error, names []string) bool {
	before := digest(set.answer(names))
	switch {
	case err != nil:
		set.err, set.incErrs, set.kinds = err, nil, nil
	case len(res.IncludeErrors) > 0:
		set.err, set.incErrs, set.kinds = nil, slices.Clone(res.IncludeErrors), nil
	default:
		kinds := make(map[string]interop.Kind, len(names))
		for _, n := range names {
			kinds[n] = res.Kinds[n]
		}
		set.err, set.incErrs, set.kinds = nil, nil, kinds
	}
	wasThere := set.has
	set.has = true
	return !wasThere || digest(set.answer(names)) != before
}

// unknownResult is the answer without a probe: every candidate is Unknown.
func unknownResult(candidates []string) interop.Result {
	r := interop.Result{Kinds: make(map[string]interop.Kind, len(candidates))}
	for _, c := range candidates {
		r.Kinds[c] = interop.Unknown
	}
	return r
}

// digest returns a text that is the same for the same answer. Of a kind,
// it keeps only what a check uses: a namespace, not a namespace, or no
// answer (Unknown), so that a probe (a fake one, or one that answers with
// more kinds) that says NotFound before a header edit and Value after it
// doesn't make the checks stale.
func digest(res interop.Result, err error, complete bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%v\x00%s\x00%d", complete, errText(err), len(res.IncludeErrors))
	for _, ie := range res.IncludeErrors {
		fmt.Fprintf(&b, "\x00%d %d %s %s %d %d %s", ie.Index, ie.Kind, ie.Include, ie.File, ie.Line, ie.Column, ie.Msg)
	}
	names := make([]string, 0, len(res.Kinds))
	for name := range res.Kinds {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		fmt.Fprintf(&b, "\x00%s=%c", name, nsClass(res.Kinds[name]))
	}
	return b.String()
}

// nsClass returns what a check uses of the kind k (see digest): 'n' for a
// namespace, '?' for no answer, '-' for the rest.
func nsClass(k interop.Kind) byte {
	switch k {
	case interop.Namespace:
		return 'n'
	case interop.Unknown:
		return '?'
	}
	return '-'
}

// A probeRecord is what the compiler lookup and the probes of one check
// answered. A summary keeps the records of its package and of the packages
// it imports, directly or not (summary.records).
//
// The record of a summary from the disk cache (disk is set, see replay) has
// the answers of a check of another run of the server: it is stale when the
// prober has answers now and they differ, and provisional until the prober
// has them all.
type probeRecord struct {
	pr   *prober
	disk bool

	mu         sync.Mutex
	cxxUsed    bool
	cxxGen     int64
	cxxPending bool
	cxx        build.CXX // the compiler that the check got
	cxxErr     string    // the error of the lookup (errText)
	uses       []probeUse
}

// A probeUse is one answer of a probe to a check.
type probeUse struct {
	set      *probeSet
	cands    []string
	digest   string
	complete bool
}

func (r *probeRecord) addCXX(gen int64, pending bool, cxx build.CXX, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cxxUsed, r.cxxGen, r.cxxPending, r.cxx, r.cxxErr = true, gen, pending, cxx, errText(err)
}

func (r *probeRecord) add(u probeUse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.uses = append(r.uses, u)
}

// used reports whether the check asked for the compiler or a probe.
func (r *probeRecord) used() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cxxUsed || len(r.uses) > 0
}

// provisional reports whether an answer was missing: the compiler was
// being looked for, or a probe didn't know a name. (For a record from
// disk: whether the prober doesn't have all its answers yet.)
func (r *probeRecord) provisional() bool {
	if r.disk {
		_, provisional := r.diskState()
		return provisional
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cxxPending {
		return true
	}
	for _, u := range r.uses {
		if !u.complete {
			return true
		}
	}
	return false
}

// stale reports whether the prober would now answer differently. An include
// set is looked up again by its key: one that was evicted may be there
// again, with newer answers.
// (The lock order is prober.mu, then probeRecord.mu: the check's hooks add
// to the record under the prober's lock.)
func (r *probeRecord) stale() bool {
	if r.disk {
		stale, _ := r.diskState()
		return stale
	}
	r.mu.Lock()
	cxxUsed, cxxGen, uses := r.cxxUsed, r.cxxGen, r.uses
	r.mu.Unlock()
	r.pr.mu.Lock()
	defer r.pr.mu.Unlock()
	if cxxUsed && r.pr.cxx.gen != cxxGen {
		return true
	}
	for _, u := range uses {
		set := u.set
		if cur := r.pr.sets[set.key]; cur != nil {
			set = cur
		}
		if digest(set.answer(u.cands)) != u.digest {
			return true
		}
	}
	return false
}

// diskState compares the answers of the record r from disk (see replay)
// with what the prober knows now: r is stale when an answer that the prober
// has differs (the compiler lookup found another compiler, or failed
// differently; a probe answered differently), and provisional when the
// prober doesn't have every answer yet. The fields of a record from disk
// never change, so they need no lock.
func (r *probeRecord) diskState() (stale, provisional bool) {
	pr := r.pr
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if r.cxxUsed {
		switch l := &pr.cxx; {
		case !l.done:
			provisional = true
		case !sameCXX(l.cxx, r.cxx) || errText(l.err) != r.cxxErr:
			return true, false
		}
	}
	for _, u := range r.uses {
		set := u.set
		if cur := pr.sets[set.key]; cur != nil {
			set = cur
		}
		res, err, complete := set.answer(u.cands)
		switch {
		case !complete:
			provisional = true
		case digest(res, err, complete) != u.digest:
			return true, false
		}
	}
	return false, provisional
}

// A diskProbe is a probeRecord on disk: what the compiler lookup and the
// probes of a check answered, all of it (a provisional record is never
// written).
type diskProbe struct {
	CXXUsed bool
	CXX     build.CXX
	CXXErr  string
	Uses    []diskProbeUse
}

// A diskProbeUse is a probeUse on disk: the include set (with the compiler
// of the diskProbe), the candidates and the answer.
type diskProbeUse struct {
	Dir      string
	Flags    []string
	Includes []string
	Cands    []string
	Digest   string
}

// diskForm returns what r keeps on disk, or false when r can't be kept: it
// is provisional, or it is from disk itself.
func (r *probeRecord) diskForm() (*diskProbe, bool) {
	if r.disk {
		return nil, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cxxPending {
		return nil, false
	}
	d := &diskProbe{CXXUsed: r.cxxUsed, CXX: r.cxx, CXXErr: r.cxxErr}
	for _, u := range r.uses {
		if !u.complete {
			return nil, false
		}
		// The fields of a set that make its key never change.
		d.Uses = append(d.Uses, diskProbeUse{Dir: u.set.dir, Flags: u.set.flags, Includes: u.set.includes, Cands: u.cands, Digest: u.digest})
	}
	return d, true
}

// replay returns the record of a summary from disk with the answers d, and
// asks the prober for them in the background as the check did: it looks
// for the compiler (if it hasn't yet), and probes each include set with the
// compiler of d (the one that the check got), unless the prober knows the
// answers. When they come, the record is no longer provisional, and it is
// stale if they differ (Config.OnProbe says when).
func (pr *prober) replay(d *diskProbe) *probeRecord {
	r := &probeRecord{pr: pr, disk: true, cxxUsed: d.CXXUsed, cxx: d.CXX, cxxErr: d.CXXErr}
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if d.CXXUsed {
		if l := &pr.cxx; !l.done || l.err != nil && time.Since(l.at) >= cxxRetry {
			pr.lookCXXLocked()
		}
	}
	for _, u := range d.Uses {
		set := pr.setLocked(d.CXX, u.Dir, u.Flags, u.Includes)
		r.uses = append(r.uses, probeUse{set: set, cands: u.Cands, digest: u.Digest, complete: true})
		if len(set.names)+len(u.Cands) > maxProbeNames {
			clear(set.names)
		}
		for _, c := range u.Cands {
			set.names[c] = true
		}
		switch _, _, complete := set.answer(u.Cands); {
		case !complete:
			for _, c := range u.Cands {
				if _, ok := set.kinds[c]; !ok {
					set.want[c] = true
				}
			}
			pr.startLocked(set)
		case time.Since(set.validated) >= revalidateAfter:
			pr.startLocked(set)
		}
	}
	return r
}
