// phase: 09

// reflect.Type.Method(i).Type across packages: methods whose signatures name the types of other packages, instances of a generic type of another package (also with a type argument of a third package), an embedded struct of another package (promoted methods), named and unnamed interfaces of other packages; and a struct with an instance of another package's generic type by value and through a pointer that only its descriptor names (the instance used to be left undefined: "incomplete type", "undefined reference to gp_td_gp_inst_..."). The output is real Go's.
package main

import (
	"fmt"
	"reflect"

	"example.com/reflmt/a"
	"example.com/reflmt/b"
)

type Local struct{}

func (Local) Make(t a.T) (a.U, error) { return nil, nil }

func dump(t reflect.Type) {
	fmt.Println(t, t.NumMethod())
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		fmt.Println(" ", m.Name, m.PkgPath, m.Type)
	}
}

func main() {
	dump(reflect.TypeOf(a.T{}))
	dump(reflect.TypeOf(&a.T{}))
	dump(reflect.TypeOf(a.Box[int]{}))
	dump(reflect.TypeOf(&a.Box[b.W]{}))
	dump(reflect.TypeOf((*a.Getter)(nil)).Elem())
	dump(reflect.TypeOf(b.W{}))
	dump(reflect.TypeOf(&b.W{}))
	dump(reflect.TypeOf((*b.Mine)(nil)).Elem())
	var i interface{ Make(a.T) (a.U, error) } = Local{}
	dump(reflect.TypeOf(&i).Elem())
	dump(reflect.TypeOf(i))
	_ = a.Anon()
	for i := 0; i < reflect.TypeOf(b.W{}).NumField(); i++ {
		f := reflect.TypeOf(b.W{}).Field(i)
		fmt.Println(" field", f.Name, f.Type)
	}
	m, _ := reflect.TypeOf(&b.W{}).MethodByName("Iface")
	fmt.Println(m.Type.In(1) == reflect.TypeOf(&i).Elem())
}

// Output:
// a.T 2
//   Keyed  func(a.T, map[a.T]*a.U) [2]a.U
//   Show  func(a.T, a.U) string
// *a.T 3
//   Keyed  func(*a.T, map[a.T]*a.U) [2]a.U
//   Show  func(*a.T, a.U) string
//   Use  func(*a.T, interface { Get() a.T }) a.U
// a.Box[int] 2
//   Get  func(a.Box[int]) int
//   Pair  func(a.Box[int]) (a.Box[int], *a.Box[int])
// *a.Box[example.com/reflmt/b.W] 3
//   Each  func(*a.Box[example.com/reflmt/b.W], func(int, b.W) bool)
//   Get  func(*a.Box[example.com/reflmt/b.W]) b.W
//   Pair  func(*a.Box[example.com/reflmt/b.W]) (a.Box[example.com/reflmt/b.W], *a.Box[example.com/reflmt/b.W])
// a.Getter 2
//   Get  func() a.T
//   set example.com/reflmt/a func(a.U)
// b.W 3
//   Keyed  func(b.W, map[a.T]*a.U) [2]a.U
//   Show  func(b.W, a.U) string
//   Wrap  func(b.W, a.T, ...a.U) a.Box[example.com/reflmt/a.T]
// *b.W 5
//   Iface  func(*b.W, interface { Make(a.T) (a.U, error) })
//   Keyed  func(*b.W, map[a.T]*a.U) [2]a.U
//   Show  func(*b.W, a.U) string
//   Use  func(*b.W, interface { Get() a.T }) a.U
//   Wrap  func(*b.W, a.T, ...a.U) a.Box[example.com/reflmt/a.T]
// b.Mine 1
//   Do  func(a.U) b.W
// interface { Make(a.T) (a.U, error) } 1
//   Make  func(a.T) (a.U, error)
// main.Local 1
//   Make  func(main.Local, a.T) (a.U, error)
//  field T a.T
//  field Extra a.Box[string]
//  field Ptr *a.Box[float64]
// true
