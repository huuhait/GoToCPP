package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
)

// The precompiled header of the probes' library block (pch.go, step 12.14). The tests count
// operations (headers made, runs that loaded one, runs of the includes question) and compare the
// answers with and without the header; none of them reads a clock.

// pchCase is a package whose probes and declarations need the library block and a header of its own.
func pchCase(t *testing.T, flags ...string) (Config, *collect.Set, GenConfig, DeclConfig, []DeclQuery) {
	t.Helper()
	dir := t.TempDir()
	writeHeader(t, dir, "decls.hpp", declsHeader)
	cfg := clangCacheConfig(t, dir, flags...)
	r := ref(call(pth("strlen"), lv(str())))
	set := collect.NewSet(r, ref(pth("BUFSIZ")), ref(call(pth("nothing"))))
	includes := []string{"<cstdio>", "<cstring>", `"./decls.hpp"`}
	gc := GenConfig{Includes: includes, Kinds: map[string]interop.Kind{"strlen": interop.Value, "BUFSIZ": interop.Macro}}
	dc := DeclConfig{Includes: includes}
	queries := []DeclQuery{
		{DeclType, "geo::Vec"}, {DeclType, "geo::Body"}, {DeclType, "Rect"}, {DeclType, "std::vector<int>"},
		{DeclFunc, "geo::f"}, {DeclFunc, "strlen"},
	}
	return cfg, set, gc, dc, queries
}

// answers runs the probes and the declarations of a case and returns them as the tests compare them:
// the table of the probes and the table of the declarations, with the timing and the statistics
// left out. The headers each run read (the deps) are part of the answer.
func pchAnswers(
	t *testing.T, rn *Runner, cfg Config, set *collect.Set, gc GenConfig, dc DeclConfig, qs []DeclQuery,
) (string, string) {
	t.Helper()
	tab, err := rn.Probe(context.Background(), cfg, set, gc)
	if err != nil {
		t.Fatal(err)
	}
	decls, err := rn.Decls(context.Background(), cfg, dc, qs)
	if err != nil {
		t.Fatal(err)
	}
	d := *decls
	d.Timing, d.Stats, d.Cached, d.Runs = Timing{}, Stats{}, false, 0
	b, err := json.Marshal(&d)
	if err != nil {
		t.Fatal(err)
	}
	return stable(t, tab), string(b)
}

func pchFiles(t *testing.T, cfg Config) []string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(cfg.CacheDir, "pch", "*", "*.pch"))
	return files
}

// Every run loads the header, and the answers are the ones of runs that do not.
func TestPCHAnswersAreTheSameAsWithout(t *testing.T) {
	cfg, set, gc, dc, qs := pchCase(t)
	t.Setenv(EnvPCH, "off")
	ResetCacheCounters()
	plainProbe, plainDecls := pchAnswers(t, new(Runner), cfg, set, gc, dc, qs)
	if c := CacheCounters(); c.PCHBuilds != 0 || c.PCHUses != 0 {
		t.Fatalf("with %s=off: %d headers made, %d runs loaded one", EnvPCH, c.PCHBuilds, c.PCHUses)
	}

	cfg2, _, _, _, _ := pchCase(t) // (another cache folder: nothing is answered from the first)
	cfg2.Dir = cfg.Dir
	t.Setenv(EnvPCH, "wait")
	ResetCacheCounters()
	rn := new(Runner)
	pchProbe, pchDecls := pchAnswers(t, rn, cfg2, set, gc, dc, qs)
	c := CacheCounters()
	if c.PCHBuilds != 1 {
		t.Errorf("%d headers made, want 1", c.PCHBuilds)
	}
	// The probe runs, the one run for the functions and the runs for the program's own types (geo::Vec,
	// geo::Body, Rect) load it; the run for std::vector<int> does not (TestPCHTypeRunsKeepTheMembers).
	if want := int64(rn.Runs() + 1 + 3); c.PCHUses != want || rn.DeclRuns() != 5 {
		t.Errorf("%d runs loaded the header, want %d (probes %d, declarations %d)", c.PCHUses, want, rn.Runs(), rn.DeclRuns())
	}
	if c.TypeRechecks != 0 {
		t.Errorf("%d type runs were made again without the header", c.TypeRechecks)
	}
	if c.PCHRejects != 0 {
		t.Errorf("%d runs were refused for the header", c.PCHRejects)
	}
	if got := len(pchFiles(t, cfg2)); got != 1 {
		t.Errorf("%d .pch files in the cache, want 1", got)
	}
	// The keys of the tables depend on the cache folder only through the files read, and the folders differ:
	// the answers are compared as the tests of the cache compare them.
	if pchProbe != plainProbe {
		t.Errorf("the probe table differs with the header:\nwithout: %s\nwith:    %s", plainProbe, pchProbe)
	}
	if pchDecls != plainDecls {
		t.Errorf("the declaration table differs with the header:\nwithout: %s\nwith:    %s", plainDecls, pchDecls)
	}
}

