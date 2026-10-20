package check

import (
	"go/ast"
	"go/token"
	"go/types"
)

// MustInfo tells what the `!` forms of a package were (Phase 10), after the lowering took them out of
// its syntax trees (ADR-037: the trees that analyzers see are plain Go). gocpp vet reads it for its
// analyzers about `!`: a program that shows a panicking `!` in a library, or a `go` statement that
// drops the error of a must function. Positions are the ones of the original text. The info is nil
// for a package without `!` forms, and for one that did not pass the checks of the rewrite.
type MustInfo struct {
	// Funcs are the must functions (declared with `!`), in source order. The declaration is
	// the one in the tree: its name has lost the `!` and its results got the error.
	Funcs []MustFunc

	// Calls are the calls written F!(...), in the order the lowering found them. The calls that a
	// must function makes of a call that drops its error (os.Remove(p)) are not here: nothing in
	// the source says `!` there.
	Calls []MustCall

	// Handled are the calls that a must function handles without a `!` in the text: they drop
	// their error (os.Remove(p) as a statement, data := os.ReadFile(p)). Their Pos and End are
	// the span of the call, Bang is NoPos. The editor marks them (step 11.13); gocpp vet does
	// not read them.
	Handled []MustCall

	// Stmts are the statements of the source whose code the lowering wrote, with the code. Every
	// statement that has a F!(...) or a handled call is here (also a statement inside another one:
	// the smallest one that has a place is the one of that call). The editor prints them to show
	// what a `!` is (hover, "show expansion").
	Stmts []MustStmt
}

// A MustStmt is a statement of the source, as written (its span), and the statements that stand
// for it after the lowering: the call and the error check, or a block that holds them. The
// nodes are those of the checked tree and must not be changed.
type MustStmt struct {
	Pos, End token.Pos
	Lowered  []ast.Stmt
}

// A MustFunc is a function declared with `!`.
type MustFunc struct {
	Decl *ast.FuncDecl
	Name string    // the name the user wrote: main for main!, whose declaration in the tree is must_main
	Exit bool      // main! and init!: the error is printed and the program exits with 1
	Bang token.Pos // the place of the ! after the name
}

// A MustKind is what a `!` does when the error is not nil.
type MustKind int

const (
	MustReturn MustKind = iota // the function returns the error (its last result is error)
	MustFatal                  // t.Fatal(err): a named *testing.T/B/F or testing.TB parameter, possibly inherited by a closure
	MustPanic                  // panic(err): any other function, and package variable initializers
	MustExit                   // the end of main!, init! and a main with `!`: print and exit with 1
)

func (k MustKind) String() string {
	return [...]string{"return", "fatal", "panic", "exit"}[k]
}

// A MustCall is one F!(...) of the source.
type MustCall struct {
	// Pos and End are the span of F!(...).
	Pos, End token.Pos

	// Func is the *ast.FuncDecl or *ast.FuncLit that the call is in, or nil for the initializer of
	// a package variable. A func literal is its own function: its result decides the Kind.
	Func ast.Node

	Kind     MustKind
	Deferred bool      // defer F!(...)
	Bang     token.Pos // the place of the ! in F!(...); NoPos for a call in Handled

	// Name is the place of the name of the function that is called (F in F!(x), x.F!(y), F![T](z)), as
	// the source has it: the lowering moves some names (a deferred call) to places of its own.
	Name token.Pos

	// Callee is the name itself: the node, which the lowering sometimes moves to code of its own (a loop's
	// condition, a deferred call), where it has another place. The checker's Uses still has it.
	Callee *ast.Ident
}

// noteCall records the helper call h, which the lowering is about to replace, made in
// the function that fc describes. It is recorded once, if the lowering ever reaches a call twice
// (no program of the tests does: this only keeps the list free of repeats).
func (mp *mustPass) noteCall(h *mustCall, fc *fnCtx) {
	if mp.noted == nil {
		mp.noted = make(map[*mustCall]bool)
	}
	if mp.noted[h] {
		return
	}
	mp.noted[h] = true
	c := MustCall{Pos: h.helper.Pos(), End: h.helper.End(), Func: fc.node, Kind: mp.callKind(fc), Deferred: h.deferred, Bang: h.bang}
	if id := calleeIdent(h.inner.Fun); id != nil {
		c.Name, c.Callee = id.Pos(), id
	}
	if !h.explicit {
		mp.info.Handled = append(mp.info.Handled, c)
		return
	}
	mp.info.Calls = append(mp.info.Calls, c)
}

