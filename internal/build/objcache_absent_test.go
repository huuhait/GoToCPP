package build

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A header that appears EARLIER on the include path in the second folder (next to the
// including unit) must make the second program a miss.
func TestRevAbsentAcrossGenRoots(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		write := func(gen, rel, text string) {
			p := filepath.Join(gen, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		const un, unM = "gpkg/s/s.cpp", "gpkg/s/main.cpp"
		sources := func(gen string) {
			write(gen, "x.h", "#define XV 7\n")
			write(gen, "gpkg/s/pkg.hpp", "#pragma once\nint s_value();\n")
			write(gen, un, "#include \"gpkg/s/pkg.hpp\"\n#include \"x.h\"\nint s_value() { return XV; }\n")
			write(gen, unM, "#include <gocpp/rt.hpp>\n#include <cstdio>\n#include \"gpkg/s/pkg.hpp\"\nint main() { std::printf(\"%d\\n\", s_value()); }\n")
		}
		build := func(gen string) (string, string) {
			work := t.TempDir()
			var tr strings.Builder
			b := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: work, Cache: cache, Trace: &tr, Stderr: os.Stderr, GenRoot: gen}
			pkgs := []PackageBuild{{Path: "s", Dir: gen, Units: []string{filepath.Join(gen, un), filepath.Join(gen, unM)}}}
			bin := filepath.Join(work, "p")
			if err := b.Link(context.Background(), pkgs, bin); err != nil {
				var ce *CompileError
				if errors.As(err, &ce) {
					t.Fatalf("%v\n%s\n%s", ce, ce.Command, ce.Output)
				}
				t.Fatal(err)
			}
			o, err := exec.Command(bin).Output()
			if err != nil {
				t.Fatal(err)
			}
			return tr.String(), string(o)
		}
		gen1, gen2 := t.TempDir(), t.TempDir()
		sources(gen1)
		sources(gen2)
		build(gen1)
		_, out := build(gen2)
		if out != "7\n" {
			t.Fatalf("second: %q", out)
		}
		// now a header with the same name next to the unit in gen2 only
		write(gen2, "gpkg/s/x.h", "#define XV 9\n")
		tr, out := build(gen2)
		if out != "9\n" {
			t.Fatalf("STALE HIT: gen2 has gpkg/s/x.h (XV 9) earlier on the path but the program printed %q\n%s", out, tr)
		}
	})
}
