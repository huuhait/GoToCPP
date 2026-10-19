// phase: 09
// Go 1.25's strconv/quote_test.go (Quote, QuoteToASCII, QuoteToGraphic, QuoteRune*, Unquote, UnquoteChar, QuotedPrefix, CanBackquote, IsPrint, IsGraphic) through a small harness; expected output from real Go.

package main

import (
	"fmt"
	. "strconv"
	"unicode"
)

func replaceAll(s, old string) string { // strings.ReplaceAll(s, old, "") for a one-byte old
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != old[0] {
			b = append(b, s[i])
		}
	}
	return string(b)
}

// Verify that our IsPrint agrees with unicode.IsPrint.
func TestIsPrint(t *testT) {
	n := 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		checks++
		if IsPrint(r) != unicode.IsPrint(r) {
			t.Errorf("IsPrint(%U)=%t incorrect", r, IsPrint(r))
			n++
			checks++
			if n > 10 {
				return
			}
		}
	}
}

// Verify that our IsGraphic agrees with unicode.IsGraphic.
func TestIsGraphic(t *testT) {
	n := 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		checks++
		if IsGraphic(r) != unicode.IsGraphic(r) {
			t.Errorf("IsGraphic(%U)=%t incorrect", r, IsGraphic(r))
			n++
			checks++
			if n > 10 {
				return
			}
		}
	}
}

type quoteTest struct {
	in      string
	out     string
	ascii   string
	graphic string
}

var quotetests = []quoteTest{
	{"\a\b\f\r\n\t\v", `"\a\b\f\r\n\t\v"`, `"\a\b\f\r\n\t\v"`, `"\a\b\f\r\n\t\v"`},
	{"\\", `"\\"`, `"\\"`, `"\\"`},
	{"abc\xffdef", `"abc\xffdef"`, `"abc\xffdef"`, `"abc\xffdef"`},
	{"\u263a", `"☺"`, `"\u263a"`, `"☺"`},
	{"\U0010ffff", `"\U0010ffff"`, `"\U0010ffff"`, `"\U0010ffff"`},
	{"\x04", `"\x04"`, `"\x04"`, `"\x04"`},
	// Some non-printable but graphic runes. Final column is double-quoted.
	{"!\u00a0!\u2000!\u3000!", `"!\u00a0!\u2000!\u3000!"`, `"!\u00a0!\u2000!\u3000!"`, "\"!\u00a0!\u2000!\u3000!\""},
	{"\x7f", `"\x7f"`, `"\x7f"`, `"\x7f"`},
}

func TestQuote(t *testT) {
	for _, tt := range quotetests {
		checks++
		if out := Quote(tt.in); out != tt.out {
			t.Errorf("Quote(%s) = %s, want %s", tt.in, out, tt.out)
		}
		checks++
		if out := AppendQuote([]byte("abc"), tt.in); string(out) != "abc"+tt.out {
			t.Errorf("AppendQuote(%q, %s) = %s, want %s", "abc", tt.in, out, "abc"+tt.out)
		}
	}
}

func TestQuoteToASCII(t *testT) {
	for _, tt := range quotetests {
		checks++
		if out := QuoteToASCII(tt.in); out != tt.ascii {
			t.Errorf("QuoteToASCII(%s) = %s, want %s", tt.in, out, tt.ascii)
		}
		checks++
		if out := AppendQuoteToASCII([]byte("abc"), tt.in); string(out) != "abc"+tt.ascii {
			t.Errorf("AppendQuoteToASCII(%q, %s) = %s, want %s", "abc", tt.in, out, "abc"+tt.ascii)
		}
	}
}

func TestQuoteToGraphic(t *testT) {
	for _, tt := range quotetests {
		checks++
		if out := QuoteToGraphic(tt.in); out != tt.graphic {
			t.Errorf("QuoteToGraphic(%s) = %s, want %s", tt.in, out, tt.graphic)
		}
		checks++
		if out := AppendQuoteToGraphic([]byte("abc"), tt.in); string(out) != "abc"+tt.graphic {
			t.Errorf("AppendQuoteToGraphic(%q, %s) = %s, want %s", "abc", tt.in, out, "abc"+tt.graphic)
		}
	}
}

var benchQuoteBuf []byte

var benchQuoteRuneBuf []byte

type quoteRuneTest struct {
	in      rune
	out     string
	ascii   string
	graphic string
}

var quoterunetests = []quoteRuneTest{
	{'a', `'a'`, `'a'`, `'a'`},
	{'\a', `'\a'`, `'\a'`, `'\a'`},
	{'\\', `'\\'`, `'\\'`, `'\\'`},
	{0xFF, `'ÿ'`, `'\u00ff'`, `'ÿ'`},
	{0x263a, `'☺'`, `'\u263a'`, `'☺'`},
	{0xdead, `'�'`, `'\ufffd'`, `'�'`},
	{0xfffd, `'�'`, `'\ufffd'`, `'�'`},
	{0x0010ffff, `'\U0010ffff'`, `'\U0010ffff'`, `'\U0010ffff'`},
	{0x0010ffff + 1, `'�'`, `'\ufffd'`, `'�'`},
	{0x04, `'\x04'`, `'\x04'`, `'\x04'`},
	// Some differences between graphic and printable. Note the last column is double-quoted.
	{'\u00a0', `'\u00a0'`, `'\u00a0'`, "'\u00a0'"},
	{'\u2000', `'\u2000'`, `'\u2000'`, "'\u2000'"},
	{'\u3000', `'\u3000'`, `'\u3000'`, "'\u3000'"},
}

