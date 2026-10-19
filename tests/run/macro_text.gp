// phase: 10

// #text(x) (Phase 10 step 10.8): the source text of the argument, exactly as written, except
// that a line break with its indentation and each comment become one space. The argument is
// not evaluated and not type-checked: it may name things that don't exist.

package main

import "fmt"

func check(ok bool, text string, line int) {
	if !ok {
		fmt.Printf("line %d: assertion failed: %s\n", line, text)
	}
}

func Sprint(a ...any) string { return fmt.Sprint(a...) }

const name = #text(not.declared)

func main() {
	a, b := 3, 4
	fmt.Println(#text(a + b))
	fmt.Println(#text(a   +   b))
	fmt.Println(#text(a+b))
	fmt.Println(#text(Sprint(a, b) + "x"))
	fmt.Println(#text("héllo, 世界" + "\n"))
	fmt.Println(#text(`raw  string`))
	fmt.Println(#text(a /* inline comment */ + b))
	fmt.Println(#text(
		a +
			b,
	))
	fmt.Println(#text(Sprint(a, // first
		b))) // second
	fmt.Println(#text(func() int { return a * b }()))
	fmt.Println(#text(map[string][]int{"k": {1, 2}}["k"][0]))
	fmt.Println(#text(neverDeclared.Field[0].Call(1, 2)))
	fmt.Println(#text(a > b && !(a == b) || b != 0))
	fmt.Println(#text(0x1F + 1_000 + 'x' + 1.5e3))
	fmt.Println(#text([]int{1, 2, 3}[1:2]))
	fmt.Println(name, len(#text(abc)))
	check(a > b, #text(a > b), #line())
	check(a < b, #text(a < b), #line())
	check(
		a+b == 8,
		#text(a+b == 8),
		#line(),
	)
	s := #text(#line())
	fmt.Println(s)
}

// Output:
// a + b
// a   +   b
// a+b
// Sprint(a, b) + "x"
// "héllo, 世界" + "\n"
// `raw  string`
// a + b
// a + b
// Sprint(a, b)
// func() int { return a * b }()
// map[string][]int{"k": {1, 2}}["k"][0]
// neverDeclared.Field[0].Call(1, 2)
// a > b && !(a == b) || b != 0
// 0x1F + 1_000 + 'x' + 1.5e3
// []int{1, 2, 3}[1:2]
// not.declared 3
// line 43: assertion failed: a > b
// line 48: assertion failed: a+b == 8
// #line()
