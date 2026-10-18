// phase: 05

// A type switch without a variable, nested type switches and switches in closures (step 5.5).
package main

import "fmt"

func kind(x any) int {
	switch x.(type) {
	case int:
		return 1
	case string:
		return 2
	}
	return 0
}

func main() {
	pairs := [][2]any{{1, "a"}, {"b", 2}, {1, 2}, {nil, nil}, {2.5, "c"}}
	for _, p := range pairs {
		switch a := p[0].(type) {
		case int:
			switch b := p[1].(type) {
			case int:
				fmt.Println("int int", a+b)
			case string:
				fmt.Println("int string", a, b)
			}
		case string:
			switch p[1].(type) {
			case int:
				fmt.Println("string int", a)
			default:
				fmt.Println("string other")
			}
		default:
			fmt.Println("default", kind(p[0]), kind(p[1]))
		}
	}
	f := func(x any) string {
		switch x.(type) {
		case int:
			return "i"
		}
		return "-"
	}
	fmt.Println(f(1), f("a"))
}

// Output:
// int string 1 a
// string int b
// int int 3
// default 0 0
// default 0 2
// i -
