// Package server is the language server of "gocpp lsp" (Phase 11): the
// LSP lifecycle (initialize, shutdown, exit) and the text of the open
// documents (Overlays), kept up to date with incremental changes.
//
// Step 11.1 builds only that: the server offers no features yet, so its
// capabilities are the text sync alone. Snapshots, the check pipeline and
// the features (diagnostics, hover, ...) come in the next steps.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/version"
)

// Run serves the LSP over in and out (stdin and stdout for "gocpp lsp")
// with a new Server and returns the exit code (Server.Serve). It logs to
// logger, which may be nil.
func Run(ctx context.Context, in io.Reader, out io.Writer, logger *log.Logger) int {
	return New(logger).Serve(ctx, in, out)
}

// Serve serves the LSP over in and out until the client's exit
// notification or the end of in, and returns the exit code: 0 when the
// client sent shutdown first, 1 when not (LSP 3.17 "exit"), and 1 when the
// stream breaks. Nothing but LSP messages is ever written to out. Serve
// may be called once.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) int {
	s.conn = protocol.NewConn(in, out, s.log)
	err := s.conn.Serve(ctx, s)
	s.mu.Lock()
	shutdown, exited := s.lifecycle == stateShutdown, s.exited
	s.mu.Unlock()
	switch {
	case err == nil && exited:
		// The exit notification.
	case errors.Is(err, io.EOF):
		s.log.Printf("the client closed the connection without the exit notification")
	default:
		s.log.Printf("the connection broke: %v", err)
		return 1
	}
	if !shutdown {
		if exited {
			s.log.Printf("exit without shutdown: exit status 1")
		}
		return 1
	}
	return 0
}

// A lifecycle is where the server is in the LSP lifecycle.
type lifecycle int

const (
	stateNew         lifecycle = iota // before initialize
	stateInitialized                  // initialize has been answered
	stateShutdown                     // after shutdown: only exit is left
)

// A Server is the state of one language server: one client connection. It
// is a protocol.Handler.
type Server struct {
	log      *log.Logger
	overlays *Overlays
	conn     *protocol.Conn // set by Serve

	mu        sync.Mutex
	lifecycle lifecycle
	trace     protocol.TraceValue
	exited    bool // the exit notification came
	// warned holds the documents whose change failed and that the user
	// was told about (didChange); didOpen removes a document.
	warned map[protocol.DocumentURI]bool
}

// New returns a server that logs to logger, which may be nil.
func New(logger *log.Logger) *Server {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Server{log: logger, overlays: NewOverlays(), trace: protocol.Off, warned: make(map[protocol.DocumentURI]bool)}
}

// Overlays returns the set of the open documents.
func (s *Server) Overlays() *Overlays { return s.overlays }

func (s *Server) state() lifecycle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lifecycle
}

func (s *Server) setState(l lifecycle) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lifecycle = l
}

// The methods the server knows. A request whose method is not here gets a
// MethodNotFound error; a notification is dropped.
var (
	requests = map[string]func(*Server, context.Context, json.RawMessage) (any, error){
		"initialize": (*Server).initialize,
		"shutdown":   (*Server).shutdown,
	}
	notifications = map[string]func(*Server, context.Context, json.RawMessage) error{
		"initialized":                      (*Server).initialized,
		"$/setTrace":                       (*Server).setTrace,
		"textDocument/didOpen":             (*Server).didOpen,
		"textDocument/didChange":           (*Server).didChange,
		"textDocument/didClose":            (*Server).didClose,
		"textDocument/didSave":             (*Server).didSave,
		"workspace/didChangeConfiguration": ignore, // no settings yet
		// Requests are handled one at a time, so a $/cancelRequest always
		// comes after its request is answered (protocol.Conn.Serve). Step
		// 11.2 keeps the read loop in order but runs requests in
		// goroutines, with a context per request ID that $/cancelRequest
		// cancels (the answer is then RequestCancelled, or ContentModified
		// when a change made the result stale).
		"$/cancelRequest": ignore,
	}
)

func ignore(*Server, context.Context, json.RawMessage) error { return nil }

// Handle handles one request or notification (protocol.Handler).
func (s *Server) Handle(ctx context.Context, req *protocol.Request) (any, error) {
	if req.Method == "exit" && req.IsNotification() {
		s.mu.Lock()
		s.exited = true
		s.mu.Unlock()
		return nil, protocol.ErrStop
	}
	// LSP 3.17 "initialize": before it, a request gets the error
	// ServerNotInitialized and a notification is dropped. "shutdown": after
	// it, a request gets InvalidRequest, and a notification is dropped too.
	switch s.state() {
	case stateNew:
		if req.Method != "initialize" {
			if req.IsNotification() {
				return nil, nil
			}
			return nil, protocol.Errorf(protocol.ServerNotInitialized, "%s before initialize", req.Method)
		}
	case stateShutdown:
		if req.IsNotification() {
			return nil, nil
		}
		return nil, protocol.Errorf(protocol.InvalidRequest, "%s after shutdown", req.Method)
	}
	if req.IsNotification() {
		h, ok := notifications[req.Method]
		if !ok {
			// "$/" notifications may be ignored without a word (LSP
			// 3.17 "$ Notifications and Requests").
			if !strings.HasPrefix(req.Method, "$/") {
				s.log.Printf("ignored the notification %s", req.Method)
			}
			return nil, nil
		}
		return nil, h(s, ctx, req.Params)
	}
	h, ok := requests[req.Method]
	if !ok {
		return nil, protocol.Errorf(protocol.MethodNotFound, "method not found: %s", req.Method)
	}
	return h(s, ctx, req.Params)
}

