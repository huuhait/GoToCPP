// phase: 09
// Go 1.25's regexp/example_test.go (Match, Find, FindAll*, ReplaceAll*, Expand, Split, SubexpNames, SubexpIndex, QuoteMeta, Longest, MatchReader, named groups, the String methods) run in order; expected output from real Go.

package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
)


func Example() {
	// Compile the expression once, usually at init time.
	// Use raw strings to avoid having to quote the backslashes.
	var validID = regexp.MustCompile(`^[a-z]+\[[0-9]+\]$`)

	fmt.Println(validID.MatchString("adam[23]"))
	fmt.Println(validID.MatchString("eve[7]"))
	fmt.Println(validID.MatchString("Job[48]"))
	fmt.Println(validID.MatchString("snakey"))

}

func ExampleMatch() {
	matched, err := regexp.Match(`foo.*`, []byte(`seafood`))
	fmt.Println(matched, err)
	matched, err = regexp.Match(`bar.*`, []byte(`seafood`))
	fmt.Println(matched, err)
	matched, err = regexp.Match(`a(b`, []byte(`seafood`))
	fmt.Println(matched, err)

}

func ExampleMatchString() {
	matched, err := regexp.MatchString(`foo.*`, "seafood")
	fmt.Println(matched, err)
	matched, err = regexp.MatchString(`bar.*`, "seafood")
	fmt.Println(matched, err)
	matched, err = regexp.MatchString(`a(b`, "seafood")
	fmt.Println(matched, err)

}

func ExampleQuoteMeta() {
	fmt.Println(regexp.QuoteMeta(`Escaping symbols like: .+*?()|[]{}^$`))

}

func ExampleRegexp_Find() {
	re := regexp.MustCompile(`foo.?`)
	fmt.Printf("%q\n", re.Find([]byte(`seafood fool`)))

}

func ExampleRegexp_FindAll() {
	re := regexp.MustCompile(`foo.?`)
	fmt.Printf("%q\n", re.FindAll([]byte(`seafood fool`), -1))

}

func ExampleRegexp_FindAllSubmatch() {
	re := regexp.MustCompile(`foo(.?)`)
	fmt.Printf("%q\n", re.FindAllSubmatch([]byte(`seafood fool`), -1))

}

func ExampleRegexp_FindSubmatch() {
	re := regexp.MustCompile(`foo(.?)`)
	fmt.Printf("%q\n", re.FindSubmatch([]byte(`seafood fool`)))

}

func ExampleRegexp_Match() {
	re := regexp.MustCompile(`foo.?`)
	fmt.Println(re.Match([]byte(`seafood fool`)))
	fmt.Println(re.Match([]byte(`something else`)))

}

func ExampleRegexp_FindString() {
	re := regexp.MustCompile(`foo.?`)
	fmt.Printf("%q\n", re.FindString("seafood fool"))
	fmt.Printf("%q\n", re.FindString("meat"))

}

func ExampleRegexp_FindStringIndex() {
	re := regexp.MustCompile(`ab?`)
	fmt.Println(re.FindStringIndex("tablett"))
	fmt.Println(re.FindStringIndex("foo") == nil)

}

func ExampleRegexp_FindStringSubmatch() {
	re := regexp.MustCompile(`a(x*)b(y|z)c`)
	fmt.Printf("%q\n", re.FindStringSubmatch("-axxxbyc-"))
	fmt.Printf("%q\n", re.FindStringSubmatch("-abzc-"))

}

func ExampleRegexp_FindAllString() {
	re := regexp.MustCompile(`a.`)
	fmt.Println(re.FindAllString("paranormal", -1))
	fmt.Println(re.FindAllString("paranormal", 2))
	fmt.Println(re.FindAllString("graal", -1))
	fmt.Println(re.FindAllString("none", -1))

}

