// phase: 09
// Go 1.25's regexp/syntax/parse_test.go (Parse with every flag combination, the dumps of the trees, the Perl and POSIX classes, case folding, the invalid patterns and their error texts, the repeat and nesting limits, String of the parsed tree, minFold and maxFold against package unicode) through a small harness; it is an internal test of package syntax: minFold and maxFold are Go's constants copied, and TestAppendRangeCollapse (the unexported appendRange) is dropped; expected output from real Go.

package main

import (
	"fmt"
	"regexp/syntax"
	"strings"
	"sync/atomic"
	"unicode"
)


type parseTest struct {
	Regexp string
	Dump   string
}

var parseTests = []parseTest{
	// Base cases
	{`a`, `lit{a}`},
	{`a.`, `cat{lit{a}dot{}}`},
	{`a.b`, `cat{lit{a}dot{}lit{b}}`},
	{`ab`, `str{ab}`},
	{`a.b.c`, `cat{lit{a}dot{}lit{b}dot{}lit{c}}`},
	{`abc`, `str{abc}`},
	{`a|^`, `alt{lit{a}bol{}}`},
	{`a|b`, `cc{0x61-0x62}`},
	{`(a)`, `cap{lit{a}}`},
	{`(a)|b`, `alt{cap{lit{a}}lit{b}}`},
	{`a*`, `star{lit{a}}`},
	{`a+`, `plus{lit{a}}`},
	{`a?`, `que{lit{a}}`},
	{`a{2}`, `rep{2,2 lit{a}}`},
	{`a{2,3}`, `rep{2,3 lit{a}}`},
	{`a{2,}`, `rep{2,-1 lit{a}}`},
	{`a*?`, `nstar{lit{a}}`},
	{`a+?`, `nplus{lit{a}}`},
	{`a??`, `nque{lit{a}}`},
	{`a{2}?`, `nrep{2,2 lit{a}}`},
	{`a{2,3}?`, `nrep{2,3 lit{a}}`},
	{`a{2,}?`, `nrep{2,-1 lit{a}}`},
	// Malformed { } are treated as literals.
	{`x{1001`, `str{x{1001}`},
	{`x{9876543210`, `str{x{9876543210}`},
	{`x{9876543210,`, `str{x{9876543210,}`},
	{`x{2,1`, `str{x{2,1}`},
	{`x{1,9876543210`, `str{x{1,9876543210}`},
	{``, `emp{}`},
	{`|`, `emp{}`}, // alt{emp{}emp{}} but got factored
	{`|x|`, `alt{emp{}lit{x}emp{}}`},
	{`.`, `dot{}`},
	{`^`, `bol{}`},
	{`$`, `eol{}`},
	{`\|`, `lit{|}`},
	{`\(`, `lit{(}`},
	{`\)`, `lit{)}`},
	{`\*`, `lit{*}`},
	{`\+`, `lit{+}`},
	{`\?`, `lit{?}`},
	{`{`, `lit{{}`},
	{`}`, `lit{}}`},
	{`\.`, `lit{.}`},
	{`\^`, `lit{^}`},
	{`\$`, `lit{$}`},
	{`\\`, `lit{\}`},
	{`[ace]`, `cc{0x61 0x63 0x65}`},
	{`[abc]`, `cc{0x61-0x63}`},
	{`[a-z]`, `cc{0x61-0x7a}`},
	{`[a]`, `lit{a}`},
	{`\-`, `lit{-}`},
	{`-`, `lit{-}`},
	{`\_`, `lit{_}`},
	{`abc`, `str{abc}`},
	{`abc|def`, `alt{str{abc}str{def}}`},
	{`abc|def|ghi`, `alt{str{abc}str{def}str{ghi}}`},

	// Posix and Perl extensions
	{`[[:lower:]]`, `cc{0x61-0x7a}`},
	{`[a-z]`, `cc{0x61-0x7a}`},
	{`[^[:lower:]]`, `cc{0x0-0x60 0x7b-0x10ffff}`},
	{`[[:^lower:]]`, `cc{0x0-0x60 0x7b-0x10ffff}`},
	{`(?i)[[:lower:]]`, `cc{0x41-0x5a 0x61-0x7a 0x17f 0x212a}`},
	{`(?i)[a-z]`, `cc{0x41-0x5a 0x61-0x7a 0x17f 0x212a}`},
	{`(?i)[^[:lower:]]`, `cc{0x0-0x40 0x5b-0x60 0x7b-0x17e 0x180-0x2129 0x212b-0x10ffff}`},
	{`(?i)[[:^lower:]]`, `cc{0x0-0x40 0x5b-0x60 0x7b-0x17e 0x180-0x2129 0x212b-0x10ffff}`},
	{`\d`, `cc{0x30-0x39}`},
	{`\D`, `cc{0x0-0x2f 0x3a-0x10ffff}`},
	{`\s`, `cc{0x9-0xa 0xc-0xd 0x20}`},
	{`\S`, `cc{0x0-0x8 0xb 0xe-0x1f 0x21-0x10ffff}`},
	{`\w`, `cc{0x30-0x39 0x41-0x5a 0x5f 0x61-0x7a}`},
	{`\W`, `cc{0x0-0x2f 0x3a-0x40 0x5b-0x5e 0x60 0x7b-0x10ffff}`},
	{`(?i)\w`, `cc{0x30-0x39 0x41-0x5a 0x5f 0x61-0x7a 0x17f 0x212a}`},
	{`(?i)\W`, `cc{0x0-0x2f 0x3a-0x40 0x5b-0x5e 0x60 0x7b-0x17e 0x180-0x2129 0x212b-0x10ffff}`},
	{`[^\\]`, `cc{0x0-0x5b 0x5d-0x10ffff}`},
	//	{ `\C`, `byte{}` },  // probably never

	// Unicode, negatives, and a double negative.
	{`\p{Braille}`, `cc{0x2800-0x28ff}`},
	{`\P{Braille}`, `cc{0x0-0x27ff 0x2900-0x10ffff}`},
	{`\p{^Braille}`, `cc{0x0-0x27ff 0x2900-0x10ffff}`},
	{`\P{^Braille}`, `cc{0x2800-0x28ff}`},
	{`\pZ`, `cc{0x20 0xa0 0x1680 0x2000-0x200a 0x2028-0x2029 0x202f 0x205f 0x3000}`},
	{`[\p{Braille}]`, `cc{0x2800-0x28ff}`},
	{`[\P{Braille}]`, `cc{0x0-0x27ff 0x2900-0x10ffff}`},
	{`[\p{^Braille}]`, `cc{0x0-0x27ff 0x2900-0x10ffff}`},
	{`[\P{^Braille}]`, `cc{0x2800-0x28ff}`},
	{`[\pZ]`, `cc{0x20 0xa0 0x1680 0x2000-0x200a 0x2028-0x2029 0x202f 0x205f 0x3000}`},
	{`\p{Lu}`, mkCharClass(unicode.IsUpper)},
	{`\p{Uppercase_Letter}`, mkCharClass(unicode.IsUpper)},
	{`\p{upper case-let ter}`, mkCharClass(unicode.IsUpper)},
	{`\p{__upper case-let ter}`, mkCharClass(unicode.IsUpper)},
	{`[\p{Lu}]`, mkCharClass(unicode.IsUpper)},
	{`(?i)[\p{Lu}]`, mkCharClass(isUpperFold)},
	{`\p{Any}`, `dot{}`},
	{`\p{^Any}`, `cc{}`},
	{`(?i)\p{ascii}`, `cc{0x0-0x7f 0x17f 0x212a}`},
	{`\p{Assigned}`, mkCharClass(func(r rune) bool { return !unicode.In(r, unicode.Cn) })},
	{`\p{^Assigned}`, mkCharClass(func(r rune) bool { return unicode.In(r, unicode.Cn) })},

	// Hex, octal.
	{`[\012-\234]\141`, `cat{cc{0xa-0x9c}lit{a}}`},
	{`[\x{41}-\x7a]\x61`, `cat{cc{0x41-0x7a}lit{a}}`},

	// More interesting regular expressions.
	{`a{,2}`, `str{a{,2}}`},
	{`\.\^\$\\`, `str{.^$\}`},
	{`[a-zABC]`, `cc{0x41-0x43 0x61-0x7a}`},
	{`[^a]`, `cc{0x0-0x60 0x62-0x10ffff}`},
	{`[α-ε☺]`, `cc{0x3b1-0x3b5 0x263a}`}, // utf-8
	{`a*{`, `cat{star{lit{a}}lit{{}}`},

	// Test precedences
	{`(?:ab)*`, `star{str{ab}}`},
	{`(ab)*`, `star{cap{str{ab}}}`},
	{`ab|cd`, `alt{str{ab}str{cd}}`},
	{`a(b|c)d`, `cat{lit{a}cap{cc{0x62-0x63}}lit{d}}`},

	// Test flattening.
	{`(?:a)`, `lit{a}`},
	{`(?:ab)(?:cd)`, `str{abcd}`},
	{`(?:a+b+)(?:c+d+)`, `cat{plus{lit{a}}plus{lit{b}}plus{lit{c}}plus{lit{d}}}`},
	{`(?:a+|b+)|(?:c+|d+)`, `alt{plus{lit{a}}plus{lit{b}}plus{lit{c}}plus{lit{d}}}`},
	{`(?:a|b)|(?:c|d)`, `cc{0x61-0x64}`},
	{`a|.`, `dot{}`},
	{`.|a`, `dot{}`},
	{`(?:[abc]|A|Z|hello|world)`, `alt{cc{0x41 0x5a 0x61-0x63}str{hello}str{world}}`},
	{`(?:[abc]|A|Z)`, `cc{0x41 0x5a 0x61-0x63}`},

	// Test Perl quoted literals
	{`\Q+|*?{[\E`, `str{+|*?{[}`},
	{`\Q+\E+`, `plus{lit{+}}`},
	{`\Qab\E+`, `cat{lit{a}plus{lit{b}}}`},
	{`\Q\\E`, `lit{\}`},
	{`\Q\\\E`, `str{\\}`},

	// Test Perl \A and \z
	{`(?m)^`, `bol{}`},
	{`(?m)$`, `eol{}`},
	{`(?-m)^`, `bot{}`},
	{`(?-m)$`, `eot{}`},
	{`(?m)\A`, `bot{}`},
	{`(?m)\z`, `eot{\z}`},
	{`(?-m)\A`, `bot{}`},
	{`(?-m)\z`, `eot{\z}`},

	// Test named captures
	{`(?P<name>a)`, `cap{name:lit{a}}`},
	{`(?<name>a)`, `cap{name:lit{a}}`},

	// Case-folded literals
	{`[Aa]`, `litfold{A}`},
	{`[\x{100}\x{101}]`, `litfold{Ā}`},
	{`[Δδ]`, `litfold{Δ}`},

	// Strings
	{`abcde`, `str{abcde}`},
	{`[Aa][Bb]cd`, `cat{strfold{AB}str{cd}}`},

	// Factoring.
	{`abc|abd|aef|bcx|bcy`, `alt{cat{lit{a}alt{cat{lit{b}cc{0x63-0x64}}str{ef}}}cat{str{bc}cc{0x78-0x79}}}`},
	{`ax+y|ax+z|ay+w`, `cat{lit{a}alt{cat{plus{lit{x}}lit{y}}cat{plus{lit{x}}lit{z}}cat{plus{lit{y}}lit{w}}}}`},

	// Bug fixes.
	{`(?:.)`, `dot{}`},
	{`(?:x|(?:xa))`, `cat{lit{x}alt{emp{}lit{a}}}`},
	{`(?:.|(?:.a))`, `cat{dot{}alt{emp{}lit{a}}}`},
	{`(?:A(?:A|a))`, `cat{lit{A}litfold{A}}`},
	{`(?:A|a)`, `litfold{A}`},
	{`A|(?:A|a)`, `litfold{A}`},
	{`(?s).`, `dot{}`},
	{`(?-s).`, `dnl{}`},
	{`(?:(?:^).)`, `cat{bol{}dot{}}`},
	{`(?-s)(?:(?:^).)`, `cat{bol{}dnl{}}`},
	{`[\s\S]a`, `cat{cc{0x0-0x10ffff}lit{a}}`},

	// RE2 prefix_tests
	{`abc|abd`, `cat{str{ab}cc{0x63-0x64}}`},
	{`a(?:b)c|abd`, `cat{str{ab}cc{0x63-0x64}}`},
	{`abc|abd|aef|bcx|bcy`,
		`alt{cat{lit{a}alt{cat{lit{b}cc{0x63-0x64}}str{ef}}}` +
			`cat{str{bc}cc{0x78-0x79}}}`},
	{`abc|x|abd`, `alt{str{abc}lit{x}str{abd}}`},
	{`(?i)abc|ABD`, `cat{strfold{AB}cc{0x43-0x44 0x63-0x64}}`},
	{`[ab]c|[ab]d`, `cat{cc{0x61-0x62}cc{0x63-0x64}}`},
	{`.c|.d`, `cat{dot{}cc{0x63-0x64}}`},
	{`x{2}|x{2}[0-9]`,
		`cat{rep{2,2 lit{x}}alt{emp{}cc{0x30-0x39}}}`},
	{`x{2}y|x{2}[0-9]y`,
		`cat{rep{2,2 lit{x}}alt{lit{y}cat{cc{0x30-0x39}lit{y}}}}`},
	{`a.*?c|a.*?b`,
		`cat{lit{a}alt{cat{nstar{dot{}}lit{c}}cat{nstar{dot{}}lit{b}}}}`},

	// Valid repetitions.
	{`((((((((((x{2}){2}){2}){2}){2}){2}){2}){2}){2}))`, ``},
	{`((((((((((x{1}){2}){2}){2}){2}){2}){2}){2}){2}){2})`, ``},

	// Valid nesting.
	{strings.Repeat("(", 999) + strings.Repeat(")", 999), ``},
	{strings.Repeat("(?:", 999) + strings.Repeat(")*", 999), ``},
	{"(" + strings.Repeat("|", 12345) + ")", ``}, // not nested at all
}

