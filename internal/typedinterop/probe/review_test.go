package probe

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
)

// valueOf probes the names (macros or constants) with real clang and returns
// the results by name.
func probeMacros(t *testing.T, dir string, includes []string, names ...string) map[string]*Result {
	t.Helper()
	var refs []*collect.Ref
	kinds := map[string]interop.Kind{}
	for _, n := range names {
		r := ref(pth(n))
		r.WantValue = true
		refs = append(refs, r)
		kinds[n] = interop.Macro
	}
	tab, err := new(Runner).Probe(context.Background(), clangConfig(t, dir), collect.NewSet(refs...), GenConfig{Includes: includes, Kinds: kinds})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*Result{}
	for i := range tab.Results {
		r := &tab.Results[i]
		out[strings.TrimPrefix(r.Key, "::")] = r
	}
	return out
}

// S1: a string literal deep in the tree (the "" of __builtin_nanf("")) is not the value.
func TestDecodeDeclStringLiteralMustBeTheInitializer(t *testing.T) {
	nan := `{"kind":"VarDecl","name":"gp_probe_0_s","type":{"qualType":"const int"},"inner":[
	  {"kind":"ExprWithCleanups","inner":[{"kind":"CallExpr","inner":[
	    {"kind":"ImplicitCastExpr","inner":[{"kind":"DeclRefExpr"}]},
	    {"kind":"MaterializeTemporaryExpr","inner":[{"kind":"ParenExpr","inner":[{"kind":"CallExpr","inner":[
	      {"kind":"ImplicitCastExpr","inner":[{"kind":"DeclRefExpr"}]},
	      {"kind":"ImplicitCastExpr","inner":[{"kind":"StringLiteral","value":"\"\""}]}]}]}]}]}]}]}`
	f, err := decodeDecl([]byte(nan))
	if err != nil || f.str != nil {
		t.Fatalf("NAN: %+v %v", f, err)
	}
	// A char pointer built from a literal deeper than the call's argument is no string value either.
	deep := `{"kind":"VarDecl","name":"gp_probe_0_s","type":{"qualType":"const char *const"},"inner":[
	  {"kind":"CallExpr","inner":[{"kind":"ImplicitCastExpr","inner":[{"kind":"DeclRefExpr"}]},
	    {"kind":"CallExpr","inner":[{"kind":"DeclRefExpr"},{"kind":"StringLiteral","value":"\"x\""}]}]}]}`
	if f, err := decodeDecl([]byte(deep)); err != nil || f.str != nil {
		t.Fatalf("deep: %+v %v", f, err)
	}
	direct := `{"kind":"VarDecl","name":"gp_probe_0_s","type":{"qualType":"const char *const"},"inner":[
	  {"kind":"CallExpr","inner":[{"kind":"ImplicitCastExpr","inner":[{"kind":"DeclRefExpr"}]},
	    {"kind":"ParenExpr","inner":[{"kind":"StringLiteral","value":"\"My\\nApp\""}]}]}]}`
	f, err = decodeDecl([]byte(direct))
	if err != nil || f.str == nil || *f.str != "My\nApp" {
		t.Fatalf("direct: %+v %v", f, err)
	}
}

func TestRealClangNanIsAFloatNotAString(t *testing.T) {
	res := probeMacros(t, t.TempDir(), []string{"<cmath>", "<cfloat>", "<climits>"}, "NAN", "INFINITY", "HUGE_VAL", "DBL_MAX", "INT_MAX")
	if v := res["NAN"].Value; v == nil || v.Kind != "float" || !math.IsNaN(v.Float) {
		t.Errorf("NAN: %+v (value %v)", res["NAN"], res["NAN"].Value)
	}
	if v := res["INFINITY"].Value; v == nil || v.Kind != "float" || !math.IsInf(v.Float, 1) {
		t.Errorf("INFINITY: %+v", res["INFINITY"].Value)
	}
	if v := res["DBL_MAX"].Value; v == nil || v.Float != math.MaxFloat64 {
		t.Errorf("DBL_MAX: %+v", res["DBL_MAX"].Value)
	}
	if v := res["INT_MAX"].Value; v == nil || v.Int.Int64() != math.MaxInt32 {
		t.Errorf("INT_MAX: %+v", res["INT_MAX"].Value)
	}
}

