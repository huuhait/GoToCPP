package cache

import (
	"context"
	"errors"
	"fmt"
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

func (f *fakeProbe) probe(ctx context.Context, _ build.CXX, _ string, _, includes, cands []string) (interop.Result, error) {
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
	// cxxRetry passes: the lookup error is made old, not waited for (a short
	// cxxRetry and a sleep make a second lookup too early on a busy machine).
	w.p.prober.mu.Lock()
	w.p.prober.cxx.at = time.Now().Add(-2 * cxxRetry)
	w.p.prober.mu.Unlock()
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

// TestProberFlags: the package's flags are part of an include set (another
// flag is another set, and its probe gets the flags), also for a record
// from disk.
func TestProberFlags(t *testing.T) {
	var mu sync.Mutex
	var flagsSeen [][]string
	probe := func(_ context.Context, _ build.CXX, _ string, flags, _, cands []string) (interop.Result, error) {
		mu.Lock()
		flagsSeen = append(flagsSeen, slices.Clone(flags))
		mu.Unlock()
		r := interop.Result{Kinds: make(map[string]interop.Kind)}
		for _, c := range cands {
			r.Kinds[c] = interop.NotNamespace
			if slices.Contains(flags, "-DNS") {
				r.Kinds[c] = interop.Namespace
			}
		}
		return r, nil
	}
	cxx := build.CXX{Path: "/fake/c++", Family: build.Clang, Version: "clang version 99"}
	find := func(context.Context) (build.CXX, error) { return cxx, nil }
	pr := newProber(find, probe, nil)
	defer pr.close()
	ctx := context.Background()
	r := &probeRecord{pr: pr}
	pr.findCXX(r)(ctx)
	pr.wait()
	if got, err := pr.findCXX(r)(ctx); err != nil || got.Path != cxx.Path {
		t.Fatalf("compiler %v, %v", got, err)
	}
	ask := func(flags []string, want interop.Kind) {
		t.Helper()
		res, err := pr.probeFor(r)(ctx, cxx, "/d", flags, []string{"<x.h>"}, []string{"ns"})
		if err != nil || res.Kinds["ns"] != want {
			t.Errorf("flags %q: %v, %v; want %v", flags, res.Kinds["ns"], err, want)
		}
	}
	ask([]string{"-DNS"}, interop.Unknown) // no answer yet
	pr.wait()
	ask([]string{"-DNS"}, interop.Namespace)
	ask(nil, interop.Unknown) // another include set
	pr.wait()
	ask(nil, interop.NotNamespace)
	ask([]string{"-DNS"}, interop.Namespace)
	pr.mu.Lock()
	nSets := len(pr.sets)
	pr.mu.Unlock()
	mu.Lock()
	seen := slices.Clone(flagsSeen)
	mu.Unlock()
	if nSets != 2 || len(seen) != 2 || !slices.Equal(seen[0], []string{"-DNS"}) || seen[1] != nil {
		t.Errorf("%d include sets, probes with the flags %q; want 2 sets, [-DNS] then none", nSets, seen)
	}

	// On disk, a use keeps its flags, and a replay probes with them. (A
	// record with an answer that was missing is not written: a new one.)
	r = &probeRecord{pr: pr}
	pr.findCXX(r)(ctx)
	ask([]string{"-DNS"}, interop.Namespace)
	ask(nil, interop.NotNamespace)
	d, ok := r.diskForm()
	if !ok || len(d.Uses) != 2 || !slices.Equal(d.Uses[0].Flags, []string{"-DNS"}) || d.Uses[1].Flags != nil {
		t.Fatalf("diskForm = %+v, %v", d, ok)
	}
	pr2 := newProber(find, probe, nil)
	defer pr2.close()
	r2 := pr2.replay(d)
	pr2.wait()
	if r2.stale() || r2.provisional() {
		t.Errorf("replayed record: stale %v, provisional %v; want neither", r2.stale(), r2.provisional())
	}
	mu.Lock()
	seen = slices.Clone(flagsSeen[2:])
	mu.Unlock()
	slices.SortFunc(seen, func(a, b []string) int { return len(a) - len(b) })
	if len(seen) != 2 || seen[0] != nil || !slices.Equal(seen[1], []string{"-DNS"}) {
		t.Errorf("the replay probed with the flags %q, want none and [-DNS]", seen)
	}
}

// A fakePkgConfig answers pkg-config runs with cflags, or with err. A run
// waits until the test lets it go (release).
type fakePkgConfig struct {
	mu      sync.Mutex
	runs    int
	cflags  []string
	err     error
	started chan struct{}
	release chan struct{}
}

func newFakePkgConfig(cflags ...string) *fakePkgConfig {
	return &fakePkgConfig{cflags: cflags, started: make(chan struct{}, 100), release: make(chan struct{}, 100)}
}

func (f *fakePkgConfig) run(ctx context.Context, _ string, _ []string) ([]string, []string, error) {
	f.started <- struct{}{}
	select {
	case <-f.release:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs++
	return slices.Clone(f.cflags), []string{"-llib"}, f.err
}

// TestProbePkgConfig: the check of a package with //gocpp:pkg-config
// doesn't wait for pkg-config: it is provisional, and its package is not
// probed, until pkg-config answers in the background; then the check is
// stale, and the next one probes with the flags. An old answer is checked
// again in the background when a check uses it; an error then makes the
// checks stale, and the next one has the error at the directive.
func TestProbePkgConfig(t *testing.T) {
	f := newFakeProbe()
	pc := newFakePkgConfig("-DLIB")
	changes := make(chan struct{}, 100)
	src := "//gocpp:pkg-config lib\npackage main\n\nimport \"<vector>\"\n\nfunc main() {\n\tvar v std.vector[int]\n\tvar w stdd.vector[int]\n\t_, _ = v, w\n}\n"
	cfg := testConfig()
	cfg.CXX = func(context.Context) (build.CXX, error) {
		return build.CXX{Path: "/fake/c++", Family: build.Clang}, nil
	}
	cfg.Probe = f.probe
	cfg.PkgConfig = pc.run
	cfg.OnProbe = func() { changes <- struct{}{} }
	w := newWorld(t, cfg, map[string]string{"gocpp.mod": "module m\n", "main.gp": src})
	t.Cleanup(w.p.Close)
	pr := w.p.prober
	pr.mu.Lock()
	pr.lookCXXLocked()
	pr.mu.Unlock()
	<-changes // the compiler was found
	pr.wait()

	pkg := w.pkg(".")
	if !pkg.Provisional() || w.errs(pkg) != "" {
		t.Fatalf("first check: provisional %v, errors:\n%s", pkg.Provisional(), w.errs(pkg))
	}
	<-pc.started // pkg-config runs, held
	if len(f.started) != 0 {
		t.Error("the package was probed without its flags")
	}
	pc.release <- struct{}{}
	<-changes
	pr.wait()
	if !pkg.Stale() {
		t.Fatal("the first check is not stale after pkg-config answered")
	}
	w.pkg(".") // probes with the flags, in the background
	f.release <- struct{}{}
	pr.wait()
	pkg = w.pkg(".")
	if pkg.Provisional() || !strings.Contains(w.errs(pkg), "undefined: stdd") {
		t.Errorf("after pkg-config and the probe: provisional %v, errors:\n%s", pkg.Provisional(), w.errs(pkg))
	}
	pr.mu.Lock()
	var flags [][]string
	for _, set := range pr.sets {
		flags = append(flags, set.flags)
	}
	pr.mu.Unlock()
	if want := slices.Concat([]string{"-DLIB"}, build.LanguageFlags()); len(flags) != 1 || !slices.Equal(flags[0], want) {
		t.Errorf("the probes had the flags %q, want %q", flags, want)
	}

	// The answer gets old, and pkg-config now fails: a check that uses the
	// old answer runs pkg-config again in the background.
	pc.mu.Lock()
	pc.err = errors.New("pkg-config --cflags -- lib: exit status 1")
	pc.mu.Unlock()
	pr.mu.Lock()
	for _, e := range pr.pcs {
		e.validated = time.Time{}
	}
	pr.mu.Unlock()
	w.open("main.gp", src+"\n")
	old := w.pkg(".")
	<-pc.started
	pc.release <- struct{}{}
	<-changes
	pr.wait()
	if !old.Stale() {
		t.Fatal("the check with the old answer is not stale")
	}
	if errs := w.errs(w.pkg(".")); !strings.Contains(errs, "main.gp:1:1: pkg-config --cflags -- lib: exit status 1") {
		t.Errorf("errors after pkg-config failed:\n%s", errs)
	}
	if pc.runs != 2 {
		t.Errorf("%d pkg-config runs, want 2", pc.runs)
	}
}

// TestProberPkgConfigDisk: a record keeps its pkg-config answers on disk,
// and a replay runs pkg-config again: provisional until it answers, then
// stale if the answer differs.
func TestProberPkgConfigDisk(t *testing.T) {
	pc := newFakePkgConfig("-DA")
	find := func(context.Context) (build.CXX, error) {
		return build.CXX{Path: "/fake/c++", Family: build.Clang}, nil
	}
	probe := newFakeProbe().probe
	pr := newProber(find, probe, nil)
	pr.pc = pc.run
	defer pr.close()
	ctx := context.Background()
	r := &probeRecord{pr: pr}
	if _, _, err := pr.pkgConfigFor(r)(ctx, "/d", []string{"lib"}); err != build.ErrPending {
		t.Fatalf("the first answer: %v, want build.ErrPending", err)
	}
	if _, ok := r.diskForm(); ok {
		t.Error("a record with a pending answer can be written")
	}
	pc.release <- struct{}{}
	pr.wait()
	r = &probeRecord{pr: pr}
	cflags, libs, err := pr.pkgConfigFor(r)(ctx, "/d", []string{"lib"})
	if err != nil || !slices.Equal(cflags, []string{"-DA"}) || !slices.Equal(libs, []string{"-llib"}) || r.provisional() || r.stale() {
		t.Fatalf("the answer: %q, %q, %v; provisional %v, stale %v", cflags, libs, err, r.provisional(), r.stale())
	}
	d, ok := r.diskForm()
	if !ok || len(d.PkgConfigs) != 1 || d.PkgConfigs[0].Dir != "/d" || !slices.Equal(d.PkgConfigs[0].Pkgs, []string{"lib"}) {
		t.Fatalf("diskForm = %+v, %v", d, ok)
	}

	for _, c := range []struct {
		cflags []string
		stale  bool
	}{{[]string{"-DA"}, false}, {[]string{"-DB"}, true}} {
		pc.mu.Lock()
		pc.cflags = c.cflags
		pc.mu.Unlock()
		pr2 := newProber(find, probe, nil)
		pr2.pc = pc.run
		r2 := pr2.replay(d)
		<-pc.started
		if !r2.provisional() {
			t.Errorf("%q: the replayed record is not provisional while pkg-config runs", c.cflags)
		}
		pc.release <- struct{}{}
		pr2.wait()
		if r2.provisional() || r2.stale() != c.stale {
			t.Errorf("%q: the replayed record: provisional %v, stale %v; want stale %v", c.cflags, r2.provisional(), r2.stale(), c.stale)
		}
		pr2.close()
	}
}

// TestProberPkgConfigLimit: the pkg-config runs share the places of the
// probes, so at most maxProbes run at the same time.
func TestProberPkgConfigLimit(t *testing.T) {
	pc := newFakePkgConfig("-DA")
	find := func(context.Context) (build.CXX, error) {
		return build.CXX{Path: "/fake/c++", Family: build.Clang}, nil
	}
	pr := newProber(find, newFakeProbe().probe, nil)
	pr.pc = pc.run
	defer pr.close()
	r := &probeRecord{pr: pr}
	const n = maxProbes + 3
	for i := range n {
		pr.pkgConfigFor(r)(context.Background(), "/d", []string{fmt.Sprint("lib", i)})
	}
	for range maxProbes {
		<-pc.started
	}
	time.Sleep(50 * time.Millisecond) // a run that shouldn't start would by now
	if extra := len(pc.started); extra != 0 {
		t.Errorf("%d pkg-config runs at the same time, want at most %d", maxProbes+extra, maxProbes)
	}
	for range n {
		pc.release <- struct{}{}
	}
	pr.wait()
	if pc.runs != n {
		t.Errorf("%d runs, want %d", pc.runs, n)
	}
}
