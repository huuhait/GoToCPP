package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/stdlib"
)

// The tests of the diagnostics as you type (diagnostics.go) talk to a
// server over pipes (client), with module trees on disk.

// errNoCXX is the error of the C++ compiler lookup of the tests.
var errNoCXX = errors.New("no C++ compiler in the tests")

// testConfig is the pipeline of the tests: the standard library stubs, for
// linux/amd64, without the phase gates, and no C++ compiler.
func testConfig() cache.Config {
	cfg := pipelineConfig()
	cfg.GOOS, cfg.GOARCH, cfg.Phase = "linux", "amd64", check.LastPhase
	cfg.CXX = func(context.Context) (build.CXX, error) { return build.CXX{}, errNoCXX }
	return cfg
}

// A dclient is a client of a server over a module tree.
type dclient struct {
	*client
	root string
}

// startDiag writes the files (slash-separated names) into a new folder and
// starts a server with the pipeline cfg, initialized.
func startDiag(t *testing.T, cfg cache.Config, files map[string]string, setup func(*Server)) *dclient {
	t.Helper()
	root := writeTree(t, files)
	c := &dclient{client: startServer(t, newServer(nil, cfg, debounce.RealClock{}), setup), root: root}
	c.call("initialize", diagInitParams)
	c.notify("initialized", "{}")
	return c
}

// diagInitParams are the params of initialize of a client that takes
// relatedInformation.
const diagInitParams = `{"processId":1,"rootUri":null,"capabilities":{"textDocument":{"publishDiagnostics":{"relatedInformation":true}}}}`

// writeTree writes the files (slash-separated names relative to the
// folder) into a new temporary folder and returns it.
func writeTree(t testing.TB, files map[string]string) string {
	t.Helper()
	root := filepath.Clean(t.TempDir())
	for name, src := range files {
		writeFile(t, root, name, src)
	}
	return root
}

func writeFile(t testing.TB, root, name, src string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (c *dclient) uri(name string) protocol.DocumentURI {
	return protocol.URIFromPath(filepath.Join(c.root, filepath.FromSlash(name)))
}

func (c *dclient) open(name, text string) {
	c.t.Helper()
	c.notify("textDocument/didOpen", didOpen(c.uri(name), 1, text))
}

func (c *dclient) change(name string, version int, text string) {
	c.t.Helper()
	c.notify("textDocument/didChange", fullChange(c.uri(name), version, text))
}

// next returns the next publish of the file name.
func (c *dclient) next(name string) *publish {
	c.t.Helper()
	return c.nextPublish(c.uri(name))
}

// quiet checks that no diagnostics of the file name come for a while.
func (c *dclient) quiet(name string, d time.Duration) {
	c.t.Helper()
	time.Sleep(d)
	for _, p := range c.unseen() {
		if p.URI == c.uri(name) {
			c.t.Errorf("unexpected diagnostics of %s: %s", name, p.raw)
		}
	}
}

// format returns the diagnostics of p, one per line:
// "line:char-line:char severity: message" (0-based, UTF-16), and the
// related places indented below.
func format(p *publish) string {
	var b strings.Builder
	for _, d := range p.Diagnostics {
		sev := "error"
		if d.Severity == protocol.SeverityWarning {
			sev = "warning"
		}
		r := d.Range
		fmt.Fprintf(&b, "%d:%d-%d:%d %s: %s\n", r.Start.Line, r.Start.Character, r.End.Line, r.End.Character, sev, d.Message)
		for _, ri := range d.RelatedInformation {
			r := ri.Location.Range
			fmt.Fprintf(&b, "\t%s %d:%d-%d:%d %s\n", ri.Location.URI.Base(), r.Start.Line, r.Start.Character, r.End.Line, r.End.Character, ri.Message)
		}
		if d.Source != "gocpp" {
			fmt.Fprintf(&b, "\tsource %q\n", d.Source)
		}
	}
	return b.String()
}

// want checks the version and the diagnostics of p.
func want(t *testing.T, p *publish, version int32, diags string) {
	t.Helper()
	if got := format(p); p.Version != version || got != diags {
		t.Errorf("%s version %d, diagnostics:\n%s\nwant version %d:\n%s", p.URI.Base(), p.Version, got, version, diags)
	}
}

// TestDiagnosticsAsYouType: an error is published with its exact range in
// UTF-16 (the emoji before it takes two units) and the document version;
// the fix publishes an empty list; closing the file clears it.
func TestDiagnosticsAsYouType(t *testing.T) {
	checkGoroutines(t)
	src := "package main\n\nfunc main() {\n\t_ = \"😀\" + y\n}\n"
	c := startDiag(t, testConfig(), map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src}, nil)
	c.open("main.gp", src)
	want(t, c.next("main.gp"), 1, "3:12-3:13 error: undefined: y\n")

	c.change("main.gp", 2, strings.Replace(src, "+ y", "+ \"y\"", 1))
	want(t, c.next("main.gp"), 2, "")

	// Two errors, one after the other on a line with a \r\n end.
	c.change("main.gp", 3, "package main\r\n\r\nfunc main() {\r\n\tvar x int = \"s\"; y\r\n}\r\n")
	want(t, c.next("main.gp"), 3, "3:5-3:6 error: declared and not used: x\n"+
		"3:13-3:16 error: cannot use \"s\" (untyped string constant) as int value in variable declaration\n"+
		"3:18-3:19 error: undefined: y\n")

	c.notify("textDocument/didClose", docParams(c.uri("main.gp")))
	want(t, c.next("main.gp"), 0, "")
	c.quiet("main.gp", 100*time.Millisecond)
	c.call("shutdown", "")
	c.notify("exit", "")
	if code := c.wait(); code != 0 {
		t.Errorf("exit code %d", code)
	}
}

