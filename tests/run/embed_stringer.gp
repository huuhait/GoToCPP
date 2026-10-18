// phase: 05

// fmt calls a promoted String or Error method (through the embedded struct, through an embedded
// pointer, and not through an unexported field), and prints embedded fields as fields.
package main

import "fmt"

type Base struct{ n int }

func (b Base) String() string { return fmt.Sprint("Base#", b.n) }

type Derived struct {
	Base
	extra int
}

type Over struct {
	Base
	extra int
}

func (o Over) String() string { return "Over" }

type PtrS struct {
	*Base
}

type boomErr struct{}

func (boomErr) Error() string { return "boom" }

type E struct{ boomErr }

type W struct {
	err boomErr
	n   int
}

func main() {
	d := Derived{Base{7}, 8}
	fmt.Println(d, &d, d.String())
	fmt.Printf("%v %+v %s\n", d, d, d)
	fmt.Println(Over{Base{1}, 2}, PtrS{&Base{3}})
	fmt.Println([]Derived{{Base{1}, 1}, {Base{2}, 2}}, map[string]Derived{"k": d})
	e := E{boomErr{}}
	fmt.Println(e, e.Error())
	fmt.Println(W{boomErr{}, 1}.n)
}


// Output:
// Base#7 Base#7 Base#7
// Base#7 Base#7 Base#7
// Over Base#3
// [Base#1 Base#2] map[k:Base#7]
// boom boom
// 1
