package cache

import (
	"bytes"
	"context"
	"fmt"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

func fakeMemberIndexer(t *testing.T) *memberIndexer {
	t.Helper()
	t.Setenv("GOCPP_INTEROP", "typed")
	ix := newMemberIndexer(testConfig())
	t.Cleanup(ix.close)
	return ix
}

func waitMemberEntry(t *testing.T, ix *memberIndexer, key string) *memberFacts {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		ix.mu.Lock()
		entry := ix.entries[key]
		var facts *memberFacts
		if entry != nil {
			facts = entry.facts
		}
		ix.mu.Unlock()
		if facts != nil {
			return facts
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("no completed member entry")
		}
	}
}

func TestMemberIndexCoalescesAndJoins(t *testing.T) {
	ix := fakeMemberIndexer(t)
	started := make(chan struct{}, 1)
	var calls atomic.Int32
	ix.run = func(ctx context.Context, _ *Package) (*memberFacts, error) {
		calls.Add(1)
		started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	pkg := &Package{cost: 1000}
	for range 20 {
		ix.schedule(context.Background(), "same", pkg)
	}
	headerSignal(t, started)
	ix.close()
	if calls.Load() != 1 || ix.bytes != 0 || len(ix.entries) != 0 {
		t.Fatalf("close/coalescing: calls=%d bytes=%d entries=%d", calls.Load(), ix.bytes, len(ix.entries))
	}
}

func TestMemberIndexRefreshRejectsOldGeneration(t *testing.T) {
	ix := fakeMemberIndexer(t)
	started := make(chan struct{}, 1)
	var calls atomic.Int32
	object := types.NewVar(token.NoPos, nil, "x", types.Typ[types.Invalid])
	ix.run = func(ctx context.Context, _ *Package) (*memberFacts, error) {
		name := "new"
		if calls.Add(1) == 1 {
			started <- struct{}{}
			<-ctx.Done()
			name = "old" // deliberately returns success despite cancellation
		}
		return &memberFacts{bindings: map[*types.Var]memberAnswer{
			object: {names: []string{name}},
		}, bytes: 256}, nil
	}
	ix.schedule(context.Background(), "entry", &Package{cost: 1000})
	headerSignal(t, started)
	ix.refresh()
	facts := waitMemberEntry(t, ix, "entry")
	if facts.bindings[object].names[0] != "new" || calls.Load() != 2 {
		t.Fatalf("old generation published: %+v, calls=%d", facts, calls.Load())
	}
}

func TestMemberIndexBoundsRetainedInputs(t *testing.T) {
	ix := fakeMemberIndexer(t)
	var active atomic.Int32
	var peak atomic.Int32
	ix.run = func(ctx context.Context, _ *Package) (*memberFacts, error) {
		count := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); count > old && !peak.CompareAndSwap(old, count); old = peak.Load() {
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	for i := range 100 {
		ix.schedule(context.Background(), fmt.Sprint(i), &Package{cost: 4 << 20})
		ix.mu.Lock()
		if ix.bytes > maxMemberIndexBytes || len(ix.entries) > maxMemberIndexes || len(ix.jobs) > maxMemberIndexes {
			t.Error("retained inputs exceeded bounds")
		}
		ix.mu.Unlock()
	}
	ix.close()
	if peak.Load() > maxProbes || active.Load() != 0 || ix.bytes != 0 {
		t.Fatalf("worker ownership: peak=%d active=%d bytes=%d", peak.Load(), active.Load(), ix.bytes)
	}
}

func TestMemberIndexDoesNotPublishFailedWork(t *testing.T) {
	ix := fakeMemberIndexer(t)
	finished := make(chan struct{}, 2)
	ix.run = func(context.Context, *Package) (*memberFacts, error) {
		finished <- struct{}{}
		return &memberFacts{bytes: 128}, fmt.Errorf("failed compiler")
	}
	ix.schedule(context.Background(), "failed", &Package{cost: 1000})
	headerSignal(t, finished)
	headerSignal(t, finished)
	ix.mu.Lock()
	entry := ix.entries["failed"]
	published := entry == nil || entry.facts != nil
	ix.mu.Unlock()
	if published {
		t.Fatal("failed work was published")
	}
}

func TestMemberIndexSnapshotCancellation(t *testing.T) {
	ix := fakeMemberIndexer(t)
	started := make(chan struct{}, 1)
	ix.run = func(ctx context.Context, _ *Package) (*memberFacts, error) {
		started <- struct{}{}
		<-ctx.Done()
		return &memberFacts{bytes: 128}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	ix.schedule(ctx, "old", &Package{cost: 1000})
	headerSignal(t, started)
	cancel()
	ix.close()
	if ix.bytes != 0 || len(ix.entries) != 0 {
		t.Fatal("canceled snapshot retained member facts")
	}
}

func TestMemberAnalysisPreservesPublishedPackage(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	defer probe.SetRacyWindow(0)()
	world := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/member\n",
		"main.gp":   "package main\nimport \"./api.hpp\"\nfunc main() { var x api.Widget; _ = x.field }\n",
		"api.hpp":   "struct Widget { int field; void method(); };\n",
	})
	t.Cleanup(world.p.Close)
	started := make(chan struct{}, 1)
	proceed := make(chan struct{})
	run := world.p.members.run
	world.p.members.run = func(ctx context.Context, pkg *Package) (*memberFacts, error) {
		started <- struct{}{}
		select {
		case <-proceed:
			return run(ctx, pkg)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	pkg := world.pkg(".")
	headerSignal(t, started)
	before := memberPackageFingerprint(t, pkg)
	snap, release := world.s.Snapshot()
	defer release()
	for _, object := range pkg.Check.Info.Uses {
		if variable, ok := object.(*types.Var); ok {
			requireTypeNotReady(t, snap, pkg, variable, "pending native type was exposed")
		}
	}
	close(proceed)
	facts := waitMemberEntry(t, world.p.members, memberEntryKey(snap, pkg))
	if len(facts.bindings) != 1 {
		t.Fatalf("bindings = %d, want 1", len(facts.bindings))
	}
	if after := memberPackageFingerprint(t, pkg); !bytes.Equal(before, after) {
		t.Fatal("background analysis changed the published package")
	}
	for object := range facts.bindings {
		native, partial, known := snap.ForeignVariableType(pkg, object)
		if !known || partial || native != "Widget" {
			t.Fatalf("native type = %q, partial=%v, ready=%v", native, partial, known)
		}
		if facts.bytes < int64(len(native)) {
			t.Fatalf("accounting = %d bytes for a %d-byte native type", facts.bytes, len(native))
		}
		names, incomplete, ready := snap.MemberSymbols(pkg, object)
		if !ready || incomplete || len(names) != 2 {
			t.Fatalf("member symbols = %v, %v, %v", names, incomplete, ready)
		}
		names[0] = "changed"
		again, _, _ := snap.MemberSymbols(pkg, object)
		if again[0] == "changed" {
			t.Fatal("request mutated completed facts")
		}
		variant := &Package{Check: pkg.Check, Program: pkg.Program, variant: true}
		if _, _, ready := snap.MemberSymbols(variant, object); ready {
			t.Fatal("original facts leaked into a repaired variant")
		}
		requireTypeNotReady(t, snap, variant, object, "native type leaked into a repaired variant")
		other := types.NewVar(object.Pos(), object.Pkg(), object.Name(), object.Type())
		if _, _, ready := snap.MemberSymbols(pkg, other); ready {
			t.Fatal("same spelling/position was mistaken for the same binding")
		}
		requireTypeNotReady(t, snap, pkg, other,
			"native type matched another object with the same spelling/position")
	}
}

// requireTypeNotReady checks that no native type is published for the binding and that the
// not-ready result is the documented zero triple, which cannot be mistaken for a partial type.
func requireTypeNotReady(
	t *testing.T, snap *Snapshot, pkg *Package, object *types.Var, why string,
) {
	t.Helper()
	native, partial, ready := snap.ForeignVariableType(pkg, object)
	if ready || native != "" || partial {
		t.Fatalf("%s: got (%q, partial=%v, ready=%v), want (\"\", false, false)",
			why, native, partial, ready)
	}
}

func TestMemberIndexHeaderRefresh(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	defer probe.SetRacyWindow(0)()
	world := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/refresh\n",
		"main.gp":   "package main\nimport \"./api.hpp\"\nfunc f() { var x api.Widget; _ = x.first }\n",
		"api.hpp":   "struct Widget { int first; };\n",
	})
	t.Cleanup(world.p.Close)
	pkg := world.pkg(".")
	snap, release := world.s.Snapshot()
	defer release()
	key := memberEntryKey(snap, pkg)
	before := waitMemberEntry(t, world.p.members, key)
	if err := os.WriteFile(filepath.Join(world.root, "api.hpp"),
		[]byte("struct Widget { int second; };\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	world.p.HeadersChanged()
	after := waitMemberEntry(t, world.p.members, key)
	if after == before {
		t.Fatal("header refresh retained old facts")
	}
	for _, answer := range after.bindings {
		if !slices.Equal(answer.names, []string{"second"}) {
			t.Fatalf("refreshed names = %v", answer.names)
		}
	}
}

func TestForeignVariableTypeHeaderRefresh(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	defer probe.SetRacyWindow(0)()
	const declarations = "struct First { int field; }; struct Second { int field; };\n"
	world := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/native\n",
		"main.gp": "package main\nimport \"./api.hpp\"\n" +
			"func f() { x := api.make(); _ = x.field }\n",
		"api.hpp": declarations + "First make();\n",
	})
	t.Cleanup(world.p.Close)
	pkg := world.pkg(".")
	snap, release := world.s.Snapshot()
	defer release()
	key := memberEntryKey(snap, pkg)
	before := waitMemberEntry(t, world.p.members, key)
	var variable *types.Var
	for object := range before.bindings {
		variable = object
	}
	if native, _, ready := snap.ForeignVariableType(pkg, variable); !ready || native != "First" {
		t.Fatalf("initial native type = %q, ready=%v", native, ready)
	}
	if err := os.WriteFile(filepath.Join(world.root, "api.hpp"),
		[]byte(declarations+"Second make();\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	world.p.HeadersChanged()
	waitMemberEntry(t, world.p.members, key)
	if native, _, ready := snap.ForeignVariableType(pkg, variable); !ready || native != "Second" {
		t.Fatalf("refreshed native type = %q, ready=%v", native, ready)
	}
}

func TestMemberIndexOverlaySuppressesNewFacts(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "typed")
	world := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/overlay\n",
		"main.gp":   "package main\nimport \"./api.hpp\"\nfunc f() { var x api.Widget; _ = x.field }\n",
		"api.hpp":   "struct Widget { int field; };\n",
	})
	t.Cleanup(world.p.Close)
	var calls atomic.Int32
	world.p.members.run = func(context.Context, *Package) (*memberFacts, error) {
		calls.Add(1)
		return &memberFacts{bytes: 128}, nil
	}
	world.open("notes.txt", "an unsaved non-source document")
	world.pkg(".")
	world.p.members.mu.Lock()
	entries := len(world.p.members.entries)
	world.p.members.mu.Unlock()
	if entries != 0 || calls.Load() != 0 {
		t.Fatalf("non-.gp overlay admitted work: entries=%d calls=%d", entries, calls.Load())
	}
}