// TestDiagnosticsKinds: syntax errors, warnings, phase gates and the
// related places of go/types (relatedInformation), each with its range.
func TestDiagnosticsKinds(t *testing.T) {
	cfg := testConfig()
	cfg.Phase = 2 // the gates of Phase 03 and later are closed
	files := map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp":   "// gocpp:link m\npackage main\n\nfunc main() {\n\tf := func() {}\n\tf()\n}\n",
		"x/x.gp":    "package x\n\nfunc F() {\n\tx := 1\n\tvar x int\n\t_ = x\n}\n",
		"y/y.gp":    "package y\n\nfunc F() {\n\tif {\n}\n",
	}
	c := startDiag(t, cfg, files, nil)
	c.open("main.gp", files["main.gp"])
	want(t, c.next("main.gp"), 1,
		"0:0-0:15 warning: \"// gocpp:link\" is an ordinary comment, not a directive: write //gocpp:link, with no space after //\n"+
			"4:1-4:2 error: function values are not supported yet (Phase 03)\n"+
			"4:6-4:15 error: closures are not supported yet (Phase 03)\n")
	c.open("x/x.gp", files["x/x.gp"])
	want(t, c.next("x/x.gp"), 1, "4:5-4:6 error: x redeclared in this block\n\tx.gp 3:1-3:2 other declaration of x\n")
	c.open("y/y.gp", files["y/y.gp"])
	// Every syntax error (gocpp check shows the first one of a line).
	want(t, c.next("y/y.gp"), 1, "3:4-3:5 error: missing condition in if statement\n"+
		"5:0-5:0 error: expected ';', found 'EOF'\n5:0-5:0 error: expected '}', found 'EOF'\n")
}

