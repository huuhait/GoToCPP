package cache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

const foreignAPIHeader = "// Starts it.\nint Func(int a);\nint Hidden(int b);\n"

const foreignSource = `package main
import "./api.hpp"
import "<string>"

func main() {
	api.Func(1)
	var name std.string
	_ = name
}

func shadowed(api struct{ Func int }) int {
	return api.Func
}
`

func foreignWorld(t *testing.T) *world {
	t.Helper()
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	if _, err := probe.FindClang(context.Background()); err != nil {
		t.Skip(err)
	}
	t.Cleanup(probe.SetRacyWindow(0))
	world := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/foreign\n",
		"main.gp":   foreignSource,
		"api.hpp":   foreignAPIHeader,
	})
	t.Cleanup(world.p.Close)
	return world
}

func TestForeignNamesAreTheNamesTheFileWrites(t *testing.T) {
	world := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/names\n",
		"main.gp":   foreignSource + "\nfunc more() { var t std.chrono.seconds; _ = t }\n",
		"api.hpp":   foreignAPIHeader,
	})
	t.Cleanup(world.p.Close)
	pkg := world.pkg(".")
	got := foreignNames(pkg.Check.Load.Files[0])
	want := []foreignName{{scope: "", name: "Func"}, {scope: "std", name: "string"},
		{scope: "std", name: "chrono"}, {scope: "std::chrono", name: "seconds"}}
	for _, name := range want {
		if !slices.Contains(got, name) {
			t.Errorf("names = %v, missing %v", got, name)
		}
	}
	if count := len(got); count != len(want) {
		t.Errorf("names = %v, want exactly %v (the shadowed api.Func is no use of the header)", got, want)
	}
}

// waitForeign waits for the background worker of the package and returns the snapshot.
func waitForeign(t *testing.T, world *world) (*Snapshot, func(), *Package) {
	t.Helper()
	pkg := world.pkg(".")
	world.p.headers.wg.Wait()
	snap, release := world.s.Snapshot()
	return snap, release, pkg
}

func TestForeignDeclarationsAreCompletedBackgroundFacts(t *testing.T) {
	world := foreignWorld(t)
	snap, release, pkg := waitForeign(t, world)
	defer release()
	file := pkg.Check.Load.Files[0]
	decls, ok := snap.ForeignDeclarations(pkg, file, "", "Func")
	if !ok || len(decls) != 1 {
		t.Fatalf("api.Func = %v %v", decls, ok)
	}
	d := decls[0]
	if d.File != filepath.Join(world.root, "api.hpp") || d.Line != 1 || d.Col != 4 || d.Length != 4 ||
		d.Signature != "int Func(int a)" || d.Doc != "Starts it." {
		t.Errorf("api.Func = %+v", d)
	}
	if std, ok := snap.ForeignDeclarations(pkg, file, "std", "string"); !ok || len(std) != 1 ||
		!filepath.IsAbs(std[0].File) {
		t.Errorf("std.string = %v %v", std, ok)
	}
	t.Run("a name that the file does not write has no fact", func(t *testing.T) {
		if got, ok := snap.ForeignDeclarations(pkg, file, "", "Hidden"); ok || got != nil {
			t.Errorf("Hidden = %v", got)
		}
	})
	t.Run("a name that is not in the header has no fact", func(t *testing.T) {
		if got, ok := snap.ForeignDeclarations(pkg, file, "", "Missing"); ok || got != nil {
			t.Errorf("Missing = %v", got)
		}
	})
	t.Run("a repaired variant has no fact", func(t *testing.T) {
		variant := &Package{Check: pkg.Check, Program: pkg.Program, variant: true}
		if got, ok := snap.ForeignDeclarations(variant, file, "", "Func"); ok || got != nil {
			t.Errorf("variant = %v", got)
		}
	})
	t.Run("the answer is a copy", func(t *testing.T) {
		decls[0].Signature = "changed"
		again, _ := snap.ForeignDeclarations(pkg, file, "", "Func")
		if again[0].Signature != "int Func(int a)" {
			t.Errorf("the slot was changed through an answer: %v", again)
		}
	})
}

func TestForeignDeclarationsNeverWaitForTheWorker(t *testing.T) {
	world := foreignWorld(t)
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	world.p.headers.locate = func(ctx context.Context, cfg probe.Config, includes []string,
		req probe.LocateRequest,
	) (*probe.Locations, error) {
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil, ctx.Err()
	}
	t.Cleanup(func() { close(release) })
	pkg := world.pkg(".")
	headerSignal(t, started)
	snap, done := world.s.Snapshot()
	defer done()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		if got, ok := snap.ForeignDeclarations(pkg, pkg.Check.Load.Files[0], "", "Func"); ok || got != nil {
			t.Errorf("an unfinished fact answered: %v", got)
		}
	}()
	headerSignal(t, finished)
}

