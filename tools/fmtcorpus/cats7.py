from lib import case

case("fmt_spread_wrappers",
     "A final argument a... (a []any) is the list of operands, not one operand: logf(f, a...) wrappers around Printf, Sprintf, Println, Print, Sprint, Sprintln, Fprintf, Fprintln, Errorf, Appendf; nil and empty slices; slice literals; nested wrappers; append results.",
     '''type S struct{ A int }

func (s S) String() string { return "S!" }

func logf(f string, a ...any) { fmt.Printf(f, a...) }

func logln(a ...any) { fmt.Println(a...) }

func logp(a ...any) { fmt.Print(a...) }

func sf(f string, a ...any) string { return fmt.Sprintf(f, a...) }

func sp(a ...any) string { return fmt.Sprint(a...) }

func sln(a ...any) string { return fmt.Sprintln(a...) }

func ef(f string, a ...any) error { return fmt.Errorf(f, a...) }

func outer(f string, a ...any) { logf(f, a...) }

func deep(f string, a ...any) string { return sf(f, a...) }

func fe(f string, a ...any) { fmt.Fprintf(os.Stderr, f, a...); fmt.Fprintln(os.Stdout, a...); fmt.Fprint(os.Stdout, a...); fmt.Fprintln(os.Stdout) }

func app(b []byte, f string, a ...any) []byte { b = fmt.Appendf(b, f, a...); b = fmt.Append(b, a...); return fmt.Appendln(b, a...) }

func count(a ...any) int { return len(a) }

func pass(a ...any) int { return count(a...) }
''',
     '''	logf("%d %s %v\\n", 1, "a", S{2})
	logf("no args\\n")
	logf("%d %s\\n")
	logf("%d\\n", 1, 2, 3)
	logln("x", 1, 2.5, nil, S{3}, []int{4})
	logln()
	logp("a", "b", 1, 2, "c\\n")
	fmt.Println(sf("%5d|%-5s|%v|%T", 7, "ab", S{1}, S{1}), sp(1, 2, "x", 3), len(sln("a", "b")))
	fmt.Println(sf("%v %v", []any{1, "z"}...), sf("%v", []any{[]any{1, 2}}...), sf("%v", []any{}))
	var nilargs []any
	logln(nilargs...)
	logf("nil: %d\\n", nilargs...)
	fmt.Println(sp(nilargs...) == "", sf("x", nilargs...), sln(nilargs...) == "\\n")
	outer("%s-%s-%d\\n", "p", "q", 3)
	fmt.Println(deep("%v|%v", 1, nil), deep("%d", S{}) )
	args := []any{"w", 5}
	logf("%s %d\\n", args...)
	logln(args...)
	logf("%s %d %v\\n", append([]any{"first"}, args...)...)
	fmt.Println(count(args...), pass(args...), pass(), pass(nilargs...), count(nilargs...))
	errs0 := fmt.Errorf("e0")
	e := ef("wrap %w and %w, %d", S{1}, errs0, 3)
	fmt.Println(e)
	errs := []any{fmt.Errorf("one"), fmt.Errorf("two")}
	e2 := ef("%w + %w", errs...)
	u, ok := e2.(interface{ Unwrap() []error })
	fmt.Println(e2, ok, len(u.Unwrap()))
	e3 := ef("only %w", errs[0])
	fmt.Println(e3, errors.Is(e3, errs[0].(error)))
	fe("to stderr %d\\n", 9)
	fe("%s\\n", "hi")
	fmt.Printf("%q\\n", app([]byte("<"), "%d,", 1, "x"))
	fmt.Printf("%q\\n", app(nil, "none"))
	mixed := []any{1, int8(2), uint(3), 4.5, true, 'r', "s", S{9}, &S{8}, map[string]int{"k": 1}, [2]int{5, 6}, nil, fmt.Errorf("err")}
	logf("%v %v %v %v %v %v %v %v %v %v %v %v %v\\n", mixed...)
	logf("%T %T %T %T %T %T %T %T %T %T %T %T\\n", mixed[:12]...)
	logf("%d|%+v|%#v|%x|%q\\n", mixed[:5]...)
	logf("%[2]v %[1]v %*d %.*f\\n", mixed[:4]...)
	var sw any = []any{1, 2}
	fmt.Println(sw)
	defer logln(args...)
	defer fmt.Printf("%v %v\\n", args...)
	fmt.Println("end")
''', imports=("errors", "fmt", "os"), stderr=".*")

