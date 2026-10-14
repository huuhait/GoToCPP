// Package server is the language server of "gocpp lsp" (Phase 11): the
// LSP lifecycle (initialize, shutdown, exit), the text of the open
// documents kept up to date with incremental changes, and the way requests
// run (step 11.2).
//
// Notifications are handled one at a time, in order, on the read loop
// (protocol.Handler); each change of a document makes a new snapshot
// (cache.Session). A request takes the snapshot that is current when it
// comes, on the read loop too, so it sees every change that came before it
// and none after; then it runs on its own goroutine, at the same time as
// the next messages, until it answers or is cancelled.
//
// The server offers no features yet, so its capabilities are the text sync
// alone. The session's snapshots have the memoized check pipeline
// (cache.Pipeline, step 11.3), which the features (diagnostics, hover, ...)
// of the next steps use.
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
	"time"

	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/version"
	"github.com/huuhait/gocpp/stdlib"
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
// input breaks. A client that stops reading out (it closed the pipe) is
// like one that closes in: after shutdown, that is exit code 0. Nothing but
// LSP messages is ever written to out. Serve may be called once.
//
// When the read loop ends, the session shuts down at once (the background
// work stops), and nothing more is written after exit: the requests still
// running are cancelled and get no answer, and a notification fails.
// Serve waits up to a second for them to return (protocol.Conn.Serve). An
// exit after shutdown first lets shutdown's answer out (Notify).
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) int {
	s.conn = protocol.NewConn(in, out, s.log)
	s.conn.OnEnd(s.session.Shutdown)
	err := s.conn.Serve(ctx, s)
	s.mu.Lock()
	shutdown, exited := s.lifecycle == stateShutdown, s.exited
	s.mu.Unlock()
	switch {
	case err == nil && exited:
		// The exit notification.
	case errors.Is(err, io.EOF):
		s.log.Printf("the client closed the connection without the exit notification")
	case errors.As(err, new(*protocol.WriteError)):
		s.log.Printf("the client stopped reading: %v", err)
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

// shutdownWait is how long the shutdown request waits for the requests in
// flight to answer before it cancels them.
const shutdownWait = time.Second

// A Server is the state of one language server: one client connection. It
// is a protocol.Handler.
type Server struct {
	log     *log.Logger
	session *cache.Session
	conn    *protocol.Conn // set by Serve
	// requests are the methods of the requests that run on a snapshot
	// (see method); tests add their own before Serve.
	requests     map[string]method
	shutdownWait time.Duration

	mu        sync.Mutex
	lifecycle lifecycle
	trace     protocol.TraceValue
	exited    bool // the exit notification came
	// warned holds the documents whose change failed and that the user
	// was told about (didChange); didOpen removes a document.
	warned map[protocol.DocumentURI]bool
	// stale holds, per document, the requests in flight whose result a
	// change of the document makes stale (method.contentModified). The
	// value is the request's own token, so that a request that ends
	// removes itself and never a newer request with the same ID.
	stale map[protocol.DocumentURI]map[protocol.ID]*staleToken
	// shutdownID is the ID of the shutdown request, once it came.
	shutdownID *protocol.ID
}

type staleToken struct{}

// A method is a request that works on a snapshot.
type method struct {
	// run does the work on the request's own goroutine, with what env
	// gives it: the snapshot that was current when the request came, and
	// never a newer one. ctx is done when the request is cancelled (the
	// answer has been sent then).
	run func(ctx context.Context, env env, params json.RawMessage) (any, error)
	// contentModified: a change of the request's document (the
	// textDocument.uri of its params) while it runs makes its result
	// stale, so the change cancels it with ContentModified (LSP 3.17
	// "ContentModified"; semantic tokens, for example). Without it, a
	// request finishes on its snapshot, and the client gets the answer
	// for the text it had when it asked (hover, completion, ...).
	contentModified bool
}

// An env is what the work of a request may use.
type env struct {
	snapshot *cache.Snapshot
	log      *log.Logger
	// notify sends a notification to the client (protocol.Conn.Notify).
	notify func(method string, params any) error
}

// New returns a server that logs to logger, which may be nil.
func New(logger *log.Logger) *Server {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Server{
		log:          logger,
		session:      cache.NewSession(newPipeline().NewState()),
		requests:     map[string]method{}, // the features come with steps 11.4 to 11.9
		shutdownWait: shutdownWait,
		trace:        protocol.Off,
		warned:       make(map[protocol.DocumentURI]bool),
		stale:        make(map[protocol.DocumentURI]map[protocol.ID]*staleToken),
	}
}

// newPipeline returns the check pipeline of a session: the gocpp standard
// library, for the system that runs the server, as gocpp check does.
func newPipeline() *cache.Pipeline {
	return cache.NewPipeline(cache.Config{Stdlib: stdlib.FS(), GoStd: stdlib.IsStd})
}

// Session returns the server's session: the snapshots of the open
// documents.
func (s *Server) Session() *cache.Session { return s.session }

func (s *Server) state() lifecycle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lifecycle
}

// The notifications the server knows; another one is dropped. They run on
// the read loop ($/cancelRequest is protocol.Conn's own).
var notifications = map[string]func(*Server, context.Context, json.RawMessage) error{
	"initialized":                      (*Server).initialized,
	"$/setTrace":                       (*Server).setTrace,
	"textDocument/didOpen":             (*Server).didOpen,
	"textDocument/didChange":           (*Server).didChange,
	"textDocument/didClose":            (*Server).didClose,
	"textDocument/didSave":             (*Server).didSave,
	"workspace/didChangeConfiguration": ignore, // no settings yet
}

func ignore(*Server, context.Context, json.RawMessage) error { return nil }

// Notify handles one notification, on the read loop (protocol.Handler).
func (s *Server) Notify(ctx context.Context, req *protocol.Request) error {
	if req.Method == "exit" {
		s.mu.Lock()
		s.exited = true
		shutdownID := s.shutdownID
		s.mu.Unlock()
		if shutdownID != nil {
			// The client should wait for shutdown's answer before exit,
			// but a script may not. Shutdown answers within shutdownWait
			// (it cancels what runs longer), so its answer, and the ones
			// before it, go out before nothing more is written.
			ctx, cancel := context.WithTimeout(ctx, s.shutdownWait+time.Second)
			defer cancel()
			s.conn.Wait(ctx)
		}
		return protocol.ErrStop
	}
	// LSP 3.17 "initialize": a notification before it is dropped.
	// "shutdown": after it, a notification is dropped too.
	if s.state() != stateInitialized {
		return nil
	}
	h, ok := notifications[req.Method]
	if !ok {
		// "$/" notifications may be ignored without a word (LSP 3.17 "$
		// Notifications and Requests").
		if !strings.HasPrefix(req.Method, "$/") {
			s.log.Printf("ignored the notification %s", req.Method)
		}
		return nil
	}
	return h(s, ctx, req.Params)
}

// Call starts one request, on the read loop (protocol.Handler): the
// lifecycle requests are answered here, in order; another request takes the
// current snapshot here and does its work in the returned Work.
func (s *Server) Call(ctx context.Context, req *protocol.Request) protocol.Work {
	// LSP 3.17 "initialize": before it, a request gets the error
	// ServerNotInitialized. "shutdown": after it, a request gets
	// InvalidRequest.
	switch s.state() {
	case stateNew:
		if req.Method != "initialize" {
			return protocol.Reply(nil, protocol.Errorf(protocol.ServerNotInitialized, "%s before initialize", req.Method))
		}
	case stateShutdown:
		return protocol.Reply(nil, protocol.Errorf(protocol.InvalidRequest, "%s after shutdown", req.Method))
	}
	switch req.Method {
	case "initialize":
		return protocol.Reply(s.initialize(req.Params))
	case "shutdown":
		return s.shutdown(*req.ID)
	}
	m, ok := s.requests[req.Method]
	if !ok {
		return protocol.Reply(nil, protocol.Errorf(protocol.MethodNotFound, "method not found: %s", req.Method))
	}
	snap, release := s.session.Snapshot()
	id := *req.ID
	var uri protocol.DocumentURI
	var token *staleToken
	if m.contentModified {
		uri = documentOf(req.Params)
		token = s.addStale(uri, id)
	}
	e := env{snapshot: snap, log: s.log, notify: s.conn.Notify}
	return func(ctx context.Context) (any, error) {
		defer release()
		if token != nil {
			defer s.removeStale(uri, id, token)
		}
		return m.run(ctx, e, req.Params)
	}
}

// documentOf returns the document of a request: the textDocument.uri of its
// params, or "" when they have none.
func documentOf(params json.RawMessage) protocol.DocumentURI {
	var p struct {
		TextDocument struct {
			URI protocol.DocumentURI `json:"uri"`
		} `json:"textDocument"`
	}
	// A bad URI is a decode error; the request's own decode reports it.
	if json.Unmarshal(params, &p) != nil {
		return ""
	}
	return p.TextDocument.URI
}

// addStale records the request id on uri (method.contentModified) and
// returns its token, or nil when the request has no document.
func (s *Server) addStale(uri protocol.DocumentURI, id protocol.ID) *staleToken {
	if uri == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stale[uri] == nil {
		s.stale[uri] = make(map[protocol.ID]*staleToken)
	}
	token := new(staleToken)
	s.stale[uri][id] = token
	return token
}

// removeStale removes the request id with the token from uri, unless a
// newer request with the same ID has taken its place.
func (s *Server) removeStale(uri protocol.DocumentURI, id protocol.ID, token *staleToken) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stale[uri][id] != token {
		return
	}
	delete(s.stale[uri], id)
	if len(s.stale[uri]) == 0 {
		delete(s.stale, uri)
	}
}

