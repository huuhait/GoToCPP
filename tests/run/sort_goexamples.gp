// phase: 09
// Go 1.25's sort/example_test.go run in order; expected output from real Go.

package main

import (
	"fmt"
	"math"
	"sort"
	"sync/atomic"
)


func ExampleInts() {
	s := []int{5, 2, 6, 3, 1, 4} // unsorted
	sort.Ints(s)
	fmt.Println(s)

}

func ExampleIntsAreSorted() {
	s := []int{1, 2, 3, 4, 5, 6} // sorted ascending
	fmt.Println(sort.IntsAreSorted(s))

	s = []int{6, 5, 4, 3, 2, 1} // sorted descending
	fmt.Println(sort.IntsAreSorted(s))

	s = []int{3, 2, 4, 1, 5} // unsorted
	fmt.Println(sort.IntsAreSorted(s))

}

func ExampleFloat64s() {
	s := []float64{5.2, -1.3, 0.7, -3.8, 2.6} // unsorted
	sort.Float64s(s)
	fmt.Println(s)

	s = []float64{math.Inf(1), math.NaN(), math.Inf(-1), 0.0} // unsorted
	sort.Float64s(s)
	fmt.Println(s)

}

func ExampleFloat64sAreSorted() {
	s := []float64{0.7, 1.3, 2.6, 3.8, 5.2} // sorted ascending
	fmt.Println(sort.Float64sAreSorted(s))

	s = []float64{5.2, 3.8, 2.6, 1.3, 0.7} // sorted descending
	fmt.Println(sort.Float64sAreSorted(s))

	s = []float64{5.2, 1.3, 0.7, 3.8, 2.6} // unsorted
	fmt.Println(sort.Float64sAreSorted(s))

}

func ExampleReverse() {
	s := []int{5, 2, 6, 3, 1, 4} // unsorted
	sort.Sort(sort.Reverse(sort.IntSlice(s)))
	fmt.Println(s)

}

func ExampleSlice() {
	people := []struct {
		Name string
		Age  int
	}{
		{"Gopher", 7},
		{"Alice", 55},
		{"Vera", 24},
		{"Bob", 75},
	}
	sort.Slice(people, func(i, j int) bool { return people[i].Name < people[j].Name })
	fmt.Println("By name:", people)

	sort.Slice(people, func(i, j int) bool { return people[i].Age < people[j].Age })
	fmt.Println("By age:", people)

}

func ExampleSliceIsSorted() {
	numbers := []int{1, 2, 3, 4, 5, 6}

	isSortedAsc := sort.SliceIsSorted(numbers, func(i, j int) bool {
		return numbers[i] < numbers[j]
	})
	fmt.Printf("%v sorted ascending: %t\n", numbers, isSortedAsc)

	numbersDesc := []int{6, 5, 4, 3, 2, 1}

	isSortedDesc := sort.SliceIsSorted(numbersDesc, func(i, j int) bool {
		return numbersDesc[i] > numbersDesc[j]
	})
	fmt.Printf("%v sorted descending: %t\n", numbers, isSortedDesc)

	unsortedNumbers := []int{1, 3, 2, 4, 5}

	isSortedUnsorted := sort.SliceIsSorted(unsortedNumbers, func(i, j int) bool {
		return unsortedNumbers[i] < unsortedNumbers[j]
	})
	fmt.Printf("%v unsorted slice sorted: %t\n", unsortedNumbers, isSortedUnsorted)

}

func ExampleSliceStable() {

	people := []struct {
		Name string
		Age  int
	}{
		{"Alice", 25},
		{"Elizabeth", 75},
		{"Alice", 75},
		{"Bob", 75},
		{"Alice", 75},
		{"Bob", 25},
		{"Colin", 25},
		{"Elizabeth", 25},
	}

	// Sort by name, preserving original order
	sort.SliceStable(people, func(i, j int) bool { return people[i].Name < people[j].Name })
	fmt.Println("By name:", people)

	// Sort by age preserving name order
	sort.SliceStable(people, func(i, j int) bool { return people[i].Age < people[j].Age })
	fmt.Println("By age,name:", people)

}

func ExampleStrings() {
	s := []string{"Go", "Bravo", "Gopher", "Alpha", "Grin", "Delta"}
	sort.Strings(s)
	fmt.Println(s)

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
	fmt.Println("== ExampleInts")
	ExampleInts()
	fmt.Println("== ExampleIntsAreSorted")
	ExampleIntsAreSorted()
	fmt.Println("== ExampleFloat64s")
	ExampleFloat64s()
	fmt.Println("== ExampleFloat64sAreSorted")
	ExampleFloat64sAreSorted()
	fmt.Println("== ExampleReverse")
	ExampleReverse()
	fmt.Println("== ExampleSlice")
	ExampleSlice()
	fmt.Println("== ExampleSliceIsSorted")
	ExampleSliceIsSorted()
	fmt.Println("== ExampleSliceStable")
	ExampleSliceStable()
	fmt.Println("== ExampleStrings")
	ExampleStrings()
}

// Output:
// == ExampleInts
// [1 2 3 4 5 6]
// == ExampleIntsAreSorted
// true
// false
// false
// == ExampleFloat64s
// [-3.8 -1.3 0.7 2.6 5.2]
// [NaN -Inf 0 +Inf]
// == ExampleFloat64sAreSorted
// true
// false
// false
// == ExampleReverse
// [6 5 4 3 2 1]
// == ExampleSlice
// By name: [{Alice 55} {Bob 75} {Gopher 7} {Vera 24}]
// By age: [{Gopher 7} {Vera 24} {Alice 55} {Bob 75}]
// == ExampleSliceIsSorted
// [1 2 3 4 5 6] sorted ascending: true
// [1 2 3 4 5 6] sorted descending: true
// [1 3 2 4 5] unsorted slice sorted: false
// == ExampleSliceStable
// By name: [{Alice 25} {Alice 75} {Alice 75} {Bob 75} {Bob 25} {Colin 25} {Elizabeth 75} {Elizabeth 25}]
// By age,name: [{Alice 25} {Bob 25} {Colin 25} {Elizabeth 25} {Alice 75} {Alice 75} {Bob 75} {Elizabeth 75}]
// == ExampleStrings
// [Alpha Bravo Delta Go Gopher Grin]
