// phase: 09
// Go 1.25's sort/sort_test.go (Sort, Stable, Reverse, Slice, SliceStable, the Bentley-McIlroy sort tests with the swap limits, the adversary that counts comparisons, IsSorted, Strings/Ints/Float64s with NaN, the non-deterministic comparison) through a small harness; Heapsort and ReverseRange are local copies of Go's unexported functions (export_test.go); math/rand/v2 is a splitmix generator; testing.Short() is true; expected output from real Go.

package main

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"sort"
	"sync/atomic"
)


// sort's export_test.go (Heapsort, ReverseRange) names unexported functions: local copies of Go's heapSort and reverseRange
// (zsortinterface.go, sort.go) over sort.Interface.
func siftDown(data sort.Interface, lo, hi, first int) {
	root := lo
	for {
		child := 2*root + 1
		if child >= hi {
			return
		}
		if child+1 < hi && data.Less(first+child, first+child+1) {
			child++
		}
		if !data.Less(first+root, first+child) {
			return
		}
		data.Swap(first+root, first+child)
		root = child
	}
}

func Heapsort(data sort.Interface) {
	first := 0
	lo := 0
	hi := data.Len()
	for i := (hi - 1) / 2; i >= 0; i-- {
		siftDown(data, i, hi, first)
	}
	for i := hi - 1; i >= 0; i-- {
		data.Swap(first, first+i)
		siftDown(data, lo, i, first)
	}
}

func ReverseRange(data sort.Interface, a, b int) {
	i := a
	j := b - 1
	for i < j {
		data.Swap(i, j)
		i++
		j--
	}
}

func (r *rngT) Float32() float32 { return float32(r.Uint64()>>40) / (1 << 24) }

var ints = [...]int{74, 59, 238, -784, 9845, 959, 905, 0, 0, 42, 7586, -5467984, 7586}
var float64s = [...]float64{74.3, 59.0, math.Inf(1), 238.2, -784.0, 2.3, math.NaN(), math.NaN(), math.Inf(-1), 9845.768, -959.7485, 905, 7.8, 7.8}
var stringsData = [...]string{"", "Hello", "foo", "bar", "foo", "f00", "%*&^*&^&", "***"}

func TestSortIntSlice(t *testT) {
	data := ints
	a := sort.IntSlice(data[0:])
	sort.Sort(a)
	atomic.AddInt64(&checks, 1)
	if !sort.IsSorted(a) {
		t.Errorf("sorted %v", ints)
		t.Errorf("   got %v", data)
	}
}

func TestSortFloat64Slice(t *testT) {
	data := float64s
	a := sort.Float64Slice(data[0:])
	sort.Sort(a)
	atomic.AddInt64(&checks, 1)
	if !sort.IsSorted(a) {
		t.Errorf("sorted %v", float64s)
		t.Errorf("   got %v", data)
	}
}

// Compare Sort with slices.Sort sorting a float64 slice containing NaNs.
func TestSortFloat64sCompareSlicesSort(t *testT) {
	slice1 := slices.Clone(float64s[:])
	slice2 := slices.Clone(float64s[:])

	sort.Sort(sort.Float64Slice(slice1))
	slices.Sort(slice2)

	// Compare for equality using cmp.Compare, which considers NaNs equal.
	atomic.AddInt64(&checks, 1)
	if !slices.EqualFunc(slice1, slice2, func(a, b float64) bool { return cmp.Compare(a, b) == 0 }) {
		t.Errorf("mismatch between Sort and slices.Sort: got %v, want %v", slice1, slice2)
	}
}

func TestSortStringSlice(t *testT) {
	data := stringsData
	a := sort.StringSlice(data[0:])
	sort.Sort(a)
	atomic.AddInt64(&checks, 1)
	if !sort.IsSorted(a) {
		t.Errorf("sorted %v", stringsData)
		t.Errorf("   got %v", data)
	}
}

func TestInts(t *testT) {
	data := ints
	sort.Ints(data[0:])
	atomic.AddInt64(&checks, 1)
	if !sort.IntsAreSorted(data[0:]) {
		t.Errorf("sorted %v", ints)
		t.Errorf("   got %v", data)
	}
}

func TestFloat64s(t *testT) {
	data := float64s
	sort.Float64s(data[0:])
	atomic.AddInt64(&checks, 1)
	if !sort.Float64sAreSorted(data[0:]) {
		t.Errorf("sorted %v", float64s)
		t.Errorf("   got %v", data)
	}
}

