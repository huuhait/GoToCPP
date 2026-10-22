package probe

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/collect"
)

// Tests added after the review of step 12.7: what the cache must notice when a header is missing
// in another header, hidden by an earlier folder, or edited while the compiler runs; and the
// rest of what the review found untested.

func libGC(names ...string) GenConfig {
	return GenConfig{Includes: []string{`"lib.hpp"`}, Kinds: kindsOf(names...)}
}

// A header that is missing inside another header is a cached negative answer, and the places where it
// would be are watched, for the probes and for the declarations.
func TestCacheNestedMissingHeader(t *testing.T) {
	f := newFake(t)
	f.header(t, "lib.hpp", "#include \"inner.hpp\"\n")
	names := []string{"f0"}
	set := fnSet(names...)
	probe := func() *Table {
		tab, err := new(Runner).Probe(context.Background(), f.cfg(), set, libGC(names...))
		if err != nil {
			t.Fatal(err)
		}
		return tab
	}
	tab := probe()
	if tab.Results[0].OK || len(tab.HeaderErrors) == 0 || !strings.Contains(tab.HeaderErrors[0].Msg, "'inner.hpp' file not found") {
		t.Fatalf("%+v %+v", tab.Results[0], tab.HeaderErrors)
	}
	if again := probe(); !again.Cached || f.runs() != 1 {
		t.Fatalf("the negative answer is not cached: %d runs", f.runs())
	}
	// The watched places: next to the header that includes it, the package folder, the compiler's folders.
	has := func(p string) bool {
		for _, d := range tab.Deps {
			if d == p {
				return true
			}
		}
		return false
	}
	if !has(filepath.Join(f.inc, "inner.hpp")) || !has(filepath.Join(f.dir, "inner.hpp")) || !has(filepath.Join(os.Getenv("FAKE_SYS"), "inner.hpp")) {
		t.Errorf("deps %v", tab.Deps)
	}
	f.header(t, "inner.hpp", "// now there\n")
	tab = probe()
	if f.runs() != 2 || !tab.Results[0].OK || len(tab.HeaderErrors) != 0 {
		t.Errorf("after the header appeared: %d runs, %+v %+v", f.runs(), tab.Results[0], tab.HeaderErrors)
	}
	if again := probe(); !again.Cached || f.runs() != 2 {
		t.Errorf("not cached after: %d runs", f.runs())
	}
}

func TestCacheNestedMissingHeaderInDeclarations(t *testing.T) {
	f := newFake(t)
	f.header(t, "lib.hpp", "#include \"inner.hpp\"\n")
	dc := DeclConfig{Includes: []string{`"lib.hpp"`}}
	q := []DeclQuery{{DeclType, "A"}}
	decls := func() *DeclTable {
		tab, err := new(Runner).Decls(context.Background(), f.cfg(), dc, q)
		if err != nil {
			t.Fatal(err)
		}
		return tab
	}
	decls()
	if tab := decls(); !tab.Cached || f.runs() != 1 {
		t.Fatalf("not cached: %d runs", f.runs())
	}
	f.header(t, "inner.hpp", "// now there\n")
	if tab := decls(); f.runs() != 2 || tab.Cached {
		t.Errorf("after the header appeared: %d runs", f.runs())
	}
}

// A header that appears in a folder before the one where an included header was found hides it.
func TestCacheHiddenHeader(t *testing.T) {
	f := newFake(t)
	d1, d2 := t.TempDir(), f.inc
	t.Setenv("FAKE_SYS", d1+"|"+d2) // the compiler's folders, in order: d1 first
	f.header(t, "lib.hpp", "//\n")  // found in d2
	names := []string{"f0"}
	set := fnSet(names...)
	probe := func() {
		if _, err := new(Runner).Probe(context.Background(), f.cfg(), set, libGC(names...)); err != nil {
			t.Fatal(err)
		}
	}
	probe()
	probe()
	if f.runs() != 1 {
		t.Fatalf("%d runs", f.runs())
	}
	p := filepath.Join(d1, "lib.hpp")
	os.WriteFile(p, []byte("// hides the other\n"), 0o644)
	probe()
	if f.runs() != 2 {
		t.Errorf("%d runs: a header that hides the one that was found was not noticed", f.runs())
	}
}

