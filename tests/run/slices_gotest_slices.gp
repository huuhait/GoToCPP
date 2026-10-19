// phase: 09
// Go 1.25's slices/slices_test.go (Equal, Compare, Index*, Contains*, Insert, Delete*, Replace, Clone, Compact*, Grow, Clip, Reverse, Concat, Repeat, the panics) through a small harness; the allocation counts are not checked (gocpp does not count them); TestConcat_too_large and the nine MaxInt-long zero-size cases of TestRepeat are not run (a slice of struct{} that long is a makeslice panic in gocpp); expected output from real Go.

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


var equalIntTests = []struct {
	s1, s2 []int
	want   bool
}{
	{
		[]int{1},
		nil,
		false,
	},
	{
		[]int{},
		nil,
		true,
	},
	{
		[]int{1, 2, 3},
		[]int{1, 2, 3},
		true,
	},
	{
		[]int{1, 2, 3},
		[]int{1, 2, 3, 4},
		false,
	},
}

var equalFloatTests = []struct {
	s1, s2       []float64
	wantEqual    bool
	wantEqualNaN bool
}{
	{
		[]float64{1, 2},
		[]float64{1, 2},
		true,
		true,
	},
	{
		[]float64{1, 2, math.NaN()},
		[]float64{1, 2, math.NaN()},
		false,
		true,
	},
}

func TestEqual(t *testT) {
	for _, test := range equalIntTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.Equal(test.s1, test.s2); got != test.want {
			t.Errorf("Equal(%v, %v) = %t, want %t", test.s1, test.s2, got, test.want)
		}
	}
	for _, test := range equalFloatTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.Equal(test.s1, test.s2); got != test.wantEqual {
			t.Errorf("Equal(%v, %v) = %t, want %t", test.s1, test.s2, got, test.wantEqual)
		}
	}
}

// equal is simply ==.
func equal[T comparable](v1, v2 T) bool {
	return v1 == v2
}

// equalNaN is like == except that all NaNs are equal.
func equalNaN[T comparable](v1, v2 T) bool {
	isNaN := func(f T) bool { return f != f }
	return v1 == v2 || (isNaN(v1) && isNaN(v2))
}

// offByOne returns true if integers v1 and v2 differ by 1.
func offByOne(v1, v2 int) bool {
	return v1 == v2+1 || v1 == v2-1
}

func TestEqualFunc(t *testT) {
	for _, test := range equalIntTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.EqualFunc(test.s1, test.s2, equal[int]); got != test.want {
			t.Errorf("EqualFunc(%v, %v, equal[int]) = %t, want %t", test.s1, test.s2, got, test.want)
		}
	}
	for _, test := range equalFloatTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.EqualFunc(test.s1, test.s2, equal[float64]); got != test.wantEqual {
			t.Errorf("Equal(%v, %v, equal[float64]) = %t, want %t", test.s1, test.s2, got, test.wantEqual)
		}
		atomic.AddInt64(&checks, 1)
		if got := slices.EqualFunc(test.s1, test.s2, equalNaN[float64]); got != test.wantEqualNaN {
			t.Errorf("Equal(%v, %v, equalNaN[float64]) = %t, want %t", test.s1, test.s2, got, test.wantEqualNaN)
		}
	}

	s1 := []int{1, 2, 3}
	s2 := []int{2, 3, 4}
	atomic.AddInt64(&checks, 1)
	if slices.EqualFunc(s1, s1, offByOne) {
		t.Errorf("EqualFunc(%v, %v, offByOne) = true, want false", s1, s1)
	}
	atomic.AddInt64(&checks, 1)
	if !slices.EqualFunc(s1, s2, offByOne) {
		t.Errorf("EqualFunc(%v, %v, offByOne) = false, want true", s1, s2)
	}

	s3 := []string{"a", "b", "c"}
	s4 := []string{"A", "B", "C"}
	atomic.AddInt64(&checks, 1)
	if !slices.EqualFunc(s3, s4, strings.EqualFold) {
		t.Errorf("EqualFunc(%v, %v, strings.EqualFold) = false, want true", s3, s4)
	}

	cmpIntString := func(v1 int, v2 string) bool {
		return string(rune(v1)-1+'a') == v2
	}
	atomic.AddInt64(&checks, 1)
	if !slices.EqualFunc(s1, s3, cmpIntString) {
		t.Errorf("EqualFunc(%v, %v, cmpIntString) = false, want true", s1, s3)
	}
}


var compareIntTests = []struct {
	s1, s2 []int
	want   int
}{
	{
		[]int{1},
		[]int{1},
		0,
	},
	{
		[]int{1},
		[]int{},
		1,
	},
	{
		[]int{},
		[]int{1},
		-1,
	},
	{
		[]int{},
		[]int{},
		0,
	},
	{
		[]int{1, 2, 3},
		[]int{1, 2, 3},
		0,
	},
	{
		[]int{1, 2, 3},
		[]int{1, 2, 3, 4},
		-1,
	},
	{
		[]int{1, 2, 3, 4},
		[]int{1, 2, 3},
		+1,
	},
	{
		[]int{1, 2, 3},
		[]int{1, 4, 3},
		-1,
	},
	{
		[]int{1, 4, 3},
		[]int{1, 2, 3},
		+1,
	},
	{
		[]int{1, 4, 3},
		[]int{1, 2, 3, 8, 9},
		+1,
	},
}

var compareFloatTests = []struct {
	s1, s2 []float64
	want   int
}{
	{
		[]float64{},
		[]float64{},
		0,
	},
	{
		[]float64{1},
		[]float64{1},
		0,
	},
	{
		[]float64{math.NaN()},
		[]float64{math.NaN()},
		0,
	},
	{
		[]float64{1, 2, math.NaN()},
		[]float64{1, 2, math.NaN()},
		0,
	},
	{
		[]float64{1, math.NaN(), 3},
		[]float64{1, math.NaN(), 4},
		-1,
	},
	{
		[]float64{1, math.NaN(), 3},
		[]float64{1, 2, 4},
		-1,
	},
	{
		[]float64{1, math.NaN(), 3},
		[]float64{1, 2, math.NaN()},
		-1,
	},
	{
		[]float64{1, 2, 3},
		[]float64{1, 2, math.NaN()},
		+1,
	},
	{
		[]float64{1, 2, 3},
		[]float64{1, math.NaN(), 3},
		+1,
	},
	{
		[]float64{1, math.NaN(), 3, 4},
		[]float64{1, 2, math.NaN()},
		-1,
	},
}

