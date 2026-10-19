// phase: 09
// Go 1.25's strconv/ctoa_test.go (FormatComplex, AppendComplex) through a small harness; expected output from real Go.

package main

import (
	"fmt"
	. "strconv"
)

func TestFormatComplex(t *testT) {
	tests := []struct {
		c       complex128
		fmt     byte
		prec    int
		bitSize int
		out     string
	}{
		// a variety of signs
		{1 + 2i, 'g', -1, 128, "(1+2i)"},
		{3 - 4i, 'g', -1, 128, "(3-4i)"},
		{-5 + 6i, 'g', -1, 128, "(-5+6i)"},
		{-7 - 8i, 'g', -1, 128, "(-7-8i)"},

		// test that fmt and prec are working
		{3.14159 + 0.00123i, 'e', 3, 128, "(3.142e+00+1.230e-03i)"},
		{3.14159 + 0.00123i, 'f', 3, 128, "(3.142+0.001i)"},
		{3.14159 + 0.00123i, 'g', 3, 128, "(3.14+0.00123i)"},

		// ensure bitSize rounding is working
		{1.2345678901234567 + 9.876543210987654i, 'f', -1, 128, "(1.2345678901234567+9.876543210987654i)"},
		{1.2345678901234567 + 9.876543210987654i, 'f', -1, 64, "(1.2345679+9.876543i)"},

		// other cases are handled by FormatFloat tests
	}
	for _, test := range tests {
		out := FormatComplex(test.c, test.fmt, test.prec, test.bitSize)
		checks++
		if out != test.out {
			t.Fatalf("FormatComplex(%v, %q, %d, %d) = %q; want %q",
				test.c, test.fmt, test.prec, test.bitSize, out, test.out)
		}
	}
}

func TestFormatComplexInvalidBitSize(t *testT) {
	defer func() {
		checks++
		if r := recover(); r == nil {
			t.Fatalf("expected panic due to invalid bitSize")
		}
	}()
	_ = FormatComplex(1+2i, 'g', -1, 100)
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
var checks int

func (t *testT) Helper()                           {}
func (t *testT) Parallel()                         {}
func (t *testT) Name() string                      { return t.name }
func (t *testT) Log(args ...any)                   {}
func (t *testT) Logf(format string, args ...any)   {}
func (t *testT) Skip(args ...any)                  { panic(skipExit{}) }
func (t *testT) Skipf(f string, args ...any)       { panic(skipExit{}) }
func (t *testT) report(msg string)                 { t.failed = true; fmt.Println("FAIL", t.name+":", msg) }
func (t *testT) Errorf(format string, args ...any) { t.report(fmt.Sprintf(format, args...)) }
func (t *testT) Error(args ...any)                 { t.report(fmt.Sprint(args...)) }
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
		checks = 0
		runTest(t, tc.f)
		if t.failed {
			failed++
			fmt.Println("--- FAIL:", tc.name)
		} else {
			fmt.Println("ok", tc.name, "subtests:", subtests, "checks:", checks)
		}
	}
	fmt.Println("tests:", len(tests), "failed:", failed)
	if failed > 0 {
		panic("tests failed")
	}
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
func (r *rngT) Uint32() uint32   { return uint32(r.Uint64() >> 32) }
func (r *rngT) Intn(n int) int   { return int(r.Uint64()>>1) % n }
func (r *rngT) Float64() float64 { return float64(r.Uint64()>>11) / (1 << 53) }

func main() {
	tests_ := []testCase{
		{"TestFormatComplex", TestFormatComplex},
		{"TestFormatComplexInvalidBitSize", TestFormatComplexInvalidBitSize},
	}
	runTests(tests_)
}

// Output:
// ok TestFormatComplex subtests: 0 checks: 9
// ok TestFormatComplexInvalidBitSize subtests: 0 checks: 1
// tests: 2 failed: 0
