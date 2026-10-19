package diff

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"sort"
	"strings"

	"github.com/huuhait/gocpp/tools/difftest/gen"
)

// Reducer shrinks a program that makes the subject differ from the reference, by deleting
// declarations and statements, unwrapping blocks and simplifying literals and arguments, as
// long as the program still builds with the reference and still shows the same kind of
// difference. The edits are on the source text at the positions of its syntax tree (delta
// debugging over the edits), so what is left is still the generator's readable code.
type Reducer struct {
	Ref, Sub Backend
	Budget   int // builds of the subject, the initial check included
	Mode     gen.Mode
	Log      io.Writer

	builds int
	prog   *gen.Program
	orig   Mismatch
	goes   bool // the program starts goroutines: check that the reference is deterministic
	last   Mismatch
}

var errBudget = errors.New("reduction budget used up")

const pkgPrefix = "package main\n\n"

type edit struct {
	start, end int
	repl       string
}

// Reduce returns the reduced source of p (declarations only, like Program.Src), the mismatch
// it shows, and the number of builds of the subject it used. The source is "" when p does not
// show the mismatch when it runs alone.
func (r *Reducer) Reduce(ctx context.Context, p *gen.Program, orig Mismatch) (string, Mismatch, int, error) {
	r.prog, r.orig = p, orig
	r.goes = strings.Contains(p.Src, "go func") || strings.Contains(p.Src, "go ")
	cur := p.Src
	m, ok, err := r.interesting(ctx, cur)
	if err != nil {
		return "", orig, r.builds, err
	}
	if !ok {
		return "", orig, r.builds, nil
	}
	r.last = m
	passes := []func(string) []edit{entryEdits, declEdits, stmtEdits, unwrapEdits, literalEdits, argEdits, declEdits}
	for round := 0; round < 3; round++ {
		changed := false
		for _, gen := range passes {
			next, err := r.ddmin(ctx, cur, gen)
			if err == errBudget {
				return r.finish(cur), r.last, r.builds, nil
			}
			if err != nil {
				return "", orig, r.builds, err
			}
			if next != cur {
				cur, changed = next, true
			}
		}
		if !changed {
			break
		}
	}
	return r.finish(cur), r.last, r.builds, nil
}

func (r *Reducer) finish(src string) string {
	if out, err := format.Source([]byte(pkgPrefix + src)); err == nil {
		return strings.TrimPrefix(string(out), pkgPrefix)
	}
	return src
}

// interesting builds the program with the reference and the subject and reports whether it
// shows the original kind of mismatch.
func (r *Reducer) interesting(ctx context.Context, src string) (Mismatch, bool, error) {
	if err := ctx.Err(); err != nil {
		return Mismatch{}, false, err
	}
	q := *r.prog
	q.Src = src
	bundle, err := gen.Bundle([]*gen.Program{&q}, r.Mode)
	if err != nil {
		return Mismatch{}, false, nil
	}
	ref, err := r.Ref.Run(ctx, bundle)
	if err != nil {
		return Mismatch{}, false, err
	}
	if !ref.BuildOK || ref.TimedOut || ref.Truncated || (r.Mode == gen.Recovered && ref.Exit != 0) {
		return Mismatch{}, false, nil
	}
	if r.goes {
		ref2, err := r.Ref.Run(ctx, bundle)
		if err != nil {
			return Mismatch{}, false, err
		}
		if ref2.Stdout != ref.Stdout || ref2.Exit != ref.Exit {
			return Mismatch{}, false, nil
		}
	}
	if r.builds >= r.Budget {
		return Mismatch{}, false, errBudget
	}
	r.builds++
	sub, err := r.Sub.Run(ctx, bundle)
	if err != nil {
		return Mismatch{}, false, err
	}
	var mms []Mismatch
	switch {
	case !sub.BuildOK:
		k := KindBuild
		if sub.ICE {
			k = KindICE
		}
		sig := FirstError(sub.BuildErr)
		if sub.ICE {
			sig = iceSig(sub.BuildErr)
		}
		mms = []Mismatch{{Prog: q.ID, Kind: k, Sig: sig, Detail: clip(sub.BuildErr, 3000)}}
	case r.Mode == gen.Recovered:
		mms, _ = CompareBundle([]int{q.ID}, RefOf(ref), sub)
	default:
		mms = CompareStandalone(q.ID, ref, sub)
	}
	for _, m := range mms {
		if m.Kind != r.orig.Kind {
			continue
		}
		if (m.Kind == KindBuild || m.Kind == KindICE) && m.Sig != r.orig.Sig && MaskSig(m.Sig) != MaskSig(r.orig.Sig) {
			continue
		}
		return m, true, nil
	}
	return Mismatch{}, false, nil
}

