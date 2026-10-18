// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: interface \{\} is \[\]int, not main\.Celsius

// Named types, composite types and builtin types in the message (step 5.5).
package main

import "fmt"

type Celsius float64

func main() {
	var x any = Celsius(1)
	fmt.Println(x.(Celsius))
	var y any = []int{1}
	fmt.Println(y.(Celsius))
}

// Output:
// 1
