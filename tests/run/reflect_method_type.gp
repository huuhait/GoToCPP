// phase: 09

// reflect.Type.Method(i).Type (and MethodByName) is the func type of the method: the receiver
// first for a concrete type (value and pointer receivers, promoted methods of embedded structs and
// pointers, instances of a generic type, named func and slice types, types of the standard
// library), no receiver for an interface (named, unnamed, with an unexported method, the runtime's
// error and fmt interfaces); the types are real Go's (a method expression's type is the same).
package main

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

type P struct{ N int }

func (p P) String() string                             { return fmt.Sprint(p.N) }
func (p *P) Set(n int)                                 { p.N = n }
func (p P) Pair(a, b int) (int, error)                 { return a + b, nil }
func (p P) Vari(f string, xs ...any) []string          { return nil }
func (p *P) Funcy(f func(int) bool) map[string][]P     { return nil }
func (p P) Chans(c <-chan int, d chan<- *P) chan P     { return nil }
func (p P) Anon(x struct{ A, B int }) interface{ M() } { return nil }
func (p P) Iface(r io.Reader, e error) fmt.Stringer    { return nil }
func (p P) Arr(a [3]byte, r []rune) [2]*P              { return [2]*P{} }
func (p P) unexported()                                {}

type MyInt int

func (m MyInt) Double() MyInt { return m * 2 }

type Inner struct{}

func (Inner) Hello(s string) string { return s }
func (*Inner) Bye()                 {}

type Outer struct {
	Inner
	X int
}

type OuterP struct {
	*Inner
}

type Box[T any] struct{ v T }

func (b Box[T]) Get() T                 { return b.v }
func (b *Box[T]) Put(v T)               { b.v = v }
func (b Box[T]) Map(f func(T) T) Box[T] { return Box[T]{f(b.v)} }

type Shape interface {
	Area() float64
	Scale(f float64) Shape
	unexp(int)
}

type Fn func(int) int

func (f Fn) Call(x int) int { return f(x) }

type Lst []int

func (l Lst) Len() int { return len(l) }

func dump(t reflect.Type) {
	fmt.Println(t.String(), t.NumMethod())
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		fmt.Printf("  %d %s %q %v", m.Index, m.Name, m.PkgPath, m.Type)
		if m.Type != nil {
			fmt.Printf(" kind=%v in=%d out=%d variadic=%v", m.Type.Kind(), m.Type.NumIn(), m.Type.NumOut(), m.Type.IsVariadic())
			if m.Type.NumIn() > 0 {
				fmt.Printf(" in0=%v", m.Type.In(0))
			}
		}
		fmt.Println()
	}
}

func main() {
	dump(reflect.TypeOf(P{}))
	dump(reflect.TypeOf(&P{}))
	dump(reflect.TypeOf(MyInt(0)))
	dump(reflect.TypeOf(Outer{}))
	dump(reflect.TypeOf(&Outer{}))
	dump(reflect.TypeOf(OuterP{}))
	dump(reflect.TypeOf(Box[int]{}))
	dump(reflect.TypeOf(&Box[string]{}))
	dump(reflect.TypeOf((*Shape)(nil)).Elem())
	dump(reflect.TypeOf((*fmt.Stringer)(nil)).Elem())
	dump(reflect.TypeOf((*error)(nil)).Elem())
	dump(reflect.TypeOf((*io.ReadWriteCloser)(nil)).Elem())
	dump(reflect.TypeOf((*fmt.State)(nil)).Elem())
	dump(reflect.TypeOf((*fmt.Formatter)(nil)).Elem())
	dump(reflect.TypeOf((*fmt.ScanState)(nil)).Elem())
	dump(reflect.TypeOf((*fmt.Scanner)(nil)).Elem())
	dump(reflect.TypeOf((*interface{ Get(map[string]int) error })(nil)).Elem())
	dump(reflect.TypeOf(Fn(nil)))
	dump(reflect.TypeOf(Lst(nil)))
	dump(reflect.TypeOf(strings.NewReader("")))
	dump(reflect.TypeOf(errors.New("x")))
	dump(reflect.TypeOf(struct{ Inner }{}))
	m, ok := reflect.TypeOf(P{}).MethodByName("Pair")
	fmt.Println(m.Type, ok, m.Type == reflect.TypeOf(P.Pair), m.Type == reflect.TypeOf(func(P, int, int) (int, error) { return 0, nil }))
	m2, _ := reflect.TypeOf(&P{}).MethodByName("Set")
	fmt.Println(m2.Type == reflect.TypeOf((*P).Set))
	m3, _ := reflect.TypeOf((*Shape)(nil)).Elem().MethodByName("Area")
	fmt.Println(m3.Type == reflect.TypeOf(func() float64 { return 0 }), m3.Type.Name() == "")
}

