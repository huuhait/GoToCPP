// phase: 09
// Go 1.25's strconv/example_test.go (31 documented examples of the package, run in order) through a small harness; expected output from real Go.

package main

import (
	"fmt"
	"strconv"
)

func ExampleAppendBool() {
	b := []byte("bool:")
	b = strconv.AppendBool(b, true)
	fmt.Println(string(b))

}

func ExampleAppendFloat() {
	b32 := []byte("float32:")
	b32 = strconv.AppendFloat(b32, 3.1415926535, 'E', -1, 32)
	fmt.Println(string(b32))

	b64 := []byte("float64:")
	b64 = strconv.AppendFloat(b64, 3.1415926535, 'E', -1, 64)
	fmt.Println(string(b64))

}

func ExampleAppendInt() {
	b10 := []byte("int (base 10):")
	b10 = strconv.AppendInt(b10, -42, 10)
	fmt.Println(string(b10))

	b16 := []byte("int (base 16):")
	b16 = strconv.AppendInt(b16, -42, 16)
	fmt.Println(string(b16))

}

func ExampleAppendQuote() {
	b := []byte("quote:")
	b = strconv.AppendQuote(b, `"Fran & Freddie's Diner"`)
	fmt.Println(string(b))

}

func ExampleAppendQuoteRune() {
	b := []byte("rune:")
	b = strconv.AppendQuoteRune(b, '☺')
	fmt.Println(string(b))

}

func ExampleAppendQuoteRuneToASCII() {
	b := []byte("rune (ascii):")
	b = strconv.AppendQuoteRuneToASCII(b, '☺')
	fmt.Println(string(b))

}

func ExampleAppendQuoteToASCII() {
	b := []byte("quote (ascii):")
	b = strconv.AppendQuoteToASCII(b, `"Fran & Freddie's Diner"`)
	fmt.Println(string(b))

}

func ExampleAppendUint() {
	b10 := []byte("uint (base 10):")
	b10 = strconv.AppendUint(b10, 42, 10)
	fmt.Println(string(b10))

	b16 := []byte("uint (base 16):")
	b16 = strconv.AppendUint(b16, 42, 16)
	fmt.Println(string(b16))

}

func ExampleAtoi() {
	v := "10"
	if s, err := strconv.Atoi(v); err == nil {
		fmt.Printf("%T, %v", s, s)
	}

}

func ExampleCanBackquote() {
	fmt.Println(strconv.CanBackquote("Fran & Freddie's Diner ☺"))
	fmt.Println(strconv.CanBackquote("`can't backquote this`"))

}

func ExampleFormatBool() {
	v := true
	s := strconv.FormatBool(v)
	fmt.Printf("%T, %v\n", s, s)

}

func ExampleFormatFloat() {
	v := 3.1415926535

	s32 := strconv.FormatFloat(v, 'E', -1, 32)
	fmt.Printf("%T, %v\n", s32, s32)

	s64 := strconv.FormatFloat(v, 'E', -1, 64)
	fmt.Printf("%T, %v\n", s64, s64)

	// fmt.Println uses these arguments to print floats
	fmt64 := strconv.FormatFloat(v, 'g', -1, 64)
	fmt.Printf("%T, %v\n", fmt64, fmt64)

}

func ExampleFormatInt() {
	v := int64(-42)

	s10 := strconv.FormatInt(v, 10)
	fmt.Printf("%T, %v\n", s10, s10)

	s16 := strconv.FormatInt(v, 16)
	fmt.Printf("%T, %v\n", s16, s16)

}

func ExampleFormatUint() {
	v := uint64(42)

	s10 := strconv.FormatUint(v, 10)
	fmt.Printf("%T, %v\n", s10, s10)

	s16 := strconv.FormatUint(v, 16)
	fmt.Printf("%T, %v\n", s16, s16)

}

func ExampleIsGraphic() {
	shamrock := strconv.IsGraphic('☘')
	fmt.Println(shamrock)

	a := strconv.IsGraphic('a')
	fmt.Println(a)

	bel := strconv.IsGraphic('\007')
	fmt.Println(bel)

}

func ExampleIsPrint() {
	c := strconv.IsPrint('\u263a')
	fmt.Println(c)

	bel := strconv.IsPrint('\007')
	fmt.Println(bel)

}

func ExampleItoa() {
	i := 10
	s := strconv.Itoa(i)
	fmt.Printf("%T, %v\n", s, s)

}

func ExampleParseBool() {
	v := "true"
	if s, err := strconv.ParseBool(v); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}

}

func ExampleParseFloat() {
	v := "3.1415926535"
	if s, err := strconv.ParseFloat(v, 32); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}
	if s, err := strconv.ParseFloat(v, 64); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}
	if s, err := strconv.ParseFloat("NaN", 32); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}
	// ParseFloat is case insensitive
	if s, err := strconv.ParseFloat("nan", 32); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}
	if s, err := strconv.ParseFloat("inf", 32); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}
	if s, err := strconv.ParseFloat("+Inf", 32); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}
	if s, err := strconv.ParseFloat("-Inf", 32); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}
	if s, err := strconv.ParseFloat("-0", 32); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}
	if s, err := strconv.ParseFloat("+0", 32); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}

}

