// phase: 05

// Slices, arrays and maps of values with String and Error methods: each element is printed
// through its method (also pointers to such values); nil elements.
package main

import "fmt"

type Name string

func (n Name) String() string { return "<" + string(n) + ">" }

type Code int

func (c Code) Error() string { return fmt.Sprintf("code %d", int(c)) }

type PS struct{ n int }

func (p *PS) String() string { return fmt.Sprintf("ps%d", p.n) }

type VS struct{ n int }

func (v VS) String() string { return fmt.Sprintf("vs%d", v.n) }

func main() {
	fmt.Println([]Name{"a", "b"}, [2]Code{1, 2}, map[Name]Code{"k": 3, "a": 4})
	fmt.Printf("%v %s %q %x %d\n", []Name{"a"}, []Name{"b"}, []Name{"c"}, []Name{"d"}, []Name{"e"})
	fmt.Printf("%v %s %q %d\n", []Code{1}, []Code{2}, []Code{3}, []Code{4})
	fmt.Printf("%v %+v %#v\n", []Name{"a"}, []Name{"a"}, []Name{"a"})
	p1, p2 := &PS{1}, &PS{2}
	fmt.Println([]*PS{p1, p2}, []PS{{3}}, []VS{{4}}, []*VS{{5}})
	var np *PS
	var nv *VS
	fmt.Println([]*PS{nil, p1}, np, nv, []*VS{nv})
	fmt.Printf("%v %v\n", [2]*PS{p1, nil}, map[string]*PS{"k": p2, "n": nil})
	fmt.Printf("%v\n", []any{Name("x"), Code(2), p1, VS{6}, 7, nil, "s"})
	fmt.Printf("%v\n", []error{Code(1), nil, Code(2)})
	fmt.Printf("%v\n", []fmt.Stringer{Name("s"), nil, VS{1}})
	fmt.Printf("%s\n", []any{Name("x"), Code(2), 7})
}

// Output:
// [<a> <b>] [code 1 code 2] map[<a>:code 4 <k>:code 3]
// [<a>] [<b>] ["<c>"] [3c643e] [%!d(main.Name=e)]
// [code 1] [code 2] ["code 3"] [4]
// [<a>] [<a>] []main.Name{"a"}
// [ps1 ps2] [{3}] [vs4] [vs5]
// [<nil> ps1] <nil> <nil> [<nil>]
// [ps1 <nil>] map[k:ps2 n:<nil>]
// [<x> code 2 ps1 vs6 7 <nil> s]
// [code 1 <nil> code 2]
// [<s> <nil> vs1]
// [<x> code 2 %!s(int=7)]
