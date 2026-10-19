// phase: 09
// Go 1.25's bytes/example_test.go (the documented examples of Buffer, Reader and the functions: Compare, Contains*, Count, Cut*, Equal, Fields*, HasPrefix, Index*, Join, Map, Repeat, Replace*, Split*, Title, To*, Trim*; the iterator examples and the base64 and slices ones are not included) run in order; expected output from real Go.

package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"unicode"
)


func ExampleBuffer() {
	var b bytes.Buffer // A Buffer needs no initialization.
	b.Write([]byte("Hello "))
	fmt.Fprintf(&b, "world!")
	b.WriteTo(os.Stdout)

}

func ExampleBuffer_Bytes() {
	buf := bytes.Buffer{}
	buf.Write([]byte{'h', 'e', 'l', 'l', 'o', ' ', 'w', 'o', 'r', 'l', 'd'})
	os.Stdout.Write(buf.Bytes())

}

func ExampleBuffer_AvailableBuffer() {
	var buf bytes.Buffer
	for i := 0; i < 4; i++ {
		b := buf.AvailableBuffer()
		b = strconv.AppendInt(b, int64(i), 10)
		b = append(b, ' ')
		buf.Write(b)
	}
	os.Stdout.Write(buf.Bytes())

}

func ExampleBuffer_Cap() {
	buf1 := bytes.NewBuffer(make([]byte, 10))
	buf2 := bytes.NewBuffer(make([]byte, 0, 10))
	fmt.Println(buf1.Cap())
	fmt.Println(buf2.Cap())

}

func ExampleBuffer_Grow() {
	var b bytes.Buffer
	b.Grow(64)
	bb := b.Bytes()
	b.Write([]byte("64 bytes or fewer"))
	fmt.Printf("%q", bb[:b.Len()])

}

func ExampleBuffer_Len() {
	var b bytes.Buffer
	b.Grow(64)
	b.Write([]byte("abcde"))
	fmt.Printf("%d", b.Len())

}

func ExampleBuffer_Next() {
	var b bytes.Buffer
	b.Grow(64)
	b.Write([]byte("abcde"))
	fmt.Printf("%s\n", b.Next(2))
	fmt.Printf("%s\n", b.Next(2))
	fmt.Printf("%s", b.Next(2))

}

func ExampleBuffer_Read() {
	var b bytes.Buffer
	b.Grow(64)
	b.Write([]byte("abcde"))
	rdbuf := make([]byte, 1)
	n, err := b.Read(rdbuf)
	if err != nil {
		panic(err)
	}
	fmt.Println(n)
	fmt.Println(b.String())
	fmt.Println(string(rdbuf))

}

func ExampleBuffer_ReadByte() {
	var b bytes.Buffer
	b.Grow(64)
	b.Write([]byte("abcde"))
	c, err := b.ReadByte()
	if err != nil {
		panic(err)
	}
	fmt.Println(c)
	fmt.Println(b.String())

}

func ExampleClone() {
	b := []byte("abc")
	clone := bytes.Clone(b)
	fmt.Printf("%s\n", clone)
	clone[0] = 'd'
	fmt.Printf("%s\n", b)
	fmt.Printf("%s\n", clone)

}

func ExampleCompare() {
	// Interpret Compare's result by comparing it to zero.
	var a, b []byte
	if bytes.Compare(a, b) < 0 {
		// a less b
	}
	if bytes.Compare(a, b) <= 0 {
		// a less or equal b
	}
	if bytes.Compare(a, b) > 0 {
		// a greater b
	}
	if bytes.Compare(a, b) >= 0 {
		// a greater or equal b
	}

	// Prefer Equal to Compare for equality comparisons.
	if bytes.Equal(a, b) {
		// a equal b
	}
	if !bytes.Equal(a, b) {
		// a not equal b
	}
}

func ExampleContains() {
	fmt.Println(bytes.Contains([]byte("seafood"), []byte("foo")))
	fmt.Println(bytes.Contains([]byte("seafood"), []byte("bar")))
	fmt.Println(bytes.Contains([]byte("seafood"), []byte("")))
	fmt.Println(bytes.Contains([]byte(""), []byte("")))

}

