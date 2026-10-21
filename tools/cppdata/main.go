// Command cppdata measures when the C/C++ data of the language server reaches the editor
// (step 12.13d): from the didOpen of a .gp file that imports header packages to the first
// answer that carries each kind of data (completion after a header package or a namespace,
// member completion, the native type of a variable, hover and definition of a named API
// symbol). It drives the real "gocpp lsp" binary over stdio, with the real clang of the
// environment, and polls each kind from its own goroutine, so the figures are the moment the
// data was available to a request, not a count of requests.
//
//	nix develop -c go build -o bin/gocpp ./cmd/gocpp
//	nix develop -c go run ./tools/cppdata -gocpp bin/gocpp -runs 5
//
// A cold run starts with an empty cache folder (the probe disk cache, the precompiled
// headers); a warm run starts the server again with the folder that the cold run filled. Run it
// on a quiet machine, one run at a time, under the shared perf lock and `taskset`.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/bench"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// A probe is one kind of C/C++ data: a request at a cursor and the test of its answer.
type probe struct {
	kind   string
	method string
	cursor string // text of the source; the cursor is at the "|"
	ok     func(text string) bool
}

type scenario struct {
	name   string
	files  map[string]string
	main   string
	probes []probe
}

const raylibSource = `//gocpp:pkg-config raylib
package main

import rl "<raylib.h>"

func main() {
	rl.InitWindow(800, 450, "demo")
	var pos rl.Vector2
	pos.x = 1
	rl.CloseWindow()
}
`

const sqliteSource = `//gocpp:pkg-config sqlite3
package main

import "<sqlite3.h>"

func main() {
	var db *sqlite3.sqlite3
	rc := sqlite3.sqlite3_open(":memory:", &db)
	_ = rc == sqlite3.SQLITE_OK
	sqlite3.sqlite3_close(db)
}
`

const cppStdSource = `package main

import (
	"<algorithm>"
	"<map>"
	"<string>"
	"<vector>"
)

func main() {
	var name std.string
	size := name.size()
	nums := std.vector[int]{5, 3, 8, 1}
	std.sort(nums.begin(), nums.end())
	_ = size
}
`

const localSource = `package main

import (
	"fmt"

	"./shape.hpp"
	"<algorithm>"
	"<map>"
	"<string>"
	"<vector>"
)

func main() {
	counts := std.map[std.string, int]{}
	counts[std.string("a")]++
	nums := std.vector[int]{}
	nums.push_back(1)
	std.sort(nums.begin(), nums.end())
	box := shape.Box{}
	box.w = 2
	fmt.Println(box.area(), shape.perimeter(box), counts.size())
}
`

const localHeader = `#pragma once

struct Box {
  int w = 0;
  int h = 0;
  int area() const { return w * h; }
};

inline int perimeter(const Box& b) { return 2 * (b.w + b.h); }
`

func has(sub string) func(string) bool {
	return func(text string) bool { return strings.Contains(text, sub) }
}

func scenarios() []scenario {
	return []scenario{
		{name: "raylib", main: "main.gp",
			files: map[string]string{"main.gp": raylibSource}, probes: []probe{
				{"header-package completion", "completion", "\trl.|InitWindow(", has("InitWindow")},
				{"member completion", "completion", "\tpos.|x = 1", has("ready")},
				{"variable type hover", "hover", "\tvar po|s rl.Vector2", has("C/C++ native type")},
				{"API hover", "hover", "\trl.Init|Window(", has("declared at")},
				{"API definition", "definition", "\trl.Init|Window(", has("raylib.h")},
				{"member hover", "hover", "\tpos.|x = 1", has("declared at")},
			}},
		{name: "sqlite3", main: "main.gp",
			files: map[string]string{"main.gp": sqliteSource}, probes: []probe{
				{"header-package completion", "completion", "sqlite3.|sqlite3_open(",
					has("sqlite3_open")},
				{"API hover", "hover", "sqlite3.sqlite3_o|pen(", has("declared at")},
				{"API definition", "definition", "sqlite3.sqlite3_o|pen(", has("sqlite3.h")},
			}},
		{name: "local-header", main: "main.gp",
			files: map[string]string{"main.gp": localSource, "shape.hpp": localHeader},
			probes: []probe{
				{"header-package completion", "completion", "\tbox := shape.|Box{}", has("Box")},
				{"namespace completion", "completion", "\tnums := std.|vector", has("\"vector\"")},
				{"member completion", "completion", "\tbox.|w = 2", has("ready")},
				{"variable type hover", "hover", "\tbo|x := shape.Box{}", has("C/C++ native type")},
				{"API hover", "hover", "shape.perim|eter(box)", has("declared at")},
				{"API definition", "definition", "shape.perim|eter(box)", has("shape.hpp")},
				{"member hover", "hover", "box.ar|ea()", has("declared at")},
			}},
		{name: "cpp-std", main: "main.gp",
			files: map[string]string{"main.gp": cppStdSource}, probes: []probe{
				{"namespace completion", "completion", "\tnums := std.|vector", has("\"vector\"")},
				{"member completion", "completion", "size := name.|size()", has("size")},
				{"variable type hover", "hover", "\tvar na|me std.string",
					has("C/C++ native type")},
				{"API hover", "hover", "\tstd.so|rt(", has("(overloads)")},
				{"API definition", "definition", "\tstd.so|rt(", has("algorithm")},
				{"member hover", "hover", "size := name.si|ze()", has("declared at")},
			}},
	}
}

