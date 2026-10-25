package cppgen

import (
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/lower"
)

// The GC engine's emitter: the walking skeleton of job 16.1c (phase-16 README §1.3, §2.2 and
// §2.3 rule 7; the real lowering is steps 16.7 and 16.8).
//
// It is a separate, small path next to EmitProgram, not an engine threaded through internal/
// lower: internal/lower is built around the reference-counting engine's values (gp::ptr,
// gp::string, the type descriptors, interfaces, the interop probes, the phase gates), and
// threading a second engine through it is the multi-day refactor that steps 16.7/16.8 are for.
// The two paths share internal/cppir: this file builds the same tree and gets the same printer,
// its #line directives and its layout. The design question is reported for the ADR of 16.2.
//
// What it emits, and nothing else:
//   - a package main -> namespace gopkg_main (the plan's gopkg_<escaped path> rule, README
//     §2.2; the escaping is the one of gocpp's gpkg_ names, ADR-010, with the prefix changed);
//   - func declarations with int, string and bool parameters and one result, calls of the
//     package's own functions, := and var, assignment (also +=, -=, ...), integer arithmetic
//     and comparisons, shifts and division with Go's meaning (gp::gc::shl/shr/div/rem), string
//     comparison and +=, if/else and for (three-clause and condition-only), break and continue,
//     and the builtins print, println and len(string);
//   - Go's evaluation order: the arguments of a call and the operands of a binary expression are
//     evaluated left to right, through temporaries where C++ would not order them;
//   - Go's layout: int is int64_t, bool is bool, string is gp::gc::string (16 bytes, README
//     §2.2);
//   - #line directives at the .go lines, from the positions of the AST;
//   - a plain int main() that calls the package's main. No gp::run_main: that is the ARC
//     engine's wrapper, and the GC engine's runtime sources are the gp::gc ones.
//
// Every construct outside that subset is an error with the .go position, not silently different
// C++: func init (it would run before main, which this path does not do), methods, generics,
// imports, other types, switch, range, go, defer, pointers, slices, maps, interfaces, structs,
// closures and multiple results all say that a later job adds them. Two spike limits are errors
// too, not silent differences: a side effect on the right of && or || (short-circuit would have
// to be kept with a lambda), and a for post statement with more than one effect (it cannot be
// hoisted into the loop head). A division by zero or a negative shift count prints Go's panic
// line and exits with status 2 (arith.cpp's spike_panic; step 16.6 brings the real panic).

// A GCConfig says how EmitGo writes the C++.
type GCConfig struct {
	// OutputPath is the absolute path of the file that the C++ goes to, or "" for gocpp emit.
	// The lines that have no place in the .go source (the int main wrapper) get their #line
	// directives from it, as cppir does for the ARC engine's generated lines.
	OutputPath string
}

// EmitGo writes the C++ of the type-checked Go package pkg (its files, with the information of
// CheckGo) in the GC engine's world. A construct outside the subset of job 16.1c is an error
// that names its .go position.
func EmitGo(fset *token.FileSet, pkg *types.Package, info *types.Info, files []*ast.File, cfg GCConfig) ([]byte, error) {
	if len(files) == 0 {
		return nil, errors.New("cppgen: the GC engine has no files to emit")
	}
	e := &gcEmitter{fset: fset, pkg: pkg, info: info, ns: gcNamespace(pkg.Path())}
	file := &cppir.File{}
	file.Decls = append(file.Decls, &cppir.Include{Path: "gocpp/gc/rt.hpp", System: true})
	var decls []cppir.Decl
	var funcs []*cppir.Func
	mainSeen := false
	for _, f := range files {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv != nil {
					return nil, e.errf(d.Pos(), "the GC engine does not support methods yet (job 16.1c; step 16.8 adds them)")
				}
				if d.Name.Name == "init" {
					// Go runs init before main, in the order of the files (and of the imports); this
					// path emits no such call, and an emitted but never called init would be silent
					// wrong code. Step 16.8 lowers init with the rest of the package's startup.
					return nil, e.errf(d.Pos(), "the GC engine does not support func init yet (job 16.1c: it would not run before main; step 16.8 adds the package's initialization)")
				}
				if d.Name.Name == "main" {
					mainSeen = true
				}
				fn, err := e.funcDecl(d)
				if err != nil {
					return nil, err
				}
				funcs = append(funcs, fn)
			case *ast.GenDecl:
				if d.Tok == token.IMPORT {
					return nil, e.errf(d.Pos(), "the GC engine does not support imports yet (job 16.1c: Go source without imports; steps 16.9-16.13 add Go's standard library)")
				}
				return nil, e.errf(d.Pos(), "the GC engine does not support %s declarations yet (job 16.1c)", d.Tok)
			default:
				return nil, e.errf(d.Pos(), "the GC engine does not support this declaration yet (job 16.1c)")
			}
		}
	}
	if !mainSeen {
		return nil, e.errf(files[0].Package, "package main has no main function")
	}
	// A prototype for every function, then the definitions: Go lets a function be called before
	// its declaration and from another file, and C++ needs the declaration first.
	for _, fn := range funcs {
		proto := *fn
		proto.Body = nil
		decls = append(decls, &proto)
	}
	for _, fn := range funcs {
		decls = append(decls, fn)
	}
	file.Decls = append(file.Decls, &cppir.Namespace{
		Pos:   e.pos(files[0].Package),
		Name:  e.ns,
		Decls: decls,
		End:   e.pos(files[len(files)-1].End()),
	})
	// int main(): the GC engine's program entry, a plain C++ main function. The ARC engine's
	// gp::run_main (runtime/include/gocpp/main.hpp) starts the reference-counting runtime; the
	// GC engine has nothing to start yet (job 16.1d registers the main thread).
	file.Decls = append(file.Decls, &cppir.Func{
		Result: &cppir.TypeName{Name: "int"},
		Name:   "main",
		Body: &cppir.Block{Stmts: []cppir.Stmt{
			&cppir.ExprStmt{X: &cppir.Call{Fun: &cppir.Name{Name: e.ns + "::main"}}},
			&cppir.Return{X: &cppir.Lit{Text: "0"}},
		}},
	})
	return cppir.Print(file, cfg.OutputPath), nil
}