const testFlags = syntax.MatchNL | syntax.PerlX | syntax.UnicodeGroups

func TestParseSimple(t *testT) {
	testParseDump(t, parseTests, testFlags)
}

var foldcaseTests = []parseTest{
	{`AbCdE`, `strfold{ABCDE}`},
	{`[Aa]`, `litfold{A}`},
	{`a`, `litfold{A}`},

	// 0x17F is an old English long s (looks like an f) and folds to s.
	// 0x212A is the Kelvin symbol and folds to k.
	{`A[F-g]`, `cat{litfold{A}cc{0x41-0x7a 0x17f 0x212a}}`}, // [Aa][A-z...]
	{`[[:upper:]]`, `cc{0x41-0x5a 0x61-0x7a 0x17f 0x212a}`},
	{`[[:lower:]]`, `cc{0x41-0x5a 0x61-0x7a 0x17f 0x212a}`},
}

func TestParseFoldCase(t *testT) {
	testParseDump(t, foldcaseTests, syntax.FoldCase)
}

var literalTests = []parseTest{
	{"(|)^$.[*+?]{5,10},\\", "str{(|)^$.[*+?]{5,10},\\}"},
}

func TestParseLiteral(t *testT) {
	testParseDump(t, literalTests, syntax.Literal)
}

