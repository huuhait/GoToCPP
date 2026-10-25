// Command closure is the import-closure analysis of job 16.1f-stubs (Phase 16
// README §3 step 16.1): for Go's container/list, strings, strconv and sort,
// which packages they import under the GC engine's build constraints (README
// §2.7), and exactly which identifiers of runtime, internal/bytealg and
// internal/reflectlite the closure uses.
//
// The walk stops at the three stub packages: they are replaced wholesale by
// gostd/spike/stub/, so their own imports are not translated: runtime's 23
// direct imports (22 internal packages plus unsafe), the 19 internal packages
// reached transitively through the runtime that are outside the closure, and
// internal/unsafeheader through internal/reflectlite (20 external packages in
// all, summary.external_packages in the JSON).
//
// The package graph is read from the Go source tree with go/build's
// Context.MatchFile (not ImportDir, which refuses a compiler other than gc or
// gccgo), then every package is parsed and type-checked with go/types and a
// source importer over the same selection. The type checker is what makes the
// identifier list exact: a selector use, a method call on a reflectlite.Value
// or an identifier that names a reflectlite.Type is a *types.Object whose Pkg
// is the stub package. The linkname directives are read from the comments and
// classified (pull, push, one-argument, variable, linker), and the runtime
// source is scanned for the pushes into closure packages, which the runtime
// stub has to provide.
//
// The output is the committed gostd/spike/closure.json. It is deterministic:
// the walk, the type check and every sort are sequential and stable.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const schema = 1

// The four spike packages of README §3 step 16.1.
var roots = []string{"container/list", "strings", "strconv", "sort"}

// The packages the job writes stubs for; the closure walk does not descend
// into them.
var stubs = []string{"runtime", "internal/bytealg", "internal/reflectlite"}

// tagSet is the build constraint set of Phase 16 README §2.7, for one target.
// The values are the ones the committed dry run uses
// (gostd/dryrun/go1.25.14/linux-amd64.json, tools/gostddryrun/tags.go); they
// are pinned here so this tool has no dependency on that tool's package main.
type tagSet struct {
	GOOS        string   `json:"goos"`
	GOARCH      string   `json:"goarch"`
	Compiler    string   `json:"compiler"`
	CgoEnabled  bool     `json:"cgo_enabled"`
	ReleaseTags []string `json:"release_tags"`
	ToolTags    []string `json:"tool_tags"`
	BuildTags   []string `json:"build_tags"`
}

func releaseTags(minorMinor int) []string {
	out := make([]string, 0, minorMinor)
	for i := 1; i <= minorMinor; i++ {
		out = append(out, fmt.Sprintf("go1.%d", i))
	}
	return out
}

func tagsFor(goos, goarch, version string) (*tagSet, error) {
	minor, err := goMinor(version)
	if err != nil {
		return nil, err
	}
	t := &tagSet{
		GOOS:       goos,
		GOARCH:     goarch,
		Compiler:   "gocpp",
		CgoEnabled: false,
		// The release tags come from the target's own VERSION, so the tool
		// does not need a table per release.
		ReleaseTags: releaseTags(minor),
		// go1.25's default experiments and architecture level, as
		// tools/gostddryrun/tags.go pins them (README §2.7).
		ToolTags: []string{
			"amd64.v1",
			"goexperiment.aliastypeparams",
			"goexperiment.dwarf5",
			"goexperiment.regabiargs",
			"goexperiment.regabiwrappers",
			"goexperiment.swissmap",
			"goexperiment.synchashtriemap",
		},
		BuildTags: []string{"math_big_pure_go", "purego"},
	}
	if goos == "linux" {
		t.BuildTags = append(t.BuildTags, "unix")
	}
	sort.Strings(t.BuildTags)
	if goarch != "amd64" {
		return nil, fmt.Errorf("no pinned tool tags for %s (only amd64)", goarch)
	}
	return t, nil
}

func goMinor(version string) (int, error) {
	// "go1.25.14" -> 25
	var major, minor, patch int
	if _, err := fmt.Sscanf(version, "go%d.%d.%d", &major, &minor, &patch); err != nil {
		return 0, fmt.Errorf("VERSION %q: %v", version, err)
	}
	if major != 1 {
		return 0, fmt.Errorf("unsupported Go major version in %q", version)
	}
	return minor, nil
}

func newContext(goroot string, t *tagSet) *build.Context {
	return &build.Context{
		GOROOT:      goroot,
		GOOS:        t.GOOS,
		GOARCH:      t.GOARCH,
		Compiler:    t.Compiler,
		CgoEnabled:  t.CgoEnabled,
		BuildTags:   append([]string(nil), t.BuildTags...),
		ToolTags:    append([]string(nil), t.ToolTags...),
		ReleaseTags: append([]string(nil), t.ReleaseTags...),
	}
}

// dirFiles is the selected file set of one package directory.
type dirFiles struct {
	goFiles []string
	sFiles  []string
}

