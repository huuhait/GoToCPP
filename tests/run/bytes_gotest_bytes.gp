// phase: 09
// Go 1.25's bytes/bytes_test.go (Equal, Compare, Index*, Count, Split*, Fields*, Lines and the Seq iterators, Map, Title, ToUpper/Lower, Trim*, Repeat, Replace, EqualFold, Cut*, Contains*, Clone, Buffer.Grow/Truncate panics) through a small harness; the allocation counts are not checked; expected output from real Go.

package main

import (
	"bytes"
	"fmt"
	"iter"
	"math"
	"slices"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
	"unsafe"
)

var compareTests = []struct {
	a, b []byte
	i    int
}{
	{[]byte(""), []byte(""), 0},
	{[]byte("a"), []byte(""), 1},
	{[]byte(""), []byte("a"), -1},
	{[]byte("abc"), []byte("abc"), 0},
	{[]byte("abd"), []byte("abc"), 1},
	{[]byte("abc"), []byte("abd"), -1},
	{[]byte("ab"), []byte("abc"), -1},
	{[]byte("abc"), []byte("ab"), 1},
	{[]byte("x"), []byte("ab"), 1},
	{[]byte("ab"), []byte("x"), -1},
	{[]byte("x"), []byte("a"), 1},
	{[]byte("b"), []byte("x"), -1},
	// test runtime·memeq's chunked implementation
	{[]byte("abcdefgh"), []byte("abcdefgh"), 0},
	{[]byte("abcdefghi"), []byte("abcdefghi"), 0},
	{[]byte("abcdefghi"), []byte("abcdefghj"), -1},
	{[]byte("abcdefghj"), []byte("abcdefghi"), 1},
	// nil tests
	{nil, nil, 0},
	{[]byte(""), nil, 0},
	{nil, []byte(""), 0},
	{[]byte("a"), nil, 1},
	{nil, []byte("a"), -1},
}

func sliceOfString(s [][]byte) []string {
	result := make([]string, len(s))
	for i, v := range s {
		result[i] = string(v)
	}
	return result
}

func collect(t *testT, seq iter.Seq[[]byte]) [][]byte {
	out := slices.Collect(seq)
	out1 := slices.Collect(seq)
	atomic.AddInt64(&checks, 1)
	if !slices.Equal(sliceOfString(out), sliceOfString(out1)) {
		t.Fatalf("inconsistent seq:\n%s\n%s", out, out1)
	}
	return out
}

type LinesTest struct {
	a string
	b []string
}

var linesTests = []LinesTest{
	{a: "abc\nabc\n", b: []string{"abc\n", "abc\n"}},
	{a: "abc\r\nabc", b: []string{"abc\r\n", "abc"}},
	{a: "abc\r\n", b: []string{"abc\r\n"}},
	{a: "\nabc", b: []string{"\n", "abc"}},
	{a: "\nabc\n\n", b: []string{"\n", "abc\n", "\n"}},
}

func TestLines(t *testT) {
	for _, s := range linesTests {
		result := sliceOfString(slices.Collect(bytes.Lines([]byte(s.a))))
		atomic.AddInt64(&checks, 1)
		if !slices.Equal(result, s.b) {
			t.Errorf(`slices.Collect(Lines(%q)) = %q; want %q`, s.a, result, s.b)
		}
	}
}

// For ease of reading, the test cases use strings that are converted to byte
// slices before invoking the functions.

var abcd = "abcd"
var faces = "☺☻☹"
var commas = "1,2,3,4"
var dots = "1....2....3....4"

type BinOpTest struct {
	a string
	b string
	i int
}

func TestEqual(t *testT) {
	// Run the tests and check for allocation at the same time.
	allocs := allocsPerRun(10, func() {
		for _, tt := range compareTests {
			eql := bytes.Equal(tt.a, tt.b)
			atomic.AddInt64(&checks, 1)
			if eql != (tt.i == 0) {
				t.Errorf(`Equal(%q, %q) = %v`, tt.a, tt.b, eql)
			}
		}
	})
	atomic.AddInt64(&checks, 1)
	if allocs > 0 {
		t.Errorf("Equal allocated %v times", allocs)
	}
}

func TestEqualExhaustive(t *testT) {
	var size = 128
	atomic.AddInt64(&checks, 1)
	if true {
		size = 32
	}
	a := make([]byte, size)
	b := make([]byte, size)
	b_init := make([]byte, size)
	// randomish but deterministic data
	for i := 0; i < size; i++ {
		a[i] = byte(17 * i)
		b_init[i] = byte(23*i + 100)
	}

	for len := 0; len <= size; len++ {
		for x := 0; x <= size-len; x++ {
			for y := 0; y <= size-len; y++ {
				copy(b, b_init)
				copy(b[y:y+len], a[x:x+len])
				atomic.AddInt64(&checks, 1)
				if !bytes.Equal(a[x:x+len], b[y:y+len]) || !bytes.Equal(b[y:y+len], a[x:x+len]) {
					t.Errorf("Equal(%d, %d, %d) = false", len, x, y)
				}
			}
		}
	}
}

// make sure Equal returns false for minimally different strings. The data
// is all zeros except for a single one in one location.
func TestNotEqual(t *testT) {
	var size = 128
	atomic.AddInt64(&checks, 1)
	if true {
		size = 32
	}
	a := make([]byte, size)
	b := make([]byte, size)

	for len := 0; len <= size; len++ {
		for x := 0; x <= size-len; x++ {
			for y := 0; y <= size-len; y++ {
				for diffpos := x; diffpos < x+len; diffpos++ {
					a[diffpos] = 1
					atomic.AddInt64(&checks, 1)
					if bytes.Equal(a[x:x+len], b[y:y+len]) || bytes.Equal(b[y:y+len], a[x:x+len]) {
						t.Errorf("NotEqual(%d, %d, %d, %d) = true", len, x, y, diffpos)
					}
					a[diffpos] = 0
				}
			}
		}
	}
}

var indexTests = []BinOpTest{
	{"", "", 0},
	{"", "a", -1},
	{"", "foo", -1},
	{"fo", "foo", -1},
	{"foo", "baz", -1},
	{"foo", "foo", 0},
	{"oofofoofooo", "f", 2},
	{"oofofoofooo", "foo", 4},
	{"barfoobarfoo", "foo", 3},
	{"foo", "", 0},
	{"foo", "o", 1},
	{"abcABCabc", "A", 3},
	// cases with one byte strings - test IndexByte and special case in Index()
	{"", "a", -1},
	{"x", "a", -1},
	{"x", "x", 0},
	{"abc", "a", 0},
	{"abc", "b", 1},
	{"abc", "c", 2},
	{"abc", "x", -1},
	{"barfoobarfooyyyzzzyyyzzzyyyzzzyyyxxxzzzyyy", "x", 33},
	{"fofofofooofoboo", "oo", 7},
	{"fofofofofofoboo", "ob", 11},
	{"fofofofofofoboo", "boo", 12},
	{"fofofofofofoboo", "oboo", 11},
	{"fofofofofoooboo", "fooo", 8},
	{"fofofofofofoboo", "foboo", 10},
	{"fofofofofofoboo", "fofob", 8},
	{"fofofofofofofoffofoobarfoo", "foffof", 12},
	{"fofofofofoofofoffofoobarfoo", "foffof", 13},
	{"fofofofofofofoffofoobarfoo", "foffofo", 12},
	{"fofofofofoofofoffofoobarfoo", "foffofo", 13},
	{"fofofofofoofofoffofoobarfoo", "foffofoo", 13},
	{"fofofofofofofoffofoobarfoo", "foffofoo", 12},
	{"fofofofofoofofoffofoobarfoo", "foffofoob", 13},
	{"fofofofofofofoffofoobarfoo", "foffofoob", 12},
	{"fofofofofoofofoffofoobarfoo", "foffofooba", 13},
	{"fofofofofofofoffofoobarfoo", "foffofooba", 12},
	{"fofofofofoofofoffofoobarfoo", "foffofoobar", 13},
	{"fofofofofofofoffofoobarfoo", "foffofoobar", 12},
	{"fofofofofoofofoffofoobarfoo", "foffofoobarf", 13},
	{"fofofofofofofoffofoobarfoo", "foffofoobarf", 12},
	{"fofofofofoofofoffofoobarfoo", "foffofoobarfo", 13},
	{"fofofofofofofoffofoobarfoo", "foffofoobarfo", 12},
	{"fofofofofoofofoffofoobarfoo", "foffofoobarfoo", 13},
	{"fofofofofofofoffofoobarfoo", "foffofoobarfoo", 12},
	{"fofofofofoofofoffofoobarfoo", "ofoffofoobarfoo", 12},
	{"fofofofofofofoffofoobarfoo", "ofoffofoobarfoo", 11},
	{"fofofofofoofofoffofoobarfoo", "fofoffofoobarfoo", 11},
	{"fofofofofofofoffofoobarfoo", "fofoffofoobarfoo", 10},
	{"fofofofofoofofoffofoobarfoo", "foobars", -1},
	{"foofyfoobarfoobar", "y", 4},
	{"oooooooooooooooooooooo", "r", -1},
	{"oxoxoxoxoxoxoxoxoxoxoxoy", "oy", 22},
	{"oxoxoxoxoxoxoxoxoxoxoxox", "oy", -1},
	// test fallback to Rabin-Karp.
	{"000000000000000000000000000000000000000000000000000000000000000000000001", "0000000000000000000000000000000000000000000000000000000000000000001", 5},
	// test fallback to IndexRune
	{"oxoxoxoxoxoxoxoxoxoxox☺", "☺", 22},
	// invalid UTF-8 byte sequence (must be longer than bytealg.MaxBruteForce to
	// test that we don't use IndexRune)
	{"xx0123456789012345678901234567890123456789012345678901234567890120123456789012345678901234567890123456xxx\xed\x9f\xc0", "\xed\x9f\xc0", 105},
}