func ExampleRegexp_FindAllStringSubmatch() {
	re := regexp.MustCompile(`a(x*)b`)
	fmt.Printf("%q\n", re.FindAllStringSubmatch("-ab-", -1))
	fmt.Printf("%q\n", re.FindAllStringSubmatch("-axxb-", -1))
	fmt.Printf("%q\n", re.FindAllStringSubmatch("-ab-axb-", -1))
	fmt.Printf("%q\n", re.FindAllStringSubmatch("-axxb-ab-", -1))

}

func ExampleRegexp_FindAllStringSubmatchIndex() {
	re := regexp.MustCompile(`a(x*)b`)
	// Indices:
	//    01234567   012345678
	//    -ab-axb-   -axxb-ab-
	fmt.Println(re.FindAllStringSubmatchIndex("-ab-", -1))
	fmt.Println(re.FindAllStringSubmatchIndex("-axxb-", -1))
	fmt.Println(re.FindAllStringSubmatchIndex("-ab-axb-", -1))
	fmt.Println(re.FindAllStringSubmatchIndex("-axxb-ab-", -1))
	fmt.Println(re.FindAllStringSubmatchIndex("-foo-", -1))

}

func ExampleRegexp_FindSubmatchIndex() {
	re := regexp.MustCompile(`a(x*)b`)
	// Indices:
	//    01234567   012345678
	//    -ab-axb-   -axxb-ab-
	fmt.Println(re.FindSubmatchIndex([]byte("-ab-")))
	fmt.Println(re.FindSubmatchIndex([]byte("-axxb-")))
	fmt.Println(re.FindSubmatchIndex([]byte("-ab-axb-")))
	fmt.Println(re.FindSubmatchIndex([]byte("-axxb-ab-")))
	fmt.Println(re.FindSubmatchIndex([]byte("-foo-")))

}

func ExampleRegexp_Longest() {
	re := regexp.MustCompile(`a(|b)`)
	fmt.Println(re.FindString("ab"))
	re.Longest()
	fmt.Println(re.FindString("ab"))

}

func ExampleRegexp_MatchString() {
	re := regexp.MustCompile(`(gopher){2}`)
	fmt.Println(re.MatchString("gopher"))
	fmt.Println(re.MatchString("gophergopher"))
	fmt.Println(re.MatchString("gophergophergopher"))

}

func ExampleRegexp_NumSubexp() {
	re0 := regexp.MustCompile(`a.`)
	fmt.Printf("%d\n", re0.NumSubexp())

	re := regexp.MustCompile(`(.*)((a)b)(.*)a`)
	fmt.Println(re.NumSubexp())

}

func ExampleRegexp_ReplaceAll() {
	re := regexp.MustCompile(`a(x*)b`)
	fmt.Printf("%s\n", re.ReplaceAll([]byte("-ab-axxb-"), []byte("T")))
	fmt.Printf("%s\n", re.ReplaceAll([]byte("-ab-axxb-"), []byte("$1")))
	fmt.Printf("%s\n", re.ReplaceAll([]byte("-ab-axxb-"), []byte("$1W")))
	fmt.Printf("%s\n", re.ReplaceAll([]byte("-ab-axxb-"), []byte("${1}W")))

	re2 := regexp.MustCompile(`a(?P<1W>x*)b`)
	fmt.Printf("%s\n", re2.ReplaceAll([]byte("-ab-axxb-"), []byte("$1W")))
	fmt.Printf("%s\n", re2.ReplaceAll([]byte("-ab-axxb-"), []byte("${1}W")))

}

func ExampleRegexp_ReplaceAllLiteralString() {
	re := regexp.MustCompile(`a(x*)b`)
	fmt.Println(re.ReplaceAllLiteralString("-ab-axxb-", "T"))
	fmt.Println(re.ReplaceAllLiteralString("-ab-axxb-", "$1"))
	fmt.Println(re.ReplaceAllLiteralString("-ab-axxb-", "${1}"))

}

