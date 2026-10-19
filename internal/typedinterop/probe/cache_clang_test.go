package probe

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
)

// The cache with the real clang++.

func clangCacheConfig(t *testing.T, dir string, flags ...string) Config {
	cfg := clangConfig(t, dir, flags...)
	cfg.CacheDir = filepath.Join(t.TempDir(), cacheSub)
	// (a time limit that makes a lock that is never let go fail the test in seconds; a real run takes one)
	cfg.Timeout = 30 * time.Second
	t.Cleanup(func() {
		if entries, _ := os.ReadDir(filepath.Join(cfg.CacheDir, "locks")); len(entries) != 0 {
			t.Errorf("locks left behind: %v", entries)
		}
	})
	return cfg
}

func byKey(tab *Table, key string) *Result {
	for i := range tab.Results {
		if tab.Results[i].Key == key {
			return &tab.Results[i]
		}
	}
	return nil
}

func realRefs() (*collect.Set, GenConfig) {
	r1 := ref(pth("LIBV"))
	r1.WantValue = true
	set := collect.NewSet(r1, ref(pth("BUFSIZ")), ref(call(pth("strlen"), lit(`"abc"`))), ref(call(pth("strlen"), lv(str()))), ref(call(pth("nothing"))),
		ref(&collect.Select{X: lv(&collect.Decay{X: call(pth("fopen"), lit(`"x"`), lit(`"r"`))}), Name: "_flags"}))
	kinds := map[string]interop.Kind{"LIBV": interop.Macro, "BUFSIZ": interop.Macro, "strlen": interop.Value, "fopen": interop.Value, "FILE": interop.Type}
	return set, GenConfig{Includes: []string{"<cstdio>", "<cstring>", `"lib.hpp"`}, Kinds: kinds}
}

func TestRealClangCacheColdWarm(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "lib.hpp", "#define LIBV 41\n")
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(dir, "lib.hpp"), old, old)
	cfg := clangCacheConfig(t, dir)
	set, gc := realRefs()
	rn1 := new(Runner)
	cold, err := rn1.Probe(context.Background(), cfg, set, gc)
	if err != nil {
		t.Fatal(err)
	}
	if rn1.Runs() != 1 || cold.Cached {
		t.Fatalf("cold: %d runs", rn1.Runs())
	}
	rn2 := new(Runner)
	warm, err := rn2.Probe(context.Background(), cfg, set, gc)
	if err != nil {
		t.Fatal(err)
	}
	if rn2.Runs() != 0 || !warm.Cached || warm.Cache.Hits != warm.Cache.Units {
		t.Fatalf("warm: %d runs, %+v", rn2.Runs(), warm.Cache)
	}
	if a, b := stable(t, cold), stable(t, warm); a != b {
		t.Errorf("cold and warm differ:\n%s\n%s", a, b)
	}
	if v := byKey(warm, "::LIBV").Value; v == nil || v.Int.Int64() != 41 {
		t.Errorf("LIBV %v", v)
	}
	// An edit of the header: the same size, the same time.
	st, _ := os.Stat(filepath.Join(dir, "lib.hpp"))
	writeHeader(t, dir, "lib.hpp", "#define LIBV 42\n")
	os.Chtimes(filepath.Join(dir, "lib.hpp"), st.ModTime(), st.ModTime())
	rn3 := new(Runner)
	edited, err := rn3.Probe(context.Background(), cfg, set, gc)
	if err != nil {
		t.Fatal(err)
	}
	if rn3.Runs() != 1 || edited.Cached || byKey(edited, "::LIBV").Value.Int.Int64() != 42 || edited.Key == cold.Key {
		t.Errorf("edit: %d runs, LIBV %v", rn3.Runs(), byKey(edited, "::LIBV").Value)
	}
	// A touch without a change.
	os.Chtimes(filepath.Join(dir, "lib.hpp"), old, old)
	rn4 := new(Runner)
	if again, err := rn4.Probe(context.Background(), cfg, set, gc); err != nil || rn4.Runs() != 0 || !again.Cached {
		t.Errorf("touch: %v, %d runs", err, rn4.Runs())
	}
	// A new reference: only it runs.
	set2 := collect.NewSet(append(append([]*collect.Ref(nil), set.Refs...), ref(call(pth("strlen"), lit(`"zzz"`))))...)
	rn5 := new(Runner)
	more, err := rn5.Probe(context.Background(), cfg, set2, gc)
	if err != nil {
		t.Fatal(err)
	}
	if rn5.Runs() != 1 || more.Cache.Ran != 1 || more.Cache.Hits != len(set.Refs) {
		t.Errorf("one more reference: %d runs, %+v", rn5.Runs(), more.Cache)
	}
	for _, r := range more.Results {
		if r.Skipped == "" && r.Key == `call(::strlen|lit("zzz"))` && (!r.OK || r.Base != "unsigned long") {
			t.Errorf("the new result %+v", r)
		}
	}
}

