package check

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
)

// The second pass for promoted C++ members (Phase 12 step 12.9, gap E2, ADR-098).
//
//	type S struct {
//		std.vector[int]
//		std.string
//	}
//	s.push_back(1)  // the vector's: s.vector.push_back(1)
//	s.append("x")   // the string's
//	s.size()        // both have size: ambiguous, as in Go
//
// go/types has no type for an embedded C++ class (every C/C++ type is invalid in it), so the
// first pass can only decide the selectors whose answer does not depend on the class's members
// (promoted.go: one embedded C++ type, no Go member at the same depth). The others are the
// candidates of the second pass:
//
//   - x.name that go/types found nothing for, on a struct that embeds a C++ class (directly or
//     through embedded structs): a member of that class, or an error;
//   - x.name that go/types resolved to a Go member of a struct that also embeds a C++ class at
//     the same or a shallower depth: Go's depth rule picks between them, and only the class knows
//     whether it has the member.
//
// With Config.CppMembers, Check asks the hook which classes have which names (one question per
// embedded field and name; the answers come from the C++ compiler, the typed probe of
// internal/typedinterop, and are cached on disk there) and applies Go's rules of selectors to
// the answers. The model of the class is a member table (the names the class has, and how they
// can be used) and not go/types objects: go/types cannot represent a C++ class, and a class
// synthesized for it would make the lookup of methods, the method sets and the assignability
// of the struct depend on a C++ declaration that stock go/types never reads. So this pass does
// not run go/types a second time: it re-decides what the first pass could not, with the
// results where the first pass looked (Package.PromotedPath, the false errors that stay, the
// opaque pass and the phase gates).
//
// What the answers decide for a selector x.name, with d the depth of a member (the number of
// embedded fields to its owner: a member of the Go struct itself has depth 0, a member of the C++
// class that is the embedded field of x has depth 1):
//
//	no member at all                            error: "has no member" (the class's words)
//	exactly one member at the smallest depth    that one: a Go member as go/types has it, or the
//	                                            class's, reached through the embedded fields
//	several at the smallest depth               error: ambiguous selector
//	the class's member hides a deeper Go one    error (the Go selection is already in Info, and
//	                                            the lowering would write the Go member): name the field
//	the member is private or protected          error: not accessible (also from a private base)
//	the member is a deleted function            error
//	the name is a type or a macro               error: not a member
//
// Members of the class are everything the C++ compiler finds for obj.name: data members,
// static members, enumerators, member functions with all their overloads (const and non-const)
// and member templates (the use then names the instantiation: s.convert[int](x), one per
// call), also those of its bases. Operators are not names in Go, so they are not promoted
// (s[0] and s + t stay what go/types says; write s.vector[0]). A class that has an operator-> and
// no member of the name is not decided (typed code generation writes x->name for it, ADR-087),
// and neither is a selector for which the hook has no answer: those stay as the first pass left
// them, and the C++ compiler decides.

// A MemberStatus is what the C++ compiler says about the name of a member of a class.
type MemberStatus int

const (
	// MemberUnknown: the hook has no answer (no compiler, a probe that failed, a class it cannot
	// write in C++). The selector stays as the first pass left it.
	MemberUnknown MemberStatus = iota
	// MemberFound: the class has a member of that name that its users can name: a data member, a
	// static member, an enumerator, a member function (any overload, also a template).
	MemberFound
	// MemberMissing: the class has no member of that name.
	MemberMissing
	// MemberPrivate and MemberProtected: the class (or a base) has the member, but not for its users.
	MemberPrivate
	MemberProtected
	// MemberAmbiguous: the name is found in several bases of the class (an error in C++).
	MemberAmbiguous
	// MemberDeleted: the member is a function that is declared = delete.
	MemberDeleted
	// MemberNotValue: the name is a member type (a nested class or a typedef), not a value.
	MemberNotValue
	// MemberMacro: the name is a macro of the headers, so the C++ text of a member of that name
	// is something else.
	MemberMacro
)

