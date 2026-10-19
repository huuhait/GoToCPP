// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type T struct{}

func (T) Zeta() int      { return 0 }
func (T) Alpha() string  { return "" }
func (T) mid() bool      { return false }
func (T) Beta(int) error { return nil }
func (*T) PtrOnly()      {}
func (*T) another()      {}

type Celsius float64

func (c Celsius) String() string { return "c" }

func main() {
	t := reflect.TypeOf(T{})
	fmt.Println(t.NumMethod())
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		fmt.Println(i, m.Name, m.Index, m.IsExported(), m.PkgPath == "")
	}
	pt := reflect.TypeOf(&T{})
	fmt.Println(pt.NumMethod())
	for i := 0; i < pt.NumMethod(); i++ {
		fmt.Println(i, pt.Method(i).Name)
	}
	m, ok := t.MethodByName("Beta")
	fmt.Println(m.Name, m.Index, ok)
	_, ok = t.MethodByName("PtrOnly")
	fmt.Println(ok)
	_, ok = pt.MethodByName("PtrOnly")
	fmt.Println(ok)
	_, ok = t.MethodByName("mid")
	fmt.Println(ok)
	fmt.Println(reflect.TypeOf(Celsius(1)).NumMethod(), reflect.TypeOf(1).NumMethod(), reflect.TypeOf([]int{}).NumMethod())
	fmt.Println(reflect.ValueOf(T{}).NumMethod(), reflect.ValueOf(&T{}).NumMethod())
}

// Output:
// 3
// 0 Alpha 0 true true
// 1 Beta 1 true true
// 2 Zeta 2 true true
// 4
// 0 Alpha
// 1 Beta
// 2 PtrOnly
// 3 Zeta
// Beta 1 true
// false
// true
// false
// 1 0 0
// 3 4
