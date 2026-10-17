package server

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/debounce"
)

// oneBigFile returns a module with one file of about 200*n lines (the
// generated code of bigPackage).
func oneBigFile(n int) map[string]string {
	var b strings.Builder
	b.WriteString("package big\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\n")
	for i := range n {
		fmt.Fprintf(&b, "// T%d is a generated type.\ntype T%d struct {\n\tName  string\n\tCount int\n\tTags  []string\n}\n\n", i, i)
		fmt.Fprintf(&b, "func (t T%d) String() string { return fmt.Sprintf(\"%%s:%%d\", t.Name, t.Count) }\n\n", i)
		for j := range 8 {
			fmt.Fprintf(&b, "// F%d_%d is a generated function.\nfunc F%d_%d(n int) int {\n", i, j, i, j)
			b.WriteString("\ttotal := 0\n\tparts := []string{}\n")
			b.WriteString("\tfor k := 0; k < n; k++ {\n\t\tif k%2 == 0 {\n\t\t\ttotal += k\n\t\t} else {\n\t\t\ttotal -= k / 2\n\t\t}\n")
			b.WriteString("\t\tparts = append(parts, fmt.Sprint(k))\n\t}\n")
			fmt.Fprintf(&b, "\tt := T%d{Name: strings.Join(parts, \",\"), Count: total}\n", i)
			b.WriteString("\tif strings.HasPrefix(t.String(), \"x\") {\n\t\treturn -1\n\t}\n")
			b.WriteString("\treturn total // EDIT\n}\n\n")
		}
	}
	return map[string]string{"gocpp.mod": "module example.com/mem\n", "big/big.gp": b.String()}
}

// heapAfterGC returns the live heap after a collection, in bytes.
func heapAfterGC() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// TestMemoryOutlineRequests: a document that changes and is asked for its
// outline, workspace symbols, semantic tokens, hints and links after every
// edit (an editor does that at each keystroke) does not make the server grow:
// what the requests keep is the symbols of the newest text of each file, not
// its parse. (The first version of the symbol table held every parsed file it
// had seen, about 20 MB for each edit of a 10,000-line file.)
func TestMemoryOutlineRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("takes a minute")
	}
	files := oneBigFile(12) // about 2,500 lines
	root := writeTree(t, files)
	c := &dclient{client: startServer(t, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
	c.skipShown = true
	c.call("initialize", fmt.Sprintf(`{"processId":1,"rootUri":%q,"capabilities":{}}`, fileURI(root)))
	c.notify("initialized", "{}")
	name := "big/big.gp"
	text := files[name]
	c.open(name, text)
	rng := `"range":{"start":{"line":0,"character":0},"end":{"line":100000,"character":0}}`
	requests := []string{
		`textDocument/documentSymbol|{"textDocument":{"uri":%q}}`,
		`workspace/symbol|{"query":"zz"}`,
		`textDocument/semanticTokens/full|{"textDocument":{"uri":%q}}`,
		`textDocument/inlayHint|{"textDocument":{"uri":%q},` + rng + `}`,
		`textDocument/documentLink|{"textDocument":{"uri":%q}}`,
	}
	const first, last = 30, 180
	var base, mid uint64
	for i := 1; i <= last; i++ {
		text += fmt.Sprintf("\nvar zz%d = %d\n", i, i)
		c.change(name, i+1, text)
		for _, r := range requests {
			method, params, _ := strings.Cut(r, "|")
			if strings.Contains(params, "%q") {
				params = fmt.Sprintf(params, c.uri(name))
			}
			if resp := c.call(method, params); resp.Error != nil {
				t.Fatalf("%s: %v", method, resp.Error)
			}
		}
		switch i {
		case first:
			base = heapAfterGC()
		case (first + last) / 2:
			mid = heapAfterGC()
		}
	}
	end := heapAfterGC()
	grown, lateGrown := int64(end)-int64(base), int64(end)-int64(mid)
	t.Logf("live heap after %d edits: %d MB; %d MB more after %d more, %d MB of it in the last half", first, base>>20, grown>>20, last-first, lateGrown>>20)
	// The caches fill up to their caps first (the first half); then the heap is flat.
	if lateGrown > 30<<20 {
		t.Errorf("the live heap grew by %d MB in the last %d edits, want it flat", lateGrown>>20, (last-first)/2)
	}
	if grown > 120<<20 {
		t.Errorf("the live heap grew by %d MB in %d edits (each with five requests), want no growth", grown>>20, last-first)
	}
}
