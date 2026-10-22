package probe

import (
	"encoding/json"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
)

// codecSource has two references, the second with a local variable alias.
func codecSource(t *testing.T) *Source {
	t.Helper()
	r1 := ref(call(pth("f1")))
	r1.WantValue = true // main, value and string lines
	src := Generate(collect.NewSet(ref(call(pth("f0"))), r1), GenConfig{Includes: []string{"<cstdio>"}, Kinds: kindsOf("f0", "f1")})
	if len(src.Units) != 2 || len(src.Units[1].Lines) != 3 {
		t.Fatalf("%d units", len(src.Units))
	}
	return src
}

func TestUnitCodecRoundTrip(t *testing.T) {
	src := codecSource(t)
	b := newBatch(src, []int{1}) // only the second unit: its lines move
	u := &src.Units[1]
	bl := b.first // the batch's line of the unit's first line
	consts := []*Const{
		{Kind: "int", Int: new(big.Int).Lsh(big.NewInt(1), 100)},
		{Kind: "int", Int: big.NewInt(-5)},
		{Kind: "bool", Int: big.NewInt(1)},
		{Kind: "float", Float: math.Float64frombits(0x7ff8000000000123)}, // a NaN with a payload
		{Kind: "float", Float: math.Inf(-1)},
		{Kind: "float", Float: math.Copysign(0, -1)},
		{Kind: "string", Str: "a\x00b\n\"héllo\u2028"},
	}
	for _, c := range consts {
		r := &Result{ID: u.Ref, Key: src.Keys[u.Ref], OK: true, Form: FormExpr, Type: "int", Base: "int", Category: CatPRValue, Value: c,
			ErrorAt:        &Diag{File: probeFileName, Line: bl, Col: 5, Severity: "error", Msg: "m"},
			Notes:          []Diag{{File: probeFileName, Line: bl, Col: 2, Severity: "note"}, {File: probeFileName, Line: src.HelperFrom + 1, Severity: "note"}, {File: "/usr/include/x.h", Line: 77, Severity: "note"}, {File: probeFileName, Line: bl + 40, Severity: "note"}, {File: probeFileName, Line: bl + 2, Severity: "note"}},
			RecoveredAt:    &Diag{File: probeFileName, Line: bl, Severity: "error"},
			RecoveredNotes: []Diag{{File: "h.hpp", Line: 3}},
		}
		body, err := encodeUnit(b, src, 1, r)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := decodeUnit(body, src, 1)
		if !ok {
			t.Fatalf("%v: cannot decode %s", c, body)
		}
		at := u.At[0]
		want := *r
		want.ErrorAt = &Diag{File: probeFileName, Line: at, Col: 5, Severity: "error", Msg: "m"}
		want.RecoveredAt = &Diag{File: probeFileName, Line: at, Severity: "error"}
		want.Notes = []Diag{{File: probeFileName, Line: at, Col: 2, Severity: "note"}, {File: probeFileName, Line: src.HelperFrom + 1, Severity: "note"}, {File: "/usr/include/x.h", Line: 77, Severity: "note"}, {File: probeFileName, Line: 0, Severity: "note"}, {File: probeFileName, Line: u.At[2], Severity: "note"}}
		if !reflect.DeepEqual(got.Notes, want.Notes) || !reflect.DeepEqual(got.ErrorAt, want.ErrorAt) || !reflect.DeepEqual(got.RecoveredAt, want.RecoveredAt) || !reflect.DeepEqual(got.RecoveredNotes, want.RecoveredNotes) {
			t.Errorf("places:\n got %+v %+v\nwant %+v %+v", got.Notes, got.ErrorAt, want.Notes, want.ErrorAt)
		}
		if got.Value.Kind != c.Kind || got.Value.Str != c.Str || math.Float64bits(got.Value.Float) != math.Float64bits(c.Float) || (c.Int != nil && got.Value.Int.Cmp(c.Int) != 0) {
			t.Errorf("value %v became %v", c, got.Value)
		}
		if got.ID != u.Ref || got.Key != src.Keys[u.Ref] || got.Type != "int" || !got.OK {
			t.Errorf("result %+v", got)
		}
	}
}

func TestUnitCodecRejectsDamage(t *testing.T) {
	src := codecSource(t)
	for _, body := range []string{
		``, `{`, `[]`, `{"format":2,"kind":"u"}`, `{"format":1,"kind":"c"}`,
		`{"format":1,"kind":"u","result":{"r":{"ok":true},"v":{"kind":"int","int":"zz"}}}`,
		`{"format":1,"kind":"u","result":{"r":{"ok":true},"v":{"kind":"weird"}}}`,
		`{"format":1,"kind":"u","result":{"r":{"ok":true,"errorAt":{"severity":"error","msg":"x"}},"ea":{"k":"o","n":99}}}`,
		`{"format":1,"kind":"u","result":{"r":{"ok":true,"errorAt":{"severity":"error","msg":"x"}},"ea":{"k":"a","a":"gp_decl_none"}}}`,
		`{"format":1,"kind":"u","result":{"r":{"ok":true,"notes":[{"severity":"note"}]},"n":[]}}`,
	} {
		if _, ok := decodeUnit([]byte(body), src, 0); ok {
			t.Errorf("accepted %s", body)
		}
	}
}

// A unit with the same probe in another position or with another number has the same hash;
// a different probe has another.
func TestUnitHash(t *testing.T) {
	a := Generate(fnSet("f0", "f1", "f2"), GenConfig{Kinds: kindsOf("f0", "f1", "f2")})
	b := Generate(fnSet("g", "f1", "f2"), GenConfig{Kinds: kindsOf("g", "f1", "f2")})
	byKey := func(s *Source) map[string]string {
		m := map[string]string{}
		for _, u := range s.Units {
			m[s.Keys[u.Ref]] = u.Hash
		}
		return m
	}
	ma, mb := byKey(a), byKey(b)
	if ma["call(::f1|)"] != mb["call(::f1|)"] || ma["call(::f2|)"] != mb["call(::f2|)"] {
		t.Error("the hash depends on the position")
	}
	if ma["call(::f0|)"] == ma["call(::f1|)"] {
		t.Error("two probes have one hash")
	}
	if a.Units[0].Ref == b.Units[0].Ref && a.Preamble != b.Preamble {
		t.Error("the preamble depends on the references")
	}
	// The units' lines are the file's lines.
	for _, s := range []*Source{a, b} {
		lines := strings.Split(s.Text, "\n")
		for _, u := range s.Units {
			for i, l := range u.Lines {
				if lines[u.At[i]-1] != l.Text {
					t.Errorf("unit %d line %d: %q vs %q", u.Ref, i, lines[u.At[i]-1], l.Text)
				}
			}
		}
		if !strings.HasPrefix(s.Text, s.Preamble) {
			t.Error("the preamble is not the start of the file")
		}
	}
}

func TestCanonPath(t *testing.T) {
	same := [][2]string{
		{`C:\Users\Me\Proj`, `c:/users/me/proj/`},
		{`C:\Users\Me\Proj\.\sub\..`, `C:/Users/Me/Proj`},
		{`c:\msys64\mingw64\include`, `C:/MSYS64/mingw64/include`},
	}
	for _, p := range same {
		if canonPathFor("windows", p[0]) != canonPathFor("windows", p[1]) {
			t.Errorf("%s and %s are not one folder on windows", p[0], p[1])
		}
	}
	if canonPathFor("linux", "/A/b/") == canonPathFor("linux", "/a/b") {
		t.Error("linux paths are case sensitive")
	}
	if canonPathFor("linux", `/a\b`) == canonPathFor("linux", `/a/b`) {
		t.Error("a backslash is a name character on linux")
	}
	// The key has it: the same folder spelled two ways is one key on windows, not on linux.
	cxx := build.CXX{Path: "/x/clang++", Family: build.Clang, Version: "21"}
	src := &Source{Text: "x"}
	k := func(dir string) string { return PreKey(Config{CXX: cxx, Dir: dir}, src) }
	if (runtime.GOOS == "windows") != (k(`C:\Proj`) == k(`c:/proj`)) {
		t.Errorf("PreKey and the folder's spelling on %s", runtime.GOOS)
	}
}

