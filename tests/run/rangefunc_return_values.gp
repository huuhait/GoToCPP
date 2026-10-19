// phase: 07

package main

import "fmt"

func Count(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		defer fmt.Println("cleanup", n)
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func first(n, target int) (int, string) {
	for x := range Count(n) {
		if x == target {
			return x * 100, fmt.Sprint("found ", x)
		}
	}
	return -1, "none"
}

func named(n int) (res int, err error) {
	for x := range Count(n) {
		for y := range Count(n) {
			if x*y == 6 {
				res = x + y
				return
			}
		}
	}
	err = fmt.Errorf("none for %d", n)
	return
}

func nested(n int) int {
	for x := range Count(n) {
		for y := range Count(n) {
			for z := range Count(n) {
				if x+y+z == 7 {
					return x*100 + y*10 + z
				}
			}
		}
	}
	return 0
}

func conv(n int) (any, error) {
	for x := range Count(n) {
		if x == 2 {
			return x, nil
		}
	}
	return "nothing", fmt.Errorf("e")
}

func voidFn(n int) {
	for x := range Count(n) {
		if x == 1 {
			fmt.Println("early return")
			return
		}
	}
	fmt.Println("not reached when n > 1")
}

func shadow() (r int) {
	for r := range Count(3) {
		if r == 2 {
			return 7
		}
	}
	return 0
}

func main() {
	fmt.Println(first(5, 3))
	fmt.Println(first(2, 3))
	fmt.Println(named(5))
	fmt.Println(named(2))
	fmt.Println(nested(5))
	fmt.Println(nested(2))
	fmt.Println(conv(5))
	fmt.Println(conv(1))
	voidFn(3)
	voidFn(1)
	fmt.Println(shadow())
}

// Output:
// cleanup 5
// 300 found 3
// cleanup 2
// -1 none
// cleanup 5
// cleanup 5
// cleanup 5
// cleanup 5
// 5 <nil>
// cleanup 2
// cleanup 2
// cleanup 2
// 0 none for 2
// cleanup 5
// cleanup 5
// cleanup 5
// cleanup 5
// cleanup 5
// cleanup 5
// 34
// cleanup 2
// cleanup 2
// cleanup 2
// cleanup 2
// cleanup 2
// cleanup 2
// cleanup 2
// 0
// cleanup 5
// 2 <nil>
// cleanup 1
// nothing e
// early return
// cleanup 3
// cleanup 1
// not reached when n > 1
// cleanup 3
// 7
