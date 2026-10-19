package extest_test

import (
	"fmt"

	"example.com/gp/extest"
)

func ExampleAdd() {
	fmt.Println(extest.Add(1, 2))
	// Output: 3
}

func ExampleT_M() {
	extest.T{}.M()
}

func Example() {
	fmt.Println("package")
	// Output: package
}
