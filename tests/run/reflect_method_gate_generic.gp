// phase: 09

// reflect.Method reached only through a generic constraint (Method(int) R with R inferred as
// reflect.Method; the program never names the type): the method tables still carry the func
// types (the gate of ADR-035 item 11 looks at the types of expressions and at type arguments).
package main

import (
	"fmt"
	"reflect"
)

type P struct{ N int }

func (P) M(x int) string { return "" }

type methoder[R any] interface{ Method(int) R }

func get[T methoder[R], R any](t T) R { return t.Method(0) }

func show[T methoder[R], R any](t T) {
	fmt.Println(reflect.ValueOf(t.Method(0)).Field(2).Interface())
}

func main() {
	r := get(reflect.TypeOf(P{}))
	fmt.Println(reflect.ValueOf(r).FieldByName("Type").Interface())
	show(reflect.TypeOf(P{}))
}

// Output:
// func(main.P, int) string
// func(main.P, int) string
