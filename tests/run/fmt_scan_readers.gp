// phase: 09

// Fscan, Fscanln and Fscanf on every kind of reader: one with ReadRune and UnreadRune (strings.Reader, bufio.Reader, bytes.Buffer,
// bytes.Reader), one with only Read (the rune read ahead is lost between calls, bad UTF-8 is kept byte by byte), a reader that
// returns one byte at a time, data together with io.EOF, empty reads, os.Stdin replaced by a pipe for Scan, Scanln and Scanf, and
// the functions as values, in go and defer statements.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

type readOnly struct{ r io.Reader }

func (r readOnly) Read(b []byte) (int, error) { return r.r.Read(b) }

type oneByte struct{ r io.Reader }

func (r oneByte) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	return r.r.Read(b[:1])
}

// counting counts the Read calls, and returns an empty read now and then.
type counting struct {
	data  string
	reads int
	empty bool
}

func (c *counting) Read(b []byte) (int, error) {
	c.reads++
	if c.empty && c.reads%2 == 1 {
		return 0, nil
	}
	if c.data == "" {
		return 0, io.EOF
	}
	n := copy(b, c.data[:1])
	c.data = c.data[n:]
	return n, nil
}

// runeReader has ReadRune and UnreadRune, and counts them.
type runeReader struct {
	rs      []rune
	pos     int
	reads   int
	unreads int
}

func (r *runeReader) Read(b []byte) (int, error) { return 0, io.EOF }
func (r *runeReader) ReadRune() (rune, int, error) {
	r.reads++
	if r.pos >= len(r.rs) {
		return 0, 0, io.EOF
	}
	c := r.rs[r.pos]
	r.pos++
	return c, len(string(c)), nil
}
func (r *runeReader) UnreadRune() error {
	r.unreads++
	r.pos--
	return nil
}

func main() {
	var a, b int
	var s string
	var r rune

	fmt.Println("-- successive calls")
	readers := map[string]func(string) io.Reader{
		"strings":  func(s string) io.Reader { return strings.NewReader(s) },
		"bufio":    func(s string) io.Reader { return bufio.NewReader(strings.NewReader(s)) },
		"bytes":    func(s string) io.Reader { return bytes.NewBufferString(s) },
		"readOnly": func(s string) io.Reader { return readOnly{strings.NewReader(s)} },
		"oneByte":  func(s string) io.Reader { return oneByte{strings.NewReader(s)} },
		"counting": func(s string) io.Reader { return &counting{data: s} },
		"empties":  func(s string) io.Reader { return &counting{data: s, empty: true} },
	}
	for _, name := range []string{"strings", "bufio", "bytes", "readOnly", "oneByte", "counting", "empties"} {
		mk := readers[name]
		rd := mk("12 34 x56 78\nrest")
		n, err := fmt.Fscan(rd, &a)
		fmt.Println(name, "Fscan", n, err, a)
		n, err = fmt.Fscan(rd, &a)
		fmt.Println(name, "Fscan", n, err, a)
		n, err = fmt.Fscan(rd, &s)
		fmt.Println(name, "Fscan", n, err, s)
		n, err = fmt.Fscanf(rd, "%d", &a)
		fmt.Println(name, "Fscanf", n, err, a)
		n, err = fmt.Fscanf(rd, "%d", &a)
		fmt.Println(name, "Fscanf", n, err, a)
		n, err = fmt.Fscanf(rd, "%c", &r)
		fmt.Println(name, "Fscanf %c", n, err, string(r))
		n, err = fmt.Fscanln(rd, &s)
		fmt.Println(name, "Fscanln", n, err, s)
		n, err = fmt.Fscanln(rd, &s)
		fmt.Println(name, "Fscanln", n, err, s)
	}

	fmt.Println("-- no space between the items: the look-ahead rune")
	for _, name := range []string{"strings", "readOnly", "oneByte"} {
		rd := readers[name]("1,2,3")
		for i := 0; i < 3; i++ {
			a = -1
			n, err := fmt.Fscanf(rd, "%d,", &a)
			fmt.Println(name, "Fscanf", n, err, a)
		}
		rd = readers[name]("12ab34cd")
		n, err := fmt.Fscanf(rd, "%d", &a)
		fmt.Println(name, "Fscanf %d", n, err, a)
		n, err = fmt.Fscanf(rd, "%c", &r)
		fmt.Println(name, "Fscanf %c", n, err, string(r))
		n, err = fmt.Fscanf(rd, "%2s", &s)
		fmt.Println(name, "Fscanf %2s", n, err, s)
	}

	fmt.Println("-- bad UTF-8 on a reader without UnreadRune")
	for _, in := range []string{"\xc2", "\xc2X", "\xe4\xb8", "\xe4\xb8X", "\xf0\x9f\x98", "a\xffb", "\xc3\xa9\xc3", "\x80\x80"} {
		for _, name := range []string{"strings", "readOnly", "oneByte"} {
			var r1, r2, r3 rune
			rd := readers[name](in)
			n, err := fmt.Fscanf(rd, "%c%c%c", &r1, &r2, &r3)
			fmt.Printf("%-9s %q %d %v %q %q %q\n", name, in, n, err, r1, r2, r3)
		}
	}

	fmt.Println("-- rune scanner calls")
	rr := &runeReader{rs: []rune("12 ab")}
	n, err := fmt.Fscan(rr, &a, &s)
	fmt.Println(n, err, a, s, rr.reads, rr.unreads)
	rr = &runeReader{rs: []rune("1é2")}
	n, err = fmt.Fscanf(rr, "%c%c%c", &r, &r, &r)
	fmt.Println(n, err, string(r), rr.reads, rr.unreads)

	fmt.Println("-- Read calls")
	c := &counting{data: "42\n"}
	n, err = fmt.Fscanln(c, &a)
	fmt.Println(n, err, a, c.reads)
	c = &counting{data: "42"}
	n, err = fmt.Fscan(c, &a)
	fmt.Println(n, err, a, c.reads)
	c = &counting{data: ""}
	n, err = fmt.Fscan(c, &a)
	fmt.Println(n, err, a, c.reads)

	fmt.Println("-- a loop to EOF")
	rd := strings.NewReader("3 1 4 1 5 9 2 6 x 5")
	sum := 0
	for {
		var v int
		_, err := fmt.Fscan(rd, &v)
		if err != nil {
			fmt.Println("stopped:", err, err == io.EOF)
			break
		}
		sum += v
	}
	fmt.Println("sum", sum)
	rd = strings.NewReader("3 1 4\n1 5 9\n")
	sum = 0
	for {
		var v int
		_, err := fmt.Fscan(rd, &v)
		if err == io.EOF {
			break
		}
		sum += v
	}
	fmt.Println("sum", sum)
	br := bufio.NewReader(strings.NewReader("line one\nline two\n"))
	for {
		var w1, w2 string
		n, err := fmt.Fscanln(br, &w1, &w2)
		fmt.Printf("%d %v %q %q\n", n, err, w1, w2)
		if err != nil {
			break
		}
	}

	fmt.Println("-- os.Stdin")
	pr, pw, err := os.Pipe()
	if err != nil {
		fmt.Println(err)
		return
	}
	old := os.Stdin
	os.Stdin = pr
	go func() {
		io.WriteString(pw, "7 8\nnine ten\n11\n12 13\nabc 14\n")
		pw.Close()
	}()
	n, err = fmt.Scan(&a, &b)
	fmt.Println(n, err, a, b)
	n, err = fmt.Scanln(&s)
	fmt.Println(n, err, s)
	n, err = fmt.Scanln(&s, &s)
	fmt.Println(n, err, s)
	n, err = fmt.Scanf("%d\n", &a)
	fmt.Println(n, err, a)
	n, err = fmt.Scanf("%d %d\n", &a, &b)
	fmt.Println(n, err, a, b)
	n, err = fmt.Scanf("%s %d", &s, &a)
	fmt.Println(n, err, s, a)
	n, err = fmt.Scan(&a)
	fmt.Println(n, err, a)
	os.Stdin = old

	fmt.Println("-- functions as values, go and defer")
	scan := fmt.Sscan
	sscanf := fmt.Sscanf
	fscan := fmt.Fscan
	n, err = scan("5 6", &a, &b)
	fmt.Println(n, err, a, b)
	n, err = sscanf("7-8", "%d-%d", &a, &b)
	fmt.Println(n, err, a, b)
	n, err = fscan(strings.NewReader("9 10"), &a, &b)
	fmt.Println(n, err, a, b)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer fmt.Sscan("1", &a)
		a = 100
	}()
	wg.Wait()
	fmt.Println(a)
	done := make(chan bool)
	go func() {
		fmt.Sscan("77 78", &a, &b)
		done <- true
	}()
	<-done
	fmt.Println(a, b)
	t := func() (string, int) { return "42 43", 0 }
	_ = t
	args := []any{&a, &b}
	n, err = fmt.Sscan("21 22", args...)
	fmt.Println(n, err, a, b)
	n, err = fmt.Sscanf("23 24", "%d %d", args...)
	fmt.Println(n, err, a, b)
	n, err = fmt.Fscan(strings.NewReader("25 26"), args...)
	fmt.Println(n, err, a, b)
}

