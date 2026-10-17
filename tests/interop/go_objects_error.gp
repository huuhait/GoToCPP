// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./go_objects_error\.gp:16:2: go statements are not supported yet \(Phase 08\)$

// go statements on C++ objects (a member call, &x of a local) wait for Phase 08, like every go
// statement: the deferred-call machinery of step 4.8 keeps a local alive for a defer, and a
// goroutine would need the same for its whole life.
package main

import (
	"./objects.h"
)

func main() {
	var c objects.Counter
	go c.inc()
	go objects.bump_ptr(&c)
}

// Output:
