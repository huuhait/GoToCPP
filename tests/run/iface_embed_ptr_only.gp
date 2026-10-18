// phase: 05

// A struct that embeds a type whose methods all have pointer receivers, and goes into an interface by
// value, has none of them in its method set (the table of *O has them, the table of O is empty):
// assertions to the interface fail for the value and succeed for the pointer, and the pointer sees the
// changes. With an embedded pointer the value has them too.
package main

import "fmt"

type Incer interface{ Inc() int }

type Counter struct{ n int }

func (c *Counter) Inc() int { c.n++; return c.n }

type ByValue struct {
	Counter
	tag string
}

type ByPointer struct {
	*Counter
	tag string
}

type Nested struct{ ByValue }

type Empty struct{}

type Holder struct {
	Empty
	Counter
}

func check(name string, x any) {
	i, ok := x.(Incer)
	if ok {
		fmt.Println(name, ok, i.Inc())
	} else {
		fmt.Println(name, ok)
	}
}

func main() {
	bv := ByValue{Counter{10}, "v"}
	check("ByValue", bv)
	check("*ByValue", &bv)
	check("ByValue again", bv)
	bp := ByPointer{&Counter{20}, "p"}
	check("ByPointer", bp)
	check("*ByPointer", &bp)
	check("Nested", Nested{bv})
	check("*Nested", &Nested{bv})
	check("Holder", Holder{})
	check("*Holder", &Holder{})
	check("Empty", Empty{})
	fmt.Println(bv.n, bp.n)
	switch v := any(&bv).(type) {
	case Incer:
		fmt.Println("switch Incer", v.Inc())
	}
	switch any(bv).(type) {
	case Incer:
		fmt.Println("wrong")
	case ByValue:
		fmt.Println("switch ByValue")
	}
}

// Output:
// ByValue false
// *ByValue true 11
// ByValue again false
// ByPointer true 21
// *ByPointer true 22
// Nested false
// *Nested true 12
// Holder false
// *Holder true 1
// Empty false
// 11 22
// switch Incer 12
// switch ByValue