// gcNamespace returns the C++ namespace of a package of the GC engine: gopkg_ and the import
// path escaped as gocpp escapes its gpkg_ names (README §2.2; ADR-010 for the escaping). A
// program has one package main, so its namespace is always gopkg_main.
func gcNamespace(importPath string) string {
	return "gopkg_" + strings.TrimPrefix(lower.NamespaceName(importPath), "gpkg_")
}

// A gcEmitter writes the C++ of one Go package.
type gcEmitter struct {
	fset *token.FileSet
	pkg  *types.Package
	info *types.Info
	ns   string

	// temps numbers the temporaries that fix Go's evaluation order (temp). A user name that
	// starts with gp_ is escaped by lower.Escape, so gp_t0 can never be a Go name.
	temps int
}

// temp returns the name of the next temporary: gp_t0, gp_t1, ...
func (e *gcEmitter) temp() string {
	name := fmt.Sprintf("gp_t%d", e.temps)
	e.temps++
	return name
}

// A gcExpr is a C++ expression with the statements that must run before it, in Go's evaluation
// order: the arguments of a call and the operands of a binary expression go left to right, and
// C++ leaves both orders open. The statements declare temporaries, so they must be emitted in
// the block that holds the expression.
type gcExpr struct {
	pre []cppir.Stmt
	x   cppir.Expr
}

// effectful reports whether evaluating x has an effect that its order with another expression
// can be seen through: a call of print, println, a package function or an arithmetic helper
// (gp::gc::div, ...). A literal, a name, len(string) (gp::gc::len) and operators over pure
// operands are pure. A user function is taken as effectful: the spike has no purity analysis.
func effectful(x cppir.Expr) bool {
	switch x := x.(type) {
	case *cppir.Call:
		if name, ok := x.Fun.(*cppir.Name); ok && name.Name == "gp::gc::len" {
			for _, a := range x.Args {
				if effectful(a) {
					return true
				}
			}
			return false
		}
		return true
	case *cppir.Binary:
		return effectful(x.X) || effectful(x.Y)
	case *cppir.Unary:
		return effectful(x.X)
	case *cppir.Paren:
		return effectful(x.X)
	case *cppir.Cast:
		return effectful(x.X)
	}
	return false // Lit, Name
}

// hoist appends the preludes of v and, when the expression itself has an effect, declares it in
// a temporary: the temporary runs the effect at this point in Go's order, and the expression
// that uses it is pure. pos is the .go position of the temporary's line.
func (e *gcEmitter) hoist(v gcExpr, pos token.Pos) ([]cppir.Stmt, cppir.Expr) {
	pre, x := v.pre, v.x
	if effectful(x) {
		name := e.temp()
		pre = append(pre, &cppir.VarDecl{
			Pos:  e.pos(pos),
			Type: &cppir.TypeName{Name: "auto"},
			Vars: []cppir.Declarator{{Name: name, Init: x}},
		})
		x = &cppir.Name{Name: name}
	}
	return pre, x
}

// unwrapParen returns x without its outer parentheses: (x) is the same lvalue as x for the
// compound shift and division assignments, which write x twice.
func unwrapParen(x cppir.Expr) cppir.Expr {
	for {
		p, ok := x.(*cppir.Paren)
		if !ok {
			return x
		}
		x = p.X
	}
}

// withPre wraps st in a block with the prelude statements, when there are any.
func withPre(pre []cppir.Stmt, st cppir.Stmt, pos cppir.Pos) cppir.Stmt {
	if len(pre) == 0 {
		return st
	}
	return &cppir.Block{Pos: pos, Stmts: append(pre, st), End: pos}
}

// errf returns an error at pos (a .go position), prefixed with the position when it is valid.
func (e *gcEmitter) errf(pos token.Pos, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	if p := e.fset.Position(pos); p.IsValid() {
		return fmt.Errorf("%s: %s", p, msg)
	}
	return errors.New(msg)
}

// pos returns the cppir place of a .go position.
func (e *gcEmitter) pos(p token.Pos) cppir.Pos {
	if !p.IsValid() {
		return cppir.Pos{}
	}
	q := e.fset.Position(p)
	return cppir.Pos{File: q.Filename, Line: q.Line}
}

// typeName returns the C++ type of a Go type: int64_t, gp::gc::string or bool. The rest of Go's
// types come with step 16.5.
func (e *gcEmitter) typeName(t types.Type, pos token.Pos) (cppir.Type, error) {
	if b, ok := t.(*types.Basic); ok {
		switch b.Kind() {
		case types.Int, types.Int64, types.UntypedInt:
			return &cppir.TypeName{Name: "int64_t"}, nil
		case types.String, types.UntypedString:
			return &cppir.TypeName{Name: "gp::gc::string"}, nil
		case types.Bool, types.UntypedBool:
			return &cppir.TypeName{Name: "bool"}, nil
		}
	}
	return nil, e.errf(pos, "the GC engine supports only int, string and bool yet, not %s (job 16.1c; step 16.5 adds Go's other values)", types.TypeString(t, nil))
}

