// phase: 09
// Go 1.25's regexp/syntax/simplify_test.go (Simplify: repeats, empty-width operators, character classes) through a small harness; expected output from real Go.

package main

import (
	"fmt"
	"regexp/syntax"
	"sync/atomic"
)


var simplifyTests = []struct {
	Regexp string
	Simple string
}{
	// Already-simple constructs
	{`a`, `a`},
	{`ab`, `ab`},
	{`a|b`, `[ab]`},
	{`ab|cd`, `ab|cd`},
	{`(ab)*`, `(ab)*`},
	{`(ab)+`, `(ab)+`},
	{`(ab)?`, `(ab)?`},
	{`.`, `(?s:.)`},
	{`^`, `(?m:^)`},
	{`$`, `(?m:$)`},
	{`[ac]`, `[ac]`},
	{`[^ac]`, `[^ac]`},

	// Posix character classes
	{`[[:alnum:]]`, `[0-9A-Za-z]`},
	{`[[:alpha:]]`, `[A-Za-z]`},
	{`[[:blank:]]`, `[\t ]`},
	{`[[:cntrl:]]`, `[\x00-\x1f\x7f]`},
	{`[[:digit:]]`, `[0-9]`},
	{`[[:graph:]]`, `[!-~]`},
	{`[[:lower:]]`, `[a-z]`},
	{`[[:print:]]`, `[ -~]`},
	{`[[:punct:]]`, "[!-/:-@\\[-`\\{-~]"},
	{`[[:space:]]`, `[\t-\r ]`},
	{`[[:upper:]]`, `[A-Z]`},
	{`[[:xdigit:]]`, `[0-9A-Fa-f]`},

	// Perl character classes
	{`\d`, `[0-9]`},
	{`\s`, `[\t\n\f\r ]`},
	{`\w`, `[0-9A-Z_a-z]`},
	{`\D`, `[^0-9]`},
	{`\S`, `[^\t\n\f\r ]`},
	{`\W`, `[^0-9A-Z_a-z]`},
	{`[\d]`, `[0-9]`},
	{`[\s]`, `[\t\n\f\r ]`},
	{`[\w]`, `[0-9A-Z_a-z]`},
	{`[\D]`, `[^0-9]`},
	{`[\S]`, `[^\t\n\f\r ]`},
	{`[\W]`, `[^0-9A-Z_a-z]`},

	// Posix repetitions
	{`a{1}`, `a`},
	{`a{2}`, `aa`},
	{`a{5}`, `aaaaa`},
	{`a{0,1}`, `a?`},
	// The next three are illegible because Simplify inserts (?:)
	// parens instead of () parens to avoid creating extra
	// captured subexpressions. The comments show a version with fewer parens.
	{`(a){0,2}`, `(?:(a)(a)?)?`},                       //       (aa?)?
	{`(a){0,4}`, `(?:(a)(?:(a)(?:(a)(a)?)?)?)?`},       //   (a(a(aa?)?)?)?
	{`(a){2,6}`, `(a)(a)(?:(a)(?:(a)(?:(a)(a)?)?)?)?`}, // aa(a(a(aa?)?)?)?
	{`a{0,2}`, `(?:aa?)?`},                             //       (aa?)?
	{`a{0,4}`, `(?:a(?:a(?:aa?)?)?)?`},                 //   (a(a(aa?)?)?)?
	{`a{2,6}`, `aa(?:a(?:a(?:aa?)?)?)?`},               // aa(a(a(aa?)?)?)?
	{`a{0,}`, `a*`},
	{`a{1,}`, `a+`},
	{`a{2,}`, `aa+`},
	{`a{5,}`, `aaaaa+`},

	// Test that operators simplify their arguments.
	{`(?:a{1,}){1,}`, `a+`},
	{`(a{1,}b{1,})`, `(a+b+)`},
	{`a{1,}|b{1,}`, `a+|b+`},
	{`(?:a{1,})*`, `(?:a+)*`},
	{`(?:a{1,})+`, `a+`},
	{`(?:a{1,})?`, `(?:a+)?`},
	{``, `(?:)`},
	{`a{0}`, `(?:)`},

	// Character class simplification
	{`[ab]`, `[ab]`},
	{`[abc]`, `[a-c]`},
	{`[a-za-za-z]`, `[a-z]`},
	{`[A-Za-zA-Za-z]`, `[A-Za-z]`},
	{`[ABCDEFGH]`, `[A-H]`},
	{`[AB-CD-EF-GH]`, `[A-H]`},
	{`[W-ZP-XE-R]`, `[E-Z]`},
	{`[a-ee-gg-m]`, `[a-m]`},
	{`[a-ea-ha-m]`, `[a-m]`},
	{`[a-ma-ha-e]`, `[a-m]`},
	{`[a-zA-Z0-9 -~]`, `[ -~]`},

	// Empty character classes
	{`[^[:cntrl:][:^cntrl:]]`, `[^\x00-\x{10FFFF}]`},

	// Full character classes
	{`[[:cntrl:][:^cntrl:]]`, `(?s:.)`},

	// Unicode case folding.
	{`(?i)A`, `(?i:A)`},
	{`(?i)a`, `(?i:A)`},
	{`(?i)[A]`, `(?i:A)`},
	{`(?i)[a]`, `(?i:A)`},
	{`(?i)K`, `(?i:K)`},
	{`(?i)k`, `(?i:K)`},
	{`(?i)\x{212a}`, "(?i:K)"},
	{`(?i)[K]`, "[Kk\u212A]"},
	{`(?i)[k]`, "[Kk\u212A]"},
	{`(?i)[\x{212a}]`, "[Kk\u212A]"},
	{`(?i)[a-z]`, "[A-Za-z\u017F\u212A]"},
	{`(?i)[\x00-\x{FFFD}]`, "[\\x00-\uFFFD]"},
	{`(?i)[\x00-\x{10FFFF}]`, `(?s:.)`},

	// Empty string as a regular expression.
	// The empty string must be preserved inside parens in order
	// to make submatches work right, so these tests are less
	// interesting than they might otherwise be. String inserts
	// explicit (?:) in place of non-parenthesized empty strings,
	// to make them easier to spot for other parsers.
	{`(a|b|c|)`, `([a-c]|(?:))`},
	{`(a|b|)`, `([ab]|(?:))`},
	{`(|)`, `()`},
	{`a()`, `a()`},
	{`(()|())`, `(()|())`},
	{`(a|)`, `(a|(?:))`},
	{`ab()cd()`, `ab()cd()`},
	{`()`, `()`},
	{`()*`, `()*`},
	{`()+`, `()+`},
	{`()?`, `()?`},
	{`(){0}`, `(?:)`},
	{`(){1}`, `()`},
	{`(){1,}`, `()+`},
	{`(){0,2}`, `(?:()()?)?`},
}