// The probe's errors survive the cache with their places.
func TestRealClangCacheKeepsErrors(t *testing.T) {
	dir := t.TempDir()
	cfg := clangCacheConfig(t, dir)
	set := collect.NewSet(ref(call(pth("strlen"), lv(str()))), ref(call(pth("nothing"))), ref(pth("BUFSIZ")))
	gc := GenConfig{Includes: []string{"<cstring>", "<cstdio>"}, Kinds: map[string]interop.Kind{"strlen": interop.Value, "BUFSIZ": interop.Macro}}
	cold, err := new(Runner).Probe(context.Background(), cfg, set, gc)
	if err != nil {
		t.Fatal(err)
	}
	rn := new(Runner)
	warm, err := rn.Probe(context.Background(), cfg, set, gc)
	if err != nil || rn.Runs() != 0 {
		t.Fatalf("%v, %d runs", err, rn.Runs())
	}
	n := 0
	for i, r := range cold.Results {
		if r.ErrorAt != nil {
			n++
			if l := cold.Source.Lines[r.ErrorAt.Line]; l.Ref != r.ID {
				t.Errorf("%s: error at line %d of ref %d", r.Key, r.ErrorAt.Line, l.Ref)
			}
			if w := warm.Results[i]; w.ErrorAt == nil || *w.ErrorAt != *r.ErrorAt || len(w.Notes) != len(r.Notes) {
				t.Errorf("%s: the warm error differs: %+v vs %+v", r.Key, w.ErrorAt, r.ErrorAt)
			}
		}
	}
	if n < 2 {
		t.Errorf("%d errors", n)
	}
	if stable(t, cold) != stable(t, warm) {
		t.Error("cold and warm differ")
	}
}

// Cached and batched results are the results of one run of the whole file.
func TestRealClangBatchesEqualOneRun(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "lib.hpp", "#define LIBV 41\n")
	set, gc := realRefs()
	whole := clangConfig(t, dir)
	whole.CacheDir = "off"
	t.Setenv(EnvBatch, "100000")
	one, err := new(Runner).Probe(context.Background(), whole, set, gc)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvBatch, "1")
	t.Setenv(EnvJobs, "6")
	rn := new(Runner)
	many, err := rn.Probe(context.Background(), whole, set, gc)
	if err != nil {
		t.Fatal(err)
	}
	if rn.Runs() < 2 {
		t.Errorf("%d runs", rn.Runs())
	}
	if answers(t, one) != answers(t, many) {
		t.Errorf("batched results differ:\n%s\n%s", answers(t, one), answers(t, many))
	}
	// And the old, direct way (the whole file as one text, no units) gives the same types.
	direct := *one.Source
	direct.Preamble, direct.Units = "", nil
	tab, err := new(Runner).Run(context.Background(), whole, &direct)
	if err != nil {
		t.Fatal(err)
	}
	for i := range tab.Results {
		a, b := tab.Results[i], one.Results[i]
		if a.OK != b.OK || a.Type != b.Type || a.Error != b.Error || a.Category != b.Category {
			t.Errorf("%s: whole file %+v, units %+v", a.Key, a, b)
		}
	}
}

