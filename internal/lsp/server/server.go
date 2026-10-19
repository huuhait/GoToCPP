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
// The session's snapshots have the memoized check pipeline (cache.Pipeline,
// step 11.3). The features so far: the text sync, the diagnostics as you
// type (step 11.4, diagnostics.go: textDocument/publishDiagnostics, with
// workspace/didChangeWatchedFiles for the files that change on disk),
// hover, go to definition and signature help (step 11.5, navigation.go),
// completion (step 11.6, completion.go), and formatting and organize
// imports (step 11.7, format.go), and references, document highlights and
// rename (step 11.8, refs.go and rename.go), and document and workspace
// symbols (step 11.8, symbols.go), and semantic tokens (semantic.go), inlay
// hints (hints.go), document links (links.go) and the quick fix "Add import"
// (actions.go) (step 11.9). The features for ! and macros come with step
// 11.13.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/version"
	"github.com/huuhait/gocpp/stdlib"
)

// Run serves the LSP over in and out (stdin and stdout for "gocpp lsp")
// with a new Server and returns the exit code (Server.Serve). It logs to
// logger, which may be nil. Unlike New, the server keeps the summaries of
// the packages it checks in the cache on disk that the environment says
// (openDisk), so that its next run starts warm.
func Run(ctx context.Context, in io.Reader, out io.Writer, logger *log.Logger) int {
	return RunWith(ctx, in, out, logger, Options{})
}

// RunWith is Run with the options of the slow-request log and the span
// recorder (step 11.12).
func RunWith(ctx context.Context, in io.Reader, out io.Writer, logger *log.Logger, opts Options) int {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	cfg := pipelineConfig()
	cfg.Trace = opts.Trace
	cfg.Phase = opts.Phase
	cfg.MaxCheckBytes = int64(opts.CheckCacheMB) << 20
	if disk := openDisk(logger); disk != nil {
		defer disk.Close()
		cfg.Disk = disk
	}
	return newServerOpts(logger, cfg, debounce.RealClock{}, opts).Serve(ctx, in, out)
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
	s.conn.OnEnd(func() {
		// The background work stops: nothing is checked or published
		// any more.
		s.session.Shutdown()
		s.diags.stop()
		s.comp.stop()
		s.pipeline.Close()
	})
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
	log      *log.Logger
	pipeline *cache.Pipeline
	session  *cache.Session
	diags    *diagnostics
	obs      *observer // the slow log, the spans and the stats (observe.go)
	// wideFamily is Options.WideFamily.
	wideFamily bool
	// dynamicLimit is Options.DynamicLimit, 0 for the default; negative: none.
	dynLimitOpt atomic.Int64
	conn        *protocol.Conn // set by Serve
	// requests are the methods of the requests that run on a snapshot
	// (see method); tests add their own before Serve.
	requests     map[string]method
	shutdownWait time.Duration
	nav          navigation    // hover, definition, signature help (navigation.go)
	comp         completion    // completion (completion.go)
	refactor     refactor      // references and rename (refs.go, rename.go)
	symbols      symbolFeature // document and workspace symbols (symbols.go)
	syn          synTable      // the syntax index of the dynamic family (dynindex.go)
	modsMu       sync.Mutex
	mods         struct {
		gen  uint64
		key  string
		dirs []string
	} // the modules under the workspace folders (family.go)
	modsTold map[string]bool

	mu        sync.Mutex
	lifecycle lifecycle
	trace     protocol.TraceValue
	exited    bool // the exit notification came
	// watchFiles: the client can register workspace/didChangeWatchedFiles
	// (initialize), so initialized does.
	watchFiles bool
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
	return newServer(logger, pipelineConfig(), debounce.RealClock{})
}

// newServer returns a server with the pipeline configuration cfg (whose
// OnProbe it sets) and the clock of its diagnostics' debouncer.
func newServer(logger *log.Logger, cfg cache.Config, clock debounce.Clock) *Server {
	return newServerOpts(logger, cfg, clock, Options{})
}

// newServerOpts is newServer with the options opts.
func newServerOpts(logger *log.Logger, cfg cache.Config, clock debounce.Clock, opts Options) *Server {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	s := &Server{
		log:          logger,
		requests:     map[string]method{}, // the features come with steps 11.5 to 11.9
		shutdownWait: shutdownWait,
		trace:        protocol.Off,
		warned:       make(map[protocol.DocumentURI]bool),
		stale:        make(map[protocol.DocumentURI]map[protocol.ID]*staleToken),
		wideFamily:   opts.WideFamily,
	}
	s.dynLimitOpt.Store(int64(opts.DynamicLimit))
	s.obs = newObserver(s, opts)
	s.addObservability()
	s.addNavigation()
	s.addRefactor()
	s.addSymbols()
	s.addSemantic()
	s.addHints()
	s.addLinks()
	s.requests["textDocument/codeAction"] = method{run: s.codeActions, contentModified: true}
	for name, m := range formatMethods {
		s.requests[name] = m
	}
	s.diags = newDiagnostics(s, clock)
	cfg.OnProbe = s.diags.probed
	if cfg.Trace == nil {
		cfg.Trace = opts.Trace
	}
	s.pipeline = cache.NewPipeline(cfg)
	s.addCompletion(s.pipeline)
	s.session = cache.NewSession(s.pipeline.NewState())
	return s
}

// pipelineConfig returns the configuration of the check pipeline of a
// session: the gocpp standard library, for the system that runs the
// server, as gocpp check does.
func pipelineConfig() cache.Config {
	return cache.Config{Stdlib: stdlib.FS(), GoStd: stdlib.IsStd}
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
	"workspace/didChangeWatchedFiles":  (*Server).didChangeWatchedFiles,
	"workspace/didChangeConfiguration": (*Server).didChangeConfiguration,
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
	ctx, span := s.obs.begin(ctx, req.Method)
	defer s.obs.end(span, func() string { return fmt.Sprintf("uri=%s (now: %s)", documentOf(req.Params), s.snapshotInfo()) })
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
		ctx, span := s.obs.begin(ctx, req.Method, "snapshot", snap.ID())
		defer s.obs.end(span, func() string {
			return fmt.Sprintf("uri=%s snapshot=%d (now: %s)", documentOf(req.Params), snap.ID(), s.snapshotInfo())
		})
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
	s.watchFiles = p.Capabilities.Workspace.DidChangeWatchedFiles.DynamicRegistration
	s.mu.Unlock()
	s.diags.related.Store(p.Capabilities.TextDocument.PublishDiagnostics.RelatedInformation)
	if raw, err := json.Marshal(p.InitializationOptions); err == nil {
		s.applySettings(raw)
	}
	if c := p.ClientInfo; c != nil {
		s.log.Printf("initialize: client %s %s", c.Name, c.Version)
	}
	utf16 := protocol.UTF16
	res := &protocol.InitializeResult{
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
	}
	s.nav.initialize(p.Capabilities, &res.Capabilities)
	observabilityCapabilities(&res.Capabilities)
	s.refactor.initialize(p.Capabilities, &res.Capabilities)
	s.symbols.initialize(&p, &res.Capabilities)
	semanticCapabilities(&res.Capabilities)
	hintCapabilities(&res.Capabilities)
	linkCapabilities(&res.Capabilities)
	formatCapabilities(&res.Capabilities, &p.Capabilities)
	s.comp.initialize(p.Capabilities, &res.Capabilities, s.nav.hoverMarkdown.Load())
	return res, nil
}

// watchedFiles are the files whose changes on disk the server wants to hear
// of (workspace/didChangeWatchedFiles): the .gp files and gocpp.mod files,
// which the snapshots read, and the C/C++ headers, which the namespace probe
// reads.
var watchedFiles = []string{"**/*.gp", "**/gocpp.mod", "**/*.{h,hh,hpp,hxx,inc}"}

// initialized registers workspace/didChangeWatchedFiles when the client can
// (like gopls; a client without dynamic registration may send it anyway).
func (s *Server) initialized(context.Context, json.RawMessage) error {
	s.warmSymbols() // the index of the workspace, in the background
	s.mu.Lock()
	watch := s.watchFiles
	s.mu.Unlock()
	if !watch {
		return nil
	}
	watchers := make([]protocol.FileSystemWatcher, len(watchedFiles))
	for i, glob := range watchedFiles {
		watchers[i] = protocol.FileSystemWatcher{GlobPattern: protocol.GlobPattern{Value: glob}}
	}
	return s.conn.Request("client/registerCapability", &protocol.RegistrationParams{
		Registrations: []protocol.Registration{{
			ID:              "gocpp-watched-files",
			Method:          "workspace/didChangeWatchedFiles",
			RegisterOptions: protocol.DidChangeWatchedFilesRegistrationOptions{Watchers: watchers},
		}},
	})
}

