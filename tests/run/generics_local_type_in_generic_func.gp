// phase: 07

// A generic type declared inside a generic function takes the function's type parameters as well as its own: Box[string] is a
// different type in G[int] and in G[bool], and %T writes the function's type arguments, a semicolon, then the type's own.
package main

import "fmt"

func G[T any](x T) any {
	type Box[U any] struct {
		u U
		t T
	}
	b := Box[string]{"s", x}
	fmt.Printf("%v %T\n", b, b)
	return b
}

func Two[T, V any](x T, y V) {
	type Box[U, W any] struct {
		u U
		t T
		w W
	}
	type loc struct{ v V }
	fmt.Printf("%T\n", Box[string, int]{})
	fmt.Printf("%T\n", &Box[string, Box[int, int]]{})
	fmt.Printf("%T\n", loc{})
}

func main() {
	a, b, c := G(3), G("x"), G(4)
	fmt.Println(a == b, a == c, b == G("x"))
	Two(3, "x")
	Two("x", 2.5)
}

// Output:
// {s 3} main.Box[int;string]
// {s x} main.Box[string;string]
// {s 4} main.Box[int;string]
// {s x} main.Box[string;string]
// false false true
// main.Box[int,string;string,int]
// *main.Box[int,string;string,main.Box[int,string;int,int]·2]
// main.loc[int,string]
// main.Box[string,float64;string,int]
// *main.Box[string,float64;string,main.Box[string,float64;int,int]·2]
// main.loc[string,float64]
