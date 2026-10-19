// phase: 09

package main

import (
	"errors"
	"fmt"
	"reflect"
)

type Celsius float64
type Names []string
type Table map[string]int
type Point struct{ X, Y int }
type Fn func(int) int
type Ch chan int
type Ptr *Point
type Arr [4]byte
type Iface interface{ Foo() }

func main() {
	vals := []any{
		Celsius(1), Names{}, Table{}, Point{}, &Point{}, Fn(nil), Ch(nil), Ptr(nil), Arr{}, errors.New("e"),
		1.5, []string{}, map[string]int{}, struct{ X, Y int }{}, (func(int) int)(nil),
	}
	for _, v := range vals {
		t := reflect.TypeOf(v)
		fmt.Printf("%-24s kind=%-8v name=%-10q pkg=%q\n", t.String(), t.Kind(), t.Name(), t.PkgPath())
	}
	fmt.Println(reflect.TypeOf(Names{}) == reflect.TypeOf([]string{}))
	fmt.Println(reflect.TypeOf(Table{}).ConvertibleTo(reflect.TypeOf(map[string]int{})))
	fmt.Println(reflect.TypeOf(Names{}).Elem(), reflect.TypeOf(Table{}).Key(), reflect.TypeOf(Arr{}).Len())
	fmt.Println(reflect.TypeOf((*Iface)(nil)).Elem().Name(), reflect.TypeOf((*error)(nil)).Elem().Name(), reflect.TypeOf((*error)(nil)).Elem().PkgPath() == "")
}

// Output:
// main.Celsius             kind=float64  name="Celsius"  pkg="main"
// main.Names               kind=slice    name="Names"    pkg="main"
// main.Table               kind=map      name="Table"    pkg="main"
// main.Point               kind=struct   name="Point"    pkg="main"
// *main.Point              kind=ptr      name=""         pkg=""
// main.Fn                  kind=func     name="Fn"       pkg="main"
// main.Ch                  kind=chan     name="Ch"       pkg="main"
// main.Ptr                 kind=ptr      name="Ptr"      pkg="main"
// main.Arr                 kind=array    name="Arr"      pkg="main"
// *errors.errorString      kind=ptr      name=""         pkg=""
// float64                  kind=float64  name="float64"  pkg=""
// []string                 kind=slice    name=""         pkg=""
// map[string]int           kind=map      name=""         pkg=""
// struct { X int; Y int }  kind=struct   name=""         pkg=""
// func(int) int            kind=func     name=""         pkg=""
// false
// true
// string string 4
// Iface error true
