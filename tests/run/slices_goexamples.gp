// phase: 09
// Go 1.25's slices/example_test.go (the documented examples of the functions of package slices) run in order; expected output from real Go.

package main

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
)


func ExampleBinarySearch() {
	names := []string{"Alice", "Bob", "Vera"}
	n, found := slices.BinarySearch(names, "Vera")
	fmt.Println("Vera:", n, found)
	n, found = slices.BinarySearch(names, "Bill")
	fmt.Println("Bill:", n, found)

}

func ExampleBinarySearchFunc() {
	type Person struct {
		Name string
		Age  int
	}
	people := []Person{
		{"Alice", 55},
		{"Bob", 24},
		{"Gopher", 13},
	}
	n, found := slices.BinarySearchFunc(people, Person{"Bob", 0}, func(a, b Person) int {
		return strings.Compare(a.Name, b.Name)
	})
	fmt.Println("Bob:", n, found)

}

func ExampleCompact() {
	seq := []int{0, 1, 1, 2, 3, 5, 8}
	seq = slices.Compact(seq)
	fmt.Println(seq)

}

func ExampleCompactFunc() {
	names := []string{"bob", "Bob", "alice", "Vera", "VERA"}
	names = slices.CompactFunc(names, strings.EqualFold)
	fmt.Println(names)

}

func ExampleCompare() {
	names := []string{"Alice", "Bob", "Vera"}
	fmt.Println("Equal:", slices.Compare(names, []string{"Alice", "Bob", "Vera"}))
	fmt.Println("V < X:", slices.Compare(names, []string{"Alice", "Bob", "Xena"}))
	fmt.Println("V > C:", slices.Compare(names, []string{"Alice", "Bob", "Cat"}))
	fmt.Println("3 > 2:", slices.Compare(names, []string{"Alice", "Bob"}))

}

func ExampleCompareFunc() {
	numbers := []int{0, 43, 8}
	strings := []string{"0", "0", "8"}
	result := slices.CompareFunc(numbers, strings, func(n int, s string) int {
		sn, err := strconv.Atoi(s)
		if err != nil {
			return 1
		}
		return cmp.Compare(n, sn)
	})
	fmt.Println(result)

}

func ExampleContainsFunc() {
	numbers := []int{0, 42, -10, 8}
	hasNegative := slices.ContainsFunc(numbers, func(n int) bool {
		return n < 0
	})
	fmt.Println("Has a negative:", hasNegative)
	hasOdd := slices.ContainsFunc(numbers, func(n int) bool {
		return n%2 != 0
	})
	fmt.Println("Has an odd number:", hasOdd)

}

func ExampleDelete() {
	letters := []string{"a", "b", "c", "d", "e"}
	letters = slices.Delete(letters, 1, 4)
	fmt.Println(letters)

}

func ExampleDeleteFunc() {
	seq := []int{0, 1, 1, 2, 3, 5, 8}
	seq = slices.DeleteFunc(seq, func(n int) bool {
		return n%2 != 0 // delete the odd numbers
	})
	fmt.Println(seq)

}

func ExampleEqual() {
	numbers := []int{0, 42, 8}
	fmt.Println(slices.Equal(numbers, []int{0, 42, 8}))
	fmt.Println(slices.Equal(numbers, []int{10}))

}

func ExampleEqualFunc() {
	numbers := []int{0, 42, 8}
	strings := []string{"000", "42", "0o10"}
	equal := slices.EqualFunc(numbers, strings, func(n int, s string) bool {
		sn, err := strconv.ParseInt(s, 0, 64)
		if err != nil {
			return false
		}
		return n == int(sn)
	})
	fmt.Println(equal)

}

func ExampleIndex() {
	numbers := []int{0, 42, 8}
	fmt.Println(slices.Index(numbers, 8))
	fmt.Println(slices.Index(numbers, 7))

}

func ExampleIndexFunc() {
	numbers := []int{0, 42, -10, 8}
	i := slices.IndexFunc(numbers, func(n int) bool {
		return n < 0
	})
	fmt.Println("First negative at index", i)

}

func ExampleInsert() {
	names := []string{"Alice", "Bob", "Vera"}
	names = slices.Insert(names, 1, "Bill", "Billie")
	names = slices.Insert(names, len(names), "Zac")
	fmt.Println(names)

}

func ExampleIsSorted() {
	fmt.Println(slices.IsSorted([]string{"Alice", "Bob", "Vera"}))
	fmt.Println(slices.IsSorted([]int{0, 2, 1}))

}

