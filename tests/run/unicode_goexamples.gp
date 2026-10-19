// phase: 09
// Go 1.25's unicode/example_test.go (the documented examples: Is, In, IsOneOf, SimpleFold, SpecialCase, To*) run in order; expected output from real Go.

package main

import (
	"fmt"
	"unicode"
)

// Functions starting with "Is" can be used to inspect which table of range a
// rune belongs to. Note that runes may fit into more than one range.
func Example_is() {

	// constant with mixed type runes
	const mixed = "\b5Ὂg̀9! ℃ᾭG"
	for _, c := range mixed {
		fmt.Printf("For %q:\n", c)
		if unicode.IsControl(c) {
			fmt.Println("\tis control rune")
		}
		if unicode.IsDigit(c) {
			fmt.Println("\tis digit rune")
		}
		if unicode.IsGraphic(c) {
			fmt.Println("\tis graphic rune")
		}
		if unicode.IsLetter(c) {
			fmt.Println("\tis letter rune")
		}
		if unicode.IsLower(c) {
			fmt.Println("\tis lower case rune")
		}
		if unicode.IsMark(c) {
			fmt.Println("\tis mark rune")
		}
		if unicode.IsNumber(c) {
			fmt.Println("\tis number rune")
		}
		if unicode.IsPrint(c) {
			fmt.Println("\tis printable rune")
		}
		if !unicode.IsPrint(c) {
			fmt.Println("\tis not printable rune")
		}
		if unicode.IsPunct(c) {
			fmt.Println("\tis punct rune")
		}
		if unicode.IsSpace(c) {
			fmt.Println("\tis space rune")
		}
		if unicode.IsSymbol(c) {
			fmt.Println("\tis symbol rune")
		}
		if unicode.IsTitle(c) {
			fmt.Println("\tis title case rune")
		}
		if unicode.IsUpper(c) {
			fmt.Println("\tis upper case rune")
		}
	}

}

func ExampleSimpleFold() {
	fmt.Printf("%#U\n", unicode.SimpleFold('A'))      // 'a'
	fmt.Printf("%#U\n", unicode.SimpleFold('a'))      // 'A'
	fmt.Printf("%#U\n", unicode.SimpleFold('K'))      // 'k'
	fmt.Printf("%#U\n", unicode.SimpleFold('k'))      // '\u212A' (Kelvin symbol, K)
	fmt.Printf("%#U\n", unicode.SimpleFold('\u212A')) // 'K'
	fmt.Printf("%#U\n", unicode.SimpleFold('1'))      // '1'

}

func ExampleTo() {
	const lcG = 'g'
	fmt.Printf("%#U\n", unicode.To(unicode.UpperCase, lcG))
	fmt.Printf("%#U\n", unicode.To(unicode.LowerCase, lcG))
	fmt.Printf("%#U\n", unicode.To(unicode.TitleCase, lcG))

	const ucG = 'G'
	fmt.Printf("%#U\n", unicode.To(unicode.UpperCase, ucG))
	fmt.Printf("%#U\n", unicode.To(unicode.LowerCase, ucG))
	fmt.Printf("%#U\n", unicode.To(unicode.TitleCase, ucG))

}

func ExampleToLower() {
	const ucG = 'G'
	fmt.Printf("%#U\n", unicode.ToLower(ucG))

}
func ExampleToTitle() {
	const ucG = 'g'
	fmt.Printf("%#U\n", unicode.ToTitle(ucG))

}

func ExampleToUpper() {
	const ucG = 'g'
	fmt.Printf("%#U\n", unicode.ToUpper(ucG))

}

func ExampleSpecialCase() {
	t := unicode.TurkishCase

	const lci = 'i'
	fmt.Printf("%#U\n", t.ToLower(lci))
	fmt.Printf("%#U\n", t.ToTitle(lci))
	fmt.Printf("%#U\n", t.ToUpper(lci))

	const uci = 'İ'
	fmt.Printf("%#U\n", t.ToLower(uci))
	fmt.Printf("%#U\n", t.ToTitle(uci))
	fmt.Printf("%#U\n", t.ToUpper(uci))

}

func ExampleIsDigit() {
	fmt.Printf("%t\n", unicode.IsDigit('৩'))
	fmt.Printf("%t\n", unicode.IsDigit('A'))

}

func ExampleIsNumber() {
	fmt.Printf("%t\n", unicode.IsNumber('Ⅷ'))
	fmt.Printf("%t\n", unicode.IsNumber('A'))

}

func ExampleIsLetter() {
	fmt.Printf("%t\n", unicode.IsLetter('A'))
	fmt.Printf("%t\n", unicode.IsLetter('7'))

}