func TestEarlierShadows(t *testing.T) {
	root := t.TempDir()
	abs := func(s string) string { return filepath.Join(root, s) }
	dirs := []string{abs("a"), abs("b"), abs("b/c"), abs("d")}
	deps := []string{abs("b/c/h.h"), abs("d/sub/g.h"), abs("a/f.h"), abs("elsewhere/e.h")}
	check := func(user []string, want ...string) {
		t.Helper()
		got := earlierShadows(deps, dirs, user)
		w := map[string]bool{}
		for _, p := range want {
			w[abs(p)] = true
		}
		if len(got) != len(w) {
			t.Fatalf("added %v", got)
		}
		for _, p := range got {
			if !w[p] {
				t.Errorf("unexpected %s", p)
			}
		}
	}
	// h.h was found in b/c (the longest folder): a and b come first; g.h in d: a, b and b/c.
	check(nil, "a/h.h", "b/h.h", "a/sub/g.h", "b/sub/g.h", "b/c/sub/g.h")
	// The folders that the flags name are all watched, whatever their order.
	// (a folder of the flags is not cleaned: a ".." after a link is the parent of its target)
	raw := abs("u") + filepath.FromSlash("/x/..")
	got := earlierShadows([]string{abs("d/h.h")}, []string{abs("d")}, []string{raw})
	if len(got) != 1 || got[0] != joinRaw(raw, "h.h") {
		t.Errorf("raw folder: %v", got)
	}
	check([]string{abs("u")}, "a/h.h", "b/h.h", "u/h.h", "a/sub/g.h", "b/sub/g.h", "b/c/sub/g.h", "u/sub/g.h", "u/f.h")
}

// The order of the search folders is the flags', the environment's and then the compiler's, and keeps
// a folder that does not exist (clang's own list leaves it out).
func TestSearchOrder(t *testing.T) {
	root := t.TempDir()
	abs := func(s string) string { return filepath.Join(root, s) }
	t.Setenv("CPATH", abs("p1")+string(os.PathListSeparator)+abs("p2"))
	t.Setenv("CPLUS_INCLUDE_PATH", abs("p3")+filepath.FromSlash("/x/../p4")) // (not cleaned: x may be a link)
	cfg := Config{Dir: root, Flags: []string{"-I" + abs("i1"), "-isystem", abs("s1"), "-iquote", abs("q1"), "-I", "rel", "-isystem" + abs("s2"), "-idirafter", abs("a1"), "-idirafter" + abs("a2"), "-iquote" + abs("q2")}}
	order, user := searchOrder(cfg, []string{abs("s1"), abs("sys")})
	want := []string{root, abs("q1"), abs("q2"), abs("i1"), abs("s1"), abs("rel"), abs("s2"), abs("a1"), abs("a2"), abs("p1"), abs("p2"), abs("p3") + filepath.FromSlash("/x/../p4"), abs("sys")}
	if !slices.Equal(order, want) {
		t.Errorf("order %v\nwant  %v", order, want)
	}
	if len(user) != len(want)-1 || user[0] != root {
		t.Errorf("user %v", user)
	}
}

