from lib import case
from cats3 import NORM

# ---- Errorf ----
case("fmt_errorf_basic",
     "fmt.Errorf formats like Sprintf; the result is an error whose type is *errors.errorString without %w, *fmt.wrapError with one %w, *fmt.wrapErrors with several.",
     '''type MyErr struct{ code int }

func (e *MyErr) Error() string { return fmt.Sprintf("my error %d", e.code) }

type ValErr struct{ n int }

func (e ValErr) Error() string { return fmt.Sprintf("val %d", e.n) }

func unwrap(err error) error {
	u, ok := err.(interface{ Unwrap() error })
	if !ok {
		return nil
	}
	return u.Unwrap()
}

func unwrapMany(err error) []error {
	u, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return nil
	}
	return u.Unwrap()
}
''',
     '''	e1 := fmt.Errorf("plain %d %s", 1, "x")
	fmt.Println(e1, e1.Error())
	fmt.Printf("%T %v %q %s\\n", e1, e1, e1, e1)
	base := &MyErr{7}
	e2 := fmt.Errorf("wrapped: %w", base)
	fmt.Println(e2)
	fmt.Printf("%T %v %t %t\\n", e2, e2, unwrap(e2) == error(base), unwrap(e1) == nil)
	e3 := fmt.Errorf("two: %w and %w", base, ValErr{2})
	fmt.Println(e3)
	fmt.Printf("%T %d %v\\n", e3, len(unwrapMany(e3)), unwrapMany(e3))
	e4 := fmt.Errorf("deep: %w", e2)
	fmt.Println(e4, unwrap(unwrap(e4)) == error(base))
	e5 := fmt.Errorf("value: %w", ValErr{9})
	fmt.Println(e5, unwrap(e5))
	e6 := fmt.Errorf("nil: %w", nil)
	fmt.Printf("%v %T %v\\n", e6, e6, unwrap(e6))
	e7 := fmt.Errorf("%w", base)
	fmt.Println(e7, e7.Error() == base.Error())
	e8 := fmt.Errorf("no verb")
	fmt.Println(e8, fmt.Errorf("%d%%", 50))
	fmt.Println(fmt.Errorf("%[2]w %[1]w", ValErr{1}, ValErr{2}), fmt.Errorf("%w %[1]w", base))
	fmt.Println(len(unwrapMany(fmt.Errorf("%[2]w %[1]w", ValErr{1}, ValErr{2}))), len(unwrapMany(fmt.Errorf("%w %[1]w", base))))
''')

case("fmt_errorf_bad_w",
     "%w is only for Errorf and only for errors: Sprintf, Printf and Errorf with a non-error operand print the error form %!w(...); the other verbs work in Errorf as in Sprintf.",
     "type E struct{}\n\nfunc (E) Error() string { return \"E\" }\n",
     '''	fmt.Println(fmt.Sprintf("%w", E{}))
	fmt.Printf("%w|%w\\n", E{}, 3)
	e := fmt.Errorf("a %w b", 5)
	fmt.Println(e)
	fmt.Printf("%T\\n", e)
	e = fmt.Errorf("%w %w", E{}, "str")
	fmt.Println(e)
	fmt.Printf("%T\\n", e)
	e = fmt.Errorf("%v %+v %#v %s %d %x %q %10v|%-6s|", E{}, E{}, E{}, E{}, E{}, E{}, E{}, E{}, E{})
	fmt.Println(e)
	e = fmt.Errorf("%d items, %5.2f%%, %v", 3, 99.5, []int{1})
	fmt.Println(e)
	e = fmt.Errorf("%d %d", 1)
	fmt.Println(e)
	e = fmt.Errorf("x", 1, "y")
	fmt.Println(e)
''')