func ExampleIsLower() {
	fmt.Printf("%t\n", unicode.IsLower('a'))
	fmt.Printf("%t\n", unicode.IsLower('A'))

}

func ExampleIsUpper() {
	fmt.Printf("%t\n", unicode.IsUpper('A'))
	fmt.Printf("%t\n", unicode.IsUpper('a'))

}

func ExampleIsTitle() {
	fmt.Printf("%t\n", unicode.IsTitle('ǅ'))
	fmt.Printf("%t\n", unicode.IsTitle('a'))

}

func ExampleIsSpace() {
	fmt.Printf("%t\n", unicode.IsSpace(' '))
	fmt.Printf("%t\n", unicode.IsSpace('\n'))
	fmt.Printf("%t\n", unicode.IsSpace('\t'))
	fmt.Printf("%t\n", unicode.IsSpace('a'))

}

// ---- a small harness instead of package testing (step 9.9); Go's tests are below, unchanged except as listed above ----

type testT struct {
	name   string
	failed bool
}

type fatalExit struct{}
type skipExit struct{}

var subtests int

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
		runTest(t, tc.f)
		if t.failed {
			failed++
			fmt.Println("--- FAIL:", tc.name)
		} else if subtests > 0 {
			fmt.Println("ok", tc.name, subtests)
		} else {
			fmt.Println("ok", tc.name)
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
	fmt.Println("== Example_is")
	Example_is()
	fmt.Println("== ExampleSimpleFold")
	ExampleSimpleFold()
	fmt.Println("== ExampleTo")
	ExampleTo()
	fmt.Println("== ExampleToLower")
	ExampleToLower()
	fmt.Println("== ExampleToTitle")
	ExampleToTitle()
	fmt.Println("== ExampleToUpper")
	ExampleToUpper()
	fmt.Println("== ExampleSpecialCase")
	ExampleSpecialCase()
	fmt.Println("== ExampleIsDigit")
	ExampleIsDigit()
	fmt.Println("== ExampleIsNumber")
	ExampleIsNumber()
	fmt.Println("== ExampleIsLetter")
	ExampleIsLetter()
	fmt.Println("== ExampleIsLower")
	ExampleIsLower()
	fmt.Println("== ExampleIsUpper")
	ExampleIsUpper()
	fmt.Println("== ExampleIsTitle")
	ExampleIsTitle()
	fmt.Println("== ExampleIsSpace")
	ExampleIsSpace()
}

// Output:
// == Example_is
// For '\b':
// 	is control rune
// 	is not printable rune
// For '5':
// 	is digit rune
// 	is graphic rune
// 	is number rune
// 	is printable rune
// For 'Ὂ':
// 	is graphic rune
// 	is letter rune
// 	is printable rune
// 	is upper case rune
// For 'g':
// 	is graphic rune
// 	is letter rune
// 	is lower case rune
// 	is printable rune
// For '̀':
// 	is graphic rune
// 	is mark rune
// 	is printable rune
// For '9':
// 	is digit rune
// 	is graphic rune
// 	is number rune
// 	is printable rune
// For '!':
// 	is graphic rune
// 	is printable rune
// 	is punct rune
// For ' ':
// 	is graphic rune
// 	is printable rune
// 	is space rune
// For '℃':
// 	is graphic rune
// 	is printable rune
// 	is symbol rune
// For 'ᾭ':
// 	is graphic rune
// 	is letter rune
// 	is printable rune
// 	is title case rune
// For 'G':
// 	is graphic rune
// 	is letter rune
// 	is printable rune
// 	is upper case rune
// == ExampleSimpleFold
// U+0061 'a'
// U+0041 'A'
// U+006B 'k'
// U+212A 'K'
// U+004B 'K'
// U+0031 '1'
// == ExampleTo
// U+0047 'G'
// U+0067 'g'
// U+0047 'G'
// U+0047 'G'
// U+0067 'g'
// U+0047 'G'
// == ExampleToLower
// U+0067 'g'
// == ExampleToTitle
// U+0047 'G'
// == ExampleToUpper
// U+0047 'G'
// == ExampleSpecialCase
// U+0069 'i'
// U+0130 'İ'
// U+0130 'İ'
// U+0069 'i'
// U+0130 'İ'
// U+0130 'İ'
// == ExampleIsDigit
// true
// false
// == ExampleIsNumber
// true
// false
// == ExampleIsLetter
// true
// false
// == ExampleIsLower
// true
// false
// == ExampleIsUpper
// true
// false
// == ExampleIsTitle
// true
// false
// == ExampleIsSpace
// true
// true
// true
// false
