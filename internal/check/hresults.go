package check

import (
	"context"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
)

// The calls that return an HRESULT (typed interop, ADR-137) are Go errors in the program. Check makes
// them so before it is done with the package, in the syntax tree: typed interop tells which calls
// they are (Config.HResults), and Check wraps each one in the conversion error(call), which the
// rewrite of the conversions of opaque values turns into gp_cgoh.As[error](call) like any other.
// That is the whole mechanism: go/types sees an expression of type error where the program has the
// call, so a variable declared from it has the type error everywhere, e.Error() is a string, the
// call goes into append and a map literal as an error, e = nil is Go's, and every misuse of the
// error is go/types' own message. The lowering writes gp::hresult_error(call) for the conversion.

// A HResultCalls says which calls of the package p, the result of a check of the package in which
// the conversions of opaque values are rewritten (the rewrite has converged), return the typedef
// HRESULT and are used as the error that Go sees: not dropped, not converted to a number, and not
// the argument of error(...) already. A hook that cannot tell (no compiler, a header that does not
// compile) returns nil and the error, and Check goes on without it.
type HResultCalls func(ctx context.Context, fset *token.FileSet, p *Package) ([]*ast.CallExpr, error)

// errorName is the name of Go's predeclared error type as a program writes it.
const errorName = "error"

// wrapHResults wraps each of the calls in the conversion error(call), as a rewrite does (the same
// gp_cgoh.As[error](call) is the result), at the place of the call, and reports whether there was
// any. The calls are those of the files of lp.
func (cv *converter) wrapHResults(calls []*ast.CallExpr) bool {
	wrapped := false
	for _, call := range calls {
		file := cv.fileOf(call)
		if file == nil {
			continue
		}
		name := cv.importName(file)
		pos, end := call.Pos(), call.End()
		cv.rememberText(call)
		inner := *call
		*call = ast.CallExpr{
			Fun: &ast.IndexExpr{
				X: &ast.SelectorExpr{
					X:   &ast.Ident{NamePos: pos, Name: name},
					Sel: &ast.Ident{NamePos: pos, Name: asName},
				},
				Lbrack: pos,
				// gp_cgoh.Error, not error: the program may declare its own error in the scope of the call
				Index: &ast.SelectorExpr{
					X:   &ast.Ident{NamePos: pos, Name: name},
					Sel: &ast.Ident{NamePos: pos, Name: errorTypeName},
				},
				Rbrack: pos,
			},
			Lparen: pos,
			Args:   []ast.Expr{&inner},
			Rparen: end - 1,
		}
		if cv.hresults == nil {
			cv.hresults = make(map[*ast.CallExpr]bool)
		}
		cv.hresults[call] = true
		wrapped = true
	}
	return wrapped
}

// fileOf returns the file of the package that holds the node n, or nil.
func (cv *converter) fileOf(n ast.Node) *ast.File {
	for _, f := range cv.lp.Files {
		if f.AST.FileStart <= n.Pos() && n.End() <= f.AST.FileEnd {
			return f.AST
		}
	}
	return nil
}

// unwrapErrorConversions turns each name.As[error](x) of msg, the wrapper of an HRESULT call, into x:
// the user wrote the call, and the message says it.
func unwrapErrorConversions(msg, name string) string {
	for _, spelled := range []string{errorName, name + "." + errorTypeName} {
		msg = unwrapErrorConversion(msg, name+"."+asName+"["+spelled+"](")
	}
	// the wrapper without its call: go/types names the function of the call in "2 variables but f returns 1 value"
	return strings.ReplaceAll(msg, name+"."+asName+"["+name+"."+errorTypeName+"]", calleeMarker)
}

// unwrapErrorConversion turns each prefix...) of msg into its argument.
func unwrapErrorConversion(msg, prefix string) string {
	from := 0
	for {
		i := strings.Index(msg[from:], prefix)
		if i < 0 {
			return msg
		}
		i += from
		from = i + 1
		if i > 0 && isNameByte(msg[i-1]) {
			continue
		}
		end := closing(msg, i+len(prefix))
		if end < 0 {
			continue
		}
		msg = msg[:i] + msg[i+len(prefix):end-1] + msg[end:]
		from = i
	}
}

