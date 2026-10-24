package load

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

// Embedding files in the program (step 10.8, ADR-058): the macro #embed("f")
// and the directive //go:embed of Go. Both give a constant string with the
// bytes of a file, found at compile time:
//
//	var logo []byte      // with //go:embed logo.png
//	s := #embed("a.txt") // a string constant
//
// The rules are Go's (package embed): a pattern is a path of slashes, relative
// to the folder of the file, without ., .. or empty parts, and may use the
// wildcards of path.Match; a folder is embedded as all the files in it
// (except those whose name starts with . or _, unless the pattern starts
// with all:); a pattern must match something; a match must be a regular file
// (no symbolic link, device or socket), in the module (no folder with its own
// gocpp.mod), without special characters in its name. What is different:
//
//   - a variable of type embed.FS is an error until Phase 09 has io/fs
//     ("embed.FS is not available yet (Phase 09)"): only string and []byte
//     work, with patterns that name one file in all;
//   - #embed("f") takes one file name, not a pattern;
//   - a file is at most maxEmbedFile bytes and a package embeds at most
//     maxEmbedTotal, because the C++ compiler needs memory in proportion
//     (measured: 16 MiB takes 0.2 to 1.1 GB, a package at 64 MiB up to
//     1.8 GB), and says so with an error, not with an out-of-memory;
//   - a folder with one file does for a string or []byte variable, and a
//     folder walk skips what is not a regular file (as Go does).
//
// The loader reads everything through Config.FS (the folders it lists, the
// files it reads), so the up-to-date check of a build, which records those
// reads, sees a changed, added or removed file, and so does the object cache,
// whose key is the C++ text that holds the bytes.

var (
	// maxEmbedFile is the biggest file that can be embedded.
	maxEmbedFile int64 = 16 << 20

	// maxEmbedTotal is the most that a package can embed. (Both are variables
	// so that a test can lower them.)
	maxEmbedTotal int64 = 64 << 20
)

// placeholderLen is the length of the string that #embed gives in the language server, which
// reads no file: long enough that constant expressions on the length and the bytes of the
// content (a bound, an index, a divisor) don't fail.
const placeholderLen = 1 << 20

// An embedPattern is a pattern of a //go:embed line, with its place.
type embedPattern struct {
	text string
	pos  token.Pos
}

// An embedMatch is a file that a pattern matched.
type embedMatch struct {
	rel string // the path from the folder of the pattern, with slashes
	abs string // the name on disk
}

const embedPrefix = "//go:embed"

// The kinds of variable that //go:embed accepts.
const (
	embedBad = iota
	embedString
	embedBytes
	embedFS
)

// goEmbed finds the //go:embed lines of the file and gives each variable that
// they are on its value: var s string becomes var s string = ("...").
func (b *builtinPass) goEmbed() {
	f := b.f.AST
	type directive struct {
		c        *ast.Comment
		patterns []embedPattern
	}
	var dirs []directive
	for _, g := range f.Comments {
		for _, c := range g.List {
			rest, ok := strings.CutPrefix(c.Text, embedPrefix)
			if !ok || rest != "" && rest[0] != ' ' && rest[0] != '\t' {
				continue
			}
			pats, err := parseEmbedPatterns(rest, c.Slash+token.Pos(len(embedPrefix)))
			if err != "" {
				b.errorf(c.Slash, "%s", err)
				continue
			}
			if len(pats) == 0 {
				b.errorf(c.Slash, "usage: //go:embed pattern...")
				continue
			}
			dirs = append(dirs, directive{c, pats})
		}
	}
	if len(dirs) == 0 {
		return
	}

	// The places where a variable starts: the var of a declaration without
	// parentheses, each spec of a group.
	type target struct {
		spec *ast.ValueSpec
		pats []embedPattern
		pos  token.Pos
	}
	var targets []*target
	var others []token.Pos // where the other declarations start, and their end
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			others = append(others, d.Pos())
			continue
		}
		if gd.Lparen.IsValid() {
			// A directive above `var (` is misplaced: it goes on the specs.
			others = append(others, gd.TokPos)
		}
		for _, s := range gd.Specs {
			pos := s.Pos()
			if !gd.Lparen.IsValid() {
				pos = gd.TokPos
			}
			targets = append(targets, &target{spec: s.(*ast.ValueSpec), pos: pos})
		}
	}
	if b.embedName == "" {
		b.errorf(dirs[0].c.Slash, `go:embed requires import "embed" (or import _ "embed", if package is not used)`)
		return
	}
	for _, dir := range dirs {
		at := dir.c.Slash
		if d := declAround(f, at); d != nil {
			if fd, ok := d.(*ast.FuncDecl); ok && varAfterInFunc(fd, at) {
				b.errorf(at, "go:embed cannot apply to var inside func")
			} else {
				b.errorf(at, "misplaced go:embed directive")
			}
			continue
		}
		// The next place where something starts must be a variable.
		var next *target
		for _, t := range targets {
			if t.pos > at && (next == nil || t.pos < next.pos) {
				next = t
			}
		}
		nextOther := slices.IndexFunc(others, func(p token.Pos) bool { return p > at && (next == nil || p < next.pos) })
		if next == nil || nextOther >= 0 {
			b.errorf(at, "misplaced go:embed directive")
			continue
		}
		next.pats = append(next.pats, dir.patterns...)
	}
	for _, t := range targets {
		if len(t.pats) > 0 {
			b.embedVar(t.spec, t.pats)
		}
	}
}

