// phase: 09
// Differential sweep of package bytes: 60000 pseudo-random triples (text, pattern, replacement) over ASCII, multi-byte and invalid
// UTF-8 alphabets go through about 80 functions and Buffer/Reader sequences; one hash per function is printed. The expected output comes from
// real Go 1.25 (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"fmt"
	"bytes"
	"unicode"
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

func (x *hasher) bs(a [][]byte) {
	x.int(len(a))
	for _, s := range a {
		x.bytes(string(s))
	}
}

func main() {
	r := &rngT{54321}
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
		s := []byte(gen(r, 40))
		t := []byte(gen(r, 4))
		u := []byte(gen(r, 6))
		ts := string(t)
		n := r.intn(7) - 2
		get("Index").int(bytes.Index(s, t))
		get("LastIndex").int(bytes.LastIndex(s, t))
		get("IndexAny").int(bytes.IndexAny(s, ts))
		get("LastIndexAny").int(bytes.LastIndexAny(s, ts))
		get("IndexByte").int(bytes.IndexByte(s, byte(t0(ts))))
		get("LastIndexByte").int(bytes.LastIndexByte(s, byte(t0(ts))))
		get("IndexRune").int(bytes.IndexRune(s, rune(t0r(ts))))
		get("IndexFunc").int(bytes.IndexFunc(s, isX))
		get("LastIndexFunc").int(bytes.LastIndexFunc(s, isX))
		get("Contains").bool(bytes.Contains(s, t))
		get("ContainsAny").bool(bytes.ContainsAny(s, ts))
		get("ContainsRune").bool(bytes.ContainsRune(s, rune(t0r(ts))))
		get("ContainsFunc").bool(bytes.ContainsFunc(s, isX))
		get("Count").int(bytes.Count(s, t))
		get("HasPrefix").bool(bytes.HasPrefix(s, t))
		get("HasSuffix").bool(bytes.HasSuffix(s, t))
		get("EqualFold").bool(bytes.EqualFold(s, t))
		get("EqualFold2").bool(bytes.EqualFold(s, bytes.ToUpper(s)))
		get("Equal").bool(bytes.Equal(s, t))
		get("Compare").int(bytes.Compare(s, t))
		get("Split").bs(bytes.Split(s, t))
		get("SplitN").bs(bytes.SplitN(s, t, n))
		get("SplitAfter").bs(bytes.SplitAfter(s, t))
		get("SplitAfterN").bs(bytes.SplitAfterN(s, t, n))
		get("Fields").bs(bytes.Fields(s))
		get("FieldsFunc").bs(bytes.FieldsFunc(s, isX))
		get("Join").bytes(string(bytes.Join(bytes.Split(s, t), u)))
		get("Repeat").bytes(string(bytes.Repeat(t, r.intn(5))))
		get("Replace").bytes(string(bytes.Replace(s, t, u, n)))
		get("ReplaceAll").bytes(string(bytes.ReplaceAll(s, t, u)))
		get("ToUpper").bytes(string(bytes.ToUpper(s)))
		get("ToLower").bytes(string(bytes.ToLower(s)))
		get("ToTitle").bytes(string(bytes.ToTitle(s)))
		get("ToValidUTF8").bytes(string(bytes.ToValidUTF8(s, u)))
		get("ToUpperSpecial").bytes(string(bytes.ToUpperSpecial(unicode.TurkishCase, s)))
		get("ToLowerSpecial").bytes(string(bytes.ToLowerSpecial(unicode.TurkishCase, s)))
		get("Title").bytes(string(bytes.Title(s)))
		get("Map").bytes(string(bytes.Map(mapf, s)))
		get("Runes").int(len(bytes.Runes(s)))
		get("TrimSpace").bytes(string(bytes.TrimSpace(s)))
		get("Trim").bytes(string(bytes.Trim(s, ts)))
		get("TrimLeft").bytes(string(bytes.TrimLeft(s, ts)))
		get("TrimRight").bytes(string(bytes.TrimRight(s, ts)))
		get("TrimFunc").bytes(string(bytes.TrimFunc(s, isX)))
		get("TrimLeftFunc").bytes(string(bytes.TrimLeftFunc(s, isX)))
		get("TrimRightFunc").bytes(string(bytes.TrimRightFunc(s, isX)))
		get("TrimPrefix").bytes(string(bytes.TrimPrefix(s, t)))
		get("TrimSuffix").bytes(string(bytes.TrimSuffix(s, t)))
		a, b, ok := bytes.Cut(s, t)
		get("Cut").bytes(string(a) + "|" + string(b))
		get("Cut").bool(ok)
		a, ok = bytes.CutPrefix(s, t)
		get("CutPrefix").bytes(string(a))
		get("CutPrefix").bool(ok)
		a, ok = bytes.CutSuffix(s, t)
		get("CutSuffix").bytes(string(a))
		get("CutSuffix").bool(ok)
		get("Clone").bytes(string(bytes.Clone(s)))

		// Buffer: a mix of the methods, driven by the data
		var bb bytes.Buffer
		bb.Grow(r.intn(20))
		bb.Write(s)
		bb.WriteByte('|')
		bb.WriteRune(rune(t0r(ts)))
		bb.WriteString(string(u))
		get("Buffer").bytes(bb.String())
		get("Buffer").int(bb.Len())
		get("Buffer").int(bb.Cap() - bb.Cap()) // Cap is not part of the contract: only that it works
		p := make([]byte, 1+r.intn(7))
		k, _ := bb.Read(p)
		get("BufferRead").int(k)
		get("BufferRead").bytes(string(p[:k]))
		get("BufferNext").bytes(string(bb.Next(r.intn(5))))
		c, err := bb.ReadByte()
		get("BufferReadByte").int(int(c))
		get("BufferReadByte").bool(err == nil)
		if err == nil {
			get("BufferUnread").bool(bb.UnreadByte() == nil)
		}
		rr, size, err := bb.ReadRune()
		get("BufferReadRune").int(int(rr) + size)
		get("BufferReadRune").bool(err == nil)
		line, err := bb.ReadBytes('|')
		get("BufferReadBytes").bytes(string(line))
		get("BufferReadBytes").bool(err == nil)
		bb.Truncate(r.intn(bb.Len() + 1))
		get("BufferTruncate").bytes(bb.String())
		get("BufferTruncateRange").bytes(tryTruncate(&bb, r.intn(bb.Len()+5)-2)) // below 0 and above Len panic
		get("BufferTruncateRange").int(bb.Len())
		bb.Reset()
		get("BufferReset").int(bb.Len())

		// Reader: read in pieces
		rd := bytes.NewReader(s)
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
		rd.Seek(int64(r.intn(len(s)+1)), 0)
		get("ReaderSeek").int(rd.Len())
	}
	for _, n := range names {
		fmt.Printf("%-16s %016x\n", n, hs[n].h)
	}
}

