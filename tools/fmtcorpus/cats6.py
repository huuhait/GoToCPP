from lib import case
from cats3 import NORM

case("fmt_named_kinds",
     "Named types of every kind with String methods: slices, maps, arrays, funcs, pointers, channels-free; the method wins over the underlying value's formatting for %v %s, not for %d; nested in each other.",
     '''type IntList []int

func (l IntList) String() string { return fmt.Sprintf("IntList(%d)", len(l)) }

type Dict map[string]int

func (d Dict) String() string { return fmt.Sprintf("Dict(%d)", len(d)) }

type Triple [3]int

func (t Triple) String() string { return fmt.Sprintf("Triple%v", [3]int(t)) }

type Op func(int) int

func (o Op) String() string { return "Op" }

type Ref *int

type Wrapper struct {
	L IntList
	D Dict
	T Triple
	O Op
}

type Celsius float64

func (c Celsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }

type Temps []Celsius

type Grid [2][2]Celsius

type Index map[Celsius][]Celsius
''',
     '''	l := IntList{1, 2, 3}
	d := Dict{"a": 1}
	t := Triple{1, 2, 3}
	op := Op(func(x int) int { return x })
	fmt.Println(l, d, t, op)
	fmt.Printf("%v %s %d %q %x\\n", l, l, l, l, l)
	fmt.Printf("%v %s %d\\n", d, d, d)
	fmt.Printf("%v %s %d %v\\n", t, t, t, [3]int(t))
	fmt.Printf("%v %s %T\\n", op, op, op)
	fmt.Printf("%v %+v\\n", Wrapper{l, d, t, op}, Wrapper{l, d, t, op})
	fmt.Printf("%#v\\n", Wrapper{l, d, t, nil})
	fmt.Println(Temps{1, 2.55}, Grid{{1, 2}, {3, 4}}, Index{1: {2, 3}})
	fmt.Printf("%v %.2v %5.1f %d\\n", Temps{1.25}, Temps{1.25}, Temps{1.25}, Temps{1.25})
	var r Ref
	x := 3
	r = &x
	fmt.Println(r != nil, *r)
	fmt.Println([]IntList{{1}, nil}, map[string]Dict{"k": {}}, [1]Triple{})
	var np Ref
	fmt.Println(np)
''')

case("fmt_tuple_calls",
     "Print, Println, Sprint and Sprintln of a call with several results pass every result as an operand; Printf returns (n, err) which can be used; results of different kinds.",
     '''type S struct{ A int }

func two() (int, string) { return 1, "a" }

func three() (S, []int, error) { return S{2}, []int{3}, nil }

func four() (int64, uint8, float32, map[string]bool) {
	return 5, 6, 7.5, map[string]bool{"k": true}
}

func pair() (n int, err error) { return fmt.Println("in pair") }
''',
     '''	fmt.Println(two())
	fmt.Println(three())
	fmt.Println(four())
	fmt.Print(two())
	fmt.Print("\\n")
	s := fmt.Sprint(three())
	fmt.Println(s, len(fmt.Sprintln(four())))
	n, err := pair()
	fmt.Println(n, err)
	n, err = fmt.Printf("%d%s\\n", 1, "x")
	fmt.Println(n, err)
	if _, err := fmt.Println(); err != nil {
		fmt.Println("error")
	}
	fmt.Printf("%T\\n", err)
''')

case("fmt_defer_go",
     "fmt calls in defer statements and closures evaluate their operands when the defer runs (Go: when the defer statement runs); with values that change afterwards.",
     "type P struct{ X int }\n",
     '''	x := 1
	p := P{1}
	xs := []int{1}
	defer fmt.Println("deferred", x, p, xs)
	defer fmt.Printf("%d %v %v\\n", x, p, xs)
	defer func() { fmt.Println("closure", x, p, xs) }()
	x = 2
	p.X = 2
	xs[0] = 2
	xs = append(xs, 3)
	defer fmt.Print(x, " ", p, " ", xs, "\\n")
	fmt.Println("main", x, p, xs)
''')

