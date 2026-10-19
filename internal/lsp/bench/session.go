package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// A Session is an editor's session with one server on one project: the
// open documents, and the operations an editor does (open, type, save,
// ask).
type Session struct {
	P       *Proc
	C       *Client
	Proj    *Project
	Root    string // the folder of the project
	Timeout time.Duration

	Facts    map[string]string // what scenarios found out (the number of references), for the notes
	docs     map[string]*Doc   // the open documents by Rel
	initTime time.Duration     // the process start to the answer to initialize
}

// A Doc is an open document.
type Doc struct {
	Rel     string
	URI     protocol.DocumentURI
	Version int32
	Text    string
	Disk    string // the text on disk
}

// capabilities is what the client says it can do: what a typical editor
// does (markdown hovers, snippets, hierarchical symbols, semantic tokens,
// rename preparation), so that both servers do their normal work.
var capabilities = map[string]any{
	"workspace": map[string]any{
		"workspaceFolders": true,
		"configuration":    true,
		"symbol":           map[string]any{},
	},
	"textDocument": map[string]any{
		"synchronization":    map[string]any{"didSave": true},
		"publishDiagnostics": map[string]any{"relatedInformation": true},
		"hover":              map[string]any{"contentFormat": []string{"markdown", "plaintext"}},
		"completion": map[string]any{"completionItem": map[string]any{
			"snippetSupport": true, "documentationFormat": []string{"markdown", "plaintext"}}},
		"signatureHelp":  map[string]any{"signatureInformation": map[string]any{"documentationFormat": []string{"markdown", "plaintext"}}},
		"documentSymbol": map[string]any{"hierarchicalDocumentSymbolSupport": true},
		"rename":         map[string]any{"prepareSupport": true},
		"definition":     map[string]any{"linkSupport": true},
		"semanticTokens": map[string]any{
			"requests":       map[string]any{"full": true, "range": true},
			"tokenTypes":     []string{"namespace", "type", "class", "enum", "interface", "struct", "typeParameter", "parameter", "variable", "property", "enumMember", "event", "function", "method", "macro", "keyword", "modifier", "comment", "string", "number", "regexp", "operator"},
			"tokenModifiers": []string{"declaration", "definition", "readonly", "static", "deprecated", "abstract", "async", "modification", "documentation", "defaultLibrary"},
			"formats":        []string{"relative"},
		},
		"inlayHint": map[string]any{},
	},
}

// Initialize sends initialize and initialized for the folder root. It
// returns the time from the start of the process to the answer.
func (s *Session) Initialize(ctx context.Context) (time.Duration, error) {
	uri := string(protocol.URIFromPath(s.Root))
	params := map[string]any{
		"processId":    os.Getpid(),
		"clientInfo":   map[string]any{"name": "lspbench", "version": "1"},
		"rootUri":      uri,
		"capabilities": capabilities,
		"workspaceFolders": []map[string]any{{
			"uri": uri, "name": filepath.Base(s.Root),
		}},
	}
	if s.P.Target.Settings != nil {
		params["initializationOptions"] = s.P.Target.Settings
	}
	if _, err := s.C.Call(ctx, "initialize", params, nil); err != nil {
		return 0, err
	}
	d := time.Since(s.P.Started())
	if err := s.C.Notify("initialized", map[string]any{}); err != nil {
		return 0, err
	}
	return d, nil
}

func (s *Session) uri(rel string) protocol.DocumentURI {
	return protocol.URIFromPath(filepath.Join(s.Root, filepath.FromSlash(rel+s.P.Target.Ext)))
}

func (s *Session) path(rel string) string {
	return filepath.Join(s.Root, filepath.FromSlash(rel+s.P.Target.Ext))
}

// Open sends didOpen of the file rel (a Source of the project), with its
// text on disk, and returns the time of the send for the wait that follows.
func (s *Session) Open(rel string) (time.Time, error) {
	d := s.docs[rel]
	if d == nil {
		data, err := os.ReadFile(s.path(rel))
		if err != nil {
			return time.Time{}, err
		}
		d = &Doc{Rel: rel, URI: s.uri(rel), Text: string(data), Disk: string(data)}
		s.docs[rel] = d
	}
	d.Version++
	lang := "gocpp"
	if s.P.Target.Ext == ".go" {
		lang = "go"
	}
	t := time.Now()
	return t, s.C.Notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": d.URI, "languageId": lang, "version": d.Version, "text": d.Text}})
}

// Close sends didClose of the file rel; the document keeps its version,
// so that a later Open goes on from it.
func (s *Session) Close(rel string) error {
	d := s.docs[rel]
	if d == nil {
		return fmt.Errorf("%s is not open", rel)
	}
	d.Text = d.Disk
	return s.C.Notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": d.URI}})
}

// Doc returns the open document rel.
func (s *Session) Doc(rel string) *Doc { return s.docs[rel] }