var lastIndexTests = []BinOpTest{
	{"", "", 0},
	{"", "a", -1},
	{"", "foo", -1},
	{"fo", "foo", -1},
	{"foo", "foo", 0},
	{"foo", "f", 0},
	{"oofofoofooo", "f", 7},
	{"oofofoofooo", "foo", 7},
	{"barfoobarfoo", "foo", 9},
	{"foo", "", 3},
	{"foo", "o", 2},
	{"abcABCabc", "A", 3},
	{"abcABCabc", "a", 6},
}

var indexAnyTests = []BinOpTest{
	{"", "", -1},
	{"", "a", -1},
	{"", "abc", -1},
	{"a", "", -1},
	{"a", "a", 0},
	{"\x80", "\xffb", 0},
	{"aaa", "a", 0},
	{"abc", "xyz", -1},
	{"abc", "xcz", 2},
	{"ab☺c", "x☺yz", 2},
	{"a☺b☻c☹d", "cx", len("a☺b☻")},
	{"a☺b☻c☹d", "uvw☻xyz", len("a☺b")},
	{"aRegExp*", ".(|)*+?^$[]", 7},
	{dots + dots + dots, " ", -1},
	{"012abcba210", "\xffb", 4},
	{"012\x80bcb\x80210", "\xffb", 3},
	{"0123456\xcf\x80abc", "\xcfb\x80", 10},
}

var lastIndexAnyTests = []BinOpTest{
	{"", "", -1},
	{"", "a", -1},
	{"", "abc", -1},
	{"a", "", -1},
	{"a", "a", 0},
	{"\x80", "\xffb", 0},
	{"aaa", "a", 2},
	{"abc", "xyz", -1},
	{"abc", "ab", 1},
	{"ab☺c", "x☺yz", 2},
	{"a☺b☻c☹d", "cx", len("a☺b☻")},
	{"a☺b☻c☹d", "uvw☻xyz", len("a☺b")},
	{"a.RegExp*", ".(|)*+?^$[]", 8},
	{dots + dots + dots, " ", -1},
	{"012abcba210", "\xffb", 6},
	{"012\x80bcb\x80210", "\xffb", 7},
	{"0123456\xcf\x80abc", "\xcfb\x80", 10},
}

// Execute f on each test case.  funcName should be the name of f; it's used
// in failure reports.
func runIndexTests(t *testT, f func(s, sep []byte) int, funcName string, testCases []BinOpTest) {
	for _, test := range testCases {
		a := []byte(test.a)
		b := []byte(test.b)
		actual := f(a, b)
		atomic.AddInt64(&checks, 1)
		if actual != test.i {
			t.Errorf("%s(%q,%q) = %v; want %v", funcName, a, b, actual, test.i)
		}
	}
	var allocTests = []struct {
		a []byte
		b []byte
		i int
	}{
		// case for function Index.
		{[]byte("000000000000000000000000000000000000000000000000000000000000000000000001"), []byte("0000000000000000000000000000000000000000000000000000000000000000001"), 5},
		// case for function LastIndex.
		{[]byte("000000000000000000000000000000000000000000000000000000000000000010000"), []byte("00000000000000000000000000000000000000000000000000000000000001"), 3},
	}
	allocs := allocsPerRun(100, func() {
		atomic.AddInt64(&checks, 1)
		if i := bytes.Index(allocTests[1].a, allocTests[1].b); i != allocTests[1].i {
			t.Errorf("Index([]byte(%q), []byte(%q)) = %v; want %v", allocTests[1].a, allocTests[1].b, i, allocTests[1].i)
		}
		atomic.AddInt64(&checks, 1)
		if i := bytes.LastIndex(allocTests[0].a, allocTests[0].b); i != allocTests[0].i {
			t.Errorf("LastIndex([]byte(%q), []byte(%q)) = %v; want %v", allocTests[0].a, allocTests[0].b, i, allocTests[0].i)
		}
	})
	atomic.AddInt64(&checks, 1)
	if allocs != 0 {
		t.Errorf("expected no allocations, got %f", allocs)
	}
}

func runIndexAnyTests(t *testT, f func(s []byte, chars string) int, funcName string, testCases []BinOpTest) {
	for _, test := range testCases {
		a := []byte(test.a)
		actual := f(a, test.b)
		atomic.AddInt64(&checks, 1)
		if actual != test.i {
			t.Errorf("%s(%q,%q) = %v; want %v", funcName, a, test.b, actual, test.i)
		}
	}
}

func TestIndex(t *testT)     { runIndexTests(t, bytes.Index, "Index", indexTests) }
func TestLastIndex(t *testT) { runIndexTests(t, bytes.LastIndex, "LastIndex", lastIndexTests) }
func TestIndexAny(t *testT)  { runIndexAnyTests(t, bytes.IndexAny, "IndexAny", indexAnyTests) }
func TestLastIndexAny(t *testT) {
	runIndexAnyTests(t, bytes.LastIndexAny, "LastIndexAny", lastIndexAnyTests)
}

func TestIndexByte(t *testT) {
	for _, tt := range indexTests {
		atomic.AddInt64(&checks, 1)
		if len(tt.b) != 1 {
			continue
		}
		a := []byte(tt.a)
		b := tt.b[0]
		pos := bytes.IndexByte(a, b)
		atomic.AddInt64(&checks, 1)
		if pos != tt.i {
			t.Errorf(`IndexByte(%q, '%c') = %v`, tt.a, b, pos)
		}
	}
}

func TestLastIndexByte(t *testT) {
	testCases := []BinOpTest{
		{"", "q", -1},
		{"abcdef", "q", -1},
		{"abcdefabcdef", "a", len("abcdef")},      // something in the middle
		{"abcdefabcdef", "f", len("abcdefabcde")}, // last byte
		{"zabcdefabcdef", "z", 0},                 // first byte
		{"a☺b☻c☹d", "b", len("a☺")},               // non-ascii
	}
	for _, test := range testCases {
		actual := bytes.LastIndexByte([]byte(test.a), test.b[0])
		atomic.AddInt64(&checks, 1)
		if actual != test.i {
			t.Errorf("LastIndexByte(%q,%c) = %v; want %v", test.a, test.b[0], actual, test.i)
		}
	}
}

// test a larger buffer with different sizes and alignments
func TestIndexByteBig(t *testT) {
	var n = 1024
	atomic.AddInt64(&checks, 1)
	if true {
		n = 128
	}
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		// different start alignments
		b1 := b[i:]
		for j := 0; j < len(b1); j++ {
			b1[j] = 'x'
			pos := bytes.IndexByte(b1, 'x')
			atomic.AddInt64(&checks, 1)
			if pos != j {
				t.Errorf("IndexByte(%q, 'x') = %v", b1, pos)
			}
			b1[j] = 0
			pos = bytes.IndexByte(b1, 'x')
			atomic.AddInt64(&checks, 1)
			if pos != -1 {
				t.Errorf("IndexByte(%q, 'x') = %v", b1, pos)
			}
		}
		// different end alignments
		b1 = b[:i]
		for j := 0; j < len(b1); j++ {
			b1[j] = 'x'
			pos := bytes.IndexByte(b1, 'x')
			atomic.AddInt64(&checks, 1)
			if pos != j {
				t.Errorf("IndexByte(%q, 'x') = %v", b1, pos)
			}
			b1[j] = 0
			pos = bytes.IndexByte(b1, 'x')
			atomic.AddInt64(&checks, 1)
			if pos != -1 {
				t.Errorf("IndexByte(%q, 'x') = %v", b1, pos)
			}
		}
		// different start and end alignments
		b1 = b[i/2 : n-(i+1)/2]
		for j := 0; j < len(b1); j++ {
			b1[j] = 'x'
			pos := bytes.IndexByte(b1, 'x')
			atomic.AddInt64(&checks, 1)
			if pos != j {
				t.Errorf("IndexByte(%q, 'x') = %v", b1, pos)
			}
			b1[j] = 0
			pos = bytes.IndexByte(b1, 'x')
			atomic.AddInt64(&checks, 1)
			if pos != -1 {
				t.Errorf("IndexByte(%q, 'x') = %v", b1, pos)
			}
		}
	}
}

// test a small index across all page offsets
func TestIndexByteSmall(t *testT) {
	b := make([]byte, 5015) // bigger than a page
	// Make sure we find the correct byte even when straddling a page.
	for i := 0; i <= len(b)-15; i++ {
		for j := 0; j < 15; j++ {
			b[i+j] = byte(100 + j)
		}
		for j := 0; j < 15; j++ {
			p := bytes.IndexByte(b[i:i+15], byte(100+j))
			atomic.AddInt64(&checks, 1)
			if p != j {
				t.Errorf("IndexByte(%q, %d) = %d", b[i:i+15], 100+j, p)
			}
		}
		for j := 0; j < 15; j++ {
			b[i+j] = 0
		}
	}
	// Make sure matches outside the slice never trigger.
	for i := 0; i <= len(b)-15; i++ {
		for j := 0; j < 15; j++ {
			b[i+j] = 1
		}
		for j := 0; j < 15; j++ {
			p := bytes.IndexByte(b[i:i+15], byte(0))
			atomic.AddInt64(&checks, 1)
			if p != -1 {
				t.Errorf("IndexByte(%q, %d) = %d", b[i:i+15], 0, p)
			}
		}
		for j := 0; j < 15; j++ {
			b[i+j] = 0
		}
	}
}

