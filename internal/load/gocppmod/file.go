package gocppmod

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// A File is a gocpp.mod file, read statement by statement.
type File struct {
	Syntax *FileSyntax

	Module  *Module    // the module line, or nil
	Gocpp   *Gocpp     // the gocpp line, or nil
	Cxx     *Cxx       // the cxx line (a C++ standard), or nil
	Require []*Require // the require lines, in file order
	Replace []*Replace // the replace lines, in file order

	// Cflags, Ldflags and PkgConfig are the module-wide C and C++
	// settings, one entry for each line (Args).
	Cflags, Ldflags, PkgConfig []*Args

	// Unknown holds the statements that gocpp doesn't read: a verb that it
	// doesn't know, a block with more than the verb before its "(", and
	// every brace block (the recipe of a C++ library: cxx PATH { ... }).
	// They are in Syntax too, and Format writes them back unchanged.
	Unknown []Stmt

	idx struct {
		once      sync.Once
		require   map[string]*Require // by module path, the first line
		replace   map[string]*Replace // by path and version, see replaceKey
		replaceBy map[string]*Replace // by module path, the first line
	}
}

func replaceKey(path, version string) string { return path + "\x00" + version }

func (f *File) index() {
	f.idx.once.Do(func() {
		f.idx.require = make(map[string]*Require, len(f.Require))
		for _, r := range f.Require {
			if f.idx.require[r.Path] == nil {
				f.idx.require[r.Path] = r
			}
		}
		f.idx.replace = make(map[string]*Replace, len(f.Replace))
		f.idx.replaceBy = make(map[string]*Replace, len(f.Replace))
		for _, r := range f.Replace {
			f.idx.replace[replaceKey(r.OldPath, r.OldVersion)] = r
			if f.idx.replaceBy[r.OldPath] == nil {
				f.idx.replaceBy[r.OldPath] = r
			}
		}
	})
}

// FindRequire returns the require line of the module with the longest path
// that importPath is in (the path itself or a folder of it), or nil.
func (f *File) FindRequire(importPath string) *Require {
	f.index()
	for p := importPath; p != ""; {
		if r := f.idx.require[p]; r != nil {
			return r
		}
		i := strings.LastIndexByte(p, '/')
		if i < 0 {
			break
		}
		p = p[:i]
	}
	return nil
}

// FindReplace returns the replace line for the module path at version: the
// line for that version, else the line for every version, or nil.
func (f *File) FindReplace(path, version string) *Replace {
	f.index()
	if version != "" {
		if r := f.idx.replace[replaceKey(path, version)]; r != nil {
			return r
		}
	}
	return f.idx.replace[replaceKey(path, "")]
}

// FindReplacedPath returns the first replace line for the module with the
// longest path that importPath is in, for any version, or nil.
func (f *File) FindReplacedPath(importPath string) *Replace {
	f.index()
	for p := importPath; p != ""; {
		if r := f.idx.replaceBy[p]; r != nil {
			return r
		}
		i := strings.LastIndexByte(p, '/')
		if i < 0 {
			break
		}
		p = p[:i]
	}
	return nil
}

// A Module is the module line: module example.com/app.
type Module struct {
	Path   string // the path, unquoted
	Syntax *Line
}

// A Gocpp is the gocpp line, the minimum gocpp version: gocpp 0.1.
type Gocpp struct {
	Version string // "0.1" or "0.1.2"
	Syntax  *Line
}

// A Cxx is the cxx line, the C++ standard of the module: cxx c++20.
type Cxx struct {
	Std    string
	Syntax *Line
}

// A Require is a require line: require example.com/lib v1.2.3.
type Require struct {
	Path, Version string
	Indirect      bool // the line ends with // indirect
	Syntax        *Line
}

// A Replace is a replace line: replace old [version] => new [version].
// With no NewVersion, NewPath is a local folder, absolute or starting
// with ./ or ../ (Local).
type Replace struct {
	OldPath, OldVersion string
	NewPath, NewVersion string
	Syntax              *Line
}

// Local reports whether the replacement is a local folder.
func (r *Replace) Local() bool { return r.NewVersion == "" }

// Args are the arguments of a cflags, ldflags or pkg-config line, with the
// quotes and backslashes removed as in a //gocpp: directive:
//
//	cflags -DGAME_DEBUG "-DNAME=a b"
type Args struct {
	Args   []string
	Syntax *Line
}