func ExampleRegexp_ReplaceAllString() {
	re := regexp.MustCompile(`a(x*)b`)
	fmt.Println(re.ReplaceAllString("-ab-axxb-", "T"))
	fmt.Println(re.ReplaceAllString("-ab-axxb-", "$1"))
	fmt.Println(re.ReplaceAllString("-ab-axxb-", "$1W"))
	fmt.Println(re.ReplaceAllString("-ab-axxb-", "${1}W"))

	re2 := regexp.MustCompile(`a(?P<1W>x*)b`)
	fmt.Printf("%s\n", re2.ReplaceAllString("-ab-axxb-", "$1W"))
	fmt.Println(re.ReplaceAllString("-ab-axxb-", "${1}W"))

}

func ExampleRegexp_ReplaceAllStringFunc() {
	re := regexp.MustCompile(`[^aeiou]`)
	fmt.Println(re.ReplaceAllStringFunc("seafood fool", strings.ToUpper))

}

func ExampleRegexp_SubexpNames() {
	re := regexp.MustCompile(`(?P<first>[a-zA-Z]+) (?P<last>[a-zA-Z]+)`)
	fmt.Println(re.MatchString("Alan Turing"))
	fmt.Printf("%q\n", re.SubexpNames())
	reversed := fmt.Sprintf("${%s} ${%s}", re.SubexpNames()[2], re.SubexpNames()[1])
	fmt.Println(reversed)
	fmt.Println(re.ReplaceAllString("Alan Turing", reversed))

}

func ExampleRegexp_SubexpIndex() {
	re := regexp.MustCompile(`(?P<first>[a-zA-Z]+) (?P<last>[a-zA-Z]+)`)
	fmt.Println(re.MatchString("Alan Turing"))
	matches := re.FindStringSubmatch("Alan Turing")
	lastIndex := re.SubexpIndex("last")
	fmt.Printf("last => %d\n", lastIndex)
	fmt.Println(matches[lastIndex])

}

func ExampleRegexp_Split() {
	a := regexp.MustCompile(`a`)
	fmt.Println(a.Split("banana", -1))
	fmt.Println(a.Split("banana", 0))
	fmt.Println(a.Split("banana", 1))
	fmt.Println(a.Split("banana", 2))
	zp := regexp.MustCompile(`z+`)
	fmt.Println(zp.Split("pizza", -1))
	fmt.Println(zp.Split("pizza", 0))
	fmt.Println(zp.Split("pizza", 1))
	fmt.Println(zp.Split("pizza", 2))

}

func ExampleRegexp_Expand() {
	content := []byte(`
	# comment line
	option1: value1
	option2: value2

	# another comment line
	option3: value3
`)

	// Regex pattern captures "key: value" pair from the content.
	pattern := regexp.MustCompile(`(?m)(?P<key>\w+):\s+(?P<value>\w+)$`)

	// Template to convert "key: value" to "key=value" by
	// referencing the values captured by the regex pattern.
	template := []byte("$key=$value\n")

	result := []byte{}

	// For each match of the regex in the content.
	for _, submatches := range pattern.FindAllSubmatchIndex(content, -1) {
		// Apply the captured submatches to the template and append the output
		// to the result.
		result = pattern.Expand(result, template, content, submatches)
	}
	fmt.Println(string(result))

}

func ExampleRegexp_ExpandString() {
	content := `
	# comment line
	option1: value1
	option2: value2

	# another comment line
	option3: value3
`

	// Regex pattern captures "key: value" pair from the content.
	pattern := regexp.MustCompile(`(?m)(?P<key>\w+):\s+(?P<value>\w+)$`)

	// Template to convert "key: value" to "key=value" by
	// referencing the values captured by the regex pattern.
	template := "$key=$value\n"

	result := []byte{}

	// For each match of the regex in the content.
	for _, submatches := range pattern.FindAllStringSubmatchIndex(content, -1) {
		// Apply the captured submatches to the template and append the output
		// to the result.
		result = pattern.ExpandString(result, template, content, submatches)
	}
	fmt.Println(string(result))

}