func TestQuoteRune(t *testT) {
	for _, tt := range quoterunetests {
		checks++
		if out := QuoteRune(tt.in); out != tt.out {
			t.Errorf("QuoteRune(%U) = %s, want %s", tt.in, out, tt.out)
		}
		checks++
		if out := AppendQuoteRune([]byte("abc"), tt.in); string(out) != "abc"+tt.out {
			t.Errorf("AppendQuoteRune(%q, %U) = %s, want %s", "abc", tt.in, out, "abc"+tt.out)
		}
	}
}

func TestQuoteRuneToASCII(t *testT) {
	for _, tt := range quoterunetests {
		checks++
		if out := QuoteRuneToASCII(tt.in); out != tt.ascii {
			t.Errorf("QuoteRuneToASCII(%U) = %s, want %s", tt.in, out, tt.ascii)
		}
		checks++
		if out := AppendQuoteRuneToASCII([]byte("abc"), tt.in); string(out) != "abc"+tt.ascii {
			t.Errorf("AppendQuoteRuneToASCII(%q, %U) = %s, want %s", "abc", tt.in, out, "abc"+tt.ascii)
		}
	}
}

func TestQuoteRuneToGraphic(t *testT) {
	for _, tt := range quoterunetests {
		checks++
		if out := QuoteRuneToGraphic(tt.in); out != tt.graphic {
			t.Errorf("QuoteRuneToGraphic(%U) = %s, want %s", tt.in, out, tt.graphic)
		}
		checks++
		if out := AppendQuoteRuneToGraphic([]byte("abc"), tt.in); string(out) != "abc"+tt.graphic {
			t.Errorf("AppendQuoteRuneToGraphic(%q, %U) = %s, want %s", "abc", tt.in, out, "abc"+tt.graphic)
		}
	}
}

type canBackquoteTest struct {
	in  string
	out bool
}

var canbackquotetests = []canBackquoteTest{
	{"`", false},
	{string(rune(0)), false},
	{string(rune(1)), false},
	{string(rune(2)), false},
	{string(rune(3)), false},
	{string(rune(4)), false},
	{string(rune(5)), false},
	{string(rune(6)), false},
	{string(rune(7)), false},
	{string(rune(8)), false},
	{string(rune(9)), true}, // \t
	{string(rune(10)), false},
	{string(rune(11)), false},
	{string(rune(12)), false},
	{string(rune(13)), false},
	{string(rune(14)), false},
	{string(rune(15)), false},
	{string(rune(16)), false},
	{string(rune(17)), false},
	{string(rune(18)), false},
	{string(rune(19)), false},
	{string(rune(20)), false},
	{string(rune(21)), false},
	{string(rune(22)), false},
	{string(rune(23)), false},
	{string(rune(24)), false},
	{string(rune(25)), false},
	{string(rune(26)), false},
	{string(rune(27)), false},
	{string(rune(28)), false},
	{string(rune(29)), false},
	{string(rune(30)), false},
	{string(rune(31)), false},
	{string(rune(0x7F)), false},
	{`' !"#$%&'()*+,-./:;<=>?@[\]^_{|}~`, true},
	{`0123456789`, true},
	{`ABCDEFGHIJKLMNOPQRSTUVWXYZ`, true},
	{`abcdefghijklmnopqrstuvwxyz`, true},
	{`☺`, true},
	{"\x80", false},
	{"a\xe0\xa0z", false},
	{"\ufeffabc", false},
	{"a\ufeffz", false},
}

func TestCanBackquote(t *testT) {
	for _, tt := range canbackquotetests {
		checks++
		if out := CanBackquote(tt.in); out != tt.out {
			t.Errorf("CanBackquote(%q) = %v, want %v", tt.in, out, tt.out)
		}
	}
}

type unQuoteTest struct {
	in  string
	out string
}

