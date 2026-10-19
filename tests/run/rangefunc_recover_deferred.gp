// phase: 07
package main

import "fmt"

func g(n int) func(func(int) bool) {
	return func(yield func(int) bool) {
		for i := 0; i < n; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

func recInIter(yield func(int) bool) {
	fmt.Println("iter recover:", recover())
	yield(1)
}

func t1() {
	defer func() {
		for i := range g(2) {
			fmt.Println("t1 body", i, "recover:", recover())
		}
	}()
	panic("t1 panic")
}

func t2() {
	defer func() {
		for i := range recInIter {
			fmt.Println("t2 body", i)
		}
		fmt.Println("t2 after loop, recover:", recover())
	}()
	panic("t2 panic")
}

func t3() {
	defer func() {
		for i := range g(1) {
			func() {
				fmt.Println("t3 nested func recover:", recover())
			}()
			_ = i
		}
		fmt.Println("t3 end recover:", recover())
	}()
	panic("t3 panic")
}

func t4() (r int) {
	defer func() { fmt.Println("t4 outer", recover()) }()
	defer func() {
		for i := range g(3) {
			if v := recover(); v != nil {
				r = 100 + i
				fmt.Println("t4 recovered", v)
			}
		}
	}()
	panic("t4")
}

func t5() {
	defer func() {
		for i := range g(2) {
			for j := range g(2) {
				if v := recover(); v != nil {
					fmt.Println("t5 recovered", v, i, j)
				}
			}
		}
	}()
	panic("t5")
}

func t6() {
	defer func() {
		defer func() { fmt.Println("t6 inner deferred recover:", recover()) }()
		for i := range g(2) {
			defer func() { fmt.Println("t6 body-defer", i, recover()) }()
		}
	}()
	panic("t6")
}

func run(name string, f func()) {
	defer func() { fmt.Println(name, "escaped:", recover()) }()
	f()
}

func main() {
	run("t1", t1)
	run("t2", t2)
	run("t3", t3)
	fmt.Println(t4())
	run("t5", t5)
	run("t6", t6)
}

// Output:
// t1 body 0 recover: <nil>
// t1 body 1 recover: <nil>
// t1 escaped: t1 panic
// iter recover: <nil>
// t2 body 1
// t2 after loop, recover: t2 panic
// t2 escaped: <nil>
// t3 nested func recover: <nil>
// t3 end recover: t3 panic
// t3 escaped: <nil>
// t4 outer t4
// 0
// t5 escaped: t5
// t6 body-defer 1 <nil>
// t6 body-defer 0 <nil>
// t6 inner deferred recover: <nil>
// t6 escaped: t6
