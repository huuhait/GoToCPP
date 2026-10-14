package load

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/diag"
)

// A Directive is one //gocpp: line of a .gp file, such as
//
//	//gocpp:link m pthread
//
// The build uses the directives of all files of a package: pkg-config,
// link, cflags and ldflags in Phases 04 and 06, extern-c and no-extern-c in
// Phase 04, and no-win32-defaults in Phase 14. docs/spec/interop.md
// ("Building C and C++ sources") says what each one does.
type Directive struct {
	// Pos is the position of the "//" that starts the directive: the place
	// for messages about it.
	Pos token.Pos

	// Name is the name after "//gocpp:": "pkg-config", "link", "cflags",
	// "ldflags", "extern-c", "no-extern-c" or "no-win32-defaults".
	Name string

	// Args are the arguments in the order they were written, with their
	// quotes and backslashes removed as in a #cgo line: ["m", "pthread"].
	// Args is nil for no-win32-defaults, which takes none. Each line is
	// its own Directive; the build adds up the arguments of a repeated
	// directive.
	Args []string
}

// directivePrefix starts every directive. Like "//go:build", it has no
// space after the "//".
const directivePrefix = "//gocpp:"

// A directiveSpec says what the arguments of a directive must look like.
type directiveSpec struct {
	name    string
	usage   string // the arguments in the "usage:" error, such as "library..."
	minArgs int
	maxArgs int // -1 means no limit

	// check returns the error message for a bad argument, or "". It is
	// nil when every argument is fine, as for cflags: the flags are passed
	// on as they are written.
	check func(arg string) string
}

// directiveSpecs are all the directives, in the order of the table in
// docs/spec/interop.md. When two names are equally close to a misspelled
// one, suggestDirective gives them in this order.
var directiveSpecs = []directiveSpec{
	{name: "pkg-config", usage: "package...", minArgs: 1, maxArgs: -1, check: checkPkgConfigArg},
	{name: "link", usage: "library...", minArgs: 1, maxArgs: -1, check: checkLinkArg},
	{name: "cflags", usage: "flag...", minArgs: 1, maxArgs: -1},
	{name: "ldflags", usage: "flag...", minArgs: 1, maxArgs: -1},
	{name: "extern-c", usage: "header", minArgs: 1, maxArgs: 1, check: checkHeaderArg},
	{name: "no-extern-c", usage: "header", minArgs: 1, maxArgs: 1, check: checkHeaderArg},
	{name: "no-win32-defaults", minArgs: 0, maxArgs: 0},
}

// ParseDirectives returns the //gocpp: directives of f in source order. It
// adds an error to errs for every mistake, and leaves a directive with a
// mistake out of the result. It also adds a warning for a comment that
// looks like a broken directive. f must be parsed with
// parser.ParseComments: without its comments, f has no directives.
//
// The rules (docs/spec/interop.md has them for users):
//   - A directive is a line comment that starts with exactly "//gocpp:",
//     then a name, then the arguments. Every line comment that starts with
//     "//gocpp:" counts, so a misspelled name is an error, not an ordinary
//     comment. "// gocpp:link m" (with a space) is an ordinary comment, and
//     so is every /* */ comment. But before the package clause, such a
//     comment gets a warning when it looks like a broken directive (see
//     lookalike).
//   - Directives come before the package clause: in the package doc
//     comment or in a comment of their own. A directive after the package
//     clause is an error.
//   - The arguments are split as in a #cgo line (go/build's splitQuoted):
//     white space separates them, quotes keep white space and are removed,
//     also in the middle of an argument, and a backslash escapes the next
//     character (see split). A missing closing quote, a backslash at the
//     end of the line and an empty argument ("") are errors.
//   - An argument that starts with "//" (not quoted, not escaped) starts a
//     comment, up to the end of the line: "//gocpp:link m // libm" links
//     m. So a test can put a // ERROR comment at the end of a directive.
//   - An unknown name is an error, with a "did you mean" hint when a known
//     name is close (suggestDirective). A wrong number of arguments is an
//     error, and so is a bad pkg-config package name, library name or
//     header (directiveSpecs).
func ParseDirectives(fset *token.FileSet, f *ast.File, errs *diag.List) []Directive {
	if !f.Package.IsValid() {
		return nil // no package clause: the parser has reported that
	}
	p := directiveParser{fset: fset, errs: errs}
	var dirs []Directive
	for _, g := range f.Comments {
		for _, c := range g.List {
			if !strings.HasPrefix(c.Text, directivePrefix) {
				if c.Slash < f.Package {
					p.lookalike(c)
				}
				continue
			}
			if c.Slash > f.Package {
				errs.Errorf(fset.Position(c.Slash), "//gocpp: directives must come before the package clause")
				continue
			}
			if d, ok := p.parse(c); ok {
				dirs = append(dirs, d)
			}
		}
	}
	return dirs
}

