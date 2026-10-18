from lib import case

NORM = '''
// norm replaces every hexadecimal address (0x...) with 0xP: addresses differ from run to run.
func norm(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '0' && i+1 < len(s) && s[i+1] == 'x' {
			j := i + 2
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] >= 'a' && s[j] <= 'f') {
				j++
			}
			if j > i+2 {
				out = append(out, "0xP"...)
				i = j - 1
				continue
			}
		}
		out = append(out, s[i])
	}
	return string(out)
}
'''

def pr(fmtstr, *args, ln=True):
    a = ", ".join(args)
    return '\tfmt.Printf("%s\\n", %s)\n' % (fmtstr, a) if args else '\tfmt.Printf("%s\\n")\n' % fmtstr

# ---------------- structs ----------------
case("fmt_struct_basic",
     "Structs: %v {1 2}, %+v with field names, %#v in Go syntax, %T; every kind of field; nested and empty structs.",
     '''type Point struct{ X, Y int }

type Rec struct {
	Name   string
	Age    int
	Score  float64
	OK     bool
	R      rune
	B      byte
	Tags   []string
	Attrs  map[string]int
	Pt     Point
	Inner  struct{ A, B int }
	Arr    [2]bool
	Nil    []int
	NilMap map[string]int
	Any    any
}

type Empty struct{}

type Blank struct {
	A int
	_ int
	B string
}
''',
     '''	r := Rec{"ann", 31, 9.5, true, 'x', 7, []string{"a", "b"}, map[string]int{"z": 1, "a": 2}, Point{1, 2}, struct{ A, B int }{3, 4}, [2]bool{true, false}, nil, nil, 42}
	fmt.Printf("%v\\n%+v\\n%#v\\n%T\\n", r, r, r, r)
	fmt.Printf("%v %+v %#v %T\\n", Empty{}, Empty{}, Empty{}, Empty{})
	fmt.Printf("%v %+v %#v\\n", Blank{A: 1, B: "x"}, Blank{A: 1, B: "x"}, Blank{A: 1, B: "x"})
	fmt.Printf("%v %+v %#v\\n", Point{}, &Point{3, 4}, &Point{5, 6})
	fmt.Printf("%v %+v %#v\\n", struct{}{}, struct{ A int }{1}, struct {
		A int `json:"a"`
		b string
	}{1, "x"})
	fmt.Println(r, Point{7, 8}, &Point{9, 10})
''')

case("fmt_struct_verbs",
     "A verb applies to every field of a struct: width, precision and flags too; fields that don't fit the verb print Go's error form.",
     "type M struct {\n\tN    int\n\tF    float64\n\tS    string\n\tB    bool\n\tP    struct{ A, B int }\n}\n",
     '''	m := M{255, 2.5, "str", true, struct{ A, B int }{1, 2}}
	for _, f := range []string{"%v", "%+v", "%#v", "%d", "%x", "%X", "%o", "%b", "%s", "%q", "%t", "%f", "%.2f", "%e", "%g", "%5v", "%-5v", "%05d", "%8.3v", "%c", "%U", "%p", "%08.3f", "%+d", "% d", "%+q", "%#x", "%6.2f"} {
		fmt.Printf("%-8s [", f)
		fmt.Printf(f, m)
		fmt.Printf("]\\n")
	}
	fmt.Printf("%d %s\\n", &m, &m)
''')

case("fmt_struct_embedded",
     "Embedded structs and pointers: the field name is the type name (%+v); promoted fields; embedded Stringers take over the whole struct.",
     NORM + '''type Base struct{ ID int }

type Named struct {
	Base
	Name string
}

type Deep struct {
	*Named
	Level int
}

type Inner struct{ V int }

type outer struct {
	Inner
	W int
}

type Pub struct{ A int }

type holder struct {
	Pub
	x int
}

type Shown struct{ N int }

func (s Shown) String() string { return "shown" }

type Wrap struct {
	Shown
	Extra int
}

type Ptr struct{ N int }

func (p *Ptr) String() string { return "ptr-string" }

type WrapPtr struct {
	*Ptr
	Z int
}
''',
     '''	n := Named{Base{1}, "ann"}
	fmt.Printf("%v %+v %#v\\n", n, n, n)
	d := Deep{&n, 3}
	fmt.Printf("%v\\n", norm(fmt.Sprintf("%v", d)))
	fmt.Printf("%+v\\n", norm(fmt.Sprintf("%+v", d)))
	fmt.Println(n.ID, n.Base.ID, d.Name, d.ID)
	fmt.Printf("%v %+v %#v\\n", outer{Inner{1}, 2}, outer{Inner{1}, 2}, outer{Inner{1}, 2})
	fmt.Printf("%v %+v %#v\\n", holder{Pub{1}, 2}, holder{Pub{1}, 2}, holder{Pub{1}, 2})
	w := Wrap{Shown{1}, 2}
	fmt.Printf("%v %+v %#v %s %d\\n", w, w, w, w, w)
	wp := WrapPtr{&Ptr{1}, 2}
	fmt.Printf("%v %+v %s\\n", wp, wp, wp)
	fmt.Printf("%v %s\\n", &w, &w)
''' , imports=("fmt",))
tests_norm = ["fmt_struct_embedded"]

