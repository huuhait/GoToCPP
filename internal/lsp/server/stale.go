package server

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/fspath"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// The views of the files that have a syntax error (step 11.13, review round 2).
//
// A file that is being typed has an error at almost every keystroke, and the answers for it come from the check of
// a repaired text (repair.go), which is a check of the whole package, with the lowering of its `!` forms: tens of
// milliseconds for a big package, more than a request may take. So a request does not wait for it when it does
// not have to. The server keeps, for each such file, its last *repairView*: the repaired check of some earlier text,
// built in the background (one build at a time for a file, always of the newest text, whatever the number of
// requests). Reuse is conservative: all intervening snapshot events must be successful edits to this file alone,
// and the changed text must lie wholly within the same blanked function body, preserving its lexical boundary,
// everything outside it, and its line count (macro hidden arguments depend on lines). The old partial hints/tokens
// of that body are omitted, not shifted onto the current body; a hover in it waits for its current partial check.
// A signature, sibling/import edit, open/close, disk invalidation, or unknown snapshot event forces a current build.
//
// Safe answers outside that body are shifted across the edit. When the newest build finishes the client is asked
// to refresh tokens and hints if supported. A request that cannot reuse a view waits for its build, or falls back
// to its own snapshot's check (syncViewQuery) when concurrent editing keeps the builder ahead/behind it. Views are
// never used for rename, references or code actions, which need the text as it is.

// A shift maps the offsets of a text to the offsets of another that differs from it in one place: the old text
// from p to oldEnd was replaced by the new text from p to newEnd.
type shift struct{ p, oldEnd, newEnd int }

// diffShift returns the shift from old to new: the common prefix and the common suffix (which do not overlap) are
// the same text.
func diffShift(old, new []byte) shift {
	n := min(len(old), len(new))
	p := 0
	for p < n && old[p] == new[p] {
		p++
	}
	s := 0
	for s < n-p && old[len(old)-1-s] == new[len(new)-1-s] {
		s++
	}
	return shift{p, len(old) - s, len(new) - s}
}

// toNew maps an offset of the old text (a place between characters, a hint's) to the new text, or false for an offset
// at or inside what was replaced: the character before or after the change may have joined the one next to it.
func (h shift) toNew(o int) (int, bool) {
	switch {
	case o < h.p:
		return o, true
	case o > h.oldEnd:
		return o + h.newEnd - h.oldEnd, true
	}
	return 0, false
}

// toOld maps an offset of the new text to the old one (the place a request asks about), or false for an offset
// inside what is new.
func (h shift) toOld(o int) (int, bool) {
	switch {
	case o < h.p:
		return o, true
	case o >= h.newEnd:
		return o + h.oldEnd - h.newEnd, true
	}
	return 0, false
}

// rangeToNew maps the range [a, b) of the old text (a token, the range of a hover) to the new text: it must lie
// before the change or after it, without touching it, since a word next to it may have grown.
func (h shift) rangeToNew(a, b int) (int, int, bool) {
	switch {
	case b < h.p:
		return a, b, true
	case a > h.oldEnd:
		d := h.newEnd - h.oldEnd
		return a + d, b + d, true
	}
	return 0, 0, false
}

// A repairView is the repaired check of one text of a file.
type repairView struct {
	key    viewKey
	text   []byte
	mapper *protocol.Mapper
	r      *repair
	snap   *cache.Snapshot
	free   func()         // releases snap
	pkg    *cache.Package // the package as it is (with load errors)
	fixed  *cache.Package // the package with the repaired text
	refs   int            // under the lock of the fileView: the fileView's own and the requests that use it
}

type viewKey struct {
	hash     cache.Hash
	snapshot uint64
}

// defaultViewGap is views.gap of a server.
const defaultViewGap = 100 * time.Millisecond