// funcDecl writes one function of the package.
func (e *gcEmitter) funcDecl(d *ast.FuncDecl) (*cppir.Func, error) {
	if d.Type.TypeParams != nil {
		return nil, e.errf(d.Pos(), "the GC engine does not support generic functions yet (job 16.1c; step 16.8 adds generics)")
	}
	obj, ok := e.info.Defs[d.Name].(*types.Func)
	if !ok {
		return nil, e.errf(d.Pos(), "internal error: no type information for function %s", d.Name.Name)
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok {
		return nil, e.errf(d.Pos(), "internal error: %s is not a function", d.Name.Name)
	}
	if sig.Variadic() {
		return nil, e.errf(d.Pos(), "the GC engine does not support variadic functions yet (job 16.1c)")
	}
	if d.Name.Name == "main" && (sig.Params().Len() != 0 || sig.Results().Len() != 0) {
		return nil, e.errf(d.Pos(), "func main must have no parameters and no results")
	}
	params := make([]cppir.Param, sig.Params().Len())
	for i := 0; i < sig.Params().Len(); i++ {
		v := sig.Params().At(i)
		t, err := e.typeName(v.Type(), d.Pos())
		if err != nil {
			return nil, err
		}
		name := lower.Escape(v.Name())
		if v.Name() == "" {
			name = fmt.Sprintf("gp_p%d", i)
		}
		params[i] = cppir.Param{Type: t, Name: name}
	}
	result := cppir.Type(&cppir.TypeName{Name: "void"})
	if sig.Results().Len() > 1 {
		return nil, e.errf(d.Pos(), "the GC engine does not support multiple results yet (job 16.1c; step 16.8 adds them)")
	}
	if sig.Results().Len() == 1 {
		v := sig.Results().At(0)
		if v.Name() != "" {
			return nil, e.errf(d.Pos(), "the GC engine does not support named results yet (job 16.1c)")
		}
		t, err := e.typeName(v.Type(), d.Pos())
		if err != nil {
			return nil, err
		}
		result = t
	}
	if d.Body == nil {
		return nil, e.errf(d.Pos(), "the GC engine does not support a function without a body (job 16.1c)")
	}
	e.temps = 0 // one gp_t0.. family per function
	body, err := e.block(d.Body)
	if err != nil {
		return nil, err
	}
	return &cppir.Func{
		Pos:    e.pos(d.Pos()),
		Result: result,
		Name:   lower.Escape(d.Name.Name),
		Params: params,
		Body:   body,
	}, nil
}

// block writes a block statement.
func (e *gcEmitter) block(b *ast.BlockStmt) (*cppir.Block, error) {
	out := &cppir.Block{Pos: e.pos(b.Lbrace), End: e.pos(b.Rbrace)}
	for _, s := range b.List {
		if _, ok := s.(*ast.EmptyStmt); ok {
			continue // a lone ";" (or an empty label body): nothing in C++
		}
		pre, st, err := e.stmt(s)
		if err != nil {
			return nil, err
		}
		out.Stmts = append(out.Stmts, pre...)
		if st != nil {
			out.Stmts = append(out.Stmts, st)
		}
	}
	return out, nil
}

// stmt writes one statement and returns the statements that must run before it, in Go's
// evaluation order. The caller (block) writes both in the same block, so a variable that the
// statement declares stays in scope for the statements after it.
func (e *gcEmitter) stmt(s ast.Stmt) ([]cppir.Stmt, cppir.Stmt, error) {
	switch s := s.(type) {
	case *ast.BlockStmt:
		b, err := e.block(s)
		return nil, b, err
	case *ast.ReturnStmt:
		if len(s.Results) > 1 {
			return nil, nil, e.errf(s.Pos(), "the GC engine does not support multiple results yet (job 16.1c)")
		}
		var v gcExpr
		if len(s.Results) == 1 {
			var err error
			if v, err = e.expr(s.Results[0]); err != nil {
				return nil, nil, err
			}
		}
		return v.pre, &cppir.Return{Pos: e.pos(s.Return), X: v.x}, nil
	case *ast.AssignStmt:
		return e.assign(s)
	case *ast.DeclStmt:
		return e.declStmt(s)
	case *ast.ExprStmt:
		v, err := e.expr(s.X)
		if err != nil {
			return nil, nil, err
		}
		return v.pre, &cppir.ExprStmt{Pos: e.pos(s.Pos()), X: v.x}, nil
	case *ast.IfStmt:
		st, err := e.ifStmt(s)
		return nil, st, err
	case *ast.ForStmt:
		st, err := e.forStmt(s)
		return nil, st, err
	case *ast.IncDecStmt:
		v, err := e.expr(s.X)
		if err != nil {
			return nil, nil, err
		}
		return v.pre, &cppir.ExprStmt{Pos: e.pos(s.Pos()), X: &cppir.Postfix{X: v.x, Op: s.Tok.String()}}, nil
	case *ast.BranchStmt:
		switch s.Tok {
		case token.BREAK:
			return nil, &cppir.Break{Pos: e.pos(s.Pos())}, nil
		case token.CONTINUE:
			return nil, &cppir.Continue{Pos: e.pos(s.Pos())}, nil
		}
		return nil, nil, e.errf(s.Pos(), "%s is not supported by the GC engine yet (job 16.1c)", s.Tok)
	}
	return nil, nil, e.errf(s.Pos(), "the GC engine does not support %s yet (job 16.1c)", gcStmtName(s))
}

// assign writes an assignment, including :=, which declares a variable with the type that
// go/types gave it. The left side of the subset is a name, so evaluating it twice for a
// compound shift or division (x = gp::gc::shl(x, y)) is harmless.
func (e *gcEmitter) assign(s *ast.AssignStmt) ([]cppir.Stmt, cppir.Stmt, error) {
	if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
		return nil, nil, e.errf(s.Pos(), "the GC engine supports only one value on each side of an assignment yet (job 16.1c)")
	}
	if s.Tok == token.DEFINE {
		id, ok := s.Lhs[0].(*ast.Ident)
		if !ok {
			return nil, nil, e.errf(s.Pos(), "the GC engine does not support this := target yet (job 16.1c)")
		}
		v, ok := e.info.Defs[id].(*types.Var)
		if !ok {
			return nil, nil, e.errf(s.Pos(), "internal error: no type information for %s", id.Name)
		}
		t, err := e.typeName(v.Type(), s.Pos())
		if err != nil {
			return nil, nil, err
		}
		rhs, err := e.expr(s.Rhs[0])
		if err != nil {
			return nil, nil, err
		}
		return rhs.pre, &cppir.VarDecl{Pos: e.pos(s.Pos()), Type: t, Vars: []cppir.Declarator{{Name: lower.Escape(id.Name), Init: rhs.x}}}, nil
	}
	lhs, err := e.expr(s.Lhs[0])
	if err != nil {
		return nil, nil, err
	}
	rhs, err := e.expr(s.Rhs[0])
	if err != nil {
		return nil, nil, err
	}
	pre := append(lhs.pre, rhs.pre...)
	// A shift, division or remainder compound assignment needs Go's operation (the shift count
	// and the zero divisor rules), not C++'s: x op= y becomes x = gp::gc::op(x, y).
	if helper, ok := gcArithAssign(s.Tok); ok {
		if len(lhs.pre) > 0 {
			return nil, nil, e.errf(s.Pos(), "the GC engine does not support this assignment target yet (job 16.1c)")
		}
		target := unwrapParen(lhs.x)
		if _, ok := target.(*cppir.Name); !ok {
			return nil, nil, e.errf(s.Pos(), "the GC engine does not support this assignment target yet (job 16.1c)")
		}
		call := &cppir.Call{Fun: &cppir.Name{Name: helper}, Args: []cppir.Expr{target, rhs.x}}
		return pre, &cppir.ExprStmt{Pos: e.pos(s.Pos()), X: &cppir.Assign{Op: "=", X: target, Y: call}}, nil
	}
	op, ok := gcAssignOp(s.Tok)
	if !ok {
		return nil, nil, e.errf(s.Pos(), "the GC engine does not support %s yet (job 16.1c)", s.Tok)
	}
	return pre, &cppir.ExprStmt{Pos: e.pos(s.Pos()), X: &cppir.Assign{Op: op, X: lhs.x, Y: rhs.x}}, nil
}