// A PathCheck tells whether a module path is valid. If it is not, the
// message of the error has the reason, which may be empty.
type PathCheck func(path string) (ok bool, reason string)

// Parse reads a gocpp.mod file. name is the file name in the errors. check
// validates the module paths of the module, require and replace lines; if
// it is nil, CheckPath does (with its reason). The result has all the statements that could
// be read, also when the errors are not empty.
func Parse(name string, data []byte, check PathCheck) (*File, ErrorList) {
	syn, errs := ParseSyntax(name, data)
	if check == nil {
		check = func(p string) (bool, string) {
			msg := CheckPath(p)
			return msg == "", msg
		}
	}
	r := &reader{f: &File{Syntax: syn}, name: name, errs: errs, check: check,
		requires: make(map[string]*Require), replaces: make(map[string]*Replace)}
	for _, s := range syn.Stmt {
		switch s := s.(type) {
		case *Line:
			r.stmt(s.Token[0], s.Token[1:], s, s)
		case *Block:
			r.block(s)
		}
	}
	r.finish()
	return r.f, r.errs
}

type reader struct {
	f     *File
	name  string
	errs  ErrorList
	check PathCheck

	requires map[string]*Require // by module path, the first line
	replaces map[string]*Replace // by replaceKey
}

func (r *reader) errorf(at Position, format string, args ...any) {
	r.errs = append(r.errs, Error{Filename: r.name, Pos: at, Err: fmt.Sprintf(format, args...)})
}

// block reads a block: each of its lines is a statement of the verb of the
// block. A block that is not a plain "verb (" of a known verb is kept as it
// is, if its verb could be one of a later gocpp (a block of a keyword of
// gocpp.mod in a wrong shape is an error).
func (r *reader) block(b *Block) {
	if b.Unterminated {
		return // the parser reported it; its lines are not statements
	}
	verb := b.Token[0]
	switch {
	case knownVerbs[verb] && !(verb == "cxx" && b.Open == '{') && (b.Open != '(' || len(b.Token) != 1):
		r.errorf(b.Start, "unknown block type: %s %c", strings.Join(b.Token, " "), b.Open)
	case !knownVerbs[verb]:
		if !validVerb(verb) {
			r.errorf(b.Start, "unknown directive: %s", verb)
			return
		}
		r.f.Unknown = append(r.f.Unknown, b)
	case b.Open == '{':
		r.f.Unknown = append(r.f.Unknown, b) // the recipe of a C++ library
	default:
		for _, l := range b.Line {
			r.stmt(verb, l.Token, l, l)
		}
	}
}

// recipeHint is added to the errors of a cxx line that looks like a recipe
// block that does not open as one.
const recipeHint = " (a C or C++ library recipe is a block that opens with { at the end of its own line, and has its statements on the lines below: cxx module/path {)"

// knownVerbs are the verbs that gocpp reads.
var knownVerbs = map[string]bool{
	"module": true, "gocpp": true, "cxx": true, "require": true, "replace": true,
	"cflags": true, "ldflags": true, "pkg-config": true,
}

var verbRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// validVerb reports whether v can be the verb of a statement that gocpp doesn't
// read: lower-case letters, digits and "-", like the verbs of gocpp.mod and
// go.mod. A module path at the start of a line, a quote or a version is a
// mistake (a line outside its block), not a verb of a later gocpp.
func validVerb(v string) bool { return verbRE.MatchString(v) }

