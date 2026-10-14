package cppgen

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
)

// Statements (Phase 02 README §2.4). This step has blocks, expression statements (calls),
// return, :=, var, and = with one value on each side; step 2.4 adds the rest (parallel and
// op assignments, ++ and --, if, for, switch, labels, goto). Each statement is written as C++
// lines mapped to the statement's .gp line.

// stmts writes the statements of a block.
func (fg *funcGen) stmts(list []ast.Stmt) {
	for _, s := range list {
		fg.stmt(s)
	}
}

func (fg *funcGen) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.EmptyStmt:
	case *ast.BlockStmt:
		fg.block(s)
	case *ast.ExprStmt:
		fg.line(s.Pos(), fg.expr(s.X).text+";")
	case *ast.ReturnStmt:
		fg.returnStmt(s)
	case *ast.AssignStmt:
		fg.assignStmt(s)
	case *ast.DeclStmt:
		fg.declStmt(s)
	default:
		fg.g.unsupported(s.Pos(), nodeKind(s))
	}
}

// line writes one line of C++ mapped to the .gp line of pos.
func (fg *funcGen) line(pos token.Pos, text string) {
	fg.g.w.line(fg.g.pos(pos), text)
}

// block writes a Go block as a C++ block, which is a scope in both languages.
func (fg *funcGen) block(b *ast.BlockStmt) {
	fg.line(b.Lbrace, "{")
	fg.g.w.indent()
	fg.names.openScope()
	fg.stmts(b.List)
	fg.names.closeScope()
	fg.g.w.dedent()
	fg.line(b.Rbrace, "}")
}

// returnStmt writes return. Several results are a std::tuple, which `return {a, b};` makes
// from its values left to right; a bare return in a function with named results returns them.
func (fg *funcGen) returnStmt(s *ast.ReturnStmt) {
	switch {
	case len(s.Results) == 0 && len(fg.fn.results) == 0:
		fg.line(s.Pos(), "return;")
	case len(s.Results) == 0:
		fg.line(s.Pos(), "return "+braceList(fg.fn.results)+";")
	case len(s.Results) == 1:
		// One result, or a call whose results are the function's: return f();
		fg.line(s.Pos(), "return "+fg.expr(s.Results[0]).text+";")
	default:
		fg.line(s.Pos(), "return {"+fg.exprs(s.Results, false)+"};")
	}
}

// braceList returns the names as the value of a return: x, or {x, y}.
func braceList(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	s := "{"
	for i, n := range names {
		if i > 0 {
			s += ", "
		}
		s += n
	}
	return s + "}"
}

func (fg *funcGen) assignStmt(s *ast.AssignStmt) {
	switch {
	case s.Tok == token.DEFINE:
		fg.declare(s.Pos(), s.Lhs, s.Rhs)
	case s.Tok == token.ASSIGN && len(s.Lhs) == 1 && len(s.Rhs) == 1:
		var t target
		if !isBlank(s.Lhs[0]) {
			t.name = fg.expr(s.Lhs[0]).text
		}
		fg.line(s.Pos(), t.assign(fg.expr(s.Rhs[0]).text))
	default:
		fg.g.unsupported(s.Pos(), "the assignment "+s.Tok.String()+" with these operands (step 2.4)")
	}
}

func (fg *funcGen) declStmt(s *ast.DeclStmt) {
	d := s.Decl.(*ast.GenDecl)
	switch d.Tok {
	case token.CONST:
		// Every use of a constant is folded: nothing to write.
	case token.VAR:
		for _, spec := range d.Specs {
			spec := spec.(*ast.ValueSpec)
			lhs := make([]ast.Expr, len(spec.Names))
			for i, id := range spec.Names {
				lhs[i] = id
			}
			fg.declare(spec.Pos(), lhs, spec.Values)
		}
	default:
		fg.g.unsupported(s.Pos(), "local type declarations (Phase 03)")
	}
}

// isBlank reports whether e is the blank identifier _.
func isBlank(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "_"
}

