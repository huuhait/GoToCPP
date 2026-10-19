// phase: 09

package main

import (
	"errors"
	"fmt"
	"reflect"
)

type Shape interface{ Area() float64 }
type Named interface{ Name() string }
type Both interface {
	Shape
	Named
}
type Sq struct{ S float64 }

func (s Sq) Area() float64 { return s.S * s.S }
func (s Sq) Name() string  { return "sq" }

type Circ struct{ R float64 }

func (c *Circ) Area() float64 { return c.R }

type Str string

func (s Str) String() string { return string(s) }

func main() {
	shape := reflect.TypeOf((*Shape)(nil)).Elem()
	named := reflect.TypeOf((*Named)(nil)).Elem()
	both := reflect.TypeOf((*Both)(nil)).Elem()
	str := reflect.TypeOf((*fmt.Stringer)(nil)).Elem()
	errt := reflect.TypeOf((*error)(nil)).Elem()
	empty := reflect.TypeOf((*any)(nil)).Elem()
	types := []reflect.Type{reflect.TypeOf(Sq{}), reflect.TypeOf(&Sq{}), reflect.TypeOf(Circ{}), reflect.TypeOf(&Circ{}),
		reflect.TypeOf(Str("")), reflect.TypeOf(1), reflect.TypeOf(errors.New("x")), shape, both, named}
	for _, t := range types {
		fmt.Printf("%-12v shape=%v named=%v both=%v str=%v err=%v empty=%v\n", t, t.Implements(shape), t.Implements(named), t.Implements(both), t.Implements(str), t.Implements(errt), t.Implements(empty))
	}
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.TypeOf(1).Implements(reflect.TypeOf(1))
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.TypeOf(1).Implements(nil)
	}()
}

// Output:
// main.Sq      shape=true named=true both=true str=false err=false empty=true
// *main.Sq     shape=true named=true both=true str=false err=false empty=true
// main.Circ    shape=false named=false both=false str=false err=false empty=true
// *main.Circ   shape=true named=false both=false str=false err=false empty=true
// main.Str     shape=false named=false both=false str=true err=false empty=true
// int          shape=false named=false both=false str=false err=false empty=true
// *errors.errorString shape=false named=false both=false str=false err=true empty=true
// main.Shape   shape=true named=false both=false str=false err=false empty=true
// main.Both    shape=true named=true both=true str=false err=false empty=true
// main.Named   shape=false named=true both=false str=false err=false empty=true
// recovered: reflect: non-interface type passed to Type.Implements
// recovered: reflect: nil type passed to Type.Implements
