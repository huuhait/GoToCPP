// phase: 03

// range over an array in a block that only a temporary owns: the loop holds its own copy, so
// the values stay valid while it runs (*f(), f()[i], **f(), *(&[N]T{...})).
package main

import "fmt"

func mk() *[3]int {
	return &[3]int{1, 2, 3}
}

func mks() [][3]int {
	return [][3]int{{4, 5, 6}, {7, 8, 9}}
}

func mkpp() **[3]int {
	p := &[3]int{10, 20, 30}
	return &p
}

func main() {
	sum := 0
	for _, v := range *mk() {
		sum += v
	}
	fmt.Println(sum)
	for i, v := range mks()[1] {
		sum += i * v
	}
	fmt.Println(sum)
	for _, v := range **mkpp() {
		sum += v
	}
	fmt.Println(sum)
	for _, v := range *(&[3]int{100, 200, 300}) {
		sum += v
	}
	fmt.Println(sum)
	for _, s := range *(&[2]string{"he", "llo"}) {
		fmt.Print(s)
	}
	fmt.Println()
}
