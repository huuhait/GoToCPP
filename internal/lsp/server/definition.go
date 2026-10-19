package server

import (
	"context"
	"encoding/json"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// textDocument/definition and textDocument/typeDefinition (step 11.5): the
// declaring name of the name under the cursor, in this package or in a
// package of the workspace that it imports (whose objects come from export
// data, with the file and line of their declaration: declOf finds the
// name there). An import path goes to the package's files, a header
// import to the header file (cache.Pipeline.HeaderFile). A C/C++ name has
// no definition yet (Phase 12), and neither has a name of the standard
// library, which is embedded in gocpp and has no file on disk, nor a
// predeclared name.

func (s *Server) definition(ctx context.Context, e env, params json.RawMessage) (any, error) {
	return s.locations(ctx, e, params, (*query).definition)
}

func (s *Server) typeDefinition(ctx context.Context, e env, params json.RawMessage) (any, error) {
	return s.locations(ctx, e, params, (*query).typeDefinition)
}

// locations runs a request whose answer is a list of locations: find's,
// or an empty list.
func (s *Server) locations(ctx context.Context, e env, params json.RawMessage, find func(*query, context.Context) ([]protocol.Location, error)) (any, error) {
	p, err := decodePosition(params)
	if err != nil {
		return nil, err
	}
	q, err := newQuery(ctx, e.snapshot, p.TextDocument.URI, p.Position)
	if err != nil {
		return nil, err
	}
	locs := []protocol.Location{}
	if q != nil {
		found, err := find(q, ctx)
		if err != nil {
			return nil, err
		}
		locs = append(locs, found...)
	}
	return locs, nil
}

// definition returns where the name at the place of q is declared.
func (q *query) definition(ctx context.Context) ([]protocol.Location, error) {
	if info := q.file.Info; info != nil {
		for _, spec := range info.Packages {
			if q.contains(spec.Path) {
				return q.packageLocations(importPath(spec))
			}
		}
		for _, inc := range info.Includes {
			if q.specPart(inc.Spec) != nil {
				path, err := q.headerFile(ctx, inc)
				if path == "" || err != nil {
					return nil, err
				}
				return []protocol.Location{{URI: protocol.URIFromPath(path)}}, nil
			}
		}
	}
	if u, _, _ := q.cppUseAt(); u != nil {
		return nil, nil // Phase 12
	}
	id, _ := identAt(q.file, q.pos)
	if id == nil {
		return nil, nil
	}
	obj := q.objectAt(id)
	if obj == nil {
		return nil, nil
	}
	return q.objectLocation(ctx, obj)
}

// typeDefinition returns where the type of the name at the place of q is
// declared: the named types in it (*T, []T, map[K]V, chan T, and the
// results of a function).
func (q *query) typeDefinition(ctx context.Context) ([]protocol.Location, error) {
	id, _ := identAt(q.file, q.pos)
	if id == nil {
		return nil, nil
	}
	obj := q.objectAt(id)
	if obj == nil {
		return nil, nil
	}
	var t types.Type
	switch obj := obj.(type) {
	case *types.TypeName:
		t = obj.Type()
	case *types.Var, *types.Const, *types.Func:
		t = obj.Type()
	default:
		return nil, nil
	}
	var locs []protocol.Location
	for _, tn := range namedTypes(t) {
		found, err := q.objectLocation(ctx, tn)
		if err != nil {
			return nil, err
		}
		locs = append(locs, found...)
	}
	return locs, nil
}

// namedTypes returns the type names of the named types in t: t itself, or
// what a pointer, slice, array, map or channel holds, or the results of a
// function; each once.
func namedTypes(t types.Type) []*types.TypeName {
	var names []*types.TypeName
	seen := make(map[*types.TypeName]bool)
	var walk func(t types.Type)
	walk = func(t types.Type) {
		switch t := t.(type) {
		case *types.Named:
			if tn := t.Obj(); !seen[tn] {
				seen[tn] = true
				names = append(names, tn)
			}
		case *types.Alias:
			if tn := t.Obj(); !seen[tn] {
				seen[tn] = true
				names = append(names, tn)
			}
		case *types.Pointer:
			walk(t.Elem())
		case *types.Slice:
			walk(t.Elem())
		case *types.Array:
			walk(t.Elem())
		case *types.Chan:
			walk(t.Elem())
		case *types.Map:
			walk(t.Key())
			walk(t.Elem())
		case *types.Signature:
			for i := range t.Results().Len() {
				walk(t.Results().At(i).Type())
			}
		}
	}
	walk(t)
	return names
}

// objectLocation returns where obj is declared: its name, in a .gp file.
func (q *query) objectLocation(ctx context.Context, obj types.Object) ([]protocol.Location, error) {
	if isBuiltin(obj) {
		return nil, nil
	}
	d, err := q.declOf(ctx, obj)
	if d == nil || err != nil {
		return nil, err
	}
	if q.snap.IsStdFile(d.pf.Path) {
		return nil, nil // embedded: no file to open
	}
	m := protocol.NewMapper(string(d.pf.Src))
	end := d.off + len(obj.Name())
	switch {
	case d.end > 0:
		end = d.end
	case d.ident != nil:
		end = d.pf.Tok.Offset(d.ident.End())
	}
	return []protocol.Location{{
		URI:   protocol.URIFromPath(d.pf.Path),
		Range: protocol.Range{Start: m.Position(d.off), End: m.Position(end)},
	}}, nil
}

// packageLocations returns the package clauses of the files of the
// imported package path (not its test files), or nothing for a standard
// package.
func (q *query) packageLocations(path string) ([]protocol.Location, error) {
	dep := q.lp.Imports[path]
	if dep == nil || dep.Std {
		return nil, nil
	}
	files, err := q.snap.PackageFiles(dep)
	if err != nil {
		return nil, err
	}
	var locs []protocol.Location
	// TODO(Phase 11): PackageFiles lists also the files that the build
	// constraints leave out (x_windows.gp on Linux); gopls lists only the
	// compiled files. The same goes for packageHover's package doc.
	for _, file := range files {
		if strings.HasSuffix(file, "_test.gp") || filepath.Dir(file) != filepath.Clean(dep.Dir) {
			continue
		}
		src, err := q.snap.ReadFile(file)
		if err != nil {
			continue
		}
		loc := protocol.Location{URI: protocol.URIFromPath(file)}
		if r, ok := packageClause(file, src); ok {
			loc.Range = r
		}
		locs = append(locs, loc)
	}
	return locs, nil
}

// packageClause returns the range of the package name of the file, or
// false.
func packageClause(path string, src []byte) (protocol.Range, bool) {
	f, tok := parsePackageClause(path, src)
	if f == nil || f.Name == nil || !f.Name.Pos().IsValid() {
		return protocol.Range{}, false
	}
	m := protocol.NewMapper(string(src))
	return protocol.Range{Start: m.Position(tok.Offset(f.Name.Pos())), End: m.Position(tok.Offset(f.Name.End()))}, true
}