func memberPackageFingerprint(t *testing.T, pkg *Package) []byte {
	t.Helper()
	var result bytes.Buffer
	for _, file := range pkg.Check.Load.Files {
		if err := format.Node(&result, pkg.Fset(), file.AST); err != nil {
			t.Fatal(err)
		}
	}
	var rows []string
	for id, object := range pkg.Check.Info.Uses {
		rows = append(rows, fmt.Sprintf("use:%d:%p:%s", id.Pos(), object, object))
	}
	for id, object := range pkg.Check.Info.Defs {
		rows = append(rows, fmt.Sprintf("def:%d:%p:%v", id.Pos(), object, object))
	}
	for expr, value := range pkg.Check.Info.Types {
		rows = append(rows, fmt.Sprintf("type:%d:%d:%v:%v", expr.Pos(), expr.End(), value.Type, value.Value))
	}
	slices.Sort(rows)
	for _, row := range rows {
		fmt.Fprintln(&result, row)
	}
	fmt.Fprintf(&result, "\nuses=%d defs=%d types=%d errors=%d conversions=%d\n",
		len(pkg.Check.Info.Uses), len(pkg.Check.Info.Defs), len(pkg.Check.Info.Types),
		len(pkg.Check.Errors.Diagnostics()), len(pkg.Check.Conversions))
	return result.Bytes()
}