// String returns the status as a word.
func (s MemberStatus) String() string {
	switch s {
	case MemberFound:
		return "found"
	case MemberMissing:
		return "missing"
	case MemberPrivate:
		return "private"
	case MemberProtected:
		return "protected"
	case MemberAmbiguous:
		return "ambiguous"
	case MemberDeleted:
		return "deleted"
	case MemberNotValue:
		return "not a value"
	case MemberMacro:
		return "macro"
	}
	return "unknown"
}

// has reports whether the status means that the class has a member of the name (that Go's rule
// of selectors counts, also when its users cannot use it).
func (s MemberStatus) has() bool {
	switch s {
	case MemberFound, MemberPrivate, MemberProtected, MemberAmbiguous, MemberDeleted:
		return true
	}
	return false
}

// A MemberQuery asks whether the C++ class written by Class has a member called Name.
type MemberQuery struct {
	// Class is the type expression of the embedded field (std.vector[int], *Node, a named type
	// declared over a C++ type), in the AST of the package after the loader's rewrite.
	Class ast.Expr

	// Name is the member name.
	Name string
}

// A MemberAnswer answers one MemberQuery.
type MemberAnswer struct {
	Status MemberStatus

	// Why is the compiler's own text for the status, for debugging and tests.
	Why string
}

// CppMembers answers the member queries of the package p, the first pass's result (p.Types,
// p.Info, p.Load and the flags are set; p.Opaque holds what the first pass knows). It returns one
// answer for each query, in order. An error, or a short result, leaves the queries unanswered
// (MemberUnknown): Check goes on as without the hook.
type CppMembers func(ctx context.Context, p *Package, queries []MemberQuery) ([]MemberAnswer, error)

// A cppSel is what the second pass decided about a selector.
type cppSel struct {
	// path is the path of the embedded fields to the C++ class that has the member (the indexes
	// of Selection.Index), nil for a selector that stays the Go member go/types found, and for an
	// error.
	path []int

	// field is the embedded field of the C++ class at the end of path.
	field *types.Var

	// opaque says that the selector is a C++ expression for the opaque pass: a member of a
	// class, or an error about one (its errors are reported, and the expression is not typed).
	opaque bool

	// err says that the second pass reported an error for the selector.
	err bool
}

// A cppField is an embedded C++ class of a struct type, as a candidate selector sees it.
type cppField struct {
	path  []int
	field *types.Var
	class ast.Expr
}

// A cppCand is a candidate selector of the second pass.
type cppCand struct {
	sel    *ast.SelectorExpr
	t      types.Type // the type of sel.X
	name   string
	fields []cppField
	// goDepth is the depth of the Go member go/types found (or the Go members it called
	// ambiguous), -1 when there is none; goN is how many.
	goDepth, goN int
	// bad says that a class could not be named (no type expression in this package).
	bad bool
}

// CppMembersAsked reports how many questions the second pass asked of the hook for the package
// (0 when the first pass had no candidate, or there was no hook): the cost of the second pass.
func (p *Package) CppMembersAsked() int { return p.cppAsked }

// PromotedField returns the embedded field of the C++ class that the selector sel takes its member
// from (the last field of PromotedPath), or nil.
func (p *Package) PromotedField(sel *ast.SelectorExpr) *types.Var {
	if p == nil {
		return nil
	}
	if r, ok := p.cppSels[sel]; ok {
		return r.field
	}
	if path := p.PromotedPath(sel); path != nil {
		return fieldAtPath(p.Info.Types[sel.X].Type, path)
	}
	return nil
}

// fieldAtPath returns the field at the end of path, starting at the struct type t (or a pointer to one).
func fieldAtPath(t types.Type, path []int) *types.Var {
	var f *types.Var
	for _, i := range path {
		if p, ok := types.Unalias(t).Underlying().(*types.Pointer); ok {
			t = p.Elem()
		}
		st, ok := types.Unalias(t).Underlying().(*types.Struct)
		if !ok || i >= st.NumFields() {
			return nil
		}
		f = st.Field(i)
		t = f.Type()
	}
	return f
}

