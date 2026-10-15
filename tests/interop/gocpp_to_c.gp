// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./gocpp_to_c\.gp:16:8: cannot use fv \(variable of type float64\) as C\.int value in argument to q\.Put: convert it, C\.int\(fv\)$

// A gocpp value doesn't convert implicitly to a C/C++ type either, as in cgo: C++'s conversion
// of 1e30 to an int would be undefined behavior. C.int(fv) converts it with Go's rules.
package main

type Q struct{ n C.int }

func (q *Q) Put(n C.int) { q.n = n }

func main() {
	fv := 1e30
	var q Q
	q.Put(fv)
}

// Output:
