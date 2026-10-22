package cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/icheck"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The errors of the automatic argument adaptations (ADR-130, ADR-134) in the editor: a constant
// that does not fit the C++ parameter it is passed to, a runtime string at a format place, a Go
// string that a C function keeps. They need what clang says about the callee's parameters, so they
// come from a background analysis of the package, like the member facts: the same probe and
// declaration queries `gocpp check` makes, and the same pass (icheck.Adaptations), whose answer is
// kept here as diagnostics. Nothing waits for it:
//
//   - a package is scheduled after its ordinary check, when it has a call that an adaptation
//     could refuse (icheck.HasAdaptableCalls: a pass over the checked package, no compiler);
//   - AdaptDiagnostics reads only what is ready, and gives nothing for a package whose analysis is
//     not done, failed, or found the headers in a state that the compiler decides (a header that
//     does not compile): no diagnostic is better than a wrong one;
//   - when an analysis ends, Config.OnAdapt tells the server, which publishes the diagnostics of
//     the package again.
//
// There is one entry for each package folder, for the package value of its last check. The
// entry does not depend on the snapshot: a new snapshot that still has the same package value
// (an edit in another folder) keeps the diagnostics, so they do not flicker. An edit of the
// package makes a new value, and its diagnostics come again when the analysis is done. The
// analysis is valid while the headers it read are the same (probe.InputGuard), and HeadersChanged
// runs it again.

const maxAdaptEntries = 16
const maxAdaptBytes = 32 << 20

// adaptTimeout is the longest an analysis runs: the declaration queries of a large header (Dear
// ImGui) are the cold cost.
const adaptTimeout = 60 * time.Second

type adaptEntry struct {
	key    string
	pkg    *Package
	parent context.Context
	ctx    context.Context
	cancel context.CancelFunc
	used   uint64
	bytes  int64

	// ready says that the analysis ended well; diags are its diagnostics (none is an answer).
	ready bool
	diags []diag.Diagnostic
}

type adaptIndexer struct {
	mu      sync.Mutex
	closed  bool
	entries map[string]*adaptEntry // by folder and session
	clock   uint64
	bytes   int64
	wg      sync.WaitGroup
	sem     chan struct{}
	ctx     context.Context
	stop    context.CancelFunc
	cfg     Config

	// timeout bounds one analysis; the tests make it short.
	timeout time.Duration

	// run is the analysis; the tests replace it.
	run func(ctx context.Context, pkg *Package) ([]diag.Diagnostic, bool, error)
}

func newAdaptIndexer(cfg Config) *adaptIndexer {
	setting, err := typedinterop.ResolveMode("", os.Getenv)
	if err != nil || setting.Mode == typedinterop.Opaque {
		return nil
	}
	ctx, stop := context.WithCancel(context.Background())
	ix := &adaptIndexer{
		entries: make(map[string]*adaptEntry), sem: make(chan struct{}, maxProbes),
		ctx: ctx, stop: stop, cfg: cfg, timeout: adaptTimeout,
	}
	ix.run = func(ctx context.Context, pkg *Package) ([]diag.Diagnostic, bool, error) {
		return analyzeAdaptations(ctx, cfg, pkg)
	}
	return ix
}

func (ix *adaptIndexer) close() {
	if ix == nil {
		return
	}
	ix.mu.Lock()
	ix.closed = true
	ix.stop()
	for key, entry := range ix.entries {
		entry.cancel()
		delete(ix.entries, key)
	}
	ix.bytes = 0
	ix.mu.Unlock()
	ix.wg.Wait()
}

// refresh runs the analyses again: the headers may have changed on disk. The diagnostics of
// the entries are forgotten at once (they may describe headers that are gone).
func (ix *adaptIndexer) refresh() {
	if ix == nil {
		return
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.closed {
		return
	}
	for _, entry := range ix.entries {
		entry.cancel()
		ix.bytes -= entry.bytes
		entry.ready, entry.diags, entry.bytes = false, nil, 0
		ix.start(entry)
	}
}

func adaptEntryKey(snap *Snapshot, pkg *Package) string {
	return fmt.Sprintf("%p\x00%s", snap.session, pkg.Check.Load.Dir)
}

// scheduleAdapt is called after an ordinary package check. It does no disk access or compiler
// discovery.
func (s *Snapshot) scheduleAdapt(pkg *Package) {
	st, ok := s.state.(*state)
	if !ok || st.p.adapt == nil || pkg == nil || pkg.Check == nil || pkg.IsVariant() ||
		pkg.Check.Opaque == nil || pkg.Check.Info == nil || pkg.cost > maxAdaptBytes/2 {
		return
	}
	// A package with no header import has no C or C++ call (and the pass over its tree would
	// cost on the hot path of every pure-Go package).
	if len(typedinterop.UnitIncludes(pkg.Check)) == 0 {
		return
	}
	// The build stops at the errors of the program before it adapts anything.
	if pkg.Check.Errors.HasErrors() || pkg.Program.Errors.HasErrors() {
		return
	}
	if !probe.ProbesTarget(st.p.cfg.GOOS, st.p.cfg.GOARCH) && !typedinterop.StdHeadersOnly(pkg.Check) {
		return
	}
	for _, overlay := range s.overlays {
		if !strings.HasSuffix(overlay.URI().Path(), ".gp") {
			return // clang reads the disk, not the editor's unsaved headers
		}
	}
	if !pkg.hasAdaptableCalls() {
		return
	}
	st.p.adapt.schedule(adaptEntryKey(s, pkg), pkg)
}

type adaptableCallsKey struct{}

// adaptableCallsScans counts the passes over a package tree that hasAdaptableCalls made; a test
// shows that a package without header imports makes none.
var adaptableCallsScans atomic.Int64

// hasAdaptableCalls is icheck.HasAdaptableCalls on the package, once.
func (pkg *Package) hasAdaptableCalls() bool {
	return pkg.Memo(adaptableCallsKey{}, func() any {
		adaptableCallsScans.Add(1)
		return icheck.HasAdaptableCalls(pkg.Check)
	}).(bool)
}

func (ix *adaptIndexer) schedule(key string, pkg *Package) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.closed {
		return
	}
	ix.clock++
	if entry := ix.entries[key]; entry != nil {
		entry.used = ix.clock
		if entry.pkg == pkg {
			return // the analysis is under way, or done
		}
		entry.cancel() // an older package value of the folder
		ix.bytes -= entry.bytes
		delete(ix.entries, key)
	}
	cost := max(pkg.cost, 256)
	for len(ix.entries) >= maxAdaptEntries || ix.bytes+cost > maxAdaptBytes {
		var oldest string
		for k, entry := range ix.entries {
			if oldest == "" || entry.used < ix.entries[oldest].used {
				oldest = k
			}
		}
		if oldest == "" {
			return
		}
		ix.entries[oldest].cancel()
		ix.bytes -= ix.entries[oldest].bytes
		delete(ix.entries, oldest)
	}
	entry := &adaptEntry{key: key, pkg: pkg, parent: ix.ctx, used: ix.clock, bytes: cost}
	ix.entries[key] = entry
	ix.bytes += cost
	ix.start(entry)
}

