package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/expand"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/fspath"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// "Show expansion" (step 11.13, ADR-099): a code action on a declaration that has a `!`, a macro or a
// #Derive in it, whose command prints what `gocpp expand` prints for the file, from the files of the
// snapshot (the unsaved text too), and shows it.
//
// The action is cheap: it is offered when the declaration around the cursor has a mark of Phase 10 in it,
// which the indexes of the checked package say (phase10Index). The command is not: it loads the package
// and what it imports from source and checks it again with the bodies of #Derive written, the check of
// gocpp build (cache.Snapshot.CheckExpand), then prints the file with package expand, the printer of
// gocpp expand. It runs when the user asks, never as they type.
//
// The text is shown as a document of its own: written to a file `<name>.gpexpand` in a folder of the
// server's (removed when the server ends) and opened with window/showDocument, with the declaration that
// the cursor was in selected. A client that cannot show documents gets the text in the command's result.
// The file has the extension of the language, so a client colors it, but it belongs to no package: the
// server publishes no diagnostics for it.

// ExpandCommand is the command of workspace/executeCommand that shows the expansion of the declaration
// at a place: its argument is an ExpandArgs.
const ExpandCommand = "gocpp.showExpansion"

// ExpandExt is the extension of the files that show an expansion.
const ExpandExt = ".gpexpand"

// ExpandArgs is the argument of ExpandCommand: the document and the place in it.
type ExpandArgs struct {
	URI   protocol.DocumentURI `json:"uri"`
	Range protocol.Range       `json:"range"`
}

// ExpandResult is the result of ExpandCommand.
type ExpandResult struct {
	// URI is the file that holds the expansion (empty if none could be written).
	URI protocol.DocumentURI `json:"uri,omitempty"`
	// Text is the expansion of the file.
	Text string `json:"text"`
	// Selection is the declaration that the place was in, in Text (nil when it was in none that is expanded).
	Selection *protocol.Range `json:"selection,omitempty"`
}

// expansions holds the folder of the files that show an expansion.
type expansions struct {
	mu      sync.Mutex
	dir     string
	closed  bool          // the server has ended: nothing is written again
	stop    chan struct{} // closed with the folder: ends keepFresh
	showDoc bool          // the client can show documents (initialize)
}

// canShow reports whether the client can show documents.
func (x *expansions) canShow() bool {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.showDoc
}

// close removes the folder, and makes write refuse (a request that is still running when the connection ends
// must not make the folder again).
func (x *expansions) close() {
	x.mu.Lock()
	dir := x.dir
	x.dir, x.closed = "", true
	if x.stop != nil {
		close(x.stop)
		x.stop = nil
	}
	x.mu.Unlock()
	if dir != "" {
		os.RemoveAll(dir)
	}
}

// expandDirPrefix and expandMarker identify the folders of this feature in the temporary folder: a server that is
// killed leaves its folder behind, and the next one removes the old ones.
const (
	expandDirPrefix = "gocpp-lsp-expand-"
	expandMarker    = ".gocpp-lsp-expand"
	expandStaleAge  = 24 * time.Hour
)

// touchEvery is how often a running server touches the marker of its folder (a session that lasts longer than
// expandStaleAge must not have its folder taken for the one of a killed server); a variable for the tests.
var touchEvery = time.Hour

// ownerCheck says whether a folder belongs to the user of the process (a variable for the tests).
var ownerCheck = ownedByUs

// removeStale removes the folders of servers that were killed: in the temporary folder, named like ours, with our
// marker file in them, owned by us, and older than a day. Nothing else is touched, and nothing of another user is read:
// the owner is checked first, and the marker is read as a regular file (load.ReadRegular: a named pipe is no wait).
func removeStale(owned func(os.FileInfo) bool) {
	tmp := os.TempDir()
	entries, err := os.ReadDir(tmp)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), expandDirPrefix) {
			continue
		}
		dir := filepath.Join(tmp, e.Name())
		if fi, err := os.Stat(dir); err != nil || !owned(fi) {
			continue
		}
		marker, err := os.Stat(filepath.Join(dir, expandMarker))
		if err != nil || !marker.Mode().IsRegular() || time.Since(marker.ModTime()) < expandStaleAge {
			continue
		}
		data, err := load.ReadRegular(filepath.Join(dir, expandMarker))
		if load.IsNotRegular(err) {
			continue // swapped for something that is not a file since the Stat: left alone
		}
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 && pidAlive(pid) {
			continue // the server that made it is running
		}
		os.RemoveAll(dir)
	}
}

// keepFresh touches the marker of the folder dir every touchEvery until the folder is gone.
func (x *expansions) keepFresh(dir string, stop chan struct{}, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			now := time.Now()
			os.Chtimes(filepath.Join(dir, expandMarker), now, now)
		}
	}
}

