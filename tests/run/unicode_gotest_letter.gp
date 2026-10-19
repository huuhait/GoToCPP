// phase: 09
// Go 1.25's unicode/letter_test.go (IsLetter, IsUpper, To*, SimpleFold, TurkishCase, SpecialCase, negative runes, Latin-1 offsets, the optimized paths) through a small harness; expected output from real Go.

package main

import (
	"fmt"
	"unicode"
)

var upperTest = []rune{
	0x41,
	0xc0,
	0xd8,
	0x100,
	0x139,
	0x14a,
	0x178,
	0x181,
	0x376,
	0x3cf,
	0x13bd,
	0x1f2a,
	0x2102,
	0x2c00,
	0x2c10,
	0x2c20,
	0xa650,
	0xa722,
	0xff3a,
	0x10400,
	0x1d400,
	0x1d7ca,
}

var notupperTest = []rune{
	0x40,
	0x5b,
	0x61,
	0x185,
	0x1b0,
	0x377,
	0x387,
	0x2150,
	0xab7d,
	0xffff,
	0x10000,
}

var letterTest = []rune{
	0x41,
	0x61,
	0xaa,
	0xba,
	0xc8,
	0xdb,
	0xf9,
	0x2ec,
	0x535,
	0x620,
	0x6e6,
	0x93d,
	0xa15,
	0xb99,
	0xdc0,
	0xedd,
	0x1000,
	0x1200,
	0x1312,
	0x1401,
	0x2c00,
	0xa800,
	0xf900,
	0xfa30,
	0xffda,
	0xffdc,
	0x10000,
	0x10300,
	0x10400,
	0x20000,
	0x2f800,
	0x2fa1d,
}

var notletterTest = []rune{
	0x20,
	0x35,
	0x375,
	0x619,
	0x700,
	0x1885,
	0xfffe,
	0x1ffff,
	0x10ffff,
}

// Contains all the special cased Latin-1 chars.
var spaceTest = []rune{
	0x09,
	0x0a,
	0x0b,
	0x0c,
	0x0d,
	0x20,
	0x85,
	0xA0,
	0x2000,
	0x3000,
}

type caseT struct {
	cas     int
	in, out rune
}

