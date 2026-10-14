// phase: 02

// Calls in the arguments of a call run left to right (g++ and clang++
// order them differently without the evaluation-order pass). The
// functions print as they run, so the order shows in the output.
package main

import "fmt"

func tr(name string, v int) int {
	fmt.Print(name, " ")
	return v
}

func trs(name string) string {
	fmt.Print(name, " ")
	return name
}

func add(a, b int) int {
	return a + b
}

func three(a, b, c int) int {
	return a*100 + b*10 + c
}

func pair(a, b int) (int, int) {
	return tr("p1", a), tr("p2", b)
}

func main() {
	fmt.Println(tr("a", 1), tr("b", 2))
	fmt.Println(tr("a", 1), tr("b", 2), tr("c", 3), tr("d", 4))
	fmt.Println(add(tr("x", 1), tr("y", 2)))
	fmt.Println(three(tr("1", 1), tr("2", 2), tr("3", 3)))
	fmt.Println(add(add(tr("a", 1), tr("b", 2)), add(tr("c", 3), tr("d", 4))))
	fmt.Println(three(tr("x", 1), 5, tr("y", 2)))
	fmt.Println(trs("s1") + trs("s2") + trs("s3"))
	fmt.Println(add(pair(tr("i", 1), tr("j", 2))))
	fmt.Printf("%d %s %d\n", tr("f1", 1), trs("f2"), tr("f3", 3))
	local := 7
	fmt.Println(local, tr("after-local", 8), local)
	println(tr("to-stderr-1", 1), tr("to-stderr-2", 2))
	fmt.Println()
}