// stmt reads one statement of the verb with the arguments args, written in
// line. whole is what goes into File.Unknown if the verb is not known.
func (r *reader) stmt(verb string, args []string, line *Line, whole Stmt) {
	at := line.Start
	switch verb {
	case "module":
		if r.f.Module != nil {
			r.errorf(at, "repeated module statement")
			return
		}
		if len(args) != 1 {
			r.errorf(at, "usage: module module/path")
			return
		}
		p, err := unquote(args[0])
		if err != "" {
			r.errorf(at, "%s", err)
			return
		}
		if ok, why := r.check(p); !ok {
			r.errorf(at, "invalid module path %q%s", p, reason(why))
			return
		}
		r.f.Module = &Module{Path: p, Syntax: line}

	case "gocpp":
		if r.f.Gocpp != nil {
			r.errorf(at, "repeated gocpp statement")
			return
		}
		if len(args) != 1 {
			r.errorf(at, "usage: gocpp 0.1")
			return
		}
		v, err := unquote(args[0])
		if err != "" {
			r.errorf(at, "%s", err)
			return
		}
		if !versionRE.MatchString(v) {
			r.errorf(at, "invalid gocpp version %q: must match format 0.1", v)
			return
		}
		r.f.Gocpp = &Gocpp{Version: v, Syntax: line}

	case "cxx":
		if len(args) != 1 {
			if !line.InBlock {
				hint := ""
				if strings.ContainsAny(strings.Join(args, " "), "/{}") {
					hint = recipeHint // a module path or a brace: the user meant a recipe
				}
				r.errorf(at, "usage: cxx c++20%s", hint)
			}
			return
		}
		if r.f.Cxx != nil {
			r.errorf(at, "repeated cxx statement")
			return
		}
		std, err := unquote(args[0])
		if err != "" {
			r.errorf(at, "%s", err)
			return
		}
		if !stdRE.MatchString(std) {
			hint := ""
			if strings.ContainsAny(std, "/{}") {
				hint = recipeHint
			}
			r.errorf(at, "invalid C++ standard %q: want c++17, c++20, gnu++20, ...%s", std, hint)
			return
		}
		r.f.Cxx = &Cxx{Std: std, Syntax: line}

	case "require":
		if len(args) != 2 {
			r.errorf(at, "usage: require module/path v1.2.3")
			return
		}
		path, err := unquote(args[0])
		if err != "" {
			r.errorf(at, "%s", err)
			return
		}
		ver, err := unquote(args[1])
		if err != "" {
			r.errorf(at, "%s", err)
			return
		}
		if ok, why := r.check(path); !ok {
			r.errorf(at, "invalid module path %q%s", path, reason(why))
			return
		}
		if !ValidVersion(ver) {
			r.errorf(at, "require %s: invalid module version %q: must be of the form v1.2.3", path, ver)
			return
		}
		ver = CanonicalVersion(ver)
		if old := r.requires[path]; old != nil && old.Version != ver {
			r.errorf(at, "require %s: version %s, but %s is required at line %d: gocpp has no version selection yet, keep one",
				path, ver, old.Version, old.Syntax.Start.Line)
			return
		}
		req := &Require{Path: path, Version: ver, Indirect: isIndirect(line), Syntax: line}
		if r.requires[path] == nil {
			r.requires[path] = req
		}
		r.f.Require = append(r.f.Require, req)

	case "replace":
		r.replace(args, line)

	case "cflags", "ldflags", "pkg-config":
		if len(args) == 0 {
			r.errorf(at, "usage: %s %s...", verb, map[string]string{"cflags": "flag", "ldflags": "flag", "pkg-config": "package"}[verb])
			return
		}
		a := &Args{Syntax: line}
		for _, w := range args {
			s, err := SplitWord(w)
			if err != "" {
				r.errorf(at, "%s in %s", err, verb)
				return
			}
			if verb == "pkg-config" {
				if msg := checkPkgConfigName(s); msg != "" {
					r.errorf(at, "%s", msg)
					return
				}
			}
			a.Args = append(a.Args, s)
		}
		switch verb {
		case "cflags":
			r.f.Cflags = append(r.f.Cflags, a)
		case "ldflags":
			r.f.Ldflags = append(r.f.Ldflags, a)
		default:
			r.f.PkgConfig = append(r.f.PkgConfig, a)
		}

	default:
		// Not a verb of gocpp: kept. A line of a block is kept as the
		// block, once.
		if !validVerb(verb) {
			r.errorf(at, "unknown directive: %s", verb)
		} else if !line.InBlock {
			r.f.Unknown = append(r.f.Unknown, whole)
		}
	}
}

func reason(why string) string {
	if why == "" {
		return ""
	}
	return ": " + why
}

// HasModuleLine reports whether the file has a module statement, also a
// broken one.
func (f *File) HasModuleLine() bool {
	for _, s := range f.Syntax.Stmt {
		switch s := s.(type) {
		case *Line:
			if s.Token[0] == "module" {
				return true
			}
		case *Block:
			if s.Token[0] == "module" {
				return true
			}
		}
	}
	return false
}

func isIndirect(l *Line) bool {
	for _, c := range l.Suffix {
		if strings.TrimSpace(strings.TrimPrefix(c.Token, "//")) == "indirect" ||
			strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(c.Token, "//")), "indirect;") {
			return true
		}
	}
	return false
}

