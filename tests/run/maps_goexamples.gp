// phase: 09
// Go 1.25's maps/example_test.go (Clone, Copy, DeleteFunc, Equal, EqualFunc, Keys, Values, All, Insert, Collect) run in order; expected output from real Go.

package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync/atomic"
)


func ExampleClone() {
	m1 := map[string]int{
		"key": 1,
	}
	m2 := maps.Clone(m1)
	m2["key"] = 100
	fmt.Println(m1["key"])
	fmt.Println(m2["key"])

	m3 := map[string][]int{
		"key": {1, 2, 3},
	}
	m4 := maps.Clone(m3)
	fmt.Println(m4["key"][0])
	m4["key"][0] = 100
	fmt.Println(m3["key"][0])
	fmt.Println(m4["key"][0])

}

func ExampleCopy() {
	m1 := map[string]int{
		"one": 1,
		"two": 2,
	}
	m2 := map[string]int{
		"one": 10,
	}

	maps.Copy(m2, m1)
	fmt.Println("m2 is:", m2)

	m2["one"] = 100
	fmt.Println("m1 is:", m1)
	fmt.Println("m2 is:", m2)

	m3 := map[string][]int{
		"one": {1, 2, 3},
		"two": {4, 5, 6},
	}
	m4 := map[string][]int{
		"one": {7, 8, 9},
	}

	maps.Copy(m4, m3)
	fmt.Println("m4 is:", m4)

	m4["one"][0] = 100
	fmt.Println("m3 is:", m3)
	fmt.Println("m4 is:", m4)

}

func ExampleDeleteFunc() {
	m := map[string]int{
		"one":   1,
		"two":   2,
		"three": 3,
		"four":  4,
	}
	maps.DeleteFunc(m, func(k string, v int) bool {
		return v%2 != 0 // delete odd values
	})
	fmt.Println(m)

}

func ExampleEqual() {
	m1 := map[int]string{
		1:    "one",
		10:   "Ten",
		1000: "THOUSAND",
	}
	m2 := map[int]string{
		1:    "one",
		10:   "Ten",
		1000: "THOUSAND",
	}
	m3 := map[int]string{
		1:    "one",
		10:   "ten",
		1000: "thousand",
	}

	fmt.Println(maps.Equal(m1, m2))
	fmt.Println(maps.Equal(m1, m3))

}

func ExampleEqualFunc() {
	m1 := map[int]string{
		1:    "one",
		10:   "Ten",
		1000: "THOUSAND",
	}
	m2 := map[int][]byte{
		1:    []byte("One"),
		10:   []byte("Ten"),
		1000: []byte("Thousand"),
	}
	eq := maps.EqualFunc(m1, m2, func(v1 string, v2 []byte) bool {
		return strings.ToLower(v1) == strings.ToLower(string(v2))
	})
	fmt.Println(eq)

}

func ExampleAll() {
	m1 := map[string]int{
		"one": 1,
		"two": 2,
	}
	m2 := map[string]int{
		"one": 10,
	}
	maps.Insert(m2, maps.All(m1))
	fmt.Println("m2 is:", m2)

}

func ExampleKeys() {
	m1 := map[int]string{
		1:    "one",
		10:   "Ten",
		1000: "THOUSAND",
	}
	keys := slices.Sorted(maps.Keys(m1))
	fmt.Println(keys)

}

func ExampleValues() {
	m1 := map[int]string{
		1:    "one",
		10:   "Ten",
		1000: "THOUSAND",
	}
	values := slices.Sorted(maps.Values(m1))
	fmt.Println(values)

}

func ExampleInsert() {
	m1 := map[int]string{
		1000: "THOUSAND",
	}
	s1 := []string{"zero", "one", "two", "three"}
	maps.Insert(m1, slices.All(s1))
	fmt.Println("m1 is:", m1)

}

func ExampleCollect() {
	s1 := []string{"zero", "one", "two", "three"}
	m1 := maps.Collect(slices.All(s1))
	fmt.Println("m1 is:", m1)

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
	fmt.Println("== ExampleCopy")
	ExampleCopy()
	fmt.Println("== ExampleDeleteFunc")
	ExampleDeleteFunc()
	fmt.Println("== ExampleEqual")
	ExampleEqual()
	fmt.Println("== ExampleEqualFunc")
	ExampleEqualFunc()
	fmt.Println("== ExampleAll")
	ExampleAll()
	fmt.Println("== ExampleKeys")
	ExampleKeys()
	fmt.Println("== ExampleValues")
	ExampleValues()
	fmt.Println("== ExampleInsert")
	ExampleInsert()
	fmt.Println("== ExampleCollect")
	ExampleCollect()
}

// Output:
// == ExampleClone
// 1
// 100
// 1
// 100
// 100
// == ExampleCopy
// m2 is: map[one:1 two:2]
// m1 is: map[one:1 two:2]
// m2 is: map[one:100 two:2]
// m4 is: map[one:[1 2 3] two:[4 5 6]]
// m3 is: map[one:[100 2 3] two:[4 5 6]]
// m4 is: map[one:[100 2 3] two:[4 5 6]]
// == ExampleDeleteFunc
// map[four:4 two:2]
// == ExampleEqual
// true
// false
// == ExampleEqualFunc
// true
// == ExampleAll
// m2 is: map[one:1 two:2]
// == ExampleKeys
// [1 10 1000]
// == ExampleValues
// [THOUSAND Ten one]
// == ExampleInsert
// m1 is: map[0:zero 1:one 2:two 3:three 1000:THOUSAND]
// == ExampleCollect
// m1 is: map[0:zero 1:one 2:two 3:three]