func TestStrings(t *testT) {
	data := stringsData
	sort.Strings(data[0:])
	atomic.AddInt64(&checks, 1)
	if !sort.StringsAreSorted(data[0:]) {
		t.Errorf("sorted %v", stringsData)
		t.Errorf("   got %v", data)
	}
}

func TestSlice(t *testT) {
	data := stringsData
	sort.Slice(data[:], func(i, j int) bool {
		return data[i] < data[j]
	})
	atomic.AddInt64(&checks, 1)
	if !sort.SliceIsSorted(data[:], func(i, j int) bool { return data[i] < data[j] }) {
		t.Errorf("sorted %v", stringsData)
		t.Errorf("   got %v", data)
	}
}

func TestSortLarge_Random(t *testT) {
	n := 1000000
	atomic.AddInt64(&checks, 1)
	if true {
		n /= 100
	}
	data := make([]int, n)
	for i := 0; i < len(data); i++ {
		data[i] = rng.Intn(100)
	}
	atomic.AddInt64(&checks, 1)
	if sort.IntsAreSorted(data) {
		t.Fatalf("terrible rand.rand")
	}
	sort.Ints(data)
	atomic.AddInt64(&checks, 1)
	if !sort.IntsAreSorted(data) {
		t.Errorf("sort didn't sort - 1M ints")
	}
}

func TestReverseSortIntSlice(t *testT) {
	data := ints
	data1 := ints
	a := sort.IntSlice(data[0:])
	sort.Sort(a)
	r := sort.IntSlice(data1[0:])
	sort.Sort(sort.Reverse(r))
	for i := 0; i < len(data); i++ {
		atomic.AddInt64(&checks, 1)
		if a[i] != r[len(data)-1-i] {
			t.Errorf("reverse sort didn't sort")
		}
		atomic.AddInt64(&checks, 1)
		if i > len(data)/2 {
			break
		}
	}
}

func TestBreakPatterns(t *testT) {
	// Special slice used to trigger breakPatterns.
	data := make([]int, 30)
	for i := range data {
		data[i] = 10
	}
	data[(len(data)/4)*1] = 0
	data[(len(data)/4)*2] = 1
	data[(len(data)/4)*3] = 2
	sort.Sort(sort.IntSlice(data))
}

func TestReverseRange(t *testT) {
	data := []int{1, 2, 3, 4, 5, 6, 7}
	ReverseRange(sort.IntSlice(data), 0, len(data))
	for i := len(data) - 1; i > 0; i-- {
		atomic.AddInt64(&checks, 1)
		if data[i] > data[i-1] {
			t.Fatalf("reverseRange didn't work")
		}
	}

	data1 := []int{1, 2, 3, 4, 5, 6, 7}
	data2 := []int{1, 2, 5, 4, 3, 6, 7}
	ReverseRange(sort.IntSlice(data1), 2, 5)
	for i, v := range data1 {
		atomic.AddInt64(&checks, 1)
		if v != data2[i] {
			t.Fatalf("reverseRange didn't work")
		}
	}
}

type nonDeterministicTestingData struct {
	r *rngT
}

func (t *nonDeterministicTestingData) Len() int {
	return 500
}
func (t *nonDeterministicTestingData) Less(i, j int) bool {
	atomic.AddInt64(&checks, 1)
	if i < 0 || j < 0 || i >= t.Len() || j >= t.Len() {
		panic("nondeterministic comparison out of bounds")
	}
	return t.r.Float32() < 0.5
}
func (t *nonDeterministicTestingData) Swap(i, j int) {
	atomic.AddInt64(&checks, 1)
	if i < 0 || j < 0 || i >= t.Len() || j >= t.Len() {
		panic("nondeterministic comparison out of bounds")
	}
}

func TestNonDeterministicComparison(t *testT) {
	// Ensure that sort.Sort does not panic when Less returns inconsistent results.
	// See https://golang.org/issue/14377.
	defer func() {
		atomic.AddInt64(&checks, 1)
		if r := recover(); r != nil {
			t.Error(r)
		}
	}()

	td := &nonDeterministicTestingData{
		r: rng,
	}

	for i := 0; i < 10; i++ {
		sort.Sort(td)
	}
}













const (
	_Sawtooth = iota
	_Rand
	_Stagger
	_Plateau
	_Shuffle
	_NDist
)

const (
	_Copy = iota
	_Reverse
	_ReverseFirstHalf
	_ReverseSecondHalf
	_Sorted
	_Dither
	_NMode
)

