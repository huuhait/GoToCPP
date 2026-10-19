// phase: 05

// The first report: a return in a function with a named result and a defer stored into the
// inner variable of the same name (it compiled, and returned the result's zero value): a range
// variable, a block, an if init, a swap of two results.
package main

import "fmt"

func F(xs []int, v int) (i int, p any) {
	defer func() { p = recover() }()
	for i, x := range xs {
		if x == v {
			return i, nil
		}
	}
	return -1, nil
}

func block() (i int) {
	defer func() {}()
	{
		i := 7
		return i
	}
}

func ifInit() (i int) {
	defer func() {}()
	if i := 9; i > 0 {
		return i
	}
	return 1
}

func swap() (i, j int) {
	i, j = 1, 2
	defer func() {}()
	return j, i
}

func swapInner() (i, j int) {
	i, j = 1, 2
	defer func() {}()
	for i := 5; i < 6; i++ {
		return j, i
	}
	return
}

func main() {
	fmt.Println(F([]int{5, 6, 7}, 7))
	fmt.Println(F([]int{5, 6, 7}, 9))
	fmt.Println(block())
	fmt.Println(ifInit())
	fmt.Println(swap())
	fmt.Println(swapInner())
}

// Output:
// 2 <nil>
// -1 <nil>
// 7
// 9
// 2 1
// 2 5
