package main

import (
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
)

// Facts are what the classifier reads from the text of one test file.
type Facts struct {
	Imports []string // import paths, sorted, without duplicates
	Uses    map[string]bool
}

var (
	importLineRx = regexp.MustCompile(`(?m)^\s*(?:import\s*)?(?:[A-Za-z_][A-Za-z_0-9]*\s+|\.\s+)?"([^"\n]+)"\s*(?://.*)?$`)
	importDecl   = regexp.MustCompile(`(?ms)^import\s*(?:\(.*?\)|[^\n]*)`)

	// selectorRx finds pkg.Name uses of the packages the classifier cares about.
	selectorRx = regexp.MustCompile(`\b(runtime|debug|reflect|unsafe|os|syscall|atomic|sync|time|testing|weak|unique|signal|exec|metrics|pprof)\.([A-Z][A-Za-z0-9_]*)`)

	pragmaRx = regexp.MustCompile(`(?m)^//go:([a-z_0-9]+)`)
)

// scanFacts reads the imports and the qualified names of a test source.
func scanFacts(src string) Facts {
	f := Facts{Uses: map[string]bool{}}
	set := map[string]bool{}
	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "x.go", src, parser.ImportsOnly)
	if file != nil && len(file.Imports) > 0 {
		for _, im := range file.Imports {
			if p, err := strconv.Unquote(im.Path.Value); err == nil {
				set[p] = true
			}
		}
	}
	if len(set) == 0 {
		// a file with a syntax error (an errorcheck test): read the import lines
		// of the first import declarations by hand
		head := src
		if i := strings.Index(src, "\nfunc "); i >= 0 {
			head = src[:i]
		}
		for _, d := range importDecl.FindAllString(head, -1) {
			for _, m := range importLineRx.FindAllStringSubmatch(d, -1) {
				set[m[1]] = true
			}
			if m := regexp.MustCompile(`^import\s+(?:[A-Za-z_.][A-Za-z_0-9]*\s+)?"([^"]+)"`).FindStringSubmatch(d); m != nil {
				set[m[1]] = true
			}
		}
	}
	for p := range set {
		f.Imports = append(f.Imports, p)
	}
	sort.Strings(f.Imports)
	code := stripComments(src)
	for _, m := range selectorRx.FindAllStringSubmatch(code, -1) {
		f.Uses[m[1]+"."+m[2]] = true
	}
	for _, m := range pragmaRx.FindAllStringSubmatch(src, -1) {
		f.Uses["//go:"+m[1]] = true
	}
	return f
}

// stripComments removes // comments and /* */ comments and the contents of
// string and rune literals, well enough for searching for names: it keeps
// the line structure.
func stripComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				if src[i] == '\n' {
					b.WriteByte('\n')
				}
				i++
			}
			i += 2
		case c == '"':
			b.WriteString(`""`)
			i++
			for i < len(src) && src[i] != '"' && src[i] != '\n' {
				if src[i] == '\\' {
					i++
				}
				i++
			}
			i++
		case c == '`':
			b.WriteString("``")
			i++
			for i < len(src) && src[i] != '`' {
				if src[i] == '\n' {
					b.WriteByte('\n')
				}
				i++
			}
			i++
		case c == '\'':
			b.WriteString("''")
			i++
			for i < len(src) && src[i] != '\'' && src[i] != '\n' {
				if src[i] == '\\' {
					i++
				}
				i++
			}
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// merge returns the union of the facts of two files of one test.
func (f Facts) merge(g Facts) Facts {
	set := map[string]bool{}
	for _, p := range f.Imports {
		set[p] = true
	}
	for _, p := range g.Imports {
		set[p] = true
	}
	out := Facts{Uses: map[string]bool{}}
	for p := range set {
		out.Imports = append(out.Imports, p)
	}
	sort.Strings(out.Imports)
	for u := range f.Uses {
		out.Uses[u] = true
	}
	for u := range g.Uses {
		out.Uses[u] = true
	}
	return out
}

// declaresReservedC reports whether a file declares a name that gocpp reserves
// (C: deviation D19), by the loader's own rule (load.CheckReservedC). A file
// that does not parse is read as far as the parser got.
func declaresReservedC(src string) bool {
	if !strings.Contains(src, "C") {
		return false
	}
	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "x.go", src, parser.SkipObjectResolution)
	if file == nil {
		return false
	}
	var errs diag.List
	load.CheckReservedC(fset, file, &errs)
	return errs.HasErrors()
}

// useList returns the qualified names a test uses, sorted: the classifier's
// first match must not depend on map order (the manifest is deterministic).
func (f Facts) useList() []string {
	us := make([]string, 0, len(f.Uses))
	for u := range f.Uses {
		us = append(us, u)
	}
	sort.Strings(us)
	return us
}