case("fmt_nil_interfaces",
     "A nil interface where a function of fmt takes one: fmt.FormatString(nil, 'v') and Fprintf(nil, ...) panic with a nil dereference; fmt.State, Stringer, GoStringer and Formatter variables are nil interfaces that compare equal to nil and can be assigned; a nil Stringer is <nil> to Println.",
     '''func use(s fmt.Stringer, g fmt.GoStringer, f fmt.Formatter, st fmt.State) string {
	return fmt.Sprint(s == nil, g == nil, f == nil, st == nil, s, g, f, st)
}
''',
     '''	var s fmt.Stringer
	var st fmt.State
	fmt.Println(use(nil, nil, nil, nil), use(s, nil, nil, st))
	fmt.Printf("%v %T %d\\n", s, s, st)
	defer func() { fmt.Println("recovered:", recover()) }()
	fmt.Println(fmt.FormatString(nil, 'v'))
''')

case("fmt_errorf_reordered",
     "Errorf with %w and argument indexes: the wrapped errors of %[2]w %[1]w are in argument order after sorting; a repeated %[1]w counts once; operands that are not errors are skipped by Unwrap() []error; %w with width and flags.",
     '''func many(err error) []error {
	u, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return nil
	}
	return u.Unwrap()
}

func one(err error) error {
	u, ok := err.(interface{ Unwrap() error })
	if !ok {
		return nil
	}
	return u.Unwrap()
}
''',
     '''	e1, e2, e3 := fmt.Errorf("e1"), fmt.Errorf("e2"), fmt.Errorf("e3")
	for _, f := range []string{"%[2]w %[1]w", "%[1]w %[2]w %[1]w", "%[3]w %[1]w %[2]w", "%w %[1]w", "%[2]w %w", "%[3]w %[3]w", "%w %w %w", "%[1]w|%[3]w"} {
		err := fmt.Errorf(f, e1, e2, e3)
		w := many(err)
		fmt.Printf("%-18s -> %v | %T | %d:", f, err, err, len(w))
		for _, x := range w {
			fmt.Print(" ", x)
		}
		fmt.Println()
	}
	err := fmt.Errorf("%[2]w", e1, e2)
	fmt.Println(err, one(err), many(err) == nil)
	err = fmt.Errorf("%[1]w %[1]w", e1)
	fmt.Println(err, one(err), many(err) == nil, len(many(err)))
	err = fmt.Errorf("%w %w %w", e1, 7, e3)
	fmt.Println(err, len(many(err)))
	err = fmt.Errorf("%5w|%-5w|%+w|%#w", e1, e2, e3, e1)
	fmt.Println(err, len(many(err)))
	err = fmt.Errorf("%[3]w %[1]w", e1, e2, e3, e1)
	fmt.Println(err, len(many(err)), many(err)[0] == e1)
''')

case("fmt_mutating_methods",
     "A String method that changes what is being printed: an operand is a copy, but through a pointer fmt reads the slice again for each element (Stamp changes a global printed after it, MapEdit deletes from the map that holds it, Clear and Drop empty the slice or the slice of slices being printed).",
     '''type Stamp int

var later = []int{1, 2}

func (s Stamp) String() string {
	later = nil
	return "stamp"
}

type MapEdit int

var edited map[int]MapEdit

func (m MapEdit) String() string {
	for k := range edited {
		delete(edited, k)
	}
	return "edit" + fmt.Sprint(int(m))
}

type Clear int

type Holder struct{ L []Clear }

var held Holder

func (Clear) String() string {
	held.L = nil
	return "c"
}

type Drop int

var drops [][]Drop

func (Drop) String() string {
	drops = nil
	return "d"
}

type Pair struct{ A, B Clear }

var pp *Pair
''',
     '''	fmt.Println(Stamp(1), later)
	edited = map[int]MapEdit{1: 1, 2: 2, 3: 3}
	fmt.Println(edited, len(edited))
	edited = map[int]MapEdit{1: 1, 2: 2, 3: 3}
	fmt.Printf("%v %d\\n", &edited, len(edited))
	held = Holder{[]Clear{1, 2, 3}}
	fmt.Println(held, held.L == nil)
	held = Holder{[]Clear{1, 2, 3}}
	fmt.Println(&held, held.L == nil)
	drops = [][]Drop{{1, 2}, {3}}
	fmt.Println(&drops, drops == nil)
	drops = [][]Drop{{1, 2}, {3}}
	fmt.Println(drops, drops == nil)
	held = Holder{[]Clear{1, 2, 3}}
	fmt.Printf("%v %+v %s\\n", &held, &held, &held)
''')