case("fmt_funcs_values",
     "Func values print their code address (nil funcs print <nil>); %T names the func type; method values and closures; here only the shape of the output is checked.",
     NORM + "type T struct{}\n\nfunc (T) M(int) int { return 1 }\n\nfunc f(a int, b string) bool { return true }\n",
     '''	var nf func()
	g := func(x int) int { return x }
	fmt.Println(nf, norm(fmt.Sprint(f)) , norm(fmt.Sprint(g)), norm(fmt.Sprint(T{}.M)))
	fmt.Printf("%T %T %T %T\\n", nf, f, g, T{}.M)
	fmt.Printf("%v %t\\n", nf == nil, fmt.Sprint(f)[:2] == "0x")
	fmt.Println(norm(fmt.Sprintf("%v %+v %#v", f, f, f)))
	fmt.Println(norm(fmt.Sprint([]func(){nil, func() {}})), norm(fmt.Sprint(map[string]func(){"a": nil})))
	fmt.Println(norm(fmt.Sprint(struct{ F func() }{func() {}})))
	fmt.Printf("%d %s %x\\n", nf, nf, nf)
	fmt.Println(fmt.Sprintf("%p", f) == fmt.Sprintf("%v", f), fmt.Sprintf("%p", nf))
''')

case("fmt_big_values",
     "Large arrays and structs (more than 128 KiB) are printed from the heap copy that the emitter makes; a slice of a million elements; deep nesting.",
     "type Big struct {\n\tA [20000]int\n\tS string\n}\n",
     '''	var a [30000]int
	a[0], a[29999] = 1, 2
	s := fmt.Sprint(a)
	fmt.Println(len(s), s[:10], s[len(s)-10:])
	var b Big
	b.A[19999] = 7
	b.S = "big"
	t := fmt.Sprintf("%+v", b)
	fmt.Println(len(t), t[:12], t[len(t)-14:])
	pb := &b
	u := fmt.Sprint(pb)
	fmt.Println(len(u), u[:3])
	sl := make([]int, 1000000)
	sl[999999] = 5
	v := fmt.Sprint(sl)
	fmt.Println(len(v), v[len(v)-6:])
	nest := [][][]int{{{1}, {2, 3}}, {{}}, nil}
	fmt.Println(nest, fmt.Sprintf("%v|%d|%+v", nest, nest, nest))
	deep := []any{[]any{[]any{[]any{[]any{1, "x"}}}}}
	fmt.Println(deep, fmt.Sprintf("%#v", deep))
''')

case("fmt_runes_strings_misc",
     "Runes, bytes and strings in one place: %c %q %U %x of runes, string(rune) values, Print of runes (numbers), range over strings, Sprintf with unicode padding, invalid UTF-8 in %q and %x.",
     "",
     '''	r := 'é'
	b := byte('a')
	fmt.Println(r, b, string(r), string(rune(b)))
	fmt.Printf("%c %q %U %#U %x %d %v\\n", r, r, r, r, r, r, r)
	fmt.Printf("%c%c%c %q\\n", 0x65e5, 0x672c, 0x8a9e, 0x65e5)
	fmt.Printf("%5c|%-5c|%05d|%+q\\n", r, r, r, r)
	fmt.Printf("%q %+q %#q %x % x %X\\n", "héllo", "héllo", "héllo", "héllo", "héllo", "héllo")
	fmt.Printf("%q %x %q\\n", "a\\xffb", "a\\xffb", "\\xff")
	fmt.Printf("%s|%10s|%-10s|%.2s|%10.2s|\\n", "日本語", "日本語", "日本語", "日本語", "日本語")
	for i, c := range "aé日😀" {
		fmt.Printf("%d:%c(%U,%d);", i, c, c, c)
	}
	fmt.Println()
	fmt.Println([]rune("日本"), []byte("日本"), string([]rune{26085, 26412}))
	fmt.Printf("%v %s %q %x\\n", []rune("ab"), []rune("ab"), []rune("ab"), []rune("ab"))
	fmt.Printf("%c %c %c\\n", []rune("abc"), []byte("abc"), [2]int{65, 66})
	fmt.Printf("%q %q\\n", 'x', []int32{'x', 'y'})
	fmt.Printf("%08.3f|%+.2e|%x|%o|%b\\n", 3.14159, 12345.678, 255, 8, 5)
	fmt.Printf("%3d|%-3d|%03d|% d|%+d\\n", 7, 7, 7, 7, 7)
	fmt.Printf("%t %v %5t|%-7t|\\n", true, false, true, false)
''')