// views are the views of the files of a server.
type views struct {
	session *cache.Session
	ctx     context.Context
	cancel  context.CancelFunc
	refresh func() // asks the client to ask again for tokens and hints; may be nil
	// gap is the least time the builder of a file waits after a build before the next one, and half again the time the
	// last build took if that is more, unless a request that cannot be answered from the last view asks for it at once:
	// while someone types, the builds are at most 40% of the time (the views are a few keystrokes old) and not all of it.
	gap time.Duration
	// hold, if not nil, is waited for at the start of every build (the tests use it to make an answer from an older text happen).
	hold chan struct{}

	any   atomic.Bool // a view was made (a file had a syntax error)
	mu    sync.Mutex
	files map[string]*fileView
}

func newViews(session *cache.Session) *views {
	ctx, cancel := context.WithCancel(context.Background())
	return &views{session: session, ctx: ctx, cancel: cancel, gap: defaultViewGap, files: make(map[string]*fileView)}
}

// close stops the builds and lets go of the snapshots.
func (vs *views) close() {
	if vs == nil {
		return
	}
	vs.cancel()
	vs.mu.Lock()
	files := vs.files
	vs.files = make(map[string]*fileView)
	vs.mu.Unlock()
	for _, fv := range files {
		fv.mu.Lock()
		old := fv.last
		fv.last = nil
		fv.mu.Unlock()
		fv.unref(old)
	}
}

// A fileView is what the server knows of one file.
type fileView struct {
	vs   *views
	uri  protocol.DocumentURI
	path string

	mu sync.Mutex
	// changeStart..changeEnd is a contiguous run of snapshots made only by successful didChanges to this file.
	// Any unobserved event (another file, disk invalidation, recheck, open/close) breaks the run. This is deliberately
	// conservative: equal target text does not mean equal semantic inputs.
	changeStart, changeEnd uint64
	last                   *repairView   // the newest completed view, or nil
	running                bool          // a build runs
	want                   bool          // a request wants a build of the newest text
	wantOrig               bool          // and the check of the text as it is too (hints and tokens need it)
	lastEnd                time.Time     // when the last build ended
	lastDur                time.Duration // and how long it took
	staleServed            bool          // an answer from an older text was given since the last build
	changed                chan struct{} // closed when a build ends
	kick                   chan struct{} // a request that waits for a build asks for it at once (buffered: one)
}

func (vs *views) file(uri protocol.DocumentURI) *fileView {
	path := uri.Path()
	key := fspath.Key(path)
	vs.mu.Lock()
	defer vs.mu.Unlock()
	fv := vs.files[key]
	if fv == nil {
		fv = &fileView{vs: vs, uri: uri, path: path, changed: make(chan struct{}), kick: make(chan struct{}, 1)}
		vs.files[key] = fv
	}
	return fv
}

// changed records a successful edit, without retaining a chain of snapshots. before and after bracket DidChange:
// if background work made another snapshot meanwhile, it is not known to be a target-file-only edit.
func (vs *views) changed(uri protocol.DocumentURI, before, after uint64) {
	if vs == nil || !vs.any.Load() || after != before+1 {
		return
	}
	vs.mu.Lock()
	fv := vs.files[fspath.Key(uri.Path())]
	vs.mu.Unlock()
	if fv == nil {
		return
	}
	fv.mu.Lock()
	if fv.changeEnd != before {
		fv.changeStart = after
	}
	fv.changeEnd = after
	fv.mu.Unlock()
}

// sameInputs reports whether all semantic inputs except this file are known unchanged (under fv.mu).
func (fv *fileView) sameInputs(v *repairView, snapshot uint64) bool {
	id := v.key.snapshot
	return id == snapshot || id < snapshot && fv.changeStart <= id+1 && fv.changeEnd >= snapshot
}

