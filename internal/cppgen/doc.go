// Package cppgen writes the C++ of a type-checked gocpp package: one readable C++20
// translation unit, which compiles with the runtime (runtime/, gocpp/rt.hpp) and the driver's
// flags. The driver calls EmitPackage with a package that check.Check checked.
//
// It walks the typed AST (go/ast and types.Info); Phase 03 adds a lowering pass in front when
// the rewrites get complex. The files (Phase 02 README §2.2):
//
//   - file.go: EmitPackage, the prologue (#include <gocpp/rt.hpp>), the flat package namespace
//     (gpkg_main), the prototypes and the C++ main wrapper, which calls gp::run_main;
//   - decl.go: functions (parameters, results as a std::tuple, named results), package
//     variables (constinit) and the package's initialization (gp_var_init, the init
//     functions);
//   - stmt.go and expr.go: statements and expressions, with constants folded by go/types;
//   - types.go: the C++ spelling of the types;
//   - names.go: the C++ names: a _ suffix for C++ keywords, compiler words and macros
//     (cxx_words.txt, cxx_macros.txt) and for user names that start with gp_, which are the
//     generated code's; new names where C++ would see a declaration differently than Go
//     (v := v); the gp_ temporaries;
//   - writer.go: indentation, and the #line directives that point every generated line back
//     at its .gp line, for debuggers and compiler errors.
//
// Phase 02 fills it for a small subset of the language, and every later phase widens it. Phase
// 06 adds the per-package layout: one pkg.hpp per package, one .cpp per .gp file and a
// pkg_init.cpp.
package cppgen
