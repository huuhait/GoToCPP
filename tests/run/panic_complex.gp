// phase: 03
// ExitCode: 2
// Stderr: ^\(\+1\.000000e\+000-0\.000000e\+000i\) \(\+InfNaNi\)\npanic: \(-1\.500000e\+000-2\.000000e\+000i\)\n\ngoroutine 1 \[running\]:\n

// A complex division by zero gives infinities, not a panic; panic with a
// complex value prints it in the runtime's format, like println.
package main

import "fmt"

func main() {
	var zero complex128
	c := complex(1, -real(zero))
	println(c, c/zero)
	fmt.Println("dividing done")
	panic(-c * complex(1.5, 2) / (1 + 0i))
}

// Output:
// dividing done