// What a MinGW clang writes: CRLF line ends, forward slashes with a drive letter, spaces
// and a continued line.
func TestWindowsOutputsOfTheCompiler(t *testing.T) {
	if runtime.GOOS == "windows" {
		dep := "gp_probe: C:/msys64/mingw64/include/c++/15/vector \\\r\n C:/Program\\ Files/x/a.h \\\r\n C:/proj/gp_probe.cpp\r\n"
		got := interop.ParseDeps([]byte(dep), `C:\proj\gp_probe.cpp`, `C:\proj`)
		want := []string{`C:\msys64\mingw64\include\c++\15\vector`, `C:\Program Files\x\a.h`}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("deps %q, want %q", got, want)
		}
	}
	// The diagnostics and the search list are read with CRLF line ends too.
	out := []byte("In file included from C:/proj/gp_probe.cpp:3:\r\nC:/proj/gp_probe.cpp:7:5: error: boom\r\nC:/proj/gp_probe.cpp:7:2: note: here\r\n")
	groups := ParseDiags(out, `C:/proj/gp_probe.cpp`)
	if len(groups) != 1 || groups[0].Err.Msg != "boom" || groups[0].Err.File != probeFileName || len(groups[0].Notes) != 1 || groups[0].Notes[0].Msg != "here" {
		t.Errorf("groups %+v", groups)
	}
	v := []byte("clang version 21\r\n#include \"...\" search starts here:\r\n#include <...> search starts here:\r\n C:/msys64/mingw64/include/c++/15\r\n C:/msys64/mingw64/include\r\n /Library/Frameworks (framework directory)\r\nEnd of search list.\r\n")
	dirs := parseSearchDirs(v)
	if len(dirs) != 3 || dirs[0] != filepath.Clean("C:/msys64/mingw64/include/c++/15") || dirs[2] != filepath.Clean("/Library/Frameworks") {
		t.Errorf("dirs %q", dirs)
	}
}

func TestDepsHashOfManyFilesIsTheSameAsOneByOne(t *testing.T) {
	dir := t.TempDir()
	var deps []string
	for i := 0; i < 60; i++ {
		p := filepath.Join(dir, "h"+string(rune('a'+i%26))+string(rune('a'+i/26))+".h")
		os.WriteFile(p, []byte(strings.Repeat("x", i)), 0o644)
		deps = append(deps, p)
	}
	deps = append(deps, filepath.Join(dir, "missing.h"))
	if a, b := DepsHash(deps), DepsHash(deps[:len(deps)-1]); a == b {
		t.Error("a missing file does not count")
	}
	all := DepsHash(deps)
	rev := append([]string(nil), deps...)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	if DepsHash(rev) != all {
		t.Error("the order of the files matters")
	}
	// A change of one file, with the same size and modification time.
	p := deps[10]
	st, _ := os.Stat(p)
	os.WriteFile(p, []byte(strings.Repeat("y", 10)), 0o644)
	os.Chtimes(p, st.ModTime(), st.ModTime())
	if DepsHash(deps) == all {
		t.Error("a change of the same size and time was not seen")
	}
	_ = time.Now
}