// declTexts lists the types of a table, written out here and not with the cache's own walker.
func declTexts(tab *DeclTable) []*TypeText {
	var out []*TypeText
	for _, k := range sortedKeys(tab.Records) {
		r := tab.Records[k]
		for i := range r.Fields {
			out = append(out, &r.Fields[i].Type)
		}
		for i := range r.Statics {
			out = append(out, &r.Statics[i].Type)
		}
		for i := range r.Methods {
			out = append(out, &r.Methods[i].Sig)
		}
		for i := range r.Bases {
			out = append(out, &r.Bases[i].Type)
		}
	}
	for _, k := range sortedKeys(tab.Enums) {
		out = append(out, &tab.Enums[k].Underlying)
	}
	for _, k := range sortedKeys(tab.Funcs) {
		for i := range tab.Funcs[k].Overloads {
			out = append(out, &tab.Funcs[k].Overloads[i].Sig)
		}
	}
	return out
}

func TestRealClangDeclsCache(t *testing.T) {
	cfg, dc := declsSetup(t)
	cfg.CacheDir = filepath.Join(t.TempDir(), cacheSub)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(cfg.Dir, "decls.hpp"), old, old)
	ctx := context.Background()
	cold, err := new(Runner).Decls(ctx, cfg, dc, someTypes)
	if err != nil {
		t.Fatal(err)
	}
	rn := new(Runner)
	warm, err := rn.Decls(ctx, cfg, dc, someTypes)
	if err != nil {
		t.Fatal(err)
	}
	if warm.Runs != 0 || !warm.Cached || rn.DeclRuns() != 0 {
		t.Fatalf("warm: %d runs, cached %v", warm.Runs, warm.Cached)
	}
	c, w := *cold, *warm
	c.Timing, c.Stats, c.Cached, c.Runs = Timing{}, Stats{}, false, 0
	w.Timing, w.Stats, w.Cached, w.Runs = Timing{}, Stats{}, false, 0
	if jsonOf(&c) != jsonOf(&w) {
		t.Errorf("cold and warm differ:\n%s\n%s", jsonOf(&c), jsonOf(&w))
	}
	ct, wt := declTexts(cold), declTexts(warm)
	if len(ct) == 0 || len(ct) != len(wt) {
		t.Fatalf("%d and %d types", len(ct), len(wt))
	}
	for i := range ct {
		if !ctype.Equal(ct[i].T, wt[i].T) || ct[i].T == nil && ct[i].Err == "" {
			t.Errorf("type %d (%q): cold %v warm %v", i, ct[i].Text, ct[i].T, wt[i].T)
		}
	}
	if len(cold.Enums) == 0 {
		t.Error("no enum in the test")
	}
	for k, e := range cold.Enums {
		for i, v := range e.Values {
			if w := warm.Enums[k].Values[i]; w.Name != v.Name || w.Value.Cmp(v.Value) != 0 {
				t.Errorf("enumerator %v vs %v", v, w)
			}
		}
	}
	// The header changes: only the runs that read it run again (all, here), and see the change.
	st, _ := os.Stat(filepath.Join(cfg.Dir, "decls.hpp"))
	writeHeader(t, cfg.Dir, "decls.hpp", strings.Replace(declsHeader, "struct Vec { double x, y;", "struct Vec { double x, y, z;", 1))
	os.Chtimes(filepath.Join(cfg.Dir, "decls.hpp"), st.ModTime().Add(-time.Hour), st.ModTime().Add(-time.Hour))
	rn2 := new(Runner)
	third, err := rn2.Decls(ctx, cfg, dc, someTypes)
	if err != nil {
		t.Fatal(err)
	}
	if third.Runs == 0 || len(third.Records["geo::Vec"].Fields) != 4 {
		t.Errorf("after the edit: %d runs, %v", third.Runs, third.Records["geo::Vec"].Fields)
	}
	// Only one name asked for: one run (the other types of the same file are other runs).
	rn3 := new(Runner)
	if _, err := rn3.Decls(ctx, cfg, dc, someTypes[:1]); err != nil {
		t.Fatal(err)
	}
	if rn3.DeclRuns() != 0 {
		t.Errorf("a name that was cached ran: %d", rn3.DeclRuns())
	}
}

