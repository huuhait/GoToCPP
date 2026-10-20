// phase: 09

// The errors of the Scan family: Go's texts, io.EOF and io.ErrUnexpectedEOF (==), the bad arguments (not a pointer, a type that
// can't be scanned, nil), too many and too few operands, bad formats, and what a failed scan returns.
package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

type point struct{ X, Y int }
type myInt int
type myF float32
type myBytes []byte
type myInts []int

func report(n int, err error) {
	kind := "other"
	switch {
	case err == nil:
		kind = "nil"
	case err == io.EOF:
		kind = "EOF"
	case err == io.ErrUnexpectedEOF:
		kind = "ErrUnexpectedEOF"
	case errors.Is(err, io.EOF):
		kind = "wraps EOF"
	}
	fmt.Printf("n=%d err=%v [%s]\n", n, err, kind)
}

func main() {
	var i, j int
	var s, t string
	var f float64
	var b bool
	var p point
	var m myInt
	var mf myF
	var mb myBytes
	var mi myInts

	fmt.Println("-- EOF")
	report(fmt.Sscan("", &i))
	report(fmt.Sscan("   ", &i))
	report(fmt.Sscan("1", &i, &j))
	report(fmt.Sscan("1 ", &i, &j))
	report(fmt.Sscanln("", &i))
	report(fmt.Sscanln("1", &i, &j))
	report(fmt.Sscanf("", "%d", &i))
	report(fmt.Sscanf("1", "%d %d", &i, &j))
	report(fmt.Sscanf("1 ", "%d %d", &i, &j))
	report(fmt.Sscanf("1", "%d,%d", &i, &j))
	report(fmt.Sscanf("1,", "%d,%d", &i, &j))
	report(fmt.Sscanf("", "", &i))
	report(fmt.Sscanf("", ""))
	report(fmt.Sscanf("abc", ""))
	report(fmt.Sscan("", &s))
	report(fmt.Sscan("", &f))
	report(fmt.Sscan("", &b))
	report(fmt.Sscan("\"abc", &s))
	report(fmt.Sscanf("\"abc", "%q", &s))
	report(fmt.Sscanf("`abc", "%q", &s))
	report(fmt.Sscanf("6", "%x", &s))
	report(fmt.Sscanf("", "%q", &s))
	report(fmt.Sscanf("X", "X%d", &i))
	report(fmt.Sscanf("X", "XY"))
	report(fmt.Sscanf("X", "XY%d", &i))
	report(fmt.Sscanf("XY", "%dXY", &i))

	fmt.Println("-- syntax")
	report(fmt.Sscan("abc", &i))
	report(fmt.Sscan("-", &i))
	report(fmt.Sscan("+x", &i))
	report(fmt.Sscan("1.5", &i))
	report(fmt.Sscan("1e3", &i))
	report(fmt.Sscan("x", &f))
	report(fmt.Sscan("1.5.5", &f))
	report(fmt.Sscan("1e", &f))
	report(fmt.Sscan("maybe", &b))
	report(fmt.Sscan("trux", &b))
	report(fmt.Sscan("1+", &b))
	report(fmt.Sscanf("12", "%z", &i))
	report(fmt.Sscanf("12", "%!", &i))
	report(fmt.Sscanf("12", "%", &i))
	report(fmt.Sscanf("12", "%5", &i))
	report(fmt.Sscanf("12", "%d%", &i))
	report(fmt.Sscanf("12%", "%d%", &i))
	report(fmt.Sscanf("12", "%d %", &i))
	report(fmt.Sscanf("x", "%d", &i))
	report(fmt.Sscanf("1", "%s%d", &s, &i))
	report(fmt.Sscanf("true", "%d", &b))
	report(fmt.Sscanf("1.5", "%d", &f))
	report(fmt.Sscanf("abc", "%d", &s))
	report(fmt.Sscanf("5", "%t", &i))
	report(fmt.Sscanf("5", "%q", &i))
	report(fmt.Sscanf("5", "%c", &s))
	report(fmt.Sscanf("5", "%c", &f))
	report(fmt.Sscanf("5", "%c", &b))

	fmt.Println("-- operands")
	report(fmt.Sscan("1 2 3", &i, &j))
	report(fmt.Sscan("1 2", &i, &j, &s))
	report(fmt.Sscanf("1 2 3", "%d %d", &i, &j))
	report(fmt.Sscanf("1 2 3", "%d %d %d", &i, &j))
	report(fmt.Sscanf("1 2", "%d %d", &i, &j, &s))
	report(fmt.Sscanf("1 2", "%d %d", &i))
	report(fmt.Sscanf("1 2", "%d %d"))
	report(fmt.Sscanf("1 2", "%d", i, j))
	report(fmt.Sscanf("1", "%d%%", &i))
	report(fmt.Sscanf("1%", "%d%%", &i))
	report(fmt.Sscanf("1%", "%d%%%%", &i))
	report(fmt.Sscanf("1 %", "%d %%", &i))
	report(fmt.Sscanf("1", "%d", &i, &j))
	report(fmt.Sscanf("1", "%d%d", &i))
	report(fmt.Sscanf("1", "%d %d\n", &i))
	report(fmt.Sscanf("12", "%1d%1d%1d", &i, &j, &m))

	fmt.Println("-- bad arguments")
	report(fmt.Sscan("1", i))
	report(fmt.Sscan("1", s))
	report(fmt.Sscan("1", p))
	report(fmt.Sscan("1", &p))
	report(fmt.Sscan("1", m))
	report(fmt.Sscan("1", &m))
	report(fmt.Sscan("1", &mf))
	report(fmt.Sscan("1 2", &mb))
	report(fmt.Sscan("1", &mi))
	report(fmt.Sscan("1", &[]int{}))
	report(fmt.Sscan("1", new([]string)))
	report(fmt.Sscan("1", &map[string]int{}))
	report(fmt.Sscan("1", new(map[string]int)))
	report(fmt.Sscan("1", new([2]byte)))
	report(fmt.Sscan("1", new(any)))
	report(fmt.Sscan("1", new(*int)))
	report(fmt.Sscan("1", new(func())))
	report(fmt.Sscan("1", new(chan int)))
	report(fmt.Sscan("1", new(error)))
	report(fmt.Sscan("1", new(struct{})))
	report(fmt.Sscan("1", new(**int)))
	report(fmt.Sscan("1", 5))
	report(fmt.Sscan("1", "x"))
	report(fmt.Sscan("1", 1.5))
	report(fmt.Sscan("1", []int{1}))
	report(fmt.Sscan("1", map[int]int{}))
	report(fmt.Sscan("1", func() {}))
	report(fmt.Sscan("1", true))
	report(fmt.Sscan("1", 'x'))
	fmt.Println(i, j, s, t, f, b, m, mf, mb, mi, p)

	fmt.Println("-- panics")
	for _, run := range []func(){
		func() { fmt.Sscan("1", nil) },
		func() { fmt.Sscan("1", &i, nil) },
		func() { fmt.Sscan("1", (*int)(nil)) },
		func() { fmt.Sscan("abc", (*int)(nil)) },
		func() { fmt.Sscan("1", (*string)(nil)) },
		func() { fmt.Fscan(nil, &i) },
	} {
		func() {
			defer func() {
				r := recover()
				fmt.Println("recovered:", r != nil)
			}()
			run()
		}()
	}

	fmt.Println("-- the reader's errors")
	report(fmt.Fscan(errReader{}, &i))
	report(fmt.Fscan(&errAfter{data: "12 "}, &i, &j))
	report(fmt.Fscan(&errAfter{data: "12"}, &i, &j))
	report(fmt.Fscanf(&errAfter{data: "12"}, "%d", &i))
	report(fmt.Fscanln(&errAfter{data: "12\n"}, &i))
	report(fmt.Fscan(&dataEOF{data: "12"}, &i))
	report(fmt.Fscan(strings.NewReader("zz"), &i))
	fmt.Println(i)
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("disk on fire") }

