package probe

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/collect"
)

func TestVolatileIn(t *testing.T) {
	dm := "#define A __LINE__\n#define B A\n#define C (B + 1)\n#define D(x) x + __FILE__\n#define E \"__LINE__\"\n#define F '_'\n#define G __LINE__X\n" +
		"#define LOOP1 LOOP2\n#define LOOP2 LOOP1\n#define H(a) __COUNTER__\n#define ok 1\n#define __cplusplus 202002L\n#define EMPTY\n" +
		"#define NAMES \"a\\\"__DATE__\" 1\n#define T __TIME__\n#define U1 T\n#define U2 __TIMESTAMP__\n#define U3 __BASE_FILE__\n#define U4 __INCLUDE_LEVEL__\n#define U5 __FILE_NAME__\n"
	got := volatileIn([]byte(dm))
	want := map[string]string{"A": "__LINE__", "B": "__LINE__", "C": "__LINE__", "D": "__FILE__", "H": "__COUNTER__", "T": "__TIME__", "U1": "__TIME__",
		"U2": "__TIMESTAMP__", "U3": "__BASE_FILE__", "U4": "__INCLUDE_LEVEL__", "U5": "__FILE_NAME__"}
	if len(got) != len(want) {
		t.Errorf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %q, want %q", k, got[k], v)
		}
	}
	for _, k := range []string{"E", "F", "G", "LOOP1", "LOOP2", "ok", "__cplusplus", "EMPTY", "NAMES"} {
		if _, bad := got[k]; bad {
			t.Errorf("%s is not volatile", k)
		}
	}
}

func TestStripLiterals(t *testing.T) {
	for in, want := range map[string]string{`a "b" c`: `a   c`, `'"' x`: `  x`, `"a\"b" y`: `  y`, `L'x' + z`: `L  + z`} {
		if got := stripLiterals(in); got != want {
			t.Errorf("stripLiterals(%q) = %q, want %q", in, got, want)
		}
	}
}

// The answer is cached like the probe's: a second Runner runs no compiler, an edit of a header is seen.
func TestVolatileMacrosCache(t *testing.T) {
	dir := t.TempDir()
	h := filepath.Join(dir, "v.hpp")
	write := func(body string) {
		if err := os.WriteFile(h, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-time.Hour)
		os.Chtimes(h, old, old)
	}
	write("#define W __LINE__\n#define OK 1\n")
	cfg := clangCacheConfig(t, dir)
	src := Generate(collect.NewSet(), GenConfig{Includes: []string{`"./v.hpp"`}})
	ctx := context.Background()
	m1, err := new(Runner).VolatileMacros(ctx, cfg, src)
	if err != nil || m1["W"] != "__LINE__" || m1["OK"] != "" {
		t.Fatalf("cold: %v, %v", m1, err)
	}
	ResetCacheCounters()
	m2, err := new(Runner).VolatileMacros(ctx, cfg, src)
	if err != nil || m2["W"] != "__LINE__" || CacheCounters().DiskHits == 0 {
		t.Fatalf("warm: %v, %v, %+v", m2, err, CacheCounters())
	}
	write("#define W 2\n#define OK __DATE__\n")
	m3, err := new(Runner).VolatileMacros(ctx, cfg, src)
	if err != nil || m3["OK"] != "__DATE__" || m3["W"] != "" {
		t.Fatalf("after the edit: %v, %v", m3, err)
	}
}