// Output:
// main.P 7
//   0 Anon "" func(main.P, struct { A int; B int }) interface { M() } kind=func in=2 out=1 variadic=false in0=main.P
//   1 Arr "" func(main.P, [3]uint8, []int32) [2]*main.P kind=func in=3 out=1 variadic=false in0=main.P
//   2 Chans "" func(main.P, <-chan int, chan<- *main.P) chan main.P kind=func in=3 out=1 variadic=false in0=main.P
//   3 Iface "" func(main.P, io.Reader, error) fmt.Stringer kind=func in=3 out=1 variadic=false in0=main.P
//   4 Pair "" func(main.P, int, int) (int, error) kind=func in=3 out=2 variadic=false in0=main.P
//   5 String "" func(main.P) string kind=func in=1 out=1 variadic=false in0=main.P
//   6 Vari "" func(main.P, string, ...interface {}) []string kind=func in=3 out=1 variadic=true in0=main.P
// *main.P 9
//   0 Anon "" func(*main.P, struct { A int; B int }) interface { M() } kind=func in=2 out=1 variadic=false in0=*main.P
//   1 Arr "" func(*main.P, [3]uint8, []int32) [2]*main.P kind=func in=3 out=1 variadic=false in0=*main.P
//   2 Chans "" func(*main.P, <-chan int, chan<- *main.P) chan main.P kind=func in=3 out=1 variadic=false in0=*main.P
//   3 Funcy "" func(*main.P, func(int) bool) map[string][]main.P kind=func in=2 out=1 variadic=false in0=*main.P
//   4 Iface "" func(*main.P, io.Reader, error) fmt.Stringer kind=func in=3 out=1 variadic=false in0=*main.P
//   5 Pair "" func(*main.P, int, int) (int, error) kind=func in=3 out=2 variadic=false in0=*main.P
//   6 Set "" func(*main.P, int) kind=func in=2 out=0 variadic=false in0=*main.P
//   7 String "" func(*main.P) string kind=func in=1 out=1 variadic=false in0=*main.P
//   8 Vari "" func(*main.P, string, ...interface {}) []string kind=func in=3 out=1 variadic=true in0=*main.P
// main.MyInt 1
//   0 Double "" func(main.MyInt) main.MyInt kind=func in=1 out=1 variadic=false in0=main.MyInt
// main.Outer 1
//   0 Hello "" func(main.Outer, string) string kind=func in=2 out=1 variadic=false in0=main.Outer
// *main.Outer 2
//   0 Bye "" func(*main.Outer) kind=func in=1 out=0 variadic=false in0=*main.Outer
//   1 Hello "" func(*main.Outer, string) string kind=func in=2 out=1 variadic=false in0=*main.Outer
// main.OuterP 2
//   0 Bye "" func(main.OuterP) kind=func in=1 out=0 variadic=false in0=main.OuterP
//   1 Hello "" func(main.OuterP, string) string kind=func in=2 out=1 variadic=false in0=main.OuterP
// main.Box[int] 2
//   0 Get "" func(main.Box[int]) int kind=func in=1 out=1 variadic=false in0=main.Box[int]
//   1 Map "" func(main.Box[int], func(int) int) main.Box[int] kind=func in=2 out=1 variadic=false in0=main.Box[int]
// *main.Box[string] 3
//   0 Get "" func(*main.Box[string]) string kind=func in=1 out=1 variadic=false in0=*main.Box[string]
//   1 Map "" func(*main.Box[string], func(string) string) main.Box[string] kind=func in=2 out=1 variadic=false in0=*main.Box[string]
//   2 Put "" func(*main.Box[string], string) kind=func in=2 out=0 variadic=false in0=*main.Box[string]
// main.Shape 3
//   0 Area "" func() float64 kind=func in=0 out=1 variadic=false
//   1 Scale "" func(float64) main.Shape kind=func in=1 out=1 variadic=false in0=float64
//   2 unexp "main" func(int) kind=func in=1 out=0 variadic=false in0=int
// fmt.Stringer 1
//   0 String "" func() string kind=func in=0 out=1 variadic=false
// error 1
//   0 Error "" func() string kind=func in=0 out=1 variadic=false
// io.ReadWriteCloser 3
//   0 Close "" func() error kind=func in=0 out=1 variadic=false
//   1 Read "" func([]uint8) (int, error) kind=func in=1 out=2 variadic=false in0=[]uint8
//   2 Write "" func([]uint8) (int, error) kind=func in=1 out=2 variadic=false in0=[]uint8
// fmt.State 4
//   0 Flag "" func(int) bool kind=func in=1 out=1 variadic=false in0=int
//   1 Precision "" func() (int, bool) kind=func in=0 out=2 variadic=false
//   2 Width "" func() (int, bool) kind=func in=0 out=2 variadic=false
//   3 Write "" func([]uint8) (int, error) kind=func in=1 out=2 variadic=false in0=[]uint8
// fmt.Formatter 1
//   0 Format "" func(fmt.State, int32) kind=func in=2 out=0 variadic=false in0=fmt.State
// fmt.ScanState 6
//   0 Read "" func([]uint8) (int, error) kind=func in=1 out=2 variadic=false in0=[]uint8
//   1 ReadRune "" func() (int32, int, error) kind=func in=0 out=3 variadic=false
//   2 SkipSpace "" func() kind=func in=0 out=0 variadic=false
//   3 Token "" func(bool, func(int32) bool) ([]uint8, error) kind=func in=2 out=2 variadic=false in0=bool
//   4 UnreadRune "" func() error kind=func in=0 out=1 variadic=false
//   5 Width "" func() (int, bool) kind=func in=0 out=2 variadic=false
// fmt.Scanner 1
//   0 Scan "" func(fmt.ScanState, int32) error kind=func in=2 out=1 variadic=false in0=fmt.ScanState
// interface { Get(map[string]int) error } 1
//   0 Get "" func(map[string]int) error kind=func in=1 out=1 variadic=false in0=map[string]int
// main.Fn 1
//   0 Call "" func(main.Fn, int) int kind=func in=2 out=1 variadic=false in0=main.Fn
// main.Lst 1
//   0 Len "" func(main.Lst) int kind=func in=1 out=1 variadic=false in0=main.Lst
// *strings.Reader 11
//   0 Len "" func(*strings.Reader) int kind=func in=1 out=1 variadic=false in0=*strings.Reader
//   1 Read "" func(*strings.Reader, []uint8) (int, error) kind=func in=2 out=2 variadic=false in0=*strings.Reader
//   2 ReadAt "" func(*strings.Reader, []uint8, int64) (int, error) kind=func in=3 out=2 variadic=false in0=*strings.Reader
//   3 ReadByte "" func(*strings.Reader) (uint8, error) kind=func in=1 out=2 variadic=false in0=*strings.Reader
//   4 ReadRune "" func(*strings.Reader) (int32, int, error) kind=func in=1 out=3 variadic=false in0=*strings.Reader
//   5 Reset "" func(*strings.Reader, string) kind=func in=2 out=0 variadic=false in0=*strings.Reader
//   6 Seek "" func(*strings.Reader, int64, int) (int64, error) kind=func in=3 out=2 variadic=false in0=*strings.Reader
//   7 Size "" func(*strings.Reader) int64 kind=func in=1 out=1 variadic=false in0=*strings.Reader
//   8 UnreadByte "" func(*strings.Reader) error kind=func in=1 out=1 variadic=false in0=*strings.Reader
//   9 UnreadRune "" func(*strings.Reader) error kind=func in=1 out=1 variadic=false in0=*strings.Reader
//   10 WriteTo "" func(*strings.Reader, io.Writer) (int64, error) kind=func in=2 out=2 variadic=false in0=*strings.Reader
// *errors.errorString 1
//   0 Error "" func(*errors.errorString) string kind=func in=1 out=1 variadic=false in0=*errors.errorString
// struct { main.Inner } 1
//   0 Hello "" func(struct { main.Inner }, string) string kind=func in=2 out=1 variadic=false in0=struct { main.Inner }
// func(main.P, int, int) (int, error) true true true
// true
// true true
