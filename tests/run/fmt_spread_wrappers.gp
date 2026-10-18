// phase: 05
// Stderr: .*

// A final argument a... (a []any) is the list of operands, not one operand: logf(f, a...)
// wrappers around Printf, Sprintf, Println, Print, Sprint, Sprintln, Fprintf, Fprintln, Errorf,
// Appendf; nil and empty slices; slice literals; nested wrappers; append results.
package main

import (
	"errors"
	"fmt"
	"os"
)

type S struct{ A int }

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

func main() {
	logf("%d %s %v\n", 1, "a", S{2})
	logf("no args\n")
	logf("%d %s\n")
	logf("%d\n", 1, 2, 3)
	logln("x", 1, 2.5, nil, S{3}, []int{4})
	logln()
	logp("a", "b", 1, 2, "c\n")
	fmt.Println(sf("%5d|%-5s|%v|%T", 7, "ab", S{1}, S{1}), sp(1, 2, "x", 3), len(sln("a", "b")))
	fmt.Println(sf("%v %v", []any{1, "z"}...), sf("%v", []any{[]any{1, 2}}...), sf("%v", []any{}))
	var nilargs []any
	logln(nilargs...)
	logf("nil: %d\n", nilargs...)
	fmt.Println(sp(nilargs...) == "", sf("x", nilargs...), sln(nilargs...) == "\n")
	outer("%s-%s-%d\n", "p", "q", 3)
	fmt.Println(deep("%v|%v", 1, nil), deep("%d", S{}) )
	args := []any{"w", 5}
	logf("%s %d\n", args...)
	logln(args...)
	logf("%s %d %v\n", append([]any{"first"}, args...)...)
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
	fe("to stderr %d\n", 9)
	fe("%s\n", "hi")
	fmt.Printf("%q\n", app([]byte("<"), "%d,", 1, "x"))
	fmt.Printf("%q\n", app(nil, "none"))
	mixed := []any{1, int8(2), uint(3), 4.5, true, 'r', "s", S{9}, &S{8}, map[string]int{"k": 1}, [2]int{5, 6}, nil, fmt.Errorf("err")}
	logf("%v %v %v %v %v %v %v %v %v %v %v %v %v\n", mixed...)
	logf("%T %T %T %T %T %T %T %T %T %T %T %T\n", mixed[:12]...)
	logf("%d|%+v|%#v|%x|%q\n", mixed[:5]...)
	logf("%[2]v %[1]v %*d %.*f\n", mixed[:4]...)
	var sw any = []any{1, 2}
	fmt.Println(sw)
	defer logln(args...)
	defer fmt.Printf("%v %v\n", args...)
	fmt.Println("end")
}

// Output:
// 1 a S!
// no args
// %!d(MISSING) %!s(MISSING)
// 1
// %!(EXTRA int=2, int=3)x 1 2.5 <nil> S! [4]
//
// ab1 2c
//     7|ab   |S!|main.S 1 2x3 4
// 1 z [1 2] []
//
// nil: %!d(MISSING)
// true x true
// p-q-3
// 1|<nil> {0}
// w 5
// w 5
// first %!d(string=w) 5
// 2 2 0 0 0
// wrap %!w(main.S={1}) and e0, 3
// one + two true 2
// only one true
// 9
// 9
// hi
// hi
// "<1,%!(EXTRA string=x)1x1 x\n"
// "none\n"
// 1 2 3 4.5 true 114 s S! S! map[k:1] [5 6] <nil> err
// int int8 uint float64 bool int32 string main.S *main.S map[string]int [2]int <nil>
// 1|2|0x3|0x1.2p+02|%!q(bool=true)
// 2 1  3 %!(BADPREC)%!f(MISSING)
// [1 2]
// end
// w 5
// w 5