func ExampleParseInt() {
	v32 := "-354634382"
	if s, err := strconv.ParseInt(v32, 10, 32); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}
	if s, err := strconv.ParseInt(v32, 16, 32); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}

	v64 := "-3546343826724305832"
	if s, err := strconv.ParseInt(v64, 10, 64); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}
	if s, err := strconv.ParseInt(v64, 16, 64); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}

}

func ExampleParseUint() {
	v := "42"
	if s, err := strconv.ParseUint(v, 10, 32); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}
	if s, err := strconv.ParseUint(v, 10, 64); err == nil {
		fmt.Printf("%T, %v\n", s, s)
	}

}

func ExampleQuote() {
	// This string literal contains a tab character.
	s := strconv.Quote(`"Fran & Freddie's Diner	☺"`)
	fmt.Println(s)

}

func ExampleQuoteRune() {
	s := strconv.QuoteRune('☺')
	fmt.Println(s)

}

func ExampleQuoteRuneToASCII() {
	s := strconv.QuoteRuneToASCII('☺')
	fmt.Println(s)

}

func ExampleQuoteRuneToGraphic() {
	s := strconv.QuoteRuneToGraphic('☺')
	fmt.Println(s)

	s = strconv.QuoteRuneToGraphic('\u263a')
	fmt.Println(s)

	s = strconv.QuoteRuneToGraphic('\u000a')
	fmt.Println(s)

	s = strconv.QuoteRuneToGraphic('	') // tab character
	fmt.Println(s)

}

func ExampleQuoteToASCII() {
	// This string literal contains a tab character.
	s := strconv.QuoteToASCII(`"Fran & Freddie's Diner	☺"`)
	fmt.Println(s)

}

func ExampleQuoteToGraphic() {
	s := strconv.QuoteToGraphic("☺")
	fmt.Println(s)

	// This string literal contains a tab character.
	s = strconv.QuoteToGraphic("This is a \u263a	\u000a")
	fmt.Println(s)

	s = strconv.QuoteToGraphic(`" This is a ☺ \n "`)
	fmt.Println(s)

}

func ExampleQuotedPrefix() {
	s, err := strconv.QuotedPrefix("not a quoted string")
	fmt.Printf("%q, %v\n", s, err)
	s, err = strconv.QuotedPrefix("\"double-quoted string\" with trailing text")
	fmt.Printf("%q, %v\n", s, err)
	s, err = strconv.QuotedPrefix("`or backquoted` with more trailing text")
	fmt.Printf("%q, %v\n", s, err)
	s, err = strconv.QuotedPrefix("'\u263a' is also okay")
	fmt.Printf("%q, %v\n", s, err)

}

func ExampleUnquote() {
	s, err := strconv.Unquote("You can't unquote a string without quotes")
	fmt.Printf("%q, %v\n", s, err)
	s, err = strconv.Unquote("\"The string must be either double-quoted\"")
	fmt.Printf("%q, %v\n", s, err)
	s, err = strconv.Unquote("`or backquoted.`")
	fmt.Printf("%q, %v\n", s, err)
	s, err = strconv.Unquote("'\u263a'") // single character only allowed in single quotes
	fmt.Printf("%q, %v\n", s, err)
	s, err = strconv.Unquote("'\u2639\u2639'")
	fmt.Printf("%q, %v\n", s, err)

}

func ExampleUnquoteChar() {
	v, mb, t, err := strconv.UnquoteChar(`\"Fran & Freddie's Diner\"`, '"')
	if err != nil {
		panic(err)
	}

	fmt.Println("value:", string(v))
	fmt.Println("multibyte:", mb)
	fmt.Println("tail:", t)

}

