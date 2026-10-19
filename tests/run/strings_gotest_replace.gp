// phase: 09
// Go 1.25's strings/replace_test.go (NewReplacer: the generic, byte and single-string replacers, WriteString, the table of cases) through a small harness; expected output from real Go.

package main

import (
	"bytes"
	"fmt"
	"strings"
	"sync/atomic"
)


var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&apos;",
)

var htmlUnescaper = strings.NewReplacer(
	"&amp;", "&",
	"&lt;", "<",
	"&gt;", ">",
	"&quot;", `"`,
	"&apos;", "'",
)

// The http package's old HTML escaping function.
func oldHTMLEscape(s string) string {
	s = strings.Replace(s, "&", "&amp;", -1)
	s = strings.Replace(s, "<", "&lt;", -1)
	s = strings.Replace(s, ">", "&gt;", -1)
	s = strings.Replace(s, `"`, "&quot;", -1)
	s = strings.Replace(s, "'", "&apos;", -1)
	return s
}

var capitalLetters = strings.NewReplacer("a", "A", "b", "B")

// TestReplacer tests the replacer implementations.
func TestReplacer(t *testT) {
	type testCase struct {
		r       *strings.Replacer
		in, out string
	}
	var testCases []testCase

	// str converts 0xff to "\xff". This isn't just string(b) since that converts to UTF-8.
	str := func(b byte) string {
		return string([]byte{b})
	}
	var s []string

	// inc maps "\x00"->"\x01", ..., "a"->"b", "b"->"c", ..., "\xff"->"\x00".
	s = nil
	for i := 0; i < 256; i++ {
		s = append(s, str(byte(i)), str(byte(i+1)))
	}
	inc := strings.NewReplacer(s...)

	// Test cases with 1-byte old strings, 1-byte new strings.
	testCases = append(testCases,
		testCase{capitalLetters, "brad", "BrAd"},
		testCase{capitalLetters, strings.Repeat("a", (32<<10)+123), strings.Repeat("A", (32<<10)+123)},
		testCase{capitalLetters, "", ""},

		testCase{inc, "brad", "csbe"},
		testCase{inc, "\x00\xff", "\x01\x00"},
		testCase{inc, "", ""},

		testCase{strings.NewReplacer("a", "1", "a", "2"), "brad", "br1d"},
	)

	// repeat maps "a"->"a", "b"->"bb", "c"->"ccc", ...
	s = nil
	for i := 0; i < 256; i++ {
		n := i + 1 - 'a'
		atomic.AddInt64(&checks, 1)
		if n < 1 {
			n = 1
		}
		s = append(s, str(byte(i)), strings.Repeat(str(byte(i)), n))
	}
	repeat := strings.NewReplacer(s...)

	// Test cases with 1-byte old strings, variable length new strings.
	testCases = append(testCases,
		testCase{htmlEscaper, "No changes", "No changes"},
		testCase{htmlEscaper, "I <3 escaping & stuff", "I &lt;3 escaping &amp; stuff"},
		testCase{htmlEscaper, "&&&", "&amp;&amp;&amp;"},
		testCase{htmlEscaper, "", ""},

		testCase{repeat, "brad", "bbrrrrrrrrrrrrrrrrrradddd"},
		testCase{repeat, "abba", "abbbba"},
		testCase{repeat, "", ""},

		testCase{strings.NewReplacer("a", "11", "a", "22"), "brad", "br11d"},
	)

	// The remaining test cases have variable length old strings.

	testCases = append(testCases,
		testCase{htmlUnescaper, "&amp;amp;", "&amp;"},
		testCase{htmlUnescaper, "&lt;b&gt;HTML&apos;s neat&lt;/b&gt;", "<b>HTML's neat</b>"},
		testCase{htmlUnescaper, "", ""},

		testCase{strings.NewReplacer("a", "1", "a", "2", "xxx", "xxx"), "brad", "br1d"},

		testCase{strings.NewReplacer("a", "1", "aa", "2", "aaa", "3"), "aaaa", "1111"},

		testCase{strings.NewReplacer("aaa", "3", "aa", "2", "a", "1"), "aaaa", "31"},
	)

	// gen1 has multiple old strings of variable length. There is no
	// overall non-empty common prefix, but some pairwise common prefixes.
	gen1 := strings.NewReplacer(
		"aaa", "3[aaa]",
		"aa", "2[aa]",
		"a", "1[a]",
		"i", "i",
		"longerst", "most long",
		"longer", "medium",
		"long", "short",
		"xx", "xx",
		"x", "X",
		"X", "Y",
		"Y", "Z",
	)
	testCases = append(testCases,
		testCase{gen1, "fooaaabar", "foo3[aaa]b1[a]r"},
		testCase{gen1, "long, longerst, longer", "short, most long, medium"},
		testCase{gen1, "xxxxx", "xxxxX"},
		testCase{gen1, "XiX", "YiY"},
		testCase{gen1, "", ""},
	)

	// gen2 has multiple old strings with no pairwise common prefix.
	gen2 := strings.NewReplacer(
		"roses", "red",
		"violets", "blue",
		"sugar", "sweet",
	)
	testCases = append(testCases,
		testCase{gen2, "roses are red, violets are blue...", "red are red, blue are blue..."},
		testCase{gen2, "", ""},
	)

	// gen3 has multiple old strings with an overall common prefix.
	gen3 := strings.NewReplacer(
		"abracadabra", "poof",
		"abracadabrakazam", "splat",
		"abraham", "lincoln",
		"abrasion", "scrape",
		"abraham", "isaac",
	)
	testCases = append(testCases,
		testCase{gen3, "abracadabrakazam abraham", "poofkazam lincoln"},
		testCase{gen3, "abrasion abracad", "scrape abracad"},
		testCase{gen3, "abba abram abrasive", "abba abram abrasive"},
		testCase{gen3, "", ""},
	)

	// foo{1,2,3,4} have multiple old strings with an overall common prefix
	// and 1- or 2- byte extensions from the common prefix.
	foo1 := strings.NewReplacer(
		"foo1", "A",
		"foo2", "B",
		"foo3", "C",
	)
	foo2 := strings.NewReplacer(
		"foo1", "A",
		"foo2", "B",
		"foo31", "C",
		"foo32", "D",
	)
	foo3 := strings.NewReplacer(
		"foo11", "A",
		"foo12", "B",
		"foo31", "C",
		"foo32", "D",
	)
	foo4 := strings.NewReplacer(
		"foo12", "B",
		"foo32", "D",
	)
	testCases = append(testCases,
		testCase{foo1, "fofoofoo12foo32oo", "fofooA2C2oo"},
		testCase{foo1, "", ""},

		testCase{foo2, "fofoofoo12foo32oo", "fofooA2Doo"},
		testCase{foo2, "", ""},

		testCase{foo3, "fofoofoo12foo32oo", "fofooBDoo"},
		testCase{foo3, "", ""},

		testCase{foo4, "fofoofoo12foo32oo", "fofooBDoo"},
		testCase{foo4, "", ""},
	)

	// genAll maps "\x00\x01\x02...\xfe\xff" to "[all]", amongst other things.
	allBytes := make([]byte, 256)
	for i := range allBytes {
		allBytes[i] = byte(i)
	}
	allString := string(allBytes)
	genAll := strings.NewReplacer(
		allString, "[all]",
		"\xff", "[ff]",
		"\x00", "[00]",
	)
	testCases = append(testCases,
		testCase{genAll, allString, "[all]"},
		testCase{genAll, "a\xff" + allString + "\x00", "a[ff][all][00]"},
		testCase{genAll, "", ""},
	)

	// Test cases with empty old strings.

	blankToX1 := strings.NewReplacer("", "X")
	blankToX2 := strings.NewReplacer("", "X", "", "")
	blankHighPriority := strings.NewReplacer("", "X", "o", "O")
	blankLowPriority := strings.NewReplacer("o", "O", "", "X")
	blankNoOp1 := strings.NewReplacer("", "")
	blankNoOp2 := strings.NewReplacer("", "", "", "A")
	blankFoo := strings.NewReplacer("", "X", "foobar", "R", "foobaz", "Z")
	testCases = append(testCases,
		testCase{blankToX1, "foo", "XfXoXoX"},
		testCase{blankToX1, "", "X"},

		testCase{blankToX2, "foo", "XfXoXoX"},
		testCase{blankToX2, "", "X"},

		testCase{blankHighPriority, "oo", "XOXOX"},
		testCase{blankHighPriority, "ii", "XiXiX"},
		testCase{blankHighPriority, "oiio", "XOXiXiXOX"},
		testCase{blankHighPriority, "iooi", "XiXOXOXiX"},
		testCase{blankHighPriority, "", "X"},

		testCase{blankLowPriority, "oo", "OOX"},
		testCase{blankLowPriority, "ii", "XiXiX"},
		testCase{blankLowPriority, "oiio", "OXiXiOX"},
		testCase{blankLowPriority, "iooi", "XiOOXiX"},
		testCase{blankLowPriority, "", "X"},

		testCase{blankNoOp1, "foo", "foo"},
		testCase{blankNoOp1, "", ""},

		testCase{blankNoOp2, "foo", "foo"},
		testCase{blankNoOp2, "", ""},

		testCase{blankFoo, "foobarfoobaz", "XRXZX"},
		testCase{blankFoo, "foobar-foobaz", "XRX-XZX"},
		testCase{blankFoo, "", "X"},
	)

	// single string replacer

	abcMatcher := strings.NewReplacer("abc", "[match]")

	testCases = append(testCases,
		testCase{abcMatcher, "", ""},
		testCase{abcMatcher, "ab", "ab"},
		testCase{abcMatcher, "abc", "[match]"},
		testCase{abcMatcher, "abcd", "[match]d"},
		testCase{abcMatcher, "cabcabcdabca", "c[match][match]d[match]a"},
	)

	// Issue 6659 cases (more single string replacer)

	noHello := strings.NewReplacer("Hello", "")
	testCases = append(testCases,
		testCase{noHello, "Hello", ""},
		testCase{noHello, "Hellox", "x"},
		testCase{noHello, "xHello", "x"},
		testCase{noHello, "xHellox", "xx"},
	)

	// No-arg test cases.

	nop := strings.NewReplacer()
	testCases = append(testCases,
		testCase{nop, "abc", "abc"},
		testCase{nop, "", ""},
	)

	// Run the test cases.

	for i, tc := range testCases {
		atomic.AddInt64(&checks, 1)
		if s := tc.r.Replace(tc.in); s != tc.out {
			t.Errorf("%d. Replace(%q) = %q, want %q", i, tc.in, s, tc.out)
		}
		var buf bytes.Buffer
		n, err := tc.r.WriteString(&buf, tc.in)
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf("%d. WriteString: %v", i, err)
			continue
		}
		got := buf.String()
		atomic.AddInt64(&checks, 1)
		if got != tc.out {
			t.Errorf("%d. WriteString(%q) wrote %q, want %q", i, tc.in, got, tc.out)
			continue
		}
		atomic.AddInt64(&checks, 1)
		if n != len(tc.out) {
			t.Errorf("%d. WriteString(%q) wrote correct string but reported %d bytes; want %d (%q)",
				i, tc.in, n, len(tc.out), tc.out)
		}
	}
}