func TestIndexRune(t *testT) {
	tests := []struct {
		in   string
		rune rune
		want int
	}{
		{"", 'a', -1},
		{"", '☺', -1},
		{"foo", '☹', -1},
		{"foo", 'o', 1},
		{"foo☺bar", '☺', 3},
		{"foo☺☻☹bar", '☹', 9},
		{"a A x", 'A', 2},
		{"some_text=some_value", '=', 9},
		{"☺a", 'a', 3},
		{"a☻☺b", '☺', 4},
		{"𠀳𠀗𠀾𠁄𠀧𠁆𠁂𠀫𠀖𠀪𠀲𠀴𠁀𠀨𠀿", '𠀿', 56},

		// 2 bytes
		{"ӆ", 'ӆ', 0},
		{"a", 'ӆ', -1},
		{"  ӆ", 'ӆ', 2},
		{"  a", 'ӆ', -1},
		{strings.Repeat("ц", 64) + "ӆ", 'ӆ', 128}, // test cutover
		{strings.Repeat("ц", 64), 'ӆ', -1},

		// 3 bytes
		{"Ꚁ", 'Ꚁ', 0},
		{"a", 'Ꚁ', -1},
		{"  Ꚁ", 'Ꚁ', 2},
		{"  a", 'Ꚁ', -1},
		{strings.Repeat("Ꙁ", 64) + "Ꚁ", 'Ꚁ', 192}, // test cutover
		{strings.Repeat("Ꙁ", 64) + "Ꚁ", '䚀', -1},  // 'Ꚁ' and '䚀' share the same last two bytes

		// 4 bytes
		{"𡌀", '𡌀', 0},
		{"a", '𡌀', -1},
		{"  𡌀", '𡌀', 2},
		{"  a", '𡌀', -1},
		{strings.Repeat("𡋀", 64) + "𡌀", '𡌀', 256}, // test cutover
		{strings.Repeat("𡋀", 64) + "𡌀", '𣌀', -1},  // '𡌀' and '𣌀' share the same last two bytes

		// RuneError should match any invalid UTF-8 byte sequence.
		{"�", '�', 0},
		{"\xff", '�', 0},
		{"☻x�", '�', len("☻x")},
		{"☻x\xe2\x98", '�', len("☻x")},
		{"☻x\xe2\x98�", '�', len("☻x")},
		{"☻x\xe2\x98x", '�', len("☻x")},

		// Invalid rune values should never match.
		{"a☺b☻c☹d\xe2\x98�\xff�\xed\xa0\x80", -1, -1},
		{"a☺b☻c☹d\xe2\x98�\xff�\xed\xa0\x80", 0xD800, -1}, // Surrogate pair
		{"a☺b☻c☹d\xe2\x98�\xff�\xed\xa0\x80", utf8.MaxRune + 1, -1},

		// Test the cutover to bytealg.Index when it is triggered in
		// the middle of rune that contains consecutive runs of equal bytes.
		{"aaaaaKKKK\U000bc104", '\U000bc104', 17}, // cutover: (n + 16) / 8
		{"aaaaaKKKK鄄", '鄄', 17},
		{"aaKKKKKa\U000bc104", '\U000bc104', 18}, // cutover: 4 + n>>4
		{"aaKKKKKa鄄", '鄄', 18},
	}
	for _, tt := range tests {
		atomic.AddInt64(&checks, 1)
		if got := bytes.IndexRune([]byte(tt.in), tt.rune); got != tt.want {
			t.Errorf("IndexRune(%q, %d) = %v; want %v", tt.in, tt.rune, got, tt.want)
		}
	}

	haystack := []byte("test世界")
	allocs := allocsPerRun(1000, func() {
		atomic.AddInt64(&checks, 1)
		if i := bytes.IndexRune(haystack, 's'); i != 2 {
			t.Fatalf("'s' at %d; want 2", i)
		}
		atomic.AddInt64(&checks, 1)
		if i := bytes.IndexRune(haystack, '世'); i != 4 {
			t.Fatalf("'世' at %d; want 4", i)
		}
	})
	atomic.AddInt64(&checks, 1)
	if allocs != 0 {
		t.Errorf("expected no allocations, got %f", allocs)
	}
}

// test count of a single byte across page offsets
func TestCountByte(t *testT) {
	b := make([]byte, 5015) // bigger than a page
	windows := []int{1, 2, 3, 4, 15, 16, 17, 31, 32, 33, 63, 64, 65, 128}
	testCountWindow := func(i, window int) {
		for j := 0; j < window; j++ {
			b[i+j] = byte(100)
			p := bytes.Count(b[i:i+window], []byte{100})
			atomic.AddInt64(&checks, 1)
			if p != j+1 {
				t.Errorf("TestCountByte.Count(%q, 100) = %d", b[i:i+window], p)
			}
		}
	}

	maxWnd := windows[len(windows)-1]

	for i := 0; i <= 2*maxWnd; i++ {
		for _, window := range windows {
			atomic.AddInt64(&checks, 1)
			if window > len(b[i:]) {
				window = len(b[i:])
			}
			testCountWindow(i, window)
			for j := 0; j < window; j++ {
				b[i+j] = byte(0)
			}
		}
	}
	for i := 4096 - (maxWnd + 1); i < len(b); i++ {
		for _, window := range windows {
			atomic.AddInt64(&checks, 1)
			if window > len(b[i:]) {
				window = len(b[i:])
			}
			testCountWindow(i, window)
			for j := 0; j < window; j++ {
				b[i+j] = byte(0)
			}
		}
	}
}

// Make sure we don't count bytes outside our window
func TestCountByteNoMatch(t *testT) {
	b := make([]byte, 5015)
	windows := []int{1, 2, 3, 4, 15, 16, 17, 31, 32, 33, 63, 64, 65, 128}
	for i := 0; i <= len(b); i++ {
		for _, window := range windows {
			atomic.AddInt64(&checks, 1)
			if window > len(b[i:]) {
				window = len(b[i:])
			}
			// Fill the window with non-match
			for j := 0; j < window; j++ {
				b[i+j] = byte(100)
			}
			// Try to find something that doesn't exist
			p := bytes.Count(b[i:i+window], []byte{0})
			atomic.AddInt64(&checks, 1)
			if p != 0 {
				t.Errorf("TestCountByteNoMatch(%q, 0) = %d", b[i:i+window], p)
			}
			for j := 0; j < window; j++ {
				b[i+j] = byte(0)
			}
		}
	}
}

var bmbuf []byte

func valName(x int) string {
	atomic.AddInt64(&checks, 1)
	if s := x >> 20; s<<20 == x {
		return fmt.Sprintf("%dM", s)
	}
	atomic.AddInt64(&checks, 1)
	if s := x >> 10; s<<10 == x {
		return fmt.Sprintf("%dK", s)
	}
	return fmt.Sprint(x)
}


var indexSizes = []int{10, 32, 4 << 10, 4 << 20, 64 << 20}

var isRaceBuilder = strings.HasSuffix("", "-race")


















type SplitTest struct {
	s   string
	sep string
	n   int
	a   []string
}

var splittests = []SplitTest{
	{"", "", -1, []string{}},
	{abcd, "a", 0, nil},
	{abcd, "", 2, []string{"a", "bcd"}},
	{abcd, "a", -1, []string{"", "bcd"}},
	{abcd, "z", -1, []string{"abcd"}},
	{abcd, "", -1, []string{"a", "b", "c", "d"}},
	{commas, ",", -1, []string{"1", "2", "3", "4"}},
	{dots, "...", -1, []string{"1", ".2", ".3", ".4"}},
	{faces, "☹", -1, []string{"☺☻", ""}},
	{faces, "~", -1, []string{faces}},
	{faces, "", -1, []string{"☺", "☻", "☹"}},
	{"1 2 3 4", " ", 3, []string{"1", "2", "3 4"}},
	{"1 2", " ", 3, []string{"1", "2"}},
	{"123", "", 2, []string{"1", "23"}},
	{"123", "", 17, []string{"1", "2", "3"}},
	{"bT", "T", math.MaxInt / 4, []string{"b", ""}},
	{"\xff-\xff", "", -1, []string{"\xff", "-", "\xff"}},
	{"\xff-\xff", "-", -1, []string{"\xff", "\xff"}},
}

func TestSplit(t *testT) {
	for _, tt := range splittests {
		a := bytes.SplitN([]byte(tt.s), []byte(tt.sep), tt.n)

		// Appending to the results should not change future results.
		var x []byte
		for _, v := range a {
			x = append(v, 'z')
		}

		result := sliceOfString(a)
		atomic.AddInt64(&checks, 1)
		if !slices.Equal(result, tt.a) {
			t.Errorf(`Split(%q, %q, %d) = %v; want %v`, tt.s, tt.sep, tt.n, result, tt.a)
			continue
		}

		atomic.AddInt64(&checks, 1)
		if tt.n < 0 {
			b := sliceOfString(slices.Collect(bytes.SplitSeq([]byte(tt.s), []byte(tt.sep))))
			atomic.AddInt64(&checks, 1)
			if !slices.Equal(b, tt.a) {
				t.Errorf(`collect(SplitSeq(%q, %q)) = %v; want %v`, tt.s, tt.sep, b, tt.a)
			}
		}

		atomic.AddInt64(&checks, 1)
		if tt.n == 0 || len(a) == 0 {
			continue
		}

		atomic.AddInt64(&checks, 1)
		if want := tt.a[len(tt.a)-1] + "z"; string(x) != want {
			t.Errorf("last appended result was %s; want %s", x, want)
		}

		s := bytes.Join(a, []byte(tt.sep))
		atomic.AddInt64(&checks, 1)
		if string(s) != tt.s {
			t.Errorf(`Join(Split(%q, %q, %d), %q) = %q`, tt.s, tt.sep, tt.n, tt.sep, s)
		}
		atomic.AddInt64(&checks, 1)
		if tt.n < 0 {
			b := sliceOfString(bytes.Split([]byte(tt.s), []byte(tt.sep)))
			atomic.AddInt64(&checks, 1)
			if !slices.Equal(result, b) {
				t.Errorf("Split disagrees withSplitN(%q, %q, %d) = %v; want %v", tt.s, tt.sep, tt.n, b, a)
			}
		}
		atomic.AddInt64(&checks, 1)
		if len(a) > 0 {
			in, out := a[0], s
			atomic.AddInt64(&checks, 1)
			if cap(in) == cap(out) && &in[:1][0] == &out[:1][0] {
				t.Errorf("Join(%#v, %q) didn't copy", a, tt.sep)
			}
		}
	}
}

