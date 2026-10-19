package server

import (
	"cmp"
	"context"
	"encoding/json"
	"go/ast"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/lsp/safe"
)

// textDocument/documentSymbol and workspace/symbol (step 11.8).
//
// The symbols of a file come from its syntax tree alone (the parse cache,
// cache.Snapshot.ParsePath), so they are there while the file has errors, and
// cost no type check: the declarations of the package level (functions,
// methods, types with their fields or interface methods, constants and
// variables), as gopls shows them. The workspace symbols are those of every
// .gp file of the packages of the workspace: the modules of the workspace
// folders and of the open documents (the folders without a gocpp.mod too: all
// the .gp files under them). Each file's symbols are kept by its parsed file,
// so the index is the parse cache plus a small table, and a change of a file
// replaces only its own symbols. It is built in the background after
// initialized and when a document is opened, so the first query is not the one
// that parses the workspace.
//
// A query is matched like gopls's fuzzy matcher, approximately: every
// character of the query must appear in the name in order, ignoring case, and
// names that match better come first (the whole name, a prefix, a substring,
// camel-case or word starts, few gaps); at most maxWorkspaceSymbols come back.

// maxWorkspaceSymbols is the number of results of a workspace/symbol, at
// most (gopls's limit).
const maxWorkspaceSymbols = 100

// symbolFeature holds what the symbol requests learned from initialize.
type symbolFeature struct {
	hierarchical atomic.Bool
	warming      atomic.Bool // the background index is being built

	mu    sync.Mutex
	roots []string // the workspace folders

	// files holds the symbols of each file, one entry per file name, for the
	// content that they were made from. The table holds only the symbols, never
	// the parsed file (its tree, text and file set are big, and the parse cache
	// has a memory cap that a pointer here would defeat).
	filesMu sync.Mutex
	files   map[string]symbolEntry
}

// A symbolEntry is the symbols of one file for the content with the hash.
type symbolEntry struct {
	hash cache.Hash
	syms []symNode
	// flat are the workspace symbols of the file (made on the first query
	// that needs them): without the package, which is the query's.
	flat []wsSymbol
}

// maxSymbolFiles is the number of files whose symbols are kept before the table
// starts over (the files of a workspace: more than that are not indexed whole).
const maxSymbolFiles = 100000

func (sf *symbolFeature) initialize(p *protocol.ParamInitialize, caps *protocol.ServerCapabilities) {
	sf.hierarchical.Store(p.Capabilities.TextDocument.DocumentSymbol.HierarchicalDocumentSymbolSupport)
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.roots = nil
	for _, f := range p.WorkspaceFolders {
		if path := protocol.DocumentURI(f.URI).Path(); path != "" {
			sf.roots = append(sf.roots, filepath.Clean(path))
		}
	}
	if len(sf.roots) == 0 && p.RootURI != "" {
		if path := p.RootURI.Path(); path != "" {
			sf.roots = append(sf.roots, filepath.Clean(path))
		}
	}
	caps.DocumentSymbolProvider = &protocol.Or_ServerCapabilities_documentSymbolProvider{Value: true}
	caps.WorkspaceSymbolProvider = &protocol.Or_ServerCapabilities_workspaceSymbolProvider{Value: true}
}

func (s *Server) addSymbols() {
	s.requests["textDocument/documentSymbol"] = method{run: s.documentSymbol}
	s.requests["workspace/symbol"] = method{run: s.workspaceSymbol}
}

// A symNode is a symbol of a file: its name, where it is in the file (byte
// offsets) and its members.
type symNode struct {
	name   string
	detail string
	kind   protocol.SymbolKind
	// start and end are the declaration; selStart and selEnd the name.
	start, end       int
	selStart, selEnd int
	children         []symNode
}

