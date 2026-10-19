// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pub struct{ A int }
type T struct {
	Exp   int
	unexp int
	Pub
	pub  Pub
	Ptr  *int
	priv *int
}

func main() {
	n := 5
	x := T{1, 2, Pub{3}, Pub{4}, &n, &n}
	v := reflect.ValueOf(&x).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		fmt.Printf("%s canset=%v caninterface=%v canaddr=%v kind=%v\n", v.Type().Field(i).Name, f.CanSet(), f.CanInterface(), f.CanAddr(), f.Kind())
	}
	fmt.Println(v.Field(1).Int(), v.Field(3).Field(0).Int(), v.Field(3).Field(0).CanSet(), v.Field(2).Field(0).CanSet())
	fmt.Println(v.FieldByName("unexp").CanSet(), v.FieldByName("A").CanSet(), v.FieldByName("pub").FieldByName("A").CanSet())
	fmt.Println(v.Field(5).Elem().Int(), v.Field(5).Elem().CanSet(), v.Field(4).Elem().CanSet())
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		v.Field(1).Interface()
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		v.Field(1).SetInt(9)
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		v.Field(3).Field(0).SetInt(9)
	}()
	fmt.Println(x.unexp)
}

// Output:
// Exp canset=true caninterface=true canaddr=true kind=int
// unexp canset=false caninterface=false canaddr=true kind=int
// Pub canset=true caninterface=true canaddr=true kind=struct
// pub canset=false caninterface=false canaddr=true kind=struct
// Ptr canset=true caninterface=true canaddr=true kind=ptr
// priv canset=false caninterface=false canaddr=true kind=ptr
// 2 4 false true
// false true false
// 5 false true
// recovered: reflect.Value.Interface: cannot return value obtained from unexported field or method
// recovered: reflect: reflect.Value.SetInt using value obtained using unexported field
// recovered: reflect: reflect.Value.SetInt using value obtained using unexported field
// 2
