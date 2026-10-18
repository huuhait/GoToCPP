// phase: 08

// A receive as an operand: in arguments, literals, assignments, returns, conditions and unary
// expressions, evaluated left to right.
package main

import "fmt"

type pair struct{ a, b int }

func fill(vals ...int) chan int {
	c := make(chan int, len(vals)+1)
	for _, v := range vals {
		c <- v
	}
	return c
}

func two(c chan int) (int, int) { return <-c, <-c }

func sum3(a, b, c int) int { return a*100 + b*10 + c }

func main() {
	c := fill(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18)
	fmt.Println(sum3(<-c, <-c, <-c))
	fmt.Println(pair{<-c, <-c}, pair{b: <-c, a: <-c})
	fmt.Println([]int{<-c, <-c, <-c})
	fmt.Println(map[int]int{<-c: <-c})
	s := append([]int{}, <-c, <-c)
	fmt.Println(s)
	x := 0
	x += <-c
	x -= <-c
	fmt.Println(x)
	a, b := two(c)
	fmt.Println(a, b)

	d := fill(5, 6, 7)
	var arr [3]int
	arr[0], arr[1] = <-d, <-d
	p := &arr[2]
	*p = <-d
	fmt.Println(arr)

	e := fill(10, 3)
	fmt.Println(-<-e, !(<-e > 5))
	f := fill(1, 0)
	fmt.Println(<-f == 1 && <-f == 0)
	g := fill(1, 2, 3, 4)
	for v := <-g; v < 3; v = <-g {
		fmt.Print(v, ",")
	}
	fmt.Println()
	switch <-g {
	case 4:
		fmt.Println("switch on a receive")
	default:
		fmt.Println("no")
	}
	h := fill(7)
	if v := <-h; v > 5 {
		fmt.Println("if with init", v)
	}
	cc := make(chan chan int, 1)
	cc <- fill(55)
	fmt.Println(<-<-cc)
	out := make(chan int, 1)
	in := fill(66)
	out <- <-in
	fmt.Println(<-out)
}

// Output:
// 123
// {4 5} {7 6}
// [8 9 10]
// map[11:12]
// [13 14]
// -1
// 17 18
// [5 6 7]
// -10 true
// true
// 1,2,
// switch on a receive
// if with init 7
// 55
// 66
