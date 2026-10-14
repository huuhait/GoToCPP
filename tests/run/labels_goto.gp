// phase: 02

// Labeled break and continue out of nested loops and out of a switch, and
// goto forward, backward and out of a loop.
package main

import "fmt"

func findPair(target int) (int, int) {
	a, b := -1, -1
outer:
	for i := 1; i < 10; i++ {
		for j := i; j < 10; j++ {
			if i*j == target {
				a, b = i, j
				break outer
			}
		}
	}
	return a, b
}

func main() {
	fmt.Println(findPair(24))
	fmt.Println(findPair(97))

rows:
	for i := range 4 {
		for j := range 4 {
			if j > i {
				fmt.Println("|")
				continue rows
			}
			fmt.Print(i*j, " ")
		}
		fmt.Println("full row")
	}

	n := 0
loop:
	for {
		switch {
		case n >= 3:
			break loop
		default:
			n++
		}
	}
	fmt.Println("n =", n)

	i := 0
again:
	if i < 5 {
		fmt.Print(i, " ")
		i++
		goto again
	}
	fmt.Println("|")

	x := 10
	if x > 5 {
		goto skip
	}
	fmt.Println("not printed")
skip:
	fmt.Println("after skip")

	for k := 0; ; k++ {
		if k*k > 50 {
			fmt.Println("first square over 50:", k*k)
			goto done
		}
	}
done:
	fmt.Println("done")

	count := 0
next:
	for a := range 3 {
		for b := range 3 {
			if b == a {
				continue next
			}
			count++
		}
	}
	fmt.Println("count", count)
}

// Output:
// 3 8
// -1 -1
// 0 |
// 0 1 |
// 0 2 4 |
// 0 3 6 9 full row
// n = 3
// 0 1 2 3 4 |
// after skip
// first square over 50: 64
// done
// count 3
