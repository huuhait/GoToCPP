// phase: 09
// Go 1.25's strings/example_test.go (the documented examples of Builder, Reader, Replacer and the functions: Compare, Contains*, Count, Cut*, EqualFold, Fields*, HasPrefix, Index*, Join, Map, Repeat, Replace*, Split*, Title, To*, Trim*; the iterator examples are not included) run in order; expected output from real Go.

package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"unicode"
	"unsafe"
)


func ExampleClone() {
	s := "abc"
	clone := strings.Clone(s)
	fmt.Println(s == clone)
	fmt.Println(unsafe.StringData(s) == unsafe.StringData(clone))

}

func ExampleBuilder() {
	var b strings.Builder
	for i := 3; i >= 1; i-- {
		fmt.Fprintf(&b, "%d...", i)
	}
	b.WriteString("ignition")
	fmt.Println(b.String())

}

func ExampleCompare() {
	fmt.Println(strings.Compare("a", "b"))
	fmt.Println(strings.Compare("a", "a"))
	fmt.Println(strings.Compare("b", "a"))

}

func ExampleContains() {
	fmt.Println(strings.Contains("seafood", "foo"))
	fmt.Println(strings.Contains("seafood", "bar"))
	fmt.Println(strings.Contains("seafood", ""))
	fmt.Println(strings.Contains("", ""))

}

func ExampleContainsAny() {
	fmt.Println(strings.ContainsAny("team", "i"))
	fmt.Println(strings.ContainsAny("fail", "ui"))
	fmt.Println(strings.ContainsAny("ure", "ui"))
	fmt.Println(strings.ContainsAny("failure", "ui"))
	fmt.Println(strings.ContainsAny("foo", ""))
	fmt.Println(strings.ContainsAny("", ""))

}

func ExampleContainsRune() {
	// Finds whether a string contains a particular Unicode code point.
	// The code point for the lowercase letter "a", for example, is 97.
	fmt.Println(strings.ContainsRune("aardvark", 97))
	fmt.Println(strings.ContainsRune("timeout", 97))

}

func ExampleContainsFunc() {
	f := func(r rune) bool {
		return r == 'a' || r == 'e' || r == 'i' || r == 'o' || r == 'u'
	}
	fmt.Println(strings.ContainsFunc("hello", f))
	fmt.Println(strings.ContainsFunc("rhythms", f))

}

func ExampleCount() {
	fmt.Println(strings.Count("cheese", "e"))
	fmt.Println(strings.Count("five", "")) // before & after each rune

}

func ExampleCut() {
	show := func(s, sep string) {
		before, after, found := strings.Cut(s, sep)
		fmt.Printf("Cut(%q, %q) = %q, %q, %v\n", s, sep, before, after, found)
	}
	show("Gopher", "Go")
	show("Gopher", "ph")
	show("Gopher", "er")
	show("Gopher", "Badger")

}

func ExampleCutPrefix() {
	show := func(s, prefix string) {
		after, found := strings.CutPrefix(s, prefix)
		fmt.Printf("CutPrefix(%q, %q) = %q, %v\n", s, prefix, after, found)
	}
	show("Gopher", "Go")
	show("Gopher", "ph")

}

func ExampleCutSuffix() {
	show := func(s, suffix string) {
		before, found := strings.CutSuffix(s, suffix)
		fmt.Printf("CutSuffix(%q, %q) = %q, %v\n", s, suffix, before, found)
	}
	show("Gopher", "Go")
	show("Gopher", "er")

}

func ExampleEqualFold() {
	fmt.Println(strings.EqualFold("Go", "go"))
	fmt.Println(strings.EqualFold("AB", "ab")) // true because comparison uses simple case-folding
	fmt.Println(strings.EqualFold("ß", "ss"))  // false because comparison does not use full case-folding

}

func ExampleFields() {
	fmt.Printf("Fields are: %q", strings.Fields("  foo bar  baz   "))

}

