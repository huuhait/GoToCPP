package vet

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/huuhait/gocpp/internal/check"
)

// The analyzers about the Phase 10 forms: `go` in must functions, a wrap macro's #body() in a go
// statement, and lazy parameters. They read check.Package.Must and load.Package.Macros through
// inputsAnalyzer, because the trees that they see are plain Go (ADR-037, ADR-075).

// gobangAnalyzer finds the go statements that drop the error of a `!`.
//
// A `!` hands its error to the caller of the function that holds it. Another goroutine has no
// caller to hand it to, so `go` drops what the function returns (Phase 10 README §2.1: the error
// of `go f()` "isn't handled"). Reported:
//
//   - go func() error { ... F!(...) ... }(): the `!` returns from the func literal, and `go` throws
//     the error away. Anywhere.
//   - go f(...) in a must function, when f's last result is error: the must function handles the
//     errors of its calls, but not this one. (`go F!(...)` is a compile error already.)
var gobangAnalyzer = &analysis.Analyzer{
	Name:             "gobang",
	Doc:              "report go statements that drop the error of a ! (go func() error { ... F!() ... }(), go f() in a must function)",
	Requires:         []*analysis.Analyzer{inputsAnalyzer},
	RunDespiteErrors: true,
	Run:              runGobang,
}

func runGobang(pass *analysis.Pass) (any, error) {
	in := inputs(pass)
	if in.must == nil {
		return nil, nil
	}
	info := pass.TypesInfo
	// The func literals that have a `!` of their own that returns the error.
	litBang := make(map[*ast.FuncLit]bool)
	for _, c := range in.must.Calls {
		if lit, ok := c.Func.(*ast.FuncLit); ok && c.Kind == check.MustReturn {
			litBang[lit] = true
		}
	}
	mustDecl := make(map[*ast.FuncDecl]string) // the name the user wrote
	for _, f := range in.must.Funcs {
		mustDecl[f.Decl] = f.Name
		if f.Decl.Recv != nil {
			mustDecl[f.Decl] = funcName(f.Decl)
		}
	}
	reported := make(map[*ast.GoStmt]bool)
	report := func(s *ast.GoStmt, format string, args ...any) {
		if !reported[s] {
			reported[s] = true
			pass.Report(analysis.Diagnostic{Pos: s.Pos(), End: s.End(), Category: "gobang", Message: fmt.Sprintf(format, args...)})
		}
	}
	// walk visits the go statements of body that belong to the function itself (not to the func
	// literals in it); enclosing says which function that is.
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			s, ok := n.(*ast.GoStmt)
			if !ok {
				return true
			}
			if lit, ok := ast.Unparen(s.Call.Fun).(*ast.FuncLit); ok && litBang[lit] {
				report(s, "go func() error { ... ! ... }(): the ! returns its error from the func literal, and go drops what the function returns, so nobody sees it; handle the error inside the literal (send it on a channel, log it, or use an errgroup)")
			}
			return true
		})
	}
	for _, f := range pass.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			name, isMust := mustDecl[fd]
			if !ok || fd.Body == nil || !isMust {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.FuncLit:
					return false // its own function
				case *ast.GoStmt:
					if lastIsError(info.Types[n.Call.Fun].Type) && !ignorable(info, n.Call, "") {
						report(n, "go %s in the must function %s: the error that it returns is dropped, because a goroutine can't return it from %s; handle the error in the goroutine (send it on a channel, log it, or use an errgroup)",
							callText(n.Call), name, name)
					}
				}
				return true
			})
		}
	}
	return nil, nil
}

// callText is the call of a go statement as a message shows it: f(...), x.m(...), func() error {...}().
func callText(call *ast.CallExpr) string {
	if lit, ok := ast.Unparen(call.Fun).(*ast.FuncLit); ok {
		return "func" + strings.TrimPrefix(types.ExprString(lit.Type), "func") + " {...}()"
	}
	return types.ExprString(call.Fun) + "(...)"
}

// lastIsError reports whether t is a function type whose last result is exactly error.
func lastIsError(t types.Type) bool {
	if t == nil {
		return false
	}
	sig, ok := t.Underlying().(*types.Signature)
	if !ok || sig.Results().Len() == 0 {
		return false
	}
	return isErrorType(sig.Results().At(sig.Results().Len() - 1).Type())
}

var errorType = types.Universe.Lookup("error").Type()

func isErrorType(t types.Type) bool { return types.Identical(t, errorType) }

// macroDecl returns the function declaration that a macro became (ADR-075: a function with the
// macro's name and hidden parameters), or nil.
func macroDecl(pass *analysis.Pass, name string) *ast.FuncDecl {
	for _, f := range pass.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == name && fd.Body != nil {
				return fd
			}
		}
	}
	return nil
}

