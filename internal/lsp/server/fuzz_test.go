package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/bench"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// The random-edit differential test (Phase 11 step 11.12, README §2.5): a
// reference project gets thousands of random edits through didChange, the
// way an editor sends them: characters inserted and deleted, lines added
// and removed, identifiers renamed everywhere in a file, syntax broken and
// fixed, an exported signature changed, files saved, closed and opened,
// closed files changed on disk. After every batch, once the server has no
// more background work, what it answers from its incremental state
// (snapshots, memoized checks, API-hash short-circuits) must equal what a
// server that starts from nothing answers for the same files: the same
// diagnostics, document symbols, references, hovers, definitions and
// workspace symbols. A difference is a bug in the incremental invalidation.
//
// The test is deterministic: the edits come from a seeded generator and
// don't depend on what the server does (an edit is a few random numbers
// that are applied to whatever text the file has, so any subset of the
// edits is a sequence too, which is what the shrinking uses). It prints the
// seed and the edits, and cuts the failing sequence down to a small one.
//
//	GOCPP_LSP_FUZZ_SEED=7 GOCPP_LSP_FUZZ_EDITS=5000 go test -race -run TestRandomEdits ./internal/lsp/server
//
// GOCPP_LSP_FUZZ_SHRINK=0 turns the shrinking off, GOCPP_LSP_FUZZ_SHRINK_SECS=n gives it n seconds (40).

// A fuzzEdit is one step of a sequence. The numbers A and B and the text S
// are the random choices, and mean something for every text.
type fuzzEdit struct {
	Kind string // insert, delete, line+, line-, rename, break, undo, reset, api, api2, save, close, open, full, disk, create, remove, move, mod
	File int    // the index in fuzzFiles
	A, B uint32
	S    string
}

func (e fuzzEdit) String() string {
	return fmt.Sprintf("{%q, %d, %d, %d, %q}", e.Kind, e.File, e.A, e.B, e.S)
}

// The files the fuzzer works on: the first fuzzOpen are open at the start.
var (
	fuzzFiles = []string{"core/core", "pkg000/f01", "pkg001/f01", "pkg001/f02", "pkg002/f00", "cmd/app/main",
		"core/log", "pkg000/f00", "pkg000/f02", "pkg001/f00", "pkg002/f01", "pkg002/f02",
		// The extras, which the file edits create, delete and move: mid
		// re-exports a type and a function of pkg000 (an API change goes
		// through two levels to user); pkg003 has two names for one file
		// (a move); new is a folder that does not exist at first, and user3
		// imports it.
		"mid/mid", "user/user", "pkg003/p0", "pkg003/p1", "user/user2", "new/n0", "user/user3"}
	fuzzOpen = 6
)

// The indexes of the extras in fuzzFiles.
const (
	fzMid, fzUser, fzP0, fzP1, fzUser2, fzNew, fzUser3 = 12, 13, 14, 15, 16, 17, 18
)

var typeRx = regexp.MustCompile(`type (\w+) struct`)

// fuzzExtras returns the text of the extra files (see fuzzFiles).
func fuzzExtras(proj *bench.Project) map[string]string {
	mod := proj.Module
	typ := typeRx.FindStringSubmatch(proj.Source("pkg000/f01"))[1]
	p3 := "package pkg003\n\n// Name returns a name.\nfunc Name() string { return \"p\" }\n"
	return map[string]string{
		"mid/mid":    "package mid\n\nimport \"" + mod + "/pkg000\"\n\n// R is a type of pkg000.\ntype R = pkg000." + typ + "\n\n// New is a function of pkg000.\nvar New = pkg000.New" + typ + "\n",
		"user/user":  "package user\n\nimport \"" + mod + "/mid\"\n\n// Use makes an R.\nfunc Use() int {\n\tr := mid.New(\"x\")\n\tn, _ := r.Add(\"t\")\n\treturn n\n}\n",
		"pkg003/p0":  p3,
		"pkg003/p1":  p3,
		"user/user2": "package user\n\nimport \"" + mod + "/pkg003\"\n\n// Use2 calls pkg003.\nfunc Use2() string { return pkg003.Name() }\n",
		"new/n0":     "package newpkg\n\n// F is a function.\nfunc F() int { return 1 }\n",
		"user/user3": "package user\n\nimport \"" + mod + "/new\"\n\n// Use3 calls new.\nfunc Use3() int { return newpkg.F() }\n",
	}
}

// fuzzAbsent are the extras that are not on disk at first.
var fuzzAbsent = map[string]bool{"pkg003/p1": true, "new/n0": true}

// fuzzSpec is the project: three packages of three files, core, main and
// the long file (which the fuzzer doesn't open).
var fuzzSpec = bench.Spec{Name: "fuzz", Seed: 11, Packages: 3, Files: 3}

var fuzzAlphabet = []string{"a", "b", "x", "(", ")", "{", "}", "[", "]", "\"", ".", ",", ":=", "=", "+", " ", "\t", "\n", "_", "0", "9", "*", "&", "<-", "go ", "func ", "if ", "return "}

var fuzzLines = []string{"\tx := 1\n", "\treturn\n", "\tfmt.Println(1)\n", "\tx := \n", "}\n", "// a comment\n", "\tfor {\n", "\tvar z int = \"s\"\n",
	"func extra() {}\n", "type T2 struct{}\n", "\tr.Add(\"t\")\n", "\tcore.Label(\"x\")\n", "import \"os\"\n", "\tdefer r.mu.Unlock()\n", "\tgo func() {}()\n", "\tr.\n"}

var fuzzNames = []string{"x", "tag", "Label", "Add", "T", "v", "r", "core", "fmt", "NewBase", "sum", "it", "items", "Base", "String"}

