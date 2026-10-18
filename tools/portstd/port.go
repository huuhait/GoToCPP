package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// An OutFile is a file of the port.
type OutFile struct {
	Name    string // the name in stdlib/<pkg>/, with .gp
	From    string // Go's name, with .go
	Content []byte
	For     []string // the systems it was taken for
}

// A Native is a bodyless function that became a native declaration.
type Native struct {
	File string // the .gp name
	Func string
	Link string // the //go:linkname target, or ""
	Impl string // the C++ name, or "local"
}

// A Finding is something the tool noticed and the maintainer should know:
// a blocker stops the port, the rest is for PORT.md and -list.
type Finding struct {
	Kind    string // asm, linkname-pull, linkname-var, embed, cgo, native-todo, go:nosplit, unsafe, ...
	File    string // the .gp name
	Line    int
	Detail  string
	Blocker bool
}

// Result is a ported package, in memory.
type Result struct {
	Pkg      string
	Name     string
	Pin      Pin
	Opts     Options
	Files    []OutFile
	Skipped  []Skip
	Tests    int
	Patches  []AppliedPatch
	Notes    []string // the @note lines of the rules file
	Natives  []Native
	Findings []Finding
	Imports  []string // the import paths of the taken files, sorted
}

// Blockers returns the findings that stop the port.
func (r *Result) Blockers() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Blocker {
			out = append(out, f)
		}
	}
	return out
}

// A Porter ports packages of one Go tree.
type Porter struct {
	Root string // the extracted tree (Source.Root)
	Cfg  *Config
}

// headerLine is the first line of every ported file. The check mode tells
// ported files from hand-written ones by it.
func headerLine(pin Pin, pkg, from string) string {
	return fmt.Sprintf("%s%s src/%s/%s; DO NOT EDIT by hand; see PORT.md.", portedMarker, strings.TrimPrefix(pin.Version, "go"), pkg, from)
}

// portedMarker starts the header line of a ported file.
const portedMarker = "// Code ported from Go "

// Port makes the port of pkg in memory.
func (p *Porter) Port(pkg string, opts Options) (*Result, error) {
	pin := p.Cfg.Pin
	sel, err := selectPackage(p.Root, pin, pkg, opts)
	if err != nil {
		return nil, err
	}
	rules, err := p.Cfg.loadRules(pkg)
	if err != nil {
		return nil, err
	}
	var upstream []string
	for _, f := range sel.Taken {
		upstream = append(upstream, f.Name)
	}
	if err := rules.checkFiles(upstream); err != nil {
		return nil, fmt.Errorf("package %s: %v", pkg, err)
	}
	res := &Result{Pkg: pkg, Name: sel.Name, Pin: pin, Opts: opts, Skipped: sel.Skipped, Tests: sel.Tests, Notes: rules.Notes}
	for _, a := range sel.Asm {
		res.Findings = append(res.Findings, Finding{Kind: "asm", File: a, Detail: "assembly file kept by the build constraints: no pure-Go variant for this target", Blocker: true})
	}
	imports := make(map[string]bool)
	for _, f := range sel.Taken {
		if reason, skip := rules.Skips[f.Name]; skip {
			res.Skipped = append(res.Skipped, Skip{File: f.Name, Reason: "patch: " + reason})
			continue
		}
		src, err := os.ReadFile(filepath.Join(p.Root, "src", filepath.FromSlash(pkg), f.Name))
		if err != nil {
			return nil, err
		}
		text, applied, err := rules.apply(f.Name, string(src))
		if err != nil {
			return nil, fmt.Errorf("package %s: %v", pkg, err)
		}
		res.Patches = append(res.Patches, applied...)
		out, err := p.processFile(res, pkg, f, text, imports)
		if err != nil {
			return nil, fmt.Errorf("package %s: %s: %v", pkg, f.Name, err)
		}
		res.Files = append(res.Files, out)
	}
	slices.SortFunc(res.Skipped, func(a, b Skip) int { return strings.Compare(a.File, b.File) })
	res.Imports = sortedKeys(imports)
	slices.SortFunc(res.Findings, func(a, b Finding) int {
		if c := strings.Compare(a.File, b.File); c != 0 {
			return c
		}
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(a.Kind, b.Kind)
	})
	slices.SortFunc(res.Natives, func(a, b Native) int {
		if c := strings.Compare(a.File, b.File); c != 0 {
			return c
		}
		return strings.Compare(a.Func, b.Func)
	})
	return res, nil
}

// lineEdits are the changes to the lines of a file, by 1-based line number.
type lineEdits struct {
	del    map[int]bool
	repl   map[int]string
	before map[int][]string
	after  map[int][]string
}

