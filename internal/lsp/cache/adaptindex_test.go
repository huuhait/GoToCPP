package cache

import (
	"context"
	"go/token"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The cache of the argument adaptation diagnostics (adaptindex.go) with a made-up analysis: what is
// ready when, for which package value, and what is never scheduled.

const adaptSource = "package main\nimport \"./api.hpp\"\nfunc main() { api.take_char(200) }\n"

type adaptWorld struct {
	*world
	runs  atomic.Int32
	ready chan string // the folders OnAdapt was called with
}

// newAdaptWorld is a world over a package with the source, whose analysis is run, a function of
// the package value (nil: one diagnostic).
func newAdaptWorld(t *testing.T, source string, run func(ctx context.Context, pkg *Package) ([]diag.Diagnostic, bool, error)) *adaptWorld {
	t.Helper()
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Cleanup(probe.SetRacyWindow(0))
	w := &adaptWorld{ready: make(chan string, 16)}
	cfg := testConfig()
	cfg.CXX = func(ctx context.Context) (build.CXX, error) { return build.FindCXXFor(ctx, cfg.GOOS, cfg.GOARCH) }
	cfg.OnAdapt = func(dir string) { w.ready <- dir }
	p := NewPipeline(cfg)
	if p.adapt == nil {
		t.Fatal("no adaptation indexer in typed mode")
	}
	p.adapt.run = func(ctx context.Context, pkg *Package) ([]diag.Diagnostic, bool, error) {
		w.runs.Add(1)
		if run != nil {
			return run(ctx, pkg)
		}
		return []diag.Diagnostic{{Pos: token.Position{Filename: "x.gp", Line: 3, Column: 10}, Msg: "refused"}}, true, nil
	}
	w.world = &world{t: t, root: writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/adapt\n", "main.gp": source,
		"api.hpp": "inline void take_char(char c) {}\n",
	}), p: p, s: NewSession(p.NewState())}
	t.Cleanup(p.Close)
	return w
}

func (w *adaptWorld) waitReady() {
	w.t.Helper()
	select {
	case <-w.ready:
	case <-time.After(10 * time.Second):
		w.t.Fatal("OnAdapt was not called")
	}
}

func (w *adaptWorld) diags(pkg *Package) ([]diag.Diagnostic, bool) {
	snap, release := w.s.Snapshot()
	defer release()
	return snap.AdaptDiagnostics(pkg)
}

