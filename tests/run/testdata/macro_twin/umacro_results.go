// The plain-Go twin of umacro_results.gp: the code of VM written into each function.

package main

import (
	"errors"
	"fmt"
)

func g(fail bool) error {
	if fail {
		return errors.New("g failed")
	}
	return nil
}

func pair(a int) (int, string) { return a * 2, "pair" }

func shadowErr(fail bool) (n int, err error) {
	fmt.Println("vm", "main.shadowErr")
	defer fmt.Println("vm end", "main.shadowErr")
	if err := g(fail); err != nil {
		return 7, err
	}
	return 1, nil
}

func shadowN() (n int) {
	fmt.Println("vm", "main.shadowN")
	defer fmt.Println("vm end", "main.shadowN")
	{
		n := 10
		_ = n
		return 5
	}
}

func shadowLoop() (n int) {
	fmt.Println("vm", "main.shadowLoop")
	defer fmt.Println("vm end", "main.shadowLoop")
	n = 1
	for n := 0; n < 3; n++ {
		if n == 2 {
			return n + 100
		}
	}
	return
}

func forward(a int) (int, string) {
	fmt.Println("vm", "main.forward")
	defer fmt.Println("vm end", "main.forward")
	return pair(a)
}

func bare(a int) (sum int, label string) {
	fmt.Println("vm", "main.bare")
	defer fmt.Println("vm end", "main.bare")
	label = "bare"
	for i := range a {
		sum += i
	}
	return
}

func deferred(a int) (n int, err error) {
	fmt.Println("vm", "main.deferred")
	defer fmt.Println("vm end", "main.deferred")
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
			n = -1
		}
	}()
	defer func() { n *= 2 }()
	n = a
	if a > 3 {
		panic("big")
	}
	return n + 1, nil
}

func unnamed(a int) (int, error) {
	fmt.Println("vm", "main.unnamed")
	defer fmt.Println("vm end", "main.unnamed")
	if a < 0 {
		return 0, errors.New("negative")
	}
	return a, nil
}

func blank(a int) (_ int, ok bool) {
	fmt.Println("vm", "main.blank")
	defer fmt.Println("vm end", "main.blank")
	return a, a > 0
}

func main() {
	fmt.Println(shadowErr(true))
	fmt.Println(shadowErr(false))
	fmt.Println(shadowN())
	fmt.Println(shadowLoop())
	fmt.Println(forward(4))
	fmt.Println(bare(5))
	fmt.Println(deferred(1))
	fmt.Println(deferred(9))
	fmt.Println(unnamed(-1))
	fmt.Println(unnamed(3))
	fmt.Println(blank(2))
}
