package server

// Formatting and organize imports (step 11.7).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
	gpformat "github.com/huuhait/gocpp/internal/format"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// formatMethods are the requests of step 11.7. A change of the document
// makes their edits stale, so it cancels them (ContentModified).
var formatMethods = map[string]method{
	"textDocument/formatting": {run: formatting, contentModified: true},
	"textDocument/codeAction": {run: codeAction, contentModified: true},
}

// formatCapabilities adds the capabilities of step 11.7 to caps, for a
// client with the capabilities client: formatting, and the code action
// source.organizeImports. Like gopls, the server names its code action
// kinds only to a client that understands code action literals (the LSP
// allows CodeActionOptions only then).
func formatCapabilities(caps *protocol.ServerCapabilities, client *protocol.ClientCapabilities) {
	caps.DocumentFormattingProvider = &protocol.Or_ServerCapabilities_documentFormattingProvider{Value: true}
	caps.CodeActionProvider = true
	if len(client.TextDocument.CodeAction.CodeActionLiteralSupport.CodeActionKind.ValueSet) > 0 {
		caps.CodeActionProvider = &protocol.CodeActionOptions{
			CodeActionKinds: []protocol.CodeActionKind{protocol.SourceOrganizeImports},
		}
	}
}

// A document is the text of a document in a snapshot: its overlay, or the
// file on disk.
type document struct {
	path    string
	src     []byte
	mapper  *protocol.Mapper
	version int32 // the overlay's version; 0 for a file on disk
}

// documentOfSnapshot returns the document uri of the snapshot.
func documentOfSnapshot(snap *cache.Snapshot, uri protocol.DocumentURI) (*document, error) {
	path := uri.Path()
	if path == "" {
		return nil, protocol.Errorf(protocol.InvalidParams, "%s is not a file", uri)
	}
	if o := snap.Overlay(uri); o != nil {
		if o.Broken() {
			// The text is not the editor's: edits would go to the
			// wrong places (Session.DidChange).
			return nil, protocol.Errorf(protocol.RequestFailed, "%s: the server's copy of the file no longer matches the editor's", uri)
		}
		return &document{path: path, src: []byte(o.Text()), mapper: o.Mapper(), version: o.Version()}, nil
	}
	src, err := snap.ReadFile(path)
	if err != nil {
		return nil, protocol.Errorf(protocol.RequestFailed, "%v", err)
	}
	return &document{path: path, src: src, mapper: protocol.NewMapper(string(src))}, nil
}

// formatting answers textDocument/formatting: the edits that turn the
// document into what gocpp fmt makes of it (internal/format.Source, the
// same formatter), as small edits (computeEdits). The options of the
// request (tab size, ...) are ignored, as in gopls: the format is gofmt's.
//
// Like gopls, a document with syntax errors (or errors in its //gocpp:
// directives) is not formatted: the answer is an error (RequestFailed)
// with the first error.
func formatting(ctx context.Context, e env, params json.RawMessage) (any, error) {
	var p protocol.DocumentFormattingParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	doc, err := documentOfSnapshot(e.snapshot, p.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	edits, err := formatEdits(ctx, doc)
	if err != nil {
		return nil, err
	}
	if edits == nil {
		edits = []protocol.TextEdit{}
	}
	return edits, nil
}

// formatEdits returns the edits that format doc.
func formatEdits(ctx context.Context, doc *document) ([]protocol.TextEdit, error) {
	res, err := gpformat.Source(doc.path, doc.src)
	if err != nil {
		return nil, formatError(err)
	}
	// A safety net: the result must parse. go/printer has a known bug
	// that drops needed parentheses (range (std.vector[int]{7, 8}), see
	// the plan's TODO); then the file is left alone.
	if _, err := parser.ParseFile(token.NewFileSet(), doc.path, res, gpformat.ParserMode); err != nil {
		return nil, protocol.Errorf(protocol.RequestFailed, "gocpp fmt would break the file, so it is not formatted: %v", firstError(err))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return computeEdits(ctx, doc.mapper, string(res)), nil
}

// formatError returns the answer to a formatting request whose file
// doesn't format: RequestFailed with the first error.
func formatError(err error) error {
	return protocol.Errorf(protocol.RequestFailed, "%v", firstError(err))
}

// firstError returns the first error of a scanner.ErrorList and how many
// more there are, or err itself.
func firstError(err error) string {
	var list scanner.ErrorList
	if errors.As(err, &list) && len(list) > 0 {
		if len(list) == 1 {
			return list[0].Error()
		}
		return list[0].Error() + " (and " + strconv.Itoa(len(list)-1) + " more errors)"
	}
	return err.Error()
}

// codeAction answers textDocument/codeAction. The only code action so far
// is "Organize Imports" (source.organizeImports), for the whole document
// (the range of the request doesn't matter), which also serves the
// editor's "organize imports on save" (VS Code's editor.codeActionsOnSave
// asks for exactly that kind). The kinds follow gopls: a kind in
// context.only takes its sub-kinds too ("source" takes
// "source.organizeImports"); without context.only, a request that the
// editor made by itself (triggerKind Automatic, as when the cursor moves)
// gets only quick fixes, and any other request gets every kind.
//
// Like gopls, there is no action when the document has syntax errors or
// when its imports are organized already.
func codeAction(ctx context.Context, e env, params json.RawMessage) (any, error) {
	var p protocol.CodeActionParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	actions := []protocol.CodeAction{}
	if !kindWanted(p.Context, protocol.SourceOrganizeImports) {
		return actions, nil
	}
	uri := p.TextDocument.URI
	doc, err := documentOfSnapshot(e.snapshot, uri)
	if err != nil {
		return nil, err
	}
	edits, err := organizeEdits(ctx, e.snapshot, uri, doc)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		e.log.Printf("organize imports of %s: %v", uri, err)
		return actions, nil
	}
	if len(edits) == 0 {
		return actions, nil
	}
	elems := make([]protocol.Or_TextDocumentEdit_edits_Elem, len(edits))
	for i, ed := range edits {
		elems[i] = protocol.Or_TextDocumentEdit_edits_Elem{Value: ed}
	}
	actions = append(actions, protocol.CodeAction{
		Title: "Organize Imports",
		Kind:  protocol.SourceOrganizeImports,
		Edit: &protocol.WorkspaceEdit{DocumentChanges: []protocol.DocumentChange{{
			TextDocumentEdit: &protocol.TextDocumentEdit{
				TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
					Version:                doc.version,
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
				},
				Edits: elems,
			},
		}}},
	})
	return actions, nil
}

