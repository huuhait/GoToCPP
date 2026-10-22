package cache

import (
	"context"
	"go/ast"
	"slices"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// Declarations of the foreign names a program writes (step 12.13c2): raylib.InitWindow,
// std.string, sqlite3.Open. Once the header index of a slot is ready, the same background
// worker asks the compiler (probe.Runner.Locate, bounded and cached by the runner) where the
// headers declare the names that the files of the slot use, and keeps the answers in the slot.
// A request reads them like the index's names: no compiler, no waiting, nothing for a name that
// has no completed answer. The slot belongs to one snapshot and is dropped, with its answers,
// when the headers change (HeadersChanged) or the snapshot is replaced.

// maxForeignNames bounds the names of one slot; a program with more names gets no answer for
// the later ones.
const maxForeignNames = 96

// A foreignName is a name as the Go source writes it: the C++ namespace in front of it
// (without a leading ::, "" for a header package) and the name.
type foreignName struct {
	scope string
	name  string
}

func foreignKey(scope, name string) string {
	return strings.TrimPrefix(scope, "::") + "\x00" + name
}

// foreignNames lists the names that the file writes after a header package or a namespace,
// from the rewrite's provenance and the checked syntax tree (scoping decided by Go, as in the
// hover). A namespace prefix of a longer path is listed too: the index says it is a namespace
// and it is not looked up.
func foreignNames(file *load.File) []foreignName {
	rw := file.Rewrite
	if rw == nil || len(rw.Uses) == 0 || file.AST == nil {
		return nil
	}
	bySel := make(map[*ast.SelectorExpr]*load.CppUse, len(rw.Uses))
	for _, use := range rw.Uses {
		if use.Bare || use.Sel == nil || use.Kind != load.HeaderUse && use.Kind != load.NamespaceUse {
			continue
		}
		bySel[use.Sel] = use
	}
	var names []foreignName
	ast.Inspect(file.AST, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		path := []string{sel.Sel.Name}
		for e := sel; ; {
			if use := bySel[e]; use != nil {
				scope := ""
				if use.Kind == load.NamespaceUse {
					scope = strings.TrimPrefix(use.Namespace, "::")
				}
				if len(path) > 1 {
					scope = strings.Join(append(nonEmpty(scope), path[:len(path)-1]...), "::")
				}
				name := foreignName{scope: scope, name: path[len(path)-1]}
				if !slices.Contains(names, name) && len(names) < maxForeignNames {
					names = append(names, name)
				}
				break
			}
			x, ok := e.X.(*ast.SelectorExpr)
			if !ok {
				break
			}
			path = append([]string{x.Sel.Name}, path...)
			e = x
		}
		return true
	})
	return names
}

func nonEmpty(scope string) []string {
	if scope == "" {
		return nil
	}
	return []string{scope}
}

// locateNames asks for the declarations of the names that the index shows as declared or as
// macros. It runs in the worker of the slot, after the index is published, and publishes its
// answers only while the slot is still the current one.
func (ix *headerIndexer) locateNames(ctx context.Context, entry *headerEntry, key string, cfg probe.Config,
	includes []string, index *probe.HeaderIndex, names []foreignName,
) {
	if len(names) == 0 || ix.locate == nil || index == nil || ctx.Err() != nil {
		return
	}
	type wanted struct {
		user  foreignName
		query probe.LocateName
	}
	var wants []wanted
	queries := make(map[string]probe.LocateName)
	var request probe.LocateRequest
	for _, name := range names {
		canonical, macro, ok := index.Resolve(name.scope, name.name)
		if !ok {
			continue
		}
		query := probe.LocateName{Scope: canonical, Name: name.name, Macro: macro}
		wants = append(wants, wanted{user: name, query: query})
		if !slices.Contains(request.Names, query) {
			request.Names = append(request.Names, query)
		}
	}
	if len(request.Names) == 0 {
		return
	}
	var locations *probe.Locations
	var err error
	// One retry covers headers just saved inside the runner's conservative race window.
	for attempt := 0; attempt < 2; attempt++ {
		locations, err = ix.locate(ctx, cfg, includes, request)
		if err == nil && locations != nil || ctx.Err() != nil {
			break
		}
		if attempt == 0 {
			timer := time.NewTimer(revalidateAfter)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
	}
	if err != nil || locations == nil || ctx.Err() != nil {
		return
	}
	decls := make(map[string][]probe.Declaration, len(wants))
	for _, want := range wants {
		if found := locations.Named(want.query.Scope, want.query.Name); len(found) > 0 {
			decls[foreignKey(want.user.scope, want.user.name)] = found
			queries[foreignKey(want.user.scope, want.user.name)] = want.query
		}
	}
	cost := locations.Bytes()
	if ix.beforePublish != nil {
		ix.beforePublish()
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ctx.Err() == nil && ix.entries[key] == entry && len(decls) > 0 &&
		ix.bytes+cost <= maxHeaderIndexBytes {
		entry.decls = decls
		entry.queries = queries
		entry.bytes += cost
		ix.bytes += cost
	}
}

// ForeignDeclarations returns the completed declarations of a name that the file writes after
// a header package (scope "") or a namespace (scope "std", "std::chrono"), or false. Like
// HeaderSymbols it reads only what a background worker has published for this snapshot: it
// never starts, waits for or revalidates a compiler run, and it gives nothing for a name that
// the compiler did not prove, for a stale slot or while the headers are being read again.
func (s *Snapshot) ForeignDeclarations(pkg *Package, file *load.File, scope, name string,
) ([]probe.Declaration, bool) {
	st, ok := s.state.(*state)
	if !ok || st.p.headers == nil || pkg == nil || pkg.Check == nil || pkg.IsVariant() || file == nil {
		return nil, false
	}
	ix := st.p.headers
	key := headerEntryKey(s, pkg, headerIncludes(pkg, file))
	ix.mu.Lock()
	defer ix.mu.Unlock()
	entry := ix.entries[key]
	if entry == nil || entry.ctx.Err() != nil {
		return nil, false
	}
	found := entry.decls[foreignKey(scope, name)]
	if len(found) == 0 {
		return nil, false
	}
	return slices.Clone(found), true
}