func TestForeignDeclarationsAreDroppedWhenHeadersChange(t *testing.T) {
	world := foreignWorld(t)
	snap, release, pkg := waitForeign(t, world)
	defer release()
	file := pkg.Check.Load.Files[0]
	edited := []byte("\n\n" + foreignAPIHeader)
	if err := os.WriteFile(filepath.Join(world.root, "api.hpp"), edited, 0o644); err != nil {
		t.Fatal(err)
	}
	var runs atomic.Int32
	locate := world.p.headers.locate
	hold := make(chan struct{})
	world.p.headers.locate = func(ctx context.Context, cfg probe.Config, includes []string,
		req probe.LocateRequest,
	) (*probe.Locations, error) {
		runs.Add(1)
		<-hold
		return locate(ctx, cfg, includes, req)
	}
	world.p.HeadersChanged()
	if got, ok := snap.ForeignDeclarations(pkg, file, "", "Func"); ok {
		t.Fatalf("a fact of the old headers answered while they are read again: %v", got)
	}
	close(hold)
	world.p.headers.wg.Wait()
	decls, ok := snap.ForeignDeclarations(pkg, file, "", "Func")
	if !ok || len(decls) != 1 || decls[0].Line != 3 {
		t.Fatalf("api.Func after the edit = %v %v, want line 3", decls, ok)
	}
	if runs.Load() != 1 {
		t.Errorf("locate runs after the refresh = %d, want 1", runs.Load())
	}
}

func TestForeignDeclarationsOfAnEditedSnapshotComeFromItsOwnWorker(t *testing.T) {
	world := foreignWorld(t)
	_, release, _ := waitForeign(t, world)
	release()
	world.open("main.gp", foreignSource)
	world.edit("main.gp", foreignSource+"\nfunc extra() { api.Hidden(2) }\n")
	hold := make(chan struct{})
	locate := world.p.headers.locate
	world.p.headers.locate = func(ctx context.Context, cfg probe.Config, includes []string,
		req probe.LocateRequest,
	) (*probe.Locations, error) {
		<-hold
		return locate(ctx, cfg, includes, req)
	}
	pkg := world.pkg(".")
	snap, done := world.s.Snapshot()
	defer done()
	file := pkg.Check.Load.Files[0]
	if got, ok := snap.ForeignDeclarations(pkg, file, "", "Hidden"); ok {
		t.Fatalf("the new snapshot answered before its worker finished: %v", got)
	}
	close(hold)
	world.p.headers.wg.Wait()
	got, ok := snap.ForeignDeclarations(pkg, file, "", "Hidden")
	if !ok || len(got) != 1 || got[0].Line != 2 {
		t.Fatalf("api.Hidden = %v %v", got, ok)
	}
}

func TestForeignDeclarationsRespectTheByteBudget(t *testing.T) {
	world := foreignWorld(t)
	ix := world.p.headers
	locate := ix.locate
	ix.locate = func(ctx context.Context, cfg probe.Config, includes []string,
		req probe.LocateRequest,
	) (*probe.Locations, error) {
		got, err := locate(ctx, cfg, includes, req)
		ix.mu.Lock()
		ix.bytes = maxHeaderIndexBytes // the cache is full when the answer arrives
		ix.mu.Unlock()
		return got, err
	}
	snap, release, pkg := waitForeign(t, world)
	defer release()
	if got, ok := snap.ForeignDeclarations(pkg, pkg.Check.Load.Files[0], "", "Func"); ok {
		t.Fatalf("an answer over the budget was kept: %v", got)
	}
}

func TestForeignDeclarationsNotWithoutAnIndex(t *testing.T) {
	ix := fakeHeaderIndexer(t)
	var asked atomic.Bool
	ix.locate = func(context.Context, probe.Config, []string, probe.LocateRequest) (*probe.Locations, error) {
		asked.Store(true)
		return nil, nil
	}
	ix.run = func(context.Context, probe.Config, []string) (*probe.HeaderIndex, error) {
		return new(probe.HeaderIndex), nil
	}
	ix.schedule(context.Background(), "key", "/package", nil, []string{`"api.hpp"`},
		foreignName{name: "Func"})
	ix.wg.Wait()
	if asked.Load() {
		t.Error("the compiler was asked for names that the index does not have")
	}
}

func TestForeignDeclarationsStopWithTheRequestContext(t *testing.T) {
	ix := fakeHeaderIndexer(t)
	ctx, cancel := context.WithCancel(context.Background())
	canceled := make(chan struct{})
	runner := new(probe.Runner)
	ix.locate = func(_ context.Context, cfg probe.Config, includes []string,
		request probe.LocateRequest,
	) (*probe.Locations, error) {
		// The slot is replaced while its worker has a complete answer in hand.
		got, err := runner.Locate(context.Background(), cfg, includes, request)
		cancel()
		close(canceled)
		return got, err
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"), []byte(foreignAPIHeader), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := probe.FindClang(context.Background()); err != nil {
		t.Skip(err)
	}
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	ix.find = func(ctx context.Context) (build.CXX, error) { return probe.FindClang(ctx) }
	ix.schedule(ctx, "key", dir, nil, []string{`"api.hpp"`}, foreignName{name: "Func"})
	headerSignal(t, canceled)
	ix.wg.Wait()
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if entry := ix.entries["key"]; entry != nil && len(entry.decls) != 0 {
		t.Errorf("facts of a canceled slot: %v", entry.decls)
	}
}

