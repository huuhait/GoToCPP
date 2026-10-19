// phase: 07

// fmt verbs on values of type parameters: %v %+v %#v %T %d %x %q %s %c and widths, for several type arguments.
package main

import "fmt"

type Level int
type Pt struct{ X, Y int }
type Tag string
type Opt struct {
	Name *string
	Tags []Tag
	Lv   Level
}

func (l Level) String() string { return [...]string{"low", "mid", "high"}[l] }

func Show[T any](x T) string {
	return fmt.Sprintf("%v|%+v|%#v|%T", x, x, x, x)
}

func Verbs[T any](x T, verbs ...string) {
	for _, v := range verbs {
		fmt.Printf("%s=%"+v[1:]+" ", v, x)
	}
	fmt.Println("end")
}

func Join[T any](xs []T) string {
	s := ""
	for i, x := range xs {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprint(x)
	}
	return s
}

func Pad[T any](x T) string { return fmt.Sprintf("[%6v][%-6v][%06v]", x, x, x) }

func main() {
	fmt.Println(Show(42))
	fmt.Println(Show(int8(-5)))
	fmt.Println(Show(3.5))
	fmt.Println(Show("hi"))
	fmt.Println(Show(Tag("t")))
	fmt.Println(Show(Level(2)))
	fmt.Println(Show(Pt{1, 2}))
	fmt.Println(Show(&Pt{1, 2}) != "")
	fmt.Println(Show([]Tag{"a"}))
	fmt.Println(Show(map[string]Level{"k": 1}))
	fmt.Println(Show([2]bool{true}))
	fmt.Println(Show(Opt{Tags: []Tag{"x"}, Lv: 1}))
	fmt.Println(Show[any](nil))
	fmt.Println(Show[error](nil))
	fmt.Println(Show(struct{ A any }{3}))
	Verbs(255, "%d", "%x", "%X", "%o", "%b", "%c", "%q", "%U", "%e", "%s")
	Verbs("héllo", "%s", "%q", "%x", "%v", "%10s", "%-10s|", "%d")
	Verbs(3.14159, "%v", "%g", "%f", "%.2f", "%8.3f", "%e", "%d")
	Verbs(Level(1), "%v", "%d", "%s", "%q", "%x")
	Verbs(true, "%v", "%t", "%d")
	fmt.Println(Join([]Level{0, 1, 2}), Join([]float64{1, 2.5}), Join([]any{nil, 1, "a"}), Join([]Pt{{1, 2}}))
	fmt.Println(Pad(5), Pad("ab"), Pad(Level(1)), Pad(1.5))
}

// Output:
// 42|42|42|int
// -5|-5|-5|int8
// 3.5|3.5|3.5|float64
// hi|hi|"hi"|string
// t|t|"t"|main.Tag
// high|high|2|main.Level
// {1 2}|{X:1 Y:2}|main.Pt{X:1, Y:2}|main.Pt
// true
// [a]|[a]|[]main.Tag{"a"}|[]main.Tag
// map[k:mid]|map[k:mid]|map[string]main.Level{"k":1}|map[string]main.Level
// [true false]|[true false]|[2]bool{true, false}|[2]bool
// {<nil> [x] mid}|{Name:<nil> Tags:[x] Lv:mid}|main.Opt{Name:(*string)(nil), Tags:[]main.Tag{"x"}, Lv:1}|main.Opt
// <nil>|<nil>|<nil>|<nil>
// <nil>|<nil>|<nil>|<nil>
// {3}|{A:3}|struct { A interface {} }{A:3}|struct { A interface {} }
// %d=255 %x=ff %X=FF %o=377 %b=11111111 %c=ÿ %q='ÿ' %U=U+00FF %e=%!e(int=255) %s=%!s(int=255) end
// %s=héllo %q="héllo" %x=68c3a96c6c6f %v=héllo %10s=     héllo %-10s|=héllo     | %d=%!d(string=héllo) end
// %v=3.14159 %g=3.14159 %f=3.141590 %.2f=3.14 %8.3f=   3.142 %e=3.141590e+00 %d=%!d(float64=3.14159) end
// %v=mid %d=1 %s=mid %q="mid" %x=6d6964 end
// %v=true %t=true %d=%!d(bool=true) end
// low,mid,high 1,2.5 <nil>,1,a {1 2}
// [     5][5     ][000005] [    ab][ab    ][0000ab] [   mid][mid   ][000mid] [   1.5][1.5   ][0001.5]
