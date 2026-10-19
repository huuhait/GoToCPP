package vet

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

// ignorederrAnalyzer finds a call statement that drops an error, in a package that uses `!`.
//
// A package that writes `!` has chosen to handle errors visibly: a must function returns every error
// its code drops, and `F!(...)` returns or panics with the error of one call. In such a package a call
// statement that throws its error away, in a function whose last result is error or that has a `!`
// of its own, is either a forgotten check or a decision to write down: `_ = os.Remove(path)`. (Inside
// a must function the compiler handles the dropped errors itself, so only the func literals in it are
// checked: they are separate functions.) A package without `!` is plain Go, and go vet has no check
// like this one, so nothing is reported there.
//
// Only an expression statement counts (an assignment to _ is a decision, and so is a deferred or go
// call), and only a call whose last result is exactly error. Not reported: a call that comes right
// before a return of something other than nil in a function that returns an error (the cleanup on a
// failure path: if err != nil { f.Close(); return err }), and the calls whose errors are not worth a
// check: the fmt printers (Print, Printf, Println, Fprint, Fprintf, Fprintln), the Write methods of
// strings.Builder and bytes.Buffer (they never fail) and of bufio.Writer (the error is sticky and
// comes back from Flush), the Write of a hash, and math/rand's Read. (gobang, which looks at `go f()`,
// does not make that exception: a goroutine that drops the error of a printer is not reported either
// way, only calls whose result type ends in error are looked at there.) In a function whose `!` panics
// the advice "write f!(...)" means a panic on that error too.
var ignorederrAnalyzer = &analysis.Analyzer{
	Name:             "ignorederr",
	Doc:              "report a call whose error result is dropped in a function that returns an error (handle it, write _ = f(), or use f!())",
	Requires:         []*analysis.Analyzer{inputsAnalyzer},
	RunDespiteErrors: true,
	Run:              runIgnoredErr,
}

func runIgnoredErr(pass *analysis.Pass) (any, error) {
	in := inputs(pass)
	if in.must == nil {
		return nil, nil // a package that does not use ! has chosen plain Go error handling
	}
	info := pass.TypesInfo
	// The functions that have a `!` of their own: the author handles errors that way there.
	bang := make(map[ast.Node]bool)
	if in.must != nil {
		for _, c := range in.must.Calls {
			if c.Func != nil {
				bang[c.Func] = true
			}
		}
	}
	cleanups := make(map[*ast.ExprStmt]bool) // call statements right before a return of another error
	var check func(sig *types.Signature, fn, body ast.Node)
	check = func(sig *types.Signature, fn, body ast.Node) {
		returnsErr := sig != nil && sig.Results().Len() > 0 && isErrorType(sig.Results().At(sig.Results().Len()-1).Type())
		expects := returnsErr || bang[fn]
		why := "a function that returns an error"
		if !returnsErr {
			why = "a function that uses !"
		}
		ast.Inspect(body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncLit:
				s, _ := info.Types[n].Type.(*types.Signature)
				check(s, n, n.Body)
				return false
			case *ast.BlockStmt:
				if returnsErr {
					cleanup(n.List, cleanups)
				}
			case *ast.CaseClause:
				if returnsErr {
					cleanup(n.Body, cleanups)
				}
			case *ast.CommClause:
				if returnsErr {
					cleanup(n.Body, cleanups)
				}
			case *ast.ExprStmt:
				if !expects || cleanups[n] {
					return true
				}
				call, ok := ast.Unparen(n.X).(*ast.CallExpr)
				if !ok || !callEndsInError(info, call) {
					return true
				}
				name := calleeName(info, call)
				if ignorable(info, call, name) {
					return true
				}
				var fix string
				if !returnsErr {
					fix = fmt.Sprintf("handle it, write %s!(...) to handle it the way this function does, or write _ = %s(...) if ignoring it is intended", name, name)
				} else {
					fix = fmt.Sprintf("handle it, write %s!(...) to return it, or write _ = %s(...) if ignoring it is intended", name, name)
				}
				pass.Report(analysis.Diagnostic{
					Pos: call.Pos(), End: call.End(), Category: "ignorederr",
					Message: fmt.Sprintf("the error result of %s is dropped, in %s: %s", name, why, fix),
				})
			}
			return true
		})
	}
	for _, f := range pass.Files {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				fn, _ := info.Defs[d.Name].(*types.Func)
				if fn == nil {
					continue
				}
				check(fn.Signature(), d, d.Body)
			case *ast.GenDecl:
				// func literals in package variables
				check(nil, d, d)
			}
		}
	}
	return nil, nil
}

