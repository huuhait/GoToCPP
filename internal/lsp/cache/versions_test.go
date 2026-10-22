package cache

import (
	"context"
	"errors"
	"fmt"
	"go/types"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestVersionsKept: the cache of checks keeps the last keptVersions checks
// of a package, however often it is edited; an edit and its undo (two
// texts that alternate) find their checks in it.
func TestVersionsKept(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	src := app["shapes/shapes.gp"]
	w.pkg("shapes")
	w.open("shapes/shapes.gp", src)
	entries := func() int { n, _ := w.p.checks.Stats(); return n }
	base := entries()
	for i := range 12 {
		w.edit("shapes/shapes.gp", src+fmt.Sprintf("\nvar V%d = %d\n", i, i))
		w.pkg("shapes")
	}
	if got := entries(); got > base+keptVersions {
		t.Errorf("%d checks in the cache after 12 edits of one package (%d before), want at most %d more", got, base, keptVersions)
	}
	// Two texts that alternate: after the first round, no check is made.
	a, b := src+"\nvar A = 1\n", src+"\nvar B = 1\n"
	for _, text := range []string{a, b, a, b} {
		w.edit("shapes/shapes.gp", text)
		w.pkg("shapes")
	}
	before := w.checks()
	for _, text := range []string{a, b, a, b, a} {
		w.edit("shapes/shapes.gp", text)
		w.pkg("shapes")
	}
	if n := w.checks() - before; n != 0 {
		t.Errorf("%d checks for two texts that alternate, want 0", n)
	}
}

// Imported API changes invalidate the decoded universe even when the importer
// source is unchanged. A revert restores answers; a new importer text then
// exercises decoding instead of merely hitting a previous package check.
func TestImportSetAPIChangeAndRevert(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	base := app["geom/geom.gp"]
	w.open("geom/geom.gp", base)
	first := w.pkg("geom")
	if errs := w.errs(first); errs != "" {
		t.Fatal(errs)
	}
	id := pkgID{dir: filepath.Join(w.root, "geom")}
	set := w.p.importSets.sets[id]
	if set == nil {
		t.Fatal("no decoded imports")
	}
	w.open("shapes/shapes.gp", "package shapes\n\ntype Square struct{ Edge int }\n")
	changed := w.pkg("geom")
	if errs := w.errs(changed); !strings.Contains(errs, "Side") {
		t.Fatalf("unchanged importer missed imported API change: %s", errs)
	}
	newSet := w.p.importSets.sets[id]
	if newSet == set {
		t.Fatal("reused import set after imported API change")
	}
	w.edit("shapes/shapes.gp", app["shapes/shapes.gp"])
	if errs := w.errs(w.pkg("geom")); errs != "" {
		t.Fatalf("revert retained stale error: %s", errs)
	}
	w.edit("geom/geom.gp", base+"\n// unique text after the revert\n")
	if errs := w.errs(w.pkg("geom")); errs != "" {
		t.Fatalf("new check after revert used stale imports: %s", errs)
	}
	if w.p.importSets.sets[id] == newSet {
		t.Fatal("reused changed imports after revert")
	}
	// An old snapshot's imported objects must not have been mutated.
	if errs := w.errs(first); errs != "" {
		t.Fatalf("old package changed: %s", errs)
	}
}

func TestImportSetDependencyIdentity(t *testing.T) {
	var sets importSets
	id := pkgID{dir: "/p"}
	a, b := new(summary), new(summary)
	set := &importSet{deps: []*summary{a, nil}}
	sets.put(id, set)
	if sets.get(id, []*summary{a, nil}) != set {
		t.Fatal("missed identical summaries")
	}
	for _, deps := range [][]*summary{{b, nil}, {a}, {nil, a}, {a, b}} {
		if sets.get(id, deps) != nil {
			t.Fatal("reused a different dependency universe")
		}
	}
	if sets.get(pkgID{dir: "/q"}, []*summary{a, nil}) != nil {
		t.Fatal("shared a universe across packages")
	}
}

func TestVersionKeeperNote(t *testing.T) {
	p := NewPipeline(testConfig())
	var v versionKeeper
	id := pkgID{dir: "/x"}
	for i := range 10 {
		v.note(p, id, Hash{byte(i)})
	}
	if got := len(v.keys[id]); got != keptVersions {
		t.Errorf("%d keys kept, want %d", got, keptVersions)
	}
	// A key seen again moves to the end and does not push another out.
	last := append([]Hash(nil), v.keys[id]...)
	v.note(p, id, last[0])
	if v.keys[id][keptVersions-1] != last[0] || len(v.keys[id]) != keptVersions {
		t.Errorf("keys %v after seeing %v again", v.keys[id], last[0])
	}
}

// TestImportSetPositions: the second check of a package takes the decoded
// imports of the first (importset.go), and the positions of their objects
// still read, in the new file set of the check.
func TestImportSetPositions(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	src := app["geom/geom.gp"]
	w.open("geom/geom.gp", src)
	first := w.pkg("geom")
	w.edit("geom/geom.gp", src+"\nvar Extra = 1\n")
	second := w.pkg("geom")
	if first == second {
		t.Fatal("the edit made no new check")
	}
	if first.Check.Types.Imports()[0] != second.Check.Types.Imports()[0] {
		t.Fatal("the body-only edit did not reuse decoded imports")
	}
	if len(w.p.importSets.sets) == 0 {
		t.Fatal("no import set kept")
	}
	for i, pkg := range []*Package{first, second} {
		found := 0
		for _, imp := range pkg.Check.Types.Imports() {
			for _, name := range imp.Scope().Names() {
				pos := pkg.Fset().Position(imp.Scope().Lookup(name).Pos())
				// (Without the files of the decode, the positions of the imports fall into the
				// first file of the package: a name, a line, the wrong file.)
				if pos.Filename == "" || pos.Line == 0 || strings.HasSuffix(imp.Path(), "shapes") && !strings.Contains(pos.Filename, "shapes") {
					t.Errorf("check %d: %s.%s is at %v", i, imp.Path(), name, pos)
				}
				found++
			}
		}
		if found == 0 {
			t.Errorf("check %d: no imported object", i)
		}
	}
}

func variantWorld(t *testing.T) *world {
	t.Helper()
	w := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a/a.gp":    "package a\nvar Result = 0\n",
	})
	t.Cleanup(func() {
		w.s.Shutdown()
		w.p.Close()
	})
	return w
}