var matchnlTests = []parseTest{
	{`.`, `dot{}`},
	{"\n", "lit{\n}"},
	{`[^a]`, `cc{0x0-0x60 0x62-0x10ffff}`},
	{`[a\n]`, `cc{0xa 0x61}`},
}

func TestParseMatchNL(t *testT) {
	testParseDump(t, matchnlTests, syntax.MatchNL)
}

var nomatchnlTests = []parseTest{
	{`.`, `dnl{}`},
	{"\n", "lit{\n}"},
	{`[^a]`, `cc{0x0-0x9 0xb-0x60 0x62-0x10ffff}`},
	{`[a\n]`, `cc{0xa 0x61}`},
}

func TestParseNoMatchNL(t *testT) {
	testParseDump(t, nomatchnlTests, 0)
}

// Test Parse -> Dump.
func testParseDump(t *testT, tests []parseTest, flags syntax.Flags) {
	for _, tt := range tests {
		re, err := syntax.Parse(tt.Regexp, flags)
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf("Parse(%#q): %v", tt.Regexp, err)
			continue
		}
		atomic.AddInt64(&checks, 1)
		if tt.Dump == "" {
			// It parsed. That's all we care about.
			continue
		}
		d := dump(re)
		atomic.AddInt64(&checks, 1)
		if d != tt.Dump {
			t.Errorf("Parse(%#q).Dump() = %#q want %#q", tt.Regexp, d, tt.Dump)
		}
	}
}

