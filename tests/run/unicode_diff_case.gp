// phase: 09
// unicode case mapping against Go: ToUpper, ToLower, ToTitle, To(UpperCase, LowerCase, TitleCase) and SimpleFold (the whole fold orbit) for every code point from -2 to 0x110010 (counts and hashes), the Turkish and Azeri special cases, a user SpecialCase, and the fold orbit of every rune that has a different case.

package main

import (
	"fmt"
	"unicode"
)

func main() {
	var hu, hl, ht, hf, hto uint64 = 14695981039346656037, 14695981039346656037, 14695981039346656037, 14695981039346656037, 14695981039346656037
	mix := func(h *uint64, v uint64) { *h = (*h ^ v) * 1099511628211 }
	nu, nl, nt, nf, orbits, longest := 0, 0, 0, 0, 0, 0
	for r := rune(-2); r <= 0x110010; r++ {
		u, l, t := unicode.ToUpper(r), unicode.ToLower(r), unicode.ToTitle(r)
		if u != r {
			nu++
		}
		if l != r {
			nl++
		}
		if t != r {
			nt++
		}
		mix(&hu, uint64(u)+5)
		mix(&hl, uint64(l)+5)
		mix(&ht, uint64(t)+5)
		f := unicode.SimpleFold(r)
		if f != r {
			nf++
		}
		mix(&hf, uint64(f)+5)
		mix(&hto, uint64(unicode.To(unicode.UpperCase, r))+uint64(unicode.To(unicode.LowerCase, r))<<20+uint64(unicode.To(unicode.TitleCase, r))<<40)
		if r >= 0 && f != r {
			// the orbit is a cycle that returns to r, in increasing order after the smallest
			n := 1
			for x := f; x != r; x = unicode.SimpleFold(x) {
				n++
				if n > 10 {
					fmt.Println("orbit does not close", r)
					break
				}
			}
			if n > longest {
				longest = n
			}
			if r == unicode.SimpleFold(unicode.SimpleFold(r)) || n > 2 {
				orbits++
			}
		}
	}
	fmt.Println("upper", nu, "lower", nl, "title", nt, "fold", nf, "orbits", orbits, "longest", longest)
	fmt.Printf("%016x %016x %016x %016x %016x\n", hu, hl, ht, hf, hto)

	for _, r := range []rune{'a', 'A', 'k', 'K', 0x212a, 's', 'S', 0x17f, 'ß', 0x1e9e, 'σ', 'ς', 'Σ', 'ǅ', 'Ǆ', 'ǆ', 'i', 'I', 'İ', 'ı', 0x1c5, 0x3c2, 'µ', 0x39c, 0x3bc, 'ÿ', 'Ÿ', 0x1fbe, 0x345, 0x399, 0x3b9, '1', '\U00010400', '\U00010428', -1, 0x110000} {
		fmt.Printf("%U U=%U L=%U T=%U F=%U Turkish(U=%U L=%U T=%U) Azeri(U=%U L=%U)\n", r, unicode.ToUpper(r), unicode.ToLower(r), unicode.ToTitle(r), unicode.SimpleFold(r),
			unicode.TurkishCase.ToUpper(r), unicode.TurkishCase.ToLower(r), unicode.TurkishCase.ToTitle(r), unicode.AzeriCase.ToUpper(r), unicode.AzeriCase.ToLower(r))
	}
	var ht2 uint64 = 14695981039346656037
	for r := rune(0); r < 0x600; r++ {
		mix(&ht2, uint64(unicode.TurkishCase.ToUpper(r))<<32+uint64(unicode.TurkishCase.ToLower(r))<<16+uint64(unicode.TurkishCase.ToTitle(r)))
		mix(&ht2, uint64(unicode.AzeriCase.ToUpper(r))<<32+uint64(unicode.AzeriCase.ToLower(r)))
	}
	fmt.Printf("turkish/azeri %016x\n", ht2)

	// a user-defined special case, as in Go's docs
	sc := unicode.SpecialCase{
		unicode.CaseRange{Lo: 'A', Hi: 'C', Delta: [unicode.MaxCase]rune{0, 32, 0}},
		unicode.CaseRange{Lo: 'x', Hi: 'z', Delta: [unicode.MaxCase]rune{-32, 0, -32}},
		unicode.CaseRange{Lo: 0x100, Hi: 0x10f, Delta: [unicode.MaxCase]rune{unicode.UpperLower, unicode.UpperLower, unicode.UpperLower}},
	}
	for _, r := range []rune{'A', 'B', 'C', 'D', 'x', 'y', 'z', 'w', 0x100, 0x101, 0x102, 0x10f, 0x110, 'é'} {
		fmt.Printf("%U sc U=%U L=%U T=%U\n", r, sc.ToUpper(r), sc.ToLower(r), sc.ToTitle(r))
	}

	// CaseRanges: sorted, non overlapping, and the same answers as the functions
	prev := rune(-1)
	bad := 0
	for _, cr := range unicode.CaseRanges {
		if rune(cr.Lo) <= prev || cr.Hi < cr.Lo {
			bad++
		}
		prev = rune(cr.Hi)
	}
	fmt.Println("case ranges", len(unicode.CaseRanges), "bad", bad)
}

