package server

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// What the user is told about a search that reaches further than the name they
// asked for (step 11.8, review round 1).
//
// The family of a method is wider than gopls's. gopls renames the methods that
// a use that needs the interface ties together (a value of one type passed
// where the interface is wanted). gocpp renames every method that has the
// name and the shape of a method of an interface of the module (the type
// would satisfy the interface), whether or not a use needs it: renaming
// Lener.Len also renames Counterer.Len of a type that nothing converts to
// Lener. So a rename of a method always says which other declarations it
// changed, and which of them no use of the module needs to be one, so the user
// can undo it. (A narrower family is a TODO: plans/.../TODO.md.)

// otherDeclarations tells the user which declarations beyond the one they
// asked about a rename of a method changed, with an info message.
func (rn *renamer) otherDeclarations() {
	sym := rn.sym
	if len(sym.objs) != 1 || rn.e.notify == nil {
		return
	}
	asked, ok := sym.objs[0].(*types.Func)
	if !ok || recvBase(asked) == nil {
		return
	}
	type decl struct {
		name string
		pos  token.Position
		typ  types.Type // the receiver's type (never an interface)
	}
	var others []decl
	for _, h := range rn.res.hits {
		fn, ok := h.obj.(*types.Func)
		if !ok || !h.def || recvBase(fn) == nil || origin(fn) == origin(asked) {
			continue
		}
		others = append(others, decl{objectName(fn), h.position(), recvBase(fn)})
	}
	if len(others) == 0 {
		return
	}
	required := rn.requiredTypes()
	where := func(d decl) string {
		return fmt.Sprintf("%s (%s:%d)", d.name, filepath.Base(d.pos.Filename), d.pos.Line)
	}
	var all, unused []string
	for i, d := range others {
		if i < 5 {
			all = append(all, where(d))
		}
		if !types.IsInterface(d.typ) && !required[typeKey(d.typ)] {
			unused = append(unused, where(d))
		}
	}
	msg := fmt.Sprintf("gocpp rename: %s was also renamed in %d other method declaration(s) that have its name and shape: %s", sym.name, len(others), strings.Join(all, ", "))
	if len(others) > len(all) {
		msg += ", ..."
	}
	msg += ". gocpp renames every method of the module that has the name and shape of a method of the family's interfaces, which is wider than gopls (only methods that a use ties to the interface)."
	if len(unused) > 0 {
		msg += " Not needed by any use that gocpp found (no value of the type is converted to or asserted from the interface): " + strings.Join(unused[:min(5, len(unused))], ", ")
		if len(unused) > 5 {
			msg += ", ..."
		}
		msg += "."
	}
	msg += " Undo the rename if some of them are unrelated."
	if err := rn.e.notify("window/showMessage", &protocol.ShowMessageParams{Type: protocol.Info, Message: msg}); err != nil {
		rn.e.log.Printf("window/showMessage: %v", err)
	}
}

// typeKey names a named type (or the type a pointer points to) independently
// of the check it is from: its package path and name; "" for any other type.
func typeKey(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := types.Unalias(t).(*types.Named); ok && n.Obj().Pkg() != nil {
		return strings.TrimSuffix(n.Obj().Pkg().Path(), "_test") + "." + n.Obj().Name()
	}
	return ""
}