// Clang reads the members of a class that comes from a precompiled header when something uses them, and its
// JSON dump prints the members read so far: a type run with the header would answer std::mutex without its
// methods and struct tm without its fields. A type run does not load the header for a name of the library
// (std::mutex, std::basic_string_view<char>), and one that does (tm: a global name could be anyone's) is made
// again without it when its answer is a declaration of the header. The answers have all the members.
func TestPCHTypeRunsKeepTheMembers(t *testing.T) {
	dir := t.TempDir()
	cfg := clangCacheConfig(t, dir)
	includes := []string{"<mutex>", "<ctime>", "<cstdio>"}
	dc := DeclConfig{Includes: includes}
	qs := []DeclQuery{{DeclType, "std::mutex"}, {DeclType, "tm"}, {DeclType, "std::basic_string_view<char>"}}
	t.Setenv(EnvPCH, "wait")
	ResetCacheCounters()
	rn := new(Runner)
	decls, err := rn.Decls(context.Background(), cfg, dc, qs)
	if err != nil {
		t.Fatal(err)
	}
	// Only the run for tm tried the header, and its answer (a declaration of <ctime>, read from the header) was refused.
	if c := CacheCounters(); c.PCHUses != 1 || c.TypeRechecks != 1 {
		t.Errorf("%d type runs loaded the header and %d were made again, want 1 and 1", c.PCHUses, c.TypeRechecks)
	}
	if rn.DeclRuns() == 0 {
		t.Fatal("no run: the test shows nothing")
	}
	mutex := decls.Records["std::mutex"]
	if mutex == nil || len(mutex.Methods) == 0 {
		t.Errorf("std::mutex has no methods: %+v", mutex)
	}
	tm := decls.Records["tm"]
	if tm == nil || len(tm.Fields) < 9 {
		t.Errorf("struct tm has too few fields: %+v", tm)
	}
	view := decls.Records["std::basic_string_view<char>"]
	if view == nil || len(view.Methods) == 0 {
		t.Errorf("std::basic_string_view<char> has no methods: %+v", view)
	}
}

// A second process (a new Runner on the same cache folder) finds the header on disk.
func TestPCHIsMadeOnceAndShared(t *testing.T) {
	cfg, set, gc, dc, qs := pchCase(t)
	t.Setenv(EnvPCH, "wait")
	ResetCacheCounters()
	pchAnswers(t, new(Runner), cfg, set, gc, dc, qs)
	if c := CacheCounters(); c.PCHBuilds != 1 {
		t.Fatalf("first process made %d headers", c.PCHBuilds)
	}
	// Other references, so that the compiler has to run again.
	set2 := collect.NewSet(ref(call(pth("strlen"), lit(`"abc"`))), ref(pth("EOF")))
	gc2 := gc
	gc2.Kinds = map[string]interop.Kind{"strlen": interop.Value, "EOF": interop.Macro}
	ResetCacheCounters()
	rn := new(Runner)
	if _, err := rn.Probe(context.Background(), cfg, set2, gc2); err != nil {
		t.Fatal(err)
	}
	c := CacheCounters()
	if rn.Runs() == 0 {
		t.Fatal("the second process ran no compiler: the test shows nothing")
	}
	if c.PCHBuilds != 0 || c.PCHUses != int64(rn.Runs()) {
		t.Errorf("second process: %d headers made, %d runs loaded one, %d runs", c.PCHBuilds, c.PCHUses, rn.Runs())
	}
}