# ---- Printf engine ----
case("fmt_printf_errors",
     "Printf's error forms: %!d(string=x) wrong type, %!d(MISSING), %!(EXTRA int=2, string=y), %!(NOVERB), %!z(int=1) unknown verb, %!(BADWIDTH), %!(BADPREC), %!d(BADINDEX), nil operands.",
     "type S string\n\ntype T struct{ A int }\n",
     '''	fmt.Printf("%d %s\\n", "x", 5)
	fmt.Printf("%d %d\\n", 1)
	fmt.Printf("%d\\n", 1, 2, "y", nil, S("s"), T{3})
	fmt.Printf("%z %!\\n", 1)
	fmt.Printf("%d %")
	fmt.Println()
	fmt.Printf("%")
	fmt.Println()
	fmt.Printf("%*d|%-*d|%.*f\\n", 5, 42, 5, 42, 2, 3.14159)
	fmt.Printf("%*d\\n", "x", 42)
	fmt.Printf("%.*d\\n", "x", 42)
	fmt.Printf("%*d|\\n", -5, 42)
	fmt.Printf("%.*d\\n", -1, 42)
	fmt.Printf("%*d\\n")
	fmt.Printf("%.*d %d\\n", 3)
	fmt.Printf("%[3]d %[1]d\\n", 1, 2)
	fmt.Printf("%[0]d %[x]d %[1\\n", 1)
	fmt.Printf("%[1]d %[1]d %[2]d\\n", 1, 2)
	fmt.Printf("%[2]d %d %d\\n", 1, 2, 3)
	fmt.Printf("%d %[1]d %d\\n", 10, 20)
	fmt.Printf("%[1]*d|%[3]*.[2]*[1]f|\\n", 7, 2, 12.0)
	fmt.Printf("%[2]*[1]d|\\n", 5, 6)
	fmt.Printf("%d %d %[1]d\\n", 1, 2, 3)
	fmt.Printf("%[2]d\\n", 1, 2, 3)
	fmt.Printf("%d%%\\n", 100)
	fmt.Printf("%5%|%-5%|%05%\\n")
	fmt.Printf("%s %v %d\\n", nil, nil, nil)
	fmt.Printf("%!d %!%\\n", 1)
	fmt.Printf("%q\\n", fmt.Sprintf("\\xff%d\\xc3", 1))
	fmt.Printf("%é %日\\n", 1, 2)
	fmt.Printf("%.d|%.0d|%5.d|\\n", 0, 0, 0)
	fmt.Printf("%q %q\\n", fmt.Sprintf("%99999999d", 1), fmt.Sprintf("%.99999999f", 1.0))
	fmt.Printf("%-+ 08.3d|\\n", 5)
	fmt.Printf("%#+ -0x|\\n", 255)
''')

case("fmt_print_spacing",
     "Print adds a space between operands when neither is a string; Println always adds spaces and a newline; Sprint, Sprintln, Print with no operands and with nil.",
     "type S string\n\ntype E struct{}\n\nfunc (E) Error() string { return \"err\" }\n\ntype St struct{}\n\nfunc (St) String() string { return \"st\" }\n",
     '''	fmt.Print(1, 2, 3, "\\n")
	fmt.Print("a", "b", "c", "\\n")
	fmt.Print("a", 1, "b", 2, 3, "c", "\\n")
	fmt.Print(1, "a", 2, "\\n")
	fmt.Print(1.5, 2.5, true, false, 'x', "\\n")
	fmt.Print(S("a"), S("b"), 1, S("c"), "\\n")
	fmt.Print(E{}, E{}, St{}, St{}, "\\n")
	fmt.Print(nil, nil, 1, nil, "x", nil, "\\n")
	fmt.Print([]int{1}, []int{2}, "\\n")
	fmt.Print("\\n")
	fmt.Print()
	fmt.Println()
	fmt.Println(1, 2, 3)
	fmt.Println("a", "b")
	fmt.Println("a", 1, "b")
	fmt.Println(nil, nil)
	fmt.Println(S("x"), E{}, St{}, [2]St{}, "end")
	fmt.Printf("%q %q %q %q\\n", fmt.Sprint(), fmt.Sprint(1, 2), fmt.Sprint("a", "b"), fmt.Sprint("a", 1, 2, "b"))
	fmt.Printf("%q %q %q\\n", fmt.Sprintln(), fmt.Sprintln(1, "a", 2), fmt.Sprintln("a"))
	fmt.Printf("%q %q\\n", fmt.Sprint(1, nil, 2), fmt.Sprint(nil, nil))
	fmt.Printf("%q %q\\n", fmt.Sprint(St{}, 1, St{}), fmt.Sprint("x", St{}, "y"))
	var e error
	var i any
	fmt.Print(e, i, "|", e, i, "\\n")
	fmt.Println(e, i)
''')

