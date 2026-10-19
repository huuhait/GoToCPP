// phase: 07

package main

import "fmt"

func Seq0(n int) func(yield func() bool) {
	return func(yield func() bool) {
		for range n {
			if !yield() {
				return
			}
		}
	}
}

func Seq1(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i * i) {
				return
			}
		}
	}
}

func Seq2(words ...string) func(yield func(int, string) bool) {
	return func(yield func(int, string) bool) {
		for i, w := range words {
			if !yield(i, w) {
				return
			}
		}
	}
}

func main() {
	for range Seq0(3) {
		fmt.Println("tick")
	}
	for x := range Seq1(4) {
		fmt.Println("x", x)
	}
	for range Seq1(2) {
		fmt.Println("no var")
	}
	for k, v := range Seq2("a", "b", "c") {
		fmt.Println(k, v)
	}
	for k := range Seq2("p", "q") {
		fmt.Println("key", k)
	}
	for _, v := range Seq2("p", "q") {
		fmt.Println("val", v)
	}
	for _, _ = range Seq2("z") {
		fmt.Println("blank blank")
	}
	var total int
	for x := range Seq1(5) {
		total += x
	}
	fmt.Println("total", total)
}

// Output:
// tick
// tick
// tick
// x 0
// x 1
// x 4
// x 9
// no var
// no var
// 0 a
// 1 b
// 2 c
// key 0
// key 1
// val p
// val q
// blank blank
// total 30