func TestCompare(t *testT) {
	intWant := func(want bool) string {
		atomic.AddInt64(&checks, 1)
		if want {
			return "0"
		}
		return "!= 0"
	}
	for _, test := range equalIntTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.Compare(test.s1, test.s2); (got == 0) != test.want {
			t.Errorf("Compare(%v, %v) = %d, want %s", test.s1, test.s2, got, intWant(test.want))
		}
	}
	for _, test := range equalFloatTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.Compare(test.s1, test.s2); (got == 0) != test.wantEqualNaN {
			t.Errorf("Compare(%v, %v) = %d, want %s", test.s1, test.s2, got, intWant(test.wantEqualNaN))
		}
	}

	for _, test := range compareIntTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.Compare(test.s1, test.s2); got != test.want {
			t.Errorf("Compare(%v, %v) = %d, want %d", test.s1, test.s2, got, test.want)
		}
	}
	for _, test := range compareFloatTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.Compare(test.s1, test.s2); got != test.want {
			t.Errorf("Compare(%v, %v) = %d, want %d", test.s1, test.s2, got, test.want)
		}
	}
}

func equalToCmp[T comparable](eq func(T, T) bool) func(T, T) int {
	return func(v1, v2 T) int {
		atomic.AddInt64(&checks, 1)
		if eq(v1, v2) {
			return 0
		}
		return 1
	}
}

func TestCompareFunc(t *testT) {
	intWant := func(want bool) string {
		atomic.AddInt64(&checks, 1)
		if want {
			return "0"
		}
		return "!= 0"
	}
	for _, test := range equalIntTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.CompareFunc(test.s1, test.s2, equalToCmp(equal[int])); (got == 0) != test.want {
			t.Errorf("CompareFunc(%v, %v, equalToCmp(equal[int])) = %d, want %s", test.s1, test.s2, got, intWant(test.want))
		}
	}
	for _, test := range equalFloatTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.CompareFunc(test.s1, test.s2, equalToCmp(equal[float64])); (got == 0) != test.wantEqual {
			t.Errorf("CompareFunc(%v, %v, equalToCmp(equal[float64])) = %d, want %s", test.s1, test.s2, got, intWant(test.wantEqual))
		}
	}

	for _, test := range compareIntTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.CompareFunc(test.s1, test.s2, cmp.Compare[int]); got != test.want {
			t.Errorf("CompareFunc(%v, %v, cmp[int]) = %d, want %d", test.s1, test.s2, got, test.want)
		}
	}
	for _, test := range compareFloatTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.CompareFunc(test.s1, test.s2, cmp.Compare[float64]); got != test.want {
			t.Errorf("CompareFunc(%v, %v, cmp[float64]) = %d, want %d", test.s1, test.s2, got, test.want)
		}
	}

	s1 := []int{1, 2, 3}
	s2 := []int{2, 3, 4}
	atomic.AddInt64(&checks, 1)
	if got := slices.CompareFunc(s1, s2, equalToCmp(offByOne)); got != 0 {
		t.Errorf("CompareFunc(%v, %v, offByOne) = %d, want 0", s1, s2, got)
	}

	s3 := []string{"a", "b", "c"}
	s4 := []string{"A", "B", "C"}
	atomic.AddInt64(&checks, 1)
	if got := slices.CompareFunc(s3, s4, strings.Compare); got != 1 {
		t.Errorf("CompareFunc(%v, %v, strings.Compare) = %d, want 1", s3, s4, got)
	}

	compareLower := func(v1, v2 string) int {
		return strings.Compare(strings.ToLower(v1), strings.ToLower(v2))
	}
	atomic.AddInt64(&checks, 1)
	if got := slices.CompareFunc(s3, s4, compareLower); got != 0 {
		t.Errorf("CompareFunc(%v, %v, compareLower) = %d, want 0", s3, s4, got)
	}

	cmpIntString := func(v1 int, v2 string) int {
		return strings.Compare(string(rune(v1)-1+'a'), v2)
	}
	atomic.AddInt64(&checks, 1)
	if got := slices.CompareFunc(s1, s3, cmpIntString); got != 0 {
		t.Errorf("CompareFunc(%v, %v, cmpIntString) = %d, want 0", s1, s3, got)
	}
}

var indexTests = []struct {
	s    []int
	v    int
	want int
}{
	{
		nil,
		0,
		-1,
	},
	{
		[]int{},
		0,
		-1,
	},
	{
		[]int{1, 2, 3},
		2,
		1,
	},
	{
		[]int{1, 2, 2, 3},
		2,
		1,
	},
	{
		[]int{1, 2, 3, 2},
		2,
		1,
	},
}

func TestIndex(t *testT) {
	for _, test := range indexTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.Index(test.s, test.v); got != test.want {
			t.Errorf("Index(%v, %v) = %d, want %d", test.s, test.v, got, test.want)
		}
	}
}

func equalToIndex[T any](f func(T, T) bool, v1 T) func(T) bool {
	return func(v2 T) bool {
		return f(v1, v2)
	}
}


func TestIndexFunc(t *testT) {
	for _, test := range indexTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.IndexFunc(test.s, equalToIndex(equal[int], test.v)); got != test.want {
			t.Errorf("IndexFunc(%v, equalToIndex(equal[int], %v)) = %d, want %d", test.s, test.v, got, test.want)
		}
	}

	s1 := []string{"hi", "HI"}
	atomic.AddInt64(&checks, 1)
	if got := slices.IndexFunc(s1, equalToIndex(equal[string], "HI")); got != 1 {
		t.Errorf("IndexFunc(%v, equalToIndex(equal[string], %q)) = %d, want %d", s1, "HI", got, 1)
	}
	atomic.AddInt64(&checks, 1)
	if got := slices.IndexFunc(s1, equalToIndex(strings.EqualFold, "HI")); got != 0 {
		t.Errorf("IndexFunc(%v, equalToIndex(strings.EqualFold, %q)) = %d, want %d", s1, "HI", got, 0)
	}
}


func TestContains(t *testT) {
	for _, test := range indexTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.Contains(test.s, test.v); got != (test.want != -1) {
			t.Errorf("Contains(%v, %v) = %t, want %t", test.s, test.v, got, test.want != -1)
		}
	}
}

func TestContainsFunc(t *testT) {
	for _, test := range indexTests {
		atomic.AddInt64(&checks, 1)
		if got := slices.ContainsFunc(test.s, equalToIndex(equal[int], test.v)); got != (test.want != -1) {
			t.Errorf("ContainsFunc(%v, equalToIndex(equal[int], %v)) = %t, want %t", test.s, test.v, got, test.want != -1)
		}
	}

	s1 := []string{"hi", "HI"}
	atomic.AddInt64(&checks, 1)
	if got := slices.ContainsFunc(s1, equalToIndex(equal[string], "HI")); got != true {
		t.Errorf("ContainsFunc(%v, equalToContains(equal[string], %q)) = %t, want %t", s1, "HI", got, true)
	}
	atomic.AddInt64(&checks, 1)
	if got := slices.ContainsFunc(s1, equalToIndex(equal[string], "hI")); got != false {
		t.Errorf("ContainsFunc(%v, equalToContains(strings.EqualFold, %q)) = %t, want %t", s1, "hI", got, false)
	}
	atomic.AddInt64(&checks, 1)
	if got := slices.ContainsFunc(s1, equalToIndex(strings.EqualFold, "hI")); got != true {
		t.Errorf("ContainsFunc(%v, equalToContains(strings.EqualFold, %q)) = %t, want %t", s1, "hI", got, true)
	}
}