func tryTruncate(bb *bytes.Buffer, n int) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint(r)
		}
	}()
	bb.Truncate(n)
	return "ok"
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
// Index            1483d40cce3b1f19
// LastIndex        a44a7cd45a8b1ee2
// IndexAny         721840c7150adad3
// LastIndexAny     7938dce6f3e13235
// IndexByte        7b7e4d979ba71f87
// LastIndexByte    6dbc22e1d7ed99ad
// IndexRune        bf2e87dcea7a1df6
// IndexFunc        5e0e2daee15e3926
// LastIndexFunc    ee0ad365d1e4b421
// Contains         19350da12908a2a4
// ContainsAny      33915a74d5dbac6d
// ContainsRune     503f94f9c19df2bc
// ContainsFunc     7b40083be1cf2934
// Count            ec1303512e01256d
// HasPrefix        2b48aea6f8b4dd2d
// HasSuffix        b25630ed99435a1c
// EqualFold        66c660e124c855bd
// EqualFold2       59619f0f80865f25
// Equal            c2d503f291efcefc
// Compare          286b718f0a929be0
// Split            abefe7a678e9f364
// SplitN           e82761394e35a31b
// SplitAfter       07771c27f01fa709
// SplitAfterN      106deaac70593176
// Fields           04676dfd83f39d1e
// FieldsFunc       dbc24296efc45513
// Join             5c92b62c7670844a
// Repeat           24194d1c7d44176d
// Replace          d26c42d59c2c6617
// ReplaceAll       1a68dd3c6708cc4b
// ToUpper          0d6702697fbe61be
// ToLower          76447c7454af8c0d
// ToTitle          e7ef1bb2fff57b27
// ToValidUTF8      41d4ba5cfbdfa9c1
// ToUpperSpecial   0d6702697fbe61be
// ToLowerSpecial   76447c7454af8c0d
// Title            96c0073728c3a7e7
// Map              4c136d778e3c5066
// Runes            e69bea3371ed42f1
// TrimSpace        88bd9a6583687502
// Trim             41047342f9171227
// TrimLeft         7f4a138f287349df
// TrimRight        d9ea50a3a93200fc
// TrimFunc         34cbfdd0c6001012
// TrimLeftFunc     dcd0f2d4f9347b30
// TrimRightFunc    a3d9e9ee889ed358
// TrimPrefix       58c88b0937f9a201
// TrimSuffix       5cf67378d7bb5cbc
// Cut              c6b979a689bd30ca
// CutPrefix        b3e065b121968007
// CutSuffix        15de76c5d896d13b
// Clone            fd0acd2676e86d5f
// Buffer           61e2456cdf95aaef
// BufferRead       b77019a8e6b1feb9
// BufferNext       9f790289c07a3a32
// BufferReadByte   e56e81fc30ab22e8
// BufferUnread     4d61086dbf9a9525
// BufferReadRune   ae669033ba8e315e
// BufferReadBytes  56c242fc6b9386d8
// BufferTruncate   95d40474badb35d7
// BufferTruncateRange 5c8881d266428c51
// BufferReset      db98c6a8294826a5
// Reader           253508b0109d0522
// ReaderSeek       9f43c9033b316ac4