func TestJSONOfTheCacheFilesIsStable(t *testing.T) {
	// The variant file lists keep their order and the latest first.
	rn := new(Runner)
	dir := t.TempDir()
	dir = filepath.Join(dir, cacheSub)
	name := entryName("c", "x", "y")
	h := filepath.Join(t.TempDir(), "h")
	os.WriteFile(h, []byte("1"), 0o644)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(h, old, old)
	var hashes []string
	for i := 0; i < 6; i++ {
		os.WriteFile(h, []byte{byte('a' + i)}, 0o644)
		os.Chtimes(h, old, old)
		v := variant{Deps: []string{h}, DepsHash: DepsHash([]string{h})}
		hashes = append(hashes, v.DepsHash)
		rn.addVariant(dir, name, "c", "", v, true)
	}
	body, ok := rn.get(dir, name)
	if !ok {
		t.Fatal("no file")
	}
	var vf variantFile
	if err := json.Unmarshal(body, &vf); err != nil {
		t.Fatal(err)
	}
	if len(vf.Variants) != maxVariants || vf.Variants[0].DepsHash != hashes[5] || vf.Variants[1].DepsHash != hashes[4] {
		t.Errorf("%d variants, first %v", len(vf.Variants), vf.Variants[0].DepsHash == hashes[5])
	}
	// The contents of now find their variant, and an old one does too while it is kept.
	if v := rn.findVariant(dir, name, "c"); v == nil || v.DepsHash != hashes[5] {
		t.Error("the current contents have no variant")
	}
	os.WriteFile(h, []byte{'c'}, 0o644)
	os.Chtimes(h, old, old)
	if v := rn.findVariant(dir, name, "c"); v == nil || v.DepsHash != hashes[2] {
		t.Error("an older variant is not found")
	}
	os.WriteFile(h, []byte{'a'}, 0o644)
	os.Chtimes(h, old, old)
	if v := rn.findVariant(dir, name, "c"); v != nil {
		t.Error("a variant that was dropped is found")
	}
}

// A header list expires after a week: the cache cannot see every header that could hide one.
func TestCacheVariantExpires(t *testing.T) {
	rn := new(Runner)
	dir := filepath.Join(t.TempDir(), cacheSub)
	h := filepath.Join(t.TempDir(), "h")
	os.WriteFile(h, []byte("x"), 0o644)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(h, old, old)
	name := entryName("c", "x", "y")
	v := variant{Deps: []string{h}, DepsHash: DepsHash([]string{h}), Created: time.Now().Add(-maxVariantAge - time.Hour).UnixNano()}
	rn.addVariant(dir, name, "c", "", v, true)
	if rn.findVariant(dir, name, "c") != nil {
		t.Error("a header list of eight days was used")
	}
	v.Created = time.Now().Add(-maxVariantAge + time.Hour).UnixNano()
	rn.addVariant(dir, name, "c", "", v, true)
	if rn.findVariant(dir, name, "c") == nil {
		t.Error("a header list of six days was not used")
	}
}

// A Runner that has StableHeaders hashes a list of headers once (a command reads them once); one
// that has not sees an edit at once (the language server's Runner: the user edits headers).
func TestStableHeadersHashAListOnce(t *testing.T) {
	dir := t.TempDir()
	h := filepath.Join(dir, "a.h")
	if err := os.WriteFile(h, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := []string{h}
	stable, live := new(Runner), new(Runner)
	stable.StableHeaders()
	before := stable.depsHash(deps)
	if before != live.depsHash(deps) || before != DepsHash(deps) {
		t.Fatal("the hash of a list depends on the Runner")
	}
	if err := os.WriteFile(h, []byte("two!"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := live.depsHash(deps); got == before {
		t.Error("a Runner without StableHeaders did not see the edit")
	}
	if got := stable.depsHash(deps); got != before {
		t.Error("a Runner with StableHeaders hashed the list again")
	}
}

// Two lists of headers have two hashes, also for a Runner that remembers them.
func TestStableHeadersTellTwoListsApart(t *testing.T) {
	dir := t.TempDir()
	var lists [][]string
	for _, body := range []string{"one", "two"} {
		h := filepath.Join(dir, body+".h")
		if err := os.WriteFile(h, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		lists = append(lists, []string{h})
	}
	rn := new(Runner)
	rn.StableHeaders()
	if rn.depsHash(lists[0]) == rn.depsHash(lists[1]) {
		t.Error("two different lists of headers have the same hash")
	}
	if rn.depsHash(lists[0]) != DepsHash(lists[0]) || rn.depsHash(lists[1]) != DepsHash(lists[1]) {
		t.Error("the remembered hash is not the hash of the list")
	}
}
