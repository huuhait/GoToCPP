package check

import (
	"context"
	"errors"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"math/big"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"testing"
)

// The tests of the substitution of C/C++ constants (constsubst.go, Phase 12 step 12.8). They run no
// compiler: Config.Constants is a function of the test that answers from a table, and the header
// "<lib.h>" does not exist (fakeProbeConfig).

// constAnswers builds a Config.Constants over a table of answers for names of the header package lib
// (lib.NAME) and of the namespace std (std.NAME), and records the queries.
type constAnswers struct {
	mu      sync.Mutex
	hdr     map[string]ConstAnswer
	ns      map[string]ConstAnswer
	queries []ConstQuery
	err     error
}

func (c *constAnswers) fn(_ context.Context, q ConstQuery) (map[string]ConstAnswer, error) {
	c.mu.Lock()
	c.queries = append(c.queries, q)
	c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	out := make(map[string]ConstAnswer)
	for _, p := range q.Paths {
		name := strings.Join(p.Parts, ".")
		if a, ok := c.hdr[name]; ok && p.Ns == "" {
			a.Spelling = strings.Join(p.Parts, "::")
			out[p.Key()] = a
		}
		if a, ok := c.ns[name]; ok && p.Ns != "" {
			a.Spelling = p.Ns + "::" + strings.Join(p.Parts, "::")
			out[p.Key()] = a
		}
	}
	return out, nil
}

func intAns(v int64) ConstAnswer  { return ConstAnswer{Kind: ConstInt, Int: big.NewInt(v)} }
func charAns(v int64) ConstAnswer { return ConstAnswer{Kind: ConstChar, Int: big.NewInt(v)} }
func boolAns(b bool) ConstAnswer {
	return ConstAnswer{Kind: ConstBool, Int: big.NewInt(map[bool]int64{false: 0, true: 1}[b])}
}
func fltAns(f float64) ConstAnswer { return ConstAnswer{Kind: ConstFloat, Float: f} }
func strAns(s string) ConstAnswer  { return ConstAnswer{Kind: ConstString, Str: s} }
func notConst(why string) ConstAnswer {
	return ConstAnswer{NotConst: true, Why: why}
}

// checkConsts checks the file main.gp (a package main with the import of <lib.h> added by the test
// source itself) with the answers and returns the main package, what gocpp check prints and the
// answers' record. phase is the phase of the gates.
func checkConsts(t *testing.T, src string, c *constAnswers, phase int) (*Package, string) {
	t.Helper()
	prog, root := loadTree(t, map[string]string{"main.gp": src})
	cfg := fakeProbeConfig(phase)
	if c != nil {
		cfg.Constants = c.fn
	}
	r := Check(prog, cfg)
	return mainPkg(t, prog, r), errorText(t, prog, r, root, true)
}

func constVal(t *testing.T, p *Package, name string) string {
	t.Helper()
	obj, _ := p.Types.Scope().Lookup(name).(*types.Const)
	if obj == nil {
		t.Fatalf("no constant %s in the package", name)
	}
	return obj.Val().ExactString()
}