// Processes that start together make the header once (the lock of its key), and the others load it.
func TestPCHConcurrentProcessesMakeItOnce(t *testing.T) {
	cfg, set, gc, _, _ := pchCase(t)
	t.Setenv(EnvPCH, "wait")
	ResetCacheCounters()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := new(Runner).Probe(context.Background(), cfg, set, gc); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if c := CacheCounters(); c.PCHBuilds != 1 {
		t.Errorf("%d headers made by 4 processes, want 1", c.PCHBuilds)
	}
}

// A header that appears earlier on the search path than a library header makes the precompiled block
// stale, as it would make a result stale: it is made again, and the new header is seen.
func TestPCHIsStaleWhenALibraryHeaderIsShadowed(t *testing.T) {
	shadow := t.TempDir()
	cfg, _, gc, _, _ := pchCase(t, "-I", shadow)
	t.Setenv(EnvPCH, "wait")
	set := collect.NewSet(ref(pth("GP_SHADOW_V")))
	set.Refs[0].WantValue = true
	gc.Kinds = map[string]interop.Kind{"GP_SHADOW_V": interop.Macro}
	ResetCacheCounters()
	rn := new(Runner) // (one Runner: the header is stale for the process that made it)
	tab, err := rn.Probe(context.Background(), cfg, set, gc)
	if err != nil {
		t.Fatal(err)
	}
	if r := byKey(tab, "::GP_SHADOW_V"); r == nil || r.OK {
		t.Fatalf("before the shadow: %+v", r)
	}
	if c := CacheCounters(); c.PCHBuilds != 1 {
		t.Fatalf("%d headers made", c.PCHBuilds)
	}
	writeHeader(t, shadow, "span", "#include_next <span>\n#define GP_SHADOW_V 7\n")
	tab, err = rn.Probe(context.Background(), cfg, set, gc)
	if err != nil {
		t.Fatal(err)
	}
	r := byKey(tab, "::GP_SHADOW_V")
	if r == nil || !r.OK || r.Value == nil || r.Value.Int.Int64() != 7 {
		t.Errorf("after the shadow: %+v", r)
	}
	if c := CacheCounters(); c.PCHBuilds != 2 || c.PCHUses < 1 {
		t.Errorf("after the shadow: %d headers made in all (want 2), %d runs loaded one", c.PCHBuilds, c.PCHUses)
	}
}

// A header that the compiler refuses to load (here a file of garbage with the right name) costs one
// run without it: the answer is the right one, the file goes, and the next run makes it again.
func TestPCHRefusedByTheCompilerIsMadeAgain(t *testing.T) {
	cfg, set, gc, dc, qs := pchCase(t)
	t.Setenv(EnvPCH, "wait")
	ResetCacheCounters()
	good, goodDecls := pchAnswers(t, new(Runner), cfg, set, gc, dc, qs)
	files := pchFiles(t, cfg)
	if len(files) != 1 {
		t.Fatalf("%d headers", len(files))
	}
	info, err := os.Stat(files[0])
	if err != nil {
		t.Fatal(err)
	}
	garbage := bytes.Repeat([]byte("not a precompiled header\n"), int(info.Size())/25+1)[:info.Size()]
	if err := os.WriteFile(files[0], garbage, 0o600); err != nil { // (the size the meta file says)
		t.Fatal(err)
	}
	// Other references, so that the compiler has to run.
	set2 := collect.NewSet(ref(call(pth("strlen"), lit(`"abc"`))))
	gc2 := gc
	gc2.Kinds = map[string]interop.Kind{"strlen": interop.Value}
	ResetCacheCounters()
	tab, err := new(Runner).Probe(context.Background(), cfg, set2, gc2)
	if err != nil {
		t.Fatal(err)
	}
	if r := byKey(tab, `call(::strlen|lit("abc"))`); r == nil || !r.OK || r.Base != "unsigned long" {
		t.Errorf("the answer with a refused header: %+v", r)
	}
	if c := CacheCounters(); c.PCHRejects != 1 {
		t.Errorf("%d runs were refused, want 1", c.PCHRejects)
	}
	// The next process makes a new header, and the first case gives the first answers.
	_ = good
	_ = goodDecls
	ResetCacheCounters()
	cfg3 := cfg
	cfg3.CacheDir = filepath.Join(t.TempDir(), cacheSub) // (a folder where the pch is made again from nothing)
	again, againDecls := pchAnswers(t, new(Runner), cfg3, set, gc, dc, qs)
	if again != good || againDecls != goodDecls {
		t.Errorf("the answers changed:\n%s\n%s", good, again)
	}
}

