// phase: 09
// Go 1.25's slices/sort_test.go and iter_test.go (Sort, SortFunc, SortStableFunc, IsSorted*, BinarySearch*, Min/Max* with NaN, sorting of ints, strings, floats and structs, the adversarial inputs of the pdqsort tests; All, Backward, Values, AppendSeq, Collect, Sorted*, Chunk) through a small harness; math/rand is the harness's splitmix generator; expected output from real Go.

package main

import (
	"cmp"
	"fmt"
	"iter"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
)

func panics(f func()) (b bool) {
	defer func() {
		if x := recover(); x != nil {
			b = true
		}
	}()
	f()
	return false
}

var ints = [...]int{74, 59, 238, -784, 9845, 959, 905, 0, 0, 42, 7586, -5467984, 7586}
var float64s = [...]float64{74.3, 59.0, math.Inf(1), 238.2, -784.0, 2.3, math.Inf(-1), 9845.768, -959.7485, 905, 7.8, 7.8, 74.3, 59.0, math.Inf(1), 238.2, -784.0, 2.3}
var strs = [...]string{"", "Hello", "foo", "bar", "foo", "f00", "%*&^*&^&", "***"}

func TestSortIntSlice(t *testT) {
	data := slices.Clone(ints[:])
	slices.Sort(data)
	atomic.AddInt64(&checks, 1)
	if !slices.IsSorted(data) {
		t.Errorf("sorted %v", ints)
		t.Errorf("   got %v", data)
	}
}

func TestSortFuncIntSlice(t *testT) {
	data := slices.Clone(ints[:])
	slices.SortFunc(data, func(a, b int) int { return a - b })
	atomic.AddInt64(&checks, 1)
	if !slices.IsSorted(data) {
		t.Errorf("sorted %v", ints)
		t.Errorf("   got %v", data)
	}
}

func TestSortFloat64Slice(t *testT) {
	data := slices.Clone(float64s[:])
	slices.Sort(data)
	atomic.AddInt64(&checks, 1)
	if !slices.IsSorted(data) {
		t.Errorf("sorted %v", float64s)
		t.Errorf("   got %v", data)
	}
}

