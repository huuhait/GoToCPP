// Package interop will hold what gocpp knows about C and C++ headers: the
// headers a package uses, their macros, and the clang probes that ask the C++
// compiler for the real type of each C/C++ name a program uses. With these
// types, C/C++ mistakes are reported before any C++ is generated, editors can
// complete C/C++ names, and "gocpp bindgen" can show a header as gocpp code.
//
// Phase 12 (typed interop) fills it. Until then C/C++ names are opaque, and
// only the simpler compiler probes of Phases 01 and 04 run (is a name a
// namespace, a type, a template or a value; which names are macros).
package interop