// TestDiagnosticsImporters: an edit that changes the API of a package
// updates the diagnostics of an open package that imports it; an edit in a
// function body checks only the edited package and publishes nothing for
// the importer.
func TestDiagnosticsImporters(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"main.gp":    "package main\n\nimport \"example.com/m/lib\"\n\nfunc main() {\n\tvar s string = lib.F()\n\t_ = s\n}\n",
		"lib/lib.gp": "package lib\n\nfunc F() int { return 1 }\n",
	}
	c := startDiag(t, testConfig(), files, nil)
	c.open("main.gp", files["main.gp"])
	want(t, c.next("main.gp"), 1, "5:16-5:23 error: cannot use lib.F() (value of type int) as string value in variable declaration\n")
	c.open("lib/lib.gp", files["lib/lib.gp"])
	want(t, c.next("lib/lib.gp"), 1, "")

	c.change("lib/lib.gp", 2, "package lib\n\nfunc F() string { return \"a\" }\n")
	want(t, c.next("lib/lib.gp"), 2, "")
	want(t, c.next("main.gp"), 1, "")

	_, _, before := c.s.pipeline.Stats()
	c.change("lib/lib.gp", 3, "package lib\n\nfunc F() string { return \"b\" }\n")
	want(t, c.next("lib/lib.gp"), 3, "")
	c.quiet("main.gp", 200*time.Millisecond)
	if _, _, after := c.s.pipeline.Stats(); after != before+1 {
		t.Errorf("%d checks after a body edit, want 1 (lib only)", after-before)
	}

	// A body edit with an error in lib: lib's own diagnostics, nothing
	// for main.
	c.change("lib/lib.gp", 4, "package lib\n\nfunc F() string { return 1 }\n")
	want(t, c.next("lib/lib.gp"), 4, "2:25-2:26 error: cannot use 1 (untyped int constant) as string value in return statement\n")
	c.quiet("main.gp", 100*time.Millisecond)
}

