// phase: 07

// A type parameter named C hides C inside its function: C.int there is the type parameter's, as Go's scoping says.
package main

func Ok[C any](x C) C { return x }

func Bad[C any](x C) C {
	var n C.int // ERROR "C.int is not a type"
	_ = n
	return x
}

func main() {}
