package doc

import (
	"bytes"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// A header is a C or C++ header, or a C++ namespace: the packages that gocpp
// does not read. A .gp file that imports one uses its names as pkg.Name, and
// the C++ compiler checks them, so gocpp has no declarations to document.
// What "gocpp doc" can say about it is where the header is, which name it
// gets in a .gp file, what its Doxygen comments say (read textually from the
// header file, not from the headers it includes), and, with the compiler,
// what a name is (the kind probe of internal/interop).
type header struct {
	spec    string // what the user typed: "<stdio.h>", "./physics.h", "github.com/x/y/z.h", "std"
	ns      bool   // a namespace, not a header
	include string // the include text for the probe: "<stdio.h>" or "\"/abs/physics.h\""
	file    string // the header file ("" if it is not known)
	note    string // why file is "" for a system header
	path    string // the import path of the header in a .gp file: "<stdio.h>", "./physics.h", "github.com/x/y/z.h"
}

var headerExts = []string{".h", ".hh", ".hpp", ".hxx", ".h++", ".inl"}

// hasHeaderExt reports whether the path ends in a C or C++ header extension.
func hasHeaderExt(p string) bool {
	for _, ext := range headerExts {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

// headerArg reports whether the argument arg names a header: "<stdio.h>" or
// "<vector>" (a system header), or a path ending in a header extension: a
// file relative to the current folder, a header of a module (a first element
// with a dot: "github.com/ocornut/imgui/imgui.h"), or else a system header
// spelled without the <>. The error is for a header that is not there.
func (r *runner) headerArg(arg string) (*header, bool, error) {
	switch {
	case strings.HasPrefix(arg, "<") && strings.HasSuffix(arg, ">") && len(arg) > 2:
		return r.systemHeader(arg, arg[1:len(arg)-1]), true, nil
	case !hasHeaderExt(arg):
		return nil, false, nil
	}
	local := func(file string) (*header, bool, error) {
		return &header{spec: arg, path: arg, file: file, include: strconv.Quote(filepath.ToSlash(file))}, true, nil
	}
	abs := arg
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(r.dir, filepath.FromSlash(arg))
	}
	if fi, err := os.Stat(abs); err == nil && !fi.IsDir() {
		return local(abs)
	} else if isDotSlash(arg) || filepath.IsAbs(arg) {
		return nil, true, fmt.Errorf("header %s: no such file (%s)", arg, abs)
	}
	if load.IsModuleHeaderPath(arg) {
		var best *root
		for _, rt := range r.modules().roots {
			if strings.HasPrefix(arg, rt.importPath+"/") && (best == nil || len(rt.importPath) > len(best.importPath)) {
				best = rt
			}
		}
		if best == nil {
			return nil, true, fmt.Errorf("no required module provides header %s", arg)
		}
		file := filepath.Join(best.dir, filepath.FromSlash(strings.TrimPrefix(arg, best.importPath+"/")))
		if fi, err := os.Stat(file); err != nil || fi.IsDir() {
			return nil, true, fmt.Errorf("header %s is not in module %s (%s)", arg, best.importPath, file)
		}
		return local(file)
	}
	// A system header spelled without the <>: but a.b.h may also be the field
	// h of the type b in the package a, which go doc would show.
	if !strings.Contains(arg, "/") && r.resolvesAsSymbolPath(arg) {
		return nil, false, nil
	}
	return r.systemHeader("<"+arg+">", arg), true, nil
}

// resolvesAsSymbolPath reports whether the argument, read as [package.]symbol,
// names a package that is there (the usual case of parseArgs with one
// argument), or a name in the package of the current folder.
func (r *runner) resolvesAsSymbolPath(arg string) bool {
	defer r.dirs.Reset()
	if !strings.ContainsAny(arg, `/\`) && token.IsExported(arg) {
		if _, err := r.importDir("."); err == nil {
			return true
		}
	}
	for start := 0; start < len(arg); {
		period := strings.Index(arg[start:], ".")
		if period < 0 {
			period = len(arg)
		} else {
			period += start
		}
		if _, err := r.importPath(arg[:period]); err == nil {
			return true
		}
		for {
			path, ok := r.findNextPackage(arg[:period])
			if !ok {
				break
			}
			if _, err := r.importDirEntry(path); err == nil {
				return true
			}
		}
		r.dirs.Reset()
		start = period + 1
	}
	return false
}

// notFoundNote is the note of a system header that the compiler does not
// find: an error for "gocpp doc", unlike a compiler that fails.
const notFoundNote = "the C++ compiler does not find it"

// systemHeader makes the header <name>, and finds its file.
func (r *runner) systemHeader(path, name string) *header {
	h := &header{spec: path, path: path, include: path}
	file, err := r.probe().FindHeader(r.ctx, name)
	switch {
	case err != nil:
		h.note = err.Error()
	case file == "":
		h.note = notFoundNote
	default:
		h.file = file
	}
	return h
}

// namespaceArg returns the namespace that the argument names, or nil: std,
// std.vector (the namespace std and a name in it), and ::a::b.
func (r *runner) namespaceArg(arg string) *header {
	if arg == "std" || strings.HasPrefix(arg, "std.") || strings.HasPrefix(arg, "std::") || strings.HasPrefix(arg, "::") {
		return &header{spec: arg, ns: true}
	}
	return nil
}

// nsText is what "gocpp doc std" prints.
const nsText = `A C++ namespace is not a package: it needs no import. Every name that the
headers imported by a file declare in it can be used as %[1]s.Name; the C++
compiler checks the name when the program is built. To use it under another
name, import it with that name: import fs "::std::filesystem".

Which names exist depends on the headers, so ask with one of them, for example:

	gocpp doc '<vector>' std.vector
`

// headerDoc prints the documentation of a header or a namespace.
func (r *runner) headerDoc(t target) (err error) {
	h := t.hdr
	var buf bytes.Buffer
	defer func() {
		if err == nil { // an error comes alone, without a half of the text
			r.w.Write(buf.Bytes())
		}
	}()
	if h.ns {
		first := strings.TrimPrefix(strings.ReplaceAll(strings.SplitN(h.spec, ".", 2)[0], "::", "."), ".")
		if first == "" {
			first = "std"
		}
		if !r.opts.Short {
			fmt.Fprintf(&buf, "namespace %s // C++ namespace\n\n", strings.TrimPrefix(first, "."))
		}
		fmt.Fprintf(&buf, nsText, strings.TrimPrefix(first, "."))
		return nil
	}
	name, why, err := headerPackageName(h.path)
	if err != nil {
		return err
	}
	if h.file == "" && h.note == notFoundNote && t.symbol == "" {
		return fmt.Errorf("header %s: not found (%s)", h.spec, h.note)
	}
	var decls []hdrDecl
	if h.file != "" {
		src, err := os.ReadFile(h.file)
		if err != nil {
			return fmt.Errorf("header %s: %v", h.spec, errorText(err))
		}
		decls = extractDecls(string(src))
	}
	if !r.opts.Short {
		fmt.Fprintf(&buf, "header %s\n\n", h.spec)
	}
	symbol := t.symbol
	if symbol == "" {
		if !r.opts.Short {
			buf.WriteString("Opaque package: gocpp does not read what the header declares, so it has no\n")
			buf.WriteString("documentation for it; the C++ compiler checks the names that a file uses.\n\n")
			if name != "" {
				fmt.Fprintf(&buf, "Package name: %s\n", name)
			} else {
				fmt.Fprintf(&buf, "Package name: none (%s; give it a name: import x %q)\n", why, h.path)
			}
			switch {
			case h.file != "":
				fmt.Fprintf(&buf, "Header file: %s\n", h.file)
			case h.note != "":
				fmt.Fprintf(&buf, "Header file: not found (%s)\n", h.note)
			}
		}
		r.printDecls(&buf, decls, r.opts.All)
		return nil
	}
	// A name in the header: its Doxygen comment, or what the compiler says.
	if !strings.ContainsAny(symbol, ".:") {
		var found bool
		for _, d := range decls {
			if matchName(symbol, d.name, r.opts.MatchCase) {
				r.printDecls(&buf, []hdrDecl{d}, true)
				found = true
			}
		}
		if found {
			return nil
		}
	}
	path := strings.ReplaceAll(strings.TrimPrefix(symbol, "::"), ".", "::")
	kind, err := r.probe().Kind(r.ctx, []string{h.include}, path)
	switch {
	case err != nil:
		return fmt.Errorf("cannot tell what %s is in header %s: %v", symbol, h.spec, err)
	case kind == "not found":
		return fmt.Errorf("no symbol %s in header %s", symbol, h.spec)
	case kind == "unknown":
		return fmt.Errorf("cannot tell what %s is in header %s", symbol, h.spec)
	}
	if r.opts.Short {
		fmt.Fprintf(&buf, "%s: %s\n", symbol, kind)
		return nil
	}
	fmt.Fprintf(&buf, "%s: %s, declared after #include %s\n\n", symbol, kindText(kind), strings.Trim(h.include, "\""))
	buf.WriteString("The header has no documentation comment for it.\n")
	return nil
}

// kindText words the kind that the probe found.
func kindText(kind string) string {
	switch kind {
	case "namespace":
		return "a C++ namespace"
	case "type":
		return "a type"
	case "template":
		return "a class or alias template (spelled name[args] in a .gp file)"
	case "value":
		return "a value (an object, a function or an enumerator)"
	case "value template":
		return "a function or variable template (spelled name[args])"
	case "macro":
		return "a macro"
	}
	return kind
}

// printDecls prints the declarations with a Doxygen comment: one line each,
// or with their comments (all).
func (r *runner) printDecls(buf *bytes.Buffer, decls []hdrDecl, all bool) {
	if len(decls) == 0 {
		return
	}
	if !r.opts.Short {
		if !bytes.HasSuffix(buf.Bytes(), []byte("\n\n")) {
			buf.WriteString("\n")
		}
		buf.WriteString("DECLARATIONS WITH DOCUMENTATION COMMENTS\n\n")
	}
	for _, d := range decls {
		buf.WriteString(d.decl + "\n")
		if all && !r.opts.Short {
			for line := range strings.Lines(d.comment) {
				if strings.TrimSpace(line) == "" {
					buf.WriteString("\n")
				} else {
					buf.WriteString(indent + line)
				}
			}
			buf.WriteString("\n")
		}
	}
}

// headerPackageName returns the name that the import of the header path
// gets in a .gp file (the default name: ClassifyImports decides), or why it
// has none.
func headerPackageName(path string) (name, why string, err error) {
	fset := token.NewFileSet()
	f, perr := parser.ParseFile(fset, "x.gp", "package p\nimport "+strconv.Quote(path)+"\n", parser.ImportsOnly)
	if perr != nil {
		return "", "", fmt.Errorf("header %s: %v", path, perr)
	}
	errs := new(diag.List)
	info := load.ClassifyImports(fset, f, map[string]token.Pos{},
		func(string) string { return "" }, func(string) []string { return nil }, errs)
	if errs.HasErrors() {
		return "", "", fmt.Errorf("header %s: %s", path, errs.Diagnostics()[0].Msg)
	}
	if len(info.Includes) != 1 {
		return "", "", fmt.Errorf("%s is not a header", path)
	}
	inc := info.Includes[0]
	if inc.Name != "" {
		return inc.Name, "", nil
	}
	return "", fmt.Sprintf("%s is not a usable name: %s", inc.WouldBe, inc.Why), nil
}

// A hdrDecl is a declaration of a header with a Doxygen comment.
type hdrDecl struct {
	name    string // the declared name, as well as it can be found
	decl    string // the declaration on one line, without its body
	comment string // the comment text, without the comment marks
}

var (
	reTemplate = regexp.MustCompile(`^template\s*<`)
	reTypeKw   = regexp.MustCompile(`\b(?:struct|class|union|enum(?:\s+class|\s+struct)?|namespace)\s+(?:\[\[[^\]]*\]\]\s*)*([A-Za-z_]\w*)`)
	reUsing    = regexp.MustCompile(`^using\s+([A-Za-z_]\w*)\s*=`)
	reDefine   = regexp.MustCompile(`^#\s*define\s+([A-Za-z_]\w*)`)
	reFuncName = regexp.MustCompile(`([A-Za-z_~]\w*|operator\s*\S+)\s*\(`)
	reTypedef  = regexp.MustCompile(`\(\s*\*\s*([A-Za-z_]\w*)\s*\)`)
	reLastName = regexp.MustCompile(`([A-Za-z_]\w*)\s*(?:\[[^\]]*\]\s*)*(?:=.*)?;?\s*$`)
)

// extractDecls reads the declarations that have a Doxygen comment (///,
// //!, /** */ or /*! */) right before them, from the text of one header. It
// is a line reader, not a parser: a declaration is the lines from the
// comment up to a ; or a {, and the name is guessed from its shape (a
// function, a type, a macro, a typedef, a variable). A declaration that it
// can't make sense of is left out.
func extractDecls(src string) []hdrDecl {
	lines := strings.Split(strings.ReplaceAll(decodeHeaderText(src), "\r\n", "\n"), "\n")
	var out []hdrDecl
	for i := 0; i < len(lines); {
		comment, next, ok := readDoxyComment(lines, i)
		if !ok {
			i++
			continue
		}
		decl, name, end := readDeclaration(lines, next)
		i = max(end, next, i+1)
		if decl != "" && name != "" {
			out = append(out, hdrDecl{name: name, decl: decl, comment: comment})
		}
	}
	return out
}

// readDoxyComment reads a Doxygen comment that starts at lines[i]. It
// returns its text and the index of the line after it.
func readDoxyComment(lines []string, i int) (text string, next int, ok bool) {
	t := strings.TrimSpace(lines[i])
	var out []string
	switch {
	case strings.HasPrefix(t, "///") && !strings.HasPrefix(t, "////"), strings.HasPrefix(t, "//!"):
		j := i
		for ; j < len(lines); j++ {
			l := strings.TrimSpace(lines[j])
			if !(strings.HasPrefix(l, "///") && !strings.HasPrefix(l, "////") || strings.HasPrefix(l, "//!")) {
				break
			}
			out = append(out, strings.TrimPrefix(l[3:], " "))
		}
		return strings.Join(out, "\n") + "\n", j, true
	case strings.HasPrefix(t, "/**") && !strings.HasPrefix(t, "/**/") && !strings.HasPrefix(t, "/***"), strings.HasPrefix(t, "/*!"):
		j := i
		for ; j < len(lines); j++ {
			l := strings.TrimSpace(lines[j])
			if j == i {
				l = strings.TrimPrefix(strings.TrimPrefix(l, "/**"), "/*!")
			}
			// The end comes first: " */" on a line of its own must not lose
			// its star to the margin.
			end := false
			if k := strings.Index(l, "*/"); k >= 0 {
				l, end = strings.TrimRight(l[:k], "*"), true
			}
			if j != i {
				l = strings.TrimPrefix(l, "*") // the margin
			}
			l = strings.TrimSpace(l)
			if !(l == "" && (j == i || end)) {
				out = append(out, l)
			}
			if end {
				j++
				return strings.Join(out, "\n") + "\n", j, true
			}
		}
		// A comment that never ends is not a documentation comment.
		return "", i, false
	}
	return "", i, false
}

// readDeclaration reads the declaration that starts at lines[i] (a line that
// is not blank: a comment is documented only when the declaration follows
// it directly). It returns the declaration as one line, its name and the
// index of the line after it. An empty declaration means that it is not one.
func readDeclaration(lines []string, i int) (decl, name string, end int) {
	if i >= len(lines) || strings.TrimSpace(lines[i]) == "" {
		return "", "", i
	}
	first := strings.TrimSpace(lines[i])
	if strings.HasPrefix(first, "#") {
		if m := reDefine.FindStringSubmatch(first); m != nil {
			j := i
			var parts []string
			for ; j < len(lines); j++ {
				l := strings.TrimSpace(lines[j])
				cont := strings.HasSuffix(l, `\`)
				parts = append(parts, strings.TrimSpace(strings.TrimSuffix(l, `\`)))
				if !cont {
					j++
					break
				}
			}
			return strings.Join(parts, " "), m[1], j
		}
		return "", "", i + 1 // #include, #pragma, #ifndef: not a declaration
	}
	if strings.HasPrefix(first, "}") || strings.HasPrefix(first, "//") || strings.HasPrefix(first, "/*") {
		return "", "", i
	}
	var parts []string
	j := i
	for ; j < len(lines) && j < i+30; j++ {
		l := strings.TrimSpace(stripLineComment(lines[j]))
		if k := strings.Index(l, "{"); k >= 0 {
			parts = append(parts, strings.TrimSpace(l[:k]))
			j++
			break
		}
		parts = append(parts, l)
		if strings.HasSuffix(l, ";") {
			j++
			break
		}
	}
	decl = strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
	if decl == "" {
		return "", "", j
	}
	return decl, declName(decl), j
}

// stripLineComment cuts a // comment off the end of a line.
func stripLineComment(l string) string {
	if k := strings.Index(l, "//"); k >= 0 {
		return l[:k]
	}
	return l
}

// declName guesses the name that a declaration (one line) declares.
func declName(decl string) string {
	d := decl
	// Skip template<...>, balancing the angle brackets.
	for reTemplate.MatchString(d) {
		depth, k := 0, strings.Index(d, "<")
		for ; k < len(d); k++ {
			if d[k] == '<' {
				depth++
			} else if d[k] == '>' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		if k >= len(d) {
			return ""
		}
		d = strings.TrimSpace(d[k+1:])
	}
	if m := reUsing.FindStringSubmatch(d); m != nil {
		return m[1]
	}
	if strings.HasPrefix(d, "typedef") {
		if m := reTypedef.FindStringSubmatch(d); m != nil {
			return m[1]
		}
		if k := strings.Index(d, "("); k < 0 {
			if m := reLastName.FindStringSubmatch(strings.TrimSuffix(d, ";")); m != nil {
				return m[1]
			}
		} else if m := reFuncName.FindStringSubmatch(d); m != nil {
			return m[1]
		}
		return ""
	}
	if m := reTypeKw.FindStringSubmatch(d); m != nil && !strings.Contains(d[:strings.Index(d, m[0])], "(") {
		// A function that returns a struct ("struct Foo make(void)") is a
		// function: a "(" after the type name means that.
		rest := d[strings.Index(d, m[0])+len(m[0]):]
		if !strings.HasPrefix(strings.TrimSpace(rest), "(") && !strings.Contains(rest, "(") {
			return m[1]
		}
	}
	if k := strings.Index(d, "("); k >= 0 {
		if m := reFuncName.FindStringSubmatch(d); m != nil {
			return strings.TrimSpace(m[1])
		}
		return ""
	}
	if m := reLastName.FindStringSubmatch(d); m != nil {
		return m[1]
	}
	return ""
}

var _ fs.FS // keep io/fs for the Windows build of root handling

// cp1252 holds the characters of the bytes 0x80 to 0x9f in Windows-1252 (0
// for the five that it leaves undefined).
var cp1252 = [32]rune{
	0x20ac, 0, 0x201a, 0x0192, 0x201e, 0x2026, 0x2020, 0x2021, 0x02c6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017d, 0,
	0, 0x2018, 0x2019, 0x201c, 0x201d, 0x2022, 0x2013, 0x2014, 0x02dc, 0x2122, 0x0161, 0x203a, 0x0153, 0, 0x017e, 0x0178,
}

// decodeHeaderText returns the text of a header as UTF-8. A leading byte
// order mark is dropped. Valid UTF-8 stays as it is; each byte that is not
// part of a valid sequence is read as Windows-1252, the usual encoding of old
// headers, so a file with a stray Latin-1 byte keeps its other characters.
func decodeHeaderText(src string) string {
	src = strings.TrimPrefix(src, "\uFEFF")
	if utf8.ValidString(src) {
		return src
	}
	var sb strings.Builder
	for i := 0; i < len(src); {
		r, n := utf8.DecodeRuneInString(src[i:])
		if r != utf8.RuneError || n != 1 {
			sb.WriteString(src[i : i+n])
			i += n
			continue
		}
		c := src[i]
		switch {
		case c >= 0x80 && c < 0xa0 && cp1252[c-0x80] != 0:
			sb.WriteRune(cp1252[c-0x80])
		case c >= 0x80 && c < 0xa0:
			sb.WriteRune(utf8.RuneError)
		default:
			sb.WriteRune(rune(c))
		}
		i++
	}
	return sb.String()
}