// requiredTypes returns the types (typeKey) of the module that a use needs to
// be the interface of the family: a value of the type is passed, assigned,
// returned, sent, put in a composite literal or compared where an interface
// with a method of the family's name is wanted, converted to one, or asserted
// from one.
func (rn *renamer) requiredTypes() map[string]bool {
	name := rn.sym.name
	required := make(map[string]bool)
	wants := func(t types.Type) bool {
		if t == nil || !types.IsInterface(t) {
			return false
		}
		it, ok := t.Underlying().(*types.Interface)
		if !ok {
			return false
		}
		for m := range it.Methods() {
			if m.Name() == name {
				return true
			}
		}
		return false
	}
	mark := func(info *types.Info, e ast.Expr, target types.Type) {
		if e == nil || !wants(target) {
			return
		}
		if t := info.TypeOf(e); t != nil && !types.IsInterface(t) {
			if k := typeKey(t); k != "" {
				required[k] = true
			}
		}
	}
	for _, v := range rn.res.views {
		if v.info == nil {
			continue
		}
		info := v.info
		for _, f := range v.files {
			var sigs []*types.Signature
			var walk func(n ast.Node) bool
			walk = func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.FuncDecl:
					if sig, ok := info.TypeOf(n.Name).(*types.Signature); ok {
						sigs = append(sigs, sig)
						ast.Inspect(n.Body, walk)
						sigs = sigs[:len(sigs)-1]
					}
					return false
				case *ast.FuncLit:
					if sig, ok := info.TypeOf(n).(*types.Signature); ok {
						sigs = append(sigs, sig)
						ast.Inspect(n.Body, walk)
						sigs = sigs[:len(sigs)-1]
					}
					return false
				case *ast.AssignStmt:
					if len(n.Lhs) == len(n.Rhs) && n.Tok == token.ASSIGN {
						for i := range n.Rhs {
							mark(info, n.Rhs[i], info.TypeOf(n.Lhs[i]))
						}
					}
				case *ast.ValueSpec:
					if n.Type != nil && len(n.Values) > 0 {
						for _, val := range n.Values {
							mark(info, val, info.TypeOf(n.Type))
						}
					}
				case *ast.ReturnStmt:
					if len(sigs) > 0 {
						res := sigs[len(sigs)-1].Results()
						if res.Len() == len(n.Results) {
							for i, r := range n.Results {
								mark(info, r, res.At(i).Type())
							}
						}
					}
				case *ast.SendStmt:
					if ch, ok := info.TypeOf(n.Chan).Underlying().(*types.Chan); ok {
						mark(info, n.Value, ch.Elem())
					}
				case *ast.CallExpr:
					if tv, ok := info.Types[n.Fun]; ok && tv.IsType() && len(n.Args) == 1 {
						mark(info, n.Args[0], tv.Type) // a conversion
					} else if ok && n.Fun != nil {
						if sig, ok := types.Unalias(info.TypeOf(n.Fun)).Underlying().(*types.Signature); ok {
							params := sig.Params()
							for i, a := range n.Args {
								switch {
								case i < params.Len()-1 || i == params.Len()-1 && !sig.Variadic():
									mark(info, a, params.At(i).Type())
								case params.Len() > 0 && sig.Variadic():
									if sl, ok := params.At(params.Len() - 1).Type().(*types.Slice); ok && !n.Ellipsis.IsValid() {
										mark(info, a, sl.Elem())
									}
								}
							}
						}
					}
				case *ast.CompositeLit:
					switch t := types.Unalias(info.TypeOf(n)).Underlying().(type) {
					case *types.Slice:
						for _, el := range n.Elts {
							mark(info, litValue(el), t.Elem())
						}
					case *types.Array:
						for _, el := range n.Elts {
							mark(info, litValue(el), t.Elem())
						}
					case *types.Map:
						for _, el := range n.Elts {
							if kv, ok := el.(*ast.KeyValueExpr); ok {
								mark(info, kv.Value, t.Elem())
								mark(info, kv.Key, t.Key())
							}
						}
					case *types.Struct:
						for i, el := range n.Elts {
							if kv, ok := el.(*ast.KeyValueExpr); ok {
								if id, ok := kv.Key.(*ast.Ident); ok {
									if fv, ok := info.Uses[id].(*types.Var); ok {
										mark(info, kv.Value, fv.Type())
									}
								}
							} else if i < t.NumFields() {
								mark(info, el, t.Field(i).Type())
							}
						}
					}
				case *ast.BinaryExpr:
					if n.Op == token.EQL || n.Op == token.NEQ {
						mark(info, n.X, info.TypeOf(n.Y))
						mark(info, n.Y, info.TypeOf(n.X))
					}
				case *ast.TypeAssertExpr:
					if n.Type != nil && wants(info.TypeOf(n.X)) {
						if t := info.TypeOf(n.Type); t != nil && !types.IsInterface(t) {
							required[typeKey(t)] = true
						}
					}
				case *ast.TypeSwitchStmt:
					var x ast.Expr
					switch a := n.Assign.(type) {
					case *ast.ExprStmt:
						x = a.X
					case *ast.AssignStmt:
						if len(a.Rhs) == 1 {
							x = a.Rhs[0]
						}
					}
					if ta, ok := x.(*ast.TypeAssertExpr); ok && wants(info.TypeOf(ta.X)) {
						for _, c := range n.Body.List {
							for _, te := range c.(*ast.CaseClause).List {
								if t := info.TypeOf(te); t != nil && !types.IsInterface(t) {
									required[typeKey(t)] = true
								}
							}
						}
					}
				}
				return true
			}
			ast.Inspect(f, walk)
		}
	}
	delete(required, "")
	return required
}