func TestSortStringSlice(t *testT) {
	data := slices.Clone(strs[:])
	slices.Sort(data)
	atomic.AddInt64(&checks, 1)
	if !slices.IsSorted(data) {
		t.Errorf("sorted %v", strs)
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
	if slices.IsSorted(data) {
		t.Fatalf("terrible rand.rand")
	}
	slices.Sort(data)
	atomic.AddInt64(&checks, 1)
	if !slices.IsSorted(data) {
		t.Errorf("sort didn't sort - 1M ints")
	}
}

type intPair struct {
	a, b int
}

type intPairs []intPair

// Pairs compare on a only.
func intPairCmp(x, y intPair) int {
	return x.a - y.a
}

// Record initial order in B.
func (d intPairs) initB() {
	for i := range d {
		d[i].b = i
	}
}

// InOrder checks if a-equal elements were not reordered.
// If reversed is true, expect reverse ordering.
func (d intPairs) inOrder(reversed bool) bool {
	lastA, lastB := -1, 0
	for i := 0; i < len(d); i++ {
		atomic.AddInt64(&checks, 1)
		if lastA != d[i].a {
			lastA = d[i].a
			lastB = d[i].b
			continue
		}
		atomic.AddInt64(&checks, 1)
		if !reversed {
			atomic.AddInt64(&checks, 1)
			if d[i].b <= lastB {
				return false
			}
		} else {
			atomic.AddInt64(&checks, 1)
			if d[i].b >= lastB {
				return false
			}
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
	if slices.IsSortedFunc(data, intPairCmp) {
		t.Fatalf("terrible rand.rand")
	}
	data.initB()
	slices.SortStableFunc(data, intPairCmp)
	atomic.AddInt64(&checks, 1)
	if !slices.IsSortedFunc(data, intPairCmp) {
		t.Errorf("Stable didn't sort %d ints", n)
	}
	atomic.AddInt64(&checks, 1)
	if !data.inOrder(false) {
		t.Errorf("Stable wasn't stable on %d ints", n)
	}

	// already sorted
	data.initB()
	slices.SortStableFunc(data, intPairCmp)
	atomic.AddInt64(&checks, 1)
	if !slices.IsSortedFunc(data, intPairCmp) {
		t.Errorf("Stable shuffled sorted %d ints (order)", n)
	}
	atomic.AddInt64(&checks, 1)
	if !data.inOrder(false) {
		t.Errorf("Stable shuffled sorted %d ints (stability)", n)
	}

	// sorted reversed
	for i := 0; i < len(data); i++ {
		data[i].a = len(data) - i
	}
	data.initB()
	slices.SortStableFunc(data, intPairCmp)
	atomic.AddInt64(&checks, 1)
	if !slices.IsSortedFunc(data, intPairCmp) {
		t.Errorf("Stable didn't sort %d ints", n)
	}
	atomic.AddInt64(&checks, 1)
	if !data.inOrder(false) {
		t.Errorf("Stable wasn't stable on %d ints", n)
	}
}

type S struct {
	a int
	b string
}

func cmpS(s1, s2 S) int {
	return cmp.Compare(s1.a, s2.a)
}

func TestMinMax(t *testT) {
	intCmp := func(a, b int) int { return a - b }

	tests := []struct {
		data    []int
		wantMin int
		wantMax int
	}{
		{[]int{7}, 7, 7},
		{[]int{1, 2}, 1, 2},
		{[]int{2, 1}, 1, 2},
		{[]int{1, 2, 3}, 1, 3},
		{[]int{3, 2, 1}, 1, 3},
		{[]int{2, 1, 3}, 1, 3},
		{[]int{2, 2, 3}, 2, 3},
		{[]int{3, 2, 3}, 2, 3},
		{[]int{0, 2, -9}, -9, 2},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.data), func(t *testT) {
			gotMin := slices.Min(tt.data)
			atomic.AddInt64(&checks, 1)
			if gotMin != tt.wantMin {
				t.Errorf("Min got %v, want %v", gotMin, tt.wantMin)
			}

			gotMinFunc := slices.MinFunc(tt.data, intCmp)
			atomic.AddInt64(&checks, 1)
			if gotMinFunc != tt.wantMin {
				t.Errorf("MinFunc got %v, want %v", gotMinFunc, tt.wantMin)
			}

			gotMax := slices.Max(tt.data)
			atomic.AddInt64(&checks, 1)
			if gotMax != tt.wantMax {
				t.Errorf("Max got %v, want %v", gotMax, tt.wantMax)
			}

			gotMaxFunc := slices.MaxFunc(tt.data, intCmp)
			atomic.AddInt64(&checks, 1)
			if gotMaxFunc != tt.wantMax {
				t.Errorf("MaxFunc got %v, want %v", gotMaxFunc, tt.wantMax)
			}
		})
	}

	svals := []S{
		{1, "a"},
		{2, "a"},
		{1, "b"},
		{2, "b"},
	}

	gotMin := slices.MinFunc(svals, cmpS)
	wantMin := S{1, "a"}
	atomic.AddInt64(&checks, 1)
	if gotMin != wantMin {
		t.Errorf("MinFunc(%v) = %v, want %v", svals, gotMin, wantMin)
	}

	gotMax := slices.MaxFunc(svals, cmpS)
	wantMax := S{2, "a"}
	atomic.AddInt64(&checks, 1)
	if gotMax != wantMax {
		t.Errorf("MaxFunc(%v) = %v, want %v", svals, gotMax, wantMax)
	}
}

func TestMinMaxNaNs(t *testT) {
	fs := []float64{1.0, 999.9, 3.14, -400.4, -5.14}
	atomic.AddInt64(&checks, 1)
	if slices.Min(fs) != -400.4 {
		t.Errorf("got min %v, want -400.4", slices.Min(fs))
	}
	atomic.AddInt64(&checks, 1)
	if slices.Max(fs) != 999.9 {
		t.Errorf("got max %v, want 999.9", slices.Max(fs))
	}

	// No matter which element of fs is replaced with a NaN, both Min and Max
	// should propagate the NaN to their output.
	for i := 0; i < len(fs); i++ {
		testfs := slices.Clone(fs)
		testfs[i] = math.NaN()

		fmin := slices.Min(testfs)
		atomic.AddInt64(&checks, 1)
		if !math.IsNaN(fmin) {
			t.Errorf("got min %v, want NaN", fmin)
		}

		fmax := slices.Max(testfs)
		atomic.AddInt64(&checks, 1)
		if !math.IsNaN(fmax) {
			t.Errorf("got max %v, want NaN", fmax)
		}
	}
}

func TestMinMaxPanics(t *testT) {
	intCmp := func(a, b int) int { return a - b }
	emptySlice := []int{}

	atomic.AddInt64(&checks, 1)
	if !panics(func() { _ = slices.Min(emptySlice) }) {
		t.Errorf("Min([]): got no panic, want panic")
	}

	atomic.AddInt64(&checks, 1)
	if !panics(func() { _ = slices.Max(emptySlice) }) {
		t.Errorf("Max([]): got no panic, want panic")
	}

	atomic.AddInt64(&checks, 1)
	if !panics(func() { _ = slices.MinFunc(emptySlice, intCmp) }) {
		t.Errorf("MinFunc([]): got no panic, want panic")
	}

	atomic.AddInt64(&checks, 1)
	if !panics(func() { _ = slices.MaxFunc(emptySlice, intCmp) }) {
		t.Errorf("MaxFunc([]): got no panic, want panic")
	}
}

func TestBinarySearch(t *testT) {
	str1 := []string{"foo"}
	str2 := []string{"ab", "ca"}
	str3 := []string{"mo", "qo", "vo"}
	str4 := []string{"ab", "ad", "ca", "xy"}

	// slice with repeating elements
	strRepeats := []string{"ba", "ca", "da", "da", "da", "ka", "ma", "ma", "ta"}

	// slice with all element equal
	strSame := []string{"xx", "xx", "xx"}

	tests := []struct {
		data      []string
		target    string
		wantPos   int
		wantFound bool
	}{
		{[]string{}, "foo", 0, false},
		{[]string{}, "", 0, false},

		{str1, "foo", 0, true},
		{str1, "bar", 0, false},
		{str1, "zx", 1, false},

		{str2, "aa", 0, false},
		{str2, "ab", 0, true},
		{str2, "ad", 1, false},
		{str2, "ca", 1, true},
		{str2, "ra", 2, false},

		{str3, "bb", 0, false},
		{str3, "mo", 0, true},
		{str3, "nb", 1, false},
		{str3, "qo", 1, true},
		{str3, "tr", 2, false},
		{str3, "vo", 2, true},
		{str3, "xr", 3, false},

		{str4, "aa", 0, false},
		{str4, "ab", 0, true},
		{str4, "ac", 1, false},
		{str4, "ad", 1, true},
		{str4, "ax", 2, false},
		{str4, "ca", 2, true},
		{str4, "cc", 3, false},
		{str4, "dd", 3, false},
		{str4, "xy", 3, true},
		{str4, "zz", 4, false},

		{strRepeats, "da", 2, true},
		{strRepeats, "db", 5, false},
		{strRepeats, "ma", 6, true},
		{strRepeats, "mb", 8, false},

		{strSame, "xx", 0, true},
		{strSame, "ab", 0, false},
		{strSame, "zz", 3, false},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testT) {
			{
				pos, found := slices.BinarySearch(tt.data, tt.target)
				atomic.AddInt64(&checks, 1)
				if pos != tt.wantPos || found != tt.wantFound {
					t.Errorf("BinarySearch got (%v, %v), want (%v, %v)", pos, found, tt.wantPos, tt.wantFound)
				}
			}

			{
				pos, found := slices.BinarySearchFunc(tt.data, tt.target, strings.Compare)
				atomic.AddInt64(&checks, 1)
				if pos != tt.wantPos || found != tt.wantFound {
					t.Errorf("BinarySearchFunc got (%v, %v), want (%v, %v)", pos, found, tt.wantPos, tt.wantFound)
				}
			}
		})
	}
}