func TestRealClangTooLargeRefusalOnDisk(t *testing.T) {
	cfg, dc := declsSetup(t)
	cfg.CacheDir = filepath.Join(t.TempDir(), cacheSub)
	dc.MaxJSON = 4096
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(cfg.Dir, "decls.hpp"), old, old)
	q := []DeclQuery{{DeclType, "Rect"}}
	cold, err := new(Runner).Decls(context.Background(), cfg, dc, q)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cold.Errors["type Rect"], "larger than the limit") {
		t.Fatalf("errors %v", cold.Errors)
	}
	rn := new(Runner)
	warm, err := rn.Decls(context.Background(), cfg, dc, q)
	if err != nil || rn.DeclRuns() != 0 || !warm.Cached || warm.Errors["type Rect"] != cold.Errors["type Rect"] {
		t.Errorf("%v, %d runs, %v", err, rn.DeclRuns(), warm.Errors)
	}
	b, _ := json.Marshal(warm.Deps)
	if !strings.Contains(string(b), "decls.hpp") {
		t.Errorf("deps %s", b)
	}
}

// ---------------------------------------------------------------- after the review

func backdate(t *testing.T, paths ...string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	for _, p := range paths {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
}

// A header missing inside another header: the answer has the error, is cached, and the header that
// appears next to the includer (or anywhere it is searched) makes another answer.
func TestRealClangNestedMissingHeader(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "lib.hpp", "#include \"inner.hpp\"\n#define LIBV 41\n")
	backdate(t, filepath.Join(dir, "lib.hpp"))
	cfg := clangCacheConfig(t, dir)
	set, gc := realRefs()
	run := func() (*Table, *Runner) {
		rn := new(Runner)
		tab, err := rn.Probe(context.Background(), cfg, set, gc)
		if err != nil {
			t.Fatal(err)
		}
		return tab, rn
	}
	first, _ := run()
	if len(first.HeaderErrors) == 0 || !strings.Contains(first.HeaderErrors[0].Msg, "inner.hpp") {
		t.Fatalf("header errors %+v", first.HeaderErrors)
	}
	if again, rn := run(); !again.Cached || rn.Runs() != 0 {
		t.Fatalf("the negative answer is not cached (%d runs)", rn.Runs())
	}
	writeHeader(t, dir, "inner.hpp", "// now there\n")
	backdate(t, filepath.Join(dir, "inner.hpp"))
	tab, rn := run()
	if rn.Runs() != 1 || len(tab.HeaderErrors) != 0 {
		t.Errorf("after the header appeared: %d runs, errors %+v", rn.Runs(), tab.HeaderErrors)
	}
	// The same for the declarations.
	dir2 := t.TempDir()
	writeHeader(t, dir2, "decls.hpp", "#include \"inner.hpp\"\n"+declsHeader)
	backdate(t, filepath.Join(dir2, "decls.hpp"))
	dcfg := clangCacheConfig(t, dir2)
	dc := DeclConfig{Includes: []string{`"./decls.hpp"`}}
	q := []DeclQuery{{DeclType, "Rect"}}
	decls := func() (*DeclTable, *Runner) {
		rn := new(Runner)
		tab, err := rn.Decls(context.Background(), dcfg, dc, q)
		if err != nil {
			t.Fatal(err)
		}
		return tab, rn
	}
	decls()
	if _, rn := decls(); rn.DeclRuns() != 0 {
		t.Fatalf("not cached: %d runs", rn.DeclRuns())
	}
	writeHeader(t, dir2, "inner.hpp", "// now there\n")
	backdate(t, filepath.Join(dir2, "inner.hpp"))
	if tab, rn := decls(); rn.DeclRuns() == 0 || tab.Records["Rect"] == nil {
		t.Errorf("after the header appeared: %d runs, %v", rn.DeclRuns(), tab.Errors)
	}
}

