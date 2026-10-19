// phase: 09
// Differential sweep of package path: 200000 pseudo-random paths and patterns go through Clean, Split, Join, Base, Dir,
// Ext, IsAbs and Match (with every error); one hash per function is printed. The expected output comes from real
// Go 1.25 (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"fmt"
	"path"
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

var pieces = []string{"/", "/", "a", "b", "..", ".", "ab", "x.y", ".go", "é", "*", "?", "[a-c]", "[^b]", "\\", "[", "]", "-", "{", "a*b", "**", "c?d"}

func gen(r *rngT, n int) string {
	var b []byte
	for i := r.intn(n + 1); i > 0; i-- {
		b = append(b, pieces[r.intn(len(pieces))]...)
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

func main() {
	r := &rngT{99}
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
	for it := 0; it < 200000; it++ {
		p := gen(r, 8)
		q := gen(r, 5)
		get("Clean").bytes(path.Clean(p))
		d, f := path.Split(p)
		get("Split").bytes(d + "|" + f)
		get("Join").bytes(path.Join(p, q))
		get("Join3").bytes(path.Join(p, "", q, ".."))
		get("Base").bytes(path.Base(p))
		get("Dir").bytes(path.Dir(p))
		get("Ext").bytes(path.Ext(p))
		if path.IsAbs(p) {
			get("IsAbs").bytes("y")
		} else {
			get("IsAbs").bytes("n")
		}
		ok, err := path.Match(q, p)
		get("Match").bytes(fmt.Sprint(ok, err))
		ok, err = path.Match(q, q)
		get("MatchSelf").bytes(fmt.Sprint(ok, err))
	}
	for _, n := range names {
		fmt.Printf("%-10s %016x\n", n, hs[n].h)
	}
}

// Output:
// Clean      03363cf75027ccdc
// Split      b78b6482cf0f87a0
// Join       adbea42bedcb8a3a
// Join3      072b2f6fd1bf25df
// Base       0df717c871c5f385
// Dir        2d1536f89649e25e
// Ext        58c7946f11cafc5c
// IsAbs      f4a9fecc2b390e1e
// Match      2b645d971af210c8
// MatchSelf  7107b2e83bf8bdba
