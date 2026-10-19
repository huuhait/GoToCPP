package vet

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/huuhait/gocpp/internal/check"
)

// panicbangAnalyzer finds the `!` that panic inside a library.
//
// What `!` does when the error is not nil depends on the function that holds it (ADR-013): a
// function whose last result is error returns the error, a function with a *testing.T parameter
// calls t.Fatal, main! prints it and exits, and any other function panics with the error. The
// last form is Go's Must convention (regexp.MustCompile), right for a program, but a library
// should hand its errors to the caller: a panic in a library ends the program of someone who
// never wrote a `!`. So the analyzer reports
//
//   - every F!(...) of a function with no error result in a package that is not main (not in a
//     _test.gp file, and not in a function whose name starts with Must or must: the convention
//     says that it panics), at the `!`;
//   - every call, in such a package, of a function of another package that panics through a
//     `!` of its own: directly or through the functions it calls (whole-program: the fact travels
//     with the package, like printf's), at the call.
//
// A function that calls recover() is left alone by both rules, and stops the chain: it may have
// stopped the panic.
//
// The `!` forms are plain Go in the trees (ADR-037), so the places and the kinds come from
// check.Package.Must; the call graph is the static one (typeutil.StaticCallee): a call through an
// interface or a func value is not followed.
var panicbangAnalyzer = &analysis.Analyzer{
	Name:             "panicbang",
	Doc:              "report F!(...) that panics (in a function with no error result) inside a library package, and calls of functions of other packages that do",
	Requires:         []*analysis.Analyzer{inputsAnalyzer},
	FactTypes:        []analysis.Fact{new(panicsFact)},
	RunDespiteErrors: true,
	Run:              runPanicbang,
}

// panicsFact says that the function can panic because of a `!` that has no error to return.
type panicsFact struct {
	Site string // file:line:col of the `!`, as the user writes it
	Func string // the function that has it: pkg.f
}

func (*panicsFact) AFact() {}

func (f *panicsFact) String() string { return "panics via ! in " + f.Func }

func runPanicbang(pass *analysis.Pass) (any, error) {
	in := inputs(pass)
	info := pass.TypesInfo
	lib := pass.Pkg.Name() != "main"

	// The function declaration of each func literal, and the ones that call recover().
	declOf := make(map[*ast.FuncLit]*ast.FuncDecl)
	recovers := make(map[*ast.FuncDecl]bool)
	var decls []*ast.FuncDecl
	for _, f := range pass.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			decls = append(decls, fd)
			generated := make(map[*ast.CallExpr]bool) // the recover() that the lowering of defer F!(...) writes
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.FuncLit:
					declOf[n] = fd
				case *ast.AssignStmt:
					// defer F!(...) in a function with no error result becomes a deferred func that
					// does must_t1 := recover() first, so that a panic that is running goes on
					// (mustlower.go, deferStmt): the compiler's recover, not the user's.
					if n.Tok == token.DEFINE && len(n.Lhs) == 1 && len(n.Rhs) == 1 {
						if id, ok := n.Lhs[0].(*ast.Ident); ok && strings.HasPrefix(id.Name, "must_t") {
							if call, ok := n.Rhs[0].(*ast.CallExpr); ok {
								generated[call] = true
							}
						}
					}
				case *ast.CallExpr:
					if id, ok := ast.Unparen(n.Fun).(*ast.Ident); ok && !generated[n] {
						if _, ok := info.Uses[id].(*types.Builtin); ok && id.Name == "recover" {
							recovers[fd] = true
						}
					}
				}
				return true
			})
		}
	}

	// The panicking `!`: where, and in which declaration.
	type site struct {
		call check.MustCall
		decl *ast.FuncDecl // nil for a package variable's initializer
	}
	var sites []site
	if in.must != nil {
		for _, c := range in.must.Calls {
			if c.Kind != check.MustPanic {
				continue
			}
			s := site{call: c}
			switch fn := c.Func.(type) {
			case *ast.FuncDecl:
				s.decl = fn
			case *ast.FuncLit:
				s.decl = declOf[fn]
			}
			sites = append(sites, s)
		}
	}
	exempt := func(fd *ast.FuncDecl) bool {
		return fd != nil && (strings.HasPrefix(fd.Name.Name, "Must") || strings.HasPrefix(fd.Name.Name, "must"))
	}
	inTest := func(pos token.Pos) bool {
		return strings.HasSuffix(pass.Fset.PositionFor(pos, false).Filename, "_test.gp")
	}
	src := newSourceText(pass)

	// 1. The `!` that panics, in a library.
	direct := make(map[*ast.FuncDecl]*panicsFact)
	for _, s := range sites {
		if s.decl != nil && s.call.Func == ast.Node(s.decl) && !recovers[s.decl] && direct[s.decl] == nil {
			direct[s.decl] = &panicsFact{
				Site: shortPos(pass.Fset, s.call.Pos),
				Func: pass.Pkg.Name() + "." + funcName(s.decl),
			}
		}
		if !lib || exempt(s.decl) || inTest(s.call.Pos) || s.decl != nil && recovers[s.decl] {
			continue
		}
		code := src.text(s.call.Pos, s.call.End)
		where := whereIn(s.call.Func, s.decl)
		fix := "give it an error result, or declare it with a ! after its name so that it returns the error"
		switch s.call.Func.(type) {
		case *ast.FuncLit:
			fix = "give the func literal an error result"
		case nil:
			fix = "move the call into a function that returns the error"
		}
		pass.Report(analysis.Diagnostic{
			Pos: s.call.Pos, End: s.call.End, Category: "panicbang",
			Message: fmt.Sprintf("%s panics here: %s has no error result, so an error from the call becomes a panic. A library should return its errors: %s", code, where, fix),
		})
	}

	// 2. The functions that can panic through the calls they make (whole program).
	var edges = make(map[*ast.FuncDecl][]*ast.CallExpr)
	for _, fd := range decls {
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				edges[fd] = append(edges[fd], call)
			}
			return true
		})
	}
	panics := make(map[*ast.FuncDecl]*panicsFact, len(direct))
	for fd, f := range direct {
		panics[fd] = f
	}
	objOf := func(fd *ast.FuncDecl) *types.Func {
		fn, _ := info.Defs[fd.Name].(*types.Func)
		return fn
	}
	byObj := make(map[*types.Func]*panicsFact) // the functions of this package that panic
	for fd, f := range panics {
		if obj := objOf(fd); obj != nil {
			byObj[obj] = f
		}
	}
	calleeFact := func(call *ast.CallExpr) (*types.Func, *panicsFact) {
		g, _ := typeutil.Callee(info, call).(*types.Func)
		if g == nil {
			return nil, nil
		}
		g = g.Origin()
		if g.Pkg() == pass.Pkg {
			return g, byObj[g]
		}
		var f panicsFact
		if g.Pkg() != nil && pass.ImportObjectFact(g, &f) {
			return g, &f
		}
		return g, nil
	}
	for changed := true; changed; {
		changed = false
		for _, fd := range decls {
			if panics[fd] != nil || recovers[fd] {
				continue
			}
			for _, call := range edges[fd] {
				if _, f := calleeFact(call); f != nil {
					panics[fd] = f
					if obj := objOf(fd); obj != nil {
						byObj[obj] = f
					}
					changed = true
					break
				}
			}
		}
	}
	for fd, f := range panics {
		if obj := objOf(fd); obj != nil {
			pass.ExportObjectFact(obj, f)
		}
	}

	// 3. In a library: calls of functions of other packages that panic.
	if lib {
		for _, fd := range decls {
			if recovers[fd] || exempt(fd) || inTest(fd.Pos()) {
				continue
			}
			for _, call := range edges[fd] {
				g, f := calleeFact(call)
				if f == nil || g.Pkg() == pass.Pkg {
					continue
				}
				pass.Report(analysis.Diagnostic{
					Pos: call.Pos(), End: call.End(), Category: "panicbang",
					Message: fmt.Sprintf("%s can panic: %s uses ! at %s in a function with no error result, so an error there becomes a panic; a library should not let that reach its callers (handle it with recover, or ask %s for a version that returns the error)",
						types.ExprString(call.Fun), f.Func, f.Site, g.Pkg().Name()),
				})
			}
		}
	}
	return nil, nil
}

