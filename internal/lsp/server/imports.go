package server

import (
	"bytes"
	"cmp"
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"

	gpformat "github.com/huuhait/gocpp/internal/format"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// organizeImports returns src, the source of the parsed file f (tok is its
// token.File), with its imports organized, for the code action
// "source.organizeImports" (Phase 11 README §2.3). It works like goimports
// (gopls's "Organize Imports"), without adding missing imports (that is
// auto-import, step 11.6):
//
//   - All import declarations become one. An import that unused names (by
//     the byte offset of the start of its spec) is removed, with its own
//     comments: those on its line. The caller passes only gocpp packages
//     that the type check reported as "imported and not used". Header and
//     namespace imports are never removed (see unusedImports).
//   - A group of imports is a run of import lines with no empty line and
//     no comment line between them (as in gofmt). Each group keeps its
//     place, and its gocpp imports are sorted like goimports sorts them:
//     the standard packages (no dot in the first part of the path) first,
//     then the others, each by path, with an empty line between the two.
//     The same import twice in a group becomes one, as in gofmt. An empty
//     line comes before the comment lines of a group that follows a group
//     of two or more imports, as goimports usually does.
//   - The header and namespace imports ("<vector>", "./engine.hpp",
//     fs "::std::filesystem") move after all the gocpp imports, in the
//     order in which they are written and in the same groups: they are
//     never sorted, because the order of #include lines can matter (see
//     internal/format).
//   - The comment lines above a group stay above it. When the group mixes
//     gocpp and header imports, they stay with the part of the group that
//     has its first import. When every import of the group is removed,
//     they go to the next group (or to the end of the imports; when no
//     import is left at all, they stay as comments where the imports
//     were). So no comment line is lost, but such a comment can end up
//     above imports that it doesn't describe. A comment before an import
//     on its line (/* p */ "x") stays there (goimports moves it up a line).
//
// The rest of the file does not change, but for an empty line after the
// imports when there is none (gofmt wants it); the import declarations are
// printed the way gocpp fmt prints them. When no import is left, the
// declarations go with the empty line after them. src itself is returned
// when the imports are organized already.
//
// Two rules keep the rest of the file safe:
//   - When code (anything but comments and spaces) shares a line with an
//     import declaration, its "(" or its ")" (package p; import "fmt", or
//     import "fmt"; var x = 1), src is returned as it is: no organizing.
//     gofmt puts such code on a line of its own, and an editor formats on
//     save right before it organizes the imports.
//   - The tokens of the file outside the import declarations, comments
//     included, must be the same before and after (sameCode). If not,
//     organizeImports returns an error.
func organizeImports(filename string, src []byte, f *ast.File, tok *token.File, unused map[int]bool) ([]byte, error) {
	var decls []*ast.GenDecl
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.IMPORT {
			break // the imports come first
		}
		decls = append(decls, g)
	}
	if len(decls) == 0 {
		return src, nil
	}
	toks := scanFile(filename, src)
	if codeOnImportLines(decls, tok, toks) {
		return src, nil
	}
	o := &organizer{filename: filename, src: src, tok: tok, unused: unused}
	o.collect(f, decls)
	text, err := o.render(len(decls) == 1 && decls[0].Lparen.IsValid())
	if err != nil {
		return nil, err
	}
	start, end := o.start, o.end
	if text == "" {
		start, end = o.removal(start, end)
	}
	var out bytes.Buffer
	out.Write(src[:start])
	out.WriteString(text)
	left := [2]int{start, out.Len()} // where the comment lines are left, if no import is
	if o.commentsOnly {
		// A comment after the ")" goes on the next line.
		e := end
		for e < len(src) && (src[e] == ' ' || src[e] == '\t') {
			e++
		}
		if e < len(src) && lineEnd(src[e:]) == "" {
			out.WriteString(o.eol())
			end = e
		}
	}
	if text != "" {
		// gofmt wants an empty line between the imports and what comes
		// after them on the next line. (It accepts none after a single
		// import with a comment, but the new imports may look different.)
		rest := src[end:]
		eol := lineEnd(rest)
		next := bytes.TrimLeft(rest[len(eol):], " \t")
		if eol != "" && len(next) > 0 && lineEnd(next) == "" {
			out.WriteString(eol)
		}
	}
	out.Write(src[end:])
	if bytes.Equal(out.Bytes(), src) {
		return src, nil
	}
	if !o.commentsOnly {
		left = [2]int{}
	}
	if err := sameCode(filename, src, decls, tok, toks, unused, out.Bytes(), left); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// eol returns the line end of the first line of src ("\n" when it has none).
func (o *organizer) eol() string {
	if i := bytes.IndexByte(o.src, '\n'); i > 0 && o.src[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

// A scanned is a token of a file, without the semicolons that the scanner
// puts at line ends.
type scanned struct {
	off  int
	line int
	tok  token.Token
	lit  string
}

// scanFile returns the tokens of src, with the comments.
func scanFile(filename string, src []byte) []scanned {
	fset := token.NewFileSet()
	file := fset.AddFile(filename, -1, len(src))
	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)
	var toks []scanned
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			return toks
		}
		if tok == token.SEMICOLON && lit == "\n" {
			continue
		}
		toks = append(toks, scanned{file.Offset(pos), file.Line(pos), tok, lit})
	}
}