// The scanner of `clang -E -dI`: the directives with the file that has each, the files entered, and the
// signs that the output is only a part. The output is written in pieces of every size.
func TestDirectiveScanner(t *testing.T) {
	root := t.TempDir()
	abs := func(s string) string { return filepath.Join(root, s) }
	src := abs("tmp/gp_probe.cpp")
	out := "# 1 \"" + markerPath(src) + "\"\n" +
		"# 1 \"<built-in>\" 1\n" +
		"#include \"leaf.h\" /* clang -E -dI */\n" +
		"# 1 \"<built-in>\"\n" +
		"# 1 \"inc/leaf.h\" 1\n" +
		"int a_very_long_line_of_text " + strings.Repeat("x", 3<<20) + "\n" +
		"# 2 \"" + markerPath(src) + "\" 2\n" +
		"#include <a/b.h> /* clang -E -dI */\n" +
		"# 1 \"inc/a/b.h\" 1\n" +
		"#include \"../foo.h\" /* clang -E -dI */\r\n" +
		"# 1 \"inc/a/b.h\"\n" +
		"# 1 \"inc/x/../foo.h\" 1\n" +
		"# 2 \"inc/a/b.h\" 2\n" +
		"#include \"caf.h\" /* clang -E -dI */\n" +
		"# 3 \"inc/caf\\303\\251 \\\"q\\\".h\" 1\n" +
		"# 2 \"inc/a/b.h\" 2\n" +
		"#import \"foo.h\" /* clang -E -dI */\n" +
		"# 9 \"inc/a/b.h\"\n" +
		"#include_next <b.h> /* clang -E -dI */\n" +
		"#include \"nothere.h\" /* clang -E -dI */\n" +
		"int x = 1; #include \"not a directive\"\n" +
		"#pragma once\n"
	want := []directive{
		{"", true, "leaf.h", true},
		{"", false, "a/b.h", true},
		{abs("inc/a/b.h"), true, "../foo.h", true},
		{abs("inc/a/b.h"), true, "caf.h", true},
		{abs("inc/a/b.h"), true, "foo.h", false}, // (skipped: no marker)
		{abs("inc/a/b.h"), false, "b.h", false},
		{abs("inc/a/b.h"), true, "nothere.h", false},
	}
	// (a path is kept as the compiler wrote it, a ".." included)
	wantEntered := []string{abs("inc/leaf.h"), abs("inc/a/b.h"), joinRaw(root, "inc/x/../foo.h"), abs("inc/caf\u00e9 \"q\".h")}
	for _, piece := range []int{len(out), 4096, 7, 1} {
		sc := newDirectiveScanner(root, src)
		for i := 0; i < len(out); i += piece {
			sc.Write([]byte(out[i:min(len(out), i+piece)]))
		}
		if !samePaths(sc.info.directives, want) {
			t.Errorf("pieces of %d: directives %+v", piece, sc.info.directives)
		}
		if !slices.Equal(slashes(sc.info.entered), slashes(wantEntered)) {
			t.Errorf("pieces of %d: entered %q\nwant %q", piece, sc.info.entered, wantEntered)
		}
		if sc.overlong || sc.unterminated() {
			t.Errorf("pieces of %d: overlong %v unterminated %v for a complete output", piece, sc.overlong, sc.unterminated())
		}
	}
	// An output that ends in the middle of a line (the compiler was stopped) and a directive line that is
	// longer than anything real are not complete.
	sc := newDirectiveScanner(root, src)
	sc.Write([]byte("# 1 \"" + markerPath(src) + "\"\n#include \"a.h\" /* clang -E -dI */\n# 1 \"inc/a"))
	if !sc.unterminated() || len(sc.info.directives) != 1 {
		t.Errorf("a cut output: unterminated %v, %+v", sc.unterminated(), sc.info.directives)
	}
	sc = newDirectiveScanner(root, src)
	sc.Write([]byte("#include \"" + strings.Repeat("a", 2<<20) + "\" /* clang -E -dI */\n#include \"b.h\"\n"))
	if !sc.overlong || len(sc.info.directives) != 1 || sc.info.directives[0].name != "b.h" {
		t.Errorf("an overlong directive: overlong %v, %+v", sc.overlong, sc.info.directives)
	}
}

func slashes(ps []string) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = filepath.ToSlash(p)
	}
	return out
}

func samePaths(got, want []directive) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		g, w := got[i], want[i]
		g.includer, w.includer = filepath.ToSlash(g.includer), filepath.ToSlash(w.includer)
		if g != w {
			return false
		}
	}
	return true
}

// The folder of the file that has an include is watched for the name as it was spelled, `..` and all,
// and the folders of the flags are watched for every name of a file that is not the compiler's own.
func TestIncluderShadows(t *testing.T) {
	root := t.TempDir()
	abs := func(s string) string { return filepath.Join(root, s) }
	order := []string{abs("pkg"), abs("inc"), abs("leafdir"), abs("sys")}
	user := order[:3]
	ds := []directive{
		{"", true, "sub/lib.hpp", true},                              // the probe file: the package folder
		{abs("inc/a/b/lib.hpp"), true, "../foo.h", true},             // inc/a/foo.h, not inc/a/b/foo.h
		{abs("inc/a/b/lib.hpp"), false, "<angle>.h", true},           // an angle include: the folders of the flags only
		{abs("inc/a/b/lib.hpp"), true, abs("elsewhere/abs.h"), true}, // an absolute name has no other place
		{abs("sys/vector"), true, "from_system.h", true},             // an include of a compiler's header is not watched
	}
	got := slashes(includerShadows(ds, abs("pkg"), order, user)) // (the separators of a joined name are its own)
	for _, p := range slashes([]string{
		abs("pkg/sub/lib.hpp"), abs("inc/sub/lib.hpp"), abs("leafdir/sub/lib.hpp"),
		joinRaw(abs("inc/a/b"), "../foo.h"), joinRaw(abs("pkg"), "../foo.h"), joinRaw(abs("inc"), "../foo.h"), // (../foo.h from the includer and from each folder of the flags: not cleaned)
		abs("inc/<angle>.h"), abs("pkg/<angle>.h"),
	}) {
		if !slices.Contains(got, p) {
			t.Errorf("not watched: %s", p)
		}
	}
	for _, p := range got {
		if strings.Contains(p, "from_system") || strings.Contains(p, "abs.h") || p == filepath.ToSlash(abs("inc/a/b/foo.h")) || p == filepath.ToSlash(abs("inc/a/b/<angle>.h")) {
			t.Errorf("watched: %s", p)
		}
	}
	// Without the includes: every folder of a header that was read is watched for every name.
	guess := slashes(guessIncluderShadows([]string{abs("inc/sub/lib.hpp"), abs("leafdir/leaf.h"), abs("sys/vector")}, abs("pkg"), order, user))
	for _, p := range slashes([]string{abs("inc/sub/leaf.h"), abs("pkg/leaf.h"), abs("leafdir/lib.hpp"), abs("pkg/sub/lib.hpp")}) {
		if !slices.Contains(guess, p) {
			t.Errorf("guess lacks %s: %v", p, guess)
		}
	}
	for _, p := range guess {
		if strings.Contains(p, "vector") || strings.HasPrefix(p, filepath.ToSlash(abs("sys"))) {
			t.Errorf("guess watches the compiler's own: %s", p)
		}
	}
}

