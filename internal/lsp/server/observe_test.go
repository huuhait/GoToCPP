package server

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/lsp/trace"
)

// The tests of step 11.12's observability: the slow-request log, the spans
// (gocpp lsp -trace) and gocpp/stats.

var observeFiles = map[string]string{
	"gocpp.mod": "module example.com/obs\n",
	"obs/obs.gp": `package obs

import "fmt"

// Point is a point.
type Point struct{ X, Y int }

func (p Point) String() string { return fmt.Sprint(p.X, p.Y) }

func Use() string { return Point{1, 2}.String() }
`,
}

// startObserve starts a server with the options opts over the files.
func startObserve(t *testing.T, opts Options) *dclient {
	t.Helper()
	root := writeTree(t, observeFiles)
	c := &dclient{client: startServer(t, newServerOpts(nil, testConfig(), debounce.RealClock{}, opts), nil), root: root}
	c.call("initialize", markdownInit)
	c.notify("initialized", "{}")
	return c
}

func hoverAtPoint(c *dclient) {
	c.t.Helper()
	src := observeFiles["obs/obs.gp"]
	if resp := c.call("textDocument/hover", c.posParams("obs/obs.gp", src, "type Po|int struct")); resp.Error != nil {
		c.t.Fatal(resp.Error)
	}
}

// With a threshold of one nanosecond every piece of work is slow: the log
// has the method, the document, the snapshot and the phases of the check.
func TestSlowLog(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the clock of a Windows process ticks every half millisecond or more: a fast request measures 0 ns, and the test counts on a duration")
	}
	c := startObserve(t, Options{Slow: time.Nanosecond})
	name := "obs/obs.gp"
	c.open(name, observeFiles[name])
	c.next(name)
	hoverAtPoint(c)
	logs := c.logs.String()
	for _, want := range []string{
		"slow: textDocument/hover took ",
		"slow: textDocument/didOpen took ",
		"slow: diagnostics took ",
		"uri=" + string(c.uri(name)),
		"snapshot=",
		"overlays=1",
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("the log has no %q:\n%s", want, logs)
		}
	}
	// The diagnostics run checked the package: its phases are in the line.
	var line string
	for _, l := range strings.Split(logs, "\n") {
		if strings.HasPrefix(l, "slow: diagnostics took ") {
			line = l
			if strings.Contains(l, "typecheck") {
				break
			}
		}
	}
	for _, phase := range []string{"phases:", " package ", " check ", " typecheck ", " load "} {
		if !strings.Contains(line, phase) {
			t.Errorf("the diagnostics line has no %q: %s", phase, line)
		}
	}
}

func TestSlowLogThreshold(t *testing.T) {
	// Nothing in the tests takes an hour; a negative threshold is off.
	for _, slow := range []time.Duration{time.Hour, -1} {
		c := startObserve(t, Options{Slow: slow})
		name := "obs/obs.gp"
		c.open(name, observeFiles[name])
		c.next(name)
		hoverAtPoint(c)
		if logs := c.logs.String(); strings.Contains(logs, "slow:") {
			t.Errorf("threshold %v: the log has slow work:\n%s", slow, logs)
		}
	}
}

