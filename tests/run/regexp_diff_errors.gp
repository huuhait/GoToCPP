// phase: 09
// Differential test of the errors of package regexp and regexp/syntax: about 150 invalid and borderline patterns print their error text
// (and the syntax.ErrorCode and the Expr of the *syntax.Error), the limits (nesting depth 1000, repeat counts up to 1000, the size of the
// program, very long patterns and classes) are tried at and over the boundary, MustCompile panics with Go's text, and 4000 random
// mutations of patterns (a character deleted, inserted or replaced) are hashed with their error text. The expected output comes from real
// Go 1.25 (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"strings"
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

var curated = []string{
	``, `a`, `(`, `)`, `(a`, `a)`, `((a)`, `(a))`, `[`, `[a`, `[a-`, `[a-]`, `[]`, `[]a]`, `[^]`, `[^]a]`, `[z-a]`, `[a-a]`, `[\d-z]`, `[a-\d]`, `[[:alpha:]`, `[[:alpha:]-z]`,
	`[[:foo:]]`, `[[:^alpha:]]`, `[[:alpha:`, `[[:`, `[[.a.]]`, `[[=a=]]`, `\`, `a\`, `\q`, `\8`, `\9`, `\1`, `\0`, `\07`, `\08`, `\08a`, `\x`, `\x1`, `\x{`, `\x{1`, `\x{110000}`, `\x{10FFFF}`, `\xZZ`,
	`\x{}`, `\pN`, `\p`, `\p{`, `\p{N`, `\p{Foo}`, `\pQ`, `\p{^Greek}`, `\P{^Greek}`, `\p{Latin}`, `\pZ`, `\p{Zs}`, `\PN`, `\p{IsGreek}`, `\p{greek}`, `\Q`, `\Qa`, `\Qa\E`, `\E`, `\Qa\Eb\E`, `\Q\E*`,
	`*`, `+`, `?`, `a**`, `a++`, `a??`, `a*?`, `a*+`, `a+*`, `a?*`, `a{`, `a{1`, `a{1,`, `a{1,2`, `a{,2}`, `a{1,2}`, `a{2,1}`, `a{1001}`, `a{1000}`, `a{0,1001}`, `a{1,1001}`, `a{1000,1000}`,
	`a{1000}{2}`, `(a{1000}){2}`, `(a{500}){2}`, `a{1}{2}{3}`, `a{2}*`, `a{2}+`, `a{2}?`, `a{2}??`, `a**b`, `^*`, `$*`, `\b*`, `(?:)*`, `()*`, `(|)*`, `(|a)+`, `(a|)+`, `(a*)*`, `(a*)+`, `(a+)*`, `(a?)*`,
	`(?`, `(?)`, `(?a)`, `(?i`, `(?i)`, `(?i)a`, `(?i:a)`, `(?i:a`, `(?i-)`, `(?-i)`, `(?--i)`, `(?i-i)`, `(?-)`, `(?^)`, `(?^i)`, `(?z)`, `(?=a)`, `(?!a)`, `(?<=a)`, `(?<!a)`, `(?>a)`, `(?|a)`, `(?#c)`, `(?#`,
	`(?P<n>a)`, `(?P<n>a)(?P<n>b)`, `(?P<n>a`, `(?P<>a)`, `(?P<1>a)`, `(?P<n!>a)`, `(?P<n-m>a)`, `(?P=n)`, `(?P>a)`, `(?P`, `(?P<`, `(?P<n`, `(?<n>a)`, `(?<n>a)(?<n>b)`, `(?<>a)`, `(?<`, `(?<n`,
	`(?<é>a)`, `(?P<é>a)`, `(?P<_a1>a)`, `(?P<a b>a)`, `(?U)a*`, `(?U)a*?`, `(?s).`, `(?m)^$`, `(?imsU)`, `(?imsU-imsU)`, `(?i)(?-i)a`, `(?s:.)(?-s:.)`, `x*`, `x*y`, `\C`, `\C+`, `\z`, `\Z`, `\A`, `\a`, `\f`, `\v`,
	`\cA`, `\e`, `\h`, `\R`, `\X`, `\G`, `\k<n>`, `\g{1}`, `\<`, `\>`, `\_`, `\-`, `\é`, `[\é]`, `[\_]`, `[\Q]`, `[\b]`, `[\B]`, `[\A]`, `[\z]`, `[\p{Greek}-z]`, `[a-\p{Greek}]`, "\xff", "a\xffb", "[\xff]", "[a-\xff]",
	"(?P<\xff>a)", "\xc3", "\xe4\xb8", "[^\xff]", "a|b|", "|", "||", "|a", "a||b", "()", "(())", "(|)", "(?:|)", "[\\x00-\\x{10FFFF}]", "[^\\x00-\\x{10FFFF}]", "[^\\x00-\\x{10FFFF}]+", "[\\x{10FFFF}-\\x{10FFFF}]", `.*?`, `.+?`,
}

// show quotes the text when it is not valid UTF-8 (an error text repeats the pattern as it is).
func show(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return fmt.Sprintf("%q", s)
}

func try(pat string) string {
	re, err := regexp.Compile(pat)
	if err != nil {
		s := err.Error()
		if se, ok := err.(*syntax.Error); ok {
			return fmt.Sprintf("error %s | code=%q expr=%q", show(s), se.Code, se.Expr)
		}
		return "error " + show(s)
	}
	return fmt.Sprintf("ok %q n=%d", re.String(), re.NumSubexp())
}

func recoverText(f func()) (s string) {
	defer func() { s = fmt.Sprint(recover()) }()
	f()
	return "no panic"
}

type hasher struct{ h uint64 }

func (x *hasher) str(s string) {
	for i := 0; i < len(s); i++ {
		x.h ^= uint64(s[i])
		x.h *= 1099511628211
	}
	x.h ^= 0xff
	x.h *= 1099511628211
}

func main() {
	for _, p := range curated {
		fmt.Printf("%q: %s\n", p, try(p))
	}

	// Limits.
	fmt.Println("== limits")
	for _, n := range []int{1, 2, 100, 999, 1000, 1001, 1002, 2000} {
		p := strings.Repeat("(", n) + "a" + strings.Repeat(")", n)
		fmt.Println("nest", n, shorten(try(p)))
		p = strings.Repeat("(?:", n) + "a" + strings.Repeat(")", n)
		fmt.Println("nest nc", n, shorten(try(p)))
		p = strings.Repeat("(a|", n) + "a" + strings.Repeat(")", n)
		fmt.Println("nest alt", n, shorten(try(p)))
		p = "a" + strings.Repeat("*", n)
		fmt.Println("stars", n, shorten(try(p)))
		p = "a" + strings.Repeat("{2}", n)
		fmt.Println("repeat of repeat", n, shorten(try(p)))
		p = strings.Repeat("a?", n) + strings.Repeat("a", n)
		fmt.Println("a?^n a^n", n, shorten(try(p)))
	}
	for _, p := range []string{`(a{1000}){1000}`, `((a{100}){100}){100}`, `(a{10}){10}{10}{10}`, `((((((a{10}){10}){10}){10}){10}){10})`, `(abc){1000}`, `[a-z]{1000}`, `.{1000}`, `(.{1000}){3}`, `(\pL{500}){2}`, `(?i)(\pL{100}){10}`,
		`((a{30}){30}){30}`, `((a{30}){30}){30}{3}`, `(((a{100}){100}){100}){100}`, `a{1000}b{1000}c{1000}`, `(?s:.{1000}){100}`} {
		fmt.Println("size", p, shorten(try(p)))
	}
	for _, n := range []int{1000, 10000, 100000} {
		fmt.Println("long literal", n, shorten(try(strings.Repeat("ab", n))))
		fmt.Println("long alternation", n/10, shorten(try(strings.Join(strings.Split(strings.Repeat("x", n/10), ""), "|"))))
		fmt.Println("long class", n/10, shorten(try("["+strings.Repeat("a-cx", n/10)+"]")))
		fmt.Println("long cats", n/10, shorten(try(strings.Repeat("(a)", n/10))))
	}

	// Panics of MustCompile and friends.
	fmt.Println("== panics")
	for _, p := range []string{`a**`, `(`, "\xff", `\8`, `a{1001}`, `[z-a]`} {
		fmt.Println(show(recoverText(func() { regexp.MustCompile(p) })))
		fmt.Println(show(recoverText(func() { regexp.MustCompilePOSIX(p) })))
	}
	fmt.Println(regexp.MatchString(`(`, "a"))
	fmt.Println(regexp.Match(`a{1001}`, []byte("a")))
	fmt.Println(regexp.MatchReader(`[z-a]`, strings.NewReader("a")))
	_, err := regexp.CompilePOSIX(`\pL`)
	fmt.Println(err)
	_, err = regexp.CompilePOSIX(`(?i)a`)
	fmt.Println(err)
	_, err = regexp.CompilePOSIX(`a*?`)
	fmt.Println(err)
	_, err = regexp.CompilePOSIX(`\d`)
	fmt.Println(err)
	_, err = regexp.CompilePOSIX(`[[:alpha:]]\b`)
	fmt.Println(err)
	for _, e := range []syntax.ErrorCode{syntax.ErrInternalError, syntax.ErrInvalidCharClass, syntax.ErrInvalidCharRange, syntax.ErrInvalidEscape, syntax.ErrInvalidNamedCapture, syntax.ErrInvalidPerlOp, syntax.ErrInvalidRepeatOp, syntax.ErrInvalidRepeatSize, syntax.ErrInvalidUTF8, syntax.ErrMissingBracket, syntax.ErrMissingParen, syntax.ErrMissingRepeatArgument, syntax.ErrTrailingBackslash, syntax.ErrUnexpectedParen, syntax.ErrNestingDepth, syntax.ErrLarge} {
		fmt.Println(e.String(), (&syntax.Error{Code: e, Expr: "x"}).Error())
	}

	// Random mutations.
	fmt.Println("== mutations")
	r := &rngT{4242}
	var h hasher
	h.h = 14695981039346656037
	alphabet := []string{"a", "(", ")", "[", "]", "{", "}", "*", "+", "?", "|", "^", "$", "\\", ".", "-", ":", "<", ">", "P", "i", "s", "1", "2", ",", "é", "\xff", "=", "!", "#", "\\p", "\\d", "[:", ":]", "(?", "(?P<n>", "{1,2}", "\\x{", "\\Q", "\\E"}
	nerr := 0
	for it := 0; it < 4000; it++ {
		base := curated[r.intn(len(curated))]
		if r.intn(3) == 0 {
			base += curated[r.intn(len(curated))]
		}
		b := []byte(base)
		for k := 1 + r.intn(3); k > 0; k-- {
			pos := 0
			if len(b) > 0 {
				pos = r.intn(len(b) + 1)
			}
			ins := alphabet[r.intn(len(alphabet))]
			switch r.intn(3) {
			case 0:
				if pos < len(b) {
					b = append(b[:pos:pos], b[pos+1:]...)
				}
			case 1:
				b = append(b[:pos:pos], append([]byte(ins), b[pos:]...)...)
			default:
				if pos < len(b) {
					b = append(b[:pos:pos], append([]byte(ins), b[pos+1:]...)...)
				}
			}
		}
		res := try(string(b))
		if strings.HasPrefix(res, "error") {
			nerr++
		}
		h.str(string(b))
		h.str(res)
	}
	fmt.Println("errors", nerr, "of 4000")
	fmt.Printf("mutations %016x\n", h.h)
}

func shorten(s string) string {
	if len(s) > 150 {
		return s[:120] + "..." + s[len(s)-20:]
	}
	return s
}

// Output:
// "": ok "" n=0
// "a": ok "a" n=0
// "(": error error parsing regexp: missing closing ): `(` | code="missing closing )" expr="("
// ")": error error parsing regexp: unexpected ): `)` | code="unexpected )" expr=")"
// "(a": error error parsing regexp: missing closing ): `(a` | code="missing closing )" expr="(a"
// "a)": error error parsing regexp: unexpected ): `a)` | code="unexpected )" expr="a)"
// "((a)": error error parsing regexp: missing closing ): `((a)` | code="missing closing )" expr="((a)"
// "(a))": error error parsing regexp: unexpected ): `(a))` | code="unexpected )" expr="(a))"
// "[": error error parsing regexp: missing closing ]: `[` | code="missing closing ]" expr="["
// "[a": error error parsing regexp: missing closing ]: `[a` | code="missing closing ]" expr="[a"
// "[a-": error error parsing regexp: missing closing ]: `[a-` | code="missing closing ]" expr="[a-"
// "[a-]": ok "[a-]" n=0
// "[]": error error parsing regexp: missing closing ]: `[]` | code="missing closing ]" expr="[]"
// "[]a]": ok "[]a]" n=0
// "[^]": error error parsing regexp: missing closing ]: `[^]` | code="missing closing ]" expr="[^]"
// "[^]a]": ok "[^]a]" n=0
// "[z-a]": error error parsing regexp: invalid character class range: `z-a` | code="invalid character class range" expr="z-a"
// "[a-a]": ok "[a-a]" n=0
// "[\\d-z]": ok "[\\d-z]" n=0
// "[a-\\d]": error error parsing regexp: invalid escape sequence: `\d` | code="invalid escape sequence" expr="\\d"
// "[[:alpha:]": error error parsing regexp: missing closing ]: `[[:alpha:]` | code="missing closing ]" expr="[[:alpha:]"
// "[[:alpha:]-z]": ok "[[:alpha:]-z]" n=0
// "[[:foo:]]": error error parsing regexp: invalid character class range: `[:foo:]` | code="invalid character class range" expr="[:foo:]"
// "[[:^alpha:]]": ok "[[:^alpha:]]" n=0
// "[[:alpha:": error error parsing regexp: missing closing ]: `[[:alpha:` | code="missing closing ]" expr="[[:alpha:"
// "[[:": error error parsing regexp: missing closing ]: `[[:` | code="missing closing ]" expr="[[:"
// "[[.a.]]": ok "[[.a.]]" n=0
// "[[=a=]]": ok "[[=a=]]" n=0
// "\\": error error parsing regexp: trailing backslash at end of expression: `` | code="trailing backslash at end of expression" expr=""
// "a\\": error error parsing regexp: trailing backslash at end of expression: `` | code="trailing backslash at end of expression" expr=""
// "\\q": error error parsing regexp: invalid escape sequence: `\q` | code="invalid escape sequence" expr="\\q"
// "\\8": error error parsing regexp: invalid escape sequence: `\8` | code="invalid escape sequence" expr="\\8"
// "\\9": error error parsing regexp: invalid escape sequence: `\9` | code="invalid escape sequence" expr="\\9"
// "\\1": error error parsing regexp: invalid escape sequence: `\1` | code="invalid escape sequence" expr="\\1"
// "\\0": ok "\\0" n=0
// "\\07": ok "\\07" n=0
// "\\08": ok "\\08" n=0
// "\\08a": ok "\\08a" n=0
// "\\x": error error parsing regexp: invalid escape sequence: `\x` | code="invalid escape sequence" expr="\\x"
// "\\x1": error error parsing regexp: invalid escape sequence: `\x1` | code="invalid escape sequence" expr="\\x1"
// "\\x{": error error parsing regexp: invalid escape sequence: `\x{` | code="invalid escape sequence" expr="\\x{"
// "\\x{1": error error parsing regexp: invalid escape sequence: `\x{1` | code="invalid escape sequence" expr="\\x{1"
// "\\x{110000}": error error parsing regexp: invalid escape sequence: `\x{110000` | code="invalid escape sequence" expr="\\x{110000"
// "\\x{10FFFF}": ok "\\x{10FFFF}" n=0
// "\\xZZ": error error parsing regexp: invalid escape sequence: `\xZZ` | code="invalid escape sequence" expr="\\xZZ"
// "\\x{}": error error parsing regexp: invalid escape sequence: `\x{}` | code="invalid escape sequence" expr="\\x{}"
// "\\pN": ok "\\pN" n=0
// "\\p": error error parsing regexp: invalid character class range: `\p` | code="invalid character class range" expr="\\p"
// "\\p{": error error parsing regexp: invalid character class range: `\p{` | code="invalid character class range" expr="\\p{"
// "\\p{N": error error parsing regexp: invalid character class range: `\p{N` | code="invalid character class range" expr="\\p{N"
// "\\p{Foo}": error error parsing regexp: invalid character class range: `\p{Foo}` | code="invalid character class range" expr="\\p{Foo}"
// "\\pQ": error error parsing regexp: invalid character class range: `\pQ` | code="invalid character class range" expr="\\pQ"
// "\\p{^Greek}": ok "\\p{^Greek}" n=0
// "\\P{^Greek}": ok "\\P{^Greek}" n=0
// "\\p{Latin}": ok "\\p{Latin}" n=0
// "\\pZ": ok "\\pZ" n=0
// "\\p{Zs}": ok "\\p{Zs}" n=0
// "\\PN": ok "\\PN" n=0
// "\\p{IsGreek}": error error parsing regexp: invalid character class range: `\p{IsGreek}` | code="invalid character class range" expr="\\p{IsGreek}"
// "\\p{greek}": ok "\\p{greek}" n=0
// "\\Q": ok "\\Q" n=0
// "\\Qa": ok "\\Qa" n=0
// "\\Qa\\E": ok "\\Qa\\E" n=0
// "\\E": error error parsing regexp: invalid escape sequence: `\E` | code="invalid escape sequence" expr="\\E"
// "\\Qa\\Eb\\E": error error parsing regexp: invalid escape sequence: `\E` | code="invalid escape sequence" expr="\\E"
// "\\Q\\E*": error error parsing regexp: missing argument to repetition operator: `*` | code="missing argument to repetition operator" expr="*"
// "*": error error parsing regexp: missing argument to repetition operator: `*` | code="missing argument to repetition operator" expr="*"
// "+": error error parsing regexp: missing argument to repetition operator: `+` | code="missing argument to repetition operator" expr="+"
// "?": error error parsing regexp: missing argument to repetition operator: `?` | code="missing argument to repetition operator" expr="?"
// "a**": error error parsing regexp: invalid nested repetition operator: `**` | code="invalid nested repetition operator" expr="**"
// "a++": error error parsing regexp: invalid nested repetition operator: `++` | code="invalid nested repetition operator" expr="++"
// "a??": ok "a??" n=0
// "a*?": ok "a*?" n=0
// "a*+": error error parsing regexp: invalid nested repetition operator: `*+` | code="invalid nested repetition operator" expr="*+"
// "a+*": error error parsing regexp: invalid nested repetition operator: `+*` | code="invalid nested repetition operator" expr="+*"
// "a?*": error error parsing regexp: invalid nested repetition operator: `?*` | code="invalid nested repetition operator" expr="?*"
// "a{": ok "a{" n=0
// "a{1": ok "a{1" n=0
// "a{1,": ok "a{1," n=0
// "a{1,2": ok "a{1,2" n=0
// "a{,2}": ok "a{,2}" n=0
// "a{1,2}": ok "a{1,2}" n=0
// "a{2,1}": error error parsing regexp: invalid repeat count: `{2,1}` | code="invalid repeat count" expr="{2,1}"
// "a{1001}": error error parsing regexp: invalid repeat count: `{1001}` | code="invalid repeat count" expr="{1001}"
// "a{1000}": ok "a{1000}" n=0
// "a{0,1001}": error error parsing regexp: invalid repeat count: `{0,1001}` | code="invalid repeat count" expr="{0,1001}"
// "a{1,1001}": error error parsing regexp: invalid repeat count: `{1,1001}` | code="invalid repeat count" expr="{1,1001}"
// "a{1000,1000}": ok "a{1000,1000}" n=0
// "a{1000}{2}": error error parsing regexp: invalid nested repetition operator: `{1000}{2}` | code="invalid nested repetition operator" expr="{1000}{2}"
// "(a{1000}){2}": error error parsing regexp: invalid repeat count: `{2}` | code="invalid repeat count" expr="{2}"
// "(a{500}){2}": ok "(a{500}){2}" n=1
// "a{1}{2}{3}": error error parsing regexp: invalid nested repetition operator: `{1}{2}` | code="invalid nested repetition operator" expr="{1}{2}"
// "a{2}*": error error parsing regexp: invalid nested repetition operator: `{2}*` | code="invalid nested repetition operator" expr="{2}*"
// "a{2}+": error error parsing regexp: invalid nested repetition operator: `{2}+` | code="invalid nested repetition operator" expr="{2}+"
// "a{2}?": ok "a{2}?" n=0
// "a{2}??": error error parsing regexp: invalid nested repetition operator: `{2}??` | code="invalid nested repetition operator" expr="{2}??"
// "a**b": error error parsing regexp: invalid nested repetition operator: `**` | code="invalid nested repetition operator" expr="**"
// "^*": ok "^*" n=0
// "$*": ok "$*" n=0
// "\\b*": ok "\\b*" n=0
// "(?:)*": ok "(?:)*" n=0
// "()*": ok "()*" n=1
// "(|)*": ok "(|)*" n=1
// "(|a)+": ok "(|a)+" n=1
// "(a|)+": ok "(a|)+" n=1
// "(a*)*": ok "(a*)*" n=1
// "(a*)+": ok "(a*)+" n=1
// "(a+)*": ok "(a+)*" n=1
// "(a?)*": ok "(a?)*" n=1
// "(?": error error parsing regexp: invalid or unsupported Perl syntax: `(?` | code="invalid or unsupported Perl syntax" expr="(?"
// "(?)": ok "(?)" n=0
// "(?a)": error error parsing regexp: invalid or unsupported Perl syntax: `(?a` | code="invalid or unsupported Perl syntax" expr="(?a"
// "(?i": error error parsing regexp: invalid or unsupported Perl syntax: `(?i` | code="invalid or unsupported Perl syntax" expr="(?i"
// "(?i)": ok "(?i)" n=0
// "(?i)a": ok "(?i)a" n=0
// "(?i:a)": ok "(?i:a)" n=0
// "(?i:a": error error parsing regexp: missing closing ): `(?i:a` | code="missing closing )" expr="(?i:a"
// "(?i-)": error error parsing regexp: invalid or unsupported Perl syntax: `(?i-)` | code="invalid or unsupported Perl syntax" expr="(?i-)"
// "(?-i)": ok "(?-i)" n=0
// "(?--i)": error error parsing regexp: invalid or unsupported Perl syntax: `(?--` | code="invalid or unsupported Perl syntax" expr="(?--"
// "(?i-i)": ok "(?i-i)" n=0
// "(?-)": error error parsing regexp: invalid or unsupported Perl syntax: `(?-)` | code="invalid or unsupported Perl syntax" expr="(?-)"
// "(?^)": error error parsing regexp: invalid or unsupported Perl syntax: `(?^` | code="invalid or unsupported Perl syntax" expr="(?^"
// "(?^i)": error error parsing regexp: invalid or unsupported Perl syntax: `(?^` | code="invalid or unsupported Perl syntax" expr="(?^"
// "(?z)": error error parsing regexp: invalid or unsupported Perl syntax: `(?z` | code="invalid or unsupported Perl syntax" expr="(?z"
// "(?=a)": error error parsing regexp: invalid or unsupported Perl syntax: `(?=` | code="invalid or unsupported Perl syntax" expr="(?="
// "(?!a)": error error parsing regexp: invalid or unsupported Perl syntax: `(?!` | code="invalid or unsupported Perl syntax" expr="(?!"
// "(?<=a)": error error parsing regexp: invalid named capture: `(?<=a)` | code="invalid named capture" expr="(?<=a)"
// "(?<!a)": error error parsing regexp: invalid named capture: `(?<!a)` | code="invalid named capture" expr="(?<!a)"
// "(?>a)": error error parsing regexp: invalid or unsupported Perl syntax: `(?>` | code="invalid or unsupported Perl syntax" expr="(?>"
// "(?|a)": error error parsing regexp: invalid or unsupported Perl syntax: `(?|` | code="invalid or unsupported Perl syntax" expr="(?|"
// "(?#c)": error error parsing regexp: invalid or unsupported Perl syntax: `(?#` | code="invalid or unsupported Perl syntax" expr="(?#"
// "(?#": error error parsing regexp: invalid or unsupported Perl syntax: `(?#` | code="invalid or unsupported Perl syntax" expr="(?#"
// "(?P<n>a)": ok "(?P<n>a)" n=1
// "(?P<n>a)(?P<n>b)": ok "(?P<n>a)(?P<n>b)" n=2
// "(?P<n>a": error error parsing regexp: missing closing ): `(?P<n>a` | code="missing closing )" expr="(?P<n>a"
// "(?P<>a)": error error parsing regexp: invalid named capture: `(?P<>` | code="invalid named capture" expr="(?P<>"
// "(?P<1>a)": ok "(?P<1>a)" n=1
// "(?P<n!>a)": error error parsing regexp: invalid named capture: `(?P<n!>` | code="invalid named capture" expr="(?P<n!>"
// "(?P<n-m>a)": error error parsing regexp: invalid named capture: `(?P<n-m>` | code="invalid named capture" expr="(?P<n-m>"
// "(?P=n)": error error parsing regexp: invalid or unsupported Perl syntax: `(?P` | code="invalid or unsupported Perl syntax" expr="(?P"
// "(?P>a)": error error parsing regexp: invalid or unsupported Perl syntax: `(?P` | code="invalid or unsupported Perl syntax" expr="(?P"
// "(?P": error error parsing regexp: invalid or unsupported Perl syntax: `(?P` | code="invalid or unsupported Perl syntax" expr="(?P"
// "(?P<": error error parsing regexp: invalid or unsupported Perl syntax: `(?P` | code="invalid or unsupported Perl syntax" expr="(?P"
// "(?P<n": error error parsing regexp: invalid named capture: `(?P<n` | code="invalid named capture" expr="(?P<n"
// "(?<n>a)": ok "(?<n>a)" n=1
// "(?<n>a)(?<n>b)": ok "(?<n>a)(?<n>b)" n=2
// "(?<>a)": error error parsing regexp: invalid named capture: `(?<>` | code="invalid named capture" expr="(?<>"
// "(?<": error error parsing regexp: invalid or unsupported Perl syntax: `(?<` | code="invalid or unsupported Perl syntax" expr="(?<"
// "(?<n": error error parsing regexp: invalid named capture: `(?<n` | code="invalid named capture" expr="(?<n"
// "(?<é>a)": error error parsing regexp: invalid named capture: `(?<é>` | code="invalid named capture" expr="(?<é>"
// "(?P<é>a)": error error parsing regexp: invalid named capture: `(?P<é>` | code="invalid named capture" expr="(?P<é>"
// "(?P<_a1>a)": ok "(?P<_a1>a)" n=1
// "(?P<a b>a)": error error parsing regexp: invalid named capture: `(?P<a b>` | code="invalid named capture" expr="(?P<a b>"
// "(?U)a*": ok "(?U)a*" n=0
// "(?U)a*?": ok "(?U)a*?" n=0
// "(?s).": ok "(?s)." n=0
// "(?m)^$": ok "(?m)^$" n=0
// "(?imsU)": ok "(?imsU)" n=0
// "(?imsU-imsU)": ok "(?imsU-imsU)" n=0
// "(?i)(?-i)a": ok "(?i)(?-i)a" n=0
// "(?s:.)(?-s:.)": ok "(?s:.)(?-s:.)" n=0
// "x*": ok "x*" n=0
// "x*y": ok "x*y" n=0
// "\\C": error error parsing regexp: invalid escape sequence: `\C` | code="invalid escape sequence" expr="\\C"
// "\\C+": error error parsing regexp: invalid escape sequence: `\C` | code="invalid escape sequence" expr="\\C"
// "\\z": ok "\\z" n=0
// "\\Z": error error parsing regexp: invalid escape sequence: `\Z` | code="invalid escape sequence" expr="\\Z"
// "\\A": ok "\\A" n=0
// "\\a": ok "\\a" n=0
// "\\f": ok "\\f" n=0
// "\\v": ok "\\v" n=0
// "\\cA": error error parsing regexp: invalid escape sequence: `\c` | code="invalid escape sequence" expr="\\c"
// "\\e": error error parsing regexp: invalid escape sequence: `\e` | code="invalid escape sequence" expr="\\e"
// "\\h": error error parsing regexp: invalid escape sequence: `\h` | code="invalid escape sequence" expr="\\h"
// "\\R": error error parsing regexp: invalid escape sequence: `\R` | code="invalid escape sequence" expr="\\R"
// "\\X": error error parsing regexp: invalid escape sequence: `\X` | code="invalid escape sequence" expr="\\X"
// "\\G": error error parsing regexp: invalid escape sequence: `\G` | code="invalid escape sequence" expr="\\G"
// "\\k<n>": error error parsing regexp: invalid escape sequence: `\k` | code="invalid escape sequence" expr="\\k"
// "\\g{1}": error error parsing regexp: invalid escape sequence: `\g` | code="invalid escape sequence" expr="\\g"
// "\\<": ok "\\<" n=0
// "\\>": ok "\\>" n=0
// "\\_": ok "\\_" n=0
// "\\-": ok "\\-" n=0
// "\\é": error error parsing regexp: invalid escape sequence: `\é` | code="invalid escape sequence" expr="\\é"
// "[\\é]": error error parsing regexp: invalid escape sequence: `\é` | code="invalid escape sequence" expr="\\é"
// "[\\_]": ok "[\\_]" n=0
// "[\\Q]": error error parsing regexp: invalid escape sequence: `\Q` | code="invalid escape sequence" expr="\\Q"
// "[\\b]": error error parsing regexp: invalid escape sequence: `\b` | code="invalid escape sequence" expr="\\b"
// "[\\B]": error error parsing regexp: invalid escape sequence: `\B` | code="invalid escape sequence" expr="\\B"
// "[\\A]": error error parsing regexp: invalid escape sequence: `\A` | code="invalid escape sequence" expr="\\A"
// "[\\z]": error error parsing regexp: invalid escape sequence: `\z` | code="invalid escape sequence" expr="\\z"
// "[\\p{Greek}-z]": ok "[\\p{Greek}-z]" n=0
// "[a-\\p{Greek}]": error error parsing regexp: invalid escape sequence: `\p` | code="invalid escape sequence" expr="\\p"
// "\xff": error "error parsing regexp: invalid UTF-8: `\xff`" | code="invalid UTF-8" expr="\xff"
// "a\xffb": error "error parsing regexp: invalid UTF-8: `\xffb`" | code="invalid UTF-8" expr="\xffb"
// "[\xff]": error "error parsing regexp: invalid UTF-8: `\xff]`" | code="invalid UTF-8" expr="\xff]"
// "[a-\xff]": error "error parsing regexp: invalid UTF-8: `\xff]`" | code="invalid UTF-8" expr="\xff]"
// "(?P<\xff>a)": error "error parsing regexp: invalid UTF-8: `\xff`" | code="invalid UTF-8" expr="\xff"
// "\xc3": error "error parsing regexp: invalid UTF-8: `\xc3`" | code="invalid UTF-8" expr="\xc3"
// "\xe4\xb8": error "error parsing regexp: invalid UTF-8: `\xe4\xb8`" | code="invalid UTF-8" expr="\xe4\xb8"
// "[^\xff]": error "error parsing regexp: invalid UTF-8: `\xff]`" | code="invalid UTF-8" expr="\xff]"
// "a|b|": ok "a|b|" n=0
// "|": ok "|" n=0
// "||": ok "||" n=0
// "|a": ok "|a" n=0
// "a||b": ok "a||b" n=0
// "()": ok "()" n=1
// "(())": ok "(())" n=2
// "(|)": ok "(|)" n=1
// "(?:|)": ok "(?:|)" n=0
// "[\\x00-\\x{10FFFF}]": ok "[\\x00-\\x{10FFFF}]" n=0
// "[^\\x00-\\x{10FFFF}]": ok "[^\\x00-\\x{10FFFF}]" n=0
// "[^\\x00-\\x{10FFFF}]+": ok "[^\\x00-\\x{10FFFF}]+" n=0
// "[\\x{10FFFF}-\\x{10FFFF}]": ok "[\\x{10FFFF}-\\x{10FFFF}]" n=0
// ".*?": ok ".*?" n=0
// ".+?": ok ".+?" n=0
// == limits
// nest 1 ok "(a)" n=1
// nest nc 1 ok "(?:a)" n=0
// nest alt 1 ok "(a|a)" n=1
// stars 1 ok "a*" n=0
// repeat of repeat 1 ok "a{2}" n=0
// a?^n a^n 1 ok "a?a" n=0
// nest 2 ok "((a))" n=2
// nest nc 2 ok "(?:(?:a))" n=0
// nest alt 2 ok "(a|(a|a))" n=2
// stars 2 error error parsing regexp: invalid nested repetition operator: `**` | code="invalid nested repetition operator" expr="**"
// repeat of repeat 2 error error parsing regexp: invalid nested repetition operator: `{2}{2}` | code="invalid nested repetition operator" expr="{2}{2}"
// a?^n a^n 2 ok "a?a?aa" n=0
// nest 100 ok "((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((a)))))))))))))))...)))))))))))))" n=100
// nest nc 100 ok "(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?...)))))))))))))))" n=0
// nest alt 100 ok "(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a...)))))))))))))" n=100
// stars 100 error error parsing regexp: invalid nested repetition operator: `**` | code="invalid nested repetition operator" expr="**"
// repeat of repeat 100 error error parsing regexp: invalid nested repetition operator: `{2}{2}` | code="invalid nested repetition operator" expr="{2}{2}"
// a?^n a^n 100 ok "a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?...aaaaaaaaaaaaaaa" n=0
// nest 999 ok "((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((...)))))))))))))" n=999
// nest nc 999 ok "(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?...)))))))))))))))" n=0
// nest alt 999 error error parsing regexp: expression nests too deeply: `(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a...)))))))))))))))))))"
// stars 999 error error parsing regexp: invalid nested repetition operator: `**` | code="invalid nested repetition operator" expr="**"
// repeat of repeat 999 error error parsing regexp: invalid nested repetition operator: `{2}{2}` | code="invalid nested repetition operator" expr="{2}{2}"
// a?^n a^n 999 ok "a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?...aaaaaaaaaaaaaaa" n=0
// nest 1000 error error parsing regexp: expression nests too deeply: `((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((...)))))))))))))))))))"
// nest nc 1000 ok "(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?...)))))))))))))))" n=0
// nest alt 1000 error error parsing regexp: expression nests too deeply: `(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a...)))))))))))))))))))"
// stars 1000 error error parsing regexp: invalid nested repetition operator: `**` | code="invalid nested repetition operator" expr="**"
// repeat of repeat 1000 error error parsing regexp: invalid nested repetition operator: `{2}{2}` | code="invalid nested repetition operator" expr="{2}{2}"
// a?^n a^n 1000 ok "a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?...aaaaaaaaaaaaaaa" n=0
// nest 1001 error error parsing regexp: expression nests too deeply: `((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((...)))))))))))))))))))"
// nest nc 1001 ok "(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?...)))))))))))))))" n=0
// nest alt 1001 error error parsing regexp: expression nests too deeply: `(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a...)))))))))))))))))))"
// stars 1001 error error parsing regexp: invalid nested repetition operator: `**` | code="invalid nested repetition operator" expr="**"
// repeat of repeat 1001 error error parsing regexp: invalid nested repetition operator: `{2}{2}` | code="invalid nested repetition operator" expr="{2}{2}"
// a?^n a^n 1001 ok "a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?...aaaaaaaaaaaaaaa" n=0
// nest 1002 error error parsing regexp: expression nests too deeply: `((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((...)))))))))))))))))))"
// nest nc 1002 ok "(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?...)))))))))))))))" n=0
// nest alt 1002 error error parsing regexp: expression nests too deeply: `(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a...)))))))))))))))))))"
// stars 1002 error error parsing regexp: invalid nested repetition operator: `**` | code="invalid nested repetition operator" expr="**"
// repeat of repeat 1002 error error parsing regexp: invalid nested repetition operator: `{2}{2}` | code="invalid nested repetition operator" expr="{2}{2}"
// a?^n a^n 1002 ok "a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?...aaaaaaaaaaaaaaa" n=0
// nest 2000 error error parsing regexp: expression nests too deeply: `((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((...)))))))))))))))))))"
// nest nc 2000 ok "(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?:(?...)))))))))))))))" n=0
// nest alt 2000 error error parsing regexp: expression nests too deeply: `(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a|(a...)))))))))))))))))))"
// stars 2000 error error parsing regexp: invalid nested repetition operator: `**` | code="invalid nested repetition operator" expr="**"
// repeat of repeat 2000 error error parsing regexp: invalid nested repetition operator: `{2}{2}` | code="invalid nested repetition operator" expr="{2}{2}"
// a?^n a^n 2000 ok "a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?a?...aaaaaaaaaaaaaaa" n=0
// size (a{1000}){1000} error error parsing regexp: invalid repeat count: `{1000}` | code="invalid repeat count" expr="{1000}"
// size ((a{100}){100}){100} error error parsing regexp: invalid repeat count: `{100}` | code="invalid repeat count" expr="{100}"
// size (a{10}){10}{10}{10} error error parsing regexp: invalid nested repetition operator: `{10}{10}` | code="invalid nested repetition operator" expr="{10}{10}"
// size ((((((a{10}){10}){10}){10}){10}){10}) error error parsing regexp: invalid repeat count: `{10}` | code="invalid repeat count" expr="{10}"
// size (abc){1000} ok "(abc){1000}" n=1
// size [a-z]{1000} ok "[a-z]{1000}" n=0
// size .{1000} ok ".{1000}" n=0
// size (.{1000}){3} error error parsing regexp: invalid repeat count: `{3}` | code="invalid repeat count" expr="{3}"
// size (\pL{500}){2} ok "(\\pL{500}){2}" n=1
// size (?i)(\pL{100}){10} ok "(?i)(\\pL{100}){10}" n=1
// size ((a{30}){30}){30} error error parsing regexp: invalid repeat count: `{30}` | code="invalid repeat count" expr="{30}"
// size ((a{30}){30}){30}{3} error error parsing regexp: invalid repeat count: `{30}` | code="invalid repeat count" expr="{30}"
// size (((a{100}){100}){100}){100} error error parsing regexp: invalid repeat count: `{100}` | code="invalid repeat count" expr="{100}"
// size a{1000}b{1000}c{1000} ok "a{1000}b{1000}c{1000}" n=0
// size (?s:.{1000}){100} error error parsing regexp: invalid repeat count: `{100}` | code="invalid repeat count" expr="{100}"
// long literal 1000 ok "abababababababababababababababababababababababababababababababababababababababababababababababababababababababababab...bababababababab" n=0
// long alternation 100 ok "x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|...x|x|x|x|x|x|x|x" n=0
// long class 100 ok "[a-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-c...cxa-cxa-cxa-cx]" n=0
// long cats 100 ok "(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a...)(a)(a)(a)(a)" n=100
// long literal 10000 ok "abababababababababababababababababababababababababababababababababababababababababababababababababababababababababab...bababababababab" n=0
// long alternation 1000 ok "x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|...x|x|x|x|x|x|x|x" n=0
// long class 1000 ok "[a-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-c...cxa-cxa-cxa-cx]" n=0
// long cats 1000 ok "(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a...(a)(a)(a)(a)" n=1000
// long literal 100000 ok "abababababababababababababababababababababababababababababababababababababababababababababababababababababababababab...bababababababab" n=0
// long alternation 10000 ok "x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|x|...x|x|x|x|x|x|x|x" n=0
// long class 10000 ok "[a-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-cxa-c...cxa-cxa-cxa-cx]" n=0
// long cats 10000 ok "(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a)(a...a)(a)(a)(a)" n=10000
// == panics
// regexp: Compile(`a**`): error parsing regexp: invalid nested repetition operator: `**`
// <nil>
// regexp: Compile(`(`): error parsing regexp: missing closing ): `(`
// regexp: CompilePOSIX(`(`): error parsing regexp: missing closing ): `(`
// "regexp: Compile(\"\\xff\"): error parsing regexp: invalid UTF-8: `\xff`"
// "regexp: CompilePOSIX(\"\\xff\"): error parsing regexp: invalid UTF-8: `\xff`"
// regexp: Compile(`\8`): error parsing regexp: invalid escape sequence: `\8`
// regexp: CompilePOSIX(`\8`): error parsing regexp: invalid escape sequence: `\8`
// regexp: Compile(`a{1001}`): error parsing regexp: invalid repeat count: `{1001}`
// regexp: CompilePOSIX(`a{1001}`): error parsing regexp: invalid repeat count: `{1001}`
// regexp: Compile(`[z-a]`): error parsing regexp: invalid character class range: `z-a`
// regexp: CompilePOSIX(`[z-a]`): error parsing regexp: invalid character class range: `z-a`
// false error parsing regexp: missing closing ): `(`
// false error parsing regexp: invalid repeat count: `{1001}`
// false error parsing regexp: invalid character class range: `z-a`
// error parsing regexp: invalid escape sequence: `\p`
// error parsing regexp: missing argument to repetition operator: `?`
// <nil>
// error parsing regexp: invalid escape sequence: `\d`
// error parsing regexp: invalid escape sequence: `\b`
// regexp/syntax: internal error error parsing regexp: regexp/syntax: internal error: `x`
// invalid character class error parsing regexp: invalid character class: `x`
// invalid character class range error parsing regexp: invalid character class range: `x`
// invalid escape sequence error parsing regexp: invalid escape sequence: `x`
// invalid named capture error parsing regexp: invalid named capture: `x`
// invalid or unsupported Perl syntax error parsing regexp: invalid or unsupported Perl syntax: `x`
// invalid nested repetition operator error parsing regexp: invalid nested repetition operator: `x`
// invalid repeat count error parsing regexp: invalid repeat count: `x`
// invalid UTF-8 error parsing regexp: invalid UTF-8: `x`
// missing closing ] error parsing regexp: missing closing ]: `x`
// missing closing ) error parsing regexp: missing closing ): `x`
// missing argument to repetition operator error parsing regexp: missing argument to repetition operator: `x`
// trailing backslash at end of expression error parsing regexp: trailing backslash at end of expression: `x`
// unexpected ) error parsing regexp: unexpected ): `x`
// expression nests too deeply error parsing regexp: expression nests too deeply: `x`
// expression too large error parsing regexp: expression too large: `x`
// == mutations
// errors 2632 of 4000
// mutations ae130e13410eefdc
