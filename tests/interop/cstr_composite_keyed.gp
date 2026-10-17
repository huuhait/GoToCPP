// phase: 04
// ExitCode: 1
// Stderr: C\.cstr\(s\) \(C\.cstr holder\) cannot be stored in a composite literal: its C string could outlive the holder; use C\.CString and C\.free for a retained C string

// A C pointer field must not receive a temporary holder or a holder copy. Keyed literals
// use field-assignment lambdas; positional literals obey the same no-holder-escape policy.
package main

import "./cgostr.h"

func main() {
	s := "temporary" + " holder"
	item := cgostr.CStringItem{name: C.cstr(s)}
	_ = item
}

// Output:
