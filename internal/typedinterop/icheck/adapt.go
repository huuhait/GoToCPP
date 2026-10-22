package icheck

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/adapt"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The argument adaptations of typed interop (ADR-130, ADR-134) refuse some arguments of a call into
// C/C++: a constant that does not fit the parameter, a runtime string where a format string is
// expected, a string that the callee keeps. `gocpp build` reports them while it lowers the call;
// this pass reports the same errors, with the same words at the same place, from the same decision
// (adapt.Decide), so that `gocpp check`, `gocpp vet` and the editor show them without a build.
//
// An error here is an error of the language, certain from the Go program and the declarations
// alone, so it is never downgraded to a warning for a g++ build, unlike the failed probes: the
// build stops at it with either compiler. The pass reports nothing where it cannot tell: a callee
// without declarations or with a template among its overloads, a program whose typed lowering is
// off (it declares generic functions or types), a call with `...`.

// Adaptations returns the refusals of the argument adaptations in the package p, from a report that
// Analyze made with Options.Decls (and StringToCStr, as the build has it), in source order; the
// string is why nothing was checked, or "". The language server runs it alone: the failed probes
// of CheckReport are the compiler's to report there.
func Adaptations(p *check.Package, fset *token.FileSet, rep *typedinterop.Report) ([]diag.Diagnostic, string) {
	res := &Result{Report: rep}
	if res.Skipped = skipReason(rep); res.Skipped != "" {
		return nil, res.Skipped
	}
	c := &checker{p: p, fset: fset, rep: rep, res: res}
	c.adaptations()
	sortDiags(res.Diags)
	return res.Diags, ""
}

// HasAdaptableCalls reports whether the package has a call whose callee is a C/C++ name and with an
// argument that an adaptation looks at (a number constant, a Go string, the address of a variable,
// a float): the only calls that can have a refusal. It reads the checked package alone, no compiler
// has run, so the language server asks it before it starts the analysis.
func HasAdaptableCalls(p *check.Package) bool {
	if p == nil || p.Info == nil || p.Opaque == nil || p.Load == nil || declaresGenerics(p) {
		return false
	}
	facts := adaptFacts{p}
	found := false
	for _, f := range p.Load.Files {
		if f.AST == nil {
			continue
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if found {
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok || call.Ellipsis.IsValid() || !p.Opaque.IsOpaque(call.Fun) {
				return true
			}
			for _, arg := range call.Args {
				if adapt.Describe(facts, arg, func(ast.Expr) string { return "" }).Class != adapt.ClassNone {
					found = true
					break
				}
			}
			return true
		})
	}
	return found
}

// adaptations goes over the calls of the package and reports the refusals of the adaptations.
func (c *checker) adaptations() {
	if declaresGenerics(c.p) {
		return // the lowering is not typed then (lower.typedOn), so it adapts nothing and refuses nothing
	}
	pass := &adaptPass{c: c, facts: adaptFacts{c.p}}
	for _, f := range c.p.Load.Files {
		pass.parents = pass.parents[:0]
		ast.Inspect(f.AST, pass.visit)
	}
}

// declaresGenerics reports whether the package declares a generic function or type.
func declaresGenerics(p *check.Package) bool {
	for _, f := range p.Load.Files {
		for _, d := range f.AST.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Type.TypeParams != nil {
					return true
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					if ts, ok := s.(*ast.TypeSpec); ok && ts.TypeParams != nil {
						return true
					}
				}
			}
		}
	}
	return false
}

type adaptPass struct {
	c       *checker
	facts   adaptFacts
	parents []ast.Node // the nodes above the one visited
}

func (a *adaptPass) visit(n ast.Node) bool {
	if n == nil {
		a.parents = a.parents[:len(a.parents)-1]
		return true
	}
	a.parents = append(a.parents, n)
	if call, ok := n.(*ast.CallExpr); ok {
		a.call(call)
	}
	return true
}

// parentOf returns the node above n, which is on the way down the tree (the node being visited or
// one of its ancestors, the last of a.parents).
func (a *adaptPass) parentOf(n ast.Node) ast.Node {
	for i := len(a.parents) - 1; i >= 0; i-- {
		if a.parents[i] == n {
			if i == 0 {
				return nil
			}
			return a.parents[i-1]
		}
	}
	return nil
}

// deferred reports whether the call c is the call of a defer or go statement.
func (a *adaptPass) deferred(c *ast.CallExpr) bool {
	if len(a.parents) < 2 {
		return false
	}
	switch s := a.parents[len(a.parents)-2].(type) {
	case *ast.DeferStmt:
		return s.Call == c
	case *ast.GoStmt:
		return s.Call == c
	}
	return false
}

