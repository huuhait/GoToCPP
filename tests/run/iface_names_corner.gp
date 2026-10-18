// phase: 05

// Method names that are C++ keywords or the name of their type, anonymous struct and array types
// in interfaces, defer through an interface (step 5.4).
package main

import "fmt"

type Foo interface{ Foo() string }
type Switch interface {
	new() int
	delete(int)
	class() string
	this() bool
}

type impl struct{ n int }

func (i *impl) new() int       { i.n++; return i.n }
func (i *impl) delete(k int)   { i.n -= k }
func (i *impl) class() string  { return "impl" }
func (i *impl) this() bool     { return true }
func (i impl) Foo() string     { return "foo" }

type Closer interface{ Close() string }
type res struct{ name string }

func (r res) Close() string { fmt.Println("closing", r.name); return r.name }

func use(c Closer) {
	defer c.Close()
	fmt.Println("using", c.(res).name)
}

func main() {
	var f Foo = impl{}
	fmt.Println(f.Foo())
	var s Switch = &impl{}
	s.new()
	s.new()
	s.delete(1)
	fmt.Println(s.new(), s.class(), s.this())
	var a any = struct {
		X int
		Y string
	}{1, "y"}
	st := a.(struct {
		X int
		Y string
	})
	fmt.Println(st.X, st.Y)
	_, ok := a.(struct{ X, Y int })
	fmt.Println(ok)
	var arr any = [3]string{"a", "b", "c"}
	fmt.Println(arr.([3]string)[1])
	_, ok = arr.([2]string)
	fmt.Println(ok)
	use(res{"r1"})
	c := Closer(res{"r2"})
	defer fmt.Println("deferred last")
	defer c.Close()
	c = res{"r3"}
	defer c.Close()
}

// Output:
// foo
// 2 impl true
// 1 y
// false
// b
// false
// using r1
// closing r1
// closing r3
// closing r2
// deferred last
