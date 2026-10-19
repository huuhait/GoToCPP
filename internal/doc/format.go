package doc

import (
	"go/token"
	"io"

	"github.com/huuhait/gocpp/internal/syntax/printer"
)

// printMode is the mode of go/format: spaces for alignment, tabs for
// indentation, and numbers in their canonical form (the value is
// go/printer's normalizeNumbers, which is not exported: go/format defines
// the same constant).
const printMode = printer.UseSpaces | printer.TabIndent | 1<<30

var printConfig = printer.Config{Mode: printMode, Tabwidth: 8}

// formatNode prints node as go/format.Node does, with gocpp's printer, which
// also prints the syntax of Phase 10. node is an *ast.File,
// *printer.CommentedNode, or any declaration, spec, statement or expression.
// Unlike go/format.Node it does not sort the imports: doc never prints a
// whole file.
func formatNode(dst io.Writer, fset *token.FileSet, node any) error {
	return printConfig.Fprint(dst, fset, node)
}