func TestForeignDeclarationsOfASlotThatIsNoLongerTheCurrentOneAreDropped(t *testing.T) {
	clang, err := probe.FindClang(context.Background())
	if err != nil {
		t.Skip(err)
	}
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Cleanup(probe.SetRacyWindow(0))
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"), []byte(foreignAPIHeader), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := probe.Config{CXX: clang, Dir: dir, CacheDir: "off"}
	runner := new(probe.Runner)
	includes := []string{`"api.hpp"`}
	index, err := runner.Index(context.Background(), cfg, includes)
	if err != nil {
		t.Fatal(err)
	}
	ix := fakeHeaderIndexer(t)
	ix.locate = runner.Locate
	names := []foreignName{{name: "Func"}}
	current := &headerEntry{ctx: context.Background(), cancel: func() {}}
	replaced := &headerEntry{ctx: context.Background(), cancel: func() {}}
	ix.entries["key"] = current
	ix.locateNames(context.Background(), replaced, "key", cfg, includes, index, names)
	if len(replaced.decls) != 0 || len(current.decls) != 0 {
		t.Fatalf("facts for a replaced slot were kept: %v %v", replaced.decls, current.decls)
	}
	ix.locateNames(context.Background(), current, "key", cfg, includes, index, names)
	if len(current.decls[foreignKey("", "Func")]) != 1 {
		t.Fatalf("facts for the current slot = %v", current.decls)
	}
}

func TestForeignDeclarationRetryDoesNotHoldAProbeSlot(t *testing.T) {
	ix := fakeHeaderIndexer(t)
	var locates atomic.Int32
	failed := make(chan struct{}, 1)
	ix.locate = func(context.Context, probe.Config, []string, probe.LocateRequest) (*probe.Locations, error) {
		if locates.Add(1) == 1 {
			failed <- struct{}{}
		}
		return nil, errors.New("the compiler failed") // the worker waits to retry
	}
	clang, err := probe.FindClang(context.Background())
	if err != nil {
		t.Skip(err)
	}
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Cleanup(probe.SetRacyWindow(0))
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.hpp"), []byte(foreignAPIHeader), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := new(probe.Runner)
	started := make(chan int32, 3)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	ix.run = func(ctx context.Context, _ probe.Config, includes []string) (*probe.HeaderIndex, error) {
		started <- locates.Load()
		if includes[0] != `"a.hpp"` {
			select { // an index that takes its time holds its probe slot
			case <-release:
			case <-ctx.Done():
			}
			return new(probe.HeaderIndex), nil
		}
		return runner.Index(ctx, probe.Config{CXX: clang, Dir: dir, CacheDir: "off"}, includes)
	}
	ix.schedule(context.Background(), "a", dir, nil, []string{`"a.hpp"`}, foreignName{name: "Func"})
	<-started
	headerSignal(t, failed) // a's index is published, and its first locate has failed
	for _, key := range []string{"b", "c"} {
		ix.schedule(context.Background(), key, "/package", nil, []string{`"` + key + `.hpp"`})
	}
	// Both other indexes start while a waits for its retry: a holds no probe slot.
	for range 2 {
		select {
		case seen := <-started:
			if seen != 1 {
				t.Fatalf("an index started only after the retry of another slot (locates = %d)", seen)
			}
		case <-time.After(time.Minute):
			t.Fatal("an index run is blocked by the declaration retry of another")
		}
	}
}

func TestForeignDeclarationsStopWhenTheSlotEndsBeforePublishing(t *testing.T) {
	ix := fakeHeaderIndexer(t)
	if _, err := probe.FindClang(context.Background()); err != nil {
		t.Skip(err)
	}
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Cleanup(probe.SetRacyWindow(0))
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "api.hpp"), []byte(foreignAPIHeader), 0o644); err != nil {
		t.Fatal(err)
	}
	clang, _ := probe.FindClang(context.Background())
	cfg := probe.Config{CXX: clang, Dir: dir, CacheDir: "off"}
	runner := new(probe.Runner)
	includes := []string{`"api.hpp"`}
	index, err := runner.Index(context.Background(), cfg, includes)
	if err != nil {
		t.Fatal(err)
	}
	ix.locate = runner.Locate
	ctx, cancel := context.WithCancel(context.Background())
	ix.beforePublish = cancel // the slot ends after the answer is complete, before it is kept
	entry := &headerEntry{ctx: ctx, cancel: cancel}
	ix.entries["key"] = entry
	ix.locateNames(ctx, entry, "key", cfg, includes, index, []foreignName{{name: "Func"}})
	if len(entry.decls) != 0 {
		t.Fatalf("facts of a slot that ended were kept: %v", entry.decls)
	}
}