// A directiveParser reads the directives of one file.
type directiveParser struct {
	fset *token.FileSet
	errs *diag.List
	c    *ast.Comment // the directive being read
	bad  bool         // an error was reported for c
}

// A directiveArg is an argument of a directive, without its quotes and
// backslashes.
type directiveArg struct {
	text string
	off  int // the byte offset in the comment text where it starts (maybe at a quote)
}

// errorf reports an error at byte offset off of the comment text.
func (p *directiveParser) errorf(off int, format string, args ...any) {
	p.errs.Errorf(p.fset.Position(p.c.Slash+token.Pos(off)), format, args...)
	p.bad = true
}

// parse reads one directive. It reports false if the directive has a
// mistake.
func (p *directiveParser) parse(c *ast.Comment) (Directive, bool) {
	p.c, p.bad = c, false
	text := c.Text
	// The name ends at the same white space that separates the arguments.
	nameEnd := len(text)
	if i := strings.IndexFunc(text[len(directivePrefix):], unicode.IsSpace); i >= 0 {
		nameEnd = len(directivePrefix) + i
	}
	name := text[len(directivePrefix):nameEnd]
	var spec *directiveSpec
	for i := range directiveSpecs {
		if directiveSpecs[i].name == name {
			spec = &directiveSpecs[i]
			break
		}
	}
	if spec == nil {
		p.unknown(name, text[nameEnd:])
		return Directive{}, false
	}

	args := p.split(text, nameEnd, name)
	if p.bad {
		return Directive{}, false
	}
	if len(args) < spec.minArgs || spec.maxArgs >= 0 && len(args) > spec.maxArgs {
		usage := "//gocpp:" + name
		if spec.usage != "" {
			usage += " " + spec.usage
		}
		p.errorf(0, "usage: %s", usage)
		return Directive{}, false
	}
	d := Directive{Pos: c.Slash, Name: name}
	for _, a := range args {
		if spec.check != nil {
			if msg := spec.check(a.text); msg != "" {
				p.errorf(a.off, "%s", msg)
				continue
			}
		}
		d.Args = append(d.Args, a.text)
	}
	if p.bad {
		return Directive{}, false
	}
	return d, true
}

// unknown reports a directive whose name is not known. rest is the text
// after the name.
func (p *directiveParser) unknown(name, rest string) {
	var msg string
	if name == "" {
		// "//gocpp:" alone, or a space after the colon ("//gocpp: link m"),
		// where the first word may be the name.
		msg = "missing directive name after //gocpp:"
		if words := strings.Fields(rest); len(words) > 0 {
			name = words[0]
		}
	} else {
		msg = fmt.Sprintf("unknown //gocpp: directive %q", name)
	}
	if names := suggestDirective(name); len(names) > 0 {
		for i, n := range names {
			names[i] = "//gocpp:" + n
		}
		msg += "; did you mean " + strings.Join(names, " or ") + "?"
	}
	p.errorf(0, "%s", msg)
}

