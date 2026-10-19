// phase: 09
// std-internal: tests internal/bytealg, which only the standard library may import

// internal/bytealg at its edges: nil and empty arguments, matches at the first and the last byte,
// bytes above 127 (the order is of unsigned bytes), views that start inside a bigger array or
// string (so the data pointer is not aligned), every length around the 8-byte words and the
// 8160-byte blocks of Count, a megabyte, and MakeNoZero's panic. The expected values are written
// by hand; tests/twin_test.go (GOCPP_RUN_VS_GO=1) builds the same file with real Go, with the functions replaced
// by their equivalents in bytes and strings.
package main

import (
	"fmt"
	"internal/bytealg"
)

var checks, failures int

func check(what string, got, want int) {
	checks++
	if got != want {
		failures++
		fmt.Printf("FAIL %s: got %d, want %d\n", what, got, want)
	}
}

func checkBool(what string, got, want bool) {
	checks++
	if got != want {
		failures++
		fmt.Printf("FAIL %s: got %v, want %v\n", what, got, want)
	}
}

func nilAndEmpty() {
	var nb []byte
	eb := []byte{}
	for _, b := range [][]byte{nb, eb} {
		check("IndexByte empty", bytealg.IndexByte(b, 0), -1)
		check("LastIndexByte empty", bytealg.LastIndexByte(b, 0), -1)
		check("Count empty", bytealg.Count(b, 0), 0)
		check("Index empty haystack", bytealg.Index(b, []byte("ab")), -1)
		check("Index empty needle", bytealg.Index([]byte("ab"), b), 0)
		check("Compare empty", bytealg.Compare(b, nb), 0)
		check("Compare empty, nonempty", bytealg.Compare(b, []byte{0}), -1)
		check("Compare nonempty, empty", bytealg.Compare([]byte{0}, b), 1)
		checkBool("Equal empty", bytealg.Equal(b, nb), true)
		checkBool("Equal empty, nonempty", bytealg.Equal(b, []byte{0}), false)
	}
	check("IndexByteString empty", bytealg.IndexByteString("", 'a'), -1)
	check("LastIndexByteString empty", bytealg.LastIndexByteString("", 'a'), -1)
	check("CountString empty", bytealg.CountString("", 'a'), 0)
	check("IndexString empty haystack", bytealg.IndexString("", "ab"), -1)
	check("IndexString empty needle", bytealg.IndexString("ab", ""), 0)
	check("CompareString empty", bytealg.CompareString("", ""), 0)
	check("CompareString empty, nonempty", bytealg.CompareString("", "a"), -1)
	check("CompareString nonempty, empty", bytealg.CompareString("a", ""), 1)
	check("MakeNoZero(0)", len(bytealg.MakeNoZero(0)), 0)
	checkBool("MakeNoZero(0) is not nil", bytealg.MakeNoZero(0) != nil, true)
}

func positions() {
	// A byte at each position of arrays of every length up to 70, in views that start at each of
	// the first 8 bytes of a bigger array.
	big := make([]byte, 100)
	for off := 0; off < 8; off++ {
		for n := 1; n <= 70; n++ {
			for pos := 0; pos < n; pos++ {
				v := big[off : off+n]
				for i := range v {
					v[i] = 'a'
				}
				v[pos] = 'z'
				check("IndexByte position", bytealg.IndexByte(v, 'z'), pos)
				check("LastIndexByte position", bytealg.LastIndexByte(v, 'z'), pos)
				check("IndexByteString position", bytealg.IndexByteString(string(v), 'z'), pos)
				check("Count position", bytealg.Count(v, 'z'), 1)
				check("Count the others", bytealg.Count(v, 'a'), n-1)
			}
		}
	}
}

func counts() {
	// Count for every length around the boundaries of 8 bytes and of its 8160-byte blocks (and 2040, the old 255 words), with all
	// bytes equal to the counted one, none of them, and every third; from each of 8 offsets.
	big := make([]byte, 16400)
	lengths := []int{}
	for n := 0; n <= 20; n++ {
		lengths = append(lengths, n)
	}
	for _, edge := range []int{2040, 4080, 8160, 16320} {
		for d := -9; d <= 9; d++ {
			lengths = append(lengths, edge+d)
		}
	}
	for _, n := range lengths {
		for off := 0; off < 8; off++ {
			v := big[off : off+n]
			for i := range v {
				v[i] = 7
			}
			check("Count all", bytealg.Count(v, 7), n)
			check("Count none", bytealg.Count(v, 8), 0)
			for i := range v {
				v[i] = byte(i % 3)
			}
			want := (n + 2) / 3
			check("Count every third", bytealg.Count(v, 0), want)
			check("CountString every third", bytealg.CountString(string(v), 0), want)
			for i := range v {
				v[i] = 255
			}
			check("Count 255", bytealg.Count(v, 255), n)
			check("Count 255, not 127", bytealg.Count(v, 127), 0)
		}
	}
}

