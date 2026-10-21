package server

import (
	"go/ast"
	"go/types"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// memberCompletionStatus is an optional extension of CompletionList. IsIncomplete retains
// its normal prefix-filtering meaning; this distinguishes pending/unsupported receiver facts
// from a ready complete-empty or ready partial result, including when there are no items.
type memberCompletionStatus struct {
	Ready      bool `json:"ready"`
	Incomplete bool `json:"incomplete"`
}

type memberCompletionList struct {
	*protocol.CompletionList
	Members *memberCompletionStatus `json:"x-gocpp-members"`
}

func (c *completer) receiverMembers(x ast.Expr) {
	id, ok := ast.Unparen(x).(*ast.Ident)
	if !ok {
		return // calls, dereferences, promoted fields and arbitrary expressions remain unsupported
	}
	object, ok := c.q.info.Uses[id].(*types.Var)
	if !ok || object.IsField() {
		return
	}
	c.memberStatus = &memberCompletionStatus{Incomplete: true}
	if c.q.pkg.IsVariant() || c.q.blanked != nil || c.q.shift != nil {
		return // never transfer original object facts into repaired or stale binding tables
	}
	names, incomplete, ready := c.q.snap.MemberSymbols(c.q.pkg, object)
	c.memberStatus = &memberCompletionStatus{Ready: ready, Incomplete: incomplete}
	seen := make(map[string]bool)
	for _, candidate := range c.cands {
		seen[candidate.name] = true
	}
	detail := "C/C++ member"
	if incomplete {
		detail += " (partial result)"
	}
	for _, name := range names {
		if seen[name] || !c.match(name) {
			continue
		}
		seen[name] = true
		c.cands = append(c.cands, &candidate{
			name: name, kind: protocol.TextCompletion, detail: detail, score: stdScore,
		})
	}
}