type sample struct {
	Scenario string  `json:"scenario"`
	Mode     string  `json:"mode"`
	Run      int     `json:"run"`
	Kind     string  `json:"kind"`
	Millis   float64 `json:"ms"` // from didOpen; -1: never within the timeout
}

func main() {
	os.Exit(run())
}

func run() int {
	compare := flag.String("compare", "",
		"compare lspbench JSON reports: name=file[,file];name=file...; measure nothing")
	gocpp := flag.String("gocpp", "", "the gocpp binary")
	runs := flag.Int("runs", 5, "runs per scenario and mode")
	only := flag.String("only", "", "scenario names, comma-separated (default: all)")
	jsonOut := flag.String("json", "", "write the samples as JSON to `file`")
	timeout := flag.Duration("timeout", 30*time.Second, "the longest wait for a kind of data")
	poll := flag.Duration("poll", 5*time.Millisecond, "the pause between two polls of one kind")
	modes := flag.String("modes", "cold,warm", "the start modes to measure: cold, warm or both")
	typing := flag.Duration("typing", 0,
		"type a comment at the end of the file, one key every `d`, while the data is awaited "+
			"(0: no typing)")
	edit := flag.Bool("edit", false,
		"after the data is there, edit the document and measure when it is there again")
	env := flag.String("env", "GOCPP_INTEROP=typed", "environment of gocpp lsp, comma-separated")
	workdir := flag.String("workdir", "", "where the projects and caches go (default: temporary)")
	flag.Parse()
	if *compare != "" {
		if err := compareReports(os.Stdout, *compare); err != nil {
			fmt.Fprintln(os.Stderr, "cppdata:", err)
			return 1
		}
		return 0
	}
	if *gocpp == "" {
		fmt.Fprintln(os.Stderr, "cppdata: pass -gocpp")
		return 2
	}
	abs, err := filepath.Abs(*gocpp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cppdata:", err)
		return 2
	}
	dir := *workdir
	if dir == "" {
		dir, err = os.MkdirTemp("", "cppdata-")
		if err != nil {
			fmt.Fprintln(os.Stderr, "cppdata:", err)
			return 2
		}
		defer os.RemoveAll(dir)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var samples []sample
	for _, sc := range scenarios() {
		if *only != "" && !slices.Contains(strings.Split(*only, ","), sc.name) {
			continue
		}
		for r := 0; r < *runs && ctx.Err() == nil; r++ {
			caches := filepath.Join(dir, fmt.Sprintf("%s-cache-%d", sc.name, r))
			for _, mode := range strings.Split(*modes, ",") {
				got, err := measure(ctx, abs, *env, dir, caches, sc, *timeout, *poll, *edit,
					*typing)
				if err != nil {
					fmt.Fprintf(os.Stderr, "cppdata: %s %s run %d: %v\n", sc.name, mode, r, err)
					return 1
				}
				for _, kind := range got {
					samples = append(samples, sample{sc.name, mode, r, kind.name, kind.ms})
				}
			}
			os.RemoveAll(caches)
		}
	}
	report(os.Stdout, samples)
	if *jsonOut != "" {
		data, _ := json.MarshalIndent(samples, "", " ")
		if err := os.WriteFile(*jsonOut, data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "cppdata:", err)
			return 1
		}
	}
	return 0
}

type kindTime struct {
	name string
	ms   float64
}