// embeddedClassExprs maps each embedded field of the structs declared in the package's files to
// the type expression it was declared with.
func embeddedClassExprs(files []*ast.File, info *types.Info) map[*types.Var]ast.Expr {
	m := make(map[*types.Var]ast.Expr)
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			st, ok := n.(*ast.StructType)
			if !ok || st.Fields == nil {
				return true
			}
			for _, fld := range st.Fields.List {
				if len(fld.Names) != 0 {
					continue
				}
				id := embeddedFieldIdent(fld.Type)
				if id == nil {
					continue
				}
				if v, ok := info.Defs[id].(*types.Var); ok {
					m[v] = fld.Type
				}
			}
			return true
		})
	}
	return m
}

// embeddedFieldIdent returns the name of an embedded field from its type expression, as go/types
// makes it (T, *T, pkg.T, T[int], pkg.T[int, string]).
func embeddedFieldIdent(e ast.Expr) *ast.Ident {
	switch e := e.(type) {
	case *ast.Ident:
		return e
	case *ast.StarExpr:
		if _, ok := e.X.(*ast.StarExpr); !ok {
			return embeddedFieldIdent(e.X)
		}
	case *ast.SelectorExpr:
		return e.Sel
	case *ast.IndexExpr:
		return embeddedFieldIdent(e.X)
	case *ast.IndexListExpr:
		return embeddedFieldIdent(e.X)
	case *ast.ParenExpr:
		return embeddedFieldIdent(e.X)
	}
	return nil
}

// cppCandidates finds the selectors of the first pass that the second pass decides.
func (p *Package) cppCandidates() []*cppCand {
	if p.Types == nil || p.Info == nil {
		return nil
	}
	var classes map[*types.Var]ast.Expr
	hidden, _ := p.rewriteNodes()
	var out []*cppCand
	for _, f := range p.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if n == nil || hidden[n] {
				return false
			}
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name == "_" {
				return true
			}
			tv, ok := p.Info.Types[sel.X]
			if !ok || !tv.IsValue() || tv.Type == nil || tv.Type == types.Typ[types.Invalid] {
				return true
			}
			var found [][]int
			opaqueEmbedded(tv.Type, nil, make(map[*types.Struct]bool), &found)
			if len(found) == 0 {
				return true
			}
			c := &cppCand{sel: sel, t: tv.Type, name: sel.Sel.Name, goDepth: -1}
			obj, index, _ := types.LookupFieldOrMethod(tv.Type, true, p.Types, c.name)
			resolved := p.Info.Selections[sel] != nil
			switch {
			case obj != nil && len(index) > 0:
				c.goDepth, c.goN = len(index)-1, 1
			case obj == nil && index != nil:
				c.goDepth, c.goN = len(index)-1, 2 // go/types: ambiguous selector
			case !resolved && goMember(tv.Type, c.name, make(map[types.Type]bool)):
				return true // a Go member that the package cannot name: go/types' error is the right one
			}
			if resolved && c.goDepth < 0 {
				return true // a selection of something else (a method value of a C++ type?)
			}
			if resolved {
				// The shadow case: only a class at the same or a shallower depth matters.
				shallow := found[:0:0]
				for _, path := range found {
					if len(path) <= c.goDepth {
						shallow = append(shallow, path)
					}
				}
				if len(shallow) == 0 {
					return true
				}
				found = shallow
			}
			if classes == nil {
				classes = embeddedClassExprs(p.astFiles(), p.Info)
			}
			for _, path := range found {
				fld := fieldAtPath(tv.Type, path)
				if fld == nil {
					c.bad = true
					continue
				}
				expr := classes[fld.Origin()]
				if expr == nil {
					c.bad = true // the struct is declared in another package: its Opaque has the type expression
					continue
				}
				c.fields = append(c.fields, cppField{path: path, field: fld, class: expr})
			}
			out = append(out, c)
			return true
		})
	}
	return out
}