func selectDir(ctxt *build.Context, dir string) (*dirFiles, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := &dirFiles{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		ok, err := ctxt.MatchFile(dir, name)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", filepath.Join(dir, name), err)
		}
		if !ok {
			continue
		}
		switch {
		case strings.HasSuffix(name, ".go"):
			out.goFiles = append(out.goFiles, name)
		case strings.HasSuffix(name, ".s"):
			out.sFiles = append(out.sFiles, name)
		}
	}
	sort.Strings(out.goFiles)
	sort.Strings(out.sFiles)
	return out, nil
}

// A location is one use of a stub identifier: where, and in which function.
type location struct {
	Package   string `json:"package"`
	File      string `json:"file"` // relative to GOROOT/src
	Line      int    `json:"line"`
	Col       int    `json:"col"`
	Enclosing string `json:"enclosing"` // "func F", "func (T) M", or "package scope"
	Form      string `json:"form"`      // call, value, type, method-call, method-value, qualified
}

// An identifier is one name the closure needs from a stub package.
type identifier struct {
	Name    string     `json:"name"`
	Kind    string     `json:"kind"` // func, type, var, const, method
	Object  string     `json:"object,omitempty"`
	Uses    []location `json:"uses"`
	Stubbed string     `json:"stub_note,omitempty"`
}

// A stubUse is the report for one stub package.
type stubUse struct {
	Package     string       `json:"package"`
	Identifiers []identifier `json:"identifiers"`
	Direct      []string     `json:"directly_imported_by"`
	// Bodyless are the stub package's own declarations that have no body in
	// Go's source (assembly or runtime pushes): the part a pure-Go stub has to
	// replace.
	Bodyless []bodylessDecl `json:"bodyless_in_go_source"`
}

type bodylessDecl struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Name string `json:"name"`
	Sig  string `json:"sig"`
	Note string `json:"note,omitempty"`
}

// A linkname is one //go:linkname directive of a closure package.
type linkname struct {
	Package string `json:"package"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Kind    string `json:"kind"` // pull, push, one-argument, variable, linker
	Local   string `json:"local"`
	Target  string `json:"target,omitempty"`
	Sig     string `json:"sig,omitempty"`
}

// A packageInfo is one node of the import graph.
type packageInfo struct {
	Path     string         `json:"path"`
	Name     string         `json:"name"`
	Dir      string         `json:"dir"` // relative to GOROOT/src
	Files    []string       `json:"files"`
	SFiles   []string       `json:"s_files,omitempty"`
	Imports  []string       `json:"imports"`
	StubDeps []string       `json:"stub_imports,omitempty"`
	Why      string         `json:"why"` // "" for a root, else "imported"
	Bodyless []bodylessDecl `json:"bodyless,omitempty"`
	Linkname []linkname     `json:"linkname,omitempty"`
}

// A runtimePush is a symbol the real runtime pushes into a closure package;
// the runtime stub has to provide it (as a native in 16.1f).
type runtimePush struct {
	Symbol string `json:"symbol"`       // package-qualified linker symbol
	Impl   string `json:"runtime_impl"` // the runtime's local function name
	Kind   string `json:"kind"`         // "push" (the runtime defines it) or "pull" (the runtime references the target package's own symbol)
	File   string `json:"file"`         // runtime source, relative to GOROOT/src
	Line   int    `json:"line"`
	Sig    string `json:"runtime_sig,omitempty"`
}

// A pull is a bodyless declaration of a closure package whose linker name is
// another package's symbol, and whose target package is outside the closure:
// the stub packages, the runtime stub or a package the engine must provide as
// a native/overlay (internal/runtime/atomic) has to define it.
type runtimePull struct {
	Package       string `json:"package"`
	Local         string `json:"local"`
	Target        string `json:"target"`
	TargetPackage string `json:"target_package"`
	Sig           string `json:"sig"`
	File          string `json:"file"`
	Line          int    `json:"line"`
}

// A native is a bodyless declaration outside the three stubs: the GC engine
// needs an overlay or a native for it (README §2.7), which is not this job's
// scope but is part of what the four packages need to build in the engine.
type native struct {
	Package  string `json:"package"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Name     string `json:"name"`
	Sig      string `json:"sig"`
	Provided string `json:"provided_by,omitempty"` // "runtime push", "" when none
}

type summary struct {
	Packages            int            `json:"packages"`
	PackagesByWhy       map[string]int `json:"packages_by_why"`
	Edges               int            `json:"import_edges"`
	StubIdentifiers     map[string]int `json:"stub_identifiers"`
	StubUsesTotal       map[string]int `json:"stub_uses_total"`
	LinknamesByKind     map[string]int `json:"linknames_by_kind"`
	RuntimePulls        int            `json:"runtime_pulls"`
	RuntimePushes       int            `json:"runtime_pushes_into_closure"`
	ExternalPackages    int            `json:"external_packages"`
	NativesOutsideStubs int            `json:"natives_outside_stubs"`
}

