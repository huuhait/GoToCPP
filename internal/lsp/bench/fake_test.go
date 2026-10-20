package bench

import (
	"context"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// A fakeServer is a language server that answers like a real one would
// but computes nothing: every request gets a fixed answer after its delay,
// and a document that contains "sumx" (what the keystroke scenario types)
// or ends a function with a syntax error gets an error diagnostic. The
// tests of the harness use it, and make it slow on purpose to see the
// budget gate trip.
type fakeServer struct {
	delay map[string]time.Duration // by method; "diagnostics" delays the publish after a change

	mu    sync.Mutex
	texts map[protocol.DocumentURI]string
	calls map[string]int
	w     *protocol.Writer
	wg    sync.WaitGroup
	stop  sync.Once
	out   io.Closer
}

func newFake(delay map[string]time.Duration) *fakeServer {
	return &fakeServer{delay: delay, texts: map[protocol.DocumentURI]string{}, calls: map[string]int{}}
}

func (f *fakeServer) count(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[method]
}

// target is a Target whose servers are f (one at a time).
func (f *fakeServer) target(name, ext string) Target {
	return Target{Name: name, Ext: ext, Command: []string{"fake"}, Start: func(ctx context.Context, dir, cacheDir string) (*Proc, error) {
		inR, inW := io.Pipe()   // client → server
		outR, outW := io.Pipe() // server → client
		f.w = protocol.NewWriter(outW)
		f.out = outW
		go f.serve(inR)
		t := Target{Name: name, Ext: ext}
		return NewPipeProc(t, outR, inW, func() { f.stop.Do(func() { inW.Close(); f.wg.Wait(); outW.Close() }) }), nil
	}}
}

func (f *fakeServer) serve(in io.Reader) {
	r := protocol.NewReader(in)
	for {
		body, err := r.Read()
		if err != nil {
			return
		}
		m, err := protocol.DecodeMessage(body)
		if err != nil {
			return
		}
		req, ok := m.(*protocol.Request)
		if !ok {
			continue
		}
		f.mu.Lock()
		f.calls[req.Method]++
		f.mu.Unlock()
		if req.IsNotification() {
			f.notification(req)
			continue
		}
		f.wg.Add(1)
		go func() {
			defer f.wg.Done()
			time.Sleep(f.delay[req.Method])
			f.reply(req)
		}()
	}
}

func (f *fakeServer) reply(req *protocol.Request) {
	var result any
	switch req.Method {
	case "initialize":
		result = map[string]any{"capabilities": map[string]any{}}
	case "textDocument/hover":
		result = map[string]any{"contents": "x"}
	case "textDocument/completion":
		result = map[string]any{"items": []any{map[string]any{"label": "x"}}}
	case "textDocument/signatureHelp":
		result = map[string]any{"signatures": []any{map[string]any{"label": "f()"}}}
	case "textDocument/formatting":
		result = []any{map[string]any{"range": map[string]any{"start": map[string]any{"line": 0, "character": 0}, "end": map[string]any{"line": 0, "character": 0}}, "newText": " "}}
	case "textDocument/prepareRename":
		result = map[string]any{"start": map[string]any{"line": 0, "character": 0}, "end": map[string]any{"line": 0, "character": 1}}
	case "textDocument/rename":
		result = map[string]any{"changes": map[string]any{"file:///x": []any{}}}
	case "textDocument/semanticTokens/full":
		result = map[string]any{"data": []int{0, 0, 1, 0, 0}}
	case "gocpp/stats":
		result = map[string]any{"version": "fake"}
	case "shutdown":
		result = nil
	default: // definition, references, documentSymbol, workspace/symbol
		result = []any{map[string]any{"name": "x", "uri": "file:///x", "range": map[string]any{"start": map[string]any{"line": 0, "character": 0}, "end": map[string]any{"line": 0, "character": 1}}}}
	}
	data, _ := json.Marshal(result)
	body, _ := protocol.EncodeMessage(&protocol.Response{ID: req.ID, Result: data})
	f.w.Write(body)
}

func (f *fakeServer) notification(req *protocol.Request) {
	switch req.Method {
	case "textDocument/didOpen":
		var p protocol.DidOpenTextDocumentParams
		if json.Unmarshal(req.Params, &p) != nil {
			return
		}
		f.mu.Lock()
		f.texts[p.TextDocument.URI] = p.TextDocument.Text
		f.mu.Unlock()
		f.publish(p.TextDocument.URI, p.TextDocument.Version, 0)
	case "textDocument/didClose":
		var p protocol.DidCloseTextDocumentParams
		if json.Unmarshal(req.Params, &p) != nil {
			return
		}
		f.mu.Lock()
		delete(f.texts, p.TextDocument.URI)
		f.mu.Unlock()
		f.publish(p.TextDocument.URI, 0, 0)
	case "textDocument/didChange":
		var p protocol.DidChangeTextDocumentParams
		if json.Unmarshal(req.Params, &p) != nil {
			return
		}
		uri := p.TextDocument.URI
		f.mu.Lock()
		text := f.texts[uri]
		for _, c := range p.ContentChanges {
			if c.Range != nil {
				m := protocol.NewMapper(text)
				start, end, _ := m.RangeOffsets(*c.Range)
				text = text[:start] + c.Text + text[end:]
			} else {
				text = c.Text
			}
		}
		f.texts[uri] = text
		var others []protocol.DocumentURI
		for u := range f.texts {
			if u != uri {
				others = append(others, u)
			}
		}
		f.mu.Unlock()
		f.publish(uri, p.TextDocument.Version, f.delay["diagnostics"])
		for _, u := range others { // a change can change the diagnostics of the importers
			f.publish(u, 0, f.delay["diagnostics"])
		}
	}
}

var undefinedName = regexp.MustCompile(`undefinedName[0-9]+`)

// publish sends the diagnostics of uri after delay.
func (f *fakeServer) publish(uri protocol.DocumentURI, version int32, delay time.Duration) {
	f.mu.Lock()
	text := f.texts[uri]
	f.mu.Unlock()
	diags := []any{}
	if m := undefinedName.FindString(text); m != "" {
		diags = append(diags, map[string]any{"severity": 1, "message": "undefined: " + m, "range": map[string]any{"start": map[string]any{"line": 1, "character": 0}, "end": map[string]any{"line": 1, "character": 1}}})
	}
	if strings.Contains(text, "sumx") || strings.Contains(text, ", n int") && strings.Contains(string(uri), "f00") {
		diags = append(diags, map[string]any{"severity": 1, "message": "error", "range": map[string]any{"start": map[string]any{"line": 1, "character": 0}, "end": map[string]any{"line": 1, "character": 1}}})
	}
	if strings.Contains(text, "stdd.string") { // the namespace probe finds it
		diags = append(diags, map[string]any{"severity": 1, "message": "undefined: stdd", "range": map[string]any{"start": map[string]any{"line": 1, "character": 0}, "end": map[string]any{"line": 1, "character": 1}}})
	}
	// main.go (the user of the changed signature) has the error when the
	// signature file does.
	if strings.HasSuffix(string(uri), "cmd/app/main.go") || strings.HasSuffix(string(uri), "cmd/app/main.gp") {
		f.mu.Lock()
		for u, t := range f.texts {
			if strings.Contains(string(u), "f00") && strings.Contains(t, "Label(s string, n int)") {
				diags = append(diags, map[string]any{"severity": 1, "message": "not enough arguments", "range": map[string]any{"start": map[string]any{"line": 1, "character": 0}, "end": map[string]any{"line": 1, "character": 1}}})
				break
			}
		}
		f.mu.Unlock()
	}
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		time.Sleep(delay)
		data, _ := json.Marshal(map[string]any{"uri": uri, "version": version, "diagnostics": diags})
		body, _ := protocol.EncodeMessage(&protocol.Request{Method: "textDocument/publishDiagnostics", Params: data})
		f.w.Write(body)
	}()
}
