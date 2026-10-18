// phase: 05

// The switch variable shadows an outer variable of the same name; it is declared in each clause
// with the type of the clause, and may be unused in some of them (step 5.5).
package main

import "fmt"

func main() {
	v := "outer"
	var x any = 5
	switch v := x.(type) {
	case int:
		fmt.Println("int", v+1)
	case string:
		fmt.Println("string", v)
	}
	fmt.Println(v)
	x = "s"
	switch x := x.(type) {
	case int:
	case string:
		fmt.Println(len(x))
	default:
		_ = x
	}
	switch y := x.(type) {
	case int, string:
		fmt.Println("both", y == x)
	}
	var z any = 1.5
	switch z := z.(type) {
	case float64:
		z *= 2
		fmt.Println(z)
	}
	fmt.Println(z.(float64))
}

// Output:
// int 6
// outer
// 1
// both true
// 3
// 1.5