case("fmt_embedded_readonly",
     "The read-only rules of reflect for embedded fields: emb is unexported and embedded in Pub, Pub is exported and embedded in the unexported hid, Top embeds hid and *pemb; exported fields of an unexported embedded struct are open again, everything inside an unexported field is closed; the same inside slices, maps, arrays, pointers and interfaces.",
     '''type Celsius float64

func (c Celsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }

type Pub2 struct{ D Celsius }

type emb struct {
	C Celsius
	c Celsius
	L []Celsius
	M map[string]Celsius
	P Pub2
	p Pub2
}

type Pub struct {
	emb
	E Celsius
}

type hid struct {
	Pub
	F Celsius
	g Celsius
}

type pemb struct{ H Celsius }

type Top struct {
	hid
	*pemb
	Pubs []Pub
	pubs []Pub
	x    Pub
}
''',
     '''	e := emb{1, 2, []Celsius{3}, map[string]Celsius{"k": 4}, Pub2{5}, Pub2{6}}
	pub := Pub{e, 7}
	fmt.Println(pub)
	fmt.Printf("%+v\\n%v\\n%s\\n%d\\n", pub, &pub, []Pub{pub}, pub)
	fmt.Println(map[string]Pub{"m": pub}, [1]Pub{pub})
	h := hid{pub, 8, 9}
	fmt.Println(h)
	fmt.Printf("%+v\\n", h)
	t := Top{h, nil, []Pub{pub}, []Pub{pub}, pub}
	fmt.Println(t)
	fmt.Printf("%+v\\n", &t)
	fmt.Println([]Top{t}, map[int]Top{1: t})
	fmt.Println(e, struct{ emb }{e}, struct{ x emb }{e})
	var a any = pub
	fmt.Println(a, []any{pub, e}, struct{ A any; a any }{pub, pub})
''')

case("fmt_panics_runtime_errors",
     "A String method that fails with a runtime error: an index out of range, a nil dereference, a plain panic and a nil pointer receiver print as %!v(PANIC=String method: runtime error: ...), in every position and verb.",
     '''type Point struct{ X, Y int }

type Boom struct{ N int }

func (b Boom) String() string {
	if b.N == 1 {
		panic("boom")
	}
	if b.N == 2 {
		s := []int{1, 2, 3}
		i := 5
		return fmt.Sprint(s[i])
	}
	if b.N == 3 {
		var p *Point
		return fmt.Sprint(p.X)
	}
	if b.N == 4 {
		var m map[string]int
		m["a"] = 1
		return ""
	}
	if b.N == 5 {
		z := 0
		return fmt.Sprint(1 / z)
	}
	if b.N == 6 {
		var a any = "s"
		return fmt.Sprint(a.(int))
	}
	return "ok"
}

type Deref struct{ V int }

func (p *Deref) String() string { return "Deref(" + fmt.Sprint(p.V) + ")" }
''',
     '''	fmt.Println(Boom{0}, Boom{1})
	fmt.Println([]Boom{{0}, {1}, {2}})
	fmt.Printf("%s|%v|%d|%q|%5v\\n", Boom{1}, Boom{2}, Boom{1}, Boom{1}, Boom{1})
	for i := 3; i <= 6; i++ {
		fmt.Println(Boom{i})
	}
	fmt.Println(map[string]Boom{"a": {1}, "b": {0}})
	fmt.Println(&Deref{3}, struct{ D *Deref }{nil})
	var nd *Deref
	fmt.Printf("%v|%s|%8v|\\n", nd, nd, nd)
	fmt.Println(struct{ B Boom }{Boom{3}}, struct{ b Boom }{Boom{3}})
''')

case("fmt_quote_whole_range",
     "%q and %+q of every code point from -2 to U+110001 (as runes), hashed with FNV-1a in the program, so that an error in the table of printable runes shows (it is checked against Go's output); invalid strings too.",
     '''type fnv uint64

func (h *fnv) add(s string) {
	for i := 0; i < len(s); i++ {
		*h = (*h ^ fnv(s[i])) * 1099511628211
	}
}
''',
     '''	h := fnv(14695981039346656037)
	n := 0
	for r := rune(-2); r <= 0x110001; r++ {
		h.add(fmt.Sprintf("%q", r))
		if r < 0x2000 || r&0x7f == 0 {
			h.add(fmt.Sprintf("%+q%q%+q", r, string(r), string(r)))
		}
		n++
	}
	fmt.Println(n, uint64(h))
	h = fnv(14695981039346656037)
	for b := 0; b < 256; b++ {
		for c := 0; c < 256; c += 15 {
			s := string([]byte{byte(b), byte(c), 'x'})
			h.add(fmt.Sprintf("%q%+q", s, s))
		}
	}
	fmt.Println(uint64(h))
''')

case("fmt_weak_pointers",
     "weak.Pointer prints as Go's struct { _ [0]*T; u unsafe.Pointer }: %v {[] <nil>}, %+v with the field names, %#v with the types of the fields; %T names the instance; %d of the zero value.",
     "type T struct{ N int }\n",
     '''	var z weak.Pointer[T]
	var zi weak.Pointer[int]
	fmt.Printf("%v %+v\\n", z, z)
	fmt.Printf("%#v\\n%#v\\n", z, zi)
	fmt.Printf("%T %T %d\\n", z, []weak.Pointer[T]{}, z)
	fmt.Println(z, []weak.Pointer[int]{zi}, struct{ W weak.Pointer[T] }{})
	fmt.Printf("%#v\\n", struct{ W weak.Pointer[T] }{})
''', imports=("fmt", "weak"))