// reusableBody verifies the lexical/declaration boundary of a stale repair. Only edits wholly inside the same
// blanked function body can reuse its repaired check, and never across a line change (macro hidden line arguments).
// The complete text outside that body and every blanked boundary must match, not merely the function signature.
// Whole-file answers omit that body's old partial results; a request inside it waits for its current partial check.
func reusableBody(v *repairView, pr *prepared, sh shift) ([2]int, bool) {
	for i, sp := range v.r.blanked {
		lo, hi := sp[0], sp[1]
		if sh.p < lo || sh.oldEnd > hi {
			continue
		}
		nhi := hi + sh.newEnd - sh.oldEnd
		if nhi < lo || nhi > len(pr.src) || len(v.r.blanked) != len(pr.r.blanked) ||
			!bytes.Equal(v.text[:lo], pr.src[:lo]) || !bytes.Equal(v.text[hi:], pr.src[nhi:]) ||
			bytes.Count(v.text[lo:hi], []byte{'\n'}) != bytes.Count(pr.src[lo:nhi], []byte{'\n'}) {
			return [2]int{}, false
		}
		for j, old := range v.r.blanked {
			want := old
			if j == i {
				want[1] = nhi
			} else if j > i {
				want[0] += nhi - hi
				want[1] += nhi - hi
			}
			if pr.r.blanked[j] != want {
				return [2]int{}, false
			}
		}
		// A whole declaration or macro body is not safe: it can declare names/signatures used elsewhere.
		starts := declStarts(v.text)
		next, _ := slices.BinarySearch(starts, lo)
		if next == 0 {
			return [2]int{}, false
		}
		start, end := starts[next-1], len(v.text)
		if next < len(starts) {
			end = starts[next]
		}
		a, b, body := bodySpan(v.text, start, end)
		if !body || a != lo || b != hi || !bytes.Equal(v.r.text[:lo], pr.r.text[:lo]) || !bytes.Equal(v.r.text[hi:], pr.r.text[nhi:]) {
			return [2]int{}, false
		}
		for _, line := range bytes.Split(v.text[start:lo], []byte{'\n'}) {
			if hasWord(line, "macro") {
				return [2]int{}, false
			}
		}
		return sp, true
	}
	return [2]int{}, false
}

// ref takes a reference of v (under fv.mu).
func (fv *fileView) ref(v *repairView) *repairView {
	if v != nil {
		v.refs++
	}
	return v
}

// unref lets go of a reference of v; the last one releases the snapshot.
func (fv *fileView) unref(v *repairView) {
	if v == nil {
		return
	}
	fv.mu.Lock()
	v.refs--
	last := v.refs == 0
	fv.mu.Unlock()
	if last && v.free != nil {
		v.free()
	}
}

// A prepared is a file that needs a repair, and its repair. pkg is nil on the fast path (a file that has a view
// already and a syntax error: the package has load errors, and the request need not wait for the check that says so).
type prepared struct {
	path   string
	pkg    *cache.Package
	pf     *cache.ParsedFile
	r      *repair
	src    []byte
	mapper *protocol.Mapper
}

// known reports whether the file has a view (it had a syntax error and was repaired).
func (vs *views) known(uri protocol.DocumentURI) bool {
	if !vs.any.Load() { // no file had a syntax error yet: the usual session
		return false
	}
	vs.mu.Lock()
	fv := vs.files[fspath.Key(uri.Path())]
	vs.mu.Unlock()
	if fv == nil {
		return false
	}
	fv.mu.Lock()
	defer fv.mu.Unlock()
	return fv.last != nil
}

// forget lets go of the view of the file (it is closed): the snapshot it holds can go. (A view is kept while the file
// is good: the next keystroke that breaks it is answered from it.)
func (vs *views) forget(uri protocol.DocumentURI) {
	if vs == nil {
		return
	}
	key := fspath.Key(uri.Path())
	vs.mu.Lock()
	fv := vs.files[key]
	vs.mu.Unlock()
	if fv == nil {
		return
	}
	fv.mu.Lock()
	old := fv.last
	fv.last = nil
	fv.mu.Unlock()
	fv.unref(old)
}

