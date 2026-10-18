// phase: 05

// A value with a String method prints through it for %v %s %q %x %X and by Print and Println, not
// for %d or %#v; value and pointer receivers; the method set decides.
package main

import "fmt"

type V int

func (v V) String() string { return fmt.Sprintf("V<%d>", int(v)) }

type P struct{ n int }

func (p *P) String() string { return fmt.Sprintf("P<%d>", p.n) }

type S struct{ a, b int }

func (s S) String() string { return fmt.Sprintf("S<%d,%d>", s.a, s.b) }

type Sl []int

func (s Sl) String() string { return fmt.Sprintf("Sl%d", len(s)) }

type M map[string]int

func (m M) String() string { return "M" }

type F func()

func (f F) String() string { return "F" }

type Str string

func (s Str) String() string { return "Str:" + string(s) }

type Fl float64

func (f Fl) String() string { return fmt.Sprintf("Fl:%.1f", float64(f)) }

type Wrong struct{}

func (Wrong) String(x int) string { return "wrong" }

type Wrong2 struct{}

func (Wrong2) String() int { return 1 }

type Field struct{ String string }

type FuncField struct{ String func() string }

func main() {
	for _, f := range []string{"%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%8v|", "%-8s|", "%.2v"} {
		fmt.Printf("%-6s[", f)
		fmt.Printf(f, V(3))
		fmt.Printf("]\n")
	}
	for _, f := range []string{"%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%8v|", "%-8s|", "%.2v"} {
		fmt.Printf("%-6s[", f)
		fmt.Printf(f, S{1, 2})
		fmt.Printf("]\n")
	}
	for _, f := range []string{"%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%8v|", "%-8s|", "%.2v"} {
		fmt.Printf("%-6s[", f)
		fmt.Printf(f, Sl{1, 2})
		fmt.Printf("]\n")
	}
	for _, f := range []string{"%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%8v|", "%-8s|", "%.2v"} {
		fmt.Printf("%-6s[", f)
		fmt.Printf(f, M{})
		fmt.Printf("]\n")
	}
	for _, f := range []string{"%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%8v|", "%-8s|", "%.2v"} {
		fmt.Printf("%-6s[", f)
		fmt.Printf(f, Str("q"))
		fmt.Printf("]\n")
	}
	for _, f := range []string{"%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%8v|", "%-8s|", "%.2v"} {
		fmt.Printf("%-6s[", f)
		fmt.Printf(f, Fl(2.5))
		fmt.Printf("]\n")
	}
	for _, f := range []string{"%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%8v|", "%-8s|", "%.2v"} {
		fmt.Printf("%-6s[", f)
		fmt.Printf(f, &S{3, 4})
		fmt.Printf("]\n")
	}
	for _, f := range []string{"%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%8v|", "%-8s|", "%.2v"} {
		fmt.Printf("%-6s[", f)
		fmt.Printf(f, P{5})
		fmt.Printf("]\n")
	}
	for _, f := range []string{"%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%8v|", "%-8s|", "%.2v"} {
		fmt.Printf("%-6s[", f)
		fmt.Printf(f, &P{6})
		fmt.Printf("]\n")
	}
	fmt.Println(V(1), S{2, 3}, &S{4, 5}, P{6}, &P{7}, Sl{1}, M{}, F(nil), Str("a"), Fl(1))
	fmt.Print(V(1), V(2), "x", V(3), "\n")
	fmt.Println(Wrong{}, Wrong2{}, Field{"f"}, FuncField{}.String == nil)
	var sp *P
	var vp *S
	fmt.Println(sp, vp)
	fmt.Printf("%v %s %d\n", sp, vp, sp)
	fmt.Printf("%v %v\n", []V{1, 2}, map[V]V{1: 2})
	fmt.Printf("%v %v\n", [1]S{{1, 2}}, struct{ X V }{9})
}

// Output:
// %v    [V<3>]
// %s    [V<3>]
// %d    [3]
// %x    [563c333e]
// %q    ["V<3>"]
// %#v   [3]
// %+v   [V<3>]
// %8v|  [    V<3>|]
// %-8s| [V<3>    |]
// %.2v  [V<]
// %v    [S<1,2>]
// %s    [S<1,2>]
// %d    [{1 2}]
// %x    [533c312c323e]
// %q    ["S<1,2>"]
// %#v   [main.S{a:1, b:2}]
// %+v   [S<1,2>]
// %8v|  [  S<1,2>|]
// %-8s| [S<1,2>  |]
// %.2v  [S<]
// %v    [Sl2]
// %s    [Sl2]
// %d    [[1 2]]
// %x    [536c32]
// %q    ["Sl2"]
// %#v   [main.Sl{1, 2}]
// %+v   [Sl2]
// %8v|  [     Sl2|]
// %-8s| [Sl2     |]
// %.2v  [Sl]
// %v    [M]
// %s    [M]
// %d    [map[]]
// %x    [4d]
// %q    ["M"]
// %#v   [main.M{}]
// %+v   [M]
// %8v|  [       M|]
// %-8s| [M       |]
// %.2v  [M]
// %v    [Str:q]
// %s    [Str:q]
// %d    [%!d(main.Str=q)]
// %x    [5374723a71]
// %q    ["Str:q"]
// %#v   ["q"]
// %+v   [Str:q]
// %8v|  [   Str:q|]
// %-8s| [Str:q   |]
// %.2v  [St]
// %v    [Fl:2.5]
// %s    [Fl:2.5]
// %d    [%!d(main.Fl=2.5)]
// %x    [466c3a322e35]
// %q    ["Fl:2.5"]
// %#v   [2.5]
// %+v   [Fl:2.5]
// %8v|  [  Fl:2.5|]
// %-8s| [Fl:2.5  |]
// %.2v  [Fl]
// %v    [S<3,4>]
// %s    [S<3,4>]
// %d    [&{3 4}]
// %x    [533c332c343e]
// %q    ["S<3,4>"]
// %#v   [&main.S{a:3, b:4}]
// %+v   [S<3,4>]
// %8v|  [  S<3,4>|]
// %-8s| [S<3,4>  |]
// %.2v  [S<]
// %v    [{5}]
// %s    [{%!s(int=5)}]
// %d    [{5}]
// %x    [{5}]
// %q    [{'\x05'}]
// %#v   [main.P{n:5}]
// %+v   [{n:5}]
// %8v|  [{       5}|]
// %-8s| [{%!s(int=5       )}|]
// %.2v  [{05}]
// %v    [P<6>]
// %s    [P<6>]
// %d    [&{6}]
// %x    [503c363e]
// %q    ["P<6>"]
// %#v   [&main.P{n:6}]
// %+v   [P<6>]
// %8v|  [    P<6>|]
// %-8s| [P<6>    |]
// %.2v  [P<]
// V<1> S<2,3> S<4,5> {6} P<7> Sl1 M F Str:a Fl:1.0
// V<1> V<2>xV<3>
// {} {} {f} true
// <nil> <nil>
// <nil> <nil> 0
// [V<1> V<2>] map[V<1>:V<2>]
// [S<1,2>] {V<9>}