// A header that was cut off (a copy that stopped, a full disk) is not loaded: its size is not the one the
// description says, so it is made again, and the answers are the ones of runs with no header.
func TestPCHCutOffIsMadeAgainWithTheSameAnswers(t *testing.T) {
	cfg, set, gc, dc, qs := pchCase(t)
	t.Setenv(EnvPCH, "off")
	plainProbe, plainDecls := pchAnswers(t, new(Runner), cfg, set, gc, dc, qs)
	cfg2, _, _, _, _ := pchCase(t)
	cfg2.Dir = cfg.Dir
	t.Setenv(EnvPCH, "wait")
	good, goodDecls := pchAnswers(t, new(Runner), cfg2, set, gc, dc, qs)
	if good != plainProbe || goodDecls != plainDecls {
		t.Fatalf("the answers with a header differ:\n%s\n%s", good, plainProbe)
	}
	files := pchFiles(t, cfg2)
	if len(files) != 1 {
		t.Fatalf("%d headers", len(files))
	}
	info, err := os.Stat(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(files[0], info.Size()/2); err != nil {
		t.Fatal(err)
	}
	// Other references, so that the compiler has to run.
	set2 := collect.NewSet(ref(call(pth("strlen"), lit(`"abc"`))))
	gc2 := gc
	gc2.Kinds = map[string]interop.Kind{"strlen": interop.Value}
	ResetCacheCounters()
	tab, err := new(Runner).Probe(context.Background(), cfg2, set2, gc2)
	if err != nil {
		t.Fatal(err)
	}
	if r := byKey(tab, `call(::strlen|lit("abc"))`); r == nil || !r.OK || r.Base != "unsigned long" {
		t.Errorf("the answer with a cut-off header: %+v", r)
	}
	c := CacheCounters()
	if c.PCHBuilds != 1 {
		t.Errorf("%d headers made, want 1 (the cut-off one is not used)", c.PCHBuilds)
	}
	if c.PCHRejects != 0 {
		t.Errorf("%d runs were refused by the compiler: the size should have said no first", c.PCHRejects)
	}
	// The remade header is not the same size as the first one: clang writes the working directory of
	// its run into the file, and the run is made in a temporary folder with a random name, so the
	// sizes differ by a few bytes when the names differ in length. What makes it whole is that it
	// has the size that its description says, and is not the cut-off file.
	if fi, err := os.Stat(files[0]); err != nil || fi.Size() <= info.Size()/2 {
		t.Errorf("the header was not made again: %v %v", fi, err)
	}
	if _, _, _, whole := lookPCH(strings.TrimSuffix(files[0], ".pch")); !whole {
		t.Error("the header made again is not the one its description says (size, files)")
	}
}

// Nothing is made without a cache folder, for g++, or when the environment says off.
func TestPCHNeedsNoHeaderWhenItCannotBeKept(t *testing.T) {
	cfg, _, _, _, _ := pchCase(t)
	t.Setenv(EnvPCH, "wait")
	text := "// title\n" + stdBlock() + "int x;\n"
	rn := new(Runner)
	if !hasStdBlock(text) {
		t.Fatal("hasStdBlock does not see the block")
	}
	if hasStdBlock("// title\n#include <vector>\n") || hasStdBlock("int x;") {
		t.Error("hasStdBlock takes another text for the block")
	}
	gcc := cfg
	gcc.CXX.Family = build.GCC
	if got := rn.withPCH(context.Background(), gcc, text); got.pch != nil {
		t.Error("a header for g++")
	}
	nodir := cfg
	nodir.CacheDir = "off"
	if got := rn.withPCH(context.Background(), nodir, text); got.pch != nil {
		t.Error("a header without a cache folder")
	}
	if got := rn.withPCH(context.Background(), cfg, "// title\n#include <vector>\n"); got.pch != nil {
		t.Error("a header for a text that does not start with the block")
	}
	t.Setenv(EnvPCH, "off")
	if got := rn.withPCH(context.Background(), cfg, text); got.pch != nil || len(pchFiles(t, cfg)) != 0 {
		t.Error("a header with the variable off")
	}
	t.Setenv(EnvPCH, "wait")
	if got := rn.withPCH(context.Background(), cfg, text); got.pch == nil {
		t.Error("no header for a text that starts with the block")
	}
}

// The key of the header says what changes the parse of the library: not the package folder, not the
// limits of the dump; the flags, the compiler and the block.
func TestPCHKey(t *testing.T) {
	base := clangConfig(t, "/a")
	tc := toolchain{Hash: "t1"}
	key := pchKey(base, tc)
	same := base
	same.Dir, same.MaxDecl, same.MaxJSON = "/b", 5, 6
	if pchKey(same, tc) != key {
		t.Error("the package folder or the limits are in the key of the header")
	}
	flag := base
	flag.Flags = []string{"-DX=1"}
	if pchKey(flag, tc) == key {
		t.Error("the flags are not in the key")
	}
	if pchKey(base, toolchain{Hash: "t2"}) == key {
		t.Error("the toolchain is not in the key")
	}
	other := base
	other.CXX.Version += "x"
	if pchKey(other, tc) == key {
		t.Error("the compiler version is not in the key")
	}
}

func TestPCHArguments(t *testing.T) {
	args := []string{"-std=c++20", "-w", "-MD", "-x", "c++", "/tmp/a.cpp"}
	want := "-std=c++20 -w -MD -include-pch /p.pch -x c++ /tmp/a.cpp"
	if got := strings.Join(withPCHArgs(args, "/p.pch"), " "); got != want {
		t.Errorf("withPCHArgs: %s", got)
	}
	if got := strings.Join(asHeader(args), " "); got != "-std=c++20 -w -MD -x c++-header /tmp/a.cpp" {
		t.Errorf("asHeader: %s", got)
	}
	if strings.Join(args, " ") != "-std=c++20 -w -MD -x c++ /tmp/a.cpp" {
		t.Error("the arguments were changed in place")
	}
}

func TestPCHMessage(t *testing.T) {
	for _, s := range []string{
		"fatal error: file 'x.hpp' has been modified since the precompiled header '/c/p.pch' was built: mtime changed",
		"error: malformed or corrupted AST file: 'invalid ...'",
		"fatal error: PCH file built from a different branch (clang version 99) than the compiler",
	} {
		if !pchMessage([]byte(s)) {
			t.Errorf("not recognized: %s", s)
		}
	}
	if pchMessage([]byte("error: use of undeclared identifier 'foo'")) {
		t.Error("an error of the program is taken for the header")
	}
}

// The precompiled headers kept in the cache folder are bounded.
func TestPCHPruneKeepsTheNewest(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	for i, name := range []string{"aa/1", "bb/2", "cc/3", "dd/4", "ee/5"} {
		base := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(base), 0o700); err != nil {
			t.Fatal(err)
		}
		for _, ext := range []string{".json", ".pch", ".hpp"} {
			if err := os.WriteFile(base+ext, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		when := now.Add(time.Duration(i-10) * time.Minute) // older and older towards "aa"
		os.Chtimes(base+".json", when, when)
	}
	// "aa" is the oldest; it stays because it was just made (the caller names it).
	prunePCH(root, 2, filepath.Join(root, "aa/1"))
	var left []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".pch") {
			left = append(left, strings.TrimPrefix(p, root+"/"))
		}
		return nil
	})
	if strings.Join(left, " ") != "aa/1.pch dd/4.pch ee/5.pch" {
		t.Errorf("left: %v (the newest two and the one asked to stay)", left)
	}
}