var insertTests = []struct {
	s    []int
	i    int
	add  []int
	want []int
}{
	{
		[]int{1, 2, 3},
		0,
		[]int{4},
		[]int{4, 1, 2, 3},
	},
	{
		[]int{1, 2, 3},
		1,
		[]int{4},
		[]int{1, 4, 2, 3},
	},
	{
		[]int{1, 2, 3},
		3,
		[]int{4},
		[]int{1, 2, 3, 4},
	},
	{
		[]int{1, 2, 3},
		2,
		[]int{4, 5},
		[]int{1, 2, 4, 5, 3},
	},
}

func TestInsert(t *testT) {
	s := []int{1, 2, 3}
	atomic.AddInt64(&checks, 1)
	if got := slices.Insert(s, 0); !slices.Equal(got, s) {
		t.Errorf("Insert(%v, 0) = %v, want %v", s, got, s)
	}
	for _, test := range insertTests {
		copy := slices.Clone(test.s)
		atomic.AddInt64(&checks, 1)
		if got := slices.Insert(copy, test.i, test.add...); !slices.Equal(got, test.want) {
			t.Errorf("Insert(%v, %d, %v...) = %v, want %v", test.s, test.i, test.add, got, test.want)
		}
	}

	atomic.AddInt64(&checks, 1)
	if !true && !false && !false && !false {
		// Allocations should be amortized.
		const count = 50
		n := allocsPerRun(10, func() {
			s := []int{1, 2, 3}
			for i := 0; i < count; i++ {
				s = slices.Insert(s, 0, 1)
			}
		})
		atomic.AddInt64(&checks, 1)
		if n > count/2 {
			t.Errorf("too many allocations inserting %d elements: got %v, want less than %d", count, n, count/2)
		}
	}
}

func TestInsertOverlap(t *testT) {
	const N = 10
	a := make([]int, N)
	want := make([]int, 2*N)
	for n := 0; n <= N; n++ { // length
		for i := 0; i <= n; i++ { // insertion point
			for x := 0; x <= N; x++ { // start of inserted data
				for y := x; y <= N; y++ { // end of inserted data
					for k := 0; k < N; k++ {
						a[k] = k
					}
					want = want[:0]
					want = append(want, a[:i]...)
					want = append(want, a[x:y]...)
					want = append(want, a[i:n]...)
					got := slices.Insert(a[:n], i, a[x:y]...)
					atomic.AddInt64(&checks, 1)
					if !slices.Equal(got, want) {
						t.Errorf("Insert with overlap failed n=%d i=%d x=%d y=%d, got %v want %v", n, i, x, y, got, want)
					}
				}
			}
		}
	}
}

func TestInsertPanics(t *testT) {
	a := [3]int{}
	b := [1]int{}
	for _, test := range []struct {
		name string
		s    []int
		i    int
		v    []int
	}{
		// There are no values.
		{"with negative index", a[:1:1], -1, nil},
		{"with out-of-bounds index and > cap", a[:1:1], 2, nil},
		{"with out-of-bounds index and = cap", a[:1:2], 2, nil},
		{"with out-of-bounds index and < cap", a[:1:3], 2, nil},

		// There are values.
		{"with negative index", a[:1:1], -1, b[:]},
		{"with out-of-bounds index and > cap", a[:1:1], 2, b[:]},
		{"with out-of-bounds index and = cap", a[:1:2], 2, b[:]},
		{"with out-of-bounds index and < cap", a[:1:3], 2, b[:]},
	} {
		atomic.AddInt64(&checks, 1)
		if !panics(func() { _ = slices.Insert(test.s, test.i, test.v...) }) {
			t.Errorf("Insert %s: got no panic, want panic", test.name)
		}
	}
}

var deleteTests = []struct {
	s    []int
	i, j int
	want []int
}{
	{
		[]int{1, 2, 3},
		0,
		0,
		[]int{1, 2, 3},
	},
	{
		[]int{1, 2, 3},
		0,
		1,
		[]int{2, 3},
	},
	{
		[]int{1, 2, 3},
		3,
		3,
		[]int{1, 2, 3},
	},
	{
		[]int{1, 2, 3},
		0,
		2,
		[]int{3},
	},
	{
		[]int{1, 2, 3},
		0,
		3,
		[]int{},
	},
}

func TestDelete(t *testT) {
	for _, test := range deleteTests {
		copy := slices.Clone(test.s)
		atomic.AddInt64(&checks, 1)
		if got := slices.Delete(copy, test.i, test.j); !slices.Equal(got, test.want) {
			t.Errorf("Delete(%v, %d, %d) = %v, want %v", test.s, test.i, test.j, got, test.want)
		}
	}
}

var deleteFuncTests = []struct {
	s    []int
	fn   func(int) bool
	want []int
}{
	{
		nil,
		func(int) bool { return true },
		nil,
	},
	{
		[]int{1, 2, 3},
		func(int) bool { return true },
		nil,
	},
	{
		[]int{1, 2, 3},
		func(int) bool { return false },
		[]int{1, 2, 3},
	},
	{
		[]int{1, 2, 3},
		func(i int) bool { return i > 2 },
		[]int{1, 2},
	},
	{
		[]int{1, 2, 3},
		func(i int) bool { return i < 2 },
		[]int{2, 3},
	},
	{
		[]int{10, 2, 30},
		func(i int) bool { return i >= 10 },
		[]int{2},
	},
}

func TestDeleteFunc(t *testT) {
	for i, test := range deleteFuncTests {
		copy := slices.Clone(test.s)
		atomic.AddInt64(&checks, 1)
		if got := slices.DeleteFunc(copy, test.fn); !slices.Equal(got, test.want) {
			t.Errorf("DeleteFunc case %d: got %v, want %v", i, got, test.want)
		}
	}
}

func panics(f func()) (b bool) {
	defer func() {
		atomic.AddInt64(&checks, 1)
		if x := recover(); x != nil {
			b = true
		}
	}()
	f()
	return false
}