case("fmt_sprintf_widths_composites",
     "Width, precision and flags apply to each element of a composite value, to each key and value of a map, and to each field of a struct, not to the whole.",
     "type P struct {\n\tX float64\n\tY string\n\tZ []int\n}\n",
     '''	p := P{3.14159, "ab", []int{1, 22}}
	fmt.Printf("[%8.2v]\\n[%-8v]\\n[%08v]\\n[%+v]\\n[%+.1f]\\n", p, p, p, p, p)
	fmt.Printf("[%5v]\\n[%-5v]\\n[%05d]\\n[%6.3v]\\n", []int{1, 22}, []int{1, 22}, []int{1, 22}, []float64{1.23456, 2})
	fmt.Printf("[%5v]\\n[%-5v]\\n[%5s]\\n", map[string]int{"a": 1, "bb": 22}, map[string]int{"a": 1}, map[string]string{"k": "v"})
	fmt.Printf("[%5v]\\n[%-7v]\\n[%3d]\\n", [2][2]int{{1, 2}, {3, 4}}, struct{ A, B int }{1, 2}, &struct{ A, B int }{1, 2})
	fmt.Printf("[%6.2f]\\n[%6.2f]\\n[%x]\\n[%X]\\n", []float64{1.5, 2.25}, [2]float64{3, 4}, []int{255, 16}, []string{"hi", "yo"})
	fmt.Printf("[%10v]\\n[%-10v]\\n", "str", "str")
	fmt.Printf("[%10v]\\n[%-10v]\\n", true, nil)
	fmt.Printf("[%q]\\n[%5q]\\n[%x]\\n[% x]\\n[%#x]\\n", []string{"a", "b"}, []string{"a"}, "hello", "hello", "hello")
	fmt.Printf("[% d]\\n[%+d]\\n[%+.1e]\\n", []int{1, -2}, []int{1, -2}, []float64{1234.5})
''')

case("fmt_pointers_p",
     "%p and %v of pointers print addresses (replaced by 0xP here); %d %x %o %b format the address as a number; slices, maps, funcs and unsafe pointers with %p; nil pointers.",
     NORM + "type T struct{ n int }\n",
     '''	x := 1
	t := &T{1}
	sl := []int{1}
	m := map[string]int{}
	f := func() {}
	var np *int
	var ns []int
	var nm map[int]int
	var nf func()
	for _, f := range []string{"%p", "%v", "%+v", "%#v", "%s", "%t", "%T", "%10p|", "%-10p|", "%#x", "%#v"} {
		fmt.Printf("%-5s ", f)
		fmt.Println(norm(fmt.Sprintf(f, &x)), norm(fmt.Sprintf(f, t)), norm(fmt.Sprintf(f, np)))
	}
	for _, f := range []string{"%d", "%x", "%X", "%o", "%b", "%#p", "%08x", "%+d"} {
		fmt.Printf("%-5s %v %v\\n", f, len(fmt.Sprintf(f, &x)) > 3, fmt.Sprintf(f, np))
	}
	fmt.Println(norm(fmt.Sprintf("%p %p %p", sl, m, f)), norm(fmt.Sprintf("%p %p %p", ns, nm, nf)))
	fmt.Println(norm(fmt.Sprintf("%v %v %v", &sl, &m, &f)))
	fmt.Println(fmt.Sprintf("%p", sl)[:2], fmt.Sprintf("%p", t)[:2], fmt.Sprintf("%p", &x) == fmt.Sprintf("%v", &x))
	fmt.Println(fmt.Sprintf("%d", &x) != "", fmt.Sprintf("%v", f)[:2], fmt.Sprintf("%v", nf), fmt.Sprintf("%v", ns))
	var a, b any = &x, &x
	fmt.Println(a == b, fmt.Sprintf("%p", a) == fmt.Sprintf("%p", b))
	fmt.Printf("%p %p\\n", nil, 5)
''')

case("fmt_depth_pointers",
     "A pointer to a struct, array, slice or map prints as &{...} only at the top level; one level down it prints its address; pointers inside slices and maps print addresses.",
     NORM + "type N struct {\n\tV int\n\tP *N\n}\n",
     '''	n := &N{1, &N{2, nil}}
	fmt.Println(norm(fmt.Sprint(n)))
	fmt.Println(norm(fmt.Sprint(*n)))
	fmt.Println(norm(fmt.Sprint([]*N{n})))
	fmt.Println(norm(fmt.Sprint(map[string]*N{"a": n})))
	fmt.Println(norm(fmt.Sprint(struct{ P *N }{n})))
	fmt.Println(norm(fmt.Sprint([1]*[]int{&[]int{1}})))
	pp := &n
	fmt.Println(norm(fmt.Sprint(pp)))
	var a any = n
	fmt.Println(norm(fmt.Sprint(a)))
	fmt.Println(norm(fmt.Sprint([]any{n, *n})))
	fmt.Println(norm(fmt.Sprintf("%+v %#v", n, n)))
	arr := &[2]int{1, 2}
	sl := &[]string{"a"}
	mp := &map[int]int{1: 2}
	fmt.Println(arr, sl, mp, &arr != nil)
	fmt.Println(norm(fmt.Sprint(&arr, &sl)))
	fmt.Printf("%v %+v %#v %d\\n", arr, arr, arr, arr)
	fmt.Printf("%s %x\\n", &[]string{"s"}, &[]int{255})
''')