// The includes of a preamble are asked once for as long as its headers have the same contents (each batch of
// units and each declaration run of a preamble asks); not for a text with no header; and a compiler that is
// stopped in the middle of its answer gets the conservative one: the package folder is watched for the name of
// a header that another header includes.
func TestCacheIncludeInfoQuestion(t *testing.T) {
	t.Run("asked_once", func(t *testing.T) {
		f := newFake(t)
		f.header(t, "lib.hpp", "#include \"inner.hpp\"\n")
		f.header(t, "inner.hpp", "// inner\n")
		rn := new(Runner)
		read := []string{filepath.Join(f.inc, "lib.hpp"), filepath.Join(f.inc, "inner.hpp")}
		text := "#include \"lib.hpp\"\n"
		info := rn.includeInfo(context.Background(), f.cfg(), text, read, false, "")
		if !info.complete || len(info.directives) != 2 || !info.directives[0].entered || info.directives[1].includer != read[0] {
			t.Fatalf("%+v", info)
		}
		rn.includeInfo(context.Background(), f.cfg(), text, read, false, "")
		if f.count("D") != 1 {
			t.Errorf("asked %d times for one preamble", f.count("D"))
		}
		f.header(t, "inner.hpp", "// changed\n")
		rn.includeInfo(context.Background(), f.cfg(), text, read, false, "")
		if f.count("D") != 2 {
			t.Errorf("a changed header: asked %d times", f.count("D"))
		}
		rn.includeInfo(context.Background(), f.cfg(), "#define X 1\n#include \"lib.hpp\"\n", read, false, "")
		if f.count("D") != 3 {
			t.Errorf("another preamble: asked %d times", f.count("D"))
		}
	})
	t.Run("declaration_runs_share_it", func(t *testing.T) {
		// Each declaration run has a text of its own (the type it asks for is in it), but the same preamble.
		f := newFake(t)
		f.header(t, "lib.hpp", "#include \"inner.hpp\"\n")
		f.header(t, "inner.hpp", "// inner\n")
		dc := DeclConfig{Includes: []string{`"lib.hpp"`}}
		rn := new(Runner)
		for _, q := range []DeclQuery{{DeclType, "A"}, {DeclType, "B"}, {DeclFunc, "f"}, {DeclFunc, "g"}} {
			if _, err := rn.Decls(context.Background(), f.cfg(), dc, []DeclQuery{q}); err != nil {
				t.Fatal(err)
			}
		}
		if f.runs() != 4 || f.count("D") != 1 {
			t.Errorf("%d declaration runs asked the includes %d times, want 4 and 1", f.runs(), f.count("D"))
		}
	})
	t.Run("no_header", func(t *testing.T) {
		f := newFake(t)
		names := []string{"f0"}
		if _, err := new(Runner).Probe(context.Background(), f.cfg(), fnSet(names...), GenConfig{Kinds: kindsOf(names...)}); err != nil {
			t.Fatal(err)
		}
		if f.count("D") != 0 {
			t.Errorf("asked for a group with no header: %d", f.count("D"))
		}
	})
	t.Run("a_cut_answer_is_not_complete", func(t *testing.T) {
		for _, bad := range []string{"", "hang", "cut", "long", "noenter"} {
			t.Run("fake_"+bad, func(t *testing.T) {
				f := newFake(t)
				t.Setenv("FAKE_DI", bad)
				f.header(t, "lib.hpp", "#include \"inner.hpp\"\n")
				f.header(t, "inner.hpp", "// inner\n")
				limitEndsWhenHung(t)
				cfg := f.cfg()
				read := []string{filepath.Join(f.inc, "lib.hpp"), filepath.Join(f.inc, "inner.hpp")}
				info := new(Runner).includeInfo(context.Background(), cfg, "#include \"lib.hpp\"\n", read, false, "")
				if info.complete != (bad == "") {
					t.Errorf("%q: complete %v", bad, info.complete)
				}
				// An incomplete answer is not kept: the next question is asked again, and it can be complete.
				if bad != "" {
					t.Setenv("FAKE_DI", "")
					rn := new(Runner)
					t.Setenv("FAKE_DI", bad)
					rn.includeInfo(context.Background(), cfg, "#include \"lib.hpp\"\n", read, false, "")
					t.Setenv("FAKE_DI", "")
					before := f.count("D")
					again := rn.includeInfo(context.Background(), cfg, "#include \"lib.hpp\"\n", read, false, "")
					if !again.complete || f.count("D") != before+1 {
						t.Errorf("%q: the incomplete answer was kept: complete %v, %d questions after %d", bad, again.complete, f.count("D"), before)
					}
					t.Setenv("FAKE_DI", bad)
				}
				// The probe still works, with the conservative answer.
				names := []string{"f0"}
				tab, err := new(Runner).Probe(context.Background(), cfg, fnSet(names...), libGC(names...))
				if err != nil {
					t.Fatalf("%q: %v", bad, err)
				}
				if !tab.Results[0].OK || !slices.Contains(tab.Deps, filepath.Join(f.inc, "inner.hpp")) {
					t.Errorf("%q: %+v %v", bad, tab.Results[0], tab.Deps)
				}
			})
		}
	})
}