// IsErrorConversion reports whether the call is error(x) of an opaque value x, rewritten to
// gp_cgoh.As[error](x): the wrapper of an HRESULT call, or one that the program wrote.
func (p *Package) IsErrorConversion(call *ast.CallExpr) bool {
	cv := p.Conversion(call)
	if cv == nil || cv.Kind != AsCall {
		return false
	}
	switch t := cv.Type.(type) {
	case *ast.Ident:
		// the universe error, not a type of the program's that is named error (type error uint8; error(n))
		return t.Name == errorName && p.Info != nil && p.Info.Uses[t] == types.Universe.Lookup(errorName)
	case *ast.SelectorExpr: // the wrapper of an HRESULT call: gp_cgoh.Error
		if id, ok := t.X.(*ast.Ident); ok && p.Info != nil {
			pn, ok := p.Info.Uses[id].(*types.PkgName)
			return ok && pn.Imported().Path() == load.CgohPath && t.Sel.Name == errorTypeName
		}
	}
	return false
}

// userExprText returns the text of the node n as the user wrote it: the loader's rewrite changed
// hresult.f(x) to C.f(x) for go/types.
func (cv *converter) userExprText(n ast.Node) string {
	user := types.ExprString(n.(ast.Expr))
	for _, f := range cv.lp.Files {
		if f.Rewrite == nil {
			continue
		}
		for _, u := range f.Rewrite.Uses {
			if u.Kind == load.HeaderUse && u.Sel != nil && n.Pos() <= u.Sel.Pos() && u.Sel.End() <= n.End() {
				user = strings.Replace(user, types.ExprString(u.Sel), u.Ident.Name+"."+u.Sel.Sel.Name, 1)
			}
		}
	}
	return user
}

// rememberText records how a message says the call, which the loader's rewrite changed (hresult.f(x)
// is C.f(x) for go/types): the message shows what the user wrote.
func (cv *converter) rememberText(call *ast.CallExpr) {
	text := types.ExprString(call)
	user := cv.userExprText(call)
	if user != text {
		if cv.userText == nil {
			cv.userText = make(map[string]string)
		}
		cv.userText[text] = user
	}
}

// calleeMarker stands in a message for the function of a wrapped call, where go/types names it
// without the call ("assignment mismatch: 2 variables but f returns 1 value"): the wrapper has no
// name of the user's, and hintHResults puts the callee's text at the place of the error.
const calleeMarker = "\x00callee\x00"

// calleeTexts resolves the calleeMarker of the messages of p.Errors: the error is at the wrapped call,
// and the message says its function as the user wrote it.
func (cv *converter) calleeTexts(p *Package) {
	found := false
	for _, d := range p.Errors.Diagnostics() {
		found = found || strings.Contains(d.Msg, calleeMarker)
	}
	if !found {
		return
	}
	at := make(map[token.Position]string)
	for call := range cv.hresults {
		if len(call.Args) == 1 {
			if inner, ok := call.Args[0].(*ast.CallExpr); ok {
				at[cv.fset.Position(call.Pos())] = cv.userExprText(inner.Fun)
			}
		}
	}
	p.Errors = p.Errors.Rewrite(func(d diag.Diagnostic) []diag.Diagnostic {
		if strings.Contains(d.Msg, calleeMarker) {
			// (The error is at the call, or at the parentheses round it, v, w := (f(1)): then the wrapper is
			// the first one that starts after the error, as the marker is in the message of that one.)
			callee, best := errorName, -1
			for pos, text := range at {
				if pos.Filename != d.Pos.Filename || pos.Offset < d.Pos.Offset {
					continue
				}
				if best < 0 || pos.Offset < best {
					callee, best = text, pos.Offset
				}
			}
			d.Msg = strings.ReplaceAll(d.Msg, calleeMarker, callee)
		}
		return []diag.Diagnostic{d}
	})
}

// userTexts turns the calls of msg that the rewrite of the loader changed back into what the user wrote.
func (cv *converter) userTexts(msg string) string {
	for text, user := range cv.userText {
		msg = strings.ReplaceAll(msg, text, user)
	}
	return msg
}

// HResultAdvice is the line that an error of go/types gets when the statement it is in has a call
// that returns an HRESULT, or a variable that holds the error of one: the program that was written for
// the number, compared with 0 or used in arithmetic, is what goes wrong.
const HResultAdvice = "the call returns an HRESULT, which is an error here (nil when the code is not negative): " +
	"compare it with nil; the code is int32(call) on the call itself, or HResult() through errors.As"