// measure starts the server on a fresh copy of the project with the caches in caches (the
// folder as the previous call left it: empty for the first), opens the file, and returns the
// time of each kind of data. The first entry is the first publish of diagnostics.
func measure(ctx context.Context, binary, env, base, caches string, sc scenario,
	timeout, pollEvery time.Duration, edit bool, keyEvery time.Duration,
) ([]kindTime, error) {
	// The folder is the same for the cold and the warm start: the probe keys name it.
	root := filepath.Join(base, sc.name+"-proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	files := map[string]string{"gocpp.mod": "module example.com/cppdata\n"}
	for name, text := range sc.files {
		files[name] = text
	}
	for name, text := range files {
		path := filepath.Join(root, name)
		if old, err := os.ReadFile(path); err == nil && string(old) == text {
			continue
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			return nil, err
		}
	}
	// The probe cache does not keep an answer that depends on a file changed less than two
	// seconds before the compiler started (the racy window): let the files age first.
	for name := range files {
		if info, err := os.Stat(filepath.Join(root, name)); err == nil {
			time.Sleep(2500*time.Millisecond - time.Since(info.ModTime()))
		}
	}
	target := bench.Target{Name: "gocpp", Ext: ".gp", Command: []string{binary, "lsp"},
		Env: strings.Split(env, ",")}
	proc, err := bench.StartProc(ctx, target, root, caches)
	if err != nil {
		return nil, err
	}
	defer proc.Kill()
	client := proc.Client
	uri := protocol.URIFromPath(root)
	_, err = client.Call(ctx, "initialize", map[string]any{
		"processId": os.Getpid(), "rootUri": uri,
		"workspaceFolders": []map[string]any{{"uri": uri, "name": "cppdata"}},
		"capabilities": map[string]any{"textDocument": map[string]any{
			"hover": map[string]any{"contentFormat": []string{"markdown", "plaintext"}},
			"completion": map[string]any{"completionItem": map[string]any{
				"documentationFormat": []string{"markdown", "plaintext"}}},
			"publishDiagnostics": map[string]any{"versionSupport": true},
		}},
	}, nil)
	if err != nil {
		return nil, err
	}
	if err := client.Notify("initialized", map[string]any{}); err != nil {
		return nil, err
	}
	docURI := protocol.URIFromPath(filepath.Join(root, sc.main))
	source := sc.files[sc.main]
	opened := time.Now()
	err = client.Notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": docURI, "languageId": "gocpp", "version": 1, "text": source}})
	if err != nil {
		return nil, err
	}
	typingCtx, stopTyping := context.WithCancel(ctx)
	defer stopTyping()
	var typed sync.WaitGroup
	if keyEvery > 0 {
		typed.Add(1)
		go func() {
			defer typed.Done()
			typeComment(typingCtx, client, docURI, source, keyEvery)
		}()
	}
	var mu sync.Mutex
	var out []kindTime
	var wg sync.WaitGroup
	var requests latencies
	record := func(name string, ms float64) {
		mu.Lock()
		out = append(out, kindTime{name, ms})
		mu.Unlock()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		wctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		anyPublish := func(*bench.Publish) bool { return true }
		if _, err := client.WaitPublish(wctx, docURI, 0, anyPublish); err != nil {
			record("first diagnostics", -1)
			return
		}
		record("first diagnostics", float64(time.Since(opened).Microseconds())/1000)
	}()
	for _, p := range sc.probes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ms, err := pollKind(ctx, client, docURI, source, p, opened, timeout, pollEvery,
				&requests)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cppdata: %s: %v\n", p.kind, err)
			}
			record(p.kind, ms)
		}()
	}
	wg.Wait()
	stopTyping()
	typed.Wait()
	for _, kind := range requests.summary() {
		record(kind.name, kind.ms)
	}
	if edit {
		// A keystroke makes a new snapshot: when is the data there again for it?
		edited := time.Now()
		lines := strings.Count(source, "\n")
		err := client.Notify("textDocument/didChange", map[string]any{
			"textDocument": map[string]any{"uri": docURI, "version": 2},
			"contentChanges": []map[string]any{{
				"range": map[string]any{
					"start": map[string]any{"line": lines, "character": 0},
					"end":   map[string]any{"line": lines, "character": 0},
				},
				"text": "// edit\n",
			}},
		})
		if err != nil {
			return nil, err
		}
		for _, p := range sc.probes {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ms, _ := pollKind(ctx, client, docURI, source, p, edited, timeout, pollEvery, nil)
				record("after edit: "+p.kind, ms)
			}()
		}
		wg.Wait()
	}
	proc.Stop()
	return out, nil
}

// typeComment appends a comment line to the document, one character per tick, until ctx ends.
// The text stays valid, so the server checks the original text, as it does for a user who types
// a comment; every key is a new snapshot.
func typeComment(ctx context.Context, client *bench.Client, uri protocol.DocumentURI, source string,
	every time.Duration,
) {
	line := strings.Count(source, "\n")
	column := 0
	text := "//"
	for version := 2; ctx.Err() == nil; version++ {
		err := client.Notify("textDocument/didChange", map[string]any{
			"textDocument": map[string]any{"uri": uri, "version": version},
			"contentChanges": []map[string]any{{
				"range": map[string]any{
					"start": map[string]any{"line": line, "character": column},
					"end":   map[string]any{"line": line, "character": column},
				},
				"text": text,
			}},
		})
		if err != nil {
			return
		}
		column += len(text)
		text = "k"
		select {
		case <-time.After(every):
		case <-ctx.Done():
		}
	}
}