// declStmt writes a var declaration. A declaration without a value gets Go's zero value, which
// is C++'s {} for int, bool and gp::gc::string.
func (e *gcEmitter) declStmt(s *ast.DeclStmt) ([]cppir.Stmt, cppir.Stmt, error) {
	gd, ok := s.Decl.(*ast.GenDecl)
	if !ok || gd.Tok != token.VAR {
		return nil, nil, e.errf(s.Pos(), "the GC engine supports only var declarations here yet (job 16.1c)")
	}
	if len(gd.Specs) != 1 {
		return nil, nil, e.errf(s.Pos(), "the GC engine supports one variable per var declaration yet (job 16.1c)")
	}
	vs, ok := gd.Specs[0].(*ast.ValueSpec)
	if !ok || len(vs.Names) != 1 {
		return nil, nil, e.errf(s.Pos(), "the GC engine supports one variable per var declaration yet (job 16.1c)")
	}
	v, ok := e.info.Defs[vs.Names[0]].(*types.Var)
	if !ok {
		return nil, nil, e.errf(s.Pos(), "internal error: no type information for %s", vs.Names[0].Name)
	}
	t, err := e.typeName(v.Type(), s.Pos())
	if err != nil {
		return nil, nil, err
	}
	init := gcExpr{x: &cppir.InitList{}} // Go's zero value: int64_t x{}; gp::gc::string s{}; bool b{};
	switch len(vs.Values) {
	case 0:
	case 1:
		if init, err = e.expr(vs.Values[0]); err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, e.errf(s.Pos(), "the GC engine supports one value per var declaration yet (job 16.1c)")
	}
	return init.pre, &cppir.VarDecl{Pos: e.pos(s.Pos()), Type: t, Vars: []cppir.Declarator{{Name: lower.Escape(vs.Names[0].Name), Init: init.x}}}, nil
}