// codeOnImportLines reports whether a token that is not a comment and not
// part of an import declaration is on a line of the start, the "(", the
// ")" or the end of an import declaration in decls (tok is their file).
func codeOnImportLines(decls []*ast.GenDecl, tok *token.File, toks []scanned) bool {
	lines := make(map[int]bool)
	type span struct{ start, end int }
	var spans []span
	for _, d := range decls {
		lines[tok.Line(d.Pos())] = true
		lines[tok.Line(d.End())] = true
		if d.Lparen.IsValid() {
			lines[tok.Line(d.Lparen)] = true
		}
		if d.Rparen.IsValid() {
			lines[tok.Line(d.Rparen)] = true
		}
		spans = append(spans, span{tok.Offset(d.Pos()), tok.Offset(d.End())})
	}
	for _, t := range toks {
		if t.tok == token.COMMENT || !lines[t.line] {
			continue
		}
		if !slices.ContainsFunc(spans, func(s span) bool { return s.start <= t.off && t.off < s.end }) {
			return true
		}
	}
	return false
}

// outside returns the tokens of toks that are not in the import
// declarations decls (of the file tok): from the start of the first one to
// the end of the last one, with the comments right after it on its last
// line when it has no parentheses (the comment of its import); nor in the
// byte range skip.
func outside(decls []*ast.GenDecl, tok *token.File, toks []scanned, skip [2]int) []scanned {
	start, end := -1, -1
	parens := true
	if len(decls) > 0 {
		last := decls[len(decls)-1]
		start, end, parens = tok.Offset(decls[0].Pos()), tok.Offset(last.End()), last.Lparen.IsValid()
	}
	var out []scanned
	after := false // the tokens after end, on its line, are comments so far
	for _, t := range toks {
		switch {
		case start <= t.off && t.off < end:
			after = true
			continue
		case after && !parens && t.tok == token.COMMENT && t.line == tok.Line(tok.Pos(end)):
			continue
		case skip[0] <= t.off && t.off < skip[1]:
			continue
		}
		after = false
		out = append(out, t)
	}
	return out
}