// fileSymbols returns the symbols of the parsed file: its declarations of the
// package level. A node with errors still gives what the parser kept.
func fileSymbols(pf *cache.ParsedFile) []symNode {
	if pf == nil || pf.File == nil {
		return nil
	}
	var out []symNode
	off := func(p token.Pos) int { return pf.Tok.Offset(p) }
	node := func(name, detail string, kind protocol.SymbolKind, whole ast.Node, id *ast.Ident) symNode {
		return symNode{name: name, detail: detail, kind: kind,
			start: off(whole.Pos()), end: off(whole.End()), selStart: off(id.Pos()), selEnd: off(id.End())}
	}
	text := func(n ast.Node) string { return nodeText(pf.Fset, n) }
	for _, d := range pf.File.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Name == nil || !d.Name.Pos().IsValid() {
				continue
			}
			name, kind := d.Name.Name, protocol.Function
			if d.Recv != nil && len(d.Recv.List) > 0 {
				name = "(" + text(d.Recv.List[0].Type) + ")." + name
				kind = protocol.Method
			}
			out = append(out, node(name, text(d.Type), kind, d, d.Name))
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch spec := spec.(type) {
				case *ast.ValueSpec:
					kind := protocol.Variable
					if d.Tok == token.CONST {
						kind = protocol.Constant
					}
					detail := ""
					if spec.Type != nil {
						detail = text(spec.Type)
					}
					for _, id := range spec.Names {
						if id.Name != "_" && id.Pos().IsValid() {
							out = append(out, node(id.Name, detail, kind, spec, id))
						}
					}
				case *ast.TypeSpec:
					if spec.Name == nil || !spec.Name.Pos().IsValid() {
						continue
					}
					out = append(out, typeSymbol(pf, spec, node, text))
				}
			}
		}
	}
	return out
}

// typeSymbol returns the symbol of a type declaration, with the fields of a
// struct or the methods of an interface as its children.
func typeSymbol(pf *cache.ParsedFile, spec *ast.TypeSpec, node func(string, string, protocol.SymbolKind, ast.Node, *ast.Ident) symNode, text func(ast.Node) string) symNode {
	switch t := spec.Type.(type) {
	case *ast.StructType:
		n := node(spec.Name.Name, "struct", protocol.Struct, spec, spec.Name)
		if t.Fields != nil {
			for _, f := range t.Fields.List {
				if len(f.Names) == 0 {
					if id := embeddedName(f.Type); id != nil {
						n.children = append(n.children, node(id.Name, text(f.Type), protocol.Field, f, id))
					}
					continue
				}
				for _, id := range f.Names {
					if id.Name != "_" {
						n.children = append(n.children, node(id.Name, text(f.Type), protocol.Field, f, id))
					}
				}
			}
		}
		return n
	case *ast.InterfaceType:
		n := node(spec.Name.Name, "interface", protocol.Interface, spec, spec.Name)
		if t.Methods != nil {
			for _, f := range t.Methods.List {
				if len(f.Names) == 0 {
					if id := embeddedName(f.Type); id != nil {
						n.children = append(n.children, node(id.Name, text(f.Type), protocol.Interface, f, id))
					}
					continue
				}
				for _, id := range f.Names {
					n.children = append(n.children, node(id.Name, text(f.Type), protocol.Method, f, id))
				}
			}
		}
		return n
	case *ast.FuncType:
		return node(spec.Name.Name, text(spec.Type), protocol.Function, spec, spec.Name)
	}
	return node(spec.Name.Name, text(spec.Type), protocol.Class, spec, spec.Name)
}

// nodeText returns the source of n, gofmt'd, on one line.
func nodeText(fset *token.FileSet, n ast.Node) string {
	if n == nil {
		return ""
	}
	return strings.Join(strings.Fields(formatNode(fset, n)), " ")
}

// forget drops the symbols of the files that were deleted on disk (a deleted
// folder stands for everything in it).
func (sf *symbolFeature) forget(events []protocol.FileEvent) {
	sf.filesMu.Lock()
	defer sf.filesMu.Unlock()
	for _, e := range events {
		path := e.URI.Path()
		if e.Type != protocol.Deleted || path == "" {
			continue
		}
		delete(sf.files, path)
		prefix := path + string(filepath.Separator)
		for name := range sf.files {
			if strings.HasPrefix(name, prefix) {
				delete(sf.files, name)
			}
		}
	}
}

