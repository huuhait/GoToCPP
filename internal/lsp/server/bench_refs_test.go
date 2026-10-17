package server

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// BenchmarkReferences and BenchmarkRename measure the budget "find references
// / rename across the workspace < 300 ms (p95)" (README §2.1) end to end, over
// pipes, on the 10k-line package after the first check (warm: each request
// ran once): references of a type used in every file, of a function, a method
// (which also looks at the interfaces it implements, fmt.Stringer among
// them), a field, and a local variable; and renames of a local variable, a
// function and a field (each one checks the packages it edits again). Run them
// with
//
//	go test -run '^$' -bench 'References|Rename' -benchtime 50x ./internal/lsp/server
func BenchmarkReferences(b *testing.B) {
	benchRefs(b, "textDocument/references", []navMark{
		{"big/f25.gp", "type T25| struct"},
		{"big/f25.gp", "func F25_|3(n int)"},
		{"big/f25.gp", "func (t T25) Str|ing()"},
		{"big/f25.gp", "\tNam|e  string"},
		{"big/f25.gp", "tot|al := 0"},
		{"main.gp", "big.F0|_0(1)"},
	}, `,"context":{"includeDeclaration":true}`)
}

func BenchmarkRename(b *testing.B) {
	benchRefs(b, "textDocument/rename", []navMark{
		{"big/f25.gp", "tot|al := 0"},
		{"big/f25.gp", "func F25_|3(n int)"},
		{"big/f25.gp", "\tNam|e  string"},
		{"main.gp", "big.F0|_0(1)"},
	}, `,"newName":"Renamed"`)
}

func BenchmarkDocumentSymbol(b *testing.B) {
	files := bigPackage()
	c := startBench(b, files)
	c.open("big/f25.gp", files["big/f25.gp"])
	params := docParams(c.uri("big/f25.gp"))
	c.call("textDocument/documentSymbol", params)
	lat := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for range b.N {
		start := time.Now()
		resp := c.call("textDocument/documentSymbol", params)
		lat = append(lat, time.Since(start))
		if resp.Error != nil {
			b.Fatal(resp.Error)
		}
	}
	b.StopTimer()
	reportLatency(b, lat, "p95 < 30 ms")
}

// BenchmarkWorkspaceSymbol measures workspace/symbol on the 10k-line package
// (about 2000 symbols), warm and cold (a new server: it parses the package).
func BenchmarkWorkspaceSymbol(b *testing.B) {
	files := bigPackage()
	c := startRefsRootB(b, files)
	queries := []string{"F25_3", "t25", "", "String", "count"}
	for _, q := range queries {
		c.call("workspace/symbol", fmt.Sprintf(`{"query":%q}`, q))
	}
	lat := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for i := range b.N {
		start := time.Now()
		resp := c.call("workspace/symbol", fmt.Sprintf(`{"query":%q}`, queries[i%len(queries)]))
		lat = append(lat, time.Since(start))
		if resp.Error != nil {
			b.Fatal(resp.Error)
		}
	}
	b.StopTimer()
	reportLatency(b, lat, "p95 < 100 ms")
}

func BenchmarkWorkspaceSymbolCold(b *testing.B) {
	files := bigPackage()
	root := writeTree(b, files)
	var lat []time.Duration
	for range b.N {
		c := startOnRoot(b, root)
		start := time.Now()
		resp := c.call("workspace/symbol", `{"query":"F25_3"}`)
		lat = append(lat, time.Since(start))
		if resp.Error != nil {
			b.Fatal(resp.Error)
		}
		c.in.Close()
	}
	b.StopTimer()
	reportLatency(b, lat, "no budget: the index is made in the background after initialized")
}

// startRefsRootB is startRefsRoot for a benchmark.
func startRefsRootB(b *testing.B, files map[string]string) *dclient {
	return startOnRoot(b, writeTree(b, files))
}

func startOnRoot(b testing.TB, root string) *dclient {
	c := &dclient{client: startServer(b, newServerForBench(), nil), root: root}
	uri := fileURI(root)
	c.call("initialize", fmt.Sprintf(`{"processId":1,"rootUri":%q,"workspaceFolders":[{"uri":%q,"name":"ws"}],"capabilities":{}}`, uri, uri))
	c.notify("initialized", "{}")
	return c
}

// benchRefs runs the requests of method at the marks, after one run of each
// (the warm-up), and reports their latency. tail is the rest of the params
// after the position.
func benchRefs(b *testing.B, method string, marks []navMark, tail string) {
	files := bigPackage()
	c := startBench(b, files)
	params := make([]string, len(marks))
	for i, m := range marks {
		c.open(m.file, files[m.file])
		params[i] = strings.TrimSuffix(c.posParams(m.file, files[m.file], m.mark), "}") + tail + "}"
	}
	for i, p := range params {
		resp := c.call(method, p)
		if resp.Error != nil || string(resp.Result) == "null" || string(resp.Result) == "[]" {
			b.Fatalf("%s at %q: %s (error %v)", method, marks[i].mark, resp.Result, resp.Error)
		}
	}
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
	reportLatency(b, lat, "p95 < 300 ms")
}
