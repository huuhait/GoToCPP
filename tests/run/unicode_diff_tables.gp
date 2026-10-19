// phase: 09
// unicode tables against Go: every code point from -2 to 0x110010 through IsLetter, IsUpper, IsLower, IsTitle, IsDigit, IsNumber, IsSpace, IsPunct, IsSymbol, IsMark, IsControl, IsGraphic, IsPrint (counts and a hash of the answers), the size of every table of Categories, Scripts, Properties, FoldCategory and FoldScript (sorted by name), Is, In and IsOneOf with every category and script on sample runes, Version.

package main

import (
	"fmt"
	"unicode"
)

func main() {
	fmt.Println(unicode.Version, unicode.MaxRune, unicode.ReplacementChar, unicode.MaxASCII, unicode.MaxLatin1, unicode.UpperCase, unicode.LowerCase, unicode.TitleCase, unicode.MaxCase)
	preds := []struct {
		name string
		f    func(rune) bool
	}{
		{"IsLetter", unicode.IsLetter}, {"IsUpper", unicode.IsUpper}, {"IsLower", unicode.IsLower}, {"IsTitle", unicode.IsTitle},
		{"IsDigit", unicode.IsDigit}, {"IsNumber", unicode.IsNumber}, {"IsSpace", unicode.IsSpace}, {"IsPunct", unicode.IsPunct},
		{"IsSymbol", unicode.IsSymbol}, {"IsMark", unicode.IsMark}, {"IsControl", unicode.IsControl}, {"IsGraphic", unicode.IsGraphic}, {"IsPrint", unicode.IsPrint},
	}
	counts := make([]int, len(preds))
	hashes := make([]uint64, len(preds))
	for i := range hashes {
		hashes[i] = 14695981039346656037
	}
	var all uint64 = 14695981039346656037
	for r := rune(-2); r <= 0x110010; r++ {
		var mask uint64
		for i, p := range preds {
			if p.f(r) {
				counts[i]++
				mask |= 1 << uint(i)
				hashes[i] ^= uint64(r) + 3
				hashes[i] *= 1099511628211
			}
		}
		all ^= mask
		all *= 1099511628211
		if r >= 0 && r < 0x80 && r%16 == 0 {
			fmt.Printf("%#x %013b\n", r, mask)
		}
	}
	for i, p := range preds {
		fmt.Printf("%s %d %016x\n", p.name, counts[i], hashes[i])
	}
	fmt.Printf("all %016x\n", all)

	// the tables
	for _, m := range []struct {
		name string
		t    map[string]*unicode.RangeTable
	}{{"Categories", unicode.Categories}, {"Scripts", unicode.Scripts}, {"Properties", unicode.Properties}, {"FoldCategory", unicode.FoldCategory}, {"FoldScript", unicode.FoldScript}} {
		var names []string
		for k := range m.t {
			names = append(names, k)
		}
		sortStrings(names)
		fmt.Println(m.name, len(names))
		for _, k := range names {
			t := m.t[k]
			n := 0
			var h uint64 = 14695981039346656037
			for _, r := range t.R16 {
				n += int((r.Hi-r.Lo)/r.Stride) + 1
				h = (h ^ uint64(r.Lo)<<32 ^ uint64(r.Hi)<<16 ^ uint64(r.Stride)) * 1099511628211
			}
			for _, r := range t.R32 {
				n += int((r.Hi-r.Lo)/r.Stride) + 1
				h = (h ^ uint64(r.Lo)<<32 ^ uint64(r.Hi)<<16 ^ uint64(r.Stride)) * 1099511628211
			}
			// the table agrees with Is on a sample
			in := 0
			for r := rune(0); r < 0x30000; r += 7 {
				if unicode.Is(t, r) {
					in++
				}
			}
			fmt.Printf("  %s r16=%d r32=%d latin=%d runes=%d sample=%d %016x\n", k, len(t.R16), len(t.R32), t.LatinOffset, n, in, h)
		}
	}

	// Is, In, IsOneOf
	samples := []rune{'A', 'z', '0', '٣', ' ', ' ', 'é', 'ǅ', 'ß', 'Σ', 'ς', 'ж', '日', '한', 'א', 'ء', '́', '€', '\U0001f600', '\U0001d7d8', ' ', 0x10ffff, 0xe000, 0xd800, -1, 0x110000}
	cats := []*unicode.RangeTable{unicode.Letter, unicode.Lu, unicode.Ll, unicode.Lt, unicode.Nd, unicode.Nl, unicode.No, unicode.P, unicode.S, unicode.M, unicode.Zs, unicode.Cc, unicode.Co, unicode.Cs, unicode.Han, unicode.Latin, unicode.Cyrillic, unicode.Greek, unicode.Hangul, unicode.Hiragana, unicode.White_Space, unicode.Hex_Digit, unicode.Other_Alphabetic, unicode.Dash}
	for _, r := range samples {
		var is, in, one uint32
		for i, t := range cats {
			if unicode.Is(t, r) {
				is |= 1 << uint(i)
			}
		}
		in2 := unicode.In(r, cats...)
		one2 := unicode.IsOneOf(cats, r)
		_ = in
		_ = one
		fmt.Printf("%U %08x %v %v %v %v\n", r, is, in2, one2, unicode.IsOneOf(cats[:3], r), unicode.In(r, unicode.Han, unicode.Hiragana))
	}
	fmt.Println(len(unicode.GraphicRanges), len(unicode.PrintRanges), len(unicode.CaseRanges))
	fmt.Println(unicode.Letter == unicode.L, unicode.Upper == unicode.Lu, unicode.Han == unicode.Scripts["Han"], unicode.Digit == unicode.Nd, unicode.Number == unicode.N, unicode.Mark == unicode.M, unicode.Punct == unicode.P, unicode.Symbol == unicode.S, unicode.Space == unicode.Z, unicode.Title == unicode.Lt, unicode.Lower == unicode.Ll)
}

