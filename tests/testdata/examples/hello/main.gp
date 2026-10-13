// phase: 02

// Output: a greeting. This doc comment starts like an // Output: line, and
// the next line like a // Stderr: line; examples ignore both.
// Stderr: nothing
package main

import "fmt"

// ExitCode: a comment after the package clause that starts like an
// // ExitCode: line is not one.
func main() {
	fmt.Println("hello")
}
