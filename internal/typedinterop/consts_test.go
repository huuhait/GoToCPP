package typedinterop

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// The constant probe of step 12.8 (gap E1, ADR-097) against real clang: the header zoo of
// testdata/consts has every shape of constant (and of thing that is not one) that a program can
// ask for in an array length, a const declaration, a case or a shift count.

// zooHeaders copies the zoo into a new folder, with old modification times (the cache does not
// keep a header that was written in the last two seconds).
func zooHeaders(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"macros.hpp", "second.hpp", "collide.hpp"} {
		b, err := os.ReadFile(filepath.Join("testdata", "consts", name))
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// hp is a name through the header package macros (macros.NAME).
func hp(parts ...string) check.CppPath { return check.CppPath{Hdr: "macros", Parts: parts} }

// text is the one-line form of an answer that the tests compare.
func text(a check.ConstAnswer, ok bool) string {
	if !ok {
		return "absent"
	}
	switch a.Kind {
	case check.ConstInt:
		return "int " + a.Int.String()
	case check.ConstChar:
		return "char " + a.Int.String()
	case check.ConstBool:
		return "bool " + a.Int.String()
	case check.ConstFloat:
		return fmt.Sprintf("float %v", a.Float)
	case check.ConstString:
		return fmt.Sprintf("string %q", a.Str)
	}
	if a.NotConst {
		return "not constant"
	}
	return "unknown"
}

func ask(t *testing.T, dir string, flags, includes []string, paths []check.CppPath, opts Options) map[string]check.ConstAnswer {
	t.Helper()
	opts.Clang = tsupport.Clang(t)
	ans, err := constValues(context.Background(), check.ConstQuery{Dir: dir, Flags: flags, Includes: includes, Paths: paths}, opts)
	if err != nil {
		t.Fatal(err)
	}
	return ans
}

func TestConstZoo(t *testing.T) {
	dir := zooHeaders(t)
	type tc struct {
		path check.CppPath
		want string
	}
	var cases []tc
	add := func(want string, paths ...check.CppPath) {
		for _, p := range paths {
			cases = append(cases, tc{p, want})
		}
	}
	ints := map[string]string{
		"M_INT": "42", "M_NEG": "-7", "M_BIG": "2147483648", "M_U64": "18446744073709551615", "M_I64MIN": "-9223372036854775808",
		"M_ALIAS": "42", "M_ALIAS2": "42", "M_EXPR": "85", "M_CAST": "5", "M_SIZEOF": "4", "M_SHIFT": "1024", "M_HEX": "255",
		"M_OCT": "511", "M_BIN": "11", "M_LL": "123456789012", "M_TERNARY": "1", "M_I128": "1267650600228229401496703205376",
		"kMax": "16", "kConstInt": "8", "RED": "0", "GREEN": "5", "BLUE": "6", "HUGE_ENUM": "1099511627776", "kFav": "5",
		"REDEF": "2", "PLAT": "1", "FLAGGED": "0",
	}
	for name, v := range ints {
		add("int "+v, hp(name))
	}
	for name, v := range map[string]string{"M_FLT": "0.1", "M_F32": "1.5", "M_PI2": "6.283185307179586", "M_HALF": "0.5", "M_BIGF": "1e+300",
		"M_TINY": "5e-324", "M_NEGF": "-2.5", "M_LD": "0.5", "kHalf": "0.5"} {
		add("float "+v, hp(name))
	}
	add(`string "MyApp\n\"q\" AA"`, hp("M_STR"))
	add(`string ""`, hp("M_EMPTY"))
	add(`string "héllo"`, hp("M_UTF8"))
	add(`string "a\x00b"`, hp("M_NUL"))
	add(`string "\t\\\a"`, hp("M_TAB"))
	add("char 97", hp("M_CHR"))
	add("char 10", hp("M_CHR2"))
	add("char 92", hp("M_CHR3"))
	add("char 120", hp("M_WCHR"))
	add("char 121", hp("M_C16"))
	add("char 47", hp("kSep"))
	add("bool 1", hp("M_BOOL"))
	add("bool 0", hp("M_NOTB"))
	add("int 3", hp("kPt", "x"))
	add("int 4", hp("kPt", "y"))
	add("int 64", hp("Limits", "kCap"))
	add("int 3", hp("geo", "kDim"))
	add("int 6", hp("geo", "deep", "kN"))
	add("not constant", hp("M_NAN"), hp("M_INF"), hp("M_FN"), hp("M_SIDE"), hp("M_CALL"), hp("M_PTR"), hp("M_STMT"), hp("kMutable"),
		hp("kRuntime"), hp("kName"), hp("kArr"), hp("Pt"), hp("kPt"))
	add("not constant", hp("WHERE"), hp("WHICH"), hp("WHICHNAME"), hp("CNT"), hp("STAMP"), hp("CLOCK"), hp("MODIFIED"), hp("BASE"), hp("LEVEL"),
		hp("VIA"), hp("VIA2"), hp("VIAFN"))
	add("int 99", hp("kDim"))
	add("string \"mentions __LINE__ and __FILE__ in a string only\"", hp("INSTR"))
	// An enumerator of an enum class is no integer, a name that the headers do not declare (#undef)
	// has no answer: both stay as the user wrote them.
	add("absent", hp("Mode", "A"), hp("UNDEF_LATER"), hp("nothing_here"))
	var paths []check.CppPath
	for _, c := range cases {
		paths = append(paths, c.path)
	}
	ans := ask(t, dir, nil, []string{`"./macros.hpp"`}, paths, Options{})
	for _, c := range cases {
		a, ok := ans[c.path.Key()]
		if got := text(a, ok); got != c.want {
			t.Errorf("%s: %s, want %s", strings.Join(c.path.Parts, "::"), got, c.want)
		}
	}
}

// Why a name is not a constant is told in words, and a name that is a type says so.
func TestConstReasons(t *testing.T) {
	dir := zooHeaders(t)
	ans := ask(t, dir, nil, []string{`"./macros.hpp"`}, []check.CppPath{hp("M_FN"), hp("M_SIDE"), hp("M_NAN"), hp("Pt"), hp("M_INF"), hp("kPt"), hp("kName"), hp("M_RAW")}, Options{})
	for path, want := range map[string]string{
		"M_FN":   "it is a macro that does not expand to an expression (it is empty, function-like or a statement): use of undeclared identifier 'M_FN'",
		"M_SIDE": "the C++ compiler cannot evaluate it at compile time (non-type template argument is not a constant expression)",
		"M_NAN":  "its value NaN is not a number that a Go constant can have (Go constants are finite)",
		"Pt":     "it names a type, not a value",
		"kPt":    "its type const Pt is not a number, a character, a boolean or a string literal, the only things that Go constants can be",
		"M_RAW":  "the string has bytes that are not valid UTF-8, which the compiler's dump cannot give back exactly",
		"kName":  "its type const char *const is not a number, a character, a boolean or a string literal, the only things that Go constants can be",
		"M_INF":  "its value +Inf is not a number that a Go constant can have (Go constants are finite)",
	} {
		if got := ans[hp(path).Key()].Why; got != want {
			t.Errorf("%s: %q, want %q", path, got, want)
		}
	}
}

// Names through the C and C++ standard headers, a namespace and a flag.
func TestConstStdAndFlags(t *testing.T) {
	dir := zooHeaders(t)
	std := func(parts ...string) check.CppPath { return check.CppPath{Ns: "::std", Parts: parts} }
	hdr := func(h string, parts ...string) check.CppPath { return check.CppPath{Hdr: h, Parts: parts} }
	paths := []check.CppPath{hdr("cstdio", "BUFSIZ"), hdr("cstdio", "EOF"), hdr("climits", "INT_MAX"), hdr("climits", "UINT_MAX"),
		hdr("climits", "LONG_MAX"), hdr("climits", "CHAR_BIT"), hdr("cmath", "M_PI"), std("numbers", "pi"), hdr("macros", "FLAGGED")}
	ans := ask(t, dir, []string{"-DFROM_FLAG=7"}, []string{"<cstdio>", "<climits>", "<cmath>", "<numbers>", `"./macros.hpp"`}, paths, Options{})
	for i, want := range []string{"int 8192", "int -1", "int 2147483647", "int 4294967295", "int 9223372036854775807", "int 8", "float 3.141592653589793",
		"float 3.141592653589793", "int 7"} {
		if got := text(ans[paths[i].Key()], true); got != want {
			t.Errorf("%v: %s, want %s", paths[i], got, want)
		}
	}
}

// A header that redefines a macro of another: the value is the one at the end of the includes, in
// the order of the includes, as the build sees it.
func TestConstRedefinedByAnotherHeader(t *testing.T) {
	dir := zooHeaders(t)
	paths := []check.CppPath{hp("M_INT"), hp("M_ALIAS"), hp("M_EXPR"), hp("SECOND_ONLY")}
	for _, includes := range [][]string{{`"./macros.hpp"`, `"./second.hpp"`}, {`"./second.hpp"`, `"./macros.hpp"`}} {
		ans := ask(t, dir, nil, includes, paths, Options{})
		for i, want := range []string{"int 43", "int 43", "int 87", "int 44"} {
			if got := text(ans[paths[i].Key()], true); got != want {
				t.Errorf("%v %v: %s, want %s", includes, paths[i].Parts, got, want)
			}
		}
	}
	ans := ask(t, dir, nil, []string{`"./macros.hpp"`}, []check.CppPath{hp("M_INT"), hp("SECOND_ONLY")}, Options{})
	if got := text(ans[hp("M_INT").Key()], true); got != "int 42" {
		t.Errorf("without the second header: %s", got)
	}
	if _, ok := ans[hp("SECOND_ONLY").Key()]; ok {
		t.Error("a macro of a header that is not included has an answer")
	}
}

// No clang, a header that is not found and another target give no answer, no error: the names
// stay opaque.
func TestConstNoAnswer(t *testing.T) {
	dir := zooHeaders(t)
	clang := tsupport.Clang(t)
	paths := []check.CppPath{hp("M_INT")}
	ans, err := constValues(context.Background(), check.ConstQuery{Dir: dir, Includes: []string{`"./nothere.hpp"`}, Paths: paths}, Options{Clang: clang})
	if ans != nil || err != nil {
		t.Errorf("a missing header: %v, %v", ans, err)
	}
	q := check.ConstQuery{
		Dir: dir, Includes: []string{`"./macros.hpp"`}, Paths: paths, GOOS: "plan9", GOARCH: "riscv64",
	}
	ans, err = constValues(context.Background(), q, Options{Clang: clang})
	if ans != nil || err != nil {
		t.Errorf("another target: %v, %v", ans, err)
	}
	t.Setenv("GOCPP_CLANG", "no-such-clang-xyz")
	ans, err = constValues(context.Background(), check.ConstQuery{Dir: dir, Includes: []string{`"./macros.hpp"`}, Paths: paths}, Options{})
	if ans != nil || err != nil {
		t.Errorf("no clang: %v, %v", ans, err)
	}
}

// The answers are cached per name under the contents of the headers (step 12.7): a second process
// runs no compiler, a header edit is seen, a new name runs only itself.
func TestConstCache(t *testing.T) {
	tsupport.Clang(t)
	dir := zooHeaders(t)
	old := time.Now().Add(-time.Hour)
	for _, n := range []string{"macros.hpp", "second.hpp", "collide.hpp"} {
		os.Chtimes(filepath.Join(dir, n), old, old)
	}
	t.Setenv(probe.EnvProbeCache, t.TempDir())
	t.Setenv(probe.EnvNoProbeCache, "")
	defer probe.SetRacyWindow(0)()
	includes := []string{`"./macros.hpp"`}
	paths := []check.CppPath{hp("M_INT"), hp("M_STR"), hp("M_PI2"), hp("M_SIDE")}
	rn1 := new(probe.Runner)
	ans := ask(t, dir, nil, includes, paths, Options{Runner: rn1})
	if rn1.Runs() != 1 || text(ans[hp("M_INT").Key()], true) != "int 42" {
		t.Fatalf("cold: %d runs, %v", rn1.Runs(), ans)
	}
	rn2 := new(probe.Runner)
	warm := ask(t, dir, nil, includes, paths, Options{Runner: rn2})
	if rn2.Runs() != 0 {
		t.Errorf("warm: %d compiler runs", rn2.Runs())
	}
	for _, p := range paths {
		if text(ans[p.Key()], true) != text(warm[p.Key()], true) {
			t.Errorf("%v: cold %s, warm %s", p.Parts, text(ans[p.Key()], true), text(warm[p.Key()], true))
		}
	}
	// One more name: only it runs.
	rn3 := new(probe.Runner)
	more := ask(t, dir, nil, includes, append(paths, hp("M_HEX")), Options{Runner: rn3})
	if rn3.Runs() != 1 || text(more[hp("M_HEX").Key()], true) != "int 255" {
		t.Errorf("one more name: %d runs", rn3.Runs())
	}
	// A header edit with the same size and time: the value changes.
	f := filepath.Join(dir, "macros.hpp")
	st, _ := os.Stat(f)
	b, _ := os.ReadFile(f)
	if err := os.WriteFile(f, []byte(strings.Replace(string(b), "#define M_INT 42", "#define M_INT 44", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(f, st.ModTime(), st.ModTime())
	rn4 := new(probe.Runner)
	edited := ask(t, dir, nil, includes, paths, Options{Runner: rn4})
	if text(edited[hp("M_INT").Key()], true) != "int 44" || rn4.Runs() == 0 {
		t.Errorf("after the edit: %s, %d runs", text(edited[hp("M_INT").Key()], true), rn4.Runs())
	}
	// A flag is another key.
	rn5 := new(probe.Runner)
	flagged := ask(t, dir, []string{"-DFROM_FLAG=5"}, includes, []check.CppPath{hp("FLAGGED")}, Options{Runner: rn5})
	if text(flagged[hp("FLAGGED").Key()], true) != "int 5" {
		t.Errorf("flag: %s", text(flagged[hp("FLAGGED").Key()], true))
	}
}

func TestConstBigIntIsExact(t *testing.T) {
	dir := zooHeaders(t)
	ans := ask(t, dir, nil, []string{`"./macros.hpp"`}, []check.CppPath{hp("M_I128")}, Options{})
	want, _ := new(big.Int).SetString("1267650600228229401496703205376", 10)
	if a := ans[hp("M_I128").Key()]; a.Int == nil || a.Int.Cmp(want) != 0 {
		t.Errorf("%v", a)
	}
}

// The macros whose value is the probe file's own are refused, with the builtin that makes them so, also
// through other macros; a name inside a string is no use.
func TestConstVolatileMacros(t *testing.T) {
	dir := zooHeaders(t)
	names := map[string]string{"WHERE": "__LINE__", "WHICH": "__FILE__", "WHICHNAME": "__FILE_NAME__", "CNT": "__COUNTER__", "STAMP": "__DATE__",
		"CLOCK": "__TIME__", "MODIFIED": "__TIMESTAMP__", "BASE": "__BASE_FILE__", "LEVEL": "__INCLUDE_LEVEL__", "VIA": "__LINE__", "VIA2": "__LINE__"}
	var paths []check.CppPath
	for n := range names {
		paths = append(paths, hp(n))
	}
	paths = append(paths, hp("INSTR"), hp("M_INT"))
	ans := ask(t, dir, nil, []string{`"./macros.hpp"`}, paths, Options{})
	for n, builtin := range names {
		a := ans[hp(n).Key()]
		want := "its value depends on where or when it is used (the macro expands to " + builtin + "), so the probe's value would not be the program's"
		if !a.NotConst || a.Why != want {
			t.Errorf("%s: %+v, want not constant: %s", n, a, want)
		}
	}
	if a := ans[hp("INSTR").Key()]; a.Kind != check.ConstString {
		t.Errorf("a macro with the builtin's name in a string: %+v", a)
	}
	if a := ans[hp("M_INT").Key()]; a.Kind != check.ConstInt {
		t.Errorf("M_INT: %+v", a)
	}
}

// A header with macros named like the probe's own template parameters and variables (N, T, V, K, C, U, t,
// v, a) does not stop the probe.
func TestConstProbeHelpersSurviveMacros(t *testing.T) {
	dir := zooHeaders(t)
	ans := ask(t, dir, nil, []string{`"./collide.hpp"`, `"./macros.hpp"`}, []check.CppPath{hp("AFTER"), hp("N"), hp("M_INT"), hp("M_STR")}, Options{})
	for path, want := range map[string]string{"AFTER": "int 13", "N": "int 4", "M_INT": "int 42", "M_STR": `string "MyApp\n\"q\" AA"`} {
		if got := text(ans[hp(path).Key()], true); got != want {
			t.Errorf("%s: %s, want %s", path, got, want)
		}
	}
}

// A header path and a namespace path with the same names are two names; two header packages that are
// namespaces too are asked in groups.
func TestConstSameNamesTwoMeanings(t *testing.T) {
	dir := zooHeaders(t)
	global := hp("kDim")
	inNs := check.CppPath{Ns: "::geo", Parts: []string{"kDim"}}
	ans := ask(t, dir, nil, []string{`"./macros.hpp"`}, []check.CppPath{global, inNs}, Options{})
	if g, n := text(ans[global.Key()], true), text(ans[inNs.Key()], true); g != "int 99" || n != "int 3" {
		t.Errorf("global %s, namespace %s", g, n)
	}
	// Two header packages: geo.kDim looks into the namespace geo first, other.kDim is the global one.
	geoH := check.CppPath{Hdr: "geo", Parts: []string{"kDim"}}
	otherH := check.CppPath{Hdr: "other", Parts: []string{"kDim"}}
	ans = ask(t, dir, nil, []string{`"./macros.hpp"`}, []check.CppPath{geoH, otherH}, Options{})
	if g, o := text(ans[geoH.Key()], true), text(ans[otherH.Key()], true); g != "int 3" || o != "int 99" {
		t.Errorf("geo.kDim %s, other.kDim %s", g, o)
	}
}

// The probe says how to spell each name in the program.
func TestConstSpelling(t *testing.T) {
	dir := zooHeaders(t)
	paths := []check.CppPath{hp("M_INT"), hp("kMax"), hp("geo", "deep", "kN"), hp("kPt", "x"), hp("Limits", "kCap"), hp("RED")}
	ans := ask(t, dir, nil, []string{`"./macros.hpp"`}, paths, Options{})
	for i, want := range []string{"M_INT", "::kMax", "::geo::deep::kN", "(::kPt).x", "::Limits::kCap", "::RED"} {
		if got := ans[paths[i].Key()].Spelling; got != want {
			t.Errorf("%v: spelling %q, want %q", paths[i].Parts, got, want)
		}
	}
}