var splitaftertests = []SplitTest{
	{abcd, "a", -1, []string{"a", "bcd"}},
	{abcd, "z", -1, []string{"abcd"}},
	{abcd, "", -1, []string{"a", "b", "c", "d"}},
	{commas, ",", -1, []string{"1,", "2,", "3,", "4"}},
	{dots, "...", -1, []string{"1...", ".2...", ".3...", ".4"}},
	{faces, "☹", -1, []string{"☺☻☹", ""}},
	{faces, "~", -1, []string{faces}},
	{faces, "", -1, []string{"☺", "☻", "☹"}},
	{"1 2 3 4", " ", 3, []string{"1 ", "2 ", "3 4"}},
	{"1 2 3", " ", 3, []string{"1 ", "2 ", "3"}},
	{"1 2", " ", 3, []string{"1 ", "2"}},
	{"123", "", 2, []string{"1", "23"}},
	{"123", "", 17, []string{"1", "2", "3"}},
}

func TestSplitAfter(t *testT) {
	for _, tt := range splitaftertests {
		a := bytes.SplitAfterN([]byte(tt.s), []byte(tt.sep), tt.n)

		// Appending to the results should not change future results.
		var x []byte
		for _, v := range a {
			x = append(v, 'z')
		}

		result := sliceOfString(a)
		atomic.AddInt64(&checks, 1)
		if !slices.Equal(result, tt.a) {
			t.Errorf(`Split(%q, %q, %d) = %v; want %v`, tt.s, tt.sep, tt.n, result, tt.a)
			continue
		}

		atomic.AddInt64(&checks, 1)
		if tt.n < 0 {
			b := sliceOfString(slices.Collect(bytes.SplitAfterSeq([]byte(tt.s), []byte(tt.sep))))
			atomic.AddInt64(&checks, 1)
			if !slices.Equal(b, tt.a) {
				t.Errorf(`collect(SplitAfterSeq(%q, %q)) = %v; want %v`, tt.s, tt.sep, b, tt.a)
			}
		}

		atomic.AddInt64(&checks, 1)
		if want := tt.a[len(tt.a)-1] + "z"; string(x) != want {
			t.Errorf("last appended result was %s; want %s", x, want)
		}

		s := bytes.Join(a, nil)
		atomic.AddInt64(&checks, 1)
		if string(s) != tt.s {
			t.Errorf(`Join(Split(%q, %q, %d), %q) = %q`, tt.s, tt.sep, tt.n, tt.sep, s)
		}
		atomic.AddInt64(&checks, 1)
		if tt.n < 0 {
			b := sliceOfString(bytes.SplitAfter([]byte(tt.s), []byte(tt.sep)))
			atomic.AddInt64(&checks, 1)
			if !slices.Equal(result, b) {
				t.Errorf("SplitAfter disagrees withSplitAfterN(%q, %q, %d) = %v; want %v", tt.s, tt.sep, tt.n, b, a)
			}
		}
	}
}

type FieldsTest struct {
	s string
	a []string
}

var fieldstests = []FieldsTest{
	{"", []string{}},
	{" ", []string{}},
	{" \t ", []string{}},
	{"  abc  ", []string{"abc"}},
	{"1 2 3 4", []string{"1", "2", "3", "4"}},
	{"1  2  3  4", []string{"1", "2", "3", "4"}},
	{"1\t\t2\t\t3\t4", []string{"1", "2", "3", "4"}},
	{"1\u20002\u20013\u20024", []string{"1", "2", "3", "4"}},
	{"\u2000\u2001\u2002", []string{}},
	{"\n™\t™\n", []string{"™", "™"}},
	{faces, []string{faces}},
}

func TestFields(t *testT) {
	for _, tt := range fieldstests {
		b := []byte(tt.s)
		a := bytes.Fields(b)

		// Appending to the results should not change future results.
		var x []byte
		for _, v := range a {
			x = append(v, 'z')
		}

		result := sliceOfString(a)
		atomic.AddInt64(&checks, 1)
		if !slices.Equal(result, tt.a) {
			t.Errorf("Fields(%q) = %v; want %v", tt.s, a, tt.a)
			continue
		}

		result2 := sliceOfString(collect(t, bytes.FieldsSeq([]byte(tt.s))))
		atomic.AddInt64(&checks, 1)
		if !slices.Equal(result2, tt.a) {
			t.Errorf(`collect(FieldsSeq(%q)) = %v; want %v`, tt.s, result2, tt.a)
		}

		atomic.AddInt64(&checks, 1)
		if string(b) != tt.s {
			t.Errorf("slice changed to %s; want %s", string(b), tt.s)
		}
		atomic.AddInt64(&checks, 1)
		if len(tt.a) > 0 {
			atomic.AddInt64(&checks, 1)
			if want := tt.a[len(tt.a)-1] + "z"; string(x) != want {
				t.Errorf("last appended result was %s; want %s", x, want)
			}
		}
	}
}

func TestFieldsFunc(t *testT) {
	for _, tt := range fieldstests {
		a := bytes.FieldsFunc([]byte(tt.s), unicode.IsSpace)
		result := sliceOfString(a)
		atomic.AddInt64(&checks, 1)
		if !slices.Equal(result, tt.a) {
			t.Errorf("FieldsFunc(%q, unicode.IsSpace) = %v; want %v", tt.s, a, tt.a)
			continue
		}
	}
	pred := func(c rune) bool { return c == 'X' }
	var fieldsFuncTests = []FieldsTest{
		{"", []string{}},
		{"XX", []string{}},
		{"XXhiXXX", []string{"hi"}},
		{"aXXbXXXcX", []string{"a", "b", "c"}},
	}
	for _, tt := range fieldsFuncTests {
		b := []byte(tt.s)
		a := bytes.FieldsFunc(b, pred)

		// Appending to the results should not change future results.
		var x []byte
		for _, v := range a {
			x = append(v, 'z')
		}

		result := sliceOfString(a)
		atomic.AddInt64(&checks, 1)
		if !slices.Equal(result, tt.a) {
			t.Errorf("FieldsFunc(%q) = %v, want %v", tt.s, a, tt.a)
		}

		result2 := sliceOfString(collect(t, bytes.FieldsFuncSeq([]byte(tt.s), pred)))
		atomic.AddInt64(&checks, 1)
		if !slices.Equal(result2, tt.a) {
			t.Errorf(`collect(FieldsFuncSeq(%q)) = %v; want %v`, tt.s, result2, tt.a)
		}

		atomic.AddInt64(&checks, 1)
		if string(b) != tt.s {
			t.Errorf("slice changed to %s; want %s", b, tt.s)
		}
		atomic.AddInt64(&checks, 1)
		if len(tt.a) > 0 {
			atomic.AddInt64(&checks, 1)
			if want := tt.a[len(tt.a)-1] + "z"; string(x) != want {
				t.Errorf("last appended result was %s; want %s", x, want)
			}
		}
	}
}

// Test case for any function which accepts and returns a byte slice.
// For ease of creation, we write the input byte slice as a string.
type StringTest struct {
	in  string
	out []byte
}

var upperTests = []StringTest{
	{"", []byte("")},
	{"ONLYUPPER", []byte("ONLYUPPER")},
	{"abc", []byte("ABC")},
	{"AbC123", []byte("ABC123")},
	{"azAZ09_", []byte("AZAZ09_")},
	{"longStrinGwitHmixofsmaLLandcAps", []byte("LONGSTRINGWITHMIXOFSMALLANDCAPS")},
	{"long\u0250string\u0250with\u0250nonascii\u2C6Fchars", []byte("LONG\u2C6FSTRING\u2C6FWITH\u2C6FNONASCII\u2C6FCHARS")},
	{"\u0250\u0250\u0250\u0250\u0250", []byte("\u2C6F\u2C6F\u2C6F\u2C6F\u2C6F")}, // grows one byte per char
	{"a\u0080\U0010FFFF", []byte("A\u0080\U0010FFFF")},                           // test utf8.RuneSelf and utf8.MaxRune
}

var lowerTests = []StringTest{
	{"", []byte("")},
	{"abc", []byte("abc")},
	{"AbC123", []byte("abc123")},
	{"azAZ09_", []byte("azaz09_")},
	{"longStrinGwitHmixofsmaLLandcAps", []byte("longstringwithmixofsmallandcaps")},
	{"LONG\u2C6FSTRING\u2C6FWITH\u2C6FNONASCII\u2C6FCHARS", []byte("long\u0250string\u0250with\u0250nonascii\u0250chars")},
	{"\u2C6D\u2C6D\u2C6D\u2C6D\u2C6D", []byte("\u0251\u0251\u0251\u0251\u0251")}, // shrinks one byte per char
	{"A\u0080\U0010FFFF", []byte("a\u0080\U0010FFFF")},                           // test utf8.RuneSelf and utf8.MaxRune
}

const space = "\t\v\r\f\n\u0085\u00a0\u2000\u3000"

var trimSpaceTests = []StringTest{
	{"", nil},
	{"  a", []byte("a")},
	{"b  ", []byte("b")},
	{"abc", []byte("abc")},
	{space + "abc" + space, []byte("abc")},
	{" ", nil},
	{"\u3000 ", nil},
	{" \u3000", nil},
	{" \t\r\n \t\t\r\r\n\n ", nil},
	{" \t\r\n x\t\t\r\r\n\n ", []byte("x")},
	{" \u2000\t\r\n x\t\t\r\r\ny\n \u3000", []byte("x\t\t\r\r\ny")},
	{"1 \t\r\n2", []byte("1 \t\r\n2")},
	{" x\x80", []byte("x\x80")},
	{" x\xc0", []byte("x\xc0")},
	{"x \xc0\xc0 ", []byte("x \xc0\xc0")},
	{"x \xc0", []byte("x \xc0")},
	{"x \xc0 ", []byte("x \xc0")},
	{"x \xc0\xc0 ", []byte("x \xc0\xc0")},
	{"x ☺\xc0\xc0 ", []byte("x ☺\xc0\xc0")},
	{"x ☺ ", []byte("x ☺")},
}