func order() {
	check("Compare high byte", bytealg.Compare([]byte{0xff}, []byte{0x01}), 1)
	check("Compare high byte, reversed", bytealg.Compare([]byte{0x01}, []byte{0xff}), -1)
	check("Compare prefix", bytealg.Compare([]byte("abc"), []byte("abcd")), -1)
	check("Compare equal", bytealg.Compare([]byte("abc"), []byte("abc")), 0)
	check("CompareString high byte", bytealg.CompareString("\xff", "\x7f"), 1)
	check("CompareString NUL", bytealg.CompareString("a\x00b", "a\x00c"), -1)
	check("CompareString NUL is not the end", bytealg.CompareString("a\x00", "a"), 1)
	long := make([]byte, 1000)
	long2 := make([]byte, 1000)
	check("Compare equal, long", bytealg.Compare(long, long2), 0)
	long2[999] = 1
	check("Compare differ at the end", bytealg.Compare(long, long2), -1)
	checkBool("Equal differ at the end", bytealg.Equal(long, long2), false)
	long2[999] = 0
	checkBool("Equal long", bytealg.Equal(long, long2), true)
	checkBool("Equal the same slice", bytealg.Equal(long, long), true)
	checkBool("Equal, one shorter", bytealg.Equal(long, long2[:999]), false)
}

func searches() {
	hay := "the quick brown fox jumps over the lazy dog"
	check("IndexString first", bytealg.IndexString(hay, "the"), 0)
	check("IndexString last", bytealg.IndexString(hay, "dog"), 40)
	check("IndexString middle", bytealg.IndexString(hay, "jumps"), 20)
	check("IndexString second the", bytealg.IndexString(hay[1:], "the"), 30)
	check("IndexString missing", bytealg.IndexString(hay, "cat"), -1)
	check("IndexString longer than the haystack", bytealg.IndexString("ab", "abc"), -1)
	check("IndexString the whole", bytealg.IndexString(hay, hay), 0)
	check("IndexString NUL", bytealg.IndexString("a\x00b\x00c", "b\x00"), 2)
	check("Index", bytealg.Index([]byte(hay), []byte("lazy")), 35)
	check("Index of a view", bytealg.Index([]byte(hay)[3:], []byte("er")), 25)
	check("IndexRabinKarp", bytealg.IndexRabinKarp(hay, "over"), 26)
	check("IndexRabinKarp missing", bytealg.IndexRabinKarp(hay, "overt"), -1)
	check("IndexRabinKarp bytes", bytealg.IndexRabinKarp([]byte(hay), []byte("fox")), 16)
	check("LastIndexRabinKarp", bytealg.LastIndexRabinKarp(hay, "the"), 31)
	check("LastIndexRabinKarp missing", bytealg.LastIndexRabinKarp(hay, "cat"), -1)
	check("LastIndexRabinKarp bytes", bytealg.LastIndexRabinKarp([]byte(hay), []byte("o")), 41)
	check("IndexRabinKarp the whole", bytealg.IndexRabinKarp("abc", "abc"), 0)
	// A needle that is a long run of one byte: the search must not slow down to a crawl or miss it.
	run := make([]byte, 5000)
	for i := range run {
		run[i] = 'a'
	}
	run[4990] = 'b'
	needle := make([]byte, 60)
	for i := range needle {
		needle[i] = 'a'
	}
	check("Index a run of a", bytealg.Index(run, needle), 0)
	check("Index a run of a, after b", bytealg.Index(run[4991:], needle), -1)
	check("IndexRabinKarp a run of a", bytealg.IndexRabinKarp(run[4980:], needle[:11]), -1)
}

func constants() {
	check("MaxBruteForce", bytealg.MaxBruteForce, 64)
	checkBool("MaxLen is at least 4", bytealg.MaxLen >= 4, true)
	check("Cutover(0)", bytealg.Cutover(0), 2)
	check("Cutover(100)", bytealg.Cutover(100), 14)
	check("PrimeRK", bytealg.PrimeRK, 16777619)
	h, pow := bytealg.HashStr("")
	check("HashStr empty", int(h), 0)
	check("HashStr empty pow", int(pow), 1)
	h, pow = bytealg.HashStr("a")
	check("HashStr a", int(h), 97)
	check("HashStr a pow", int(pow), 16777619)
	h, pow = bytealg.HashStr("ab")
	check("HashStr ab", int(h), 97*16777619+98)
	prime := uint32(16777619)
	check("HashStr ab pow", int(pow), int(prime*prime))
	hr, _ := bytealg.HashStrRev("ab")
	check("HashStrRev ab", int(hr), 98*16777619+97)
}

func makeNoZero() {
	b := bytealg.MakeNoZero(100)
	check("MakeNoZero len", len(b), 100)
	checkBool("MakeNoZero cap", cap(b) >= 100, true)
	for i := range b {
		b[i] = byte(i)
	}
	check("MakeNoZero write", int(b[99]), 99)
	b = append(b[:50], 1, 2, 3)
	check("MakeNoZero append", len(b), 53)
	func() {
		defer func() { fmt.Println("MakeNoZero(-1):", recover()) }()
		bytealg.MakeNoZero(-1)
	}()
}

func main() {
	nilAndEmpty()
	positions()
	counts()
	order()
	searches()
	constants()
	makeNoZero()
	// A megabyte: the bytes are 0 to 250 in turn, so each of them occurs the same number of times.
	mb := make([]byte, 1<<20)
	for i := range mb {
		mb[i] = byte(i % 251)
	}
	check("Count in a megabyte", bytealg.Count(mb, 3), (1<<20)/251+boolInt((1<<20)%251 > 3))
	check("IndexByte in a megabyte", bytealg.IndexByte(mb, 250), 250)
	check("LastIndexByte in a megabyte", bytealg.LastIndexByte(mb, 250), 1<<20-(1<<20)%251-1)
	check("IndexByte of a byte that is not there", bytealg.IndexByte(mb, 251), -1)
	fmt.Println(checks, "checks,", failures, "failures")
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Output:
// MakeNoZero(-1): runtime error: makeslice: len out of range
// 104139 checks, 0 failures