// call decides every argument of the call c that has an adaptation class.
func (a *adaptPass) call(c *ast.CallExpr) {
	if c.Ellipsis.IsValid() || len(c.Args) == 0 {
		return
	}
	p := a.c.p
	args := make([]adapt.Arg, len(c.Args))
	adaptable := false
	for i, arg := range c.Args {
		args[i] = adapt.Describe(a.facts, arg, p.SourceText)
		adaptable = adaptable || args[i].Class != adapt.ClassNone
	}
	if !adaptable {
		return
	}
	hasString := false
	for _, arg := range args {
		hasString = hasString || arg.Class == adapt.ClassString || arg.Class == adapt.ClassStringConst
	}
	sigs, name, known := a.c.calleeSigs(c)
	if !known || len(sigs) == 0 {
		return
	}
	rep := a.c.rep
	call := &adapt.Call{
		Name: name,
		Sigs: sigs,
		Args: args,
		Source: func(i int) string {
			if i == adapt.AtCall {
				return p.SourceText(c.Fun)
			}
			return p.SourceText(c.Args[i])
		},
		Deferred: a.deferred(c),
		MethodKeepsCharPtr: func(cs adapt.CharSet) (string, bool) {
			sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
			if !ok {
				return "", false
			}
			v, _ := rep.Typed(rep.MemberObject(p, sel))
			if v == nil {
				return "", false
			}
			return adapt.MethodKeepsCharPtr(rep.Mapper, v, cs)
		},
		AliasUseOK: func() bool { return adapt.AliasUseOK(a.parentOf, c) },
	}
	if hasString {
		// The other arguments that are the address of a char pointer: the callee can store a
		// pointer into the string there.
		for i, arg := range c.Args {
			args[i].CharPtrAddr = a.charPtrAddr(arg)
		}
	}
	for i := range c.Args {
		if args[i].Class == adapt.ClassNone {
			continue
		}
		d := adapt.Decide(rep.Mapper, call, i)
		if d.Outcome != adapt.Refuse {
			continue
		}
		pos := c.Pos()
		if d.At != adapt.AtCall {
			pos = c.Args[d.At].Pos()
		}
		a.c.refusal(pos, d.Msg)
	}
}

// refusal adds the error of an adaptation: an error with either compiler.
func (c *checker) refusal(pos token.Pos, msg string) {
	c.res.Diags = append(c.res.Diags, diag.Diagnostic{
		Pos:      c.fset.Position(pos),
		Severity: diag.Error,
		Msg:      msg,
	})
	c.res.Stats.Adapt++
}

// calleeSigs returns the overloads that the callee of c can be, and its name, as the lowering
// has them: for a free function, the overload set of its name; for a member of a C++ object, the
// methods of that name of its class and bases. false when it is something else, or has no
// declaration, or a template among the candidates.
func (c *checker) calleeSigs(call *ast.CallExpr) ([]adapt.Sig, string, bool) {
	rep := c.rep
	fun := ast.Unparen(call.Fun)
	if ref := rep.Set.RefOf(fun); ref != nil {
		if path, ok := ref.Expr.(*collect.Path); ok {
			name, template, ok := probe.FuncNameOf(rep.Kinds, path)
			if !ok || template {
				return nil, "", false
			}
			sigs, ok := adapt.FuncSigs(rep.Decls, name)
			return sigs, name, ok
		}
	}
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return nil, "", false
	}
	v, _ := rep.Typed(rep.MemberObject(c.p, sel))
	if v == nil {
		return nil, "", false
	}
	sigs, ok := adapt.MethodSigs(rep.Mapper, v, sel.Sel.Name)
	return sigs, sel.Sel.Name, ok
}

// adaptFacts are the answers about the Go program that adapt.Describe asks, from the checked
// package.
type adaptFacts struct{ p *check.Package }

func (f adaptFacts) Operand(e ast.Expr) types.TypeAndValue { return f.p.Info.Types[e] }

func (f adaptFacts) Opaque(e ast.Expr) bool { return f.p.Opaque.IsOpaque(e) }

func (f adaptFacts) Holder(e ast.Expr) bool { return f.p.Holders().IsHolder(e) }

func (f adaptFacts) Variable(e ast.Expr) bool { return adapt.IsVariableName(f.p.Info, e) }

// CPtrVariable reports whether e is a variable or a field declared with a pointer to a C/C++ type
// (var p *lib.T). Only a declaration that names the type tells: a variable that gets its type from
// its value, and a parameter, are not known here (the build knows them).
func (f adaptFacts) CPtrVariable(e ast.Expr) bool {
	return adapt.CPtrDeclared(f.p.DeclaredTypeExpr(e), f.p.IsCTypeExpr)
}

// charPtrAddr returns the C++ name of the character type of the pointer whose address the argument
// a is (&end of a *C.char variable: char **), or "".
func (a *adaptPass) charPtrAddr(arg ast.Expr) string {
	typed, _ := a.c.rep.Typed(arg)
	return adapt.CharPtrAddr(typed, arg, a.c.p.DeclaredTypeExpr, a.c.p.IsC)
}