// paramObj returns the object of the parameter of fd that is called name, or nil.
func paramObj(info *types.Info, fd *ast.FuncDecl, name string) types.Object {
	for _, field := range fd.Type.Params.List {
		for _, id := range field.Names {
			if id.Name == name {
				return info.Defs[id]
			}
		}
	}
	return nil
}

// unusedlazyAnalyzer finds lazy macro parameters that the macro never uses.
//
// A lazy parameter is not evaluated at the use: the macro calls msg() when it wants the value
// (Phase 10 README §2.2). One that the macro never calls is a mistake: the caller's argument is
// never evaluated, so the parameter does nothing. A parameter that the macro only gives to #text
// is not reported: the text of the argument is a use.
var unusedlazyAnalyzer = &analysis.Analyzer{
	Name:             "unusedlazy",
	Doc:              "report lazy macro parameters that the macro never uses",
	Requires:         []*analysis.Analyzer{inputsAnalyzer},
	RunDespiteErrors: true,
	Run:              runUnusedLazy,
}

func runUnusedLazy(pass *analysis.Pass) (any, error) {
	in := inputs(pass)
	info := pass.TypesInfo
	for _, name := range sortedKeys(in.macros) {
		sig := in.macros[name]
		fd := macroDecl(pass, sig.Name)
		if fd == nil {
			continue
		}
		for _, p := range sig.Params {
			if !p.Lazy || slices.Contains(sig.Text, p.Name) {
				continue
			}
			obj := paramObj(info, fd, p.Name)
			if obj == nil || p.Name == "_" {
				continue
			}
			used := false
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && info.Uses[id] == obj {
					used = true
				}
				return !used
			})
			if !used {
				pass.Report(analysis.Diagnostic{
					Pos: obj.Pos(), End: obj.Pos() + token.Pos(len(p.Name)), Category: "unusedlazy",
					Message: fmt.Sprintf("the lazy parameter %s of macro %s is never used: its argument is not evaluated at the use, and nothing calls it; remove the parameter, or call %s() where the value is needed", p.Name, sig.Name, p.Name),
				})
			}
		}
	}
	return nil, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// unwaitedbodyAnalyzer finds a wrap macro that runs #body() in a go statement and does not wait
// for it.
//
// #body() runs the code of the function the macro is on (Phase 10 README §2.2). A macro that
// starts it in a goroutine must wait for it before it returns (InThread does:
// done := make(chan struct{}); go func() { defer close(done); #body() }(); <-done), else the
// function returns before its code has run, with zero results. The analyzer looks at the go
// statements of a wrap macro whose goroutine calls the body, and what the goroutine signals:
// close(ch), ch <- v, wg.Done(). It reports the go statement when the goroutine signals nothing,
// or when the macro never receives from that channel (<-ch, range ch, select) or calls Wait on
// that WaitGroup. A goroutine that signals through something the analysis can't name (a result
// of a call, an element of a slice) is not reported.
var unwaitedbodyAnalyzer = &analysis.Analyzer{
	Name:             "unwaitedbody",
	Doc:              "report #body() started in a go statement that the wrap macro does not wait for",
	Requires:         []*analysis.Analyzer{inputsAnalyzer},
	RunDespiteErrors: true,
	Run:              runUnwaitedBody,
}

func runUnwaitedBody(pass *analysis.Pass) (any, error) {
	in := inputs(pass)
	info := pass.TypesInfo
	for _, name := range sortedKeys(in.macros) {
		sig := in.macros[name]
		if !sig.Wrap {
			continue
		}
		fd := macroDecl(pass, sig.Name)
		if fd == nil {
			continue
		}
		body := paramObj(info, fd, "gp_body")
		if body == nil {
			continue
		}
		isBody := func(n ast.Node) *ast.CallExpr {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return nil
			}
			if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok && info.Uses[id] == body {
				return call
			}
			return nil
		}
		given := argumentKeys(info, fd.Body)
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			gs, ok := n.(*ast.GoStmt)
			if !ok {
				return true
			}
			var bodyCall *ast.CallExpr
			ast.Inspect(gs.Call, func(m ast.Node) bool {
				if c := isBody(m); c != nil && bodyCall == nil {
					bodyCall = c
				}
				return bodyCall == nil
			})
			if bodyCall == nil {
				return true
			}
			signals, unknown := signalKeys(info, gs.Call)
			if unknown {
				return true
			}
			// A channel or WaitGroup that is given to a call can be signaled or waited for there.
			for _, k := range signals {
				if given[k] {
					return true
				}
			}
			waits, unknownWait := waitKeys(info, fd.Body)
			if unknownWait {
				return true
			}
			if len(signals) == 0 {
				pass.Report(analysis.Diagnostic{
					Pos: bodyCall.Pos(), End: bodyCall.End(), Category: "unwaitedbody",
					Message: fmt.Sprintf("#body() runs in a go statement of macro %s, and the goroutine tells nobody when it is done (no close(ch), ch <- v or wg.Done()): the macro returns before the function's code has run; wait for it, as InThread does", sig.Name),
				})
				return true
			}
			for _, k := range signals {
				if waits[k] {
					return true
				}
			}
			pass.Report(analysis.Diagnostic{
				Pos: bodyCall.Pos(), End: bodyCall.End(), Category: "unwaitedbody",
				Message: fmt.Sprintf("#body() runs in a go statement of macro %s that nothing waits for: the goroutine signals through %s, but the macro never receives from it or calls Wait before it returns, so the function returns before its code has run; wait for it, as InThread does (<-done)", sig.Name, signals[0].Name()),
			})
			return true
		})
	}
	return nil, nil
}

