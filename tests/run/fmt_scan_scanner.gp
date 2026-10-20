// phase: 09

// Types with a Scan method (fmt.Scanner) and the fmt.ScanState they get: ReadRune, UnreadRune, SkipSpace, Token with and without a
// function, Width, Read (always an error), the verb, errors and io.EOF from Scan, a pointer or a value receiver, scans inside scans
// (Fscan on the state), Scanners as elements of an interface, and in Scanln and Scanf.
package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

type word string

func (w *word) Scan(state fmt.ScanState, verb rune) error {
	tok, err := state.Token(true, nil)
	if err != nil {
		return err
	}
	if len(tok) == 0 {
		return errors.New("no word")
	}
	*w = word(strings.ToUpper(string(tok)))
	return nil
}

type digits int

func (d *digits) Scan(state fmt.ScanState, verb rune) error {
	tok, err := state.Token(false, unicode.IsDigit)
	if err != nil {
		return err
	}
	*d = digits(len(tok))
	return nil
}

type logged struct {
	verbs []rune
	width string
	rest  string
}

func (l *logged) Scan(state fmt.ScanState, verb rune) error {
	l.verbs = append(l.verbs, verb)
	w, ok := state.Width()
	l.width = fmt.Sprint(w, ok)
	state.SkipSpace()
	var rs []rune
	for {
		r, size, err := state.ReadRune()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if r == ';' {
			state.UnreadRune()
			break
		}
		rs = append(rs, r)
		_ = size
	}
	l.rest = string(rs)
	return nil
}

type readMe struct{ msg string }

func (r *readMe) Scan(state fmt.ScanState, verb rune) error {
	n, err := state.Read(make([]byte, 4))
	r.msg = fmt.Sprint(n, err)
	return nil
}

// byValue has a value receiver: it can be passed by value, but then it stores nowhere.
type byValue struct{ n *int }

func (b byValue) Scan(state fmt.ScanState, verb rune) error {
	tok, err := state.Token(true, nil)
	*b.n = len(tok)
	return err
}

type failing struct{ err error }

func (f failing) Scan(state fmt.ScanState, verb rune) error { return f.err }

type pair struct{ a, b int }

func (p *pair) Scan(state fmt.ScanState, verb rune) error {
	if _, err := fmt.Fscan(state, &p.a, &p.b); err != nil {
		return err
	}
	return nil
}

type paren struct{ inner pair }

func (p *paren) Scan(state fmt.ScanState, verb rune) error {
	if _, err := fmt.Fscanf(state, "(%d,%d)", &p.inner.a, &p.inner.b); err != nil {
		return err
	}
	return nil
}

type line string

func (l *line) Scan(state fmt.ScanState, verb rune) error {
	var rs []rune
	for {
		r, _, err := state.ReadRune()
		if err != nil {
			if err == io.EOF && len(rs) > 0 {
				break
			}
			return err
		}
		if r == '\n' {
			break
		}
		rs = append(rs, r)
	}
	*l = line(rs)
	return nil
}

type panicky struct{}

func (panicky) Scan(state fmt.ScanState, verb rune) error { panic("boom") }

type skipper struct{ got string }

func (s *skipper) Scan(state fmt.ScanState, verb rune) error {
	state.SkipSpace()
	r, _, err := state.ReadRune()
	s.got = string(r)
	return err
}