// kindWanted reports whether the code action kind is asked for (see
// codeAction).
func kindWanted(c protocol.CodeActionContext, kind protocol.CodeActionKind) bool {
	if len(c.Only) == 0 {
		if c.TriggerKind != nil && *c.TriggerKind == protocol.CodeActionAutomatic {
			return kind == protocol.QuickFix
		}
		return true
	}
	for _, only := range c.Only {
		if only == "" || only == kind || strings.HasPrefix(string(kind), string(only)+".") {
			return true
		}
	}
	return false
}

// organizeEdits returns the edits of "Organize Imports" for doc, the
// document uri of snap (organizeImports). It is an error when the
// document has syntax errors.
func organizeEdits(ctx context.Context, snap *cache.Snapshot, uri protocol.DocumentURI, doc *document) ([]protocol.TextEdit, error) {
	pf, err := snap.ParseFile(ctx, uri, cache.ParseFull)
	if err != nil {
		return nil, err
	}
	if pf.Err != nil {
		return nil, pf.Err
	}
	if !bytes.Equal(pf.Src, doc.src) {
		// Both come from the snapshot, so this should not happen.
		return nil, errors.New("the parsed file is not the document")
	}
	unused := unusedImports(ctx, snap, uri, pf)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	res, err := organizeImports(pf.Path, pf.Src, pf.File, pf.Tok, unused)
	if err != nil {
		return nil, err
	}
	// A safety net, as for formatting: the result must parse.
	if _, err := parser.ParseFile(token.NewFileSet(), pf.Path, res, gpformat.ParserMode); err != nil {
		return nil, fmt.Errorf("the organized file would not parse: %v", firstError(err))
	}
	return computeEdits(ctx, doc.mapper, string(res)), nil
}

// unusedImports returns the gocpp imports of the parsed file pf (the
// document uri) that the type check of its package reports as "imported
// and not used", by the byte offsets of their specs. It is empty when the
// check can't say for sure: when the package doesn't check (the error is
// dropped: organize imports still sorts), or when the background
// namespace probe hasn't answered the check yet or answers it differently
// now (cache.Package.Provisional and Stale), because its answers decide
// which names are C++ names and so which imports are used.
//
// Header and namespace imports are never removed (v0): the checker never
// reports them, because a header also brings macros and other things that
// the checker can't see being used.
func unusedImports(ctx context.Context, snap *cache.Snapshot, uri protocol.DocumentURI, pf *cache.ParsedFile) map[int]bool {
	pkg, err := snap.PackageOf(ctx, uri)
	if err != nil || pkg.Check == nil || pkg.Provisional() || pkg.Stale() {
		return nil
	}
	specs := make(map[int]*ast.ImportSpec)
	for _, s := range pf.File.Imports {
		specs[pf.Tok.Offset(s.Pos())] = s
	}
	unused := make(map[int]bool)
	for _, d := range pkg.Check.Errors.Diagnostics() {
		if d.Code != diag.UnusedImport || d.Pos.Filename != pf.Path {
			continue
		}
		s := specs[d.Pos.Offset]
		if s == nil {
			continue
		}
		path, err := strconv.Unquote(s.Path.Value)
		// The message starts with the path ("\"fmt\" imported and not
		// used"): a check of other text can't remove the wrong import.
		if err != nil || load.IsHeaderOrNamespace(path) || !strings.HasPrefix(d.Msg, strconv.Quote(path)+" imported") {
			continue
		}
		unused[d.Pos.Offset] = true
	}
	return unused
}
