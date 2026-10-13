// Package cppgen will write the C++: names and mangling (a C++ keyword or a
// C macro name gets a _ suffix, and gp_ names are kept for generated code),
// C++ types, expressions, statements and declarations, and #line directives
// that point every generated line back at its .gp line, for debuggers and
// compiler errors.
//
// Phase 02 fills it for a small subset of the language, and every later
// phase widens it. Phase 06 adds the per-package layout: one pkg.hpp per
// package, one .cpp per .gp file and a pkg_init.cpp.
package cppgen