func TestSlowMessage(t *testing.T) {
	sum := trace.Summary{Name: "textDocument/completion", Total: 612345 * time.Microsecond, Phases: []trace.Phase{
		{Name: "check", Total: 600 * time.Millisecond, Count: 1},
		{Name: "parse", Total: 3 * time.Millisecond, Count: 4},
	}}
	got := slowMessage(sum, 500*time.Millisecond, "uri=file:///a.gp snapshot=7")
	want := "slow: textDocument/completion took 612.345ms (over 500ms) uri=file:///a.gp snapshot=7; phases: check 600ms, parse 3ms (x4)"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestTraceSpans(t *testing.T) {
	rec := trace.NewRecorder(0)
	c := startObserve(t, Options{Trace: rec})
	name := "obs/obs.gp"
	c.open(name, observeFiles[name])
	c.next(name)
	hoverAtPoint(c)
	var buf strings.Builder
	if err := rec.WriteChrome(&buf); err != nil {
		t.Fatal(err)
	}
	var out struct {
		TraceEvents []struct {
			Name string
			Ph   string
			Tid  int64
		}
	}
	if err := json.Unmarshal([]byte(buf.String()), &out); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range out.TraceEvents {
		if e.Ph == "X" {
			seen[e.Name] = true
		}
	}
	for _, want := range []string{"textDocument/hover", "textDocument/didOpen", "diagnostics", "package", "check", "load", "typecheck"} {
		if !seen[want] {
			t.Errorf("no span %q; have %v", want, seen)
		}
	}
}

func TestStats(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the clock of a Windows process ticks every half millisecond or more: a fast request measures 0 ns, and the test counts on a duration")
	}
	c := startObserve(t, Options{})
	name := "obs/obs.gp"
	c.open(name, observeFiles[name])
	c.next(name)
	for range 3 {
		hoverAtPoint(c)
	}
	// The span of a run of diagnostics ends after its publish: the count
	// of the run that the next() above waited for is there a moment later.
	for range 400 {
		var st Stats
		if resp := c.call(GocppStats, ""); resp.Error == nil && json.Unmarshal(resp.Result, &st) == nil && st.Requests["diagnostics"].Count >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	for _, call := range []func() *protocol.Response{
		func() *protocol.Response { return c.call(GocppStats, "") },
		func() *protocol.Response {
			return c.call("workspace/executeCommand", `{"command":"`+StatsCommand+`"}`)
		},
	} {
		resp := call()
		if resp.Error != nil {
			t.Fatal(resp.Error)
		}
		var st Stats
		if err := json.Unmarshal(resp.Result, &st); err != nil {
			t.Fatalf("%v: %s", err, resp.Result)
		}
		if st.Version == "" || st.Snapshots.Live < 1 || st.Snapshots.Open != 1 || st.Snapshots.Current == 0 {
			t.Errorf("snapshots: %+v", st.Snapshots)
		}
		if st.Pipeline.Checks < 1 || st.Pipeline.Parses < 1 {
			t.Errorf("pipeline: %+v", st.Pipeline)
		}
		check := st.Pipeline.Caches["check"]
		if check.Entries < 1 || check.Bytes <= 0 || check.Misses < 1 {
			t.Errorf("check cache: %+v", check)
		}
		if hr := check.HitRate; hr < 0 || hr > 1 {
			t.Errorf("hit rate %v", hr)
		}
		hover := st.Requests["textDocument/hover"]
		if hover.Count != 3 || hover.MaxMs < hover.P50Ms || hover.P95Ms < hover.P50Ms || hover.TotalMs <= 0 {
			t.Errorf("hover: %+v", hover)
		}
		if st.Requests["diagnostics"].Count < 1 || st.Requests["textDocument/didOpen"].Count != 1 {
			t.Errorf("requests: %+v", st.Requests)
		}
		if st.Memory.HeapAlloc == 0 || st.Memory.Goroutines < 1 || st.Memory.Sys == 0 {
			t.Errorf("memory: %+v", st.Memory)
		}
	}
	resp := c.call("workspace/executeCommand", `{"command":"nope"}`)
	wantError(t, resp, protocol.InvalidParams, "unknown command")
}

func TestStatsCommandAdvertised(t *testing.T) {
	root := writeTree(t, observeFiles)
	c := &dclient{client: startServer(t, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
	resp := c.call("initialize", markdownInit)
	var res protocol.InitializeResult
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		t.Fatal(err)
	}
	p := res.Capabilities.ExecuteCommandProvider
	if p == nil || len(p.Commands) != 2 || p.Commands[0] != StatsCommand || p.Commands[1] != ExpandCommand {
		t.Errorf("executeCommandProvider: %+v", p)
	}
}