// argumentKeys returns the variables that the body gives to a call as an argument (not to close,
// len or cap): what the callee does with a channel or a WaitGroup is out of sight.
func argumentKeys(info *types.Info, body ast.Node) map[types.Object]bool {
	given := make(map[types.Object]bool)
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
			if _, builtin := info.Uses[id].(*types.Builtin); builtin && (id.Name == "close" || id.Name == "len" || id.Name == "cap") {
				return true
			}
		}
		for _, a := range call.Args {
			if k := keyObj(info, a); k != nil {
				given[k] = true
			}
		}
		return true
	})
	return given
}

// keyObj names the variable or field that expr stands for: x, x.f, &x, *x, x[i] (of x). nil when
// the analysis can't tell.
func keyObj(info *types.Info, expr ast.Expr) types.Object {
	for {
		switch e := expr.(type) {
		case *ast.ParenExpr:
			expr = e.X
		case *ast.StarExpr:
			expr = e.X
		case *ast.UnaryExpr:
			if e.Op != token.AND {
				return nil
			}
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.Ident:
			return info.ObjectOf(e)
		case *ast.SelectorExpr:
			if sel := info.Selections[e]; sel != nil {
				return sel.Obj()
			}
			return info.Uses[e.Sel]
		default:
			return nil
		}
	}
}

// signalKeys returns what the goroutine signals through: the channels it closes or sends to, the
// WaitGroups it calls Done on. unknown is true when it signals through something that has no name.
func signalKeys(info *types.Info, root ast.Node) (keys []types.Object, unknown bool) {
	add := func(e ast.Expr) {
		if k := keyObj(info, e); k != nil {
			keys = append(keys, k)
		} else {
			unknown = true
		}
	}
	ast.Inspect(root, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SendStmt:
			add(n.Chan)
		case *ast.CallExpr:
			if id, ok := ast.Unparen(n.Fun).(*ast.Ident); ok && id.Name == "close" && len(n.Args) == 1 {
				if _, ok := info.Uses[id].(*types.Builtin); ok {
					add(n.Args[0])
				}
			}
			if sel, ok := ast.Unparen(n.Fun).(*ast.SelectorExpr); ok && sel.Sel.Name == "Done" && isWaiter(info.Types[sel.X].Type) {
				add(sel.X)
			}
		}
		return true
	})
	return keys, unknown
}

// waitKeys returns what the macro body waits on outside the go statements: <-ch, range ch
// (select receives are <-ch too), wg.Wait(). A wait inside a goroutine is not the macro's wait.
func waitKeys(info *types.Info, body ast.Node) (keys map[types.Object]bool, unknown bool) {
	keys = make(map[types.Object]bool)
	add := func(e ast.Expr) {
		if k := keyObj(info, e); k != nil {
			keys[k] = true
		} else {
			unknown = true
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.GoStmt:
			return false // a wait inside another goroutine is not the macro's
		case *ast.UnaryExpr:
			if n.Op == token.ARROW {
				add(n.X)
			}
		case *ast.RangeStmt:
			if t := info.Types[n.X].Type; t != nil {
				if _, ok := t.Underlying().(*types.Chan); ok {
					add(n.X)
				}
			}
		case *ast.CallExpr:
			if sel, ok := ast.Unparen(n.Fun).(*ast.SelectorExpr); ok && sel.Sel.Name == "Wait" && isWaiter(info.Types[sel.X].Type) {
				add(sel.X)
			}
		}
		return true
	})
	return keys, unknown
}

// isWaiter reports whether t is a sync.WaitGroup or a *sync.WaitGroup, or the errgroup kind
// of type with Done/Wait methods.
func isWaiter(t types.Type) bool {
	if t == nil {
		return false
	}
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := t.(*types.Named)
	if !ok || n.Obj().Pkg() == nil {
		return false
	}
	return n.Obj().Pkg().Path() == "sync" && n.Obj().Name() == "WaitGroup"
}