// astFiles returns the ASTs of the package's files.
func (p *Package) astFiles() []*ast.File {
	files := make([]*ast.File, len(p.Load.Files))
	for i, f := range p.Load.Files {
		files[i] = f.AST
	}
	return files
}

// resolveCppMembers runs the second pass on the package p (after the first pass and the
// provisional opaque pass): it asks the hook about the candidates and records and reports what
// the answers decide. It does nothing when there is no candidate: the first pass stands.
func (p *Package) resolveCppMembers(fset *token.FileSet, ctx context.Context, hook CppMembers) {
	cands := p.cppCandidates()
	if len(cands) == 0 {
		return
	}
	// One question for each class expression and name.
	type qkey struct {
		class ast.Expr
		name  string
	}
	index := make(map[qkey]int)
	var queries []MemberQuery
	for _, c := range cands {
		if c.bad {
			continue
		}
		for _, f := range c.fields {
			k := qkey{f.class, c.name}
			if _, ok := index[k]; !ok {
				index[k] = len(queries)
				queries = append(queries, MemberQuery{Class: f.class, Name: c.name})
			}
		}
	}
	if len(queries) == 0 {
		return
	}
	p.cppAsked = len(queries)
	// The hook's collector reads the type expressions of the embedded fields from the opaque pass; a
	// package with no candidate (nearly all) never pays for it twice.
	p.Opaque = provisionalOpaque(fset, p)
	answers, err := hook(ctx, p, queries)
	if err != nil || len(answers) != len(queries) {
		return
	}
	p.cppSels = make(map[*ast.SelectorExpr]*cppSel)
	drop := make(map[token.Pos]bool) // the go/types errors that the second pass replaces
	var added []diag.Diagnostic
	for _, c := range cands {
		if c.bad {
			continue
		}
		ans := make([]MemberAnswer, len(c.fields))
		unknown := false
		for i, f := range c.fields {
			ans[i] = answers[index[qkey{f.class, c.name}]]
			unknown = unknown || ans[i].Status == MemberUnknown
		}
		if unknown {
			continue
		}
		r, msg := p.decideCpp(c, ans)
		p.cppSels[c.sel] = r
		if msg != "" {
			added = append(added, diag.Diagnostic{Pos: fset.Position(c.sel.Sel.Pos()), End: fset.Position(c.sel.Sel.End()), Msg: msg})
		}
		if p.Info.Selections[c.sel] == nil {
			drop[c.sel.Sel.Pos()] = true
		}
	}
	if len(drop) > 0 {
		// The "x.name undefined" of go/types is false (a member of a class) or is said better here.
		seen := make(map[token.Position]bool)
		for pos := range drop {
			seen[fset.Position(pos)] = true
		}
		p.Errors = p.Errors.Rewrite(func(d diag.Diagnostic) []diag.Diagnostic {
			if seen[d.Pos] && d.Severity == diag.Error && strings.Contains(d.Msg, " undefined (type ") && strings.Contains(d.Msg, " has no field or method ") {
				return nil
			}
			return []diag.Diagnostic{d}
		})
	}
	slices.SortStableFunc(added, func(a, b diag.Diagnostic) int { return strings.Compare(a.Msg, b.Msg) })
	for _, d := range added {
		p.Errors.Add(d)
	}
}

