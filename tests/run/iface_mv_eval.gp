// phase: 05

// Where the receiver of a method value is evaluated (step 5.9): once, at the expression, and
// func values made from interface methods are never nil. Also error values, conversions and
// assertions as receivers, and multiple results passed on.
package main

import "fmt"

type Namer interface{ Name() string }

type N struct{ s string }

func (n N) Name() string { return n.s }

var calls int

func next() Namer {
	calls++
	return N{fmt.Sprint("n", calls)}
}

type MyErr struct{ code int }

func (e *MyErr) Error() string { return fmt.Sprint("code ", e.code) }

func fail(code int) error { return &MyErr{code} }

func pair(f func() (string, error)) string {
	s, err := f()
	return fmt.Sprint(s, "/", err)
}

type Src interface{ Read() (string, error) }
type File struct{ data string }

func (f File) Read() (string, error) {
	if f.data == "" {
		return "", fail(7)
	}
	return f.data, nil
}

func take(s string, err error) string { return fmt.Sprint(s, "|", err) }

func main() {
	f := next().Name
	fmt.Println(calls, f(), f(), calls)
	g := next().Name
	fmt.Println(calls, g(), f())

	var any1 any = N{"asserted"}
	h := any1.(Namer).Name
	any1 = N{"changed"}
	fmt.Println(h())

	k := Namer(N{"converted"}).Name
	fmt.Println(k())

	var nilf func() string
	fmt.Println(nilf == nil, f == nil, f != nil, g != nil)
	var fs [3]func() string
	fs[1] = f
	for i, x := range fs {
		fmt.Println(i, x == nil)
	}

	e := fail(3)
	msg := e.Error
	e = fail(4)
	fmt.Println(msg(), e.Error())
	errs := map[int]func() string{}
	for i := 1; i <= 3; i++ {
		errs[i] = fail(i * 10).Error
	}
	fmt.Println(errs[1](), errs[2](), errs[3]())

	var src Src = File{"payload"}
	rd := src.Read
	fmt.Println(take(rd()))
	fmt.Println(pair(rd))
	src = File{}
	fmt.Println(pair(src.Read))
	fmt.Println(take(rd()))
}

// Output:
// 1 n1 n1 1
// 2 n2 n1
// asserted
// converted
// true false true true
// 0 true
// 1 false
// 2 true
// code 3 code 4
// code 10 code 20 code 30
// payload|<nil>
// payload/<nil>
// /code 7
// payload|<nil>