func newLineEdits() *lineEdits {
	return &lineEdits{del: map[int]bool{}, repl: map[int]string{}, before: map[int][]string{}, after: map[int][]string{}}
}

// apply returns the text with the changes made.
func (e *lineEdits) apply(lines []string) []string {
	var out []string
	for i, l := range lines {
		n := i + 1
		out = append(out, e.before[n]...)
		switch {
		case e.del[n]:
		case hasKey(e.repl, n):
			out = append(out, e.repl[n])
		default:
			out = append(out, l)
		}
		out = append(out, e.after[n]...)
	}
	return out
}

func hasKey(m map[int]string, n int) bool {
	_, ok := m[n]
	return ok
}

// A linkname is one //go:linkname directive.
type linkname struct {
	c      *ast.Comment
	line   int
	local  string
	remote string
}

// processFile ports one file whose text has the patches applied.
func (p *Porter) processFile(res *Result, pkg string, sel fileSel, text string, imports map[string]bool) (OutFile, error) {
	pin := p.Cfg.Pin
	name := strings.TrimSuffix(sel.Name, ".go") + ".gp"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, sel.Name, text, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return OutFile{}, err
	}
	lines := strings.Split(text, "\n")
	edits := newLineEdits()
	lineOf := func(pos token.Pos) int { return fset.Position(pos).Line }
	finding := func(kind string, pos token.Pos, blocker bool, format string, args ...any) {
		res.Findings = append(res.Findings, Finding{Kind: kind, File: name, Line: lineOf(pos), Detail: fmt.Sprintf(format, args...), Blocker: blocker})
	}
	env := tagEnv{arch: res.Opts.Arch, tags: res.Opts.Tags, release: pin.minor()}

	// The build constraints of the header (before the package clause).
	var haveGoBuild bool
	var plusBuild []*ast.Comment
	for _, g := range f.Comments {
		if g.Pos() >= f.Package {
			break
		}
		for _, c := range g.List {
			switch {
			case strings.HasPrefix(c.Text, "//go:build "):
				haveGoBuild = true
				line := c.Text
				repl, err := env.rewriteBuildLine(line, sel.For)
				if err != nil {
					return OutFile{}, err
				}
				if repl == "" {
					edits.del[lineOf(c.Slash)] = true
				} else {
					if repl != line {
						edits.repl[lineOf(c.Slash)] = repl
					}
					finding("go:build", c.Slash, false, "kept: %s", repl)
				}
			case strings.HasPrefix(c.Text, "// +build "):
				plusBuild = append(plusBuild, c)
			}
		}
	}
	for _, c := range plusBuild {
		if !haveGoBuild {
			finding("plus-build", c.Slash, true, "a // +build line without //go:build: the tool does not rewrite it")
			continue
		}
		edits.del[lineOf(c.Slash)] = true
	}

	// Declarations and directives.
	funcs := make(map[string]*ast.FuncDecl)
	vars := make(map[string]bool)
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				funcs[d.Name.Name] = d
			}
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				for _, s := range d.Specs {
					for _, n := range s.(*ast.ValueSpec).Names {
						vars[n.Name] = true
					}
				}
			}
		}
	}
	linknames := make(map[string]linkname)
	var directives []linkname
	for _, g := range f.Comments {
		for _, c := range g.List {
			rest, ok := strings.CutPrefix(c.Text, "//go:")
			if !ok || g.Pos() < f.Package {
				continue
			}
			dname, args, _ := strings.Cut(rest, " ")
			dname, _, _ = strings.Cut(dname, "\t")
			switch {
			case dname == "build" || dname == "generate":
			case dname == "linkname":
				fs := strings.Fields(args)
				if len(fs) < 1 || len(fs) > 2 {
					finding("linkname-syntax", c.Slash, true, "%s", c.Text)
					continue
				}
				ln := linkname{c: c, line: lineOf(c.Slash), local: fs[0]}
				if len(fs) == 2 {
					ln.remote = fs[1]
				}
				linknames[ln.local] = ln
				directives = append(directives, ln)
			case dname == "embed":
				finding("embed", c.Slash, true, "//go:embed is not supported in the standard library (Phase 10 has #embed)")
			case strings.HasPrefix(dname, "cgo_"):
				finding("cgo", c.Slash, true, "%s", c.Text)
			default:
				finding("go:"+dname, c.Slash, false, "kept as a comment")
			}
		}
	}

	// Linkname directives: every one goes away; a pull of a bodyless function
	// becomes a native declaration through the table.
	natives := make(map[*ast.FuncDecl]NativeEntry)
	for _, ln := range directives {
		edits.del[ln.line] = true
		fd := funcs[ln.local]
		switch {
		case fd == nil && vars[ln.local]:
			finding("linkname-var", ln.c.Slash, true, "%s: a variable that the runtime provides: patch it (patches/%s)", ln.c.Text, patchFile(pkg))
		case fd == nil:
			finding("linkname-syntax", ln.c.Slash, true, "%s: no such function or variable in this file", ln.c.Text)
		case fd.Body != nil && ln.remote == "":
			finding("linkname-export", ln.c.Slash, false, "%s: only marks the function as used by other packages; dropped", ln.local)
		case fd.Body != nil:
			finding("linkname-push", ln.c.Slash, false, "%s -> %s: gives the function to the runtime; dropped", ln.local, ln.remote)
		}
	}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body != nil {
			continue
		}
		if fd.Recv != nil {
			finding("bodyless-method", fd.Pos(), true, "method %s has no body", fd.Name.Name)
			continue
		}
		ln, hasLink := linknames[fd.Name.Name]
		keys := []string{pkg + "." + fd.Name.Name}
		if hasLink && ln.remote != "" {
			keys = []string{ln.remote, keys[0]}
		}
		var entry NativeEntry
		var found bool
		for _, k := range keys {
			if entry, found = p.Cfg.Natives[k]; found {
				break
			}
		}
		switch {
		case !found && hasLink && ln.remote != "":
			finding("linkname-pull", fd.Pos(), true, "%s has no entry in natives.txt (%s)", fd.Name.Name, strings.Join(keys, " or "))
		case !found:
			finding("asm", fd.Pos(), true, "%s has no body: assembly, or implemented by the runtime; no entry %s in natives.txt", fd.Name.Name, keys[0])
		case entry.TODO != "":
			finding("native-todo", fd.Pos(), true, "%s: TODO: %s", fd.Name.Name, entry.TODO)
		default:
			natives[fd] = entry
			link := ""
			if hasLink {
				link = ln.remote
			}
			res.Natives = append(res.Natives, Native{File: name, Func: fd.Name.Name, Link: link, Impl: entry.Impl})
		}
	}
	for fd, entry := range natives {
		marker := "//gocpp:native"
		if entry.Impl != "local" {
			marker += " " + entry.Impl
		}
		if fd.Doc == nil {
			edits.before[lineOf(fd.Pos())] = append(edits.before[lineOf(fd.Pos())], marker)
			continue
		}
		last := ""
		for i := len(fd.Doc.List) - 1; i >= 0; i-- {
			c := fd.Doc.List[i]
			if !edits.del[lineOf(c.Slash)] {
				last = c.Text
				break
			}
		}
		add := []string{marker}
		if last != "" && last != "//" && !strings.HasPrefix(last, "//go:") && !strings.HasPrefix(last, "//gocpp:") {
			add = []string{"//", marker}
		}
		end := lineOf(fd.Doc.End())
		edits.after[end] = append(edits.after[end], add...)
	}

	// Things that need a human: unsafe, reflect.
	var unsafeUses int
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "unsafe" {
				unsafeUses++
			}
		}
		return true
	})
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		imports[path] = true
		if path == "unsafe" && unsafeUses > 0 {
			finding("unsafe", imp.Pos(), false, "%d uses of the unsafe package (Phase 04 keeps gocpp memory alive for unsafe.String/Slice)", unsafeUses)
		}
		if path == "reflect" || path == "internal/reflectlite" {
			finding("reflect", imp.Pos(), false, "imports %s: needs the reflect subset (step 9.3)", path)
		}
	}

	out := edits.apply(lines)
	out = collapseHeaderBlanks(out)
	body := strings.Join(out, "\n")
	content := headerLine(pin, pkg, sel.Name) + "\n\n" + body
	if formatted, err := format.Source([]byte(content)); err != nil {
		return OutFile{}, fmt.Errorf("the port does not parse: %v", err)
	} else if !bytes.Equal(formatted, []byte(content)) {
		res.Findings = append(res.Findings, Finding{Kind: "format", File: name, Detail: "the port is not gofmt-clean: a patch or a rewrite left odd spacing"})
	}
	return OutFile{Name: name, From: sel.Name, Content: []byte(content), For: sel.For}, nil
}

// collapseHeaderBlanks removes the second of two blank lines in a row before
// the package clause (a deleted //go:build line leaves them), and blank lines
// at the very top.
func collapseHeaderBlanks(lines []string) []string {
	var out []string
	for i, l := range lines {
		if strings.HasPrefix(l, "package ") || l == "package" {
			return append(out, lines[i:]...)
		}
		if strings.TrimSpace(l) == "" && (len(out) == 0 || strings.TrimSpace(out[len(out)-1]) == "") {
			continue
		}
		out = append(out, l)
	}
	return out
}