var unquotetests = []unQuoteTest{
	{`""`, ""},
	{`"a"`, "a"},
	{`"abc"`, "abc"},
	{`"☺"`, "☺"},
	{`"hello world"`, "hello world"},
	{`"\xFF"`, "\xFF"},
	{`"\377"`, "\377"},
	{`"\u1234"`, "\u1234"},
	{`"\U00010111"`, "\U00010111"},
	{`"\U0001011111"`, "\U0001011111"},
	{`"\a\b\f\n\r\t\v\\\""`, "\a\b\f\n\r\t\v\\\""},
	{`"'"`, "'"},

	{`'a'`, "a"},
	{`'☹'`, "☹"},
	{`'\a'`, "\a"},
	{`'\x10'`, "\x10"},
	{`'\377'`, "\377"},
	{`'\u1234'`, "\u1234"},
	{`'\U00010111'`, "\U00010111"},
	{`'\t'`, "\t"},
	{`' '`, " "},
	{`'\''`, "'"},
	{`'"'`, "\""},

	{"``", ``},
	{"`a`", `a`},
	{"`abc`", `abc`},
	{"`☺`", `☺`},
	{"`hello world`", `hello world`},
	{"`\\xFF`", `\xFF`},
	{"`\\377`", `\377`},
	{"`\\`", `\`},
	{"`\n`", "\n"},
	{"`	`", `	`},
	{"` `", ` `},
	{"`a\rb`", "ab"},
}

var misquoted = []string{
	``,
	`"`,
	`"a`,
	`"'`,
	`b"`,
	`"\"`,
	`"\9"`,
	`"\19"`,
	`"\129"`,
	`'\'`,
	`'\9'`,
	`'\19'`,
	`'\129'`,
	`'ab'`,
	`"\x1!"`,
	`"\U12345678"`,
	`"\z"`,
	"`",
	"`xxx",
	"``x\r",
	"`\"",
	`"\'"`,
	`'\"'`,
	"\"\n\"",
	"\"\\n\n\"",
	"'\n'",
	`"\udead"`,
	`"\ud83d\ude4f"`,
}

func TestUnquote(t *testT) {
	for _, tt := range unquotetests {
		testUnquote(t, tt.in, tt.out, nil)
	}
	for _, tt := range quotetests {
		testUnquote(t, tt.out, tt.in, nil)
	}
	for _, s := range misquoted {
		testUnquote(t, s, "", ErrSyntax)
	}
}

// Issue 23685: invalid UTF-8 should not go through the fast path.
func TestUnquoteInvalidUTF8(t *testT) {
	tests := []struct {
		in string

		// one of:
		want    string
		wantErr error
	}{
		{in: `"foo"`, want: "foo"},
		{in: `"foo`, wantErr: ErrSyntax},
		{in: `"` + "\xc0" + `"`, want: "\xef\xbf\xbd"},
		{in: `"a` + "\xc0" + `"`, want: "a\xef\xbf\xbd"},
		{in: `"\t` + "\xc0" + `"`, want: "\t\xef\xbf\xbd"},
	}
	for _, tt := range tests {
		testUnquote(t, tt.in, tt.want, tt.wantErr)
	}
}

func testUnquote(t *testT, in, want string, wantErr error) {
	// Test Unquote.
	got, gotErr := Unquote(in)
	checks++
	if got != want || gotErr != wantErr {
		t.Errorf("Unquote(%q) = (%q, %v), want (%q, %v)", in, got, gotErr, want, wantErr)
	}

	// Test QuotedPrefix.
	// Adding an arbitrary suffix should not change the result of QuotedPrefix
	// assume that the suffix doesn't accidentally terminate a truncated input.
	checks++
	if gotErr == nil {
		want = in
	}
	suffix := "\n\r\\\"`'" // special characters for quoted strings
	checks++
	if len(in) > 0 {
		suffix = replaceAll(suffix, in[:1])
	}
	in += suffix
	got, gotErr = QuotedPrefix(in)
	checks++
	if gotErr == nil && wantErr != nil {
		_, wantErr = Unquote(got) // original input had trailing junk, reparse with only valid prefix
		want = got
	}
	checks++
	if got != want || gotErr != wantErr {
		t.Errorf("QuotedPrefix(%q) = (%q, %v), want (%q, %v)", in, got, gotErr, want, wantErr)
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
		{"TestIsPrint", TestIsPrint},
		{"TestIsGraphic", TestIsGraphic},
		{"TestQuote", TestQuote},
		{"TestQuoteToASCII", TestQuoteToASCII},
		{"TestQuoteToGraphic", TestQuoteToGraphic},
		{"TestQuoteRune", TestQuoteRune},
		{"TestQuoteRuneToASCII", TestQuoteRuneToASCII},
		{"TestQuoteRuneToGraphic", TestQuoteRuneToGraphic},
		{"TestCanBackquote", TestCanBackquote},
		{"TestUnquote", TestUnquote},
		{"TestUnquoteInvalidUTF8", TestUnquoteInvalidUTF8},
	}
	runTests(tests_)
}

// Output:
// ok TestIsPrint subtests: 0 checks: 1114112
// ok TestIsGraphic subtests: 0 checks: 1114112
// ok TestQuote subtests: 0 checks: 16
// ok TestQuoteToASCII subtests: 0 checks: 16
// ok TestQuoteToGraphic subtests: 0 checks: 16
// ok TestQuoteRune subtests: 0 checks: 26
// ok TestQuoteRuneToASCII subtests: 0 checks: 26
// ok TestQuoteRuneToGraphic subtests: 0 checks: 26
// ok TestCanBackquote subtests: 0 checks: 43
// ok TestUnquote subtests: 0 checks: 355
// ok TestUnquoteInvalidUTF8 subtests: 0 checks: 25
// tests: 11 failed: 0
