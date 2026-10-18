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
	"time"
)

// TestLinkPackages builds a program of two packages (Link): the units of both include a header
// of the generated folder by its path (GenRoot is an -iquote folder), the object cache keeps each
// unit (also when its header was written a moment ago: the generated files are exempt from the
// racy window), a changed header recompiles the units that include it and no other, and the
// public flags of a package reach the units of the other.
func TestLinkPackages(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		gen := t.TempDir()
		write := func(rel, text string) {
			p := filepath.Join(gen, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(text), 0o644); err != nil { // (not aged: written just now)
				t.Fatal(err)
			}
		}
		const (
			hdrA = "gpkg/a/pkg.hpp"
			hdrB = "gpkg/b/pkg.hpp"
			unA  = "gpkg/a/a.cpp"
			unB  = "gpkg/b/b.cpp"
			unM  = "gpkg/b/main.cpp"
		)
		sources := func(valueA string) {
			write(hdrA, "#pragma once\n#ifndef FROM_A\n#error the public flags of package a are missing\n#endif\nint a_value();\n")
			write(hdrB, "#pragma once\n#include \"gpkg/a/pkg.hpp\"\nint b_value();\n")
			write(unA, "#include \"gpkg/a/pkg.hpp\"\nint a_value() { return "+valueA+"; }\n")
			write(unB, "#include \"gpkg/b/pkg.hpp\"\nint b_value() { return a_value() + 10; }\n")
			write(unM, "#include <gocpp/rt.hpp>\n#include <cstdio>\n#include \"gpkg/b/pkg.hpp\"\nint main() { std::printf(\"%d\\n\", b_value()); }\n")
		}
		build := func() (trace, out string) {
			work := t.TempDir()
			var tr strings.Builder
			b := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: work, Cache: cache, Trace: &tr, Stderr: os.Stderr, GenRoot: gen}
			pkgs := []PackageBuild{
				{Path: "a", Dir: gen, Flags: PackageFlags{Compile: []string{"-DFROM_A=1"}}, Units: []string{filepath.Join(gen, unA)}},
				// b includes a's header: a's flags are public.
				{Path: "b", Dir: gen, Public: []string{"-DFROM_A=1"}, Units: []string{filepath.Join(gen, unB), filepath.Join(gen, unM)}},
			}
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
		compiled := func(trace string) []string {
			var units []string
			for _, line := range strings.Split(trace, "\n") {
				if !strings.HasPrefix(line, cxx.Path+" ") {
					continue
				}
				for _, u := range []string{unA, unB, unM} {
					if strings.Contains(line, " -c "+filepath.Join(gen, u)+" ") || strings.Contains(line, " "+filepath.Join(gen, u)+" -o ") {
						units = append(units, u)
					}
				}
			}
			return units
		}
		hits := func(trace string) (n int) {
			for _, u := range []string{unA, unB, unM} {
				if strings.Contains(trace, "# cache hit "+u+" ") {
					n++
				}
			}
			return
		}

		// The runtime is built first (a build with a cold runtime takes longer than the racy
		// window, which would hide what the headers written just now do to the cache).
		warmDir := t.TempDir()
		warm := filepath.Join(warmDir, "w.cpp")
		if err := os.WriteFile(warm, []byte("#include <gocpp/rt.hpp>\nint main() {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		wb := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: t.TempDir(), Cache: cache, Stderr: os.Stderr}
		if err := wb.Link(context.Background(), []PackageBuild{{Path: "w", Dir: warmDir, Units: []string{warm}}}, filepath.Join(warmDir, "w")); err != nil {
			t.Fatal(err)
		}

		sources("1")
		first, out := build()
		if out != "11\n" || len(compiled(first)) != 3 || hits(first) != 0 {
			t.Fatalf("first build: %q, compiled %v, %d hits:\n%s", out, compiled(first), hits(first), first)
		}
		second, out := build()
		if out != "11\n" || len(compiled(second)) != 0 || hits(second) != 3 {
			t.Fatalf("second build: %q, compiled %v, %d hits (the units are in the cache):\n%s", out, compiled(second), hits(second), second)
		}
		// A function body of a: only its unit.
		time.Sleep(10 * time.Millisecond)
		sources("2")
		third, out := build()
		if out != "12\n" || strings.Join(compiled(third), " ") != unA || hits(third) != 2 {
			t.Fatalf("third build: %q, compiled %v, %d hits:\n%s", out, compiled(third), hits(third), third)
		}
		// A header of a: all the units that include it, directly or not.
		write(hdrA, "#pragma once\n#ifndef FROM_A\n#error the public flags of package a are missing\n#endif\nint a_value(); // changed\n")
		fourth, out := build()
		if out != "12\n" || len(compiled(fourth)) != 3 {
			t.Fatalf("fourth build: %q, compiled %v:\n%s", out, compiled(fourth), fourth)
		}
		// Without the public flags the header says #error: the flags really reach b's units.
		work := t.TempDir()
		b := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: work, Cache: t.TempDir(), Stderr: os.Stderr, GenRoot: gen}
		err := b.Link(context.Background(), []PackageBuild{{Path: "b", Dir: gen, Units: []string{filepath.Join(gen, unB)}}}, filepath.Join(work, "p"))
		var ce *CompileError
		if !errors.As(err, &ce) || ce.PkgPath != "b" || ce.Unit != filepath.Join(gen, unB) || !strings.Contains(string(ce.Output), "public flags") {
			t.Errorf("a unit without the public flags: %v, want a CompileError of package b with the #error", err)
		}
	})
}