// fuzzGenerate returns n random edits for the seed.
func fuzzGenerate(seed uint64, n int) []fuzzEdit {
	rng := rand.New(rand.NewPCG(seed, seed*2+1))
	pick := func(xs []string) string { return xs[rng.IntN(len(xs))] }
	var out []fuzzEdit
	for range n {
		e := fuzzEdit{A: rng.Uint32(), B: rng.Uint32()}
		file := rng.IntN(fuzzOpen) // mostly the open ones
		if rng.IntN(5) == 0 {
			file = rng.IntN(len(fuzzFiles))
		}
		e.File = file
		switch r := rng.IntN(100); {
		case r < 12:
			e.Kind, e.S = "insert", pick(fuzzAlphabet)
		case r < 18:
			e.Kind = "delete"
		case r < 23:
			e.Kind, e.S = "line+", pick(fuzzLines)
		case r < 26:
			e.Kind = "line-"
		case r < 30:
			e.Kind, e.S = "rename", pick(fuzzNames)
		case r < 34:
			e.Kind = "break"
		case r < 45:
			e.Kind = "undo"
		case r < 57:
			e.Kind = "reset"
		case r < 64:
			e.Kind, e.File = "api", 4
		case r < 71:
			e.Kind, e.File = "api2", 1
		case r < 76:
			e.Kind = "save"
		case r < 79:
			e.Kind = "close"
		case r < 82:
			e.Kind = "open"
		case r < 84:
			e.Kind, e.S = "full", pick(fuzzAlphabet)
		case r < 87:
			e.Kind, e.S, e.File = "disk", pick(fuzzAlphabet), fuzzOpen+rng.IntN(len(fuzzFiles)-fuzzOpen)
		case r < 92:
			e.Kind, e.File = "create", fzMid+rng.IntN(len(fuzzFiles)-fzMid)
		case r < 96:
			e.Kind, e.File = "remove", fuzzOpen+rng.IntN(len(fuzzFiles)-fuzzOpen)
		case r < 98:
			e.Kind, e.File = "move", fzP0+rng.IntN(2)
		default:
			e.Kind = "mod"
		}
		out = append(out, e)
	}
	return out
}

// A fuzzDoc is a file as the test model has it.
type fuzzDoc struct {
	open    bool
	text    string // the editor's text when open, else the text on disk
	version int32
	hist    []string // the texts before the last edits, for undo
}

// A fuzzRun is a server driven through a sequence of edits, and the model
// of what the editor and the disk have.
type fuzzRun struct {
	t    testing.TB
	c    *dclient
	docs []*fuzzDoc
	disk map[string]string // the text of every file of the project on disk, by slash name
	orig map[string]string // the text of the generated project: a file that is reset gets it back
	mod  string            // the text of gocpp.mod

	clean, broken int // the open files, over all the answers, with no diagnostics and with some
}

