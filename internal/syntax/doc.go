// Package syntax holds gocpp's fork of Go's go/scanner and go/parser
// (from the dev shell's Go, Go 1.25.14 now), in the subpackages scanner and
// parser. The fork still makes stock go/ast nodes, so the stock go/types can
// check them. FORK.md says which upstream files are copied, lists every
// change and patch, and says how to re-fork with update.sh.
//
// Phase 01 fills it: the fork, FORK.md, the upstream tests, and three patches
// for C++ names: a Go keyword after "." on the same line is a name
// (std.map, node.type()), type arguments may be constants (std.array[int, 5]),
// and type names may have more than one dot (std.chrono.seconds). Phase 10
// adds three more patches (name!( for must calls, #Name for macros, macro
// declarations with lazy parameters) and a small fork of go/printer.
package syntax