// modified cancels, with ContentModified, the requests in flight whose
// result the change of uri made stale.
func (s *Server) modified(uri protocol.DocumentURI) {
	s.mu.Lock()
	ids := s.stale[uri]
	delete(s.stale, uri)
	s.mu.Unlock()
	for id := range ids {
		s.conn.Cancel(id, protocol.Errorf(protocol.ContentModified, "%s changed", uri))
	}
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

func (s *Server) initialize(params json.RawMessage) (any, error) {
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

// shutdown starts the shutdown request id. From now on, every request but
// exit gets an error (Call). The answer waits until the requests in flight
// have answered, for up to s.shutdownWait; then the ones left are
// cancelled (RequestCancelled). So the answer to shutdown comes after the
// answers to every request before it.
func (s *Server) shutdown(id protocol.ID) protocol.Work {
	s.mu.Lock()
	s.lifecycle = stateShutdown
	s.shutdownID = &id
	s.mu.Unlock()
	return func(ctx context.Context) (any, error) {
		wctx, cancel := context.WithTimeout(ctx, s.shutdownWait)
		defer cancel()
		if s.conn.Wait(wctx, id) || ctx.Err() != nil {
			// All answered, or the connection ended: then nothing is
			// written any more.
			return nil, nil
		}
		s.log.Printf("shutdown: cancelled the requests still running after %v", s.shutdownWait)
		s.conn.CancelAll(protocol.Errorf(protocol.RequestCancelled, "the server shuts down"), id)
		// The answers are written now, or being written: wait for that
		// too, but not long (the output may be stuck).
		wctx, cancel = context.WithTimeout(ctx, time.Second)
		defer cancel()
		s.conn.Wait(wctx, id)
		return nil, nil
	}
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
	uri := p.TextDocument.URI
	replaced, err := s.session.DidOpen(p.TextDocument)
	if err != nil {
		return err
	}
	if replaced {
		s.log.Printf("didOpen: %s was open already; it has the new text now", uri)
		s.modified(uri)
	}
	s.mu.Lock()
	delete(s.warned, uri)
	s.mu.Unlock()
	return nil
}

func (s *Server) didChange(_ context.Context, params json.RawMessage) error {
	var p protocol.DidChangeTextDocumentParams
	if err := decode(params, &p); err != nil {
		return err
	}
	uri, version := p.TextDocument.URI, p.TextDocument.Version
	snap, release := s.session.Snapshot()
	old := snap.Overlay(uri)
	release()
	if old != nil && version <= old.Version() {
		// The spec wants the version to go up. The text is still the
		// client's, so the change is kept.
		s.log.Printf("didChange: %s goes from version %d to %d", uri, old.Version(), version)
	}
	err := s.session.DidChange(uri, version, p.ContentChanges)
	if old != nil {
		s.modified(uri) // a failed change made it broken: stale too
	}
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
	if err := s.session.DidClose(p.TextDocument.URI); err != nil {
		return err
	}
	s.modified(p.TextDocument.URI)
	return nil
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
