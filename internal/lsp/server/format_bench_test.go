package server

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/debounce"
)

// thousandLines returns a formatted file of package big of about 1,000
// lines that imports fmt, strings and os (os unused when unusedOS is set),
// with its imports out of order.
func thousandLines(unusedOS bool) string {
	var b strings.Builder
	b.WriteString("package big\n\nimport (\n\t\"strings\"\n\t\"os\"\n\t\"fmt\"\n)\n\n")
	if !unusedOS {
		b.WriteString("var _ = os.Args\n\n")
	}
	for i := 0; strings.Count(b.String(), "\n") < 990; i++ {
		fmt.Fprintf(&b, "// F%d is a generated function.\nfunc F%d(n int) int {\n", i, i)
		b.WriteString("\ttotal := 0\n\tparts := []string{}\n")
		b.WriteString("\tfor k := 0; k < n; k++ {\n\t\tif k%2 == 0 {\n\t\t\ttotal += k\n\t\t} else {\n\t\t\ttotal -= k / 2\n\t\t}\n")
		b.WriteString("\t\tparts = append(parts, fmt.Sprint(k))\n\t}\n")
		b.WriteString("\tif strings.HasPrefix(strings.Join(parts, \",\"), \"x\") {\n\t\treturn -1\n\t}\n")
		b.WriteString("\tm := map[string]int{}\n\tfor _, p := range parts {\n\t\tm[p]++\n\t}\n")
		b.WriteString("\treturn total + len(m)\n}\n\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// latencies runs f b.N times and reports the p50 and p95 latency.
func latencies(b *testing.B, budget time.Duration, f func(i int)) {
	lat := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for i := range b.N {
		start := time.Now()
		f(i)
		lat = append(lat, time.Since(start))
	}
	b.StopTimer()
	slices.Sort(lat)
	p50, p95 := lat[len(lat)/2], lat[min(len(lat)-1, len(lat)*95/100)]
	b.ReportMetric(float64(p50.Microseconds())/1000, "p50-ms")
	b.ReportMetric(float64(p95.Microseconds())/1000, "p95-ms")
	b.Logf("%d requests: p50 %v, p95 %v (budget: p95 < %v)", len(lat), p50, p95, budget)
}

// BenchmarkFormat measures the budget "format a 1,000-line file < 20 ms
// (p95)" (README §2.1) end to end, over pipes: textDocument/formatting of
// an open 1,000-line file that is formatted already (format on save),
// that has one line to fix, and that has lost all its indentation (every
// line but the top-level ones changes). Run it with
//
//	go test -run '^$' -bench Format -benchtime 200x ./internal/lsp/server
func BenchmarkFormat(b *testing.B) {
	src := thousandLines(false)
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimLeft(l, "\t")
	}
	for _, bc := range []struct{ name, text string }{
		{"formatted", src},
		{"one line", strings.Replace(src, "\ttotal := 0\n", "\ttotal   :=   0\n", 1)},
		{"no indentation", strings.Join(lines, "\n")},
	} {
		b.Run(bc.name, func(b *testing.B) {
			files := map[string]string{"gocpp.mod": "module example.com/bench\n", "big/big.gp": src}
			root := writeTree(b, files)
			c := &dclient{client: startServer(b, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
			c.call("initialize", initParams)
			c.open("big/big.gp", bc.text)
			params := fmt.Sprintf(`{"textDocument":{"uri":%q},"options":{"tabSize":8,"insertSpaces":false}}`, c.uri("big/big.gp"))
			latencies(b, 20*time.Millisecond, func(int) {
				if resp := c.call("textDocument/formatting", params); resp.Error != nil {
					b.Fatal(resp.Error)
				}
			})
		})
	}
}

// BenchmarkOrganizeImports measures "Organize Imports" (the code action) of
// an open 1,000-line file with an unused import, whose package is checked
// (the check is cached, as after the diagnostics). gopls's budget for code
// actions is the one of hover (< 30 ms, README §2.1).
func BenchmarkOrganizeImports(b *testing.B) {
	src := thousandLines(true)
	files := map[string]string{"gocpp.mod": "module example.com/bench\n", "big/big.gp": src}
	root := writeTree(b, files)
	c := &dclient{client: startServer(b, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
	c.call("initialize", initParams)
	c.open("big/big.gp", src)
	if p := c.next("big/big.gp"); len(p.Diagnostics) != 1 {
		b.Fatalf("diagnostics:\n%s", format(p))
	}
	params := fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},`+
		`"context":{"diagnostics":[],"only":["source.organizeImports"]}}`, c.uri("big/big.gp"))
	latencies(b, 30*time.Millisecond, func(int) {
		resp := c.call("textDocument/codeAction", params)
		if resp.Error != nil || !strings.Contains(string(resp.Result), "Organize Imports") {
			b.Fatalf("got %s (%v)", resp.Result, resp.Error)
		}
	})
}