// symbolsOf returns the symbols of the parsed file, from the table when it has
// them for the same content.
func (sf *symbolFeature) symbolsOf(pf *cache.ParsedFile) []symNode {
	sf.filesMu.Lock()
	e, ok := sf.files[pf.Path]
	sf.filesMu.Unlock()
	if ok && e.hash == pf.Hash {
		return e.syms
	}
	syms := fileSymbols(pf)
	sf.filesMu.Lock()
	if sf.files == nil || len(sf.files) >= maxSymbolFiles {
		sf.files = make(map[string]symbolEntry)
	}
	sf.files[pf.Path] = symbolEntry{hash: pf.Hash, syms: syms}
	sf.filesMu.Unlock()
	return syms
}

// documentSymbol answers textDocument/documentSymbol.
func (s *Server) documentSymbol(ctx context.Context, e env, params json.RawMessage) (any, error) {
	var p protocol.DocumentSymbolParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err // a cached parse would answer anyway
	}
	pf, err := e.snapshot.ParseFile(ctx, p.TextDocument.URI, cache.ParseFull)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, protocol.Errorf(protocol.RequestFailed, "%v", err)
	}
	syms := s.symbols.symbolsOf(pf)
	m := protocol.NewMapper(string(pf.Src))
	rng := func(start, end int) protocol.Range {
		return protocol.Range{Start: m.Position(start), End: m.Position(end)}
	}
	if s.symbols.hierarchical.Load() {
		var conv func(list []symNode) []protocol.DocumentSymbol
		conv = func(list []symNode) []protocol.DocumentSymbol {
			out := make([]protocol.DocumentSymbol, 0, len(list))
			for _, n := range list {
				out = append(out, protocol.DocumentSymbol{
					Name: n.name, Detail: n.detail, Kind: n.kind,
					Range: rng(n.start, n.end), SelectionRange: rng(n.selStart, n.selEnd),
					Children: conv(n.children),
				})
			}
			return out
		}
		return conv(syms), nil
	}
	pkg := ""
	if pf.File.Name != nil {
		pkg = pf.File.Name.Name
	}
	out := []protocol.SymbolInformation{}
	var flat func(list []symNode, container string)
	flat = func(list []symNode, container string) {
		for _, n := range list {
			out = append(out, protocol.SymbolInformation{
				Name: n.name, Kind: n.kind, ContainerName: container,
				Location: protocol.Location{URI: p.TextDocument.URI, Range: rng(n.start, n.end)},
			})
			flat(n.children, n.name)
		}
	}
	flat(syms, pkg)
	return out, nil
}

// A wsSymbol is a symbol of the workspace for a query.
type wsSymbol struct {
	name      string // Type.Member for a member
	kind      protocol.SymbolKind
	container string // the import path of the package ("" for a folder without a module)
	path      string // the file
	off, end  int    // the name
	score     int
	lower     string // name in lower case
	ascii     bool   // name is ASCII only
}

// workspaceSymbol answers workspace/symbol.
func (s *Server) workspaceSymbol(ctx context.Context, e env, params json.RawMessage) (any, error) {
	var p protocol.WorkspaceSymbolParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	files, err := s.workspaceFileSymbols(ctx, e.snapshot)
	if err != nil {
		return nil, err
	}
	sq := newSymQuery(p.Query)
	var hits []wsSymbol
	for _, f := range files {
		for _, sym := range f.syms {
			if score, ok := sq.match(&sym); ok {
				sym.score = score
				sym.container = f.container
				hits = append(hits, sym)
			}
		}
	}
	slices.SortFunc(hits, func(a, b wsSymbol) int {
		return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(len(a.name), len(b.name)),
			strings.Compare(a.name, b.name), strings.Compare(a.path, b.path), cmp.Compare(a.off, b.off))
	})
	if len(hits) > maxWorkspaceSymbols {
		hits = hits[:maxWorkspaceSymbols]
	}
	docs := newDocs(e.snapshot)
	out := make([]protocol.SymbolInformation, 0, len(hits))
	for _, h := range hits {
		doc, err := docs.get(h.path)
		if err != nil {
			continue // the file is gone
		}
		out = append(out, protocol.SymbolInformation{
			Name: h.name, Kind: h.kind, ContainerName: h.container,
			Location: protocol.Location{
				URI:   protocol.URIFromPath(h.path),
				Range: protocol.Range{Start: doc.mapper.Position(h.off), End: doc.mapper.Position(h.end)},
			},
		})
	}
	return out, nil
}