func TestConstSubstitutionValues(t *testing.T) {
	t.Parallel()
	const src = `package main

import (
	"<lib.h>"
	"<numbers>"
)

const bufsz = lib.BUFSIZ
const half = lib.BUFSIZ / 2
const eof = lib.EOF
const mask = lib.BIG >> 60
const chr = lib.CH
const nl = lib.NL
const on = lib.ON
const name = lib.NAME
const pi2 = std.numbers.pi * 2
const tenth = lib.TENTH * 3
const conv = uint8(lib.SMALL)
const nlen = len(lib.NAME)
const fn = float64(lib.SMALL) / 4
const mm = max(lib.SMALL, 10)
const neg = -lib.SMALL
const par = (lib.SMALL + 1) * 2
const ng = lib.NEGF

var buf [lib.BUFSIZ]byte
var grid [lib.SMALL + 1][lib.SMALL * 2]int

func main() {
	_ = buf
	_ = grid
}
`
	c := &constAnswers{
		hdr: map[string]ConstAnswer{
			"BUFSIZ": intAns(8192), "EOF": intAns(-1), "BIG": {Kind: ConstInt, Int: new(big.Int).SetUint64(math.MaxUint64)},
			"CH": charAns('a'), "NL": charAns('\n'), "ON": boolAns(true), "NAME": strAns("My\"App\n"),
			"TENTH": fltAns(0.1), "SMALL": intAns(5), "NEGF": fltAns(-2.5),
		},
		ns: map[string]ConstAnswer{"numbers.pi": fltAns(math.Pi)},
	}
	p, errs := checkConsts(t, src, c, LastPhase)
	if errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	for name, want := range map[string]string{
		"bufsz": "8192", "half": "4096", "eof": "-1", "mask": "15", "chr": "97", "nl": "10", "on": "true",
		"name": `"My\"App\n"`, "conv": "5", "nlen": "7", "mm": "10", "neg": "-5", "par": "12", "ng": "-5/2", "fn": "5/4",
	} {
		if got := constVal(t, p, name); got != want {
			t.Errorf("const %s = %s, want %s", name, got, want)
		}
	}
	// A float is the exact double: 0.1 * 3 is not 0.3 for go/types either.
	want := new(big.Rat).SetFloat64(0.1)
	want.Mul(want, big.NewRat(3, 1))
	if got := constant.ToFloat(p.Types.Scope().Lookup("tenth").(*types.Const).Val()); constant.Compare(got, token.EQL, constant.Make(want)) == false {
		t.Errorf("tenth = %s, want exactly three times the double 0.1", got.ExactString())
	}
	if f, _ := constant.Float64Val(p.Types.Scope().Lookup("pi2").(*types.Const).Val()); f != 2*math.Pi {
		t.Errorf("pi2 = %v, want %v", f, 2*math.Pi)
	}
	if got := p.Types.Scope().Lookup("buf").Type().String(); got != "[8192]byte" {
		t.Errorf("type of buf: %s", got)
	}
	if got := p.Types.Scope().Lookup("grid").Type().String(); got != "[6][10]int" {
		t.Errorf("type of grid: %s", got)
	}
	// The list of what was replaced is in source order and says what and where.
	var uses []string
	for _, u := range p.Consts {
		uses = append(uses, u.Written+"="+u.Value+"@"+u.Where)
	}
	for _, w := range []string{"lib.BUFSIZ=8192@constant declaration", "lib.EOF=-1@constant declaration", "lib.CH='a'@constant declaration",
		"lib.NL='\\n'@constant declaration", "lib.ON=true@constant declaration", "std.numbers.pi=0x1.921fb54442d18p+01@constant declaration",
		"lib.BUFSIZ=8192@array length", "lib.SMALL=5@array length"} {
		if !slices.Contains(uses, w) {
			t.Errorf("Package.Consts lacks %s:\n%s", w, strings.Join(uses, "\n"))
		}
	}
	// One query per package, each path once, sorted.
	if len(c.queries) != 1 {
		t.Fatalf("%d queries, want 1", len(c.queries))
	}
	q := c.queries[0]
	if !slices.Equal(q.Includes, []string{"<lib.h>", "<numbers>"}) || q.GOOS != "" || q.GOARCH != "" {
		t.Errorf("query: includes %v target %q/%q", q.Includes, q.GOOS, q.GOARCH)
	}
	var keys []string
	for _, p := range q.Paths {
		keys = append(keys, p.Key())
	}
	if !slices.IsSorted(keys) || len(slices.Compact(slices.Clone(keys))) != len(keys) {
		t.Errorf("paths are not sorted and unique: %q", keys)
	}
}