func main() {
	var w word
	var d digits
	var i int
	var l logged

	fmt.Println("-- Token")
	n, err := fmt.Sscan("hello world", &w, &i)
	fmt.Println(n, err, w, i)
	n, err = fmt.Sscan("  hello\nworld  ", &w)
	fmt.Println(n, err, w)
	var w2 word
	n, err = fmt.Sscan("a b", &w, &w2)
	fmt.Println(n, err, w, w2)
	n, err = fmt.Sscan("", &w)
	fmt.Println(n, err, w)
	n, err = fmt.Sscan("12345abc", &d)
	fmt.Println(n, err, d)
	n, err = fmt.Sscan("abc", &d)
	fmt.Println(n, err, d)
	n, err = fmt.Sscan("12 34", &d, &i)
	fmt.Println(n, err, d, i)
	n, err = fmt.Sscanf("12 34", "%v %d", &d, &i)
	fmt.Println(n, err, d, i)
	n, err = fmt.Sscanf("12,34", "%v,%d", &d, &i)
	fmt.Println(n, err, d, i)

	fmt.Println("-- verbs, width, ReadRune, UnreadRune")
	n, err = fmt.Sscanf("  abc;def", "%v;%d", &l, &i)
	fmt.Printf("%d %v %q %q %q %q\n", n, err, string(l.verbs), l.width, l.rest, i)
	l = logged{}
	n, err = fmt.Sscanf("abcdef", "%3x", &l)
	fmt.Printf("%d %v %q %q %q\n", n, err, string(l.verbs), l.width, l.rest)
	l = logged{}
	n, err = fmt.Sscanf("abcdef", "%q%s", &l, &w)
	fmt.Printf("%d %v %q %q %q %q\n", n, err, string(l.verbs), l.width, l.rest, w)
	l = logged{}
	n, err = fmt.Sscan("é世 x;", &l)
	fmt.Printf("%d %v %q %q %q\n", n, err, string(l.verbs), l.width, l.rest)
	l = logged{}
	n, err = fmt.Sscanln("abc\ndef", &l)
	fmt.Printf("%d %v %q %q %q\n", n, err, string(l.verbs), l.width, l.rest)
	l = logged{}
	n, err = fmt.Sscan("abc\ndef", &l)
	fmt.Printf("%d %v %q %q %q\n", n, err, string(l.verbs), l.width, l.rest)

	fmt.Println("-- Read")
	var rm readMe
	n, err = fmt.Sscan("abcd", &rm)
	fmt.Println(n, err, rm.msg)

	fmt.Println("-- value receiver, errors")
	var cnt int
	n, err = fmt.Sscan("abc def", byValue{&cnt})
	fmt.Println(n, err, cnt)
	n, err = fmt.Sscan("", byValue{&cnt})
	fmt.Println(n, err, cnt)
	e := errors.New("custom")
	n, err = fmt.Sscan("x", failing{e})
	fmt.Println(n, err, err == e)
	n, err = fmt.Sscan("x", failing{io.EOF})
	fmt.Println(n, err, err == io.ErrUnexpectedEOF, err == io.EOF)
	n, err = fmt.Sscan("x", failing{io.ErrUnexpectedEOF})
	fmt.Println(n, err, err == io.ErrUnexpectedEOF)
	n, err = fmt.Sscan("1 x", &i, failing{e})
	fmt.Println(n, err, i)
	n, err = fmt.Sscanf("1 x", "%d %v", &i, failing{nil})
	fmt.Println(n, err, i)
	n, err = fmt.Sscanf("1", "%d %v", &i, failing{nil})
	fmt.Println(n, err, i)
	var iface any = &w
	n, err = fmt.Sscan("iface", iface)
	fmt.Println(n, err, w)
	var sc fmt.Scanner = &d
	n, err = fmt.Sscan("123", sc)
	fmt.Println(n, err, d)
	var nilScanner fmt.Scanner
	func() {
		defer func() { fmt.Println("recovered:", recover() != nil) }()
		fmt.Sscan("x", nilScanner)
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		fmt.Sscan("x", panicky{})
	}()

	fmt.Println("-- scans inside scans")
	var p pair
	n, err = fmt.Sscan("1 2 3", &p, &i)
	fmt.Println(n, err, p, i)
	n, err = fmt.Sscan("1", &p)
	fmt.Println(n, err, p)
	n, err = fmt.Sscan("1 x", &p)
	fmt.Println(n, err, p)
	var pp paren
	n, err = fmt.Sscan("(3,4) 5", &pp, &i)
	fmt.Println(n, err, pp, i)
	n, err = fmt.Sscanf("(3,4)(5,6)", "%v%d", &pp, &i)
	fmt.Println(n, err, pp, i)
	n, err = fmt.Sscan("(3;4)", &pp)
	fmt.Println(n, err, pp)

	fmt.Println("-- lines")
	var l1, l2 line
	n, err = fmt.Sscan("first line\nsecond line\n", &l1, &l2)
	fmt.Printf("%d %v %q %q\n", n, err, l1, l2)
	n, err = fmt.Sscanln("first line\nsecond line\n", &l1, &l2)
	fmt.Printf("%d %v %q %q\n", n, err, l1, l2)
	n, err = fmt.Sscanf("first line\nsecond line\n", "%v%v", &l1, &l2)
	fmt.Printf("%d %v %q %q\n", n, err, l1, l2)
	n, err = fmt.Sscanf("first line\nsecond line\n", "%v\n%v", &l1, &l2)
	fmt.Printf("%d %v %q %q\n", n, err, l1, l2)

	fmt.Println("-- SkipSpace follows the mode")
	var sk skipper
	for _, in := range []string{"  x", "\nx", " \n x", "x"} {
		n, err = fmt.Sscan(in, &sk)
		fmt.Printf("Sscan   %q %d %v %q\n", in, n, err, sk.got)
		sk.got = ""
		n, err = fmt.Sscanln(in, &sk)
		fmt.Printf("Sscanln %q %d %v %q\n", in, n, err, sk.got)
		sk.got = ""
		n, err = fmt.Sscanf(in, "%v", &sk)
		fmt.Printf("Sscanf  %q %d %v %q\n", in, n, err, sk.got)
		sk.got = ""
	}

	fmt.Println("-- readers")
	r := strings.NewReader("alpha beta 12")
	n, err = fmt.Fscan(r, &w, &w2, &d)
	fmt.Println(n, err, w, w2, d)
}

