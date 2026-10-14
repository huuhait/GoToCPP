// phase: 02

// Labeled break and continue through nested loops and switches, goto in
// both directions, a goto loop that replaces for, a labeled block, a goto
// back over a declaration, a goto out of a block with a string local, and
// continue in a loop whose post is a parallel assignment.
package main

import "fmt"

func search(target int) (int, int, int) {
	steps := 0
	for a := 1; a < 20; a++ {
	inner:
		for b := a; b < 20; b++ {
			for c := b; c < 30; c++ {
				steps++
				switch {
				case a*a+b*b == c*c && a+b+c == target:
					return a, b, c
				case a*a+b*b < c*c:
					continue inner
				}
			}
		}
	}
	return steps, 0, 0
}

func gotoLoop(n int) int {
	sum, i := 0, 0
top:
	if i >= n {
		goto end
	}
	sum += i
	i++
	goto top
end:
	return sum
}

func trs(name string) string {
	fmt.Print(name, " ")
	return name
}

func labeledBlock() int {
	i := 0
blk:
	{
		i++
		if i < 3 {
			goto blk
		}
	}
	return i
}

func backOverDecl() {
	n := 0
again:
	s := trs("x") + "y"
	n++
	if n < 3 {
		goto again
	}
	fmt.Println(s, n)
}

func outOfBlock() {
	for k := range 2 {
		{
			t := trs("inner") + "!"
			if len(t) > k {
				goto out
			}
			fmt.Println("not reached", t)
		}
	out:
		fmt.Println("out", k)
	}
}

func parallelPost() {
outer:
	for k := 0; k < 3; k++ {
		for i, j := 0, 6; i < j; i, j = i+1, j-1 {
			if i == 1 {
				continue
			}
			if j == 4 && k == 1 {
				continue outer
			}
			fmt.Print(k, ":", i, ",", j, " ")
		}
		fmt.Print("| ")
	}
	fmt.Println()
}

func main() {
	fmt.Println(labeledBlock())
	backOverDecl()
	outOfBlock()
	parallelPost()
	fmt.Println(search(12))
	fmt.Println(search(30))
	fmt.Println(search(1000))
	fmt.Println(gotoLoop(10), gotoLoop(0), gotoLoop(1000))

	count := 0
outer:
	for i := range 10 {
		for j := range 10 {
			switch {
			case j == i:
				continue outer
			case i+j > 12:
				break outer
			}
			count++
		}
	}
	fmt.Println("count", count)

	i := 0
	for {
		i++
		if i%2 == 0 {
			goto next
		}
		fmt.Print(i, ",")
	next:
		if i > 8 {
			break
		}
	}
	fmt.Println()

	n := 0
L1:
	for {
	L2:
		for {
			n++
			switch n % 4 {
			case 1:
				continue L2
			case 2:
				break L2
			case 3:
				continue L1
			}
			if n > 10 {
				break L1
			}
		}
		fmt.Print("after L2 ", n, "; ")
	}
	fmt.Println(n)
}
