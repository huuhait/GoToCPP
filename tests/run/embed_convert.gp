// phase: 05

// Conversions between struct types with embedded fields: a defined type with the same
// underlying struct has the promoted methods of the embedded fields but none that the original
// declared itself; struct tags are ignored in a conversion.
package main

import "fmt"

type Base struct{ n int }

func (b Base) Get() int  { return b.n }
func (b *Base) Inc()     { b.n++ }

type Outer struct {
	Base
	name string
}

func (o Outer) Own() string { return "own " + o.name }

type Copy Outer

type Tagged struct {
	Base `json:"base"`
	name string `json:"name"`
}

func main() {
	o := Outer{Base{1}, "o"}
	c := Copy(o)
	c.Inc()
	fmt.Println(o.n, c.n, c.Get(), c.name)
	o2 := Outer(c)
	fmt.Println(o2.Own(), o2.Get())
	t := Tagged(o)
	t.Inc()
	fmt.Println(t.Get(), t.n, o.n)
	pc := (*Copy)(&o)
	pc.Inc()
	fmt.Println(o.n, pc.Get())
	var anon struct {
		Base
		name string
	} = struct {
		Base
		name string
	}(o)
	anon.Inc()
	fmt.Println(anon.Get(), o.n)
	o = Outer(anon)
	fmt.Println(o.Own(), o.n)
}

// Output:
// 1 2 2 o
// own o 2
// 2 2 1
// 2 2
// 3 2
// own o 3