func TestBinarySearchInts(t *testT) {
	data := []int{20, 30, 40, 50, 60, 70, 80, 90}
	tests := []struct {
		target    int
		wantPos   int
		wantFound bool
	}{
		{20, 0, true},
		{23, 1, false},
		{43, 3, false},
		{80, 6, true},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.target), func(t *testT) {
			{
				pos, found := slices.BinarySearch(data, tt.target)
				atomic.AddInt64(&checks, 1)
				if pos != tt.wantPos || found != tt.wantFound {
					t.Errorf("BinarySearch got (%v, %v), want (%v, %v)", pos, found, tt.wantPos, tt.wantFound)
				}
			}

			{
				cmp := func(a, b int) int {
					return a - b
				}
				pos, found := slices.BinarySearchFunc(data, tt.target, cmp)
				atomic.AddInt64(&checks, 1)
				if pos != tt.wantPos || found != tt.wantFound {
					t.Errorf("BinarySearchFunc got (%v, %v), want (%v, %v)", pos, found, tt.wantPos, tt.wantFound)
				}
			}
		})
	}
}

func TestBinarySearchFloats(t *testT) {
	data := []float64{math.NaN(), -0.25, 0.0, 1.4}
	tests := []struct {
		target    float64
		wantPos   int
		wantFound bool
	}{
		{math.NaN(), 0, true},
		{math.Inf(-1), 1, false},
		{-0.25, 1, true},
		{0.0, 2, true},
		{1.4, 3, true},
		{1.5, 4, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.target), func(t *testT) {
			{
				pos, found := slices.BinarySearch(data, tt.target)
				atomic.AddInt64(&checks, 1)
				if pos != tt.wantPos || found != tt.wantFound {
					t.Errorf("BinarySearch got (%v, %v), want (%v, %v)", pos, found, tt.wantPos, tt.wantFound)
				}
			}
		})
	}
}