func ExampleContainsAny() {
	fmt.Println(bytes.ContainsAny([]byte("I like seafood."), "fÄo!"))
	fmt.Println(bytes.ContainsAny([]byte("I like seafood."), "去是伟大的."))
	fmt.Println(bytes.ContainsAny([]byte("I like seafood."), ""))
	fmt.Println(bytes.ContainsAny([]byte(""), ""))

}

func ExampleContainsRune() {
	fmt.Println(bytes.ContainsRune([]byte("I like seafood."), 'f'))
	fmt.Println(bytes.ContainsRune([]byte("I like seafood."), 'ö'))
	fmt.Println(bytes.ContainsRune([]byte("去是伟大的!"), '大'))
	fmt.Println(bytes.ContainsRune([]byte("去是伟大的!"), '!'))
	fmt.Println(bytes.ContainsRune([]byte(""), '@'))

}

func ExampleContainsFunc() {
	f := func(r rune) bool {
		return r >= 'a' && r <= 'z'
	}
	fmt.Println(bytes.ContainsFunc([]byte("HELLO"), f))
	fmt.Println(bytes.ContainsFunc([]byte("World"), f))

}

func ExampleCount() {
	fmt.Println(bytes.Count([]byte("cheese"), []byte("e")))
	fmt.Println(bytes.Count([]byte("five"), []byte(""))) // before & after each rune

}

func ExampleCut() {
	show := func(s, sep string) {
		before, after, found := bytes.Cut([]byte(s), []byte(sep))
		fmt.Printf("Cut(%q, %q) = %q, %q, %v\n", s, sep, before, after, found)
	}
	show("Gopher", "Go")
	show("Gopher", "ph")
	show("Gopher", "er")
	show("Gopher", "Badger")

}

func ExampleCutPrefix() {
	show := func(s, prefix string) {
		after, found := bytes.CutPrefix([]byte(s), []byte(prefix))
		fmt.Printf("CutPrefix(%q, %q) = %q, %v\n", s, prefix, after, found)
	}
	show("Gopher", "Go")
	show("Gopher", "ph")

}

func ExampleCutSuffix() {
	show := func(s, suffix string) {
		before, found := bytes.CutSuffix([]byte(s), []byte(suffix))
		fmt.Printf("CutSuffix(%q, %q) = %q, %v\n", s, suffix, before, found)
	}
	show("Gopher", "Go")
	show("Gopher", "er")

}

func ExampleEqual() {
	fmt.Println(bytes.Equal([]byte("Go"), []byte("Go")))
	fmt.Println(bytes.Equal([]byte("Go"), []byte("C++")))

}

func ExampleEqualFold() {
	fmt.Println(bytes.EqualFold([]byte("Go"), []byte("go")))

}

func ExampleFields() {
	fmt.Printf("Fields are: %q", bytes.Fields([]byte("  foo bar  baz   ")))

}

func ExampleFieldsFunc() {
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	fmt.Printf("Fields are: %q", bytes.FieldsFunc([]byte("  foo1;bar2,baz3..."), f))

}

func ExampleHasPrefix() {
	fmt.Println(bytes.HasPrefix([]byte("Gopher"), []byte("Go")))
	fmt.Println(bytes.HasPrefix([]byte("Gopher"), []byte("C")))
	fmt.Println(bytes.HasPrefix([]byte("Gopher"), []byte("")))

}

func ExampleHasSuffix() {
	fmt.Println(bytes.HasSuffix([]byte("Amigo"), []byte("go")))
	fmt.Println(bytes.HasSuffix([]byte("Amigo"), []byte("O")))
	fmt.Println(bytes.HasSuffix([]byte("Amigo"), []byte("Ami")))
	fmt.Println(bytes.HasSuffix([]byte("Amigo"), []byte("")))

}

