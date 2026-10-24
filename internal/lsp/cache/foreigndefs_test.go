package cache

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

const definitionsHeader = `#pragma once
// A 2D vector.
typedef struct Vector2 {
    float x;   // X component
    float y;
} Vector2;
typedef enum { RED, GREEN = 5 } Hue;
int add(int a, int b);
int declaredOnly(int a);
extern int counter;
`

const definitionsSource = `#include "geo.h"
int counter = 3;
int add(int a, int b) { return a + b; }
`

const definitionsMain = `package main
import "./geo.h"

func main() {
	_ = geo.add(1, 2)
	_ = geo.declaredOnly(1)
	_ = geo.counter
	var w geo.Vector2
	var h geo.Hue
	_, _ = w, h
}
`

func definitionsWorld(t *testing.T, files map[string]string) *world {
	t.Helper()
	return definitionsWorldWith(t, testConfig(), files)
}

func definitionsWorldWith(t *testing.T, cfg Config, files map[string]string) *world {
	t.Helper()
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	if _, err := probe.FindClang(context.Background()); err != nil {
		t.Skip(err)
	}
	t.Cleanup(probe.SetRacyWindow(0))
	files["gocpp.mod"] = "module example.com/geo\n"
	world := newWorld(t, cfg, files)
	t.Cleanup(world.p.Close)
	return world
}

func TestForeignDefinitionsAreFoundInThePackageSources(t *testing.T) {
	world := definitionsWorld(t, map[string]string{
		"main.gp": definitionsMain, "geo.h": definitionsHeader, "geo.cpp": definitionsSource,
	})
	snap, release, pkg := waitForeign(t, world)
	defer release()
	file := pkg.Check.Load.Files[0]
	source := filepath.Join(world.root, "geo.cpp")
	t.Run("a function", func(t *testing.T) {
		defs, ok := snap.ForeignDefinitions(pkg, file, "", "add")
		if !ok || len(defs) != 1 || defs[0].File != source || defs[0].Line != 2 ||
			!defs[0].Defined {
			t.Fatalf("add = %+v %v, want the definition at line 2 of geo.cpp", defs, ok)
		}
		decls, _ := snap.ForeignDeclarations(pkg, file, "", "add")
		if len(decls) != 1 || decls[0].Type != defs[0].Type || decls[0].Defined {
			t.Errorf("add declared %+v, defined %+v, want one type", decls, defs)
		}
	})
	t.Run("a variable", func(t *testing.T) {
		defs, ok := snap.ForeignDefinitions(pkg, file, "", "counter")
		if !ok || len(defs) != 1 || defs[0].File != source || defs[0].Line != 1 {
			t.Fatalf("counter = %+v %v", defs, ok)
		}
	})
	t.Run("a function that no source defines", func(t *testing.T) {
		if defs, ok := snap.ForeignDefinitions(pkg, file, "", "declaredOnly"); ok {
			t.Errorf("declaredOnly = %+v, want no definition", defs)
		}
	})
	t.Run("a type has none", func(t *testing.T) {
		if defs, ok := snap.ForeignDefinitions(pkg, file, "", "Vector2"); ok {
			t.Errorf("Vector2 = %+v", defs)
		}
	})
}

func TestForeignDefinitionsNeedASourceThatCompiles(t *testing.T) {
	world := definitionsWorld(t, map[string]string{
		"main.gp": definitionsMain, "geo.h": definitionsHeader,
		"geo.cpp": definitionsSource + "int broken( { \n",
	})
	snap, release, pkg := waitForeign(t, world)
	defer release()
	if defs, ok := snap.ForeignDefinitions(pkg, pkg.Check.Load.Files[0], "", "add"); ok {
		t.Errorf("add = %+v, want nothing from a source with an error", defs)
	}
	file := pkg.Check.Load.Files[0]
	if decls, ok := snap.ForeignDeclarations(pkg, file, "", "add"); !ok || len(decls) != 1 {
		t.Errorf("the declaration was lost with the definition: %v %v", decls, ok)
	}
}