// lookalike warns about an ordinary line comment before the package clause
// that looks like a broken directive: "gocpp:" in any letter case after
// the "//" and any white space, then a name that is a directive name or
// close to one (suggestDirective). Such a comment is often made by gofmt,
// which keeps only well-formed directives in a doc comment as they are:
// "//gocpp:Link m" becomes "// gocpp:Link m", an ordinary comment, and the
// library would then silently not be linked. (gocpp fmt leaves a file with
// directive errors unchanged, step 1.16.)
func (p *directiveParser) lookalike(c *ast.Comment) {
	const prefix = "gocpp:"
	text, ok := strings.CutPrefix(c.Text, "//")
	if !ok {
		return // a /* */ comment
	}
	rest := strings.TrimLeftFunc(text, unicode.IsSpace)
	spaced := len(rest) < len(text)
	if len(rest) < len(prefix) || !strings.EqualFold(rest[:len(prefix)], prefix) {
		return
	}
	rest = rest[len(prefix):]
	name := rest
	if i := strings.IndexFunc(rest, unicode.IsSpace); i >= 0 {
		name = rest[:i]
	}
	names := suggestDirective(name)
	if len(names) == 0 {
		return
	}
	for i, n := range names {
		names[i] = "//gocpp:" + n
	}
	shown := c.Text[:len(c.Text)-len(rest)+len(name)] // "// gocpp:Link"
	msg := fmt.Sprintf("%q is an ordinary comment, not a directive: write %s", shown, strings.Join(names, " or "))
	if spaced {
		msg += ", with no space after //"
	}
	p.errs.Add(diag.Diagnostic{Pos: p.fset.Position(c.Slash), Severity: diag.Warning, Msg: msg})
}

// split splits the arguments in text[off:] of the directive name. It
// splits like the go command splits the arguments of a #cgo line
// (splitQuoted in go/build, which cmd/cgo copies), so code ported from cgo
// gets the same arguments:
//   - white space (unicode.IsSpace) separates the arguments;
//   - a single or double quote starts a quoted part, also in the middle of
//     an argument. It keeps white space and runs to the next quote of the
//     same kind. The quotes are removed: -DMSG="a b" gives -DMSG=a b;
//   - a backslash makes the next character an ordinary one, inside or
//     outside quotes, and is removed: \" gives ", and \\ gives \.
//
// Two rules are gocpp's own: an argument that starts with "//" (not quoted,
// not escaped) starts a comment, which runs to the end of the line; and an
// empty argument ("") is an error, where splitQuoted gives an empty
// string. split reports the first mistake and stops there.
func (p *directiveParser) split(text string, off int, name string) []directiveArg {
	var (
		args     []directiveArg
		arg      []byte // the current argument, without its quotes and backslashes
		inArg    bool   // arg has started, at start
		start    int    // the offset in text where arg starts
		quote    rune   // the open quote, or 0
		quoteAt  int    // the offset of the open quote
		escaped  bool   // the character before was a backslash, at escapeAt
		escapeAt int
	)
	for i := off; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if !inArg {
			if unicode.IsSpace(r) {
				i += size
				continue
			}
			if strings.HasPrefix(text[i:], "//") {
				break // a comment, up to the end of the line
			}
			inArg, start = true, i
		}
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped, escapeAt = true, i
			i += size
			continue
		case quote != 0:
			if r == quote {
				quote = 0
				i += size
				continue
			}
		case r == '"' || r == '\'':
			quote, quoteAt = r, i
			i += size
			continue
		case unicode.IsSpace(r):
			if len(arg) == 0 { // only quotes: "" or ''
				p.errorf(start, "empty argument in //gocpp:%s", name)
				return nil
			}
			args = append(args, directiveArg{text: string(arg), off: start})
			arg, inArg = arg[:0], false
			i += size
			continue
		}
		// AppendRune turns a byte that is not UTF-8 into U+FFFD, as
		// splitQuoted does.
		arg = utf8.AppendRune(arg, r)
		i += size
	}
	switch {
	case quote != 0: // splitQuoted checks the quote first, too
		p.errorf(quoteAt, "unterminated quote in //gocpp:%s", name)
		return nil
	case escaped:
		p.errorf(escapeAt, "backslash at the end of the line in //gocpp:%s", name)
		return nil
	case inArg && len(arg) == 0:
		p.errorf(start, "empty argument in //gocpp:%s", name)
		return nil
	case inArg:
		args = append(args, directiveArg{text: string(arg), off: start})
	}
	return args
}