func ExampleIndex() {
	fmt.Println(bytes.Index([]byte("chicken"), []byte("ken")))
	fmt.Println(bytes.Index([]byte("chicken"), []byte("dmr")))

}

func ExampleIndexByte() {
	fmt.Println(bytes.IndexByte([]byte("chicken"), byte('k')))
	fmt.Println(bytes.IndexByte([]byte("chicken"), byte('g')))

}

func ExampleIndexFunc() {
	f := func(c rune) bool {
		return unicode.Is(unicode.Han, c)
	}
	fmt.Println(bytes.IndexFunc([]byte("Hello, 世界"), f))
	fmt.Println(bytes.IndexFunc([]byte("Hello, world"), f))

}

func ExampleIndexAny() {
	fmt.Println(bytes.IndexAny([]byte("chicken"), "aeiouy"))
	fmt.Println(bytes.IndexAny([]byte("crwth"), "aeiouy"))

}

func ExampleIndexRune() {
	fmt.Println(bytes.IndexRune([]byte("chicken"), 'k'))
	fmt.Println(bytes.IndexRune([]byte("chicken"), 'd'))

}

func ExampleJoin() {
	s := [][]byte{[]byte("foo"), []byte("bar"), []byte("baz")}
	fmt.Printf("%s", bytes.Join(s, []byte(", ")))

}

func ExampleLastIndex() {
	fmt.Println(bytes.Index([]byte("go gopher"), []byte("go")))
	fmt.Println(bytes.LastIndex([]byte("go gopher"), []byte("go")))
	fmt.Println(bytes.LastIndex([]byte("go gopher"), []byte("rodent")))

}

func ExampleLastIndexAny() {
	fmt.Println(bytes.LastIndexAny([]byte("go gopher"), "MüQp"))
	fmt.Println(bytes.LastIndexAny([]byte("go 地鼠"), "地大"))
	fmt.Println(bytes.LastIndexAny([]byte("go gopher"), "z,!."))

}

func ExampleLastIndexByte() {
	fmt.Println(bytes.LastIndexByte([]byte("go gopher"), byte('g')))
	fmt.Println(bytes.LastIndexByte([]byte("go gopher"), byte('r')))
	fmt.Println(bytes.LastIndexByte([]byte("go gopher"), byte('z')))

}

func ExampleLastIndexFunc() {
	fmt.Println(bytes.LastIndexFunc([]byte("go gopher!"), unicode.IsLetter))
	fmt.Println(bytes.LastIndexFunc([]byte("go gopher!"), unicode.IsPunct))
	fmt.Println(bytes.LastIndexFunc([]byte("go gopher!"), unicode.IsNumber))

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
	fmt.Printf("%s\n", bytes.Map(rot13, []byte("'Twas brillig and the slithy gopher...")))

}

func ExampleReader_Len() {
	fmt.Println(bytes.NewReader([]byte("Hi!")).Len())
	fmt.Println(bytes.NewReader([]byte("こんにちは!")).Len())

}

func ExampleRepeat() {
	fmt.Printf("ba%s", bytes.Repeat([]byte("na"), 2))

}

func ExampleReplace() {
	fmt.Printf("%s\n", bytes.Replace([]byte("oink oink oink"), []byte("k"), []byte("ky"), 2))
	fmt.Printf("%s\n", bytes.Replace([]byte("oink oink oink"), []byte("oink"), []byte("moo"), -1))

}

func ExampleReplaceAll() {
	fmt.Printf("%s\n", bytes.ReplaceAll([]byte("oink oink oink"), []byte("oink"), []byte("moo")))

}

func ExampleRunes() {
	rs := bytes.Runes([]byte("go gopher"))
	for _, r := range rs {
		fmt.Printf("%#U\n", r)
	}

}

func ExampleSplit() {
	fmt.Printf("%q\n", bytes.Split([]byte("a,b,c"), []byte(",")))
	fmt.Printf("%q\n", bytes.Split([]byte("a man a plan a canal panama"), []byte("a ")))
	fmt.Printf("%q\n", bytes.Split([]byte(" xyz "), []byte("")))
	fmt.Printf("%q\n", bytes.Split([]byte(""), []byte("Bernardo O'Higgins")))

}