func TestBinarySearchFunc(t *testT) {
	data := []int{1, 10, 11, 2} // sorted lexicographically
	cmp := func(a int, b string) int {
		return strings.Compare(strconv.Itoa(a), b)
	}
	pos, found := slices.BinarySearchFunc(data, "2", cmp)
	atomic.AddInt64(&checks, 1)
	if pos != 3 || !found {
		t.Errorf("BinarySearchFunc(%v, %q, cmp) = %v, %v, want %v, %v", data, "2", pos, found, 3, true)
	}
}


func TestAll(t *testT) {
	for size := 0; size < 10; size++ {
		var s []int
		for i := range size {
			s = append(s, i)
		}
		ei, ev := 0, 0
		cnt := 0
		for i, v := range slices.All(s) {
			atomic.AddInt64(&checks, 1)
			if i != ei || v != ev {
				t.Errorf("at iteration %d got %d, %d want %d, %d", cnt, i, v, ei, ev)
			}
			ei++
			ev++
			cnt++
		}
		atomic.AddInt64(&checks, 1)
		if cnt != size {
			t.Errorf("read %d values expected %d", cnt, size)
		}
	}
}

func TestBackward(t *testT) {
	for size := 0; size < 10; size++ {
		var s []int
		for i := range size {
			s = append(s, i)
		}
		ei, ev := size-1, size-1
		cnt := 0
		for i, v := range slices.Backward(s) {
			atomic.AddInt64(&checks, 1)
			if i != ei || v != ev {
				t.Errorf("at iteration %d got %d, %d want %d, %d", cnt, i, v, ei, ev)
			}
			ei--
			ev--
			cnt++
		}
		atomic.AddInt64(&checks, 1)
		if cnt != size {
			t.Errorf("read %d values expected %d", cnt, size)
		}
	}
}

func TestValues(t *testT) {
	for size := 0; size < 10; size++ {
		var s []int
		for i := range size {
			s = append(s, i)
		}
		ev := 0
		cnt := 0
		for v := range slices.Values(s) {
			atomic.AddInt64(&checks, 1)
			if v != ev {
				t.Errorf("at iteration %d got %d want %d", cnt, v, ev)
			}
			ev++
			cnt++
		}
		atomic.AddInt64(&checks, 1)
		if cnt != size {
			t.Errorf("read %d values expected %d", cnt, size)
		}
	}
}

func testSeq(yield func(int) bool) {
	for i := 0; i < 10; i += 2 {
		atomic.AddInt64(&checks, 1)
		if !yield(i) {
			return
		}
	}
}

var testSeqResult = []int{0, 2, 4, 6, 8}

func TestAppendSeq(t *testT) {
	s := slices.AppendSeq([]int{1, 2}, testSeq)
	want := append([]int{1, 2}, testSeqResult...)
	atomic.AddInt64(&checks, 1)
	if !slices.Equal(s, want) {
		t.Errorf("got %v, want %v", s, want)
	}
}

func TestCollect(t *testT) {
	s := slices.Collect(testSeq)
	want := testSeqResult
	atomic.AddInt64(&checks, 1)
	if !slices.Equal(s, want) {
		t.Errorf("got %v, want %v", s, want)
	}
}

var iterTests = [][]string{
	nil,
	{"a"},
	{"a", "b"},
	{"b", "a"},
	strs[:],
}