// sameCode checks that the organized file res has the tokens of src (whose
// import declarations are decls, in the file tok, and whose tokens are
// toks) outside the import declarations, comments included. left is the
// byte range of res where the comment lines of the imports were left,
// when no import is left; it must hold comments only.
//
// It also checks the imports themselves (sameImports): the header and
// namespace imports are the same, in the same order, and the gocpp
// imports are those of src but the unused ones (by the offsets of their
// specs), each one at least once and no more often than in src (the same
// import twice may become one).
func sameCode(filename string, src []byte, decls []*ast.GenDecl, tok *token.File, toks []scanned, unused map[int]bool, res []byte, left [2]int) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, res, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		return fmt.Errorf("organize imports: the result doesn't parse: %v", err)
	}
	var newDecls []*ast.GenDecl
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			newDecls = append(newDecls, g)
		}
	}
	resToks := scanFile(filename, res)
	for _, t := range resToks {
		if left[0] <= t.off && t.off < left[1] && t.tok != token.COMMENT {
			return fmt.Errorf("organize imports: code where only comments should be left, at offset %d", t.off)
		}
	}
	a := outside(decls, tok, toks, [2]int{})
	b := outside(newDecls, fset.File(f.FileStart), resToks, left)
	if !slices.EqualFunc(a, b, func(x, y scanned) bool { return x.tok == y.tok && x.lit == y.lit }) {
		return fmt.Errorf("organize imports: the code outside the imports would change")
	}
	var before []*ast.ImportSpec
	for _, d := range decls {
		for _, s := range d.Specs {
			if s := s.(*ast.ImportSpec); !unused[tok.Offset(s.Pos())] || load.IsHeaderOrNamespace(specPath(s)) {
				before = append(before, s)
			}
		}
	}
	return sameImports(before, f.Imports)
}

// specPath returns the unquoted path of s, or "" when its literal is bad.
func specPath(s *ast.ImportSpec) string {
	p, err := strconv.Unquote(s.Path.Value)
	if err != nil {
		return ""
	}
	return p
}

// sameImports checks the imports after organizing against those before,
// without the removed ones (see sameCode).
func sameImports(before, after []*ast.ImportSpec) error {
	key := func(s *ast.ImportSpec) string {
		if s.Name != nil {
			return s.Name.Name + " " + s.Path.Value
		}
		return s.Path.Value
	}
	split := func(specs []*ast.ImportSpec) (headers []string, gocpp map[string]int) {
		gocpp = make(map[string]int)
		for _, s := range specs {
			if load.IsHeaderOrNamespace(specPath(s)) {
				headers = append(headers, key(s))
			} else {
				gocpp[key(s)]++
			}
		}
		return headers, gocpp
	}
	h1, g1 := split(before)
	h2, g2 := split(after)
	if !slices.Equal(h1, h2) {
		return fmt.Errorf("organize imports: the header imports would change: %v, then %v", h1, h2)
	}
	for k, n := range g1 {
		if g2[k] < 1 || g2[k] > n {
			return fmt.Errorf("organize imports: the import %s would be there %d times, not %d", k, g2[k], n)
		}
	}
	for k := range g2 {
		if g1[k] == 0 {
			return fmt.Errorf("organize imports: a new import %s", k)
		}
	}
	return nil
}

// lineEnd returns the line end at the start of b ("\n" or "\r\n"), or "".
func lineEnd(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte("\n")):
		return "\n"
	case bytes.HasPrefix(b, []byte("\r\n")):
		return "\r\n"
	}
	return ""
}

// An organizer holds the imports of one file while organizeImports works.
type organizer struct {
	filename string
	src      []byte
	tok      *token.File
	unused   map[int]bool

	// start and end are the byte offsets of the part of src that the
	// new imports replace: from the first import keyword to the end of
	// the last import declaration (with the comments on its line when it
	// has no parentheses).
	start, end int

	runs []*importRun
	// tail are the comment lines after the last import, inside the
	// parentheses; tailBlank says that an empty line comes before them.
	tail      []commentLine
	tailBlank bool
	removed   bool // an unused import was removed
	// commentsOnly: no import is left, but comment lines are (render).
	commentsOnly bool
	// parenComment: a comment follows the ")" of the last declaration on
	// its line. Then a single import keeps the parentheses: without them,
	// the comment would become the import's own.
	parenComment bool
}