// hintHResults adds HResultAdvice to the errors of go/types in p that are in a statement (the header
// of a compound one) which has a wrapped call or a use of a variable that holds one.
func (cv *converter) hintHResults(p *Package) {
	if len(cv.hresults) == 0 || p.Info == nil {
		return
	}
	cv.calleeTexts(p)
	vars := cv.hresultVars(p)
	p.hresultCalls, p.hresultVars = cv.hresults, vars
	if !hasHardErrors(p.Errors) {
		cv.checkErrorTargets(p)
		return
	}
	type span struct {
		file     string
		from, to int
	}
	var spans []span
	for _, f := range cv.lp.Files {
		add := func(nodes ...ast.Node) {
			for _, n := range nodes {
				if n == nil || !cv.mentionsHResult(n, p, vars) {
					continue
				}
				from, to := cv.fset.Position(n.Pos()), cv.fset.Position(n.End())
				spans = append(spans, span{from.Filename, from.Offset, to.Offset})
			}
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt, *ast.ExprStmt, *ast.ReturnStmt, *ast.IncDecStmt, *ast.SendStmt, *ast.ValueSpec:
				add(s)
			case *ast.IfStmt:
				add(s.Init, s.Cond)
			case *ast.SwitchStmt:
				add(s.Init, s.Tag)
				if s.Tag != nil && cv.mentionsHResult(s.Tag, p, vars) {
					for _, cl := range s.Body.List { // the cases are measured against the tag
						for _, e := range cl.(*ast.CaseClause).List {
							from, to := cv.fset.Position(e.Pos()), cv.fset.Position(e.End())
							spans = append(spans, span{from.Filename, from.Offset, to.Offset})
						}
					}
				}
			case *ast.ForStmt:
				add(s.Init, s.Cond, s.Post)
			case *ast.RangeStmt:
				add(s.X)
			}
			return true
		})
	}
	if len(spans) == 0 {
		return
	}
	p.Errors = p.Errors.Rewrite(func(d diag.Diagnostic) []diag.Diagnostic {
		if d.Severity != diag.Error || !strings.Contains(strings.ToLower(d.Msg), "error") {
			return []diag.Diagnostic{d}
		}
		for _, sp := range spans {
			if d.Pos.Filename == sp.file && sp.from <= d.Pos.Offset && d.Pos.Offset <= sp.to {
				d.Related = append(d.Related, diag.Related{Msg: HResultAdvice})
				break
			}
		}
		return []diag.Diagnostic{d}
	})
}

// hresultVars returns the variables of p that hold the error of a wrapped call: the ones declared or
// assigned from one, or from such a variable.
func (cv *converter) hresultVars(p *Package) map[types.Object]bool {
	vars := make(map[types.Object]bool)
	holds := func(e ast.Expr) bool {
		e = ast.Unparen(e)
		if call, ok := e.(*ast.CallExpr); ok {
			return cv.hresults[call]
		}
		if id, ok := e.(*ast.Ident); ok {
			return vars[p.Info.Uses[id]]
		}
		return false
	}
	object := func(id *ast.Ident) types.Object {
		if o := p.Info.Defs[id]; o != nil {
			return o
		}
		return p.Info.Uses[id]
	}
	for changed := true; changed; {
		changed = false
		note := func(lhs ast.Expr, rhs ast.Expr) {
			if id, ok := ast.Unparen(lhs).(*ast.Ident); ok && holds(rhs) {
				if o := object(id); o != nil && !vars[o] {
					vars[o] = true
					changed = true
				}
			}
		}
		for _, f := range cv.lp.Files {
			ast.Inspect(f.AST, func(n ast.Node) bool {
				switch s := n.(type) {
				case *ast.AssignStmt:
					if len(s.Lhs) == len(s.Rhs) {
						for i := range s.Lhs {
							note(s.Lhs[i], s.Rhs[i])
						}
					}
				case *ast.ValueSpec:
					if len(s.Names) == len(s.Values) {
						for i := range s.Names {
							note(s.Names[i], s.Values[i])
						}
					}
				}
				return true
			})
		}
	}
	return vars
}