// Output:
// -- Token
// 1 expected integer HELLO 0
// 1 <nil> HELLO
// 2 <nil> A B
// 0 no word A
// 1 <nil> 5
// 1 <nil> 0
// 2 <nil> 2 34
// 2 <nil> 2 34
// 2 <nil> 2 34
// -- verbs, width, ReadRune, UnreadRune
// 1 expected integer "v" "0 false" "abc" '"'
// 1 <nil> "x" "3 true" "abc"
// 1 no word "q" "0 false" "abcdef" "A"
// 1 <nil> "v" "0 false" "é世 x"
// 1 <nil> "v" "0 false" "abc\n"
// 1 <nil> "v" "0 false" "abc\ndef"
// -- Read
// 1 <nil> 0 ScanState's Read should not be called. Use ReadRune
// -- value receiver, errors
// 1 <nil> 3
// 1 <nil> 0
// 0 custom true
// 0 unexpected EOF true false
// 0 unexpected EOF true
// 1 custom 1
// 2 <nil> 1
// 2 <nil> 1
// 1 <nil> IFACE
// 1 <nil> 3
// recovered: true
// recovered: boom
// -- scans inside scans
// 2 <nil> {1 2} 3
// 0 unexpected EOF {1 2}
// 0 expected integer {1 2}
// 2 <nil> {{3 4}} 5
// 1 expected integer {{3 4}} 5
// 0 input does not match format {{3 4}}
// -- lines
// 2 <nil> "first line" "second line"
// 1 unexpected EOF "first line" "second line"
// 2 <nil> "first line" "second line"
// 1 newline in format does not match input "first line" "second line"
// -- SkipSpace follows the mode
// Sscan   "  x" 1 <nil> "x"
// Sscanln "  x" 1 <nil> "x"
// Sscanf  "  x" 1 <nil> "x"
// Sscan   "\nx" 1 <nil> "x"
// Sscanln "\nx" 0 unexpected newline ""
// Sscanf  "\nx" 0 unexpected newline ""
// Sscan   " \n x" 1 <nil> "x"
// Sscanln " \n x" 0 unexpected newline ""
// Sscanf  " \n x" 0 unexpected newline ""
// Sscan   "x" 1 <nil> "x"
// Sscanln "x" 1 <nil> "x"
// Sscanf  "x" 1 <nil> "x"
// -- readers
// 3 <nil> ALPHA BETA 0
