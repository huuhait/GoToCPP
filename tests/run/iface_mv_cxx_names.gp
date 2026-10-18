// phase: 05

// Interface methods named like C++ keywords and runtime names (step 5.9): method values and
// method expressions use the same wrapper names as calls.
package main

import "fmt"

type Kw interface {
	new() int
	delete(int) int
	class() string
	operator() string
	template(a ...int) int
	this() *int
	namespace() (int, int)
	gp_x() int
	size() int
	data() string
}

type impl struct{ n int }

func (i impl) new() int              { return i.n + 1 }
func (i impl) delete(x int) int      { return i.n - x }
func (i impl) class() string         { return "class" }
func (i impl) operator() string      { return "op" }
func (i impl) template(a ...int) int { return len(a) + i.n }
func (i impl) this() *int            { return &i.n }
func (i impl) namespace() (int, int) { return i.n, -i.n }
func (i impl) gp_x() int             { return 7 }
func (i impl) size() int             { return 8 }
func (i impl) data() string          { return "data" }

func main() {
	var k Kw = impl{10}
	fn, fd, fc, fo, ft := k.new, k.delete, k.class, k.operator, k.template
	fth, fns := k.this, k.namespace
	fmt.Println(fn(), fd(3), fc(), fo(), ft(1, 2, 3), *fth())
	a, b := fns()
	fmt.Println(a, b, k.gp_x(), k.size(), k.data())
	e1, e2 := Kw.new, Kw.delete
	fmt.Println(e1(k), e2(k, 4), Kw.class(k), Kw.template(k), Kw.size(k))
}

// Output:
// 11 7 class op 13 10
// 10 -10 7 8 data
// 11 6 class 10 8
