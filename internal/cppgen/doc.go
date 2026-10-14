// Package cppgen writes the C++ of a type-checked gocpp package: one readable C++20
// translation unit, which compiles with the runtime (runtime/, gocpp/rt.hpp) and the driver's
// flags. The driver calls EmitPackage with a package that check.Check checked.
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