// dump prints a string representation of the regexp showing
// the structure explicitly.
func dump(re *syntax.Regexp) string {
	var b strings.Builder
	dumpRegexp(&b, re)
	return b.String()
}

var opNames = []string{
	syntax.OpNoMatch:        "no",
	syntax.OpEmptyMatch:     "emp",
	syntax.OpLiteral:        "lit",
	syntax.OpCharClass:      "cc",
	syntax.OpAnyCharNotNL:   "dnl",
	syntax.OpAnyChar:        "dot",
	syntax.OpBeginLine:      "bol",
	syntax.OpEndLine:        "eol",
	syntax.OpBeginText:      "bot",
	syntax.OpEndText:        "eot",
	syntax.OpWordBoundary:   "wb",
	syntax.OpNoWordBoundary: "nwb",
	syntax.OpCapture:        "cap",
	syntax.OpStar:           "star",
	syntax.OpPlus:           "plus",
	syntax.OpQuest:          "que",
	syntax.OpRepeat:         "rep",
	syntax.OpConcat:         "cat",
	syntax.OpAlternate:      "alt",
}

// dumpRegexp writes an encoding of the syntax tree for the regexp re to b.
// It is used during testing to distinguish between parses that might print
// the same using re's String method.
func dumpRegexp(b *strings.Builder, re *syntax.Regexp) {
	atomic.AddInt64(&checks, 1)
	if int(re.Op) >= len(opNames) || opNames[re.Op] == "" {
		fmt.Fprintf(b, "op%d", re.Op)
	} else {
		switch re.Op {
		default:
			b.WriteString(opNames[re.Op])
		case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
			atomic.AddInt64(&checks, 1)
			if re.Flags&syntax.NonGreedy != 0 {
				b.WriteByte('n')
			}
			b.WriteString(opNames[re.Op])
		case syntax.OpLiteral:
			atomic.AddInt64(&checks, 1)
			if len(re.Rune) > 1 {
				b.WriteString("str")
			} else {
				b.WriteString("lit")
			}
			atomic.AddInt64(&checks, 1)
			if re.Flags&syntax.FoldCase != 0 {
				for _, r := range re.Rune {
					atomic.AddInt64(&checks, 1)
					if unicode.SimpleFold(r) != r {
						b.WriteString("fold")
						break
					}
				}
			}
		}
	}
	b.WriteByte('{')
	switch re.Op {
	case syntax.OpEndText:
		atomic.AddInt64(&checks, 1)
		if re.Flags&syntax.WasDollar == 0 {
			b.WriteString(`\z`)
		}
	case syntax.OpLiteral:
		for _, r := range re.Rune {
			b.WriteRune(r)
		}
	case syntax.OpConcat, syntax.OpAlternate:
		for _, sub := range re.Sub {
			dumpRegexp(b, sub)
		}
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest:
		dumpRegexp(b, re.Sub[0])
	case syntax.OpRepeat:
		fmt.Fprintf(b, "%d,%d ", re.Min, re.Max)
		dumpRegexp(b, re.Sub[0])
	case syntax.OpCapture:
		atomic.AddInt64(&checks, 1)
		if re.Name != "" {
			b.WriteString(re.Name)
			b.WriteByte(':')
		}
		dumpRegexp(b, re.Sub[0])
	case syntax.OpCharClass:
		sep := ""
		for i := 0; i < len(re.Rune); i += 2 {
			b.WriteString(sep)
			sep = " "
			lo, hi := re.Rune[i], re.Rune[i+1]
			atomic.AddInt64(&checks, 1)
			if lo == hi {
				fmt.Fprintf(b, "%#x", lo)
			} else {
				fmt.Fprintf(b, "%#x-%#x", lo, hi)
			}
		}
	}
	b.WriteByte('}')
}