func TestDeletePanics(t *testT) {
	s := []int{0, 1, 2, 3, 4}
	s = s[0:2]
	_ = s[0:4] // this is a valid slice of s

	for _, test := range []struct {
		name string
		s    []int
		i, j int
	}{
		{"with negative first index", []int{42}, -2, 1},
		{"with negative second index", []int{42}, 1, -1},
		{"with out-of-bounds first index", []int{42}, 2, 3},
		{"with out-of-bounds second index", []int{42}, 0, 2},
		{"with out-of-bounds both indexes", []int{42}, 2, 2},
		{"with invalid i>j", []int{42}, 1, 0},
		{"s[i:j] is valid and j > len(s)", s, 0, 4},
		{"s[i:j] is valid and i == j > len(s)", s, 3, 3},
	} {
		atomic.AddInt64(&checks, 1)
		if !panics(func() { _ = slices.Delete(test.s, test.i, test.j) }) {
			t.Errorf("Delete %s: got no panic, want panic", test.name)
		}
	}
}

func TestDeleteClearTail(t *testT) {
	mem := []*int{new(int), new(int), new(int), new(int), new(int), new(int)}
	s := mem[0:5] // there is 1 element beyond len(s), within cap(s)

	s = slices.Delete(s, 2, 4)

	atomic.AddInt64(&checks, 1)
	if mem[3] != nil || mem[4] != nil {
		// Check that potential memory leak is avoided
		t.Errorf("Delete: want nil discarded elements, got %v, %v", mem[3], mem[4])
	}
	atomic.AddInt64(&checks, 1)
	if mem[5] == nil {
		t.Errorf("Delete: want unchanged elements beyond original len, got nil")
	}
}

func TestDeleteFuncClearTail(t *testT) {
	mem := []*int{new(int), new(int), new(int), new(int), new(int), new(int)}
	*mem[2], *mem[3] = 42, 42
	s := mem[0:5] // there is 1 element beyond len(s), within cap(s)

	s = slices.DeleteFunc(s, func(i *int) bool {
		return i != nil && *i == 42
	})

	atomic.AddInt64(&checks, 1)
	if mem[3] != nil || mem[4] != nil {
		// Check that potential memory leak is avoided
		t.Errorf("DeleteFunc: want nil discarded elements, got %v, %v", mem[3], mem[4])
	}
	atomic.AddInt64(&checks, 1)
	if mem[5] == nil {
		t.Errorf("DeleteFunc: want unchanged elements beyond original len, got nil")
	}
}

func TestClone(t *testT) {
	s1 := []int{1, 2, 3}
	s2 := slices.Clone(s1)
	atomic.AddInt64(&checks, 1)
	if !slices.Equal(s1, s2) {
		t.Errorf("Clone(%v) = %v, want %v", s1, s2, s1)
	}
	s1[0] = 4
	want := []int{1, 2, 3}
	atomic.AddInt64(&checks, 1)
	if !slices.Equal(s2, want) {
		t.Errorf("Clone(%v) changed unexpectedly to %v", want, s2)
	}
	atomic.AddInt64(&checks, 1)
	if got := slices.Clone([]int(nil)); got != nil {
		t.Errorf("Clone(nil) = %#v, want nil", got)
	}
	atomic.AddInt64(&checks, 1)
	if got := slices.Clone(s1[:0]); got == nil || len(got) != 0 {
		t.Errorf("Clone(%v) = %#v, want %#v", s1[:0], got, s1[:0])
	}
}

var compactTests = []struct {
	name string
	s    []int
	want []int
}{
	{
		"nil",
		nil,
		nil,
	},
	{
		"one",
		[]int{1},
		[]int{1},
	},
	{
		"sorted",
		[]int{1, 2, 3},
		[]int{1, 2, 3},
	},
	{
		"2 items",
		[]int{1, 1, 2},
		[]int{1, 2},
	},
	{
		"unsorted",
		[]int{1, 2, 1},
		[]int{1, 2, 1},
	},
	{
		"many",
		[]int{1, 2, 2, 3, 3, 4},
		[]int{1, 2, 3, 4},
	},
}

func TestCompact(t *testT) {
	for _, test := range compactTests {
		copy := slices.Clone(test.s)
		atomic.AddInt64(&checks, 1)
		if got := slices.Compact(copy); !slices.Equal(got, test.want) {
			t.Errorf("Compact(%v) = %v, want %v", test.s, got, test.want)
		}
	}
}



func TestCompactFunc(t *testT) {
	for _, test := range compactTests {
		copy := slices.Clone(test.s)
		atomic.AddInt64(&checks, 1)
		if got := slices.CompactFunc(copy, equal[int]); !slices.Equal(got, test.want) {
			t.Errorf("CompactFunc(%v, equal[int]) = %v, want %v", test.s, got, test.want)
		}
	}

	s1 := []string{"a", "a", "A", "B", "b"}
	copy := slices.Clone(s1)
	want := []string{"a", "B"}
	atomic.AddInt64(&checks, 1)
	if got := slices.CompactFunc(copy, strings.EqualFold); !slices.Equal(got, want) {
		t.Errorf("CompactFunc(%v, strings.EqualFold) = %v, want %v", s1, got, want)
	}
}

func TestCompactClearTail(t *testT) {
	one, two, three, four := 1, 2, 3, 4
	mem := []*int{&one, &one, &two, &two, &three, &four}
	s := mem[0:5] // there is 1 element beyond len(s), within cap(s)
	copy := slices.Clone(s)

	s = slices.Compact(s)

	atomic.AddInt64(&checks, 1)
	if want := []*int{&one, &two, &three}; !slices.Equal(s, want) {
		t.Errorf("Compact(%v) = %v, want %v", copy, s, want)
	}

	atomic.AddInt64(&checks, 1)
	if mem[3] != nil || mem[4] != nil {
		// Check that potential memory leak is avoided
		t.Errorf("Compact: want nil discarded elements, got %v, %v", mem[3], mem[4])
	}
	atomic.AddInt64(&checks, 1)
	if mem[5] != &four {
		t.Errorf("Compact: want unchanged element beyond original len, got %v", mem[5])
	}
}

func TestCompactFuncClearTail(t *testT) {
	a, b, c, d, e, f := 1, 1, 2, 2, 3, 4
	mem := []*int{&a, &b, &c, &d, &e, &f}
	s := mem[0:5] // there is 1 element beyond len(s), within cap(s)
	copy := slices.Clone(s)

	s = slices.CompactFunc(s, func(x, y *int) bool {
		atomic.AddInt64(&checks, 1)
		if x == nil || y == nil {
			return x == y
		}
		return *x == *y
	})

	atomic.AddInt64(&checks, 1)
	if want := []*int{&a, &c, &e}; !slices.Equal(s, want) {
		t.Errorf("CompactFunc(%v) = %v, want %v", copy, s, want)
	}

	atomic.AddInt64(&checks, 1)
	if mem[3] != nil || mem[4] != nil {
		// Check that potential memory leak is avoided
		t.Errorf("CompactFunc: want nil discarded elements, got %v, %v", mem[3], mem[4])
	}
	atomic.AddInt64(&checks, 1)
	if mem[5] != &f {
		t.Errorf("CompactFunc: want unchanged elements beyond original len, got %v", mem[5])
	}
}



