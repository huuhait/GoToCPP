// phase: 09

// A struct of package b holds instances of a generic type of package a (by value and through a pointer) that nothing but the struct's descriptor names: the descriptor of the instance is defined (it was left undefined: "incomplete type", "undefined reference to gp_td_gp_inst_...").
package main

import (
	"fmt"
	"reflect"

	"example.com/gtdesconly/b"
)

func main() {
	var w any = b.W{}
	t := reflect.TypeOf(w)
	for i := 0; i < t.NumField(); i++ {
		fmt.Println(t.Field(i).Name, t.Field(i).Type, t.Field(i).Type.Kind())
	}
	fmt.Printf("%+v\n", w)
}

// Output:
// Ptr *a.Box[string] ptr
// Val a.Box[int] struct
// {Ptr:<nil> Val:{v:0}}
