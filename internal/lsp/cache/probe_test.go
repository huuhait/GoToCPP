package cache

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
)

// The tests of the namespace probe off the hot path (probe.go), with a fake
// compiler and a fake probe that the test holds and lets go.

// A fakeProbe answers the probes: std and the names in namespaces are
// namespaces, every other name is not. A probe waits until the test lets
// it go (release), and records its candidates.
type fakeProbe struct {
	mu         sync.Mutex
	calls      [][]string
	namespaces []string
	incErr     bool // answer with an include error
	started    chan struct{}
	release    chan struct{}
}

func newFakeProbe() *fakeProbe {
	return &fakeProbe{namespaces: []string{"std"}, started: make(chan struct{}, 100), release: make(chan struct{}, 100)}
}

func (f *fakeProbe) probe(ctx context.Context, _ build.CXX, _ string, includes, cands []string) (interop.Result, error) {
	f.started <- struct{}{}
	select {
	case <-f.release:
	case <-ctx.Done():
		return interop.Result{}, ctx.Err()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, slices.Clone(cands))
	r := interop.Result{Kinds: make(map[string]interop.Kind)}
	if f.incErr {
		r.IncludeErrors = []interop.IncludeError{{Index: 0, Include: includes[0], Kind: interop.HeaderNotFound, Msg: "'vectorr' file not found"}}
		return r, nil
	}
	for _, c := range cands {
		r.Kinds[c] = interop.NotNamespace
		if slices.Contains(f.namespaces, c) {
			r.Kinds[c] = interop.Namespace
		}
	}
	return r, nil
}

func (f *fakeProbe) callsNow() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func probeWorld(t *testing.T, f *fakeProbe, src string, onProbe func()) *world {
	cfg := testConfig()
	cfg.CXX = func(context.Context) (build.CXX, error) {
		return build.CXX{Path: "/fake/c++", Family: build.Clang}, nil
	}
	cfg.Probe = f.probe
	cfg.OnProbe = onProbe
	w := newWorld(t, cfg, map[string]string{"gocpp.mod": "module m\n", "main.gp": src})
	t.Cleanup(w.p.Close)
	// The compiler is found first, so that the first check is the one
	// that starts the probe.
	pr := w.p.prober
	pr.mu.Lock()
	pr.lookCXXLocked()
	pr.mu.Unlock()
	pr.wait()
	return w
}