case("fmt_fprint_writers",
     "Fprint, Fprintln and Fprintf write to os.Stdout and os.Stderr and return Go's (n int, err error); Print, Println and Printf return the same; Append, Appendf and Appendln append to a byte slice.",
     "",
     '''	n, err := fmt.Fprintln(os.Stdout, "to stdout", 1, 2)
	fmt.Println(n, err)
	n, err = fmt.Fprintf(os.Stdout, "%d-%s\\n", 7, "x")
	fmt.Println(n, err)
	n, err = fmt.Fprint(os.Stdout, "a", 1, 2, "b\\n")
	fmt.Println(n, err)
	n, err = fmt.Fprintln(os.Stderr, "to stderr")
	fmt.Println(n, err)
	n, err = fmt.Fprintf(os.Stderr, "stderr %d\\n", 1)
	n, err = fmt.Print("print ", 1, "\\n")
	fmt.Println(n, err)
	n, err = fmt.Printf("%s\\n", "printf")
	fmt.Println(n, err)
	n, err = fmt.Println("println", 3)
	fmt.Println(n, err == nil)
	_, _ = fmt.Println()
	b := []byte("x:")
	b = fmt.Append(b, 1, 2, "s")
	b = fmt.Appendf(b, " %05d", 42)
	b = fmt.Appendln(b, "end", 1)
	fmt.Printf("%q\\n", b)
	fmt.Println(string(fmt.Append(nil, "only")), fmt.Appendf(nil, "%d", 12), len(fmt.Appendln(nil)))
	s := fmt.Sprintf("%v", 1)
	fmt.Println(s, fmt.Sprint(), fmt.Sprintln() == "\\n")
''', imports=("fmt", "os"), stderr=".*")

case("fmt_errorf_errors_pkg",
     "Errors made by Errorf work with package errors: Is and As find the wrapped errors through Unwrap() error and Unwrap() []error, errors.Unwrap peels one %w, Join and Errorf mix; %T names stay Go's.",
     '''type NotFound struct{ Name string }

func (e *NotFound) Error() string { return "not found: " + e.Name }

type Temp struct{}

func (Temp) Error() string   { return "temp" }
func (Temp) Temporary() bool { return true }

var ErrBase = errors.New("base")
''',
     '''	e1 := fmt.Errorf("layer1: %w", ErrBase)
	e2 := fmt.Errorf("layer2: %w", e1)
	fmt.Println(e2, errors.Is(e2, ErrBase), errors.Is(e2, e1), errors.Is(e1, e2))
	fmt.Println(errors.Unwrap(e2) == e1, errors.Unwrap(e1) == ErrBase, errors.Unwrap(ErrBase) == nil)
	nf := &NotFound{"x"}
	e3 := fmt.Errorf("lookup: %w", nf)
	var target *NotFound
	fmt.Println(errors.As(e3, &target), target == nf, target.Name)
	var tmp interface{ Temporary() bool }
	e4 := fmt.Errorf("a: %w, b: %w", Temp{}, ErrBase)
	fmt.Println(e4, errors.Is(e4, ErrBase), errors.As(e4, &tmp), tmp != nil && tmp.Temporary())
	fmt.Println(errors.Unwrap(e4) == nil)
	e5 := errors.Join(e1, fmt.Errorf("other"), nil)
	fmt.Printf("%q %v\\n", e5.Error(), errors.Is(e5, ErrBase))
	e6 := fmt.Errorf("outer: %w", e5)
	fmt.Println(errors.Is(e6, ErrBase), fmt.Sprint(e6) == "outer: layer1: base\\nother")
	fmt.Printf("%T %T %T %T %T\\n", e1, e4, ErrBase, e5, fmt.Errorf("plain"))
	fmt.Println(errors.Is(fmt.Errorf("%w", nil), nil), fmt.Errorf("no wrap %v", ErrBase) == ErrBase, errors.Is(fmt.Errorf("no wrap %v", ErrBase), ErrBase))
''', imports=("errors", "fmt"))
