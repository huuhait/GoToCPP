package probe

import (
	"context"
	"encoding/json"
	"github.com/huuhait/gocpp/internal/build"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// fakeCXX writes a shell script that stands for clang++ and returns a CXX
// for it.
func simpleSource(t testing.TB) *Source {
	t.Helper()
	return Generate(collect.NewSet(ref(pth("BUFSIZ"))), GenConfig{Kinds: stdKinds})
}

func TestDecodeDecl(t *testing.T) {
	f, err := decodeDecl([]byte(`{"kind":"TypeAliasDecl","name":"gp_probe_3_v","type":{"desugaredQualType":"gocpp_probe_internal_val<(__int128)-16>","qualType":"gocpp_probe_internal_val<gocpp_probe_internal_pr::enc((-16))>"},
	  "inner":[{"kind":"TemplateArgument","inner":[{"kind":"ConstantExpr","value":"-16"}]}]}`))
	if err != nil || f.value == nil || f.value.String() != "-16" {
		t.Fatalf("%+v %v", f, err)
	}
	f, err = decodeDecl([]byte(`{"kind":"VarDecl","name":"gp_probe_0_s","type":{"qualType":"const char *const"},"inner":[{"kind":"StringLiteral","value":"\"My\\nApp\\x41\\101\\\\\""}]}`))
	if err != nil || f.str == nil || *f.str != "My\nAppAA\\" {
		t.Fatalf("%+v %v", f, err)
	}
	f, _ = decodeDecl([]byte(`{"name":"gp_probe_1","type":{"qualType":"gocpp_probe_internal_c<int *const &>"}}`))
	if f.typ != "int *const &" {
		t.Errorf("type %q", f.typ)
	}
	if _, err := decodeDecl([]byte(`{`)); err == nil {
		t.Error("no error for bad JSON")
	}
}

func TestParseProbeName(t *testing.T) {
	for name, want := range map[string]probeName{
		"gp_probe_0": {0, RoleMain}, "gp_probe_12_alt": {12, RoleAlt}, "gp_probe_7_v": {7, RoleValue}, "gp_probe_7_s": {7, RoleString},
	} {
		if got, ok := parseProbeName(name); !ok || got != want {
			t.Errorf("%s: %v %v", name, got, ok)
		}
	}
	for _, bad := range []string{"", "gp_probe_", "gp_probe_x", "gp_probe_1_z", "gp_decl_1", "xgp_probe_1"} {
		if _, ok := parseProbeName(bad); ok {
			t.Errorf("%q parses", bad)
		}
	}
}

func TestParseDiags(t *testing.T) {
	out := `In file included from gp_probe.cpp:12:
/usr/include/x.h:3:5: error: unknown type name 'foo'
    3 | foo x;
gp_probe.cpp:70:20: warning: unused thing
/tmp/zz/gp_probe.cpp:71:9: error: no matching function for call to 'f'
/tmp/zz/gp_probe.cpp:5:3: note: candidate function not viable: requires 1 argument
/usr/include/y.h:9:1: note: declared here
clang++: error: linker command failed
/tmp/zz/gp_probe.cpp:80:1: fatal error: too many errors
`
	g := ParseDiags([]byte(out), "/tmp/zz/gp_probe.cpp")
	if len(g) != 4 {
		t.Fatalf("%d groups: %+v", len(g), g)
	}
	if g[0].Err.File != "/usr/include/x.h" || g[0].Err.Line != 3 || g[0].Err.Msg != "unknown type name 'foo'" {
		t.Errorf("group 0: %+v", g[0])
	}
	if g[1].Err.File != "gp_probe.cpp" || g[1].Err.Line != 71 || len(g[1].Notes) != 2 || g[1].Notes[0].File != "gp_probe.cpp" {
		t.Errorf("group 1: %+v", g[1])
	}
	if g[2].Err.File != "" || g[2].Err.Msg != "linker command failed" || g[3].Err.Severity != "fatal error" {
		t.Errorf("groups 2, 3: %+v %+v", g[2], g[3])
	}
}

func TestSplitType(t *testing.T) {
	cases := []struct {
		in       string
		expr     bool
		base     string
		ref      string
		cat      Category
		constant bool
	}{
		{"int", true, "int", "", CatPRValue, false},
		{"long &", true, "long", "&", CatLValue, false},
		{"const int &", true, "const int", "&", CatLValue, true},
		{"int &&", true, "int", "&&", CatXValue, false},
		{"const char *", true, "const char *", "", CatPRValue, false},
		{"char *const", true, "char *const", "", CatPRValue, true},
		{"char *const &", true, "char *const", "&", CatLValue, true},
		{"int (&)[4]", true, "int [4]", "&", CatLValue, false},
		{"void (&)(int)", true, "void (int)", "&", CatLValue, false},
		{"std::basic_string<char>", false, "std::basic_string<char>", "", CatNone, false},
		{"const std::vector<long>", true, "const std::vector<long>", "", CatPRValue, true},
		{"unsigned char[4] &", true, "unsigned char[4]", "&", CatLValue, false},
		{"const char *&", true, "const char *", "&", CatLValue, false},
		{"int *const&", true, "int *const", "&", CatLValue, true},
	}
	for _, tc := range cases {
		var r Result
		r.splitType(tc.in, tc.expr)
		if r.Base != tc.base || r.Ref != tc.ref || r.Category != tc.cat || r.Const != tc.constant {
			t.Errorf("%q: %+v", tc.in, r)
		}
	}
}

func TestConstJSON(t *testing.T) {
	var r Result
	r.OK = true
	r.splitType("unsigned long long", true)
	big, _ := new(big.Int).SetString("18446744073709551615", 10)
	r.Value = &Const{Kind: "int", Int: big}
	b, err := json.Marshal(r)
	if err != nil || !strings.Contains(string(b), `"value":{"kind":"int","value":"18446744073709551615"}`) {
		t.Errorf("%s %v", b, err)
	}
	if got := (&Const{Kind: "string", Str: "a\"b"}).String(); got != `"a\"b"` {
		t.Errorf("string %s", got)
	}
}

func TestUnquoteC(t *testing.T) {
	for in, want := range map[string]string{
		`"abc"`:         "abc",
		`"a\nb\tc"`:     "a\nb\tc",
		`"\"q\""`:       `"q"`,
		`"\\"`:          `\`,
		`"\x41\x4a"`:    "AJ",
		`"\0"`:          "\x00",
		`"\101\1011"`:   "AA1",
		`""`:            "",
		`"caf\303\251"`: "café",
	} {
		raw, _ := json.Marshal(in)
		got, err := unquoteC(raw)
		if err != nil || got != want {
			t.Errorf("%s: %q %v, want %q", in, got, err, want)
		}
	}
	if _, err := unquoteC([]byte(`"no quotes"`)); err == nil {
		t.Error("no error without quotes")
	}
}

// ------------------------------------------------------------ real clang

func clangConfig(t testing.TB, dir string, flags ...string) Config {
	return Config{CXX: tsupport.Clang(t), Dir: dir, Flags: flags}
}

func writeHeader(t testing.TB, dir, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRealClangBasics(t *testing.T) {
	cfg := clangConfig(t, t.TempDir())
	set := collect.NewSet(ref(pth("BUFSIZ")), ref(call(pth("strlen"), lit(`"abc"`))), ref(call(pth("strlen"), lv(str()))))
	tab, err := new(Runner).Probe(context.Background(), cfg, set, GenConfig{Includes: []string{"<cstdio>", "<cstring>"}, Kinds: stdKinds})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range tab.Results {
		if r.OK {
			got[r.Key] = r.Category.String() + " " + r.Base
		} else {
			got[r.Key] = "error " + r.Error
		}
	}
	if got["::BUFSIZ"] != "prvalue int" || got[`call(::strlen|lit("abc"))`] != "prvalue unsigned long" ||
		!strings.HasPrefix(got["call(::strlen|arg&(stand:string()))"], "error no viable conversion") {
		t.Errorf("results %v", got)
	}
	if v := tab.Result(0).Value; v == nil || v.Int.Int64() != 8192 {
		t.Errorf("BUFSIZ value %v", v)
	}
	if tab.Timing.Total <= 0 || tab.Timing.Compile <= 0 || tab.Stats.Decls == 0 || len(tab.Deps) == 0 || tab.Key == "" {
		t.Errorf("timing %+v stats %+v deps %d", tab.Timing, tab.Stats, len(tab.Deps))
	}
}

func (c Category) String() string { return string(c) }

func TestRealClangFlagsReachTheProbe(t *testing.T) {
	cfg := clangConfig(t, t.TempDir(), "-DGP_X=5", "-DGP_Y=(GP_X*2)")
	kinds := map[string]interop.Kind{"GP_X": interop.Macro, "GP_Y": interop.Macro}
	r1, r2 := ref(pth("GP_X")), ref(pth("GP_Y"))
	r1.WantValue, r2.WantValue = true, true
	tab, err := new(Runner).Probe(context.Background(), cfg, collect.NewSet(r1, r2), GenConfig{Kinds: kinds})
	if err != nil {
		t.Fatal(err)
	}
	if v := tab.Result(0).Value; v == nil || v.Int.Int64() != 5 {
		t.Errorf("GP_X: %+v", tab.Result(0))
	}
	if v := tab.Result(1).Value; v == nil || v.Int.Int64() != 10 {
		t.Errorf("GP_Y: %+v", tab.Result(1))
	}
}

func TestRealClangErrorInAnUnrelatedHeader(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "bad.hpp", "int broken = ;\nint fine() { return 1; }\n")
	cfg := clangConfig(t, dir)
	set := collect.NewSet(ref(pth("BUFSIZ")), ref(call(pth("fine"))))
	kinds := map[string]interop.Kind{"BUFSIZ": interop.Macro, "fine": interop.Value}
	tab, err := new(Runner).Probe(context.Background(), cfg, set, GenConfig{Includes: []string{"<cstdio>", `"bad.hpp"`}, Kinds: kinds})
	if err != nil {
		t.Fatal(err)
	}
	if len(tab.HeaderErrors) == 0 || !strings.HasSuffix(tab.HeaderErrors[0].File, "bad.hpp") {
		t.Errorf("header errors %+v", tab.HeaderErrors)
	}
	for _, r := range tab.Results {
		if !r.OK {
			t.Errorf("the probe %s failed: %s", r.Key, r.Error)
		}
	}
}

func TestRealClangErrorsAreAttributedToTheirProbe(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "lib.hpp", "int f(int); int f(const char*);\n")
	cfg := clangConfig(t, dir)
	set := collect.NewSet(ref(call(pth("f"), lit("1"))), ref(call(pth("f"), lit("1"), lit("2"))), ref(call(pth("f"), lit(`"s"`))), ref(call(pth("g"))))
	kinds := map[string]interop.Kind{"f": interop.Value, "g": interop.NotFound}
	tab, err := new(Runner).Probe(context.Background(), cfg, set, GenConfig{Includes: []string{`"lib.hpp"`}, Kinds: kinds})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]*Result{}
	for i := range tab.Results {
		by[tab.Results[i].Key] = &tab.Results[i]
	}
	if r := by["call(::f|lit(1))"]; !r.OK || r.Base != "int" {
		t.Errorf("f(1): %+v", r)
	}
	bad := by["call(::f|lit(1),lit(2))"]
	if bad.OK || !strings.Contains(bad.Error, "no matching function for call to 'f'") || len(bad.Notes) < 2 {
		t.Fatalf("f(1, 2): %+v", bad)
	}
	for _, n := range bad.Notes {
		if !strings.Contains(n.Msg, "candidate function") || n.Severity != "note" {
			t.Errorf("note %+v", n)
		}
	}
	if bad.ErrorAt == nil || bad.ErrorAt.File != "gp_probe.cpp" {
		t.Errorf("error at %+v", bad.ErrorAt)
	}
	if r := by["call(::f|lit(\"s\"))"]; !r.OK || r.Base != "int" {
		t.Errorf("f(\"s\"): %+v", r)
	}
	if r := by["call(::g|)"]; r.OK || !strings.Contains(r.Error, "'g'") {
		t.Errorf("g(): %+v", r)
	}
}

func TestRunnerMemoAndHeaderEdits(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "lib.hpp", "#define LIBV 1\n")
	cfg := clangConfig(t, dir)
	r := ref(pth("LIBV"))
	r.WantValue = true
	set := collect.NewSet(r)
	gc := GenConfig{Includes: []string{`"lib.hpp"`}, Kinds: map[string]interop.Kind{"LIBV": interop.Macro}}
	rn := new(Runner)
	value := func(tab *Table) int64 { return tab.Result(0).Value.Int.Int64() }
	t1, err := rn.Probe(context.Background(), cfg, set, gc)
	if err != nil || value(t1) != 1 || t1.Cached {
		t.Fatalf("first: %v %+v", err, t1)
	}
	t2, err := rn.Probe(context.Background(), cfg, set, gc)
	if err != nil || !t2.Cached || rn.Runs() != 1 || value(t2) != 1 || t2.Key != t1.Key {
		t.Fatalf("second: %v cached %v runs %d", err, t2.Cached, rn.Runs())
	}
	// An edit of the header (same size, and within the same second) makes another key
	// and a new run.
	time.Sleep(2100 * time.Millisecond)
	writeHeader(t, dir, "lib.hpp", "#define LIBV 2\n")
	t3, err := rn.Probe(context.Background(), cfg, set, gc)
	if err != nil {
		t.Fatalf("third: %v", err)
	}
	if t3.Cached || rn.Runs() != 2 || value(t3) != 2 || t3.Key == t1.Key {
		t.Fatalf("third: cached %v runs %d key same %v", t3.Cached, rn.Runs(), t3.Key == t1.Key)
	}
	// Another probe file is another run, with the same compiler and flags.
	r2 := ref(call(pth("nothing")))
	t4, err := rn.Probe(context.Background(), cfg, collect.NewSet(r, r2), gc)
	if err != nil || t4.Cached || rn.Runs() != 3 {
		t.Fatalf("fourth: %v %v %d", err, t4.Cached, rn.Runs())
	}
}

func TestKeys(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "h.hpp", "int a;\n")
	cfg := Config{CXX: build.CXX{Path: filepath.Join(dir, "clang++"), Family: build.Clang, Version: "21"}, Dir: dir, Flags: []string{"-DA=1"}}
	src := simpleSource(t)
	base := Key(cfg, src, []string{filepath.Join(dir, "h.hpp")})
	if base != Key(cfg, src, []string{filepath.Join(dir, "h.hpp")}) {
		t.Fatal("the key is not stable")
	}
	cfg2 := cfg
	cfg2.Flags = []string{"-DA=2"}
	cfg3 := cfg
	cfg3.CXX.Version = "other"
	cfg4 := cfg
	cfg4.Dir = t.TempDir()
	src2 := Generate(collect.NewSet(ref(pth("BUFSIZ")), ref(pth("EOF"))), GenConfig{Kinds: stdKinds})
	t.Setenv("CPATH", "/somewhere")
	different := map[string]string{
		"flags":   Key(cfg2, src, []string{filepath.Join(dir, "h.hpp")}),
		"version": Key(cfg3, src, []string{filepath.Join(dir, "h.hpp")}),
		"folder":  Key(cfg4, src, []string{filepath.Join(dir, "h.hpp")}),
		"source":  Key(cfg, src2, []string{filepath.Join(dir, "h.hpp")}),
		"env":     Key(cfg, src, []string{filepath.Join(dir, "h.hpp")}),
		"missing": Key(cfg, src, []string{filepath.Join(dir, "h.hpp"), filepath.Join(dir, "nothing.hpp")}),
	}
	seen := map[string]string{base: "base"}
	for name, k := range different {
		if k == base {
			t.Errorf("the key doesn't depend on the %s", name)
		}
		if other, dup := seen[k]; dup {
			t.Errorf("keys of %s and %s are equal", name, other)
		}
		seen[k] = name
	}
	// The content, not the time: the same bytes give the same key after a touch;
	// other bytes of the same size give another key.
	time.Sleep(2100 * time.Millisecond)
	t.Setenv("CPATH", "")
	k0 := Key(cfg, src, []string{filepath.Join(dir, "h.hpp")})
	os.Chtimes(filepath.Join(dir, "h.hpp"), time.Now(), time.Now())
	if k1 := Key(cfg, src, []string{filepath.Join(dir, "h.hpp")}); k1 != k0 {
		t.Error("a touch changed the key")
	}
	writeHeader(t, dir, "h.hpp", "int b;\n")
	if k2 := Key(cfg, src, []string{filepath.Join(dir, "h.hpp")}); k2 == k0 {
		t.Error("an edit of the same size didn't change the key")
	}
}

func TestTableJSON(t *testing.T) {
	cfg := clangConfig(t, t.TempDir())
	tab, err := new(Runner).Probe(context.Background(), cfg, collect.NewSet(ref(pth("BUFSIZ"))), GenConfig{Includes: []string{"<cstdio>"}, Kinds: stdKinds})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(tab)
	if err != nil {
		t.Fatal(err)
	}
	var back struct {
		Results []struct {
			OK    bool
			Type  string
			Value struct{ Kind, Value string }
		}
		Timing struct{ Total int64 }
	}
	if err := json.Unmarshal(b, &back); err != nil || len(back.Results) != 1 || back.Results[0].Value.Value != "8192" || back.Timing.Total == 0 {
		t.Errorf("%s %v", b, err)
	}
}

// An error inside a header that a probe line caused (the instantiation of a
// class template that the line asked for) belongs to that line, not to the
// header errors.
func TestRealClangInstantiationErrorBelongsToTheProbe(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "lib.hpp", "template <class T> struct Bad { static_assert(sizeof(T) == 0, \"never\"); T v; };\nstruct Fine { int v; };\n")
	kinds := map[string]interop.Kind{"Bad": interop.Template, "Fine": interop.Type}
	bad := sel(lv(index(pth("Bad"), nsp("C", "int"))), "v")
	fine := sel(lv(pth("Fine")), "v")
	tab, err := new(Runner).Probe(context.Background(), clangConfig(t, dir), collect.NewSet(ref(bad), ref(fine)), GenConfig{Includes: []string{`"lib.hpp"`}, Kinds: kinds})
	if err != nil {
		t.Fatal(err)
	}
	if len(tab.HeaderErrors) != 0 {
		t.Errorf("header errors %+v: the instantiation's error is the probe's", tab.HeaderErrors)
	}
	var okFine, failedBad bool
	for _, r := range tab.Results {
		switch {
		case strings.Contains(r.Key, "Fine"):
			okFine = r.OK && r.Base == "int"
		case strings.Contains(r.Key, "Bad"):
			failedBad = !r.OK && strings.Contains(r.Error, "never")
		}
	}
	if !okFine || !failedBad {
		t.Errorf("results %+v", tab.Results)
	}
}