// shutdown starts the shutdown request id. From now on, every request but
// exit gets an error (Call). The answer waits until the requests in flight
// have answered, for up to s.shutdownWait; then the ones left are
// cancelled (RequestCancelled). So the answer to shutdown comes after the
// answers to every request before it. The diagnostics stop first: none is
// published after the answer.
func (s *Server) shutdown(id protocol.ID) protocol.Work {
	s.mu.Lock()
	s.lifecycle = stateShutdown
	s.shutdownID = &id
	s.mu.Unlock()
	return func(ctx context.Context) (any, error) {
		s.diags.stop()
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
	s.diags.scheduleFile(uri, 0)
	if path := uri.Path(); path != "" {
		s.comp.index.warm(filepath.Dir(path)) // the packages to import (autoimport.go)
	}
	s.warmSymbols()
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
		s.diags.scheduleChange(uri)
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
	s.diags.scheduleFile(p.TextDocument.URI, 0)
	return nil
}

func (s *Server) didSave(_ context.Context, params json.RawMessage) error {
	// The file on disk now has the text of the overlay, which the
	// snapshots read anyway: the run finds what it published, and
	// publishes nothing (it is cheap: everything is cached).
	var p protocol.DidSaveTextDocumentParams
	if err := decode(params, &p); err != nil {
		return err
	}
	// A client without a file watcher reports no change on disk but this:
	// the saved file may have another package name now.
	s.comp.index.changed([]protocol.FileEvent{{URI: p.TextDocument.URI, Type: protocol.Changed}})
	s.diags.scheduleFile(p.TextDocument.URI, 0)
	return nil
}

// isHeader reports whether the file is a C/C++ header (watchedFiles).
func isHeader(path string) bool {
	switch filepath.Ext(path) {
	case ".h", ".hh", ".hpp", ".hxx", ".inc":
		return true
	}
	return false
}

// didChangeWatchedFiles takes in files that changed on disk, outside the
// editor (a git checkout, a generator, another editor): a new snapshot
// reads them again (Session.Invalidate) and the open packages they are in
// are diagnosed again, after the quiet time of a didChange (a checkout
// reports many files, in several notifications); their importers follow
// when their API changed. A changed file of a package that is not open, a
// gocpp.mod file or a folder (anything else but a header) can change every
// open package: every open package is diagnosed again (only what the
// change touches is checked again, step 11.3), and the package that is not
// open is not checked for itself. A
// header changes no snapshot: the namespace probe checks its answers again
// in the background (Pipeline.HeadersChanged), and the packages whose
// answers changed are diagnosed again (diagnostics.probed). The events of
// an open document change nothing: its overlay is what counts.
func (s *Server) didChangeWatchedFiles(_ context.Context, params json.RawMessage) error {
	var p protocol.DidChangeWatchedFilesParams
	if err := decode(params, &p); err != nil {
		return err
	}
	s.comp.index.changed(p.Changes) // a package may be new, or gone
	s.symbols.forget(p.Changes)
	var uris []protocol.DocumentURI
	dirs := make(map[string]bool)
	headers, all := false, false
	for _, c := range p.Changes {
		path := c.URI.Path()
		switch {
		case path == "":
			continue
		case isHeader(path):
			headers = true
			continue
		case strings.HasSuffix(path, ".gp"):
			dirs[filepath.Dir(path)] = true
		default:
			all = true // gocpp.mod, or a folder
		}
		uris = append(uris, c.URI)
	}
	if headers {
		s.pipeline.HeadersChanged()
	}
	if len(uris) == 0 {
		return nil
	}
	if err := s.session.Invalidate(uris); err != nil {
		return err
	}
	if all {
		s.diags.scheduleOpen("", s.diags.delay)
		return nil
	}
	snap, release := s.session.Snapshot()
	open := openDirs(snap)
	release()
	for dir := range dirs {
		if !open[dir] {
			s.diags.scheduleOpen("", s.diags.delay)
			return nil
		}
	}
	for dir := range dirs {
		s.diags.schedule(dir, s.diags.delay)
	}
	return nil
}

// Trace returns the trace level that the client set ($/setTrace, or the
// trace of initialize).
func (s *Server) Trace() protocol.TraceValue {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.trace
}