type result struct {
	Schema    int           `json:"schema"`
	Tool      string        `json:"tool"`
	GoVersion string        `json:"go_version"`
	GoMinor   string        `json:"go_minor"`
	Tags      *tagSet       `json:"tags"`
	Roots     []string      `json:"roots"`
	Stubs     []string      `json:"stubs"`
	Packages  []packageInfo `json:"packages"`
	StubUsage []stubUse     `json:"stub_usage"`
	Runtime   struct {
		PullsOutOfClosure []runtimePull `json:"pulls_out_of_closure"`
		Pushes            []runtimePush `json:"pushes_into_closure"`
	} `json:"runtime"`
	Natives []native `json:"natives_outside_stubs"`
	Summary summary  `json:"summary"`
	Notes   []string `json:"notes"`
}

func main() {
	goroot := flag.String("goroot", "", "Go source tree to read (default: go env GOROOT)")
	goos := flag.String("goos", "linux", "target GOOS")
	goarch := flag.String("goarch", "amd64", "target GOARCH")
	out := flag.String("out", "", "write the JSON here (default: stdout)")
	flag.Parse()

	if *goroot == "" {
		*goroot = runtime.GOROOT()
	}
	if *goroot == "" {
		fmt.Fprintln(os.Stderr, "closure: no GOROOT; pass -goroot")
		os.Exit(2)
	}
	res, err := run(*goroot, *goos, *goarch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "closure: %v\n", err)
		os.Exit(1)
	}
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "closure: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')
	if *out == "" {
		os.Stdout.Write(data)
		return
	}
	if err := os.WriteFile(*out, data, 0o666); err != nil {
		fmt.Fprintf(os.Stderr, "closure: %v\n", err)
		os.Exit(1)
	}
}

// analyser is one run.
type analyser struct {
	goroot    string
	version   string
	minor     int
	tags      *tagSet
	ctxt      *build.Context
	goVersion string
	fset      *token.FileSet
	stubSet   map[string]bool
	// selected files per resolved package path
	files map[string]*dirFiles
	// the closure in discovery order (roots first), and the direct imports
	order   []string
	imports map[string][]string
	// type check cache
	checked  map[string]*types.Package
	checking map[string]bool
	infos    map[string]*types.Info
	// uses: stub path -> identifier name -> *identifier
	uses map[string]map[string]*identifier
	// per-package source information
	linknames map[string][]linkname
	bodyless  map[string][]bodylessDecl
	// enclosing-function ranges, per file path
	funcRanges map[string][]funcRange
	// external marks the packages reached only as a stub package's own
	// dependency: type-checked, not reported in the closure.
	external map[string]bool
}

type funcRange struct {
	start, end token.Pos
	name       string
}

func run(goroot, goos, goarch string) (*result, error) {
	versionBytes, err := os.ReadFile(filepath.Join(goroot, "VERSION"))
	if err != nil {
		return nil, fmt.Errorf("read VERSION: %v", err)
	}
	version := strings.TrimSpace(string(versionBytes))
	minor, err := goMinor(version)
	if err != nil {
		return nil, err
	}
	tags, err := tagsFor(goos, goarch, version)
	if err != nil {
		return nil, err
	}
	a := &analyser{
		goroot:     goroot,
		version:    version,
		minor:      minor,
		tags:       tags,
		ctxt:       newContext(goroot, tags),
		goVersion:  fmt.Sprintf("go1.%d", minor),
		fset:       token.NewFileSet(),
		stubSet:    map[string]bool{},
		files:      map[string]*dirFiles{},
		imports:    map[string][]string{},
		checked:    map[string]*types.Package{},
		checking:   map[string]bool{},
		infos:      map[string]*types.Info{},
		uses:       map[string]map[string]*identifier{},
		linknames:  map[string][]linkname{},
		bodyless:   map[string][]bodylessDecl{},
		funcRanges: map[string][]funcRange{},
		external:   map[string]bool{},
	}
	for _, s := range stubs {
		a.stubSet[s] = true
		a.uses[s] = map[string]*identifier{}
	}

	// 1. Walk the import closure.
	for _, r := range roots {
		if err := a.walk(r); err != nil {
			return nil, err
		}
	}

	// 2. Parse and type-check every package, roots first so the dependency
	// order is stable, and collect the identifier uses.
	for _, p := range a.order {
		if err := a.check(p); err != nil {
			return nil, err
		}
	}

	// 3. Runtime pushes into closure packages, read from the runtime source.
	pushes, err := a.runtimePushes()
	if err != nil {
		return nil, err
	}

	res := a.result(pushes)
	return res, nil
}

// resolveImport maps an import of a std package to its path under GOROOT/src,
// or "" for unsafe/builtin/C. The closure of these four packages has no
// vendored imports.
func (a *analyser) resolveImport(imp string) string {
	switch imp {
	case "unsafe", "C", "builtin":
		return ""
	}
	dir := filepath.Join(a.goroot, "src", filepath.FromSlash(imp))
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		return imp
	}
	return ""
}

func (a *analyser) walk(path string) error {
	if _, ok := a.imports[path]; ok {
		return nil
	}
	files, err := selectDir(a.ctxt, filepath.Join(a.goroot, "src", filepath.FromSlash(path)))
	if err != nil {
		return err
	}
	if len(files.goFiles) == 0 {
		return fmt.Errorf("package %s has no Go file under the gocpp constraints", path)
	}
	a.files[path] = files
	a.imports[path] = nil
	a.order = append(a.order, path)
	if a.stubSet[path] {
		return nil
	}
	seen := map[string]bool{}
	for _, name := range files.goFiles {
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(a.goroot, "src", filepath.FromSlash(path), name), nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("%s/%s: %v", path, name, err)
		}
		for _, spec := range f.Imports {
			imp, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			resolved := a.resolveImport(imp)
			if resolved == "" {
				continue
			}
			if !seen[resolved] {
				seen[resolved] = true
				a.imports[path] = append(a.imports[path], resolved)
			}
		}
	}
	sort.Strings(a.imports[path])
	for _, imp := range a.imports[path] {
		if err := a.walk(imp); err != nil {
			return err
		}
	}
	return nil
}

// check parses, records directives and type-checks one package.
func (a *analyser) check(path string) error {
	if _, ok := a.checked[path]; ok {
		return nil
	}
	if a.checking[path] {
		return nil // the importer leaves the cycle to go/types
	}
	a.checking[path] = true
	defer delete(a.checking, path)

	files := a.files[path]
	dir := filepath.Join(a.goroot, "src", filepath.FromSlash(path))
	asts := make([]*ast.File, 0, len(files.goFiles))
	for _, name := range files.goFiles {
		full := filepath.Join(dir, name)
		f, err := parser.ParseFile(a.fset, full, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("%s: %v", path, err)
		}
		asts = append(asts, f)
		a.recordDirectives(path, full, f)
	}
	info := &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
		Scopes:     map[ast.Node]*types.Scope{},
	}
	conf := types.Config{
		GoVersion: a.goVersion,
		Sizes:     types.SizesFor("gc", a.tags.GOARCH),
		Importer:  sourceImporter{a: a, from: path},
		Error:     func(err error) { /* recorded by the caller via the package */ },
	}
	var typeErrs []error
	conf.Error = func(err error) { typeErrs = append(typeErrs, err) }
	pkg, err := conf.Check(path, a.fset, asts, info)
	if err != nil {
		return fmt.Errorf("type-check %s: %v (first errors: %v)", path, err, firstErrors(typeErrs, 5))
	}
	a.checked[path] = pkg
	a.infos[path] = info
	a.collect(path, asts, info)
	return nil
}

func firstErrors(errs []error, n int) []string {
	var out []string
	for i, err := range errs {
		if i >= n {
			break
		}
		out = append(out, err.Error())
	}
	return out
}

// sourceImporter checks imports from source under the gocpp constraints.
type sourceImporter struct {
	a    *analyser
	from string
}

func (i sourceImporter) Import(path string) (*types.Package, error) {
	if path == "unsafe" {
		return types.Unsafe, nil
	}
	resolved := i.a.resolveImport(path)
	if resolved == "" {
		return nil, fmt.Errorf("import %q of %s: not found in %s/src", path, i.from, i.a.goroot)
	}
	if _, ok := i.a.files[resolved]; !ok {
		if err := i.a.walkExternal(resolved); err != nil {
			return nil, err
		}
	}
	if err := i.a.check(resolved); err != nil {
		return nil, err
	}
	return i.a.checked[resolved], nil
}

// walkExternal records a package reached only through an import (the stub
// packages' own dependencies: runtime's internals), without adding it to the
// reported closure. It is still selected and parsed so the type check has the
// declarations.
func (a *analyser) walkExternal(path string) error {
	if _, ok := a.imports[path]; ok {
		return nil
	}
	files, err := selectDir(a.ctxt, filepath.Join(a.goroot, "src", filepath.FromSlash(path)))
	if err != nil {
		return err
	}
	if len(files.goFiles) == 0 {
		return fmt.Errorf("package %s has no Go file under the gocpp constraints", path)
	}
	a.files[path] = files
	a.imports[path] = nil
	a.order = append(a.order, path)
	a.external[path] = true
	seen := map[string]bool{}
	for _, name := range files.goFiles {
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(a.goroot, "src", filepath.FromSlash(path), name), nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range f.Imports {
			imp, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			r := a.resolveImport(imp)
			if r == "" || seen[r] {
				continue
			}
			seen[r] = true
			a.imports[path] = append(a.imports[path], r)
		}
	}
	sort.Strings(a.imports[path])
	for _, imp := range a.imports[path] {
		if err := a.walkExternal(imp); err != nil {
			return err
		}
	}
	return nil
}

