package interop

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// TestProbeIncludeDirsInKey checks the probes of a module header (Phase 06
// step 6.1b): the header is included by its absolute name and finds the
// other headers of its library through the module's folder, an -I flag. The
// answer depends on that flag, so the key of the cache (in memory and on
// disk) has it: the same header and include text with another folder gives
// the answer of that folder, never the cached answer of the first.
func TestProbeIncludeDirsInKey(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		pkg := t.TempDir()
		lib := t.TempDir() // the header of a library, outside the package folder
		writeFiles(t, lib, map[string]string{"lib.h": "#pragma once\n#include \"deps/dep.h\"\n"})
		a, b := t.TempDir(), t.TempDir()
		writeFiles(t, a, map[string]string{"deps/dep.h": "#pragma once\nnamespace from_a { int v; }\n"})
		writeFiles(t, b, map[string]string{"deps/dep.h": "#pragma once\nnamespace from_b { int v; }\n"})
		// The include text of a module header: quoted, absolute.
		includes := []string{`"` + filepath.ToSlash(filepath.Join(lib, "lib.h")) + `"`}
		cands := []string{"from_a", "from_b"}

		disk := t.TempDir()
		ctx := context.Background()
		for pass, c := range []*cache{diskCacheAt(disk), diskCacheAt(disk)} { // the second is another process
			ra, err := c.probe(ctx, cxx, pkg, []string{"-I" + a}, includes, cands)
			if err != nil {
				t.Fatal(err)
			}
			rb, err := c.probe(ctx, cxx, pkg, []string{"-I" + b}, includes, cands)
			if err != nil {
				t.Fatal(err)
			}
			rn, err := c.probe(ctx, cxx, pkg, nil, includes, cands)
			if err != nil {
				t.Fatal(err)
			}
			checkKinds(t, ra, []kindCase{{"from_a", Namespace}, {"from_b", NotFound}})
			checkKinds(t, rb, []kindCase{{"from_a", NotFound}, {"from_b", Namespace}})
			// Without the module's folder, the library's own include is not found.
			if len(rn.IncludeErrors) != 1 {
				t.Errorf("without -I: %+v, want one include error", rn)
			}
			if pass == 1 && c.diskHits.Load() < 2 {
				t.Errorf("the second process read %d answers from the disk, want the two without errors", c.diskHits.Load())
			}
		}
	})
}
