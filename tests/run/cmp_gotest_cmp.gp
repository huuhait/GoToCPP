// phase: 09
// Go 1.25's cmp/cmp_test.go (Less, Compare of ints, strings, floats with NaN, -0 and infinities, and uintptr; Or; the documented examples) through a small harness; sort.Float64s is a local insertion sort in the same order; expected output from real Go.

package main

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync/atomic"
	"unsafe"
)


func sortFloat64s(x []float64) { // sort.Float64s: package sort comes with the rest of step 9.4; same order (NaNs first)
	less := func(a, b float64) bool { return a < b || (a != a && b == b) }
	for i := 1; i < len(x); i++ {
		for j := i; j > 0 && less(x[j], x[j-1]); j-- {
			x[j], x[j-1] = x[j-1], x[j]
		}
	}
}

var negzero = math.Copysign(0, -1)
var nonnilptr uintptr = uintptr(unsafe.Pointer(&negzero))
var nilptr uintptr = uintptr(unsafe.Pointer(nil))

var tests = []struct {
	x, y    any
	compare int
}{
	{1, 2, -1},
	{1, 1, 0},
	{2, 1, +1},
	{"a", "aa", -1},
	{"a", "a", 0},
	{"aa", "a", +1},
	{1.0, 1.1, -1},
	{1.1, 1.1, 0},
	{1.1, 1.0, +1},
	{math.Inf(1), math.Inf(1), 0},
	{math.Inf(-1), math.Inf(-1), 0},
	{math.Inf(-1), 1.0, -1},
	{1.0, math.Inf(-1), +1},
	{math.Inf(1), 1.0, +1},
	{1.0, math.Inf(1), -1},
	{math.NaN(), math.NaN(), 0},
	{0.0, math.NaN(), +1},
	{math.NaN(), 0.0, -1},
	{math.NaN(), math.Inf(-1), -1},
	{math.Inf(-1), math.NaN(), +1},
	{0.0, 0.0, 0},
	{negzero, negzero, 0},
	{negzero, 0.0, 0},
	{0.0, negzero, 0},
	{negzero, 1.0, -1},
	{negzero, -1.0, +1},
	{nilptr, nonnilptr, -1},
	{nonnilptr, nilptr, 1},
	{nonnilptr, nonnilptr, 0},
}

func TestLess(t *testT) {
	for _, test := range tests {
		var b bool
		switch test.x.(type) {
		case int:
			b = cmp.Less(test.x.(int), test.y.(int))
		case string:
			b = cmp.Less(test.x.(string), test.y.(string))
		case float64:
			b = cmp.Less(test.x.(float64), test.y.(float64))
		case uintptr:
			b = cmp.Less(test.x.(uintptr), test.y.(uintptr))
		}
		atomic.AddInt64(&checks, 1)
		if b != (test.compare < 0) {
			t.Errorf("Less(%v, %v) == %t, want %t", test.x, test.y, b, test.compare < 0)
		}
	}
}

func TestCompare(t *testT) {
	for _, test := range tests {
		var c int
		switch test.x.(type) {
		case int:
			c = cmp.Compare(test.x.(int), test.y.(int))
		case string:
			c = cmp.Compare(test.x.(string), test.y.(string))
		case float64:
			c = cmp.Compare(test.x.(float64), test.y.(float64))
		case uintptr:
			c = cmp.Compare(test.x.(uintptr), test.y.(uintptr))
		}
		atomic.AddInt64(&checks, 1)
		if c != test.compare {
			t.Errorf("Compare(%v, %v) == %d, want %d", test.x, test.y, c, test.compare)
		}
	}
}

