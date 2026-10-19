package check

import (
	"go/ast"
	"go/token"
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
}

// A MustFunc is a function declared with `!`.
type MustFunc struct {
	Decl *ast.FuncDecl
	Name string // the name the user wrote: main for main!, whose declaration in the tree is must_main
	Exit bool   // main! and init!: the error is printed and the program exits with 1
}

// A MustKind is what a `!` does when the error is not nil.
type MustKind int

const (
	MustReturn MustKind = iota // the function returns the error (its last result is error)
	MustFatal                  // t.Fatal(err): a function with a *testing.T, B or F parameter
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
	Deferred bool // defer F!(...)
}

// noteCall records the explicit helper call h, which the lowering is about to replace, made in
// the function that fc describes. It is recorded once, if the lowering ever reaches a call twice
// (no program of the tests does: this only keeps the list free of repeats).
func (mp *mustPass) noteCall(h *mustCall, fc *fnCtx) {
	if !h.explicit {
		return
	}
	if mp.noted == nil {
		mp.noted = make(map[*mustCall]bool)
	}
	if mp.noted[h] {
		return
	}
	mp.noted[h] = true
	kind := MustPanic
	switch {
	case fc.exit:
		kind = MustExit
	case fc.kind == failReturn && mp.exits(fc.node):
		kind = MustExit
	case fc.kind == failReturn:
		kind = MustReturn
	case fc.kind == failFatal:
		kind = MustFatal
	}
	mp.info.Calls = append(mp.info.Calls, MustCall{
		Pos: h.helper.Pos(), End: h.helper.End(), Func: fc.node, Kind: kind, Deferred: h.deferred,
	})
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
		mp.info.Funcs = append(mp.info.Funcs, MustFunc{Decl: mf.decl, Name: mf.name, Exit: mf.exit})
	}
	return &mp.info
}
