// Package interop holds what gocpp knows about C and C++ headers: the
// headers a package uses, their macros, and the probes that ask the C++
// compiler about the C/C++ names a program uses.
//
// Phase 04 has two small compiler probes, run once per include set (the
// compiler, the folder of the .gp files, the package's flags and the
// includes of a group of files) and cached:
//   - ProbeKinds (nsprobe.go) says what each C/C++ path is: a namespace, a
//     type, a template, a value, a value template or a macro. check.Check
//     uses it for the namespace candidates of the loader (std in
//     std.vector; Phase 01's namespace probe grew into it), and the
//     lowerer (steps 4.3 and 4.5) for the :: or . after a path and for
//     template arguments versus an index after it.
//   - ProbeMacros (macroprobe.go) lists the macros after the includes, for
//     the unqualified spelling of macros and the macros that collide with
//     the names gocpp emits.
//
// The lowerer asks per include set, with the same arguments as the check
// (for the files of a package with the same header imports): then its
// paths come from the cache, or from one more run of the compiler for the
// paths that the check didn't ask for. To keep one run per include set,
// gocpp build, run and emit give the check a probe (check.Config.Probe)
// that asks ProbeKinds for the candidates and all the paths that the
// lowerer will ask for (lower.ProbePaths); gocpp check and the language
// server keep asking only for the namespace candidates, so that new member
// names don't start new runs while someone types. The macro probe is a run of its own (-E -dM
// can't share a run with -fsyntax-only), but it is cheap: it only
// preprocesses.
//
// Phase 12 (typed interop) adds the clang probes that ask for the real type
// of each C/C++ name: with them, C/C++ mistakes are reported before any C++
// is generated, editors can complete C/C++ names, and "gocpp bindgen" can
// show a header as gocpp code.
package interop
