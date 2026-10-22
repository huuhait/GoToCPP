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

// countingScript runs the real compiler and leaves a line in $COUNT_LOG for every run.
const countingScript = `#!/bin/sh
echo run >> "$COUNT_LOG"
exec "$COUNT_REAL" "$@"
`

func compilerRuns(t *testing.T, log string) int {
	t.Helper()
	data, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "\n")
}

// The compiler runs of a package of twenty sources (sixteen are read), counted at the compiler.
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
	if defs, ok := memoDefinitions(t, world, "add", time.Minute); !ok || len(defs) != 1 {
		t.Fatalf("add = %+v %v", defs, ok)
	}
	cold := compilerRuns(t, count)
	t.Logf("compiler runs, 16 sources read, first snapshot: %d", cold)
	if cold < 3*16 {
		t.Fatalf("cold runs = %d, want at least three for each of the sixteen sources", cold)
	}

	if err := os.WriteFile(count, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	world.edit("main.gp", memoMain+"\nfunc extra() int { return 2 }\n")
	if defs, ok := memoDefinitions(t, world, "add", time.Minute); !ok || len(defs) != 1 {
		t.Fatalf("add = %+v %v", defs, ok)
	}
	gp := compilerRuns(t, count)
	t.Logf("compiler runs, an edit of the .gp file: %d", gp)
	if gp != 0 {
		t.Errorf("an edit of the .gp file ran the compiler %d times, want none", gp)
	}

	if err := os.WriteFile(count, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	edited := "// comment\n" + memoAdd
	writeInWorld(t, world, "geo.cpp", edited)
	world.p.HeadersChanged()
	for deadline := time.Now().Add(time.Minute); ; {
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
	t.Logf("compiler runs, an edit of one source: %d", source)
	if source == 0 || source > 3 {
		t.Errorf("an edit of one source ran the compiler %d times, want its three runs", source)
	}
}
