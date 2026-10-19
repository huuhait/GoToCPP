// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Addr struct {
	Street string
	Zip    int
}
type Person struct {
	Name  string
	Home  Addr
	Work  *Addr
	Other []Addr
}

func main() {
	p := Person{Name: "ann", Work: &Addr{"w", 1}, Other: make([]Addr, 2)}
	v := reflect.ValueOf(&p).Elem()
	v.FieldByName("Home").FieldByName("Zip").SetInt(12345)
	v.FieldByName("Work").Elem().FieldByName("Street").SetString("changed")
	v.FieldByName("Other").Index(1).FieldByName("Street").SetString("o1")
	fmt.Printf("%+v %+v %+v\n", p.Home, *p.Work, p.Other)
	home := v.FieldByName("Home")
	home.Set(reflect.ValueOf(Addr{"new", 9}))
	fmt.Println(p.Home, home.Addr().Interface().(*Addr).Zip)
	home.Addr().Interface().(*Addr).Zip = 77
	fmt.Println(p.Home.Zip)
	w := v.FieldByName("Work")
	w.Set(reflect.Zero(w.Type()))
	fmt.Println(p.Work == nil, w.IsNil())
}

// Output:
// {Street: Zip:12345} {Street:changed Zip:1} [{Street: Zip:0} {Street:o1 Zip:0}]
// {new 9} 9
// 77
// true true