// Type runs keep the full block, including its dependencies, with acceleration enabled or disabled.
func TestTypeRunsKeepFullBlockAnswers(t *testing.T) {
	queries := []DeclQuery{
		{DeclType, "geo::Vec"}, {DeclType, "geo::Body"}, {DeclType, "geo::Kind"}, {DeclType, "Rect"}, {DeclType, "Box"},
		{DeclType, "Nest"}, {DeclType, "Opaque"}, {DeclType, "size_t"}, {DeclType, "Id"}, {DeclType, "Missing"},
	}
	full, accelerated := typeBlockAnswers(t, "decls.hpp", declsHeader, queries)
	if accelerated != full {
		t.Errorf("the declarations differ:\n%s", firstDifferenceText(full, accelerated))
	}
}

// typeBlockAnswers compares declarations with PCH off and enabled, including their dependencies.
func typeBlockAnswers(t *testing.T, name, text string, queries []DeclQuery) (string, string) {
	t.Helper()
	headers := t.TempDir()
	writeHeader(t, headers, name, text)
	table := func(mode string) string {
		t.Setenv(EnvPCH, mode)
		cfg := clangCacheConfig(t, headers)
		dc := DeclConfig{Includes: []string{`"./` + name + `"`}}
		decls, err := new(Runner).Decls(context.Background(), cfg, dc, queries)
		if err != nil {
			t.Fatal(err)
		}
		d := *decls
		d.Timing, d.Stats, d.Cached, d.Runs = Timing{}, Stats{}, false, 0
		b, err := json.Marshal(&d)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	full := table("off")
	return full, table("wait")
}

func firstDifferenceText(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return "at byte " + strconv.Itoa(i) + ":\nwithout: ..." + a[max(0, i-150):min(len(a), i+250)] +
		"\nwith:    ..." + b[max(0, i-150):min(len(b), i+250)]
}

// Feature macros, include guards and header order must have the same meaning as in the full block.
func TestTypeRunsKeepFullBlockPreprocessor(t *testing.T) {
	cases := []struct {
		name, text, query string
	}{
		{"conf.hpp", "#pragma once\n#include <vector>\n#if defined(__cpp_lib_span)\nstruct Mode { int a; int b; };\n" +
			"#else\nstruct Mode { int a; };\n#endif\ninline Mode make() { return {}; }\n", "Mode"},
		{"any.hpp", "#pragma once\n#ifdef __cpp_lib_any\nstruct Any { int a; int b; };\n" +
			"#else\nstruct Any { int a; };\n#endif\n", "Any"},
		{"endian.hpp", "#pragma once\n#ifdef EOF\nstruct Libc { int a; int b; };\n#else\nstruct Libc { int a; };\n#endif\n",
			"Libc"},
		{"debug.hpp", "#pragma once\n#define _GLIBCXX_DEBUG 1\n#include <vector>\n" +
			"struct Dbg { std::vector<int> v; int a; };\n", "Dbg"},
		{"feature.hpp", "#pragma once\n#ifdef _GLIBCXX_STRING\nstruct Str { int a; int b; };\n" +
			"#else\nstruct Str { int a; };\n#endif\n", "Str"},
		{"needs.hpp", "#pragma once\nstruct NeedsString { std::string name; int id; };\n", "NeedsString"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			full, accelerated := typeBlockAnswers(t, c.name, c.text, []DeclQuery{{DeclType, c.query}})
			if accelerated != full {
				t.Errorf("the declarations differ:\n%s", firstDifferenceText(full, accelerated))
			}
			var decls DeclTable
			if err := json.Unmarshal([]byte(accelerated), &decls); err != nil {
				t.Fatal(err)
			}
			if decls.Records[c.query] == nil {
				t.Errorf("%s was not answered: %v", c.query, decls.Errors)
			}
		})
	}
}

