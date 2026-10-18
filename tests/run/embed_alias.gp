// phase: 05

// An embedded alias: the field is named after the alias, and an alias of a pointer type embeds
// the pointer.
package main

import "fmt"

type A struct{ x int }

func (a A) Hello() string { return fmt.Sprint("hello ", a.x) }
func (a *A) Set(v int)    { a.x = v }

type Alias = A
type P = *A

type E struct {
	Alias
	k int
}

type UsesP struct {
	P
	k int
}

func main() {
	e := E{Alias{5}, 1}
	e.Set(6)
	fmt.Println(e.Alias, e.Alias.x, e.x, e.Hello(), e)
	u := UsesP{&A{3}, 1}
	u.Set(9)
	fmt.Println(u.Hello(), u.x, u.P.x, *u.P)
}

// Output:
// {6} 6 6 hello 6 {{6} 1}
// hello 9 9 9 {9}