// TestDiagnosticsWatchedFiles: the server registers
// workspace/didChangeWatchedFiles when the client can, and a file changed
// on disk, or a new folder, changes the diagnostics of the open importer.
func TestDiagnosticsWatchedFiles(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"main.gp":    "package main\n\nimport \"example.com/m/lib\"\n\nfunc main() {\n\tvar s string = lib.F()\n\t_ = s\n}\n",
		"lib/lib.gp": "package lib\n\nfunc F() int { return 1 }\n",
	}
	root := writeTree(t, files)
	c := &dclient{client: startServer(t, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
	c.call("initialize", `{"processId":1,"rootUri":null,"capabilities":{"workspace":{"didChangeWatchedFiles":{"dynamicRegistration":true}}}}`)
	c.notify("initialized", "{}")
	m := c.read()
	req, ok := m.(*protocol.Request)
	if !ok || req.IsNotification() || req.Method != "client/registerCapability" {
		t.Fatalf("got %+v, want client/registerCapability", m)
	}
	if got, want := string(req.Params), `{"registrations":[{"id":"gocpp-watched-files","method":"workspace/didChangeWatchedFiles","registerOptions":{"watchers":[{"globPattern":"**/*.gp"},{"globPattern":"**/gocpp.mod"},{"globPattern":"**/*.{h,hh,hpp,hxx,inc}"}]}}]}`; got != want {
		t.Errorf("registration:\n%s\nwant\n%s", got, want)
	}
	id, _ := json.Marshal(req.ID)
	c.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":null}`, id))

	c.open("main.gp", files["main.gp"])
	want(t, c.next("main.gp"), 1, "5:16-5:23 error: cannot use lib.F() (value of type int) as string value in variable declaration\n")

	watched := func(changes ...string) {
		var events []string
		for _, ch := range changes {
			name, kind, _ := strings.Cut(ch, " ")
			events = append(events, fmt.Sprintf(`{"uri":%q,"type":%s}`, c.uri(name), kind))
		}
		c.notify("workspace/didChangeWatchedFiles", `{"changes":[`+strings.Join(events, ",")+`]}`)
	}
	writeFile(t, root, "lib/lib.gp", "package lib\n\nfunc F() string { return \"a\" }\n")
	watched("lib/lib.gp 2")
	want(t, c.next("main.gp"), 1, "")
	// lib is not open: only main ran (lib was checked as its import).
	c.s.diags.mu.Lock()
	if st := c.s.diags.pkgs[filepath.Join(root, "lib")]; st != nil {
		t.Errorf("a run for the folder lib, which has no open file")
	}
	c.s.diags.mu.Unlock()

	// A new folder: the import of a package that isn't there yet.
	src := "package main\n\nimport \"example.com/m/lib\"\nimport \"example.com/m/util\"\n\nfunc main() { _ = lib.F() + util.G() }\n"
	c.change("main.gp", 2, src)
	p := c.next("main.gp")
	if !strings.Contains(format(p), "util") {
		t.Fatalf("diagnostics of the missing import:\n%s", format(p))
	}
	writeFile(t, root, "util/util.gp", "package util\n\nfunc G() string { return \"g\" }\n")
	watched("util/util.gp 1")
	want(t, c.next("main.gp"), 2, "")

	// A deleted folder (the event names only the folder).
	if err := os.RemoveAll(filepath.Join(root, "util")); err != nil {
		t.Fatal(err)
	}
	watched("util 3")
	if p := c.next("main.gp"); !strings.Contains(format(p), "util") {
		t.Errorf("after the folder was deleted:\n%s", format(p))
	}
	// The last file of a package deleted (a file event): the package is
	// gone, and its importer sees it.
	if err := os.Remove(filepath.Join(root, "lib", "lib.gp")); err != nil {
		t.Fatal(err)
	}
	watched("lib/lib.gp 3")
	if p := c.next("main.gp"); !strings.Contains(format(p), "lib") {
		t.Errorf("after lib.gp was deleted:\n%s", format(p))
	}
	if strings.Contains(c.logs.String(), "answered with an error") || strings.Contains(c.logs.String(), "dropped a response") {
		t.Errorf("logs:\n%s", c.logs.String())
	}
}

// TestDiagnosticsStale: a run whose result is stale publishes nothing. The
// run for version 2 is held until version 3 has come (which cancels it);
// then only version 3 is published. And a run that finds a newer snapshot
// after its check (a body edit in another package, which schedules no run
// for this one) checks again on the newest snapshot before it publishes.
func TestDiagnosticsStale(t *testing.T) {
	checkGoroutines(t)
	type call struct {
		snap uint64
		try  int
	}
	var mu sync.Mutex
	hold := false
	var calls []call
	held, release := make(chan struct{}, 10), make(chan struct{})
	src := "package main\n\nfunc main() { _ = x1 }\n"
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src, "other/o.gp": "package other\n\nfunc f() { _ = 1 }\n"}
	var mainDir string
	c := startDiag(t, testConfig(), files, func(s *Server) {
		s.diags.checked = func(_ context.Context, dir string, snapID uint64, try int) {
			if dir != mainDir {
				return
			}
			mu.Lock()
			calls = append(calls, call{snapID, try})
			h := hold
			hold = false
			mu.Unlock()
			if h {
				held <- struct{}{}
				<-release
			}
		}
	})
	mainDir = c.root
	c.open("main.gp", src)
	want(t, c.next("main.gp"), 1, "2:18-2:20 error: undefined: x1\n")

	mu.Lock()
	hold = true
	mu.Unlock()
	c.change("main.gp", 2, strings.Replace(src, "x1", "x2", 1))
	wait(t, held)
	c.change("main.gp", 3, strings.Replace(src, "x1", "x3", 1))
	time.Sleep(50 * time.Millisecond) // the run of version 3 is due, and waits
	release <- struct{}{}
	want(t, c.next("main.gp"), 3, "2:18-2:20 error: undefined: x3\n")
	c.quiet("main.gp", 100*time.Millisecond)

	// The other package is open, and its first run (no API known yet)
	// has run main again.
	c.open("other/o.gp", files["other/o.gp"])
	want(t, c.next("other/o.gp"), 1, "")
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	hold, calls = true, nil
	mu.Unlock()
	c.change("main.gp", 4, strings.Replace(src, "x1", "x4", 1))
	wait(t, held)
	c.change("other/o.gp", 2, "package other\n\nfunc f() { _ = 2 }\n")
	want(t, c.next("other/o.gp"), 2, "")
	release <- struct{}{}
	want(t, c.next("main.gp"), 4, "2:18-2:20 error: undefined: x4\n")
	c.quiet("main.gp", 100*time.Millisecond)
	mu.Lock()
	if len(calls) != 2 || calls[0].try != 0 || calls[1].try != 1 || calls[1].snap <= calls[0].snap {
		t.Errorf("the run checked %+v, want try 0 and then try 1 on a newer snapshot", calls)
	}
	mu.Unlock()
}

