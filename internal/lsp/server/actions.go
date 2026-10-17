package server

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// textDocument/codeAction (steps 11.7 and 11.9): "Organize Imports"
// (source.organizeImports, format.go) and the quick fix "Add import", for the
// diagnostics that the client sends along ("undefined: strings": the name of
// a package that the file doesn't import): one action for each package of the
// embedded standard library or of the document's module with that name, with
// the same edit that completion's auto-import makes (importEdits: the import
// goes into its group, and the imports are sorted as Organize Imports does).
// The actions for `!` (add `!` where an error is ignored, make a `!`
// function, expand `!`) and for macros come with step 11.13, after Phase 10.

// undefinedPattern matches the message of the diagnostic for a package that
// is not imported.
var undefinedPattern = regexp.MustCompile(`^undefined: ([\p{L}_][\p{L}\p{N}_]*)$`)

// codeActions answers textDocument/codeAction: the actions of every kind that
// the request wants (kindWanted).
func (s *Server) codeActions(ctx context.Context, e env, params json.RawMessage) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	res, err := codeAction(ctx, e, params)
	if err != nil {
		return nil, err
	}
	actions := res.([]protocol.CodeAction)
	var p protocol.CodeActionParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if kindWanted(p.Context, protocol.QuickFix) {
		fixes, err := s.missingImports(ctx, e, p)
		if err != nil {
			return nil, err
		}
		actions = append(actions, fixes...)
	}
	return actions, nil
}

// missingImports returns the quick fixes "Add import" for the undefined
// names of the diagnostics of the request.
func (s *Server) missingImports(ctx context.Context, e env, p protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	var actions []protocol.CodeAction
	var doc *document
	var pf *cache.ParsedFile
	seen := make(map[string]bool)
	for _, d := range p.Context.Diagnostics {
		m := undefinedPattern.FindStringSubmatch(d.Message)
		if m == nil || seen[m[1]] {
			continue
		}
		name := m[1]
		if doc == nil {
			var err error
			if doc, err = documentOfSnapshot(e.snapshot, p.TextDocument.URI); err != nil {
				return nil, err
			}
		}
		// The diagnostic is of the text the client has; the name must still
		// be there.
		start, end, err := doc.mapper.RangeOffsets(d.Range)
		if err != nil || end > len(doc.src) || string(doc.src[start:end]) != name {
			continue
		}
		q, err := newQuery(ctx, e.snapshot, p.TextDocument.URI, d.Range.Start)
		if err != nil {
			return nil, err
		}
		if q == nil {
			continue
		}
		if pf == nil {
			if pf, err = e.snapshot.ParseFile(ctx, p.TextDocument.URI, cache.ParseFull); err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				return nil, nil
			}
		}
		seen[name] = true
		var cands []importable
		for _, c := range s.comp.index.importables(q) {
			if c.name == name {
				cands = append(cands, c)
			}
		}
		// The standard packages first, then by path (importables sorts them).
		slices.SortStableFunc(cands, func(a, b importable) int {
			if a.std != b.std {
				if a.std {
					return -1
				}
				return 1
			}
			return strings.Compare(a.path, b.path)
		})
		for _, c := range cands {
			edits := importEdits(ctx, q, pf, c, s.comp.index.isStd)
			if len(edits) == 0 {
				continue
			}
			elems := make([]protocol.Or_TextDocumentEdit_edits_Elem, len(edits))
			for i, ed := range edits {
				elems[i] = protocol.Or_TextDocumentEdit_edits_Elem{Value: ed}
			}
			actions = append(actions, protocol.CodeAction{
				Title:       fmt.Sprintf("Add import %q", c.path),
				Kind:        protocol.QuickFix,
				Diagnostics: []protocol.Diagnostic{d},
				IsPreferred: len(cands) == 1,
				Edit: &protocol.WorkspaceEdit{DocumentChanges: []protocol.DocumentChange{{
					TextDocumentEdit: &protocol.TextDocumentEdit{
						TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
							Version:                doc.version,
							TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: p.TextDocument.URI},
						},
						Edits: elems,
					},
				}}},
			})
		}
	}
	return actions, nil
}
