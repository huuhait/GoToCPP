// Package check type-checks gocpp packages with the stock go/types.
//
// Check takes the program that load.Load loaded and checks each package
// after the packages it imports, with FakeImportC: every C.X, which the
// loader made out of header and namespace uses (raylib.X, std.X), is
// "opaque", so go/types accepts it without a type, and the C++ compiler
// checks it later. Each package gets its types.Package, a types.Info with
// every map filled, and its errors, which read like go build's. A package
// with load errors, or one that imports such a package, is not checked.
// After go/types, Check type-checks the type arguments of opaque
// instantiations (std.vector[strings.Builder]) with types.CheckExpr,
// because go/types stops at their opaque base.
//
// Phase 01 adds more on top (steps 1.10 to 1.12): the gp_as[T] rewrite of
// conversions of opaque values (and a second check), the namespace probe,
// the opaque pass, which records which expressions come from C/C++, and the
// unsupported-feature checker, which rejects every feature that is not
// built yet with a message naming the phase that brings it (a phase gate),
// so nobody gets broken C++. Each later phase removes the gates of the
// features it builds.
package check
