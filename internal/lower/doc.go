// Package lower turns a type-checked gocpp package into C++: a cppir tree, which maps 1:1 to
// C++ syntax and which cppir.Print writes (Phase 03 README §1).
//
//	typed AST ──► lower (analysis + rewrites) ──► cppir (C++ syntax tree) ──► cppir.Print
//
// internal/cppgen runs the stages: cppgen.EmitPackage calls Package, then cppir.Print.
//
// # What lowering decides, and what the printer decides
//
// Lowering decides what the C++ is: everything that depends on Go. Go's order of evaluation
// (the gp_ temporaries of order.go), the names (names.go: escapes, the gp_ temporaries, a
// shadowing variable's Scope, ADR-022), the C++ types, the small-integer casts and the
// runtime helpers for division and shifts (ops.go), the qualification that turns off C++'s
// argument-dependent lookup (funcCallName), the exact type of a constant where C++ deduces it
// (exactExpr), the three shapes of a switch, the labels of break and continue (flow.go), and
// which statements share a C++ line: each .gp statement becomes one C++ line where it can (a
// cppir.Line, an inline cppir.Block), so a debugger steps over it once. Each node carries the
// .gp place of its line (gen.pos), and generated-only code (the prototypes, the main wrapper,
// gp_init_funcs) has the zero Pos.
//
// The printer (internal/cppir) decides only the layout: indentation, the #line directives
// (exactly where a line's place is not the line before + 1, and back to Config.OutputPath for
// generated-only lines), the joined braces (} else {, } break;, the } } } of nested Scopes),
// and the parentheses that C++'s precedence needs. The parentheses that only help a reader or
// quiet a warning, (a & b) | c and (a < b) == c, are lowering's: it adds a cppir.Paren (at).
// So lowering never writes C++ text with layout in it, and a Phase 03 feature is a lowering
// rule plus, where C++ needs new syntax, a cppir node.
//
// # Building the tree
//
// Lowering walks the Go AST in source order and appends to the statement list that is open
// (funcGen.add); a node with a body (a block, a loop, a case, a Scope) opens its list
// (funcGen.push), and closing it (funcGen.pop) sets the place of its closing brace. "Insert a
// temporary before the current statement" is a statement in front of it in the same list, or
// on its line. Names and temporaries are chosen while the tree is built, in source order, so
// the output doesn't depend on map order.
//
// # Files
//
//   - file.go: Package, the translation unit (#include <gocpp/rt.hpp>, the flat package
//     namespace gpkg_main, the prototypes, the C++ main wrapper, which calls gp::run_main),
//     and small constructors of cppir nodes;
//   - decl.go: functions (parameters, results as a std::tuple, named results), package
//     variables (constinit) and the package's initialization (gp_var_init, the init
//     functions);
//   - stmt.go, control.go and flow.go: statements (control.go has if, for, range and switch,
//     flow.go finds where each break and continue goes), and the statement lists (add, push,
//     pop);
//   - expr.go: expressions, with constants folded by go/types;
//   - order.go: Go's order of evaluation: the calls (and the other things whose order can be
//     seen) that C++ could evaluate in another order go into gp_ temporaries first;
//   - box.go: the boxing analysis, a pre-pass over each function that decides which of its
//     variables live in a box on the heap (gp::ptr<T> x = gp::new_<T>(...), each use (*x));
//   - addr.go: &x, the owner table (every pointer shares the owner of what it points into);
//   - slices.go: slices (step 3.5): literals, make, append, copy, clear, the conversions
//     between strings and slices, and the move of s in s = append(s, x);
//   - arrays.go: arrays (step 3.6): literals (constinit ones too), a[i] and p[i], slicing,
//     the owner of &a[i] (addrOf), the conversions from slices, and range over arrays (with a
//     copy only where the body could change the array);
//   - maps.go: maps (step 3.7): literals in gc's order, m[k] reads and stores, op= and ++ on an
//     entry, v, ok := m[k], make, delete, clear, range, and their order of evaluation;
//   - typedecl.go: the type declarations (step 3.8): the C++ structs of the named and the
//     anonymous struct types, with their ==, gp_hash, the fmt shim's gp_type_name and gp_fields,
//     the names of local types moved to the namespace, and the order of the definitions;
//   - structs.go: struct expressions (step 3.8): literals (keyed ones in the fields' order),
//     fields (also promoted ones, through pointers), the owners of &x.f, the operands of a target
//     x.f, and the copy before an assignment that could free its value;
//   - defer.go: defer (step 3.13): the single-exit shape of a function with defers (the
//     results as variables, a gp::defer_frame, the body in a try block, one exit per return),
//     and the deferred calls, with their arguments evaluated at the defer statement;
//   - named.go: the named types that aren't structs (step 3.9): their C++ structs over
//     gp::newtype, the conversions to and from them, and Go's implicit conversions between types
//     that are not identical (also between struct types), found before a function is lowered;
//   - ops.go: the operators (with the small-integer rule and the runtime's gp::div, gp::rem,
//     gp::shl and gp::shr), conversions (gp::f2i), s[i] and the builtins len and panic;
//   - types.go: the C++ spelling of the types;
//   - names.go: the C++ names: a _ suffix for C++ keywords, compiler words and macros
//     (cxx_words.txt, cxx_macros.txt) and for user names that start with gp_, which are the
//     generated code's; the names of local variables, which keep their Go names, also when
//     they shadow one (v := v); the gp_ temporaries.
//
// Phase 02 built it for a small subset of the language (as internal/cppgen; step 3.1 moved
// it here), and every later phase widens it: Phase 03 adds the boxing analysis (box.go),
// value receivers and parameters as real copies, closures that share variables, defer frames
// and range loops; later phases add their rewrites, for example the early returns of ! (must) in
// Phase 10. Phase 06 adds the per-package layout: one pkg.hpp per package, one .cpp per .gp
// file and a pkg_init.cpp.
package lower