// decideCpp applies Go's rule of selectors to the answers about the classes that candidate c
// names. It returns what the selector is, and the message of the error when it is one.
func (p *Package) decideCpp(c *cppCand, ans []MemberAnswer) (*cppSel, string) {
	type member struct {
		depth int
		goOne bool // the Go member(s) of go/types
		field int  // the index in c.fields
	}
	var all []member
	if c.goDepth >= 0 {
		all = append(all, member{depth: c.goDepth, goOne: true})
	}
	for i, f := range c.fields {
		if ans[i].Status.has() {
			all = append(all, member{depth: len(f.path), field: i})
		}
	}
	text := p.exprString(c.sel)
	typeText := types.TypeString(c.t, types.RelativeTo(p.Types))
	className := func(i int) string { return classText(c.fields[i].class) }
	fail := func(format string, args ...any) (*cppSel, string) {
		return &cppSel{opaque: true, err: true}, fmt.Sprintf(format, args...)
	}
	if len(all) == 0 {
		// No member anywhere.
		var names []string
		extra := ""
		for i := range c.fields {
			names = append(names, className(i))
			switch ans[i].Status {
			case MemberMacro:
				extra = fmt.Sprintf(" (%s is a macro of the headers)", c.name)
			case MemberNotValue:
				extra = fmt.Sprintf(" (%s is a type of the class, not a member)", c.name)
			}
		}
		if len(names) == 1 {
			return fail("%s undefined (type %s has no field or method %s; %s has no member %s)%s", text, typeText, c.name, names[0], c.name, extra)
		}
		return fail("%s undefined (type %s has no field or method %s; none of the embedded C++ types %s has a member %s)%s", text, typeText, c.name, strings.Join(names, ", "), c.name, extra)
	}
	minDepth := all[0].depth
	for _, m := range all {
		minDepth = min(minDepth, m.depth)
	}
	var at []member
	count := 0
	for _, m := range all {
		if m.depth == minDepth {
			at = append(at, m)
			if m.goOne {
				count += max(c.goN, 1)
			} else {
				count++
			}
		}
	}
	if count > 1 {
		var who []string
		for _, m := range at {
			if m.goOne {
				who = append(who, "a Go member")
			} else {
				who = append(who, className(m.field)+"."+c.name)
			}
		}
		return fail("ambiguous selector %s (%s)", text, strings.Join(who, " and "))
	}
	m := at[0]
	if m.goOne {
		// go/types' selection is the right one; the gates need not worry about the classes.
		return &cppSel{}, ""
	}
	f := c.fields[m.field]
	st := ans[m.field].Status
	if c.goDepth >= 0 {
		return fail("%s is the member of %s, which is shallower than the Go member %s (Go's rule): name the field, %s",
			text, className(m.field), c.name, p.pathText(c, f.path))
	}
	switch st {
	case MemberPrivate:
		return fail("%s undefined (the member %s of %s is private)", text, c.name, className(m.field))
	case MemberProtected:
		return fail("%s undefined (the member %s of %s is protected)", text, c.name, className(m.field))
	case MemberAmbiguous:
		return fail("ambiguous selector %s (%s has a member %s in several bases)", text, className(m.field), c.name)
	case MemberDeleted:
		return fail("%s is a deleted function of %s", text, className(m.field))
	}
	return &cppSel{path: f.path, field: f.field, opaque: true}, ""
}

// pathText writes the selector of candidate c with the embedded fields named: s.Mid.vector.name.
func (p *Package) pathText(c *cppCand, path []int) string {
	text := p.exprString(c.sel.X)
	t := c.t
	for _, i := range path {
		if pt, ok := types.Unalias(t).Underlying().(*types.Pointer); ok {
			t = pt.Elem()
		}
		st, ok := types.Unalias(t).Underlying().(*types.Struct)
		if !ok {
			break
		}
		text += "." + st.Field(i).Name()
		t = st.Field(i).Type()
	}
	return text + "." + c.name
}

// classText is the C++ class a type expression names, as the user wrote it: std.vector[int],
// *Node (the loader's C. prefix undone).
func classText(e ast.Expr) string {
	s := types.ExprString(e)
	for _, pre := range []string{"*C.", "C."} {
		if rest, ok := strings.CutPrefix(s, pre); ok {
			return pre[:len(pre)-2] + rest
		}
	}
	return s
}

// promoted returns what the second pass decided for sel, if it looked at it.
func (p *Package) cppSelOf(sel *ast.SelectorExpr) (*cppSel, bool) {
	if p == nil || p.cppSels == nil {
		return nil, false
	}
	r, ok := p.cppSels[sel]
	return r, ok
}