// A fileSyms is the workspace symbols of one file of a package.
type fileSyms struct {
	container string // the import path of the package
	syms      []wsSymbol
}

// workspaceFileSymbols returns the symbols of every file of every package of
// the workspace in the snapshot, by file. A file's symbols are in the table
// for its content (the hash the snapshot knows without a parse), so a query
// parses nothing for the files that it has seen, and the parse cache's own
// limit on memory does not matter to it.
func (s *Server) workspaceFileSymbols(ctx context.Context, snap *cache.Snapshot) ([]fileSyms, error) {
	pkgs, err := s.workspacePackages(ctx, snap)
	if err != nil {
		return nil, err
	}
	type slot struct {
		pkg  int
		path string
		hash cache.Hash
		syms []wsSymbol
		miss bool
	}
	var slots []slot
	for i, pkg := range pkgs {
		files, err := snap.SourceFiles(pkg.dir)
		if err != nil {
			continue
		}
		for _, name := range files {
			h, err := snap.FileHash(name)
			if err != nil {
				continue
			}
			sl := slot{pkg: i, path: name, hash: h}
			sl.syms, sl.miss = s.symbols.flatOf(name, h)
			slots = append(slots, sl)
		}
	}
	// The files that have no symbols for this content are parsed, a few at a
	// time (the first query of a workspace, or after edits).
	var wg sync.WaitGroup
	var pc safe.Catcher
	sem := make(chan struct{}, 8)
	for i := range slots {
		if !slots[i].miss {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			pc.Run(func() {
				if faultInSymbols != nil {
					faultInSymbols(slots[i].path)
				}
				sl := &slots[i]
				pf, err := snap.ParsePath(ctx, sl.path, cache.ParseFull)
				if err != nil {
					sl.syms = nil
					return
				}
				sl.syms = s.symbols.makeFlat(pf)
			})
		}()
	}
	wg.Wait()
	pc.Rethrow()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]fileSyms, 0, len(slots))
	for _, sl := range slots {
		if len(sl.syms) > 0 {
			out = append(out, fileSyms{container: pkgs[sl.pkg].path, syms: sl.syms})
		}
	}
	return out, nil
}

// workspaceSymbols returns the symbols of every package of the workspace in
// the snapshot.
func (s *Server) workspaceSymbols(ctx context.Context, snap *cache.Snapshot) ([]wsSymbol, error) {
	files, err := s.workspaceFileSymbols(ctx, snap)
	if err != nil {
		return nil, err
	}
	var all []wsSymbol
	for _, f := range files {
		for _, sym := range f.syms {
			sym.container = f.container
			all = append(all, sym)
		}
	}
	return all, nil
}

// flatOf returns the workspace symbols of the file for the content with the
// hash, when the table has them (miss is false).
func (sf *symbolFeature) flatOf(path string, hash cache.Hash) (syms []wsSymbol, miss bool) {
	sf.filesMu.Lock()
	defer sf.filesMu.Unlock()
	e, ok := sf.files[path]
	if !ok || e.hash != hash || e.flat == nil {
		return nil, true
	}
	return e.flat, false
}

// makeFlat makes the workspace symbols of the parsed file, and keeps them in
// the table next to its symbols.
func (sf *symbolFeature) makeFlat(pf *cache.ParsedFile) []wsSymbol {
	nodes := sf.symbolsOf(pf)
	flat := make([]wsSymbol, 0, len(nodes))
	for _, n := range nodes {
		flat = append(flat, wsEntries(n, "", "", pf.Path)...)
	}
	if flat == nil {
		flat = []wsSymbol{} // "made" even when the file has no symbols
	}
	sf.filesMu.Lock()
	if e, ok := sf.files[pf.Path]; ok && e.hash == pf.Hash {
		e.flat = flat
		sf.files[pf.Path] = e
	}
	sf.filesMu.Unlock()
	return flat
}

