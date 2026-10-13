// Package load will find and load gocpp packages. It reads a directory of .gp
// files, sorts the imports into gocpp packages and C/C++ headers (a header is
// a package named after its file, and C++ namespaces such as std are packages
// that need no import), parses the //gocpp: directives, rewrites header and
// namespace uses for go/types (raylib.X becomes C.X, std.X becomes C.std.X),
// and gives go/types an importer that loads dependencies first.
//
// Phase 01 fills it (steps 1.6 to 1.9) and finds the module root by its
// gocpp.mod file. Phase 06 reads the rest of gocpp.mod (require, replace) and
// adds vendor/.
package load