// A header edited while the compiler runs: the contents the compiler read are not the ones
// that are hashed, so the result is kept nowhere (memory included).
func TestCacheEditDuringRun(t *testing.T) {
	for _, keep := range []bool{false, true} {
		name := "new modification time"
		if keep {
			name = "old modification time"
		}
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			f := newFake(t)
			f.header(t, "a.hpp", "// one\n")
			if _, have := changeTime(mustStat(t, filepath.Join(f.inc, "a.hpp"))); keep && !have {
				t.Skip("a system with no change time cannot see an edit that keeps the modification time")
			}
			t.Setenv("FAKE_EDIT", filepath.Join(f.inc, "a.hpp")+"|// two\n")
			if keep {
				t.Setenv("FAKE_EDIT_KEEP_MTIME", "1")
			}
			names := []string{"f0"}
			set := fnSet(names...)
			rn := new(Runner)
			probeWith(t, f, rn, set, names...)
			if f.runs() != 1 {
				t.Fatalf("%d runs", f.runs())
			}
			// The same Runner and a new one both ask again; the second answer is for the new contents and is kept.
			probeWith(t, f, rn, set, names...)
			if f.runs() != 2 {
				t.Errorf("the Runner served the result of the old contents: %d runs", f.runs())
			}
			probeWith(t, f, new(Runner), set, names...)
			if f.runs() != 2 {
				t.Errorf("%d runs: the result of the new contents was not kept", f.runs())
			}
		})
	}
}

func TestCacheEditDuringDeclarationRun(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "// one\n")
	t.Setenv("FAKE_EDIT", filepath.Join(f.inc, "a.hpp")+"|// two\n")
	dc := DeclConfig{Includes: []string{`"a.hpp"`}}
	q := []DeclQuery{{DeclType, "A"}}
	rn := new(Runner)
	for i := 0; i < 2; i++ {
		if _, err := rn.Decls(context.Background(), f.cfg(), dc, q); err != nil {
			t.Fatal(err)
		}
	}
	if f.runs() != 2 {
		t.Errorf("%d runs: a declaration run that raced with an edit was kept", f.runs())
	}
}

func TestRacyDepsLooksAtTheChangeTime(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "h.h")
	os.WriteFile(p, []byte("x"), 0o644)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(p, old, old) // the modification time is old; the change time is now
	start := time.Now().Add(-time.Second)
	if _, have := changeTime(mustStat(t, p)); have {
		if !racyDeps([]string{p}, start) {
			t.Error("a file changed after the start is not racy: the change time is not looked at")
		}
	}
	if racyDeps([]string{p}, time.Now().Add(time.Hour)) && racyWindow == 0 {
		t.Error("a file changed long before the start is racy")
	}
	// A modification time in the future is suspicious whatever the change time says.
	future := time.Now().Add(time.Hour)
	os.Chtimes(p, future, future)
	if !racyDeps([]string{p}, time.Now().Add(time.Minute)) {
		t.Error("a file with a modification time after the start is not racy")
	}
}

