package server

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/bench"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// BenchmarkGen runs the requests of the rows of `make lsp-bench` (the
// generated projects of internal/lsp/bench: references, rename, workspace
// symbols, semantic tokens) in this process, warm, so that go test's
// -cpuprofile and -memprofile show where the time goes without the cost of
// opening the project. GOCPP_BENCH_PROJECT picks the project (small, medium,
// large; default medium). Run one row with
//
//	go test -run '^$' -bench 'Gen/references-family' -benchtime 20x ./internal/lsp/server
func BenchmarkGen(b *testing.B) {
	name := os.Getenv("GOCPP_BENCH_PROJECT")
	if name == "" {
		name = "medium"
	}
	spec, ok := bench.SpecByName(name)
	if !ok {
		b.Fatalf("no project %q", name)
	}
	p := bench.Generate(spec)
	root := b.TempDir()
	if err := p.Write(root, ".gp"); err != nil {
		b.Fatal(err)
	}
	c := startOnRoot(b, root)
	c.skipShown = true // a rename that changes other declarations says so
	read := func(rel string) string {
		data, err := os.ReadFile(root + "/" + rel + ".gp")
		if err != nil {
			b.Fatal(err)
		}
		return string(data)
	}
	// where returns the params of a request at off bytes into the mark of the file.
	where := func(rel, mark string, off int) string {
		src := read(rel)
		i := strings.Index(src, mark)
		if i < 0 {
			b.Fatalf("%q is not in %s", mark, rel)
		}
		pos := protocol.NewMapper(src).Position(i + off)
		return fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d}`, c.uri(rel+".gp"), pos.Line, pos.Character)
	}
	open := func(rel string) { c.open(rel+".gp", read(rel)) }
	for _, rel := range []string{p.Hot, p.Core} {
		open(strings.TrimSuffix(rel, ".gp"))
	}
	rel := func(f string) string { return strings.TrimSuffix(f, ".gp") }
	hot, core := rel(p.Hot), rel(p.Core)
	// The first diagnostics of the open files: the project is checked.
	c.next(hot + ".gp")
	type row struct {
		name, method, params string
		wait                 bool
	}
	incl := `,"context":{"includeDeclaration":true}}`
	rows := []row{
		{"references-popular", "textDocument/references", where(core, "type Base struct", 5) + incl, false},
		{"references-local", "textDocument/references", where(hot, "\ntype ", 6) + incl, false},
		{"references-family", "textDocument/references", where(hot, ") Add(tag string)", 2) + incl, false},
		{"rename-method", "textDocument/rename", where(hot, ") Add(tag string)", 2) + `,"newName":"Plus"}`, false},
		{"rename", "textDocument/rename", where(rel(p.HotPkg+"/f00.gp"), "func Label(", 5) + `,"newName":"TagOf"}`, false},
		{"semantic-tokens", "textDocument/semanticTokens/full", fmt.Sprintf(`{"textDocument":{"uri":%q}}`, c.uri(hot+".gp")), false},
		{"document-symbol", "textDocument/documentSymbol", fmt.Sprintf(`{"textDocument":{"uri":%q}}`, c.uri(hot+".gp")), false},
	}
	for _, r := range rows {
		b.Run(r.name, func(b *testing.B) {
			open(rel(p.HotPkg + "/f00.gp"))
			resp := c.call(r.method, r.params)
			if resp.Error != nil || string(resp.Result) == "null" {
				b.Fatalf("%s: %.200s (error %v)", r.name, resp.Result, resp.Error)
			}
			n := strings.Count(string(resp.Result), `"range"`)
			lat := make([]time.Duration, 0, b.N)
			cpu0 := processCPU()
			b.ResetTimer()
			for range b.N {
				start := time.Now()
				resp := c.call(r.method, r.params)
				lat = append(lat, time.Since(start))
				if resp.Error != nil {
					b.Fatal(resp.Error)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(processCPU()-cpu0)/float64(b.N)/1e6, "cpu-ms/op")
			b.ReportMetric(float64(n), "ranges")
			reportLatency(b, lat, "none")
		})
	}
	b.Run("workspace-symbol", func(b *testing.B) {
		for _, q := range p.Queries {
			c.call("workspace/symbol", fmt.Sprintf(`{"query":%q}`, q))
		}
		lat := make([]time.Duration, 0, b.N)
		cpu0 := processCPU()
		b.ResetTimer()
		for i := range b.N {
			start := time.Now()
			resp := c.call("workspace/symbol", fmt.Sprintf(`{"query":%q}`, p.Queries[i%len(p.Queries)]))
			lat = append(lat, time.Since(start))
			if resp.Error != nil {
				b.Fatal(resp.Error)
			}
		}
		b.StopTimer()
		b.ReportMetric(float64(processCPU()-cpu0)/float64(b.N)/1e6, "cpu-ms/op")
		reportLatency(b, lat, "none")
	})
}
