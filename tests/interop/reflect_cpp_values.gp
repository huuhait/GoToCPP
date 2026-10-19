// phase: 09

// reflect on C/C++ values (step 9.3, ADR-035): a foreign value has the Go kind of its shape (std::string is a
// string, C.int an int32, C.uchar a uint8, a class is a struct without fields, a C pointer is an
// unsafe.Pointer), and its type is a type of its own: not the Go type of the same size.
package main

import (
	"fmt"
	"reflect"

	"<string>"
	"<vector>"
)

type Box struct {
	N int
	S string
}

func main() {
	vals := []any{std.string("hello"), C.int(5), C.double(2.5), C.uchar(7), std.vector[int]{}, Box{1, "a"}}
	for _, v := range vals {
		t := reflect.TypeOf(v)
		fmt.Println(t.Kind(), t.Name() != "")
	}
	rv := reflect.ValueOf(any(std.string("hello")))
	fmt.Println(rv.Kind(), rv.String(), rv.Len())
	ri := reflect.ValueOf(any(C.int(5)))
	fmt.Println(ri.Int(), ri.Type() == reflect.TypeOf(any(C.int(0))), ri.Type() == reflect.TypeOf(int32(0)))
	fmt.Println(reflect.DeepEqual(any(C.int(5)), any(C.int(5))), reflect.DeepEqual(any(C.int(5)), any(C.int(6))), reflect.DeepEqual(any(C.int(5)), 5))
	var x C.int = 3
	fmt.Println(reflect.ValueOf(any(&x)).Kind())
	z := reflect.Zero(reflect.TypeOf(any(C.double(0))))
	fmt.Println(z.Float())
	fmt.Println(reflect.ValueOf(any(std.string("a"))).Interface().(std.string) == "a")
}

// Output:
// string true
// int32 true
// float64 true
// uint8 true
// struct true
// struct true
// string hello 5
// 5 true false
// true false false
// unsafe.Pointer
// 0
// true