// An importRun is a group of imports: import lines one after the other.
type importRun struct {
	// blank says that an empty line came before the run (always true for
	// the first run of a second import declaration).
	blank bool
	// comments are the comment lines right above the run, after the
	// import before it.
	comments []commentLine
	// gap says that an empty line came between the comments and the
	// first import.
	gap   bool
	specs []*importSpec
}

// A commentLine is a comment group on lines of its own, above a run or at
// the end. blank says that an empty line comes before it (not used for
// the first one of a run, which has importRun.blank).
type commentLine struct {
	g     *ast.CommentGroup
	blank bool
}

// An importSpec is one import and its comments.
type importSpec struct {
	spec   *ast.ImportSpec
	path   string // unquoted ("" when the literal is bad)
	header bool   // a header or a namespace import
	// before are comments on the line of the import, before it; after are
	// the comments after its start on the line where it ends.
	before, after []*ast.CommentGroup
}

func (o *organizer) offset(p token.Pos) int { return o.tok.Offset(p) }
func (o *organizer) line(p token.Pos) int   { return o.tok.Line(p) }

// collect fills the runs from the import declarations of f.
func (o *organizer) collect(f *ast.File, decls []*ast.GenDecl) {
	last := decls[len(decls)-1]
	o.start, o.end = o.offset(decls[0].Pos()), o.offset(last.End())
	for _, g := range f.Comments {
		if g.Pos() < last.End() || o.line(g.Pos()) != o.line(last.End()) {
			continue
		}
		if last.Lparen.IsValid() {
			o.parenComment = true // ) // end
		} else {
			// import "fmt" // the comment of the import
			o.end = max(o.end, o.offset(g.End()))
		}
	}
	var specs []*importSpec
	var run *importRun
	for di, d := range decls {
		for si, s := range d.Specs {
			is := s.(*ast.ImportSpec)
			p, err := strconv.Unquote(is.Path.Value)
			if err != nil {
				p = ""
			}
			spec := &importSpec{spec: is, path: p, header: load.IsHeaderOrNamespace(p)}
			if run == nil || si == 0 || o.line(is.Pos()) > o.line(d.Specs[si-1].End())+1 {
				run = &importRun{blank: di > 0 && si == 0}
				o.runs = append(o.runs, run)
			}
			run.specs = append(run.specs, spec)
			specs = append(specs, spec)
		}
	}

	// Give each comment to its import: the comments after the start of
	// an import on the line where it ends are its own (after); a comment
	// on the line of an import, before it, too (before); the other
	// comment lines are above a run (comments), or after the last import
	// (tail).
	firstOf := make(map[*importSpec]*importRun)
	for _, r := range o.runs {
		firstOf[r.specs[0]] = r
	}
	next := 0 // the first import that ends after the comment
	for _, g := range o.splitGroups(f.Comments) {
		if o.offset(g.Pos()) < o.start || o.offset(g.End()) > o.end {
			continue
		}
		for next < len(specs) && specs[next].spec.End() <= g.Pos() {
			next++
		}
		switch {
		case next < len(specs) && specs[next].spec.Pos() < g.Pos():
			// Inside the import: name /* here */ "path".
			specs[next].after = append(specs[next].after, g)
		case next > 0 && o.line(g.Pos()) == o.line(specs[next-1].spec.End()):
			specs[next-1].after = append(specs[next-1].after, g)
		case next == len(specs):
			o.tail = append(o.tail, o.commentLine(o.tail, g))
		case firstOf[specs[next]] != nil && o.line(g.End()) < o.line(specs[next].spec.Pos()):
			r := firstOf[specs[next]]
			r.comments = append(r.comments, o.commentLine(r.comments, g))
		default:
			specs[next].before = append(specs[next].before, g)
		}
	}

	// The empty lines before the runs and before the tail.
	prevEnd := 0 // the last line of the import before
	for i, s := range specs {
		if r := firstOf[s]; r != nil && i > 0 && !r.blank {
			first := o.line(s.spec.Pos())
			if len(r.comments) > 0 {
				first = o.line(r.comments[0].g.Pos())
			}
			r.blank = first > prevEnd+1
		}
		if r := firstOf[s]; r != nil && len(r.comments) > 0 {
			r.gap = o.line(s.spec.Pos()) > o.line(r.comments[len(r.comments)-1].g.End())+1
		}
		prevEnd = o.endLine(s)
	}
	if len(o.tail) > 0 && len(specs) > 0 {
		o.tailBlank = o.line(o.tail[0].g.Pos()) > prevEnd+1
	}

	// Remove the unused imports. The comment lines above a run that has
	// no import left go to the next run, or to the tail.
	var kept []*importRun
	var carry *importRun // a run without imports, whose comments move on
	for _, r := range o.runs {
		r.specs = slices.DeleteFunc(r.specs, func(s *importSpec) bool {
			if o.unused[o.offset(s.spec.Pos())] && !s.header {
				o.removed = true
				return true
			}
			return false
		})
		if carry != nil {
			if len(r.comments) > 0 {
				r.comments[0].blank = r.blank
			} else {
				r.gap = carry.gap || r.blank
			}
			r.comments = append(carry.comments, r.comments...)
			r.blank = carry.blank
			carry = nil
		}
		if len(r.specs) == 0 {
			if len(r.comments) > 0 {
				carry = r
			}
			continue
		}
		kept = append(kept, r)
	}
	o.runs = kept
	if carry != nil {
		if len(o.tail) > 0 {
			o.tail[0].blank = o.tailBlank
		}
		o.tail = append(carry.comments, o.tail...)
		o.tailBlank = carry.blank
	}
}

