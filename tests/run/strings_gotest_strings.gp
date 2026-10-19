// phase: 09
// Go 1.25's strings/strings_test.go (Index, Count, Split*, Fields*, Map, Title, ToUpper/Lower, Trim*, Repeat, Replace, EqualFold, Cut*, ContainsFunc, invalid UTF-8 handling, the random index tests) through a small harness; the iterator tests (Lines, SplitSeq, FieldsSeq, ...) are not included, they need generic types; the allocation counts are not checked; expected output from real Go.

package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
	"unsafe"
)


func slicesEqual[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var abcd = "abcd"
var faces = "☺☻☹"
var commas = "1,2,3,4"
var dots = "1....2....3....4"

type IndexTest struct {
	s   string
	sep string
	out int
}

var indexTests = []IndexTest{
	{"", "", 0},
	{"", "a", -1},
	{"", "foo", -1},
	{"fo", "foo", -1},
	{"foo", "foo", 0},
	{"oofofoofooo", "f", 2},
	{"oofofoofooo", "foo", 4},
	{"barfoobarfoo", "foo", 3},
	{"foo", "", 0},
	{"foo", "o", 1},
	{"abcABCabc", "A", 3},
	{"jrzm6jjhorimglljrea4w3rlgosts0w2gia17hno2td4qd1jz", "jz", 47},
	{"ekkuk5oft4eq0ocpacknhwouic1uua46unx12l37nioq9wbpnocqks6", "ks6", 52},
	{"999f2xmimunbuyew5vrkla9cpwhmxan8o98ec", "98ec", 33},
	{"9lpt9r98i04k8bz6c6dsrthb96bhi", "96bhi", 24},
	{"55u558eqfaod2r2gu42xxsu631xf0zobs5840vl", "5840vl", 33},
	// cases with one byte strings - test special case in Index()
	{"", "a", -1},
	{"x", "a", -1},
	{"x", "x", 0},
	{"abc", "a", 0},
	{"abc", "b", 1},
	{"abc", "c", 2},
	{"abc", "x", -1},
	// test special cases in Index() for short strings
	{"", "ab", -1},
	{"bc", "ab", -1},
	{"ab", "ab", 0},
	{"xab", "ab", 1},
	{"xab"[:2], "ab", -1},
	{"", "abc", -1},
	{"xbc", "abc", -1},
	{"abc", "abc", 0},
	{"xabc", "abc", 1},
	{"xabc"[:3], "abc", -1},
	{"xabxc", "abc", -1},
	{"", "abcd", -1},
	{"xbcd", "abcd", -1},
	{"abcd", "abcd", 0},
	{"xabcd", "abcd", 1},
	{"xyabcd"[:5], "abcd", -1},
	{"xbcqq", "abcqq", -1},
	{"abcqq", "abcqq", 0},
	{"xabcqq", "abcqq", 1},
	{"xyabcqq"[:6], "abcqq", -1},
	{"xabxcqq", "abcqq", -1},
	{"xabcqxq", "abcqq", -1},
	{"", "01234567", -1},
	{"32145678", "01234567", -1},
	{"01234567", "01234567", 0},
	{"x01234567", "01234567", 1},
	{"x0123456x01234567", "01234567", 9},
	{"xx01234567"[:9], "01234567", -1},
	{"", "0123456789", -1},
	{"3214567844", "0123456789", -1},
	{"0123456789", "0123456789", 0},
	{"x0123456789", "0123456789", 1},
	{"x012345678x0123456789", "0123456789", 11},
	{"xyz0123456789"[:12], "0123456789", -1},
	{"x01234567x89", "0123456789", -1},
	{"", "0123456789012345", -1},
	{"3214567889012345", "0123456789012345", -1},
	{"0123456789012345", "0123456789012345", 0},
	{"x0123456789012345", "0123456789012345", 1},
	{"x012345678901234x0123456789012345", "0123456789012345", 17},
	{"", "01234567890123456789", -1},
	{"32145678890123456789", "01234567890123456789", -1},
	{"01234567890123456789", "01234567890123456789", 0},
	{"x01234567890123456789", "01234567890123456789", 1},
	{"x0123456789012345678x01234567890123456789", "01234567890123456789", 21},
	{"xyz01234567890123456789"[:22], "01234567890123456789", -1},
	{"", "0123456789012345678901234567890", -1},
	{"321456788901234567890123456789012345678911", "0123456789012345678901234567890", -1},
	{"0123456789012345678901234567890", "0123456789012345678901234567890", 0},
	{"x0123456789012345678901234567890", "0123456789012345678901234567890", 1},
	{"x012345678901234567890123456789x0123456789012345678901234567890", "0123456789012345678901234567890", 32},
	{"xyz0123456789012345678901234567890"[:33], "0123456789012345678901234567890", -1},
	{"", "01234567890123456789012345678901", -1},
	{"32145678890123456789012345678901234567890211", "01234567890123456789012345678901", -1},
	{"01234567890123456789012345678901", "01234567890123456789012345678901", 0},
	{"x01234567890123456789012345678901", "01234567890123456789012345678901", 1},
	{"x0123456789012345678901234567890x01234567890123456789012345678901", "01234567890123456789012345678901", 33},
	{"xyz01234567890123456789012345678901"[:34], "01234567890123456789012345678901", -1},
	{"xxxxxx012345678901234567890123456789012345678901234567890123456789012", "012345678901234567890123456789012345678901234567890123456789012", 6},
	{"", "0123456789012345678901234567890123456789", -1},
	{"xx012345678901234567890123456789012345678901234567890123456789012", "0123456789012345678901234567890123456789", 2},
	{"xx012345678901234567890123456789012345678901234567890123456789012"[:41], "0123456789012345678901234567890123456789", -1},
	{"xx012345678901234567890123456789012345678901234567890123456789012", "0123456789012345678901234567890123456xxx", -1},
	{"xx0123456789012345678901234567890123456789012345678901234567890120123456789012345678901234567890123456xxx", "0123456789012345678901234567890123456xxx", 65},
	// test fallback to Rabin-Karp.
	{"oxoxoxoxoxoxoxoxoxoxoxoy", "oy", 22},
	{"oxoxoxoxoxoxoxoxoxoxoxox", "oy", -1},
	// test fallback to IndexRune
	{"oxoxoxoxoxoxoxoxoxoxox☺", "☺", 22},
	// invalid UTF-8 byte sequence (must be longer than bytealg.MaxBruteForce to
	// test that we don't use IndexRune)
	{"xx0123456789012345678901234567890123456789012345678901234567890120123456789012345678901234567890123456xxx\xed\x9f\xc0", "\xed\x9f\xc0", 105},
}

var lastIndexTests = []IndexTest{
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

var indexAnyTests = []IndexTest{
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

var lastIndexAnyTests = []IndexTest{
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
func runIndexTests(t *testT, f func(s, sep string) int, funcName string, testCases []IndexTest) {
	for _, test := range testCases {
		actual := f(test.s, test.sep)
		atomic.AddInt64(&checks, 1)
		if actual != test.out {
			t.Errorf("%s(%q,%q) = %v; want %v", funcName, test.s, test.sep, actual, test.out)
		}
	}
}

func TestIndex(t *testT)     { runIndexTests(t, strings.Index, "Index", indexTests) }
func TestLastIndex(t *testT) { runIndexTests(t, strings.LastIndex, "LastIndex", lastIndexTests) }
func TestIndexAny(t *testT)  { runIndexTests(t, strings.IndexAny, "IndexAny", indexAnyTests) }
func TestLastIndexAny(t *testT) {
	runIndexTests(t, strings.LastIndexAny, "LastIndexAny", lastIndexAnyTests)
}

func TestIndexByte(t *testT) {
	for _, tt := range indexTests {
		atomic.AddInt64(&checks, 1)
		if len(tt.sep) != 1 {
			continue
		}
		pos := strings.IndexByte(tt.s, tt.sep[0])
		atomic.AddInt64(&checks, 1)
		if pos != tt.out {
			t.Errorf(`IndexByte(%q, %q) = %v; want %v`, tt.s, tt.sep[0], pos, tt.out)
		}
	}
}

func TestLastIndexByte(t *testT) {
	testCases := []IndexTest{
		{"", "q", -1},
		{"abcdef", "q", -1},
		{"abcdefabcdef", "a", len("abcdef")},      // something in the middle
		{"abcdefabcdef", "f", len("abcdefabcde")}, // last byte
		{"zabcdefabcdef", "z", 0},                 // first byte
		{"a☺b☻c☹d", "b", len("a☺")},               // non-ascii
	}
	for _, test := range testCases {
		actual := strings.LastIndexByte(test.s, test.sep[0])
		atomic.AddInt64(&checks, 1)
		if actual != test.out {
			t.Errorf("LastIndexByte(%q,%c) = %v; want %v", test.s, test.sep[0], actual, test.out)
		}
	}
}

func simpleIndex(s, sep string) int {
	n := len(sep)
	for i := n; i <= len(s); i++ {
		atomic.AddInt64(&checks, 1)
		if s[i-n:i] == sep {
			return i - n
		}
	}
	return -1
}

func TestIndexRandom(t *testT) {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	for times := 0; times < 10; times++ {
		for strLen := 5 + rng.Intn(5); strLen < 140; strLen += 10 { // Arbitrary
			s1 := make([]byte, strLen)
			for i := range s1 {
				s1[i] = chars[rng.Intn(len(chars))]
			}
			s := string(s1)
			for i := 0; i < 50; i++ {
				begin := rng.Intn(len(s) + 1)
				end := begin + rng.Intn(len(s)+1-begin)
				sep := s[begin:end]
				atomic.AddInt64(&checks, 1)
				if i%4 == 0 {
					pos := rng.Intn(len(sep) + 1)
					sep = sep[:pos] + "A" + sep[pos:]
				}
				want := simpleIndex(s, sep)
				res := strings.Index(s, sep)
				atomic.AddInt64(&checks, 1)
				if res != want {
					t.Errorf("Index(%s,%s) = %d; want %d", s, sep, res, want)
				}
			}
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

		// 2 bytes
		{"ӆ", 'ӆ', 0},
		{"a", 'ӆ', -1},
		{"  ӆ", 'ӆ', 2},
		{"  a", 'ӆ', -1},
		{strings.Repeat("ц", 64) + "ӆ", 'ӆ', 128}, // test cutover
		{strings.Repeat("Ꙁ", 64) + "Ꚁ", '䚀', -1},  // 'Ꚁ' and '䚀' share the same last two bytes

		// 3 bytes
		{"Ꚁ", 'Ꚁ', 0},
		{"a", 'Ꚁ', -1},
		{"  Ꚁ", 'Ꚁ', 2},
		{"  a", 'Ꚁ', -1},
		{strings.Repeat("Ꙁ", 64) + "Ꚁ", 'Ꚁ', 192}, // test cutover
		{strings.Repeat("𡋀", 64) + "𡌀", '𣌀', -1},  // '𡌀' and '𣌀' share the same last two bytes

		// 4 bytes
		{"𡌀", '𡌀', 0},
		{"a", '𡌀', -1},
		{"  𡌀", '𡌀', 2},
		{"  a", '𡌀', -1},
		{strings.Repeat("𡋀", 64) + "𡌀", '𡌀', 256}, // test cutover
		{strings.Repeat("𡋀", 64), '𡌀', -1},

		// Test the cutover to bytealg.IndexString when it is triggered in
		// the middle of rune that contains consecutive runs of equal bytes.
		{"aaaaaKKKK\U000bc104", '\U000bc104', 17}, // cutover: (n + 16) / 8
		{"aaaaaKKKK鄄", '鄄', 17},
		{"aaKKKKKa\U000bc104", '\U000bc104', 18}, // cutover: 4 + n>>4
		{"aaKKKKKa鄄", '鄄', 18},
	}
	for _, tt := range tests {
		atomic.AddInt64(&checks, 1)
		if got := strings.IndexRune(tt.in, tt.rune); got != tt.want {
			t.Errorf("IndexRune(%q, %d) = %v; want %v", tt.in, tt.rune, got, tt.want)
		}
	}

	// Make sure we trigger the cutover and string(rune) conversion.
	haystack := "test" + strings.Repeat("𡋀", 32) + "𡌀"
	allocs := allocsPerRun(1000, func() {
		atomic.AddInt64(&checks, 1)
		if i := strings.IndexRune(haystack, 's'); i != 2 {
			t.Fatalf("'s' at %d; want 2", i)
		}
		atomic.AddInt64(&checks, 1)
		if i := strings.IndexRune(haystack, '𡌀'); i != 132 {
			t.Fatalf("'𡌀' at %d; want 4", i)
		}
	})
	_ = allocs // gocpp does not count heap allocations
}

const benchmarkString = "some_text=some☺value"


var benchmarkLongString = strings.Repeat(" ", 100) + benchmarkString






type SplitTest struct {
	s   string
	sep string
	n   int
	a   []string
}

var splittests = []SplitTest{
	{"", "", -1, []string{}},
	{abcd, "", 2, []string{"a", "bcd"}},
	{abcd, "", 4, []string{"a", "b", "c", "d"}},
	{abcd, "", -1, []string{"a", "b", "c", "d"}},
	{faces, "", -1, []string{"☺", "☻", "☹"}},
	{faces, "", 3, []string{"☺", "☻", "☹"}},
	{faces, "", 17, []string{"☺", "☻", "☹"}},
	{"☺�☹", "", -1, []string{"☺", "�", "☹"}},
	{abcd, "a", 0, nil},
	{abcd, "a", -1, []string{"", "bcd"}},
	{abcd, "z", -1, []string{"abcd"}},
	{commas, ",", -1, []string{"1", "2", "3", "4"}},
	{dots, "...", -1, []string{"1", ".2", ".3", ".4"}},
	{faces, "☹", -1, []string{"☺☻", ""}},
	{faces, "~", -1, []string{faces}},
	{"1 2 3 4", " ", 3, []string{"1", "2", "3 4"}},
	{"1 2", " ", 3, []string{"1", "2"}},
	{"", "T", math.MaxInt / 4, []string{""}},
	{"\xff-\xff", "", -1, []string{"\xff", "-", "\xff"}},
	{"\xff-\xff", "-", -1, []string{"\xff", "\xff"}},
}

func TestSplit(t *testT) {
	for _, tt := range splittests {
		a := strings.SplitN(tt.s, tt.sep, tt.n)
		atomic.AddInt64(&checks, 1)
		if !slicesEqual(a, tt.a) {
			t.Errorf("Split(%q, %q, %d) = %v; want %v", tt.s, tt.sep, tt.n, a, tt.a)
			continue
		}
		atomic.AddInt64(&checks, 1)
		if tt.n == 0 {
			continue
		}
		s := strings.Join(a, tt.sep)
		atomic.AddInt64(&checks, 1)
		if s != tt.s {
			t.Errorf("Join(Split(%q, %q, %d), %q) = %q", tt.s, tt.sep, tt.n, tt.sep, s)
		}
		atomic.AddInt64(&checks, 1)
		if tt.n < 0 {
			b := strings.Split(tt.s, tt.sep)
			atomic.AddInt64(&checks, 1)
			if !slicesEqual(a, b) {
				t.Errorf("Split disagrees with SplitN(%q, %q, %d) = %v; want %v", tt.s, tt.sep, tt.n, b, a)
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
		a := strings.SplitAfterN(tt.s, tt.sep, tt.n)
		atomic.AddInt64(&checks, 1)
		if !slicesEqual(a, tt.a) {
			t.Errorf(`Split(%q, %q, %d) = %v; want %v`, tt.s, tt.sep, tt.n, a, tt.a)
			continue
		}
		s := strings.Join(a, "")
		atomic.AddInt64(&checks, 1)
		if s != tt.s {
			t.Errorf(`Join(Split(%q, %q, %d), %q) = %q`, tt.s, tt.sep, tt.n, tt.sep, s)
		}
		atomic.AddInt64(&checks, 1)
		if tt.n < 0 {
			b := strings.SplitAfter(tt.s, tt.sep)
			atomic.AddInt64(&checks, 1)
			if !slicesEqual(a, b) {
				t.Errorf("SplitAfter disagrees with SplitAfterN(%q, %q, %d) = %v; want %v", tt.s, tt.sep, tt.n, b, a)
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
	{"\u2000", []string{}},
	{"  abc  ", []string{"abc"}},
	{"1 2 3 4", []string{"1", "2", "3", "4"}},
	{"1  2  3  4", []string{"1", "2", "3", "4"}},
	{"1\t\t2\t\t3\t4", []string{"1", "2", "3", "4"}},
	{"1\u20002\u20013\u20024", []string{"1", "2", "3", "4"}},
	{"\u2000\u2001\u2002", []string{}},
	{"\n™\t™\n", []string{"™", "™"}},
	{"\n\u20001™2\u2000 \u2001 ™", []string{"1™2", "™"}},
	{"\n1\uFFFD \uFFFD2\u20003\uFFFD4", []string{"1\uFFFD", "\uFFFD2", "3\uFFFD4"}},
	{"1\xFF\u2000\xFF2\xFF \xFF", []string{"1\xFF", "\xFF2\xFF", "\xFF"}},
	{faces, []string{faces}},
}

func TestFields(t *testT) {
	for _, tt := range fieldstests {
		a := strings.Fields(tt.s)
		atomic.AddInt64(&checks, 1)
		if !slicesEqual(a, tt.a) {
			t.Errorf("Fields(%q) = %v; want %v", tt.s, a, tt.a)
			continue
		}
	}
}

var FieldsFuncTests = []FieldsTest{
	{"", []string{}},
	{"XX", []string{}},
	{"XXhiXXX", []string{"hi"}},
	{"aXXbXXXcX", []string{"a", "b", "c"}},
}

func TestFieldsFunc(t *testT) {
	for _, tt := range fieldstests {
		a := strings.FieldsFunc(tt.s, unicode.IsSpace)
		atomic.AddInt64(&checks, 1)
		if !slicesEqual(a, tt.a) {
			t.Errorf("FieldsFunc(%q, unicode.IsSpace) = %v; want %v", tt.s, a, tt.a)
			continue
		}
	}
	pred := func(c rune) bool { return c == 'X' }
	for _, tt := range FieldsFuncTests {
		a := strings.FieldsFunc(tt.s, pred)
		atomic.AddInt64(&checks, 1)
		if !slicesEqual(a, tt.a) {
			t.Errorf("FieldsFunc(%q) = %v, want %v", tt.s, a, tt.a)
		}
	}
}

// Test case for any function which accepts and returns a single string.
type StringTest struct {
	in, out string
}

// Execute f on each test case.  funcName should be the name of f; it's used
// in failure reports.
func runStringTests(t *testT, f func(string) string, funcName string, testCases []StringTest) {
	for _, tc := range testCases {
		actual := f(tc.in)
		atomic.AddInt64(&checks, 1)
		if actual != tc.out {
			t.Errorf("%s(%q) = %q; want %q", funcName, tc.in, actual, tc.out)
		}
	}
}

var upperTests = []StringTest{
	{"", ""},
	{"ONLYUPPER", "ONLYUPPER"},
	{"abc", "ABC"},
	{"AbC123", "ABC123"},
	{"azAZ09_", "AZAZ09_"},
	{"longStrinGwitHmixofsmaLLandcAps", "LONGSTRINGWITHMIXOFSMALLANDCAPS"},
	{"RENAN BASTOS 93 AOSDAJDJAIDJAIDAJIaidsjjaidijadsjiadjiOOKKO", "RENAN BASTOS 93 AOSDAJDJAIDJAIDAJIAIDSJJAIDIJADSJIADJIOOKKO"},
	{"long\u0250string\u0250with\u0250nonascii\u2C6Fchars", "LONG\u2C6FSTRING\u2C6FWITH\u2C6FNONASCII\u2C6FCHARS"},
	{"\u0250\u0250\u0250\u0250\u0250", "\u2C6F\u2C6F\u2C6F\u2C6F\u2C6F"}, // grows one byte per char
	{"a\u0080\U0010FFFF", "A\u0080\U0010FFFF"},                           // test utf8.RuneSelf and utf8.MaxRune
}

var lowerTests = []StringTest{
	{"", ""},
	{"abc", "abc"},
	{"AbC123", "abc123"},
	{"azAZ09_", "azaz09_"},
	{"longStrinGwitHmixofsmaLLandcAps", "longstringwithmixofsmallandcaps"},
	{"renan bastos 93 AOSDAJDJAIDJAIDAJIaidsjjaidijadsjiadjiOOKKO", "renan bastos 93 aosdajdjaidjaidajiaidsjjaidijadsjiadjiookko"},
	{"LONG\u2C6FSTRING\u2C6FWITH\u2C6FNONASCII\u2C6FCHARS", "long\u0250string\u0250with\u0250nonascii\u0250chars"},
	{"\u2C6D\u2C6D\u2C6D\u2C6D\u2C6D", "\u0251\u0251\u0251\u0251\u0251"}, // shrinks one byte per char
	{"A\u0080\U0010FFFF", "a\u0080\U0010FFFF"},                           // test utf8.RuneSelf and utf8.MaxRune
}

const space = "\t\v\r\f\n\u0085\u00a0\u2000\u3000"

var trimSpaceTests = []StringTest{
	{"", ""},
	{"abc", "abc"},
	{space + "abc" + space, "abc"},
	{" ", ""},
	{" \t\r\n \t\t\r\r\n\n ", ""},
	{" \t\r\n x\t\t\r\r\n\n ", "x"},
	{" \u2000\t\r\n x\t\t\r\r\ny\n \u3000", "x\t\t\r\r\ny"},
	{"1 \t\r\n2", "1 \t\r\n2"},
	{" x\x80", "x\x80"},
	{" x\xc0", "x\xc0"},
	{"x \xc0\xc0 ", "x \xc0\xc0"},
	{"x \xc0", "x \xc0"},
	{"x \xc0 ", "x \xc0"},
	{"x \xc0\xc0 ", "x \xc0\xc0"},
	{"x ☺\xc0\xc0 ", "x ☺\xc0\xc0"},
	{"x ☺ ", "x ☺"},
}

func tenRunes(ch rune) string {
	r := make([]rune, 10)
	for i := range r {
		r[i] = ch
	}
	return string(r)
}

// User-defined self-inverse mapping function
func rot13(r rune) rune {
	step := rune(13)
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
	maxRune := func(rune) rune { return unicode.MaxRune }
	m := strings.Map(maxRune, a)
	expect := tenRunes(unicode.MaxRune)
	atomic.AddInt64(&checks, 1)
	if m != expect {
		t.Errorf("growing: expected %q got %q", expect, m)
	}

	// 2. Shrink
	minRune := func(rune) rune { return 'a' }
	m = strings.Map(minRune, tenRunes(unicode.MaxRune))
	expect = a
	atomic.AddInt64(&checks, 1)
	if m != expect {
		t.Errorf("shrinking: expected %q got %q", expect, m)
	}

	// 3. Rot13
	m = strings.Map(rot13, "a to zed")
	expect = "n gb mrq"
	atomic.AddInt64(&checks, 1)
	if m != expect {
		t.Errorf("rot13: expected %q got %q", expect, m)
	}

	// 4. Rot13^2
	m = strings.Map(rot13, strings.Map(rot13, "a to zed"))
	expect = "a to zed"
	atomic.AddInt64(&checks, 1)
	if m != expect {
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
	m = strings.Map(dropNotLatin, "Hello, 세계")
	expect = "Hello"
	atomic.AddInt64(&checks, 1)
	if m != expect {
		t.Errorf("drop: expected %q got %q", expect, m)
	}

	// 6. Identity
	identity := func(r rune) rune {
		return r
	}
	orig := "Input string that we expect not to be copied."
	m = strings.Map(identity, orig)
	atomic.AddInt64(&checks, 1)
	if unsafe.StringData(orig) != unsafe.StringData(m) {
		t.Error("unexpected copy during identity map")
	}

	// 7. Handle invalid UTF-8 sequence
	replaceNotLatin := func(r rune) rune {
		atomic.AddInt64(&checks, 1)
		if unicode.Is(unicode.Latin, r) {
			return r
		}
		return utf8.RuneError
	}
	m = strings.Map(replaceNotLatin, "Hello\255World")
	expect = "Hello\uFFFDWorld"
	atomic.AddInt64(&checks, 1)
	if m != expect {
		t.Errorf("replace invalid sequence: expected %q got %q", expect, m)
	}

	// 8. Check utf8.RuneSelf and utf8.MaxRune encoding
	encode := func(r rune) rune {
		switch r {
		case utf8.RuneSelf:
			return unicode.MaxRune
		case unicode.MaxRune:
			return utf8.RuneSelf
		}
		return r
	}
	s := string(rune(utf8.RuneSelf)) + string(utf8.MaxRune)
	r := string(utf8.MaxRune) + string(rune(utf8.RuneSelf)) // reverse of s
	m = strings.Map(encode, s)
	atomic.AddInt64(&checks, 1)
	if m != r {
		t.Errorf("encoding not handled correctly: expected %q got %q", r, m)
	}
	m = strings.Map(encode, r)
	atomic.AddInt64(&checks, 1)
	if m != s {
		t.Errorf("encoding not handled correctly: expected %q got %q", s, m)
	}

	// 9. Check mapping occurs in the front, middle and back
	trimSpaces := func(r rune) rune {
		atomic.AddInt64(&checks, 1)
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}
	m = strings.Map(trimSpaces, "   abc    123   ")
	expect = "abc123"
	atomic.AddInt64(&checks, 1)
	if m != expect {
		t.Errorf("trimSpaces: expected %q got %q", expect, m)
	}
}

func TestToUpper(t *testT) { runStringTests(t, strings.ToUpper, "ToUpper", upperTests) }

func TestToLower(t *testT) { runStringTests(t, strings.ToLower, "ToLower", lowerTests) }

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
		got := strings.ToValidUTF8(tc.in, tc.repl)
		atomic.AddInt64(&checks, 1)
		if got != tc.out {
			t.Errorf("ToValidUTF8(%q, %q) = %q; want %q", tc.in, tc.repl, got, tc.out)
		}
	}
}




func TestSpecialCase(t *testT) {
	lower := "abcçdefgğhıijklmnoöprsştuüvyz"
	upper := "ABCÇDEFGĞHIİJKLMNOÖPRSŞTUÜVYZ"
	u := strings.ToUpperSpecial(unicode.TurkishCase, upper)
	atomic.AddInt64(&checks, 1)
	if u != upper {
		t.Errorf("Upper(upper) is %s not %s", u, upper)
	}
	u = strings.ToUpperSpecial(unicode.TurkishCase, lower)
	atomic.AddInt64(&checks, 1)
	if u != upper {
		t.Errorf("Upper(lower) is %s not %s", u, upper)
	}
	l := strings.ToLowerSpecial(unicode.TurkishCase, lower)
	atomic.AddInt64(&checks, 1)
	if l != lower {
		t.Errorf("Lower(lower) is %s not %s", l, lower)
	}
	l = strings.ToLowerSpecial(unicode.TurkishCase, upper)
	atomic.AddInt64(&checks, 1)
	if l != lower {
		t.Errorf("Lower(upper) is %s not %s", l, lower)
	}
}

func TestTrimSpace(t *testT) { runStringTests(t, strings.TrimSpace, "TrimSpace", trimSpaceTests) }

var trimTests = []struct {
	f            string
	in, arg, out string
}{
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

func TestTrim(t *testT) {
	for _, tc := range trimTests {
		name := tc.f
		var f func(string, string) string
		switch name {
		case "Trim":
			f = strings.Trim
		case "TrimLeft":
			f = strings.TrimLeft
		case "TrimRight":
			f = strings.TrimRight
		case "TrimPrefix":
			f = strings.TrimPrefix
		case "TrimSuffix":
			f = strings.TrimSuffix
		default:
			t.Errorf("Undefined trim function %s", name)
		}
		actual := f(tc.in, tc.arg)
		atomic.AddInt64(&checks, 1)
		if actual != tc.out {
			t.Errorf("%s(%q, %q) = %q; want %q", name, tc.in, tc.arg, actual, tc.out)
		}
	}
}



type predicate struct {
	f    func(rune) bool
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

func not(p predicate) predicate {
	return predicate{
		func(r rune) bool {
			return !p.f(r)
		},
		"not " + p.name,
	}
}

var trimFuncTests = []struct {
	f        predicate
	in       string
	trimOut  string
	leftOut  string
	rightOut string
}{
	{isSpace, space + " hello " + space,
		"hello",
		"hello " + space,
		space + " hello"},
	{isDigit, "\u0e50\u0e5212hello34\u0e50\u0e51",
		"hello",
		"hello34\u0e50\u0e51",
		"\u0e50\u0e5212hello"},
	{isUpper, "\u2C6F\u2C6F\u2C6F\u2C6FABCDhelloEF\u2C6F\u2C6FGH\u2C6F\u2C6F",
		"hello",
		"helloEF\u2C6F\u2C6FGH\u2C6F\u2C6F",
		"\u2C6F\u2C6F\u2C6F\u2C6FABCDhello"},
	{not(isSpace), "hello" + space + "hello",
		space,
		space + "hello",
		"hello" + space},
	{not(isDigit), "hello\u0e50\u0e521234\u0e50\u0e51helo",
		"\u0e50\u0e521234\u0e50\u0e51",
		"\u0e50\u0e521234\u0e50\u0e51helo",
		"hello\u0e50\u0e521234\u0e50\u0e51"},
	{isValidRune, "ab\xc0a\xc0cd",
		"\xc0a\xc0",
		"\xc0a\xc0cd",
		"ab\xc0a\xc0"},
	{not(isValidRune), "\xc0a\xc0",
		"a",
		"a\xc0",
		"\xc0a"},
	{isSpace, "",
		"",
		"",
		""},
	{isSpace, " ",
		"",
		"",
		""},
}

func TestTrimFunc(t *testT) {
	for _, tc := range trimFuncTests {
		trimmers := []struct {
			name string
			trim func(s string, f func(r rune) bool) string
			out  string
		}{
			{"TrimFunc", strings.TrimFunc, tc.trimOut},
			{"TrimLeftFunc", strings.TrimLeftFunc, tc.leftOut},
			{"TrimRightFunc", strings.TrimRightFunc, tc.rightOut},
		}
		for _, trimmer := range trimmers {
			actual := trimmer.trim(tc.in, tc.f.f)
			atomic.AddInt64(&checks, 1)
			if actual != trimmer.out {
				t.Errorf("%s(%q, %q) = %q; want %q", trimmer.name, tc.in, tc.f.name, actual, trimmer.out)
			}
		}
	}
}

var indexFuncTests = []struct {
	in          string
	f           predicate
	first, last int
}{
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
	{"\x80\x80\x80\x80", not(isValidRune), 0, 3},
}

func TestIndexFunc(t *testT) {
	for _, tc := range indexFuncTests {
		first := strings.IndexFunc(tc.in, tc.f.f)
		atomic.AddInt64(&checks, 1)
		if first != tc.first {
			t.Errorf("IndexFunc(%q, %s) = %d; want %d", tc.in, tc.f.name, first, tc.first)
		}
		last := strings.LastIndexFunc(tc.in, tc.f.f)
		atomic.AddInt64(&checks, 1)
		if last != tc.last {
			t.Errorf("LastIndexFunc(%q, %s) = %d; want %d", tc.in, tc.f.name, last, tc.last)
		}
	}
}

func equal(m string, s1, s2 string, t *testT) bool {
	atomic.AddInt64(&checks, 1)
	if s1 == s2 {
		return true
	}
	e1 := strings.Split(s1, "")
	e2 := strings.Split(s2, "")
	for i, c1 := range e1 {
		atomic.AddInt64(&checks, 1)
		if i >= len(e2) {
			break
		}
		r1, _ := utf8.DecodeRuneInString(c1)
		r2, _ := utf8.DecodeRuneInString(e2[i])
		atomic.AddInt64(&checks, 1)
		if r1 != r2 {
			t.Errorf("%s diff at %d: U+%04X U+%04X", m, i, r1, r2)
		}
	}
	return false
}

func TestCaseConsistency(t *testT) {
	// Make a string of all the runes.
	numRunes := int(unicode.MaxRune + 1)
	atomic.AddInt64(&checks, 1)
	if true {
		numRunes = 1000
	}
	a := make([]rune, numRunes)
	for i := range a {
		a[i] = rune(i)
	}
	s := string(a)
	// convert the cases.
	upper := strings.ToUpper(s)
	lower := strings.ToLower(s)

	// Consistency checks
	atomic.AddInt64(&checks, 1)
	if n := utf8.RuneCountInString(upper); n != numRunes {
		t.Error("rune count wrong in upper:", n)
	}
	atomic.AddInt64(&checks, 1)
	if n := utf8.RuneCountInString(lower); n != numRunes {
		t.Error("rune count wrong in lower:", n)
	}
	atomic.AddInt64(&checks, 1)
	if !equal("ToUpper(upper)", strings.ToUpper(upper), upper, t) {
		t.Error("ToUpper(upper) consistency fail")
	}
	atomic.AddInt64(&checks, 1)
	if !equal("ToLower(lower)", strings.ToLower(lower), lower, t) {
		t.Error("ToLower(lower) consistency fail")
	}
	/*
		  These fail because of non-one-to-oneness of the data, such as multiple
		  upper case 'I' mapping to 'i'.  We comment them out but keep them for
		  interest.
		  For instance: CAPITAL LETTER I WITH DOT ABOVE:
			unicode.ToUpper(unicode.ToLower('\u0130')) != '\u0130'

		atomic.AddInt64(&checks, 1)
		if !equal("ToUpper(lower)", ToUpper(lower), upper, t) {
			t.Error("ToUpper(lower) consistency fail");
		}
		atomic.AddInt64(&checks, 1)
		if !equal("ToLower(upper)", ToLower(upper), lower, t) {
			t.Error("ToLower(upper) consistency fail");
		}
	*/
}

var longString = "a" + string(make([]byte, 1<<16)) + "z"
var longSpaces = func() string {
	b := make([]byte, 200)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}()

var RepeatTests = []struct {
	in, out string
	count   int
}{
	{"", "", 0},
	{"", "", 1},
	{"", "", 2},
	{"-", "", 0},
	{"-", "-", 1},
	{"-", "----------", 10},
	{"abc ", "abc abc abc ", 3},
	{" ", " ", 1},
	{"--", "----", 2},
	{"===", "======", 2},
	{"000", "000000000", 3},
	{"\t\t\t\t", "\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t", 4},
	{" ", longSpaces, len(longSpaces)},
	// Tests for results over the chunkLimit
	{string(rune(0)), string(make([]byte, 1<<16)), 1 << 16},
	{longString, longString + longString, 2},
}

func TestRepeat(t *testT) {
	for _, tt := range RepeatTests {
		a := strings.Repeat(tt.in, tt.count)
		atomic.AddInt64(&checks, 1)
		if !equal("Repeat(s)", a, tt.out, t) {
			t.Errorf("Repeat(%v, %d) = %v; want %v", tt.in, tt.count, a, tt.out)
			continue
		}
	}
}

func repeat(s string, count int) (err error) {
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

	strings.Repeat(s, count)

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
			err := repeat(tt.s, tt.count)
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

func runesEqual(a, b []rune) bool {
	atomic.AddInt64(&checks, 1)
	if len(a) != len(b) {
		return false
	}
	for i, r := range a {
		atomic.AddInt64(&checks, 1)
		if r != b[i] {
			return false
		}
	}
	return true
}

var RunesTests = []struct {
	in    string
	out   []rune
	lossy bool
}{
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
		a := []rune(tt.in)
		atomic.AddInt64(&checks, 1)
		if !runesEqual(a, tt.out) {
			t.Errorf("[]rune(%q) = %v; want %v", tt.in, a, tt.out)
			continue
		}
		atomic.AddInt64(&checks, 1)
		if !tt.lossy {
			// can only test reassembly if we didn't lose information
			s := string(a)
			atomic.AddInt64(&checks, 1)
			if s != tt.in {
				t.Errorf("string([]rune(%q)) = %x; want %x", tt.in, s, tt.in)
			}
		}
	}
}

func TestReadByte(t *testT) {
	testStrings := []string{"", abcd, faces, commas}
	for _, s := range testStrings {
		reader := strings.NewReader(s)
		atomic.AddInt64(&checks, 1)
		if e := reader.UnreadByte(); e == nil {
			t.Errorf("Unreading %q at beginning: expected error", s)
		}
		var res bytes.Buffer
		for {
			b, e := reader.ReadByte()
			atomic.AddInt64(&checks, 1)
			if e == io.EOF {
				break
			}
			atomic.AddInt64(&checks, 1)
			if e != nil {
				t.Errorf("Reading %q: %s", s, e)
				break
			}
			res.WriteByte(b)
			// unread and read again
			e = reader.UnreadByte()
			atomic.AddInt64(&checks, 1)
			if e != nil {
				t.Errorf("Unreading %q: %s", s, e)
				break
			}
			b1, e := reader.ReadByte()
			atomic.AddInt64(&checks, 1)
			if e != nil {
				t.Errorf("Reading %q after unreading: %s", s, e)
				break
			}
			atomic.AddInt64(&checks, 1)
			if b1 != b {
				t.Errorf("Reading %q after unreading: want byte %q, got %q", s, b, b1)
				break
			}
		}
		atomic.AddInt64(&checks, 1)
		if res.String() != s {
			t.Errorf("Reader(%q).ReadByte() produced %q", s, res.String())
		}
	}
}

func TestReadRune(t *testT) {
	testStrings := []string{"", abcd, faces, commas}
	for _, s := range testStrings {
		reader := strings.NewReader(s)
		atomic.AddInt64(&checks, 1)
		if e := reader.UnreadRune(); e == nil {
			t.Errorf("Unreading %q at beginning: expected error", s)
		}
		res := ""
		for {
			r, z, e := reader.ReadRune()
			atomic.AddInt64(&checks, 1)
			if e == io.EOF {
				break
			}
			atomic.AddInt64(&checks, 1)
			if e != nil {
				t.Errorf("Reading %q: %s", s, e)
				break
			}
			res += string(r)
			// unread and read again
			e = reader.UnreadRune()
			atomic.AddInt64(&checks, 1)
			if e != nil {
				t.Errorf("Unreading %q: %s", s, e)
				break
			}
			r1, z1, e := reader.ReadRune()
			atomic.AddInt64(&checks, 1)
			if e != nil {
				t.Errorf("Reading %q after unreading: %s", s, e)
				break
			}
			atomic.AddInt64(&checks, 1)
			if r1 != r {
				t.Errorf("Reading %q after unreading: want rune %q, got %q", s, r, r1)
				break
			}
			atomic.AddInt64(&checks, 1)
			if z1 != z {
				t.Errorf("Reading %q after unreading: want size %d, got %d", s, z, z1)
				break
			}
		}
		atomic.AddInt64(&checks, 1)
		if res != s {
			t.Errorf("Reader(%q).ReadRune() produced %q", s, res)
		}
	}
}

var UnreadRuneErrorTests = []struct {
	name string
	f    func(*strings.Reader)
}{
	{"Read", func(r *strings.Reader) { r.Read([]byte{0}) }},
	{"ReadByte", func(r *strings.Reader) { r.ReadByte() }},
	{"UnreadRune", func(r *strings.Reader) { r.UnreadRune() }},
	{"Seek", func(r *strings.Reader) { r.Seek(0, io.SeekCurrent) }},
	{"WriteTo", func(r *strings.Reader) { r.WriteTo(&bytes.Buffer{}) }},
}

func TestUnreadRuneError(t *testT) {
	for _, tt := range UnreadRuneErrorTests {
		reader := strings.NewReader("0123456789")
		atomic.AddInt64(&checks, 1)
		if _, _, err := reader.ReadRune(); err != nil {
			// should not happen
			t.Fatal(err)
		}
		tt.f(reader)
		err := reader.UnreadRune()
		atomic.AddInt64(&checks, 1)
		if err == nil {
			t.Errorf("Unreading after %s: expected error", tt.name)
		}
	}
}

var ReplaceTests = []struct {
	in       string
	old, new string
	n        int
	out      string
}{
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
		atomic.AddInt64(&checks, 1)
		if !false { // See issue #72973.
			allocs := allocsPerRun(10, func() { strings.Replace(tt.in, tt.old, tt.new, tt.n) })
			atomic.AddInt64(&checks, 1)
			if allocs > 1 {
				t.Errorf("Replace(%q, %q, %q, %d) allocates %.2f objects", tt.in, tt.old, tt.new, tt.n, allocs)
			}
		}
		atomic.AddInt64(&checks, 1)
		if s := strings.Replace(tt.in, tt.old, tt.new, tt.n); s != tt.out {
			t.Errorf("Replace(%q, %q, %q, %d) = %q, want %q", tt.in, tt.old, tt.new, tt.n, s, tt.out)
		}
		atomic.AddInt64(&checks, 1)
		if tt.n == -1 {
			s := strings.ReplaceAll(tt.in, tt.old, tt.new)
			atomic.AddInt64(&checks, 1)
			if s != tt.out {
				t.Errorf("ReplaceAll(%q, %q, %q) = %q, want %q", tt.in, tt.old, tt.new, s, tt.out)
			}
		}
	}
}



var TitleTests = []struct {
	in, out string
}{
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
		if s := strings.Title(tt.in); s != tt.out {
			t.Errorf("Title(%q) = %q, want %q", tt.in, s, tt.out)
		}
	}
}

var ContainsTests = []struct {
	str, substr string
	expected    bool
}{
	{"abc", "bc", true},
	{"abc", "bcd", false},
	{"abc", "", true},
	{"", "a", false},

	// cases to cover code in runtime/asm_amd64.s:indexShortStr
	// 2-byte needle
	{"xxxxxx", "01", false},
	{"01xxxx", "01", true},
	{"xx01xx", "01", true},
	{"xxxx01", "01", true},
	{"01xxxxx"[1:], "01", false},
	{"xxxxx01"[:6], "01", false},
	// 3-byte needle
	{"xxxxxxx", "012", false},
	{"012xxxx", "012", true},
	{"xx012xx", "012", true},
	{"xxxx012", "012", true},
	{"012xxxxx"[1:], "012", false},
	{"xxxxx012"[:7], "012", false},
	// 4-byte needle
	{"xxxxxxxx", "0123", false},
	{"0123xxxx", "0123", true},
	{"xx0123xx", "0123", true},
	{"xxxx0123", "0123", true},
	{"0123xxxxx"[1:], "0123", false},
	{"xxxxx0123"[:8], "0123", false},
	// 5-7-byte needle
	{"xxxxxxxxx", "01234", false},
	{"01234xxxx", "01234", true},
	{"xx01234xx", "01234", true},
	{"xxxx01234", "01234", true},
	{"01234xxxxx"[1:], "01234", false},
	{"xxxxx01234"[:9], "01234", false},
	// 8-byte needle
	{"xxxxxxxxxxxx", "01234567", false},
	{"01234567xxxx", "01234567", true},
	{"xx01234567xx", "01234567", true},
	{"xxxx01234567", "01234567", true},
	{"01234567xxxxx"[1:], "01234567", false},
	{"xxxxx01234567"[:12], "01234567", false},
	// 9-15-byte needle
	{"xxxxxxxxxxxxx", "012345678", false},
	{"012345678xxxx", "012345678", true},
	{"xx012345678xx", "012345678", true},
	{"xxxx012345678", "012345678", true},
	{"012345678xxxxx"[1:], "012345678", false},
	{"xxxxx012345678"[:13], "012345678", false},
	// 16-byte needle
	{"xxxxxxxxxxxxxxxxxxxx", "0123456789ABCDEF", false},
	{"0123456789ABCDEFxxxx", "0123456789ABCDEF", true},
	{"xx0123456789ABCDEFxx", "0123456789ABCDEF", true},
	{"xxxx0123456789ABCDEF", "0123456789ABCDEF", true},
	{"0123456789ABCDEFxxxxx"[1:], "0123456789ABCDEF", false},
	{"xxxxx0123456789ABCDEF"[:20], "0123456789ABCDEF", false},
	// 17-31-byte needle
	{"xxxxxxxxxxxxxxxxxxxxx", "0123456789ABCDEFG", false},
	{"0123456789ABCDEFGxxxx", "0123456789ABCDEFG", true},
	{"xx0123456789ABCDEFGxx", "0123456789ABCDEFG", true},
	{"xxxx0123456789ABCDEFG", "0123456789ABCDEFG", true},
	{"0123456789ABCDEFGxxxxx"[1:], "0123456789ABCDEFG", false},
	{"xxxxx0123456789ABCDEFG"[:21], "0123456789ABCDEFG", false},

	// partial match cases
	{"xx01x", "012", false},                             // 3
	{"xx0123x", "01234", false},                         // 5-7
	{"xx01234567x", "012345678", false},                 // 9-15
	{"xx0123456789ABCDEFx", "0123456789ABCDEFG", false}, // 17-31, issue 15679
}

func TestContains(t *testT) {
	for _, ct := range ContainsTests {
		atomic.AddInt64(&checks, 1)
		if strings.Contains(ct.str, ct.substr) != ct.expected {
			t.Errorf("Contains(%s, %s) = %v, want %v",
				ct.str, ct.substr, !ct.expected, ct.expected)
		}
	}
}

var ContainsAnyTests = []struct {
	str, substr string
	expected    bool
}{
	{"", "", false},
	{"", "a", false},
	{"", "abc", false},
	{"a", "", false},
	{"a", "a", true},
	{"aaa", "a", true},
	{"abc", "xyz", false},
	{"abc", "xcz", true},
	{"a☺b☻c☹d", "uvw☻xyz", true},
	{"aRegExp*", ".(|)*+?^$[]", true},
	{dots + dots + dots, " ", false},
}

func TestContainsAny(t *testT) {
	for _, ct := range ContainsAnyTests {
		atomic.AddInt64(&checks, 1)
		if strings.ContainsAny(ct.str, ct.substr) != ct.expected {
			t.Errorf("ContainsAny(%s, %s) = %v, want %v",
				ct.str, ct.substr, !ct.expected, ct.expected)
		}
	}
}

var ContainsRuneTests = []struct {
	str      string
	r        rune
	expected bool
}{
	{"", 'a', false},
	{"a", 'a', true},
	{"aaa", 'a', true},
	{"abc", 'y', false},
	{"abc", 'c', true},
	{"a☺b☻c☹d", 'x', false},
	{"a☺b☻c☹d", '☻', true},
	{"aRegExp*", '*', true},
}

func TestContainsRune(t *testT) {
	for _, ct := range ContainsRuneTests {
		atomic.AddInt64(&checks, 1)
		if strings.ContainsRune(ct.str, ct.r) != ct.expected {
			t.Errorf("ContainsRune(%q, %q) = %v, want %v",
				ct.str, ct.r, !ct.expected, ct.expected)
		}
	}
}

func TestContainsFunc(t *testT) {
	for _, ct := range ContainsRuneTests {
		atomic.AddInt64(&checks, 1)
		if strings.ContainsFunc(ct.str, func(r rune) bool {
			return ct.r == r
		}) != ct.expected {
			t.Errorf("ContainsFunc(%q, func(%q)) = %v, want %v",
				ct.str, ct.r, !ct.expected, ct.expected)
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
	{"1", "2", false},
	{"utf-8", "US-ASCII", false},
}

func TestEqualFold(t *testT) {
	for _, tt := range EqualFoldTests {
		atomic.AddInt64(&checks, 1)
		if out := strings.EqualFold(tt.s, tt.t); out != tt.out {
			t.Errorf("EqualFold(%#q, %#q) = %v, want %v", tt.s, tt.t, out, tt.out)
		}
		atomic.AddInt64(&checks, 1)
		if out := strings.EqualFold(tt.t, tt.s); out != tt.out {
			t.Errorf("EqualFold(%#q, %#q) = %v, want %v", tt.t, tt.s, out, tt.out)
		}
	}
}


var CountTests = []struct {
	s, sep string
	num    int
}{
	{"", "", 1},
	{"", "notempty", 0},
	{"notempty", "", 9},
	{"smaller", "not smaller", 0},
	{"12345678987654321", "6", 2},
	{"611161116", "6", 3},
	{"notequal", "NotEqual", 0},
	{"equal", "equal", 1},
	{"abc1231231123q", "123", 3},
	{"11111", "11", 2},
}

func TestCount(t *testT) {
	for _, tt := range CountTests {
		atomic.AddInt64(&checks, 1)
		if num := strings.Count(tt.s, tt.sep); num != tt.num {
			t.Errorf("Count(%q, %q) = %d, want %d", tt.s, tt.sep, num, tt.num)
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
		if before, after, found := strings.Cut(tt.s, tt.sep); before != tt.before || after != tt.after || found != tt.found {
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
		if after, found := strings.CutPrefix(tt.s, tt.sep); after != tt.after || found != tt.found {
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
		if before, found := strings.CutSuffix(tt.s, tt.sep); before != tt.before || found != tt.found {
			t.Errorf("CutSuffix(%q, %q) = %q, %v, want %q, %v", tt.s, tt.sep, before, found, tt.before, tt.found)
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
		{"TestIndex", TestIndex},
		{"TestLastIndex", TestLastIndex},
		{"TestIndexAny", TestIndexAny},
		{"TestLastIndexAny", TestLastIndexAny},
		{"TestIndexByte", TestIndexByte},
		{"TestLastIndexByte", TestLastIndexByte},
		{"TestIndexRandom", TestIndexRandom},
		{"TestIndexRune", TestIndexRune},
		{"TestSplit", TestSplit},
		{"TestSplitAfter", TestSplitAfter},
		{"TestFields", TestFields},
		{"TestFieldsFunc", TestFieldsFunc},
		{"TestMap", TestMap},
		{"TestToUpper", TestToUpper},
		{"TestToLower", TestToLower},
		{"TestToValidUTF8", TestToValidUTF8},
		{"TestSpecialCase", TestSpecialCase},
		{"TestTrimSpace", TestTrimSpace},
		{"TestTrim", TestTrim},
		{"TestTrimFunc", TestTrimFunc},
		{"TestIndexFunc", TestIndexFunc},
		{"TestCaseConsistency", TestCaseConsistency},
		{"TestRepeat", TestRepeat},
		{"TestRepeatCatchesOverflow", TestRepeatCatchesOverflow},
		{"TestRunes", TestRunes},
		{"TestReadByte", TestReadByte},
		{"TestReadRune", TestReadRune},
		{"TestUnreadRuneError", TestUnreadRuneError},
		{"TestReplace", TestReplace},
		{"TestTitle", TestTitle},
		{"TestContains", TestContains},
		{"TestContainsAny", TestContainsAny},
		{"TestContainsRune", TestContainsRune},
		{"TestContainsFunc", TestContainsFunc},
		{"TestEqualFold", TestEqualFold},
		{"TestCount", TestCount},
		{"TestCut", TestCut},
		{"TestCutPrefix", TestCutPrefix},
		{"TestCutSuffix", TestCutSuffix},
	}
	runTests(tests_)
}

// Output:
// ok TestIndex subtests: 0 checks: 91
// ok TestLastIndex subtests: 0 checks: 13
// ok TestIndexAny subtests: 0 checks: 17
// ok TestLastIndexAny subtests: 0 checks: 17
// ok TestIndexByte subtests: 0 checks: 102
// ok TestLastIndexByte subtests: 0 checks: 6
// ok TestIndexRandom subtests: 0 checks: 275528
// ok TestIndexRune subtests: 0 checks: 2041
// ok TestSplit subtests: 0 checks: 90
// ok TestSplitAfter subtests: 0 checks: 47
// ok TestFields subtests: 0 checks: 15
// ok TestFieldsFunc subtests: 0 checks: 19
// ok TestMap subtests: 0 checks: 76
// ok TestToUpper subtests: 0 checks: 10
// ok TestToLower subtests: 0 checks: 9
// ok TestToValidUTF8 subtests: 0 checks: 14
// ok TestSpecialCase subtests: 0 checks: 4
// ok TestTrimSpace subtests: 0 checks: 16
// ok TestTrim subtests: 0 checks: 29
// ok TestTrimFunc subtests: 0 checks: 27
// ok TestIndexFunc subtests: 0 checks: 34
// ok TestCaseConsistency subtests: 0 checks: 7
// ok TestRepeat subtests: 0 checks: 30
// ok TestRepeatCatchesOverflow subtests: 0 checks: 25
// ok TestRunes subtests: 0 checks: 44
// ok TestReadByte subtests: 0 checks: 112
// ok TestReadRune subtests: 0 checks: 96
// ok TestUnreadRuneError subtests: 0 checks: 10
// ok TestReplace subtests: 0 checks: 87
// ok TestTitle subtests: 0 checks: 9
// ok TestContains subtests: 0 checks: 56
// ok TestContainsAny subtests: 0 checks: 11
// ok TestContainsRune subtests: 0 checks: 8
// ok TestContainsFunc subtests: 0 checks: 8
// ok TestEqualFold subtests: 0 checks: 26
// ok TestCount subtests: 0 checks: 10
// ok TestCut subtests: 0 checks: 8
// ok TestCutPrefix subtests: 0 checks: 6
// ok TestCutSuffix subtests: 0 checks: 6
// tests: 39 failed: 0