var caseTest = []caseT{
	// errors
	{-1, '\n', 0xFFFD},
	{unicode.UpperCase, -1, -1},
	{unicode.UpperCase, 1 << 30, 1 << 30},

	// ASCII (special-cased so test carefully)
	{unicode.UpperCase, '\n', '\n'},
	{unicode.UpperCase, 'a', 'A'},
	{unicode.UpperCase, 'A', 'A'},
	{unicode.UpperCase, '7', '7'},
	{unicode.LowerCase, '\n', '\n'},
	{unicode.LowerCase, 'a', 'a'},
	{unicode.LowerCase, 'A', 'a'},
	{unicode.LowerCase, '7', '7'},
	{unicode.TitleCase, '\n', '\n'},
	{unicode.TitleCase, 'a', 'A'},
	{unicode.TitleCase, 'A', 'A'},
	{unicode.TitleCase, '7', '7'},

	// Latin-1: easy to read the tests!
	{unicode.UpperCase, 0x80, 0x80},
	{unicode.UpperCase, 'Å', 'Å'},
	{unicode.UpperCase, 'å', 'Å'},
	{unicode.LowerCase, 0x80, 0x80},
	{unicode.LowerCase, 'Å', 'å'},
	{unicode.LowerCase, 'å', 'å'},
	{unicode.TitleCase, 0x80, 0x80},
	{unicode.TitleCase, 'Å', 'Å'},
	{unicode.TitleCase, 'å', 'Å'},

	// 0131;LATIN SMALL LETTER DOTLESS I;Ll;0;L;;;;;N;;;0049;;0049
	{unicode.UpperCase, 0x0131, 'I'},
	{unicode.LowerCase, 0x0131, 0x0131},
	{unicode.TitleCase, 0x0131, 'I'},

	// 0133;LATIN SMALL LIGATURE IJ;Ll;0;L;<compat> 0069 006A;;;;N;LATIN SMALL LETTER I J;;0132;;0132
	{unicode.UpperCase, 0x0133, 0x0132},
	{unicode.LowerCase, 0x0133, 0x0133},
	{unicode.TitleCase, 0x0133, 0x0132},

	// 212A;KELVIN SIGN;Lu;0;L;004B;;;;N;DEGREES KELVIN;;;006B;
	{unicode.UpperCase, 0x212A, 0x212A},
	{unicode.LowerCase, 0x212A, 'k'},
	{unicode.TitleCase, 0x212A, 0x212A},

	// From an UpperLower sequence
	// A640;CYRILLIC CAPITAL LETTER ZEMLYA;Lu;0;L;;;;;N;;;;A641;
	{unicode.UpperCase, 0xA640, 0xA640},
	{unicode.LowerCase, 0xA640, 0xA641},
	{unicode.TitleCase, 0xA640, 0xA640},
	// A641;CYRILLIC SMALL LETTER ZEMLYA;Ll;0;L;;;;;N;;;A640;;A640
	{unicode.UpperCase, 0xA641, 0xA640},
	{unicode.LowerCase, 0xA641, 0xA641},
	{unicode.TitleCase, 0xA641, 0xA640},
	// A64E;CYRILLIC CAPITAL LETTER NEUTRAL YER;Lu;0;L;;;;;N;;;;A64F;
	{unicode.UpperCase, 0xA64E, 0xA64E},
	{unicode.LowerCase, 0xA64E, 0xA64F},
	{unicode.TitleCase, 0xA64E, 0xA64E},
	// A65F;CYRILLIC SMALL LETTER YN;Ll;0;L;;;;;N;;;A65E;;A65E
	{unicode.UpperCase, 0xA65F, 0xA65E},
	{unicode.LowerCase, 0xA65F, 0xA65F},
	{unicode.TitleCase, 0xA65F, 0xA65E},

	// From another UpperLower sequence
	// 0139;LATIN CAPITAL LETTER L WITH ACUTE;Lu;0;L;004C 0301;;;;N;LATIN CAPITAL LETTER L ACUTE;;;013A;
	{unicode.UpperCase, 0x0139, 0x0139},
	{unicode.LowerCase, 0x0139, 0x013A},
	{unicode.TitleCase, 0x0139, 0x0139},
	// 013F;LATIN CAPITAL LETTER L WITH MIDDLE DOT;Lu;0;L;<compat> 004C 00B7;;;;N;;;;0140;
	{unicode.UpperCase, 0x013f, 0x013f},
	{unicode.LowerCase, 0x013f, 0x0140},
	{unicode.TitleCase, 0x013f, 0x013f},
	// 0148;LATIN SMALL LETTER N WITH CARON;Ll;0;L;006E 030C;;;;N;LATIN SMALL LETTER N HACEK;;0147;;0147
	{unicode.UpperCase, 0x0148, 0x0147},
	{unicode.LowerCase, 0x0148, 0x0148},
	{unicode.TitleCase, 0x0148, 0x0147},

	// Lowercase lower than uppercase.
	// AB78;CHEROKEE SMALL LETTER GE;Ll;0;L;;;;;N;;;13A8;;13A8
	{unicode.UpperCase, 0xab78, 0x13a8},
	{unicode.LowerCase, 0xab78, 0xab78},
	{unicode.TitleCase, 0xab78, 0x13a8},
	{unicode.UpperCase, 0x13a8, 0x13a8},
	{unicode.LowerCase, 0x13a8, 0xab78},
	{unicode.TitleCase, 0x13a8, 0x13a8},

	// Last block in the 5.1.0 table
	// 10400;DESERET CAPITAL LETTER LONG I;Lu;0;L;;;;;N;;;;10428;
	{unicode.UpperCase, 0x10400, 0x10400},
	{unicode.LowerCase, 0x10400, 0x10428},
	{unicode.TitleCase, 0x10400, 0x10400},
	// 10427;DESERET CAPITAL LETTER EW;Lu;0;L;;;;;N;;;;1044F;
	{unicode.UpperCase, 0x10427, 0x10427},
	{unicode.LowerCase, 0x10427, 0x1044F},
	{unicode.TitleCase, 0x10427, 0x10427},
	// 10428;DESERET SMALL LETTER LONG I;Ll;0;L;;;;;N;;;10400;;10400
	{unicode.UpperCase, 0x10428, 0x10400},
	{unicode.LowerCase, 0x10428, 0x10428},
	{unicode.TitleCase, 0x10428, 0x10400},
	// 1044F;DESERET SMALL LETTER EW;Ll;0;L;;;;;N;;;10427;;10427
	{unicode.UpperCase, 0x1044F, 0x10427},
	{unicode.LowerCase, 0x1044F, 0x1044F},
	{unicode.TitleCase, 0x1044F, 0x10427},

	// First one not in the 5.1.0 table
	// 10450;SHAVIAN LETTER PEEP;Lo;0;L;;;;;N;;;;;
	{unicode.UpperCase, 0x10450, 0x10450},
	{unicode.LowerCase, 0x10450, 0x10450},
	{unicode.TitleCase, 0x10450, 0x10450},

	// Non-letters with case.
	{unicode.LowerCase, 0x2161, 0x2171},
	{unicode.UpperCase, 0x0345, 0x0399},
}

