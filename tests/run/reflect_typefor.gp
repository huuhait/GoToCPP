// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type S struct{ A int }

func main() {
	fmt.Println(reflect.TypeFor[int](), reflect.TypeFor[string](), reflect.TypeFor[[]S](), reflect.TypeFor[*S]())
	fmt.Println(reflect.TypeFor[error](), reflect.TypeFor[any](), reflect.TypeFor[fmt.Stringer]())
	fmt.Println(reflect.TypeFor[error]().Kind(), reflect.TypeFor[map[string]int]().Kind(), reflect.TypeFor[S]().Kind())
	fmt.Println(reflect.TypeFor[int]() == reflect.TypeOf(0), reflect.TypeFor[*S]() == reflect.TypeOf(&S{}))
}

// Output:
// int string []main.S *main.S
// error interface {} fmt.Stringer
// interface map struct
// true true