// A header edited between two batches of one probe: the batches saw different contents, so the
// results are asked again for the contents of now (and no result of the old contents is used).
func TestCacheHeadersChangeBetweenBatches(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "// one\n")
	t.Setenv("GOCPP_PROBE_BATCH", "1")
	t.Setenv(EnvJobs, "2")
	var calls atomic.Int32
	stored := make(chan struct{})
	var once sync.Once
	hookGroupStored = func() { once.Do(func() { close(stored) }) }
	hookRunGroup = func([]string) {
		if calls.Add(1) == 2 {
			<-stored // the first batch is done, with the first contents
			p := filepath.Join(f.inc, "a.hpp")
			os.WriteFile(p, []byte("// two\n"), 0o644)
			old := time.Now().Add(-time.Hour)
			os.Chtimes(p, old, old)
		}
	}
	t.Cleanup(func() { hookRunGroup, hookGroupStored = nil, nil })
	names := []string{"f0", "f1"}
	tab := probeWith(t, f, new(Runner), fnSet(names...), names...)
	if f.runs() != 3 {
		t.Errorf("%d runs: want the two batches and the one that saw old contents again", f.runs())
	}
	for _, r := range tab.Results {
		if !r.OK {
			t.Errorf("%+v", r)
		}
	}
}

func mustStat(t *testing.T, p string) os.FileInfo {
	t.Helper()
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return fi
}

// -march=native results depend on the machine: kept in memory only.
func TestCacheNativeArchStaysInMemory(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	cfg := f.cfg()
	cfg.Flags = []string{"-march=native"}
	names := []string{"f0"}
	rn := new(Runner)
	for i := 0; i < 2; i++ {
		if _, err := rn.Probe(context.Background(), cfg, fnSet(names...), gc(names...)); err != nil {
			t.Fatal(err)
		}
	}
	if f.runs() != 1 {
		t.Errorf("not kept in memory: %d runs", f.runs())
	}
	if u := CacheUsage(f.cache); u.Entries != 0 {
		t.Errorf("written to disk: %+v", u)
	}
}

// Callers of one Runner that need the same answer share the run, with no cache folder.
func TestCacheFlightWithoutADisk(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	t.Setenv("FAKE_SLEEP", "0.5")
	cfg := f.cfg()
	cfg.CacheDir = "off"
	rn := new(Runner)
	names := fnNames(3)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := rn.Probe(context.Background(), cfg, fnSet(names...), gc(names...)); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if f.runs() != 1 || rn.Runs() != 1 {
		t.Errorf("%d compiler runs", f.runs())
	}
}

// A note that points at an alias line is placed at the alias line of the file it is read into.
func TestUnitCodecPlacesAliasNotes(t *testing.T) {
	def := &collect.Ref{Expr: &collect.Decay{X: call(pth("f9"))}}
	d0 := collect.NewSet(def).Refs[0]
	dr := &collect.DeclRef{Def: d0}
	use := ref(call(sel(lv(dr), "size")))
	set := collect.NewSet(def, use)
	for _, r := range set.Refs {
		if r.Key == d0.Key {
			dr.Def = r
		}
	}
	set = collect.NewSet(def, use) // (the keys of the copies are the same)
	for _, r := range set.Refs {
		if r.Key == d0.Key {
			dr.Def = r
		}
	}
	src := Generate(set, GenConfig{Kinds: kindsOf("f9")})
	ui := -1
	for i, u := range src.Units {
		if len(u.Aliases) > 0 {
			ui = i
		}
	}
	if ui < 0 {
		t.Fatal("no unit with an alias")
	}
	b := newBatch(src, []int{ui})
	aliasLine := 0
	for n, l := range b.src.Lines {
		if l.Role == RoleAlias {
			aliasLine = n
		}
	}
	name := aliasNameOf(src.Units[ui].Aliases[0].Text)
	r := &Result{OK: false, Error: "x", Notes: []Diag{{File: probeFileName, Line: aliasLine, Severity: "note"}}}
	body, err := encodeUnit(b, src, ui, r)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := decodeUnit(body, src, ui)
	if !ok || got.Notes[0].Line != src.aliasAt[name] || src.aliasAt[name] == 0 {
		t.Errorf("note at line %d, the alias is at %d (%v)", got.Notes[0].Line, src.aliasAt[name], ok)
	}
}

