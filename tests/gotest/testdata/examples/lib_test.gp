package lib

import "fmt"

func ExampleAdd() {
	fmt.Println(Add(1, 2))
	// Output: 3
}

func ExampleGreet() {
	fmt.Println(Greet("go"))
	fmt.Println(Greet("cpp"))
	// Output:
	// hello, go
	// hello, cpp
}

func Example_unordered() {
	fmt.Println("b")
	fmt.Println("a")
	// Unordered output:
	// a
	// b
}

func Example_noOutput() {
	fmt.Println("compiled, not run")
}
