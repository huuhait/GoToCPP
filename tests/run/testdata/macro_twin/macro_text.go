// The twin of macro_text.gp: every #text(x) is the Go string literal of the text it gives, and every
// #line() the number of the line it has in macro_text.gp (43 and 48 below).

package main

import "fmt"

func check(ok bool, text string, line int) {
	if !ok {
		fmt.Printf("line %d: assertion failed: %s\n", line, text)
	}
}

func Sprint(a ...any) string { return fmt.Sprint(a...) }

const name = "not.declared"

func main() {
	a, b := 3, 4
	fmt.Println("a + b")
	fmt.Println("a   +   b")
	fmt.Println("a+b")
	fmt.Println(`Sprint(a, b) + "x"`)
	fmt.Println(`"héllo, 世界" + "\n"`)
	fmt.Println("`raw  string`")
	fmt.Println("a + b")
	fmt.Println("a + b")
	fmt.Println("Sprint(a, b)")
	fmt.Println("func() int { return a * b }()")
	fmt.Println(`map[string][]int{"k": {1, 2}}["k"][0]`)
	fmt.Println("neverDeclared.Field[0].Call(1, 2)")
	fmt.Println("a > b && !(a == b) || b != 0")
	fmt.Println(`0x1F + 1_000 + 'x' + 1.5e3`)
	fmt.Println("[]int{1, 2, 3}[1:2]")
	fmt.Println(name, len("abc"))
	check(a > b, "a > b", 43)
	check(a < b, "a < b", 44)
	check(
		a+b == 8,
		"a+b == 8",
		48,
	)
	s := "#line()"
	fmt.Println(s)
}
