package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/debounce"
)

// BenchmarkKeystroke measures the budget "keystroke → updated gocpp
// diagnostics < 100 ms (p95)" (README §2.1) end to end, over pipes: a
// didChange of one line in a function body of a 10k-line package, then the
// publishDiagnostics of that version, with the real debounce delay
// (diagnoseDelay). Run it with
//
//	go test -run '^$' -bench Keystroke -benchtime 100x ./internal/lsp/server
//
// It reports the p50 and p95 latency of the b.N edits (the edits come one
// after the other, each after the diagnostics of the one before).
func BenchmarkKeystroke(b *testing.B) {
	files := bigPackage()
	root := writeTree(b, files)
	c := &dclient{client: startServer(b, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
	c.call("initialize", initParams)
	name := "big/f25.gp"
	src := files[name]
	c.open(name, src)
	if p := c.next(name); len(p.Diagnostics) > 0 {
		b.Fatalf("errors:\n%s", format(p))
	}
	lat := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for i := range b.N {
		start := time.Now()
		c.change(name, i+2, strings.Replace(src, "// EDIT", fmt.Sprintf("+ %d // EDIT", i), 1))
		p := c.next(name)
		lat = append(lat, time.Since(start))
		if p.Version != int32(i+2) || len(p.Diagnostics) > 0 {
			b.Fatalf("edit %d: version %d, diagnostics:\n%s", i, p.Version, format(p))
		}
	}
	b.StopTimer()
	slices.Sort(lat)
	p50, p95 := lat[len(lat)/2], lat[min(len(lat)-1, len(lat)*95/100)]
	b.ReportMetric(float64(p50.Microseconds())/1000, "p50-ms")
	b.ReportMetric(float64(p95.Microseconds())/1000, "p95-ms")
	b.Logf("%d edits: p50 %v, p95 %v (budget: p95 < 100 ms; %v of it is the debounce delay)", len(lat), p50, p95, diagnoseDelay)
}

// BenchmarkOpenCold measures "open a project → first diagnostics" (README
// §2.1: < 2 s cold) for the 10k-line package: a new server (nothing
// cached, not even the standard packages it imports) gets didOpen of a file
// of big and publishes its diagnostics.
func BenchmarkOpenCold(b *testing.B) {
	files := bigPackage()
	root := writeTree(b, files)
	name := "big/f25.gp"
	for range b.N {
		c := &dclient{client: startServer(b, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
		c.call("initialize", initParams)
		c.open(name, files[name])
		if p := c.next(name); len(p.Diagnostics) > 0 {
			b.Fatalf("errors:\n%s", format(p))
		}
		c.in.Close()
	}
}

// BenchmarkHover, BenchmarkDefinition and BenchmarkSignatureHelp measure
// the budget "hover, go-to-definition, signature help < 30 ms (p95)"
// (README §2.1) end to end, over pipes, in the 10k-line package after the
// first check (warm: each request ran once): the requests come one after
// the other, at places that rotate (names of the package, of another file,
// of the standard library and, for definition, from the main package into
// the big one, from export data). Run them with
//
//	go test -run '^$' -bench 'Hover|Definition|SignatureHelp' -benchtime 300x ./internal/lsp/server
//
// They report the p50 and p95 latency of the b.N requests.
func BenchmarkHover(b *testing.B) {
	benchNav(b, "textDocument/hover", []navMark{
		{"big/f25.gp", "t.Str|ing(), \"x\""},
		{"big/f25.gp", "strings.Jo|in(parts"},
		{"big/f25.gp", "T25{Na|me:"},
		{"big/f25.gp", "fmt.Spr|int(k)"},
		{"big/f25.gp", "func (t T2|5)"},
		{"main.gp", "big.F0|_0(1)"},
	})
}

func BenchmarkDefinition(b *testing.B) {
	benchNav(b, "textDocument/definition", []navMark{
		{"big/f25.gp", "t := T2|5{"},
		{"big/f25.gp", "t.Str|ing(), \"x\""},
		{"big/f25.gp", "for _, p := range par|ts"},
		{"main.gp", "big.F0|_0(1)"},
		{"main.gp", "big.T|0{}"},
	})
}

func BenchmarkSignatureHelp(b *testing.B) {
	benchNav(b, "textDocument/signatureHelp", []navMark{
		{"big/f25.gp", "strings.HasPrefix(t.String()|, \"x\")"},
		{"big/f25.gp", "append(parts, |fmt.Sprint(k))"},
		{"big/f25.gp", "fmt.Sprint(|k)"},
		{"main.gp", "big.F0_0(|1)"},
	})
}

// A navMark is a place in a file of bigPackage: the | of mark.
type navMark struct{ file, mark string }

// benchNav runs the requests of method at the marks, after one run of each
// (the warm-up), and reports their latency.
func benchNav(b *testing.B, method string, marks []navMark) {
	files := bigPackage()
	root := writeTree(b, files)
	c := &dclient{client: startServer(b, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
	c.call("initialize", markdownInit)
	params := make([]string, len(marks))
	for i, m := range marks {
		c.open(m.file, files[m.file])
		params[i] = c.posParams(m.file, files[m.file], m.mark)
	}
	for i, p := range params {
		if resp := c.call(method, p); resp.Error != nil || string(resp.Result) == "null" || string(resp.Result) == "[]" {
			b.Fatalf("%s at %q: %s (error %v)", method, marks[i].mark, resp.Result, resp.Error)
		}
	}
	_, _, checks := c.s.pipeline.Stats()
	lat := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for i := range b.N {
		start := time.Now()
		resp := c.call(method, params[i%len(params)])
		lat = append(lat, time.Since(start))
		if resp.Error != nil {
			b.Fatal(resp.Error)
		}
	}
	b.StopTimer()
	if _, _, n := c.s.pipeline.Stats(); n != checks {
		b.Errorf("%d checks during the requests, want none", n-checks)
	}
	slices.Sort(lat)
	p50, p95 := lat[len(lat)/2], lat[min(len(lat)-1, len(lat)*95/100)]
	b.ReportMetric(float64(p50.Microseconds())/1000, "p50-ms")
	b.ReportMetric(float64(p95.Microseconds())/1000, "p95-ms")
	b.Logf("%d requests: p50 %v, p95 %v (budget: p95 < 30 ms)", len(lat), p50, p95)
}

// bigPackage returns a module with a package big of 50 generated files of
// about 200 lines each (10k lines), which imports fmt and strings, and a
// main package that imports it (cache's bigPackage, for its benchmarks).
func bigPackage() map[string]string {
	files := map[string]string{
		"gocpp.mod": "module example.com/bench\n",
		"main.gp":   "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/bench/big\"\n)\n\nfunc main() { fmt.Println(big.F0_0(1), big.T0{}) }\n",
	}
	for i := range 50 {
		var b strings.Builder
		fmt.Fprintf(&b, "package big\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\n")
		fmt.Fprintf(&b, "// T%d is a generated type.\ntype T%d struct {\n\tName  string\n\tCount int\n\tTags  []string\n}\n\n", i, i)
		fmt.Fprintf(&b, "// String returns the name.\nfunc (t T%d) String() string { return fmt.Sprintf(\"%%s:%%d\", t.Name, t.Count) }\n\n", i)
		for j := range 8 {
			fmt.Fprintf(&b, "// F%d_%d is a generated function.\nfunc F%d_%d(n int) int {\n", i, j, i, j)
			b.WriteString("\ttotal := 0\n\tparts := []string{}\n")
			b.WriteString("\tfor k := 0; k < n; k++ {\n\t\tif k%2 == 0 {\n\t\t\ttotal += k\n\t\t} else {\n\t\t\ttotal -= k / 2\n\t\t}\n")
			b.WriteString("\t\tparts = append(parts, fmt.Sprint(k))\n\t}\n")
			fmt.Fprintf(&b, "\tt := T%d{Name: strings.Join(parts, \",\"), Count: total}\n", i)
			b.WriteString("\tif strings.HasPrefix(t.String(), \"x\") {\n\t\treturn -1\n\t}\n")
			b.WriteString("\tm := map[string]int{}\n\tfor _, p := range parts {\n\t\tm[p]++\n\t}\n")
			b.WriteString("\treturn total + len(m) // EDIT\n}\n\n")
		}
		files[fmt.Sprintf("big/f%02d.gp", i)] = b.String()
	}
	return files
}

// BenchmarkCompletion measures the budget "completion < 50 ms (p95)"
// (README §2.1) on the 10k-line package: completions in f25.gp after a
// selector (t.|, t.Str|), at a blank statement (all the names in scope:
// the 400 functions of the package, its types, the builtins), after F
// (the 400 functions), and after a package to import (bytes.). The package
// is checked before (the request uses the cached check):
//
//	go test -run '^$' -bench 'Completion' -benchtime 300x ./internal/lsp/server
//
// BenchmarkCompletionAfterEdit measures the completion that comes right
// after an edit (a new line t. in a function), so the request checks the
// package first (it doesn't parse: x. is a syntax error).
func BenchmarkCompletion(b *testing.B) {
	files := bigPackage()
	name := "big/f25.gp"
	src := strings.Replace(files[name], "\ttotal := 0\n", "\ttotal := 0\n\t\n\tF\n\tbytes.\n", 1)
	c := startBench(b, files)
	c.open(name, src)
	var params []string
	for _, mark := range []string{"t.|String(), \"x\"", "t.Str|ing(), \"x\"", "\ttotal := 0\n\t|\n", "\tF|\n", "\tbytes.|\n"} {
		params = append(params, c.posParams(name, src, mark))
	}
	for i, p := range params {
		if items := completionItems(b, c, p); len(items) == 0 {
			b.Fatalf("no completion at mark %d", i)
		}
	}
	_, _, checks := c.s.pipeline.Stats()
	lat := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for i := range b.N {
		start := time.Now()
		resp := c.call("textDocument/completion", params[i%len(params)])
		lat = append(lat, time.Since(start))
		if resp.Error != nil {
			b.Fatal(resp.Error)
		}
	}
	b.StopTimer()
	if _, _, n := c.s.pipeline.Stats(); n != checks {
		b.Errorf("%d checks during the requests, want none", n-checks)
	}
	reportLatency(b, lat, "p95 < 50 ms")
}

func BenchmarkCompletionAfterEdit(b *testing.B) {
	files := bigPackage()
	name := "big/f25.gp"
	src := files[name]
	c := startBench(b, files)
	c.open(name, src)
	c.next(name) // the diagnostics: the package is checked
	lat := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for i := range b.N {
		// Each edit makes new text: a check that the cache doesn't have.
		edited := strings.Replace(src, "\ttotal := 0\n", fmt.Sprintf("\ttotal := %d\n", i), 1)
		edited = strings.Replace(edited, "\tif strings.HasPrefix(", "\tt.\n\tif strings.HasPrefix(", 1)
		start := time.Now()
		c.change(name, i+2, edited)
		resp := c.call("textDocument/completion", c.posParams(name, edited, "\tt.|\n"))
		lat = append(lat, time.Since(start))
		if resp.Error != nil {
			b.Fatal(resp.Error)
		}
		if i == 0 && !strings.Contains(string(resp.Result), `"label":"String"`) {
			b.Fatalf("the completion: %s", resp.Result)
		}
	}
	b.StopTimer()
	reportLatency(b, lat, "p95 < 50 ms")
}

// startBench starts a server over the files, initialized for snippets.
func startBench(b *testing.B, files map[string]string) *dclient {
	root := writeTree(b, files)
	c := &dclient{client: startServer(b, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
	c.call("initialize", `{"processId":1,"rootUri":null,"capabilities":{"textDocument":{"completion":{"completionItem":{"snippetSupport":true}}}}}`)
	c.notify("initialized", "{}")
	return c
}

// completionItems returns the items of a completion with the params.
func completionItems(b *testing.B, c *dclient, params string) []json.RawMessage {
	resp := c.call("textDocument/completion", params)
	if resp.Error != nil {
		b.Fatal(resp.Error)
	}
	var list struct{ Items []json.RawMessage }
	if err := json.Unmarshal(resp.Result, &list); err != nil {
		b.Fatal(err)
	}
	return list.Items
}

// reportLatency reports the p50 and p95 of the latencies.
func reportLatency(b *testing.B, lat []time.Duration, budget string) {
	slices.Sort(lat)
	p50, p95 := lat[len(lat)/2], lat[min(len(lat)-1, len(lat)*95/100)]
	b.ReportMetric(float64(p50.Microseconds())/1000, "p50-ms")
	b.ReportMetric(float64(p95.Microseconds())/1000, "p95-ms")
	b.Logf("%d requests: p50 %v, p95 %v (budget: %s)", len(lat), p50, p95, budget)
}