func TestSimplify(t *testT) {
	for _, tt := range simplifyTests {
		re, err := syntax.Parse(tt.Regexp, syntax.MatchNL|syntax.Perl&^syntax.OneLine)
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf("Parse(%#q) = error %v", tt.Regexp, err)
			continue
		}
		s := re.Simplify().String()
		atomic.AddInt64(&checks, 1)
		if s != tt.Simple {
			t.Errorf("Simplify(%#q) = %#q, want %#q", tt.Regexp, s, tt.Simple)
		}
	}
}

// ---- a small harness instead of package testing (step 9.9); Go's tests are below, unchanged except as listed above ----

type testT struct {
	name   string
	failed bool
}

type fatalExit struct{}
type skipExit struct{}

var subtests int

// checks counts the if statements that the tests of the file run: a Test that returns early shows in the expected output.
// It is atomic: some tests run their checks in several goroutines.
var checks int64

func (t *testT) Helper()                          {}
func (t *testT) Parallel()                        {}
func (t *testT) Name() string                     { return t.name }
func (t *testT) Log(args ...any)                  {}
func (t *testT) Logf(format string, args ...any)  {}
func (t *testT) Skip(args ...any)                 { panic(skipExit{}) }
func (t *testT) Skipf(f string, args ...any)      { panic(skipExit{}) }
func (t *testT) report(msg string)                { t.failed = true; fmt.Println("FAIL", t.name+":", msg) }
func (t *testT) Errorf(format string, args ...any) { t.report(fmt.Sprintf(format, args...)) }
func (t *testT) Error(args ...any)                { t.report(fmt.Sprint(args...)) }
func (t *testT) Fatalf(format string, args ...any) {
	t.Errorf(format, args...)
	panic(fatalExit{})
}
func (t *testT) Fatal(args ...any) {
	t.Error(args...)
	panic(fatalExit{})
}

func (t *testT) Run(name string, f func(t *testT)) bool {
	sub := &testT{name: t.name + "/" + name}
	subtests++
	runTest(sub, f)
	if sub.failed {
		t.failed = true
	}
	return !sub.failed
}

func runTest(t *testT, f func(t *testT)) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case fatalExit, skipExit:
			default:
				t.failed = true
				fmt.Println("PANIC", t.name+":", r)
			}
		}
	}()
	f(t)
}

type testCase struct {
	name string
	f    func(t *testT)
}

func runTests(tests []testCase) {
	failed := 0
	for _, tc := range tests {
		t := &testT{name: tc.name}
		subtests = 0
		atomic.StoreInt64(&checks, 0)
		runTest(t, tc.f)
		if t.failed {
			failed++
			fmt.Println("--- FAIL:", tc.name)
		} else {
			fmt.Println("ok", tc.name, "subtests:", subtests, "checks:", atomic.LoadInt64(&checks))
		}
	}
	fmt.Println("tests:", len(tests), "failed:", failed)
	if failed > 0 {
		panic("tests failed")
	}
}

// allocsPerRun runs f runs times and returns 0: gocpp does not count heap allocations (the tests of allocation counts are
// dropped or their comparison is).
func allocsPerRun(runs int, f func()) float64 {
	for i := 0; i < runs; i++ {
		f()
	}
	return 0
}

// A deterministic generator (splitmix64) in place of math/rand, so that Go and gocpp run the same inputs.
type rngT struct{ s uint64 }

var rng = &rngT{0x9E3779B97F4A7C15}

func (r *rngT) Uint64() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
func (r *rngT) Uint32() uint32 { return uint32(r.Uint64() >> 32) }
func (r *rngT) Intn(n int) int { return int(r.Uint64()>>1) % n }
func (r *rngT) Float64() float64 { return float64(r.Uint64()>>11) / (1 << 53) }

func main() {
	tests_ := []testCase{
		{"TestSimplify", TestSimplify},
	}
	runTests(tests_)
}

// Output:
// ok TestSimplify subtests: 0 checks: 198
// tests: 1 failed: 0