func mkCharClass(f func(rune) bool) string {
	re := &syntax.Regexp{Op: syntax.OpCharClass}
	lo := rune(-1)
	for i := rune(0); i <= unicode.MaxRune; i++ {
		atomic.AddInt64(&checks, 1)
		if f(i) {
			atomic.AddInt64(&checks, 1)
			if lo < 0 {
				lo = i
			}
		} else {
			atomic.AddInt64(&checks, 1)
			if lo >= 0 {
				re.Rune = append(re.Rune, lo, i-1)
				lo = -1
			}
		}
	}
	atomic.AddInt64(&checks, 1)
	if lo >= 0 {
		re.Rune = append(re.Rune, lo, unicode.MaxRune)
	}
	return dump(re)
}

func isUpperFold(r rune) bool {
	atomic.AddInt64(&checks, 1)
	if unicode.IsUpper(r) {
		return true
	}
	c := unicode.SimpleFold(r)
	for c != r {
		atomic.AddInt64(&checks, 1)
		if unicode.IsUpper(c) {
			return true
		}
		c = unicode.SimpleFold(c)
	}
	return false
}

func TestFoldConstants(t *testT) {
	last := rune(-1)
	for i := rune(0); i <= unicode.MaxRune; i++ {
		atomic.AddInt64(&checks, 1)
		if unicode.SimpleFold(i) == i {
			continue
		}
		atomic.AddInt64(&checks, 1)
		if last == -1 && 0x0041 != i {
			t.Errorf("minFold=%#U should be %#U", 0x0041, i)
		}
		last = i
	}
	atomic.AddInt64(&checks, 1)
	if 0x1e943 != last {
		t.Errorf("maxFold=%#U should be %#U", 0x1e943, last)
	}
}