// fastRepair is prepareRepair for a file that has a view: when the file still has a syntax error, its package has a load error
// and says nothing more, so the request does not ask for the package (the check of the new text, which the diagnostics
// are making and which is the slowest part of a keystroke).
func fastRepair(ctx context.Context, snap *cache.Snapshot, uri protocol.DocumentURI) *prepared {
	pf, err := snap.ParseFile(ctx, uri, cache.ParseFull)
	if err != nil || pf.Err == nil {
		return nil
	}
	r := repairOf(pf, nil)
	if r == nil {
		return nil
	}
	return newPrepared(snap, uri, nil, pf, r)
}

// newPrepared makes the prepared of the file.
func newPrepared(snap *cache.Snapshot, uri protocol.DocumentURI, pkg *cache.Package, pf *cache.ParsedFile, r *repair) *prepared {
	path := uri.Path()
	src, err := snap.ReadFile(path)
	if err != nil {
		return nil
	}
	var mapper *protocol.Mapper
	if o := snap.Overlay(uri); o != nil && !o.Broken() && o.Text() == string(src) {
		mapper = o.Mapper()
	} else {
		mapper = protocol.NewMapper(string(src))
	}
	return &prepared{path, pkg, pf, r, src, mapper}
}

// prepareRepair returns the repair of the file uri in the snapshot, or nil when the file does not need one (its package has no
// load errors, the file parses and has none) or cannot have one.
func prepareRepair(ctx context.Context, snap *cache.Snapshot, uri protocol.DocumentURI) (*prepared, *cache.Package, error) {
	path := uri.Path()
	if path == "" {
		return nil, nil, nil
	}
	pkg, err := snap.PackageOf(ctx, uri)
	if err != nil || pkg.Check == nil || pkg.Check.Checked() || !pkg.Check.HasPartial() {
		return nil, pkg, err
	}
	pr := repairOfFile(ctx, snap, uri, pkg)
	return pr, pkg, nil
}

// repairOfFile is prepareRepair for a package with load errors.
func repairOfFile(ctx context.Context, snap *cache.Snapshot, uri protocol.DocumentURI, pkg *cache.Package) *prepared {
	pf, err := snap.ParseFile(ctx, uri, cache.ParseFull)
	if err != nil {
		return nil
	}
	var loadErrs []int // the load errors of this file that are not syntax errors (a macro that is not there)
	if pf.Err == nil {
		for _, lp := range pkg.Program.Roots[:1] {
			for _, d := range lp.Errors.Diagnostics() {
				if d.Severity == diag.Error && d.Pos.Filename != "" && fspath.Equal(d.Pos.Filename, pf.Path) && d.Pos.Offset >= 0 {
					loadErrs = append(loadErrs, d.Pos.Offset)
				}
			}
		}
		if len(loadErrs) == 0 {
			return nil
		}
		slices.Sort(loadErrs)
	}
	r := repairOf(pf, loadErrs)
	if r == nil {
		return nil
	}
	return newPrepared(snap, uri, pkg, pf, r)
}

// newViewQuery is newQuery for the features that look at a file (hover, definition, hints, tokens): when the file
// has a syntax error, the query is made on the check of the repaired text wherever that can answer. A request
// at a place (whole is false) outside the blanked declarations is answered by the repaired check alone: the check
// of the text as it is (the partial one, which costs a type check) is not made. A request for the whole file
// (hints, tokens) has the partial check as the second view, for the declarations that were blanked
// (query.views), made when the request needs it. With a server (e.views), the repaired check is the view
// of the file (above), possibly of an older text (stale is true: the query has a shift, and answers
// must go through it).
func newViewQuery(ctx context.Context, e env, uri protocol.DocumentURI, at protocol.Position, whole, allowStale bool) (*query, error) {
	return viewQuery(ctx, e, uri, at, whole, allowStale, false)
}