// A target is the left side of one value in a declaration or an assignment.
type target struct {
	name string // the C++ name, or "" for _
	typ  string // the C++ type of a new variable, or "" for an existing one
}

// assign returns the C++ statement that gives t the value: T x = value; for a new variable,
// x = value; for an existing one, and for _ a statement that only evaluates it.
func (t target) assign(value string) string {
	switch {
	case t.name == "":
		return "static_cast<void>(" + value + ");"
	case t.typ != "":
		return t.typ + " " + t.name + " = " + value + ";"
	}
	return t.name + " = " + value + ";"
}

// declare writes a := or a var declaration of the names lhs with the values (none for var x T).
// For :=, some of the names may be variables that exist already: they are assigned.
//
// Each new variable gets an explicit C++ type: x := e is `int64_t x = e;`, and var x T is
// `T x{};`, the zero value. A new variable whose name the values use (v := v) gets a new C++
// name, because in C++ the new name is in scope in its own initializer (funcNames.declare).
func (fg *funcGen) declare(pos token.Pos, lhs, values []ast.Expr) {
	used := fg.usedNames(values)
	targets := make([]target, len(lhs))
	for i, e := range lhs {
		id := e.(*ast.Ident)
		if id.Name == "_" {
			continue
		}
		if obj, ok := fg.g.info.Defs[id].(*types.Var); ok {
			targets[i] = target{name: fg.names.declare(obj, used), typ: fg.g.cxxType(obj.Type(), id.Pos())}
		} else {
			targets[i] = target{name: fg.expr(id).text} // redeclared by :=
		}
	}
	if len(values) == 0 {
		for _, t := range targets {
			if t.name != "" {
				fg.line(pos, t.typ+" "+t.name+"{};")
			}
		}
		return
	}
	fg.bind(pos, targets, values)
}

// bind writes the lines that give the targets their values, with Go's order: all values are
// evaluated before any target changes.
//   - One value with several results (a, b := f()): the result tuple goes into a temporary,
//     and each target gets its element.
//   - As many values as targets: in order, directly, if every target is new (or _): a value
//     can't use a variable that the same statement declares, which then comes into scope only
//     in C++ (declare renames it). If some targets exist already, all values go into
//     temporaries first, because a value can use an old target.
func (fg *funcGen) bind(pos token.Pos, targets []target, values []ast.Expr) {
	if len(values) == 1 && len(targets) > 1 {
		tmp := fg.names.temp("t")
		fg.line(pos, "auto "+tmp+" = "+fg.exactExpr(values[0]).text+";")
		for i, t := range targets {
			if t.name != "" {
				fg.line(pos, t.assign("std::get<"+strconv.Itoa(i)+">("+tmp+")"))
			}
		}
		return
	}
	direct := true
	for _, t := range targets {
		if t.name != "" && t.typ == "" {
			direct = false
		}
	}
	if direct || len(targets) == 1 {
		for i, t := range targets {
			fg.line(pos, t.assign(fg.expr(values[i]).text))
		}
		return
	}
	tmps := make([]string, len(values))
	for i, v := range values {
		tmps[i] = fg.names.temp("t")
		fg.line(pos, "auto "+tmps[i]+" = "+fg.exactExpr(v).text+";")
	}
	for i, t := range targets {
		if t.name != "" {
			fg.line(pos, t.assign(tmps[i]))
		}
	}
}

// usedNames returns the C++ names of the variables and functions that the expressions use.
func (fg *funcGen) usedNames(list []ast.Expr) map[string]bool {
	used := make(map[string]bool)
	for _, e := range list {
		ast.Inspect(e, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				switch obj := fg.g.info.Uses[id].(type) {
				case *types.Var, *types.Func:
					if name, ok := fg.names.names[obj]; ok {
						used[name] = true
					} else if name, ok := fg.g.names[obj]; ok {
						used[name] = true
					}
				}
			}
			return true
		})
	}
	return used
}