// ifStmt writes an if statement, with its optional init statement and the preludes of its
// condition in a block of its own (Go's init's scope is the if and its else, which the block
// covers).
func (e *gcEmitter) ifStmt(s *ast.IfStmt) (cppir.Stmt, error) {
	node := &cppir.If{Pos: e.pos(s.If)}
	var pre []cppir.Stmt
	if s.Init != nil {
		initPre, st, err := e.stmt(s.Init)
		if err != nil {
			return nil, err
		}
		pre = append(pre, initPre...)
		pre = append(pre, st)
	}
	cond, err := e.expr(s.Cond)
	if err != nil {
		return nil, err
	}
	pre = append(pre, cond.pre...)
	then, err := e.block(s.Body)
	if err != nil {
		return nil, err
	}
	node.Cond, node.Then = cond.x, then
	switch el := s.Else.(type) {
	case nil:
	case *ast.BlockStmt:
		if node.Else, err = e.block(el); err != nil {
			return nil, err
		}
	case *ast.IfStmt:
		if node.Else, err = e.ifStmt(el); err != nil {
			return nil, err
		}
	default:
		return nil, e.errf(s.Else.Pos(), "the GC engine does not support this else branch yet (job 16.1c)")
	}
	return withPre(pre, node, e.pos(s.If)), nil
}

// forStmt writes a for statement: for init; cond; post { } or for cond { }. When the condition
// needs preludes (a call inside it), the loop becomes for (init; ; post) { <preludes>
// if (!cond) break; body }, which keeps continue and break working.
func (e *gcEmitter) forStmt(s *ast.ForStmt) (cppir.Stmt, error) {
	node := &cppir.For{Pos: e.pos(s.For)}
	var pre []cppir.Stmt
	if s.Init != nil {
		initPre, st, err := e.stmt(s.Init)
		if err != nil {
			return nil, err
		}
		// The init's preludes go into the block that withPre writes around the loop; the init
		// statement itself stays in the for head, where its scope is the loop.
		pre = append(pre, initPre...)
		node.Init = st
	}
	cond := gcExpr{}
	if s.Cond != nil {
		var err error
		if cond, err = e.expr(s.Cond); err != nil {
			return nil, err
		}
	}
	if s.Post != nil {
		var err error
		if node.Post, err = e.forPost(s.Post); err != nil {
			return nil, err
		}
	}
	body, err := e.block(s.Body)
	if err != nil {
		return nil, err
	}
	if len(cond.pre) > 0 {
		then := &cppir.If{Pos: e.pos(s.Cond.Pos()), Cond: &cppir.Unary{Op: "!", X: cond.x}, Then: &cppir.Break{Pos: e.pos(s.Cond.Pos())}}
		stmts := append(append([]cppir.Stmt{}, cond.pre...), then)
		body = &cppir.Block{Pos: body.Pos, Stmts: append(stmts, body.Stmts...), End: body.End}
	} else {
		node.Cond = cond.x
	}
	node.Body = body
	return withPre(pre, node, e.pos(s.For)), nil
}

// forPost writes the post statement of a for loop as an expression. A post whose expression
// needs preludes (more than one effect) is an error: the preludes cannot go into the loop head.
func (e *gcEmitter) forPost(s ast.Stmt) (cppir.Expr, error) {
	switch s := s.(type) {
	case *ast.IncDecStmt:
		v, err := e.expr(s.X)
		if err != nil {
			return nil, err
		}
		if len(v.pre) > 0 {
			return nil, e.errf(s.Pos(), "the GC engine does not support a for post statement with more than one effect yet (job 16.1c)")
		}
		return &cppir.Postfix{X: v.x, Op: s.Tok.String()}, nil
	case *ast.AssignStmt:
		if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
			return nil, e.errf(s.Pos(), "the GC engine supports only one value on each side of an assignment yet (job 16.1c)")
		}
		lhs, err := e.expr(s.Lhs[0])
		if err != nil {
			return nil, err
		}
		rhs, err := e.expr(s.Rhs[0])
		if err != nil {
			return nil, err
		}
		if len(lhs.pre) > 0 || len(rhs.pre) > 0 {
			return nil, e.errf(s.Pos(), "the GC engine does not support a for post statement with more than one effect yet (job 16.1c)")
		}
		if helper, ok := gcArithAssign(s.Tok); ok {
			target := unwrapParen(lhs.x)
			if _, ok := target.(*cppir.Name); !ok {
				return nil, e.errf(s.Pos(), "the GC engine does not support this assignment target yet (job 16.1c)")
			}
			call := &cppir.Call{Fun: &cppir.Name{Name: helper}, Args: []cppir.Expr{target, rhs.x}}
			return &cppir.Assign{Op: "=", X: target, Y: call}, nil
		}
		op, ok := gcAssignOp(s.Tok)
		if !ok {
			return nil, e.errf(s.Pos(), "the GC engine does not support %s yet (job 16.1c)", s.Tok)
		}
		return &cppir.Assign{Op: op, X: lhs.x, Y: rhs.x}, nil
	case *ast.ExprStmt:
		v, err := e.expr(s.X)
		if err != nil {
			return nil, err
		}
		if len(v.pre) > 0 {
			return nil, e.errf(s.Pos(), "the GC engine does not support a for post statement with more than one effect yet (job 16.1c)")
		}
		return v.x, nil
	}
	return nil, e.errf(s.Pos(), "the GC engine does not support this for post statement yet (job 16.1c)")
}

