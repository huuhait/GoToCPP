// Package load finds and loads gocpp packages. Load reads a folder of .gp
// files (or the .gp files named on the command line) and every package it
// imports, from the module (found by its gocpp.mod file) and from the
// standard library. For each package it runs the parse-level steps of the
// front-end: it parses the files, sorts the imports into gocpp packages and
// C/C++ headers (a header is a package named after its file, and C++
// namespaces such as std are packages that need no import), parses the
// //gocpp: directives, and rewrites header and namespace uses for go/types
// (raylib.X becomes C.X, std.X becomes C.std.X). The result is a graph of
// packages in dependency order whose ASTs are ready for go/types with
// FakeImportC (step 1.10).
//
// Phase 01 reads only the module line of gocpp.mod. Phase 06 reads the rest
// (require, replace) and adds vendor/.
package load