// Edit replaces the bytes start to end of the document with text, as one
// incremental didChange, and returns the time of the send.
func (s *Session) Edit(rel string, start, end int, text string) (time.Time, error) {
	d := s.docs[rel]
	if d == nil {
		return time.Time{}, fmt.Errorf("%s is not open", rel)
	}
	m := protocol.NewMapper(d.Text)
	rng := protocol.Range{Start: m.Position(start), End: m.Position(end)}
	d.Text = d.Text[:start] + text + d.Text[end:]
	d.Version++
	t := time.Now()
	return t, s.C.Notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": d.URI, "version": d.Version},
		"contentChanges": []map[string]any{{"range": rng, "text": text}},
	})
}

// SetText replaces the whole text of the document (a change event without
// a range), and returns the time of the send.
func (s *Session) SetText(rel, text string) (time.Time, error) {
	d := s.docs[rel]
	if d == nil {
		return time.Time{}, fmt.Errorf("%s is not open", rel)
	}
	d.Text = text
	d.Version++
	t := time.Now()
	return t, s.C.Notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": d.URI, "version": d.Version},
		"contentChanges": []map[string]any{{"text": text}},
	})
}

// Save writes the text of the document to disk and sends didSave.
func (s *Session) Save(rel string) error {
	d := s.docs[rel]
	if d == nil {
		return fmt.Errorf("%s is not open", rel)
	}
	if err := os.WriteFile(s.path(rel), []byte(d.Text), 0o644); err != nil {
		return err
	}
	d.Disk = d.Text
	return s.C.Notify("textDocument/didSave", map[string]any{"textDocument": map[string]any{"uri": d.URI}})
}

// Pos returns the LSP position of the byte offset off of the document.
func (s *Session) Pos(rel string, off int) protocol.Position {
	return protocol.NewMapper(s.docs[rel].Text).Position(off)
}

// Find returns the offset of the first occurrence of needle in the
// document, plus delta.
func (s *Session) Find(rel, needle string, delta int) (int, error) {
	d := s.docs[rel]
	if d == nil {
		return 0, fmt.Errorf("%s is not open", rel)
	}
	i := strings.Index(d.Text, needle)
	if i < 0 {
		return 0, fmt.Errorf("%s has no %q", rel, needle)
	}
	return i + delta, nil
}

// Wait waits for a publish of the file rel after since (a Seq) for which
// ok is true, for at most the session's timeout, and returns the time from
// t0 to when it was read.
func (s *Session) Wait(ctx context.Context, rel string, since int64, t0 time.Time, ok func(*Publish) bool) (time.Duration, *Publish, error) {
	ctx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()
	p, err := s.C.WaitPublish(ctx, s.uri(rel), since, ok)
	if err != nil {
		return 0, nil, fmt.Errorf("no diagnostics of %s: %w", rel, err)
	}
	return p.At.Sub(t0), p, nil
}

// Call sends a request about the document and returns its latency and the
// raw result.
func (s *Session) call(ctx context.Context, method string, params any) (time.Duration, json.RawMessage, error) {
	var raw json.RawMessage
	cctx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()
	d, err := s.C.Call(cctx, method, params, &raw)
	return d, raw, err
}

// A textPos is the params of a request at a place of a document.
type textPos struct {
	TextDocument struct {
		URI protocol.DocumentURI `json:"uri"`
	} `json:"textDocument"`
	Position protocol.Position `json:"position"`
	Context  *struct {
		IncludeDeclaration bool `json:"includeDeclaration"`
	} `json:"context,omitempty"`
	NewName string `json:"newName,omitempty"`
}

func (s *Session) at(rel string, off int) textPos {
	var p textPos
	p.TextDocument.URI = s.docs[rel].URI
	p.Position = s.Pos(rel, off)
	return p
}

// nonEmpty reports whether the result of a request has something in it.
func nonEmpty(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	switch s {
	case "", "null", "[]", "{}", `{"items":[]}`, `{"items":null}`, `{"isIncomplete":false,"items":[]}`, `{"isIncomplete":true,"items":[]}`:
		return false
	}
	return true
}

// baseline returns the number of errors the server last showed in the file
// rel (a project that is clean for the server has none; a server that
// finds something in it anyway must not make the waits fail).
func (s *Session) baseline(rel string) int {
	if p := s.C.LastPublish(s.uri(rel)); p != nil {
		return p.Errors()
	}
	return 0
}

// moreErrors is true of a publish with more errors than base: the error
// that an edit introduced.
func moreErrors(base int) func(*Publish) bool {
	return func(p *Publish) bool { return p.Errors() > base }
}

// fewerErrors is true of a publish with base errors or fewer: the edit is
// undone.
func fewerErrors(base int) func(*Publish) bool {
	return func(p *Publish) bool { return p.Errors() <= base }
}