func ExampleSplitN() {
	fmt.Printf("%q\n", bytes.SplitN([]byte("a,b,c"), []byte(","), 2))
	z := bytes.SplitN([]byte("a,b,c"), []byte(","), 0)
	fmt.Printf("%q (nil = %v)\n", z, z == nil)

}

func ExampleSplitAfter() {
	fmt.Printf("%q\n", bytes.SplitAfter([]byte("a,b,c"), []byte(",")))

}

func ExampleSplitAfterN() {
	fmt.Printf("%q\n", bytes.SplitAfterN([]byte("a,b,c"), []byte(","), 2))

}

func ExampleTitle() {
	fmt.Printf("%s", bytes.Title([]byte("her royal highness")))

}

func ExampleToTitle() {
	fmt.Printf("%s\n", bytes.ToTitle([]byte("loud noises")))
	fmt.Printf("%s\n", bytes.ToTitle([]byte("брат")))

}

func ExampleToTitleSpecial() {
	str := []byte("ahoj vývojári golang")
	totitle := bytes.ToTitleSpecial(unicode.AzeriCase, str)
	fmt.Println("Original : " + string(str))
	fmt.Println("ToTitle : " + string(totitle))

}

func ExampleToValidUTF8() {
	fmt.Printf("%s\n", bytes.ToValidUTF8([]byte("abc"), []byte("\uFFFD")))
	fmt.Printf("%s\n", bytes.ToValidUTF8([]byte("a\xffb\xC0\xAFc\xff"), []byte("")))
	fmt.Printf("%s\n", bytes.ToValidUTF8([]byte("\xed\xa0\x80"), []byte("abc")))

}

func ExampleTrim() {
	fmt.Printf("[%q]", bytes.Trim([]byte(" !!! Achtung! Achtung! !!! "), "! "))

}

func ExampleTrimFunc() {
	fmt.Println(string(bytes.TrimFunc([]byte("go-gopher!"), unicode.IsLetter)))
	fmt.Println(string(bytes.TrimFunc([]byte("\"go-gopher!\""), unicode.IsLetter)))
	fmt.Println(string(bytes.TrimFunc([]byte("go-gopher!"), unicode.IsPunct)))
	fmt.Println(string(bytes.TrimFunc([]byte("1234go-gopher!567"), unicode.IsNumber)))

}

func ExampleTrimLeft() {
	fmt.Print(string(bytes.TrimLeft([]byte("453gopher8257"), "0123456789")))

}

func ExampleTrimLeftFunc() {
	fmt.Println(string(bytes.TrimLeftFunc([]byte("go-gopher"), unicode.IsLetter)))
	fmt.Println(string(bytes.TrimLeftFunc([]byte("go-gopher!"), unicode.IsPunct)))
	fmt.Println(string(bytes.TrimLeftFunc([]byte("1234go-gopher!567"), unicode.IsNumber)))

}

func ExampleTrimPrefix() {
	var b = []byte("Goodbye,, world!")
	b = bytes.TrimPrefix(b, []byte("Goodbye,"))
	b = bytes.TrimPrefix(b, []byte("See ya,"))
	fmt.Printf("Hello%s", b)

}

func ExampleTrimSpace() {
	fmt.Printf("%s", bytes.TrimSpace([]byte(" \t\n a lone gopher \n\t\r\n")))

}

func ExampleTrimSuffix() {
	var b = []byte("Hello, goodbye, etc!")
	b = bytes.TrimSuffix(b, []byte("goodbye, etc!"))
	b = bytes.TrimSuffix(b, []byte("gopher"))
	b = append(b, bytes.TrimSuffix([]byte("world!"), []byte("x!"))...)
	os.Stdout.Write(b)

}

func ExampleTrimRight() {
	fmt.Print(string(bytes.TrimRight([]byte("453gopher8257"), "0123456789")))

}