// expr writes one expression with the statements that Go's evaluation order needs before it:
// the operands of a binary expression and the arguments of a call are sequenced through
// temporaries (gcExpr.pre), left to right, where C++ leaves the order open.
func (e *gcEmitter) expr(x ast.Expr) (gcExpr, error) {
	// -2^63 is INT64_MIN, which C++ cannot write as a negated literal: -(9223372036854775808)
	// and 0 - 9223372036854775808 are valid Go constants with that value, and their C++ literal
	// would be a too-large unsigned one (with a compiler warning). Only this value is
	// special-cased; no other constant is folded, so e.g. 7 << 3 stays a gp::gc::shl call.
	if tv, ok := e.info.Types[x]; ok && tv.Value != nil && tv.Value.Kind() == constant.Int {
		if v, ok := constant.Int64Val(tv.Value); ok && v == minInt64 {
			return gcExpr{x: &cppir.Lit{Text: "INT64_MIN"}}, nil
		}
	}
	switch x := x.(type) {
	case *ast.Ident:
		obj := e.info.Uses[x]
		if obj == nil {
			obj = e.info.Defs[x]
		}
		switch o := obj.(type) {
		case *types.Var:
			return gcExpr{x: &cppir.Name{Name: lower.Escape(o.Name())}}, nil
		case *types.Func:
			return gcExpr{x: &cppir.Name{Name: lower.Escape(o.Name())}}, nil
		case *types.Const:
			return e.constExpr(o, x.Pos())
		case *types.Nil:
			return gcExpr{x: &cppir.Lit{Text: "nullptr"}}, nil
		case *types.Builtin:
			return gcExpr{}, e.errf(x.Pos(), "the builtin %s must be called (job 16.1c)", o.Name())
		}
		return gcExpr{}, e.errf(x.Pos(), "the GC engine does not support the name %s here yet (job 16.1c)", x.Name)
	case *ast.BasicLit:
		return e.basicLit(x)
	case *ast.ParenExpr:
		inner, err := e.expr(x.X)
		if err != nil {
			return gcExpr{}, err
		}
		return gcExpr{pre: inner.pre, x: &cppir.Paren{X: inner.x}}, nil
	case *ast.BinaryExpr:
		return e.binary(x)
	case *ast.UnaryExpr:
		return e.unary(x)
	case *ast.CallExpr:
		return e.call(x)
	}
	return gcExpr{}, e.errf(x.Pos(), "the GC engine does not support this expression yet (job 16.1c)")
}

// basicLit writes a literal: an integer as int64_t(...) (Go's int is int64_t) and a string as
// gp::gc::lit("...") (static data, no copy). An integer literal that does not fit int64_t is an
// error, not a too-large C++ literal: Go allows 9223372036854775808 in a constant expression
// whose value fits (9223372036854775808 / 2), and C++ would make it an unsigned literal with
// the wrong sign (or reject 2^64). -2^63 itself is handled in expr before its literal is
// visited, so the three INT64_MIN forms still work.
func (e *gcEmitter) basicLit(x *ast.BasicLit) (gcExpr, error) {
	switch x.Kind {
	case token.INT:
		if tv, ok := e.info.Types[x]; ok && tv.Value != nil && tv.Value.Kind() == constant.Int {
			if _, ok := constant.Int64Val(tv.Value); !ok {
				return gcExpr{}, e.errf(x.Pos(), "the GC engine cannot represent the integer literal %s yet (job 16.1c)", x.Value)
			}
		}
		return gcExpr{x: &cppir.Lit{Text: "int64_t(" + gcIntLiteral(x.Value) + ")"}}, nil
	case token.STRING:
		s, err := strconv.Unquote(x.Value)
		if err != nil {
			return gcExpr{}, e.errf(x.Pos(), "%v", err)
		}
		return gcExpr{x: &cppir.Lit{Text: "gp::gc::lit(" + gcStringLiteral(s) + ")"}}, nil
	}
	return gcExpr{}, e.errf(x.Pos(), "the GC engine supports only integer and string literals yet (job 16.1c)")
}

// constExpr writes a named constant (true, false and Go's predeclared constants).
func (e *gcEmitter) constExpr(c *types.Const, pos token.Pos) (gcExpr, error) {
	if b, ok := c.Type().(*types.Basic); ok {
		switch b.Kind() {
		case types.Bool, types.UntypedBool:
			if constant.BoolVal(c.Val()) {
				return gcExpr{x: &cppir.Lit{Text: "true"}}, nil
			}
			return gcExpr{x: &cppir.Lit{Text: "false"}}, nil
		case types.Int, types.Int64, types.UntypedInt:
			v, ok := constant.Int64Val(c.Val())
			if !ok {
				return gcExpr{}, e.errf(pos, "the GC engine cannot represent the constant %s yet (job 16.1c)", c.Name())
			}
			return gcExpr{x: &cppir.Lit{Text: "int64_t(" + strconv.FormatInt(v, 10) + ")"}}, nil
		case types.String, types.UntypedString:
			return gcExpr{x: &cppir.Lit{Text: "gp::gc::lit(" + gcStringLiteral(constant.StringVal(c.Val())) + ")"}}, nil
		}
	}
	return gcExpr{}, e.errf(pos, "the GC engine supports only int, string and bool constants yet, not %s (job 16.1c)", c.Name())
}

