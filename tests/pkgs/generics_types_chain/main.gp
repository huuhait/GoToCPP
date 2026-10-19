// phase: 07

// Three packages that name the same instances: a declares Box[T], b and c use Box[b.Feet], main checks identity through assertions and %T.
package main

import (
	"fmt"

	"example.com/gtchain/a"
	"example.com/gtchain/b"
	"example.com/gtchain/c"
)

func main() {
	x := b.Boxed(3)
	y := c.Double(x)
	fmt.Println(x.Get(), y.Get(), y)
	fmt.Printf("%T %T\n", x, b.Nest(1))
	v := c.Any(5)
	bx, ok := v.(a.Box[b.Feet])
	_, wrong := v.(a.Box[int])
	fmt.Println(bx.Get(), ok, wrong)
	p, q := b.Pair(1, 2.5)
	fmt.Println(p, q, p.Get()+1, q.Get()*2)
	fmt.Println(x == a.Box[b.Feet]{3}, b.Nest(2).Get().Get())
	x.Set(9)
	fmt.Println(x)
}

// Output:
// 3 6 {6}
// a.Box[example.com/gtchain/b.Feet] a.Box[example.com/gtchain/a.Box[example.com/gtchain/b.Feet]]
// 5 true false
// {1} {2.5} 2 5
// true 2
// {9}
