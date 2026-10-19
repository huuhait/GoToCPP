// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Base struct{ ID int }

func main() {
	vals := []any{
		struct{}{},
		struct{ A int }{},
		struct {
			A int `tag:"x"`
		}{},
		struct {
			A, B string
			c    []int
		}{},
		struct {
			Base
			Name string `json:"name,omitempty" xml:"n"`
		}{},
		struct {
			*Base
			F func(int) string
		}{},
		struct {
			X struct{ Y int }
		}{},
		struct {
			A int "plain tag"
		}{},
	}
	for _, v := range vals {
		t := reflect.TypeOf(v)
		fmt.Println(t, t.NumField())
	}
}

// Output:
// struct {} 0
// struct { A int } 1
// struct { A int "tag:\"x\"" } 1
// struct { A string; B string; c []int } 3
// struct { main.Base; Name string "json:\"name,omitempty\" xml:\"n\"" } 2
// struct { *main.Base; F func(int) string } 2
// struct { X struct { Y int } } 1
// struct { A int "plain tag" } 1
