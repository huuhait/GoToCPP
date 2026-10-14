// Package format formats gocpp source files the way gofmt formats Go
// files. "gocpp fmt" uses it, and the language server will use it too.
//
// It works in three steps, like gofmt:
//
//  1. Parse the file with gocpp's parser (internal/syntax), with its
//     comments. A file with syntax errors is not formatted.
//  2. Sort the imports. Like gofmt, it sorts each import group by path
//     and removes duplicate imports. A group is a run of import lines with
//     no empty line and no comment line between them.
//  3. Print the file with go/printer and the settings of gofmt.
//
// Two rules are gocpp's own:
//   - An import group that contains a header or a namespace import
//     ("<windows.h>", "./physics.h", fs "::std::filesystem") stays as it
//     is written: it is not sorted, and duplicates stay. A header becomes
//     an #include line, and in C and C++ the order of the #include lines
//     can matter: <winsock2.h> must come before <windows.h>.
//   - A file with errors in its //gocpp: directives is not formatted. In
//     the package doc comment, go/printer would turn a broken directive
//     like //gocpp:Link m into the ordinary comment // gocpp:Link m, and
//     the library would then silently not be linked.
//
// So for a Go file, the result is exactly what gofmt prints. The tests
// check this on every Go file of the Go standard library.
//
// go/format can't do this job. For a file with an import group,
// go/format.Node prints the file and parses it again with the stock
// go/parser, which fails on gocpp code like std.map, and then it sorts
// every import group. So this package has copies of the parts of Go that
// it needs, under Go's license (the LICENSE file here): the import sorting
// of go/ast (imports.go), the parsing and printing of gofmt, which also
// handles the fragments that gofmt accepts on standard input (gofmt.go),
// and the way gofmt reads and writes files (file.go).
package format