func TestIsLetter(t *testT) {
	for _, r := range upperTest {
		checks++
		if !unicode.IsLetter(r) {
			t.Errorf("IsLetter(U+%04X) = false, want true", r)
		}
	}
	for _, r := range letterTest {
		checks++
		if !unicode.IsLetter(r) {
			t.Errorf("IsLetter(U+%04X) = false, want true", r)
		}
	}
	for _, r := range notletterTest {
		checks++
		if unicode.IsLetter(r) {
			t.Errorf("IsLetter(U+%04X) = true, want false", r)
		}
	}
}

func TestIsUpper(t *testT) {
	for _, r := range upperTest {
		checks++
		if !unicode.IsUpper(r) {
			t.Errorf("IsUpper(U+%04X) = false, want true", r)
		}
	}
	for _, r := range notupperTest {
		checks++
		if unicode.IsUpper(r) {
			t.Errorf("IsUpper(U+%04X) = true, want false", r)
		}
	}
	for _, r := range notletterTest {
		checks++
		if unicode.IsUpper(r) {
			t.Errorf("IsUpper(U+%04X) = true, want false", r)
		}
	}
}

func caseString(c int) string {
	switch c {
	case unicode.UpperCase:
		return "UpperCase"
	case unicode.LowerCase:
		return "LowerCase"
	case unicode.TitleCase:
		return "TitleCase"
	}
	return "ErrorCase"
}

func TestTo(t *testT) {
	for _, c := range caseTest {
		r := unicode.To(c.cas, c.in)
		checks++
		if c.out != r {
			t.Errorf("To(U+%04X, %s) = U+%04X want U+%04X", c.in, caseString(c.cas), r, c.out)
		}
	}
}

func TestToUpperCase(t *testT) {
	for _, c := range caseTest {
		checks++
		if c.cas != unicode.UpperCase {
			continue
		}
		r := unicode.ToUpper(c.in)
		checks++
		if c.out != r {
			t.Errorf("ToUpper(U+%04X) = U+%04X want U+%04X", c.in, r, c.out)
		}
	}
}

func TestToLowerCase(t *testT) {
	for _, c := range caseTest {
		checks++
		if c.cas != unicode.LowerCase {
			continue
		}
		r := unicode.ToLower(c.in)
		checks++
		if c.out != r {
			t.Errorf("ToLower(U+%04X) = U+%04X want U+%04X", c.in, r, c.out)
		}
	}
}

func TestToTitleCase(t *testT) {
	for _, c := range caseTest {
		checks++
		if c.cas != unicode.TitleCase {
			continue
		}
		r := unicode.ToTitle(c.in)
		checks++
		if c.out != r {
			t.Errorf("ToTitle(U+%04X) = U+%04X want U+%04X", c.in, r, c.out)
		}
	}
}

func TestIsSpace(t *testT) {
	for _, c := range spaceTest {
		checks++
		if !unicode.IsSpace(c) {
			t.Errorf("IsSpace(U+%04X) = false; want true", c)
		}
	}
	for _, c := range letterTest {
		checks++
		if unicode.IsSpace(c) {
			t.Errorf("IsSpace(U+%04X) = true; want false", c)
		}
	}
}

// Check that the optimizations for IsLetter etc. agree with the tables.
// We only need to check the Latin-1 range.
func TestLetterOptimizations(t *testT) {
	for i := rune(0); i <= unicode.MaxLatin1; i++ {
		checks++
		if unicode.Is(unicode.Letter, i) != unicode.IsLetter(i) {
			t.Errorf("IsLetter(U+%04X) disagrees with Is(Letter)", i)
		}
		checks++
		if unicode.Is(unicode.Upper, i) != unicode.IsUpper(i) {
			t.Errorf("IsUpper(U+%04X) disagrees with Is(Upper)", i)
		}
		checks++
		if unicode.Is(unicode.Lower, i) != unicode.IsLower(i) {
			t.Errorf("IsLower(U+%04X) disagrees with Is(Lower)", i)
		}
		checks++
		if unicode.Is(unicode.Title, i) != unicode.IsTitle(i) {
			t.Errorf("IsTitle(U+%04X) disagrees with Is(Title)", i)
		}
		checks++
		if unicode.Is(unicode.White_Space, i) != unicode.IsSpace(i) {
			t.Errorf("IsSpace(U+%04X) disagrees with Is(White_Space)", i)
		}
		checks++
		if unicode.To(unicode.UpperCase, i) != unicode.ToUpper(i) {
			t.Errorf("ToUpper(U+%04X) disagrees with To(Upper)", i)
		}
		checks++
		if unicode.To(unicode.LowerCase, i) != unicode.ToLower(i) {
			t.Errorf("ToLower(U+%04X) disagrees with To(Lower)", i)
		}
		checks++
		if unicode.To(unicode.TitleCase, i) != unicode.ToTitle(i) {
			t.Errorf("ToTitle(U+%04X) disagrees with To(Title)", i)
		}
	}
}