// S2: long double and unsigned __int128 constants are exact or have no value.
func TestRealClangWideConstants(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "wide.hpp", `#define HALF_LD 0.5L
#define BIG128 ((unsigned __int128)1 << 100)
#define HUGE128 ((unsigned __int128)1 << 127)
#define NEG128 (-((__int128)1 << 100))
#define PI_LD 3.14159265358979323846264338327950288L
`)
	res := probeMacros(t, dir, []string{"<cfloat>", `"wide.hpp"`}, "LDBL_MAX", "HALF_LD", "PI_LD", "BIG128", "HUGE128", "NEG128")
	if r := res["LDBL_MAX"]; r.Value != nil || !strings.Contains(r.ValueError, "not exactly representable") {
		t.Errorf("LDBL_MAX: value %v error %q", r.Value, r.ValueError)
	}
	if r := res["PI_LD"]; r.Value != nil || r.ValueError == "" {
		t.Errorf("PI_LD: value %v error %q", r.Value, r.ValueError)
	}
	if r := res["HALF_LD"]; r.Value == nil || r.Value.Kind != "float" || r.Value.Float != 0.5 {
		t.Errorf("HALF_LD: %+v %q", r.Value, r.ValueError)
	}
	if r := res["BIG128"]; r.Value == nil || r.Value.Int.BitLen() != 101 {
		t.Errorf("BIG128: %+v %q", r.Value, r.ValueError)
	}
	if r := res["NEG128"]; r.Value == nil || r.Value.Int.Sign() >= 0 || r.Value.Int.BitLen() != 101 {
		t.Errorf("NEG128: %+v %q", r.Value, r.ValueError)
	}
	if r := res["HUGE128"]; r.Value != nil || !strings.Contains(r.ValueError, "not exactly representable") {
		t.Errorf("HUGE128: %+v %q", r.Value, r.ValueError)
	}
}

// S3: names of the user's headers that look like the helpers' don't change a result.
func TestRealClangHelperNameCollisions(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "names.hpp", `struct gp_c { int a; };
struct gp_val { int b; };
namespace gp_pr { struct string { int c; }; }
#define LIMIT 7
`)
	res := probeMacros(t, dir, []string{`"names.hpp"`}, "LIMIT")
	if r := res["LIMIT"]; !r.OK || r.Base != "int" || r.Value == nil || r.Value.Int.Int64() != 7 {
		t.Errorf("LIMIT: %+v", r)
	}
	// A macro with a helper's name breaks the helpers themselves: the run fails, with a reason.
	writeHeader(t, dir, "macro.hpp", "#define gocpp_probe_internal_c 1\n")
	r := ref(pth("BUFSIZ"))
	_, err := new(Runner).Probe(context.Background(), clangConfig(t, dir), collect.NewSet(r), GenConfig{Includes: []string{"<cstdio>", `"macro.hpp"`}, Kinds: stdKinds})
	if err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("error %v", err)
	}
}