// ddmin removes groups of edits as long as the program stays interesting. It returns the
// final source.
func (r *Reducer) ddmin(ctx context.Context, src string, gen func(string) []edit) (string, error) {
	n := 2
	for {
		es, err := editsOf(src, gen)
		if err != nil || len(es) == 0 {
			return src, nil
		}
		if n > len(es) {
			n = len(es)
		}
		size := (len(es) + n - 1) / n
		reduced := false
		for i := 0; i < len(es); i += size {
			j := min(i+size, len(es))
			cand := apply(src, es[i:j])
			if cand == src {
				continue
			}
			m, ok, err := r.interesting(ctx, cand)
			if err != nil {
				return src, err
			}
			if ok {
				src = cand
				r.last = m
				reduced = true
				break
			}
		}
		if reduced {
			n = max(n-1, 2)
			continue
		}
		if n >= len(es) {
			return src, nil
		}
		n = min(len(es), n*2)
	}
}

func editsOf(src string, gen func(string) []edit) ([]edit, error) {
	text := pkgPrefix + src
	if _, err := parser.ParseFile(token.NewFileSet(), "x.go", text, 0); err != nil {
		return nil, err
	}
	es := gen(text)
	// the edits are positions in text; shift to src
	for i := range es {
		es[i].start -= len(pkgPrefix)
		es[i].end -= len(pkgPrefix)
	}
	return es, nil
}

// apply applies the edits to src, leaving out those inside the span of an earlier edit.
func apply(src string, es []edit) string {
	es = append([]edit(nil), es...)
	sort.SliceStable(es, func(i, j int) bool {
		if es[i].start != es[j].start {
			return es[i].start < es[j].start
		}
		return es[i].end > es[j].end
	})
	var b strings.Builder
	pos := 0
	for _, e := range es {
		if e.start < pos {
			continue // inside an edit already applied
		}
		b.WriteString(src[pos:e.start])
		b.WriteString(e.repl)
		pos = e.end
	}
	b.WriteString(src[pos:])
	return b.String()
}

func parse(text string) (*token.FileSet, *ast.File) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", text, 0)
	if err != nil {
		return fset, nil
	}
	return fset, f
}

func off(fset *token.FileSet, p token.Pos) int { return fset.Position(p).Offset }

// declEdits deletes top-level declarations except imports and the program's entry.
func declEdits(text string) []edit {
	fset, f := parse(text)
	if f == nil {
		return nil
	}
	var es []edit
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && strings.HasPrefix(d.Name.Name, "prog_") {
				continue
			}
			start := off(fset, d.Pos())
			if d.Doc != nil {
				start = off(fset, d.Doc.Pos())
			}
			es = append(es, edit{start, off(fset, d.End()), ""})
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}
			start := off(fset, d.Pos())
			if d.Doc != nil {
				start = off(fset, d.Doc.Pos())
			}
			es = append(es, edit{start, off(fset, d.End()), ""})
		}
	}
	return es
}

// entryEdits deletes the statements of the entry function prog_N: the calls of the scenarios.
// Removing a call keeps the program valid (an unused function is fine); declEdits then removes
// what nothing calls any more.
func entryEdits(text string) []edit {
	fset, f := parse(text)
	if f == nil {
		return nil
	}
	var es []edit
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || !strings.HasPrefix(fd.Name.Name, "prog_") || fd.Body == nil {
			continue
		}
		for _, s := range fd.Body.List {
			es = append(es, edit{off(fset, s.Pos()), off(fset, s.End()), ""})
		}
	}
	return es
}