// newHoverQuery avoids building a repaired package for a place whose answer can only come from the current
// partial check anyway. The blanked body's old answers are never used; other requests keep their existing policy.
func newHoverQuery(ctx context.Context, e env, uri protocol.DocumentURI, at protocol.Position, allowStale bool) (*query, error) {
	return viewQuery(ctx, e, uri, at, false, allowStale, true)
}

func viewQuery(ctx context.Context, e env, uri protocol.DocumentURI, at protocol.Position, whole, allowStale, partialBody bool) (*query, error) {
	snap := e.snapshot
	if err := ctx.Err(); err != nil {
		return nil, err // a cached package would answer anyway
	}
	var pr *prepared
	if e.views != nil && e.views.known(uri) {
		pr = fastRepair(ctx, snap, uri)
	}
	if pr == nil {
		var pkg *cache.Package
		var err error
		pr, pkg, err = prepareRepair(ctx, snap, uri)
		if pr == nil { // the common case: the file is good, and the package has been found (once)
			switch {
			case uri.Path() == "" || errors.Is(err, cache.ErrNoPackage):
				return nil, nil
			case err != nil:
				return nil, err
			}
			return queryOfPackage(snap, pkg, uri.Path(), at)
		}
	}
	if partialBody && pr.r.within(pr.mapper.Offset(at)) {
		// queryOf would discard the repaired check at this place and call origFn. Use that exact current
		// snapshot's partial check now, without waiting for an unrelated whole-package repaired build.
		if pr.pkg != nil {
			return queryIn(snap, pr.pkg, pr.path, pr.src, at), nil
		}
		return newQuery(ctx, snap, uri, at)
	}
	if e.views == nil {
		return syncViewQuery(ctx, snap, uri, at, whole, pr)
	}
	fv := e.views.file(uri)
	key := viewKey{pr.pf.Hash, snap.ID()}
	fv.mu.Lock()
	if fv.last != nil && fv.last.key == key { // the view of this very text
		v := fv.ref(fv.last)
		fv.mu.Unlock()
		return fv.queryOf(ctx, v, pr, at, whole, nil, key)
	}
	fv.want = true
	fv.wantOrig = fv.wantOrig || whole
	if !fv.running {
		fv.running = true
		go fv.run()
	}
	wake := fv.changed
	v := fv.ref(fv.last)
	fv.mu.Unlock()
	if q, err := fv.candidate(ctx, v, pr, at, whole, allowStale, key); q != nil || err != nil {
		return q, err
	}
	// Wait for the build that is under way and take what it made if it fits; if it does not, for the next one.
	for attempt := 0; attempt < 2; attempt++ {
		select {
		case fv.kick <- struct{}{}: // (the builder need not wait for its gap)
		default:
		}
		select {
		case <-wake:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		fv.mu.Lock()
		v = fv.ref(fv.last)
		if v == nil || v.key != key {
			fv.want = true // the newest text, once more
			fv.wantOrig = fv.wantOrig || whole
			if !fv.running {
				fv.running = true
				go fv.run()
			}
		}
		wake = fv.changed
		fv.mu.Unlock()
		if q, err := fv.candidate(ctx, v, pr, at, whole, allowStale, key); q != nil || err != nil {
			return q, err
		}
	}
	return syncViewQuery(ctx, snap, uri, at, whole, pr)
}

// candidate consumes the request's reference of v, accepting only its own snapshot or a verified body-only edit
// with unchanged other inputs. In particular, a waiting non-stale request never accepts an intervening stale build.
func (fv *fileView) candidate(ctx context.Context, v *repairView, pr *prepared, at protocol.Position, whole, allowStale bool, key viewKey) (*query, error) {
	if v == nil {
		return nil, nil
	}
	if v.key == key {
		return fv.queryOf(ctx, v, pr, at, whole, nil, key)
	}
	fv.mu.Lock()
	same := fv.sameInputs(v, key.snapshot)
	fv.mu.Unlock()
	if !allowStale || !same {
		fv.unref(v)
		return nil, nil
	}
	sh := diffShift(v.text, pr.src)
	blank, ok := reusableBody(v, pr, sh)
	if !ok || !whole && blank[0] <= pr.mapper.Offset(at) && pr.mapper.Offset(at) <= blank[1]+sh.newEnd-sh.oldEnd {
		fv.unref(v)
		return nil, nil
	}
	q, err := fv.queryOf(ctx, v, pr, at, whole, &sh, key)
	if q != nil && q.shift != nil {
		q.staleBlank = blank
	}
	return q, err
}

// queryOf makes the query of the request on the view v, which has the reference of the request (it is let go of by
// query.close, or here when there is no query). sh is nil when v has the text of the request; otherwise it
// maps the offsets of the text of v to the request's (a stale answer): the query is nil when the request cannot be
// answered from v so.
func (fv *fileView) queryOf(ctx context.Context, v *repairView, pr *prepared, at protocol.Position, whole bool, sh *shift, key viewKey) (*query, error) {
	mapper := pr.mapper
	off := mapper.Offset(at)
	oldAt := at
	if sh != nil {
		o, ok := sh.toOld(off)
		if !whole && !ok {
			fv.unref(v)
			return nil, nil
		}
		if ok {
			oldAt = v.mapper.Position(o)
		}
	}
	fixed := queryIn(v.snap, v.fixed, pr.path, v.text, oldAt)
	if fixed == nil {
		fv.unref(v)
		return nil, nil
	}
	fixed.blanked, fixed.isFixed = v.r, true
	fixed.origFn = func() *query { return queryIn(v.snap, v.pkg, pr.path, v.text, oldAt) }
	fixed.release = func() { fv.unref(v) }
	if sh != nil {
		fv.mu.Lock()
		if current := fv.last; current != nil && current != v && current.key.snapshot >= key.snapshot {
			// A request may keep an old candidate while another request finishes the current build. Do not
			// return incomplete stale tokens/hints after the builder's refresh decision has already happened.
			if current.key == key {
				fv.ref(current)
				fv.mu.Unlock()
				fv.unref(v)
				return fv.queryOf(ctx, current, pr, at, whole, nil, key)
			}
			fv.mu.Unlock()
			fv.unref(v)
			return nil, nil // a newer, different snapshot cannot answer this request; let it use its own check
		}
		// This and the builder's publication/refresh decision use the same lock: either a current view
		// was taken above, or publication will see that a stale answer needs a refresh.
		fv.staleServed = true
		fv.mu.Unlock()
		fixed.shift, fixed.newMapper, fixed.newSrc = sh, mapper, pr.src
	}
	if !whole {
		oo := fixed.mapper.Offset(oldAt)
		if v.r.within(oo) {
			orig := fixed.origFn()
			if orig == nil {
				fv.unref(v)
				return nil, nil
			}
			orig.release, orig.shift, orig.newMapper, orig.newSrc = fixed.release, fixed.shift, fixed.newMapper, fixed.newSrc
			return orig, nil
		}
	}
	return fixed, nil
}

// run makes the builds, one at a time, each of the newest text, while requests want one.
func (fv *fileView) run() {
	vs := fv.vs
	for {
		fv.mu.Lock()
		if !fv.want || vs.ctx.Err() != nil {
			fv.running = false
			fv.mu.Unlock()
			return
		}
		fv.want = false
		wantOrig := fv.wantOrig
		wait := fv.last != nil && vs.gap > 0 // (the first build, and the one after a pause, start at once)
		gap := max(vs.gap, fv.lastDur*3/2)
		since := time.Since(fv.lastEnd)
		fv.mu.Unlock()
		if wait && since < gap {
			t := time.NewTimer(gap - since)
			select {
			case <-t.C:
			case <-fv.kick:
				t.Stop()
			case <-vs.ctx.Done():
				t.Stop()
			}
		}
		t0 := time.Now()
		nv := fv.build(wantOrig)
		fv.mu.Lock()
		fv.lastDur = time.Since(t0)
		if vs.ctx.Err() != nil && nv != nil { // the server ended during the build: nobody will use it
			release := nv.free
			fv.running = false
			fv.mu.Unlock()
			if release != nil {
				release()
			}
			return
		}
		old := fv.last
		fv.last = nv
		fv.lastEnd = time.Now()
		if nv != nil {
			nv.refs = 1 // the fileView's own
			vs.any.Store(true)
		}
		ch := fv.changed
		fv.changed = make(chan struct{})
		refresh := fv.staleServed && nv != nil
		fv.staleServed = false
		fv.mu.Unlock()
		close(ch)
		fv.unref(old)
		if refresh && vs.refresh != nil {
			vs.refresh()
		}
	}
}

// build makes the view of the newest text of the file, or nil when the file needs none now.
func (fv *fileView) build(wantOrig bool) *repairView {
	vs := fv.vs
	if vs.hold != nil {
		select {
		case <-vs.hold:
		case <-vs.ctx.Done():
			return nil
		}
	}
	snap, release := vs.session.Snapshot()
	keep := false
	defer func() {
		if !keep {
			release()
		}
	}()
	pr, _, _ := prepareRepair(vs.ctx, snap, fv.uri)
	if pr == nil || pr.pkg == nil {
		return nil
	}
	fixed, err := snap.PackageWithFile(vs.ctx, pr.pf.Path, pr.r.text)
	if err != nil {
		return nil
	}
	if wantOrig {
		pr.pkg.Check.PartialResult()
	}
	keep = true
	return &repairView{key: viewKey{pr.pf.Hash, snap.ID()}, text: pr.src, mapper: protocol.NewMapper(string(pr.src)), r: pr.r, snap: snap, free: release, pkg: pr.pkg, fixed: fixed}
}

// syncViewQuery is newViewQuery without views: the repaired check is made now, for this request alone.
func syncViewQuery(ctx context.Context, snap *cache.Snapshot, uri protocol.DocumentURI, at protocol.Position, whole bool, pr *prepared) (*query, error) {
	fixedPkg, err := snap.PackageWithFile(ctx, pr.pf.Path, pr.r.text)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return newQuery(ctx, snap, uri, at)
	}
	fixed := queryIn(snap, fixedPkg, pr.pf.Path, pr.src, at)
	if fixed == nil {
		return newQuery(ctx, snap, uri, at)
	}
	fixed.blanked, fixed.isFixed = pr.r, true
	fixed.origFn = func() *query {
		if pr.pkg == nil {
			pkg, err := snap.PackageOf(ctx, uri)
			if err != nil {
				return nil
			}
			pr.pkg = pkg
		}
		return queryIn(snap, pr.pkg, pr.pf.Path, pr.src, at)
	}
	if !whole && pr.r.within(fixed.mapper.Offset(at)) {
		return fixed.origFn(), nil // (nil when the file is not one of its package: as newQuery)
	}
	return fixed, nil
}

// close lets go of what the query holds (a view).
func (q *query) close() {
	if q != nil && q.release != nil {
		q.release()
		q.release = nil
	}
}

// ---- the answers of a stale query ----

// newRange maps a range of the text of the view to the request's text, or reports that it cannot be.
func (q *query) newRange(r protocol.Range) (protocol.Range, bool) {
	a, b, err := q.mapper.RangeOffsets(r)
	if err != nil {
		return r, false
	}
	na, nb, ok := q.shift.rangeToNew(a, b)
	if !ok {
		return r, false
	}
	return protocol.Range{Start: q.newMapper.Position(na), End: q.newMapper.Position(nb)}, true
}