case("fmt_struct_stringer_fields",
     "fmt calls String and Error on fields it can reach with reflection: exported fields yes, unexported fields no (CanInterface is false), also inside slices, maps and pointers; an exported field of an unexported embedded struct is open again.",
     NORM + '''type Money int

func (m Money) String() string { return fmt.Sprintf("$%d", int(m)) }

type Err struct{ msg string }

func (e Err) Error() string { return "E:" + e.msg }

type Acct struct {
	Pub   Money
	priv  Money
	Pubs  []Money
	privs []Money
	M     map[string]Money
	m     map[string]Money
	E     error
	e     error
	A     any
	a     any
	P     *Money
	p     *Money
	S     fmt.Stringer
	s     fmt.Stringer
}

type hidden struct {
	Shown  Money
	hidden Money
}

type Outer struct {
	hidden
	Top Money
}

type Outer2 struct {
	Hidden hidden
	hid    hidden
}
''',
     '''	m := Money(5)
	a := Acct{1, 2, []Money{3}, []Money{4}, map[string]Money{"k": 5}, map[string]Money{"k": 6}, Err{"a"}, Err{"b"}, Money(7), Money(8), &m, &m, Money(9), Money(10)}
	fmt.Println(norm(fmt.Sprintf("%v", a)))
	fmt.Println(norm(fmt.Sprintf("%+v", a)))
	o := Outer{hidden{1, 2}, 3}
	fmt.Printf("%v %+v\\n", o, o)
	o2 := Outer2{hidden{1, 2}, hidden{3, 4}}
	fmt.Printf("%v %+v\\n", o2, o2)
	fmt.Printf("%v %v\\n", []hidden{{1, 2}}, map[string]hidden{"k": {3, 4}})
	fmt.Printf("%s %d\\n", Acct{Pub: 1}.Pub, Acct{Pub: 1}.Pub)
''', )
tests_norm.append("fmt_struct_stringer_fields")

case("fmt_struct_pointers",
     "Pointers in structs: &{...} for a pointer to a struct, array, slice or map at the top level; inside a value a pointer prints its address (here replaced by 0xP); nil pointers print <nil>.",
     NORM + '''
type Node struct {
	V    int
	Next *Node
}

type Tree struct {
	L, R *Tree
	Val  int
}

type Holder struct {
	P  *int
	PS *Point
	PA *[2]int
	PM *map[string]int
}

type Point struct{ X, Y int }
''',
     '''	n := Node{1, &Node{2, nil}}
	fmt.Println(norm(fmt.Sprint(n)), norm(fmt.Sprint(&n)))
	fmt.Println(norm(fmt.Sprintf("%+v %+v", n, &n)))
	fmt.Println(norm(fmt.Sprintf("%#v", n)))
	t := &Tree{nil, nil, 1}
	fmt.Println(*t, t)
	fmt.Printf("%v %+v\\n", Tree{}, &Tree{})
	x := 5
	h := Holder{&x, &Point{1, 2}, &[2]int{1, 2}, &map[string]int{"a": 1}}
	fmt.Println(norm(fmt.Sprint(h)))
	fmt.Println(norm(fmt.Sprintf("%+v", h)))
	fmt.Println(Holder{})
	fmt.Printf("%v %v %v %v\\n", &[2]int{1, 2}, &[]int{3}, &map[int]bool{1: true}, &[0]int{})
	var pp **int
	px := &x
	pp = &px
	fmt.Println(norm(fmt.Sprint(pp)), pp != nil)
	var np *Node
	var ni *int
	fmt.Println(np, ni, []*int{nil}, map[string]*Node{"k": nil})
	fmt.Printf("%v %+v %#v %s %d\\n", np, np, np, np, np)
	fmt.Printf("%t %t\\n", fmt.Sprint(&x)[:2] == "0x", fmt.Sprintf("%p", &x)[:2] == "0x")
''')
tests_norm.append("fmt_struct_pointers")