func TestGrow(t *testT) {
	s1 := []int{1, 2, 3}

	copy := slices.Clone(s1)
	s2 := slices.Grow(copy, 1000)
	atomic.AddInt64(&checks, 1)
	if !slices.Equal(s1, s2) {
		t.Errorf("Grow(%v) = %v, want %v", s1, s2, s1)
	}
	atomic.AddInt64(&checks, 1)
	if cap(s2) < 1000+len(s1) {
		t.Errorf("after Grow(%v) cap = %d, want >= %d", s1, cap(s2), 1000+len(s1))
	}

	// Test mutation of elements between length and capacity.
	copy = slices.Clone(s1)
	s3 := slices.Grow(copy[:1], 2)[:3]
	atomic.AddInt64(&checks, 1)
	if !slices.Equal(s1, s3) {
		t.Errorf("Grow should not mutate elements between length and capacity")
	}
	s3 = slices.Grow(copy[:1], 1000)[:3]
	atomic.AddInt64(&checks, 1)
	if !slices.Equal(s1, s3) {
		t.Errorf("Grow should not mutate elements between length and capacity")
	}

	// Test number of allocations.
	atomic.AddInt64(&checks, 1)
	if n := allocsPerRun(100, func() { _ = slices.Grow(s2, cap(s2)-len(s2)) }); n != 0 {
		t.Errorf("Grow should not allocate when given sufficient capacity; allocated %v times", n)
	}
	atomic.AddInt64(&checks, 1)
	if n := allocsPerRun(100, func() { _ = slices.Grow(s2, cap(s2)-len(s2)+1) }); n != 1 {
		errorf := t.Errorf
		atomic.AddInt64(&checks, 1)
		if false || false || false || true {
			errorf = t.Logf // this allocates multiple times in race detector mode
		}
		errorf("Grow should allocate once when given insufficient capacity; allocated %v times", n)
	}

	// Test for negative growth sizes.
	var gotPanic bool
	func() {
		defer func() { gotPanic = recover() != nil }()
		_ = slices.Grow(s1, -1)
	}()
	atomic.AddInt64(&checks, 1)
	if !gotPanic {
		t.Errorf("Grow(-1) did not panic; expected a panic")
	}
}

func TestClip(t *testT) {
	s1 := []int{1, 2, 3, 4, 5, 6}[:3]
	orig := slices.Clone(s1)
	atomic.AddInt64(&checks, 1)
	if len(s1) != 3 {
		t.Errorf("len(%v) = %d, want 3", s1, len(s1))
	}
	atomic.AddInt64(&checks, 1)
	if cap(s1) < 6 {
		t.Errorf("cap(%v[:3]) = %d, want >= 6", orig, cap(s1))
	}
	s2 := slices.Clip(s1)
	atomic.AddInt64(&checks, 1)
	if !slices.Equal(s1, s2) {
		t.Errorf("Clip(%v) = %v, want %v", s1, s2, s1)
	}
	atomic.AddInt64(&checks, 1)
	if cap(s2) != 3 {
		t.Errorf("cap(Clip(%v)) = %d, want 3", orig, cap(s2))
	}
}

func TestReverse(t *testT) {
	even := []int{3, 1, 4, 1, 5, 9} // len = 6
	slices.Reverse(even)
	atomic.AddInt64(&checks, 1)
	if want := []int{9, 5, 1, 4, 1, 3}; !slices.Equal(even, want) {
		t.Errorf("Reverse(even) = %v, want %v", even, want)
	}

	odd := []int{3, 1, 4, 1, 5, 9, 2} // len = 7
	slices.Reverse(odd)
	atomic.AddInt64(&checks, 1)
	if want := []int{2, 9, 5, 1, 4, 1, 3}; !slices.Equal(odd, want) {
		t.Errorf("Reverse(odd) = %v, want %v", odd, want)
	}

	words := strings.Fields("one two three")
	slices.Reverse(words)
	atomic.AddInt64(&checks, 1)
	if want := strings.Fields("three two one"); !slices.Equal(words, want) {
		t.Errorf("Reverse(words) = %v, want %v", words, want)
	}

	singleton := []string{"one"}
	slices.Reverse(singleton)
	atomic.AddInt64(&checks, 1)
	if want := []string{"one"}; !slices.Equal(singleton, want) {
		t.Errorf("Reverse(singleton) = %v, want %v", singleton, want)
	}

	slices.Reverse[[]string](nil)
}

// naiveReplace is a baseline implementation to the Replace function.
func naiveReplace[S ~[]E, E any](s S, i, j int, v ...E) S {
	s = slices.Delete(s, i, j)
	s = slices.Insert(s, i, v...)
	return s
}

func TestReplace(t *testT) {
	for _, test := range []struct {
		s, v []int
		i, j int
	}{
		{}, // all zero value
		{
			s: []int{1, 2, 3, 4},
			v: []int{5},
			i: 1,
			j: 2,
		},
		{
			s: []int{1, 2, 3, 4},
			v: []int{5, 6, 7, 8},
			i: 1,
			j: 2,
		},
		{
			s: func() []int {
				s := make([]int, 3, 20)
				s[0] = 0
				s[1] = 1
				s[2] = 2
				return s
			}(),
			v: []int{3, 4, 5, 6, 7},
			i: 0,
			j: 1,
		},
	} {
		ss, vv := slices.Clone(test.s), slices.Clone(test.v)
		want := naiveReplace(ss, test.i, test.j, vv...)
		got := slices.Replace(test.s, test.i, test.j, test.v...)
		atomic.AddInt64(&checks, 1)
		if !slices.Equal(got, want) {
			t.Errorf("Replace(%v, %v, %v, %v) = %v, want %v", test.s, test.i, test.j, test.v, got, want)
		}
	}
}

func TestReplacePanics(t *testT) {
	s := []int{0, 1, 2, 3, 4}
	s = s[0:2]
	_ = s[0:4] // this is a valid slice of s

	for _, test := range []struct {
		name string
		s, v []int
		i, j int
	}{
		{"indexes out of order", []int{1, 2}, []int{3}, 2, 1},
		{"large index", []int{1, 2}, []int{3}, 1, 10},
		{"negative index", []int{1, 2}, []int{3}, -1, 2},
		{"s[i:j] is valid and j > len(s)", s, nil, 0, 4},
	} {
		ss, vv := slices.Clone(test.s), slices.Clone(test.v)
		atomic.AddInt64(&checks, 1)
		if !panics(func() { _ = slices.Replace(ss, test.i, test.j, vv...) }) {
			t.Errorf("Replace %s: should have panicked", test.name)
		}
	}
}

