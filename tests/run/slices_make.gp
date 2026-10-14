// phase: 03

// make, literals (also keyed ones), nil and empty slices: make zeroes its elements, a keyed
// literal's other elements are zero, and only nil is == nil (an empty slice is not).
package main

import "fmt"

func show(name string, s []int) {
	fmt.Print(name, " len=", len(s), " nil=", s == nil, " [")
	for i, v := range s {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(v)
	}
	fmt.Println("]", cap(s) >= len(s))
}

func main() {
	var n []int
	show("nil", n)
	show("empty", []int{})
	show("make0", make([]int, 0))
	show("make3", make([]int, 3))
	show("make2,10", make([]int, 2, 10))
	fmt.Println(cap(make([]int, 2, 10)))
	show("lit", []int{3, 1, 2})
	show("keyed", []int{4: 1, 2, 1: 9})
	show("conv", []int(nil))
	k := 2
	show("var", make([]int, k, k*2))
	n = []int{1}
	n = nil
	show("again", n)
	strs := make([]string, 2)
	fmt.Println(len(strs[0]), len(strs[1]), strs[0] == "")
	bools := []bool{2: true}
	fmt.Println(len(bools), bools[0], bools[1], bools[2])
	ptrs := make([]*int, 2)
	fmt.Println(ptrs[0] == nil, ptrs[1] == nil)
	nested := make([][]int, 2)
	fmt.Println(nested[0] == nil, len(nested[1]))
	floats := []float64{0.5, 1, 1e100}
	fmt.Println(floats[0], floats[1], floats[2])
	small := []uint8{255, 7: 1}
	fmt.Println(len(small), small[0], small[7])
}

// Output:
// nil len=0 nil=true [] true
// empty len=0 nil=false [] true
// make0 len=0 nil=false [] true
// make3 len=3 nil=false [0 0 0] true
// make2,10 len=2 nil=false [0 0] true
// 10
// lit len=3 nil=false [3 1 2] true
// keyed len=6 nil=false [0 9 0 0 1 2] true
// conv len=0 nil=true [] true
// var len=2 nil=false [0 0] true
// again len=0 nil=true [] true
// 0 0 true
// 3 false false true
// true true
// true 0
// 0.5 1 1e+100
// 8 255 1