func sortStrings(a []string) { // sort.Strings: package sort is step 9.4 (later slice)
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

// Output:
// 15.0.0 1114111 65533 127 255 0 1 2 3
// 0x0 0010000000000
// 0x10 0010000000000
// 0x20 1100001000000
// 0x30 1100000110000
// 0x40 1100010000000
// 0x50 1100000000011
// 0x60 1100100000000
// 0x70 1100000000101
// IsLetter 136104 15fd7d87710f344f
// IsUpper 1831 a49295c31c57fcd2
// IsLower 2233 fac4c272d027b0ce
// IsTitle 31 a5504d343e6ed2c0
// IsDigit 680 82d6c4beb234100d
// IsNumber 1831 fd100d835a978bf9
// IsSpace 25 3cb35e6e82b1aa58
// IsPunct 842 8b6449e9cd496aaa
// IsSymbol 7770 b56583e0b4f5193a
// IsMark 2450 a9adf932c0198448
// IsControl 65 c92cec29a0887d45
// IsGraphic 149014 2f882a16edd5d55a
// IsPrint 148998 d37b84a96614c31d
// all a1d857a3420fb255
// Categories 38
//   C r16=289 r32=320 latin=2 runes=965096 sample=8134 ec3a25ed90fcc716
//   Cc r16=2 r32=0 latin=2 runes=65 sample=9 652ef8d9fe9c8e77
//   Cf r16=12 r32=6 latin=0 runes=170 sample=11 d794f912a2f0b711
//   Cn r16=282 r32=322 latin=0 runes=825345 sample=6907 fc67bb81f7dc64cc
//   Co r16=1 r32=2 latin=0 runes=137468 sample=915 16e842241a7f1282
//   Cs r16=1 r32=0 latin=0 runes=2048 sample=292 69ce340818c2b62c
//   L r16=359 r32=264 latin=6 runes=136104 sample=18135 89d8f495de77857f
//   LC r16=86 r32=49 latin=5 runes=4095 sample=582 c3d592503e335204
//   Ll r16=122 r32=41 latin=4 runes=2233 sample=313 562223618330e549
//   Lm r16=41 r32=13 latin=0 runes=397 sample=54 462ff0c0b9cdc1b9
//   Lo r16=279 r32=205 latin=1 runes=131612 sample=17499 25af94330f56e0d9
//   Lt r16=7 r32=0 latin=0 runes=31 sample=3 c818630fb91a151e
//   Lu r16=109 r32=40 latin=3 runes=1831 sample=266 e210d490adbe9d33
//   M r16=182 r32=117 latin=0 runes=2450 sample=318 4b655666841e3509
//   Mc r16=98 r32=60 latin=0 runes=452 sample=65 bcda43f9e8db2029
//   Me r16=5 r32=0 latin=0 runes=13 sample=3 e76466921a4e23ae
//   Mn r16=183 r32=125 latin=0 runes=1985 sample=250 ba66f1ef9a49fabd
//   N r16=66 r32=69 latin=4 runes=1831 sample=263 d1da7a48034d25be
//   Nd r16=37 r32=27 latin=1 runes=680 sample=101 96d15d56201c9065
//   Nl r16=7 r32=4 latin=0 runes=236 sample=34 ea247308dd9d2299
//   No r16=28 r32=43 latin=3 runes=915 sample=128 017dc2a05dc84c6b
//   P r16=115 r32=52 latin=11 runes=842 sample=114 37b8a7c5162f7476
//   Pc r16=5 r32=0 latin=0 runes=10 sample=0 86f678863ad539a6
//   Pd r16=11 r32=1 latin=0 runes=26 sample=4 0be72f53002e4ff0
//   Pe r16=23 r32=0 latin=1 runes=77 sample=10 d5e2f912ce137b93
//   Pf r16=5 r32=0 latin=0 runes=10 sample=2 b8d35918f4ed55d5
//   Pi r16=6 r32=0 latin=0 runes=12 sample=1 b77cc57c89e3bfac
//   Po r16=113 r32=51 latin=8 runes=628 sample=88 8eb187b6df752e2b
//   Ps r16=26 r32=0 latin=1 runes=79 sample=9 278257ee0ca33ac6
//   S r16=129 r32=72 latin=10 runes=7770 sample=1120 6787c3ca43b216c3
//   Sc r16=13 r32=2 latin=2 runes=63 sample=9 e5c18ebc587bb4c1
//   Sk r16=25 r32=1 latin=3 runes=125 sample=23 552b0683bcbb8896
//   Sm r16=43 r32=6 latin=5 runes=948 sample=138 6721c4b3dad2a728
//   So r16=99 r32=67 latin=2 runes=6634 sample=950 8f41415fb0fd6af0
//   Z r16=6 r32=0 latin=1 runes=19 sample=3 6e4271b3f6321cf4
//   Zl r16=1 r32=0 latin=0 runes=1 sample=1 9799f3babd39b62c
//   Zp r16=1 r32=0 latin=0 runes=1 sample=0 9899f56dbeecb62c
//   Zs r16=5 r32=0 latin=1 runes=17 sample=2 af446c80cb96f507
// Scripts 163
//   Adlam r16=0 r32=3 latin=0 runes=88 sample=14 964cd36519791282
//   Ahom r16=0 r32=3 latin=0 runes=65 sample=9 6d88f53086e31282
//   Anatolian_Hieroglyphs r16=0 r32=1 latin=0 runes=583 sample=83 f37cb2695027b62c
//   Arabic r16=22 r32=27 latin=0 runes=1368 sample=198 a7e2c12ba8117c67
//   Armenian r16=4 r32=0 latin=0 runes=96 sample=14 697ddcc4c8037499
//   Avestan r16=0 r32=2 latin=0 runes=61 sample=9 9b8bd0ae9ad28e77
//   Balinese r16=2 r32=0 latin=0 runes=124 sample=18 499bd7fb84d68e77
//   Bamum r16=1 r32=1 latin=0 runes=657 sample=94 2761550b0dcb8e77
//   Bassa_Vah r16=0 r32=2 latin=0 runes=36 sample=4 f2a30f84104c8e77
//   Batak r16=2 r32=0 latin=0 runes=56 sample=8 c687541243c88e77
//   Bengali r16=14 r32=0 latin=0 runes=96 sample=12 aa14cb48c7a7eac9
//   Bhaiksuki r16=0 r32=4 latin=0 runes=97 sample=14 af682b41de307499
//   Bopomofo r16=3 r32=0 latin=0 runes=77 sample=11 acad6438291f1282
//   Brahmi r16=0 r32=3 latin=0 runes=115 sample=16 a2933ad30afb1282
//   Braille r16=1 r32=0 latin=0 runes=256 sample=37 6a8c8491bbc2b62c
//   Buginese r16=2 r32=0 latin=0 runes=30 sample=4 e57d9b9a03808e77
//   Buhid r16=1 r32=0 latin=0 runes=20 sample=3 fe466ca6893eb62c
//   Canadian_Aboriginal r16=2 r32=1 latin=0 runes=726 sample=103 57b8241e50211282
//   Carian r16=0 r32=1 latin=0 runes=49 sample=7 811914244d71b62c
//   Caucasian_Albanian r16=0 r32=2 latin=0 runes=53 sample=9 4dd528c3f5288e77
//   Chakma r16=0 r32=2 latin=0 runes=71 sample=11 33726687b2a38e77
//   Cham r16=4 r32=0 latin=0 runes=83 sample=10 cf809ae7fe2c7499
//   Cherokee r16=3 r32=0 latin=0 runes=172 sample=25 377e707a297b1282
//   Chorasmian r16=0 r32=1 latin=0 runes=28 sample=4 78067e5cbd26b62c
//   Common r16=82 r32=82 latin=6 runes=8301 sample=1174 bf7d28e1892703cc
//   Coptic r16=3 r32=0 latin=0 runes=137 sample=19 1500f487a7a71282
//   Cuneiform r16=0 r32=4 latin=0 runes=1234 sample=177 abe067f2f6937499
//   Cypriot r16=0 r32=5 latin=0 runes=55 sample=8 f893510b305618a9
//   Cypro_Minoan r16=0 r32=1 latin=0 runes=99 sample=14 5f3d14f2f2abb62c
//   Cyrillic r16=7 r32=2 latin=0 runes=506 sample=71 3123b25dd6f07f10
//   Deseret r16=0 r32=1 latin=0 runes=80 sample=11 fc0ff1f93972b62c
//   Devanagari r16=4 r32=1 latin=0 runes=164 sample=24 515e2a7a25171e48
//   Dives_Akuru r16=0 r32=8 latin=0 runes=72 sample=11 567b67189b646d03
//   Dogra r16=0 r32=1 latin=0 runes=60 sample=9 a7edf6283eb6b62c
//   Duployan r16=0 r32=5 latin=0 runes=143 sample=21 3f1e1625e8251e48
//   Egyptian_Hieroglyphs r16=0 r32=1 latin=0 runes=1110 sample=159 0631ee4ada70b62c
//   Elbasan r16=0 r32=1 latin=0 runes=40 sample=5 9411a4fa3bbab62c
//   Elymaic r16=0 r32=1 latin=0 runes=23 sample=4 6305f66c9977b62c
//   Ethiopic r16=32 r32=4 latin=0 runes=523 sample=72 adf698be3b5eea9b
//   Georgian r16=8 r32=0 latin=0 runes=173 sample=25 d641a25213e05105
//   Glagolitic r16=1 r32=5 latin=0 runes=134 sample=18 53578560bb26760b
//   Gothic r16=0 r32=1 latin=0 runes=27 sample=4 f71bbbd515f3b62c
//   Grantha r16=0 r32=14 latin=0 runes=85 sample=10 efdc9274e5f4878d
//   Greek r16=29 r32=3 latin=0 runes=518 sample=72 6548be0124234fab
//   Gujarati r16=14 r32=0 latin=0 runes=91 sample=12 f93bdbde906dac0a
//   Gunjala_Gondi r16=0 r32=6 latin=0 runes=63 sample=10 b5046e5c2607760b
//   Gurmukhi r16=16 r32=0 latin=0 runes=80 sample=12 a10024e1a66141e2
//   Han r16=10 r32=10 latin=0 runes=98408 sample=12754 bdb731ca48b13370
//   Hangul r16=14 r32=0 latin=0 runes=11739 sample=1679 dda63fcaa985f2f3
//   Hanifi_Rohingya r16=0 r32=2 latin=0 runes=50 sample=7 08358a3c19ce8e77
//   Hanunoo r16=1 r32=0 latin=0 runes=21 sample=3 a346a305ee9db62c
//   Hatran r16=0 r32=3 latin=0 runes=26 sample=4 5e6f66d1e9221282
//   Hebrew r16=9 r32=0 latin=0 runes=134 sample=20 a3d4a817ce5b4a99
//   Hiragana r16=2 r32=4 latin=0 runes=381 sample=55 e50cd4aefe5017aa
//   Imperial_Aramaic r16=0 r32=2 latin=0 runes=31 sample=5 722e1efb63128e77
//   Inherited r16=18 r32=9 latin=0 runes=657 sample=61 8dcb7002505a56ba
//   Inscriptional_Pahlavi r16=0 r32=2 latin=0 runes=27 sample=3 570fca22d9f18e77
//   Inscriptional_Parthian r16=0 r32=2 latin=0 runes=30 sample=4 778afc0bf5128e77
//   Javanese r16=3 r32=0 latin=0 runes=90 sample=12 f9c4460a4ae71282
//   Kaithi r16=0 r32=2 latin=0 runes=68 sample=10 ac4e6e5f5e978e77
//   Kannada r16=13 r32=0 latin=0 runes=91 sample=11 30f5b3e1f4a629c0
//   Katakana r16=7 r32=7 latin=0 runes=321 sample=45 9eb6c7fe5705eb30
//   Kawi r16=0 r32=3 latin=0 runes=86 sample=13 bb71ec1c587f1282
//   Kayah_Li r16=2 r32=0 latin=0 runes=47 sample=7 2ac7daf6dcca8e77
//   Kharoshthi r16=0 r32=8 latin=0 runes=68 sample=11 a0020fc1c471c74d
//   Khitan_Small_Script r16=0 r32=2 latin=0 runes=471 sample=67 f934ad09d9b88a6e
//   Khmer r16=4 r32=0 latin=0 runes=146 sample=19 f4695e8edd9d7499
//   Khojki r16=0 r32=2 latin=0 runes=65 sample=9 4b8a13ac77948e77
//   Khudawadi r16=0 r32=2 latin=0 runes=69 sample=11 ba7df0b8bad38e77
//   Lao r16=11 r32=0 latin=0 runes=83 sample=12 39f9ecd9572052f3
//   Latin r16=31 r32=5 latin=5 runes=1481 sample=213 cf14862e44db0477
//   Lepcha r16=3 r32=0 latin=0 runes=74 sample=11 3d6a72c97ac51282
//   Limbu r16=5 r32=0 latin=0 runes=68 sample=9 c05ce13767bacb29
//   Linear_A r16=0 r32=3 latin=0 runes=341 sample=50 9364ce200ac61282
//   Linear_B r16=0 r32=7 latin=0 runes=211 sample=30 e7098361dc1a92fe
//   Lisu r16=1 r32=1 latin=0 runes=49 sample=7 194f291a9aeb8e77
//   Lycian r16=0 r32=1 latin=0 runes=29 sample=4 4d194a83f515b62c
//   Lydian r16=0 r32=2 latin=0 runes=27 sample=4 158ab40ae0ae8e77
//   Mahajani r16=0 r32=1 latin=0 runes=39 sample=5 e2fcef8cbdf7b62c
//   Makasar r16=0 r32=1 latin=0 runes=25 sample=3 68e9138620a9b62c
//   Malayalam r16=7 r32=0 latin=0 runes=118 sample=17 8929d071662592fe
//   Mandaic r16=2 r32=0 latin=0 runes=29 sample=5 4c6b0847434d8e77
//   Manichaean r16=0 r32=2 latin=0 runes=51 sample=7 cc3acf4cf4f88e77
//   Marchen r16=0 r32=3 latin=0 runes=68 sample=9 c8b2a405ddc81282
//   Masaram_Gondi r16=0 r32=7 latin=0 runes=75 sample=11 dcabfd2124af594c
//   Medefaidrin r16=0 r32=1 latin=0 runes=91 sample=13 47a873edd6e3b62c
//   Meetei_Mayek r16=3 r32=0 latin=0 runes=79 sample=11 e2ec6033eb201282
//   Mende_Kikakui r16=0 r32=2 latin=0 runes=213 sample=30 5e8bd87d2e568e77
//   Meroitic_Cursive r16=0 r32=3 latin=0 runes=90 sample=12 07c8afc2a1c31282
//   Meroitic_Hieroglyphs r16=0 r32=1 latin=0 runes=32 sample=5 4c09ff8fd862b62c
//   Miao r16=0 r32=3 latin=0 runes=149 sample=22 46d14dfc4bfc1282
//   Modi r16=0 r32=2 latin=0 runes=79 sample=11 183aff5d58718e77
//   Mongolian r16=5 r32=1 latin=0 runes=168 sample=24 55c97bd581eb1f26
//   Mro r16=0 r32=3 latin=0 runes=43 sample=6 8efc85af6c0a1282
//   Multani r16=0 r32=5 latin=0 runes=38 sample=6 87a424ac18d01543
//   Myanmar r16=3 r32=0 latin=0 runes=223 sample=31 a8fd4cc5a0b41282
//   Nabataean r16=0 r32=2 latin=0 runes=40 sample=5 8174fd4112358e77
//   Nag_Mundari r16=0 r32=1 latin=0 runes=42 sample=6 69d866b3945cb62c
//   Nandinagari r16=0 r32=3 latin=0 runes=65 sample=10 89469ce81c8a1282
//   New_Tai_Lue r16=4 r32=0 latin=0 runes=83 sample=13 6f28b70d332e7499
//   Newa r16=0 r32=2 latin=0 runes=97 sample=14 3e714a0ba32a8e77
//   Nko r16=2 r32=0 latin=0 runes=62 sample=8 0024d29dbda98e77
//   Nushu r16=0 r32=2 latin=0 runes=397 sample=56 e5db8266034d8ced
//   Nyiakeng_Puachue_Hmong r16=0 r32=4 latin=0 runes=71 sample=12 bf0f1b001fb27499
//   Ogham r16=1 r32=0 latin=0 runes=29 sample=4 4b444ce55915b62c
//   Ol_Chiki r16=1 r32=0 latin=0 runes=48 sample=7 ea33d6dee642b62c
//   Old_Hungarian r16=0 r32=3 latin=0 runes=108 sample=15 8dd9b68dcc2f1282
//   Old_Italic r16=0 r32=2 latin=0 runes=39 sample=6 9f4cada4e8288e77
//   Old_North_Arabian r16=0 r32=1 latin=0 runes=32 sample=4 4c0bb2918b62b62c
//   Old_Permic r16=0 r32=1 latin=0 runes=43 sample=6 e71b8574fac3b62c
//   Old_Persian r16=0 r32=2 latin=0 runes=50 sample=7 dae5cb813bd68e77
//   Old_Sogdian r16=0 r32=1 latin=0 runes=40 sample=6 9407730b39bab62c
//   Old_South_Arabian r16=0 r32=1 latin=0 runes=32 sample=5 ec0be8f0e842b62c
//   Old_Turkic r16=0 r32=1 latin=0 runes=73 sample=10 f9025a06cc59b62c
//   Old_Uyghur r16=0 r32=1 latin=0 runes=26 sample=4 3a06eb1c53ccb62c
//   Oriya r16=14 r32=0 latin=0 runes=91 sample=15 139ee28f430ef2f3
//   Osage r16=0 r32=2 latin=0 runes=72 sample=10 e0db41fabd748e77
//   Osmanya r16=0 r32=2 latin=0 runes=40 sample=6 03b1314ee6088e77
//   Pahawh_Hmong r16=0 r32=5 latin=0 runes=127 sample=18 37a9d15d89241e48
//   Palmyrene r16=0 r32=1 latin=0 runes=32 sample=5 ec0882ed8242b62c
//   Pau_Cin_Hau r16=0 r32=1 latin=0 runes=57 sample=8 68f015ececa9b62c
//   Phags_Pa r16=1 r32=0 latin=0 runes=56 sample=8 e1b296f754aab62c
//   Phoenician r16=0 r32=2 latin=0 runes=29 sample=4 f5c8b9c993808e77
//   Psalter_Pahlavi r16=0 r32=3 latin=0 runes=29 sample=4 4404d6f7fee81282
//   Rejang r16=2 r32=0 latin=0 runes=37 sample=5 7ae88faf20088e77
//   Runic r16=2 r32=0 latin=0 runes=86 sample=13 ce42e3a544868e77
//   Samaritan r16=2 r32=0 latin=0 runes=61 sample=9 2d23a50c33e78e77
//   Saurashtra r16=2 r32=0 latin=0 runes=82 sample=12 b20ab68597b08e77
//   Sharada r16=0 r32=1 latin=0 runes=96 sample=13 8bfc679ddd22b62c
//   Shavian r16=0 r32=1 latin=0 runes=48 sample=7 ec0fa0691e42b62c
//   Siddham r16=0 r32=2 latin=0 runes=92 sample=13 8c43c7a5994c8e77
//   SignWriting r16=0 r32=3 latin=0 runes=672 sample=96 6771416e933f1282
//   Sinhala r16=12 r32=1 latin=0 runes=111 sample=16 c1e7c0c0930a215f
//   Sogdian r16=0 r32=1 latin=0 runes=42 sample=6 0a0757dc023cb62c
//   Sora_Sompeng r16=0 r32=2 latin=0 runes=35 sample=5 b3fc401a33b58e77
//   Soyombo r16=0 r32=1 latin=0 runes=83 sample=12 0ef10a9c53bbb62c
//   Sundanese r16=2 r32=0 latin=0 runes=72 sample=10 8f136e7f7bf48e77
//   Syloti_Nagri r16=1 r32=0 latin=0 runes=45 sample=7 9ab303b6dc05b62c
//   Syriac r16=4 r32=0 latin=0 runes=88 sample=12 c481278c039b7499
//   Tagalog r16=2 r32=0 latin=0 runes=23 sample=3 340704ef6d928e77
//   Tagbanwa r16=3 r32=0 latin=0 runes=18 sample=3 0c29bbfb46e71282
//   Tai_Le r16=2 r32=0 latin=0 runes=35 sample=4 472c5223d1998e77
//   Tai_Tham r16=5 r32=0 latin=0 runes=127 sample=18 0c037a2816821e48
//   Tai_Viet r16=2 r32=0 latin=0 runes=72 sample=10 bca937b46c018e77
//   Takri r16=0 r32=2 latin=0 runes=68 sample=9 a34e4c4e42a48e77
//   Tamil r16=15 r32=2 latin=0 runes=123 sample=17 793aba8c64177edb
//   Tangsa r16=0 r32=2 latin=0 runes=89 sample=13 bfe4bfaa5fe78e77
//   Tangut r16=0 r32=4 latin=0 runes=6914 sample=988 829462b7302b64a0
//   Telugu r16=13 r32=0 latin=0 runes=100 sample=16 c7f224a09556a2e2
//   Thaana r16=1 r32=0 latin=0 runes=50 sample=7 20612fcb9304b62c
//   Thai r16=2 r32=0 latin=0 runes=86 sample=12 446ab42ac1bd8e77
//   Tibetan r16=7 r32=0 latin=0 runes=207 sample=30 ea6afa12c56c92fe
//   Tifinagh r16=3 r32=0 latin=0 runes=59 sample=8 55f20f3cce161282
//   Tirhuta r16=0 r32=2 latin=0 runes=82 sample=12 8998d4c4f2ce8e77
//   Toto r16=0 r32=1 latin=0 runes=31 sample=5 1ae3057d401fb62c
//   Ugaritic r16=0 r32=2 latin=0 runes=31 sample=4 1146f602730a8e77
//   Vai r16=1 r32=0 latin=0 runes=300 sample=43 95bb82a5d586b62c
//   Vithkuqi r16=0 r32=8 latin=0 runes=70 sample=10 45970a2f17fdc74d
//   Wancho r16=0 r32=2 latin=0 runes=59 sample=9 917e1da7422e8e77
//   Warang_Citi r16=0 r32=2 latin=0 runes=84 sample=12 a31dfeaec6f18e77
//   Yezidi r16=0 r32=3 latin=0 runes=47 sample=7 f861c87345ed1282
//   Yi r16=2 r32=0 latin=0 runes=1220 sample=174 f86190e4b5ae8e77
//   Zanabazar_Square r16=0 r32=1 latin=0 runes=72 sample=10 f3f15c2c25dab62c
// Properties 35
//   ASCII_Hex_Digit r16=3 r32=0 latin=3 runes=22 sample=4 fbe2687fae311282
//   Bidi_Control r16=4 r32=0 latin=0 runes=12 sample=1 973e783cd9a22290
//   Dash r16=13 r32=1 latin=0 runes=30 sample=5 cb26d9a19013c485
//   Deprecated r16=5 r32=1 latin=0 runes=15 sample=3 dfabe96d11dd48c1
//   Diacritic r16=110 r32=53 latin=3 runes=1144 sample=160 a051c4fc624d6759
//   Extender r16=15 r32=8 latin=0 runes=50 sample=6 3cab1a463b4bec0c
//   Hex_Digit r16=6 r32=0 latin=3 runes=44 sample=8 bc41b7bf3f70760b
//   Hyphen r16=6 r32=0 latin=1 runes=11 sample=0 22cf79d2a79357b1
//   IDS_Binary_Operator r16=2 r32=0 latin=0 runes=10 sample=1 49242ac074428e77
//   IDS_Trinary_Operator r16=1 r32=0 latin=0 runes=2 sample=0 5e897385f45eb62c
//   Ideographic r16=7 r32=13 latin=0 runes=105854 sample=13815 59101994e9749f7a
//   Join_Control r16=1 r32=0 latin=0 runes=2 sample=1 bc9a234efc18b62c
//   Logical_Order_Exception r16=6 r32=0 latin=0 runes=19 sample=3 6076500b1cd28d0b
//   Noncharacter_Code_Point r16=2 r32=16 latin=0 runes=66 sample=5 ca52fb179e1e7047
//   Other_Alphabetic r16=148 r32=82 latin=0 runes=1425 sample=208 91431ba4b6415b4a
//   Other_Default_Ignorable_Code_Point r16=5 r32=4 latin=0 runes=3776 sample=4 d93a10c4ce6b8e9b
//   Other_Grapheme_Extend r16=10 r32=6 latin=0 runes=127 sample=7 bb5ed6ae8cc2eaad
//   Other_ID_Continue r16=3 r32=0 latin=0 runes=12 sample=3 735db5983319e331
//   Other_ID_Start r16=3 r32=0 latin=0 runes=6 sample=0 05d445b6126b9efd
//   Other_Lowercase r16=20 r32=5 latin=1 runes=311 sample=43 7928dbb7d714538e
//   Other_Math r16=63 r32=55 latin=0 runes=1362 sample=192 e5bdb014a3a36e6e
//   Other_Uppercase r16=2 r32=3 latin=0 runes=120 sample=18 192db35361401e48
//   Pattern_Syntax r16=24 r32=0 latin=10 runes=2760 sample=394 97c3d2544040836c
//   Pattern_White_Space r16=4 r32=0 latin=2 runes=11 sample=2 d555a41135f54f4d
//   Prepended_Concatenation_Mark r16=4 r32=1 latin=0 runes=13 sample=2 ea9882cff2af65b0
//   Quotation_Mark r16=10 r32=0 latin=2 runes=30 sample=2 6a163551ae2e6ed7
//   Radical r16=3 r32=0 latin=0 runes=329 sample=48 86439ad5e1f71282
//   Regional_Indicator r16=0 r32=1 latin=0 runes=26 sample=4 6bc560e146c2b62c
//   STerm r16=38 r32=27 latin=1 runes=154 sample=19 be1373c9abcaefc7
//   Sentence_Terminal r16=38 r32=27 latin=1 runes=154 sample=19 be1373c9abcaefc7
//   Soft_Dotted r16=10 r32=15 latin=1 runes=50 sample=7 d74dfa47d3d233f6
//   Terminal_Punctuation r16=55 r32=37 latin=3 runes=278 sample=38 b54cd78a5e9b797f
//   Unified_Ideograph r16=8 r32=7 latin=0 runes=97058 sample=12561 a48246f455c44e16
//   Variation_Selector r16=3 r32=1 latin=0 runes=260 sample=2 57959356cf776929
//   White_Space r16=7 r32=0 latin=2 runes=25 sample=4 6df93e940849a1f8
// FoldCategory 6
//   L r16=1 r32=0 latin=0 runes=1 sample=0 f46863791440b62c
//   Ll r16=108 r32=10 latin=3 runes=1391 sample=201 9d1433690a83d060
//   Lt r16=9 r32=0 latin=0 runes=35 sample=6 5d9da43b0fc16415
//   Lu r16=109 r32=10 latin=4 runes=1381 sample=194 d90e0b6b1ccc3f0f
//   M r16=2 r32=0 latin=0 runes=3 sample=0 cb63e3aca3a2091a
//   Mn r16=2 r32=0 latin=0 runes=3 sample=0 cb63e3aca3a2091a
// FoldScript 3
//   Common r16=1 r32=0 latin=0 runes=2 sample=1 2b67fece71b5817f
//   Greek r16=1 r32=0 latin=0 runes=2 sample=0 f4615389143e468f
//   Inherited r16=2 r32=0 latin=0 runes=3 sample=0 cb63e3aca3a2091a
// U+0041 00208003 true true true false
// U+007A 00008005 true true true false
// U+0030 00200010 true true false false
// U+0663 00000010 true true false false
// U+0020 00100400 true true false false
// U+00A0 00100400 true true false false
// U+00E9 00008005 true true true false
// U+01C5 00008009 true true true false
// U+00DF 00008005 true true true false
// U+03A3 00020003 true true true false
// U+03C2 00020005 true true true false
// U+0436 00010005 true true true false
// U+65E5 00004001 true true true true
// U+D55C 00040001 true true true false
// U+05D0 00000001 true true true false
// U+0621 00000001 true true true false
// U+0301 00000200 true true false false
// U+20AC 00000100 true true false false
// U+1F600 00000100 true true false false
// U+1D7D8 00000010 true true false false
// U+2028 00100000 true true false false
// U+10FFFF 00000000 false false false false
// U+E000 00001000 true true false false
// U+D800 00002000 true true false false
// U+FFFFFFFFFFFFFFFF 00000000 false false false false
// U+110000 00000000 false false false false
// 6 5 328
// true true true true true true true true true true true