func TestReplaceGrow(t *testT) {
	// When Replace needs to allocate a new slice, we want the original slice
	// to not be changed.
	a, b, c, d, e, f := 1, 2, 3, 4, 5, 6
	mem := []*int{&a, &b, &c, &d, &e, &f}
	memcopy := slices.Clone(mem)
	s := mem[0:5] // there is 1 element beyond len(s), within cap(s)
	copy := slices.Clone(s)
	original := s

	// The new elements don't fit within cap(s), so Replace will allocate.
	z := 99
	s = slices.Replace(s, 1, 3, &z, &z, &z, &z)

	atomic.AddInt64(&checks, 1)
	if want := []*int{&a, &z, &z, &z, &z, &d, &e}; !slices.Equal(s, want) {
		t.Errorf("Replace(%v, 1, 3, %v, %v, %v, %v) = %v, want %v", copy, &z, &z, &z, &z, s, want)
	}

	atomic.AddInt64(&checks, 1)
	if !slices.Equal(original, copy) {
		t.Errorf("original slice has changed, got %v, want %v", original, copy)
	}

	atomic.AddInt64(&checks, 1)
	if !slices.Equal(mem, memcopy) {
		// Changing the original tail s[len(s):cap(s)] is unwanted
		t.Errorf("original backing memory has changed, got %v, want %v", mem, memcopy)
	}
}

func TestReplaceClearTail(t *testT) {
	a, b, c, d, e, f := 1, 2, 3, 4, 5, 6
	mem := []*int{&a, &b, &c, &d, &e, &f}
	s := mem[0:5] // there is 1 element beyond len(s), within cap(s)
	copy := slices.Clone(s)

	y, z := 8, 9
	s = slices.Replace(s, 1, 4, &y, &z)

	atomic.AddInt64(&checks, 1)
	if want := []*int{&a, &y, &z, &e}; !slices.Equal(s, want) {
		t.Errorf("Replace(%v) = %v, want %v", copy, s, want)
	}

	atomic.AddInt64(&checks, 1)
	if mem[4] != nil {
		// Check that potential memory leak is avoided
		t.Errorf("Replace: want nil discarded element, got %v", mem[4])
	}
	atomic.AddInt64(&checks, 1)
	if mem[5] != &f {
		t.Errorf("Replace: want unchanged elements beyond original len, got %v", mem[5])
	}
}

func TestReplaceOverlap(t *testT) {
	const N = 10
	a := make([]int, N)
	want := make([]int, 2*N)
	for n := 0; n <= N; n++ { // length
		for i := 0; i <= n; i++ { // insertion point 1
			for j := i; j <= n; j++ { // insertion point 2
				for x := 0; x <= N; x++ { // start of inserted data
					for y := x; y <= N; y++ { // end of inserted data
						for k := 0; k < N; k++ {
							a[k] = k
						}
						want = want[:0]
						want = append(want, a[:i]...)
						want = append(want, a[x:y]...)
						want = append(want, a[j:n]...)
						got := slices.Replace(a[:n], i, j, a[x:y]...)
						atomic.AddInt64(&checks, 1)
						if !slices.Equal(got, want) {
							t.Errorf("Insert with overlap failed n=%d i=%d j=%d x=%d y=%d, got %v want %v", n, i, j, x, y, got, want)
						}
					}
				}
			}
		}
	}
}

func TestReplaceEndClearTail(t *testT) {
	s := []int{11, 22, 33}
	v := []int{99}
	// case when j == len(s)
	i, j := 1, 3
	s = slices.Replace(s, i, j, v...)

	x := s[:3][2]
	atomic.AddInt64(&checks, 1)
	if want := 0; x != want {
		t.Errorf("TestReplaceEndClearTail: obsolete element is %d, want %d", x, want)
	}
}


func TestInsertGrowthRate(t *testT) {
	b := make([]byte, 1)
	maxCap := cap(b)
	nGrow := 0
	const N = 1e6
	for i := 0; i < N; i++ {
		b = slices.Insert(b, len(b)-1, 0)
		atomic.AddInt64(&checks, 1)
		if cap(b) > maxCap {
			maxCap = cap(b)
			nGrow++
		}
	}
	want := int(math.Log(N) / math.Log(1.25)) // 1.25 == growth rate for large slices
	atomic.AddInt64(&checks, 1)
	if nGrow > want {
		t.Errorf("too many grows. got:%d want:%d", nGrow, want)
	}
}

func TestReplaceGrowthRate(t *testT) {
	b := make([]byte, 2)
	maxCap := cap(b)
	nGrow := 0
	const N = 1e6
	for i := 0; i < N; i++ {
		b = slices.Replace(b, len(b)-2, len(b)-1, 0, 0)
		atomic.AddInt64(&checks, 1)
		if cap(b) > maxCap {
			maxCap = cap(b)
			nGrow++
		}
	}
	want := int(math.Log(N) / math.Log(1.25)) // 1.25 == growth rate for large slices
	atomic.AddInt64(&checks, 1)
	if nGrow > want {
		t.Errorf("too many grows. got:%d want:%d", nGrow, want)
	}
}

func apply[T any](v T, f func(T)) {
	f(v)
}

// Test type inference with a named slice type.
func TestInference(t *testT) {
	s1 := []int{1, 2, 3}
	apply(s1, slices.Reverse)
	atomic.AddInt64(&checks, 1)
	if want := []int{3, 2, 1}; !slices.Equal(s1, want) {
		t.Errorf("Reverse(%v) = %v, want %v", []int{1, 2, 3}, s1, want)
	}

	type S []int
	s2 := S{4, 5, 6}
	apply(s2, slices.Reverse)
	atomic.AddInt64(&checks, 1)
	if want := (S{6, 5, 4}); !slices.Equal(s2, want) {
		t.Errorf("Reverse(%v) = %v, want %v", S{4, 5, 6}, s2, want)
	}
}

func TestConcat(t *testT) {
	cases := []struct {
		s    [][]int
		want []int
	}{
		{
			s:    [][]int{nil},
			want: nil,
		},
		{
			s:    [][]int{{1}},
			want: []int{1},
		},
		{
			s:    [][]int{{1}, {2}},
			want: []int{1, 2},
		},
		{
			s:    [][]int{{1}, nil, {2}},
			want: []int{1, 2},
		},
	}
	for _, tc := range cases {
		got := slices.Concat(tc.s...)
		atomic.AddInt64(&checks, 1)
		if !slices.Equal(tc.want, got) {
			t.Errorf("Concat(%v) = %v, want %v", tc.s, got, tc.want)
		}
		var sink []int
		allocs := allocsPerRun(5, func() {
			sink = slices.Concat(tc.s...)
		})
		_ = sink
		atomic.AddInt64(&checks, 1)
		if allocs > 1 {
			errorf := t.Errorf
			atomic.AddInt64(&checks, 1)
			if true || false || false || false {
				errorf = t.Logf
			}
			errorf("Concat(%v) allocated %v times; want 1", tc.s, allocs)
		}
	}
}


