// Copyright 2026 The gocpp Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bytealg

import (
	"bytes"
	"math/rand"
	"strings"
	"testing"
)

// The stub's contract (job 16.1f-stubs) is Go's behavior for the cases the
// four spike packages exercise. The reference is the real strings/bytes
// packages of the same Go toolchain.

func TestConstants(t *testing.T) {
	// amd64's values (index_amd64.go); MaxLen is deliberately 0 in the stub.
	if MaxBruteForce != 64 {
		t.Errorf("MaxBruteForce = %d, want 64 (amd64)", MaxBruteForce)
	}
	if MaxLen != 0 {
		t.Errorf("MaxLen = %d, want 0 (the stub takes the general path)", MaxLen)
	}
	if PrimeRK != 16777619 {
		t.Errorf("PrimeRK = %d, want 16777619", PrimeRK)
	}
	for _, n := range []int{0, 1, 2, 15, 16, 17, 63, 64, 1000} {
		if got, want := Cutover(n), (n+16)/8; got != want {
			t.Errorf("Cutover(%d) = %d, want %d (amd64)", n, got, want)
		}
	}
}

func TestIndexByteString(t *testing.T) {
	cases := []struct {
		s string
		c byte
	}{
		{"", 'a'},
		{"a", 'a'},
		{"a", 'b'},
		{"hello, world", 'o'},
		{"hello, world", 'h'},
		{"hello, world", 'd'},
		{"hello, world", 'z'},
		{"héllo wörld", 0xc3}, // first byte of é
		{string([]byte{0, 1, 2, 255}), 0xff},
		{strings.Repeat("ab", 100), 'b'},
	}
	for _, c := range cases {
		want := strings.IndexByte(c.s, c.c)
		if got := IndexByteString(c.s, c.c); got != want {
			t.Errorf("IndexByteString(%q, %q) = %d, want %d", c.s, c.c, got, want)
		}
		wantLast := strings.LastIndexByte(c.s, c.c)
		if got := LastIndexByteString(c.s, c.c); got != wantLast {
			t.Errorf("LastIndexByteString(%q, %q) = %d, want %d", c.s, c.c, got, wantLast)
		}
	}
}

func TestCountString(t *testing.T) {
	cases := []struct {
		s string
		c byte
	}{
		{"", 'a'},
		{"aaa", 'a'},
		{"abab", 'a'},
		{"héllo wörld", 'l'},
		{"héllo wörld", 0xc3},
		{string([]byte{0, 0, 0, 255}), 0},
		{strings.Repeat("abc", 200), 'c'},
	}
	for _, c := range cases {
		want := bytes.Count([]byte(c.s), []byte{c.c})
		if got := CountString(c.s, c.c); got != want {
			t.Errorf("CountString(%q, %q) = %d, want %d", c.s, c.c, got, want)
		}
	}
}

func TestCompareString(t *testing.T) {
	pairs := [][2]string{
		{"", ""}, {"", "a"}, {"a", ""}, {"a", "a"}, {"a", "b"}, {"b", "a"},
		{"abc", "abd"}, {"abd", "abc"}, {"abc", "abcd"}, {"abcd", "abc"},
		{"héllo", "hello"}, {"hello", "héllo"}, {"\x00", ""}, {"\x00", "\x01"},
		{strings.Repeat("x", 1000), strings.Repeat("x", 999)},
	}
	for _, p := range pairs {
		want := strings.Compare(p[0], p[1])
		if got := CompareString(p[0], p[1]); got != want {
			t.Errorf("CompareString(%q, %q) = %d, want %d", p[0], p[1], got, want)
		}
	}
}

func TestIndexStringAndRabinKarp(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	alphabet := []byte("abé\x00\xff")
	randString := func(max int) string {
		n := rng.Intn(max + 1)
		b := make([]byte, n)
		for i := range b {
			b[i] = alphabet[rng.Intn(len(alphabet))]
		}
		return string(b)
	}
	// The fixed cases first (empty, one byte, the whole string, longer needle,
	// and the Rabin-Karp cutover).
	fixed := [][2]string{
		{"", ""}, {"", "a"}, {"a", ""}, {"a", "a"}, {"a", "b"},
		{"hello", "hello"}, {"hello", "hellox"}, {"hello", "lo"},
		{"héllo wörld", "wörld"}, {"héllo wörld", "ö"},
		{strings.Repeat("abc", 100), "cab"},
	}
	check := func(s, sep string) {
		want := strings.Index(s, sep)
		if got := IndexString(s, sep); got != want {
			t.Errorf("IndexString(%q, %q) = %d, want %d", s, sep, got, want)
		}
		// IndexRabinKarp has the precondition 0 < len(sep) <= len(s), which
		// its callers in strings and internal/stringslite establish.
		if len(sep) > 0 && len(sep) <= len(s) {
			if got := IndexRabinKarp(s, sep); got != want {
				t.Errorf("IndexRabinKarp(%q, %q) = %d, want %d", s, sep, got, want)
			}
			if got := IndexRabinKarp([]byte(s), []byte(sep)); got != want {
				t.Errorf("IndexRabinKarp([]byte(%q), []byte(%q)) = %d, want %d", s, sep, got, want)
			}
		}
		if got := bytes.Index([]byte(s), []byte(sep)); got != want {
			t.Errorf("bytes.Index(%q, %q) = %d, want %d (reference disagrees)", s, sep, got, want)
		}
	}
	for _, p := range fixed {
		check(p[0], p[1])
	}
	for i := 0; i < 500; i++ {
		check(randString(64), randString(6))
	}
}

func TestHashStrRev(t *testing.T) {
	// HashStrRev(sep) is the hash of the reversed bytes and the multiplicative
	// factor; check it against HashStr of the reversed string, which is the
	// definition.
	for _, sep := range []string{"", "a", "ab", "abc", "hello", "héllo"} {
		rev := make([]byte, len(sep))
		for i := 0; i < len(sep); i++ {
			rev[len(sep)-1-i] = sep[i]
		}
		gotHash, gotPow := HashStrRev(sep)
		wantHash, wantPow := HashStr(string(rev))
		if gotHash != wantHash || gotPow != wantPow {
			t.Errorf("HashStrRev(%q) = (%d, %d), HashStr(reverse) = (%d, %d)", sep, gotHash, gotPow, wantHash, wantPow)
		}
		gotHash, gotPow = HashStrRev([]byte(sep))
		if gotHash != wantHash || gotPow != wantPow {
			t.Errorf("HashStrRev([]byte(%q)) = (%d, %d), want (%d, %d)", sep, gotHash, gotPow, wantHash, wantPow)
		}
	}
}

func TestMakeNoZero(t *testing.T) {
	for _, n := range []int{0, 1, 2, 7, 64, 1024} {
		b := MakeNoZero(n)
		if len(b) != n {
			t.Errorf("len(MakeNoZero(%d)) = %d", n, len(b))
		}
		if cap(b) < n {
			t.Errorf("cap(MakeNoZero(%d)) = %d", n, cap(b))
		}
		// The contract allows zeroing (see the package comment) but the bytes
		// must be writable.
		if n > 0 {
			b[0] = 1
			if b[0] != 1 {
				t.Errorf("MakeNoZero(%d) is not writable", n)
			}
		}
		if n > 1 {
			b[n-1] = 2
			if b[n-1] != 2 {
				t.Errorf("MakeNoZero(%d): last byte is not writable", n)
			}
		}
	}
}
