// phase: 09
// Go 1.25's regexp/syntax/prog_test.go (Compile: the programs of 40 patterns as text, the instructions of the program, StartCond, Prefix) through a small harness; expected output from real Go.

package main

import (
	"fmt"
	"regexp/syntax"
	"sync/atomic"
)


var compileTests = []struct {
	Regexp string
	Prog   string
}{
	{"a", `  0	fail
  1*	rune1 "a" -> 2
  2	match
`},
	{"[A-M][n-z]", `  0	fail
  1*	rune "AM" -> 2
  2	rune "nz" -> 3
  3	match
`},
	{"", `  0	fail
  1*	nop -> 2
  2	match
`},
	{"a?", `  0	fail
  1	rune1 "a" -> 3
  2*	alt -> 1, 3
  3	match
`},
	{"a??", `  0	fail
  1	rune1 "a" -> 3
  2*	alt -> 3, 1
  3	match
`},
	{"a+", `  0	fail
  1*	rune1 "a" -> 2
  2	alt -> 1, 3
  3	match
`},
	{"a+?", `  0	fail
  1*	rune1 "a" -> 2
  2	alt -> 3, 1
  3	match
`},
	{"a*", `  0	fail
  1	rune1 "a" -> 2
  2*	alt -> 1, 3
  3	match
`},
	{"a*?", `  0	fail
  1	rune1 "a" -> 2
  2*	alt -> 3, 1
  3	match
`},
	{"a+b+", `  0	fail
  1*	rune1 "a" -> 2
  2	alt -> 1, 3
  3	rune1 "b" -> 4
  4	alt -> 3, 5
  5	match
`},
	{"(a+)(b+)", `  0	fail
  1*	cap 2 -> 2
  2	rune1 "a" -> 3
  3	alt -> 2, 4
  4	cap 3 -> 5
  5	cap 4 -> 6
  6	rune1 "b" -> 7
  7	alt -> 6, 8
  8	cap 5 -> 9
  9	match
`},
	{"a+|b+", `  0	fail
  1	rune1 "a" -> 2
  2	alt -> 1, 6
  3	rune1 "b" -> 4
  4	alt -> 3, 6
  5*	alt -> 1, 3
  6	match
`},
	{"A[Aa]", `  0	fail
  1*	rune1 "A" -> 2
  2	rune "A"/i -> 3
  3	match
`},
	{"(?:(?:^).)", `  0	fail
  1*	empty 4 -> 2
  2	anynotnl -> 3
  3	match
`},
	{"(?:|a)+", `  0	fail
  1	nop -> 4
  2	rune1 "a" -> 4
  3*	alt -> 1, 2
  4	alt -> 3, 5
  5	match
`},
	{"(?:|a)*", `  0	fail
  1	nop -> 4
  2	rune1 "a" -> 4
  3	alt -> 1, 2
  4	alt -> 3, 6
  5*	alt -> 3, 6
  6	match
`},
}

func TestCompile(t *testT) {
	for _, tt := range compileTests {
		re, _ := syntax.Parse(tt.Regexp, syntax.Perl)
		p, _ := syntax.Compile(re)
		s := p.String()
		atomic.AddInt64(&checks, 1)
		if s != tt.Prog {
			t.Errorf("compiled %#q:\n--- have\n%s---\n--- want\n%s---", tt.Regexp, s, tt.Prog)
		}
	}
}


var sink any

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
		{"TestCompile", TestCompile},
	}
	runTests(tests_)
}

// Output:
// ok TestCompile subtests: 0 checks: 16
// tests: 1 failed: 0
