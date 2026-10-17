// Package syntax holds gocpp's fork of Go's go/scanner, go/parser and
// go/printer (from the dev shell's Go, Go 1.25.14 now), in the subpackages
// scanner, parser and printer. The fork still makes stock go/ast nodes, so the
// stock go/types can check them. FORK.md says which upstream files are copied,
// lists every change and patch, and says how to re-fork with update.sh.
//
// Phase 01 fills it: the fork, FORK.md, the upstream tests, and three patches
// for C++ names: a Go keyword after "." on the same line is a name
// (std.map, node.type()), type arguments may be constants (std.array[int, 5]),
// and type names may have more than one dot (std.chrono.seconds). Phase 10
// (step 10.1) adds four more: name!( for must calls and must functions, #Name
// for macros, macro declarations with lazy parameters and lines of macros on
// top of a declaration, and the printer for them. That syntax is in a mode
// that is off by default (parser.Phase10Syntax). Subpackage gpast, which is
// not a fork, says how it looks in go/ast nodes.
package syntax
