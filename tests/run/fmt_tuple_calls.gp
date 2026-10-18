// phase: 05

// Print, Println, Sprint and Sprintln of a call with several results pass every result as an
// operand; Printf returns (n, err) which can be used; results of different kinds.
package main

import "fmt"

type S struct{ A int }

func two() (int, string) { return 1, "a" }

func three() (S, []int, error) { return S{2}, []int{3}, nil }

func four() (int64, uint8, float32, map[string]bool) {
	return 5, 6, 7.5, map[string]bool{"k": true}
}

func pair() (n int, err error) { return fmt.Println("in pair") }

func main() {
	fmt.Println(two())
	fmt.Println(three())
	fmt.Println(four())
	fmt.Print(two())
	fmt.Print("\n")
	s := fmt.Sprint(three())
	fmt.Println(s, len(fmt.Sprintln(four())))
	n, err := pair()
	fmt.Println(n, err)
	n, err = fmt.Printf("%d%s\n", 1, "x")
	fmt.Println(n, err)
	if _, err := fmt.Println(); err != nil {
		fmt.Println("error")
	}
	fmt.Printf("%T\n", err)
}

// Output:
// 1 a
// {2} [3] <nil>
// 5 6 7.5 map[k:true]
// 1a
// {2} [3] <nil> 20
// in pair
// 8 <nil>
// 1x
// 3 <nil>
//
// <nil>
