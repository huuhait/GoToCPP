// phase: 09
// Differential test of the Unicode classes and the case folding of package regexp: the parsed range lists of \p{Name}, \P{Name}, [^\p{Name}],
// \p{^Name} and the same with (?i) for every general category and script of package unicode (and the Perl and POSIX classes with the flags), the folded form of
// (?i) followed by each of 40000 code points, and the match of the orbit of SimpleFold for a sample of them are hashed, and 12000 pseudo-random
// code points are matched against 24 classes. The expected output comes from real Go 1.25 (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"sort"
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

func hashClass(h *hasher, pat string, flags syntax.Flags, withString bool) {
	re, err := syntax.Parse(pat, flags)
	if err != nil {
		h.str("error " + err.Error())
		return
	}
	if withString {
		h.str(re.String())
	}
	h.u64(uint64(re.Op))
	h.u64(uint64(len(re.Rune)))
	for _, r := range re.Rune {
		h.u64(uint64(r))
	}
	s := re.Simplify()
	if withString {
		h.str(s.String())
	}
}

func sortedKeys(m map[string]*unicode.RangeTable) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func main() {
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
	nclasses := 0
	for ti, tables := range []map[string]*unicode.RangeTable{unicode.Categories, unicode.Scripts, unicode.Properties, unicode.FoldCategory, unicode.FoldScript} {
		for ki, k := range sortedKeys(tables) {
			for _, form := range []struct {
				f    string
				fold bool
			}{{`\p{%s}`, false}, {`\p{%s}`, true}, {`\P{%s}`, false}, {`[^\p{%s}]`, true}, {`\p{^%s}`, false}} {
				if ti >= 2 && form.f != `\p{%s}` { // the properties and the fold tables: two forms
					continue
				}
				fl := syntax.Perl
				if form.fold {
					fl |= syntax.FoldCase
				}
				hashClass(get("ClassParse"), fmt.Sprintf(form.f, k), fl, form.f == `\p{%s}`) // the text of a negated class is long: only the ranges are hashed
				nclasses++
			}
			// the matcher itself, on a sample of runes, for every third name
			if ki%3 != 0 {
				continue
			}
			re, err := regexp.Compile(`\p{` + k + `}`)
			if err != nil {
				get("ClassMatch").str("error " + err.Error())
				continue
			}
			reI := regexp.MustCompile(`(?i)\p{` + k + `}`)
			h := get("ClassMatch")
			for r := rune(0); r < 0x3000; r += 37 {
				h.u64(uint64(b2u(re.MatchString(string(r)))) | uint64(b2u(reI.MatchString(string(r))))<<1)
			}
			for r := rune(0x3000); r <= unicode.MaxRune; r += 1223 {
				h.u64(uint64(b2u(re.MatchString(string(r)))) | uint64(b2u(reI.MatchString(string(r))))<<1)
			}
		}
	}
	fmt.Println("classes parsed:", nclasses)

	for _, pat := range []string{`\d`, `\D`, `\s`, `\S`, `\w`, `\W`, `[\d]`, `[^\d]`, `[\D]`, `[^\D]`, `[\s\S]`, `[\w-]`, `\b`, `[[:alnum:]]`, `[[:alpha:]]`, `[[:ascii:]]`, `[[:blank:]]`, `[[:cntrl:]]`, `[[:digit:]]`, `[[:graph:]]`, `[[:lower:]]`,
		`[[:print:]]`, `[[:punct:]]`, `[[:space:]]`, `[[:upper:]]`, `[[:word:]]`, `[[:xdigit:]]`, `[[:^alnum:]]`, `[[:^upper:]]`, `[^[:^upper:]]`, `.`, `(?s).`, `[^\n]`, `[^a-z]`, `[a-zA-Z0-9_]`} {
		for _, fl := range []syntax.Flags{syntax.Perl, syntax.Perl | syntax.FoldCase, syntax.POSIX, syntax.POSIX | syntax.FoldCase, syntax.Perl | syntax.DotNL, syntax.Perl &^ syntax.ClassNL, syntax.Perl | syntax.UnicodeGroups} {
			hashClass(get("PerlPosixClass"), pat, fl, true)
		}
	}

	// (?i) and each code point: the folded form and the match of the whole orbit of SimpleFold.
	hf := get("FoldForm")
	hm := get("FoldOrbitMatch")
	for r := rune(0); r < 0x20000; r++ {
		if r >= 0xD800 && r < 0xE000 {
			continue
		}
		if r < 0x2000 || r%5 == 0 {
			hashClass(hf, "(?i)"+string(r), syntax.Perl, true)
		}
		if r < 0x600 || r%61 == 0 {
			re := regexp.MustCompile("(?i)^" + regexp.QuoteMeta(string(r)) + "$")
			n := 0
			for f := unicode.SimpleFold(r); ; f = unicode.SimpleFold(f) {
				if !re.MatchString(string(f)) {
					n |= 1 << 20
				}
				n++
				if f == r {
					break
				}
			}
			hm.u64(uint64(n))
		}
	}

	// A sample of runes against a set of classes with and without folding.
	r := &rngT{31337}
	pats := []string{`\pL`, `\p{Lu}`, `\p{Ll}`, `\pN`, `\p{Greek}`, `\p{Han}`, `[a-z]`, `[A-Z]`, `[^a-z]`, `\w`, `\d`, `\s`, `[[:alpha:]]`, `[[:upper:]]`, `\p{Cyrillic}`, `\p{Arabic}`, `\p{Mn}`, `\p{Zs}`, `\p{P}`, `[\p{Lu}\p{Nd}]`, `[^\p{L}]`, `k`, `s`, `ǆ`}
	var res []*regexp.Regexp
	for _, p := range pats {
		res = append(res, regexp.MustCompile(p), regexp.MustCompile("(?i)"+p))
	}
	hmm := get("RuneMatch")
	for i := 0; i < 12000; i++ {
		var c rune
		switch r.intn(4) {
		case 0:
			c = rune(r.intn(0x300))
		case 1:
			c = rune(r.intn(0x3000))
		case 2:
			c = rune(r.intn(0x20000))
		default:
			c = rune(r.intn(0x110000))
		}
		s := string(c)
		var bits uint64
		for j, re := range res {
			bits |= b2u(re.MatchString(s)) << uint(j)
		}
		hmm.u64(bits)
	}
	for _, n := range names {
		fmt.Printf("%-16s %016x\n", n, hs[n].h)
	}
}

func b2u(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

// Output:
// classes parsed: 1093
// ClassParse       051b9a5e6843049d
// ClassMatch       04aa3be1a2150170
// PerlPosixClass   73acf810a6985296
// FoldForm         82abe204315ee0f7
// FoldOrbitMatch   6833129b37d439e0
// RuneMatch        7d9fe91c1e1f8565
