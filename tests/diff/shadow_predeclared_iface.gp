// phase: 05

// Redefining error, any, nil, true, len and int: interfaces and type assertions use the new meanings.
package main

import "fmt"

type error interface{ Foo() string }
type any = int
type string2 = string
type T struct{}

func (T) Foo() string2 { return "foo" }
func (T) Error() string2 { return "T-error" }

type fmtT struct{ fmt int }
type stringer interface{ String() string2 }

func (f fmtT) String() string2 { return "fmtT" }

func nilf() error { return nil }

func main() {
	var e error = T{}
	fmt.Println(e.Foo(), e == nil, nilf() == nil)
	var a any = 5
	fmt.Println(a + 1)
	len := func(x int) int { return x * 2 }
	fmt.Println(len(4))
	true := false
	fmt.Println(true, !true)
	nil := 3
	fmt.Println(nil + 1)
	var x interface{} = T{}
	if er, ok := x.(error); ok {
		fmt.Println("shadowed error iface:", er.Foo())
	}
	_, isStd := x.(interface{ Error() string })
	fmt.Println(isStd)
	f := fmtT{7}
	var s stringer = f
	fmt.Println(f.fmt, s.String(), s)
	{
		int := "shadow"
		string := 3
		fmt.Println(int, string)
	}
	type T struct{ A int }
	var inner any2
	inner = T{1}
	fmt.Printf("%T %v\n", inner, inner)
}

type any2 = interface{}