// decode decodes the params of a method into v. Wrong params are an
// InvalidParams error.
func decode(params json.RawMessage, v any) error {
	if params == nil {
		return protocol.Errorf(protocol.InvalidParams, "no params")
	}
	if err := json.Unmarshal(params, v); err != nil {
		return protocol.Errorf(protocol.InvalidParams, "bad params: %v", err)
	}
	return nil
}

func (s *Server) initialize(_ context.Context, params json.RawMessage) (any, error) {
	if s.state() != stateNew {
		return nil, protocol.Errorf(protocol.InvalidRequest, "initialize was sent twice")
	}
	var p protocol.ParamInitialize
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.lifecycle = stateInitialized
	if p.Trace != nil {
		s.trace = *p.Trace
	}
	s.mu.Unlock()
	if c := p.ClientInfo; c != nil {
		s.log.Printf("initialize: client %s %s", c.Name, c.Version)
	}
	utf16 := protocol.UTF16
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			// The only position encoding (protocol.Mapper); a client
			// that offers others still gets UTF-16, the default.
			PositionEncoding: &utf16,
			TextDocumentSync: protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.Incremental,
				Save:      &protocol.SaveOptions{}, // didSave, without the text
			},
		},
		ServerInfo: &protocol.ServerInfo{Name: "gocpp", Version: version.Version},
	}, nil
}

func (s *Server) initialized(context.Context, json.RawMessage) error { return nil }

func (s *Server) shutdown(context.Context, json.RawMessage) (any, error) {
	s.setState(stateShutdown)
	return nil, nil
}

func (s *Server) setTrace(_ context.Context, params json.RawMessage) error {
	var p protocol.SetTraceParams
	if err := decode(params, &p); err != nil {
		return err
	}
	s.mu.Lock()
	s.trace = p.Value
	s.mu.Unlock()
	return nil
}

func (s *Server) didOpen(_ context.Context, params json.RawMessage) error {
	var p protocol.DidOpenTextDocumentParams
	if err := decode(params, &p); err != nil {
		return err
	}
	if _, replaced := s.overlays.Open(p.TextDocument); replaced {
		s.log.Printf("didOpen: %s was open already; it has the new text now", p.TextDocument.URI)
	}
	s.mu.Lock()
	delete(s.warned, p.TextDocument.URI)
	s.mu.Unlock()
	return nil
}

func (s *Server) didChange(_ context.Context, params json.RawMessage) error {
	var p protocol.DidChangeTextDocumentParams
	if err := decode(params, &p); err != nil {
		return err
	}
	uri, version := p.TextDocument.URI, p.TextDocument.Version
	if old := s.overlays.Get(uri); old != nil && version <= old.Version() {
		// The spec wants the version to go up. The text is still the
		// client's, so the change is kept.
		s.log.Printf("didChange: %s goes from version %d to %d", uri, old.Version(), version)
	}
	_, err := s.overlays.Change(uri, version, p.ContentChanges)
	if err != nil {
		s.warnBroken(uri)
	}
	return err
}

// warnBroken tells the user, once per document, that a change of uri failed:
// the server's copy of the text is not the editor's any more.
func (s *Server) warnBroken(uri protocol.DocumentURI) {
	s.mu.Lock()
	warned := s.warned[uri]
	s.warned[uri] = true
	s.mu.Unlock()
	if warned || s.conn == nil {
		return
	}
	msg := fmt.Sprintf("gocpp lsp: a change of %s failed, so the server's copy of the file no longer matches the editor's. Close the file and open it again.", uri)
	if err := s.conn.Notify("window/showMessage", &protocol.ShowMessageParams{Type: protocol.Error, Message: msg}); err != nil {
		s.log.Printf("window/showMessage: %v", err)
	}
}

func (s *Server) didClose(_ context.Context, params json.RawMessage) error {
	var p protocol.DidCloseTextDocumentParams
	if err := decode(params, &p); err != nil {
		return err
	}
	return s.overlays.Close(p.TextDocument.URI)
}

func (s *Server) didSave(_ context.Context, params json.RawMessage) error {
	// The file on disk now has the text of the overlay. Nothing reads the
	// disk yet (step 11.3 does).
	var p protocol.DidSaveTextDocumentParams
	return decode(params, &p)
}

// Trace returns the trace level that the client set ($/setTrace, or the
// trace of initialize).
func (s *Server) Trace() protocol.TraceValue {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.trace
}