func (r *reader) replace(args []string, line *Line) {
	at := line.Start
	const usage = "usage: replace module/path [v1.2.3] => other/module v1.4\n\t or replace module/path [v1.2.3] => ../local/directory"
	arrow := -1
	for i, a := range args {
		if a == "=>" {
			arrow = i
			break
		}
	}
	if arrow < 0 || arrow == 0 || arrow > 2 || len(args)-arrow-1 < 1 || len(args)-arrow-1 > 2 {
		r.errorf(at, "%s", usage)
		return
	}
	lhs, rhs := args[:arrow], args[arrow+1:]
	rep := &Replace{Syntax: line}
	var err string
	if rep.OldPath, err = unquote(lhs[0]); err != "" {
		r.errorf(at, "%s", err)
		return
	}
	if len(lhs) == 2 {
		if rep.OldVersion, err = unquote(lhs[1]); err != "" {
			r.errorf(at, "%s", err)
			return
		}
	}
	if rep.NewPath, err = unquote(rhs[0]); err != "" {
		r.errorf(at, "%s", err)
		return
	}
	if len(rhs) == 2 {
		if rep.NewVersion, err = unquote(rhs[1]); err != "" {
			r.errorf(at, "%s", err)
			return
		}
	}
	if ok, why := r.check(rep.OldPath); !ok {
		r.errorf(at, "invalid module path %q%s", rep.OldPath, reason(why))
		return
	}
	if rep.OldVersion != "" && !ValidVersion(rep.OldVersion) {
		r.errorf(at, "replace %s: invalid module version %q: must be of the form v1.2.3", rep.OldPath, rep.OldVersion)
		return
	}
	switch {
	case rep.NewVersion == "" && !IsDirectoryPath(rep.NewPath):
		r.errorf(at, "replacement module without version must be directory path (rooted or starting with . or ..)")
		return
	case rep.NewVersion != "" && IsDirectoryPath(rep.NewPath):
		r.errorf(at, "replacement module directory path %q cannot have version", rep.NewPath)
		return
	case rep.NewVersion != "":
		if ok, why := r.check(rep.NewPath); !ok {
			r.errorf(at, "invalid module path %q%s", rep.NewPath, reason(why))
			return
		}
		if !ValidVersion(rep.NewVersion) {
			r.errorf(at, "replace %s: invalid module version %q: must be of the form v1.2.3", rep.OldPath, rep.NewVersion)
			return
		}
	}
	rep.OldVersion, rep.NewVersion = CanonicalVersion(rep.OldVersion), CanonicalVersion(rep.NewVersion)
	key := replaceKey(rep.OldPath, rep.OldVersion)
	if old := r.replaces[key]; old != nil {
		what := rep.OldPath
		if rep.OldVersion != "" {
			what += "@" + rep.OldVersion
		}
		r.errorf(at, "multiple replacements for %s (the first is at line %d)", what, old.Syntax.Start.Line)
		return
	}
	r.replaces[key] = rep
	r.f.Replace = append(r.f.Replace, rep)
}

// finish puts the errors in the order of the file: the parser's come first.
func (r *reader) finish() {
	slices.SortStableFunc(r.errs, func(a, b Error) int { return cmp.Compare(a.Pos.Byte, b.Pos.Byte) })
}

var (
	versionRE = regexp.MustCompile(`^[0-9]+\.[0-9]+(\.[0-9]+)?$`)
	stdRE     = regexp.MustCompile(`^(c|gnu)\+\+([0-9]{2}|[0-9][a-z])$`)
	semverRE  = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)` +
		`(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)
)

// ValidVersion reports whether v is a module version: v1.2.3, with an
// optional -prerelease (also a pseudo-version, v0.0.0-20240101000000-abcdef123456)
// and +metadata, as in semantic versioning: the dot-separated parts of a
// prerelease are not empty, and a number in them has no leading zero. Go
// also accepts v1 and v1.2; gocpp wants the full form.
func ValidVersion(v string) bool {
	m := semverRE.FindStringSubmatch(v)
	if m == nil {
		return false
	}
	if pre := m[4]; pre != "" {
		for id := range strings.SplitSeq(pre[1:], ".") {
			if len(id) > 1 && id[0] == '0' && strings.Trim(id, "0123456789") == "" {
				return false
			}
		}
	}
	return true
}

