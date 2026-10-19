// phase: 07

package main

import "fmt"

func Cnt(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func a() {
	for x := range Cnt(2) {
		// not called by a deferred function: nil
		fmt.Println("recover in body", x, recover())
	}
}

func b() {
	defer func() {
		// the loop is in the deferred function: its recover stops the panic only when called by
		// the function itself
		for x := range Cnt(1) {
			fmt.Println("loop in deferred", x)
		}
		fmt.Println("deferred recover", recover())
	}()
	panic("b panic")
}

func c() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
		}
	}()
	for x := range Cnt(5) {
		if x == 2 {
			var a []int
			_ = a[x]
		}
	}
	return nil
}

func d() (n int) {
	defer func() {
		recover()
	}()
	for x := range Cnt(5) {
		n += x
		if x == 3 {
			panic("stop")
		}
	}
	return -1
}

func main() {
	a()
	b()
	fmt.Println(c())
	fmt.Println(d())
}

// Output:
// recover in body 0 <nil>
// recover in body 1 <nil>
// loop in deferred 0
// deferred recover b panic
// recovered: runtime error: index out of range [2] with length 0
// 6