// TestConstWithoutAnswers: no function, an error, or a name the function does not know leave the
// package as it was: the gate says what it said.
func TestConstWithoutAnswers(t *testing.T) {
	t.Parallel()
	const src = `package main

import "<lib.h>"

var buf [lib.BUFSIZ]byte
const n = lib.BUFSIZ

func main() { _ = buf }
`
	const gated = `# example.com/m
./main.gp:5:10: C/C++ names in constant expressions are not supported yet (Phase 12)
`
	if _, errs := checkConsts(t, src, nil, 5); errs != gated {
		t.Errorf("no function:\n%s", errs)
	}
	if _, errs := checkConsts(t, src, &constAnswers{}, 5); errs != gated {
		t.Errorf("an empty answer:\n%s", errs)
	}
	c := &constAnswers{err: errors.New("clang died")}
	_, errs := checkConsts(t, src, c, 5)
	if want := "./main.gp:5:10: warning: cannot evaluate the C/C++ constants of this package: clang died (they stay opaque)\n"; !strings.Contains(errs, want) {
		t.Errorf("an error of the function: want %q in\n%s", want, errs)
	}
	if !strings.Contains(errs, "not supported yet (Phase 12)") {
		t.Errorf("an error of the function leaves the gate:\n%s", errs)
	}
	// With the answer there is no gate error at any phase: the value is a plain constant.
	c = &constAnswers{hdr: map[string]ConstAnswer{"BUFSIZ": intAns(8192)}}
	if _, errs := checkConsts(t, src, c, 5); errs != "" {
		t.Errorf("with the answer:\n%s", errs)
	}
}

func TestConstNotConstant(t *testing.T) {
	t.Parallel()
	const src = `package main

import "<lib.h>"

var a [lib.ERRNO]int
const k = lib.INF
const m = lib.TYPE + 1

func main() {
	switch 1 {
	case lib.ERRNO:
	}
	_ = a
	x := 1 << lib.ERRNO
	_ = x
}
`
	c := &constAnswers{hdr: map[string]ConstAnswer{
		"ERRNO": notConst("the C++ compiler cannot evaluate it at compile time (not a constant expression)"),
		"INF":   notConst("its value +Inf is not a number that a Go constant can have (Go constants are finite)"),
		"TYPE":  notConst(""),
	}}
	_, errs := checkConsts(t, src, c, LastPhase)
	want := `# example.com/m
./main.gp:5:8: array length lib.ERRNO must be constant: the C++ compiler cannot evaluate it at compile time (not a constant expression)
./main.gp:6:11: lib.INF is not constant: its value +Inf is not a number that a Go constant can have (Go constants are finite)
./main.gp:7:11: lib.TYPE is not constant: it has no compile-time value
`
	if errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	// With the gates on, the error is ours alone: the name is not left for the phase gate to report again.
	_, gated := checkConsts(t, src, c, 5)
	if want := strings.Join(strings.SplitAfter(want, "\n")[:4], ""); !strings.HasPrefix(gated, want) || strings.Contains(gated, "not supported yet") {
		t.Errorf("errors with the gates:\n%s", gated)
	}
}

