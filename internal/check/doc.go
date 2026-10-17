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
// because go/types stops at their opaque base, and the expressions that
// go/types skips next to an opaque one, such as the T of x.(T)
// (checkSkipped).
//
// A conversion of an opaque value, n := int(cstring.strlen(p)), would have
// no type either, and n would be invalid. So in a package with C in it (a
// file with import "C", or an import of such a package, directly or not),
// Check rewrites such a conversion T(x) to gp_cgoh.As[T](x), a call of a
// generic function of the hidden package gocpp/cgoh, and checks the
// package again: then n is an int, and later mistakes with n are reported
// (Conversion, converter). Before the first check of such a package, the
// calls of cgo's helpers (C.GoString(p), C.CString(s), C.cstr(s), ...)
// become calls of functions of the same hidden package with cgo's
// declarations, so that s := C.GoString(p) is a string (cgohelpers.go).
//
// After the last check, the opaque pass (Package.Opaque) labels every
// expression typed or opaque (it comes from C/C++ and has no complete
// type) and keeps the type expressions of the objects declared with C/C++
// types.
//
// Before go/types, the namespace probe (nsprobe.go) asks the C++ compiler
// (Config.CXX) which names that the loader took as C++ namespaces (std in
// std.vector) the headers of each file really declare: a typo such as
// stdd.vector is "undefined: stdd", reported by go/types itself, and a
// header that isn't found or doesn't compile is an error at its import.
//
// After the last check, the unsupported-feature checker rejects every
// feature that is not built yet with a message naming the phase that
// brings it (a phase gate), so nobody gets broken C++. Each later phase
// removes the gates of the features it builds.
package check
