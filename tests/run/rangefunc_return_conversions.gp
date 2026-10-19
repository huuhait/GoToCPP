// phase: 07

package main

import "fmt"

type MyErr struct{ code int }

func (e *MyErr) Error() string { return "myerr " + fmt.Sprint(e.code) }

type Stringer interface{ String() string }

type S string

func (s S) String() string { return string(s) }

func Cnt(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func f(n int) error {
	for x := range Cnt(n) {
		if x == 2 {
			return &MyErr{x}
		}
	}
	return nil
}

func g(n int) (Stringer, error) {
	for x := range Cnt(n) {
		if x == 1 {
			return S("one"), nil
		}
	}
	return nil, fmt.Errorf("no")
}

func two() (int, int) { return 7, 8 }

func h() (int, int) {
	for range Cnt(1) {
		return two()
	}
	return 0, 0
}

func k() (float64, any) {
	for x := range Cnt(1) {
		return 3, x
	}
	return 0, nil
}

func main() {
	fmt.Println(f(5), f(1) == nil)
	fmt.Println(g(5))
	fmt.Println(g(1))
	fmt.Println(h())
	fmt.Println(k())
}

// Output:
// myerr 2 true
// one <nil>
// <nil> no
// 7 8
// 3 0
