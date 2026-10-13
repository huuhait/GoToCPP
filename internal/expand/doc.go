// Package expand will turn the parts of gocpp that Go doesn't have into plain
// Go, so the stock go/types can check them: macro declarations become
// functions, #Name(...) uses become calls (start style, or wrap style with
// #body()), #Derive(...) on a type generates methods, and the ! (must) forms,
// func f!() and F!(...), become error checks.
//
// Phase 10 fills it. Expanded files are cached per file, keyed also by the
// signatures of the macros they use.
package expand