// Execute f on each test case.  funcName should be the name of f; it's used
// in failure reports.
func runStringTests(t *testT, f func([]byte) []byte, funcName string, testCases []StringTest) {
	for _, tc := range testCases {
		actual := f([]byte(tc.in))
		atomic.AddInt64(&checks, 1)
		if actual == nil && tc.out != nil {
			t.Errorf("%s(%q) = nil; want %q", funcName, tc.in, tc.out)
		}
		atomic.AddInt64(&checks, 1)
		if actual != nil && tc.out == nil {
			t.Errorf("%s(%q) = %q; want nil", funcName, tc.in, actual)
		}
		atomic.AddInt64(&checks, 1)
		if !bytes.Equal(actual, tc.out) {
			t.Errorf("%s(%q) = %q; want %q", funcName, tc.in, actual, tc.out)
		}
	}
}

func tenRunes(r rune) string {
	runes := make([]rune, 10)
	for i := range runes {
		runes[i] = r
	}
	return string(runes)
}

// User-defined self-inverse mapping function
func rot13(r rune) rune {
	const step = 13
	atomic.AddInt64(&checks, 1)
	if r >= 'a' && r <= 'z' {
		return ((r - 'a' + step) % 26) + 'a'
	}
	atomic.AddInt64(&checks, 1)
	if r >= 'A' && r <= 'Z' {
		return ((r - 'A' + step) % 26) + 'A'
	}
	return r
}

func TestMap(t *testT) {
	// Run a couple of awful growth/shrinkage tests
	a := tenRunes('a')

	// 1.  Grow. This triggers two reallocations in Map.
	maxRune := func(r rune) rune { return unicode.MaxRune }
	m := bytes.Map(maxRune, []byte(a))
	expect := tenRunes(unicode.MaxRune)
	atomic.AddInt64(&checks, 1)
	if string(m) != expect {
		t.Errorf("growing: expected %q got %q", expect, m)
	}

	// 2. Shrink
	minRune := func(r rune) rune { return 'a' }
	m = bytes.Map(minRune, []byte(tenRunes(unicode.MaxRune)))
	expect = a
	atomic.AddInt64(&checks, 1)
	if string(m) != expect {
		t.Errorf("shrinking: expected %q got %q", expect, m)
	}

	// 3. Rot13
	m = bytes.Map(rot13, []byte("a to zed"))
	expect = "n gb mrq"
	atomic.AddInt64(&checks, 1)
	if string(m) != expect {
		t.Errorf("rot13: expected %q got %q", expect, m)
	}

	// 4. Rot13^2
	m = bytes.Map(rot13, bytes.Map(rot13, []byte("a to zed")))
	expect = "a to zed"
	atomic.AddInt64(&checks, 1)
	if string(m) != expect {
		t.Errorf("rot13: expected %q got %q", expect, m)
	}

	// 5. Drop
	dropNotLatin := func(r rune) rune {
		atomic.AddInt64(&checks, 1)
		if unicode.Is(unicode.Latin, r) {
			return r
		}
		return -1
	}
	m = bytes.Map(dropNotLatin, []byte("Hello, 세계"))
	expect = "Hello"
	atomic.AddInt64(&checks, 1)
	if string(m) != expect {
		t.Errorf("drop: expected %q got %q", expect, m)
	}

	// 6. Invalid rune
	invalidRune := func(r rune) rune {
		return utf8.MaxRune + 1
	}
	m = bytes.Map(invalidRune, []byte("x"))
	expect = "\uFFFD"
	atomic.AddInt64(&checks, 1)
	if string(m) != expect {
		t.Errorf("invalidRune: expected %q got %q", expect, m)
	}
}

func TestToUpper(t *testT) { runStringTests(t, bytes.ToUpper, "ToUpper", upperTests) }

func TestToLower(t *testT) { runStringTests(t, bytes.ToLower, "ToLower", lowerTests) }



var toValidUTF8Tests = []struct {
	in   string
	repl string
	out  string
}{
	{"", "\uFFFD", ""},
	{"abc", "\uFFFD", "abc"},
	{"\uFDDD", "\uFFFD", "\uFDDD"},
	{"a\xffb", "\uFFFD", "a\uFFFDb"},
	{"a\xffb\uFFFD", "X", "aXb\uFFFD"},
	{"a☺\xffb☺\xC0\xAFc☺\xff", "", "a☺b☺c☺"},
	{"a☺\xffb☺\xC0\xAFc☺\xff", "日本語", "a☺日本語b☺日本語c☺日本語"},
	{"\xC0\xAF", "\uFFFD", "\uFFFD"},
	{"\xE0\x80\xAF", "\uFFFD", "\uFFFD"},
	{"\xed\xa0\x80", "abc", "abc"},
	{"\xed\xbf\xbf", "\uFFFD", "\uFFFD"},
	{"\xF0\x80\x80\xaf", "☺", "☺"},
	{"\xF8\x80\x80\x80\xAF", "\uFFFD", "\uFFFD"},
	{"\xFC\x80\x80\x80\x80\xAF", "\uFFFD", "\uFFFD"},
}

func TestToValidUTF8(t *testT) {
	for _, tc := range toValidUTF8Tests {
		got := bytes.ToValidUTF8([]byte(tc.in), []byte(tc.repl))
		atomic.AddInt64(&checks, 1)
		if !bytes.Equal(got, []byte(tc.out)) {
			t.Errorf("ToValidUTF8(%q, %q) = %q; want %q", tc.in, tc.repl, got, tc.out)
		}
	}
}

func TestTrimSpace(t *testT) { runStringTests(t, bytes.TrimSpace, "TrimSpace", trimSpaceTests) }

type RepeatTest struct {
	in, out string
	count   int
}

var longString = "a" + string(make([]byte, 1<<16)) + "z"

var RepeatTests = []RepeatTest{
	{"", "", 0},
	{"", "", 1},
	{"", "", 2},
	{"-", "", 0},
	{"-", "-", 1},
	{"-", "----------", 10},
	{"abc ", "abc abc abc ", 3},
	// Tests for results over the chunkLimit
	{string(rune(0)), string(make([]byte, 1<<16)), 1 << 16},
	{longString, longString + longString, 2},
}

func TestRepeat(t *testT) {
	for _, tt := range RepeatTests {
		tin := []byte(tt.in)
		tout := []byte(tt.out)
		a := bytes.Repeat(tin, tt.count)
		atomic.AddInt64(&checks, 1)
		if !bytes.Equal(a, tout) {
			t.Errorf("Repeat(%q, %d) = %q; want %q", tin, tt.count, a, tout)
			continue
		}
	}
}

func repeat(b []byte, count int) (err error) {
	defer func() {
		atomic.AddInt64(&checks, 1)
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				err = v
			default:
				err = fmt.Errorf("%s", v)
			}
		}
	}()

	bytes.Repeat(b, count)

	return
}

// See Issue golang.org/issue/16237
func TestRepeatCatchesOverflow(t *testT) {
	type testCase struct {
		s      string
		count  int
		errStr string
	}

	runTestCases := func(prefix string, tests []testCase) {
		for i, tt := range tests {
			err := repeat([]byte(tt.s), tt.count)
			atomic.AddInt64(&checks, 1)
			if tt.errStr == "" {
				atomic.AddInt64(&checks, 1)
				if err != nil {
					t.Errorf("#%d panicked %v", i, err)
				}
				continue
			}

			atomic.AddInt64(&checks, 1)
			if err == nil || !strings.Contains(err.Error(), tt.errStr) {
				t.Errorf("%s#%d got %q want %q", prefix, i, err, tt.errStr)
			}
		}
	}

	const maxInt = int(^uint(0) >> 1)

	runTestCases("", []testCase{
		0: {"--", -2147483647, "negative"},
		1: {"", maxInt, ""},
		2: {"-", 10, ""},
		3: {"gopher", 0, ""},
		4: {"-", -1, "negative"},
		5: {"--", -102, "negative"},
		6: {string(make([]byte, 255)), int((^uint(0))/255 + 1), "overflow"},
	})

	const is64Bit = 1<<(^uintptr(0)>>63)/2 != 0
	atomic.AddInt64(&checks, 1)
	if !is64Bit {
		return
	}

	runTestCases("64-bit", []testCase{
		0: {"-", maxInt, "out of range"},
	})
}

type RunesTest struct {
	in    string
	out   []rune
	lossy bool
}

var RunesTests = []RunesTest{
	{"", []rune{}, false},
	{" ", []rune{32}, false},
	{"ABC", []rune{65, 66, 67}, false},
	{"abc", []rune{97, 98, 99}, false},
	{"\u65e5\u672c\u8a9e", []rune{26085, 26412, 35486}, false},
	{"ab\x80c", []rune{97, 98, 0xFFFD, 99}, true},
	{"ab\xc0c", []rune{97, 98, 0xFFFD, 99}, true},
}

func TestRunes(t *testT) {
	for _, tt := range RunesTests {
		tin := []byte(tt.in)
		a := bytes.Runes(tin)
		atomic.AddInt64(&checks, 1)
		if !slices.Equal(a, tt.out) {
			t.Errorf("Runes(%q) = %v; want %v", tin, a, tt.out)
			continue
		}
		atomic.AddInt64(&checks, 1)
		if !tt.lossy {
			// can only test reassembly if we didn't lose information
			s := string(a)
			atomic.AddInt64(&checks, 1)
			if s != tt.in {
				t.Errorf("string(Runes(%q)) = %x; want %x", tin, s, tin)
			}
		}
	}
}

type TrimTest struct {
	f            string
	in, arg, out string
}

var trimTests = []TrimTest{
	{"Trim", "abba", "a", "bb"},
	{"Trim", "abba", "ab", ""},
	{"TrimLeft", "abba", "ab", ""},
	{"TrimRight", "abba", "ab", ""},
	{"TrimLeft", "abba", "a", "bba"},
	{"TrimLeft", "abba", "b", "abba"},
	{"TrimRight", "abba", "a", "abb"},
	{"TrimRight", "abba", "b", "abba"},
	{"Trim", "<tag>", "<>", "tag"},
	{"Trim", "* listitem", " *", "listitem"},
	{"Trim", `"quote"`, `"`, "quote"},
	{"Trim", "\u2C6F\u2C6F\u0250\u0250\u2C6F\u2C6F", "\u2C6F", "\u0250\u0250"},
	{"Trim", "\x80test\xff", "\xff", "test"},
	{"Trim", " Ġ ", " ", "Ġ"},
	{"Trim", " Ġİ0", "0 ", "Ġİ"},
	//empty string tests
	{"Trim", "abba", "", "abba"},
	{"Trim", "", "123", ""},
	{"Trim", "", "", ""},
	{"TrimLeft", "abba", "", "abba"},
	{"TrimLeft", "", "123", ""},
	{"TrimLeft", "", "", ""},
	{"TrimRight", "abba", "", "abba"},
	{"TrimRight", "", "123", ""},
	{"TrimRight", "", "", ""},
	{"TrimRight", "☺\xc0", "☺", "☺\xc0"},
	{"TrimPrefix", "aabb", "a", "abb"},
	{"TrimPrefix", "aabb", "b", "aabb"},
	{"TrimSuffix", "aabb", "a", "aabb"},
	{"TrimSuffix", "aabb", "b", "aab"},
}

type TrimNilTest struct {
	f   string
	in  []byte
	arg string
	out []byte
}

var trimNilTests = []TrimNilTest{
	{"Trim", nil, "", nil},
	{"Trim", []byte{}, "", nil},
	{"Trim", []byte{'a'}, "a", nil},
	{"Trim", []byte{'a', 'a'}, "a", nil},
	{"Trim", []byte{'a'}, "ab", nil},
	{"Trim", []byte{'a', 'b'}, "ab", nil},
	{"Trim", []byte("☺"), "☺", nil},
	{"TrimLeft", nil, "", nil},
	{"TrimLeft", []byte{}, "", nil},
	{"TrimLeft", []byte{'a'}, "a", nil},
	{"TrimLeft", []byte{'a', 'a'}, "a", nil},
	{"TrimLeft", []byte{'a'}, "ab", nil},
	{"TrimLeft", []byte{'a', 'b'}, "ab", nil},
	{"TrimLeft", []byte("☺"), "☺", nil},
	{"TrimRight", nil, "", nil},
	{"TrimRight", []byte{}, "", []byte{}},
	{"TrimRight", []byte{'a'}, "a", []byte{}},
	{"TrimRight", []byte{'a', 'a'}, "a", []byte{}},
	{"TrimRight", []byte{'a'}, "ab", []byte{}},
	{"TrimRight", []byte{'a', 'b'}, "ab", []byte{}},
	{"TrimRight", []byte("☺"), "☺", []byte{}},
	{"TrimPrefix", nil, "", nil},
	{"TrimPrefix", []byte{}, "", []byte{}},
	{"TrimPrefix", []byte{'a'}, "a", []byte{}},
	{"TrimPrefix", []byte("☺"), "☺", []byte{}},
	{"TrimSuffix", nil, "", nil},
	{"TrimSuffix", []byte{}, "", []byte{}},
	{"TrimSuffix", []byte{'a'}, "a", []byte{}},
	{"TrimSuffix", []byte("☺"), "☺", []byte{}},
}

func TestTrim(t *testT) {
	toFn := func(name string) (func([]byte, string) []byte, func([]byte, []byte) []byte) {
		switch name {
		case "Trim":
			return bytes.Trim, nil
		case "TrimLeft":
			return bytes.TrimLeft, nil
		case "TrimRight":
			return bytes.TrimRight, nil
		case "TrimPrefix":
			return nil, bytes.TrimPrefix
		case "TrimSuffix":
			return nil, bytes.TrimSuffix
		default:
			t.Errorf("Undefined trim function %s", name)
			return nil, nil
		}
	}

	for _, tc := range trimTests {
		name := tc.f
		f, fb := toFn(name)
		atomic.AddInt64(&checks, 1)
		if f == nil && fb == nil {
			continue
		}
		var actual string
		atomic.AddInt64(&checks, 1)
		if f != nil {
			actual = string(f([]byte(tc.in), tc.arg))
		} else {
			actual = string(fb([]byte(tc.in), []byte(tc.arg)))
		}
		atomic.AddInt64(&checks, 1)
		if actual != tc.out {
			t.Errorf("%s(%q, %q) = %q; want %q", name, tc.in, tc.arg, actual, tc.out)
		}
	}

	for _, tc := range trimNilTests {
		name := tc.f
		f, fb := toFn(name)
		atomic.AddInt64(&checks, 1)
		if f == nil && fb == nil {
			continue
		}
		var actual []byte
		atomic.AddInt64(&checks, 1)
		if f != nil {
			actual = f(tc.in, tc.arg)
		} else {
			actual = fb(tc.in, []byte(tc.arg))
		}
		report := func(s []byte) string {
			atomic.AddInt64(&checks, 1)
			if s == nil {
				return "nil"
			} else {
				return fmt.Sprintf("%q", s)
			}
		}
		atomic.AddInt64(&checks, 1)
		if len(actual) != 0 {
			t.Errorf("%s(%s, %q) returned non-empty value", name, report(tc.in), tc.arg)
		} else {
			actualNil := actual == nil
			outNil := tc.out == nil
			atomic.AddInt64(&checks, 1)
			if actualNil != outNil {
				t.Errorf("%s(%s, %q) got nil %t; want nil %t", name, report(tc.in), tc.arg, actualNil, outNil)
			}
		}
	}
}

type predicate struct {
	f    func(r rune) bool
	name string
}

var isSpace = predicate{unicode.IsSpace, "IsSpace"}
var isDigit = predicate{unicode.IsDigit, "IsDigit"}
var isUpper = predicate{unicode.IsUpper, "IsUpper"}
var isValidRune = predicate{
	func(r rune) bool {
		return r != utf8.RuneError
	},
	"IsValidRune",
}

type TrimFuncTest struct {
	f        predicate
	in       string
	trimOut  []byte
	leftOut  []byte
	rightOut []byte
}

func not(p predicate) predicate {
	return predicate{
		func(r rune) bool {
			return !p.f(r)
		},
		"not " + p.name,
	}
}

var trimFuncTests = []TrimFuncTest{
	{isSpace, space + " hello " + space,
		[]byte("hello"),
		[]byte("hello " + space),
		[]byte(space + " hello")},
	{isDigit, "\u0e50\u0e5212hello34\u0e50\u0e51",
		[]byte("hello"),
		[]byte("hello34\u0e50\u0e51"),
		[]byte("\u0e50\u0e5212hello")},
	{isUpper, "\u2C6F\u2C6F\u2C6F\u2C6FABCDhelloEF\u2C6F\u2C6FGH\u2C6F\u2C6F",
		[]byte("hello"),
		[]byte("helloEF\u2C6F\u2C6FGH\u2C6F\u2C6F"),
		[]byte("\u2C6F\u2C6F\u2C6F\u2C6FABCDhello")},
	{not(isSpace), "hello" + space + "hello",
		[]byte(space),
		[]byte(space + "hello"),
		[]byte("hello" + space)},
	{not(isDigit), "hello\u0e50\u0e521234\u0e50\u0e51helo",
		[]byte("\u0e50\u0e521234\u0e50\u0e51"),
		[]byte("\u0e50\u0e521234\u0e50\u0e51helo"),
		[]byte("hello\u0e50\u0e521234\u0e50\u0e51")},
	{isValidRune, "ab\xc0a\xc0cd",
		[]byte("\xc0a\xc0"),
		[]byte("\xc0a\xc0cd"),
		[]byte("ab\xc0a\xc0")},
	{not(isValidRune), "\xc0a\xc0",
		[]byte("a"),
		[]byte("a\xc0"),
		[]byte("\xc0a")},
	// The nils returned by TrimLeftFunc are odd behavior, but we need
	// to preserve backwards compatibility.
	{isSpace, "",
		nil,
		nil,
		[]byte("")},
	{isSpace, " ",
		nil,
		nil,
		[]byte("")},
}

func TestTrimFunc(t *testT) {
	for _, tc := range trimFuncTests {
		trimmers := []struct {
			name string
			trim func(s []byte, f func(r rune) bool) []byte
			out  []byte
		}{
			{"TrimFunc", bytes.TrimFunc, tc.trimOut},
			{"TrimLeftFunc", bytes.TrimLeftFunc, tc.leftOut},
			{"TrimRightFunc", bytes.TrimRightFunc, tc.rightOut},
		}
		for _, trimmer := range trimmers {
			actual := trimmer.trim([]byte(tc.in), tc.f.f)
			atomic.AddInt64(&checks, 1)
			if actual == nil && trimmer.out != nil {
				t.Errorf("%s(%q, %q) = nil; want %q", trimmer.name, tc.in, tc.f.name, trimmer.out)
			}
			atomic.AddInt64(&checks, 1)
			if actual != nil && trimmer.out == nil {
				t.Errorf("%s(%q, %q) = %q; want nil", trimmer.name, tc.in, tc.f.name, actual)
			}
			atomic.AddInt64(&checks, 1)
			if !bytes.Equal(actual, trimmer.out) {
				t.Errorf("%s(%q, %q) = %q; want %q", trimmer.name, tc.in, tc.f.name, actual, trimmer.out)
			}
		}
	}
}

type IndexFuncTest struct {
	in          string
	f           predicate
	first, last int
}

var indexFuncTests = []IndexFuncTest{
	{"", isValidRune, -1, -1},
	{"abc", isDigit, -1, -1},
	{"0123", isDigit, 0, 3},
	{"a1b", isDigit, 1, 1},
	{space, isSpace, 0, len(space) - 3}, // last rune in space is 3 bytes
	{"\u0e50\u0e5212hello34\u0e50\u0e51", isDigit, 0, 18},
	{"\u2C6F\u2C6F\u2C6F\u2C6FABCDhelloEF\u2C6F\u2C6FGH\u2C6F\u2C6F", isUpper, 0, 34},
	{"12\u0e50\u0e52hello34\u0e50\u0e51", not(isDigit), 8, 12},

	// tests of invalid UTF-8
	{"\x801", isDigit, 1, 1},
	{"\x80abc", isDigit, -1, -1},
	{"\xc0a\xc0", isValidRune, 1, 1},
	{"\xc0a\xc0", not(isValidRune), 0, 2},
	{"\xc0☺\xc0", not(isValidRune), 0, 4},
	{"\xc0☺\xc0\xc0", not(isValidRune), 0, 5},
	{"ab\xc0a\xc0cd", not(isValidRune), 2, 4},
	{"a\xe0\x80cd", not(isValidRune), 1, 2},
}

func TestIndexFunc(t *testT) {
	for _, tc := range indexFuncTests {
		first := bytes.IndexFunc([]byte(tc.in), tc.f.f)
		atomic.AddInt64(&checks, 1)
		if first != tc.first {
			t.Errorf("IndexFunc(%q, %s) = %d; want %d", tc.in, tc.f.name, first, tc.first)
		}
		last := bytes.LastIndexFunc([]byte(tc.in), tc.f.f)
		atomic.AddInt64(&checks, 1)
		if last != tc.last {
			t.Errorf("LastIndexFunc(%q, %s) = %d; want %d", tc.in, tc.f.name, last, tc.last)
		}
	}
}

type ReplaceTest struct {
	in       string
	old, new string
	n        int
	out      string
}

var ReplaceTests = []ReplaceTest{
	{"hello", "l", "L", 0, "hello"},
	{"hello", "l", "L", -1, "heLLo"},
	{"hello", "x", "X", -1, "hello"},
	{"", "x", "X", -1, ""},
	{"radar", "r", "<r>", -1, "<r>ada<r>"},
	{"", "", "<>", -1, "<>"},
	{"banana", "a", "<>", -1, "b<>n<>n<>"},
	{"banana", "a", "<>", 1, "b<>nana"},
	{"banana", "a", "<>", 1000, "b<>n<>n<>"},
	{"banana", "an", "<>", -1, "b<><>a"},
	{"banana", "ana", "<>", -1, "b<>na"},
	{"banana", "", "<>", -1, "<>b<>a<>n<>a<>n<>a<>"},
	{"banana", "", "<>", 10, "<>b<>a<>n<>a<>n<>a<>"},
	{"banana", "", "<>", 6, "<>b<>a<>n<>a<>n<>a"},
	{"banana", "", "<>", 5, "<>b<>a<>n<>a<>na"},
	{"banana", "", "<>", 1, "<>banana"},
	{"banana", "a", "a", -1, "banana"},
	{"banana", "a", "a", 1, "banana"},
	{"☺☻☹", "", "<>", -1, "<>☺<>☻<>☹<>"},
}

func TestReplace(t *testT) {
	for _, tt := range ReplaceTests {
		var (
			in  = []byte(tt.in)
			old = []byte(tt.old)
			new = []byte(tt.new)
		)
		atomic.AddInt64(&checks, 1)
		if !false {
			allocs := allocsPerRun(10, func() { bytes.Replace(in, old, new, tt.n) })
			atomic.AddInt64(&checks, 1)
			if allocs > 1 {
				t.Errorf("Replace(%q, %q, %q, %d) allocates %.2f objects", tt.in, tt.old, tt.new, tt.n, allocs)
			}
		}
		in = append(in, "<spare>"...)
		in = in[:len(tt.in)]
		out := bytes.Replace(in, old, new, tt.n)
		atomic.AddInt64(&checks, 1)
		if s := string(out); s != tt.out {
			t.Errorf("Replace(%q, %q, %q, %d) = %q, want %q", tt.in, tt.old, tt.new, tt.n, s, tt.out)
		}
		atomic.AddInt64(&checks, 1)
		if cap(in) == cap(out) && &in[:1][0] == &out[:1][0] {
			t.Errorf("Replace(%q, %q, %q, %d) didn't copy", tt.in, tt.old, tt.new, tt.n)
		}
		atomic.AddInt64(&checks, 1)
		if tt.n == -1 {
			out := bytes.ReplaceAll(in, old, new)
			atomic.AddInt64(&checks, 1)
			if s := string(out); s != tt.out {
				t.Errorf("ReplaceAll(%q, %q, %q) = %q, want %q", tt.in, tt.old, tt.new, s, tt.out)
			}
		}
	}
}



type TitleTest struct {
	in, out string
}

var TitleTests = []TitleTest{
	{"", ""},
	{"a", "A"},
	{" aaa aaa aaa ", " Aaa Aaa Aaa "},
	{" Aaa Aaa Aaa ", " Aaa Aaa Aaa "},
	{"123a456", "123a456"},
	{"double-blind", "Double-Blind"},
	{"ÿøû", "Ÿøû"},
	{"with_underscore", "With_underscore"},
	{"unicode \xe2\x80\xa8 line separator", "Unicode \xe2\x80\xa8 Line Separator"},
}

func TestTitle(t *testT) {
	for _, tt := range TitleTests {
		atomic.AddInt64(&checks, 1)
		if s := string(bytes.Title([]byte(tt.in))); s != tt.out {
			t.Errorf("Title(%q) = %q, want %q", tt.in, s, tt.out)
		}
	}
}

var ToTitleTests = []TitleTest{
	{"", ""},
	{"a", "A"},
	{" aaa aaa aaa ", " AAA AAA AAA "},
	{" Aaa Aaa Aaa ", " AAA AAA AAA "},
	{"123a456", "123A456"},
	{"double-blind", "DOUBLE-BLIND"},
	{"ÿøû", "ŸØÛ"},
}

func TestToTitle(t *testT) {
	for _, tt := range ToTitleTests {
		atomic.AddInt64(&checks, 1)
		if s := string(bytes.ToTitle([]byte(tt.in))); s != tt.out {
			t.Errorf("ToTitle(%q) = %q, want %q", tt.in, s, tt.out)
		}
	}
}

var EqualFoldTests = []struct {
	s, t string
	out  bool
}{
	{"abc", "abc", true},
	{"ABcd", "ABcd", true},
	{"123abc", "123ABC", true},
	{"αβδ", "ΑΒΔ", true},
	{"abc", "xyz", false},
	{"abc", "XYZ", false},
	{"abcdefghijk", "abcdefghijX", false},
	{"abcdefghijk", "abcdefghij\u212A", true},
	{"abcdefghijK", "abcdefghij\u212A", true},
	{"abcdefghijkz", "abcdefghij\u212Ay", false},
	{"abcdefghijKz", "abcdefghij\u212Ay", false},
}

func TestEqualFold(t *testT) {
	for _, tt := range EqualFoldTests {
		atomic.AddInt64(&checks, 1)
		if out := bytes.EqualFold([]byte(tt.s), []byte(tt.t)); out != tt.out {
			t.Errorf("EqualFold(%#q, %#q) = %v, want %v", tt.s, tt.t, out, tt.out)
		}
		atomic.AddInt64(&checks, 1)
		if out := bytes.EqualFold([]byte(tt.t), []byte(tt.s)); out != tt.out {
			t.Errorf("EqualFold(%#q, %#q) = %v, want %v", tt.t, tt.s, out, tt.out)
		}
	}
}

var cutTests = []struct {
	s, sep        string
	before, after string
	found         bool
}{
	{"abc", "b", "a", "c", true},
	{"abc", "a", "", "bc", true},
	{"abc", "c", "ab", "", true},
	{"abc", "abc", "", "", true},
	{"abc", "", "", "abc", true},
	{"abc", "d", "abc", "", false},
	{"", "d", "", "", false},
	{"", "", "", "", true},
}

func TestCut(t *testT) {
	for _, tt := range cutTests {
		atomic.AddInt64(&checks, 1)
		if before, after, found := bytes.Cut([]byte(tt.s), []byte(tt.sep)); string(before) != tt.before || string(after) != tt.after || found != tt.found {
			t.Errorf("Cut(%q, %q) = %q, %q, %v, want %q, %q, %v", tt.s, tt.sep, before, after, found, tt.before, tt.after, tt.found)
		}
	}
}

var cutPrefixTests = []struct {
	s, sep string
	after  string
	found  bool
}{
	{"abc", "a", "bc", true},
	{"abc", "abc", "", true},
	{"abc", "", "abc", true},
	{"abc", "d", "abc", false},
	{"", "d", "", false},
	{"", "", "", true},
}

func TestCutPrefix(t *testT) {
	for _, tt := range cutPrefixTests {
		atomic.AddInt64(&checks, 1)
		if after, found := bytes.CutPrefix([]byte(tt.s), []byte(tt.sep)); string(after) != tt.after || found != tt.found {
			t.Errorf("CutPrefix(%q, %q) = %q, %v, want %q, %v", tt.s, tt.sep, after, found, tt.after, tt.found)
		}
	}
}

var cutSuffixTests = []struct {
	s, sep string
	before string
	found  bool
}{
	{"abc", "bc", "a", true},
	{"abc", "abc", "", true},
	{"abc", "", "abc", true},
	{"abc", "d", "abc", false},
	{"", "d", "", false},
	{"", "", "", true},
}

func TestCutSuffix(t *testT) {
	for _, tt := range cutSuffixTests {
		atomic.AddInt64(&checks, 1)
		if before, found := bytes.CutSuffix([]byte(tt.s), []byte(tt.sep)); string(before) != tt.before || found != tt.found {
			t.Errorf("CutSuffix(%q, %q) = %q, %v, want %q, %v", tt.s, tt.sep, before, found, tt.before, tt.found)
		}
	}
}

func TestBufferGrowNegative(t *testT) {
	defer func() {
		atomic.AddInt64(&checks, 1)
		if err := recover(); err == nil {
			t.Fatal("Grow(-1) should have panicked")
		}
	}()
	var b bytes.Buffer
	b.Grow(-1)
}

func TestBufferTruncateNegative(t *testT) {
	defer func() {
		atomic.AddInt64(&checks, 1)
		if err := recover(); err == nil {
			t.Fatal("Truncate(-1) should have panicked")
		}
	}()
	var b bytes.Buffer
	b.Truncate(-1)
}

func TestBufferTruncateOutOfRange(t *testT) {
	defer func() {
		atomic.AddInt64(&checks, 1)
		if err := recover(); err == nil {
			t.Fatal("Truncate(20) should have panicked")
		}
	}()
	var b bytes.Buffer
	b.Write(make([]byte, 10))
	b.Truncate(20)
}

var containsTests = []struct {
	b, subslice []byte
	want        bool
}{
	{[]byte("hello"), []byte("hel"), true},
	{[]byte("日本語"), []byte("日本"), true},
	{[]byte("hello"), []byte("Hello, world"), false},
	{[]byte("東京"), []byte("京東"), false},
}

func TestContains(t *testT) {
	for _, tt := range containsTests {
		atomic.AddInt64(&checks, 1)
		if got := bytes.Contains(tt.b, tt.subslice); got != tt.want {
			t.Errorf("Contains(%q, %q) = %v, want %v", tt.b, tt.subslice, got, tt.want)
		}
	}
}

var ContainsAnyTests = []struct {
	b        []byte
	substr   string
	expected bool
}{
	{[]byte(""), "", false},
	{[]byte(""), "a", false},
	{[]byte(""), "abc", false},
	{[]byte("a"), "", false},
	{[]byte("a"), "a", true},
	{[]byte("aaa"), "a", true},
	{[]byte("abc"), "xyz", false},
	{[]byte("abc"), "xcz", true},
	{[]byte("a☺b☻c☹d"), "uvw☻xyz", true},
	{[]byte("aRegExp*"), ".(|)*+?^$[]", true},
	{[]byte(dots + dots + dots), " ", false},
}

func TestContainsAny(t *testT) {
	for _, ct := range ContainsAnyTests {
		atomic.AddInt64(&checks, 1)
		if bytes.ContainsAny(ct.b, ct.substr) != ct.expected {
			t.Errorf("ContainsAny(%s, %s) = %v, want %v",
				ct.b, ct.substr, !ct.expected, ct.expected)
		}
	}
}

var ContainsRuneTests = []struct {
	b        []byte
	r        rune
	expected bool
}{
	{[]byte(""), 'a', false},
	{[]byte("a"), 'a', true},
	{[]byte("aaa"), 'a', true},
	{[]byte("abc"), 'y', false},
	{[]byte("abc"), 'c', true},
	{[]byte("a☺b☻c☹d"), 'x', false},
	{[]byte("a☺b☻c☹d"), '☻', true},
	{[]byte("aRegExp*"), '*', true},
}

func TestContainsRune(t *testT) {
	for _, ct := range ContainsRuneTests {
		atomic.AddInt64(&checks, 1)
		if bytes.ContainsRune(ct.b, ct.r) != ct.expected {
			t.Errorf("ContainsRune(%q, %q) = %v, want %v",
				ct.b, ct.r, !ct.expected, ct.expected)
		}
	}
}

func TestContainsFunc(t *testT) {
	for _, ct := range ContainsRuneTests {
		atomic.AddInt64(&checks, 1)
		if bytes.ContainsFunc(ct.b, func(r rune) bool {
			return ct.r == r
		}) != ct.expected {
			t.Errorf("ContainsFunc(%q, func(%q)) = %v, want %v",
				ct.b, ct.r, !ct.expected, ct.expected)
		}
	}
}

func TestClone(t *testT) {
	var cloneTests = [][]byte{
		[]byte(nil),
		[]byte{},
		bytes.Clone([]byte{}),
		[]byte(strings.Repeat("a", 42))[:0],
		[]byte(strings.Repeat("a", 42))[:0:0],
		[]byte("short"),
		[]byte(strings.Repeat("a", 42)),
	}
	for _, input := range cloneTests {
		clone := bytes.Clone(input)
		atomic.AddInt64(&checks, 1)
		if !bytes.Equal(clone, input) {
			t.Errorf("Clone(%q) = %q; want %q", input, clone, input)
		}

		atomic.AddInt64(&checks, 1)
		if input == nil && clone != nil {
			t.Errorf("Clone(%#v) return value should be equal to nil slice.", input)
		}

		atomic.AddInt64(&checks, 1)
		if input != nil && clone == nil {
			t.Errorf("Clone(%#v) return value should not be equal to nil slice.", input)
		}

		atomic.AddInt64(&checks, 1)
		if cap(input) != 0 && unsafe.SliceData(input) == unsafe.SliceData(clone) {
			t.Errorf("Clone(%q) return value should not reference inputs backing memory.", input)
		}
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
	tests_ := []testCase{
		{"TestLines", TestLines},
		{"TestEqual", TestEqual},
		{"TestEqualExhaustive", TestEqualExhaustive},
		{"TestNotEqual", TestNotEqual},
		{"TestIndex", TestIndex},
		{"TestLastIndex", TestLastIndex},
		{"TestIndexAny", TestIndexAny},
		{"TestLastIndexAny", TestLastIndexAny},
		{"TestIndexByte", TestIndexByte},
		{"TestLastIndexByte", TestLastIndexByte},
		{"TestIndexByteBig", TestIndexByteBig},
		{"TestIndexByteSmall", TestIndexByteSmall},
		{"TestIndexRune", TestIndexRune},
		{"TestCountByte", TestCountByte},
		{"TestCountByteNoMatch", TestCountByteNoMatch},
		{"TestSplit", TestSplit},
		{"TestSplitAfter", TestSplitAfter},
		{"TestFields", TestFields},
		{"TestFieldsFunc", TestFieldsFunc},
		{"TestMap", TestMap},
		{"TestToUpper", TestToUpper},
		{"TestToLower", TestToLower},
		{"TestToValidUTF8", TestToValidUTF8},
		{"TestTrimSpace", TestTrimSpace},
		{"TestRepeat", TestRepeat},
		{"TestRepeatCatchesOverflow", TestRepeatCatchesOverflow},
		{"TestRunes", TestRunes},
		{"TestTrim", TestTrim},
		{"TestTrimFunc", TestTrimFunc},
		{"TestIndexFunc", TestIndexFunc},
		{"TestReplace", TestReplace},
		{"TestTitle", TestTitle},
		{"TestToTitle", TestToTitle},
		{"TestEqualFold", TestEqualFold},
		{"TestCut", TestCut},
		{"TestCutPrefix", TestCutPrefix},
		{"TestCutSuffix", TestCutSuffix},
		{"TestBufferGrowNegative", TestBufferGrowNegative},
		{"TestBufferTruncateNegative", TestBufferTruncateNegative},
		{"TestBufferTruncateOutOfRange", TestBufferTruncateOutOfRange},
		{"TestContains", TestContains},
		{"TestContainsAny", TestContainsAny},
		{"TestContainsRune", TestContainsRune},
		{"TestContainsFunc", TestContainsFunc},
		{"TestClone", TestClone},
	}
	runTests(tests_)
}

// Output:
// ok TestLines subtests: 0 checks: 5
// ok TestEqual subtests: 0 checks: 211
// ok TestEqualExhaustive subtests: 0 checks: 12530
// ok TestNotEqual subtests: 0 checks: 98737
// ok TestIndex subtests: 0 checks: 258
// ok TestLastIndex subtests: 0 checks: 214
// ok TestIndexAny subtests: 0 checks: 17
// ok TestLastIndexAny subtests: 0 checks: 17
// ok TestIndexByte subtests: 0 checks: 71
// ok TestLastIndexByte subtests: 0 checks: 6
// ok TestIndexByteBig subtests: 0 checks: 49281
// ok TestIndexByteSmall subtests: 0 checks: 150030
// ok TestIndexRune subtests: 0 checks: 2043
// ok TestCountByte subtests: 0 checks: 620803
// ok TestCountByteNoMatch subtests: 0 checks: 140448
// ok TestSplit subtests: 0 checks: 155
// ok TestSplitAfter subtests: 0 checks: 81
// ok TestFields subtests: 0 checks: 62
// ok TestFieldsFunc subtests: 0 checks: 33
// ok TestMap subtests: 0 checks: 45
// ok TestToUpper subtests: 0 checks: 27
// ok TestToLower subtests: 0 checks: 24
// ok TestToValidUTF8 subtests: 0 checks: 14
// ok TestTrimSpace subtests: 0 checks: 60
// ok TestRepeat subtests: 0 checks: 9
// ok TestRepeatCatchesOverflow subtests: 0 checks: 25
// ok TestRunes subtests: 0 checks: 19
// ok TestTrim subtests: 0 checks: 203
// ok TestTrimFunc subtests: 0 checks: 81
// ok TestIndexFunc subtests: 0 checks: 32
// ok TestReplace subtests: 0 checks: 106
// ok TestTitle subtests: 0 checks: 9
// ok TestToTitle subtests: 0 checks: 7
// ok TestEqualFold subtests: 0 checks: 22
// ok TestCut subtests: 0 checks: 8
// ok TestCutPrefix subtests: 0 checks: 6
// ok TestCutSuffix subtests: 0 checks: 6
// ok TestBufferGrowNegative subtests: 0 checks: 1
// ok TestBufferTruncateNegative subtests: 0 checks: 1
// ok TestBufferTruncateOutOfRange subtests: 0 checks: 1
// ok TestContains subtests: 0 checks: 4
// ok TestContainsAny subtests: 0 checks: 11
// ok TestContainsRune subtests: 0 checks: 8
// ok TestContainsFunc subtests: 0 checks: 8
// ok TestClone subtests: 0 checks: 28
// tests: 45 failed: 0