func TestTurkishCase(t *testT) {
	lower := []rune("abcçdefgğhıijklmnoöprsştuüvyz")
	upper := []rune("ABCÇDEFGĞHIİJKLMNOÖPRSŞTUÜVYZ")
	for i, l := range lower {
		u := upper[i]
		checks++
		if unicode.TurkishCase.ToLower(l) != l {
			t.Errorf("lower(U+%04X) is U+%04X not U+%04X", l, unicode.TurkishCase.ToLower(l), l)
		}
		checks++
		if unicode.TurkishCase.ToUpper(u) != u {
			t.Errorf("upper(U+%04X) is U+%04X not U+%04X", u, unicode.TurkishCase.ToUpper(u), u)
		}
		checks++
		if unicode.TurkishCase.ToUpper(l) != u {
			t.Errorf("upper(U+%04X) is U+%04X not U+%04X", l, unicode.TurkishCase.ToUpper(l), u)
		}
		checks++
		if unicode.TurkishCase.ToLower(u) != l {
			t.Errorf("lower(U+%04X) is U+%04X not U+%04X", u, unicode.TurkishCase.ToLower(l), l)
		}
		checks++
		if unicode.TurkishCase.ToTitle(u) != u {
			t.Errorf("title(U+%04X) is U+%04X not U+%04X", u, unicode.TurkishCase.ToTitle(u), u)
		}
		checks++
		if unicode.TurkishCase.ToTitle(l) != u {
			t.Errorf("title(U+%04X) is U+%04X not U+%04X", l, unicode.TurkishCase.ToTitle(l), u)
		}
	}
}

var simpleFoldTests = []string{
	// SimpleFold(x) returns the next equivalent rune > x or wraps
	// around to smaller values.

	// Easy cases.
	"Aa",
	"δΔ",

	// ASCII special cases.
	"KkK",
	"Ssſ",

	// Non-ASCII special cases.
	"ρϱΡ",
	"ͅΙιι",

	// Extra special cases: has lower/upper but no case fold.
	"İ",
	"ı",

	// Upper comes before lower (Cherokee).
	"\u13b0\uab80",
}

func TestSimpleFold(t *testT) {
	for _, tt := range simpleFoldTests {
		cycle := []rune(tt)
		r := cycle[len(cycle)-1]
		for _, out := range cycle {
			checks++
			if r := unicode.SimpleFold(r); r != out {
				t.Errorf("SimpleFold(%#U) = %#U, want %#U", r, r, out)
			}
			r = out
		}
	}

	checks++
	if r := unicode.SimpleFold(-42); r != -42 {
		t.Errorf("SimpleFold(-42) = %v, want -42", r)
	}
}

// Running 'go test -calibrate' runs the calibration to find a plausible
// cutoff point for linear search of a range list vs. binary search.
// We create a fake table and then time how long it takes to do a
// sequence of searches within that table, for all possible inputs
// relative to the ranges (something before all, in each, between each, after all).
// This assumes that all possible runes are equally likely.
// In practice most runes are ASCII so this is a conservative estimate
// of an effective cutoff value. In practice we could probably set it higher
// than what this function recommends.

func TestLatinOffset(t *testT) {
	var maps = []map[string]*unicode.RangeTable{
		unicode.Categories,
		unicode.FoldCategory,
		unicode.FoldScript,
		unicode.Properties,
		unicode.Scripts,
	}
	for _, m := range maps {
		for name, tab := range m {
			i := 0
			for i < len(tab.R16) && tab.R16[i].Hi <= unicode.MaxLatin1 {
				i++
			}
			checks++
			if tab.LatinOffset != i {
				t.Errorf("%s: LatinOffset=%d, want %d", name, tab.LatinOffset, i)
			}
		}
	}
}