// mentionsHResult reports whether the node n has a wrapped call or a use of one of the variables.
func (cv *converter) mentionsHResult(n ast.Node, p *Package, vars map[types.Object]bool) bool {
	found := false
	ast.Inspect(n, func(m ast.Node) bool {
		switch m := m.(type) {
		case *ast.SelectorExpr:
			// hr.Error(), f().HResult(): the error is used as an error, whatever else is wrong in the statement
			switch x := ast.Unparen(m.X).(type) {
			case *ast.CallExpr:
				if cv.hresults[x] {
					return false
				}
			case *ast.Ident:
				if vars[p.Info.Uses[x]] {
					return false
				}
			}
		case *ast.CallExpr:
			found = found || cv.hresults[m]
		case *ast.Ident:
			found = found || vars[p.Info.Uses[m]] || vars[p.Info.Defs[m]]
		}
		return !found
	})
	return found
}

// isHResultError reports whether the expression e is the error of a call that returns an HRESULT
// (ADR-137): the wrapped call, or a variable that holds one.
func (p *Package) isHResultError(e ast.Expr) bool {
	switch e := ast.Unparen(e).(type) {
	case *ast.CallExpr:
		return p.hresultCalls[e]
	case *ast.Ident:
		return p.hresultVars[p.Info.Uses[e]]
	}
	return false
}

// checkErrorTargets reports the HRESULT errors of p that go where a C/C++ type is wanted: var n C.long =
// hr, n = f() with n a C variable, return f() in a function that returns a C type. go/types has no type
// for the target and accepts it; C++ would not, and the lowering says so at the build (cguard.go): the
// check says it too, with the same words. Only a package without any other error is looked at: then a
// type that go/types has none for is a C/C++ one.
func (cv *converter) checkErrorTargets(p *Package) {
	isError := func(e ast.Expr) bool {
		return p.isHResultError(e)
	}
	opaqueType := func(t types.Type) bool { return t == nil || !isValidType(t) }
	report := func(e ast.Expr, target ast.Expr, context string) {
		what := "value"
		if _, ok := ast.Unparen(e).(*ast.Ident); ok {
			what = "variable"
		}
		text := cv.fixMessage(types.ExprString(e))
		msg := "cannot use " + text + " (" + what + " of type error) as " + cv.fixMessage(types.ExprString(target)) + " value in " + context
		p.Errors.Add(diag.Diagnostic{
			Pos: cv.fset.Position(e.Pos()), End: cv.fset.Position(e.End()), Msg: msg,
			Related: []diag.Related{{Msg: HResultAdvice}},
		})
	}
	for _, f := range cv.lp.Files {
		var results []*ast.FieldList // the result lists of the enclosing functions
		var visit func(n ast.Node) bool
		visit = func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.FuncDecl:
				results = append(results, s.Type.Results)
				if s.Body != nil {
					ast.Inspect(s.Body, visit)
				}
				results = results[:len(results)-1]
				return false
			case *ast.FuncLit:
				results = append(results, s.Type.Results)
				ast.Inspect(s.Body, visit)
				results = results[:len(results)-1]
				return false
			case *ast.ValueSpec:
				if s.Type != nil && len(s.Names) == len(s.Values) && opaqueType(p.Info.TypeOf(s.Type)) {
					for _, v := range s.Values {
						if isError(v) {
							report(v, s.Type, "variable declaration")
						}
					}
				}
			case *ast.AssignStmt:
				if s.Tok == token.ASSIGN && len(s.Lhs) == len(s.Rhs) {
					for i, l := range s.Lhs {
						if isError(s.Rhs[i]) && !isBlankName(l) && opaqueType(p.Info.TypeOf(l)) {
							report(s.Rhs[i], l, "assignment")
						}
					}
				}
			case *ast.ReturnStmt:
				if len(results) == 0 || results[len(results)-1] == nil {
					return true
				}
				var want []ast.Expr
				for _, fld := range results[len(results)-1].List {
					for range max(1, len(fld.Names)) {
						want = append(want, fld.Type)
					}
				}
				if len(want) == len(s.Results) {
					for i, r := range s.Results {
						if isError(r) && opaqueType(p.Info.TypeOf(want[i])) {
							report(r, want[i], "return statement")
						}
					}
				}
			}
			return true
		}
		ast.Inspect(f.AST, visit)
	}
}

func isBlankName(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && id.Name == "_"
}

// IsHResultCall reports whether the call is the conversion error(f()) that Config.HResults made of
// the call f() (the wrapper, which stays the node of the call it replaced).
func (p *Package) IsHResultCall(call *ast.CallExpr) bool { return p.hresultCalls[call] }