// Equal preprocessed header text is not equal C++ lookup: the full block supplies std::begin.
func TestTypeRunsKeepFullBlockNameLookup(t *testing.T) {
	t.Setenv(EnvPCH, "wait")
	dir := t.TempDir()
	writeHeader(t, dir, "lookup.hpp", `#pragma once
#include <type_traits>
using namespace std;
template<class T> concept HasBegin = requires(T &x) { begin(x); };
struct Mode { int a; conditional_t<HasBegin<int[2]>, long, char> b; };
inline Mode make() { return {}; }
`)
	cfg := clangCacheConfig(t, dir)
	dc := DeclConfig{Includes: []string{`"./lookup.hpp"`}}
	decls, err := new(Runner).Decls(context.Background(), cfg, dc, []DeclQuery{{DeclType, "Mode"}})
	if err != nil {
		t.Fatal(err)
	}
	rec := decls.Records["Mode"]
	if rec == nil || len(rec.Fields) != 2 {
		t.Fatalf("Mode was not answered: %+v, errors %v", rec, decls.Errors)
	}
	wantSize := int64(16)
	wantAlign := int64(8)
	if cfg.CXX.GOOS == "windows" {
		wantSize = 8
		wantAlign = 4
	}
	if rec.Size != wantSize || rec.Align != wantAlign {
		t.Errorf("Mode size/alignment = %d/%d, want %d/%d with std::begin visible",
			rec.Size, rec.Align, wantSize, wantAlign)
	}
}