// commentLine returns g as the next comment line after the lines before.
func (o *organizer) commentLine(before []commentLine, g *ast.CommentGroup) commentLine {
	blank := false
	if len(before) > 0 {
		blank = o.line(g.Pos()) > o.line(before[len(before)-1].g.End())+1
	}
	return commentLine{g, blank}
}

// splitGroups returns the comment groups split by lines: a group holds
// comments that follow each other on the same line (a comment that spans
// lines ends its line on the last one). The parser puts comment lines and
// a comment before an import on the next line (// c, then /* p */ "fmt")
// into one group, but the first is above the import and the second is
// the import's own. The groups are new; the file's are not changed.
func (o *organizer) splitGroups(groups []*ast.CommentGroup) []*ast.CommentGroup {
	var out []*ast.CommentGroup
	for _, g := range groups {
		cur := &ast.CommentGroup{}
		for _, c := range g.List {
			if len(cur.List) > 0 && o.line(c.Pos()) > o.line(cur.List[len(cur.List)-1].End()) {
				out = append(out, cur)
				cur = &ast.CommentGroup{}
			}
			cur.List = append(cur.List, c)
		}
		out = append(out, cur)
	}
	return out
}

// endLine returns the last line of the import s and its comments.
func (o *organizer) endLine(s *importSpec) int {
	end := s.spec.End()
	for _, g := range s.after {
		end = max(end, g.End())
	}
	return o.line(end)
}

// group returns the goimports group of a gocpp import: 0 for a standard
// package (no dot in the first part of the path), 1 for the others.
func group(path string) int {
	first, _, _ := strings.Cut(path, "/")
	if strings.Contains(first, ".") {
		return 1
	}
	return 0
}

// comment returns the line comment of s, which gofmt sorts by after the
// path and the name.
func (s *importSpec) comment() string {
	if s.spec.Comment == nil {
		return ""
	}
	return s.spec.Comment.Text()
}

func (s *importSpec) name() string {
	if s.spec.Name == nil {
		return ""
	}
	return s.spec.Name.Name
}

