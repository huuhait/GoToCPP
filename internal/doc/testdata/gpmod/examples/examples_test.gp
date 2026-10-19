package examples

import "fmt"

// ExampleAdd shows Add.
func ExampleAdd() {
	fmt.Println(Add(1, 2))
	// Output: 3
}

func ExampleAdd_second() {
	fmt.Println(Add(2, 2))
	// Output: 4
}

func ExampleStack_Push() {
	var s Stack
	s.Push(1)
}

func Example() {
	fmt.Println("package")
	// Output: package
}