func variantSource(i int) []byte {
	return []byte(fmt.Sprintf("package a\nvar Result = %d\n", i))
}

func checkedVariant(t *testing.T, w *world, snap *Snapshot, text []byte) *Package {
	t.Helper()
	pkg, err := snap.PackageWithFile(context.Background(), filepath.Join(w.root, "a", "a.gp"), text)
	if err != nil {
		t.Fatal(err)
	}
	if pkg == nil || w.errs(pkg) != "" {
		t.Fatalf("variant did not check: %+v", pkg)
	}
	if !pkg.IsVariant() {
		t.Fatal("computed or cached patched package lost its input-key provenance")
	}
	return pkg
}

func TestVariantVersionsBounded(t *testing.T) {
	w := variantWorld(t)
	w.pkg("a")
	snap, free := w.s.Snapshot()
	defer free()
	base, _ := w.p.checks.Stats()
	for i := range 40 {
		checkedVariant(t, w, snap, variantSource(i))
		entries, _ := w.p.checks.Stats()
		if entries > base+keptVersions {
			t.Fatalf("%d cached checks after variant %d, want at most %d", entries, i, base+keptVersions)
		}
	}
	id := pkgID{dir: filepath.Join(w.root, "a")}
	if len(w.p.variantVersions.keys[id]) != keptVersions {
		t.Fatalf("variant keys: %v", w.p.variantVersions.keys[id])
	}
	before := w.checks()
	for _, i := range []int{38, 39, 38, 39} {
		checkedVariant(t, w, snap, variantSource(i))
	}
	if got := w.checks() - before; got != 0 {
		t.Errorf("recent variant reverts made %d new checks", got)
	}
}