// Output:
// -- successive calls
// strings Fscan 1 <nil> 12
// strings Fscan 1 <nil> 34
// strings Fscan 1 <nil> x56
// strings Fscanf 1 <nil> 78
// strings Fscanf 0 unexpected newline 78
// strings Fscanf %c 1 <nil> r
// strings Fscanln 1 <nil> est
// strings Fscanln 0 EOF est
// bufio Fscan 1 <nil> 12
// bufio Fscan 1 <nil> 34
// bufio Fscan 1 <nil> x56
// bufio Fscanf 1 <nil> 78
// bufio Fscanf 0 unexpected newline 78
// bufio Fscanf %c 1 <nil> r
// bufio Fscanln 1 <nil> est
// bufio Fscanln 0 EOF est
// bytes Fscan 1 <nil> 12
// bytes Fscan 1 <nil> 34
// bytes Fscan 1 <nil> x56
// bytes Fscanf 1 <nil> 78
// bytes Fscanf 0 unexpected newline 78
// bytes Fscanf %c 1 <nil> r
// bytes Fscanln 1 <nil> est
// bytes Fscanln 0 EOF est
// readOnly Fscan 1 <nil> 12
// readOnly Fscan 1 <nil> 34
// readOnly Fscan 1 <nil> x56
// readOnly Fscanf 1 <nil> 78
// readOnly Fscanf 0 expected integer 78
// readOnly Fscanf %c 1 <nil> e
// readOnly Fscanln 1 <nil> st
// readOnly Fscanln 0 EOF st
// oneByte Fscan 1 <nil> 12
// oneByte Fscan 1 <nil> 34
// oneByte Fscan 1 <nil> x56
// oneByte Fscanf 1 <nil> 78
// oneByte Fscanf 0 expected integer 78
// oneByte Fscanf %c 1 <nil> e
// oneByte Fscanln 1 <nil> st
// oneByte Fscanln 0 EOF st
// counting Fscan 1 <nil> 12
// counting Fscan 1 <nil> 34
// counting Fscan 1 <nil> x56
// counting Fscanf 1 <nil> 78
// counting Fscanf 0 expected integer 78
// counting Fscanf %c 1 <nil> e
// counting Fscanln 1 <nil> st
// counting Fscanln 0 EOF st
// empties Fscan 1 <nil> 12
// empties Fscan 1 <nil> 34
// empties Fscan 1 <nil> x56
// empties Fscanf 1 <nil> 78
// empties Fscanf 0 expected integer 78
// empties Fscanf %c 1 <nil> e
// empties Fscanln 1 <nil> st
// empties Fscanln 0 EOF st
// -- no space between the items: the look-ahead rune
// strings Fscanf 1 <nil> 1
// strings Fscanf 1 <nil> 2
// strings Fscanf 1 unexpected EOF 3
// strings Fscanf %d 1 <nil> 12
// strings Fscanf %c 1 <nil> a
// strings Fscanf %2s 1 <nil> b3
// readOnly Fscanf 1 <nil> 1
// readOnly Fscanf 1 <nil> 2
// readOnly Fscanf 1 unexpected EOF 3
// readOnly Fscanf %d 1 <nil> 12
// readOnly Fscanf %c 1 <nil> b
// readOnly Fscanf %2s 1 <nil> 34
// oneByte Fscanf 1 <nil> 1
// oneByte Fscanf 1 <nil> 2
// oneByte Fscanf 1 unexpected EOF 3
// oneByte Fscanf %d 1 <nil> 12
// oneByte Fscanf %c 1 <nil> b
// oneByte Fscanf %2s 1 <nil> 34
// -- bad UTF-8 on a reader without UnreadRune
// strings   "\xc2" 1 EOF '�' '\x00' '\x00'
// readOnly  "\xc2" 1 EOF '�' '\x00' '\x00'
// oneByte   "\xc2" 1 EOF '�' '\x00' '\x00'
// strings   "\xc2X" 2 EOF '�' 'X' '\x00'
// readOnly  "\xc2X" 2 EOF '�' 'X' '\x00'
// oneByte   "\xc2X" 2 EOF '�' 'X' '\x00'
// strings   "\xe4\xb8" 2 EOF '�' '�' '\x00'
// readOnly  "\xe4\xb8" 2 EOF '�' '�' '\x00'
// oneByte   "\xe4\xb8" 2 EOF '�' '�' '\x00'
// strings   "\xe4\xb8X" 3 <nil> '�' '�' 'X'
// readOnly  "\xe4\xb8X" 3 <nil> '�' '�' 'X'
// oneByte   "\xe4\xb8X" 3 <nil> '�' '�' 'X'
// strings   "\xf0\x9f\x98" 3 <nil> '�' '�' '�'
// readOnly  "\xf0\x9f\x98" 3 <nil> '�' '�' '�'
// oneByte   "\xf0\x9f\x98" 3 <nil> '�' '�' '�'
// strings   "a\xffb" 3 <nil> 'a' '�' 'b'
// readOnly  "a\xffb" 3 <nil> 'a' '�' 'b'
// oneByte   "a\xffb" 3 <nil> 'a' '�' 'b'
// strings   "é\xc3" 2 EOF 'é' '�' '\x00'
// readOnly  "é\xc3" 2 EOF 'é' '�' '\x00'
// oneByte   "é\xc3" 2 EOF 'é' '�' '\x00'
// strings   "\x80\x80" 2 EOF '�' '�' '\x00'
// readOnly  "\x80\x80" 2 EOF '�' '�' '\x00'
// oneByte   "\x80\x80" 2 EOF '�' '�' '\x00'
// -- rune scanner calls
// 2 <nil> 12 ab 15 9
// 3 <nil> 2 6 3
// -- Read calls
// 1 <nil> 42 3
// 1 <nil> 42 3
// 0 EOF 42 1
// -- a loop to EOF
// stopped: expected integer false
// sum 31
// sum 23
// 2 <nil> "line" "one"
// 2 <nil> "line" "two"
// 0 EOF "" ""
// -- os.Stdin
// 2 <nil> 7 8
// 1 expected newline nine
// 1 unexpected newline en
// 1 <nil> 11
// 2 <nil> 12 13
// 2 <nil> abc 14
// 0 EOF 14
// -- functions as values, go and defer
// 2 <nil> 5 6
// 2 <nil> 7 8
// 2 <nil> 9 10
// 1
// 77 78
// 2 <nil> 21 22
// 2 <nil> 23 24
// 2 <nil> 25 26