func TestHashFileIsAWholeHash(t *testing.T) {
	p := filepath.Join(t.TempDir(), "h")
	os.WriteFile(p, []byte("x"), 0o644)
	if h := hashFile(p); len(h) != 32 {
		t.Errorf("hash %q", h)
	}
}

// ---------------------------------------------------------------- the lows

func TestCachePruneMarker(t *testing.T) {
	dir := filepath.Join(t.TempDir(), cacheSub)
	for i := 0; i < 5; i++ {
		writeEntry(filepath.Join(dir, entryName("u", "x", string(rune('a'+i)))), []byte("x"))
	}
	now := time.Now()
	maybePrune(dir, Limits{MaxEntries: 3}, now)
	if u := CacheUsage(dir); u.Entries != 3 {
		t.Fatalf("%d entries", u.Entries)
	}
	for i := 0; i < 3; i++ { // more entries, and a prune that is not due
		writeEntry(filepath.Join(dir, entryName("u", "y", string(rune('a'+i)))), []byte("x"))
	}
	maybePrune(dir, Limits{MaxEntries: 3}, now.Add(time.Minute))
	if u := CacheUsage(dir); u.Entries != 6 {
		t.Errorf("pruned again within the hour: %d entries", u.Entries)
	}
	maybePrune(dir, Limits{MaxEntries: 3}, now.Add(2*time.Hour))
	if u := CacheUsage(dir); u.Entries != 3 {
		t.Errorf("not pruned after the hour: %d entries", u.Entries)
	}
}

func TestVariantFromTheFutureIsNotUsed(t *testing.T) {
	if !variantFresh(time.Now().Add(-time.Hour).UnixNano()) || variantFresh(time.Now().Add(time.Hour).UnixNano()) || variantFresh(time.Now().Add(-8*24*time.Hour).UnixNano()) {
		t.Error("variantFresh")
	}
}

// A lock whose process was killed is taken over at once.
func TestCacheDeadLockHolder(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	dead := cmd.Process.Pid // a process that has ended
	dir := t.TempDir()
	path := lockFile(dir, "k")
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte(hostName()+" "+strconv.Itoa(dead)+"\n"), 0o600)
	t0 := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	removals := 0 // (the taker removes the dead one's file and, at the release, its own: both through removeLock)
	defer func(f func(string) error) { removeFile = f }(removeFile)
	removeFile = func(p string) error { removals++; return os.Remove(p) }
	release, _ := acquireLock(ctx, dir, "k", time.Hour)
	if release == nil || time.Since(t0) > 15*time.Second {
		t.Fatalf("the lock of a dead process was not taken over (%v)", time.Since(t0))
	}
	release()
	if removals != 2 {
		t.Errorf("%d removals through removeLock, want 2", removals)
	}
	// A live process's lock (this one), and one of another machine, are waited for.
	for _, owner := range []string{hostName() + " " + strconv.Itoa(os.Getpid()), "some-other-host " + strconv.Itoa(dead)} {
		os.WriteFile(path, []byte(owner+"\n"), 0o600)
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		if rel, waited := acquireLock(ctx, dir, "k", time.Hour); rel != nil || !waited {
			t.Errorf("%q: the lock was taken", owner)
		}
		cancel()
	}
}

// The toolchain is part of the key: another search path is another compiler, and the question is asked
// once an hour, not at every run.
func TestCacheToolchain(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	names := []string{"f0"}
	set := fnSet(names...)
	probe := func() {
		if _, err := new(Runner).Probe(context.Background(), f.cfg(), set, gc(names...)); err != nil {
			t.Fatal(err)
		}
	}
	probe()
	probe()
	probe()
	if f.runs() != 1 || f.count("V") != 1 {
		t.Fatalf("%d runs, the toolchain asked %d times", f.runs(), f.count("V"))
	}
	// Another libstdc++ is chosen (the search path changed): the results are not used. (The question is
	// believed for an hour, so it is asked again by making the entry old.)
	age := func() { // the entry is two hours old
		filepath.WalkDir(filepath.Join(f.cache, "t"), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			body, ok := readEntry(p)
			var tc toolchain
			if !ok || json.Unmarshal(body, &tc) != nil {
				t.Fatal("no toolchain entry")
			}
			tc.Created = time.Now().Add(-2 * time.Hour).UnixNano()
			nb, _ := json.Marshal(tc)
			if !writeEntry(p, nb) {
				t.Fatal("cannot write")
			}
			return nil
		})
	}
	t.Setenv("FAKE_SYS", filepath.Join(t.TempDir(), "another-gcc"))
	probe()
	if f.runs() != 1 {
		t.Errorf("the changed toolchain was not noticed in the hour: %d runs", f.runs())
	}
	age()
	probe()
	if f.runs() != 2 || f.count("V") != 2 {
		t.Errorf("%d runs, toolchain asked %d times", f.runs(), f.count("V"))
	}
}