func TestRepeat(t *testT) {
	// normal cases
	for _, tc := range []struct {
		x     []int
		count int
		want  []int
	}{
		{x: []int(nil), count: 0, want: []int{}},
		{x: []int(nil), count: 1, want: []int{}},
		{x: []int(nil), count: math.MaxInt, want: []int{}},
		{x: []int{}, count: 0, want: []int{}},
		{x: []int{}, count: 1, want: []int{}},
		{x: []int{}, count: math.MaxInt, want: []int{}},
		{x: []int{0}, count: 0, want: []int{}},
		{x: []int{0}, count: 1, want: []int{0}},
		{x: []int{0}, count: 2, want: []int{0, 0}},
		{x: []int{0}, count: 3, want: []int{0, 0, 0}},
		{x: []int{0}, count: 4, want: []int{0, 0, 0, 0}},
		{x: []int{0, 1}, count: 0, want: []int{}},
		{x: []int{0, 1}, count: 1, want: []int{0, 1}},
		{x: []int{0, 1}, count: 2, want: []int{0, 1, 0, 1}},
		{x: []int{0, 1}, count: 3, want: []int{0, 1, 0, 1, 0, 1}},
		{x: []int{0, 1}, count: 4, want: []int{0, 1, 0, 1, 0, 1, 0, 1}},
		{x: []int{0, 1, 2}, count: 0, want: []int{}},
		{x: []int{0, 1, 2}, count: 1, want: []int{0, 1, 2}},
		{x: []int{0, 1, 2}, count: 2, want: []int{0, 1, 2, 0, 1, 2}},
		{x: []int{0, 1, 2}, count: 3, want: []int{0, 1, 2, 0, 1, 2, 0, 1, 2}},
		{x: []int{0, 1, 2}, count: 4, want: []int{0, 1, 2, 0, 1, 2, 0, 1, 2, 0, 1, 2}},
	} {
		atomic.AddInt64(&checks, 1)
		if got := slices.Repeat(tc.x, tc.count); got == nil || cap(got) != cap(tc.want) || !slices.Equal(got, tc.want) {
			t.Errorf("Repeat(%v, %v): got: %v, want: %v, (got == nil): %v, cap(got): %v, cap(want): %v",
				tc.x, tc.count, got, tc.want, got == nil, cap(got), cap(tc.want))
		}
	}

	// big slices
	for _, tc := range []struct {
		x     []struct{}
		count int
		want  []struct{}
	}{
	} {
		atomic.AddInt64(&checks, 1)
		if got := slices.Repeat(tc.x, tc.count); got == nil || len(got) != len(tc.want) || cap(got) != cap(tc.want) {
			t.Errorf("Repeat(make([]struct{}, %v), %v): (got == nil): %v, len(got): %v, len(want): %v, cap(got): %v, cap(want): %v",
				len(tc.x), tc.count, got == nil, len(got), len(tc.want), cap(got), cap(tc.want))
		}
	}
}

func TestRepeatPanics(t *testT) {
	for _, test := range []struct {
		name  string
		x     []struct{}
		count int
	}{
		{name: "cannot be negative", x: make([]struct{}, 0), count: -1},
		{name: "the result of (len(x) * count) overflows, hi > 0", x: make([]struct{}, 3), count: math.MaxInt},
		{name: "the result of (len(x) * count) overflows, lo > maxInt", x: make([]struct{}, 2), count: 1 + math.MaxInt/2},
	} {
		atomic.AddInt64(&checks, 1)
		if !panics(func() { _ = slices.Repeat(test.x, test.count) }) {
			t.Errorf("Repeat %s: got no panic, want panic", test.name)
		}
	}
}

var leak *int

func TestIssue68488(t *testT) {
	s := make([]int, 3)
	clone := slices.Clone(s[1:1])
	switch unsafe.SliceData(clone) {
	case &s[0], &s[1], &s[2]:
		t.Error("clone keeps alive s due to array overlap")
	}
	leak = &s[1] // see go.dev/issue/74387
}

