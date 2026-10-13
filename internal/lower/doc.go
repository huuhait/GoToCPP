// Package lower will turn Go semantics into a form that maps directly to
// C++: evaluation order, value receivers and parameters as real copies,
// closures that share variables, boxing of variables whose address is taken,
// defer frames and range loops. It builds cppir, a small tree that maps 1:1
// to C++ syntax, from the typed AST.
//
// Phase 03 fills it (step 3.1 moves the Phase 02 emitter into this shape).
// Later phases add their rewrites, for example the early returns of ! (must)
// in Phase 10.
package lower
