// phase: 05

// Conversions between interface types: to a smaller method set, to any, back by an assertion;
// the nil interface stays nil (step 5.4).
package main

import "fmt"

type Reader interface{ Read() string }
type Writer interface{ Write(s string) int }
type ReadWriter interface {
	Reader
	Writer
}
type Closer interface{ Close() bool }
type ReadWriteCloser interface {
	ReadWriter
	Closer
}

type File struct {
	data   string
	closed bool
}

func (f *File) Read() string        { return f.data }
func (f *File) Write(s string) int  { f.data += s; return len(s) }
func (f *File) Close() bool         { f.closed = true; return true }

func readAll(r Reader) string  { return r.Read() }
func writeTo(w Writer, s string) int { return w.Write(s) }
func asAny(x any) any          { return x }

func main() {
	f := &File{}
	var rwc ReadWriteCloser = f
	var rw ReadWriter = rwc
	var r Reader = rw
	var w Writer = rwc
	writeTo(w, "hello")
	writeTo(rw, " world")
	fmt.Println(readAll(r), readAll(rwc), readAll(f))
	var a any = rwc
	_, isCloser := a.(Closer)
	_, isReader := a.(Reader)
	fmt.Println(isCloser, isReader)
	var c Closer = a.(Closer)
	fmt.Println(c.Close(), f.closed)
	var nilRW ReadWriter
	var nilR Reader = nilRW
	var nilA any = nilR
	fmt.Println(nilRW == nil, nilR == nil, nilA == nil)
	b := asAny(rw)
	fmt.Println(b.(ReadWriter).Read())
	_, ok := b.(ReadWriteCloser)
	fmt.Println(ok)
	rs := []Reader{f, rw, rwc, r}
	n := 0
	for _, x := range rs {
		n += len(x.Read())
	}
	fmt.Println(n)
}

// Output:
// hello world hello world hello world
// true true
// true true
// true true true
// hello world
// true
// 44
