package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The definitions that a source gave are kept from one snapshot to the next (step 12.13e): these
// tests count the questions that the header worker asks the compiler about the sources.

const memoHeader = `#pragma once
int add(int a, int b);
int tail(int a);
`

const memoMain = `package main
import "./geo.h"

func main() {
	_ = geo.add(1, 2)
}
`

const memoMainTail = `package main
import "./geo.h"

func main() {
	_ = geo.add(1, 2)
	_ = geo.tail(1)
}
`

const memoAdd = "#include \"geo.h\"\nint add(int a, int b) { return a + b; }\n"
const memoTail = "#include \"geo.h\"\nint tail(int a) { return a; }\n"

// sourceLog records the locate calls of the sources: the base name, and the names asked.
type sourceLog struct {
	mu    sync.Mutex
	calls []sourceCall
}

type sourceCall struct {
	source string
	names  []string
}

func (l *sourceLog) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = nil
}

// bySource returns the names asked in each call, by source.
func (l *sourceLog) bySource() map[string][][]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string][][]string)
	for _, call := range l.calls {
		out[call.source] = append(out[call.source], call.names)
	}
	return out
}

func (l *sourceLog) total() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.calls)
}

// logSources makes the locate calls of the world recorded. A call is a source's when its last
// include is a .cpp file.
func logSources(world *world) *sourceLog {
	log := new(sourceLog)
	ix := world.p.headers
	locate := ix.locate
	ix.locate = func(ctx context.Context, cfg probe.Config, includes []string,
		request probe.LocateRequest,
	) (*probe.Locations, error) {
		last := strings.Trim(includes[len(includes)-1], `"`)
		if filepath.Ext(last) == ".cpp" {
			var names []string
			for _, name := range request.Names {
				names = append(names, name.Name)
			}
			slices.Sort(names)
			log.mu.Lock()
			log.calls = append(log.calls, sourceCall{filepath.Base(last), names})
			log.mu.Unlock()
		}
		return locate(ctx, cfg, includes, request)
	}
	return log
}

// memoWorld is a package of three sources; the world has run them once when it returns.
func memoWorld(t *testing.T, main string) (*world, *sourceLog) {
	t.Helper()
	world := sourcesWorld(t, memoHeader, main, map[string]string{
		"geo.cpp": memoAdd, "a0.cpp": "#include \"geo.h\"\n", "zlast.cpp": memoTail})
	log := logSources(world)
	world.open("main.gp", main)
	if defs, ok := memoDefinitions(t, world, "add", 20*time.Second); !ok || len(defs) != 1 {
		t.Fatalf("add = %+v %v, want the definition", defs, ok)
	}
	if got := len(log.bySource()); got != 3 {
		t.Fatalf("the sources asked = %v, want the three", log.bySource())
	}
	return world, log
}