type testingData struct {
	desc        string
	t           *testT
	data        []int
	maxswap     int // number of swaps allowed
	ncmp, nswap int
}

func (d *testingData) Len() int { return len(d.data) }
func (d *testingData) Less(i, j int) bool {
	d.ncmp++
	return d.data[i] < d.data[j]
}
func (d *testingData) Swap(i, j int) {
	atomic.AddInt64(&checks, 1)
	if d.nswap >= d.maxswap {
		d.t.Fatalf("%s: used %d swaps sorting slice of %d", d.desc, d.nswap, len(d.data))
	}
	d.nswap++
	d.data[i], d.data[j] = d.data[j], d.data[i]
}

func lg(n int) int {
	i := 0
	for 1<<uint(i) < n {
		i++
	}
	return i
}

func testBentleyMcIlroy(t *testT, sortf func(sort.Interface), maxswap func(int) int) {
	sizes := []int{100, 1023, 1024, 1025}
	atomic.AddInt64(&checks, 1)
	if true {
		sizes = []int{100, 127, 128, 129}
	}
	dists := []string{"sawtooth", "rand", "stagger", "plateau", "shuffle"}
	modes := []string{"copy", "reverse", "reverse1", "reverse2", "sort", "dither"}
	var tmp1, tmp2 [1025]int
	for _, n := range sizes {
		for m := 1; m < 2*n; m *= 2 {
			for dist := 0; dist < _NDist; dist++ {
				j := 0
				k := 1
				data := tmp1[0:n]
				for i := 0; i < n; i++ {
					switch dist {
					case _Sawtooth:
						data[i] = i % m
					case _Rand:
						data[i] = rng.Intn(m)
					case _Stagger:
						data[i] = (i*m + i) % n
					case _Plateau:
						data[i] = min(i, m)
					case _Shuffle:
						atomic.AddInt64(&checks, 1)
						if rng.Intn(m) != 0 {
							j += 2
							data[i] = j
						} else {
							k += 2
							data[i] = k
						}
					}
				}

				mdata := tmp2[0:n]
				for mode := 0; mode < _NMode; mode++ {
					switch mode {
					case _Copy:
						for i := 0; i < n; i++ {
							mdata[i] = data[i]
						}
					case _Reverse:
						for i := 0; i < n; i++ {
							mdata[i] = data[n-i-1]
						}
					case _ReverseFirstHalf:
						for i := 0; i < n/2; i++ {
							mdata[i] = data[n/2-i-1]
						}
						for i := n / 2; i < n; i++ {
							mdata[i] = data[i]
						}
					case _ReverseSecondHalf:
						for i := 0; i < n/2; i++ {
							mdata[i] = data[i]
						}
						for i := n / 2; i < n; i++ {
							mdata[i] = data[n-(i-n/2)-1]
						}
					case _Sorted:
						for i := 0; i < n; i++ {
							mdata[i] = data[i]
						}
						// Ints is known to be correct
						// because mode Sort runs after mode _Copy.
						sort.Ints(mdata)
					case _Dither:
						for i := 0; i < n; i++ {
							mdata[i] = data[i] + i%5
						}
					}

					desc := fmt.Sprintf("n=%d m=%d dist=%s mode=%s", n, m, dists[dist], modes[mode])
					d := &testingData{desc: desc, t: t, data: mdata[0:n], maxswap: maxswap(n)}
					sortf(d)
					// Uncomment if you are trying to improve the number of compares/swaps.
					//t.Logf("%s: ncmp=%d, nswp=%d", desc, d.ncmp, d.nswap)

					// If we were testing C qsort, we'd have to make a copy
					// of the slice and sort it ourselves and then compare
					// x against it, to ensure that qsort was only permuting
					// the data, not (for example) overwriting it with zeros.
					//
					// In go, we don't have to be so paranoid: since the only
					// mutating method Sort can call is TestingData.swap,
					// it suffices here just to check that the final slice is sorted.
					atomic.AddInt64(&checks, 1)
					if !sort.IntsAreSorted(mdata) {
						t.Fatalf("%s: ints not sorted\n\t%v", desc, mdata)
					}
				}
			}
		}
	}
}

func TestSortBM(t *testT) {
	testBentleyMcIlroy(t, sort.Sort, func(n int) int { return n * lg(n) * 12 / 10 })
}

func TestHeapsortBM(t *testT) {
	testBentleyMcIlroy(t, Heapsort, func(n int) int { return n * lg(n) * 12 / 10 })
}

func TestStableBM(t *testT) {
	testBentleyMcIlroy(t, sort.Stable, func(n int) int { return n * lg(n) * lg(n) / 3 })
}