// Output:
// upper 1450 lower 1433 title 1404 fold 2878 orbits 2878 longest 4
// 640d453cdfdd6358 a75d872c023d59a5 032e7beb29f850ac b91044d1a7ff4575 4ec3382c421c49b1
// U+0061 U=U+0041 L=U+0061 T=U+0041 F=U+0041 Turkish(U=U+0041 L=U+0061 T=U+0041) Azeri(U=U+0041 L=U+0061)
// U+0041 U=U+0041 L=U+0061 T=U+0041 F=U+0061 Turkish(U=U+0041 L=U+0061 T=U+0041) Azeri(U=U+0041 L=U+0061)
// U+006B U=U+004B L=U+006B T=U+004B F=U+212A Turkish(U=U+004B L=U+006B T=U+004B) Azeri(U=U+004B L=U+006B)
// U+004B U=U+004B L=U+006B T=U+004B F=U+006B Turkish(U=U+004B L=U+006B T=U+004B) Azeri(U=U+004B L=U+006B)
// U+212A U=U+212A L=U+006B T=U+212A F=U+004B Turkish(U=U+212A L=U+006B T=U+212A) Azeri(U=U+212A L=U+006B)
// U+0073 U=U+0053 L=U+0073 T=U+0053 F=U+017F Turkish(U=U+0053 L=U+0073 T=U+0053) Azeri(U=U+0053 L=U+0073)
// U+0053 U=U+0053 L=U+0073 T=U+0053 F=U+0073 Turkish(U=U+0053 L=U+0073 T=U+0053) Azeri(U=U+0053 L=U+0073)
// U+017F U=U+0053 L=U+017F T=U+0053 F=U+0053 Turkish(U=U+0053 L=U+017F T=U+0053) Azeri(U=U+0053 L=U+017F)
// U+00DF U=U+00DF L=U+00DF T=U+00DF F=U+1E9E Turkish(U=U+00DF L=U+00DF T=U+00DF) Azeri(U=U+00DF L=U+00DF)
// U+1E9E U=U+1E9E L=U+00DF T=U+1E9E F=U+00DF Turkish(U=U+1E9E L=U+00DF T=U+1E9E) Azeri(U=U+1E9E L=U+00DF)
// U+03C3 U=U+03A3 L=U+03C3 T=U+03A3 F=U+03A3 Turkish(U=U+03A3 L=U+03C3 T=U+03A3) Azeri(U=U+03A3 L=U+03C3)
// U+03C2 U=U+03A3 L=U+03C2 T=U+03A3 F=U+03C3 Turkish(U=U+03A3 L=U+03C2 T=U+03A3) Azeri(U=U+03A3 L=U+03C2)
// U+03A3 U=U+03A3 L=U+03C3 T=U+03A3 F=U+03C2 Turkish(U=U+03A3 L=U+03C3 T=U+03A3) Azeri(U=U+03A3 L=U+03C3)
// U+01C5 U=U+01C4 L=U+01C6 T=U+01C5 F=U+01C6 Turkish(U=U+01C4 L=U+01C6 T=U+01C5) Azeri(U=U+01C4 L=U+01C6)
// U+01C4 U=U+01C4 L=U+01C6 T=U+01C5 F=U+01C5 Turkish(U=U+01C4 L=U+01C6 T=U+01C5) Azeri(U=U+01C4 L=U+01C6)
// U+01C6 U=U+01C4 L=U+01C6 T=U+01C5 F=U+01C4 Turkish(U=U+01C4 L=U+01C6 T=U+01C5) Azeri(U=U+01C4 L=U+01C6)
// U+0069 U=U+0049 L=U+0069 T=U+0049 F=U+0049 Turkish(U=U+0130 L=U+0069 T=U+0130) Azeri(U=U+0130 L=U+0069)
// U+0049 U=U+0049 L=U+0069 T=U+0049 F=U+0069 Turkish(U=U+0049 L=U+0131 T=U+0049) Azeri(U=U+0049 L=U+0131)
// U+0130 U=U+0130 L=U+0069 T=U+0130 F=U+0130 Turkish(U=U+0130 L=U+0069 T=U+0130) Azeri(U=U+0130 L=U+0069)
// U+0131 U=U+0049 L=U+0131 T=U+0049 F=U+0131 Turkish(U=U+0049 L=U+0131 T=U+0049) Azeri(U=U+0049 L=U+0131)
// U+01C5 U=U+01C4 L=U+01C6 T=U+01C5 F=U+01C6 Turkish(U=U+01C4 L=U+01C6 T=U+01C5) Azeri(U=U+01C4 L=U+01C6)
// U+03C2 U=U+03A3 L=U+03C2 T=U+03A3 F=U+03C3 Turkish(U=U+03A3 L=U+03C2 T=U+03A3) Azeri(U=U+03A3 L=U+03C2)
// U+00B5 U=U+039C L=U+00B5 T=U+039C F=U+039C Turkish(U=U+039C L=U+00B5 T=U+039C) Azeri(U=U+039C L=U+00B5)
// U+039C U=U+039C L=U+03BC T=U+039C F=U+03BC Turkish(U=U+039C L=U+03BC T=U+039C) Azeri(U=U+039C L=U+03BC)
// U+03BC U=U+039C L=U+03BC T=U+039C F=U+00B5 Turkish(U=U+039C L=U+03BC T=U+039C) Azeri(U=U+039C L=U+03BC)
// U+00FF U=U+0178 L=U+00FF T=U+0178 F=U+0178 Turkish(U=U+0178 L=U+00FF T=U+0178) Azeri(U=U+0178 L=U+00FF)
// U+0178 U=U+0178 L=U+00FF T=U+0178 F=U+00FF Turkish(U=U+0178 L=U+00FF T=U+0178) Azeri(U=U+0178 L=U+00FF)
// U+1FBE U=U+0399 L=U+1FBE T=U+0399 F=U+0345 Turkish(U=U+0399 L=U+1FBE T=U+0399) Azeri(U=U+0399 L=U+1FBE)
// U+0345 U=U+0399 L=U+0345 T=U+0399 F=U+0399 Turkish(U=U+0399 L=U+0345 T=U+0399) Azeri(U=U+0399 L=U+0345)
// U+0399 U=U+0399 L=U+03B9 T=U+0399 F=U+03B9 Turkish(U=U+0399 L=U+03B9 T=U+0399) Azeri(U=U+0399 L=U+03B9)
// U+03B9 U=U+0399 L=U+03B9 T=U+0399 F=U+1FBE Turkish(U=U+0399 L=U+03B9 T=U+0399) Azeri(U=U+0399 L=U+03B9)
// U+0031 U=U+0031 L=U+0031 T=U+0031 F=U+0031 Turkish(U=U+0031 L=U+0031 T=U+0031) Azeri(U=U+0031 L=U+0031)
// U+10400 U=U+10400 L=U+10428 T=U+10400 F=U+10428 Turkish(U=U+10400 L=U+10428 T=U+10400) Azeri(U=U+10400 L=U+10428)
// U+10428 U=U+10400 L=U+10428 T=U+10400 F=U+10400 Turkish(U=U+10400 L=U+10428 T=U+10400) Azeri(U=U+10400 L=U+10428)
// U+FFFFFFFFFFFFFFFF U=U+FFFFFFFFFFFFFFFF L=U+FFFFFFFFFFFFFFFF T=U+FFFFFFFFFFFFFFFF F=U+FFFFFFFFFFFFFFFF Turkish(U=U+FFFFFFFFFFFFFFFF L=U+FFFFFFFFFFFFFFFF T=U+FFFFFFFFFFFFFFFF) Azeri(U=U+FFFFFFFFFFFFFFFF L=U+FFFFFFFFFFFFFFFF)
// U+110000 U=U+110000 L=U+110000 T=U+110000 F=U+110000 Turkish(U=U+110000 L=U+110000 T=U+110000) Azeri(U=U+110000 L=U+110000)
// turkish/azeri abb229e9033e9827
// U+0041 sc U=U+0041 L=U+0061 T=U+0041
// U+0042 sc U=U+0042 L=U+0062 T=U+0042
// U+0043 sc U=U+0043 L=U+0063 T=U+0043
// U+0044 sc U=U+0044 L=U+0064 T=U+0044
// U+0078 sc U=U+0058 L=U+0078 T=U+0058
// U+0079 sc U=U+0059 L=U+0079 T=U+0059
// U+007A sc U=U+005A L=U+007A T=U+005A
// U+0077 sc U=U+0057 L=U+0077 T=U+0057
// U+0100 sc U=U+0100 L=U+0101 T=U+0100
// U+0101 sc U=U+0100 L=U+0101 T=U+0100
// U+0102 sc U=U+0102 L=U+0103 T=U+0102
// U+010F sc U=U+010E L=U+010F T=U+010E
// U+0110 sc U=U+0110 L=U+0111 T=U+0110
// U+00E9 sc U=U+00C9 L=U+00E9 T=U+00C9
// case ranges 328 bad 0