// TestConstCaseAndShift: a case or a shift count is a constant only when it fits, and Go allows a
// variable there: a value that go/types rejects is taken back and the name is opaque again, as in
// opaque mode, but a duplicate case is the program's own mistake.
func TestConstCaseAndShift(t *testing.T) {
	t.Parallel()
	const src = `package main

import "<lib.h>"

func f(x int, b byte, s string) int {
	switch x {
	case lib.EOF, lib.ZERO:
		return 1
	case lib.BIG:
		return 2
	}
	switch b {
	case lib.EOF: // -1 does not fit a byte: C++ accepts it
		return 3
	}
	switch s {
	case lib.ZERO: // an int against a string: the C++ compiler decides
		return 4
	}
	switch {
	case lib.ON:
		return 5
	}
	n := 1 << lib.BITS
	m := 1 << lib.NEG // a negative count is the C++ compiler's
	n <<= lib.BITS
	return n + m
}

func main() { f(0, 0, "") }
`
	c := &constAnswers{hdr: map[string]ConstAnswer{
		"EOF": intAns(-1), "ZERO": intAns(0), "BIG": intAns(1 << 40), "ON": intAns(1), "BITS": intAns(3), "NEG": intAns(-1),
	}}
	p, errs := checkConsts(t, src, c, LastPhase)
	if errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	var got []string
	for _, u := range p.Consts {
		got = append(got, u.Pos.String()+" "+u.Written+"="+u.Value+" "+u.Where)
	}
	for _, w := range []string{"lib.EOF=-1 case", "lib.ZERO=0 case", "lib.BIG=1099511627776 case", "main.gp:24:12 lib.BITS=3 shift count", "main.gp:26:8 lib.BITS=3 shift count"} {
		if !slices.ContainsFunc(got, func(g string) bool { return strings.HasSuffix(g, w) }) {
			t.Errorf("missing %s in\n%s", w, strings.Join(got, "\n"))
		}
	}
	// Taken back: the byte, the string, the boolean switch and the negative count.
	for _, line := range []string{":13:7", ":17:7", ":21:7", ":25:12"} {
		if slices.ContainsFunc(got, func(g string) bool { return strings.Contains(g, "main.gp"+line) }) {
			t.Errorf("the substitution at %s should have been taken back:\n%s", line, strings.Join(got, "\n"))
		}
	}
	// A duplicate case stays an error.
	const dup = `package main

import "<lib.h>"

func f(x int) {
	switch x {
	case 0:
	case lib.ZERO:
	}
}

func main() { f(0) }
`
	c = &constAnswers{hdr: map[string]ConstAnswer{"ZERO": intAns(0)}}
	_, errs = checkConsts(t, dup, c, LastPhase)
	if !strings.Contains(errs, "duplicate case") {
		t.Errorf("a duplicate case through a macro is not reported:\n%s", errs)
	}
}

// TestConstWhatIsLookedInto: only constant expressions around a name are looked into.
func TestConstWhatIsLookedInto(t *testing.T) {
	t.Parallel()
	const src = `package main

import (
	"<lib.h>"
	"unsafe"
)

func g(n int) int { return n }

var a [lib.f(lib.FN)]int
var b [g(lib.GN)]int
var c [unsafe.Sizeof(lib.US)]int
var d [len(lib.NAME)]int
var e [int(lib.N)]int

func main() {}
`
	c := &constAnswers{hdr: map[string]ConstAnswer{"N": intAns(2), "NAME": strAns("ab"), "f": intAns(9), "FN": intAns(1), "GN": intAns(1), "US": intAns(1)}}
	_, _ = checkConsts(t, src, c, LastPhase)
	if len(c.queries) != 1 {
		t.Fatalf("%d queries", len(c.queries))
	}
	var names []string
	for _, p := range c.queries[0].Paths {
		names = append(names, strings.Join(p.Parts, "."))
	}
	// lib.N of the arguments of lib.f, g and unsafe.Sizeof, and C.int(...) are not constant expressions of
	// Go; lib.f is a call (its callee is not a place); len(lib.NAME) and int(lib.N) are.
	if want := []string{"N", "NAME"}; !slices.Equal(names, want) {
		t.Errorf("asked about %v, want %v", names, want)
	}
}

func TestConstMessagesNameTheMacro(t *testing.T) {
	t.Parallel()
	const src = `package main

import "<lib.h>"

var a [lib.NEG]int
const b uint8 = lib.NEG
const c = lib.NEG << 2

func main() {}
`
	c := &constAnswers{hdr: map[string]ConstAnswer{"NEG": intAns(-7)}}
	_, errs := checkConsts(t, src, c, LastPhase)
	want := `# example.com/m
./main.gp:5:8: invalid array length lib.NEG (untyped int constant -7)
./main.gp:6:17: cannot use lib.NEG (untyped int constant -7) as uint8 value in constant declaration (overflows)
`
	if errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
}