// TestDiagnosticsProbeWindow: a namespace probe answer that comes after the
// check of a package and before its publish is not lost: the publish sees
// that the package is stale, and the package is checked again.
func TestDiagnosticsProbeWindow(t *testing.T) {
	cfg := testConfig()
	cfg.CXX = func(context.Context) (build.CXX, error) {
		return build.CXX{Path: "/fake/c++", Family: build.Clang}, nil
	}
	cfg.Probe = func(_ context.Context, _ build.CXX, _ string, _, _, cands []string) (interop.Result, error) {
		r := interop.Result{Kinds: make(map[string]interop.Kind)}
		for _, c := range cands {
			r.Kinds[c] = interop.NotNamespace
			if c == "std" {
				r.Kinds[c] = interop.Namespace
			}
		}
		return r, nil
	}
	var once sync.Once
	src := "package main\n\nimport \"<vector>\"\n\nfunc main() {\n\tvar w stdd.vector[int]\n\t_ = w\n}\n"
	c := startDiag(t, cfg, map[string]string{"gocpp.mod": "module example.com/m\n"}, func(s *Server) {
		s.diags.checked = func(context.Context, string, uint64, int) {
			once.Do(func() { time.Sleep(100 * time.Millisecond) }) // the answers come meanwhile
		}
	})
	c.open("main.gp", src)
	for {
		p := c.next("main.gp") // fails after 10 s
		if got := format(p); got == "5:7-5:11 error: undefined: stdd\n" {
			break
		} else if got != "" {
			t.Fatalf("diagnostics:\n%s", got)
		}
	}
}

// TestDiagnosticsLeftOut: an open file that the package leaves out gets the
// diagnostics about the package as a whole, at its package clause: build
// constraints exclude every file of the folder, or it has only test files.
func TestDiagnosticsLeftOut(t *testing.T) {
	src := "//go:build windows\n\npackage main\n\nfunc main() {}\n"
	test := "package x\n\nfunc f() {}\n"
	c := startDiag(t, testConfig(), map[string]string{"gocpp.mod": "module example.com/m\n", "w/main.gp": src, "x/x_test.gp": test}, nil)
	c.open("w/main.gp", src)
	if got := format(c.next("w/main.gp")); !strings.HasPrefix(got, "2:8-2:12 error: package example.com/m/w: build constraints exclude all .gp files in ") {
		t.Errorf("diagnostics:\n%s", got)
	}
	c.open("x/x_test.gp", test)
	if got := format(c.next("x/x_test.gp")); !strings.HasPrefix(got, "0:8-0:9 error: example.com/m/x: no non-test .gp files in ") {
		t.Errorf("diagnostics:\n%s", got)
	}
}

// TestDiagnosticsNoRelatedInformation: a client that doesn't take
// relatedInformation gets the related places as lines of the message.
func TestDiagnosticsNoRelatedInformation(t *testing.T) {
	src := "package x\n\nfunc F() {\n\tx := 1\n\tvar x int\n\t_ = x\n}\n"
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "x/x.gp": src})
	c := &dclient{client: startServer(t, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
	c.call("initialize", initParams)
	c.open("x/x.gp", src)
	path := filepath.Join(root, "x", "x.gp")
	want(t, c.next("x/x.gp"), 1, "4:5-4:6 error: x redeclared in this block\n\t"+path+":4:2: other declaration of x\n")
}

// TestDiagnosticsDebounce: with a fake clock, a burst of changes gives one
// run, on the last text, after the quiet time.
func TestDiagnosticsDebounce(t *testing.T) {
	clock := new(debounce.FakeClock)
	src := "package main\n\nfunc main() { _ = x0 }\n"
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n"})
	c := &dclient{client: startServer(t, newServer(nil, testConfig(), clock), nil), root: root}
	c.call("initialize", initParams)
	// due waits until the server has made n timers, then lets d pass.
	due := func(n int, d time.Duration) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for clock.Timers() < n {
			if time.Now().After(deadline) {
				t.Fatalf("%d timers, want %d", clock.Timers(), n)
			}
			time.Sleep(100 * time.Microsecond)
		}
		clock.Advance(d)
	}
	c.open("main.gp", src)
	due(1, 0)
	want(t, c.next("main.gp"), 1, "2:18-2:20 error: undefined: x0\n")
	for v := 2; v <= 6; v++ {
		c.change("main.gp", v, strings.Replace(src, "x0", fmt.Sprintf("x%d", v), 1))
	}
	// Each change stopped the timer before it: one is left.
	c.call("test/sync", "") // the changes are applied (the answer comes after them)
	due(1, diagnoseDelay-time.Millisecond)
	c.quiet("main.gp", 50*time.Millisecond)
	clock.Advance(time.Millisecond)
	want(t, c.next("main.gp"), 6, "2:18-2:20 error: undefined: x6\n")
	c.quiet("main.gp", 50*time.Millisecond)
}