var linknameRE = regexp.MustCompile(`^//go:linkname[ \t]+(\S+)(?:[ \t]+(\S+))?`)

// recordDirectives reads //go:linkname directives and bodyless declarations of
// one file.
func (a *analyser) recordDirectives(path, full string, f *ast.File) {
	rel, _ := filepath.Rel(filepath.Join(a.goroot, "src"), full)
	rel = filepath.ToSlash(rel)
	// Bodyless declarations, by name (funcs and vars), to classify directives
	// and to report what needs a native.
	bodylessFuncs := map[string]*ast.FuncDecl{}
	varNames := map[string]bool{}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Body == nil && d.Recv == nil {
				bodylessFuncs[d.Name.Name] = d
			}
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				for _, spec := range d.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for _, n := range vs.Names {
							varNames[n.Name] = true
						}
					}
				}
			}
		}
	}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body != nil || fd.Recv != nil {
			continue
		}
		pos := a.fset.Position(fd.Pos())
		a.bodyless[path] = append(a.bodyless[path], bodylessDecl{
			File: rel, Line: pos.Line, Name: fd.Name.Name,
			Sig: a.funcSig(fd),
		})
	}
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			m := linknameRE.FindStringSubmatch(strings.TrimRight(c.Text, "\r"))
			if m == nil {
				continue
			}
			pos := a.fset.Position(c.Pos())
			ln := linkname{Package: path, File: rel, Line: pos.Line, Local: m[1], Target: m[2]}
			switch {
			case m[2] == "":
				ln.Kind = "one-argument"
			case !strings.Contains(m[2], "."):
				ln.Kind = "linker"
			case bodylessFuncs[m[1]] != nil:
				ln.Kind = "pull"
				ln.Sig = a.funcSig(bodylessFuncs[m[1]])
			case varNames[m[1]]:
				ln.Kind = "variable"
			default:
				ln.Kind = "push"
				if d := findFunc(f, m[1]); d != nil {
					ln.Sig = a.funcSig(d)
				}
			}
			a.linknames[path] = append(a.linknames[path], ln)
		}
	}
	sort.Slice(a.linknames[path], func(i, j int) bool {
		x, y := a.linknames[path][i], a.linknames[path][j]
		if x.File != y.File {
			return x.File < y.File
		}
		return x.Line < y.Line
	})
}

func findFunc(f *ast.File, name string) *ast.FuncDecl {
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == name {
			return fd
		}
	}
	return nil
}

// funcSig prints "func name(params) results" without types.
func (a *analyser) funcSig(d *ast.FuncDecl) string {
	var b strings.Builder
	b.WriteString("func ")
	b.WriteString(d.Name.Name)
	b.WriteString("(")
	if d.Type.Params != nil {
		for i, p := range d.Type.Params.List {
			if i > 0 {
				b.WriteString(", ")
			}
			for j, n := range p.Names {
				if j > 0 {
					b.WriteString(", ")
				}
				b.WriteString(n.Name)
			}
			if len(p.Names) > 0 {
				b.WriteString(" ")
			}
			b.WriteString(exprString(p.Type))
		}
	}
	b.WriteString(")")
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		b.WriteString(" (")
		for i, p := range d.Type.Results.List {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(exprString(p.Type))
		}
		b.WriteString(")")
	}
	return b.String()
}

func exprString(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprString(e.X)
	case *ast.SelectorExpr:
		return exprString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprString(e.Elt)
	case *ast.Ellipsis:
		return "..." + exprString(e.Elt)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + exprString(e.Value)
	case *ast.MapType:
		return "map[" + exprString(e.Key) + "]" + exprString(e.Value)
	case *ast.IndexExpr:
		return exprString(e.X) + "[" + exprString(e.Index) + "]"
	case *ast.IndexListExpr:
		s := exprString(e.X) + "["
		for i, x := range e.Indices {
			if i > 0 {
				s += ", "
			}
			s += exprString(x)
		}
		return s + "]"
	default:
		return fmt.Sprintf("%T", e)
	}
}