func TestPutName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ msg, lit, name, want string }{
		{"invalid array length -1 (untyped int constant)", "-1", "cstdio.EOF", "invalid array length cstdio.EOF (untyped int constant -1)"},
		{"8192 * 2 (untyped int constant 16384)", "8192", "cstdio.BUFSIZ", "cstdio.BUFSIZ * 2 (untyped int constant 16384)"},
		{"cannot use 18192 here", "8192", "x.A", "cannot use 18192 here"},
		{"value 81920 and 8192", "8192", "x.A", "value 81920 and x.A"},
		{"3.14 is not 3", "3", "x.A", "3.14 is not x.A"},
		{"no literal", "7", "x.A", "no literal"},
	} {
		if got := putName(tc.msg, tc.lit, tc.name); got != tc.want {
			t.Errorf("putName(%q, %q, %q) = %q, want %q", tc.msg, tc.lit, tc.name, got, tc.want)
		}
	}
}

// TestFloatLiteral: the literal is the double exactly, whatever the number.
func TestFloatLiteral(t *testing.T) {
	t.Parallel()
	for f, want := range map[float64]string{0: "0.0", 3: "3.0", 0.5: "0.5", 1000.25: "1000.25", 0.1: "0x1.999999999999ap-04", 1e300: "0x1.7e43c8800759cp+996"} {
		if got := floatLiteral(f); got != want {
			t.Errorf("floatLiteral(%v) = %s, want %s", f, got, want)
		}
	}
	rng := rand.New(rand.NewSource(1))
	check := func(f float64) {
		t.Helper()
		lit := floatLiteral(f)
		v := constant.MakeFromLiteral(lit, token.FLOAT, 0)
		if v.Kind() == constant.Unknown {
			t.Fatalf("floatLiteral(%v) = %s is not a Go literal", f, lit)
		}
		if got, exact := constant.Float64Val(v); got != f {
			t.Errorf("floatLiteral(%v) = %s reads back as %v (exact %v)", f, lit, got, exact)
		}
		// And the constant is exactly the double, not only the nearest one.
		if want := new(big.Rat).SetFloat64(f); constant.Compare(v, token.EQL, constant.Make(want)) == false {
			t.Errorf("floatLiteral(%v) = %s is not exactly the double", f, lit)
		}
	}
	for range 2000 {
		f := math.Float64frombits(rng.Uint64() &^ (1 << 63))
		if !math.IsNaN(f) && !math.IsInf(f, 0) {
			check(f)
		}
	}
	for _, f := range []float64{math.SmallestNonzeroFloat64, math.MaxFloat64, 1, 2.5, 123456789.125, math.Pi, math.E} {
		check(f)
	}
}

func TestConstExprKinds(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		a    ConstAnswer
		want string
	}{
		{intAns(0), "0"}, {intAns(-12), "-12"}, {intAns(1 << 62), "4611686018427387904"},
		{charAns('x'), "'x'"}, {charAns('\''), `'\''`}, {charAns(0), `'\x00'`}, {charAns(-1), "-1"}, {charAns(0xD800), "55296"},
		{boolAns(false), "false"}, {boolAns(true), "true"},
		{fltAns(-1.5), "-1.5"}, {fltAns(math.Copysign(0, -1)), "0.0"},
		{strAns("a\x00\xffb"), `"a\x00\xffb"`},
	} {
		_, lit, ok := constExpr(tc.a, 1)
		if !ok || lit != tc.want {
			t.Errorf("constExpr(%+v) = %q, %v; want %q", tc.a, lit, ok, tc.want)
		}
	}
	for _, a := range []ConstAnswer{fltAns(math.NaN()), fltAns(math.Inf(1)), {Kind: ConstInt}, {}} {
		if _, _, ok := constExpr(a, 1); ok {
			t.Errorf("constExpr(%+v) made an expression", a)
		}
	}
}