func ExampleFieldsFunc() {
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	fmt.Printf("Fields are: %q", strings.FieldsFunc("  foo1;bar2,baz3...", f))

}

func ExampleHasPrefix() {
	fmt.Println(strings.HasPrefix("Gopher", "Go"))
	fmt.Println(strings.HasPrefix("Gopher", "C"))
	fmt.Println(strings.HasPrefix("Gopher", ""))

}

func ExampleHasSuffix() {
	fmt.Println(strings.HasSuffix("Amigo", "go"))
	fmt.Println(strings.HasSuffix("Amigo", "O"))
	fmt.Println(strings.HasSuffix("Amigo", "Ami"))
	fmt.Println(strings.HasSuffix("Amigo", ""))

}

func ExampleIndex() {
	fmt.Println(strings.Index("chicken", "ken"))
	fmt.Println(strings.Index("chicken", "dmr"))

}

func ExampleIndexFunc() {
	f := func(c rune) bool {
		return unicode.Is(unicode.Han, c)
	}
	fmt.Println(strings.IndexFunc("Hello, 世界", f))
	fmt.Println(strings.IndexFunc("Hello, world", f))

}

func ExampleIndexAny() {
	fmt.Println(strings.IndexAny("chicken", "aeiouy"))
	fmt.Println(strings.IndexAny("crwth", "aeiouy"))

}

func ExampleIndexByte() {
	fmt.Println(strings.IndexByte("golang", 'g'))
	fmt.Println(strings.IndexByte("gophers", 'h'))
	fmt.Println(strings.IndexByte("golang", 'x'))

}
func ExampleIndexRune() {
	fmt.Println(strings.IndexRune("chicken", 'k'))
	fmt.Println(strings.IndexRune("chicken", 'd'))

}

func ExampleLastIndex() {
	fmt.Println(strings.Index("go gopher", "go"))
	fmt.Println(strings.LastIndex("go gopher", "go"))
	fmt.Println(strings.LastIndex("go gopher", "rodent"))

}

func ExampleLastIndexAny() {
	fmt.Println(strings.LastIndexAny("go gopher", "go"))
	fmt.Println(strings.LastIndexAny("go gopher", "rodent"))
	fmt.Println(strings.LastIndexAny("go gopher", "fail"))

}

func ExampleLastIndexByte() {
	fmt.Println(strings.LastIndexByte("Hello, world", 'l'))
	fmt.Println(strings.LastIndexByte("Hello, world", 'o'))
	fmt.Println(strings.LastIndexByte("Hello, world", 'x'))

}

func ExampleLastIndexFunc() {
	fmt.Println(strings.LastIndexFunc("go 123", unicode.IsNumber))
	fmt.Println(strings.LastIndexFunc("123 go", unicode.IsNumber))
	fmt.Println(strings.LastIndexFunc("go", unicode.IsNumber))

}

func ExampleJoin() {
	s := []string{"foo", "bar", "baz"}
	fmt.Println(strings.Join(s, ", "))

}

func ExampleRepeat() {
	fmt.Println("ba" + strings.Repeat("na", 2))

}

func ExampleReplace() {
	fmt.Println(strings.Replace("oink oink oink", "k", "ky", 2))
	fmt.Println(strings.Replace("oink oink oink", "oink", "moo", -1))

}

func ExampleReplaceAll() {
	fmt.Println(strings.ReplaceAll("oink oink oink", "oink", "moo"))

}

func ExampleSplit() {
	fmt.Printf("%q\n", strings.Split("a,b,c", ","))
	fmt.Printf("%q\n", strings.Split("a man a plan a canal panama", "a "))
	fmt.Printf("%q\n", strings.Split(" xyz ", ""))
	fmt.Printf("%q\n", strings.Split("", "Bernardo O'Higgins"))

}

func ExampleSplitN() {
	fmt.Printf("%q\n", strings.SplitN("a,b,c", ",", 2))
	z := strings.SplitN("a,b,c", ",", 0)
	fmt.Printf("%q (nil = %v)\n", z, z == nil)

}

