// Labels: break and continue with a label out of nested loops and switches, and
// goto forward and backward.
package main

import "fmt"

func find(target int) (int, int) {
	ri, rj := -1, -1
outer:
	for i := range 5 {
		for j := 0; j < 5; j++ {
			if i*j == target {
				ri, rj = i, j
				break outer
			}
		}
	}
	return ri, rj
}

func main() {
	fmt.Println(find(6))
	fmt.Println(find(7))

	// continue with a label: the outer loop's post runs.
rows:
	for i := 0; i < 4; i++ {
		x := i * 10
		for j := range 4 {
			if j > i {
				fmt.Println()
				continue rows
			}
			fmt.Print(x+j, " ")
		}
		fmt.Println("end", i)
	}

	// break and continue with labels from a switch inside loops.
	n := 0
loop:
	for {
		n++
		switch {
		case n%2 == 0:
			continue loop
		case n > 7:
			break loop
		}
		fmt.Print(n, " ")
	}
	fmt.Println("n", n)

	// A labeled switch: break L from a loop inside it.
sw:
	switch n {
	case 9:
		for i := 0; ; i++ {
			if i == 2 {
				fmt.Println("break sw at", i)
				break sw
			}
		}
	}

	// A range loop with continue L from an inner loop, and a 3-clause loop
	// whose post is a parallel assignment, with continue L.
count:
	for i := range 4 {
		v := i * i
		for j := 0; j < 2; j++ {
			if i == 2 {
				continue count
			}
		}
		fmt.Print(v, " ")
	}
	fmt.Println()
pairs:
	for a, b := 0, 9; a < b; a, b = a+1, b-1 {
		for c := range 3 {
			if (a+c)%3 == 0 && c > 0 {
				continue pairs
			}
		}
		fmt.Print(a, "-", b, " ")
	}
	fmt.Println()

	// goto backward: a loop.
	i := 0
again:
	fmt.Print(i, " ")
	i++
	if i < 3 {
		goto again
	}
	fmt.Println()

	// goto forward, over statements with temporaries.
	a, b := 1, 2
	if a < b {
		goto skip
	}
	a, b = b, a
	fmt.Println("not printed")
skip:
	fmt.Println("a b", a, b)

	// goto out of nested loops, then over a switch and a loop.
	for x := 0; x < 3; x++ {
		for y := 0; y < 3; y++ {
			if x+y == 3 {
				fmt.Println("goto done at", x, y)
				goto done
			}
		}
	}
done:
	goto end
	switch a {
	case 1:
		fmt.Println("never")
	}
	for range 3 {
		fmt.Println("never")
	}
end:
	fmt.Println("end")
}