// A shift place is the whole shift: an error of the shifted expression (an overflow of the type it goes to)
// is a reason to take the substitution back, not an error that opaque mode did not have; with >> too.
func TestConstShiftRootIsTheWholeShift(t *testing.T) {
	t.Parallel()
	const src = `package main

import "<lib.h>"

var a int32 = 1 << lib.S31
var b = 1 << lib.S70
var c int8 = 1000 >> lib.ONE
var d int8 = 1 << (lib.S31 - 24)
var e int16
var f = 1000 >> lib.ONE

func main() {
	e <<= lib.S31
	_, _, _, _, _ = a, b, c, d, f
}
`
	c := &constAnswers{hdr: map[string]ConstAnswer{"S31": intAns(31), "S70": intAns(70), "ONE": intAns(1)}}
	p, errs := checkConsts(t, src, c, LastPhase)
	if errs != "" {
		t.Errorf("opaque mode accepts these shifts (the C++ compiler decides), typed mode says:\n%s", errs)
	}
	if p.checks < 2 {
		t.Errorf("checks = %d: the package is checked again after a take-back", p.checks)
	}
	// Taken back: no substitution of the shifts of the first, second and third declarations stands; the
	// >> that is fine (line 11) keeps its count.
	var stands []int
	for _, u := range p.Consts {
		stands = append(stands, u.Pos.Line)
	}
	if want := []int{10, 13}; !slices.Equal(stands, want) {
		t.Errorf("substitutions stand at lines %v, want %v (a >> that fits, a <<= count)", stands, want)
	}
}

// A message about a place with several names, or a name and a literal of the user, shows the user's text
// (by the text of the node, not by the first digit that matches).
func TestConstMessageKeepsTheUsersLiteral(t *testing.T) {
	t.Parallel()
	const src = `package main

import "<lib.h>"

const u uint8 = 5 - lib.A - 1
const v uint8 = lib.A - 10 + lib.B

func main() {}
`
	c := &constAnswers{hdr: map[string]ConstAnswer{"A": intAns(5), "B": intAns(3)}}
	_, errs := checkConsts(t, src, c, LastPhase)
	if !strings.Contains(errs, "cannot use lib.A - 10 + lib.B (untyped int constant -2) as uint8 value in constant declaration (overflows)") {
		t.Errorf("several names in one expression:\n%s", errs)
	}
	if !strings.Contains(errs, "5 - lib.A - 1 (untyped int constant -1)") {
		t.Errorf("the user's literal 5 is not where the name is:\n%s", errs)
	}
	if strings.Contains(errs, "lib.A - 5 - 1") {
		t.Errorf("the name replaced the user's literal:\n%s", errs)
	}
}

func TestCppPathKeyHasTheNamespace(t *testing.T) {
	t.Parallel()
	hdr := CppPath{Hdr: "h", Parts: []string{"kDim"}}
	ns := CppPath{Ns: "::geo", Parts: []string{"kDim"}}
	other := CppPath{Hdr: "g", Parts: []string{"kDim"}}
	ns2 := CppPath{Ns: "::other", Parts: []string{"kDim"}}
	if hdr.Key() == ns.Key() || hdr.Key() == other.Key() || ns.Key() == other.Key() || ns.Key() == ns2.Key() {
		t.Errorf("keys collide: %q %q %q %q", hdr.Key(), ns.Key(), other.Key(), ns2.Key())
	}
}

// The substitutions that stand carry the kind, the exact value and the C++ spelling, for the guard.
func TestConstUsesCarryTheGuardData(t *testing.T) {
	t.Parallel()
	const src = `package main

import "<lib.h>"

var a [lib.N]int
const s = lib.NAME

func main() {}
`
	c := &constAnswers{hdr: map[string]ConstAnswer{"N": intAns(3), "NAME": strAns("x")}}
	p, errs := checkConsts(t, src, c, LastPhase)
	if errs != "" {
		t.Fatal(errs)
	}
	if len(p.Consts) != 2 {
		t.Fatalf("%d uses", len(p.Consts))
	}
	for _, u := range p.Consts {
		if u.Spelling == "" || !u.At.IsValid() || u.Answer.Kind == ConstNone {
			t.Errorf("incomplete use %+v", u)
		}
	}
}
