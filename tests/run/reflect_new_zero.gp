// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pt struct{ X, Y int }
type S struct {
	A []int
	M map[string]int
	P *Pt
	I any
	F func()
}

func main() {
	for _, v := range []any{0, "", 0.0, false, Pt{}, []int(nil), map[string]int(nil), (*Pt)(nil), [3]int{}, S{}, complex(0, 0), uint8(0)} {
		t := reflect.TypeOf(v)
		z := reflect.Zero(t)
		n := reflect.New(t)
		fmt.Printf("%-18v zero=%v new=%v %v iszero=%v %v %v\n", t, z.Interface(), n.Elem().Interface(), n.Type(), z.IsZero(), z.CanSet(), n.Elem().CanSet())
	}
	p := reflect.New(reflect.TypeOf(Pt{}))
	p.Elem().Field(0).SetInt(3)
	pt := p.Interface().(*Pt)
	fmt.Println(*pt, p.Elem().Field(0).CanSet())
	pt.Y = 4
	fmt.Println(p.Elem().Field(1))
	n1, n2 := reflect.New(reflect.TypeOf(0)), reflect.New(reflect.TypeOf(0))
	fmt.Println(n1.Interface() == n2.Interface(), n1.Interface() == n1.Interface())
	z := reflect.Zero(reflect.TypeOf((*error)(nil)).Elem())
	fmt.Println(z.Kind(), z.IsNil(), z.IsValid())
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.Zero(nil)
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.New(nil)
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.Zero(reflect.TypeOf(1)).SetInt(5)
	}()
}

// Output:
// int                zero=0 new=0 *int iszero=true false true
// string             zero= new= *string iszero=true false true
// float64            zero=0 new=0 *float64 iszero=true false true
// bool               zero=false new=false *bool iszero=true false true
// main.Pt            zero={0 0} new={0 0} *main.Pt iszero=true false true
// []int              zero=[] new=[] *[]int iszero=true false true
// map[string]int     zero=map[] new=map[] *map[string]int iszero=true false true
// *main.Pt           zero=<nil> new=<nil> **main.Pt iszero=true false true
// [3]int             zero=[0 0 0] new=[0 0 0] *[3]int iszero=true false true
// main.S             zero={[] map[] <nil> <nil> <nil>} new={[] map[] <nil> <nil> <nil>} *main.S iszero=true false true
// complex128         zero=(0+0i) new=(0+0i) *complex128 iszero=true false true
// uint8              zero=0 new=0 *uint8 iszero=true false true
// {3 0} true
// 4
// false true
// interface true true
// recovered: reflect: Zero(nil)
// recovered: reflect: New(nil)
// recovered: reflect: reflect.Value.SetInt using unaddressable value