func ExampleSplitAfter() {
	fmt.Printf("%q\n", strings.SplitAfter("a,b,c", ","))

}

func ExampleSplitAfterN() {
	fmt.Printf("%q\n", strings.SplitAfterN("a,b,c", ",", 2))

}

func ExampleTitle() {
	// Compare this example to the ToTitle example.
	fmt.Println(strings.Title("her royal highness"))
	fmt.Println(strings.Title("loud noises"))
	fmt.Println(strings.Title("брат"))

}

func ExampleToTitle() {
	// Compare this example to the Title example.
	fmt.Println(strings.ToTitle("her royal highness"))
	fmt.Println(strings.ToTitle("loud noises"))
	fmt.Println(strings.ToTitle("брат"))

}

func ExampleToTitleSpecial() {
	fmt.Println(strings.ToTitleSpecial(unicode.TurkishCase, "dünyanın ilk borsa yapısı Aizonai kabul edilir"))

}

func ExampleMap() {
	rot13 := func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return 'A' + (r-'A'+13)%26
		case r >= 'a' && r <= 'z':
			return 'a' + (r-'a'+13)%26
		}
		return r
	}
	fmt.Println(strings.Map(rot13, "'Twas brillig and the slithy gopher..."))

}

func ExampleNewReplacer() {
	r := strings.NewReplacer("<", "&lt;", ">", "&gt;")
	fmt.Println(r.Replace("This is <b>HTML</b>!"))

}

func ExampleToUpper() {
	fmt.Println(strings.ToUpper("Gopher"))

}

func ExampleToUpperSpecial() {
	fmt.Println(strings.ToUpperSpecial(unicode.TurkishCase, "örnek iş"))

}

func ExampleToLower() {
	fmt.Println(strings.ToLower("Gopher"))

}

func ExampleToLowerSpecial() {
	fmt.Println(strings.ToLowerSpecial(unicode.TurkishCase, "Örnek İş"))

}

func ExampleTrim() {
	fmt.Print(strings.Trim("¡¡¡Hello, Gophers!!!", "!¡"))

}

func ExampleTrimSpace() {
	fmt.Println(strings.TrimSpace(" \t\n Hello, Gophers \n\t\r\n"))

}

func ExampleTrimPrefix() {
	var s = "¡¡¡Hello, Gophers!!!"
	s = strings.TrimPrefix(s, "¡¡¡Hello, ")
	s = strings.TrimPrefix(s, "¡¡¡Howdy, ")
	fmt.Print(s)

}

func ExampleTrimSuffix() {
	var s = "¡¡¡Hello, Gophers!!!"
	s = strings.TrimSuffix(s, ", Gophers!!!")
	s = strings.TrimSuffix(s, ", Marmots!!!")
	fmt.Print(s)

}

func ExampleTrimFunc() {
	fmt.Print(strings.TrimFunc("¡¡¡Hello, Gophers!!!", func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}))

}

func ExampleTrimLeft() {
	fmt.Print(strings.TrimLeft("¡¡¡Hello, Gophers!!!", "!¡"))

}

func ExampleTrimLeftFunc() {
	fmt.Print(strings.TrimLeftFunc("¡¡¡Hello, Gophers!!!", func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}))

}

func ExampleTrimRight() {
	fmt.Print(strings.TrimRight("¡¡¡Hello, Gophers!!!", "!¡"))

}

func ExampleTrimRightFunc() {
	fmt.Print(strings.TrimRightFunc("¡¡¡Hello, Gophers!!!", func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}))

}

