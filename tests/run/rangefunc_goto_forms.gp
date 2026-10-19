// phase: 07

package main

import "fmt"

func Cnt(name string, n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		defer fmt.Println(name, "done")
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func forward() {
	for x := range Cnt("A", 3) {
		for y := range Cnt("B", 3) {
			if x == 1 && y == 1 {
				goto done
			}
			fmt.Println("xy", x, y)
		}
	}
	fmt.Println("skipped")
done:
	fmt.Println("done label")
}

func inBody() {
	for x := range Cnt("C", 3) {
		if x == 1 {
			goto skip
		}
		fmt.Println("x", x)
	skip:
		fmt.Println("after skip", x)
	}
}

func backward() {
	n := 0
again:
	n++
	for x := range Cnt("D", 3) {
		if x == 1 && n < 3 {
			fmt.Println("again", n)
			goto again
		}
		fmt.Println("d", n, x)
	}
}

func intoOuterBody() {
	for x := range Cnt("E", 2) {
		for y := range Cnt("F", 3) {
			if y == 1 {
				goto next
			}
			fmt.Println("ef", x, y)
		}
	next:
		fmt.Println("next", x)
	}
}

func withDefer() (r int) {
	defer func() { fmt.Println("deferred", r) }()
	for x := range Cnt("G", 5) {
		r += x
		if x == 3 {
			goto out
		}
	}
out:
	r *= 10
	return
}

func main() {
	forward()
	inBody()
	backward()
	intoOuterBody()
	fmt.Println(withDefer())
}

// Output:
// xy 0 0
// xy 0 1
// xy 0 2
// B done
// xy 1 0
// B done
// A done
// done label
// x 0
// after skip 0
// after skip 1
// x 2
// after skip 2
// C done
// d 1 0
// again 1
// D done
// d 2 0
// again 2
// D done
// d 3 0
// d 3 1
// d 3 2
// D done
// ef 0 0
// F done
// next 0
// ef 1 0
// F done
// next 1
// E done
// G done
// deferred 60
// 60
