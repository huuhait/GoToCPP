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

// The tests of the C header check off the hot path (step 4.14).

// A fakeCHeader answers the C header checks with res, or with err. A check
// waits until the test lets it go (release).
type fakeCHeader struct {
	mu      sync.Mutex
	runs    int
	res     interop.CHeader
	err     error
	started chan struct{}
	release chan struct{}
}

func newFakeCHeader(res interop.CHeader) *fakeCHeader {
	return &fakeCHeader{res: res, started: make(chan struct{}, 100), release: make(chan struct{}, 100)}
}

func (f *fakeCHeader) check(ctx context.Context, _ build.CXX, _ string, _ []string, _ string) (interop.CHeader, error) {
	f.started <- struct{}{}
	select {
	case <-f.release:
	case <-ctx.Done():
		return interop.CHeader{}, ctx.Err()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs++
	return f.res, f.err
}

// includeProbe is a probe that records the includes it gets and answers
// that nothing is a namespace.
type includeProbe struct {
	mu   sync.Mutex
	sets [][]string
}

func (p *includeProbe) probe(_ context.Context, _ build.CXX, _ string, _, includes, cands []string) (interop.Result, error) {
	p.mu.Lock()
	p.sets = append(p.sets, slices.Clone(includes))
	p.mu.Unlock()
	return unknownResult(cands), nil
}

func (p *includeProbe) asked(includes ...string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.ContainsFunc(p.sets, func(s []string) bool { return slices.Equal(s, includes) })
}

// chAnswersChanged reports whether what the prober knows of the local headers
// that the check of pkg asked about is not what the check was given.
func chAnswersChanged(pkg *Package) bool {
	for _, r := range pkg.summary.records {
		r.mu.Lock()
		uses := slices.Clone(r.chUses)
		r.mu.Unlock()
		r.pr.mu.Lock()
		for _, u := range uses {
			if r.pr.chAnswerLocked(u.entry) != u.digest {
				r.pr.mu.Unlock()
				return true
			}
		}
		r.pr.mu.Unlock()
	}
	return false
}

// TestCHeaderOffHotPath: the check of a package with a local .h header
// doesn't wait for the C header check: it gets "not wrapped" at once and
// is provisional; when the check answers in the background, it is stale,
// and the next check has the header wrapped, and the probe gets it so. An
// old answer is checked again in the background; an error then makes the
// checks stale, and the next one reports it at the import.
func TestCHeaderOffHotPath(t *testing.T) {
	ch := newFakeCHeader(interop.CHeader{C: true})
	probe := new(includeProbe)
	changes := make(chan struct{}, 100)
	src := "package main\n\nimport \"./c.h\"\n\nfunc main() {}\n"
	cfg := testConfig()
	cfg.CXX = func(context.Context) (build.CXX, error) {
		return build.CXX{Path: "/fake/c++", Family: build.Clang}, nil
	}
	cfg.Probe = probe.probe
	cfg.CHeader = ch.check
	cfg.OnProbe = func() { changes <- struct{}{} }
	w := newWorld(t, cfg, map[string]string{"gocpp.mod": "module m\n", "main.gp": src, "c.h": "int c(void);\n"})
	t.Cleanup(w.p.Close)
	pr := w.p.prober
	pr.mu.Lock()
	pr.lookCXXLocked()
	pr.mu.Unlock()
	<-changes // the compiler was found
	pr.wait()

	// The first check does not wait for the C header check: that check is held by ch until
	// the test lets it go below. It runs in a goroutine, so a check that waited for the held
	// one fails with a message after the hang limit (a hang guard, not a speed bound).
	checked := make(chan *Package, 1)
	go func() { checked <- w.pkg(".") }()
	var pkg *Package
	select {
	case pkg = <-checked:
	case <-time.After(hangLimit):
		t.Fatal("the first check waited for the held C header check")
	}
	if !pkg.Provisional() || w.errs(pkg) != "" || pkg.Check.ExternC[`"./c.h"`] {
		t.Fatalf("first check: provisional %v, ExternC %v, errors:\n%s", pkg.Provisional(), pkg.Check.ExternC, w.errs(pkg))
	}
	<-ch.started // the check runs, held
	// (The check as a whole may be stale already: the probe of its include
	// set answers in the background, at any time. What must not have changed is
	// the answer of the C header check, which is held.)
	if chAnswersChanged(pkg) {
		t.Error("the answer of the C header check changed before it answered")
	}
	ch.release <- struct{}{}
	<-changes
	pr.wait()
	if !pkg.Stale() {
		t.Fatal("the first check is not stale after the C header check answered")
	}
	w.pkg(".") // the probe of the wrapped include, in the background
	pr.wait()
	pkg = w.pkg(".")
	if pkg.Provisional() || pkg.Stale() || !pkg.Check.ExternC[`"./c.h"`] || w.errs(pkg) != "" {
		t.Errorf("after the answers: provisional %v, stale %v, ExternC %v, errors:\n%s", pkg.Provisional(), pkg.Stale(), pkg.Check.ExternC, w.errs(pkg))
	}
	if !probe.asked(`"./c.h"`) || !probe.asked(interop.ExternC(`"./c.h"`)) {
		t.Errorf("probes of %q", probe.sets)
	}

	// The answer gets old, and the check now fails.
	ch.mu.Lock()
	ch.err = errors.New("no C compiler")
	ch.mu.Unlock()
	pr.mu.Lock()
	for _, e := range pr.chs {
		e.validated = time.Time{}
	}
	pr.mu.Unlock()
	w.open("main.gp", src+"\n")
	old := w.pkg(".")
	<-ch.started
	ch.release <- struct{}{}
	<-changes
	pr.wait()
	if !old.Stale() {
		t.Fatal("the check with the old answer is not stale")
	}
	if errs := w.errs(w.pkg(".")); !strings.Contains(errs, `main.gp:3:8: cannot check whether header "./c.h" is a C header: no C compiler`) {
		t.Errorf("errors after the C header check failed:\n%s", errs)
	}
	if ch.runs != 2 {
		t.Errorf("%d C header checks, want 2", ch.runs)
	}
}

// TestProberCHeaderDisk: a record keeps its C header answers on disk, and
// a replay runs the check again: provisional until it answers, then stale
// if the answer differs (as the check uses it: wrapped or not).
func TestProberCHeaderDisk(t *testing.T) {
	ch := newFakeCHeader(interop.CHeader{C: true})
	find := func(context.Context) (build.CXX, error) {
		return build.CXX{Path: "/fake/c++", Family: build.Clang}, nil
	}
	probe := newFakeProbe().probe
	pr := newProber(find, probe, nil)
	pr.cheader = ch.check
	defer pr.close()
	ctx := context.Background()
	cxx, _ := find(ctx)
	r := &probeRecord{pr: pr}
	if res, err := pr.cheaderFor(r)(ctx, cxx, "/d", []string{"-DX"}, `"./c.h"`); err != nil || res != (interop.CHeader{}) || !r.provisional() {
		t.Fatalf("the first answer: %+v, %v; provisional %v", res, err, r.provisional())
	}
	if _, ok := r.diskForm(); ok {
		t.Error("a record with a pending answer can be written")
	}
	ch.release <- struct{}{}
	pr.wait()
	r = &probeRecord{pr: pr}
	if res, err := pr.cheaderFor(r)(ctx, cxx, "/d", []string{"-DX"}, `"./c.h"`); err != nil || !res.C || r.provisional() || r.stale() {
		t.Fatalf("the answer: %+v, %v; provisional %v, stale %v", res, err, r.provisional(), r.stale())
	}
	d, ok := r.diskForm()
	if !ok || len(d.CHeaders) != 1 || d.CHeaders[0].Dir != "/d" || d.CHeaders[0].Include != `"./c.h"` || !slices.Equal(d.CHeaders[0].Flags, []string{"-DX"}) {
		t.Fatalf("diskForm = %+v, %v", d, ok)
	}
	d.CXX = cxx // the compiler that a check records (this record has no check)

	for _, c := range []struct {
		res   interop.CHeader
		stale bool
	}{
		{interop.CHeader{C: true}, false},
		{interop.CHeader{C: true, Guard: true}, true}, // not wrapped any more
		{interop.CHeader{}, true},
	} {
		ch.mu.Lock()
		ch.res = c.res
		ch.mu.Unlock()
		pr2 := newProber(find, probe, nil)
		pr2.cheader = ch.check
		r2 := pr2.replay(d)
		<-ch.started
		if !r2.provisional() {
			t.Errorf("%+v: the replayed record is not provisional while the check runs", c.res)
		}
		ch.release <- struct{}{}
		pr2.wait()
		if r2.provisional() || r2.stale() != c.stale {
			t.Errorf("%+v: the replayed record: provisional %v, stale %v; want stale %v", c.res, r2.provisional(), r2.stale(), c.stale)
		}
		pr2.close()
	}
}
