// phase: 09
// strconv quoting against Go: Quote, QuoteToASCII, QuoteToGraphic, QuoteRune*, AppendQuote*, IsPrint, IsGraphic and CanBackquote over every code point (hashed), Unquote, UnquoteChar and QuotedPrefix on valid, malformed and edge inputs (value, tail and error), and round trips.

package main

import (
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"
)

type hasher struct{ h uint64 }

func (h *hasher) add(s string) {
	if h.h == 0 {
		h.h = 14695981039346656037
	}
	for i := 0; i < len(s); i++ {
		h.h ^= uint64(s[i])
		h.h *= 1099511628211
	}
	h.h ^= 0xff
	h.h *= 1099511628211
}

func es(err error) string {
	if err == nil {
		return "<nil>"
	}
	if errors.Is(err, strconv.ErrSyntax) {
		return "ErrSyntax"
	}
	return err.Error()
}

func main() {
	// every code point
	var hq, ha, hg, hr, hp, hb, hu hasher
	nPrint, nGraphic, nBack := 0, 0, 0
	bad := 0
	for r := rune(-2); r <= 0x110010; r++ {
		hq.add(strconv.QuoteRune(r))
		hq.add(strconv.QuoteRuneToASCII(r))
		hq.add(strconv.QuoteRuneToGraphic(r))
		if strconv.IsPrint(r) {
			nPrint++
			hp.add(strconv.Itoa(int(r)))
		}
		if strconv.IsGraphic(r) {
			nGraphic++
		}
		if r >= 0 && r < 0xd800 || r > 0xdfff && r <= 0x10ffff {
			s := string(r)
			if strconv.CanBackquote(s) {
				nBack++
			}
			q := strconv.Quote(s)
			ha.add(q)
			hg.add(strconv.QuoteToASCII(s))
			hr.add(strconv.QuoteToGraphic(s))
			u, err := strconv.Unquote(q)
			if err != nil || u != s {
				bad++
			}
			u, err = strconv.Unquote(strconv.QuoteToASCII(s))
			if err != nil || u != s {
				bad++
			}
			v, mb, tail, err := strconv.UnquoteChar(strconv.QuoteRune(r)[1:], '\'')
			hu.add(fmt.Sprint(v, mb, tail, err))
			if err != nil && r != '\'' || mb != (r >= 0x80) && err == nil && r != '\\' {
				bad++
			}
		}
		hb.add(strconv.QuoteRune(r))
	}
	fmt.Println("printable", nPrint, "graphic", nGraphic, "backquotable", nBack, "bad", bad)
	fmt.Printf("QuoteRune* %016x\nQuote %016x\nQuoteToASCII %016x\nQuoteToGraphic %016x\nIsPrint %016x\nUnquoteChar %016x\n", hq.h, ha.h, hg.h, hr.h, hp.h, hu.h)

	// strings
	strs := []string{"", "hello", "hello\n", "a\tb", "a\x00b", "\a\b\f\r\v", "\x7f", "\x80", "\xff", "\xc0\xaf", "\xed\xa0\x80", "é", "日本語", "\u00a0", "\u2028", "\U0001f600", "\U0010ffff", "\ufffd", "\ufeff", "a\ufeffb", "'", "\"", "`", "\\", "a\"b'c`d\\e", "\u200b", "\u0300", "\U000e0001", "x\u00adx", "\xf4\x90\x80\x80", "abc\xe2\x82", "tab\there"}
	for _, s := range strs {
		q := strconv.Quote(s)
		fmt.Println(q, strconv.QuoteToASCII(s), strconv.QuoteToGraphic(s), strconv.CanBackquote(s), string(strconv.AppendQuote([]byte("<"), s)), string(strconv.AppendQuoteToASCII(nil, s)), string(strconv.AppendQuoteToGraphic([]byte(">"), s)))
		u, err := strconv.Unquote(q)
		fmt.Printf("  unquote %q %v %v\n", u, err, u == s)
	}

	// Unquote: valid and malformed
	ins := []string{`""`, `"a"`, `"\""`, `"\'"`, `'a'`, `'\''`, `'\"'`, `'ab'`, `''`, "``", "`a\nb`", "`a\r\nb`", "`a`b`", `"a`, `a"`, `"a"b`, `"\x41"`, `"\x4"`, `"é"`, `"\u00e"`, `"\U0001f600"`, `"\U00110000"`, `"\ud800"`, `"\udfff"`, `"\101"`, `"\400"`, `"\777"`, `"\8"`, `"\a\b\f\n\r\t\v\\"`, `"\z"`, `"\"`, `"\`, "\"a\nb\"", "'\n'", `'\x80'`, `"\x80"`, `'\377'`, `'\u12345'`, `"日本語"`, `'日'`, `'日本'`, `"\xff"`, "\"\xff\"", "'\xff'", `"a\
b"`, `"` + "\x00" + `"`, `'\0'`, `'\00'`, `'\000'`, "`\x00`", "`\xff`", `"A\U00000042\103"`, `x`, `"x'`, "'x\"", `'''`, `"""`}
	for _, in := range ins {
		u, err := strconv.Unquote(in)
		fmt.Printf("%q -> %q %s\n", in, u, es(err))
		pre, err := strconv.QuotedPrefix(in + "tail")
		fmt.Printf("  prefix %q %s\n", pre, es(err))
	}

	// UnquoteChar with all quotes
	chars := []string{"a", "\\n", "\\x41", "\\u00e9", "\\U0001f600", "\\101", "\\'", "\\\"", "\\\\", "\\", "\\z", "\\x4", "é", "\xff", "\\400", "\\ud800", "", "'", "\"", "\\xff", "\\377"}
	for _, c := range chars {
		for _, q := range []byte{0, '\'', '"'} {
			v, mb, tail, err := strconv.UnquoteChar(c+"rest", q)
			fmt.Printf("%q %d -> %d %v %q %s\n", c, q, v, mb, tail, es(err))
		}
	}

	// valid UTF-8 check on QuoteRune of invalid runes
	for _, r := range []rune{-1, 0xd800, 0xdfff, 0x110000, utf8.RuneError, 0x7f, 0xad} {
		fmt.Println(strconv.QuoteRune(r), strconv.QuoteRuneToASCII(r), strconv.QuoteRuneToGraphic(r), string(strconv.AppendQuoteRune(nil, r)))
	}
}

// Output:
// printable 148998 graphic 149014 backquotable 1112030 bad 0
// QuoteRune* 11a28e68619fac76
// Quote 94ae2185afa36eae
// QuoteToASCII a34aa9a24c3d9e94
// QuoteToGraphic 3b39d6c91fab73ae
// IsPrint 7d66a9340c0c3b51
// UnquoteChar d0462dcc5eb81eef
// "" "" "" true <"" "" >""
//   unquote "" <nil> true
// "hello" "hello" "hello" true <"hello" "hello" >"hello"
//   unquote "hello" <nil> true
// "hello\n" "hello\n" "hello\n" false <"hello\n" "hello\n" >"hello\n"
//   unquote "hello\n" <nil> true
// "a\tb" "a\tb" "a\tb" true <"a\tb" "a\tb" >"a\tb"
//   unquote "a\tb" <nil> true
// "a\x00b" "a\x00b" "a\x00b" false <"a\x00b" "a\x00b" >"a\x00b"
//   unquote "a\x00b" <nil> true
// "\a\b\f\r\v" "\a\b\f\r\v" "\a\b\f\r\v" false <"\a\b\f\r\v" "\a\b\f\r\v" >"\a\b\f\r\v"
//   unquote "\a\b\f\r\v" <nil> true
// "\x7f" "\x7f" "\x7f" false <"\x7f" "\x7f" >"\x7f"
//   unquote "\x7f" <nil> true
// "\x80" "\x80" "\x80" false <"\x80" "\x80" >"\x80"
//   unquote "\x80" <nil> true
// "\xff" "\xff" "\xff" false <"\xff" "\xff" >"\xff"
//   unquote "\xff" <nil> true
// "\xc0\xaf" "\xc0\xaf" "\xc0\xaf" false <"\xc0\xaf" "\xc0\xaf" >"\xc0\xaf"
//   unquote "\xc0\xaf" <nil> true
// "\xed\xa0\x80" "\xed\xa0\x80" "\xed\xa0\x80" false <"\xed\xa0\x80" "\xed\xa0\x80" >"\xed\xa0\x80"
//   unquote "\xed\xa0\x80" <nil> true
// "é" "\u00e9" "é" true <"é" "\u00e9" >"é"
//   unquote "é" <nil> true
// "日本語" "\u65e5\u672c\u8a9e" "日本語" true <"日本語" "\u65e5\u672c\u8a9e" >"日本語"
//   unquote "日本語" <nil> true
// "\u00a0" "\u00a0" " " true <"\u00a0" "\u00a0" >" "
//   unquote "\u00a0" <nil> true
// "\u2028" "\u2028" "\u2028" true <"\u2028" "\u2028" >"\u2028"
//   unquote "\u2028" <nil> true
// "😀" "\U0001f600" "😀" true <"😀" "\U0001f600" >"😀"
//   unquote "😀" <nil> true
// "\U0010ffff" "\U0010ffff" "\U0010ffff" true <"\U0010ffff" "\U0010ffff" >"\U0010ffff"
//   unquote "\U0010ffff" <nil> true
// "�" "\ufffd" "�" true <"�" "\ufffd" >"�"
//   unquote "�" <nil> true
// "\ufeff" "\ufeff" "\ufeff" false <"\ufeff" "\ufeff" >"\ufeff"
//   unquote "\ufeff" <nil> true
// "a\ufeffb" "a\ufeffb" "a\ufeffb" false <"a\ufeffb" "a\ufeffb" >"a\ufeffb"
//   unquote "a\ufeffb" <nil> true
// "'" "'" "'" true <"'" "'" >"'"
//   unquote "'" <nil> true
// "\"" "\"" "\"" true <"\"" "\"" >"\""
//   unquote "\"" <nil> true
// "`" "`" "`" false <"`" "`" >"`"
//   unquote "`" <nil> true
// "\\" "\\" "\\" true <"\\" "\\" >"\\"
//   unquote "\\" <nil> true
// "a\"b'c`d\\e" "a\"b'c`d\\e" "a\"b'c`d\\e" false <"a\"b'c`d\\e" "a\"b'c`d\\e" >"a\"b'c`d\\e"
//   unquote "a\"b'c`d\\e" <nil> true
// "\u200b" "\u200b" "\u200b" true <"\u200b" "\u200b" >"\u200b"
//   unquote "\u200b" <nil> true
// "̀" "\u0300" "̀" true <"̀" "\u0300" >"̀"
//   unquote "̀" <nil> true
// "\U000e0001" "\U000e0001" "\U000e0001" true <"\U000e0001" "\U000e0001" >"\U000e0001"
//   unquote "\U000e0001" <nil> true
// "x\u00adx" "x\u00adx" "x\u00adx" true <"x\u00adx" "x\u00adx" >"x\u00adx"
//   unquote "x\u00adx" <nil> true
// "\xf4\x90\x80\x80" "\xf4\x90\x80\x80" "\xf4\x90\x80\x80" false <"\xf4\x90\x80\x80" "\xf4\x90\x80\x80" >"\xf4\x90\x80\x80"
//   unquote "\xf4\x90\x80\x80" <nil> true
// "abc\xe2\x82" "abc\xe2\x82" "abc\xe2\x82" false <"abc\xe2\x82" "abc\xe2\x82" >"abc\xe2\x82"
//   unquote "abc\xe2\x82" <nil> true
// "tab\there" "tab\there" "tab\there" true <"tab\there" "tab\there" >"tab\there"
//   unquote "tab\there" <nil> true
// "\"\"" -> "" <nil>
//   prefix "\"\"" <nil>
// "\"a\"" -> "a" <nil>
//   prefix "\"a\"" <nil>
// "\"\\\"\"" -> "\"" <nil>
//   prefix "\"\\\"\"" <nil>
// "\"\\'\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "'a'" -> "a" <nil>
//   prefix "'a'" <nil>
// "'\\''" -> "'" <nil>
//   prefix "'\\''" <nil>
// "'\\\"'" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "'ab'" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "''" -> "" <nil>
//   prefix "''" <nil>
// "``" -> "" <nil>
//   prefix "``" <nil>
// "`a\nb`" -> "a\nb" <nil>
//   prefix "`a\nb`" <nil>
// "`a\r\nb`" -> "a\nb" <nil>
//   prefix "`a\r\nb`" <nil>
// "`a`b`" -> "" ErrSyntax
//   prefix "`a`" <nil>
// "\"a" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "a\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"a\"b" -> "" ErrSyntax
//   prefix "\"a\"" <nil>
// "\"\\x41\"" -> "A" <nil>
//   prefix "\"\\x41\"" <nil>
// "\"\\x4\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"é\"" -> "é" <nil>
//   prefix "\"é\"" <nil>
// "\"\\u00e\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"\\U0001f600\"" -> "😀" <nil>
//   prefix "\"\\U0001f600\"" <nil>
// "\"\\U00110000\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"\\ud800\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"\\udfff\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"\\101\"" -> "A" <nil>
//   prefix "\"\\101\"" <nil>
// "\"\\400\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"\\777\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"\\8\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"\\a\\b\\f\\n\\r\\t\\v\\\\\"" -> "\a\b\f\n\r\t\v\\" <nil>
//   prefix "\"\\a\\b\\f\\n\\r\\t\\v\\\\\"" <nil>
// "\"\\z\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"\\\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"\\" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"a\nb\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "'\n'" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "'\\x80'" -> "\x80" <nil>
//   prefix "'\\x80'" <nil>
// "\"\\x80\"" -> "\x80" <nil>
//   prefix "\"\\x80\"" <nil>
// "'\\377'" -> "\xff" <nil>
//   prefix "'\\377'" <nil>
// "'\\u12345'" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"日本語\"" -> "日本語" <nil>
//   prefix "\"日本語\"" <nil>
// "'日'" -> "日" <nil>
//   prefix "'日'" <nil>
// "'日本'" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"\\xff\"" -> "\xff" <nil>
//   prefix "\"\\xff\"" <nil>
// "\"\xff\"" -> "�" <nil>
//   prefix "\"\xff\"" <nil>
// "'\xff'" -> "�" <nil>
//   prefix "'\xff'" <nil>
// "\"a\\\nb\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"\x00\"" -> "\x00" <nil>
//   prefix "\"\x00\"" <nil>
// "'\\0'" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "'\\00'" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "'\\000'" -> "\x00" <nil>
//   prefix "'\\000'" <nil>
// "`\x00`" -> "\x00" <nil>
//   prefix "`\x00`" <nil>
// "`\xff`" -> "\xff" <nil>
//   prefix "`\xff`" <nil>
// "\"A\\U00000042\\103\"" -> "ABC" <nil>
//   prefix "\"A\\U00000042\\103\"" <nil>
// "x" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "\"x'" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "'x\"" -> "" ErrSyntax
//   prefix "" ErrSyntax
// "'''" -> "" ErrSyntax
//   prefix "''" <nil>
// "\"\"\"" -> "" ErrSyntax
//   prefix "\"\"" <nil>
// "a" 0 -> 97 false "rest" <nil>
// "a" 39 -> 97 false "rest" <nil>
// "a" 34 -> 97 false "rest" <nil>
// "\\n" 0 -> 10 false "rest" <nil>
// "\\n" 39 -> 10 false "rest" <nil>
// "\\n" 34 -> 10 false "rest" <nil>
// "\\x41" 0 -> 65 false "rest" <nil>
// "\\x41" 39 -> 65 false "rest" <nil>
// "\\x41" 34 -> 65 false "rest" <nil>
// "\\u00e9" 0 -> 233 true "rest" <nil>
// "\\u00e9" 39 -> 233 true "rest" <nil>
// "\\u00e9" 34 -> 233 true "rest" <nil>
// "\\U0001f600" 0 -> 128512 true "rest" <nil>
// "\\U0001f600" 39 -> 128512 true "rest" <nil>
// "\\U0001f600" 34 -> 128512 true "rest" <nil>
// "\\101" 0 -> 65 false "rest" <nil>
// "\\101" 39 -> 65 false "rest" <nil>
// "\\101" 34 -> 65 false "rest" <nil>
// "\\'" 0 -> 0 false "" ErrSyntax
// "\\'" 39 -> 39 false "rest" <nil>
// "\\'" 34 -> 0 false "" ErrSyntax
// "\\\"" 0 -> 0 false "" ErrSyntax
// "\\\"" 39 -> 0 false "" ErrSyntax
// "\\\"" 34 -> 34 false "rest" <nil>
// "\\\\" 0 -> 92 false "rest" <nil>
// "\\\\" 39 -> 92 false "rest" <nil>
// "\\\\" 34 -> 92 false "rest" <nil>
// "\\" 0 -> 13 false "est" <nil>
// "\\" 39 -> 13 false "est" <nil>
// "\\" 34 -> 13 false "est" <nil>
// "\\z" 0 -> 0 false "" ErrSyntax
// "\\z" 39 -> 0 false "" ErrSyntax
// "\\z" 34 -> 0 false "" ErrSyntax
// "\\x4" 0 -> 0 false "" ErrSyntax
// "\\x4" 39 -> 0 false "" ErrSyntax
// "\\x4" 34 -> 0 false "" ErrSyntax
// "é" 0 -> 233 true "rest" <nil>
// "é" 39 -> 233 true "rest" <nil>
// "é" 34 -> 233 true "rest" <nil>
// "\xff" 0 -> 65533 true "rest" <nil>
// "\xff" 39 -> 65533 true "rest" <nil>
// "\xff" 34 -> 65533 true "rest" <nil>
// "\\400" 0 -> 0 false "" ErrSyntax
// "\\400" 39 -> 0 false "" ErrSyntax
// "\\400" 34 -> 0 false "" ErrSyntax
// "\\ud800" 0 -> 0 false "" ErrSyntax
// "\\ud800" 39 -> 0 false "" ErrSyntax
// "\\ud800" 34 -> 0 false "" ErrSyntax
// "" 0 -> 114 false "est" <nil>
// "" 39 -> 114 false "est" <nil>
// "" 34 -> 114 false "est" <nil>
// "'" 0 -> 39 false "rest" <nil>
// "'" 39 -> 0 false "" ErrSyntax
// "'" 34 -> 39 false "rest" <nil>
// "\"" 0 -> 34 false "rest" <nil>
// "\"" 39 -> 34 false "rest" <nil>
// "\"" 34 -> 0 false "" ErrSyntax
// "\\xff" 0 -> 255 false "rest" <nil>
// "\\xff" 39 -> 255 false "rest" <nil>
// "\\xff" 34 -> 255 false "rest" <nil>
// "\\377" 0 -> 255 false "rest" <nil>
// "\\377" 39 -> 255 false "rest" <nil>
// "\\377" 34 -> 255 false "rest" <nil>
// '�' '\ufffd' '�' '�'
// '�' '\ufffd' '�' '�'
// '�' '\ufffd' '�' '�'
// '�' '\ufffd' '�' '�'
// '�' '\ufffd' '�' '�'
// '\x7f' '\x7f' '\x7f' '\x7f'
// '\u00ad' '\u00ad' '\u00ad' '\u00ad'
