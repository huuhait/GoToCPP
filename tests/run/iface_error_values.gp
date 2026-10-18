// phase: 05

// The error interface from concrete types: values, pointers, named types; nil errors; several
// results (step 5.4).
package main

import "fmt"

type E1 struct{ code int }

func (e E1) Error() string { return fmt.Sprint("E1:", e.code) }

type E2 struct{ msg string }

func (e *E2) Error() string { return "E2:" + e.msg }

type E3 string

func (e E3) Error() string { return "E3:" + string(e) }

func run(n int) (int, error) {
	switch n {
	case 0:
		return 0, nil
	case 1:
		return 1, E1{11}
	case 2:
		return 2, &E2{"two"}
	case 3:
		return 3, E3("three")
	}
	return -1, &E2{fmt.Sprint("bad ", n)}
}

func main() {
	for i := 0; i < 5; i++ {
		v, err := run(i)
		if err == nil {
			fmt.Println(v, "ok")
			continue
		}
		fmt.Println(v, err.Error())
	}
	var e error = E1{5}
	var e2 error = E1{5}
	var e3 error = E1{6}
	fmt.Println(e == e2, e == e3)
	p1, p2 := &E2{"x"}, &E2{"x"}
	var q1, q2, q3 error = p1, p2, p1
	fmt.Println(q1 == q2, q1 == q3)
	var err error
	fmt.Println(err == nil)
	err = E3("a")
	fmt.Println(err != nil, err == E3("a"), err == E3("b"))
}

// Output:
// 0 ok
// 1 E1:11
// 2 E2:two
// 3 E3:three
// -1 E2:bad 4
// true false
// false true
// true
// true true false