// memoDefinitions waits for the definitions of name in the current snapshot.
func memoDefinitions(t *testing.T, world *world, name string, wait time.Duration,
) ([]probe.Declaration, bool) {
	t.Helper()
	for deadline := time.Now().Add(wait); ; {
		pkg := world.pkg(".")
		world.p.headers.wg.Wait()
		snap, release := world.s.Snapshot()
		defs, ok := snap.ForeignDefinitions(pkg, pkg.Check.Load.Files[0], "", name)
		release()
		if ok || time.Now().After(deadline) {
			return defs, ok
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// writeInWorld writes the file of the world on disk.
func writeInWorld(t *testing.T, world *world, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(world.root, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func snapshotID(world *world) uint64 {
	snap, release := world.s.Snapshot()
	defer release()
	return snap.ID()
}

func TestSourceMemoServesAnEditOfTheGoFile(t *testing.T) {
	world, log := memoWorld(t, memoMain)
	before := snapshotID(world)
	log.reset()
	world.edit("main.gp", memoMain+"\nfunc extra() int { return 2 }\n")
	defs, ok := memoDefinitions(t, world, "add", 20*time.Second)
	if !ok || len(defs) != 1 || defs[0].Line != 1 {
		t.Fatalf("add after the edit = %+v %v, want the definition", defs, ok)
	}
	if snapshotID(world) == before {
		t.Fatal("the edit made no snapshot: the test says nothing")
	}
	if got := log.bySource(); len(got) != 0 {
		t.Errorf("an edit that changes no source and asks no name ran %v", got)
	}
}

func TestSourceMemoAsksOnlyTheNewNames(t *testing.T) {
	world, log := memoWorld(t, memoMain)
	log.reset()
	world.edit("main.gp", memoMainTail)
	defs, ok := memoDefinitions(t, world, "tail", 20*time.Second)
	if !ok || len(defs) != 1 || filepath.Base(defs[0].File) != "zlast.cpp" {
		t.Fatalf("tail = %+v %v, want the definition in zlast.cpp", defs, ok)
	}
	if defs, ok := memoDefinitions(t, world, "add", time.Second); !ok || len(defs) != 1 {
		t.Errorf("add = %+v %v: the definition kept from the first snapshot is lost", defs, ok)
	}
	for source, calls := range log.bySource() {
		if len(calls) != 1 || !slices.Equal(calls[0], []string{"tail"}) {
			t.Errorf("%s was asked %v, want the new name only, once", source, calls)
		}
	}
	// and back: the names that were asked are not asked again
	log.reset()
	world.edit("main.gp", memoMain)
	if defs, ok := memoDefinitions(t, world, "add", 20*time.Second); !ok || len(defs) != 1 {
		t.Fatalf("add = %+v %v", defs, ok)
	}
	if got := log.bySource(); len(got) != 0 {
		t.Errorf("fewer names ran %v", got)
	}
	if defs, ok := memoDefinitions(t, world, "tail", 200*time.Millisecond); ok {
		t.Errorf("tail = %+v: a name the file no longer writes has a fact", defs)
	}
}

func TestSourceMemoRevalidatesOnlyAChangedSource(t *testing.T) {
	world, log := memoWorld(t, memoMainTail)
	log.reset()
	edited := "// a comment above\n\n" + memoAdd
	writeInWorld(t, world, "geo.cpp", edited)
	world.p.HeadersChanged()
	var line int
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
		defs, _ := memoDefinitions(t, world, "add", time.Second)
		if len(defs) == 1 && defs[0].Line == 3 {
			line = 3
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if line != 3 {
		t.Fatal("the definition of add did not move with the edit of its source")
	}
	got := log.bySource()
	if len(got) != 1 || len(got["geo.cpp"]) != 1 || len(got["geo.cpp"][0]) != 2 {
		t.Errorf("a source edit ran %v, want geo.cpp once, with both names", got)
	}
}

func TestSourceMemoIsDroppedWhenAHeaderChanges(t *testing.T) {
	world, log := memoWorld(t, memoMainTail)
	log.reset()
	edited := memoHeader + "// an edit\n"
	if err := os.WriteFile(filepath.Join(world.root, "geo.h"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	world.p.HeadersChanged()
	for deadline := time.Now().Add(20 * time.Second); len(log.bySource()) < 3; {
		if time.Now().After(deadline) {
			t.Fatalf("after a header edit the sources asked = %v, want all three", log.bySource())
		}
		world.p.headers.wg.Wait()
		time.Sleep(20 * time.Millisecond)
	}
	for source, calls := range log.bySource() {
		if len(calls) != 1 || len(calls[0]) != 2 {
			t.Errorf("%s was asked %v, want once with both names", source, calls)
		}
	}
}

func TestSourceMemoIsNotKeptOfAFileThatWasWrittenDuringTheRun(t *testing.T) {
	world := sourcesWorld(t, memoHeader, memoMain, map[string]string{
		"geo.cpp": memoAdd, "a0.cpp": "#include \"geo.h\"\n"})
	log := logSources(world)
	ix := world.p.headers
	locate := ix.locate
	var once sync.Once
	ix.locate = func(ctx context.Context, cfg probe.Config, includes []string,
		request probe.LocateRequest,
	) (*probe.Locations, error) {
		got, err := locate(ctx, cfg, includes, request)
		if strings.Contains(includes[len(includes)-1], "geo.cpp") {
			once.Do(func() {
				// a write that leaves the size alone, as a quick edit does
				path := filepath.Join(world.root, "geo.cpp")
				if err := os.WriteFile(path, []byte(memoAdd), 0o644); err != nil {
					t.Error(err)
				}
			})
		}
		return got, err
	}
	world.open("main.gp", memoMain)
	if defs, ok := memoDefinitions(t, world, "add", 20*time.Second); !ok || len(defs) != 1 {
		t.Fatalf("add = %+v %v", defs, ok)
	}
	log.reset()
	world.edit("main.gp", memoMain+"\nfunc extra() int { return 2 }\n")
	if defs, ok := memoDefinitions(t, world, "add", 20*time.Second); !ok || len(defs) != 1 {
		t.Fatalf("add = %+v %v", defs, ok)
	}
	got := log.bySource()
	if len(got["geo.cpp"]) != 1 || len(got["a0.cpp"]) != 0 {
		t.Errorf("after a file changed during its run: asked %v, want geo.cpp only", got)
	}
}

func TestSourceMemosAreBounded(t *testing.T) {
	ix := &headerIndexer{}
	for i := range maxSourceMemos + 10 {
		ix.keepMemo(fmt.Sprint("key", i), &sourceMemo{})
	}
	if len(ix.memos) != maxSourceMemos {
		t.Errorf("memos = %d, want the most recent %d", len(ix.memos), maxSourceMemos)
	}
	if _, ok := ix.memos["key0"]; ok {
		t.Error("the memo used least recently was kept")
	}
	if _, ok := ix.memos[fmt.Sprint("key", maxSourceMemos+9)]; !ok {
		t.Error("the newest memo was dropped")
	}
	big := &sourceMemo{defs: map[string][]probe.Declaration{
		"x": {{File: strings.Repeat("f", maxSourceMemoBytes)}}}}
	ix.keepMemo("big", big)
	if _, ok := ix.memos["big"]; !ok || ix.memoBytes < maxSourceMemoBytes || len(ix.memos) != 1 {
		t.Errorf("a memo over the byte limit: %d memos, %d bytes, want it alone", len(ix.memos),
			ix.memoBytes)
	}
	ix.dropMemo("big")
	if len(ix.memos) != 0 || ix.memoBytes != 0 {
		t.Errorf("after the drop: %d memos, %d bytes", len(ix.memos), ix.memoBytes)
	}
}

// manySourceFiles is geo.cpp, zlast.cpp and eighteen sources that only include the header: sixteen
// of them are read.
func manySourceFiles() map[string]string {
	sources := map[string]string{"geo.cpp": memoAdd, "zlast.cpp": memoTail}
	for i := range 18 {
		sources[fmt.Sprintf("a%02d.cpp", i)] = "#include \"geo.h\"\n"
	}
	return sources
}

// The questions about the sources for each kind of edit, with twenty sources of which sixteen
// are read (before the memos every edit asked all sixteen).
func TestSourceMemoQuestionsPerEdit(t *testing.T) {
	world := sourcesWorld(t, memoHeader, memoMain, manySourceFiles())
	log := logSources(world)
	world.open("main.gp", memoMain)
	if defs, ok := memoDefinitions(t, world, "add", time.Minute); !ok || len(defs) != 1 {
		t.Fatalf("add = %+v %v", defs, ok)
	}
	if got := log.total(); got != 16 {
		t.Errorf("first snapshot: %d questions, want one for each of the 16 sources read", got)
	}
	log.reset()
	world.edit("main.gp", memoMain+"\nfunc extra() int { return 2 }\n")
	memoDefinitions(t, world, "add", time.Minute)
	if got := log.total(); got != 0 {
		t.Errorf("an edit of the .gp file: %d questions, want none", got)
	}
	log.reset()
	edited := "// comment\n" + memoAdd
	writeInWorld(t, world, "geo.cpp", edited)
	world.p.HeadersChanged()
	for deadline := time.Now().Add(time.Minute); ; time.Sleep(20 * time.Millisecond) {
		defs, _ := memoDefinitions(t, world, "add", time.Second)
		if len(defs) == 1 && defs[0].Line == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the definition did not move")
		}
	}
	if got := log.total(); got != 1 {
		t.Errorf("an edit of one source: %d questions %v, want one", got, log.bySource())
	}
	log.reset()
	writeInWorld(t, world, "geo.h", memoHeader+"\n")
	world.p.HeadersChanged()
	deadline := time.Now().Add(time.Minute)
	for log.total() < 16 {
		world.p.headers.wg.Wait()
		if time.Now().After(deadline) {
			t.Fatalf("an edit of the header: %d questions, want all 16", log.total())
		}
		time.Sleep(20 * time.Millisecond)
	}
	world.p.headers.wg.Wait()
	if got := log.total(); got != 16 {
		t.Errorf("an edit of the header: %d questions, want all 16", got)
	}
}