func ExampleIsSortedFunc() {
	names := []string{"alice", "Bob", "VERA"}
	isSortedInsensitive := slices.IsSortedFunc(names, func(a, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})
	fmt.Println(isSortedInsensitive)
	fmt.Println(slices.IsSorted(names))

}

func ExampleMax() {
	numbers := []int{0, 42, -10, 8}
	fmt.Println(slices.Max(numbers))

}

func ExampleMaxFunc() {
	type Person struct {
		Name string
		Age  int
	}
	people := []Person{
		{"Gopher", 13},
		{"Alice", 55},
		{"Vera", 24},
		{"Bob", 55},
	}
	firstOldest := slices.MaxFunc(people, func(a, b Person) int {
		return cmp.Compare(a.Age, b.Age)
	})
	fmt.Println(firstOldest.Name)

}

func ExampleMin() {
	numbers := []int{0, 42, -10, 8}
	fmt.Println(slices.Min(numbers))

}

func ExampleMinFunc() {
	type Person struct {
		Name string
		Age  int
	}
	people := []Person{
		{"Gopher", 13},
		{"Bob", 5},
		{"Vera", 24},
		{"Bill", 5},
	}
	firstYoungest := slices.MinFunc(people, func(a, b Person) int {
		return cmp.Compare(a.Age, b.Age)
	})
	fmt.Println(firstYoungest.Name)

}

func ExampleReplace() {
	names := []string{"Alice", "Bob", "Vera", "Zac"}
	names = slices.Replace(names, 1, 3, "Bill", "Billie", "Cat")
	fmt.Println(names)

}

func ExampleReverse() {
	names := []string{"alice", "Bob", "VERA"}
	slices.Reverse(names)
	fmt.Println(names)

}

func ExampleSort() {
	smallInts := []int8{0, 42, -10, 8}
	slices.Sort(smallInts)
	fmt.Println(smallInts)

}

func ExampleSortFunc_caseInsensitive() {
	names := []string{"Bob", "alice", "VERA"}
	slices.SortFunc(names, func(a, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})
	fmt.Println(names)

}

func ExampleSortFunc_multiField() {
	type Person struct {
		Name string
		Age  int
	}
	people := []Person{
		{"Gopher", 13},
		{"Alice", 55},
		{"Bob", 24},
		{"Alice", 20},
	}
	slices.SortFunc(people, func(a, b Person) int {
		if n := strings.Compare(a.Name, b.Name); n != 0 {
			return n
		}
		// If names are equal, order by age
		return cmp.Compare(a.Age, b.Age)
	})
	fmt.Println(people)

}

func ExampleSortStableFunc() {
	type Person struct {
		Name string
		Age  int
	}
	people := []Person{
		{"Gopher", 13},
		{"Alice", 20},
		{"Bob", 24},
		{"Alice", 55},
	}
	// Stable sort by name, keeping age ordering of Alice intact
	slices.SortStableFunc(people, func(a, b Person) int {
		return strings.Compare(a.Name, b.Name)
	})
	fmt.Println(people)

}

func ExampleClone() {
	numbers := []int{0, 42, -10, 8}
	clone := slices.Clone(numbers)
	fmt.Println(clone)
	clone[2] = 10
	fmt.Println(numbers)
	fmt.Println(clone)

}

func ExampleGrow() {
	numbers := []int{0, 42, -10, 8}
	grow := slices.Grow(numbers, 2)
	fmt.Println(cap(numbers))
	fmt.Println(grow)
	fmt.Println(len(grow))
	fmt.Println(cap(grow))

}

func ExampleClip() {
	a := [...]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	s := a[:4:10]
	clip := slices.Clip(s)
	fmt.Println(cap(s))
	fmt.Println(clip)
	fmt.Println(len(clip))
	fmt.Println(cap(clip))

}

func ExampleConcat() {
	s1 := []int{0, 1, 2, 3}
	s2 := []int{4, 5, 6}
	concat := slices.Concat(s1, s2)
	fmt.Println(concat)

}

func ExampleContains() {
	numbers := []int{0, 1, 2, 3}
	fmt.Println(slices.Contains(numbers, 2))
	fmt.Println(slices.Contains(numbers, 4))

}

func ExampleRepeat() {
	numbers := []int{0, 1, 2, 3}
	repeat := slices.Repeat(numbers, 2)
	fmt.Println(repeat)

}

