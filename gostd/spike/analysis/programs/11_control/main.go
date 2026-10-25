// Control flow: an expression switch on a tag with a list of cases and fallthrough, a tagless
// switch, a switch with an init statement, labeled break and continue, and goto with a label.
package main

func classify(n int) string {
	switch {
	case n < 0:
		return "neg"
	case n == 0:
		return "zero"
	default:
		return "pos"
	}
}

func main() {
	for i := -1; i <= 1; i++ {
		println(i, classify(i))
	}
	switch x := 3; x {
	case 1, 2:
		println("small")
	case 3:
		println("three")
		fallthrough
	case 4:
		println("four")
	default:
		println("other")
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
			println(i, j)
		}
	}
	i := 0
loop:
	if i < 3 {
		println("goto", i)
		i++
		goto loop
	}
}