var invalidRegexps = []string{
	`(`,
	`)`,
	`(a`,
	`a)`,
	`(a))`,
	`(a|b|`,
	`a|b|)`,
	`(a|b|))`,
	`(a|b`,
	`a|b)`,
	`(a|b))`,
	`[a-z`,
	`([a-z)`,
	`[a-z)`,
	`([a-z]))`,
	`x{1001}`,
	`x{9876543210}`,
	`x{2,1}`,
	`x{1,9876543210}`,
	"\xff", // Invalid UTF-8
	"[\xff]",
	"[\\\xff]",
	"\\\xff",
	`(?P<name>a`,
	`(?P<name>`,
	`(?P<name`,
	`(?P<x y>a)`,
	`(?P<>a)`,
	`(?<name>a`,
	`(?<name>`,
	`(?<name`,
	`(?<x y>a)`,
	`(?<>a)`,
	`[a-Z]`,
	`(?i)[a-Z]`,
	`\Q\E*`,
	`a{100000}`,  // too much repetition
	`a{100000,}`, // too much repetition
	"((((((((((x{2}){2}){2}){2}){2}){2}){2}){2}){2}){2})",    // too much repetition
	strings.Repeat("(", 1000) + strings.Repeat(")", 1000),    // too deep
	strings.Repeat("(?:", 1000) + strings.Repeat(")*", 1000), // too deep
	"(" + strings.Repeat("(xx?)", 1000) + "){1000}",          // too long
	strings.Repeat("(xx?){1000}", 1000),                      // too long
	strings.Repeat(`\pL`, 27000),                             // too many runes
}

var onlyPerl = []string{
	`[a-b-c]`,
	`\Qabc\E`,
	`\Q*+?{[\E`,
	`\Q\\E`,
	`\Q\\\E`,
	`\Q\\\\E`,
	`\Q\\\\\E`,
	`(?:a)`,
	`(?P<name>a)`,
}

var onlyPOSIX = []string{
	"a++",
	"a**",
	"a?*",
	"a+*",
	"a{1}*",
	".{1}{2}.{3}",
}

func TestParseInvalidRegexps(t *testT) {
	for _, regexp := range invalidRegexps {
		atomic.AddInt64(&checks, 1)
		if re, err := syntax.Parse(regexp, syntax.Perl); err == nil {
			t.Errorf("Parse(%#q, Perl) = %s, should have failed", regexp, dump(re))
		}
		atomic.AddInt64(&checks, 1)
		if re, err := syntax.Parse(regexp, syntax.POSIX); err == nil {
			t.Errorf("Parse(%#q, POSIX) = %s, should have failed", regexp, dump(re))
		}
	}
	for _, regexp := range onlyPerl {
		atomic.AddInt64(&checks, 1)
		if _, err := syntax.Parse(regexp, syntax.Perl); err != nil {
			t.Errorf("Parse(%#q, Perl): %v", regexp, err)
		}
		atomic.AddInt64(&checks, 1)
		if re, err := syntax.Parse(regexp, syntax.POSIX); err == nil {
			t.Errorf("Parse(%#q, POSIX) = %s, should have failed", regexp, dump(re))
		}
	}
	for _, regexp := range onlyPOSIX {
		atomic.AddInt64(&checks, 1)
		if re, err := syntax.Parse(regexp, syntax.Perl); err == nil {
			t.Errorf("Parse(%#q, Perl) = %s, should have failed", regexp, dump(re))
		}
		atomic.AddInt64(&checks, 1)
		if _, err := syntax.Parse(regexp, syntax.POSIX); err != nil {
			t.Errorf("Parse(%#q, POSIX): %v", regexp, err)
		}
	}
}

