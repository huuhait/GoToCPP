package format

import (
	"go/ast"
	"go/printer"
	"go/token"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// normalizeNumbers is the unexported mode normalizeNumbers of go/printer.
// It prints number literals in one canonical form: lowercase prefixes and
// exponents (0XFF becomes 0xFF, 1E5 becomes 1e5, 0X1P-2 becomes 0x1p-2),
// and an imaginary integer literal without leading zeros (0123i becomes
// 123i). go/format and cmd/gofmt use the same value: they call it
// printerNormalizeNumbers, and Go doesn't export it because changing the
// text of code is not the printer's job. If a new Go changes the value,
// TestNormalizeNumbers fails.
const normalizeNumbers printer.Mode = 1 << 30

// config is the printer configuration of gofmt (printerMode and tabWidth
// in cmd/gofmt/gofmt.go).
var config = printer.Config{Mode: printer.UseSpaces | printer.TabIndent | normalizeNumbers, Tabwidth: 8}

// Source formats src, the contents of the gocpp file filename, the way
// gofmt formats a Go file, and returns the result. filename is used only
// in error messages.
//
// Like gofmt, Source sorts the imports. But it leaves an import group with
// a header or a namespace import exactly as it is written (see the package
// documentation).
//
// If src has syntax errors, or errors in its //gocpp: directives (see
// load.ParseDirectives), Source returns no source and the errors, as a
// scanner.ErrorList of gocpp's parser (internal/syntax/scanner), sorted by
// position. The warnings of load.ParseDirectives, for a comment that only
// looks like a directive, are not errors: such a comment is an ordinary
// comment, Source formats it like one, and gocpp check reports it.
//
// Source also returns an error, and no source, in one rare case where it
// can't keep the parentheses that a composite literal needs in the header
// of an if, for or switch statement: when the type of the literal can't be
// printed on one line, or has a comment in it (see keepParens).
func Source(filename string, src []byte) ([]byte, error) {
	return formatSource(filename, src, false)
}

// Fragment is like Source, but src may also be a part of a file: a list of
// declarations, or a list of statements (an expression is one too). gofmt
// accepts such a fragment on standard input, and Fragment formats it in
// the same way: it keeps the space before and after the code, and the
// indentation of its first line of code.
//
// A fragment has no package clause, so it can have no //gocpp: directives
// either (they come before the package clause). So Fragment checks the
// directives only when src is a complete file.
func Fragment(filename string, src []byte) ([]byte, error) {
	return formatSource(filename, src, true)
}

// formatSource is Source, and Fragment if fragmentOK is set: the steps of
// processFile in cmd/gofmt, without its flags -r and -s.
func formatSource(filename string, src []byte, fragmentOK bool) ([]byte, error) {
	fset := token.NewFileSet()
	file, sourceAdj, indentAdj, err := parse(fset, filename, src, fragmentOK)
	if err != nil {
		return nil, err
	}
	if sourceAdj == nil { // a complete file, not a fragment
		if err := checkDirectives(fset, file); err != nil {
			return nil, err
		}
	}
	sortImports(fset, file)
	if err := keepParens(fset, file); err != nil {
		return nil, err
	}
	return format(fset, file, sourceAdj, indentAdj, src, config)
}

// checkDirectives returns the errors in the //gocpp: directives of file,
// as a scanner.ErrorList sorted by position, or nil if there are none.
func checkDirectives(fset *token.FileSet, file *ast.File) error {
	var errs diag.List
	load.ParseDirectives(fset, file, &errs)
	var list scanner.ErrorList
	for _, d := range errs.Diagnostics() {
		if d.Severity == diag.Error {
			list.Add(d.Pos, d.Msg)
		}
	}
	return list.Err()
}
