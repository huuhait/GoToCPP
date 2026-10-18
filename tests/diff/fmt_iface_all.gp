// phase: 05

// fmt of interface values: Stringer, error, GoStringer, Formatter, nested containers, nil, %T, width and flags.
package main

import "fmt"

type S struct{ n int }

func (s S) String() string { return fmt.Sprint("S<", s.n, ">") }

type PS struct{ n int }

func (p *PS) String() string { return fmt.Sprint("PS<", p.n, ">") }

type E struct{ n int }

func (e E) Error() string  { return "E-error" }
func (e E) String() string { return "E-string" }

type G struct{}

func (G) GoString() string { return "G!" }

type F struct{}

func (F) Format(st fmt.State, verb rune) {
	w, wok := st.Width()
	p, pok := st.Precision()
	fmt.Fprintf(st, "F[%c w=%d,%v p=%d,%v +%v -%v #%v 0%v]", verb, w, wok, p, pok, st.Flag('+'), st.Flag('-'), st.Flag('#'), st.Flag('0'))
}

type Inner struct {
	S  S
	PS *PS
	A  any
	E  error
}
type MyInt int

func (m MyInt) String() string { return "MyInt!" }

type Color int

const (
	Red Color = iota
	Green
)

func (c Color) String() string { return [...]string{"Red", "Green"}[c] }

func main() {
	var a any = S{1}
	var e error = E{2}
	fmt.Println(a, e, &S{3}, PS{4}, &PS{5})
	fmt.Printf("%v %s %d %+v %#v %T\n", a, a, a, a, a, a)
	fmt.Printf("%v %s %d %+v %#v %T\n", e, e, e, e, e, e)
	fmt.Printf("%v %+v %#v\n", G{}, G{}, G{})
	fmt.Printf("%v|%+8.3x|%-5s|%#q|%05d\n", F{}, F{}, F{}, F{}, F{})
	in := Inner{S{6}, &PS{7}, S{8}, E{9}}
	fmt.Printf("%v\n%+v\n", in, in)
	fmt.Println([]any{S{1}, &PS{2}, nil, E{3}, 4, "x", []int{1}, map[string]S{"k": {1}}})
	fmt.Println(map[any]any{"b": S{2}}, map[any]any{S{3}: nil})
	fmt.Println(MyInt(1), []MyInt{1, 2}, map[MyInt]MyInt{1: 2}, [2]MyInt{}, &[]MyInt{1})
	fmt.Printf("%d %v %x %s|%5v|%-7v|\n", MyInt(1), MyInt(1), MyInt(255), MyInt(1), Red, Green)
	fmt.Printf("%v %d %s %q %x\n", Green, Green, Green, Green, Green)
	var np *PS
	var ns *S
	var ne error
	fmt.Println(np, ns, ne, any(nil))
	fmt.Printf("%v %v %s %d\n", np, ns, ne, ne)
	fmt.Printf("%v %+v %#v\n", []error{nil, E{1}}, struct{ E error }{E{2}}, struct{ A any }{3})
	fmt.Printf("%T %T %T %T %T\n", nil, e, &e, []error{}, map[string]any{})
	fmt.Printf("%6.2f|%-8q|%+d|%x|%X|%o|%b|%c|%U\n", 3.14159, "hi", 5, "hello", []byte("hi"), 8, 5, 'A', 0x1F600)
	fmt.Printf("%!|%z|%d|%d %d\n", 1, 2)
	fmt.Printf("%[2]d %[1]d %*d %.*f\n", 1, 2, 5, 42, 2, 3.14159)
	fmt.Printf("%v %v\n", struct{}{}, [0]int{})
	fmt.Print("a", 1, 2, "b", "c", 3.5, nil, nil, '\n')
	fmt.Print("\n")
	fmt.Println(fmt.Sprint(), fmt.Sprint(nil), fmt.Sprintln("x", 1) == "x 1\n")
	type pt struct{ X, Y int }
	fmt.Printf("%v %+v %#v %T\n", pt{1, 2}, &pt{3, 4}, []pt{{5, 6}}, map[pt]string{})
	var ip *int
	x := 3
	ip2 := &x
	fmt.Println(ip, ip2 != nil, *ip2)
	fmt.Printf("%v %t %v\n", ip == nil, true, 'x')
}