func ExampleTrimRightFunc() {
	fmt.Println(string(bytes.TrimRightFunc([]byte("go-gopher"), unicode.IsLetter)))
	fmt.Println(string(bytes.TrimRightFunc([]byte("go-gopher!"), unicode.IsPunct)))
	fmt.Println(string(bytes.TrimRightFunc([]byte("1234go-gopher!567"), unicode.IsNumber)))

}

func ExampleToLower() {
	fmt.Printf("%s", bytes.ToLower([]byte("Gopher")))

}

func ExampleToLowerSpecial() {
	str := []byte("AHOJ VÝVOJÁRİ GOLANG")
	totitle := bytes.ToLowerSpecial(unicode.AzeriCase, str)
	fmt.Println("Original : " + string(str))
	fmt.Println("ToLower : " + string(totitle))

}

func ExampleToUpper() {
	fmt.Printf("%s", bytes.ToUpper([]byte("Gopher")))

}

func ExampleToUpperSpecial() {
	str := []byte("ahoj vývojári golang")
	totitle := bytes.ToUpperSpecial(unicode.AzeriCase, str)
	fmt.Println("Original : " + string(str))
	fmt.Println("ToUpper : " + string(totitle))

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
	fmt.Println("== ExampleBuffer")
	ExampleBuffer()
	fmt.Println("== ExampleBuffer_Bytes")
	ExampleBuffer_Bytes()
	fmt.Println("== ExampleBuffer_AvailableBuffer")
	ExampleBuffer_AvailableBuffer()
	fmt.Println("== ExampleBuffer_Cap")
	ExampleBuffer_Cap()
	fmt.Println("== ExampleBuffer_Grow")
	ExampleBuffer_Grow()
	fmt.Println("== ExampleBuffer_Len")
	ExampleBuffer_Len()
	fmt.Println("== ExampleBuffer_Next")
	ExampleBuffer_Next()
	fmt.Println("== ExampleBuffer_Read")
	ExampleBuffer_Read()
	fmt.Println("== ExampleBuffer_ReadByte")
	ExampleBuffer_ReadByte()
	fmt.Println("== ExampleClone")
	ExampleClone()
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
	fmt.Println("== ExampleEqual")
	ExampleEqual()
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
	fmt.Println("== ExampleIndexByte")
	ExampleIndexByte()
	fmt.Println("== ExampleIndexFunc")
	ExampleIndexFunc()
	fmt.Println("== ExampleIndexAny")
	ExampleIndexAny()
	fmt.Println("== ExampleIndexRune")
	ExampleIndexRune()
	fmt.Println("== ExampleJoin")
	ExampleJoin()
	fmt.Println("== ExampleLastIndex")
	ExampleLastIndex()
	fmt.Println("== ExampleLastIndexAny")
	ExampleLastIndexAny()
	fmt.Println("== ExampleLastIndexByte")
	ExampleLastIndexByte()
	fmt.Println("== ExampleLastIndexFunc")
	ExampleLastIndexFunc()
	fmt.Println("== ExampleMap")
	ExampleMap()
	fmt.Println("== ExampleReader_Len")
	ExampleReader_Len()
	fmt.Println("== ExampleRepeat")
	ExampleRepeat()
	fmt.Println("== ExampleReplace")
	ExampleReplace()
	fmt.Println("== ExampleReplaceAll")
	ExampleReplaceAll()
	fmt.Println("== ExampleRunes")
	ExampleRunes()
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
	fmt.Println("== ExampleToValidUTF8")
	ExampleToValidUTF8()
	fmt.Println("== ExampleTrim")
	ExampleTrim()
	fmt.Println("== ExampleTrimFunc")
	ExampleTrimFunc()
	fmt.Println("== ExampleTrimLeft")
	ExampleTrimLeft()
	fmt.Println("== ExampleTrimLeftFunc")
	ExampleTrimLeftFunc()
	fmt.Println("== ExampleTrimPrefix")
	ExampleTrimPrefix()
	fmt.Println("== ExampleTrimSpace")
	ExampleTrimSpace()
	fmt.Println("== ExampleTrimSuffix")
	ExampleTrimSuffix()
	fmt.Println("== ExampleTrimRight")
	ExampleTrimRight()
	fmt.Println("== ExampleTrimRightFunc")
	ExampleTrimRightFunc()
	fmt.Println("== ExampleToLower")
	ExampleToLower()
	fmt.Println("== ExampleToLowerSpecial")
	ExampleToLowerSpecial()
	fmt.Println("== ExampleToUpper")
	ExampleToUpper()
	fmt.Println("== ExampleToUpperSpecial")
	ExampleToUpperSpecial()
}