case("fmt_struct_generic_shapes",
     "Structs that hold arrays, slices of structs, maps of structs, interfaces and funcs; nil funcs print <nil>.",
     '''type Item struct {
	Name string
	Qty  int
}

type Order struct {
	ID    int
	Items []Item
	ByKey map[string]Item
	Grid  [2][2]Item
	Cb    func()
	Any   any
	Err   error
}
''',
     '''	o := Order{ID: 1, Items: []Item{{"a", 1}, {"b", 2}}, ByKey: map[string]Item{"x": {"c", 3}}, Any: Item{"d", 4}}
	fmt.Printf("%v\\n%+v\\n", o, o)
	fmt.Printf("%#v\\n", o)
	fmt.Println(Order{}, Order{}.Items == nil)
	fmt.Printf("%v %+v\\n", []Order{{ID: 2}}, []Order{{ID: 3}})
	fmt.Printf("%T %T %T\\n", o, o.Items, o.Grid)
''')

# ---------------- slices and arrays ----------------
case("fmt_slice_basic",
     "Slices and arrays of numbers, strings and bools with the usual verbs: a verb applies to each element, with its width and precision; nil and empty slices; nested slices.",
     "",
     '''	ints := []int{1, -2, 300}
	strs := []string{"a", "b c", "é"}
	flts := []float64{1, 2.5, -0.125, 1e21}
	bs := []bool{true, false}
	var nilS []int
	empty := []int{}
	for _, f := range []string{"%v", "%+v", "%#v", "%d", "%x", "%X", "%o", "%b", "%c", "%q", "%U", "%5d", "%-5d|", "%05d", "%+d", "% d", "%.2d", "%s", "%t", "%f", "%8.2f"} {
		fmt.Printf("%-6s", f)
		fmt.Printf(f, ints)
		fmt.Printf(" ")
		fmt.Printf(f, strs)
		fmt.Printf(" ")
		fmt.Printf(f, flts)
		fmt.Printf(" ")
		fmt.Printf(f, bs)
		fmt.Printf(" ")
		fmt.Printf(f, nilS)
		fmt.Printf(" ")
		fmt.Printf(f, empty)
		fmt.Printf("\\n")
	}
	fmt.Println(nilS == nil, empty == nil, len(nilS), nilS, empty)
	fmt.Printf("%v %+v %#v %T\\n", [][]int{{1}, {2, 3}, nil}, [][]int{{1}, {2, 3}, nil}, [][]int{{1}, {2, 3}, nil}, [][]int{})
	fmt.Printf("%v %#v %T %v\\n", [0]int{}, [0]int{}, [0]string{}, [3][2]int{})
	fmt.Printf("%v %v\\n", [3]string{"a", "", "c"}, [2][]string{{"x"}, nil})
''')

case("fmt_slice_elements_methods",
     "Slices, arrays and maps of values with String and Error methods: each element is printed through its method (also pointers to such values); nil elements.",
     '''type Name string

func (n Name) String() string { return "<" + string(n) + ">" }

type Code int

func (c Code) Error() string { return fmt.Sprintf("code %d", int(c)) }

type PS struct{ n int }

func (p *PS) String() string { return fmt.Sprintf("ps%d", p.n) }

type VS struct{ n int }

func (v VS) String() string { return fmt.Sprintf("vs%d", v.n) }
''',
     '''	fmt.Println([]Name{"a", "b"}, [2]Code{1, 2}, map[Name]Code{"k": 3, "a": 4})
	fmt.Printf("%v %s %q %x %d\\n", []Name{"a"}, []Name{"b"}, []Name{"c"}, []Name{"d"}, []Name{"e"})
	fmt.Printf("%v %s %q %d\\n", []Code{1}, []Code{2}, []Code{3}, []Code{4})
	fmt.Printf("%v %+v %#v\\n", []Name{"a"}, []Name{"a"}, []Name{"a"})
	p1, p2 := &PS{1}, &PS{2}
	fmt.Println([]*PS{p1, p2}, []PS{{3}}, []VS{{4}}, []*VS{{5}})
	var np *PS
	var nv *VS
	fmt.Println([]*PS{nil, p1}, np, nv, []*VS{nv})
	fmt.Printf("%v %v\\n", [2]*PS{p1, nil}, map[string]*PS{"k": p2, "n": nil})
	fmt.Printf("%v\\n", []any{Name("x"), Code(2), p1, VS{6}, 7, nil, "s"})
	fmt.Printf("%v\\n", []error{Code(1), nil, Code(2)})
	fmt.Printf("%v\\n", []fmt.Stringer{Name("s"), nil, VS{1}})
	fmt.Printf("%s\\n", []any{Name("x"), Code(2), 7})
''')