func blockLists(f *ast.File, visit func(list []ast.Stmt)) {
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.BlockStmt:
			visit(n.List)
		case *ast.CaseClause:
			visit(n.Body)
		case *ast.CommClause:
			visit(n.Body)
		}
		return true
	})
}

// stmtEdits deletes statements (each deletion removes what the statement holds).
func stmtEdits(text string) []edit {
	fset, f := parse(text)
	if f == nil {
		return nil
	}
	var es []edit
	blockLists(f, func(list []ast.Stmt) {
		for _, s := range list {
			if _, ok := s.(*ast.EmptyStmt); ok {
				continue
			}
			es = append(es, edit{off(fset, s.Pos()), off(fset, s.End()), ""})
		}
	})
	return es
}

// unwrapEdits replaces an if, for or switch statement, a block or a func literal call by the
// statements of its body.
func unwrapEdits(text string) []edit {
	fset, f := parse(text)
	if f == nil {
		return nil
	}
	inner := func(b *ast.BlockStmt) string {
		return text[off(fset, b.Lbrace)+1 : off(fset, b.Rbrace)]
	}
	var es []edit
	blockLists(f, func(list []ast.Stmt) {
		for _, s := range list {
			span := edit{start: off(fset, s.Pos()), end: off(fset, s.End())}
			switch s := s.(type) {
			case *ast.IfStmt:
				e := span
				e.repl = inner(s.Body)
				es = append(es, e)
				if b, ok := s.Else.(*ast.BlockStmt); ok {
					e.repl = inner(b)
					es = append(es, e)
				}
			case *ast.ForStmt:
				e := span
				e.repl = inner(s.Body)
				es = append(es, e)
			case *ast.RangeStmt:
				e := span
				e.repl = inner(s.Body)
				es = append(es, e)
			case *ast.BlockStmt:
				e := span
				e.repl = inner(s)
				es = append(es, e)
			}
		}
	})
	return es
}

// literalEdits replaces integer, float and string literals by simple ones.
func literalEdits(text string) []edit {
	fset, f := parse(text)
	if f == nil {
		return nil
	}
	var es []edit
	ast.Inspect(f, func(n ast.Node) bool {
		bl, ok := n.(*ast.BasicLit)
		if !ok {
			return true
		}
		var repl string
		switch bl.Kind {
		case token.INT:
			if bl.Value != "0" && bl.Value != "1" {
				repl = "1"
			}
		case token.FLOAT:
			if bl.Value != "1.5" {
				repl = "1.5"
			}
		case token.STRING:
			if bl.Value != `""` && bl.Value != `"a"` && !strings.Contains(bl.Value, "%") {
				repl = `"a"`
			}
		}
		if repl != "" {
			es = append(es, edit{off(fset, bl.Pos()), off(fset, bl.End()), repl})
		}
		return true
	})
	return es
}

// argEdits removes the arguments of calls of fmt.Println and of append, and elements of
// composite literals.
func argEdits(text string) []edit {
	fset, f := parse(text)
	if f == nil {
		return nil
	}
	var es []edit
	remove := func(list []ast.Expr, skip int) {
		for i := skip; i < len(list); i++ {
			start := off(fset, list[i].Pos())
			end := off(fset, list[i].End())
			if i > 0 {
				start = off(fset, list[i-1].End())
			} else if i+1 < len(list) {
				end = off(fset, list[i+1].Pos())
			}
			es = append(es, edit{start, end, ""})
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "fmt" && sel.Sel.Name == "Println" && len(n.Args) > 1 {
					remove(n.Args, 0)
				}
			}
		case *ast.CompositeLit:
			if len(n.Elts) > 1 {
				if _, isKV := n.Elts[0].(*ast.KeyValueExpr); isKV {
					remove(n.Elts, 0)
				}
			}
		}
		return true
	})
	return es
}

func (r *Reducer) String() string { return fmt.Sprintf("reducer(%d builds)", r.builds) }
