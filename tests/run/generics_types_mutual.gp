// phase: 07
// lsan: cycle

// Mutually recursive generic types: each instance refers to the other (a reference cycle that reference counting never frees, D1: lsan: cycle).
package main

import "fmt"

type Dir[T any] struct {
	name    string
	files   []*File[T]
	parent  *Dir[T]
	payload T
}

type File[T any] struct {
	name string
	dir  *Dir[T]
	data T
}

func (d *Dir[T]) Add(name string, data T) *File[T] {
	f := &File[T]{name, d, data}
	d.files = append(d.files, f)
	return f
}

func (f *File[T]) Path() string {
	if f.dir.parent == nil {
		return "/" + f.name
	}
	return f.dir.parent.name + "/" + f.dir.name + "/" + f.name
}

func (d *Dir[T]) Sub(name string, payload T) *Dir[T] {
	return &Dir[T]{name: name, parent: d, payload: payload}
}

func main() {
	root := &Dir[int]{name: "root"}
	f := root.Add("a", 1)
	sub := root.Sub("sub", 7)
	g := sub.Add("b", 2)
	fmt.Println(f.Path(), g.Path(), f.data+g.data, sub.payload)
	s := &Dir[string]{name: "r"}
	h := s.Add("x", "data")
	fmt.Println(h.Path(), h.data, len(s.files))
	fmt.Printf("%T %T\n", f, sub)
}

// Output:
// /a root/sub/b 3 7
// /x data 1
// *main.File[int] *main.Dir[int]