# ---------------- maps ----------------
MAPCASES = [
 ("ints", "map[int]string{3: \"c\", -1: \"a\", 0: \"z\", 2: \"b\", 100: \"d\", -50: \"e\"}"),
 ("uints", "map[uint8]int{255: 1, 0: 2, 128: 3, 7: 4}"),
 ("int64", "map[int64]bool{1 << 40: true, -1 << 40: false, 5: true}"),
 ("strings", "map[string]int{\"b\": 2, \"a\": 1, \"\": 0, \"ab\": 5, \"B\": 3, \"é\": 6, \"z\": 7, \"aa\": 8}"),
 ("bools", "map[bool]string{true: \"yes\", false: \"no\"}"),
 ("floats", "map[float64]int{2.5: 1, -1: 2, 0: 3, 1e21: 4, -1e21: 5, 0.1: 6, inf: 7, ninf: 8, nan: 9}"),
 ("nzero", "map[float64]int{nzero: 1}"),
 ("float32", "map[float32]string{0.5: \"h\", 0.25: \"q\", -3: \"n\"}"),
 ("complex", "map[complex128]int{complex(1, 2): 1, complex(1, 1): 2, complex(0, 5): 3, complex(-1, 0): 4}"),
 ("arrays", "map[[2]int]string{{1, 2}: \"a\", {1, 1}: \"b\", {0, 9}: \"c\", {2, 0}: \"d\"}"),
 ("structs", "map[pt]int{{2, 1}: 1, {1, 2}: 2, {1, 1}: 3, {0, 5}: 4}"),
 ("runes", "map[rune]int{'b': 1, 'a': 2, 'é': 3, 'z': 4}"),
 ("nested", "map[string]map[int]string{\"x\": {2: \"b\", 1: \"a\"}, \"a\": {}, \"m\": nil}"),
 ("slices", "map[string][]int{\"b\": {1, 2}, \"a\": nil, \"c\": {}}"),
 ("ifacekeys", "map[any]int{3: 1, 1: 2, 2: 3}"),
 ("ifacestr", "map[any]int{\"b\": 1, \"a\": 2, \"c\": 3}"),
 ("named", "map[Color]Level{Blue: 1, Red: 2, Green: 3}"),
 ("empty", "map[string]int{}"),
]
for name, expr in MAPCASES:
    body = '''	zero := 0.0
	inf := 1 / zero
	ninf := -1 / zero
	nan := zero / zero
	nzero := -zero
	_, _, _, _ = inf, ninf, nan, nzero
	m := %s
	fmt.Println(m, len(m))
	fmt.Printf("%%v|%%+v|%%#v|%%T\\n", m, m, m, m)
	fmt.Printf("%%d|%%s|%%x|%%q|%%5v|%%-5v|\\n", m, m, m, m, m, m)
	fmt.Println(fmt.Sprint(m) == fmt.Sprint(m), fmt.Sprint(m) == fmt.Sprintf("%%v", m))
''' % expr
    case("fmt_map_%s" % name,
         "Maps print sorted by key (internal/fmtsort): the %s case, with %%v %%+v %%#v and other verbs applied to keys and values." % name,
         '''type pt struct{ x, y int }

type Color int

const (
	Red Color = iota
	Green
	Blue
)

func (c Color) String() string { return [...]string{"red", "green", "blue"}[c] }

type Level uint8
''', body)

case("fmt_map_nan_keys",
     "Several NaN keys in a map: they sort first (cmp.Compare puts NaN before every number) and print in an unspecified order among themselves; here only one NaN key.",
     "",
     '''	zero := 0.0
	nan := zero / zero
	m := map[float64]string{nan: "n", 1: "a", -1: "b"}
	fmt.Println(m)
	fmt.Printf("%v %+v %#v\\n", m, m, m)
	fmt.Println(len(m), m[nan] == "")
''')

case("fmt_map_values_and_nil",
     "Maps: nil and empty, maps of maps, slices, structs and pointers, %#v with the type name, maps inside structs and interfaces.",
     "type S struct{ M map[string]int }\n",
     '''	var nm map[string]int
	em := map[string]int{}
	fmt.Println(nm, em, nm == nil, em == nil, len(nm))
	fmt.Printf("%v %+v %#v %#v %T\\n", nm, nm, nm, em, nm)
	fmt.Printf("%v %#v\\n", S{}, S{})
	fmt.Printf("%v %#v\\n", S{map[string]int{"a": 1}}, S{map[string]int{"a": 1}})
	var a any = map[string]int{"q": 1}
	fmt.Printf("%v %#v %T\\n", a, a, a)
	fmt.Printf("%v\\n", []map[string]int{{"a": 1}, nil, {}})
	fmt.Printf("%v\\n", map[string][]string{"b": {"x", "y"}, "a": nil})
	fmt.Printf("%v\\n", map[int]map[int]int{2: {1: 1}, 1: {2: 2}})
	fmt.Printf("%v %v\\n", map[string]any{"z": 1, "a": "s", "m": nil, "k": []int{1}}, map[string]error{"e": nil})
	fmt.Printf("%v %v\\n", map[int]struct{}{2: {}, 1: {}}, map[string]bool{"t": true, "f": false})
''')