// render returns the new import declaration (without a line end), or ""
// when no import is left. parens says that the file had one import
// declaration with parentheses: then a single import keeps them, unless
// an unused import was removed (as goimports does).
func (o *organizer) render(parens bool) (string, error) {
	type outRun struct {
		blank    bool
		comments []commentLine
		gap      bool
		specs    []*importSpec
	}
	// add adds a part of a run to list. A part with no empty line and no
	// comment line before it would be one group with the part before it
	// (for gofmt too), so it joins that part.
	add := func(list []outRun, r outRun) []outRun {
		if len(list) > 0 && !r.blank && len(r.comments) == 0 {
			list[len(list)-1].specs = append(list[len(list)-1].specs, r.specs...)
			return list
		}
		return append(list, r)
	}
	var gocpp, headers []outRun
	for _, r := range o.runs {
		var g, h []*importSpec
		for _, s := range r.specs {
			if s.header {
				h = append(h, s)
			} else {
				g = append(g, s)
			}
		}
		// The comment lines go with the part that has the run's first
		// import.
		gc, hc := r.comments, []commentLine(nil)
		if r.specs[0].header {
			gc, hc = nil, r.comments
		}
		if len(g) > 0 {
			gocpp = add(gocpp, outRun{r.blank, gc, r.gap, g})
		}
		if len(h) > 0 {
			headers = add(headers, outRun{r.blank, hc, r.gap, h})
		}
	}
	for i := range gocpp {
		gocpp[i].specs = sortSpecs(gocpp[i].specs)
	}
	if len(headers) > 0 && len(gocpp) > 0 {
		headers[0].blank = true
	}
	all := append(gocpp, headers...)
	n := 0
	for _, r := range all {
		n += len(r.specs)
	}
	if n == 0 {
		// No import is left: only the comment lines, if any, stay.
		o.commentsOnly = len(o.tail) > 0
		var b strings.Builder
		for i, c := range o.tail {
			if i > 0 && c.blank {
				b.WriteString("\n")
			}
			o.writeGroup(&b, c.g, "")
			b.WriteString("\n")
		}
		return strings.TrimSuffix(b.String(), "\n"), nil
	}

	var b strings.Builder
	if n == 1 && len(all[0].comments) == 0 && len(o.tail) == 0 && len(all[0].specs[0].before) == 0 &&
		(!parens || o.removed) && !o.parenComment {
		b.WriteString("import ")
		o.writeSpec(&b, all[0].specs[0])
	} else {
		b.WriteString("import (\n")
		for i, r := range all {
			// An empty line before the comment lines that follow a group
			// of two or more imports, as goimports usually does.
			if i > 0 && (r.blank || len(r.comments) > 0 && len(all[i-1].specs) >= 2) {
				b.WriteString("\n")
			}
			if len(r.comments) > 0 {
				o.writeComments(&b, r.comments)
				if r.gap {
					b.WriteString("\n")
				}
			}
			for k, s := range r.specs {
				if k > 0 && !s.header && group(s.path) != group(r.specs[k-1].path) {
					b.WriteString("\n")
				}
				b.WriteString("\t")
				o.writeSpec(&b, s)
			}
		}
		if len(o.tail) > 0 {
			if o.tailBlank {
				b.WriteString("\n")
			}
			o.writeComments(&b, o.tail)
		}
		b.WriteString(")\n")
	}

	// Print it the way gocpp fmt does (it aligns the comments after the
	// imports, for example). The order stays: every group is sorted
	// already, and gocpp fmt doesn't sort a group with a header.
	// gofmt is not always done after one run (a line with only /* */
	// comments, which it may make itself, changes the alignment of the
	// comments after the next import), so it runs until nothing changes.
	const pkg = "package p\n\n"
	res := []byte(pkg + b.String())
	for range 3 {
		next, err := gpformat.Source(o.filename, res)
		if err != nil {
			return "", fmt.Errorf("organize imports: %v", err)
		}
		done := bytes.Equal(next, res)
		res = next
		if done {
			break
		}
	}
	text, ok := bytes.CutPrefix(res, []byte(pkg))
	if !ok {
		return "", fmt.Errorf("organize imports: unexpected output %q", res)
	}
	return strings.TrimSuffix(string(text), "\n"), nil
}

