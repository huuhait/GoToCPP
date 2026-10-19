// phase: 09
// Differential sweep of package regexp/syntax: 300 pseudo-random patterns (the generator of regexp_diff_match) are parsed with each of eight
// flag sets (Perl, POSIX, Literal, FoldCase, Perl without ClassNL, DotNL, OneLine, PerlX only); the String of the tree, of its simplification and of its
// program, the Prefix and StartCond of the program, MaxCap, CapNames, Equal, the instruction fields, and MatchRune, MatchRunePos, MatchEmptyWidth
// of the instructions on 12 runes are hashed, with the error texts. The expected output comes from real Go 1.25 (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"fmt"
	"regexp/syntax"
	"strings"
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

type hasher struct{ h uint64 }

func newHasher() *hasher { return &hasher{14695981039346656037} }
func (x *hasher) u64(v uint64) {
	for i := 0; i < 8; i++ {
		x.h ^= v & 0xff
		x.h *= 1099511628211
		v >>= 8
	}
}
func (x *hasher) str(s string) {
	for i := 0; i < len(s); i++ {
		x.h ^= uint64(s[i])
		x.h *= 1099511628211
	}
	x.h ^= 0xff
	x.h *= 1099511628211
}
func (x *hasher) ints(v []int) {
	if v == nil {
		x.u64(1 << 62)
		return
	}
	x.u64(uint64(len(v)))
	for _, i := range v {
		x.u64(uint64(i))
	}
}
func (x *hasher) intss(v [][]int) {
	if v == nil {
		x.u64(1 << 61)
		return
	}
	x.u64(uint64(len(v)))
	for _, a := range v {
		x.ints(a)
	}
}

var lits = []string{"a", "b", "c", "ab", "ba", "abc", "é", "世", "x", "\\.", "\\n", " ", "A", "B", "1", "_", "\\x41", "\\x{e9}", "\\101", "\\t", "\\Qa.b\\E", "-", "ß", "K", "k", "\\*"}
// Classes with a range that reaches into the folding range (a negated class, \\D, \\W, \\S, the full range) make Regexp.String slow (it asks
// unicode.SimpleFold for every rune of the range), so they are chosen rarely.
var classes = []string{".", "[abc]", "[a-c]", "\\d", "\\w", "\\s", "[[:alpha:]]", "\\p{Greek}", "\\p{Cherokee}", "[\\p{Nd}a]", "[a-zé-世]", "[\\d\\s]", "[é]", "[[:word:]]", "[a-c-e]", "[\\-a]", "[]a]", "\\p{Zs}", "[[:upper:][:digit:]]", "[a-c]", "[é世]"}
var heavyClasses = []string{"[^a]", "[^a-c\\n]", "\\D", "\\W", "\\S", "[[:^digit:]]", "[^é]", "\\p{^Zs}", "[\\x00-\\x{10FFFF}]", "[^]a]"}

func pickClass(r *rngT) string {
	if r.intn(30) == 0 {
		return heavyClasses[r.intn(len(heavyClasses))]
	}
	return classes[r.intn(len(classes))]
}

var anchors = []string{"^", "$", "\\b", "\\B", "\\A", "\\z", "^", "$"}
var repeats = []string{"*", "+", "?", "*?", "+?", "??", "{2}", "{1,3}", "{2,}", "{0}", "{0,1}", "{1,3}?", "{2,}?", "{,3}"}
var flagsets = []string{"(?i)", "(?s)", "(?m)", "(?U)", "(?is)", "(?im)", "(?i-s)", "(?msU)", ""}
var bad = []string{"(", ")", "[", "\\", "*", "{2,1}", "\\8", "(?<", "(?P=name)", "\\pQ", "[z-a]", "a**", "(?P<n>a)(?P<n>b)", "(?i", "\\Q", "[[:foo:]]", "x{1001}", "(?:", "(?#c)", "\\C"}

var nameN int

func gen(r *rngT, depth int) string {
	var alts []string
	for na := 1 + r.intn(2)*r.intn(3); na > 0; na-- {
		var b strings.Builder
		for np := 1 + r.intn(4); np > 0; np-- {
			var atom string
			k := r.intn(20)
			switch {
			case k < 6:
				atom = lits[r.intn(len(lits))]
			case k < 11:
				atom = pickClass(r)
			case k < 13:
				atom = anchors[r.intn(len(anchors))]
			case k < 18 && depth < 2:
				inner := gen(r, depth+1)
				switch r.intn(7) {
				case 0, 1, 2:
					atom = "(" + inner + ")"
				case 3:
					atom = "(?:" + inner + ")"
				case 4:
					nameN++
					atom = fmt.Sprintf("(?P<n%d>%s)", nameN%5+1, inner)
				case 5:
					atom = "(?i:" + inner + ")"
				default:
					atom = "(?s-i:" + inner + ")"
				}
			case k < 19:
				atom = lits[r.intn(len(lits))]
			default:
				if r.intn(25) == 0 {
					atom = bad[r.intn(len(bad))]
				} else {
					atom = pickClass(r)
				}
			}
			b.WriteString(atom)
			if r.intn(3) == 0 {
				b.WriteString(repeats[r.intn(len(repeats))])
			}
		}
		alts = append(alts, b.String())
	}
	return strings.Join(alts, "|")
}