// TestDiagnosticsBroken: a failed didChange marks the overlay broken: its
// diagnostics are cleared (they would be at the wrong places).
func TestDiagnosticsBroken(t *testing.T) {
	src := "package main\n\nfunc main() { _ = y }\n"
	c := startDiag(t, testConfig(), map[string]string{"gocpp.mod": "module example.com/m\n"}, nil)
	c.open("main.gp", src)
	want(t, c.next("main.gp"), 1, "2:18-2:19 error: undefined: y\n")
	c.notify("textDocument/didChange", fmt.Sprintf(`{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"range":{"start":{"line":2,"character":5},"end":{"line":1,"character":0}},"text":"x"}]}`, c.uri("main.gp")))
	c.readNotification("window/showMessage")
	want(t, c.next("main.gp"), 1, "")
}

// TestDiagnosticsClosedPackageFile: a file of an open package that is not
// open itself gets diagnostics without a version, and keeps them while
// another file of the package is open; closing the last one clears all.
func TestDiagnosticsClosedPackageFile(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a.gp":      "package main\n\nfunc main() { b() }\n",
		"b.gp":      "package main\n\nfunc b() { _ = z }\n",
	}
	c := startDiag(t, testConfig(), files, nil)
	c.open("a.gp", files["a.gp"])
	want(t, c.next("a.gp"), 1, "")
	want(t, c.next("b.gp"), 0, "2:15-2:16 error: undefined: z\n")
	c.open("b.gp", files["b.gp"])
	want(t, c.next("b.gp"), 1, "2:15-2:16 error: undefined: z\n")
	c.notify("textDocument/didClose", docParams(c.uri("b.gp")))
	want(t, c.next("b.gp"), 0, "2:15-2:16 error: undefined: z\n")
	c.notify("textDocument/didClose", docParams(c.uri("a.gp")))
	want(t, c.next("b.gp"), 0, "")
	want(t, c.next("a.gp"), 0, "")
}

// TestDiagnosticsNothingAfterExit: a run that ends after exit publishes
// nothing.
func TestDiagnosticsNothingAfterExit(t *testing.T) {
	held, release := make(chan struct{}, 1), make(chan struct{})
	returned := make(chan struct{})
	c := startDiag(t, testConfig(), map[string]string{"gocpp.mod": "module example.com/m\n"}, func(s *Server) {
		s.diags.checked = func(context.Context, string, uint64, int) {
			held <- struct{}{}
			<-release
		}
	})
	go func() {
		<-release
		close(returned)
	}()
	c.open("main.gp", "package main\n\nfunc main() { _ = y }\n")
	wait(t, held)
	c.notify("exit", "")
	if code := c.wait(); code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
	close(release)
	<-returned
	time.Sleep(50 * time.Millisecond)
	if ps := c.unseen(); len(ps) > 0 {
		t.Errorf("published after exit: %s", ps[0].raw)
	}
}