// whereIn describes the function that holds a `!`.
func whereIn(fn ast.Node, decl *ast.FuncDecl) string {
	switch fn := fn.(type) {
	case *ast.FuncDecl:
		return funcName(fn)
	case *ast.FuncLit:
		if decl != nil {
			return "the func literal in " + funcName(decl)
		}
		return "this func literal"
	}
	return "the initializer of this package variable"
}

// funcName is the name of a declared function as the user reads it: f, T.m, (*T).m.
func funcName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	t := fd.Recv.List[0].Type
	star := false
	if s, ok := t.(*ast.StarExpr); ok {
		t, star = s.X, true
	}
	switch x := t.(type) { // a generic receiver: T[K]
	case *ast.IndexExpr:
		t = x.X
	case *ast.IndexListExpr:
		t = x.X
	}
	name := types.ExprString(t)
	if star {
		return "(*" + name + ")." + fd.Name.Name
	}
	return name + "." + fd.Name.Name
}

// shortPos is file:line:col with the base name of the file, for messages.
func shortPos(fset *token.FileSet, pos token.Pos) string {
	p := fset.Position(pos)
	return fmt.Sprintf("%s:%d:%d", filepath.Base(p.Filename), p.Line, p.Column)
}

// sourceText reads the text of spans of the original source, for messages.
type sourceText struct {
	pass  *analysis.Pass
	files map[string][]byte
}

func newSourceText(pass *analysis.Pass) *sourceText {
	return &sourceText{pass: pass, files: make(map[string][]byte)}
}

// text returns the source between pos and end on one line, at most 40 bytes, or "F!(...)" when
// the file can't be read.
func (s *sourceText) text(pos, end token.Pos) string {
	const fallback = "F!(...)"
	p, e := s.pass.Fset.PositionFor(pos, false), s.pass.Fset.PositionFor(end, false)
	if p.Filename != e.Filename || !p.IsValid() {
		return fallback
	}
	b, ok := s.files[p.Filename]
	if !ok {
		b, _ = s.pass.ReadFile(p.Filename)
		s.files[p.Filename] = b
	}
	if p.Offset < 0 || e.Offset > len(b) || p.Offset >= e.Offset {
		return fallback
	}
	t := string(b[p.Offset:e.Offset])
	if strings.ContainsAny(t, "\n") || len(t) > 40 || !strings.Contains(t, "!") {
		if i := strings.Index(t, "!"); i >= 0 {
			return t[:i+1] + "(...)"
		}
		return fallback
	}
	return t
}