func TestSort(t *testT) {
	// Test that our comparison function is consistent with
	// sort.Float64s.
	input := []float64{1.0, 0.0, negzero, math.Inf(1), math.Inf(-1), math.NaN()}
	sortFloat64s(input)
	for i := 0; i < len(input)-1; i++ {
		atomic.AddInt64(&checks, 1)
		if cmp.Less(input[i+1], input[i]) {
			t.Errorf("Less sort mismatch at %d in %v", i, input)
		}
		atomic.AddInt64(&checks, 1)
		if cmp.Compare(input[i], input[i+1]) > 0 {
			t.Errorf("Compare sort mismatch at %d in %v", i, input)
		}
	}
}

func TestOr(t *testT) {
	cases := []struct {
		in   []int
		want int
	}{
		{nil, 0},
		{[]int{0}, 0},
		{[]int{1}, 1},
		{[]int{0, 2}, 2},
		{[]int{3, 0}, 3},
		{[]int{4, 5}, 4},
		{[]int{0, 6, 7}, 6},
	}
	for _, tc := range cases {
		atomic.AddInt64(&checks, 1)
		if got := cmp.Or(tc.in...); got != tc.want {
			t.Errorf("cmp.Or(%v) = %v; want %v", tc.in, got, tc.want)
		}
	}
}

func ExampleOr() {
	// Suppose we have some user input
	// that may or may not be an empty string
	userInput1 := ""
	userInput2 := "some text"

	fmt.Println(cmp.Or(userInput1, "default"))
	fmt.Println(cmp.Or(userInput2, "default"))
	fmt.Println(cmp.Or(userInput1, userInput2, "default"))

}

func ExampleOr_sort() {
	type Order struct {
		Product  string
		Customer string
		Price    float64
	}
	orders := []Order{
		{"foo", "alice", 1.00},
		{"bar", "bob", 3.00},
		{"baz", "carol", 4.00},
		{"foo", "alice", 2.00},
		{"bar", "carol", 1.00},
		{"foo", "bob", 4.00},
	}
	// Sort by customer first, product second, and last by higher price
	slices.SortFunc(orders, func(a, b Order) int {
		return cmp.Or(
			strings.Compare(a.Customer, b.Customer),
			strings.Compare(a.Product, b.Product),
			cmp.Compare(b.Price, a.Price),
		)
	})
	for _, order := range orders {
		fmt.Printf("%s %s %.2f\n", order.Product, order.Customer, order.Price)
	}

}

func ExampleLess() {
	fmt.Println(cmp.Less(1, 2))
	fmt.Println(cmp.Less("a", "aa"))
	fmt.Println(cmp.Less(1.0, math.NaN()))
	fmt.Println(cmp.Less(math.NaN(), 1.0))

}

func ExampleCompare() {
	fmt.Println(cmp.Compare(1, 2))
	fmt.Println(cmp.Compare("a", "aa"))
	fmt.Println(cmp.Compare(1.5, 1.5))
	fmt.Println(cmp.Compare(math.NaN(), 1.0))

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
	fmt.Println("== ExampleOr")
	ExampleOr()
	fmt.Println("== ExampleOr_sort")
	ExampleOr_sort()
	fmt.Println("== ExampleLess")
	ExampleLess()
	fmt.Println("== ExampleCompare")
	ExampleCompare()
	tests_ := []testCase{
		{"TestLess", TestLess},
		{"TestCompare", TestCompare},
		{"TestSort", TestSort},
		{"TestOr", TestOr},
	}
	runTests(tests_)
}

// Output:
// == ExampleOr
// default
// some text
// some text
// == ExampleOr_sort
// foo alice 2.00
// foo alice 1.00
// bar bob 3.00
// foo bob 4.00
// bar carol 1.00
// baz carol 4.00
// == ExampleLess
// true
// true
// false
// true
// == ExampleCompare
// -1
// -1
// 0
// -1
// ok TestLess subtests: 0 checks: 29
// ok TestCompare subtests: 0 checks: 29
// ok TestSort subtests: 0 checks: 10
// ok TestOr subtests: 0 checks: 7
// tests: 4 failed: 0