func TestVariantEvictionKeepsActiveUser(t *testing.T) {
	w := variantWorld(t)
	snap, free := w.s.Snapshot()
	defer free()
	first := checkedVariant(t, w, snap, variantSource(100))
	id := pkgID{dir: filepath.Join(w.root, "a")}
	firstKey := w.p.variantVersions.keys[id][0]
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	read := make(chan struct{})
	results := make(chan error)
	go func() {
		for range 12 {
			select {
			case <-read:
			case <-ctx.Done():
				return
			}
			var err error
			obj := first.Check.Types.Scope().Lookup("Result")
			if obj == nil || obj.Type().String() != "int" || string(first.Program.Roots[0].Files[0].Src) != string(variantSource(100)) {
				err = errors.New("eviction changed a package held by an active user")
			}
			select {
			case results <- err:
			case <-ctx.Done():
				return
			}
		}
	}()
	for i := range 12 {
		select {
		case read <- struct{}{}:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		checkedVariant(t, w, snap, variantSource(i))
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	if _, _, kept := w.p.checks.Peek(firstKey); kept {
		t.Fatal("the old variant still has cache ownership")
	}
	if obj := first.Check.Types.Scope().Lookup("Result"); obj == nil || obj.Type() != types.Typ[types.Int] {
		t.Fatal("the caller lost its old package after cache eviction")
	}
}

func TestVariantOrdinaryIsolation(t *testing.T) {
	w := variantWorld(t)
	normal := w.pkg("a")
	if normal.IsVariant() {
		t.Fatal("ordinary package was marked as patched input")
	}
	id := pkgID{dir: filepath.Join(w.root, "a")}
	normalKey := w.p.versions.keys[id][0]
	snap, free := w.s.Snapshot()
	defer free()
	identical := checkedVariant(t, w, snap, variantSource(0))
	if identical == normal || w.p.variantVersions.keys[id][0] == normalKey {
		t.Fatal("a patched-file check aliased the ordinary check with identical bytes")
	}
	for i := range 10 {
		checkedVariant(t, w, snap, variantSource(i+10))
	}
	if kept, _, ok := w.p.checks.Peek(normalKey); !ok || kept != normal {
		t.Fatal("variants evicted the ordinary edit/undo cohort")
	}
	variantKey := w.p.variantVersions.keys[id][keptVersions-1]
	w.open("a/a.gp", "package a\nvar Result = 0\n")
	for i := range 10 {
		w.edit("a/a.gp", string(variantSource(i+30)))
		w.pkg("a")
	}
	if _, _, ok := w.p.checks.Peek(variantKey); !ok {
		t.Fatal("ordinary edits evicted the separate variant cohort")
	}
}

func TestVariantFullOriginalInputKey(t *testing.T) {
	w := variantWorld(t)
	w.open("a/a.gp", "package a\nvar Result = 0\n")
	firstSnap, free := w.s.Snapshot()
	defer free()
	first := checkedVariant(t, w, firstSnap, variantSource(99))
	id := pkgID{dir: filepath.Join(w.root, "a")}
	firstKey := w.p.variantVersions.keys[id][0]
	w.edit("a/a.gp", "package a\nvar Result = 1\n")
	secondSnap, freeSecond := w.s.Snapshot()
	defer freeSecond()
	second := checkedVariant(t, w, secondSnap, variantSource(99))
	keys := w.p.variantVersions.keys[id]
	if first == second || keys[len(keys)-1] == firstKey {
		t.Fatal("identical replacement spelling erased the original input identity")
	}
	if got := checkedVariant(t, w, firstSnap, variantSource(99)); got != first {
		t.Fatal("a live old snapshot no longer resolves its own full-key variant")
	}
}

func TestVariantAPIChangeAndRevert(t *testing.T) {
	for _, mode := range []string{"sibling", "import"} {
		t.Run(mode, func(t *testing.T) {
			dep := "a/provider.gp"
			provider := "package a\nvar Provider = 1\n"
			target := "package a\nvar Result = Provider\n"
			if mode == "import" {
				dep = "dep/provider.gp"
				provider = "package dep\nvar Provider = 1\n"
				target = "package a\nimport \"example.com/m/dep\"\nvar Result = dep.Provider\n"
			}
			w := newWorld(t, testConfig(), map[string]string{"gocpp.mod": "module example.com/m\n", "a/a.gp": target, dep: provider})
			t.Cleanup(func() {
				w.s.Shutdown()
				w.p.Close()
			})
			w.open(dep, provider)
			var first *Package
			for i, want := range []string{"int", "string", "int"} {
				if i > 0 {
					text := provider
					if i == 1 {
						text = strings.Replace(text, "= 1", "= \"one\"", 1)
					}
					w.edit(dep, text)
				}
				snap, free := w.s.Snapshot()
				pkg := checkedVariant(t, w, snap, []byte(target+"// replacement is unchanged\n"))
				free()
				obj := pkg.Check.Types.Scope().Lookup("Result")
				if obj == nil || obj.Type().String() != want {
					t.Fatalf("API %s: Result=%v, want %s", mode, obj, want)
				}
				if i == 0 {
					first = pkg
				}
				if i == 1 && pkg == first {
					t.Fatal("changed API reused the old variant")
				}
				if first.Check.Types.Scope().Lookup("Result").Type().String() != "int" {
					t.Fatal("new API mutated a package held by an old user")
				}
			}
		})
	}
}

func TestVariantConcurrentSnapshotsKeepTheirAPI(t *testing.T) {
	target := "package a\nimport \"example.com/m/dep\"\nvar Result = dep.Provider\n"
	provider := "package dep\nvar Provider = 1\n"
	w := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod":       "module example.com/m\n",
		"a/a.gp":          target,
		"dep/provider.gp": provider,
	})
	t.Cleanup(func() {
		w.s.Shutdown()
		w.p.Close()
	})
	w.open("dep/provider.gp", provider)
	old, freeOld := w.s.Snapshot()
	defer freeOld()
	w.edit("dep/provider.gp", "package dep\nvar Provider = \"one\"\n")
	changed, freeChanged := w.s.Snapshot()
	defer freeChanged()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	type result struct {
		pkg  *Package
		want string
		err  error
	}
	results := make(chan result, 16)
	start := make(chan struct{})
	for i := range 16 {
		go func(i int) {
			<-start
			snapshot := old
			want := "int"
			if i%2 != 0 {
				snapshot = changed
				want = "string"
			}
			pkg, err := snapshot.PackageWithFile(ctx, filepath.Join(w.root, "a", "a.gp"), []byte(target+"// identical patched bytes\n"))
			results <- result{pkg, want, err}
		}(i)
	}
	close(start)
	var first *Package
	for range 16 {
		select {
		case got := <-results:
			if got.err != nil || got.pkg == nil {
				t.Fatalf("concurrent API check: %+v", got)
			}
			obj := got.pkg.Check.Types.Scope().Lookup("Result")
			if obj == nil || obj.Type().String() != got.want {
				t.Fatalf("concurrent Result=%v, want %s", obj, got.want)
			}
			if got.want == "int" {
				first = got.pkg
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	w.edit("dep/provider.gp", provider)
	reverted, freeReverted := w.s.Snapshot()
	defer freeReverted()
	got := checkedVariant(t, w, reverted, []byte(target+"// identical patched bytes\n"))
	if got != first || first.Check.Types.Scope().Lookup("Result").Type().String() != "int" {
		t.Fatal("concurrent changed-API checks poisoned the old snapshot or its revert")
	}
}

func TestVariantGlobalCostCapUnchanged(t *testing.T) {
	cfg := testConfig()
	cfg.MaxCheckBytes = 1
	w := newWorld(t, cfg, map[string]string{"gocpp.mod": "module example.com/m\n", "a/a.gp": "package a\nvar Result = 0\n"})
	t.Cleanup(func() {
		w.s.Shutdown()
		w.p.Close()
	})
	snap, free := w.s.Snapshot()
	defer free()
	for i := range 10 {
		checkedVariant(t, w, snap, variantSource(i))
		if entries, _ := w.p.checks.Stats(); entries != 1 {
			t.Fatalf("%d entries under the one-value global cap", entries)
		}
	}
	if w.p.checks.MaxCost() != 1 {
		t.Fatal("variant retention changed the configured cost cap")
	}
}

func TestVariantConcurrentCheckAndCancellation(t *testing.T) {
	w := variantWorld(t)
	w.pkg("a")
	snap, free := w.s.Snapshot()
	defer free()
	for range cap(w.p.sem) {
		w.p.sem <- struct{}{}
	}
	released := false
	release := func() {
		if !released {
			for range cap(w.p.sem) {
				<-w.p.sem
			}
			released = true
		}
	}
	defer release()
	ctx, cancelAll := context.WithTimeout(context.Background(), time.Minute)
	defer cancelAll()
	canceled, cancelOne := context.WithCancel(ctx)
	cancelOne()
	if _, err := snap.PackageWithFile(canceled, filepath.Join(w.root, "a", "a.gp"), variantSource(77)); !errors.Is(err, context.Canceled) {
		t.Fatalf("already canceled variant: %v", err)
	}
	type result struct {
		pkg *Package
		err error
	}
	const callers = 12
	results := make(chan result, callers)
	beforeHits, beforeMisses := w.p.checks.Counters()
	beforeChecks := w.checks()
	var ready sync.WaitGroup
	ready.Add(callers)
	for i := range callers {
		go func(i int) {
			ready.Done()
			pkg, err := snap.PackageWithFile(ctx, filepath.Join(w.root, "a", "a.gp"), variantSource(77))
			results <- result{pkg, err}
		}(i)
	}
	ready.Wait()
	// Wait only for store lookups, not for type-check scheduling: the shared promise is held at the semaphore.
	deadline := time.Now().Add(time.Minute) // (only ends a hang)
	for {
		hits, misses := w.p.checks.Counters()
		if hits >= beforeHits+callers-1 && misses >= beforeMisses+1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("variant callers did not reach the shared promise")
		}
		time.Sleep(time.Millisecond)
	}
	release()
	var first *Package
	for range callers {
		select {
		case got := <-results:
			if got.err != nil || got.pkg == nil || !got.pkg.IsVariant() {
				t.Fatalf("active variant caller or late-completion provenance: %+v", got)
			}
			if first == nil {
				first = got.pkg
			}
			if first != got.pkg {
				t.Fatal("active callers did not share the same completed variant")
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	if w.checks()-beforeChecks != 1 {
		t.Fatalf("%d checks after one canceled waiter; want one shared check", w.checks()-beforeChecks)
	}
	if got := checkedVariant(t, w, snap, variantSource(77)); got != first {
		t.Fatal("one canceled waiter discarded the other callers' completed result")
	}
}

type variantWaitContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (c *variantWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

func TestVariantKeeperPreservesInflightPromise(t *testing.T) {
	p := NewPipeline(testConfig())
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	id := pkgID{dir: "/variant-inflight"}
	key := hashOf([]byte("pending variant"))
	value := new(Package)
	started := make(chan struct{})
	release := make(chan struct{})
	type result struct {
		pkg *Package
		err error
	}
	results := make(chan result, 2)
	go func() {
		pkg, err := p.checks.Get(ctx, key, func(ctx context.Context) (*Package, error) {
			close(started)
			select {
			case <-release:
				return value, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
		results <- result{pkg, err}
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// Done is observed by Promise.Get only after this waiter has joined the existing run.
	waiterCtx, cancelWaiter := context.WithCancel(ctx)
	waiter := &variantWaitContext{Context: waiterCtx, observed: make(chan struct{})}
	canceledResult := make(chan error, 1)
	go func() {
		_, err := p.checks.Get(waiter, key, func(context.Context) (*Package, error) {
			return nil, errors.New("canceled waiter replaced the live computation")
		})
		canceledResult <- err
	}()
	select {
	case <-waiter.observed:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	cancelWaiter()
	select {
	case err := <-canceledResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled waiter: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// The canceled attempt notes the key while the original active consumer still owns the in-flight run.
	p.variantVersions.note(p, id, key)
	for i := range 12 {
		other := hashOf([]byte(fmt.Sprintf("completed variant %d", i)))
		if _, err := p.checks.Get(ctx, other, func(context.Context) (*Package, error) { return new(Package), nil }); err != nil {
			t.Fatal(err)
		}
		p.variantVersions.note(p, id, other)
	}
	go func() {
		pkg, err := p.checks.Get(ctx, key, func(context.Context) (*Package, error) {
			return nil, errors.New("eviction replaced an in-flight promise")
		})
		results <- result{pkg, err}
	}()
	close(release)
	for range 2 {
		select {
		case got := <-results:
			if got.err != nil || got.pkg != value {
				t.Fatalf("live promise changed: %+v", got)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	// Pipeline.checked notes this completion after Store.Get returns; model that final ownership step too.
	p.variantVersions.note(p, id, key)
	if entries, _ := p.checks.Stats(); entries > keptVersions {
		t.Fatalf("%d completed entries after the active variant finished", entries)
	}
}

func TestVariantCompletedErrorsBounded(t *testing.T) {
	p := NewPipeline(testConfig())
	defer p.Close()
	ctx := context.Background()
	id := pkgID{dir: "/variant-errors"}
	failure := errors.New("completed check error")
	for i := range 40 {
		key := hashOf([]byte(fmt.Sprintf("failed variant %d", i)))
		if _, err := p.checks.Get(ctx, key, func(context.Context) (*Package, error) { return nil, failure }); !errors.Is(err, failure) {
			t.Fatal(err)
		}
		if _, err := p.checked(ctx, &handle{id: id, variant: true}, &keyed{key: key}); !errors.Is(err, failure) {
			t.Fatal(err)
		}
		if entries, _ := p.checks.Stats(); entries > keptVersions {
			t.Fatalf("%d completed error values escaped variant retention", entries)
		}
	}
}

func TestVariantConfigKeyDistinctions(t *testing.T) {
	w := variantWorld(t)
	snap, free := w.s.Snapshot()
	defer free()
	checkedVariant(t, w, snap, variantSource(99))
	id := pkgID{dir: filepath.Join(w.root, "a")}
	firstKey := w.p.variantVersions.keys[id][0]
	cfg := testConfig()
	cfg.Tags = []string{"different-configuration"}
	otherPipeline := NewPipeline(cfg)
	otherSession := NewSession(otherPipeline.NewState())
	defer func() {
		otherSession.Shutdown()
		otherPipeline.Close()
	}()
	otherSnap, freeOther := otherSession.Snapshot()
	defer freeOther()
	pkg, err := otherSnap.PackageWithFile(context.Background(), filepath.Join(w.root, "a", "a.gp"), variantSource(99))
	if err != nil || pkg == nil {
		t.Fatalf("other configuration: %+v %v", pkg, err)
	}
	if key := otherPipeline.variantVersions.keys[id][0]; key == firstKey {
		t.Fatal("configuration identity disappeared from the variant key")
	}
}

func TestPackageVariantOriginCachedAndRecomputed(t *testing.T) {
	w := variantWorld(t)
	ordinary := w.pkg("a")
	if ordinary.IsVariant() {
		t.Fatal("original input has patched provenance")
	}
	snap, free := w.s.Snapshot()
	defer free()
	// Even byte-identical replacement text has the separate computed variant key/origin.
	first := checkedVariant(t, w, snap, variantSource(0))
	if cached := checkedVariant(t, w, snap, variantSource(0)); cached != first || !cached.IsVariant() {
		t.Fatal("cache hit did not preserve the patched package's immutable origin")
	}
	for i := range 12 {
		checkedVariant(t, w, snap, variantSource(i+1))
	}
	before := w.checks()
	recomputed := checkedVariant(t, w, snap, variantSource(0))
	if recomputed == first || w.checks() != before+1 || !recomputed.IsVariant() {
		t.Fatal("evicted variant was not recomputed with its own input provenance")
	}
	if !first.IsVariant() || ordinary.IsVariant() {
		t.Fatal("eviction/recomputation relabeled an active package holder")
	}
	if got := w.pkg("a"); got != ordinary || got.IsVariant() {
		t.Fatal("variant activity relabeled or replaced ordinary snapshot provenance")
	}
}
