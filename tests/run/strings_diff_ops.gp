// phase: 09
// Differential sweep of package strings: 60000 pseudo-random pairs (text, pattern) over ASCII, multi-byte and invalid
// UTF-8 alphabets go through about 70 functions; one hash per function is printed. The expected output comes from
// real Go 1.25 (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type rngT struct{ s uint64 }

func (r *rngT) next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
func (r *rngT) intn(n int) int { return int(r.next()>>1) % n }

var alphabets = [][]string{
	{"a", "b"},
	{"a", "b", "c", " ", "\n"},
	{"a", "A", "b", "B", " ", "\t", ",", "é", "É"},
	{"☺", "☻", "x", "é", "𝄞", "ß", "ǅ", "İ", "K"},
	{"a", "\xff", "\xc3", "b", "\xe2\x98", "é", "\x00"},
	{"abc", "ab", "bc", "a", ""},
}

func gen(r *rngT, maxLen int) string {
	al := alphabets[r.intn(len(alphabets))]
	n := r.intn(maxLen + 1)
	var b []byte
	for i := 0; i < n; i++ {
		b = append(b, al[r.intn(len(al))]...)
	}
	return string(b)
}

type hasher struct{ h uint64 }

func newHasher() *hasher { return &hasher{14695981039346656037} }
func (x *hasher) bytes(s string) {
	for i := 0; i < len(s); i++ {
		x.h ^= uint64(s[i])
		x.h *= 1099511628211
	}
	x.h ^= 0xff
	x.h *= 1099511628211
}
func (x *hasher) int(n int) { x.bytes(fmt.Sprint(n)) }
func (x *hasher) strs(a []string) {
	x.int(len(a))
	for _, s := range a {
		x.bytes(s)
	}
}
func (x *hasher) bool(b bool) {
	if b {
		x.int(1)
	} else {
		x.int(0)
	}
}

func main() {
	r := &rngT{12345}
	names := []string{}
	hs := map[string]*hasher{}
	get := func(n string) *hasher {
		h := hs[n]
		if h == nil {
			h = newHasher()
			hs[n] = h
			names = append(names, n)
		}
		return h
	}
	isX := func(c rune) bool { return c == 'a' || c == 'é' || c == '☺' || unicode.IsSpace(c) }
	mapf := func(c rune) rune {
		if c == 'a' {
			return -1
		}
		if c == 'b' {
			return 'B'
		}
		return unicode.ToUpper(c)
	}
	for it := 0; it < 60000; it++ {
		s := gen(r, 40)
		t := gen(r, 4)
		u := gen(r, 6)
		n := r.intn(7) - 2
		get("Index").int(strings.Index(s, t))
		get("LastIndex").int(strings.LastIndex(s, t))
		get("IndexAny").int(strings.IndexAny(s, t))
		get("LastIndexAny").int(strings.LastIndexAny(s, t))
		get("IndexByte").int(strings.IndexByte(s, byte(t0(t))))
		get("LastIndexByte").int(strings.LastIndexByte(s, byte(t0(t))))
		get("IndexRune").int(strings.IndexRune(s, rune(t0r(t))))
		get("IndexFunc").int(strings.IndexFunc(s, isX))
		get("LastIndexFunc").int(strings.LastIndexFunc(s, isX))
		get("Contains").bool(strings.Contains(s, t))
		get("ContainsAny").bool(strings.ContainsAny(s, t))
		get("ContainsRune").bool(strings.ContainsRune(s, rune(t0r(t))))
		get("ContainsFunc").bool(strings.ContainsFunc(s, isX))
		get("Count").int(strings.Count(s, t))
		get("HasPrefix").bool(strings.HasPrefix(s, t))
		get("HasSuffix").bool(strings.HasSuffix(s, t))
		get("EqualFold").bool(strings.EqualFold(s, t))
		get("EqualFold2").bool(strings.EqualFold(s, strings.ToUpper(s)))
		get("Compare").int(strings.Compare(s, t))
		get("Split").strs(strings.Split(s, t))
		get("SplitN").strs(strings.SplitN(s, t, n))
		get("SplitAfter").strs(strings.SplitAfter(s, t))
		get("SplitAfterN").strs(strings.SplitAfterN(s, t, n))
		get("Fields").strs(strings.Fields(s))
		get("FieldsFunc").strs(strings.FieldsFunc(s, isX))
		get("Join").bytes(strings.Join(strings.Split(s, t), u))
		get("Repeat").bytes(strings.Repeat(t, r.intn(5)))
		get("Replace").bytes(strings.Replace(s, t, u, n))
		get("ReplaceAll").bytes(strings.ReplaceAll(s, t, u))
		get("ToUpper").bytes(strings.ToUpper(s))
		get("ToLower").bytes(strings.ToLower(s))
		get("ToTitle").bytes(strings.ToTitle(s))
		get("ToValidUTF8").bytes(strings.ToValidUTF8(s, u))
		get("ToUpperSpecial").bytes(strings.ToUpperSpecial(unicode.TurkishCase, s))
		get("ToLowerSpecial").bytes(strings.ToLowerSpecial(unicode.TurkishCase, s))
		get("Title").bytes(strings.Title(s))
		get("Map").bytes(strings.Map(mapf, s))
		get("TrimSpace").bytes(strings.TrimSpace(s))
		get("Trim").bytes(strings.Trim(s, t))
		get("TrimLeft").bytes(strings.TrimLeft(s, t))
		get("TrimRight").bytes(strings.TrimRight(s, t))
		get("TrimFunc").bytes(strings.TrimFunc(s, isX))
		get("TrimLeftFunc").bytes(strings.TrimLeftFunc(s, isX))
		get("TrimRightFunc").bytes(strings.TrimRightFunc(s, isX))
		get("TrimPrefix").bytes(strings.TrimPrefix(s, t))
		get("TrimSuffix").bytes(strings.TrimSuffix(s, t))
		a, b, ok := strings.Cut(s, t)
		get("Cut").bytes(a + "|" + b)
		get("Cut").bool(ok)
		a, ok = strings.CutPrefix(s, t)
		get("CutPrefix").bytes(a)
		get("CutPrefix").bool(ok)
		a, ok = strings.CutSuffix(s, t)
		get("CutSuffix").bytes(a)
		get("CutSuffix").bool(ok)
		get("Clone").bytes(strings.Clone(s))
		get("RuneCount").int(utf8.RuneCountInString(s))

		// Builder: a mix of the Write methods
		var sb strings.Builder
		sb.Grow(r.intn(20))
		sb.WriteString(s)
		sb.WriteByte('|')
		sb.WriteRune(rune(t0r(t)))
		sb.Write([]byte(u))
		get("Builder").bytes(sb.String())
		get("Builder").int(sb.Len())

		// Replacer over up to three pairs; a one-byte pair set takes the byte replacer
		var rp *strings.Replacer
		switch it % 3 {
		case 0:
			rp = strings.NewReplacer("a", "1", "b", "22", "c", "")
		case 1:
			rp = strings.NewReplacer("ab", "X", "a", "Y", "abc", "Z", "", "-")
		default:
			rp = strings.NewReplacer(t, u, "é", "e")
		}
		get("Replacer").bytes(rp.Replace(s))

		// Reader: Seek, ReadAt, Len, Size, then read in pieces
		rd := strings.NewReader(s)
		pos, err := rd.Seek(int64(r.intn(len(s)+4))-2, r.intn(3))
		get("ReaderSeek").int(int(pos))
		get("ReaderSeek").bytes(fmt.Sprint(err))
		get("ReaderLen").int(rd.Len() + int(rd.Size()))
		at := make([]byte, r.intn(6))
		k, err := rd.ReadAt(at, int64(r.intn(len(s)+3))-1)
		get("ReaderAt").int(k)
		get("ReaderAt").bytes(string(at[:k]) + fmt.Sprint(err))
		rd.Seek(0, 0)
		buf := make([]byte, 1+r.intn(5))
		for {
			k, err := rd.Read(buf)
			get("Reader").int(k)
			if err != nil {
				break
			}
			if k > 0 && buf[0] > 0x7f {
				rd.UnreadByte()
				rr, size, _ := rd.ReadRune()
				get("Reader").int(int(rr) + size)
			}
		}
	}
	for _, n := range names {
		fmt.Printf("%-16s %016x\n", n, hs[n].h)
	}
}