// Output:
// == ExampleBuffer
// Hello world!== ExampleBuffer_Bytes
// hello world== ExampleBuffer_AvailableBuffer
// 0 1 2 3 == ExampleBuffer_Cap
// 10
// 10
// == ExampleBuffer_Grow
// "64 bytes or fewer"== ExampleBuffer_Len
// 5== ExampleBuffer_Next
// ab
// cd
// e== ExampleBuffer_Read
// 1
// bcde
// a
// == ExampleBuffer_ReadByte
// 97
// bcde
// == ExampleClone
// abc
// abc
// dbc
// == ExampleCompare
// == ExampleContains
// true
// false
// true
// true
// == ExampleContainsAny
// true
// true
// false
// false
// == ExampleContainsRune
// true
// false
// true
// true
// false
// == ExampleContainsFunc
// false
// true
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
// == ExampleEqual
// true
// false
// == ExampleEqualFold
// true
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
// == ExampleIndexByte
// 4
// -1
// == ExampleIndexFunc
// 7
// -1
// == ExampleIndexAny
// 2
// -1
// == ExampleIndexRune
// 4
// -1
// == ExampleJoin
// foo, bar, baz== ExampleLastIndex
// 0
// 3
// -1
// == ExampleLastIndexAny
// 5
// 3
// -1
// == ExampleLastIndexByte
// 3
// 8
// -1
// == ExampleLastIndexFunc
// 8
// 9
// -1
// == ExampleMap
// 'Gjnf oevyyvt naq gur fyvgul tbcure...
// == ExampleReader_Len
// 3
// 16
// == ExampleRepeat
// banana== ExampleReplace
// oinky oinky oink
// moo moo moo
// == ExampleReplaceAll
// moo moo moo
// == ExampleRunes
// U+0067 'g'
// U+006F 'o'
// U+0020 ' '
// U+0067 'g'
// U+006F 'o'
// U+0070 'p'
// U+0068 'h'
// U+0065 'e'
// U+0072 'r'
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
// Her Royal Highness== ExampleToTitle
// LOUD NOISES
// БРАТ
// == ExampleToTitleSpecial
// Original : ahoj vývojári golang
// ToTitle : AHOJ VÝVOJÁRİ GOLANG
// == ExampleToValidUTF8
// abc
// abc
// abc
// == ExampleTrim
// ["Achtung! Achtung"]== ExampleTrimFunc
// -gopher!
// "go-gopher!"
// go-gopher
// go-gopher!
// == ExampleTrimLeft
// gopher8257== ExampleTrimLeftFunc
// -gopher
// go-gopher!
// go-gopher!567
// == ExampleTrimPrefix
// Hello, world!== ExampleTrimSpace
// a lone gopher== ExampleTrimSuffix
// Hello, world!== ExampleTrimRight
// 453gopher== ExampleTrimRightFunc
// go-
// go-gopher
// 1234go-gopher!
// == ExampleToLower
// gopher== ExampleToLowerSpecial
// Original : AHOJ VÝVOJÁRİ GOLANG
// ToLower : ahoj vývojári golang
// == ExampleToUpper
// GOPHER== ExampleToUpperSpecial
// Original : ahoj vývojári golang
// ToUpper : AHOJ VÝVOJÁRİ GOLANG
