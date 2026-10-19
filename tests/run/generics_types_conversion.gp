// phase: 07

// Conversions between instances with identical underlying types, and to the underlying type.
package main

import "fmt"

type Pair[A, B any] struct {
	First  A
	Second B
}

type Vec[T any] []T

type Ints []int

type Celsius float64

type Temp[T ~float64] struct{ v T }

type Alias = Pair[int, string]

type Other struct {
	First  int
	Second string
}

func main() {
	p := Pair[int, string]{1, "a"}
	o := Other(p)
	back := Pair[int, string](o)
	fmt.Println(o, back == p, Alias(o))
	v := Vec[int]{1, 2, 3}
	fmt.Println(Ints(v), []int(v), Vec[int](Ints{4}))
	var al Alias = p
	fmt.Printf("%T %v\n", al, al == p)
	t := Temp[Celsius]{36.6}
	u := Temp[float64]{float64(t.v)}
	fmt.Println(t, u, Celsius(u.v))
	anon := struct {
		First  int
		Second string
	}{5, "z"}
	fmt.Println(Pair[int, string](anon), Other(anon))
}

// Output:
// {1 a} true {1 a}
// [1 2 3] [1 2 3] [4]
// main.Pair[int,string] true
// {36.6} {36.6} 36.6
// {5 z} {5 z}