// The toolchain is asked once for a Config, whatever the limits of the run that asks (a probe, a
// declaration run with its own dump limit) and however many of them start together.
func TestCacheToolchainIsAskedOnce(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	names := []string{"f0"}
	probeWith(t, f, new(Runner), fnSet(names...), names...)
	rn := new(Runner)
	qs := []DeclQuery{{DeclType, "A"}, {DeclType, "B"}, {DeclType, "C"}, {DeclType, "D"}}
	if _, err := rn.Decls(context.Background(), f.cfg(), DeclConfig{Includes: []string{`"a.hpp"`}}, qs); err != nil {
		t.Fatal(err)
	}
	if f.count("V") != 1 {
		t.Errorf("the toolchain was asked %d times", f.count("V"))
	}
}

// Results that a Runner made under one set of header contents are dropped when the headers change
// before the table is complete: a batch that another caller ran meanwhile saw the new contents.
func TestCacheDropsResultsOfOldContents(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "// one\n")
	t.Setenv("FAKE_HDR_TYPE", "a.hpp")
	t.Setenv("GOCPP_PROBE_BATCH", "1")
	t.Setenv(EnvJobs, "2")
	rn := new(Runner)
	reached := make(chan struct{})
	release := make(chan struct{})
	storedFirst := make(chan struct{})
	var calls atomic.Int32
	var once sync.Once
	hookGroupStored = func() { once.Do(func() { close(storedFirst) }) }
	hookRunGroup = func(keys []string) {
		if len(keys) != 1 || keys[0] != "call(::f1|)" {
			return
		}
		switch calls.Add(1) {
		case 1: // the first caller's batch for f1 waits
			close(reached)
			<-release
		case 2: // the second caller's batch for f1: the header changes before it runs
			p := filepath.Join(f.inc, "a.hpp")
			os.WriteFile(p, []byte("// two\n"), 0o644)
			old := time.Now().Add(-time.Hour)
			os.Chtimes(p, old, old)
		}
	}
	t.Cleanup(func() { hookRunGroup, hookGroupStored = nil, nil })
	first := make(chan *Table, 1)
	go func() { first <- probeWith(t, f, rn, fnSet("f0", "f1"), "f0", "f1") }()
	<-reached
	<-storedFirst // f0 is done, with the contents "one"
	probeWith(t, f, rn, fnSet("f1"), "f1")
	close(release)
	tab := <-first
	for _, r := range tab.Results {
		if !strings.HasSuffix(r.Type, "_two") {
			t.Errorf("%s: %s (a result of the old contents was kept)", r.Key, r.Type)
		}
	}
}

// With no cache folder there is no toolchain to ask for, but a missing header still asks for the
// compiler's search path, so that its appearance in a compiler folder is seen.
func TestCacheSearchPathWithoutACacheFolder(t *testing.T) {
	f := newFake(t)
	f.header(t, "lib.hpp", "#include \"inner.hpp\"\n")
	cfg := f.cfg()
	cfg.CacheDir = "off"
	rn := new(Runner)
	tab, err := rn.Probe(context.Background(), cfg, fnSet("f0"), libGC("f0"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range tab.Deps {
		found = found || d == filepath.Join(os.Getenv("FAKE_SYS"), "inner.hpp")
	}
	if !found || f.count("V") != 1 {
		t.Errorf("search path asked %d times; deps %v", f.count("V"), tab.Deps)
	}
}

// A toolchain entry from the future (the clock was set back) is not believed.
func TestCacheToolchainFromTheFuture(t *testing.T) {
	f := newFake(t)
	f.header(t, "a.hpp", "//\n")
	probeWith(t, f, new(Runner), fnSet("f0"), "f0")
	if f.count("V") != 1 {
		t.Fatalf("asked %d times", f.count("V"))
	}
	filepath.WalkDir(filepath.Join(f.cache, "t"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		body, _ := readEntry(p)
		var tc toolchain
		json.Unmarshal(body, &tc)
		tc.Created = time.Now().Add(time.Hour).UnixNano()
		nb, _ := json.Marshal(tc)
		writeEntry(p, nb)
		return nil
	})
	probeWith(t, f, new(Runner), fnSet("f0"), "f0")
	if f.count("V") != 2 {
		t.Errorf("a toolchain from the future was believed (%d questions)", f.count("V"))
	}
}
