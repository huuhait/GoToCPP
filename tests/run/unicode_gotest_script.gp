// phase: 09
// Go 1.25's unicode/script_test.go (every script, category and property table: Is, In, the map of names, Categories and Properties) through a small harness; expected output from real Go.

package main

import (
	"fmt"
	"unicode"
)

type T struct {
	rune   rune
	script string
}

var inCategoryTest = []T{
	{0x0081, "Cc"},
	{0x200B, "Cf"},
	{0xf0000, "Co"},
	{0xdb80, "Cs"},
	{0x0236, "Ll"},
	{0x1d9d, "Lm"},
	{0x07cf, "Lo"},
	{0x1f8a, "Lt"},
	{0x03ff, "Lu"},
	{0x0bc1, "Mc"},
	{0x20df, "Me"},
	{0x07f0, "Mn"},
	{0x1bb2, "Nd"},
	{0x10147, "Nl"},
	{0x2478, "No"},
	{0xfe33, "Pc"},
	{0x2011, "Pd"},
	{0x301e, "Pe"},
	{0x2e03, "Pf"},
	{0x2e02, "Pi"},
	{0x0022, "Po"},
	{0x2770, "Ps"},
	{0x00a4, "Sc"},
	{0xa711, "Sk"},
	{0x25f9, "Sm"},
	{0x2108, "So"},
	{0x2028, "Zl"},
	{0x2029, "Zp"},
	{0x202f, "Zs"},
	// Unifieds.
	{0x04aa, "L"},
	{0x0009, "C"},
	{0x1712, "M"},
	{0x0031, "N"},
	{0x00bb, "P"},
	{0x00a2, "S"},
	{0x00a0, "Z"},
	{0x0065, "LC"},
	// Unassigned
	{0x0378, "Cn"},
	{0x0378, "C"},
}

var inPropTest = []T{
	{0x0046, "ASCII_Hex_Digit"},
	{0x200F, "Bidi_Control"},
	{0x2212, "Dash"},
	{0xE0001, "Deprecated"},
	{0x00B7, "Diacritic"},
	{0x30FE, "Extender"},
	{0xFF46, "Hex_Digit"},
	{0x2E17, "Hyphen"},
	{0x2FFB, "IDS_Binary_Operator"},
	{0x2FF3, "IDS_Trinary_Operator"},
	{0xFA6A, "Ideographic"},
	{0x200D, "Join_Control"},
	{0x0EC4, "Logical_Order_Exception"},
	{0x2FFFF, "Noncharacter_Code_Point"},
	{0x065E, "Other_Alphabetic"},
	{0x2065, "Other_Default_Ignorable_Code_Point"},
	{0x0BD7, "Other_Grapheme_Extend"},
	{0x0387, "Other_ID_Continue"},
	{0x212E, "Other_ID_Start"},
	{0x2094, "Other_Lowercase"},
	{0x2040, "Other_Math"},
	{0x216F, "Other_Uppercase"},
	{0x0027, "Pattern_Syntax"},
	{0x0020, "Pattern_White_Space"},
	{0x06DD, "Prepended_Concatenation_Mark"},
	{0x300D, "Quotation_Mark"},
	{0x2EF3, "Radical"},
	{0x1f1ff, "Regional_Indicator"},
	{0x061F, "STerm"}, // Deprecated alias of Sentence_Terminal
	{0x061F, "Sentence_Terminal"},
	{0x2071, "Soft_Dotted"},
	{0x003A, "Terminal_Punctuation"},
	{0x9FC3, "Unified_Ideograph"},
	{0xFE0F, "Variation_Selector"},
	{0x0020, "White_Space"},
}

func TestCategories(t *testT) {
	notTested := make(map[string]bool)
	for k := range unicode.Categories {
		notTested[k] = true
	}
	for _, test := range inCategoryTest {
		checks++
		if _, ok := unicode.Categories[test.script]; !ok {
			t.Fatal(test.script, "not a known category")
		}
		checks++
		if !unicode.Is(unicode.Categories[test.script], test.rune) {
			t.Errorf("IsCategory(%U, %s) = false, want true", test.rune, test.script)
		}
		delete(notTested, test.script)
	}
	for k := range notTested {
		t.Error("category not tested:", k)
	}
}

func TestProperties(t *testT) {
	notTested := make(map[string]bool)
	for k := range unicode.Properties {
		notTested[k] = true
	}
	for _, test := range inPropTest {
		checks++
		if _, ok := unicode.Properties[test.script]; !ok {
			t.Fatal(test.script, "not a known prop")
		}
		checks++
		if !unicode.Is(unicode.Properties[test.script], test.rune) {
			t.Errorf("IsCategory(%U, %s) = false, want true", test.rune, test.script)
		}
		delete(notTested, test.script)
	}
	for k := range notTested {
		t.Error("property not tested:", k)
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
		{"TestCategories", TestCategories},
		{"TestProperties", TestProperties},
	}
	runTests(tests_)
}

// Output:
// ok TestCategories subtests: 0 checks: 78
// ok TestProperties subtests: 0 checks: 70
// tests: 2 failed: 0
