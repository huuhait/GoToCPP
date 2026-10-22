package cache

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

func fakeHeaderIndexer(t *testing.T) *headerIndexer {
	t.Helper()
	t.Setenv("GOCPP_INTEROP", "typed")
	ix := newHeaderIndexer(testConfig())
	ix.find = func(context.Context) (build.CXX, error) {
		return build.CXX{Path: "fake", Family: build.Clang}, nil
	}
	t.Cleanup(ix.close)
	return ix
}

func headerSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Minute):
		t.Fatal("header worker did not signal")
	}
}

func TestHeaderIndexSnapshotCancellation(t *testing.T) {
	ix := fakeHeaderIndexer(t)
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	ix.run = func(context.Context, probe.Config, []string) (*probe.HeaderIndex, error) {
		started <- struct{}{}
		<-release // even a result that ignores cancellation may not publish
		return new(probe.HeaderIndex), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	ix.schedule(ctx, "old", "/package", nil, []string{`"old.hpp"`})
	headerSignal(t, started)
	cancel()
	ix.schedule(context.Background(), "new", "/package", nil, []string{`"new.hpp"`})
	headerSignal(t, started)
	ix.mu.Lock()
	if ix.entries["old"] != nil || ix.entries["new"].index != nil {
		t.Error("old snapshot was retained or unfinished result was published")
	}
	ix.mu.Unlock()
	unblock()
	ix.wg.Wait()
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.entries["old"] != nil || ix.entries["new"].index == nil {
		t.Fatal("canceled result published or current result was lost")
	}
}

func TestHeaderIndexCoalescesScheduling(t *testing.T) {
	ix := fakeHeaderIndexer(t)
	started := make(chan struct{}, 1)
	var calls atomic.Int32
	ix.run = func(ctx context.Context, _ probe.Config, _ []string) (*probe.HeaderIndex, error) {
		calls.Add(1)
		started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	for range 20 {
		ix.schedule(context.Background(), "same", "/package", nil, []string{`"api.hpp"`})
	}
	headerSignal(t, started)
	ix.close()
	if calls.Load() != 1 {
		t.Fatalf("identical schedules started %d workers", calls.Load())
	}
}

func TestHeaderIndexBoundsAndClose(t *testing.T) {
	ix := fakeHeaderIndexer(t)
	ix.run = func(ctx context.Context, _ probe.Config, _ []string) (*probe.HeaderIndex, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	for i := range maxHeaderIndexes + 16 {
		ix.schedule(context.Background(), string(rune(i+1)), "/package", nil, []string{`"api.hpp"`})
	}
	ix.mu.Lock()
	if len(ix.entries) != maxHeaderIndexes {
		t.Errorf("entries = %d, want %d", len(ix.entries), maxHeaderIndexes)
	}
	ix.mu.Unlock()
	joined := make(chan struct{})
	go func() {
		ix.close()
		close(joined)
	}()
	headerSignal(t, joined)
	ix.schedule(context.Background(), "after-close", "/package", nil, nil)
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if len(ix.entries) != 0 || ix.bytes != 0 {
		t.Fatal("close retained or admitted index entries")
	}
}

func TestHeaderIndexBoundsCompletedBytes(t *testing.T) {
	ix := fakeHeaderIndexer(t)
	ix.entries["older"] = &headerEntry{
		ctx: context.Background(), cancel: func() {}, index: new(probe.HeaderIndex),
		bytes: maxHeaderIndexBytes,
	}
	ix.bytes = maxHeaderIndexBytes
	ix.run = func(context.Context, probe.Config, []string) (*probe.HeaderIndex, error) {
		return new(probe.HeaderIndex), nil
	}
	ix.schedule(context.Background(), "new", "/package", nil, []string{`"api.hpp"`})
	ix.wg.Wait()
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.entries["older"] != nil || ix.entries["new"].index == nil || ix.bytes > maxHeaderIndexBytes {
		t.Fatalf("byte eviction failed: entries=%d bytes=%d", len(ix.entries), ix.bytes)
	}
}

func TestHeaderIndexInvalidationDiscardsInFlight(t *testing.T) {
	ix := fakeHeaderIndexer(t)
	started := make(chan struct{}, 1)
	finished := make(chan struct{})
	ix.run = func(ctx context.Context, _ probe.Config, _ []string) (*probe.HeaderIndex, error) {
		close(started)
		<-ctx.Done()
		close(finished)
		return new(probe.HeaderIndex), nil
	}
	ix.schedule(context.Background(), "header", "/package", nil, []string{`"api.hpp"`})
	headerSignal(t, started)
	ix.invalidate()
	headerSignal(t, finished)
	ix.wg.Wait()
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if len(ix.entries) != 0 {
		t.Fatal("invalidated worker published a stale result")
	}
}

func TestHeaderIndexPackageSchedules(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "typed")
	world := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/index\n",
		"main.gp":   "package main\nimport \"./api.hpp\"\nfunc main() { api.Func() }\n",
		"api.hpp":   "void Func();\n",
	})
	t.Cleanup(world.p.Close)
	started := make(chan struct{}, 1)
	world.p.headers.find = func(context.Context) (build.CXX, error) {
		return build.CXX{Path: "fake", Family: build.Clang}, nil
	}
	world.p.headers.run = func(context.Context, probe.Config, []string) (*probe.HeaderIndex, error) {
		started <- struct{}{}
		return new(probe.HeaderIndex), nil
	}
	world.pkg(".")
	headerSignal(t, started)
}

func TestHeaderIndexRealPackageRefresh(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	if _, err := probe.FindClang(context.Background()); err != nil {
		t.Skip(err)
	}
	t.Cleanup(probe.SetRacyWindow(0))
	world := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/index\n",
		"main.gp":   "package main\nimport \"./api.hpp\"\nfunc main() { api.Func() }\n",
		"api.hpp":   "void Func();\n",
	})
	t.Cleanup(world.p.Close)
	results := make(chan error, 2)
	run := world.p.headers.run
	world.p.headers.run = func(ctx context.Context, cfg probe.Config, includes []string) (*probe.HeaderIndex, error) {
		index, err := run(ctx, cfg, includes)
		results <- err
		return index, err
	}
	pkg := world.pkg(".")
	select {
	case err := <-results:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Minute):
		t.Fatal("no index result")
	}
	world.p.headers.wg.Wait()
	snap, release := world.s.Snapshot()
	defer release()
	if names := snap.HeaderSymbols(pkg, pkg.Check.Load.Files[0], ""); len(names) != 1 || names[0].Name != "Func" {
		t.Fatalf("package header symbols = %v", names)
	}
	if err := os.WriteFile(filepath.Join(world.root, "api.hpp"), []byte("void Changed();\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	world.p.HeadersChanged()
	select {
	case err := <-results:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Minute):
		t.Fatal("header watcher did not refresh the index")
	}
	world.p.headers.wg.Wait()
	if names := snap.HeaderSymbols(pkg, pkg.Check.Load.Files[0], ""); len(names) != 1 || names[0].Name != "Changed" {
		t.Fatalf("refreshed header symbols = %v", names)
	}
}

func TestHeaderIndexOpaqueIsDisabled(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "opaque")
	if newHeaderIndexer(testConfig()) != nil {
		t.Fatal("opaque mode enabled clang indexing")
	}
}