var algorithmTestCases = []struct {
	r    *strings.Replacer
	want string
}{
	{capitalLetters, "*strings.byteReplacer"},
	{htmlEscaper, "*strings.byteStringReplacer"},
	{strings.NewReplacer("12", "123"), "*strings.singleStringReplacer"},
	{strings.NewReplacer("1", "12"), "*strings.byteStringReplacer"},
	{strings.NewReplacer("", "X"), "*strings.genericReplacer"},
	{strings.NewReplacer("a", "1", "b", "12", "cde", "123"), "*strings.genericReplacer"},
}


type errWriter struct{}

func (errWriter) Write(p []byte) (n int, err error) {
	return 0, fmt.Errorf("unwritable")
}

// TestWriteStringError tests that WriteString returns an error
// received from the underlying io.Writer.
func TestWriteStringError(t *testT) {
	for i, tc := range algorithmTestCases {
		n, err := tc.r.WriteString(errWriter{}, "abc")
		atomic.AddInt64(&checks, 1)
		if n != 0 || err == nil || err.Error() != "unwritable" {
			t.Errorf("%d. WriteStringError = %d, %v, want 0, unwritable", i, n, err)
		}
	}
}


















var mapdata = []struct{ name, data string }{
	{"ASCII", "a b c d e f g h i j k l m n o p q r s t u v w x y z"},
	{"Greek", "α β γ δ ε ζ η θ ι κ λ μ ν ξ ο π ρ ς σ τ υ φ χ ψ ω"},
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
		{"TestReplacer", TestReplacer},
		{"TestWriteStringError", TestWriteStringError},
	}
	runTests(tests_)
}

// Output:
// ok TestReplacer subtests: 0 checks: 556
// ok TestWriteStringError subtests: 0 checks: 6
// tests: 2 failed: 0
