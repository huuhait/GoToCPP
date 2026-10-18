// Package cppgen writes the C++ of type-checked gocpp packages. The driver calls EmitProgram with
// the packages of a program that check.Check checked: it writes the split layout (ADR-025: pkg.hpp,
// one unit per .gp file, pkg_desc.cpp, pkg_init.cpp for each package), which compiles with the
// runtime (runtime/, gocpp/rt.hpp) and the driver's flags. EmitPackage writes one package as one
// readable C++20 translation unit (the unit layout); only the tests use it (the golden files and
// the emitter's end-to-end tests, about a hundred of them).
//
// TODO(phase 06): move the golden tests to EmitProgram (their files would become the pkg.hpp and
// the units of the split layout: a churn of every golden) and drop the unit layout with
// lower.Package, which the product no longer uses.
//
// It runs the backend's stages (Phase 03 README §1):
//
//	typed AST ──► internal/lower ──► internal/cppir (a C++ syntax tree) ──► cppir.Print
//
// internal/lower decides what the C++ is (Go's semantics: the order of evaluation, the names,
// the shapes of the statements), internal/cppir holds it and writes it (indentation and the
// #line directives). cppgen itself only checks its input and passes Config.OutputPath to the
// printer; its tests are the emitter's end-to-end tests: the golden C++ of testdata/emit,
// which each compiler builds and runs against real Go, the panics, the evaluation order (also
// TestRandomOrder's random programs), the reserved names with real compilers, and what can't
// be translated yet (TestUnsupported).
package cppgen
