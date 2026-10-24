package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The errors of the automatic argument adaptations (ADR-130, ADR-134) in the editor. They come from
// a background analysis, so the package is diagnosed without them first, and they arrive in a later
// publish (cache/adaptindex.go); the tests wait for that publish, with the real clang++.

const adaptOverflowMsg = "cannot use 200 (untyped int constant) as C.char value in argument to"

func adaptProgram(arg string) string {
	return "package main\n\nimport \"./api.hpp\"\n\nfunc main() {\n\tapi.take_char(" + arg + ")\n}\n"
}

// adaptClient starts a server over a package that calls a C++ function with the argument arg.
func adaptClient(t *testing.T, arg string) *dclient {
	t.Helper()
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	defer probe.SetRacyWindow(0)()
	if _, err := probe.FindClang(context.Background()); err != nil {
		t.Skip(err)
	}
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/adapt\n",
		"p/main.gp": adaptProgram(arg),
		"p/api.hpp": "inline void take_char(char c) {}\ninline void take_short(short c) {}\n",
	})
	cfg := testConfig()
	cfg.CXX = func(ctx context.Context) (build.CXX, error) { return build.FindCXXFor(ctx, cfg.GOOS, cfg.GOARCH) }
	s := newServer(nil, cfg, debounce.RealClock{})
	t.Cleanup(s.pipeline.Close)
	c := &dclient{client: startServer(t, s, nil), root: root}
	c.call("initialize", diagInitParams)
	c.notify("initialized", "{}")
	return c
}

// publishWith waits for a publish of the file name, after the publishes seen so far, that the
// predicate accepts.
func (c *dclient) publishWith(name string, within time.Duration, ok func(*publish) bool) *publish {
	c.t.Helper()
	uri := c.uri(name)
	deadline := time.Now().Add(within)
	next := 0
	for {
		c.pubMu.Lock()
		for ; next < len(c.publishes); next++ {
			if p := c.publishes[next]; p.URI == uri && ok(p) {
				c.pubMu.Unlock()
				return p
			}
		}
		c.pubMu.Unlock()
		if time.Now().After(deadline) {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
}

func hasMessage(p *publish, text string) bool {
	for _, d := range p.Diagnostics {
		if strings.Contains(d.Message, text) {
			return true
		}
	}
	return false
}

// The first publish of the package does not wait for clang, and the refusal of the build is in a
// later one, at the argument, as an error.
func TestAdaptRefusalReachesTheEditor(t *testing.T) {
	c := adaptClient(t, "200")
	begin := time.Now()
	c.open("p/main.gp", adaptProgram("200"))
	first := c.next("p/main.gp")
	if hasMessage(first, adaptOverflowMsg) {
		t.Log("the first publish already has the refusal (a warm machine)")
	}
	p := c.publishWith("p/main.gp", hangLimit, func(p *publish) bool { return hasMessage(p, adaptOverflowMsg) })
	if p == nil && hasMessage(first, adaptOverflowMsg) {
		p = first
	}
	if p == nil {
		t.Fatalf("no publish with the refusal before the hang limit; the log:\n%s", c.logs.String())
	}
	t.Logf("the refusal arrived %v after the document was opened", time.Since(begin))
	d := p.Diagnostics[0]
	if d.Severity != protocol.SeverityError || d.Source != "gocpp" {
		t.Errorf("the refusal is %+v, want an error of gocpp", d)
	}
	// The argument is at line 6 (0-based 5), after "\tapi.take_char(".
	if got := d.Range.Start; got.Line != 5 || got.Character != 15 {
		t.Errorf("the refusal starts at %+v, want 5:15 (the argument 200)", got)
	}
}

// A fix clears it, and it comes back when the error does.
func TestAdaptRefusalIsClearedByTheFix(t *testing.T) {
	c := adaptClient(t, "200")
	c.open("p/main.gp", adaptProgram("200"))
	if c.publishWith("p/main.gp", hangLimit, func(p *publish) bool { return hasMessage(p, adaptOverflowMsg) }) == nil {
		t.Fatalf("no refusal before the hang limit; the log:\n%s", c.logs.String())
	}
	c.change("p/main.gp", 2, adaptProgram("100"))
	fixed := c.publishWith("p/main.gp", hangLimit, func(p *publish) bool {
		return p.Version == 2 && len(p.Diagnostics) == 0
	})
	if fixed == nil {
		t.Fatalf("the refusal is not cleared by the fix; the log:\n%s", c.logs.String())
	}
	// The analysis of the fixed package ends: nothing comes back.
	if c.publishWith("p/main.gp", 10*time.Second, func(p *publish) bool { return p.Version == 2 && len(p.Diagnostics) > 0 }) != nil {
		t.Errorf("a diagnostic is published for the fixed program")
	}
	c.change("p/main.gp", 3, adaptProgram("300"))
	if c.publishWith("p/main.gp", hangLimit, func(p *publish) bool {
		return p.Version == 3 && hasMessage(p, "cannot use 300 (untyped int constant) as C.char value in argument to")
	}) == nil {
		t.Fatalf("the refusal does not come back; the log:\n%s", c.logs.String())
	}
}

// A constant that fits has no diagnostic, and nothing is published for it.
func TestAdaptNoDiagnosticForAConstantThatFits(t *testing.T) {
	c := adaptClient(t, "100")
	c.open("p/main.gp", adaptProgram("100"))
	first := c.next("p/main.gp")
	if len(first.Diagnostics) != 0 {
		t.Fatalf("diagnostics for a program without errors: %s", first.raw)
	}
	if c.publishWith("p/main.gp", 10*time.Second, func(p *publish) bool { return len(p.Diagnostics) > 0 }) != nil {
		t.Errorf("a diagnostic is published for a constant that fits")
	}
}