func ExampleRegexp_FindIndex() {
	content := []byte(`
	# comment line
	option1: value1
	option2: value2
`)
	// Regex pattern captures "key: value" pair from the content.
	pattern := regexp.MustCompile(`(?m)(?P<key>\w+):\s+(?P<value>\w+)$`)

	loc := pattern.FindIndex(content)
	fmt.Println(loc)
	fmt.Println(string(content[loc[0]:loc[1]]))

}

func ExampleRegexp_FindAllSubmatchIndex() {
	content := []byte(`
	# comment line
	option1: value1
	option2: value2
`)
	// Regex pattern captures "key: value" pair from the content.
	pattern := regexp.MustCompile(`(?m)(?P<key>\w+):\s+(?P<value>\w+)$`)
	allIndexes := pattern.FindAllSubmatchIndex(content, -1)
	for _, loc := range allIndexes {
		fmt.Println(loc)
		fmt.Println(string(content[loc[0]:loc[1]]))
		fmt.Println(string(content[loc[2]:loc[3]]))
		fmt.Println(string(content[loc[4]:loc[5]]))
	}

}

func ExampleRegexp_FindAllIndex() {
	content := []byte("London")
	re := regexp.MustCompile(`o.`)
	fmt.Println(re.FindAllIndex(content, 1))
	fmt.Println(re.FindAllIndex(content, -1))

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
	fmt.Println("== Example")
	Example()
	fmt.Println("== ExampleMatch")
	ExampleMatch()
	fmt.Println("== ExampleMatchString")
	ExampleMatchString()
	fmt.Println("== ExampleQuoteMeta")
	ExampleQuoteMeta()
	fmt.Println("== ExampleRegexp_Find")
	ExampleRegexp_Find()
	fmt.Println("== ExampleRegexp_FindAll")
	ExampleRegexp_FindAll()
	fmt.Println("== ExampleRegexp_FindAllSubmatch")
	ExampleRegexp_FindAllSubmatch()
	fmt.Println("== ExampleRegexp_FindSubmatch")
	ExampleRegexp_FindSubmatch()
	fmt.Println("== ExampleRegexp_Match")
	ExampleRegexp_Match()
	fmt.Println("== ExampleRegexp_FindString")
	ExampleRegexp_FindString()
	fmt.Println("== ExampleRegexp_FindStringIndex")
	ExampleRegexp_FindStringIndex()
	fmt.Println("== ExampleRegexp_FindStringSubmatch")
	ExampleRegexp_FindStringSubmatch()
	fmt.Println("== ExampleRegexp_FindAllString")
	ExampleRegexp_FindAllString()
	fmt.Println("== ExampleRegexp_FindAllStringSubmatch")
	ExampleRegexp_FindAllStringSubmatch()
	fmt.Println("== ExampleRegexp_FindAllStringSubmatchIndex")
	ExampleRegexp_FindAllStringSubmatchIndex()
	fmt.Println("== ExampleRegexp_FindSubmatchIndex")
	ExampleRegexp_FindSubmatchIndex()
	fmt.Println("== ExampleRegexp_Longest")
	ExampleRegexp_Longest()
	fmt.Println("== ExampleRegexp_MatchString")
	ExampleRegexp_MatchString()
	fmt.Println("== ExampleRegexp_NumSubexp")
	ExampleRegexp_NumSubexp()
	fmt.Println("== ExampleRegexp_ReplaceAll")
	ExampleRegexp_ReplaceAll()
	fmt.Println("== ExampleRegexp_ReplaceAllLiteralString")
	ExampleRegexp_ReplaceAllLiteralString()
	fmt.Println("== ExampleRegexp_ReplaceAllString")
	ExampleRegexp_ReplaceAllString()
	fmt.Println("== ExampleRegexp_ReplaceAllStringFunc")
	ExampleRegexp_ReplaceAllStringFunc()
	fmt.Println("== ExampleRegexp_SubexpNames")
	ExampleRegexp_SubexpNames()
	fmt.Println("== ExampleRegexp_SubexpIndex")
	ExampleRegexp_SubexpIndex()
	fmt.Println("== ExampleRegexp_Split")
	ExampleRegexp_Split()
	fmt.Println("== ExampleRegexp_Expand")
	ExampleRegexp_Expand()
	fmt.Println("== ExampleRegexp_ExpandString")
	ExampleRegexp_ExpandString()
	fmt.Println("== ExampleRegexp_FindIndex")
	ExampleRegexp_FindIndex()
	fmt.Println("== ExampleRegexp_FindAllSubmatchIndex")
	ExampleRegexp_FindAllSubmatchIndex()
	fmt.Println("== ExampleRegexp_FindAllIndex")
	ExampleRegexp_FindAllIndex()
}

