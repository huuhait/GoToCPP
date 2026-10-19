// phase: 09
// Go 1.25's unicode/graphic_test.go (the Latin-1 predicates: IsControl, IsLetter, IsUpper, IsLower, IsNumber, IsPrint, IsGraphic, IsPunct, IsSpace, IsSymbol) through a small harness; expected output from real Go.

package main

import (
	"fmt"
	"unicode"
)

// Independently check that the special "Is" functions work
// in the Latin-1 range through the property table.

func TestIsControlLatin1(t *testT) {
	for i := rune(0); i <= unicode.MaxLatin1; i++ {
		got := unicode.IsControl(i)
		want := false
		switch {
		case 0x00 <= i && i <= 0x1F:
			want = true
		case 0x7F <= i && i <= 0x9F:
			want = true
		}
		checks++
		if got != want {
			t.Errorf("%U incorrect: got %t; want %t", i, got, want)
		}
	}
}

func TestIsLetterLatin1(t *testT) {
	for i := rune(0); i <= unicode.MaxLatin1; i++ {
		got := unicode.IsLetter(i)
		want := unicode.Is(unicode.Letter, i)
		checks++
		if got != want {
			t.Errorf("%U incorrect: got %t; want %t", i, got, want)
		}
	}
}

func TestIsUpperLatin1(t *testT) {
	for i := rune(0); i <= unicode.MaxLatin1; i++ {
		got := unicode.IsUpper(i)
		want := unicode.Is(unicode.Upper, i)
		checks++
		if got != want {
			t.Errorf("%U incorrect: got %t; want %t", i, got, want)
		}
	}
}

func TestIsLowerLatin1(t *testT) {
	for i := rune(0); i <= unicode.MaxLatin1; i++ {
		got := unicode.IsLower(i)
		want := unicode.Is(unicode.Lower, i)
		checks++
		if got != want {
			t.Errorf("%U incorrect: got %t; want %t", i, got, want)
		}
	}
}

func TestNumberLatin1(t *testT) {
	for i := rune(0); i <= unicode.MaxLatin1; i++ {
		got := unicode.IsNumber(i)
		want := unicode.Is(unicode.Number, i)
		checks++
		if got != want {
			t.Errorf("%U incorrect: got %t; want %t", i, got, want)
		}
	}
}

func TestIsPrintLatin1(t *testT) {
	for i := rune(0); i <= unicode.MaxLatin1; i++ {
		got := unicode.IsPrint(i)
		want := unicode.In(i, unicode.PrintRanges...)
		checks++
		if i == ' ' {
			want = true
		}
		checks++
		if got != want {
			t.Errorf("%U incorrect: got %t; want %t", i, got, want)
		}
	}
}

func TestIsGraphicLatin1(t *testT) {
	for i := rune(0); i <= unicode.MaxLatin1; i++ {
		got := unicode.IsGraphic(i)
		want := unicode.In(i, unicode.GraphicRanges...)
		checks++
		if got != want {
			t.Errorf("%U incorrect: got %t; want %t", i, got, want)
		}
	}
}

func TestIsPunctLatin1(t *testT) {
	for i := rune(0); i <= unicode.MaxLatin1; i++ {
		got := unicode.IsPunct(i)
		want := unicode.Is(unicode.Punct, i)
		checks++
		if got != want {
			t.Errorf("%U incorrect: got %t; want %t", i, got, want)
		}
	}
}

func TestIsSpaceLatin1(t *testT) {
	for i := rune(0); i <= unicode.MaxLatin1; i++ {
		got := unicode.IsSpace(i)
		want := unicode.Is(unicode.White_Space, i)
		checks++
		if got != want {
			t.Errorf("%U incorrect: got %t; want %t", i, got, want)
		}
	}
}

func TestIsSymbolLatin1(t *testT) {
	for i := rune(0); i <= unicode.MaxLatin1; i++ {
		got := unicode.IsSymbol(i)
		want := unicode.Is(unicode.Symbol, i)
		checks++
		if got != want {
			t.Errorf("%U incorrect: got %t; want %t", i, got, want)
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
		{"TestIsControlLatin1", TestIsControlLatin1},
		{"TestIsLetterLatin1", TestIsLetterLatin1},
		{"TestIsUpperLatin1", TestIsUpperLatin1},
		{"TestIsLowerLatin1", TestIsLowerLatin1},
		{"TestNumberLatin1", TestNumberLatin1},
		{"TestIsPrintLatin1", TestIsPrintLatin1},
		{"TestIsGraphicLatin1", TestIsGraphicLatin1},
		{"TestIsPunctLatin1", TestIsPunctLatin1},
		{"TestIsSpaceLatin1", TestIsSpaceLatin1},
		{"TestIsSymbolLatin1", TestIsSymbolLatin1},
	}
	runTests(tests_)
}

// Output:
// ok TestIsControlLatin1 subtests: 0 checks: 256
// ok TestIsLetterLatin1 subtests: 0 checks: 256
// ok TestIsUpperLatin1 subtests: 0 checks: 256
// ok TestIsLowerLatin1 subtests: 0 checks: 256
// ok TestNumberLatin1 subtests: 0 checks: 256
// ok TestIsPrintLatin1 subtests: 0 checks: 512
// ok TestIsGraphicLatin1 subtests: 0 checks: 256
// ok TestIsPunctLatin1 subtests: 0 checks: 256
// ok TestIsSpaceLatin1 subtests: 0 checks: 256
// ok TestIsSymbolLatin1 subtests: 0 checks: 256
// tests: 10 failed: 0