func TestForeignDefinitionsWithoutSourcesDoNotRun(t *testing.T) {
	world := definitionsWorld(t, geoFiles())
	runs := 0
	locate := world.p.headers.locate
	world.p.headers.locate = func(ctx context.Context, cfg probe.Config, includes []string,
		request probe.LocateRequest,
	) (*probe.Locations, error) {
		runs++
		return locate(ctx, cfg, includes, request)
	}
	snap, release, pkg := waitForeign(t, world)
	defer release()
	if _, ok := snap.ForeignDefinitions(pkg, pkg.Check.Load.Files[0], "", "add"); ok {
		t.Error("a definition without a source")
	}
}

func TestForeignTypedefsHaveTheirBodies(t *testing.T) {
	world := definitionsWorld(t, geoFiles())
	snap, release, pkg := waitForeign(t, world)
	defer release()
	file := pkg.Check.Load.Files[0]
	decls, ok := snap.ForeignDeclarations(pkg, file, "", "Vector2")
	if !ok || len(decls) != 1 {
		t.Fatalf("Vector2 = %v %v", decls, ok)
	}
	want := "typedef struct Vector2 {\n    float x; // X component\n    float y;\n} Vector2;"
	if decls[0].Signature != want || decls[0].Doc != "A 2D vector." {
		t.Errorf("Vector2 = %+v, want the signature %q", decls[0], want)
	}
	hue, _ := snap.ForeignDeclarations(pkg, file, "", "Hue")
	if len(hue) != 1 || hue[0].Signature != "typedef enum {\n    RED,\n    GREEN = 5,\n} Hue;" {
		t.Errorf("Hue = %+v", hue)
	}
}

func TestForeignTypedefBodiesAreNotRequiredForTheDeclarations(t *testing.T) {
	world := definitionsWorld(t, geoFiles())
	world.p.headers.bodies = func(context.Context, probe.Config, []string, []probe.Declaration,
	) ([]probe.Declaration, error) {
		return nil, context.DeadlineExceeded
	}
	snap, release, pkg := waitForeign(t, world)
	defer release()
	decls, ok := snap.ForeignDeclarations(pkg, pkg.Check.Load.Files[0], "", "Vector2")
	if !ok || len(decls) != 1 || decls[0].Signature != "typedef struct Vector2 Vector2;" {
		t.Errorf("Vector2 = %+v %v, want the typedef as the lookup gave it", decls, ok)
	}
}

// geoFiles is the package of main.gp and geo.h, without sources.
func geoFiles() map[string]string {
	return map[string]string{"main.gp": definitionsMain, "geo.h": definitionsHeader}
}

// geoSources is the one source geo.cpp.
func geoSources() map[string]string {
	return map[string]string{"geo.cpp": definitionsSource}
}

// sourcesWorld is a package that uses the functions of geo.h; sources are the .cpp and .c files.
func sourcesWorld(t *testing.T, header, main string, sources map[string]string) *world {
	t.Helper()
	files := map[string]string{"main.gp": main, "geo.h": header}
	for name, text := range sources {
		files[name] = text
	}
	return definitionsWorld(t, files)
}

func definitionOf(t *testing.T, w *world, name string) ([]probe.Declaration, bool) {
	t.Helper()
	snap, release, pkg := waitForeign(t, w)
	defer release()
	return snap.ForeignDefinitions(pkg, pkg.Check.Load.Files[0], "", name)
}

func TestPackageSourcesAreRankedAndBounded(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 20 {
		write(fmt.Sprintf("s%02d.cpp", i), "// nothing\n")
	}
	write("geo.cpp", "int x;\n")
	write("zz.cc", "// a source that includes the header\n#include <geo.h>\n")
	write("yy.cxx", "# include \"sub/geo.h\"\n")
	write("README.txt", "")
	write(".hidden.cpp", "")
	write("_skipped.cpp", "")
	write("tail.c", "")
	includes := []string{interop.ExternC(`"./geo.h"`)}
	read, left := packageSources(dir, includes)
	var names []string
	for _, source := range read {
		names = append(names, filepath.Base(source.path))
	}
	want := []string{"geo.cpp", "yy.cxx", "zz.cc", "s00.cpp", "s01.cpp", "s02.cpp", "s03.cpp",
		"s04.cpp", "s05.cpp", "s06.cpp", "s07.cpp", "s08.cpp", "s09.cpp", "s10.cpp", "s11.cpp",
		"s12.cpp"}
	if !slices.Equal(names, want) {
		t.Errorf("read %v\nwant %v: the stem of a header first, then the sources that "+
			"include one, then the rest", names, want)
	}
	if len(read) != 16 || left != 8 {
		t.Errorf("read %d sources and left %d out, want 16 and 8 (s13 to s19 and tail.c)",
			len(read), left)
	}
	for _, source := range read {
		if source.c {
			t.Errorf("%s is read as C", source.path)
		}
	}
}