// TestProbeOffHotPath: the check of a package with a header import doesn't
// wait for the probe: it is provisional, with the unknown names opaque and
// no error, until the probe answers; then it is stale, and the next check
// has the probe's errors. Names typed while a probe runs wait for one
// more run, which probes them all.
func TestProbeOffHotPath(t *testing.T) {
	f := newFakeProbe()
	changes := make(chan struct{}, 100)
	src := "package main\n\nimport \"<vector>\"\n\nfunc main() {\n\tvar v std.vector[int]\n\tvar w stdd.vector[int]\n\t_, _ = v, w\n}\n"
	w := probeWorld(t, f, src, func() { changes <- struct{}{} })
	<-changes // the compiler was found

	pkg := w.pkg(".")
	if !pkg.Provisional() || w.errs(pkg) != "" {
		t.Fatalf("first check: provisional %v, errors:\n%s", pkg.Provisional(), w.errs(pkg))
	}
	<-f.started // the probe runs, held
	if pkg.Stale() {
		t.Error("stale before the probe answered")
	}
	// Two more edits with new names while the probe is held: they don't
	// start probes of their own.
	w.open("main.gp", strings.Replace(src, "_, _ = v, w", "_, _ = v, w\n\tvar x foo.T\n\t_ = x", 1))
	w.pkg(".")
	w.edit("main.gp", strings.Replace(src, "_, _ = v, w", "_, _ = v, w\n\tvar x bar.T\n\t_ = x", 1))
	if pkg := w.pkg("."); !pkg.Provisional() {
		t.Error("the check with new names is not provisional")
	}
	f.release <- struct{}{}
	<-changes
	<-f.started // the second run, for the names that came meanwhile
	f.release <- struct{}{}
	<-changes
	w.p.prober.wait()
	calls := f.callsNow()
	if len(calls) != 2 || !slices.Equal(calls[0], []string{"std", "stdd"}) || !slices.Equal(calls[1], []string{"bar", "foo", "std", "stdd"}) {
		t.Errorf("probes %v, want [std stdd] and then [bar foo std stdd]", calls)
	}
	if !pkg.Stale() {
		t.Error("the first check is not stale after the probe answered")
	}
	got := w.pkg(".")
	if got.Provisional() || got.Stale() {
		t.Errorf("after the probes: provisional %v, stale %v", got.Provisional(), got.Stale())
	}
	errs := w.errs(got)
	for _, want := range []string{"undefined: stdd", "undefined: bar"} {
		if !strings.Contains(errs, want) {
			t.Errorf("errors after the probes:\n%s\nwant %q", errs, want)
		}
	}
	if strings.Contains(errs, "undefined: std\n") {
		t.Errorf("std is undefined:\n%s", errs)
	}
	// The answers are kept: a new check of a known set of names is
	// complete at once, and runs no probe.
	w.edit("main.gp", src+"\n")
	if pkg := w.pkg("."); pkg.Provisional() || !strings.Contains(w.errs(pkg), "undefined: stdd") {
		t.Errorf("an edit after the probes: provisional %v, errors:\n%s", pkg.Provisional(), w.errs(pkg))
	}
	if n := len(f.callsNow()); n != 2 {
		t.Errorf("%d probes, want 2", n)
	}
}

// TestProbeHeadersChanged: a changed header makes the prober check its
// answers again; a new answer (here an include error) makes the checks
// that used the old one stale.
func TestProbeHeadersChanged(t *testing.T) {
	f := newFakeProbe()
	changes := make(chan struct{}, 100)
	src := "package main\n\nimport \"<vector>\"\n\nfunc main() { var v std.vector[int]; _ = v }\n"
	w := probeWorld(t, f, src, func() { changes <- struct{}{} })
	<-changes // the compiler was found
	w.pkg(".")
	f.release <- struct{}{}
	<-changes
	w.p.prober.wait()
	pkg := w.pkg(".")
	if pkg.Provisional() || w.errs(pkg) != "" {
		t.Fatalf("after the probe: provisional %v, errors:\n%s", pkg.Provisional(), w.errs(pkg))
	}
	// The same answer again: nothing changes.
	w.p.HeadersChanged()
	f.release <- struct{}{}
	w.p.prober.wait()
	if pkg.Stale() || len(changes) > 0 {
		t.Errorf("the same answer: stale %v, %d changes", pkg.Stale(), len(changes))
	}
	f.mu.Lock()
	f.incErr = true
	f.mu.Unlock()
	w.p.HeadersChanged()
	f.release <- struct{}{}
	<-changes
	w.p.prober.wait()
	if !pkg.Stale() {
		t.Fatal("not stale after the include error")
	}
	if errs := w.errs(w.pkg(".")); !strings.Contains(errs, "cannot find header <vector>") {
		t.Errorf("errors after the header change:\n%s", errs)
	}
}