func ExampleNumError() {
	str := "Not a number"
	if _, err := strconv.ParseFloat(str, 64); err != nil {
		e := err.(*strconv.NumError)
		fmt.Println("Func:", e.Func)
		fmt.Println("Num:", e.Num)
		fmt.Println("Err:", e.Err)
		fmt.Println(err)
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
	fmt.Println("== ExampleAppendBool")
	ExampleAppendBool()
	fmt.Println("== ExampleAppendFloat")
	ExampleAppendFloat()
	fmt.Println("== ExampleAppendInt")
	ExampleAppendInt()
	fmt.Println("== ExampleAppendQuote")
	ExampleAppendQuote()
	fmt.Println("== ExampleAppendQuoteRune")
	ExampleAppendQuoteRune()
	fmt.Println("== ExampleAppendQuoteRuneToASCII")
	ExampleAppendQuoteRuneToASCII()
	fmt.Println("== ExampleAppendQuoteToASCII")
	ExampleAppendQuoteToASCII()
	fmt.Println("== ExampleAppendUint")
	ExampleAppendUint()
	fmt.Println("== ExampleAtoi")
	ExampleAtoi()
	fmt.Println("== ExampleCanBackquote")
	ExampleCanBackquote()
	fmt.Println("== ExampleFormatBool")
	ExampleFormatBool()
	fmt.Println("== ExampleFormatFloat")
	ExampleFormatFloat()
	fmt.Println("== ExampleFormatInt")
	ExampleFormatInt()
	fmt.Println("== ExampleFormatUint")
	ExampleFormatUint()
	fmt.Println("== ExampleIsGraphic")
	ExampleIsGraphic()
	fmt.Println("== ExampleIsPrint")
	ExampleIsPrint()
	fmt.Println("== ExampleItoa")
	ExampleItoa()
	fmt.Println("== ExampleParseBool")
	ExampleParseBool()
	fmt.Println("== ExampleParseFloat")
	ExampleParseFloat()
	fmt.Println("== ExampleParseInt")
	ExampleParseInt()
	fmt.Println("== ExampleParseUint")
	ExampleParseUint()
	fmt.Println("== ExampleQuote")
	ExampleQuote()
	fmt.Println("== ExampleQuoteRune")
	ExampleQuoteRune()
	fmt.Println("== ExampleQuoteRuneToASCII")
	ExampleQuoteRuneToASCII()
	fmt.Println("== ExampleQuoteRuneToGraphic")
	ExampleQuoteRuneToGraphic()
	fmt.Println("== ExampleQuoteToASCII")
	ExampleQuoteToASCII()
	fmt.Println("== ExampleQuoteToGraphic")
	ExampleQuoteToGraphic()
	fmt.Println("== ExampleQuotedPrefix")
	ExampleQuotedPrefix()
	fmt.Println("== ExampleUnquote")
	ExampleUnquote()
	fmt.Println("== ExampleUnquoteChar")
	ExampleUnquoteChar()
	fmt.Println("== ExampleNumError")
	ExampleNumError()
}

// Output:
// == ExampleAppendBool
// bool:true
// == ExampleAppendFloat
// float32:3.1415927E+00
// float64:3.1415926535E+00
// == ExampleAppendInt
// int (base 10):-42
// int (base 16):-2a
// == ExampleAppendQuote
// quote:"\"Fran & Freddie's Diner\""
// == ExampleAppendQuoteRune
// rune:'☺'
// == ExampleAppendQuoteRuneToASCII
// rune (ascii):'\u263a'
// == ExampleAppendQuoteToASCII
// quote (ascii):"\"Fran & Freddie's Diner\""
// == ExampleAppendUint
// uint (base 10):42
// uint (base 16):2a
// == ExampleAtoi
// int, 10== ExampleCanBackquote
// true
// false
// == ExampleFormatBool
// string, true
// == ExampleFormatFloat
// string, 3.1415927E+00
// string, 3.1415926535E+00
// string, 3.1415926535
// == ExampleFormatInt
// string, -42
// string, -2a
// == ExampleFormatUint
// string, 42
// string, 2a
// == ExampleIsGraphic
// true
// true
// false
// == ExampleIsPrint
// true
// false
// == ExampleItoa
// string, 10
// == ExampleParseBool
// bool, true
// == ExampleParseFloat
// float64, 3.1415927410125732
// float64, 3.1415926535
// float64, NaN
// float64, NaN
// float64, +Inf
// float64, +Inf
// float64, -Inf
// float64, -0
// float64, 0
// == ExampleParseInt
// int64, -354634382
// int64, -3546343826724305832
// == ExampleParseUint
// uint64, 42
// uint64, 42
// == ExampleQuote
// "\"Fran & Freddie's Diner\t☺\""
// == ExampleQuoteRune
// '☺'
// == ExampleQuoteRuneToASCII
// '\u263a'
// == ExampleQuoteRuneToGraphic
// '☺'
// '☺'
// '\n'
// '\t'
// == ExampleQuoteToASCII
// "\"Fran & Freddie's Diner\t\u263a\""
// == ExampleQuoteToGraphic
// "☺"
// "This is a ☺\t\n"
// "\" This is a ☺ \\n \""
// == ExampleQuotedPrefix
// "", invalid syntax
// "\"double-quoted string\"", <nil>
// "`or backquoted`", <nil>
// "'☺'", <nil>
// == ExampleUnquote
// "", invalid syntax
// "The string must be either double-quoted", <nil>
// "or backquoted.", <nil>
// "☺", <nil>
// "", invalid syntax
// == ExampleUnquoteChar
// value: "
// multibyte: false
// tail: Fran & Freddie's Diner\"
// == ExampleNumError
// Func: ParseFloat
// Num: Not a number
// Err: invalid syntax
// strconv.ParseFloat: parsing "Not a number": invalid syntax