func t0(t string) int {
	if len(t) == 0 {
		return 'a'
	}
	return int(t[0])
}

func t0r(t string) int {
	for _, c := range t {
		return int(c)
	}
	return 'a'
}

// Output:
// Index            5ad64741952ae50b
// LastIndex        61d0f99203035a9d
// IndexAny         85109e861865ce3a
// LastIndexAny     63ec74a66bfc6f1b
// IndexByte        5475ce10d915d90f
// LastIndexByte    2284468a2f448f90
// IndexRune        4b86ab65811e3e85
// IndexFunc        58f29d11f212f69b
// LastIndexFunc    ffd1396740d9b714
// Contains         6f48111175883c5c
// ContainsAny      b8336bab58245ac5
// ContainsRune     08415d6f5d29010c
// ContainsFunc     b2681bacf94f3c7c
// Count            1940bb54d9395ed4
// HasPrefix        6de1c51b515b85dc
// HasSuffix        f28631f65a469f3c
// EqualFold        98e19bd127d1cbbd
// EqualFold2       59619f0f80865f25
// Compare          9c705cd37cee2844
// Split            9f402779dc940220
// SplitN           570bba00766dbea9
// SplitAfter       d6a96d8b8d21f529
// SplitAfterN      0934ec38cfadcb02
// Fields           6a8ec7635fda2c43
// FieldsFunc       d802b4bbefffe935
// Join             10d93427336df945
// Repeat           52a3850bbfbac2ba
// Replace          dd33175e0fc54023
// ReplaceAll       7d195f38c2240235
// ToUpper          2f55a2274d8011b0
// ToLower          b4385a9fd2ba61ca
// ToTitle          9c197821168b9530
// ToValidUTF8      6f829fb139c27c1c
// ToUpperSpecial   2f55a2274d8011b0
// ToLowerSpecial   b4385a9fd2ba61ca
// Title            a8df54472f1e04b0
// Map              ebfc1741b378fca2
// TrimSpace        547c022df594307c
// Trim             6be5a2530c59cd57
// TrimLeft         a35c42da515af4ea
// TrimRight        61eb6920e436e505
// TrimFunc         7b238b89fa12cb44
// TrimLeftFunc     2211511c99c3217a
// TrimRightFunc    06eb389298109049
// TrimPrefix       4ada0a6a782b2fe8
// TrimSuffix       c8496cc4f989ac57
// Cut              696acb99d6a06ca1
// CutPrefix        4ae1ee347540c999
// CutSuffix        2dd52c07c9f08cb0
// Clone            a878aba5d67c372c
// RuneCount        7a9e2dcd2c248da9
// Builder          1a688830b843ad55
// Replacer         fa2e422a3376dda6
// ReaderSeek       b1380946d2187e8c
// ReaderLen        398622fc02c7a6b5
// ReaderAt         bdc46d0c63846b59
// Reader           706f723780e75bbc