// checkPkgConfigArg checks a pkg-config package name: it is not a flag,
// and it has only the characters that badNameChar allows.
func checkPkgConfigArg(arg string) string {
	if strings.HasPrefix(arg, "-") {
		return fmt.Sprintf("invalid pkg-config package name %q: flags go in //gocpp:cflags or //gocpp:ldflags", arg)
	}
	return badNameChar("pkg-config package name", arg)
}

// checkLinkArg checks a library name: it is not a flag, and it has only
// the characters that badNameChar allows. "-lm" gets its own hint, because
// it is an easy mistake: //gocpp:link adds the "-l" itself.
func checkLinkArg(arg string) string {
	if strings.HasPrefix(arg, "-") {
		if lib, ok := strings.CutPrefix(arg, "-l"); ok && lib != "" && checkLinkArg(lib) == "" {
			return fmt.Sprintf("invalid library name %q: write %s, without -l", arg, lib)
		}
		return fmt.Sprintf("invalid library name %q: flags go in //gocpp:ldflags", arg)
	}
	return badNameChar("library name", arg)
}

// badNameChar returns the error for a pkg-config package name or a library
// name (what says which) with a character that is not an ASCII letter, a
// digit or one of "_+-.", or "" if it has none. Together with the rule
// that such a name doesn't start with "-", this keeps the name from
// meaning anything else on a command line: not a flag, and not a response
// file (@file). The go command's load.SafeArg checks cgo's pkg-config
// names for the same reason.
func badNameChar(what, name string) string {
	for _, r := range name {
		switch {
		case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9':
		case r == '_', r == '+', r == '-', r == '.':
		default:
			return fmt.Sprintf("invalid character %q in %s %q", r, what, name)
		}
	}
	return ""
}

// checkHeaderArg checks the header of extern-c and no-extern-c. It is
// written like the header's import path, without the Go quotes: a system
// header in angle brackets (<lua.h>, <vector>), or a local header, whose
// path ends in one of the header extensions of the import rules (Phase 01
// README §2.3). The check is the one of the header imports (checkHeader),
// so a header that can be imported can be named here, and the other way
// around.
func checkHeaderArg(arg string) string {
	kind, msg := checkHeader(arg)
	switch {
	case kind == 0:
		return fmt.Sprintf("invalid header %q: want <name> or a path ending in %s", arg, headerExtList)
	case msg != "":
		return fmt.Sprintf("invalid header %q: %s", arg, msg)
	}
	return ""
}

// suggestDirective returns the known directive names that are closest to
// name, if any is close enough: at most one edit for every three
// characters of name, rounded up (the bound of clang's typo correction).
// The distance is the optimal string alignment distance (osaDistance) to
// the lowercased name, so "pkgconfig", "lnik" and "LINK" all find their
// directive. When several names are equally close, it returns all of
// them, in the order of directiveSpecs.
func suggestDirective(name string) []string {
	name = strings.ToLower(name)
	n := utf8.RuneCountInString(name)
	best := (n + 2) / 3
	var names []string
	for _, s := range directiveSpecs {
		// The difference in length is the least distance there can be.
		// Skipping such names also keeps a very long name cheap.
		if diff := n - len(s.name); diff > best || -diff > best {
			continue
		}
		switch d := osaDistance(name, s.name); {
		case d < best:
			best, names = d, []string{s.name}
		case d == best:
			names = append(names, s.name)
		}
	}
	return names
}

// osaDistance returns the optimal string alignment distance between a and
// b: the number of edits that turn a into b, where an edit inserts,
// deletes or changes one character, or swaps two neighboring characters,
// and no character is edited twice. It is the Levenshtein distance plus
// swaps, so the common typo "lnik" is one edit away from "link", not two.
// It counts runes, not bytes.
func osaDistance(a, b string) int {
	s, t := []rune(a), []rune(b)
	// d[i][j] is the distance between s[:i] and t[:j].
	d := make([][]int, len(s)+1)
	for i := range d {
		d[i] = make([]int, len(t)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(s); i++ {
		for j := 1; j <= len(t); j++ {
			change := 1
			if s[i-1] == t[j-1] {
				change = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+change)
			if i > 1 && j > 1 && s[i-1] == t[j-2] && s[i-2] == t[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(s)][len(t)]
}
