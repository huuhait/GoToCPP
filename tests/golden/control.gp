// phase: 02

// The control flow of Phase 02: the for forms, a range over an integer
// whose body declares the range variable again, the three switch shapes
// (a C++ switch, an if-else chain, a case index for fallthrough), labeled
// break and continue, and goto.
package main

import "fmt"

func main() {
	odd := 0
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			continue
		}
		odd += i
	}
	n, steps := 27, 0
	for n != 1 {
		if n%2 == 0 {
			n /= 2
		} else {
			n = 3*n + 1
		}
		steps++
	}
	for i := range 3 {
		i := i * 10
		fmt.Print(i, " ")
	}
	fmt.Println(odd, steps)

	switch day := steps % 7; day {
	case 0, 6:
		fmt.Println("weekend")
	case 1:
		fmt.Println("monday")
		fallthrough
	default:
		fmt.Println("weekday")
	}
	switch lang := "go"; lang {
	case "c", "c++":
		fmt.Println("old")
	case "go":
		fmt.Println("new")
	}
	switch {
	case odd > 20:
		fmt.Println("big")
		fallthrough
	case odd > 10:
		fmt.Println("medium")
	default:
		fmt.Println("small")
	}

outer:
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if j == 2 {
				continue outer
			}
			if i == 2 {
				break outer
			}
			fmt.Println(i, j)
		}
	}
	k := 0
again:
	if k < 3 {
		k++
		goto again
	}
	fmt.Println("k", k)
}