func ExampleAll() {
	names := []string{"Alice", "Bob", "Vera"}
	for i, v := range slices.All(names) {
		fmt.Println(i, ":", v)
	}

}

func ExampleBackward() {
	names := []string{"Alice", "Bob", "Vera"}
	for i, v := range slices.Backward(names) {
		fmt.Println(i, ":", v)
	}

}

func ExampleValues() {
	names := []string{"Alice", "Bob", "Vera"}
	for v := range slices.Values(names) {
		fmt.Println(v)
	}

}

func ExampleAppendSeq() {
	seq := func(yield func(int) bool) {
		for i := 0; i < 10; i += 2 {
			if !yield(i) {
				return
			}
		}
	}

	s := slices.AppendSeq([]int{1, 2}, seq)
	fmt.Println(s)

}

func ExampleCollect() {
	seq := func(yield func(int) bool) {
		for i := 0; i < 10; i += 2 {
			if !yield(i) {
				return
			}
		}
	}

	s := slices.Collect(seq)
	fmt.Println(s)

}

func ExampleSorted() {
	seq := func(yield func(int) bool) {
		flag := -1
		for i := 0; i < 10; i += 2 {
			flag = -flag
			if !yield(i * flag) {
				return
			}
		}
	}

	s := slices.Sorted(seq)
	fmt.Println(s)
	fmt.Println(slices.IsSorted(s))

}

func ExampleSortedFunc() {
	seq := func(yield func(int) bool) {
		flag := -1
		for i := 0; i < 10; i += 2 {
			flag = -flag
			if !yield(i * flag) {
				return
			}
		}
	}

	sortFunc := func(a, b int) int {
		return cmp.Compare(b, a) // the comparison is being done in reverse
	}

	s := slices.SortedFunc(seq, sortFunc)
	fmt.Println(s)

}

func ExampleSortedStableFunc() {
	type Person struct {
		Name string
		Age  int
	}

	people := []Person{
		{"Gopher", 13},
		{"Alice", 20},
		{"Bob", 5},
		{"Vera", 24},
		{"Zac", 20},
	}

	sortFunc := func(x, y Person) int {
		return cmp.Compare(x.Age, y.Age)
	}

	s := slices.SortedStableFunc(slices.Values(people), sortFunc)
	fmt.Println(s)

}