func TestValuesAppendSeq(t *testT) {
	for _, prefix := range iterTests {
		for _, s := range iterTests {
			got := slices.AppendSeq(prefix, slices.Values(s))
			want := append(prefix, s...)
			atomic.AddInt64(&checks, 1)
			if !slices.Equal(got, want) {
				t.Errorf("AppendSeq(%v, Values(%v)) == %v, want %v", prefix, s, got, want)
			}
		}
	}
}

func TestValuesCollect(t *testT) {
	for _, s := range iterTests {
		got := slices.Collect(slices.Values(s))
		atomic.AddInt64(&checks, 1)
		if !slices.Equal(got, s) {
			t.Errorf("Collect(Values(%v)) == %v, want %v", s, got, s)
		}
	}
}

func TestSorted(t *testT) {
	s := slices.Sorted(slices.Values(ints[:]))
	atomic.AddInt64(&checks, 1)
	if !slices.IsSorted(s) {
		t.Errorf("sorted %v", ints)
		t.Errorf("   got %v", s)
	}
}

func TestSortedFunc(t *testT) {
	s := slices.SortedFunc(slices.Values(ints[:]), func(a, b int) int { return a - b })
	atomic.AddInt64(&checks, 1)
	if !slices.IsSorted(s) {
		t.Errorf("sorted %v", ints)
		t.Errorf("   got %v", s)
	}
}

func TestSortedStableFunc(t *testT) {
	n, m := 1000, 100
	data := make(intPairs, n)
	for i := range data {
		data[i].a = rng.Intn(m)
	}
	data.initB()

	s := intPairs(slices.SortedStableFunc(slices.Values(data), intPairCmp))
	atomic.AddInt64(&checks, 1)
	if !slices.IsSortedFunc(s, intPairCmp) {
		t.Errorf("SortedStableFunc didn't sort %d ints", n)
	}
	atomic.AddInt64(&checks, 1)
	if !s.inOrder(false) {
		t.Errorf("SortedStableFunc wasn't stable on %d ints", n)
	}

	// iterVal converts a Seq2 to a Seq.
	iterVal := func(seq iter.Seq2[int, intPair]) iter.Seq[intPair] {
		return func(yield func(intPair) bool) {
			for _, v := range seq {
				atomic.AddInt64(&checks, 1)
				if !yield(v) {
					return
				}
			}
		}
	}

	s = intPairs(slices.SortedStableFunc(iterVal(slices.Backward(data)), intPairCmp))
	atomic.AddInt64(&checks, 1)
	if !slices.IsSortedFunc(s, intPairCmp) {
		t.Errorf("SortedStableFunc didn't sort %d reverse ints", n)
	}
	atomic.AddInt64(&checks, 1)
	if !s.inOrder(true) {
		t.Errorf("SortedStableFunc wasn't stable on %d reverse ints", n)
	}
}

func TestChunk(t *testT) {
	cases := []struct {
		name   string
		s      []int
		n      int
		chunks [][]int
	}{
		{
			name:   "nil",
			s:      nil,
			n:      1,
			chunks: nil,
		},
		{
			name:   "empty",
			s:      []int{},
			n:      1,
			chunks: nil,
		},
		{
			name:   "short",
			s:      []int{1, 2},
			n:      3,
			chunks: [][]int{{1, 2}},
		},
		{
			name:   "one",
			s:      []int{1, 2},
			n:      2,
			chunks: [][]int{{1, 2}},
		},
		{
			name:   "even",
			s:      []int{1, 2, 3, 4},
			n:      2,
			chunks: [][]int{{1, 2}, {3, 4}},
		},
		{
			name:   "odd",
			s:      []int{1, 2, 3, 4, 5},
			n:      2,
			chunks: [][]int{{1, 2}, {3, 4}, {5}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testT) {
			var chunks [][]int
			for c := range slices.Chunk(tc.s, tc.n) {
				chunks = append(chunks, c)
			}

			atomic.AddInt64(&checks, 1)
			if !chunkEqual(chunks, tc.chunks) {
				t.Errorf("Chunk(%v, %d) = %v, want %v", tc.s, tc.n, chunks, tc.chunks)
			}

			atomic.AddInt64(&checks, 1)
			if len(chunks) == 0 {
				return
			}

			// Verify that appending to the end of the first chunk does not
			// clobber the beginning of the next chunk.
			s := slices.Clone(tc.s)
			chunks[0] = append(chunks[0], -1)
			atomic.AddInt64(&checks, 1)
			if !slices.Equal(s, tc.s) {
				t.Errorf("slice was clobbered: %v, want %v", s, tc.s)
			}
		})
	}
}