func ExampleToValidUTF8() {
	fmt.Printf("%s\n", strings.ToValidUTF8("abc", "\uFFFD"))
	fmt.Printf("%s\n", strings.ToValidUTF8("a\xffb\xC0\xAFc\xff", ""))
	fmt.Printf("%s\n", strings.ToValidUTF8("\xed\xa0\x80", "abc"))

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
	fmt.Println("== ExampleClone")
	ExampleClone()
	fmt.Println("== ExampleBuilder")
	ExampleBuilder()
	fmt.Println("== ExampleCompare")
	ExampleCompare()
	fmt.Println("== ExampleContains")
	ExampleContains()
	fmt.Println("== ExampleContainsAny")
	ExampleContainsAny()
	fmt.Println("== ExampleContainsRune")
	ExampleContainsRune()
	fmt.Println("== ExampleContainsFunc")
	ExampleContainsFunc()
	fmt.Println("== ExampleCount")
	ExampleCount()
	fmt.Println("== ExampleCut")
	ExampleCut()
	fmt.Println("== ExampleCutPrefix")
	ExampleCutPrefix()
	fmt.Println("== ExampleCutSuffix")
	ExampleCutSuffix()
	fmt.Println("== ExampleEqualFold")
	ExampleEqualFold()
	fmt.Println("== ExampleFields")
	ExampleFields()
	fmt.Println("== ExampleFieldsFunc")
	ExampleFieldsFunc()
	fmt.Println("== ExampleHasPrefix")
	ExampleHasPrefix()
	fmt.Println("== ExampleHasSuffix")
	ExampleHasSuffix()
	fmt.Println("== ExampleIndex")
	ExampleIndex()
	fmt.Println("== ExampleIndexFunc")
	ExampleIndexFunc()
	fmt.Println("== ExampleIndexAny")
	ExampleIndexAny()
	fmt.Println("== ExampleIndexByte")
	ExampleIndexByte()
	fmt.Println("== ExampleIndexRune")
	ExampleIndexRune()
	fmt.Println("== ExampleLastIndex")
	ExampleLastIndex()
	fmt.Println("== ExampleLastIndexAny")
	ExampleLastIndexAny()
	fmt.Println("== ExampleLastIndexByte")
	ExampleLastIndexByte()
	fmt.Println("== ExampleLastIndexFunc")
	ExampleLastIndexFunc()
	fmt.Println("== ExampleJoin")
	ExampleJoin()
	fmt.Println("== ExampleRepeat")
	ExampleRepeat()
	fmt.Println("== ExampleReplace")
	ExampleReplace()
	fmt.Println("== ExampleReplaceAll")
	ExampleReplaceAll()
	fmt.Println("== ExampleSplit")
	ExampleSplit()
	fmt.Println("== ExampleSplitN")
	ExampleSplitN()
	fmt.Println("== ExampleSplitAfter")
	ExampleSplitAfter()
	fmt.Println("== ExampleSplitAfterN")
	ExampleSplitAfterN()
	fmt.Println("== ExampleTitle")
	ExampleTitle()
	fmt.Println("== ExampleToTitle")
	ExampleToTitle()
	fmt.Println("== ExampleToTitleSpecial")
	ExampleToTitleSpecial()
	fmt.Println("== ExampleMap")
	ExampleMap()
	fmt.Println("== ExampleNewReplacer")
	ExampleNewReplacer()
	fmt.Println("== ExampleToUpper")
	ExampleToUpper()
	fmt.Println("== ExampleToUpperSpecial")
	ExampleToUpperSpecial()
	fmt.Println("== ExampleToLower")
	ExampleToLower()
	fmt.Println("== ExampleToLowerSpecial")
	ExampleToLowerSpecial()
	fmt.Println("== ExampleTrim")
	ExampleTrim()
	fmt.Println("== ExampleTrimSpace")
	ExampleTrimSpace()
	fmt.Println("== ExampleTrimPrefix")
	ExampleTrimPrefix()
	fmt.Println("== ExampleTrimSuffix")
	ExampleTrimSuffix()
	fmt.Println("== ExampleTrimFunc")
	ExampleTrimFunc()
	fmt.Println("== ExampleTrimLeft")
	ExampleTrimLeft()
	fmt.Println("== ExampleTrimLeftFunc")
	ExampleTrimLeftFunc()
	fmt.Println("== ExampleTrimRight")
	ExampleTrimRight()
	fmt.Println("== ExampleTrimRightFunc")
	ExampleTrimRightFunc()
	fmt.Println("== ExampleToValidUTF8")
	ExampleToValidUTF8()
}

