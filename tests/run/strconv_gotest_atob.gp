// phase: 09
// Go 1.25's strconv/atob_test.go (ParseBool, FormatBool, AppendBool) through a small harness; expected output from real Go.

package main

import (
	"fmt"
	. "strconv"
)

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type atobTest struct {
	in  string
	out bool
	err error
}

var atobtests = []atobTest{
	{"", false, ErrSyntax},
	{"asdf", false, ErrSyntax},
	{"0", false, nil},
	{"f", false, nil},
	{"F", false, nil},
	{"FALSE", false, nil},
	{"false", false, nil},
	{"False", false, nil},
	{"1", true, nil},
	{"t", true, nil},
	{"T", true, nil},
	{"TRUE", true, nil},
	{"true", true, nil},
	{"True", true, nil},
}

func TestParseBool(t *testT) {
	for _, test := range atobtests {
		b, e := ParseBool(test.in)
		checks++
		if test.err != nil {
			// expect an error
			checks++
			if e == nil {
				t.Errorf("ParseBool(%s) = nil; want %s", test.in, test.err)
			} else {
				// NumError assertion must succeed; it's the only thing we return.
				checks++
				if e.(*NumError).Err != test.err {
					t.Errorf("ParseBool(%s) = %s; want %s", test.in, e, test.err)
				}
			}
		} else {
			checks++
			if e != nil {
				t.Errorf("ParseBool(%s) = %s; want nil", test.in, e)
			}
			checks++
			if b != test.out {
				t.Errorf("ParseBool(%s) = %t; want %t", test.in, b, test.out)
			}
		}
	}
}

var boolString = map[bool]string{
	true:  "true",
	false: "false",
}

func TestFormatBool(t *testT) {
	for b, s := range boolString {
		checks++
		if f := FormatBool(b); f != s {
			t.Errorf("FormatBool(%v) = %q; want %q", b, f, s)
		}
	}
}

type appendBoolTest struct {
	b   bool
	in  []byte
	out []byte
}

var appendBoolTests = []appendBoolTest{
	{true, []byte("foo "), []byte("foo true")},
	{false, []byte("foo "), []byte("foo false")},
}

func TestAppendBool(t *testT) {
	for _, test := range appendBoolTests {
		b := AppendBool(test.in, test.b)
		checks++
		if !bytesEqual(b, test.out) {
			t.Errorf("AppendBool(%q, %v) = %q; want %q", test.in, test.b, b, test.out)
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
		{"TestParseBool", TestParseBool},
		{"TestFormatBool", TestFormatBool},
		{"TestAppendBool", TestAppendBool},
	}
	runTests(tests_)
}

// Output:
// ok TestParseBool subtests: 0 checks: 42
// ok TestFormatBool subtests: 0 checks: 2
// ok TestAppendBool subtests: 0 checks: 2
// tests: 3 failed: 0