// This is based on the "antiquicksort" implementation by M. Douglas McIlroy.
// See https://www.cs.dartmouth.edu/~doug/mdmspe.pdf for more info.
type adversaryTestingData struct {
	t         *testT
	data      []int // item values, initialized to special gas value and changed by Less
	maxcmp    int   // number of comparisons allowed
	ncmp      int   // number of comparisons (calls to Less)
	nsolid    int   // number of elements that have been set to non-gas values
	candidate int   // guess at current pivot
	gas       int   // special value for unset elements, higher than everything else
}

func (d *adversaryTestingData) Len() int { return len(d.data) }

func (d *adversaryTestingData) Less(i, j int) bool {
	atomic.AddInt64(&checks, 1)
	if d.ncmp >= d.maxcmp {
		d.t.Fatalf("used %d comparisons sorting adversary data with size %d", d.ncmp, len(d.data))
	}
	d.ncmp++

	atomic.AddInt64(&checks, 1)
	if d.data[i] == d.gas && d.data[j] == d.gas {
		atomic.AddInt64(&checks, 1)
		if i == d.candidate {
			// freeze i
			d.data[i] = d.nsolid
			d.nsolid++
		} else {
			// freeze j
			d.data[j] = d.nsolid
			d.nsolid++
		}
	}

	atomic.AddInt64(&checks, 1)
	if d.data[i] == d.gas {
		d.candidate = i
	} else if d.data[j] == d.gas {
		d.candidate = j
	}

	return d.data[i] < d.data[j]
}

func (d *adversaryTestingData) Swap(i, j int) {
	d.data[i], d.data[j] = d.data[j], d.data[i]
}

func newAdversaryTestingData(t *testT, size int, maxcmp int) *adversaryTestingData {
	gas := size - 1
	data := make([]int, size)
	for i := 0; i < size; i++ {
		data[i] = gas
	}
	return &adversaryTestingData{t: t, data: data, maxcmp: maxcmp, gas: gas}
}

func TestAdversary(t *testT) {
	const size = 10000            // large enough to distinguish between O(n^2) and O(n*log(n))
	maxcmp := size * lg(size) * 4 // the factor 4 was found by trial and error
	d := newAdversaryTestingData(t, size, maxcmp)
	sort.Sort(d) // This should degenerate to heapsort.
	// Check data is fully populated and sorted.
	for i, v := range d.data {
		atomic.AddInt64(&checks, 1)
		if v != i {
			t.Fatalf("adversary data not fully sorted")
		}
	}
}

func TestStableInts(t *testT) {
	data := ints
	sort.Stable(sort.IntSlice(data[0:]))
	atomic.AddInt64(&checks, 1)
	if !sort.IntsAreSorted(data[0:]) {
		t.Errorf("nsorted %v\n   got %v", ints, data)
	}
}

type intPairs []struct {
	a, b int
}

// IntPairs compare on a only.
func (d intPairs) Len() int           { return len(d) }
func (d intPairs) Less(i, j int) bool { return d[i].a < d[j].a }
func (d intPairs) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }

// Record initial order in B.
func (d intPairs) initB() {
	for i := range d {
		d[i].b = i
	}
}

// InOrder checks if a-equal elements were not reordered.
func (d intPairs) inOrder() bool {
	lastA, lastB := -1, 0
	for i := 0; i < len(d); i++ {
		atomic.AddInt64(&checks, 1)
		if lastA != d[i].a {
			lastA = d[i].a
			lastB = d[i].b
			continue
		}
		atomic.AddInt64(&checks, 1)
		if d[i].b <= lastB {
			return false
		}
		lastB = d[i].b
	}
	return true
}