// Output:
// == ExampleClone
// true
// false
// == ExampleBuilder
// 3...2...1...ignition
// == ExampleCompare
// -1
// 0
// 1
// == ExampleContains
// true
// false
// true
// true
// == ExampleContainsAny
// false
// true
// true
// true
// false
// false
// == ExampleContainsRune
// true
// false
// == ExampleContainsFunc
// true
// false
// == ExampleCount
// 3
// 5
// == ExampleCut
// Cut("Gopher", "Go") = "", "pher", true
// Cut("Gopher", "ph") = "Go", "er", true
// Cut("Gopher", "er") = "Goph", "", true
// Cut("Gopher", "Badger") = "Gopher", "", false
// == ExampleCutPrefix
// CutPrefix("Gopher", "Go") = "pher", true
// CutPrefix("Gopher", "ph") = "Gopher", false
// == ExampleCutSuffix
// CutSuffix("Gopher", "Go") = "Gopher", false
// CutSuffix("Gopher", "er") = "Goph", true
// == ExampleEqualFold
// true
// true
// false
// == ExampleFields
// Fields are: ["foo" "bar" "baz"]== ExampleFieldsFunc
// Fields are: ["foo1" "bar2" "baz3"]== ExampleHasPrefix
// true
// false
// true
// == ExampleHasSuffix
// true
// false
// false
// true
// == ExampleIndex
// 4
// -1
// == ExampleIndexFunc
// 7
// -1
// == ExampleIndexAny
// 2
// -1
// == ExampleIndexByte
// 0
// 3
// -1
// == ExampleIndexRune
// 4
// -1
// == ExampleLastIndex
// 0
// 3
// -1
// == ExampleLastIndexAny
// 4
// 8
// -1
// == ExampleLastIndexByte
// 10
// 8
// -1
// == ExampleLastIndexFunc
// 5
// 2
// -1
// == ExampleJoin
// foo, bar, baz
// == ExampleRepeat
// banana
// == ExampleReplace
// oinky oinky oink
// moo moo moo
// == ExampleReplaceAll
// moo moo moo
// == ExampleSplit
// ["a" "b" "c"]
// ["" "man " "plan " "canal panama"]
// [" " "x" "y" "z" " "]
// [""]
// == ExampleSplitN
// ["a" "b,c"]
// [] (nil = true)
// == ExampleSplitAfter
// ["a," "b," "c"]
// == ExampleSplitAfterN
// ["a," "b,c"]
// == ExampleTitle
// Her Royal Highness
// Loud Noises
// Брат
// == ExampleToTitle
// HER ROYAL HIGHNESS
// LOUD NOISES
// БРАТ
// == ExampleToTitleSpecial
// DÜNYANIN İLK BORSA YAPISI AİZONAİ KABUL EDİLİR
// == ExampleMap
// 'Gjnf oevyyvt naq gur fyvgul tbcure...
// == ExampleNewReplacer
// This is &lt;b&gt;HTML&lt;/b&gt;!
// == ExampleToUpper
// GOPHER
// == ExampleToUpperSpecial
// ÖRNEK İŞ
// == ExampleToLower
// gopher
// == ExampleToLowerSpecial
// örnek iş
// == ExampleTrim
// Hello, Gophers== ExampleTrimSpace
// Hello, Gophers
// == ExampleTrimPrefix
// Gophers!!!== ExampleTrimSuffix
// ¡¡¡Hello== ExampleTrimFunc
// Hello, Gophers== ExampleTrimLeft
// Hello, Gophers!!!== ExampleTrimLeftFunc
// Hello, Gophers!!!== ExampleTrimRight
// ¡¡¡Hello, Gophers== ExampleTrimRightFunc
// ¡¡¡Hello, Gophers== ExampleToValidUTF8
// abc
// abc
// abc
