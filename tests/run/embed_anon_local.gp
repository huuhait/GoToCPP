// phase: 05

// Embedding in an anonymous struct and in types declared in a function (moved to namespace
// scope in C++): promoted fields and methods work on both, also through a pointer.
package main

import "fmt"

type A struct{ x int }

func (a A) Hello() string { return fmt.Sprint("hello ", a.x) }
func (a *A) Set(v int)    { a.x = v }

func main() {
	anon := struct {
		A
		n int
	}{A{1}, 2}
	anon.Set(5)
	fmt.Println(anon.Hello(), anon, anon.x)
	type local struct {
		A
		z int
	}
	l := local{A{8}, 1}
	l.Set(10)
	fmt.Println(l.Hello(), l)
	type localP struct {
		*local
	}
	lp := localP{&l}
	lp.Set(11)
	fmt.Println(l.x, lp.z, lp.Hello())
	var same struct {
		A
		n int
	}
	same = anon
	same.Set(7)
	fmt.Println(anon.x, same.x)
}

// Output:
// hello 5 {{5} 2} 5
// hello 10 {{10} 1}
// 11 1 hello 11
// 5 7