// wsEntries returns the workspace symbols of n and its members. A method of
// the form (*T).M is named T.M, a member T.F, like gopls.
func wsEntries(n symNode, parent, container, path string) []wsSymbol {
	name := n.name
	if n.kind == protocol.Method && strings.HasPrefix(name, "(") {
		if i := strings.Index(name, ")."); i > 0 {
			recv := strings.TrimLeft(name[1:i], "*")
			if j := strings.Index(recv, "["); j > 0 {
				recv = recv[:j]
			}
			name = recv + name[i+1:]
		}
	}
	if parent != "" {
		name = parent + "." + name
	}
	lower := strings.ToLower(name)
	out := []wsSymbol{{name: name, kind: n.kind, container: container, path: path, off: n.selStart, end: n.selEnd, lower: lower, ascii: isASCII(name) && isASCII(lower)}}
	for _, c := range n.children {
		out = append(out, wsEntries(c, n.name, container, path)...)
	}
	return out
}

// A wsPackage is a package of the workspace.
type wsPackage struct {
	dir  string
	path string // its import path, or "" without a module
}

// workspacePackages returns the packages of the workspace: those of the
// modules of the workspace folders and of the open documents.
func (s *Server) workspacePackages(ctx context.Context, snap *cache.Snapshot) ([]wsPackage, error) {
	s.symbols.mu.Lock()
	roots := slices.Clone(s.symbols.roots)
	s.symbols.mu.Unlock()
	for _, o := range snap.Overlays() {
		if path := o.URI().Path(); path != "" && strings.HasSuffix(path, ".gp") {
			roots = append(roots, filepath.Dir(path))
		}
	}
	mods := make(map[string]*load.Module)
	var order []string
	for _, root := range roots {
		mod := load.FindModule(root)
		if mod == nil {
			mod = &load.Module{Dir: root} // a folder without a module: its .gp files
		}
		if _, ok := mods[mod.Dir]; !ok {
			mods[mod.Dir] = mod
			order = append(order, mod.Dir)
		}
	}
	slices.Sort(order)
	var out []wsPackage
	seen := make(map[string]bool)
	for _, dir := range order {
		pkgs, err := s.comp.index.moduleWait(ctx, mods[dir])
		if err != nil {
			return nil, err
		}
		for _, p := range pkgs {
			if !seen[p.dir] {
				seen[p.dir] = true
				out = append(out, wsPackage{dir: p.dir, path: p.path})
			}
		}
	}
	// The folders of open documents that are not on disk yet.
	for _, o := range snap.Overlays() {
		path := o.URI().Path()
		if path != "" && strings.HasSuffix(path, ".gp") && !seen[filepath.Dir(path)] {
			seen[filepath.Dir(path)] = true
			out = append(out, wsPackage{dir: filepath.Dir(path)})
		}
	}
	return out, nil
}

// warm builds the index of the workspace in the background: it parses the
// files and takes their symbols.
// faultInSymbols is set by a test: it is called in the goroutine of each file
// whose symbols are made, and panics there.
var faultInSymbols func(path string)

