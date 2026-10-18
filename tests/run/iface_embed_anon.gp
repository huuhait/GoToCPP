// phase: 05

// An anonymous struct type with an embedded field has the promoted methods in its method set,
// so a value of it converts to an interface; identical anonymous types have one descriptor.
package main

import "fmt"

type Getter interface{ Get() int }

type Base struct{ n int }

func (b Base) Get() int  { return b.n }
func (b *Base) Inc()     { b.n++ }

type Incer interface{ Inc() }

func main() {
	v := struct {
		Base
		tag string
	}{Base{1}, "v"}
	var g Getter = v
	fmt.Println(g.Get())
	p := &v
	var i Incer = p
	i.Inc()
	fmt.Println(v.n, g.Get())
	var a any = v
	_, ok := a.(Incer)
	fmt.Println(ok)
	a = p
	_, ok = a.(Incer)
	fmt.Println(ok)
	w := struct {
		Base
		tag string
	}{Base{9}, "w"}
	a = w
	_, same := a.(struct {
		Base
		tag string
	})
	fmt.Println(same, a.(Getter).Get())
	type local struct {
		*Base
		z int
	}
	var l Getter = local{&Base{4}, 1}
	fmt.Println(l.Get())
	var li Incer = local{&Base{4}, 1}
	li.Inc()
}

// Output:
// 1
// 2 1
// false
// true
// true 9
// 4