// errAfter returns its data, then an error that is not io.EOF.
type errAfter struct {
	data string
	done bool
}

func (r *errAfter) Read(b []byte) (int, error) {
	if r.data == "" {
		return 0, errors.New("read failed")
	}
	n := copy(b, r.data)
	r.data = r.data[n:]
	return n, nil
}

// dataEOF returns the data together with io.EOF.
type dataEOF struct{ data string }

func (r *dataEOF) Read(b []byte) (int, error) {
	n := copy(b, r.data)
	r.data = r.data[n:]
	if r.data == "" {
		return n, io.EOF
	}
	return n, nil
}

// Output:
// -- EOF
// n=0 err=EOF [EOF]
// n=0 err=EOF [EOF]
// n=1 err=EOF [EOF]
// n=1 err=EOF [EOF]
// n=0 err=EOF [EOF]
// n=1 err=EOF [EOF]
// n=0 err=EOF [EOF]
// n=1 err=EOF [EOF]
// n=1 err=EOF [EOF]
// n=1 err=unexpected EOF [ErrUnexpectedEOF]
// n=1 err=EOF [EOF]
// n=0 err=too many operands [other]
// n=0 err=<nil> [nil]
// n=0 err=<nil> [nil]
// n=0 err=EOF [EOF]
// n=0 err=EOF [EOF]
// n=0 err=EOF [EOF]
// n=1 err=<nil> [nil]
// n=0 err=unexpected EOF [ErrUnexpectedEOF]
// n=0 err=unexpected EOF [ErrUnexpectedEOF]
// n=0 err=unexpected EOF [ErrUnexpectedEOF]
// n=0 err=EOF [EOF]
// n=0 err=EOF [EOF]
// n=0 err=unexpected EOF [ErrUnexpectedEOF]
// n=0 err=unexpected EOF [ErrUnexpectedEOF]
// n=0 err=expected integer [other]
// -- syntax
// n=0 err=expected integer [other]
// n=0 err=EOF [EOF]
// n=0 err=expected integer [other]
// n=1 err=<nil> [nil]
// n=1 err=<nil> [nil]
// n=0 err=strconv.ParseFloat: parsing "": invalid syntax [other]
// n=1 err=<nil> [nil]
// n=0 err=strconv.ParseFloat: parsing "1e": invalid syntax [other]
// n=1 err=<nil> [nil]
// n=0 err=syntax error scanning boolean [other]
// n=1 err=<nil> [nil]
// n=0 err=bad verb '%z' for integer [other]
// n=0 err=bad verb '%!' for integer [other]
// n=0 err=missing verb: % at end of format string [other]
// n=0 err=bad verb '%5' for integer [other]
// n=1 err=missing verb: % at end of format string [other]
// n=1 err=missing verb: % at end of format string [other]
// n=1 err=missing verb: % at end of format string [other]
// n=0 err=expected integer [other]
// n=1 err=EOF [EOF]
// n=0 err=bad verb '%d' for boolean [other]
// n=0 err=bad verb '%d' for float64 [other]
// n=0 err=bad verb '%d' for string [other]
// n=0 err=bad verb '%t' for integer [other]
// n=0 err=bad verb '%q' for integer [other]
// n=0 err=bad verb '%c' for string [other]
// n=0 err=bad verb '%c' for float64 [other]
// n=0 err=bad verb '%c' for boolean [other]
// -- operands
// n=2 err=<nil> [nil]
// n=2 err=EOF [EOF]
// n=2 err=<nil> [nil]
// n=2 err=too few operands for format '%d' [other]
// n=2 err=too many operands [other]
// n=1 err=too few operands for format '%d' [other]
// n=0 err=too few operands for format '%d %d' [other]
// n=0 err=type not a pointer: int [other]
// n=1 err=unexpected EOF [ErrUnexpectedEOF]
// n=1 err=<nil> [nil]
// n=1 err=unexpected EOF [ErrUnexpectedEOF]
// n=1 err=<nil> [nil]
// n=1 err=too many operands [other]
// n=1 err=too few operands for format '%d' [other]
// n=1 err=too few operands for format '%d
// ' [other]
// n=2 err=EOF [EOF]
// -- bad arguments
// n=0 err=type not a pointer: int [other]
// n=0 err=type not a pointer: string [other]
// n=0 err=type not a pointer: main.point [other]
// n=0 err=can't scan type: *main.point [other]
// n=0 err=type not a pointer: main.myInt [other]
// n=1 err=<nil> [nil]
// n=1 err=<nil> [nil]
// n=1 err=<nil> [nil]
// n=0 err=can't scan type: *main.myInts [other]
// n=0 err=can't scan type: *[]int [other]
// n=0 err=can't scan type: *[]string [other]
// n=0 err=can't scan type: *map[string]int [other]
// n=0 err=can't scan type: *map[string]int [other]
// n=0 err=can't scan type: *[2]uint8 [other]
// n=0 err=can't scan type: *interface {} [other]
// n=0 err=can't scan type: **int [other]
// n=0 err=can't scan type: *func() [other]
// n=0 err=can't scan type: *chan int [other]
// n=0 err=can't scan type: *error [other]
// n=0 err=can't scan type: *struct {} [other]
// n=0 err=can't scan type: ***int [other]
// n=0 err=type not a pointer: int [other]
// n=0 err=type not a pointer: string [other]
// n=0 err=type not a pointer: float64 [other]
// n=0 err=type not a pointer: []int [other]
// n=0 err=type not a pointer: map[int]int [other]
// n=0 err=type not a pointer: func() [other]
// n=0 err=type not a pointer: bool [other]
// n=0 err=type not a pointer: int32 [other]
// 1 2 1  1.5 true 1 1 [49] [] {0 0}
// -- panics
// recovered: true
// recovered: true
// recovered: true
// recovered: false
// recovered: true
// recovered: true
// -- the reader's errors
// n=0 err=disk on fire [other]
// n=1 err=read failed [other]
// n=0 err=read failed [other]
// n=0 err=read failed [other]
// n=1 err=<nil> [nil]
// n=1 err=<nil> [nil]
// n=0 err=expected integer [other]
// 12