// Output:
// == Example
// true
// true
// false
// false
// == ExampleMatch
// true <nil>
// false <nil>
// false error parsing regexp: missing closing ): `a(b`
// == ExampleMatchString
// true <nil>
// false <nil>
// false error parsing regexp: missing closing ): `a(b`
// == ExampleQuoteMeta
// Escaping symbols like: \.\+\*\?\(\)\|\[\]\{\}\^\$
// == ExampleRegexp_Find
// "food"
// == ExampleRegexp_FindAll
// ["food" "fool"]
// == ExampleRegexp_FindAllSubmatch
// [["food" "d"] ["fool" "l"]]
// == ExampleRegexp_FindSubmatch
// ["food" "d"]
// == ExampleRegexp_Match
// true
// false
// == ExampleRegexp_FindString
// "food"
// ""
// == ExampleRegexp_FindStringIndex
// [1 3]
// true
// == ExampleRegexp_FindStringSubmatch
// ["axxxbyc" "xxx" "y"]
// ["abzc" "" "z"]
// == ExampleRegexp_FindAllString
// [ar an al]
// [ar an]
// [aa]
// []
// == ExampleRegexp_FindAllStringSubmatch
// [["ab" ""]]
// [["axxb" "xx"]]
// [["ab" ""] ["axb" "x"]]
// [["axxb" "xx"] ["ab" ""]]
// == ExampleRegexp_FindAllStringSubmatchIndex
// [[1 3 2 2]]
// [[1 5 2 4]]
// [[1 3 2 2] [4 7 5 6]]
// [[1 5 2 4] [6 8 7 7]]
// []
// == ExampleRegexp_FindSubmatchIndex
// [1 3 2 2]
// [1 5 2 4]
// [1 3 2 2]
// [1 5 2 4]
// []
// == ExampleRegexp_Longest
// a
// ab
// == ExampleRegexp_MatchString
// false
// true
// true
// == ExampleRegexp_NumSubexp
// 0
// 4
// == ExampleRegexp_ReplaceAll
// -T-T-
// --xx-
// ---
// -W-xxW-
// --xx-
// -W-xxW-
// == ExampleRegexp_ReplaceAllLiteralString
// -T-T-
// -$1-$1-
// -${1}-${1}-
// == ExampleRegexp_ReplaceAllString
// -T-T-
// --xx-
// ---
// -W-xxW-
// --xx-
// -W-xxW-
// == ExampleRegexp_ReplaceAllStringFunc
// SeaFooD FooL
// == ExampleRegexp_SubexpNames
// true
// ["" "first" "last"]
// ${last} ${first}
// Turing Alan
// == ExampleRegexp_SubexpIndex
// true
// last => 2
// Turing
// == ExampleRegexp_Split
// [b n n ]
// []
// [banana]
// [b nana]
// [pi a]
// []
// [pizza]
// [pi a]
// == ExampleRegexp_Expand
// option1=value1
// option2=value2
// option3=value3
//
// == ExampleRegexp_ExpandString
// option1=value1
// option2=value2
// option3=value3
//
// == ExampleRegexp_FindIndex
// [18 33]
// option1: value1
// == ExampleRegexp_FindAllSubmatchIndex
// [18 33 18 25 27 33]
// option1: value1
// option1
// value1
// [35 50 35 42 44 50]
// option2: value2
// option2
// value2
// == ExampleRegexp_FindAllIndex
// [[1 3]]
// [[1 3] [4 6]]
