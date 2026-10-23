// phase: 07

// A generic interface declared inside a function, in a function and in a generic function: a value of a package-level type
// implements it, a local generic struct embeds it, and a type assertion tells the instances apart.
package main

import "fmt"

type Dog struct{ name string }

func (d Dog) Get() string { return d.name }

func Use[T interface{ Get() string }](x T) string { return x.Get() }

func G[T any](v T) {
	type Getter[R any] interface{ Get() R }
	type Named[R any] struct {
		Getter[R]
		tag T
	}
	var g Getter[string] = Dog{"rex"}
	fmt.Println(g.Get())
	n := Named[string]{Dog{"fido"}, v}
	fmt.Println(n.Get(), n.tag, Use(n))
	var a any = g
	_, ok := a.(Getter[string])
	_, ok2 := a.(Getter[int])
	fmt.Println(ok, ok2)
	fmt.Printf("%T %T\n", g, n)
}

func main() {
	type Getter[R any] interface{ Get() R }
	var g Getter[string] = Dog{"x"}
	fmt.Println(g.Get(), Use(g))
	G(1)
	G("s")
}

// Output:
// x x
// rex
// fido 1 fido
// true false
// main.Dog main.Named[int;string]
// rex
// fido s fido
// true false
// main.Dog main.Named[string;string]