func TestStability(t *testT) {
	n, m := 100000, 1000
	atomic.AddInt64(&checks, 1)
	if true {
		n, m = 1000, 100
	}
	data := make(intPairs, n)

	// random distribution
	for i := 0; i < len(data); i++ {
		data[i].a = rng.Intn(m)
	}
	atomic.AddInt64(&checks, 1)
	if sort.IsSorted(data) {
		t.Fatalf("terrible rand.rand")
	}
	data.initB()
	sort.Stable(data)
	atomic.AddInt64(&checks, 1)
	if !sort.IsSorted(data) {
		t.Errorf("Stable didn't sort %d ints", n)
	}
	atomic.AddInt64(&checks, 1)
	if !data.inOrder() {
		t.Errorf("Stable wasn't stable on %d ints", n)
	}

	// already sorted
	data.initB()
	sort.Stable(data)
	atomic.AddInt64(&checks, 1)
	if !sort.IsSorted(data) {
		t.Errorf("Stable shuffled sorted %d ints (order)", n)
	}
	atomic.AddInt64(&checks, 1)
	if !data.inOrder() {
		t.Errorf("Stable shuffled sorted %d ints (stability)", n)
	}

	// sorted reversed
	for i := 0; i < len(data); i++ {
		data[i].a = len(data) - i
	}
	data.initB()
	sort.Stable(data)
	atomic.AddInt64(&checks, 1)
	if !sort.IsSorted(data) {
		t.Errorf("Stable didn't sort %d ints", n)
	}
	atomic.AddInt64(&checks, 1)
	if !data.inOrder() {
		t.Errorf("Stable wasn't stable on %d ints", n)
	}
}

var countOpsSizes = []int{1e2, 3e2, 1e3, 3e3, 1e4, 3e4, 1e5, 3e5, 1e6}

func countOps(t *testT, algo func(sort.Interface), name string) {
	sizes := countOpsSizes
	atomic.AddInt64(&checks, 1)
	if true {
		sizes = sizes[:5]
	}
	atomic.AddInt64(&checks, 1)
	if !false {
		t.Skip("Counting skipped as non-verbose mode.")
	}
	for _, n := range sizes {
		td := testingData{
			desc:    name,
			t:       t,
			data:    make([]int, n),
			maxswap: 1<<31 - 1,
		}
		for i := 0; i < n; i++ {
			td.data[i] = rng.Intn(n / 5)
		}
		algo(&td)
		t.Logf("%s %8d elements: %11d Swap, %10d Less", name, n, td.nswap, td.ncmp)
	}
}

func TestCountStableOps(t *testT) { countOps(t, sort.Stable, "Stable") }
func TestCountSortOps(t *testT)   { countOps(t, sort.Sort, "Sort  ") }

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
		{"TestSortIntSlice", TestSortIntSlice},
		{"TestSortFloat64Slice", TestSortFloat64Slice},
		{"TestSortFloat64sCompareSlicesSort", TestSortFloat64sCompareSlicesSort},
		{"TestSortStringSlice", TestSortStringSlice},
		{"TestInts", TestInts},
		{"TestFloat64s", TestFloat64s},
		{"TestStrings", TestStrings},
		{"TestSlice", TestSlice},
		{"TestSortLarge_Random", TestSortLarge_Random},
		{"TestReverseSortIntSlice", TestReverseSortIntSlice},
		{"TestBreakPatterns", TestBreakPatterns},
		{"TestReverseRange", TestReverseRange},
		{"TestNonDeterministicComparison", TestNonDeterministicComparison},
		{"TestSortBM", TestSortBM},
		{"TestHeapsortBM", TestHeapsortBM},
		{"TestStableBM", TestStableBM},
		{"TestAdversary", TestAdversary},
		{"TestStableInts", TestStableInts},
		{"TestStability", TestStability},
		{"TestCountStableOps", TestCountStableOps},
		{"TestCountSortOps", TestCountSortOps},
	}
	runTests(tests_)
}

// Output:
// ok TestSortIntSlice subtests: 0 checks: 1
// ok TestSortFloat64Slice subtests: 0 checks: 1
// ok TestSortFloat64sCompareSlicesSort subtests: 0 checks: 1
// ok TestSortStringSlice subtests: 0 checks: 1
// ok TestInts subtests: 0 checks: 1
// ok TestFloat64s subtests: 0 checks: 1
// ok TestStrings subtests: 0 checks: 1
// ok TestSlice subtests: 0 checks: 1
// ok TestSortLarge_Random subtests: 0 checks: 3
// ok TestReverseSortIntSlice subtests: 0 checks: 16
// ok TestBreakPatterns subtests: 0 checks: 0
// ok TestReverseRange subtests: 0 checks: 13
// ok TestNonDeterministicComparison subtests: 0 checks: 35068
// ok TestSortBM subtests: 0 checks: 159387
// ok TestHeapsortBM subtests: 0 checks: 631475
// ok TestStableBM subtests: 0 checks: 603330
// ok TestAdversary subtests: 0 checks: 230878
// ok TestStableInts subtests: 0 checks: 1
// ok TestStability subtests: 0 checks: 4808
// ok TestCountStableOps subtests: 0 checks: 2
// ok TestCountSortOps subtests: 0 checks: 2
// tests: 21 failed: 0