func (s *Server) warmSymbols() {
	if !s.symbols.warming.CompareAndSwap(false, true) {
		return // one run at a time: a second would redo the first's work
	}
	go func() {
		defer s.symbols.warming.Store(false)
		if p := safe.Call(func() {
			snap, release := s.session.Snapshot()
			defer release()
			ctx := s.comp.index.ctx
			if _, err := s.workspaceSymbols(ctx, snap); err != nil && ctx.Err() == nil {
				s.log.Printf("the symbol index: %v", err)
			}
		}); p != nil {
			s.log.Printf("the symbol index: panic: %v\n%s", p.Value, p.Stack)
		}
	}()
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// A symQuery is a workspace/symbol query prepared for many names:
// matchSymbol's rules, with the work that does not depend on the name done
// once, and bytes for the names that are ASCII (almost all of them).
type symQuery struct {
	raw   string
	lower string
	ascii bool
}

func newSymQuery(query string) *symQuery {
	return &symQuery{raw: query, lower: strings.ToLower(query), ascii: isASCII(query) && isASCII(strings.ToLower(query))}
}

// match is matchSymbol(q.raw, sym.name).
func (q *symQuery) match(sym *wsSymbol) (int, bool) {
	if q.raw == "" {
		return 0, true
	}
	if !q.ascii || !sym.ascii {
		return matchSymbol(q.raw, sym.name)
	}
	ln, n, lq := sym.lower, sym.name, q.lower
	// Every way to match needs the characters of the query in the name, in
	// order: most names are out at once.
	qi := 0
	for i := 0; i < len(ln) && qi < len(lq); i++ {
		if ln[i] == lq[qi] {
			qi++
		}
	}
	if qi < len(lq) {
		return 0, false
	}
	switch {
	case ln == lq:
		return 100000, true
	case strings.HasPrefix(ln, lq):
		return 50000 - len(ln), true
	}
	isUp := func(c byte) bool { return 'A' <= c && c <= 'Z' }
	isLow := func(c byte) bool { return 'a' <= c && c <= 'z' }
	if i := strings.Index(ln, lq); i >= 0 {
		score := 20000 - i
		if n[i-1] == '.' || n[i-1] == '_' || isUp(n[i]) && isLow(n[i-1]) {
			score += 5000
		}
		return score, true
	}
	score, last, gaps := 0, -1, 0
	qi = 0
	for i := 0; i < len(ln) && qi < len(lq); i++ {
		if ln[i] != lq[qi] {
			continue
		}
		if last >= 0 && i > last+1 {
			gaps += i - last - 1
		}
		if i == 0 || n[i-1] == '.' || n[i-1] == '_' || isUp(n[i]) && isLow(n[i-1]) {
			score += 100
		}
		if n[i] == q.raw[qi] {
			score += 10 // the case matches
		}
		last = i
		qi++
	}
	return score - gaps, true
}

// matchSymbol reports whether query matches name, and how well: a higher
// score is better. The query matches when its characters appear in the name in
// order (ignoring case); the score rewards the name equal to the query, a
// prefix, a contiguous substring, matches at the start of a word (after a dot,
// an underscore or a lowercase letter) and few gaps.
func matchSymbol(query, name string) (int, bool) {
	if query == "" {
		return 0, true
	}
	qr := []rune(query)
	q, n := []rune(strings.ToLower(query)), []rune(name)
	ln := make([]rune, len(n))
	for i, r := range n {
		ln[i] = unicode.ToLower(r)
	}
	score := 0
	switch {
	case string(ln) == string(q):
		return 100000, true
	case strings.HasPrefix(string(ln), string(q)):
		return 50000 - len(ln), true
	}
	if i := indexRunes(ln, q); i >= 0 {
		// A contiguous substring: better at the start of a word, and earlier.
		score = 20000 - i
		if n[i-1] == '.' || n[i-1] == '_' || unicode.IsUpper(n[i]) && unicode.IsLower(n[i-1]) {
			score += 5000
		}
		return score, true
	}
	// A greedy match, preferring word starts.
	qi, last, gaps := 0, -1, 0
	for i := 0; i < len(ln) && qi < len(q); i++ {
		if ln[i] != q[qi] {
			continue
		}
		if last >= 0 && i > last+1 {
			gaps += i - last - 1
		}
		if i == 0 || n[i-1] == '.' || n[i-1] == '_' || unicode.IsUpper(n[i]) && unicode.IsLower(n[i-1]) {
			score += 100
		}
		if qi < len(qr) && n[i] == qr[qi] {
			score += 10 // the case matches
		}
		last = i
		qi++
	}
	if qi < len(q) {
		return 0, false
	}
	return score - gaps, true
}

// indexRunes returns the index of the first occurrence of sub in s, or -1.
func indexRunes(s, sub []rune) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if slices.Equal(s[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}