func pollKind(ctx context.Context, client *bench.Client, uri protocol.DocumentURI, source string,
	p probe, opened time.Time, timeout, pollEvery time.Duration, latencies *latencies,
) (float64, error) {
	plain := strings.ReplaceAll(source, "|", "")
	offset := strings.Index(plain, strings.ReplaceAll(p.cursor, "|", ""))
	before, _, _ := strings.Cut(p.cursor, "|")
	if offset < 0 {
		return -1, fmt.Errorf("cursor text %q not in the source", p.cursor)
	}
	offset += len(before)
	position := protocol.NewMapper(source).Position(offset)
	params := protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri}, Position: position}
	method := map[string]string{"completion": "textDocument/completion",
		"hover": "textDocument/hover", "definition": "textDocument/definition"}[p.method]
	deadline := time.Now().Add(timeout)
	last := ""
	defer func() {
		if last != "" && os.Getenv("CPPDATA_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "cppdata: %s last answer: %.300s\n", p.kind, last)
		}
	}()
	for time.Now().Before(deadline) && ctx.Err() == nil {
		var raw json.RawMessage
		sent := time.Now()
		if _, err := client.Call(ctx, method, params, &raw); err != nil {
			return -1, err
		}
		at := time.Since(opened)
		latencies.add(sent.Sub(opened), time.Since(sent))
		text := string(raw)
		if p.method == "completion" {
			text = completionText(raw)
		}
		if p.ok(text) {
			last = ""
			return float64(at.Microseconds()) / 1000, nil
		}
		last = text
		time.Sleep(pollEvery)
	}
	return -1, nil
}

// completionText keeps what the probes test: the labels, and "ready" when the member status
// of the list says the facts are available.
func completionText(raw json.RawMessage) string {
	var list struct {
		Items []struct {
			Label string `json:"label"`
		} `json:"items"`
		Members *struct {
			Ready bool `json:"ready"`
		} `json:"x-gocpp-members"`
	}
	if json.Unmarshal(raw, &list) != nil {
		return ""
	}
	var sb strings.Builder
	if list.Members != nil && list.Members.Ready {
		sb.WriteString("ready ")
	}
	for _, item := range list.Items {
		fmt.Fprintf(&sb, "%q ", item.Label)
	}
	return sb.String()
}

// requestWindow is the time after didOpen in which the latency of the requests is kept: the
// time of the cold background work, the same for every configuration.
const requestWindow = 1500 * time.Millisecond

// latencies collects how long the requests took that the polls sent in the first requestWindow
// after didOpen, while the compiler runs in the background (or does not, in opaque mode).
type latencies struct {
	mu      sync.Mutex
	samples []float64
}

func (l *latencies) add(sentAt, took time.Duration) {
	if l == nil || sentAt > requestWindow {
		return
	}
	l.mu.Lock()
	l.samples = append(l.samples, float64(took.Microseconds())/1000)
	l.mu.Unlock()
}

func (l *latencies) summary() []kindTime {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.samples) == 0 {
		return nil
	}
	values := slices.Clone(l.samples)
	return []kindTime{
		{"request latency p50 (first 1.5 s)", percentile(slices.Clone(values), 50)},
		{"request latency p95 (first 1.5 s)", percentile(slices.Clone(values), 95)},
		{"request latency max (first 1.5 s)", slices.Max(values)},
	}
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	slices.Sort(values)
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return values[mid]
	}
	return (values[mid-1] + values[mid]) / 2
}

func report(out *os.File, samples []sample) {
	type key struct{ scenario, kind string }
	var order []key
	byKey := make(map[key]map[string][]float64)
	never := make(map[key]map[string]int)
	for _, s := range samples {
		k := key{s.Scenario, s.Kind}
		if byKey[k] == nil {
			byKey[k] = make(map[string][]float64)
			never[k] = make(map[string]int)
			order = append(order, k)
		}
		if s.Millis < 0 {
			never[k][s.Mode]++
			continue
		}
		byKey[k][s.Mode] = append(byKey[k][s.Mode], s.Millis)
	}
	fmt.Fprintln(out, "| scenario | kind | cold median ms | cold min-max | warm median ms "+
		"| warm min-max | n | never |")
	fmt.Fprintln(out, "|---|---|---:|---|---:|---|---:|---:|")
	for _, k := range order {
		cold, warm := byKey[k]["cold"], byKey[k]["warm"]
		fmt.Fprintf(out, "| %s | %s | %.0f | %s | %.0f | %s | %d | %d |\n", k.scenario, k.kind,
			median(slices.Clone(cold)), span(cold), median(slices.Clone(warm)), span(warm),
			len(cold), never[k]["cold"]+never[k]["warm"])
	}
}

func span(values []float64) string {
	if len(values) == 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f-%.0f", slices.Min(values), slices.Max(values))
}
