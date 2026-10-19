// bench: reflect_fields
// category: runtime
// size: 2500000
// smoke: 1000
// twin: no
// about: reflect.TypeOf and ValueOf: field count, names and integer fields of a struct
package main

import (
	"fmt"
	"reflect"
)

type point struct {
	X, Y, Z int
	Name    string
}

func main() {
	p := point{1, 2, 3, "p"}
	total := 0
	for i := 0; i < benchN; i++ {
		p.X = i
		t := reflect.TypeOf(p)
		v := reflect.ValueOf(p)
		for j := 0; j < t.NumField(); j++ {
			total += len(t.Field(j).Name)
			if v.Field(j).Kind() == reflect.Int {
				total += int(v.Field(j).Int())
			}
		}
	}
	fmt.Println(total)
}