func ExampleChunk() {
	type Person struct {
		Name string
		Age  int
	}

	type People []Person

	people := People{
		{"Gopher", 13},
		{"Alice", 20},
		{"Bob", 5},
		{"Vera", 24},
		{"Zac", 15},
	}

	// Chunk people into []Person 2 elements at a time.
	for c := range slices.Chunk(people, 2) {
		fmt.Println(c)
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
	fmt.Println("== ExampleBinarySearch")
	ExampleBinarySearch()
	fmt.Println("== ExampleBinarySearchFunc")
	ExampleBinarySearchFunc()
	fmt.Println("== ExampleCompact")
	ExampleCompact()
	fmt.Println("== ExampleCompactFunc")
	ExampleCompactFunc()
	fmt.Println("== ExampleCompare")
	ExampleCompare()
	fmt.Println("== ExampleCompareFunc")
	ExampleCompareFunc()
	fmt.Println("== ExampleContainsFunc")
	ExampleContainsFunc()
	fmt.Println("== ExampleDelete")
	ExampleDelete()
	fmt.Println("== ExampleDeleteFunc")
	ExampleDeleteFunc()
	fmt.Println("== ExampleEqual")
	ExampleEqual()
	fmt.Println("== ExampleEqualFunc")
	ExampleEqualFunc()
	fmt.Println("== ExampleIndex")
	ExampleIndex()
	fmt.Println("== ExampleIndexFunc")
	ExampleIndexFunc()
	fmt.Println("== ExampleInsert")
	ExampleInsert()
	fmt.Println("== ExampleIsSorted")
	ExampleIsSorted()
	fmt.Println("== ExampleIsSortedFunc")
	ExampleIsSortedFunc()
	fmt.Println("== ExampleMax")
	ExampleMax()
	fmt.Println("== ExampleMaxFunc")
	ExampleMaxFunc()
	fmt.Println("== ExampleMin")
	ExampleMin()
	fmt.Println("== ExampleMinFunc")
	ExampleMinFunc()
	fmt.Println("== ExampleReplace")
	ExampleReplace()
	fmt.Println("== ExampleReverse")
	ExampleReverse()
	fmt.Println("== ExampleSort")
	ExampleSort()
	fmt.Println("== ExampleSortFunc_caseInsensitive")
	ExampleSortFunc_caseInsensitive()
	fmt.Println("== ExampleSortFunc_multiField")
	ExampleSortFunc_multiField()
	fmt.Println("== ExampleSortStableFunc")
	ExampleSortStableFunc()
	fmt.Println("== ExampleClone")
	ExampleClone()
	fmt.Println("== ExampleGrow")
	ExampleGrow()
	fmt.Println("== ExampleClip")
	ExampleClip()
	fmt.Println("== ExampleConcat")
	ExampleConcat()
	fmt.Println("== ExampleContains")
	ExampleContains()
	fmt.Println("== ExampleRepeat")
	ExampleRepeat()
	fmt.Println("== ExampleAll")
	ExampleAll()
	fmt.Println("== ExampleBackward")
	ExampleBackward()
	fmt.Println("== ExampleValues")
	ExampleValues()
	fmt.Println("== ExampleAppendSeq")
	ExampleAppendSeq()
	fmt.Println("== ExampleCollect")
	ExampleCollect()
	fmt.Println("== ExampleSorted")
	ExampleSorted()
	fmt.Println("== ExampleSortedFunc")
	ExampleSortedFunc()
	fmt.Println("== ExampleSortedStableFunc")
	ExampleSortedStableFunc()
	fmt.Println("== ExampleChunk")
	ExampleChunk()
}

// Output:
// == ExampleBinarySearch
// Vera: 2 true
// Bill: 1 false
// == ExampleBinarySearchFunc
// Bob: 1 true
// == ExampleCompact
// [0 1 2 3 5 8]
// == ExampleCompactFunc
// [bob alice Vera]
// == ExampleCompare
// Equal: 0
// V < X: -1
// V > C: 1
// 3 > 2: 1
// == ExampleCompareFunc
// 1
// == ExampleContainsFunc
// Has a negative: true
// Has an odd number: false
// == ExampleDelete
// [a e]
// == ExampleDeleteFunc
// [0 2 8]
// == ExampleEqual
// true
// false
// == ExampleEqualFunc
// true
// == ExampleIndex
// 2
// -1
// == ExampleIndexFunc
// First negative at index 2
// == ExampleInsert
// [Alice Bill Billie Bob Vera Zac]
// == ExampleIsSorted
// true
// false
// == ExampleIsSortedFunc
// true
// false
// == ExampleMax
// 42
// == ExampleMaxFunc
// Alice
// == ExampleMin
// -10
// == ExampleMinFunc
// Bob
// == ExampleReplace
// [Alice Bill Billie Cat Zac]
// == ExampleReverse
// [VERA Bob alice]
// == ExampleSort
// [-10 0 8 42]
// == ExampleSortFunc_caseInsensitive
// [alice Bob VERA]
// == ExampleSortFunc_multiField
// [{Alice 20} {Alice 55} {Bob 24} {Gopher 13}]
// == ExampleSortStableFunc
// [{Alice 20} {Alice 55} {Bob 24} {Gopher 13}]
// == ExampleClone
// [0 42 -10 8]
// [0 42 -10 8]
// [0 42 10 8]
// == ExampleGrow
// 4
// [0 42 -10 8]
// 4
// 8
// == ExampleClip
// 10
// [0 1 2 3]
// 4
// 4
// == ExampleConcat
// [0 1 2 3 4 5 6]
// == ExampleContains
// true
// false
// == ExampleRepeat
// [0 1 2 3 0 1 2 3]
// == ExampleAll
// 0 : Alice
// 1 : Bob
// 2 : Vera
// == ExampleBackward
// 2 : Vera
// 1 : Bob
// 0 : Alice
// == ExampleValues
// Alice
// Bob
// Vera
// == ExampleAppendSeq
// [1 2 0 2 4 6 8]
// == ExampleCollect
// [0 2 4 6 8]
// == ExampleSorted
// [-6 -2 0 4 8]
// true
// == ExampleSortedFunc
// [8 4 0 -2 -6]
// == ExampleSortedStableFunc
// [{Bob 5} {Gopher 13} {Alice 20} {Zac 20} {Vera 24}]
// == ExampleChunk
// [{Gopher 13} {Alice 20}]
// [{Bob 5} {Vera 24}]
// [{Zac 15}]
