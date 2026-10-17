package server

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkSemanticTokens, BenchmarkInlayHints and BenchmarkDocumentLinks
// measure the requests of step 11.9 end to end, over pipes, on a file of the
// 10k-line package (about 200 lines; the whole package is checked before: warm,
// each request ran once). They have no budget row of their own in README
// §2.1; they are held to the one of hover, p95 < 30 ms, which an editor needs
// of what it asks for at every change of the text. Run them with
//
//	go test -run '^$' -bench 'SemanticTokens|InlayHints|DocumentLinks' -benchtime 100x ./internal/lsp/server
func BenchmarkSemanticTokens(b *testing.B) {
	benchEditor(b, "textDocument/semanticTokens/full", func(c *dclient, name string) string { return docParams(c.uri(name)) })
}

func BenchmarkInlayHints(b *testing.B) {
	benchEditor(b, "textDocument/inlayHint", func(c *dclient, name string) string {
		return fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":0,"character":0},"end":{"line":100000,"character":0}}}`, c.uri(name))
	})
}

func BenchmarkDocumentLinks(b *testing.B) {
	benchEditor(b, "textDocument/documentLink", func(c *dclient, name string) string { return docParams(c.uri(name)) })
}

func benchEditor(b *testing.B, method string, params func(*dclient, string) string) {
	files := bigPackage()
	c := startBench(b, files)
	name := "big/f25.gp"
	c.open(name, files[name])
	p := params(c, name)
	if resp := c.call(method, p); resp.Error != nil || string(resp.Result) == "null" || string(resp.Result) == "[]" {
		if method != "textDocument/documentLink" {
			b.Fatalf("%s: %.200s (error %v)", method, resp.Result, resp.Error)
		}
	}
	_, _, checks := c.s.pipeline.Stats()
	lat := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for range b.N {
		start := time.Now()
		resp := c.call(method, p)
		lat = append(lat, time.Since(start))
		if resp.Error != nil {
			b.Fatal(resp.Error)
		}
	}
	b.StopTimer()
	if _, _, n := c.s.pipeline.Stats(); n != checks {
		b.Errorf("%d checks during the requests, want none", n-checks)
	}
	reportLatency(b, lat, "p95 < 30 ms")
}
