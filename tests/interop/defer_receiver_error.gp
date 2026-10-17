// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./defer_receiver_error\.gp:19:8: cannot defer a call whose object is v\[0\]: Go evaluates it when the defer statement runs, and a deferred C\+\+ call can only keep a variable's object or a temporary: use a func literal, defer func\(\) \{ \.\.\. \}\(\)$

// A deferred call on a C++ object keeps the object (the variable, boxed) or a temporary. An
// element or a pointer's target of a local is a place that Go evaluates when the defer statement
// runs and C++ would read when the call runs, so it is an error, with the func literal that
// does it.
package main

import (
	"./objects.h"
	"<vector>"
)

func main() {
	v := std.vector[objects.Counter]{}
	v.push_back(objects.Counter{})
	defer v[0].inc()
}

// Output:
