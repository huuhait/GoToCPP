// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Shape interface {
	Area() float64
	Perimeter() float64
	name() string
}
type Empty interface{}
type RW interface {
	Reader
	Writer
}
type Reader interface{ Read(p []byte) (int, error) }
type Writer interface{ Write(p []byte) (int, error) }

func main() {
	for _, v := range []any{(*Shape)(nil), (*Empty)(nil), (*RW)(nil), (*error)(nil), (*fmt.Stringer)(nil), (*any)(nil)} {
		t := reflect.TypeOf(v).Elem()
		fmt.Println(t, t.Kind(), t.NumMethod())
		for i := 0; i < t.NumMethod(); i++ {
			m := t.Method(i)
			fmt.Println("  ", m.Name, m.PkgPath != "")
		}
	}
}

// Output:
// main.Shape interface 3
//    Area false
//    Perimeter false
//    name true
// main.Empty interface 0
// main.RW interface 2
//    Read false
//    Write false
// error interface 1
//    Error false
// fmt.Stringer interface 1
//    String false
// interface {} interface 0