// callEndsInError reports whether the call is not a conversion or a builtin and its last result is
// exactly error.
func callEndsInError(info *types.Info, call *ast.CallExpr) bool {
	tv, ok := info.Types[call]
	if !ok || tv.Type == nil || tv.IsType() {
		return false
	}
	if ft, ok := info.Types[call.Fun]; ok && (ft.IsType() || ft.IsBuiltin()) {
		return false
	}
	t := tv.Type
	if tup, ok := t.(*types.Tuple); ok {
		if tup.Len() == 0 {
			return false
		}
		t = tup.At(tup.Len() - 1).Type()
	}
	return isErrorType(t)
}

// calleeName is the callee as the user wrote it: os.Remove, f.Close, run.
func calleeName(info *types.Info, call *ast.CallExpr) string {
	return types.ExprString(call.Fun)
}

// ignorable reports whether the error of the call is one that nobody checks (see the analyzer).
func ignorable(info *types.Info, call *ast.CallExpr, name string) bool {
	fn, _ := typeutil.Callee(info, call).(*types.Func)
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	pkg := fn.Pkg().Path()
	sig := fn.Signature()
	if sig.Recv() == nil {
		switch pkg {
		case "fmt":
			return strings.HasPrefix(fn.Name(), "Print") || strings.HasPrefix(fn.Name(), "Fprint")
		case "math/rand", "math/rand/v2":
			return fn.Name() == "Read"
		}
		return false
	}
	recv := sig.Recv().Type()
	if p, ok := recv.(*types.Pointer); ok {
		recv = p.Elem()
	}
	n, ok := recv.(*types.Named)
	if !ok || n.Obj().Pkg() == nil {
		return false
	}
	typ := n.Obj().Pkg().Path() + "." + n.Obj().Name()
	switch typ {
	case "strings.Builder", "bytes.Buffer":
		return strings.HasPrefix(fn.Name(), "Write")
	case "bufio.Writer":
		return strings.HasPrefix(fn.Name(), "Write")
	case "math/rand.Rand", "math/rand/v2.Rand":
		return fn.Name() == "Read"
	}
	// Write of a hash: hash.Hash and the types of crypto/ and hash/ that implement it.
	if fn.Name() == "Write" && (n.Obj().Pkg().Path() == "hash" || strings.HasPrefix(n.Obj().Pkg().Path(), "hash/") || strings.HasPrefix(n.Obj().Pkg().Path(), "crypto/")) {
		return true
	}
	return false
}

// cleanup marks the statements of list that are cleanup before a failure: a call statement that
// is followed by a return of something other than nil (or a bare return, whose results the analysis
// does not read). The error that matters then is the one that is returned, and the one that the
// cleanup call drops (f.Close() on the way out) has no better place to go.
func cleanup(list []ast.Stmt, marks map[*ast.ExprStmt]bool) {
	for i := 0; i+1 < len(list); i++ {
		es, ok := list[i].(*ast.ExprStmt)
		if !ok {
			continue
		}
		ret, ok := list[i+1].(*ast.ReturnStmt)
		if !ok {
			continue
		}
		if len(ret.Results) == 0 {
			marks[es] = true
			continue
		}
		if id, ok := ast.Unparen(ret.Results[len(ret.Results)-1]).(*ast.Ident); ok && id.Name == "nil" {
			continue
		}
		marks[es] = true
	}
}