// callKind is shared by lowered and partial information: closure testing contexts and return/exit priority
// must be the same even when another error prevents lowering.
func (mp *mustPass) callKind(fc *fnCtx) MustKind {
	if fc == nil {
		return MustPanic
	}
	switch {
	case fc.exit || fc.kind == failReturn && mp.exits(fc.node):
		return MustExit
	case fc.kind == failReturn:
		return MustReturn
	case fc.kind == failFatal:
		return MustFatal
	default:
		return MustPanic
	}
}

// exits reports whether fn is main! or init!.
func (mp *mustPass) exits(fn ast.Node) bool {
	fd, ok := fn.(*ast.FuncDecl)
	if !ok {
		return false
	}
	mf := mp.funcs[fd]
	return mf != nil && mf.exit
}

// mustInfo returns what the pass found, once the lowering is done.
func (mp *mustPass) mustInfo() *MustInfo {
	for _, mf := range mp.order {
		mp.info.Funcs = append(mp.info.Funcs, MustFunc{Decl: mf.decl, Name: mf.name, Exit: mf.exit, Bang: mf.bang})
	}
	return &mp.info
}

// partialInfo returns what the pass knows when the package has errors and the lowering did not run
// (Package.MustPartial): the written calls and the ones a must function handles, with their places and
// the failure contexts of the partial type check; no statements. The trees keep the helper calls then
// (gp_must.M1(F(x))), which have the places of the calls.
func (mp *mustPass) partialInfo(checked *types.Info) *MustInfo {
	info := &MustInfo{}
	contexts := mp.partialContexts(checked)
	for _, mf := range mp.order {
		info.Funcs = append(info.Funcs, MustFunc{Decl: mf.decl, Name: mf.name, Exit: mf.exit, Bang: mf.bang})
	}
	for _, h := range mp.calls {
		c := MustCall{Pos: h.helper.Pos(), End: h.helper.End(), Func: h.fn, Kind: mp.callKind(contexts[h.fn]), Deferred: h.deferred, Bang: h.bang}
		if id := calleeIdent(h.inner.Fun); id != nil {
			c.Name, c.Callee = id.Pos(), id
		}
		if h.explicit {
			info.Calls = append(info.Calls, c)
		} else {
			info.Handled = append(info.Handled, c)
		}
	}
	return info
}

// partialContexts uses the lowerer's context construction without rewriting any code. In particular,
// isTesting recognizes testing.TB and exact package identity, and newCtx inherits only the enclosing
// context's testing parameter (not its return or exit policy) into a closure.
func (mp *mustPass) partialContexts(info *types.Info) map[ast.Node]*fnCtx {
	contexts := make(map[ast.Node]*fnCtx)
	if len(mp.calls) == 0 || info == nil {
		return contexts
	}
	l := &lowerer{mp: mp, info: info}
	for _, file := range mp.files {
		ast.PreorderStack(file, nil, func(n ast.Node, stack []ast.Node) bool {
			var fc *fnCtx
			switch fn := n.(type) {
			case *ast.FuncDecl:
				var sig *types.Signature
				if obj, ok := info.Defs[fn.Name].(*types.Func); ok {
					sig = obj.Signature()
				}
				if mp.exits(fn) || fn.Recv == nil && fn.Name.Name == "main" && file.Name.Name == "main" && l.hasOwn(fn.Body) {
					fc = &fnCtx{kind: failReturn, typ: fn.Type, exit: true}
				} else {
					fc = l.newCtx(sig, fn.Type, "")
				}
			case *ast.FuncLit:
				outerFatal := ""
				for i := len(stack) - 1; i >= 0; i-- {
					if outer := contexts[stack[i]]; outer != nil {
						outerFatal = outer.fatal
						break
					}
				}
				fc = l.newCtx(l.sigOfLit(fn), fn.Type, outerFatal)
			default:
				return true
			}
			fc.node = n
			contexts[n] = fc
			return true
		})
	}
	return contexts
}