// declAround returns the declaration of f that has pos inside it, other than
// a var declaration (a directive may be inside a group of variables), or nil.
func declAround(f *ast.File, pos token.Pos) ast.Decl {
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.VAR {
			continue
		}
		if d.Pos() < pos && pos < d.End() {
			return d
		}
	}
	return nil
}

// varAfterInFunc reports whether the first statement that starts after pos
// in the body of fd is a declaration of variables, as in a function with
// //go:embed on top of var s string.
func varAfterInFunc(fd *ast.FuncDecl, pos token.Pos) bool {
	found, isVar := token.NoPos, false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if s, ok := n.(ast.Stmt); ok && s.Pos() > pos && (found == token.NoPos || s.Pos() < found) {
			if ds, ok := s.(*ast.DeclStmt); ok {
				gd, ok := ds.Decl.(*ast.GenDecl)
				found, isVar = s.Pos(), ok && gd.Tok == token.VAR
			} else if _, ok := s.(*ast.BlockStmt); !ok {
				found, isVar = s.Pos(), false
			}
		}
		return true
	})
	return isVar
}

// parseEmbedPatterns splits the text after //go:embed into patterns, which
// are words, or Go strings in double quotes or back quotes, and returns an
// error message if a quote is not closed or is followed by more text. pos is
// the position of the text.
func parseEmbedPatterns(text string, pos token.Pos) ([]embedPattern, string) {
	var pats []embedPattern
	for i := 0; i < len(text); {
		switch c := text[i]; {
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '"' || c == '`':
			j := i + 1
			for j < len(text) && text[j] != c {
				if c == '"' && text[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(text) {
				return nil, "invalid quoted string in //go:embed: " + text[i:]
			}
			s, err := strconv.Unquote(text[i : j+1])
			if err != nil || j+1 < len(text) && text[j+1] != ' ' && text[j+1] != '\t' {
				return nil, "invalid quoted string in //go:embed: " + text[i:min(len(text), j+2)]
			}
			pats = append(pats, embedPattern{s, pos + token.Pos(i)})
			i = j + 1
		default:
			j := i
			for j < len(text) && text[j] != ' ' && text[j] != '\t' && text[j] != '\r' {
				j++
			}
			pats = append(pats, embedPattern{text[i:j], pos + token.Pos(i)})
			i = j
		}
	}
	return pats, ""
}

// embedVar gives the variable of spec its value from the files of the
// patterns, or reports why it can't. Go reports these at the variable's
// name, not at the directive.
func (b *builtinPass) embedVar(spec *ast.ValueSpec, pats []embedPattern) {
	at := spec.Names[0].Pos()
	switch {
	case len(spec.Names) > 1:
		b.errorf(at, "go:embed cannot apply to multiple vars")
		return
	case len(spec.Values) > 0:
		b.errorf(at, "go:embed cannot apply to var with initializer")
		return
	case spec.Type == nil:
		b.errorf(at, "go:embed cannot apply to var without type")
		return
	}
	kind := b.embedKind(spec.Type, 0)
	switch kind {
	case embedBad:
		b.errorf(at, "go:embed cannot apply to var of type %s", types.ExprString(spec.Type))
		return
	case embedFS:
		b.errorf(spec.Type.Pos(), "embed.FS is not available yet (Phase 09: it needs io/fs); //go:embed works on string and []byte variables")
		return
	}
	if b.l.cfg.NoEmbedFiles {
		for _, p := range pats {
			if pat, _ := strings.CutPrefix(p.text, "all:"); !validEmbedPattern(pat) {
				b.errorf(p.pos, "pattern %s: invalid pattern syntax", p.text)
				return
			}
		}
		b.setEmbedValue(spec, kind, "")
		return
	}
	var files []embedMatch
	failed := false
	dir := filepath.Dir(b.f.Path)
	for _, p := range pats {
		ms, _, err := b.resolve(dir, p.text)
		if err != nil {
			b.errorf(p.pos, "%v", err)
			failed = true
			continue
		}
		// A folder gives the files in it, so a folder with one file does for a string.
		for _, m := range ms {
			if !slices.Contains(files, m) {
				files = append(files, m)
			}
		}
	}
	if failed {
		return
	}
	if len(files) != 1 {
		b.errorf(at, "invalid go:embed: multiple files for type %s", types.ExprString(spec.Type))
		return
	}
	data, ok := b.readEmbed(files[0], pats[0].pos)
	if !ok {
		return
	}
	b.setEmbedValue(spec, kind, data)
}

// setEmbedValue gives the variable of spec the value data: a string constant,
// converted to []byte for a variable of that kind.
func (b *builtinPass) setEmbedValue(spec *ast.ValueSpec, kind int, data string) {
	// The value has no text of its own: it sits at the end of the type, so
	// that it never covers a name (a hover or a rename at the variable's
	// name must find the name).
	at := spec.Type.End()
	lit := &ast.BasicLit{ValuePos: at, Kind: token.STRING, Value: quoteBytes(data)}
	var value ast.Expr = &ast.ParenExpr{Lparen: at, X: lit, Rparen: at}
	if kind == embedBytes {
		// A conversion to the variable's own type (the type node is shared:
		// no name is made up that a hover could find).
		value = &ast.CallExpr{Fun: spec.Type, Lparen: at, Args: []ast.Expr{value}, Rparen: at}
	}
	spec.Values = []ast.Expr{value}
}

// embedKind returns the kind of variable that type t is for //go:embed:
// string, a slice of bytes, or embed.FS (or an alias of it), looking through
// the package's own type declarations, as Go does for named types.
func (b *builtinPass) embedKind(t ast.Expr, depth int) int {
	if depth > 20 {
		return embedBad
	}
	switch t := ast.Unparen(t).(type) {
	case *ast.Ident:
		switch t.Name {
		case "string":
			return embedString
		}
		if ts := b.typeSpec(t.Name); ts != nil {
			k := b.embedKind(ts.Type, depth+1)
			if k == embedFS && !ts.Assign.IsValid() {
				return embedBad // type T embed.FS is a struct that Go doesn't take
			}
			return k
		}
	case *ast.ArrayType:
		if id, ok := t.Elt.(*ast.Ident); ok && t.Len == nil && (id.Name == "byte" || id.Name == "uint8") {
			return embedBytes
		}
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok && b.embedName != "" && x.Name == b.embedName && t.Sel.Name == "FS" {
			return embedFS
		}
	}
	return embedBad
}

// typeSpec returns the package-level type declaration of name in the
// package, or nil.
func (b *builtinPass) typeSpec(name string) *ast.TypeSpec {
	for _, f := range b.p.Files {
		for _, d := range f.AST.Decls {
			if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
				for _, s := range gd.Specs {
					if ts := s.(*ast.TypeSpec); ts.Name.Name == name {
						return ts
					}
				}
			}
		}
	}
	return nil
}

// embedCall expands #embed("file") to the constant string with the bytes
// of the file. The file is named from the folder of the source file.
func (b *builtinPass) embedCall(c *astutil.Cursor, call *ast.CallExpr) {
	var lit *ast.BasicLit
	if len(call.Args) == 1 {
		lit, _ = call.Args[0].(*ast.BasicLit)
	}
	if lit == nil || lit.Kind != token.STRING {
		b.errorf(call.Pos(), `#embed takes one argument, a string literal: #embed("file")`)
		b.replace(c, call, token.STRING, `""`)
		return
	}
	name, err := strconv.Unquote(lit.Value)
	if err != nil {
		b.errorf(lit.Pos(), "invalid string in #embed: %v", err)
		b.replace(c, call, token.STRING, `""`)
		return
	}
	if b.l.cfg.NoEmbedFiles {
		if !validEmbedPattern(quoteMeta(name)) {
			b.errorf(lit.Pos(), "#embed %s: invalid file name", showName(name))
		}
		b.replace(c, call, token.STRING, strconv.Quote(strings.Repeat(" ", placeholderLen)))
		return
	}
	m, isDir, rerr := b.resolve(filepath.Dir(b.f.Path), quoteMeta(name))
	switch {
	case rerr != nil:
		msg := strings.Replace(rerr.Error(), "invalid pattern syntax", "invalid file name", 1)
		msg = strings.Replace(msg, "pattern "+showName(quoteMeta(name)), "#embed "+showName(name), 1)
		b.errorf(lit.Pos(), "%s", msg)
	case isDir:
		b.errorf(lit.Pos(), "#embed %s: a folder can't be embedded as a string", showName(name))
	default:
		if data, ok := b.readEmbed(m[0], lit.Pos()); ok {
			b.replace(c, call, token.STRING, quoteBytes(data))
			return
		}
	}
	b.replace(c, call, token.STRING, `""`)
}

// showName returns name for an error message: as it is, or quoted when it has a control
// character (a NUL in a name would be printed raw).
func showName(name string) string {
	for _, r := range name {
		if r < ' ' || r == 0x7f {
			return strconv.Quote(name)
		}
	}
	return name
}

// quoteMeta escapes the wildcards of path.Match in name.
func quoteMeta(name string) string {
	var sb strings.Builder
	for i := 0; i < len(name); i++ {
		if strings.IndexByte(`*?[\`, name[i]) >= 0 {
			sb.WriteByte('\\')
		}
		sb.WriteByte(name[i])
	}
	return sb.String()
}

// quoteBytes returns data as the text of a Go string literal. A text that
// has no special character keeps its look.
func quoteBytes(data string) string {
	return strconv.Quote(data)
}

// readEmbed reads the file of m, and reports an error at pos if it is
// too big, or the package would embed too much. The file is opened without
// following a link and its size is asked of the open file (ReadRegularFile):
// a file swapped after the listing can't make the read grow without end.
func (b *builtinPass) readEmbed(m embedMatch, pos token.Pos) (string, bool) {
	var (
		data []byte
		err  error
	)
	if src, ok := b.l.cfg.Overlay[m.abs]; ok {
		data = src
		if int64(len(data)) > maxEmbedFile {
			err = &TooBigError{int64(len(data))}
		}
	} else if lr, ok := b.l.cfg.FS.(LimitedReader); ok {
		data, err = lr.ReadFileLimit(m.abs, maxEmbedFile)
	} else if fi, serr := b.l.cfg.FS.Stat(m.abs); serr != nil {
		err = serr
	} else if fi.Size() > maxEmbedFile {
		err = &TooBigError{fi.Size()}
	} else {
		data, err = b.l.cfg.FS.ReadFile(m.abs)
		if err == nil && int64(len(data)) > maxEmbedFile {
			err = &TooBigError{int64(len(data))}
		}
	}
	var big *TooBigError
	switch {
	case errors.As(err, &big):
		b.errorf(pos, "cannot embed %s: %d bytes is more than the limit of %d MiB for one file", m.rel, big.Size, maxEmbedFile>>20)
		return "", false
	case err != nil:
		b.errorf(pos, "cannot embed %s: %v", m.rel, err)
		return "", false
	}
	if b.embedTotal += int64(len(data)); b.embedTotal > maxEmbedTotal {
		b.errorf(pos, "cannot embed %s: the package would embed more than %d MiB", m.rel, maxEmbedTotal>>20)
		return "", false
	}
	return string(data), true
}

// resolve returns the files that the pattern matches in the folder dir, and
// whether the pattern names folders (then the files are those in them, not
// embedded as one). The error says why the pattern is not good, in the words
// of go build.
func (b *builtinPass) resolve(dir, pattern string) (files []embedMatch, isDir bool, err error) {
	pat, all := strings.CutPrefix(pattern, "all:")
	if !validEmbedPattern(pat) {
		return nil, false, fmt.Errorf("pattern %s: invalid pattern syntax", showName(pattern))
	}
	r := &embedResolver{b: b, pattern: showName(pattern), all: all}
	if err := r.glob(dir, "", strings.Split(pat, "/")); err != nil {
		return nil, false, err
	}
	if len(r.files) == 0 {
		return nil, false, fmt.Errorf("pattern %s: no matching files found", showName(pattern))
	}
	return r.files, r.dirs, nil
}

// validEmbedPattern reports whether pat is a path of slashes without ., .. or
// empty parts and with wildcards that path.Match accepts.
func validEmbedPattern(pat string) bool {
	if !fs.ValidPath(pat) || pat == "." {
		return false
	}
	_, err := path.Match(pat, "")
	return err == nil
}

// An embedResolver finds the files of one pattern.
type embedResolver struct {
	b       *builtinPass
	pattern string
	all     bool
	files   []embedMatch
	dirs    bool // a match was a folder
}

// glob matches the parts comps of the pattern in the folder abs (rel from the
// folder of the pattern).
func (r *embedResolver) glob(abs, rel string, comps []string) error {
	entries, err := r.b.listDir(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("pattern %s: %v", r.pattern, err)
	}
	last := len(comps) == 1
	for _, e := range entries {
		if ok, _ := path.Match(comps[0], e.Name()); !ok {
			continue
		}
		erel, eabs := path.Join(rel, e.Name()), filepath.Join(abs, e.Name())
		switch typ := e.Type(); {
		case typ&fs.ModeSymlink != 0:
			return fmt.Errorf("pattern %s: cannot embed irregular file %s", r.pattern, erel)
		case typ.IsDir():
			if r.b.isModuleDir(eabs) {
				return fmt.Errorf("pattern %s: cannot embed directory %s: in different module", r.pattern, erel)
			}
			if !last {
				if err := r.glob(eabs, erel, comps[1:]); err != nil {
					return err
				}
				continue
			}
			r.dirs = true
			n := len(r.files)
			if err := r.walk(eabs, erel); err != nil {
				return err
			}
			if len(r.files) == n {
				return fmt.Errorf("pattern %s: cannot embed directory %s: contains no embeddable files", r.pattern, erel)
			}
		case typ.IsRegular():
			if !last {
				continue
			}
			if err := r.add(erel, eabs); err != nil {
				return err
			}
		default:
			return fmt.Errorf("pattern %s: cannot embed irregular file %s", r.pattern, erel)
		}
	}
	return nil
}

// walk adds the files in the folder abs and the folders below it, except
// those whose name starts with . or _ (unless the pattern said all:) and
// the folders that are other modules.
func (r *embedResolver) walk(abs, rel string) error {
	entries, err := r.b.listDir(abs)
	if err != nil {
		return fmt.Errorf("pattern %s: %v", r.pattern, err)
	}
	for _, e := range entries {
		name := e.Name()
		if !r.all && (name[0] == '.' || name[0] == '_') {
			continue
		}
		erel, eabs := path.Join(rel, name), filepath.Join(abs, name)
		switch typ := e.Type(); {
		case typ&fs.ModeSymlink != 0:
			continue // a folder walk skips what is not a regular file or a folder, as Go does
		case typ.IsDir():
			if r.b.isModuleDir(eabs) {
				continue
			}
			if err := r.walk(eabs, erel); err != nil {
				return err
			}
		case typ.IsRegular():
			if err := r.add(erel, eabs); err != nil {
				return err
			}
		default:
			continue
		}
	}
	return nil
}

// add adds a file, if its name is one that Go accepts.
func (r *embedResolver) add(rel, abs string) error {
	if i := strings.IndexAny(path.Base(rel), "\"*<>?`'|/\\:"); i >= 0 {
		return fmt.Errorf("pattern %s: cannot embed file %s: invalid name %s", r.pattern, rel, path.Base(rel))
	}
	m := embedMatch{rel, abs}
	if !slices.Contains(r.files, m) {
		r.files = append(r.files, m)
	}
	return nil
}

// listDir lists the folder abs; for the build's recorder, in the way that puts every file of the
// folder in the stamp (load.FullLister).
func (b *builtinPass) listDir(abs string) ([]fs.DirEntry, error) {
	if fl, ok := b.l.cfg.FS.(FullLister); ok {
		return fl.ReadDirAll(abs)
	}
	return b.l.cfg.FS.ReadDir(abs)
}

// isModuleDir reports whether the folder abs has its own gocpp.mod file: it
// is another module, and its files are not embedded.
func (b *builtinPass) isModuleDir(abs string) bool {
	_, err := b.l.cfg.FS.Stat(filepath.Join(abs, "gocpp.mod"))
	return err == nil
}