case("fmt_embedded_interfaces",
     "Interfaces embedded in structs and interface-typed fields: a struct that embeds an interface prints through the promoted method set; fields of interface type hold values with methods.",
     '''type Shape interface{ Area() float64 }

type Sq struct{ S float64 }

func (s Sq) Area() float64 { return s.S * s.S }

func (s Sq) String() string { return fmt.Sprintf("Sq(%g)", s.S) }

type Circle struct{ R float64 }

func (c *Circle) Area() float64 { return 3 * c.R * c.R }

type Holder struct {
	Shape
	Name string
}

type NamedShape struct {
	fmt.Stringer
	N int
}

type Both interface {
	Shape
	fmt.Stringer
}

type Gallery struct {
	Items []Shape
	Index map[string]Shape
	Best  Shape
	both  Both
	Any   any
}
''',
     NORM[:0] + '''	h := Holder{Sq{2}, "sq"}
	fmt.Println(h, h.Area())
	fmt.Printf("%v %+v %#v\\n", h, h, h)
	ns := NamedShape{Sq{3}, 1}
	fmt.Println(ns)
	fmt.Printf("%v %+v %s\\n", ns, ns, ns)
	var empty Holder
	fmt.Printf("%v %+v\\n", empty, empty)
	g := Gallery{[]Shape{Sq{1}, &Circle{2}, nil}, map[string]Shape{"a": Sq{4}}, Sq{5}, Sq{6}, Sq{7}}
	s := fmt.Sprintf("%v", g)
	fmt.Println(len(s) > 10, s[:9])
	fmt.Printf("%v\\n", Gallery{})
	fmt.Printf("%+v\\n", Gallery{Best: Sq{8}})
	var b Both = Sq{9}
	fmt.Println(b, b.Area(), []Both{b})
''' )

case("fmt_chans",
     "Channels print their address (<nil> for a nil channel) and %T names the direction; %d %x work on the address; a channel in a struct, slice, map and any; named channel types.",
     NORM + '''type Pipe chan int

type Holder struct {
	C  chan string
	R  <-chan int
	S  chan<- bool
	c  chan int
}
''',
     '''	var nc chan int
	c := make(chan int, 2)
	var r <-chan int = c
	var s chan<- int = c
	p := make(Pipe, 1)
	fmt.Println(nc, nc == nil)
	fmt.Printf("%T %T %T %T %T\\n", nc, c, r, s, p)
	fmt.Printf("%v %v %v %t\\n", fmt.Sprint(c)[:2], fmt.Sprint(r)[:2], fmt.Sprint(s)[:2], fmt.Sprint(c) == fmt.Sprint(r))
	fmt.Println(fmt.Sprintf("%p", c) == fmt.Sprint(c), fmt.Sprintf("%p", nc), fmt.Sprintf("%d", nc))
	fmt.Println(norm(fmt.Sprint(Holder{c: c})), norm(fmt.Sprint(Holder{})))
	fmt.Println(norm(fmt.Sprintf("%+v", Holder{C: make(chan string)})))
	fmt.Println(norm(fmt.Sprint([]chan int{c, nil})), norm(fmt.Sprint(map[string]chan int{"a": c})))
	var a any = c
	fmt.Printf("%T %t\\n", a, a == any(c))
	fmt.Printf("%#v %#v\\n", nc, [1]chan int{})
	fmt.Println(norm(fmt.Sprintf("%v %s", c, c)))
''', phase=8)
