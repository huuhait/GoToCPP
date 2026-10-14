// phase: 03

// range over an array: the loop ranges over a copy of the array (writes in the body are not
// seen), through a pointer it doesn't (they are), the range variables are copies of hidden
// counters (Go 1.22), a loop without a value variable counts to N without evaluating the array
// when its length is a constant (also through a nil pointer), and evaluates it when it has a
// call.
package main

import "fmt"

var ga = [3]int{1, 2, 3}

var calls int

func get() [3]int {
	calls++
	return [3]int{7, 8, 9}
}

func change() {
	ga[2] = 30
}

func main() {
	a := [3]int{1, 2, 3}
	for i, v := range a {
		if i == 0 {
			a[1] = 20
			a[2] = 30
		}
		fmt.Print(v, " ")
	}
	fmt.Println(a[1], a[2])
	for i, v := range ga {
		if i == 0 {
			change()
		}
		fmt.Print(v, " ")
	}
	fmt.Println(ga[2])
	b := [3]int{1, 2, 3}
	p := &b
	for i, v := range p {
		if i == 0 {
			b[2] = 300
			p = nil
		}
		fmt.Print(v, " ")
	}
	fmt.Println(p == nil)
	c := [3]int{1, 2, 3}
	pc := &c
	for _, v := range *pc {
		pc[2] = 3000
		fmt.Print(v, " ")
	}
	fmt.Println(c[2])

	for i, v := range [2]string{"x", "y"} {
		i = 10
		v += "!"
		fmt.Print(i, v, " ")
	}
	fmt.Println("done")
	var np *[4]int
	for i := range np {
		fmt.Print(i)
	}
	for range *np {
		fmt.Print(".")
	}
	for i, _ := range np {
		fmt.Print(i)
	}
	fmt.Println()
	for i := range get() {
		fmt.Print(i)
	}
	for _, v := range get() {
		fmt.Print(v)
	}
	fmt.Println(calls)

	var ptrs []*int
	for _, v := range [3]int{5, 6, 7} {
		ptrs = append(ptrs, &v)
	}
	*ptrs[0] = 50
	fmt.Println(*ptrs[0], *ptrs[1], *ptrs[2])
	var i, v int
	for i, v = range [3]int{7, 8, 9} {
	}
	fmt.Println(i, v)
	w := [4]int{}
	for i, w[i] = range [3]int{7, 8, 9} {
	}
	fmt.Println(i, w[0], w[1], w[2], w[3])
	grid := [2][2]int{{1, 2}, {3, 4}}
	sum := 0
	for _, row := range grid {
		for _, x := range row {
			sum += x
		}
		grid[1][1] = 40
	}
	fmt.Println(sum, grid[1][1])
}

// Output:
// 1 2 3 20 30
// 1 2 3 30
// 1 2 300 true
// 1 2 3 3000
// 10x! 10y! done
// 0123....0123
// 0127892
// 50 6 7
// 2 9
// 2 8 9 7 0
// 10 40
