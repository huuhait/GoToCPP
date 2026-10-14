// phase: 03

// Go 1.22 loop variables: each iteration of a 3-clause for and of a range loop has its own
// variables, which closures keep; the post statement works on the next iteration's copy, so a
// change in the body (i += 2) is seen by the post statement, and a closure of an earlier
// iteration keeps its own value.
package main

import "fmt"

func main() {
	var fs []func() int
	for i := 0; i < 3; i++ {
		fs = append(fs, func() int { return i })
	}
	for i := 0; i < 10; i++ {
		fs = append(fs, func() int { return i * 100 })
		i += 3
	}
	for k, v := range []int{7, 8} {
		fs = append(fs, func() int { return k*10 + v })
	}
	for n := range 2 {
		fs = append(fs, func() int { return -n })
	}
	for _, f := range fs {
		fmt.Print(f(), ",")
	}
	fmt.Println()

	// A closure that changes its iteration's variable: the loop goes on from the change.
	var incs []func()
	for i := 0; i < 6; i++ {
		inc := func() { i++ }
		incs = append(incs, inc)
		inc()
		fmt.Print(i, ",")
	}
	fmt.Println(len(incs))

	// Deferred closures in a loop: each sees its own iteration's variable.
	func() {
		for i := range 3 {
			defer func() { fmt.Print("d", i, ";") }()
		}
	}()
	fmt.Println()
}

// Output:
// 0,1,2,300,700,1100,7,18,0,-1,
// 1,3,5,3
// d2;d1;d0;