func TestChunkPanics(t *testT) {
	for _, test := range []struct {
		name string
		x    []struct{}
		n    int
	}{
		{
			name: "cannot be less than 1",
			x:    make([]struct{}, 0),
			n:    0,
		},
	} {
		atomic.AddInt64(&checks, 1)
		if !panics(func() { _ = slices.Chunk(test.x, test.n) }) {
			t.Errorf("Chunk %s: got no panic, want panic", test.name)
		}
	}
}

func TestChunkRange(t *testT) {
	// Verify Chunk iteration can be stopped.
	var got [][]int
	for c := range slices.Chunk([]int{1, 2, 3, 4, -100}, 2) {
		atomic.AddInt64(&checks, 1)
		if len(got) == 2 {
			// Found enough values, break early.
			break
		}

		got = append(got, c)
	}

	atomic.AddInt64(&checks, 1)
	if want := [][]int{{1, 2}, {3, 4}}; !chunkEqual(got, want) {
		t.Errorf("Chunk iteration did not stop, got %v, want %v", got, want)
	}
}

func chunkEqual[Slice ~[]E, E comparable](s1, s2 []Slice) bool {
	return slices.EqualFunc(s1, s2, slices.Equal[Slice])
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
		{"TestSortIntSlice", TestSortIntSlice},
		{"TestSortFuncIntSlice", TestSortFuncIntSlice},
		{"TestSortFloat64Slice", TestSortFloat64Slice},
		{"TestSortStringSlice", TestSortStringSlice},
		{"TestSortLarge_Random", TestSortLarge_Random},
		{"TestStability", TestStability},
		{"TestMinMax", TestMinMax},
		{"TestMinMaxNaNs", TestMinMaxNaNs},
		{"TestMinMaxPanics", TestMinMaxPanics},
		{"TestBinarySearch", TestBinarySearch},
		{"TestBinarySearchInts", TestBinarySearchInts},
		{"TestBinarySearchFloats", TestBinarySearchFloats},
		{"TestBinarySearchFunc", TestBinarySearchFunc},
		{"TestAll", TestAll},
		{"TestBackward", TestBackward},
		{"TestValues", TestValues},
		{"TestAppendSeq", TestAppendSeq},
		{"TestCollect", TestCollect},
		{"TestValuesAppendSeq", TestValuesAppendSeq},
		{"TestValuesCollect", TestValuesCollect},
		{"TestSorted", TestSorted},
		{"TestSortedFunc", TestSortedFunc},
		{"TestSortedStableFunc", TestSortedStableFunc},
		{"TestChunk", TestChunk},
		{"TestChunkPanics", TestChunkPanics},
		{"TestChunkRange", TestChunkRange},
	}
	runTests(tests_)
}

// Output:
// ok TestSortIntSlice subtests: 0 checks: 1
// ok TestSortFuncIntSlice subtests: 0 checks: 1
// ok TestSortFloat64Slice subtests: 0 checks: 1
// ok TestSortStringSlice subtests: 0 checks: 1
// ok TestSortLarge_Random subtests: 0 checks: 3
// ok TestStability subtests: 0 checks: 6608
// ok TestMinMax subtests: 9 checks: 38
// ok TestMinMaxNaNs subtests: 0 checks: 12
// ok TestMinMaxPanics subtests: 0 checks: 4
// ok TestBinarySearch subtests: 34 checks: 68
// ok TestBinarySearchInts subtests: 4 checks: 8
// ok TestBinarySearchFloats subtests: 6 checks: 6
// ok TestBinarySearchFunc subtests: 0 checks: 1
// ok TestAll subtests: 0 checks: 55
// ok TestBackward subtests: 0 checks: 55
// ok TestValues subtests: 0 checks: 55
// ok TestAppendSeq subtests: 0 checks: 6
// ok TestCollect subtests: 0 checks: 6
// ok TestValuesAppendSeq subtests: 0 checks: 25
// ok TestValuesCollect subtests: 0 checks: 5
// ok TestSorted subtests: 0 checks: 1
// ok TestSortedFunc subtests: 0 checks: 1
// ok TestSortedStableFunc subtests: 0 checks: 6604
// ok TestChunk subtests: 6 checks: 16
// ok TestChunkPanics subtests: 0 checks: 1
// ok TestChunkRange subtests: 0 checks: 4
// tests: 26 failed: 0