// collect records the stub identifiers used by one package of the reported
// closure. The stub packages themselves and their own dependencies (runtime's
// internals, checked only so the importer has declarations) are skipped: they
// are not translated.
func (a *analyser) collect(path string, asts []*ast.File, info *types.Info) {
	if a.stubSet[path] || a.external[path] {
		return
	}
	for _, f := range asts {
		pos := a.fset.Position(f.Pos())
		rel, _ := filepath.Rel(filepath.Join(a.goroot, "src"), pos.Filename)
		rel = filepath.ToSlash(rel)
		a.buildFuncRanges(pos.Filename, f)
		file := rel
		// The selectors that are the Fun of a call, and the Sel idents of all
		// selectors (so the Ident branch does not double-record them).
		callFun := map[*ast.SelectorExpr]bool{}
		selSel := map[*ast.Ident]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
					callFun[sel] = true
				}
			case *ast.SelectorExpr:
				selSel[n.Sel] = true
			}
			return true
		})
		// Walk expression nodes; ident uses through types.Info.Uses.
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				// Package-qualified use (X is a stub package) or method
				// selection on a stub type.
				if id, ok := n.X.(*ast.Ident); ok {
					if pn, ok := info.Uses[id].(*types.PkgName); ok && a.stubSet[pn.Imported().Path()] {
						obj := info.Uses[n.Sel]
						if obj == nil {
							return true
						}
						form := "qualified"
						if callFun[n] {
							form = "call"
						}
						a.addUse(pn.Imported().Path(), obj, path, file, n.Sel.Pos(), form)
						return true
					}
				}
				if sel, ok := info.Selections[n]; ok {
					obj := sel.Obj()
					if obj != nil && obj.Pkg() != nil && a.stubSet[obj.Pkg().Path()] {
						form := "method-value"
						if callFun[n] {
							form = "method-call"
						}
						a.addUse(obj.Pkg().Path(), obj, path, file, n.Sel.Pos(), form)
					}
				}
			case *ast.Ident:
				obj := info.Uses[n]
				if obj == nil || obj.Pkg() == nil || !a.stubSet[obj.Pkg().Path()] {
					return true
				}
				// A package-qualified selector's Sel is also an Ident; it was
				// recorded above as a selector, skip its duplicate.
				if selSel[n] {
					return true
				}
				form := "value"
				if _, ok := obj.(*types.TypeName); ok {
					form = "type"
				}
				a.addUse(obj.Pkg().Path(), obj, path, file, n.Pos(), form)
			}
			return true
		})
	}
}

func (a *analyser) addUse(stubPkg string, obj types.Object, pkg, file string, pos token.Pos, form string) {
	name := obj.Name()
	kind := "value"
	switch obj.(type) {
	case *types.Func:
		sig, _ := obj.Type().(*types.Signature)
		if sig != nil && sig.Recv() != nil {
			kind = "method"
		} else {
			kind = "func"
		}
	case *types.TypeName:
		kind = "type"
	case *types.Var:
		kind = "var"
	case *types.Const:
		kind = "const"
	}
	m := a.uses[stubPkg]
	key := kind + " " + name
	id := m[key]
	if id == nil {
		id = &identifier{Name: name, Kind: kind, Object: types.ObjectString(obj, nil)}
		m[key] = id
	}
	p := a.fset.Position(pos)
	loc := location{
		Package:   pkg,
		File:      file,
		Line:      p.Line,
		Col:       p.Column,
		Enclosing: a.enclosing(p.Filename, pos),
		Form:      form,
	}
	for _, u := range id.Uses {
		if u.Package == loc.Package && u.File == loc.File && u.Line == loc.Line && u.Col == loc.Col {
			return
		}
	}
	id.Uses = append(id.Uses, loc)
}

func (a *analyser) buildFuncRanges(filename string, f *ast.File) {
	rs := make([]funcRange, 0, len(f.Decls))
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		name := "func " + fd.Name.Name
		if fd.Recv != nil && len(fd.Recv.List) > 0 {
			name = "func (" + exprString(fd.Recv.List[0].Type) + ") " + fd.Name.Name
		}
		rs = append(rs, funcRange{fd.Pos(), fd.End(), name})
	}
	a.funcRanges[filename] = rs
}

func (a *analyser) enclosing(filename string, pos token.Pos) string {
	best := ""
	bestLen := token.NoPos
	for _, r := range a.funcRanges[filename] {
		if r.start <= pos && pos < r.end {
			if best == "" || (r.end-r.start) < bestLen {
				best, bestLen = r.name, r.end-r.start
			}
		}
	}
	if best == "" {
		return "package scope"
	}
	return best
}