var runes = []rune{'a', 'b', 'c', 'A', 'é', '世', '\n', ' ', '1', '_', 0x212a, 0x10ffff, -1}

func hashProg(h *hasher, p *syntax.Prog) {
	h.str(p.String())
	h.u64(uint64(p.Start))
	h.u64(uint64(p.NumCap))
	pre, comp := p.Prefix()
	h.str(pre)
	h.u64(b2u(comp))
	h.u64(uint64(p.StartCond()))
	for i := range p.Inst {
		in := &p.Inst[i]
		h.u64(uint64(in.Op)<<32 | uint64(in.Out)<<16 | uint64(in.Arg))
		h.u64(uint64(len(in.Rune)))
		for _, c := range in.Rune {
			h.u64(uint64(c))
		}
		switch in.Op {
		case syntax.InstRune, syntax.InstRune1, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
			for _, r := range runes {
				h.u64(b2u(in.MatchRune(r)))
				pos := in.MatchRunePos(r)
				h.u64(uint64(pos + 5))
			}
		case syntax.InstEmptyWidth:
			for i, a := range runes {
				b := runes[(i+3)%len(runes)]
				h.u64(b2u(in.MatchRune(a)))
				h.u64(uint64(syntax.EmptyOpContext(a, b)))
			}
		}
	}
}

func hashTree(h *hasher, re *syntax.Regexp) {
	h.str(re.String())
	hashNodes(h, re)
}

func hashNodes(h *hasher, re *syntax.Regexp) {
	h.u64(uint64(re.Op)<<8 | uint64(re.Flags))
	h.u64(uint64(re.Min)<<20 | uint64(re.Max)<<4 | uint64(re.Cap))
	h.str(re.Name)
	for _, r := range re.Rune {
		h.u64(uint64(r))
	}
	for _, s := range re.Sub {
		hashNodes(h, s)
	}
}

func main() {
	r := &rngT{555}
	hs := map[string]*hasher{}
	var names []string
	get := func(n string) *hasher {
		h := hs[n]
		if h == nil {
			h = newHasher()
			hs[n] = h
			names = append(names, n)
		}
		return h
	}
	flagsets := []syntax.Flags{syntax.Perl, syntax.POSIX, syntax.Literal, syntax.Perl | syntax.FoldCase, syntax.Perl &^ syntax.ClassNL, syntax.Perl | syntax.DotNL, syntax.Perl | syntax.OneLine, syntax.PerlX | syntax.UnicodeGroups}
	nok, nbad := 0, 0
	for it := 0; it < 300; it++ {
		pat := flagsets2[r.intn(len(flagsets2))] + gen(r, 0)
		for fi, fl := range flagsets {
			re, err := syntax.Parse(pat, fl)
			if err != nil {
				nbad++
				get("ParseError").str(fmt.Sprint(fi, pat, " => ", err.Error()))
				continue
			}
			nok++
			hashTree(get("Parse"), re)
			get("MaxCap").u64(uint64(re.MaxCap()))
			for _, n := range re.CapNames() {
				get("CapNames").str(n)
			}
			s := re.Simplify()
			hashTree(get("Simplify"), s)
			get("Equal").u64(b2u(re.Equal(s)) | b2u(re.Equal(re))<<1)
			if re2, err := syntax.Parse(re.String(), fl); err == nil {
				get("Reparse").u64(b2u(re2.Equal(re)))
				get("Reparse").str(re2.String())
			} else {
				get("Reparse").str("error " + err.Error())
			}
			if fi < 3 || fi == 3 {
				p, err := syntax.Compile(s)
				if err != nil {
					get("CompileError").str(err.Error())
					continue
				}
				hashProg(get("Prog"), p)
			}
		}
	}
	fmt.Println("parsed", nok, "failed", nbad)
	for _, n := range names {
		fmt.Printf("%-12s %016x\n", n, hs[n].h)
	}
}

var flagsets2 = []string{"(?i)", "(?s)", "(?m)", "(?U)", "(?is)", "(?im)", "", "", "", ""}

func b2u(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

// Output:
// parsed 2100 failed 300
// Parse        62dc214c76531bf1
// MaxCap       efa87de686ee1e87
// CapNames     657601a52d4b6adb
// Simplify     4a24c0a392e54b69
// Equal        8b53f42329da2c25
// Reparse      65106e9c54d13486
// Prog         14eb8f454490ee8f
// ParseError   7782e542d5e7c200