// newFuzzRun writes the project to a new folder and starts a server on it
// with the first fuzzOpen files open.
func newFuzzRun(t testing.TB, proj *bench.Project) *fuzzRun {
	r := &fuzzRun{t: t, disk: map[string]string{}, orig: map[string]string{}, mod: "module " + proj.Module + "\n"}
	files := map[string]string{"gocpp.mod": r.mod}
	for _, f := range proj.Files {
		if f.Kind == bench.Source {
			files[f.Rel+".gp"] = f.Content
			r.disk[f.Rel] = f.Content
			r.orig[f.Rel] = f.Content
		}
	}
	for rel, text := range fuzzExtras(proj) {
		r.orig[rel] = text
		if !fuzzAbsent[rel] {
			files[rel+".gp"] = text
			r.disk[rel] = text
		}
	}
	root := writeTree(t, files)
	r.c = &dclient{client: startServer(t, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
	r.c.skipShown = true
	uri := protocol.URIFromPath(root)
	r.c.call("initialize", fmt.Sprintf(`{"processId":1,"rootUri":%q,"workspaceFolders":[{"uri":%q,"name":"fuzz"}],"capabilities":{"textDocument":{"publishDiagnostics":{"relatedInformation":true},"documentSymbol":{"hierarchicalDocumentSymbolSupport":true},"hover":{"contentFormat":["markdown","plaintext"]}}}}`, uri, uri))
	r.c.notify("initialized", "{}")
	for i, rel := range fuzzFiles {
		d := &fuzzDoc{text: r.disk[rel]}
		r.docs = append(r.docs, d)
		if i < fuzzOpen {
			r.open(i)
		}
	}
	return r
}

func (r *fuzzRun) uri(i int) protocol.DocumentURI { return r.c.uri(fuzzFiles[i] + ".gp") }

func (r *fuzzRun) open(i int) {
	d := r.docs[i]
	d.open, d.text, d.hist = true, r.disk[fuzzFiles[i]], nil
	d.version++
	r.c.notify("textDocument/didOpen", didOpen(r.uri(i), int(d.version), d.text))
}

// change sends one didChange of the file i with the changes (applied in
// order, like the protocol says), and keeps the text.
func (r *fuzzRun) change(i int, keep bool, changes ...fuzzChange) {
	d := r.docs[i]
	if keep {
		d.hist = append(d.hist, d.text)
		if len(d.hist) > 20 {
			d.hist = d.hist[1:]
		}
	}
	var events []map[string]any
	for _, ch := range changes {
		if ch.full {
			events = append(events, map[string]any{"text": ch.text})
			d.text = ch.text
			continue
		}
		m := protocol.NewMapper(d.text)
		events = append(events, map[string]any{"range": protocol.Range{Start: m.Position(ch.start), End: m.Position(ch.end)}, "text": ch.text})
		d.text = d.text[:ch.start] + ch.text + d.text[ch.end:]
	}
	d.version++
	data, err := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": r.uri(i), "version": d.version}, "contentChanges": events})
	if err != nil {
		r.t.Fatal(err)
	}
	r.c.notify("textDocument/didChange", string(data))
}

type fuzzChange struct {
	start, end int
	text       string
	full       bool
}

var identRx = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// apply does the edit: on the model, and on the server.
func (r *fuzzRun) apply(e fuzzEdit) {
	d := r.docs[e.File]
	n := len(d.text)
	switch e.Kind {
	case "insert":
		if d.open {
			off := int(e.A % uint32(n+1))
			r.change(e.File, true, fuzzChange{start: off, end: off, text: e.S})
		}
	case "delete":
		if d.open && n > 0 {
			off := int(e.A % uint32(n))
			end := min(off+1+int(e.B%10), n)
			r.change(e.File, true, fuzzChange{start: off, end: end})
		}
	case "line+":
		if d.open {
			starts := lineStarts(d.text)
			off := starts[int(e.A%uint32(len(starts)))]
			r.change(e.File, true, fuzzChange{start: off, end: off, text: e.S})
		}
	case "line-":
		if d.open {
			starts := lineStarts(d.text)
			k := int(e.A % uint32(len(starts)))
			end := n
			if k+1 < len(starts) {
				end = starts[k+1]
			}
			if starts[k] < end {
				r.change(e.File, true, fuzzChange{start: starts[k], end: end})
			}
		}
	case "rename":
		if !d.open {
			return
		}
		locs := identRx.FindAllStringIndex(d.text, -1)
		if len(locs) == 0 {
			return
		}
		word := d.text[locs[int(e.A%uint32(len(locs)))][0]:locs[int(e.A%uint32(len(locs)))][1]]
		var changes []fuzzChange
		for i := len(locs) - 1; i >= 0; i-- { // from the end: the offsets before stay right
			if d.text[locs[i][0]:locs[i][1]] == word {
				changes = append(changes, fuzzChange{start: locs[i][0], end: locs[i][1], text: e.S})
			}
		}
		r.change(e.File, true, changes...)
	case "break":
		if !d.open {
			return
		}
		var at []int
		for i := range len(d.text) {
			if strings.IndexByte("{}()[]\"", d.text[i]) >= 0 {
				at = append(at, i)
			}
		}
		if len(at) == 0 {
			return
		}
		off := at[int(e.A%uint32(len(at)))]
		repl := ""
		if e.B%3 == 0 {
			repl = "{"
		}
		r.change(e.File, true, fuzzChange{start: off, end: off + 1, text: repl})
	case "undo":
		if d.open && len(d.hist) > 0 {
			prev := d.hist[len(d.hist)-1]
			d.hist = d.hist[:len(d.hist)-1]
			r.change(e.File, false, fuzzChange{start: 0, end: len(d.text), text: prev})
		}
	case "reset": // the generated text again: the edits and saves of the file, undone
		orig := r.orig[fuzzFiles[e.File]]
		if d.open && d.text != orig {
			r.change(e.File, true, fuzzChange{start: 0, end: len(d.text), text: orig})
		} else if !d.open && r.disk[fuzzFiles[e.File]] != orig {
			r.writeDisk(e.File, orig)
			d.text = orig
			r.c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":2}]}`, r.uri(e.File)))
		}
	case "api": // the signature of an exported function that other packages call
		// The file and its caller are open and have the generated text (the
		// edit is the interesting part, not the state the random edits
		// left the files in).
		for _, i := range []int{e.File, 5} {
			if !r.docs[i].open {
				r.open(i)
			}
		}
		const old, changed = "func Label(s string)", "func Label(s string, n int)"
		// Without syntax or type errors of their own, the signature is what
		// the importer's diagnostics depend on.
		pristine := func(i int) bool {
			text := r.docs[i].text
			return text == r.orig[fuzzFiles[i]] || i == 4 && strings.Replace(text, changed, old, 1) == r.orig[fuzzFiles[i]]
		}
		for _, i := range []int{4, 5} {
			if !pristine(i) {
				r.apply(fuzzEdit{Kind: "reset", File: i})
			}
		}
		if i := strings.Index(d.text, changed); i >= 0 {
			r.change(e.File, true, fuzzChange{start: i, end: i + len(changed), text: old})
		} else if i := strings.Index(d.text, old); i >= 0 {
			r.change(e.File, true, fuzzChange{start: i, end: i + len(old), text: changed})
		}
	case "save":
		if d.open {
			r.writeDisk(e.File, d.text)
			r.c.notify("textDocument/didSave", docParams(r.uri(e.File)))
		}
	case "close":
		if d.open && e.File >= 1 { // core stays open: the workspace has an anchor
			d.open, d.text, d.hist = false, r.disk[fuzzFiles[e.File]], nil
			r.c.notify("textDocument/didClose", docParams(r.uri(e.File)))
		}
	case "open":
		if _, ok := r.disk[fuzzFiles[e.File]]; !d.open && ok {
			r.open(e.File)
		}
	case "api2":
		// pkg000's function that mid re-exports, and user that calls it
		// through mid: the API changes two levels down.
		for _, i := range []int{e.File, fzUser, fzMid} {
			if _, ok := r.disk[fuzzFiles[i]]; ok && !r.docs[i].open {
				r.open(i)
			}
		}
		typ := typeRx.FindStringSubmatch(r.orig["pkg000/f01"])[1]
		old, changed := "func New"+typ+"(name string)", "func New"+typ+"(name string, n int)"
		for _, i := range []int{e.File, fzUser, fzMid} {
			text := r.docs[i].text
			if r.docs[i].open && text != r.orig[fuzzFiles[i]] && strings.Replace(text, changed, old, 1) != r.orig[fuzzFiles[i]] {
				r.apply(fuzzEdit{Kind: "reset", File: i})
			}
		}
		if !r.docs[e.File].open {
			return
		}
		d := r.docs[e.File]
		if i := strings.Index(d.text, changed); i >= 0 {
			r.change(e.File, true, fuzzChange{start: i, end: i + len(changed), text: old})
		} else if i := strings.Index(d.text, old); i >= 0 {
			r.change(e.File, true, fuzzChange{start: i, end: i + len(old), text: changed})
		}
	case "create": // a file appears on disk (a new folder too), reported by the file, by the folder alone, or by both
		rel := fuzzFiles[e.File]
		if _, ok := r.disk[rel]; ok {
			return
		}
		r.writeDisk(e.File, r.orig[rel])
		if !d.open {
			d.text = r.orig[rel]
		}
		r.watched(e, e.File, 1, 1)
	case "remove": // a file is deleted (and its folder, when it is empty)
		rel := fuzzFiles[e.File]
		if _, ok := r.disk[rel]; !ok {
			return
		}
		path := filepath.Join(r.c.root, filepath.FromSlash(rel+".gp"))
		if err := os.Remove(path); err != nil {
			r.t.Fatal(err)
		}
		delete(r.disk, rel)
		if !d.open { // an open document keeps its text
			d.text = ""
		}
		folder := 2 // the folder only changed
		if e.B%2 == 0 && os.Remove(filepath.Dir(path)) == nil {
			folder = 3
		}
		r.watched(e, e.File, 3, folder)
	case "move": // pkg003/p0 and p1 are one file with two names
		src, dst := e.File, fzP0+fzP1-e.File
		if _, ok := r.disk[fuzzFiles[src]]; !ok {
			return
		}
		if _, ok := r.disk[fuzzFiles[dst]]; ok {
			return
		}
		from := filepath.Join(r.c.root, filepath.FromSlash(fuzzFiles[src]+".gp"))
		to := filepath.Join(r.c.root, filepath.FromSlash(fuzzFiles[dst]+".gp"))
		if err := os.Rename(from, to); err != nil {
			r.t.Fatal(err)
		}
		r.disk[fuzzFiles[dst]] = r.disk[fuzzFiles[src]]
		delete(r.disk, fuzzFiles[src])
		if !r.docs[dst].open {
			r.docs[dst].text = r.disk[fuzzFiles[dst]]
		}
		if !d.open {
			d.text = ""
		}
		r.c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":3},{"uri":%q,"type":1}]}`, r.uri(src), r.uri(dst)))
	case "mod": // gocpp.mod is rewritten: with and without a comment, or with another module path (every import path changes)
		if e.A%2 == 0 {
			if strings.HasSuffix(r.mod, "// touched\n") {
				r.mod = strings.TrimSuffix(r.mod, "// touched\n")
			} else {
				r.mod += "// touched\n"
			}
		} else if strings.Contains(r.mod, "moved") {
			r.mod = strings.Replace(r.mod, "moved", "fuzz", 1)
		} else {
			r.mod = strings.Replace(r.mod, "fuzz", "moved", 1)
		}
		writeFile(r.t, r.c.root, "gocpp.mod", r.mod)
		r.c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":2}]}`, protocol.URIFromPath(filepath.Join(r.c.root, "gocpp.mod"))))
	case "full":
		if d.open {
			off := int(e.A % uint32(n+1))
			r.change(e.File, true, fuzzChange{full: true, text: d.text[:off] + e.S + d.text[off:]})
		}
	case "disk":
		if d.open {
			return
		}
		text := r.disk[fuzzFiles[e.File]]
		off := int(e.A % uint32(len(text)+1))
		if e.B%2 == 0 || off == len(text) {
			text = text[:off] + e.S + text[off:]
		} else {
			text = text[:off] + text[off+1:]
		}
		r.writeDisk(e.File, text)
		d.text = text
		r.c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":2}]}`, r.uri(e.File)))
	default:
		r.t.Fatalf("unknown edit %v", e)
	}
}

// watched reports the change of a file to the server the way the edit says
// (A mod 3): the file, the folder alone, or both. fileType and folderType
// are the LSP file change types of each.
func (r *fuzzRun) watched(e fuzzEdit, i int, fileType, folderType int) {
	file := fmt.Sprintf(`{"uri":%q,"type":%d}`, r.uri(i), fileType)
	folder := fmt.Sprintf(`{"uri":%q,"type":%d}`, protocol.URIFromPath(filepath.Dir(filepath.Join(r.c.root, filepath.FromSlash(fuzzFiles[i]+".gp")))), folderType)
	switch e.A % 3 {
	case 0:
		r.c.notify("workspace/didChangeWatchedFiles", `{"changes":[`+file+`]}`)
	case 1:
		r.c.notify("workspace/didChangeWatchedFiles", `{"changes":[`+folder+`]}`)
	default:
		r.c.notify("workspace/didChangeWatchedFiles", `{"changes":[`+file+`,`+folder+`]}`)
	}
}

func (r *fuzzRun) writeDisk(i int, text string) {
	r.disk[fuzzFiles[i]] = text
	writeFile(r.t, r.c.root, fuzzFiles[i]+".gp", text)
}

func lineStarts(text string) []int {
	starts := []int{0}
	for i := range len(text) {
		if text[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// quiet waits until the server has no more background work: no run of the
// diagnostics is waiting or running, and nothing was published for a short
// while.
//
// publishedAll waits until every open document has diagnostics for its
// version (a server that has had a read fail under pressure, like a machine
// out of file descriptors, may not have published yet), and opens a document
// again that still has none after a while. It looks at the open documents
// only: a publish for a file that is not open is not waited for.
func (r *fuzzRun) publishedAll() {
	r.t.Helper()
	missing := func() []int {
		r.c.pubMu.Lock()
		defer r.c.pubMu.Unlock()
		var out []int
		for i, d := range r.docs {
			if !d.open {
				continue
			}
			ok := false
			for _, p := range r.c.publishes {
				ok = ok || p.URI == r.uri(i) && p.Version == d.version
			}
			if !ok {
				out = append(out, i)
			}
		}
		return out
	}
	for round := 0; round < 4; round++ {
		for start := time.Now(); time.Since(start) < 5*time.Second; time.Sleep(10 * time.Millisecond) {
			if len(missing()) == 0 {
				return
			}
		}
		for _, i := range missing() {
			r.docs[i].version++
			r.c.notify("textDocument/didOpen", didOpen(r.uri(i), int(r.docs[i].version), r.docs[i].text))
		}
		r.quiet()
	}
}

func (r *fuzzRun) quiet() {
	r.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	count := func() int {
		r.c.pubMu.Lock()
		defer r.c.pubMu.Unlock()
		return len(r.c.publishes)
	}
	for {
		stable := 0
		last := -1
		for stable < 3 {
			if time.Now().After(deadline) {
				r.t.Fatalf("the server has background work after 30 s: %v", r.c.s.diags.deb.Keys())
			}
			time.Sleep(4 * time.Millisecond)
			n := count()
			if len(r.c.s.diags.deb.Keys()) == 0 && n == last {
				stable++
			} else {
				stable = 0
			}
			last = n
		}
		// The server is idle, but the client's reader may not have decoded
		// what it wrote last (a big publish, on a starved machine). The reader
		// handles the messages in order, so once the answer to a request has
		// come, everything written before it has been taken in.
		if resp := r.c.call(GocppStats, ""); resp.Error != nil {
			r.t.Fatalf("%s: %v", GocppStats, resp.Error)
		}
		if len(r.c.s.diags.deb.Keys()) == 0 && count() == last {
			return
		}
	}
}

// answers returns what the server says about the files that are open: the
// last diagnostics of each, and the answers to the requests at places that
// are found in the text. Absolute names are replaced by ROOT.
func (r *fuzzRun) answers() map[string]string {
	r.t.Helper()
	out := map[string]string{}
	norm := func(s string) string { return strings.ReplaceAll(s, r.c.root, "ROOT") }
	r.c.pubMu.Lock()
	last := map[protocol.DocumentURI]*publish{}
	for _, p := range r.c.publishes {
		last[p.URI] = p
	}
	r.c.pubMu.Unlock()
	// The diagnostics of every file that has some (the files of the open
	// packages that are not open too), and of every open file.
	diagOf := func(p *publish) string {
		var lines []string
		if p != nil {
			for _, x := range p.Diagnostics {
				lines = append(lines, norm(fmt.Sprintf("%d:%d-%d:%d s%d %s", x.Range.Start.Line, x.Range.Start.Character, x.Range.End.Line, x.Range.End.Character, x.Severity, x.Message)))
				for _, ri := range x.RelatedInformation {
					lines = append(lines, norm(fmt.Sprintf("  related %s %d:%d %s", ri.Location.URI, ri.Location.Range.Start.Line, ri.Location.Range.Start.Character, ri.Message)))
				}
			}
		}
		return strings.Join(lines, "\n")
	}
	for uri, p := range last {
		if text := diagOf(p); text != "" {
			out["diagnostics "+norm(string(uri))] = text
		}
	}
	for i, rel := range fuzzFiles {
		d := r.docs[i]
		if !d.open {
			continue
		}
		text := diagOf(last[r.uri(i)])
		out["diagnostics "+norm(string(r.uri(i)))] = text
		// What the server has of the document is what the editor sent, and
		// its last diagnostics are for that version.
		if o := r.c.overlay(r.uri(i)); o == nil || o.Text() != d.text || o.Version() != d.version {
			out["overlay "+rel] = "the server's copy of the document is not the editor's"
		}
		if p := last[r.uri(i)]; p == nil || p.Version != d.version {
			v := int32(-1)
			if p != nil {
				v = p.Version
			}
			out["publish version "+rel] = fmt.Sprintf("the last diagnostics are for version %d, the document is at version %d", v, d.version)
		}
		if text == "" {
			r.clean++
		} else {
			r.broken++
		}
		resp := r.c.call("textDocument/documentSymbol", docParams(r.uri(i)))
		out["symbols "+rel] = norm(string(resp.Result)) + r.shown(norm)
		for _, m := range []struct{ method, needle string }{
			{"textDocument/hover", "core.NewBase("},
			{"textDocument/definition", "core.NewBase("},
			{"textDocument/definition", "it.Add("},
			{"textDocument/references", "\ntype "},
			{"textDocument/references", "type Base struct"},
			{"textDocument/references", "func Label("},
			{"textDocument/references", ") Add(tag string)"},
			{"textDocument/hover", "r.mu.Lock()"},
		} {
			off := strings.Index(d.text, m.needle)
			if off < 0 {
				continue
			}
			off += 6
			if off > len(d.text) {
				continue
			}
			pos := protocol.NewMapper(d.text).Position(off)
			params := fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d},"context":{"includeDeclaration":true}}`, r.uri(i), pos.Line, pos.Character)
			resp := r.c.call(m.method, params)
			res := norm(string(resp.Result))
			if m.method == "textDocument/references" {
				res = sortedLocations(res)
			}
			if resp.Error != nil {
				res = "error " + resp.Error.Message
			}
			res += r.shown(norm)
			out[fmt.Sprintf("%s %s %q", m.method, rel, m.needle)] = res
		}
	}
	resp := r.c.call("workspace/symbol", `{"query":"New"}`)
	out["workspace/symbol New"] = norm(string(resp.Result))
	return out
}

// shown returns the messages that the server showed to the user while it
// answered (a window/showMessage: "the results are incomplete"), as text.
func (r *fuzzRun) shown(norm func(string) string) string {
	var b strings.Builder
	for _, m := range r.c.takeShown() {
		fmt.Fprintf(&b, "\n[shown %d] %s", m.Type, norm(m.Message))
	}
	return b.String()
}

// sortedLocations returns the locations of a references answer in a fixed
// order (the order of an answer is not part of what it says).
func sortedLocations(raw string) string {
	var locs []json.RawMessage
	if json.Unmarshal([]byte(raw), &locs) != nil {
		return raw
	}
	var ss []string
	for _, l := range locs {
		ss = append(ss, string(l))
	}
	sort.Strings(ss)
	return strings.Join(ss, "\n")
}

// fuzzDiff compares the answers of the incremental server with the ones of
// a server that starts from nothing for the same disk and the same open
// files, and returns the differences.
func fuzzDiff(t testing.TB, r *fuzzRun) []string { return fuzzDiffWait(t, r, 5*time.Second) }

// fuzzDiffWait is fuzzDiff with the time that an answer may take to come right.
func fuzzDiffWait(t testing.TB, r *fuzzRun, patience time.Duration) []string {
	t.Helper()
	r.quiet()
	got := r.answers()
	// The fresh server: the files as they are on disk, the same files open
	// with the same text, opened in the same order.
	fresh := &fuzzRun{t: t, disk: maps2(r.disk), orig: r.orig, mod: r.mod}
	files := map[string]string{"gocpp.mod": r.mod}
	for rel, text := range r.disk {
		files[rel+".gp"] = text
	}
	root := writeTree(t, files)
	// The folders that are empty now (a file was deleted, its folder stayed)
	// exist in the incremental server's tree, so they exist in this one.
	filepath.WalkDir(r.c.root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			rel, _ := filepath.Rel(r.c.root, path)
			os.MkdirAll(filepath.Join(root, rel), 0o755)
		}
		return nil
	})
	fresh.c = &dclient{client: startServer(t, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
	fresh.c.skipShown = true
	uri := protocol.URIFromPath(root)
	fresh.c.call("initialize", fmt.Sprintf(`{"processId":1,"rootUri":%q,"workspaceFolders":[{"uri":%q,"name":"fuzz"}],"capabilities":{"textDocument":{"publishDiagnostics":{"relatedInformation":true},"documentSymbol":{"hierarchicalDocumentSymbolSupport":true},"hover":{"contentFormat":["markdown","plaintext"]}}}}`, uri, uri))
	fresh.c.notify("initialized", "{}")
	for i := range fuzzFiles {
		d := &fuzzDoc{text: r.docs[i].text, open: r.docs[i].open, version: 1}
		fresh.docs = append(fresh.docs, d)
		if d.open {
			fresh.c.notify("textDocument/didOpen", didOpen(fresh.uri(i), 1, d.text))
		}
	}
	fresh.quiet()
	fresh.publishedAll()
	want := fresh.answers()
	fresh.c.call("shutdown", "")
	fresh.c.notify("exit", "")
	fresh.c.wait()
	diffs := diffAnswers(got, want)
	// An answer that is different when the server looks idle but right a
	// moment later is not a stale result (the plan: "once background work
	// has finished"); one that stays different is.
	for start := time.Now(); len(diffs) > 0 && time.Since(start) < patience; {
		time.Sleep(50 * time.Millisecond)
		r.quiet()
		if diffs = diffAnswers(r.answers(), want); len(diffs) == 0 {
			late := time.Since(start).Round(time.Millisecond)
			if late > 2*time.Second {
				diffs = []string{fmt.Sprintf("the incremental server needed %v after it looked idle to have the answers of a fresh one", late)}
			} else {
				t.Logf("the incremental server needed %v more to have the answers of a fresh one", late)
			}
			break
		}
	}
	return diffs
}

// diffAnswers describes the answers that differ.
func diffAnswers(got, want map[string]string) []string {
	var diffs []string
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)
	for _, k := range keys {
		if got[k] != want[k] {
			diffs = append(diffs, fmt.Sprintf("%s:\n  incremental: %s\n  from scratch: %s", k, clip(got[k]), clip(want[k])))
		}
	}
	return diffs
}

func maps2(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func clip(s string) string {
	s = strings.ReplaceAll(s, "\n", "\n    ")
	if len(s) > 1500 {
		return s[:1500] + "..."
	}
	return s
}

// fuzzEnvInt reads an integer from the environment.
func fuzzEnvInt(name string, def int) int {
	if s := os.Getenv(name); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return def
}

// replayFails runs the edits on a new server and compares once at the end.
func replayFails(t testing.TB, proj *bench.Project, edits []fuzzEdit, every int) bool {
	r := newFuzzRun(t, proj)
	defer func() {
		r.c.call("shutdown", "")
		r.c.notify("exit", "")
		r.c.wait()
	}()
	for i, e := range edits {
		r.apply(e)
		// The server is left to finish at the same kinds of points as in
		// the test (a stale result needs a result to be stale).
		if fuzzInvalidates(e.Kind) || (i+1)%every == 0 {
			r.quiet()
			r.answers() // the requests of a comparison fill the caches too
		}
	}
	return len(fuzzDiffWait(t, r, 400*time.Millisecond)) > 0
}

// fuzzInvalidates reports whether an edit of the kind changes what other
// files see: the test compares with a fresh server right after one.
func fuzzInvalidates(kind string) bool {
	switch kind {
	case "api", "api2", "save", "disk", "close", "open", "create", "remove", "move", "mod":
		return true
	}
	return false
}

// shrink cuts the edits down to a smaller sequence that still makes the
// servers differ (delta debugging), within the time limit.
func shrink(t testing.TB, proj *bench.Project, edits []fuzzEdit, every int, limit time.Duration) []fuzzEdit {
	deadline := time.Now().Add(limit)
	fails := func(es []fuzzEdit) bool { return time.Now().Before(deadline) && replayFails(t, proj, es, every) }
	n := 2
	for len(edits) >= 2 && time.Now().Before(deadline) {
		chunk := (len(edits) + n - 1) / n
		reduced := false
		for i := 0; i < len(edits); i += chunk {
			rest := slices.Concat(edits[:i], edits[min(i+chunk, len(edits)):])
			if len(rest) < len(edits) && fails(rest) {
				edits, n, reduced = rest, max(n-1, 2), true
				break
			}
		}
		if !reduced {
			if n >= len(edits) {
				break
			}
			n = min(2*n, len(edits))
		}
	}
	return edits
}

func TestRandomEdits(t *testing.T) {
	checkGoroutines(t)
	seed := uint64(fuzzEnvInt("GOCPP_LSP_FUZZ_SEED", 1))
	count := fuzzEnvInt("GOCPP_LSP_FUZZ_EDITS", 400)
	every := fuzzEnvInt("GOCPP_LSP_FUZZ_BATCH", 20)
	if raceEnabled && os.Getenv("GOCPP_LSP_FUZZ_EDITS") == "" {
		count, every = 150, 25
	}
	if testing.Short() && os.Getenv("GOCPP_LSP_FUZZ_EDITS") == "" {
		count = 60
	}
	proj := bench.Generate(fuzzSpec)
	edits := fuzzGenerate(seed, count)
	// The same seed gives the same edits.
	if again := fuzzGenerate(seed, count); !slices.Equal(edits, again) {
		t.Fatal("the generator is not deterministic")
	}
	r := newFuzzRun(t, proj)
	defer func() {
		r.c.call("shutdown", "")
		r.c.notify("exit", "")
		r.c.wait()
	}()
	r.quiet()
	if diffs := fuzzDiff(t, r); len(diffs) > 0 {
		t.Fatalf("before any edit the servers differ:\n%s", strings.Join(diffs, "\n"))
	}
	start := time.Now()
	compared := 0
	for i, e := range edits {
		r.apply(e)
		// After an edit that invalidates other files (the signature of an
		// exported function, a save, a change on disk, a close or open),
		// at once: a later edit of an importer would heal a stale one.
		if !fuzzInvalidates(e.Kind) && (i+1)%every != 0 && i+1 != len(edits) {
			continue
		}
		compared++
		diffs := fuzzDiff(t, r)
		if len(diffs) == 0 {
			continue
		}
		prefix := edits[:i+1]
		var report strings.Builder
		shownDiffs := diffs[:min(len(diffs), 8)]
		fmt.Fprintf(&report, "seed %d: after %d edits the incremental server differs from a server that starts from scratch (%d answers differ):\n%s\n", seed, i+1, len(diffs), strings.Join(shownDiffs, "\n"))
		if os.Getenv("GOCPP_LSP_FUZZ_SHRINK") != "0" {
			repro := replayFails(t, proj, prefix, every)
			if !repro {
				report.WriteString("\nthe difference did not repeat in a new run of the same edits: it depends on timing\n")
			} else {
				small := shrink(t, proj, prefix, every, time.Duration(fuzzEnvInt("GOCPP_LSP_FUZZ_SHRINK_SECS", 40))*time.Second)
				fmt.Fprintf(&report, "\nthe shortest sequence found (%d of %d edits; fuzzEdit{Kind, File, A, B, S}; files: %v):\n", len(small), len(prefix), fuzzFiles)
				for _, e := range small {
					fmt.Fprintf(&report, "\t%v,\n", e)
				}
			}
		} else {
			fmt.Fprintf(&report, "\nthe edits:\n")
			for _, e := range prefix {
				fmt.Fprintf(&report, "\t%v,\n", e)
			}
		}
		t.Fatal(report.String())
	}
	t.Logf("seed %d: %d edits, %d comparisons with a fresh server, %v; of the open files compared, %d had no diagnostics and %d had some", seed, len(edits), compared, time.Since(start).Round(time.Millisecond), r.clean, r.broken)
	if count >= 300 && (r.clean < compared || r.broken < compared) {
		t.Errorf("the edits don't reach both kinds of state: %d clean files, %d with diagnostics in %d comparisons", r.clean, r.broken, compared)
	}
	kinds := map[string]int{}
	for _, e := range edits {
		kinds[e.Kind]++
	}
	if count >= 300 {
		for _, k := range []string{"insert", "delete", "line+", "line-", "rename", "break", "undo", "reset", "api", "api2", "save", "close", "open", "full", "disk", "create", "remove", "move", "mod"} {
			if kinds[k] == 0 {
				t.Errorf("no edit of the kind %s in %d edits", k, count)
			}
		}
	}
	_ = filepath.Join
}

// TestFuzzAPISanity: the edits do what the fuzzer thinks they do, and the
// comparison sees what they cause. Changing the signature of Label in an
// open package shows an error in the open file of main that calls it, and
// changing it back takes the error away; a main that was not told (the
// importers' diagnostics are not run) differs from the fresh server.
func TestFuzzAPISanity(t *testing.T) {
	proj := bench.Generate(fuzzSpec)
	r := newFuzzRun(t, proj)
	defer func() {
		r.c.call("shutdown", "")
		r.c.notify("exit", "")
		r.c.wait()
	}()
	r.quiet()
	main := r.answers()["diagnostics file://ROOT/cmd/app/main.gp"]
	if main != "" {
		t.Fatalf("main has diagnostics before the edit:\n%s", main)
	}
	r.apply(fuzzEdit{Kind: "api", File: 4})
	r.quiet()
	if got := r.answers()["diagnostics file://ROOT/cmd/app/main.gp"]; !strings.Contains(got, "not enough arguments in call to pkg002.Label") {
		t.Errorf("after the signature change, main has\n%s", got)
	}
	if diffs := fuzzDiff(t, r); len(diffs) > 0 {
		t.Errorf("incremental and fresh differ: %v", diffs)
	}
	r.apply(fuzzEdit{Kind: "api", File: 4})
	r.quiet()
	if got := r.answers()["diagnostics file://ROOT/cmd/app/main.gp"]; got != "" {
		t.Errorf("after the signature is back, main has\n%s", got)
	}
	// Two levels: pkg000 changes a function that mid re-exports and user calls.
	userKey := "diagnostics file://ROOT/user/user.gp"
	r.apply(fuzzEdit{Kind: "create", File: fzNew}) // user3 imports it: without it the package of user has an error that hides the call's
	r.apply(fuzzEdit{Kind: "api2", File: 1})
	r.quiet()
	if got := r.answers()[userKey]; !strings.Contains(got, "not enough arguments in call to mid.New") {
		t.Errorf("after the change in pkg000, user has\n%s", got)
	}
	if diffs := fuzzDiff(t, r); len(diffs) > 0 {
		t.Errorf("incremental and fresh differ: %v", diffs)
	}
	r.apply(fuzzEdit{Kind: "api2", File: 1})
	r.quiet()
	if got := r.answers()[userKey]; got != "" {
		t.Errorf("after the change is undone, user has\n%s", got)
	}
}

// TestFuzzTornWrite: a file that is read while a tool is writing it (empty
// for a moment: a checkout, a generator) and then written whole, with the
// change reported afterwards, ends up with the answers of a server that
// never saw the empty file.
func TestFuzzTornWrite(t *testing.T) {
	proj := bench.Generate(fuzzSpec)
	r := newFuzzRun(t, proj)
	defer func() {
		r.c.call("shutdown", "")
		r.c.notify("exit", "")
		r.c.wait()
	}()
	r.quiet()
	for round := range 3 {
		// pkg002/f01 is not open; main (open) imports pkg002.
		const target = 10
		whole := r.disk[fuzzFiles[target]]
		writeFile(t, r.c.root, fuzzFiles[target]+".gp", "") // the tool has truncated the file
		// The editor changes main: the server checks it, and reads pkg002
		// from disk as it is now.
		off := len(r.docs[5].text)
		r.change(5, true, fuzzChange{start: off, end: off, text: fmt.Sprintf("// round %d\n", round)})
		r.quiet()
		writeFile(t, r.c.root, fuzzFiles[target]+".gp", whole) // the tool has written it
		r.c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":2}]}`, r.uri(target)))
		if diffs := fuzzDiff(t, r); len(diffs) > 0 {
			t.Fatalf("round %d: after the file was written whole, the servers differ:\n%s", round, strings.Join(diffs, "\n"))
		}
	}
}

// TestFuzzUnreadableHeals: a read that fails once (a file that is not
// readable for a moment: no file descriptors left, a tool that has not
// finished) is no fact about the file. The failure is seen, the file is
// readable again without any report, and the next change of the importer
// makes the server right: a read that failed is read again, and what was
// made from it is made again, in the importers too.
func TestFuzzUnreadableHeals(t *testing.T) {
	proj := bench.Generate(fuzzSpec)
	r := newFuzzRun(t, proj)
	defer func() {
		r.c.call("shutdown", "")
		r.c.notify("exit", "")
		r.c.wait()
	}()
	const target = 10 // pkg002/f01, not open; main imports pkg002
	path := filepath.Join(r.c.root, "pkg002", "f01.gp")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(path); err == nil {
		os.Chmod(path, 0o644)
		t.Skip("the file is readable with mode 0 (root)")
	}
	r.quiet()
	r.c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":2}]}`, r.uri(target)))
	r.quiet()
	// The server has seen the failure: pkg002 is broken for it.
	if got := r.answers()["diagnostics "+strings.ReplaceAll(string(r.uri(5)), r.c.root, "ROOT")]; got == "" {
		t.Logf("main shows nothing while pkg002/f01 is unreadable (the failure may have been missed)")
	}
	if err := os.Chmod(path, 0o644); err != nil { // readable again, and nobody says so
		t.Fatal(err)
	}
	for round := range 3 {
		off := len(r.docs[5].text)
		r.change(5, true, fuzzChange{start: off, end: off, text: fmt.Sprintf("// round %d\n", round)})
		r.quiet()
	}
	if diffs := fuzzDiffWait(t, r, 0); len(diffs) > 0 {
		t.Fatalf("after the file was readable again and main changed, the servers differ:\n%s", strings.Join(diffs, "\n"))
	}
}

// fuzzRegressions are sequences that the random edits found (shortened by
// shrink), run again on every test.
var fuzzRegressions = map[string][]fuzzEdit{
	"seed 3: the references of pkg002 after main was saved and closed": {
		{"line-", 5, 2172932608, 3970374156, ""},
		{"remove", 16, 4185700295, 3881500679, ""},
		{"save", 5, 2157188486, 714729489, ""},
		{"create", 16, 3797645095, 4032379753, ""},
		{"api", 4, 2526346561, 455232558, ""},
		{"save", 5, 4089767137, 60288746, ""},
		{"close", 5, 2819467563, 3304102574, ""},
	},
}

func init() {
	fuzzRegressions["seed 2: workspace symbols of an open file whose folder was reported alone after the file was deleted"] = []fuzzEdit{
		{"api2", 1, 2590281337, 3420353540, ""},
		{"remove", 12, 2245628995, 2643224685, ""},
	}
}

func init() {
	// A module path change (A odd) changes the import path of every package.
	fuzzRegressions["the module path of gocpp.mod changes and comes back"] = []fuzzEdit{
		{"mod", 0, 1, 0, ""},
		{"mod", 0, 1, 0, ""},
	}
	fuzzRegressions["the module path changes while main is open"] = []fuzzEdit{
		{"mod", 0, 1, 0, ""},
	}
}

func TestFuzzRegressions(t *testing.T) {
	proj := bench.Generate(fuzzSpec)
	for name, edits := range fuzzRegressions {
		t.Run(name, func(t *testing.T) {
			if replayFails(t, proj, edits, 20) {
				t.Errorf("the servers differ after %d edits", len(edits))
			}
		})
	}
}