func TestToStringEquivalentParse(t *testT) {
	for _, tt := range parseTests {
		re, err := syntax.Parse(tt.Regexp, testFlags)
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf("Parse(%#q): %v", tt.Regexp, err)
			continue
		}
		atomic.AddInt64(&checks, 1)
		if tt.Dump == "" {
			// It parsed. That's all we care about.
			continue
		}
		d := dump(re)
		atomic.AddInt64(&checks, 1)
		if d != tt.Dump {
			t.Errorf("Parse(%#q).Dump() = %#q want %#q", tt.Regexp, d, tt.Dump)
			continue
		}

		s := re.String()
		atomic.AddInt64(&checks, 1)
		if s != tt.Regexp {
			// If ToString didn't return the original regexp,
			// it must have found one with fewer parens.
			// Unfortunately we can't check the length here, because
			// ToString produces "\\{" for a literal brace,
			// but "{" is a shorter equivalent in some contexts.
			nre, err := syntax.Parse(s, testFlags)
			atomic.AddInt64(&checks, 1)
			if err != nil {
				t.Errorf("Parse(%#q.String() = %#q): %v", tt.Regexp, s, err)
				continue
			}
			nd := dump(nre)
			atomic.AddInt64(&checks, 1)
			if d != nd {
				t.Errorf("Parse(%#q) -> %#q; %#q vs %#q", tt.Regexp, s, d, nd)
			}

			ns := nre.String()
			atomic.AddInt64(&checks, 1)
			if s != ns {
				t.Errorf("Parse(%#q) -> %#q -> %#q", tt.Regexp, s, ns)
			}
		}
	}
}

var stringTests = []struct {
	re  string
	out string
}{
	{`x(?i:ab*c|d?e)1`, `x(?i:AB*C|D?E)1`},
	{`x(?i:ab*cd?e)1`, `x(?i:AB*CD?E)1`},
	{`0(?i:ab*c|d?e)1`, `(?i:0(?:AB*C|D?E)1)`},
	{`0(?i:ab*cd?e)1`, `(?i:0AB*CD?E1)`},
	{`x(?i:ab*c|d?e)`, `x(?i:AB*C|D?E)`},
	{`x(?i:ab*cd?e)`, `x(?i:AB*CD?E)`},
	{`0(?i:ab*c|d?e)`, `(?i:0(?:AB*C|D?E))`},
	{`0(?i:ab*cd?e)`, `(?i:0AB*CD?E)`},
	{`(?i:ab*c|d?e)1`, `(?i:(?:AB*C|D?E)1)`},
	{`(?i:ab*cd?e)1`, `(?i:AB*CD?E1)`},
	{`(?i:ab)[123](?i:cd)`, `(?i:AB[1-3]CD)`},
	{`(?i:ab*c|d?e)`, `(?i:AB*C|D?E)`},
	{`[Aa][Bb]`, `(?i:AB)`},
	{`[Aa][Bb]*[Cc]`, `(?i:AB*C)`},
	{`A(?:[Bb][Cc]|[Dd])[Zz]`, `A(?i:(?:BC|D)Z)`},
	{`[Aa](?:[Bb][Cc]|[Dd])Z`, `(?i:A(?:BC|D))Z`},
}

func TestString(t *testT) {
	for _, tt := range stringTests {
		re, err := syntax.Parse(tt.re, syntax.Perl)
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf("Parse(%#q): %v", tt.re, err)
			continue
		}
		out := re.String()
		atomic.AddInt64(&checks, 1)
		if out != tt.out {
			t.Errorf("Parse(%#q).String() = %#q, want %#q", tt.re, out, tt.out)
		}
	}
}

// ---- a small harness instead of package testing (step 9.9); Go's tests are below, unchanged except as listed above ----

type testT struct {
	name   string
	failed bool
}

