// phase: 02

// A character after % that is no verb of Go's fmt prints fmt's error form
// (%!z(int=5)); the gates allow it from Phase 02 (internal/check/gates.go,
// formatVerbs).
package main

import "fmt"

func main() {
	fmt.Printf("%z|%!|%y\n", 5, "s", true)
	fmt.Printf("%z\n")
}