// start runs the analysis of the entry in the background. ix.mu is held.
func (ix *adaptIndexer) start(entry *adaptEntry) {
	entry.ctx, entry.cancel = context.WithTimeout(entry.parent, ix.timeout)
	ctx, pkg := entry.ctx, entry.pkg
	ix.wg.Add(1)
	go func() {
		defer ix.wg.Done()
		select {
		case ix.sem <- struct{}{}:
			defer func() { <-ix.sem }()
		case <-ctx.Done():
			return
		}
		diags, ok, err := ix.runOnce(ctx, pkg)
		if err != nil || !ok || ctx.Err() != nil {
			return
		}
		ix.mu.Lock()
		if ix.entries[entry.key] != entry || entry.ctx != ctx || ctx.Err() != nil {
			ix.mu.Unlock()
			return
		}
		entry.ready, entry.diags = true, diags
		// The timeout bounds the run, not the answer: the diagnostics stay valid after it.
		entry.cancel()
		extra := int64(len(diags)) * 256
		entry.bytes += extra
		ix.bytes += extra
		dir := pkg.Check.Load.Dir
		ix.mu.Unlock()
		if ix.cfg.OnAdapt != nil {
			ix.cfg.OnAdapt(dir)
		}
	}()
}

// runOnce is the analysis with one retry: headers just saved inside the runner's conservative
// race window make the first run fail.
func (ix *adaptIndexer) runOnce(ctx context.Context, pkg *Package) ([]diag.Diagnostic, bool, error) {
	diags, ok, err := ix.run(ctx, pkg)
	if err == nil || ctx.Err() != nil {
		return diags, ok, err
	}
	timer := time.NewTimer(revalidateAfter)
	defer timer.Stop()
	select {
	case <-timer.C:
		return ix.run(ctx, pkg)
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
}

// AdaptDiagnostics returns the diagnostics of the argument adaptations of the package, and
// whether they are ready. Like MemberSymbols it reads only what is retained: no compiler, no disk
// access, no scheduling. A package that is not ready, whose analysis failed, or whose value is
// not the one that was analyzed, has none.
func (s *Snapshot) AdaptDiagnostics(pkg *Package) ([]diag.Diagnostic, bool) {
	st, ok := s.state.(*state)
	if !ok || st.p.adapt == nil || pkg == nil || pkg.Check == nil || pkg.IsVariant() {
		return nil, false
	}
	ix := st.p.adapt
	ix.mu.Lock()
	defer ix.mu.Unlock()
	entry := ix.entries[adaptEntryKey(s, pkg)]
	if entry == nil || entry.pkg != pkg || !entry.ready {
		return nil, false
	}
	return slices.Clone(entry.diags), true
}

// analyzeAdaptations runs the probe and the declaration queries on the package, as `gocpp check`
// does, and returns the refusals of the adaptations. false (with no error) says that the
// headers are not in a state to decide (one does not compile).
func analyzeAdaptations(ctx context.Context, cfg Config, pkg *Package) ([]diag.Diagnostic, bool, error) {
	cxx, err := probe.FindClangFor(ctx, cfg.GOOS, cfg.GOARCH)
	if err != nil {
		return nil, false, err
	}
	probeConfig := probe.Config{CXX: cxx, Dir: pkg.Check.Load.Dir, Flags: pkg.Check.ProbeFlags}
	inputs, err := probe.CaptureInputs(ctx, probeConfig, typedinterop.UnitIncludes(pkg.Check))
	if err != nil {
		return nil, false, err
	}
	target := ftype.LP64
	if cfg.GOOS == "windows" {
		target = ftype.LLP64
	}
	report, err := typedinterop.Analyze(ctx, pkg.Check, pkg.Fset(), typedinterop.Options{
		Clang: cxx, Runner: new(probe.Runner), GOOS: cfg.GOOS, GOARCH: cfg.GOARCH,
		Decls: true, StringToCStr: true, Target: target, MaxJSON: 4 << 20, MaxDecl: 128 << 10,
	})
	if err != nil {
		return nil, false, err
	}
	diags, skipped := icheck.Adaptations(pkg.Check, pkg.Fset(), report)
	if skipped != "" {
		return nil, false, nil
	}
	if !inputs.Unchanged(ctx) {
		return nil, false, errors.New("adaptation analysis inputs changed")
	}
	return diags, true, nil
}
