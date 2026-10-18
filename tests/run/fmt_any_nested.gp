// phase: 05

// Interfaces inside composite values: []any, map[string]any, struct fields of interface type,
// nested any, nil interfaces print <nil>; %#v shows the dynamic types.
package main

import "fmt"

type S struct {
	A any
	B error
	C fmt.Stringer
	D []any
	E map[string]any
}

type Str string

func (s Str) String() string { return "str:" + string(s) }

func main() {
	s := S{1, nil, Str("c"), []any{1, "a", nil, []any{2}}, map[string]any{"k": []int{1}, "n": nil}}
	fmt.Printf("%v\n%+v\n%#v\n", s, s, s)
	fmt.Println(S{})
	fmt.Printf("%v %+v %#v\n", S{}, S{}, S{})
	var a any = s
	fmt.Printf("%v %T\n", a, a)
	var b any = &s
	fmt.Println(fmt.Sprintf("%v", b) == fmt.Sprintf("&%v", s))
	var c any = []any{a}
	fmt.Println(c)
	var d any = map[any]any{1: "a", 2: []any{3}}
	fmt.Println(d)
	fmt.Printf("%v %v\n", [2]any{}, [2]error{})
	fmt.Printf("%#v %#v\n", [2]any{}, [2]error{})
	fmt.Printf("%d %s\n", []any{1, "a", 2.5, nil}, []any{1, "a", 2.5, nil})
}

// Output:
// {1 <nil> str:c [1 a <nil> [2]] map[k:[1] n:<nil>]}
// {A:1 B:<nil> C:str:c D:[1 a <nil> [2]] E:map[k:[1] n:<nil>]}
// main.S{A:1, B:error(nil), C:"c", D:[]interface {}{1, "a", interface {}(nil), []interface {}{2}}, E:map[string]interface {}{"k":[]int{1}, "n":interface {}(nil)}}
// {<nil> <nil> <nil> [] map[]}
// {<nil> <nil> <nil> [] map[]} {A:<nil> B:<nil> C:<nil> D:[] E:map[]} main.S{A:interface {}(nil), B:error(nil), C:fmt.Stringer(nil), D:[]interface {}(nil), E:map[string]interface {}(nil)}
// {1 <nil> str:c [1 a <nil> [2]] map[k:[1] n:<nil>]} main.S
// true
// [{1 <nil> str:c [1 a <nil> [2]] map[k:[1] n:<nil>]}]
// map[1:a 2:[3]]
// [<nil> <nil>] [<nil> <nil>]
// [2]interface {}{interface {}(nil), interface {}(nil)} [2]error{error(nil), error(nil)}
// [1 %!d(string=a) %!d(float64=2.5) <nil>] [%!s(int=1) a %!s(float64=2.5) <nil>]