// TestProbeCXXRetry: an error to find the compiler is kept for cxxRetry,
// then the compiler is looked for again (in the background); when it is
// found, the checks with the error are stale.
func TestProbeCXXRetry(t *testing.T) {
	old := cxxRetry
	cxxRetry = 50 * time.Millisecond
	t.Cleanup(func() { cxxRetry = old })
	var mu sync.Mutex
	lookups := 0
	found := false
	f := newFakeProbe()
	cfg := testConfig()
	cfg.CXX = func(context.Context) (build.CXX, error) {
		mu.Lock()
		defer mu.Unlock()
		lookups++
		if !found {
			return build.CXX{}, errors.New("no C++ compiler found")
		}
		return build.CXX{Path: "/fake/c++", Family: build.Clang}, nil
	}
	cfg.Probe = f.probe
	src := "package main\n\nimport \"<vector>\"\n\nfunc main() { var v std.vector[int]; _ = v }\n"
	w := newWorld(t, cfg, map[string]string{"gocpp.mod": "module m\n", "main.gp": src})
	t.Cleanup(w.p.Close)
	w.pkg(".")
	w.p.prober.wait()
	pkg := w.pkg(".")
	if !strings.Contains(w.errs(pkg), "no C++ compiler found") {
		t.Fatalf("errors:\n%s", w.errs(pkg))
	}
	w.pkg(".")
	if lookups != 1 {
		t.Errorf("%d lookups before cxxRetry, want 1", lookups)
	}
	mu.Lock()
	found = true
	mu.Unlock()
	time.Sleep(cxxRetry)
	w.open("main.gp", src+"\n") // a check after cxxRetry asks again
	w.pkg(".")
	f.release <- struct{}{} // for the probe with the compiler
	w.p.prober.wait()
	if !pkg.Stale() {
		t.Error("the check with the lookup error is not stale")
	}
	w.pkg(".")
	w.p.prober.wait()
	if errs := w.errs(w.pkg(".")); errs != "" {
		t.Errorf("errors with the compiler:\n%s", errs)
	}
	if lookups != 2 {
		t.Errorf("%d lookups, want 2", lookups)
	}
}

// TestProbeTransitive: an open package P imports D (not open), whose check
// waits for a probe. P is provisional through D, and stale once D's probe
// answers; after Session.Recheck of P's folder, P's check is complete, and
// D is checked again exactly once (with P).
func TestProbeTransitive(t *testing.T) {
	f := newFakeProbe()
	changes := make(chan struct{}, 100)
	cfg := testConfig()
	cfg.CXX = func(context.Context) (build.CXX, error) {
		return build.CXX{Path: "/fake/c++", Family: build.Clang}, nil
	}
	cfg.Probe = f.probe
	cfg.OnProbe = func() { changes <- struct{}{} }
	mainSrc := "package main\n\nimport \"m/d\"\n\nfunc main() { _ = d.N() }\n"
	w := newWorld(t, cfg, map[string]string{
		"gocpp.mod": "module m\n",
		"main.gp":   mainSrc,
		"d/d.gp":    "package d\n\nimport \"<vector>\"\n\nfunc N() int { var v stdd.vector[int]; _ = v; return 1 }\n",
	})
	t.Cleanup(w.p.Close)
	pr := w.p.prober
	pr.mu.Lock()
	pr.lookCXXLocked()
	pr.mu.Unlock()
	<-changes // the compiler was found
	w.open("main.gp", mainSrc)
	p := w.pkg(".")
	if !p.Provisional() || p.Stale() {
		t.Fatalf("P: provisional %v, stale %v; want provisional through D", p.Provisional(), p.Stale())
	}
	f.release <- struct{}{}
	<-changes
	pr.wait()
	if !p.Stale() {
		t.Fatal("P is not stale after D's probe answered")
	}
	before := w.checks()
	if err := w.s.Recheck([]string{w.root}); err != nil {
		t.Fatal(err)
	}
	p2 := w.pkg(".")
	if p2.Provisional() || p2.Stale() {
		t.Errorf("P after Recheck: provisional %v, stale %v", p2.Provisional(), p2.Stale())
	}
	if n := w.checks() - before; n != 2 {
		t.Errorf("%d checks after Recheck, want 2 (D once, then P)", n)
	}
	if errs := w.errs(w.pkg("d")); !strings.Contains(errs, "undefined: stdd") {
		t.Errorf("D's errors:\n%s", errs)
	}
	if n := w.checks() - before; n != 2 {
		t.Errorf("%d checks after getting D, want still 2", n)
	}
}
