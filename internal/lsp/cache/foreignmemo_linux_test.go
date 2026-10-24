//go:build linux

package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// countingScript runs the real compiler and leaves a line in $COUNT_LOG when a run starts ("run",
// then "lookup" for a definition lookup in a source of the package: its probe file includes a
// .cpp, .cc, .cxx or .c file, "other" for the rest) and when it ends ("end").
const countingScript = `#!/bin/sh
for last; do :; done
kind=other
if [ -f "$last" ] && grep -qE '\.(cpp|cc|cxx|c)"' "$last"; then kind=lookup; fi
echo "run $kind $$ $*" >> "$COUNT_LOG"
"$COUNT_REAL" "$@"
status=$?
echo "end $$ $status" >> "$COUNT_LOG"
exit $status
`

// compilerRuns counts the definition lookups in the sources that the log has. The other work that
// an edit starts (the typed probe of the package, the compiler's search path, the header index)
// is not counted: it is legitimate, and on a loaded machine it can come in the counted window.
func compilerRuns(t *testing.T, log string) int {
	t.Helper()
	data, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count("\n"+string(data), "\nrun lookup ")
}

// settle waits until the background work of the world is over: the header index, the namespace
// prober, the adaptation and member analyses, and every compiler run that started has ended (the
// log has as many ends as runs). A phase of the test counts the runs that its own edit makes: work
// that is left of the phase before (a loaded machine is slower to end it) and that the edit stops
// and starts again would be counted with it.
func settle(t *testing.T, world *world, log string) {
	t.Helper()
	for deadline := time.Now().Add(hangLimit); ; time.Sleep(20 * time.Millisecond) {
		world.p.headers.wg.Wait()
		world.p.prober.wait()
		if world.p.adapt != nil {
			world.p.adapt.wg.Wait()
		}
		busy := false
		if ix := world.p.members; ix != nil {
			ix.mu.Lock()
			for _, entry := range ix.entries {
				busy = busy || entry.running || entry.queued
			}
			ix.mu.Unlock()
		}
		data, _ := os.ReadFile(log)
		text := "\n" + string(data)
		if !busy && strings.Count(text, "\nrun ") == strings.Count(text, "\nend ") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the background work never ended:\n%s", data)
		}
	}
}

// The lookup runs of a package of twenty sources (sixteen are read), counted at the compiler.
func TestSourceMemoCompilerRunsPerEdit(t *testing.T) {
	clang, err := probe.FindClang(t.Context())
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	compiler := filepath.Join(dir, "clang++")
	if err := os.WriteFile(compiler, []byte(countingScript), 0o755); err != nil {
		t.Fatal(err)
	}
	count := filepath.Join(dir, "runs.log")
	t.Setenv("COUNT_LOG", count)
	t.Setenv("COUNT_REAL", clang.Path)
	t.Setenv("GOCPP_CLANG", compiler)
	sources := map[string]string{"geo.cpp": memoAdd, "zlast.cpp": memoTail}
	for i := range 18 {
		sources[filepath.Join("", "a"+string(rune('a'+i))+".cpp")] = "#include \"geo.h\"\n"
	}
	world := sourcesWorld(t, memoHeader, memoMain, sources)
	world.open("main.gp", memoMain)
	if defs, ok := memoDefinitions(t, world, "add", hangLimit); !ok || len(defs) != 1 {
		t.Fatalf("add = %+v %v", defs, ok)
	}
	settle(t, world, count)
	cold := compilerRuns(t, count)
	t.Logf("lookup runs, 16 sources read, first snapshot: %d", cold)
	// Exactly three runs for each source (the discovery, the includes question and the dump); the
	// racy window is 0 here, so no answer is asked again. A lookup made twice is a failure.
	if cold != 3*16 {
		t.Fatalf("cold lookup runs = %d, want three for each of the sixteen sources, 48", cold)
	}

	settle(t, world, count)
	if err := os.WriteFile(count, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	world.edit("main.gp", memoMain+"\nfunc extra() int { return 2 }\n")
	if defs, ok := memoDefinitions(t, world, "add", hangLimit); !ok || len(defs) != 1 {
		t.Fatalf("add = %+v %v", defs, ok)
	}
	gp := compilerRuns(t, count)
	t.Logf("lookup runs, an edit of the .gp file: %d", gp)
	if gp != 0 {
		t.Errorf("an edit of the .gp file ran %d lookups, want none", gp)
	}

	settle(t, world, count)
	if err := os.WriteFile(count, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	edited := "// comment\n" + memoAdd
	writeInWorld(t, world, "geo.cpp", edited)
	world.p.HeadersChanged()
	for deadline := time.Now().Add(hangLimit); ; {
		defs, _ := memoDefinitions(t, world, "add", time.Second)
		if len(defs) == 1 && defs[0].Line == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the definition did not move")
		}
		time.Sleep(20 * time.Millisecond)
	}
	source := compilerRuns(t, count)
	runsText, _ := os.ReadFile(count)
	t.Logf("lookup runs, an edit of one source: %d\n%s", source, runsText)
	if source == 0 || source > 3 {
		t.Errorf("an edit of one source ran %d lookups, want its three runs", source)
	}
}
