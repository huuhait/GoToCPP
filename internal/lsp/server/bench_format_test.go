package server

import (
	"context"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/bench"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// BenchmarkFormatBig is the work of the format-1k row of `make lsp-bench`
// without the protocol: the formatting of a 1,000 line file whose
// indentation is gone, as edits.
func BenchmarkFormatBig(b *testing.B) {
	p := bench.Generate(bench.Small)
	good := p.Source(p.Big)
	messy := strings.ReplaceAll(good, "\n\t", "\n")
	doc := &document{path: "/x/big.gp", src: []byte(messy), mapper: protocol.NewMapper(messy)}
	ctx := context.Background()
	edits, err := formatEdits(ctx, doc)
	if err != nil || len(edits) == 0 {
		b.Fatalf("no edits: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := formatEdits(ctx, doc); err != nil {
			b.Fatal(err)
		}
	}
}
