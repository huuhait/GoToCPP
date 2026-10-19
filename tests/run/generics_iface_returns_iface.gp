// phase: 07

// Generic functions that build, store and return interface values from type parameters.
package main

import (
	"errors"
	"fmt"
)

type Animal interface{ Sound() string }
type Dog struct{ name string }
type Cat struct{ lives int }

func (d Dog) Sound() string  { return d.name + ": woof" }
func (c *Cat) Sound() string { return fmt.Sprint("meow x", c.lives) }

func Zoo[T Animal](xs ...T) []Animal {
	out := make([]Animal, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

func Box[T any](x T) any { return x }

func Wrap[T any](x T, err error) (any, error) {
	if err != nil {
		return nil, fmt.Errorf("wrap: %w", err)
	}
	return x, nil
}

func Registry[K comparable, T Animal]() (func(K, T), func(K) Animal) {
	m := map[K]Animal{}
	return func(k K, v T) { m[k] = v }, func(k K) Animal { return m[k] }
}

func Pick[T Animal](cond bool, a, b T) Animal {
	if cond {
		return a
	}
	return b
}

func ErrOf[T error](e T) error { return e }

type MyErr struct{ code int }

func (e *MyErr) Error() string { return fmt.Sprint("code ", e.code) }

func main() {
	z := Zoo(Dog{"rex"}, Dog{"fido"})
	z2 := Zoo(&Cat{9}, &Cat{3})
	for _, a := range append(z, z2...) {
		fmt.Println(a.Sound())
	}
	fmt.Println(Box(1) == Box(1), Box("a") == Box("a"), Box(1) == Box(int8(1)), Box[any](nil) == nil, Box[error](nil) == nil, Box((*int)(nil)) == nil)
	fmt.Println(Wrap(5, nil))
	fmt.Println(Wrap("x", errors.New("bad")))
	set, get := Registry[string, Dog]()
	set("a", Dog{"x"})
	fmt.Println(get("a").Sound(), get("zz") == nil)
	set2, get2 := Registry[int, Animal]()
	set2(1, &Cat{1})
	fmt.Println(get2(1).Sound())
	fmt.Println(Pick(true, Dog{"a"}, Dog{"b"}).Sound(), Pick[Animal](false, Dog{"a"}, &Cat{2}).Sound())
	var nilErr *MyErr
	e1 := ErrOf(&MyErr{4})
	e2 := ErrOf(nilErr)
	e3 := ErrOf[error](nil)
	fmt.Println(e1, e2 == nil, e3 == nil, e2 != nil)
	var target *MyErr
	fmt.Println(errors.As(e1, &target), target.code)
}

// Output:
// rex: woof
// fido: woof
// meow x9
// meow x3
// true true false true true false
// 5 <nil>
// <nil> wrap: bad
// x: woof true
// meow x1
// a: woof meow x2
// code 4 false true true
// true 4
