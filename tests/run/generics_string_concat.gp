// phase: 07

// Strings as type arguments: + and +=, comparison, and a defined string type keeps its type.
package main

import "fmt"

type Name string

func Cat[T ~string](a, b T) T { return a + " " + b }

func Repeat[T ~string](s T, n int) T {
	var out T
	for range n {
		out += s
	}
	return out
}

func Less[T ~string](a, b T) bool { return a < b }

func Join[T ~string](xs []T, sep T) T {
	var out T
	for i, x := range xs {
		if i > 0 {
			out += sep
		}
		out += x
	}
	return out
}

func main() {
	fmt.Println(Cat("a", "b"), Cat(Name("x"), Name("y")))
	fmt.Println(Repeat("ab", 3), Repeat(Name("-"), 4), Repeat("", 5) == "")
	fmt.Println(Less("a", "b"), Less(Name("b"), Name("a")), Less("", "a"), Less("abc", "abd"))
	fmt.Println(Join([]string{"a", "b", "c"}, ","), Join([]Name{"x", "y"}, "+"), Join[string](nil, ",") == "")
	fmt.Printf("%T %T\n", Cat(Name("a"), "b"), Repeat(Name("z"), 1))
}

// Output:
// a b x y
// ababab ---- true
// true false true true
// a,b,c x+y true
// main.Name main.Name
