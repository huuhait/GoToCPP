// phase: 05

// Interfaces that embed interfaces are the union of the method sets (overlapping methods are
// fine), and values convert between them by their methods; a struct that embeds both types of
// interface is assignable to each.
package main

import "fmt"

type Reader interface{ Read() string }
type Writer interface{ Write(s string) int }
type Closer interface{ Close() bool }
type ReadWriter interface {
	Reader
	Writer
}
type ReadWriteCloser interface {
	ReadWriter
	Closer
	Reader // overlapping: allowed
}

type File struct{ data string }

func (f *File) Read() string        { return f.data }
func (f *File) Write(s string) int  { f.data += s; return len(s) }
func (f *File) Close() bool         { return true }

type Buffered struct {
	*File
	n int
}

type Pair struct {
	Reader
	Writer
}

func main() {
	f := &File{"a"}
	var rwc ReadWriteCloser = f
	var rw ReadWriter = rwc
	var r Reader = rw
	rw.Write("b")
	fmt.Println(r.Read(), rwc.Close())
	b := Buffered{f, 1}
	rwc = b
	rwc.Write("c")
	fmt.Println(rwc.Read(), f.data)
	p := Pair{Reader: f, Writer: f}
	var rw2 ReadWriter = p
	rw2.Write("d")
	fmt.Println(rw2.Read(), p.Read())
	var x any = p
	_, ok := x.(Closer)
	_, ok2 := x.(ReadWriter)
	fmt.Println(ok, ok2)
	if w, ok := x.(Writer); ok {
		fmt.Println(w.Write("zz"))
	}
	x = b
	_, ok = x.(Closer)
	fmt.Println(ok)
}

// Output:
// ab true
// abc abc
// abcd abcd
// false true
// 2
// true