// runtimePushes finds the runtime's //go:linkname pushes into closure
// packages: definitions the runtime stub has to provide.
func (a *analyser) runtimePushes() ([]runtimePush, error) {
	closure := map[string]bool{}
	for _, p := range a.order {
		if !a.stubSet[p] && !a.external[p] {
			closure[p] = true
		}
	}
	dir := filepath.Join(a.goroot, "src", "runtime")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var out []runtimePush
	seen := map[string]bool{}
	for _, name := range names {
		full := filepath.Join(dir, name)
		ok, err := a.ctxt.MatchFile(dir, name)
		if err != nil || !ok {
			continue
		}
		f, err := parser.ParseFile(a.fset, full, nil, parser.ParseComments)
		if err != nil {
			continue
		}
		funcs := map[string]*ast.FuncDecl{}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil {
				funcs[fd.Name.Name] = fd
			}
		}
		kindOf := func(local string) string {
			if fd := funcs[local]; fd != nil && fd.Body == nil {
				return "pull"
			}
			return "push"
		}
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				m := linknameRE.FindStringSubmatch(strings.TrimRight(c.Text, "\r"))
				if m == nil || m[2] == "" {
					continue
				}
				dot := strings.LastIndex(m[2], ".")
				if dot < 0 {
					continue
				}
				targetPkg := m[2][:dot]
				if !closure[targetPkg] {
					continue
				}
				pos := a.fset.Position(c.Pos())
				rel, _ := filepath.Rel(filepath.Join(a.goroot, "src"), full)
				sig := ""
				if d := funcs[m[1]]; d != nil {
					sig = a.funcSig(d)
				}
				sym := m[2]
				if seen[sym] {
					continue
				}
				seen[sym] = true
				out = append(out, runtimePush{
					Symbol: sym, Impl: m[1], Kind: kindOf(m[1]), File: filepath.ToSlash(rel), Line: pos.Line, Sig: sig,
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out, nil
}

// result assembles the JSON.
func (a *analyser) result(pushes []runtimePush) *result {
	res := &result{
		Schema:    schema,
		Tool:      "gostd-spike-closure",
		GoVersion: a.version,
		GoMinor:   a.goVersion,
		Tags:      a.tags,
		Roots:     append([]string(nil), roots...),
		Stubs:     append([]string(nil), stubs...),
		Notes: []string{
			"The walk starts at the four roots and stops at runtime, internal/bytealg and internal/reflectlite: those three are replaced by gostd/spike/stub/, so their own imports are not translated.",
			"The tag set is the gocpp one of Phase 16 README 2.7 (compiler gocpp, purego, math_big_pure_go, CGO_ENABLED=0), not the gc tag set.",
			"A 'pull' linkname is a bodyless declaration of a closure package whose linker name is another package's symbol; 'push' has a body; 'one-argument' only exports the local symbol.",
			"runtime.pushes_into_closure are the definitions the runtime stub has to provide for closure packages (sync, internal/sync, iter, internal/synctest); in the GC engine they are natives.",
			"natives_outside_stubs are bodyless declarations of closure packages outside the three stubs (internal/abi intrinsics, sync/atomic assembly, sync/internal-sync/synctest runtime hooks); 16.1f does not write them, but the four packages need them in the engine.",
			"The source tree is identified by go_version only; the absolute path is deliberately not recorded, so a rerun from another copy of the same Go release is byte-identical.",
		},
	}
	rootSet := map[string]bool{}
	for _, r := range roots {
		rootSet[r] = true
	}
	for _, p := range a.order {
		if _, ok := a.files[p]; !ok {
			continue
		}
		if _, done := a.imports[p]; !done {
			continue
		}
		// Only the reported closure: neither the three stubs (replaced
		// wholesale, reported under stub_usage) nor their own dependencies
		// (runtime's internals).
		if a.isExternalOnly(p) || a.stubSet[p] {
			continue
		}
		files := a.files[p]
		pi := packageInfo{
			Path:     p,
			Dir:      filepath.ToSlash(filepath.Join("src", p)),
			Files:    append([]string(nil), files.goFiles...),
			SFiles:   append([]string(nil), files.sFiles...),
			Imports:  append([]string(nil), a.imports[p]...),
			Bodyless: a.bodyless[p],
			Linkname: a.linknames[p],
		}
		if !rootSet[p] {
			pi.Why = "imported"
		}
		for _, imp := range a.imports[p] {
			if a.stubSet[imp] {
				pi.StubDeps = append(pi.StubDeps, imp)
			}
		}
		// Name: parse once was enough; use the types package's name if checked.
		if tp, ok := a.checked[p]; ok && tp != nil {
			pi.Name = tp.Name()
		}
		res.Packages = append(res.Packages, pi)
	}
	sort.SliceStable(res.Packages, func(i, j int) bool { return res.Packages[i].Path < res.Packages[j].Path })

	// Stub usage.
	for _, s := range stubs {
		su := stubUse{Package: s}
		names := make([]string, 0, len(a.uses[s]))
		for n := range a.uses[s] {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			id := a.uses[s][n]
			sort.Slice(id.Uses, func(i, j int) bool {
				x, y := id.Uses[i], id.Uses[j]
				if x.Package != y.Package {
					return x.Package < y.Package
				}
				if x.File != y.File {
					return x.File < y.File
				}
				return x.Line < y.Line
			})
			su.Identifiers = append(su.Identifiers, *id)
		}
		// Direct importers and the stub package's own bodyless decls.
		for _, p := range a.order {
			if a.isExternalOnly(p) {
				continue
			}
			for _, imp := range a.imports[p] {
				if imp == s {
					su.Direct = append(su.Direct, p)
				}
			}
		}
		sort.Strings(su.Direct)
		su.Bodyless = a.stubBodyless(s)
		res.StubUsage = append(res.StubUsage, su)
	}

	// Runtime pulls and pushes.
	res.Runtime.PullsOutOfClosure = a.pullsOutOfClosure()
	res.Runtime.Pushes = pushes

	// Natives outside the stubs: total bodyless declarations per closure
	// package, with the runtime-push provider if there is one.
	providedBy := map[string]string{}
	for _, p := range pushes {
		if p.Kind == "pull" {
			// The runtime's own declaration is bodyless: the target package
			// (sync/atomic's assembly) defines the symbol and the runtime only
			// references it.
			providedBy[p.Symbol] = "the target package's own symbol (runtime references it as " + p.Impl + ")"
		} else {
			providedBy[p.Symbol] = "runtime push " + p.Impl
		}
	}
	for _, p := range a.order {
		if a.stubSet[p] || a.isExternalOnly(p) {
			continue
		}
		for _, b := range a.bodyless[p] {
			n := native{Package: p, File: b.File, Line: b.Line, Name: b.Name, Sig: b.Sig}
			if what, ok := providedBy[p+"."+b.Name]; ok {
				n.Provided = what
			}
			res.Natives = append(res.Natives, n)
		}
	}
	sort.SliceStable(res.Natives, func(i, j int) bool {
		x, y := res.Natives[i], res.Natives[j]
		if x.Package != y.Package {
			return x.Package < y.Package
		}
		if x.File != y.File {
			return x.File < y.File
		}
		return x.Line < y.Line
	})

	// Summary.
	s := summary{
		Packages:        len(res.Packages),
		PackagesByWhy:   map[string]int{},
		StubIdentifiers: map[string]int{},
		StubUsesTotal:   map[string]int{},
		LinknamesByKind: map[string]int{},
	}
	for _, p := range res.Packages {
		why := p.Why
		if why == "" {
			why = "root"
		}
		s.PackagesByWhy[why]++
		s.Edges += len(p.Imports)
		for _, l := range p.Linkname {
			s.LinknamesByKind[l.Kind]++
		}
	}
	for _, su := range res.StubUsage {
		s.StubIdentifiers[su.Package] = len(su.Identifiers)
		for _, id := range su.Identifiers {
			s.StubUsesTotal[su.Package] += len(id.Uses)
		}
	}
	external := 0
	for _, p := range a.order {
		if a.isExternalOnly(p) {
			external++
		}
	}
	s.ExternalPackages = external
	s.RuntimePulls = len(res.Runtime.PullsOutOfClosure)
	s.RuntimePushes = len(res.Runtime.Pushes)
	s.NativesOutsideStubs = len(res.Natives)
	res.Summary = s
	return res
}

// isExternalOnly reports whether a package was reached only as a dependency of
// a stub package (runtime's internal packages): it is type-checked but not
// part of the reported closure.
func (a *analyser) isExternalOnly(p string) bool {
	return a.external[p]
}

// stubBodyless lists the bodyless declarations of a stub package's real Go
// source, i.e. what a pure-Go stub replaces (assembly or runtime pushes).
func (a *analyser) stubBodyless(p string) []bodylessDecl {
	files := a.files[p]
	if files == nil {
		return nil
	}
	var out []bodylessDecl
	dir := filepath.Join(a.goroot, "src", filepath.FromSlash(p))
	for _, name := range files.goFiles {
		full := filepath.Join(dir, name)
		f, err := parser.ParseFile(token.NewFileSet(), full, nil, 0)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(filepath.Join(a.goroot, "src"), full)
		rel = filepath.ToSlash(rel)
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body != nil || fd.Recv != nil {
				continue
			}
			pos := a.fset.Position(fd.Pos())
			out = append(out, bodylessDecl{
				File: rel, Line: pos.Line, Name: fd.Name.Name,
				Sig:  a.funcSig(fd),
				Note: "implemented by assembly or a runtime push in Go's source; the stub replaces it",
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// pullsOutOfClosure lists the bodyless declarations of closure packages whose
// //go:linkname target is a package outside the closure (runtime, the three
// stubs, internal/runtime/atomic).
func (a *analyser) pullsOutOfClosure() []runtimePull {
	inClosure := map[string]bool{}
	for _, p := range a.order {
		if !a.stubSet[p] && !a.isExternalOnly(p) {
			inClosure[p] = true
		}
	}
	var out []runtimePull
	for _, p := range a.order {
		if a.stubSet[p] || a.isExternalOnly(p) {
			continue
		}
		for _, ln := range a.linknames[p] {
			if ln.Kind != "pull" {
				continue
			}
			dot := strings.LastIndex(ln.Target, ".")
			if dot < 0 || inClosure[ln.Target[:dot]] {
				continue
			}
			out = append(out, runtimePull{
				Package: p, Local: ln.Local, Target: ln.Target,
				TargetPackage: ln.Target[:dot], Sig: ln.Sig, File: ln.File, Line: ln.Line,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Target != out[j].Target {
			return out[i].Target < out[j].Target
		}
		if out[i].Package != out[j].Package {
			return out[i].Package < out[j].Package
		}
		return out[i].Line < out[j].Line
	})
	return out
}