// The same Runner must discard the old answer after an edit and keep normal warm-cache behavior.
func TestTypeRunsHeaderEditKeepsFullBlock(t *testing.T) {
	t.Setenv(EnvPCH, "wait")
	dir := t.TempDir()
	write := func(text string) {
		writeHeader(t, dir, "edit.hpp", text)
		old := time.Now().Add(-time.Hour)
		if err := os.Chtimes(filepath.Join(dir, "edit.hpp"), old, old); err != nil {
			t.Fatal(err)
		}
	}
	write("#pragma once\nstruct Mode { int a; };\n")
	cfg := clangCacheConfig(t, dir)
	dc := DeclConfig{Includes: []string{`"./edit.hpp"`}}
	queries := []DeclQuery{{DeclType, "Mode"}}
	rn := new(Runner)
	check := func(size int64, fields int, cached bool) {
		t.Helper()
		decls, err := rn.Decls(context.Background(), cfg, dc, queries)
		if err != nil {
			t.Fatal(err)
		}
		rec := decls.Records["Mode"]
		if rec == nil || rec.Size != size || len(rec.Fields) != fields {
			t.Fatalf("Mode = %+v, want size %d and %d fields; errors %v", rec, size, fields, decls.Errors)
		}
		if decls.Cached != cached {
			t.Errorf("cached = %v, want %v", decls.Cached, cached)
		}
	}
	check(4, 1, false)
	check(4, 1, true)
	write(`#pragma once
#if defined(__cpp_lib_span)
struct Mode { int a; int b; };
#else
struct Mode { int a; };
#endif
`)
	check(8, 2, false)
	check(8, 2, true)
	if rn.DeclRuns() != 2 {
		t.Errorf("%d runs, want exactly one per header version", rn.DeclRuns())
	}
}

// A call that makes a header does not return before the compiler that makes it is done (no orphan compiler
// outlives the call that started it), and a stopped call does not wait.
func TestWaitPCHWaitsForTheBuildsOfTheRunner(t *testing.T) {
	rn := new(Runner)
	st := &pchState{done: make(chan struct{})}
	rn.pchs = map[string]*pchState{"key": st}

	var finished atomic.Bool
	waiting := make(chan struct{})
	returned := make(chan bool)
	go func() {
		close(waiting)
		rn.WaitPCH(context.Background())
		returned <- finished.Load()
	}()
	<-waiting
	finished.Store(true) // (a WaitPCH that returns before the build is done has read false)
	close(st.done)
	if !<-returned {
		t.Error("WaitPCH returned while the build was still running")
	}

	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	other := &pchState{done: make(chan struct{})}
	other.cancel = func() { close(other.done) } // (a build that ends when it is told to)
	rn.pchs = map[string]*pchState{"other": other}
	rn.WaitPCH(stopped) // returns at once: the call was stopped
}

// The pruning of the probe cache (by age, count and size) does not take the files of a precompiled header: they
// go by their own count, and a header in use is not unused for 90 days because its description is old.
func TestPruneLeavesThePrecompiledHeaders(t *testing.T) {
	root := t.TempDir()
	old := time.Now().Add(-400 * 24 * time.Hour)
	for _, name := range []string{"pch/aa/1", "u/bb/2"} {
		base := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(base), 0o700); err != nil {
			t.Fatal(err)
		}
		for _, ext := range []string{".json", ".pch"} {
			if err := os.WriteFile(base+ext, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
			os.Chtimes(base+ext, old, old)
		}
	}
	if n := prune(root, Limits{MaxAge: 90 * 24 * time.Hour, MaxEntries: 1}, time.Now()); n != 1 {
		t.Errorf("%d entries removed, want the one result", n)
	}
	for _, name := range []string{"pch/aa/1.json", "pch/aa/1.pch"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("%s was removed by the pruning of the results", name)
		}
	}
}