// S4a: a file rewritten with the same size and modification time, soon after the
// first hash, has another hash (the racy-git rule).
func TestHashFileRacy(t *testing.T) {
	f := filepath.Join(t.TempDir(), "h.hpp")
	if err := os.WriteFile(f, []byte("int a;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mt := time.Now().Add(-100 * time.Millisecond)
	os.Chtimes(f, mt, mt)
	h1 := hashFile(f)
	os.WriteFile(f, []byte("int b;\n"), 0o644)
	os.Chtimes(f, mt, mt)
	if h2 := hashFile(f); h2 == h1 {
		t.Error("a same-size rewrite with the same mtime kept the old hash")
	}
	// The hash taken while the file was fresh is not trusted later, when the file is
	// old enough for the stat shortcut: rewrite it again with the same size and mtime.
	h2 := hashFile(f)
	time.Sleep(2100 * time.Millisecond)
	os.WriteFile(f, []byte("int c;\n"), 0o644)
	os.Chtimes(f, mt, mt)
	if h3 := hashFile(f); h3 == h2 {
		t.Error("a hash taken within 2 s of the modification was served from the cache later")
	}
	// An old file is served from the cache (a changed mtime is noticed too).
	old := time.Now().Add(-time.Hour)
	os.Chtimes(f, old, old)
	h3 := hashFile(f)
	if hashFile(f) != h3 {
		t.Error("unstable hash")
	}
	hashMu.Lock()
	_, cached := hashCache[f]
	hashMu.Unlock()
	// (Where the system gives no change time there is no shortcut: the file is read every time.)
	fi, _ := os.Stat(f)
	if _, have := changeTime(fi); have != cached {
		t.Errorf("an old file's hash: kept %v, change time available %v", cached, have)
	}
}

// S4b: the keys have the same environment as the other caches, and the limits.
func TestKeyEnvironmentAndLimits(t *testing.T) {
	cfg := Config{CXX: build.CXX{Path: "/x/clang++", Family: build.Clang, Version: "21"}, Dir: t.TempDir()}
	src := simpleSource(t)
	base := PreKey(cfg, src)
	for _, name := range []string{"CCC_ADD_ARGS", "NIX_HARDENING_ENABLE", "COMPILER_PATH", "GCC_EXEC_PREFIX", "NIX_ENFORCE_PURITY",
		"NIX_CFLAGS_COMPILE_BEFORE_x", "NIX_CC_WRAPPER_TARGET_HOST_y", "DEVELOPER_DIR", "MACOSX_DEPLOYMENT_TARGET", "CPATH"} {
		t.Setenv(name, "something")
		if PreKey(cfg, src) == base {
			t.Errorf("the key ignores %s", name)
		}
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	c2 := cfg
	c2.MaxDecl = 12345
	c3 := cfg
	c3.MaxJSON = 1 << 20
	if PreKey(c2, src) == base || PreKey(c3, src) == base || PreKey(c2, src) == PreKey(c3, src) {
		t.Error("the key ignores MaxDecl or MaxJSON")
	}
	os.Unsetenv("CPATH")
	base = PreKey(cfg, src)
	c4 := cfg
	c4.MaxDecl, c4.MaxJSON = DefaultMaxDecl, DefaultMaxJSON
	if PreKey(c4, src) != base {
		t.Error("the default limits written out change the key")
	}
}

// A header that appears earlier in the include path than the one that was
// found hides it: the next run is a new one.
func TestRealClangShadowingHeader(t *testing.T) {
	dir := t.TempDir()
	inc1, inc2 := filepath.Join(dir, "inc1"), filepath.Join(dir, "inc2")
	os.Mkdir(inc1, 0o755)
	os.Mkdir(inc2, 0o755)
	writeHeader(t, inc2, "shadow.hpp", "#define SHADOWED 2\n")
	cfg := clangConfig(t, dir, "-I"+inc1, "-I", inc2)
	r := ref(pth("SHADOWED"))
	r.WantValue = true
	set := collect.NewSet(r)
	gc := GenConfig{Includes: []string{"<shadow.hpp>"}, Kinds: map[string]interop.Kind{"SHADOWED": interop.Macro}}
	rn := new(Runner)
	t1, err := rn.Probe(context.Background(), cfg, set, gc)
	if err != nil || t1.Result(0).Value == nil || t1.Result(0).Value.Int.Int64() != 2 {
		t.Fatalf("first: %v %+v", err, t1)
	}
	found := false
	for _, d := range t1.Deps {
		found = found || d == filepath.Join(inc1, "shadow.hpp")
	}
	if !found {
		t.Errorf("deps %v lack the place where a shadowing header would appear", t1.Deps)
	}
	writeHeader(t, inc1, "shadow.hpp", "#define SHADOWED 1\n")
	t2, err := rn.Probe(context.Background(), cfg, set, gc)
	if err != nil || t2.Cached || t2.Result(0).Value == nil || t2.Result(0).Value.Int.Int64() != 1 || t2.Key == t1.Key {
		t.Fatalf("second: %v cached %v %+v", err, t2.Cached, t2.Result(0))
	}
	// A quoted include: a file in the package folder.
	writeHeader(t, inc2, "q.hpp", "#define QV 2\n")
	r2 := ref(pth("QV"))
	r2.WantValue = true
	set2 := collect.NewSet(r2)
	gc2 := GenConfig{Includes: []string{`"q.hpp"`}, Kinds: map[string]interop.Kind{"QV": interop.Macro}}
	q1, err := rn.Probe(context.Background(), cfg, set2, gc2)
	if err != nil || q1.Result(0).Value == nil || q1.Result(0).Value.Int.Int64() != 2 {
		t.Fatalf("q first: %v %+v", err, q1.Result(0))
	}
	writeHeader(t, dir, "q.hpp", "#define QV 3\n")
	q2, err := rn.Probe(context.Background(), cfg, set2, gc2)
	if err != nil || q2.Cached || q2.Result(0).Value == nil || q2.Result(0).Value.Int.Int64() != 3 {
		t.Fatalf("q second: %v cached %v", err, q2.Cached)
	}
}

// S6: an unnamed type has no file position in a result.
func TestNormalizeType(t *testing.T) {
	for in, want := range map[string]string{
		"S::(unnamed struct at /abs/dir/edge.hpp:25:3)":                "S::(unnamed struct)",
		"(unnamed union at /a b/c.h:1:2) *":                            "(unnamed union) *",
		"std::pair<(unnamed struct at /x.h:3:3), int>":                 "std::pair<(unnamed struct), int>",
		"(lambda at /home/me/x.hpp:9:12)":                              "(lambda)",
		"(anonymous namespace)::T":                                     "(anonymous namespace)::T",
		"std::vector<int> at":                                          "std::vector<int> at",
		"no viable conversion from '(unnamed struct at /d/e.hpp:1:1)'": "no viable conversion from '(unnamed struct)'",
	} {
		if got := normalizeType(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestRealClangUnnamedTypesAreStableAcrossFolders(t *testing.T) {
	const hdr = "struct S { struct { int x; int y; } pos; union { int i; float f; } u; };\nstruct S gs;\n"
	var types []string
	for i := 0; i < 2; i++ {
		dir := t.TempDir()
		writeHeader(t, dir, "edge.hpp", hdr)
		r1 := ref(sel(pth("gs"), "pos"))
		r2 := ref(sel(pth("gs"), "u"))
		kinds := map[string]interop.Kind{"gs": interop.Value}
		tab, err := new(Runner).Probe(context.Background(), clangConfig(t, dir), collect.NewSet(r1, r2), GenConfig{Includes: []string{`"edge.hpp"`}, Kinds: kinds})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range tab.Results {
			if !r.OK || strings.Contains(r.Type, dir) || strings.Contains(r.Type, " at ") {
				t.Errorf("result %+v", r)
			}
			types = append(types, r.Type)
		}
	}
	if len(types) != 4 || types[0] != types[2] || types[1] != types[3] {
		t.Errorf("types differ between folders: %v", types)
	}
}

// S7b: the declarator decides, not the template arguments.
func TestSplitTypeTemplateArguments(t *testing.T) {
	cases := []struct {
		in       string
		base     string
		ref      string
		constant bool
	}{
		{"const std::vector<int *>", "const std::vector<int *>", "", true},
		{"const std::pair<const char *, int>", "const std::pair<const char *, int>", "", true},
		{"const std::vector<int *> &", "const std::vector<int *>", "&", true},
		{"const std::function<void (int)>", "const std::function<void (int)>", "", true},
		{"const std::array<int[3], 2>", "const std::array<int[3], 2>", "", true},
		{"std::vector<const char *>", "std::vector<const char *>", "", false},
		{"std::function<void (int)> &", "std::function<void (int)>", "&", false},
		{"std::vector<int (&)[4]>", "std::vector<int (&)[4]>", "", false},
		{"void (S::*)() &", "void (S::*)() &", "", false},
		{"void (S::*)() const &&", "void (S::*)() const &&", "", false},
		{"void (S::*)() const", "void (S::*)() const", "", false},
		{"void (S::*&)()", "void (S::*)()", "&", false},
		{"int (&)[4]", "int [4]", "&", false},
		{"std::map<int (&)[4], int> (&)[2]", "std::map<int (&)[4], int> [2]", "&", false},
	}
	for _, tc := range cases {
		var r Result
		r.splitType(tc.in, true)
		if r.Base != tc.base || r.Ref != tc.ref || r.Const != tc.constant {
			t.Errorf("%q: base %q ref %q const %v; want %q %q %v", tc.in, r.Base, r.Ref, r.Const, tc.base, tc.ref, tc.constant)
		}
	}
}

func TestProbeTimeoutFromTheEnvironment(t *testing.T) {
	t.Setenv(EnvTimeout, "7m")
	if got := effTimeout(Config{}); got != 7*time.Minute {
		t.Errorf("%v", got)
	}
	if got := effTimeout(Config{Timeout: time.Second}); got != time.Second {
		t.Errorf("%v", got)
	}
	t.Setenv(EnvTimeout, "garbage")
	if got := effTimeout(Config{}); got != DefaultTimeout {
		t.Errorf("%v", got)
	}
}