// litValue returns the value of an element of a composite literal.
func litValue(e ast.Expr) ast.Expr {
	if kv, ok := e.(*ast.KeyValueExpr); ok {
		return kv.Value
	}
	return e
}

// otherModules returns the folders of the modules (gocpp.mod files) under the
// workspace folders, other than the module of the document, sorted. The walk
// is cached until a gocpp.mod file or a folder is created or deleted
// (importIndex.changed).
func (s *Server) otherModules(q *query) []string {
	cur := ""
	if q.lp.Module != nil {
		cur = q.lp.Module.Dir
	}
	s.symbols.mu.Lock()
	roots := slices.Clone(s.symbols.roots)
	s.symbols.mu.Unlock()
	gen := s.comp.index.generation()
	s.modsMu.Lock()
	defer s.modsMu.Unlock()
	key := strings.Join(roots, "\x00")
	if s.mods.gen != gen || s.mods.key != key || s.mods.dirs == nil {
		s.mods.gen, s.mods.key = gen, key
		s.mods.dirs = findModules(roots)
	}
	var out []string
	for _, d := range s.mods.dirs {
		if d != cur {
			out = append(out, d)
		}
	}
	return out
}

// findModules returns the folders with a gocpp.mod under the roots (not
// looking into hidden folders, testdata and the folders of the go command's
// kind, and at most maxIndexDirs folders in all).
func findModules(roots []string) []string {
	seen := make(map[string]bool)
	dirs := 0
	var visit func(dir string)
	visit = func(dir string) {
		if dirs >= maxIndexDirs || seen[dir] {
			return
		}
		seen[dir] = true
		dirs++
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			n := e.Name()
			switch {
			case e.IsDir():
				if !strings.HasPrefix(n, ".") && !strings.HasPrefix(n, "_") && n != "testdata" && n != "node_modules" {
					visit(filepath.Join(dir, n))
				}
			case n == "gocpp.mod":
				seen["mod:"+dir] = true
			}
		}
	}
	for _, root := range roots {
		visit(root)
	}
	var mods []string
	for k := range seen {
		if d, ok := strings.CutPrefix(k, "mod:"); ok {
			mods = append(mods, d)
		}
	}
	sort.Strings(mods)
	if mods == nil {
		mods = []string{}
	}
	return mods
}

// warnModules tells the user, once for each set of modules and request kind
// (always for a rename), that the search covers the module of the document
// only, when the workspace has other modules.
func (s *Server) warnModules(e env, what string, q *query, sym *symbol, every bool) {
	if sym.scope == scopeLocal || e.notify == nil {
		return
	}
	others := s.otherModules(q)
	if len(others) == 0 {
		return
	}
	cur := "this module"
	if q.lp.Module != nil {
		cur = "the module in " + q.lp.Module.Dir
	}
	msg := fmt.Sprintf("gocpp %s: only %s is searched. The workspace has other modules, whose uses of this name are not included: %s.", what, cur, strings.Join(others[:min(3, len(others))], ", "))
	if len(others) > 3 {
		msg += ", ..."
	}
	if !every {
		key := what + "\x00" + msg
		s.modsMu.Lock()
		told := s.modsTold[key]
		if s.modsTold == nil {
			s.modsTold = make(map[string]bool)
		}
		s.modsTold[key] = true
		s.modsMu.Unlock()
		if told {
			return
		}
	}
	if err := e.notify("window/showMessage", &protocol.ShowMessageParams{Type: protocol.Warning, Message: msg}); err != nil {
		e.log.Printf("window/showMessage: %v", err)
	}
}