// TestLinkSharedAcrossGenRoots: the same generated units in the folders of two programs (a
// standard package is the same text in every program) share their objects, whatever the folders
// are called: the key and the entry see $GEN, not the folder. A header that differs in the second
// folder is checked there, not in the first (a hit would be an object of the other header).
func TestLinkSharedAcrossGenRoots(t *testing.T) {
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
		const (
			hdr = "gpkg/s/pkg.hpp"
			un  = "gpkg/s/s.cpp"
			unM = "gpkg/s/main.cpp"
		)
		sources := func(gen, value string) {
			write(gen, hdr, "#pragma once\n#define S_VALUE "+value+"\nint s_value();\n")
			write(gen, un, "#include \"gpkg/s/pkg.hpp\"\nint s_value() { return S_VALUE; }\n")
			write(gen, unM, "#include <gocpp/rt.hpp>\n#include <cstdio>\n#include \"gpkg/s/pkg.hpp\"\nint main() { std::printf(\"%d\\n\", s_value()); }\n")
		}
		build := func(gen string) (trace, out string) {
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
		hits := func(trace string) (n int) {
			for _, u := range []string{un, unM} {
				if strings.Contains(trace, "# cache hit "+u+" ") {
					n++
				}
			}
			return
		}
		warmDir := t.TempDir()
		warm := filepath.Join(warmDir, "w.cpp")
		if err := os.WriteFile(warm, []byte("#include <gocpp/rt.hpp>\nint main() {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		wb := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: t.TempDir(), Cache: cache, Stderr: os.Stderr}
		if err := wb.Link(context.Background(), []PackageBuild{{Path: "w", Dir: warmDir, Units: []string{warm}}}, filepath.Join(warmDir, "w")); err != nil {
			t.Fatal(err)
		}

		gen1, gen2 := t.TempDir(), t.TempDir()
		sources(gen1, "7")
		sources(gen2, "7")
		first, out := build(gen1)
		if out != "7\n" || hits(first) != 0 {
			t.Fatalf("first program: %q, %d hits:\n%s", out, hits(first), first)
		}
		second, out := build(gen2)
		if out != "7\n" || hits(second) != 2 {
			t.Fatalf("second program, same units in another folder: %q, %d hits, want 2:\n%s", out, hits(second), second)
		}
		// Another header in the second folder: its units are compiled again, with its value.
		sources(gen2, "9")
		third, out := build(gen2)
		if out != "9\n" || hits(third) != 0 {
			t.Fatalf("third program, another header: %q, %d hits, want 0:\n%s", out, hits(third), third)
		}
		// The first program is still its own entry.
		fourth, out := build(gen1)
		if out != "7\n" || hits(fourth) != 2 {
			t.Fatalf("first program again: %q, %d hits, want 2:\n%s", out, hits(fourth), fourth)
		}
	})
}

func TestAppendLinkFlags(t *testing.T) {
	var link []string
	link = appendLinkFlags(link, []string{"-L/opt/x", "-lfoo", "-lm", "-pthread", "-Wl,--as-needed", "-framework", "Cocoa"})
	link = appendLinkFlags(link, []string{"-lm", "-L/opt/x", "-lbar", "-pthread", "-Wl,--as-needed", "-framework", "Cocoa", "-l"})
	want := "-L/opt/x -lfoo -lm -pthread -Wl,--as-needed -framework Cocoa -lbar -Wl,--as-needed -framework Cocoa -l"
	// A flag that names a library or a folder is added once; the others are kept as they come
	// (a pair such as -framework Cocoa stays whole).
	if got := strings.Join(link, " "); got != want {
		t.Errorf("appendLinkFlags = %q, want %q", got, want)
	}
}