// binary writes a binary expression. Its operands are evaluated left to right (through
// temporaries when they have effects); Go's &^ is a & ~b; Go's shifts and division are the
// gp::gc helpers, whose C++ operators differ; && and || keep C++'s short-circuit, so a right
// operand with an effect is an error (its preludes would run unconditionally).
func (e *gcEmitter) binary(x *ast.BinaryExpr) (gcExpr, error) {
	left, err := e.expr(x.X)
	if err != nil {
		return gcExpr{}, err
	}
	right, err := e.expr(x.Y)
	if err != nil {
		return gcExpr{}, err
	}
	switch x.Op {
	case token.LAND, token.LOR:
		if len(right.pre) > 0 || effectful(right.x) {
			return gcExpr{}, e.errf(x.Y.Pos(), "the GC engine does not support a side effect on the right of %s yet (job 16.1c: short-circuit would not be kept)", x.Op)
		}
		op, ok := gcBinaryOp(x.Op)
		if !ok {
			return gcExpr{}, e.errf(x.OpPos, "the GC engine does not support the operator %s yet (job 16.1c)", x.Op)
		}
		return gcExpr{pre: left.pre, x: &cppir.Binary{Op: op, X: left.x, Y: right.x}}, nil
	case token.AND_NOT:
		lpre, lx := e.hoist(left, x.X.Pos())
		rpre, rx := e.hoist(right, x.Y.Pos())
		return gcExpr{pre: append(lpre, rpre...), x: &cppir.Binary{Op: "&", X: lx, Y: &cppir.Unary{Op: "~", X: rx}}}, nil
	}
	if helper, ok := gcArithBinary(x.Op); ok {
		return e.callWith(helper, []gcExpr{left, right}, []token.Pos{x.X.Pos(), x.Y.Pos()}), nil
	}
	op, ok := gcBinaryOp(x.Op)
	if !ok {
		return gcExpr{}, e.errf(x.OpPos, "the GC engine does not support the operator %s yet (job 16.1c)", x.Op)
	}
	lpre, lx := e.hoist(left, x.X.Pos())
	rpre, rx := e.hoist(right, x.Y.Pos())
	return gcExpr{pre: append(lpre, rpre...), x: &cppir.Binary{Op: op, X: lx, Y: rx}}, nil
}

// unary writes a unary expression. Go's ^x (bitwise complement) is C++'s ~x. (INT64_MIN is
// handled in expr, from the constant value, for every form of -2^63.)
func (e *gcEmitter) unary(x *ast.UnaryExpr) (gcExpr, error) {
	inner, err := e.expr(x.X)
	if err != nil {
		return gcExpr{}, err
	}
	op, ok := gcUnaryOp(x.Op)
	if !ok {
		return gcExpr{}, e.errf(x.OpPos, "the GC engine does not support the operator %s yet (job 16.1c)", x.Op)
	}
	return gcExpr{pre: inner.pre, x: &cppir.Unary{Op: op, X: inner.x}}, nil
}

// call writes a call: the builtins print, println and len(string), a function of this package,
// or a conversion to int. The arguments are evaluated left to right, as Go does.
func (e *gcEmitter) call(x *ast.CallExpr) (gcExpr, error) {
	if x.Ellipsis.IsValid() {
		return gcExpr{}, e.errf(x.Pos(), "the GC engine does not support ... in a call yet (job 16.1c)")
	}
	var name string
	var conversion cppir.Type
	switch f := x.Fun.(type) {
	case *ast.Ident:
		switch o := e.info.Uses[f].(type) {
		case *types.Builtin:
			switch o.Name() {
			case "println":
				name = "gp::gc::println"
			case "print":
				name = "gp::gc::print"
			case "len":
				if len(x.Args) != 1 {
					return gcExpr{}, e.errf(x.Pos(), "len takes one argument")
				}
				if t := e.info.TypeOf(x.Args[0]); t == nil || !isStringType(t) {
					return gcExpr{}, e.errf(x.Pos(), "the GC engine supports len of a string only yet (job 16.1c)")
				}
				name = "gp::gc::len"
			default:
				return gcExpr{}, e.errf(x.Pos(), "the GC engine does not support the builtin %s yet (job 16.1c)", o.Name())
			}
		case *types.Func:
			if o.Pkg() != e.pkg {
				return gcExpr{}, e.errf(x.Pos(), "the GC engine does not support calls of imported functions yet (job 16.1c)")
			}
			name = lower.Escape(o.Name())
		case *types.TypeName:
			// A conversion, int(x): C++'s static_cast. Only int for now; a conversion to string
			// is Go's rune conversion and needs its runtime support (step 16.5).
			t, err := e.typeName(o.Type(), x.Pos())
			if err != nil {
				return gcExpr{}, err
			}
			if !isIntType(o.Type()) {
				return gcExpr{}, e.errf(x.Pos(), "the GC engine supports only conversions to int yet (job 16.1c)")
			}
			if len(x.Args) != 1 {
				return gcExpr{}, e.errf(x.Pos(), "a conversion takes one argument")
			}
			conversion = t
		default:
			return gcExpr{}, e.errf(x.Pos(), "the GC engine does not support this call yet (job 16.1c)")
		}
	case *ast.SelectorExpr:
		return gcExpr{}, e.errf(x.Pos(), "the GC engine does not support method and package-qualified calls yet (job 16.1c)")
	default:
		return gcExpr{}, e.errf(x.Pos(), "the GC engine does not support this call yet (job 16.1c)")
	}
	args := make([]gcExpr, len(x.Args))
	poss := make([]token.Pos, len(x.Args))
	for i, a := range x.Args {
		v, err := e.expr(a)
		if err != nil {
			return gcExpr{}, err
		}
		args[i], poss[i] = v, a.Pos()
	}
	if conversion != nil {
		return gcExpr{pre: args[0].pre, x: &cppir.Cast{Kind: "static_cast", Type: conversion, X: args[0].x}}, nil
	}
	return e.callWith(name, args, poss), nil
}