func TestNegativeRune(t *testT) {
	// Issue 43254
	// These tests cover negative rune handling by testing values which,
	// when cast to uint8 or uint16, look like a particular valid rune.
	// This package has Latin-1-specific optimizations, so we test all of
	// Latin-1 and representative non-Latin-1 values in the character
	// categories covered by IsGraphic, etc.
	nonLatin1 := []uint32{
		// Lu: LATIN CAPITAL LETTER A WITH MACRON
		0x0100,
		// Ll: LATIN SMALL LETTER A WITH MACRON
		0x0101,
		// Lt: LATIN CAPITAL LETTER D WITH SMALL LETTER Z WITH CARON
		0x01C5,
		// M: COMBINING GRAVE ACCENT
		0x0300,
		// Nd: ARABIC-INDIC DIGIT ZERO
		0x0660,
		// P: GREEK QUESTION MARK
		0x037E,
		// S: MODIFIER LETTER LEFT ARROWHEAD
		0x02C2,
		// Z: OGHAM SPACE MARK
		0x1680,
	}
	for i := 0; i < unicode.MaxLatin1+len(nonLatin1); i++ {
		base := uint32(i)
		checks++
		if i >= unicode.MaxLatin1 {
			base = nonLatin1[i-unicode.MaxLatin1]
		}

		// Note r is negative, but uint8(r) == uint8(base) and
		// uint16(r) == uint16(base).
		r := rune(base - 1<<31)
		checks++
		if unicode.Is(unicode.Letter, r) {
			t.Errorf("Is(Letter, 0x%x - 1<<31) = true, want false", base)
		}
		checks++
		if unicode.IsControl(r) {
			t.Errorf("IsControl(0x%x - 1<<31) = true, want false", base)
		}
		checks++
		if unicode.IsDigit(r) {
			t.Errorf("IsDigit(0x%x - 1<<31) = true, want false", base)
		}
		checks++
		if unicode.IsGraphic(r) {
			t.Errorf("IsGraphic(0x%x - 1<<31) = true, want false", base)
		}
		checks++
		if unicode.IsLetter(r) {
			t.Errorf("IsLetter(0x%x - 1<<31) = true, want false", base)
		}
		checks++
		if unicode.IsLower(r) {
			t.Errorf("IsLower(0x%x - 1<<31) = true, want false", base)
		}
		checks++
		if unicode.IsMark(r) {
			t.Errorf("IsMark(0x%x - 1<<31) = true, want false", base)
		}
		checks++
		if unicode.IsNumber(r) {
			t.Errorf("IsNumber(0x%x - 1<<31) = true, want false", base)
		}
		checks++
		if unicode.IsPrint(r) {
			t.Errorf("IsPrint(0x%x - 1<<31) = true, want false", base)
		}
		checks++
		if unicode.IsPunct(r) {
			t.Errorf("IsPunct(0x%x - 1<<31) = true, want false", base)
		}
		checks++
		if unicode.IsSpace(r) {
			t.Errorf("IsSpace(0x%x - 1<<31) = true, want false", base)
		}
		checks++
		if unicode.IsSymbol(r) {
			t.Errorf("IsSymbol(0x%x - 1<<31) = true, want false", base)
		}
		checks++
		if unicode.IsTitle(r) {
			t.Errorf("IsTitle(0x%x - 1<<31) = true, want false", base)
		}
		checks++
		if unicode.IsUpper(r) {
			t.Errorf("IsUpper(0x%x - 1<<31) = true, want false", base)
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
		{"TestIsLetter", TestIsLetter},
		{"TestIsUpper", TestIsUpper},
		{"TestTo", TestTo},
		{"TestToUpperCase", TestToUpperCase},
		{"TestToLowerCase", TestToLowerCase},
		{"TestToTitleCase", TestToTitleCase},
		{"TestIsSpace", TestIsSpace},
		{"TestLetterOptimizations", TestLetterOptimizations},
		{"TestTurkishCase", TestTurkishCase},
		{"TestSimpleFold", TestSimpleFold},
		{"TestLatinOffset", TestLatinOffset},
		{"TestNegativeRune", TestNegativeRune},
	}
	runTests(tests_)
}

// Output:
// ok TestIsLetter subtests: 0 checks: 63
// ok TestIsUpper subtests: 0 checks: 42
// ok TestTo subtests: 0 checks: 77
// ok TestToUpperCase subtests: 0 checks: 104
// ok TestToLowerCase subtests: 0 checks: 102
// ok TestToTitleCase subtests: 0 checks: 101
// ok TestIsSpace subtests: 0 checks: 42
// ok TestLetterOptimizations subtests: 0 checks: 2048
// ok TestTurkishCase subtests: 0 checks: 174
// ok TestSimpleFold subtests: 0 checks: 22
// ok TestLatinOffset subtests: 0 checks: 245
// ok TestNegativeRune subtests: 0 checks: 3945
// tests: 12 failed: 0