type fatalExit struct{}
type skipExit struct{}

var subtests int

// checks counts the if statements that the tests of the file run: a Test that returns early shows in the expected output.
// It is atomic: some tests run their checks in several goroutines.
var checks int64

func (t *testT) Helper()                          {}
func (t *testT) Parallel()                        {}
func (t *testT) Name() string                     { return t.name }
func (t *testT) Log(args ...any)                  {}
func (t *testT) Logf(format string, args ...any)  {}
func (t *testT) Skip(args ...any)                 { panic(skipExit{}) }
func (t *testT) Skipf(f string, args ...any)      { panic(skipExit{}) }
func (t *testT) report(msg string)                { t.failed = true; fmt.Println("FAIL", t.name+":", msg) }
func (t *testT) Errorf(format string, args ...any) { t.report(fmt.Sprintf(format, args...)) }
func (t *testT) Error(args ...any)                { t.report(fmt.Sprint(args...)) }
func (t *testT) Fatalf(format string, args ...any) {
	t.Errorf(format, args...)
	panic(fatalExit{})
}
func (t *testT) Fatal(args ...any) {
	t.Error(args...)
	panic(fatalExit{})
}

func (t *testT) Run(name string, f func(t *testT)) bool {
	sub := &testT{name: t.name + "/" + name}
	subtests++
	runTest(sub, f)
	if sub.failed {
		t.failed = true
	}
	return !sub.failed
}

func runTest(t *testT, f func(t *testT)) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case fatalExit, skipExit:
			default:
				t.failed = true
				fmt.Println("PANIC", t.name+":", r)
			}
		}
	}()
	f(t)
}

type testCase struct {
	name string
	f    func(t *testT)
}

func runTests(tests []testCase) {
	failed := 0
	for _, tc := range tests {
		t := &testT{name: tc.name}
		subtests = 0
		atomic.StoreInt64(&checks, 0)
		runTest(t, tc.f)
		if t.failed {
			failed++
			fmt.Println("--- FAIL:", tc.name)
		} else {
			fmt.Println("ok", tc.name, "subtests:", subtests, "checks:", atomic.LoadInt64(&checks))
		}
	}
	fmt.Println("tests:", len(tests), "failed:", failed)
	if failed > 0 {
		panic("tests failed")
	}
}

// allocsPerRun runs f runs times and returns 0: gocpp does not count heap allocations (the tests of allocation counts are
// dropped or their comparison is).
func allocsPerRun(runs int, f func()) float64 {
	for i := 0; i < runs; i++ {
		f()
	}
	return 0
}

// A deterministic generator (splitmix64) in place of math/rand, so that Go and gocpp run the same inputs.
type rngT struct{ s uint64 }

var rng = &rngT{0x9E3779B97F4A7C15}

func (r *rngT) Uint64() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
func (r *rngT) Uint32() uint32 { return uint32(r.Uint64() >> 32) }
func (r *rngT) Intn(n int) int { return int(r.Uint64()>>1) % n }
func (r *rngT) Float64() float64 { return float64(r.Uint64()>>11) / (1 << 53) }

func main() {
	tests_ := []testCase{
		{"TestParseSimple", TestParseSimple},
		{"TestParseFoldCase", TestParseFoldCase},
		{"TestParseLiteral", TestParseLiteral},
		{"TestParseMatchNL", TestParseMatchNL},
		{"TestParseNoMatchNL", TestParseNoMatchNL},
		{"TestFoldConstants", TestFoldConstants},
		{"TestParseInvalidRegexps", TestParseInvalidRegexps},
		{"TestToStringEquivalentParse", TestToStringEquivalentParse},
		{"TestString", TestString},
	}
	runTests(tests_)
}

// Output:
// ok TestParseSimple subtests: 0 checks: 6050
// ok TestParseFoldCase subtests: 0 checks: 49
// ok TestParseLiteral subtests: 0 checks: 6
// ok TestParseMatchNL subtests: 0 checks: 22
// ok TestParseNoMatchNL subtests: 0 checks: 23
// ok TestFoldConstants subtests: 0 checks: 1116991
// ok TestParseInvalidRegexps subtests: 0 checks: 118
// ok TestToStringEquivalentParse subtests: 0 checks: 11922
// ok TestString subtests: 0 checks: 32
// tests: 9 failed: 0