// Nothing is ready while the analysis runs, the diagnostics are when it ends well, and the server
// is told.
func TestAdaptDiagnosticsAreReadyWhenTheAnalysisEnds(t *testing.T) {
	proceed := make(chan struct{})
	w := newAdaptWorld(t, adaptSource, func(ctx context.Context, pkg *Package) ([]diag.Diagnostic, bool, error) {
		select {
		case <-proceed:
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
		return []diag.Diagnostic{{Msg: "refused"}}, true, nil
	})
	pkg := w.pkg(".")
	if _, ok := w.diags(pkg); ok {
		t.Fatal("diagnostics are ready before the analysis ended")
	}
	close(proceed)
	w.waitReady()
	got, ok := w.diags(pkg)
	if !ok || len(got) != 1 || got[0].Msg != "refused" {
		t.Fatalf("diagnostics = %v, ready %v", got, ok)
	}
}

// The timeout bounds the run, not the answer: diagnostics that an analysis gave in time are still
// read after the time is over.
func TestAdaptDiagnosticsOutliveTheTimeoutOfTheirAnalysis(t *testing.T) {
	w := newAdaptWorld(t, adaptSource, nil)
	w.p.adapt.timeout = 300 * time.Millisecond
	pkg := w.pkg(".")
	w.waitReady()
	time.Sleep(600 * time.Millisecond)
	got, ok := w.diags(pkg)
	if !ok || len(got) != 1 {
		t.Fatalf("diagnostics after the timeout = %v, ready %v", got, ok)
	}
}

// An analysis that failed, or that found the headers not fit to decide, leaves nothing ready and
// does not tell the server.
func TestAdaptDiagnosticsOfAnUndecidedAnalysisAreNeverReady(t *testing.T) {
	w := newAdaptWorld(t, adaptSource, func(context.Context, *Package) ([]diag.Diagnostic, bool, error) {
		return nil, false, nil
	})
	pkg := w.pkg(".")
	deadline := time.Now().Add(5 * time.Second)
	for w.runs.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	if _, ok := w.diags(pkg); ok {
		t.Error("diagnostics are ready for an analysis that did not decide")
	}
	select {
	case dir := <-w.ready:
		t.Errorf("OnAdapt(%s) for an analysis that did not decide", dir)
	default:
	}
}

// The diagnostics belong to the package value that was analyzed: an edit makes a new value with
// none ready, and the old value's are gone with its entry.
func TestAdaptDiagnosticsBelongToThePackageValue(t *testing.T) {
	w := newAdaptWorld(t, adaptSource, nil)
	old := w.pkg(".")
	w.waitReady()
	if _, ok := w.diags(old); !ok {
		t.Fatal("not ready")
	}
	w.open("main.gp", adaptSource)
	w.edit("main.gp", adaptSource+"\n")
	edited := w.pkg(".")
	if edited == old {
		t.Fatal("the edit did not make a new package value")
	}
	w.waitReady()
	if _, ok := w.diags(old); ok {
		t.Error("the old package value still has diagnostics")
	}
	if _, ok := w.diags(edited); !ok {
		t.Error("the edited package has none after its analysis")
	}
	w.p.adapt.mu.Lock()
	n := len(w.p.adapt.entries)
	w.p.adapt.mu.Unlock()
	if n != 1 {
		t.Errorf("%d entries for one folder", n)
	}
}

// A new snapshot with the same package value (another folder's edit, a document that is only
// opened) keeps the diagnostics: they do not flicker.
func TestAdaptDiagnosticsSurviveANewSnapshot(t *testing.T) {
	w := newAdaptWorld(t, adaptSource, nil)
	pkg := w.pkg(".")
	w.waitReady()
	snapBefore, release := w.s.Snapshot()
	id := snapBefore.ID()
	release()
	w.open("main.gp", adaptSource) // a new snapshot; the text is the disk's
	if again := w.pkg("."); again != pkg {
		t.Skip("the package was checked again for the open document")
	}
	snapAfter, release := w.s.Snapshot()
	defer release()
	if snapAfter.ID() == id {
		t.Fatal("no new snapshot")
	}
	if _, ok := snapAfter.AdaptDiagnostics(pkg); !ok {
		t.Error("a new snapshot lost the diagnostics of the same package value")
	}
}

// A package without a call that an adaptation could refuse, or with an error, is not analyzed.
func TestAdaptNotScheduledWhenThereIsNothingToRefuse(t *testing.T) {
	for name, source := range map[string]string{
		"no adaptable argument": "package main\nimport \"./api.hpp\"\nfunc main() { var c api.Obj; api.take_char(c) }\n",
		"a Go error":            "package main\nimport \"./api.hpp\"\nfunc main() { api.take_char(200); undefined() }\n",
		"no call":               "package main\nimport \"./api.hpp\"\nfunc main() {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			w := newAdaptWorld(t, source, nil)
			w.pkg(".")
			time.Sleep(100 * time.Millisecond)
			if n := w.runs.Load(); n != 0 {
				t.Errorf("%d analyses", n)
			}
		})
	}
}

// A package with no header import is not looked at for calls at all: the hot path of every
// pure-Go package costs only the includes of its files.
func TestAdaptPureGoPackageIsNotScanned(t *testing.T) {
	t.Run("pure Go", func(t *testing.T) {
		before := adaptableCallsScans.Load()
		w := newAdaptWorld(t, "package main\nfunc take(c int8) {}\nfunc main() { take(100) }\n", nil)
		w.pkg(".")
		if n := adaptableCallsScans.Load() - before; n != 0 {
			t.Errorf("%d passes over a package without header imports", n)
		}
	})
	t.Run("header import", func(t *testing.T) {
		before := adaptableCallsScans.Load()
		w := newAdaptWorld(t, adaptSource, nil)
		w.pkg(".")
		if n := adaptableCallsScans.Load() - before; n != 1 {
			t.Errorf("%d passes over a package with a header import, want 1", n)
		}
	})
}

// A change of the headers on disk runs the analysis again, and forgets the answer meanwhile.
func TestAdaptHeadersChangedRunsTheAnalysisAgain(t *testing.T) {
	w := newAdaptWorld(t, adaptSource, nil)
	pkg := w.pkg(".")
	w.waitReady()
	w.p.HeadersChanged()
	w.waitReady()
	if n := w.runs.Load(); n != 2 {
		t.Errorf("%d analyses, want 2", n)
	}
	if _, ok := w.diags(pkg); !ok {
		t.Error("not ready after the second analysis")
	}
}

// An analysis that is still running when the pipeline closes ends: Close does not hang on it.
func TestAdaptCloseStopsARunningAnalysis(t *testing.T) {
	started := make(chan struct{}, 1)
	w := newAdaptWorld(t, adaptSource, func(ctx context.Context, pkg *Package) ([]diag.Diagnostic, bool, error) {
		started <- struct{}{}
		<-ctx.Done()
		return nil, false, ctx.Err()
	})
	w.pkg(".")
	headerSignal(t, started)
	done := make(chan struct{})
	go func() { w.p.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close waits for the analysis")
	}
}