// TestDiagnosticsProbe: a header import does not hold back the first
// diagnostics: they come without the namespace probe (std and the typo
// stdd are opaque), and the probe, in the background, then brings the
// "undefined: stdd" of gocpp check. It runs the real C++ compiler.
func TestDiagnosticsProbe(t *testing.T) {
	if _, err := build.FindCXX(context.Background()); err != nil {
		if os.Getenv("GOCPP_REQUIRE_CXX") != "" {
			t.Fatalf("GOCPP_REQUIRE_CXX is set, but no C++ compiler: %v", err)
		}
		t.Skipf("no C++ compiler: %v", err)
	}
	firstSeen := make(chan struct{})
	probes := make(chan time.Duration, 10)
	cfg := testConfig()
	cfg.CXX = nil // the real one
	cfg.Probe = func(ctx context.Context, cxx build.CXX, dir string, flags, includes, cands []string) (interop.Result, error) {
		<-firstSeen
		start := time.Now()
		r, err := interop.ProbeNamespaces(ctx, cxx, dir, flags, includes, cands)
		probes <- time.Since(start)
		return r, err
	}
	src := "package main\n\nimport \"<vector>\"\n\nfunc main() {\n\tvar v std.vector[int]\n\tvar w stdd.vector[int]\n\t_, _ = v, w\n}\n"
	c := startDiag(t, cfg, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src}, nil)
	start := time.Now()
	c.open("main.gp", src)
	want(t, c.next("main.gp"), 1, "")
	first := time.Since(start)
	close(firstSeen)
	want(t, c.next("main.gp"), 1, "6:7-6:11 error: undefined: stdd\n")
	t.Logf("first diagnostics after %v, without the probe; then the probe took %v", first, <-probes)
}

// TestDiagnosticsSameAsCheck: for each program of tests/errors, the server
// publishes the diagnostics of gocpp check (load.Load and check.Check of
// the whole program), at the same places: the start of each range,
// converted back to a byte offset, is the offset of the diagnostic.
func TestDiagnosticsSameAsCheck(t *testing.T) {
	files, _ := filepath.Glob("../../../tests/errors/*.gp")
	if len(files) < 10 {
		t.Fatalf("only %d programs", len(files))
	}
	cfg := testConfig()
	c := startDiag(t, cfg, map[string]string{}, nil)
	compared := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(filepath.Base(f), ".gp") + "/" + filepath.Base(f)
		writeFile(t, c.root, name, string(src))
		path := filepath.Join(c.root, filepath.FromSlash(name))
		prog := load.Load(load.Config{Dir: filepath.Dir(path), Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: cfg.GOOS, GOARCH: cfg.GOARCH, AllErrors: true, Phase10Syntax: cfg.Phase >= 10}, ".")
		r := check.Check(prog, check.Config{Phase: cfg.Phase, GOOS: cfg.GOOS, GOARCH: cfg.GOARCH, CXX: cfg.CXX})
		lists := []*diag.List{prog.Errors}
		if cp := r.Package(prog.Roots[0]); cp != nil {
			lists = append(lists, cp.Errors)
		}
		var wantLines []string
		for _, l := range lists {
			for _, d := range l.Diagnostics() {
				if d.Pos.Filename != path {
					continue
				}
				msg := d.Msg
				related := 0
				for _, r := range d.Related {
					if r.Pos.Filename == "" {
						msg += "\n\t" + r.Msg
					} else if _, err := os.Stat(r.Pos.Filename); err != nil {
						msg += fmt.Sprintf("\n\t%s: %s", r.Pos, r.Msg)
					} else {
						related++
					}
				}
				wantLines = append(wantLines, fmt.Sprintf("%d %v %s (%d related)", d.Pos.Offset, d.Severity, msg, related))
			}
		}
		slices.Sort(wantLines)
		wantText := strings.Join(wantLines, "\n")
		compared += len(wantLines)

		c.open(name, string(src))
		m := protocol.NewMapper(string(src))
		deadline := time.Now().Add(10 * time.Second)
		for {
			p := c.next(name)
			var got []string
			for _, d := range p.Diagnostics {
				sev := diag.Error
				if d.Severity == protocol.SeverityWarning {
					sev = diag.Warning
				}
				got = append(got, fmt.Sprintf("%d %v %s (%d related)", m.Offset(d.Range.Start), sev, d.Message, len(d.RelatedInformation)))
			}
			slices.Sort(got)
			gotText := strings.Join(got, "\n")
			if gotText == wantText {
				break
			}
			if time.Now().After(deadline) {
				t.Errorf("%s: published\n%s\nwant (gocpp check)\n%s", filepath.Base(f), gotText, wantText)
				break
			}
			// A provisional publish (the probe's compiler lookup runs in
			// the background): the next one has it.
		}
		c.notify("textDocument/didClose", docParams(c.uri(name)))
		c.next(name)
	}
	if compared < 40 {
		t.Errorf("only %d diagnostics compared", compared)
	}
	t.Logf("%d diagnostics of %d programs", compared, len(files))
}