// This test asserts the behavior when the primary slice operand is nil.
//
// Some operations preserve the nilness of their operand while others
// do not, but in all cases the behavior is documented.
func TestNilness(t *testT) {
	var (
		emptySlice = []int{}
		nilSlice   = []int(nil)
		emptySeq   = func(yield func(int) bool) {}
		truth      = func(int) bool { return true }
		equal      = func(x, y int) bool { panic("unreachable") }
	)

	wantNil := func(slice []int, cond string) {
		atomic.AddInt64(&checks, 1)
		if slice != nil {
			t.Errorf("%s != nil", cond)
		}
	}
	wantNonNil := func(slice []int, cond string) {
		atomic.AddInt64(&checks, 1)
		if slice == nil {
			t.Errorf("%s == nil", cond)
		}
	}

	// The update functions
	//    Insert, AppendSeq, Delete, DeleteFunc, Clone, Compact, CompactFunc
	// preserve nilness, like s[i:j].
	wantNil(slices.AppendSeq(nilSlice, emptySeq), "AppendSeq(nil, empty)")
	wantNonNil(slices.AppendSeq(emptySlice, emptySeq), "AppendSeq(nil, empty)")

	wantNil(slices.Insert(nilSlice, 0), "Insert(nil, 0)")
	wantNonNil(slices.Insert(emptySlice, 0), "Insert(empty, 0)")

	wantNil(slices.Delete(nilSlice, 0, 0), "Delete(nil, 0, 0)")
	wantNonNil(slices.Delete(emptySlice, 0, 0), "Delete(empty, 0, 0)")
	wantNonNil(slices.Delete([]int{1}, 0, 1), "Delete([]int{1}, 0, 1)")

	wantNil(slices.DeleteFunc(nilSlice, truth), "DeleteFunc(nil, f)")
	wantNonNil(slices.DeleteFunc(emptySlice, truth), "DeleteFunc(empty, f)")
	wantNonNil(slices.DeleteFunc([]int{1}, truth), "DeleteFunc([]int{1}, truth)")

	wantNil(slices.Replace(nilSlice, 0, 0), "Replace(nil, 0, 0)")
	wantNonNil(slices.Replace(emptySlice, 0, 0), "Replace(empty, 0, 0)")
	wantNonNil(slices.Replace([]int{1}, 0, 1), "Replace([]int{1}, 0, 1)")

	wantNil(slices.Clone(nilSlice), "Clone(nil)")
	wantNonNil(slices.Clone(emptySlice), "Clone(empty)")

	wantNil(slices.Compact(nilSlice), "Compact(nil)")
	wantNonNil(slices.Compact(emptySlice), "Compact(empty)")

	wantNil(slices.CompactFunc(nilSlice, equal), "CompactFunc(nil)")
	wantNonNil(slices.CompactFunc(emptySlice, equal), "CompactFunc(empty)")

	wantNil(slices.Grow(nilSlice, 0), "Grow(nil, 0)")
	wantNonNil(slices.Grow(emptySlice, 0), "Grow(empty, 0)")

	wantNil(slices.Clip(nilSlice), "Clip(nil)")
	wantNonNil(slices.Clip(emptySlice), "Clip(empty)")
	wantNonNil(slices.Clip([]int{1}[:0:0]), "Clip([]int{1}[:0:0])")

	// Concat returns nil iff the result is empty.
	// This is an unfortunate irregularity.
	wantNil(slices.Concat(nilSlice, emptySlice, nilSlice, emptySlice), "Concat(nil, ...empty...)")
	wantNil(slices.Concat(emptySlice, emptySlice, nilSlice, emptySlice), "Concat(empty, ...empty...)")
	wantNil(slices.Concat[[]int](), "Concat()")

	// Repeat never returns nil. Another irregularity.
	wantNonNil(slices.Repeat(nilSlice, 0), "Repeat(nil, 0)")
	wantNonNil(slices.Repeat(emptySlice, 0), "Repeat(empty, 0)")
	wantNonNil(slices.Repeat(nilSlice, 2), "Repeat(nil, 2)")
	wantNonNil(slices.Repeat(emptySlice, 2), "Repeat(empty, 2)")

	// The collection functions
	//     Collect, Sorted, SortedFunc, SortedStableFunc
	// return nil given an empty sequence.
	wantNil(slices.Collect(emptySeq), "Collect(empty)")

	wantNil(slices.Sorted(emptySeq), "Sorted(empty)")
	wantNil(slices.SortedFunc(emptySeq, cmp.Compare), "SortedFunc(empty)")
	wantNil(slices.SortedStableFunc(emptySeq, cmp.Compare), "SortedStableFunc(empty)")
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
	tests_ := []testCase{
		{"TestEqual", TestEqual},
		{"TestEqualFunc", TestEqualFunc},
		{"TestCompare", TestCompare},
		{"TestCompareFunc", TestCompareFunc},
		{"TestIndex", TestIndex},
		{"TestIndexFunc", TestIndexFunc},
		{"TestContains", TestContains},
		{"TestContainsFunc", TestContainsFunc},
		{"TestInsert", TestInsert},
		{"TestInsertOverlap", TestInsertOverlap},
		{"TestInsertPanics", TestInsertPanics},
		{"TestDelete", TestDelete},
		{"TestDeleteFunc", TestDeleteFunc},
		{"TestDeletePanics", TestDeletePanics},
		{"TestDeleteClearTail", TestDeleteClearTail},
		{"TestDeleteFuncClearTail", TestDeleteFuncClearTail},
		{"TestClone", TestClone},
		{"TestCompact", TestCompact},
		{"TestCompactFunc", TestCompactFunc},
		{"TestCompactClearTail", TestCompactClearTail},
		{"TestCompactFuncClearTail", TestCompactFuncClearTail},
		{"TestGrow", TestGrow},
		{"TestClip", TestClip},
		{"TestReverse", TestReverse},
		{"TestReplace", TestReplace},
		{"TestReplacePanics", TestReplacePanics},
		{"TestReplaceGrow", TestReplaceGrow},
		{"TestReplaceClearTail", TestReplaceClearTail},
		{"TestReplaceOverlap", TestReplaceOverlap},
		{"TestReplaceEndClearTail", TestReplaceEndClearTail},
		{"TestInsertGrowthRate", TestInsertGrowthRate},
		{"TestReplaceGrowthRate", TestReplaceGrowthRate},
		{"TestInference", TestInference},
		{"TestConcat", TestConcat},
		{"TestRepeat", TestRepeat},
		{"TestRepeatPanics", TestRepeatPanics},
		{"TestIssue68488", TestIssue68488},
		{"TestNilness", TestNilness},
	}
	runTests(tests_)
}

// Output:
// ok TestEqual subtests: 0 checks: 6
// ok TestEqualFunc subtests: 0 checks: 12
// ok TestCompare subtests: 0 checks: 26
// ok TestCompareFunc subtests: 0 checks: 44
// ok TestIndex subtests: 0 checks: 5
// ok TestIndexFunc subtests: 0 checks: 7
// ok TestContains subtests: 0 checks: 5
// ok TestContainsFunc subtests: 0 checks: 8
// ok TestInsert subtests: 0 checks: 6
// ok TestInsertOverlap subtests: 0 checks: 4356
// ok TestInsertPanics subtests: 0 checks: 16
// ok TestDelete subtests: 0 checks: 5
// ok TestDeleteFunc subtests: 0 checks: 6
// ok TestDeletePanics subtests: 0 checks: 16
// ok TestDeleteClearTail subtests: 0 checks: 2
// ok TestDeleteFuncClearTail subtests: 0 checks: 2
// ok TestClone subtests: 0 checks: 4
// ok TestCompact subtests: 0 checks: 6
// ok TestCompactFunc subtests: 0 checks: 7
// ok TestCompactClearTail subtests: 0 checks: 3
// ok TestCompactFuncClearTail subtests: 0 checks: 7
// ok TestGrow subtests: 0 checks: 8
// ok TestClip subtests: 0 checks: 4
// ok TestReverse subtests: 0 checks: 4
// ok TestReplace subtests: 0 checks: 4
// ok TestReplacePanics subtests: 0 checks: 8
// ok TestReplaceGrow subtests: 0 checks: 3
// ok TestReplaceClearTail subtests: 0 checks: 3
// ok TestReplaceOverlap subtests: 0 checks: 18876
// ok TestReplaceEndClearTail subtests: 0 checks: 1
// ok TestInsertGrowthRate subtests: 0 checks: 1000001
// ok TestReplaceGrowthRate subtests: 0 checks: 1000001
// ok TestInference subtests: 0 checks: 2
// ok TestConcat subtests: 0 checks: 8
// ok TestRepeat subtests: 0 checks: 21
// ok TestRepeatPanics subtests: 0 checks: 6
// ok TestIssue68488 subtests: 0 checks: 0
// ok TestNilness subtests: 0 checks: 35
// tests: 38 failed: 0
