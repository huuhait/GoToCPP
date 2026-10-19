// phase: 09
// Differential sweep of package regexp: 1500 pseudo-random patterns plus 300 anchored one-pass patterns (literals, classes, Unicode classes, anchors, groups, named groups, flags,
// greedy and lazy repeats, a few invalid ones) run on 8 pseudo-random inputs each (ASCII, multi-byte, newlines, invalid UTF-8). Match,
// Find*Index, Find*SubmatchIndex, FindAll*, Replace* (templates, literal, func), Split, Expand, the Longest variants, the byte variants, and
// MatchReader / FindReaderIndex on readers that return one to three bytes at a time are hashed per function; the compile errors are hashed with their
// text. The expected output comes from real Go 1.25 (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
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
var classes = []string{".", "[abc]", "[^a]", "[a-c]", "[^a-c\\n]", "\\d", "\\D", "\\w", "\\W", "\\s", "\\S", "[[:alpha:]]", "[[:^digit:]]", "\\pL", "\\p{Greek}", "\\PL",
	"[\\p{Lu}a]", "[a-zé-世]", "[\\d\\s]", "[é]", "[^é]", "\\p{^Lu}", "[\\x00-\\x{10FFFF}]", "[[:word:]]", "[a-c-e]", "[\\-a]", "[]a]", "[^]a]", "\\pN", "\\p{Han}", "[[:upper:][:digit:]]"}
var anchors = []string{"^", "$", "\\b", "\\B", "\\A", "\\z", "^", "$"}
var repeats = []string{"*", "+", "?", "*?", "+?", "??", "{2}", "{1,3}", "{2,}", "{0}", "{0,1}", "{1,3}?", "{2,}?", "{,3}", "{1000}"}
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
				atom = classes[r.intn(len(classes))]
			case k < 13:
				atom = anchors[r.intn(len(anchors))]
			case k < 18 && depth < 3:
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
					atom = classes[r.intn(len(classes))]
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

// genOnePass builds an anchored pattern whose pieces start with different characters, so that it is a one-pass program (Go's
// compileOnePass accepts it and Regexp.doOnePass runs it), and a string that it matches.
var onePassPieces = []struct{ pat, str string }{{"a+", "aa"}, {"b*", "b"}, {"c?", "c"}, {"(d|e)", "e"}, {"[f-h]+", "gh"}, {"(i)", "i"}, {"[0-9]*", "42"}, {"_", "_"}, {`\s+`, " \n"}, {"é*", "éé"}, {"世+", "世"}, {"(?P<n1>x+)", "xx"}, {"(?:yz)?", "yz"}, {"(?i:k+)", "kK"}}

func genOnePass(r *rngT) (string, string) {
	perm := make([]int, len(onePassPieces))
	for i := range perm {
		perm[i] = i
	}
	for i := len(perm) - 1; i > 0; i-- {
		j := r.intn(i + 1)
		perm[i], perm[j] = perm[j], perm[i]
	}
	n := 2 + r.intn(5)
	pat, str := "^", ""
	for _, k := range perm[:n] {
		pat += onePassPieces[k].pat
		str += onePassPieces[k].str
	}
	switch r.intn(3) {
	case 0:
		pat += "$"
	case 1:
		pat += `\z`
	}
	return pat, str
}

var pool = []string{"a", "b", "c", "ab", "A", "B", "é", "世", "\n", " ", "1", "_", "\xff", "\xc3", "\xe4\xb8", "x", ".", "ß", "K", "k", "\t", "abab", "É", "K"}

func genInput(r *rngT) string {
	var b strings.Builder
	for n := r.intn(13); n > 0; n-- {
		b.WriteString(pool[r.intn(len(pool))])
	}
	return b.String()
}

// chunkReader returns one to three bytes per Read.
type chunkReader struct {
	s string
	r *rngT
}

func (c *chunkReader) Read(p []byte) (int, error) {
	if len(c.s) == 0 {
		return 0, eof
	}
	n := 1 + c.r.intn(3)
	if n > len(c.s) {
		n = len(c.s)
	}
	if n > len(p) {
		n = len(p)
	}
	copy(p, c.s[:n])
	c.s = c.s[n:]
	return n, nil
}

var eof = io.EOF

var templates = []string{"$0", "${0}x", "$1", "${1}", "$n1", "${n2}y", "$$", "$", "$x", "[$0]", "$1$2", "\\$1", "${", "a$1b"}

func main() {
	r := &rngT{777}
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
	ncomp, nbad := 0, 0
	for it := 0; it < 1800; it++ {
		pat := flagsets[r.intn(len(flagsets))] + gen(r, 0)
		sample := ""
		if it >= 1500 {
			pat, sample = genOnePass(r)
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			nbad++
			get("CompileError").str(pat + " => " + err.Error())
			continue
		}
		ncomp++
		get("Compile").str(re.String())
		get("Compile").u64(uint64(re.NumSubexp()))
		for _, n := range re.SubexpNames() {
			get("Names").str(n)
		}
		get("Names").u64(uint64(re.SubexpIndex("n1") + 7))
		pfx, comp := re.LiteralPrefix()
		get("LiteralPrefix").str(pfx)
		get("LiteralPrefix").u64(b2u(comp))
		relong := regexp.MustCompile(pat)
		relong.Longest()
		for k := 0; k < 8; k++ {
			s := genInput(r)
			if k == 0 && sample != "" {
				s = sample
			} else if k == 1 && sample != "" {
				s = sample + "x"
			} else if k == 2 && sample != "" {
				s = "z" + sample
			}
			b := []byte(s)
			get("MatchString").u64(b2u(re.MatchString(s)))
			get("Match").u64(b2u(re.Match(b)))
			get("FindStringIndex").ints(re.FindStringIndex(s))
			get("FindString").str(re.FindString(s))
			get("FindStringSubmatchIndex").ints(re.FindStringSubmatchIndex(s))
			get("FindSubmatch").str(fmt.Sprint(re.FindSubmatch(b)))
			get("FindAllStringSubmatchIndex").intss(re.FindAllStringSubmatchIndex(s, -1))
			get("FindAllStringIndex2").intss(re.FindAllStringIndex(s, 2))
			get("FindAllString").str(fmt.Sprintf("%q", re.FindAllString(s, -1)))
			get("FindAllSubmatch").str(fmt.Sprintf("%q", re.FindAllSubmatch(b, -1)))
			get("Longest").ints(relong.FindStringSubmatchIndex(s))
			get("LongestAll").intss(relong.FindAllStringIndex(s, -1))
			get("ReplaceAllLiteralString").str(re.ReplaceAllLiteralString(s, "<$1>"))
			get("ReplaceAllStringFunc").str(re.ReplaceAllStringFunc(s, strings.ToUpper))
			get("ReplaceAll").str(string(re.ReplaceAll(b, []byte("-$0-"))))
			t := templates[r.intn(len(templates))]
			get("ReplaceAllString").str(re.ReplaceAllString(s, t))
			get("Split").str(fmt.Sprintf("%q", re.Split(s, -1)))
			get("Split3").str(fmt.Sprintf("%q", re.Split(s, 3)))
			get("Split0").str(fmt.Sprintf("%q", re.Split(s, 0)))
			if m := re.FindStringSubmatchIndex(s); m != nil {
				get("Expand").str(string(re.ExpandString(nil, t, s, m)))
				get("Expand").str(string(re.Expand([]byte("x"), []byte(t), b, m)))
			}
			rr := bufio.NewReader(&chunkReader{s, r})
			get("MatchReader").u64(b2u(re.MatchReader(rr)))
			get("FindReaderIndex").ints(re.FindReaderIndex(bufio.NewReader(&chunkReader{s, r})))
			get("FindReaderSubmatchIndex").ints(re.FindReaderSubmatchIndex(strings.NewReader(s)))
		}
	}
	fmt.Println("compiled", ncomp, "invalid", nbad)
	for _, n := range names {
		fmt.Printf("%-26s %016x\n", n, hs[n].h)
	}
}

func b2u(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

// Output:
// compiled 1676 invalid 124
// Compile                    7f13a393d9b2ba16
// Names                      b452e6585e0f3a63
// LiteralPrefix              f3805568f5144d85
// MatchString                8b4826bff2edd085
// Match                      8b4826bff2edd085
// FindStringIndex            4d2e1bb0eec2e12e
// FindString                 bc246b527a1324a6
// FindStringSubmatchIndex    5c50d40464ff9f7a
// FindSubmatch               8e96ef53338ee36f
// FindAllStringSubmatchIndex c5a9ed1916e06fc1
// FindAllStringIndex2        c2d2fb101cc6ec6d
// FindAllString              73fb3e50248f438d
// FindAllSubmatch            c1b7298e27083811
// Longest                    10fee4c889cf44db
// LongestAll                 e7b78a4737f512e5
// ReplaceAllLiteralString    4ad70de0b63fcd03
// ReplaceAllStringFunc       9ac9f2880ba99786
// ReplaceAll                 4a87a0f74c1e1f5a
// ReplaceAllString           b88d767c1af58520
// Split                      b79796d34cd6e8e4
// Split3                     570e080224324a5a
// Split0                     a3f0bb3e4b3b2245
// Expand                     1b03e882e20e5da7
// MatchReader                8b4826bff2edd085
// FindReaderIndex            4d2e1bb0eec2e12e
// FindReaderSubmatchIndex    5c50d40464ff9f7a
// CompileError               b88a4d597957c1a1
