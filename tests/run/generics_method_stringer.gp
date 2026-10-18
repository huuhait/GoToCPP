// phase: 07

// A constraint with a method: x.String() on type arguments that are values, pointers and defined types.
package main

import "fmt"

type Stringer interface{ String() string }

type Celsius float64

func (c Celsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }

type ID int

func (i ID) String() string { return fmt.Sprintf("#%d", int(i)) }

type Name struct{ first, last string }

func (n Name) String() string { return n.first + " " + n.last }

type Counter struct{ n int }

func (c *Counter) String() string { c.n++; return fmt.Sprint("counter ", c.n) }

func Join[T Stringer](xs []T, sep string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += sep
		}
		out += x.String()
	}
	return out
}

func Show[T Stringer](x T) string { return "<" + x.String() + ">" }

func Longest[T Stringer](xs ...T) (best T) {
	for _, x := range xs {
		if len(x.String()) > len(best.String()) {
			best = x
		}
	}
	return
}

func main() {
	fmt.Println(Join([]Celsius{36.6, -4}, ", "))
	fmt.Println(Join([]ID{1, 22, 333}, "|"))
	fmt.Println(Join([]Name{{"Ada", "Lovelace"}, {"Alan", "Turing"}}, "; "))
	c := &Counter{}
	fmt.Println(Show(c), Show(c), c.n)
	fmt.Println(Join([]*Counter{c, c}, "+"), c.n)
	fmt.Println(Show[Stringer](ID(5)), Show[fmt.Stringer](Celsius(1)))
	fmt.Println(Longest(Name{"a", "b"}, Name{"abc", "def"}, Name{"x", "y"}))
	fmt.Println(Longest[ID]())
}

// Output:
// 36.6°C, -4.0°C
// #1|#22|#333
// Ada Lovelace; Alan Turing
// <counter 1> <counter 2> 2
// counter 3+counter 4 4
// <#5> <1.0°C>
// abc def
// #0