// A header that appears in a folder before the one an included header was found in hides it (the
// answers differ: the macro has another value); a header that __has_include asks for is the documented limit.
func TestRealClangHiddenHeader(t *testing.T) {
	dir := t.TempDir()
	inc1, inc2 := filepath.Join(dir, "inc1"), filepath.Join(dir, "inc2")
	os.Mkdir(inc1, 0o755)
	os.Mkdir(inc2, 0o755)
	writeHeader(t, dir, "lib.hpp", "#include <dep.h>\n")
	writeHeader(t, inc2, "dep.h", "#define DEPV 3\n")
	backdate(t, filepath.Join(dir, "lib.hpp"), filepath.Join(inc2, "dep.h"))
	cfg := clangCacheConfig(t, dir, "-I"+inc1, "-I"+inc2)
	r := ref(pth("DEPV"))
	r.WantValue = true
	set := collect.NewSet(r)
	gc := GenConfig{Includes: []string{`"lib.hpp"`}, Kinds: map[string]interop.Kind{"DEPV": interop.Macro}}
	run := func() (int64, int) {
		rn := new(Runner)
		tab, err := rn.Probe(context.Background(), cfg, set, gc)
		if err != nil {
			t.Fatal(err)
		}
		return tab.Result(0).Value.Int.Int64(), rn.Runs()
	}
	if v, _ := run(); v != 3 {
		t.Fatalf("DEPV %d", v)
	}
	if v, runs := run(); v != 3 || runs != 0 {
		t.Fatalf("not cached: %d, %d runs", v, runs)
	}
	writeHeader(t, inc1, "dep.h", "#define DEPV 7\n")
	backdate(t, filepath.Join(inc1, "dep.h"))
	if v, runs := run(); v != 7 || runs != 1 {
		t.Errorf("a header that hides the one that was found: DEPV %d after %d runs (want 7, 1)", v, runs)
	}
}

// A header edited while the compiler runs. The wrapper edits it after the real clang has exited.
func TestRealClangEditDuringRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the compiler is a shell script")
	}
	for _, keep := range []bool{false, true} {
		t.Run(map[bool]string{false: "new_time", true: "old_time"}[keep], func(t *testing.T) {
			dir := t.TempDir()
			h := filepath.Join(dir, "lib.hpp")
			writeHeader(t, dir, "lib.hpp", "#define LIBV 41\n")
			backdate(t, h)
			cfg := clangCacheConfig(t, dir)
			wrap := filepath.Join(t.TempDir(), "wrap.sh")
			flag := filepath.Join(t.TempDir(), "done")
			edit := "echo '#define LIBV 42' > " + h
			if keep {
				edit += "; touch -d '1 hour ago' " + h
			}
			script := "#!/bin/sh\n" + cfg.CXX.Path + " \"$@\"\nrc=$?\nif [ ! -e " + flag + " ] && [ \"${1:-}\" != \"--version\" ]; then case \" $* \" in *-ast-dump*) touch " + flag + "; " + edit + ";; esac; fi\nexit $rc\n"
			if err := os.WriteFile(wrap, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			cfg.CXX.Path = wrap
			set, gc := realRefs()
			rn := new(Runner)
			if _, err := rn.Probe(context.Background(), cfg, set, gc); err != nil {
				t.Fatal(err)
			}
			for _, r := range []*Runner{rn, new(Runner)} {
				tab, err := r.Probe(context.Background(), cfg, set, gc)
				if err != nil {
					t.Fatal(err)
				}
				if v := byKey(tab, "::LIBV").Value; v.Int.Int64() != 42 {
					t.Errorf("the header says 42 and the answer is %v", v)
				}
			}
		})
	}
}