// write writes the expansion text of the file with the key key (its canonical name) to the folder (made on the
// first use) as <hash of key>/<name>.gpexpand: the files of two packages that have the same name get folders
// of their own, and the name stays readable. It returns the path.
func (x *expansions) write(key, name, text string) (string, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.closed {
		return "", errors.New("the server has ended")
	}
	if x.dir == "" {
		dir, err := os.MkdirTemp("", expandDirPrefix)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(dir, expandMarker), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
		x.dir = dir
		go removeStale(ownerCheck)
		x.stop = make(chan struct{})
		go x.keepFresh(dir, x.stop, touchEvery)
	}
	now := time.Now()
	os.Chtimes(filepath.Join(x.dir, expandMarker), now, now)
	h := fnv.New64a()
	h.Write([]byte(key))
	sub := filepath.Join(x.dir, fmt.Sprintf("%016x", h.Sum64()))
	if err := os.MkdirAll(sub, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(sub, name+ExpandExt)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// phase10Index lists the places of the forms of Phase 10 in a package, sorted: what "Show expansion"
// looks at to decide whether a declaration has an expansion.
type phase10Index struct {
	marks   []token.Pos
	derives []deriveExpansion
}

// deriveExpansion associates a directive (which precedes its declaration) with the exact type the loader chose.
// Do not infer this from the nearest declaration: comments and grouped type declarations can intervene.
type deriveExpansion struct {
	start, end token.Pos
	name       string
}

type phase10Key struct{}

// phase10Of returns the index of the forms of Phase 10 of the package pkg.
func phase10Of(pkg *cache.Package) *phase10Index {
	return pkg.Memo(phase10Key{}, func() any {
		var marks []token.Pos
		if b := bangOf(pkg); b != nil {
			for _, c := range b.calls {
				marks = append(marks, c.Pos)
			}
			for _, f := range b.funcs {
				marks = append(marks, f.Decl.Name.NamePos)
			}
		}
		if len(pkg.Program.Roots) == 0 {
			return &phase10Index{marks: marks}
		}
		lp, fset := pkg.Program.Roots[0], pkg.Fset()
		for id := range lp.MacroUses {
			marks = append(marks, id.Pos())
		}
		var derives []deriveExpansion
		for _, d := range lp.Derives {
			marks = append(marks, d.Pos)
			if d.Decl != nil && d.Spec != nil {
				derives = append(derives, deriveExpansion{d.Pos, d.Decl.End(), d.Spec.Name.Name})
			}
		}
		for _, f := range lp.Files {
			if f.AST == nil || len(lp.Macros) == 0 {
				continue
			}
			for _, d := range f.AST.Decls {
				// A macro declaration: macro Name(...), which the loader turned into a func.
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Name == nil || lp.Macros[fd.Name.Name] == nil || !fd.Type.Func.IsValid() {
					continue
				}
				if tf := fset.File(fd.Type.Func); tf != nil && len(f.Src) == tf.Size() {
					if off := tf.Offset(fd.Type.Func); strings.HasPrefix(string(f.Src[off:min(off+5, len(f.Src))]), "macro") {
						marks = append(marks, fd.Type.Func)
					}
				}
			}
		}
		sort.Slice(marks, func(i, j int) bool { return marks[i] < marks[j] })
		return &phase10Index{marks: marks, derives: derives}
	}).(*phase10Index)
}

// has reports whether a mark is in [lo, hi].
func (x *phase10Index) has(lo, hi token.Pos) bool {
	i := sort.Search(len(x.marks), func(i int) bool { return x.marks[i] >= lo })
	return i < len(x.marks) && x.marks[i] <= hi
}

// expandable returns the name of the declaration of the file of q that holds the place of q and has a form
// of Phase 10 in it, and ok.
func (q *query) expandable(x *phase10Index) (name string, ok bool) {
	for _, d := range x.derives {
		if d.start <= q.pos && q.pos <= d.end {
			return d.name, true
		}
	}
	var best ast.Decl
	for _, d := range q.file.AST.Decls {
		if d.Pos() <= q.pos && q.pos <= d.End() && (best == nil || d.End()-d.Pos() < best.End()-best.Pos()) {
			best = d
		}
	}
	if best == nil || !x.has(best.Pos(), best.End()) {
		return "", false
	}
	switch d := best.(type) {
	case *ast.FuncDecl:
		return d.Name.Name, true
	case *ast.GenDecl:
		for _, s := range d.Specs {
			if ts, ok := s.(*ast.TypeSpec); ok {
				return ts.Name.Name, true
			}
		}
	}
	return "", true
}

// expansionActions returns the action "Show expansion" for the range of the request, or none.
func (s *Server) expansionActions(ctx context.Context, e env, p protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	pkg, err := e.snapshot.PackageOf(ctx, p.TextDocument.URI)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, nil // a folder with no package, or one that cannot be loaded: no action
	}
	x := phase10Of(pkg)
	if len(x.marks) == 0 {
		return nil, nil // the common case: a package with none of the forms, and no cost
	}
	q, err := newQuery(ctx, e.snapshot, p.TextDocument.URI, p.Range.Start)
	if q == nil || err != nil {
		return nil, err
	}
	name, ok := q.expandable(x)
	if !ok {
		return nil, nil
	}
	arg, err := json.Marshal(ExpandArgs{URI: p.TextDocument.URI, Range: p.Range})
	if err != nil {
		return nil, err
	}
	title := "Show expansion"
	if name != "" {
		title += " of " + name
	}
	return []protocol.CodeAction{{
		Title:   title,
		Kind:    protocol.RefactorInline,
		Command: &protocol.Command{Title: title, Command: ExpandCommand, Arguments: []json.RawMessage{arg}},
	}}, nil
}

// showExpansion is the command ExpandCommand.
func (s *Server) showExpansion(ctx context.Context, e env, args []json.RawMessage) (any, error) {
	if len(args) != 1 {
		return nil, protocol.Errorf(protocol.InvalidParams, "%s takes one argument: {uri, range}", ExpandCommand)
	}
	var a ExpandArgs
	if err := json.Unmarshal(args[0], &a); err != nil {
		return nil, protocol.Errorf(protocol.InvalidParams, "bad argument for %s: %v", ExpandCommand, err)
	}
	path := a.URI.Path()
	if path == "" || !strings.HasSuffix(path, ".gp") {
		return nil, protocol.Errorf(protocol.InvalidParams, "%s is not a .gp file", a.URI)
	}
	doc, err := documentOfSnapshot(e.snapshot, a.URI) // (the check below reads the files of the same snapshot)
	if err != nil {
		return nil, err
	}
	ec, err := e.snapshot.CheckExpand(ctx, filepath.Dir(path))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, protocol.Errorf(protocol.RequestFailed, "cannot expand: %v", err)
	}
	cp := ec.Result.Package(ec.Root)
	if cp == nil {
		return nil, protocol.Errorf(protocol.RequestFailed, "cannot expand: the package could not be checked")
	}
	for _, d := range cp.Errors.Diagnostics() {
		if d.Severity == diag.Error {
			return nil, protocol.Errorf(protocol.RequestFailed, "cannot expand: the package has errors, the first: %s: %s", d.Pos, d.Msg)
		}
	}
	for _, d := range ec.Program.Errors.Diagnostics() {
		if d.Severity == diag.Error {
			return nil, protocol.Errorf(protocol.RequestFailed, "cannot expand: the package has errors, the first: %s: %s", d.Pos, d.Msg)
		}
	}
	for _, f := range cp.Load.Files {
		if !samePath(f.Path, path) || f.AST == nil {
			continue
		}
		out, spans, err := expand.File(ec.Program.Fset, cp, f, ec.Known, false, nil)
		if err != nil {
			return nil, protocol.Errorf(protocol.RequestFailed, "cannot expand %s: %v", filepath.Base(path), err)
		}
		res := &ExpandResult{Text: string(out)}
		m := protocol.NewMapper(res.Text)
		off := doc.mapper.Offset(a.Range.Start)
		for _, sp := range spans {
			if sp.SrcStart <= off && off <= sp.SrcEnd {
				res.Selection = &protocol.Range{Start: m.Position(sp.OutStart), End: m.Position(sp.OutEnd)}
				break
			}
		}
		if file, err := s.expansions.write(fspath.Key(path), strings.TrimSuffix(filepath.Base(path), ".gp"), res.Text); err != nil {
			s.log.Printf("show expansion: %v", err)
		} else {
			res.URI = protocol.URIFromPath(file)
		}
		if res.URI != "" && s.expansions.canShow() && s.conn != nil {
			err := s.conn.Request("window/showDocument", &protocol.ShowDocumentParams{
				URI: protocol.URI(res.URI), TakeFocus: true, Selection: res.Selection,
			})
			if err != nil {
				s.log.Printf("show expansion: %v", err)
			}
		}
		return res, nil
	}
	return nil, protocol.Errorf(protocol.RequestFailed, "cannot expand: %s is not a file of its package", filepath.Base(path))
}

// samePath reports whether two names of files are the same file (see internal/lsp/fspath).
func samePath(a, b string) bool { return fspath.Equal(a, b) }
