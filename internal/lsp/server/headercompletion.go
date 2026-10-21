package server

import (
	"go/ast"
	"strings"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// headerMembers offers only actual names of an indexed header or namespace. It uses the
// rewrite's provenance rather than the spelling of a Go variable or package. Nested record
// paths are deliberately not guessed: member completion needs declaration facts (12.13b).
func (c *completer) headerMembers(x ast.Expr) {
	rw := c.q.file.Rewrite
	if rw == nil {
		return
	}
	for _, use := range rw.Uses {
		if use.Bare {
			continue
		}
		left := x
		var suffix []string
		matches := func() bool {
			return left.Pos() == use.Sel.X.Pos() && left.End() == use.Sel.X.End()
		}
		for !matches() {
			selector, ok := left.(*ast.SelectorExpr)
			if !ok {
				break
			}
			suffix = append([]string{selector.Sel.Name}, suffix...)
			left = selector.X
		}
		if !matches() {
			continue
		}
		var scope string
		switch use.Kind {
		case load.HeaderUse:
		case load.NamespaceUse:
			scope = use.Namespace
		default:
			return
		}
		if len(suffix) > 0 {
			if scope != "" {
				scope += "::"
			}
			scope += strings.Join(suffix, "::")
		}
		for _, symbol := range c.q.snap.HeaderSymbols(c.q.pkg, c.q.file, scope) {
			if !c.match(symbol.Name) {
				continue
			}
			kind := protocol.TextCompletion
			detail := "C/C++ declaration"
			if symbol.Macro {
				kind = protocol.ConstantCompletion
				detail = "C/C++ macro"
			}
			c.cands = append(c.cands, &candidate{
				name: symbol.Name, kind: kind, detail: detail, score: stdScore,
			})
		}
		return
	}
}