// callWith writes a call of name whose arguments are the gcExprs, sequenced left to right.
func (e *gcEmitter) callWith(name string, args []gcExpr, poss []token.Pos) gcExpr {
	var pre []cppir.Stmt
	cargs := make([]cppir.Expr, len(args))
	for i, a := range args {
		p, ax := e.hoist(a, poss[i])
		pre = append(pre, p...)
		cargs[i] = ax
	}
	return gcExpr{pre: pre, x: &cppir.Call{Fun: &cppir.Name{Name: name}, Args: cargs}}
}

// isIntType reports whether t is Go's int (or an untyped integer).
func isIntType(t types.Type) bool {
	b, ok := t.(*types.Basic)
	if !ok {
		return false
	}
	return b.Kind() == types.Int || b.Kind() == types.Int64 || b.Kind() == types.UntypedInt
}

// isStringType reports whether t is Go's string (or an untyped string).
func isStringType(t types.Type) bool {
	b, ok := t.(*types.Basic)
	if !ok {
		return false
	}
	return b.Kind() == types.String || b.Kind() == types.UntypedString
}

// gcIntLiteral returns a Go integer literal as a C++ one: the digit separators (1_000) go,
// and Go's 0o/0O octal prefix becomes C++'s 0. 0b, 0x and a leading 0 for octal are the same
// in both languages. The parser has already accepted the literal.
func gcIntLiteral(s string) string {
	s = strings.ReplaceAll(s, "_", "")
	if len(s) > 2 && s[0] == '0' && (s[1] == 'o' || s[1] == 'O') {
		s = "0" + s[2:]
	}
	return s
}

// gcStringLiteral returns the C++ string literal of the bytes s: printable ASCII as it is, and
// every other byte as an octal escape, so that no following character can be read as part of
// the escape.
func gcStringLiteral(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' || c == '"':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c == '\t':
			b.WriteString(`\t`)
		case c >= 0x20 && c < 0x7f:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, `\%03o`, c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// gcArithAssign returns the gp::gc helper of a compound assignment whose C++ operator does not
// have Go's meaning: the shift count rules and the zero divisor rules (arith.hpp). The others
// are plain C++ compound assignments.
func gcArithAssign(tok token.Token) (string, bool) {
	switch tok {
	case token.QUO_ASSIGN:
		return "gp::gc::div", true
	case token.REM_ASSIGN:
		return "gp::gc::rem", true
	case token.SHL_ASSIGN:
		return "gp::gc::shl", true
	case token.SHR_ASSIGN:
		return "gp::gc::shr", true
	}
	return "", false
}

// gcArithBinary returns the gp::gc helper of a binary operation whose C++ operator does not
// have Go's meaning (arith.hpp): /, %, << and >>.
func gcArithBinary(tok token.Token) (string, bool) {
	switch tok {
	case token.QUO:
		return "gp::gc::div", true
	case token.REM:
		return "gp::gc::rem", true
	case token.SHL:
		return "gp::gc::shl", true
	case token.SHR:
		return "gp::gc::shr", true
	}
	return "", false
}

// minInt64 is Go's smallest int, -2^63. expr writes it as INT64_MIN (C++ cannot write the
// negated literal) when a constant expression has that value.
const minInt64 = -1 << 63

// gcAssignOp returns the C++ assignment operator of a Go assignment token.
func gcAssignOp(tok token.Token) (string, bool) {
	switch tok {
	case token.ASSIGN:
		return "=", true
	case token.ADD_ASSIGN:
		return "+=", true
	case token.SUB_ASSIGN:
		return "-=", true
	case token.MUL_ASSIGN:
		return "*=", true
	case token.AND_ASSIGN:
		return "&=", true
	case token.OR_ASSIGN:
		return "|=", true
	case token.XOR_ASSIGN:
		return "^=", true
	}
	return "", false
}

// gcBinaryOp returns the C++ operator of a Go binary token.
func gcBinaryOp(tok token.Token) (string, bool) {
	switch tok {
	case token.ADD:
		return "+", true
	case token.SUB:
		return "-", true
	case token.MUL:
		return "*", true
	case token.AND:
		return "&", true
	case token.OR:
		return "|", true
	case token.XOR:
		return "^", true
	case token.LAND:
		return "&&", true
	case token.LOR:
		return "||", true
	case token.EQL:
		return "==", true
	case token.NEQ:
		return "!=", true
	case token.LSS:
		return "<", true
	case token.LEQ:
		return "<=", true
	case token.GTR:
		return ">", true
	case token.GEQ:
		return ">=", true
	}
	return "", false
}

// gcUnaryOp returns the C++ operator of a Go unary token. Go's ^ (bitwise complement) is C++'s
// ~. & (address) and * (dereference) need pointers, which step 16.5 adds.
func gcUnaryOp(tok token.Token) (string, bool) {
	switch tok {
	case token.ADD:
		return "+", true
	case token.SUB:
		return "-", true
	case token.NOT:
		return "!", true
	case token.XOR:
		return "~", true
	}
	return "", false
}

// gcStmtName returns the Go name of a statement for an error message.
func gcStmtName(s ast.Stmt) string {
	switch s.(type) {
	case *ast.SwitchStmt:
		return "a switch statement"
	case *ast.TypeSwitchStmt:
		return "a type switch"
	case *ast.SelectStmt:
		return "a select statement"
	case *ast.RangeStmt:
		return "a range loop"
	case *ast.GoStmt:
		return "a go statement"
	case *ast.DeferStmt:
		return "a defer statement"
	case *ast.SendStmt:
		return "a channel send"
	case *ast.LabeledStmt:
		return "a labeled statement"
	}
	return fmt.Sprintf("%T", s)
}