// sortSpecs sorts gocpp imports like goimports: by group, path, name and
// line comment. The same import twice becomes one: the first one goes, if
// it has no line comment (gofmt's rule, so gofmt finds nothing more to
// remove); its other comments go to the one that stays.
func sortSpecs(g []*importSpec) []*importSpec {
	slices.SortStableFunc(g, func(x, y *importSpec) int {
		return cmp.Or(cmp.Compare(group(x.path), group(y.path)), cmp.Compare(x.path, y.path),
			cmp.Compare(x.name(), y.name()), cmp.Compare(x.comment(), y.comment()))
	})
	for i := 0; i+1 < len(g); {
		x, y := g[i], g[i+1]
		if x.path == y.path && x.name() == y.name() && x.spec.Comment == nil {
			y.before = append(x.before, y.before...)
			y.after = append(x.after, y.after...)
			g = slices.Delete(g, i, i+1)
			continue
		}
		i++
	}
	return g
}

// writeSpec writes the import s and its comments, and a line end.
func (o *organizer) writeSpec(b *strings.Builder, s *importSpec) {
	for _, g := range s.before {
		o.writeGroup(b, g, "\t")
		b.WriteString(" ")
	}
	if s.spec.Name != nil {
		b.WriteString(s.spec.Name.Name)
		b.WriteString(" ")
	}
	b.WriteString(s.spec.Path.Value)
	for _, g := range s.after {
		b.WriteString(" ")
		o.writeGroup(b, g, "\t")
	}
	b.WriteString("\n")
}

// writeComments writes comment lines, indented, with the empty lines
// between them (not before the first one).
func (o *organizer) writeComments(b *strings.Builder, lines []commentLine) {
	for i, c := range lines {
		if i > 0 && c.blank {
			b.WriteString("\n")
		}
		b.WriteString("\t")
		o.writeGroup(b, c.g, "\t")
		b.WriteString("\n")
	}
}

// writeGroup writes the comments of a group as they are written (a
// comment on the line of the one before stays on that line; the others
// start a new line with indent).
func (o *organizer) writeGroup(b *strings.Builder, g *ast.CommentGroup, indent string) {
	for i, c := range g.List {
		if i > 0 {
			if o.line(c.Pos()) > o.line(g.List[i-1].End()) {
				b.WriteString("\n" + indent)
			} else {
				b.WriteString(" ")
			}
		}
		b.WriteString(c.Text) // without "\r" (go/scanner)
	}
}

// removal returns the byte range of src to remove when no import is left,
// for the range [start, end) of the imports. Spaces after the imports go.
// When nothing else is on their lines, the whole lines go, and also one empty line
// after them when there is one before them; at the end of the file, the
// empty line before them goes instead, so that the file ends as gocpp fmt
// ends it. A comment after the imports on their last line (") // end")
// stays, at the start of the line.
func (o *organizer) removal(start, end int) (int, int) {
	src := o.src
	spaces := func(i int) int {
		for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
			i++
		}
		return i
	}
	e := spaces(end)
	lineEnd := e == len(src) || src[e] == '\n' || src[e] == '\r' && e+1 < len(src) && src[e+1] == '\n'
	s := start
	for s > 0 && (src[s-1] == ' ' || src[s-1] == '\t') {
		s--
	}
	lineStart := s == 0 || src[s-1] == '\n'
	switch {
	case !lineEnd:
		return start, e // code or a comment comes next on the line
	case !lineStart:
		return s, e // package p; import "fmt"
	}
	switch {
	case e == len(src):
	case src[e] == '\r':
		e += 2
	default:
		e++
	}
	// The line end before s, if the line before is empty.
	blankBefore := 0
	switch {
	case s >= 2 && src[s-2] == '\n':
		blankBefore = 1
	case s >= 3 && src[s-3] == '\n' && src[s-2] == '\r':
		blankBefore = 2
	}
	switch {
	case blankBefore == 0:
	case e == len(src):
		s -= blankBefore
	case src[e] == '\n':
		e++
	case src[e] == '\r' && e+1 < len(src) && src[e+1] == '\n':
		e += 2
	}
	return s, e
}