// stdMethods are the methods of well-known standard interfaces: a method
// renamed to one of these names, with that signature, can make a type satisfy
// the interface, which changes what the standard library does with its values
// without a compile-time error (fmt calls String and Error, io.Copy calls
// Read and Write, sort.Sort calls Len, Less and Swap).
var stdMethods = map[string]struct {
	sig   string   // the signature, without the name
	iface string   // the interface that it can complete
	with  []string // the other methods that the type must have
	what  string   // what changes
}{
	"String": {"func() string", "fmt.Stringer", nil, "fmt prints the value with it"},
	"Error":  {"func() string", "error", nil, "the type can be used as an error, and fmt prints it with Error"},
	"Read":   {"func(p []byte) (n int, err error)", "io.Reader", nil, "io functions read from the value"},
	"Write":  {"func(p []byte) (n int, err error)", "io.Writer", nil, "io functions and fmt.Fprint write to the value"},
	"Close":  {"func() error", "io.Closer", nil, "code that closes an io.Closer closes the value"},
	"Len":    {"func() int", "sort.Interface", []string{"Less", "Swap"}, "sort.Sort accepts the value"},
	"Less":   {"func(i int, j int) bool", "sort.Interface", []string{"Len", "Swap"}, "sort.Sort accepts the value"},
	"Swap":   {"func(i int, j int)", "sort.Interface", []string{"Len", "Less"}, "sort.Sort accepts the value"},
}

// newImplements warns the user, with a showMessage, about the methods that the
// rename gives the name and signature of a method of a well-known standard
// interface, so that their types now satisfy it: the program compiles, and
// behaves differently.
func (rn *renamer) newImplements() {
	std, ok := stdMethods[rn.newName]
	if !ok || rn.e.notify == nil {
		return
	}
	renamed := make(map[*types.Func]bool)
	for _, h := range rn.res.hits {
		if fn, ok := h.obj.(*types.Func); ok && h.def {
			renamed[origin(fn).(*types.Func)] = true
		}
	}
	seen := make(map[string]bool)
	for fn := range renamed {
		sig, ok := fn.Type().(*types.Signature)
		rt := recvBase(fn)
		if !ok || sig.Recv() == nil || rt == nil || types.IsInterface(rt) || fn.Name() == rn.newName {
			continue
		}
		plain := types.NewSignatureType(nil, nil, nil, sig.Params(), sig.Results(), sig.Variadic())
		if !sigEqual(plain, std.sig) {
			continue
		}
		have := make(map[string]bool)
		ms := types.NewMethodSet(types.NewPointer(rt))
		for i := range ms.Len() {
			have[ms.At(i).Obj().Name()] = true
		}
		delete(have, fn.Name())
		ok = true
		for _, w := range std.with {
			ok = ok && have[w]
		}
		if !ok {
			continue
		}
		key := objectName(fn)
		if seen[key] {
			continue
		}
		seen[key] = true
		msg := fmt.Sprintf("gocpp rename: %s is now called %s and now implements %s: %s. This changes what the program does without a compile-time error.", objectName(fn), rn.newName, std.iface, std.what)
		if err := rn.e.notify("window/showMessage", &protocol.ShowMessageParams{Type: protocol.Warning, Message: msg}); err != nil {
			rn.e.log.Printf("window/showMessage: %v", err)
		}
	}
}

// sigEqual reports whether the signature sig, written without parameter
// names, is the one in want (which may name them).
func sigEqual(sig *types.Signature, want string) bool {
	typesOnly := func(t *types.Tuple) []string {
		var out []string
		for i := range t.Len() {
			out = append(out, types.TypeString(t.At(i).Type(), func(p *types.Package) string { return p.Name() }))
		}
		return out
	}
	got := "func(" + strings.Join(typesOnly(sig.Params()), ",") + ")(" + strings.Join(typesOnly(sig.Results()), ",") + ")"
	return got == wantTypes(want)
}

// wantTypes turns "func(p []byte) (n int, err error)" into
// "func([]byte,int)(...)" form: the types only, comma-joined.
func wantTypes(sig string) string {
	e, err := parser.ParseExpr(sig)
	ft, ok := e.(*ast.FuncType)
	if err != nil || !ok {
		return ""
	}
	list := func(fl *ast.FieldList) string {
		var out []string
		if fl != nil {
			for _, f := range fl.List {
				for range max(1, len(f.Names)) {
					out = append(out, types.ExprString(f.Type))
				}
			}
		}
		return strings.Join(out, ",")
	}
	return "func(" + list(ft.Params) + ")(" + list(ft.Results) + ")"
}