// CanonicalVersion returns the version v of a valid module version without its
// build metadata, except +incompatible, as the go command writes it (a
// vendor/modules.txt written by Go has the canonical form). A version that is
// not valid is returned as it is.
func CanonicalVersion(v string) string {
	if v == "" || !ValidVersion(v) {
		return v
	}
	if i := strings.IndexByte(v, '+'); i >= 0 && v[i:] != "+incompatible" {
		return v[:i]
	}
	return v
}

// IsDirectoryPath reports whether the replacement path names a folder:
// absolute, or ".", "..", or starting with ./ or ../. The check knows the
// Unix and the Windows spellings, because a gocpp.mod file moves between
// systems.
func IsDirectoryPath(p string) bool {
	return p == "." || strings.HasPrefix(p, "./") || strings.HasPrefix(p, `.\`) ||
		p == ".." || strings.HasPrefix(p, "../") || strings.HasPrefix(p, `..\`) ||
		strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\`) ||
		len(p) >= 2 && (p[0] >= 'A' && p[0] <= 'Z' || p[0] >= 'a' && p[0] <= 'z') && p[1] == ':'
}

// CheckPath returns why path can't be a module path, or "". Unlike
// golang.org/x/mod/module.CheckPath, it accepts a path without a dot in its
// first element (a local module can be called myapp), as ADR-020 says. A
// path is slash-separated elements that are not empty, not "." or "..", with
// graphic characters and none of !"#$%&'()*,:;<=>?[\]^{|}` .
func CheckPath(path string) string {
	switch {
	case path == "":
		return "empty string"
	case strings.HasPrefix(path, "/"):
		return "leading slash"
	case strings.HasSuffix(path, "/"):
		return "trailing slash"
	case strings.Contains(path, "//"):
		return "double slash"
	}
	const illegal = "!\"#$%&'()*,:;<=>?[\\]^{|}`�"
	for _, r := range path {
		if !unicode.IsGraphic(r) || unicode.IsSpace(r) || strings.ContainsRune(illegal, r) {
			return fmt.Sprintf("invalid char %q", r)
		}
	}
	for elem := range strings.SplitSeq(path, "/") {
		if elem == "." || elem == ".." {
			return fmt.Sprintf("invalid path element %q", elem)
		}
	}
	return ""
}

// checkPkgConfigName is the check of the //gocpp:pkg-config directive: a
// package name has ASCII letters, digits and "_+-." only, and is not a flag.
func checkPkgConfigName(arg string) string {
	if strings.HasPrefix(arg, "-") {
		return fmt.Sprintf("invalid pkg-config package name %q: flags go in cflags or ldflags", arg)
	}
	for _, r := range arg {
		switch {
		case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9':
		case r == '_', r == '+', r == '-', r == '.':
		default:
			return fmt.Sprintf("invalid character %q in pkg-config package name %q", r, arg)
		}
	}
	return ""
}

// unquote returns the word w, without its quotes if it is one quoted
// string ("..." or `...`), or the message for a quoted string that is not
// valid. A word that has quotes inside is left as it is.
func unquote(w string) (string, string) {
	if w == "" || w[0] != '"' && w[0] != '`' {
		return w, ""
	}
	s, err := strconv.Unquote(w)
	if err != nil {
		return "", "invalid quoted string: " + w
	}
	return s, ""
}

// SplitWord returns the argument that the word w stands for in a cflags,
// ldflags or pkg-config line, by the rules of a //gocpp: directive (and a
// backquote, as in the lexer: it keeps everything up to the next one, with no
// escapes): a single or double quote starts a quoted part (also in the middle of the word) that
// keeps white space and runs to the next quote of the same kind, a backslash
// makes the next character an ordinary one, and the quotes and backslashes
// are removed: -DMSG="a b" gives -DMSG=a b. It returns the message for an
// unterminated quote, a backslash at the end and an empty argument.
func SplitWord(w string) (string, string) {
	var (
		arg    []byte
		quote  rune
		escape bool
	)
	for i := 0; i < len(w); {
		r, size := utf8.DecodeRuneInString(w[i:])
		i += size
		switch {
		case escape:
			escape = false
		case r == '\\' && quote != '`':
			escape = true
			continue
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
		case r == '"' || r == '\'' || r == '`':
			quote = r
			continue
		}
		arg = utf8.AppendRune(arg, r)
	}
	switch {
	case quote != 0:
		return "", "unterminated quote"
	case escape:
		return "", "backslash at the end of the line"
	case len(arg) == 0:
		return "", "empty argument"
	}
	return string(arg), ""
}