func TestPackageSourcesSkipLargeFilesWithoutTakingAPlace(t *testing.T) {
	dir := t.TempDir()
	for i := range 16 {
		name := filepath.Join(dir, fmt.Sprintf("s%02d.c", i))
		if err := os.WriteFile(name, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	big := filepath.Join(dir, "a_big.cpp")
	if err := os.WriteFile(big, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(big, 512<<10+1); err != nil {
		t.Fatal(err)
	}
	read, left := packageSources(dir, nil)
	if len(read) != 16 || left != 1 {
		t.Fatalf("read %d sources and left %d out, want 16 small ones, the large counted",
			len(read), left)
	}
	for _, source := range read {
		if !source.c || filepath.Base(source.path) == "a_big.cpp" {
			t.Errorf("source %+v: want the sixteen .c files, read as C", source)
		}
	}
	if err := os.Truncate(big, 512<<10); err != nil {
		t.Fatal(err)
	}
	if read, left = packageSources(dir, nil); len(read) != 16 || left != 1 || read[0].c {
		t.Errorf("a source of exactly 512 KiB: read %d (first %+v), left %d, "+
			"want it first and one of 17 out", len(read), read[0], left)
	}
}

const manySourcesHeader = `#pragma once
int add(int a, int b);
int tail(int a);
`

const manySourcesMain = `package main
import "./geo.h"

func main() {
	_ = geo.add(1, 2)
	_ = geo.tail(1)
}
`

func TestForeignDefinitionsReadMoreThanEightSources(t *testing.T) {
	sources := map[string]string{"geo.cpp": definitionsSource}
	for i := range 8 {
		sources[fmt.Sprintf("a%d.cpp", i)] = "#include \"geo.h\"\n"
	}
	sources["zlast.cpp"] = "#include \"geo.h\"\nint tail(int a) { return a; }\n"
	world := sourcesWorld(t, manySourcesHeader, manySourcesMain, sources)
	defs, ok := definitionOf(t, world, "tail")
	if !ok || len(defs) != 1 || filepath.Base(defs[0].File) != "zlast.cpp" {
		t.Errorf("tail = %+v %v, want the definition in the tenth source", defs, ok)
	}
}

func TestForeignSourcesLeftOutAreCounted(t *testing.T) {
	sources := map[string]string{
		"geo.cpp": "#include \"geo.h\"\nint add(int a, int b) { return a + b; }\n",
		"huge.cpp": "#include \"geo.h\"\nint tail(int a) { return a; }\n" +
			strings.Repeat("//\n", 200<<10),
	}
	for i := range 17 {
		sources[fmt.Sprintf("a%02d.cpp", i)] = "#include \"geo.h\"\n"
	}
	sources["wrapped.cpp"] = "#include \"wrapper.h\"\nint tail2(int a) { return a; }\n"
	sources["wrapper.h"] = "#include \"geo.h\"\n"
	world := sourcesWorld(t, manySourcesHeader, manySourcesMain, sources)
	snap, release, pkg := waitForeign(t, world)
	defer release()
	left, limit := snap.ForeignSourcesLeftOut(pkg, pkg.Check.Load.Files[0])
	// 19 sources can be read, in this order: geo.cpp (the stem of the header), 17 that include
	// it, wrapped.cpp (which includes it through another header). 16 get a place, so the last two
	// of the 17 and wrapped.cpp are left out, and the huge source is over the size limit.
	if left != 4 || limit != 16 {
		t.Errorf("left out = %d (limit %d), want 4 and 16", left, limit)
	}
	if defs, ok := snap.ForeignDefinitions(pkg, pkg.Check.Load.Files[0], "", "tail"); ok {
		t.Errorf("tail = %+v, want nothing from a source over the limit", defs)
	}
	defs, ok := snap.ForeignDefinitions(pkg, pkg.Check.Load.Files[0], "", "add")
	if !ok || len(defs) != 1 {
		t.Errorf("add = %+v %v, want the definition of the source with the header's stem", defs, ok)
	}
}

const linkageHeader = `#pragma once
int ovr(int);
int lone(int);
int real(int);
extern int table[];
int sp(int);
int sp2(int);
int g2(int);
int w(int);
`

const linkageSource = `#include "geo.h"
static int ovr(char c) { return c; }
namespace { int lone(double d) { return 1; } }
int real(int x) { return x; }
int table[3] = {1, 2, 3};
static int sp(char c);
static int sp(char c) { return c; }
namespace { int sp2(double); int sp2(double d) { return 1; } }
int g2(int a) { return a; }
int g2(int a);
static int w(char c) { return c; }
int w(int a);
`

const linkageMain = `package main
import "./geo.h"

func main() {
	_ = geo.ovr(1)
	_ = geo.lone(1)
	_ = geo.real(1)
	_ = geo.table
	_ = geo.sp(1)
	_ = geo.sp2(1)
	_ = geo.g2(1)
	_ = geo.w(1)
}
`

func TestForeignDefinitionsIgnoreFunctionsWithInternalLinkage(t *testing.T) {
	world := sourcesWorld(t, linkageHeader, linkageMain,
		map[string]string{"geo.cpp": linkageSource})
	snap, release, pkg := waitForeign(t, world)
	defer release()
	file := pkg.Check.Load.Files[0]
	// (w: the lookup shows the redeclaration of the header's function in the source, and the chain
	// has the static function that is defined there)
	for _, name := range []string{"ovr", "lone", "w"} {
		if defs, ok := snap.ForeignDefinitions(pkg, file, "", name); ok {
			t.Errorf("%s = %+v, want no definition: the source's function is another function",
				name, defs)
		}
	}
	for name, line := range map[string]int{"real": 3, "table": 4, "g2": 8} {
		defs, ok := snap.ForeignDefinitions(pkg, file, "", name)
		if !ok || len(defs) != 1 || defs[0].Line != line || !defs[0].Defined {
			t.Errorf("%s = %+v %v, want the definition at line %d", name, defs, ok, line)
		}
	}
}

// A function that the source declares first and then defines is a function of its own when it is
// static: it does not stand for the header's declaration of the name. In an unnamed namespace it
// is told from the header's function by its parameters, which the pairing compares.
func TestForeignDefinitionsAfterAPrototypeInTheSource(t *testing.T) {
	world := sourcesWorld(t, linkageHeader, linkageMain,
		map[string]string{"geo.cpp": linkageSource})
	snap, release, pkg := waitForeign(t, world)
	defer release()
	file := pkg.Check.Load.Files[0]
	if defs, ok := snap.ForeignDefinitions(pkg, file, "", "sp"); ok {
		t.Errorf("static sp = %+v, want no definition: it has a previous declaration of its own",
			defs)
	}
	headers, _ := snap.ForeignDeclarations(pkg, file, "", "sp2")
	defs, _ := snap.ForeignDefinitions(pkg, file, "", "sp2")
	if len(headers) != 1 || headers[0].Params != "int" {
		t.Fatalf("sp2 in the header = %+v", headers)
	}
	for _, def := range defs {
		if def.Params == headers[0].Params {
			t.Errorf("sp2 definition %+v has the parameters of the header's function", def)
		}
	}
}

func TestForeignDefinitionsBeforeARedeclarationInTheSameSource(t *testing.T) {
	var chains atomic.Int32
	world := sourcesWorld(t, linkageHeader, linkageMain,
		map[string]string{"geo.cpp": linkageSource})
	chain := world.p.headers.chain
	world.p.headers.chain = func(ctx context.Context, cfg probe.Config, includes []string,
		scope, name string,
	) ([]probe.Declaration, error) {
		chains.Add(1)
		return chain(ctx, cfg, includes, scope, name)
	}
	defs, ok := definitionOf(t, world, "g2")
	if !ok || len(defs) != 1 || !defs[0].Defined || defs[0].Line != 8 {
		t.Fatalf("g2 = %+v %v, want the definition at line 8, not the redeclaration at line 9",
			defs, ok)
	}
	if chains.Load() == 0 {
		t.Error("the chain of g2 was not asked: the lookup shows the redeclaration only")
	}
	t.Run("a chain that fails fails the source, which is asked again", func(t *testing.T) {
		world := sourcesWorld(t, linkageHeader, linkageMain,
			map[string]string{"geo.cpp": linkageSource})
		world.p.headers.retry = 20 * time.Millisecond
		var failures atomic.Int32
		chain := world.p.headers.chain
		world.p.headers.chain = func(ctx context.Context, cfg probe.Config, includes []string,
			scope, name string,
		) ([]probe.Declaration, error) {
			if failures.Add(1) == 1 {
				return nil, errors.New("fails")
			}
			return chain(ctx, cfg, includes, scope, name)
		}
		defs, ok := definitionOf(t, world, "g2")
		if !ok || len(defs) != 1 || defs[0].Line != 8 {
			t.Errorf("g2 = %+v %v after a failed chain, want the definition at the retry", defs, ok)
		}
	})
}

func TestForeignDefinitionsKeepTheOverloadOfTheirType(t *testing.T) {
	header := "#pragma once\nint pick(int);\nint pick(double);\n" +
		"unsigned long size(unsigned long);\n"
	source := "#include \"geo.h\"\nint pick(double d) { return 2; }\n" +
		"unsigned long size(unsigned long n) { return n; }\n"
	main := "package main\nimport \"./geo.h\"\n\n" +
		"func main() {\n\t_ = geo.pick(1)\n\t_ = geo.size(1)\n}\n"
	world := sourcesWorld(t, header, main, map[string]string{"geo.cpp": source})
	snap, release, pkg := waitForeign(t, world)
	defer release()
	file := pkg.Check.Load.Files[0]
	defs, ok := snap.ForeignDefinitions(pkg, file, "", "pick")
	decls, _ := snap.ForeignDeclarations(pkg, file, "", "pick")
	if !ok || len(defs) != 1 || len(decls) != 2 || defs[0].Type != "int (double)" {
		t.Fatalf("pick: definitions %+v, declarations %+v, want the double overload only",
			defs, decls)
	}
	if defs, ok := snap.ForeignDefinitions(pkg, file, "", "size"); !ok || len(defs) != 1 {
		t.Errorf("size = %+v %v, want the definition", defs, ok)
	}
}

func TestForeignDefinitionsInAFileTheSourceIncludes(t *testing.T) {
	sources := map[string]string{
		"geo.cpp":  "#include \"geo.h\"\n#include \"impl.inc\"\n",
		"impl.inc": "int add(int a, int b) { return a + b; }\n",
	}
	world := sourcesWorld(t, manySourcesHeader, manySourcesMain, sources)
	defs, ok := definitionOf(t, world, "add")
	if !ok || len(defs) != 1 || filepath.Base(defs[0].File) != "impl.inc" {
		t.Errorf("add = %+v %v, want the definition in impl.inc", defs, ok)
	}
}

// waitExternC waits for the background check of the C compiler to find the header a C header.
func waitExternC(t *testing.T, w *world) {
	t.Helper()
	for deadline := time.Now().Add(hangLimit); time.Now().Before(deadline); {
		pkg := w.pkg(".")
		w.p.prober.wait()
		if len(pkg.Check.ExternC) > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the header was not found to be a C header")
}

func TestForeignDefinitionsReadCSourcesAsC(t *testing.T) {
	// A header that the C compiler also parses is put in extern "C" by the loader; one with a C++
	// member initializer is a C++ header.
	cHeader := "#pragma once\nint cadd(int a, int b);\n"
	cxxHeader := cHeader + "struct Plus { int v = 0; };\n"
	cSource := "#include \"geo.h\"\nint cadd(int a, int b) { return a + b; }\n"
	cMain := "package main\nimport \"./geo.h\"\n\nfunc main() { _ = geo.cadd(1, 2) }\n"
	t.Run("a C header is defined by the C source", func(t *testing.T) {
		cfg := testConfig()
		cfg.CXX = func(ctx context.Context) (build.CXX, error) { return probe.FindClang(ctx) }
		cfg.CHeader = func(context.Context, build.CXX, string, []string, string,
		) (interop.CHeader, error) {
			return interop.CHeader{C: true}, nil
		}
		world := definitionsWorldWith(t, cfg, map[string]string{
			"main.gp": cMain, "geo.h": cHeader, "geo.c": cSource})
		waitExternC(t, world)
		defs, ok := definitionOf(t, world, "cadd")
		if !ok || len(defs) != 1 || filepath.Base(defs[0].File) != "geo.c" {
			t.Errorf("cadd = %+v %v, want the definition in geo.c", defs, ok)
		}
	})
	t.Run("a C source has C linkage, so a C++ header is not what it defines", func(t *testing.T) {
		world := sourcesWorld(t, cxxHeader, cMain, map[string]string{"geo.c": cSource})
		if defs, ok := definitionOf(t, world, "cadd"); ok {
			t.Errorf("cadd = %+v, want nothing: geo.c defines a C function, the header a C++ one",
				defs)
		}
	})
	t.Run("the same text in a C++ source is the definition", func(t *testing.T) {
		world := sourcesWorld(t, cxxHeader, cMain, map[string]string{"geo.cpp": cSource})
		if defs, ok := definitionOf(t, world, "cadd"); !ok || len(defs) != 1 {
			t.Errorf("cadd = %+v %v, want the definition", defs, ok)
		}
	})
}

func TestForeignDefinitionsRetryASourceThatFailedOnce(t *testing.T) {
	world := sourcesWorld(t, manySourcesHeader, manySourcesMain, geoSources())
	world.p.headers.retry = 20 * time.Millisecond
	var failures atomic.Int32
	locate := world.p.headers.locate
	world.p.headers.locate = func(ctx context.Context, cfg probe.Config, includes []string,
		request probe.LocateRequest,
	) (*probe.Locations, error) {
		if strings.Contains(includes[len(includes)-1], "geo.cpp") && failures.Add(1) == 1 {
			return nil, errors.New("locate inputs changed or could not be validated")
		}
		return locate(ctx, cfg, includes, request)
	}
	defs, ok := definitionOf(t, world, "add")
	if !ok || len(defs) != 1 || failures.Load() != 2 {
		t.Errorf("add = %+v %v after %d runs of the source, want the definition at the second",
			defs, ok, failures.Load())
	}
}

func TestForeignDefinitionsOfASourceThatKeepsFailingAreGivenUpOnce(t *testing.T) {
	sources := geoSources()
	sources["bad.cpp"] = "#include \"geo.h\"\n"
	world := sourcesWorld(t, manySourcesHeader, manySourcesMain, sources)
	world.p.headers.retry = 10 * time.Millisecond
	var good, bad atomic.Int32
	locate := world.p.headers.locate
	world.p.headers.locate = func(ctx context.Context, cfg probe.Config, includes []string,
		request probe.LocateRequest,
	) (*probe.Locations, error) {
		last := includes[len(includes)-1]
		switch {
		case strings.Contains(last, "bad.cpp"):
			bad.Add(1)
			return nil, errors.New("fails")
		case strings.Contains(last, "geo.cpp"):
			good.Add(1)
		}
		return locate(ctx, cfg, includes, request)
	}
	defs, ok := definitionOf(t, world, "add")
	if !ok || len(defs) != 1 || bad.Load() != 2 || good.Load() != 1 {
		t.Errorf("add = %+v %v after %d runs of the failing source and %d of the other, "+
			"want one retry of the failing one only and the definition of the other",
			defs, ok, bad.Load(), good.Load())
	}
}

// swapSlot makes the entry of the slot another one, as a refresh does, without canceling the
// old one: the facts that the workers have for the old entry are not the slot's any more.
func swapSlot(ix *headerIndexer, from *headerEntry) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	for key, entry := range ix.entries {
		if entry == from {
			ix.entries[key] = &headerEntry{ctx: context.Background(), cancel: func() {}}
		}
	}
}

func TestForeignDefinitionsOfAReplacedSlotAreNotPublished(t *testing.T) {
	world := sourcesWorld(t, manySourcesHeader, manySourcesMain, geoSources())
	ix := world.p.headers
	var old *headerEntry
	locate := ix.locate
	ix.locate = func(ctx context.Context, cfg probe.Config, includes []string,
		request probe.LocateRequest,
	) (*probe.Locations, error) {
		if strings.Contains(includes[len(includes)-1], "geo.cpp") {
			ix.mu.Lock()
			for _, entry := range ix.entries {
				old = entry
			}
			ix.mu.Unlock()
			swapSlot(ix, old)
		}
		return locate(ctx, cfg, includes, request)
	}
	waitForeign(t, world)
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if old == nil || old.defs != nil {
		t.Errorf("the replaced entry has definitions %+v (entry %v)", old.defs, old != nil)
	}
}

// assertPlainTypedef checks that Vector2 still has the signature of the lookup, with no body.
func assertPlainTypedef(t *testing.T, decls map[string][]probe.Declaration, what string) {
	t.Helper()
	list := decls[foreignKey("", "Vector2")]
	if len(list) != 1 || list[0].Signature != "typedef struct Vector2 Vector2;" {
		t.Errorf("%s: Vector2 = %+v, want the plain typedef: a body was published over other facts",
			what, list)
	}
}

func TestForeignTypedefBodiesOfAReplacedSlotAreNotPublished(t *testing.T) {
	world := definitionsWorld(t, geoFiles())
	ix := world.p.headers
	var old *headerEntry
	bodies := ix.bodies
	ix.bodies = func(ctx context.Context, cfg probe.Config, includes []string,
		decls []probe.Declaration,
	) ([]probe.Declaration, error) {
		ix.mu.Lock()
		for _, entry := range ix.entries {
			old = entry
		}
		ix.mu.Unlock()
		swapSlot(ix, old)
		return bodies(ctx, cfg, includes, decls)
	}
	waitForeign(t, world)
	ix.mu.Lock()
	defer ix.mu.Unlock()
	assertPlainTypedef(t, old.decls, "the replaced entry")
}

func TestForeignTypedefBodiesOfChangedDeclarationsAreNotPublished(t *testing.T) {
	world := definitionsWorld(t, geoFiles())
	ix := world.p.headers
	var entry *headerEntry
	bodies := ix.bodies
	ix.bodies = func(ctx context.Context, cfg probe.Config, includes []string,
		decls []probe.Declaration,
	) ([]probe.Declaration, error) {
		filled, err := bodies(ctx, cfg, includes, decls)
		// The slot's declarations are other ones while the compiler ran: same names, same count,
		// the typedefs in other files.
		ix.mu.Lock()
		for _, e := range ix.entries {
			entry = e
		}
		moved := make(map[string][]probe.Declaration)
		for name, list := range entry.decls {
			moved[name] = slices.Clone(list)
			for i := range moved[name] {
				moved[name][i].File = "/elsewhere/" + filepath.Base(moved[name][i].File)
			}
		}
		entry.decls = moved
		ix.mu.Unlock()
		return filled, err
	}
	waitForeign(t, world)
	ix.mu.Lock()
	defer ix.mu.Unlock()
	assertPlainTypedef(t, entry.decls, "the changed declarations")
}

func TestForeignTypedefBodiesOfMoreDeclarationsAreNotPublished(t *testing.T) {
	world := definitionsWorld(t, geoFiles())
	ix := world.p.headers
	var entry *headerEntry
	bodies := ix.bodies
	ix.bodies = func(ctx context.Context, cfg probe.Config, includes []string,
		decls []probe.Declaration,
	) ([]probe.Declaration, error) {
		filled, err := bodies(ctx, cfg, includes, decls)
		// The slot got one more name while the compiler ran.
		ix.mu.Lock()
		for _, e := range ix.entries {
			entry = e
		}
		more := maps.Clone(entry.decls)
		more["Extra"] = []probe.Declaration{{Kind: "function", File: "/inc/extra.h"}}
		entry.decls = more
		ix.mu.Unlock()
		return filled, err
	}
	waitForeign(t, world)
	ix.mu.Lock()
	defer ix.mu.Unlock()
	assertPlainTypedef(t, entry.decls, "the changed declarations")
}

func TestForeignSourcesLeftOutAreCountedWithoutDefinitions(t *testing.T) {
	huge := "#include \"geo.h\"\nint tail(int a) { return a; }\n" + strings.Repeat("//\n", 200<<10)
	world := sourcesWorld(t, manySourcesHeader, manySourcesMain,
		map[string]string{"huge.cpp": huge})
	snap, release, pkg := waitForeign(t, world)
	defer release()
	left, _ := snap.ForeignSourcesLeftOut(pkg, pkg.Check.Load.Files[0])
	if left != 1 {
		t.Errorf("left out = %d, want the one source over the limit, though none gave a definition",
			left)
	}
}
