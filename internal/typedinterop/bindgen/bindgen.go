// Package bindgen writes a readable gocpp view of a C or C++ header (step
// 12.10 of Phase 12, "gocpp bindgen"): a Go-syntax .gp file of declarations
// that lists what the header offers, with the types a user would write.
//
// The view is made from clang's own answers, not from a parser of C: the
// preprocessor (-E -dD) says which macros the header defines and which files
// it includes, the JSON AST dump says what is declared, and the probe of step
// 12.2 says the type and the value of each macro. The output is a single
// package: gofmt-stable under "gocpp fmt", deterministic, and it checks with
// "gocpp check". docs/adr/092 has the design.
package bindgen

import (
	"context"
	"errors"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/format"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// ErrNoClang is the error of a run without clang++ (the fallback message of
// the other typed interop commands).
var ErrNoClang = probe.ErrNoClang

// A Config says what to translate and how.
type Config struct {
	// Header is the header as the user names it: "<raylib.h>" for a header
	// that the compiler finds in its include path, else the path of a file
	// (relative to Dir).
	Header string

	// Dir is the folder that relative paths are relative to, and the
	// compiler's working folder. Empty is the current folder.
	Dir string

	// Includes are -I folders, Defines -D macros ("NAME" or "NAME=value"),
	// Std the language standard (C++20 or newer; "c++20" when empty), Flags other compiler
	// flags (the output of pkg-config --cflags).
	Includes []string
	Defines  []string
	Std      string
	Flags    []string

	// Package is the name of the package in the output; empty takes it from
	// the header's file name, as an import of the header would.
	Package string

	// Include and Exclude are regular expressions (unanchored, as in go test
	// -run) for the C++ names (qualified: geo::Vec) of the declarations and
	// macros to write. Without Include, all are written except those that
	// match Exclude. A type that a written declaration uses is written
	// whatever the patterns say.
	Include []string
	Exclude []string

	// Scope says which of the included files are part of the view: ScopeAuto
	// when empty.
	Scope Scope

	// Clang is clang++; the zero value finds it (probe.FindClang).
	Clang build.CXX

	// CacheDir is the folder of the probe cache on disk (step 12.7) that the type and value of
	// the macros are kept in: "" is the environment's (~/.cache/gocpp/probe, off with
	// GOCPP_NOPROBECACHE=1), "off" none. The cache is keyed by the probe text, the flags and the
	// contents of every header the compiler read, so a header edit or a -D asks the compiler again.
	CacheDir string

	// Timeout limits each run of the compiler (DefaultTimeout when 0).
	Timeout time.Duration

	t       *tool
	cxx     build.CXX
	include []*regexp.Regexp
	exclude []*regexp.Regexp
	system  bool   // the header is <name>
	path    string // the absolute path of a header that is a file
}

// A Result is the view.
type Result struct {
	// Source is the .gp file, formatted.
	Source []byte
	// Package is its package name.
	Package string
	// Files are the files of the view (the header and what was taken with
	// it), absolute, in the order the preprocessor entered them.
	Files []string
	// Warnings are things the user should know (macros that were not
	// written are in the file itself).
	Warnings []string
	// Stats counts what was written.
	Stats Stats
}

// Stats counts the declarations of a view.
type Stats struct {
	Types, Funcs, Vars, Consts, Referenced, Macros, Skipped int
	Files                                                   int
	JSONBytes                                               int64

	// Where the time went: the preprocessor, the AST dump (clang and the
	// reader), the probe of the macros, and the rest.
	Preprocess, AST, Probe, Total time.Duration

	// ProbeCached says that the probe of the macros was answered by the cache, with no compiler run.
	ProbeCached bool
}

// prepare checks the Config and fills the derived fields.
func (c *Config) prepare(ctx context.Context) error {
	if c.Header == "" {
		return errors.New("no header")
	}
	if c.Dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		c.Dir = wd
	} else if abs, err := filepath.Abs(c.Dir); err == nil {
		c.Dir = abs
	}
	h := c.Header
	switch {
	case strings.HasPrefix(h, "<") && strings.HasSuffix(h, ">") && len(h) > 2:
		c.system = true
	case strings.HasPrefix(h, "<") || strings.HasSuffix(h, ">"):
		return fmt.Errorf("%q is not a header: write <name.h> for a system header or a path", h)
	default:
		p := strings.Trim(h, `"`)
		if !filepath.IsAbs(p) {
			p = filepath.Join(c.Dir, p)
		}
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			return fmt.Errorf("cannot find header %s", h)
		}
		c.path = p
	}
	if c.Scope == "" {
		c.Scope = ScopeAuto
	}
	switch c.Scope {
	case ScopeAuto, ScopeHeader, ScopeDir, ScopeTree:
	default:
		return fmt.Errorf("unknown scope %q (auto, header, dir or tree)", c.Scope)
	}
	if c.Std != "" && !stdRE.MatchString(c.Std) {
		return fmt.Errorf("-std=%s: the view is made in C++20 or newer (c++20, c++23, gnu++20, ...), the standard that gocpp compiles C++ in", c.Std)
	}
	var err error
	if c.include, err = compilePatterns(c.Include, "include"); err != nil {
		return err
	}
	if c.exclude, err = compilePatterns(c.Exclude, "exclude"); err != nil {
		return err
	}
	if c.Package == "" {
		if c.Package, err = c.defaultPackage(); err != nil {
			return err
		}
	} else if err := validPackage(c.Package); err != nil {
		return err
	}
	if c.Clang.Path != "" {
		c.cxx = c.Clang
	} else if c.cxx, err = probe.FindClang(ctx); err != nil {
		return err
	}
	return nil
}

// stdRE matches the language standards of clang that are C++20 or newer.
var stdRE = regexp.MustCompile(`^(c|gnu)\+\+(20|2a|23|2b|26|2c)$`)

func compilePatterns(list []string, what string) ([]*regexp.Regexp, error) {
	var out []*regexp.Regexp
	for _, p := range list {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("bad %s pattern %q: %v", what, p, err)
		}
		out = append(out, re)
	}
	return out, nil
}

// defaultPackage names the package after the header's file, as an import of
// the header would (docs/spec/interop.md, "Header packages and their names").
func (c *Config) defaultPackage() (string, error) {
	base := filepath.Base(strings.Trim(c.Header, `<>"`))
	if i := strings.LastIndexByte(base, '.'); i > 0 {
		base = base[:i]
	}
	name := goIdentRaw(base)
	if err := validPackage(name); err != nil {
		return "", fmt.Errorf("%v: choose a package name with -pkg", err)
	}
	return name, nil
}

// goIdentRaw replaces the characters that a Go name cannot have.
func goIdentRaw(s string) string {
	b := []byte(s)
	for i, c := range b {
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			b[i] = '_'
		}
	}
	return string(b)
}

// validPackage checks a package name: a Go name that is not a keyword or a
// predeclared name, and not C, init or _.
func validPackage(name string) error {
	switch {
	case name == "" || name[0] >= '0' && name[0] <= '9' || goIdentRaw(name) != name:
		return fmt.Errorf("%q is not a package name", name)
	case name == "C" || name == "init" || name == "_":
		return fmt.Errorf("%q cannot be a package name", name)
	case token.IsKeyword(name):
		return fmt.Errorf("%q is a Go keyword", name)
	case predeclared[name]:
		return fmt.Errorf("%q is a predeclared Go name", name)
	}
	return nil
}

var predeclared = map[string]bool{}

func init() {
	for _, n := range strings.Fields(`any bool byte comparable complex64 complex128 error float32 float64
		int int8 int16 int32 int64 rune string uint uint8 uint16 uint32 uint64 uintptr true false iota nil
		append cap clear close complex copy delete imag len make max min new panic print println real recover`) {
		predeclared[n] = true
	}
}

// includeText is the #include of the header as the compiler takes it.
func (c *Config) includeText() string {
	if c.system {
		return c.Header
	}
	return `"` + c.path + `"`
}

func (c *Config) includeLine() string { return "#include " + c.includeText() + "\n" }

// clangFlags are the flags of every compiler run.
func (c *Config) clangFlags() []string {
	var flags []string
	if c.Std != "" {
		flags = append(flags, "-std="+c.Std)
	}
	for _, d := range c.Includes {
		flags = append(flags, "-I", d)
	}
	for _, d := range c.Defines {
		flags = append(flags, "-D"+d)
	}
	return append(flags, c.Flags...)
}

// tool returns the compiler runner (the temporary folder is set by Generate).
func (c *Config) tool() *tool { return c.t }

// Generate writes the view of the header.
func Generate(ctx context.Context, cfg Config) (*Result, error) {
	c := &cfg
	if err := c.prepare(ctx); err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "gocpp-bindgen-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	tmp, _ = filepath.Abs(tmp)
	c.t = &tool{cxx: c.cxx, dir: c.Dir, flags: c.clangFlags(), tmp: tmp, timeout: c.Timeout}

	mainFile, err := c.t.source("main.cpp", c.includeLine())
	if err != nil {
		return nil, err
	}
	t0 := time.Now()
	roots := c.t.searchRoots(ctx)
	// 1. the preprocessor: the macros, and which files are in the view
	var pp *ppResult
	var ppErr error
	msgs, exit, err := c.t.run(ctx, []string{"-E", "-dD"}, mainFile, func(r readerT) error {
		pp, ppErr = parsePP(r, mainFile, c.Scope, c.Dir, roots)
		return ppErr
	})
	if err != nil {
		return nil, err
	}
	if exit != 0 {
		if e := headerError(c.Header, c.Dir, msgs); e != nil {
			return nil, e
		}
		return nil, fmt.Errorf("header %s doesn't compile%s", c.Header, excerpt(msgs))
	}
	if pp.Header == "" {
		return nil, fmt.Errorf("the compiler did not include %s", c.Header)
	}

	tPP := time.Since(t0)
	// 2. the declarations
	t1 := time.Now()
	tab := newTable()
	var jsonBytes int64
	msgs, exit, err = c.t.run(ctx, []string{"-fsyntax-only", "-Xclang", "-ast-dump=json"}, mainFile, func(r readerT) error {
		cr := &countingReader{r: r}
		defer func() { jsonBytes = cr.n }()
		return scanTU(cr, func(d *rawDecl) error { return collectDecl(tab, pp, d) })
	})
	if err != nil {
		return nil, err
	}
	if exit != 0 {
		if e := headerError(c.Header, c.Dir, msgs); e != nil {
			return nil, e
		}
		return nil, fmt.Errorf("header %s doesn't compile%s", c.Header, excerpt(msgs))
	}
	tab.link()
	tAST := time.Since(t1)

	g := newGen(c, tab, pp)
	res, err := g.generate(ctx)
	if err != nil {
		return nil, err
	}
	res.Files = slices.Clone(pp.Files)
	res.Stats.JSONBytes = jsonBytes
	res.Stats.Preprocess, res.Stats.AST, res.Stats.Total = tPP, tAST, time.Since(t0)
	res.Stats.Files = len(pp.Files)
	return res, nil
}

// collectDecl adds one top-level declaration of the dump to the table, when
// it matters: it is in a file of the view, or it is a type that another
// declaration could name.
func collectDecl(tab *table, pp *ppResult, d *rawDecl) error {
	if d.Skipped || d.Kind == "" {
		return nil
	}
	tr := fileTracker{file: d.File}
	file := tr.loc(quickLoc(d.Data))
	_, inScope := pp.Order[file]
	switch d.Kind {
	case "NamespaceDecl", "FunctionDecl", "VarDecl", "ClassTemplateDecl", "FunctionTemplateDecl",
		"TypeAliasTemplateDecl", "VarTemplateDecl":
		if !inScope {
			return nil
		}
	case "LinkageSpecDecl", "TypedefDecl", "TypeAliasDecl", "RecordDecl", "CXXRecordDecl", "EnumDecl":
	default:
		return nil
	}
	n, err := decodeNode(d.Data)
	if err != nil {
		return fmt.Errorf("decoding the AST: %w", err)
	}
	x := &addCtx{files: map[*jnode]string{}, isScope: func(f string) bool { _, ok := pp.Order[f]; return ok }}
	t2 := fileTracker{file: d.File}
	t2.walk(n, func(n *jnode, f string) { x.files[n] = f })
	tab.add(n, nil, x)
	return nil
}

// generate writes the output.
func (g *gen) generate(ctx context.Context) (*Result, error) {
	g.ctx = ctx
	g.classify()
	// the typedefs that will be stand-ins: their sizes are asked of the compiler together
	var standIns []*gdecl
	for _, d := range g.tab.all {
		if d.kind == "typedef" && g.visible(d) && hasStandInShape(d.node) {
			standIns = append(standIns, d)
		}
	}
	if len(standIns) > 0 {
		g.askSizes(standIns)
	}

	// macros and const variables: what the compiler says their values are
	var skipped []skippedMacro
	cands := macroCandidates(g.pp, g.cfg.allows, &skipped)
	enumNames := map[string]bool{}
	for _, d := range g.tab.all {
		if d.kind == "enum" && g.visible(d) {
			for _, e := range enumValues(d.node) {
				enumNames[e.Name] = true
			}
		}
	}
	var kept []*macro
	for _, m := range cands {
		if enumNames[m.Name] {
			// #define FP_NAN FP_NAN is the enumerator already (macroCandidates); another body
			// would make two constants of the one name
			skipped = append(skipped, skippedMacro{m.Name, "", "has the name of an enumerator"})
			continue
		}
		kept = append(kept, m)
	}
	cands = kept
	g.skippedMacros = skipped
	var constVars []*gdecl
	for _, d := range g.tab.all {
		if d.kind == "var" && g.visible(d) && isConstScalar(d.node) {
			constVars = append(constVars, d)
		}
	}
	tp := time.Now()
	cs, err := g.probeConstants(ctx, cands, constVars)
	if err != nil {
		return nil, err
	}
	g.probeTime = time.Since(tp)
	slices.SortFunc(cs.consts, func(a, b *constant) int { return a.Seq - b.Seq })

	g.nameAll(cs.consts)
	g.renderStructMacros(cs)
	slices.SortFunc(cs.structs, func(a, b *structMacro) int { return a.Seq - b.Seq })
	for _, s := range cs.structs {
		s.Name = g.fresh(s.Name)
	}

	var st Stats
	// main pass
	var constB, typeB, varB, funcB, extraB strings.Builder
	for _, d := range g.tab.all {
		if !g.visible(d) {
			continue
		}
		switch d.kind {
		case "record", "enum":
			if d.name == "" {
				if d.kind == "enum" && d.complete {
					g.writeEnum(&typeB, d)
					st.Types++
				}
				continue
			}
			g.writeType(&typeB, d)
			st.Types++
		case "typedef":
			if g.isStandard(d) {
				continue // size_t, uint8_t, va_list: written as Go types wherever they are used
			}
			g.writeType(&typeB, d)
			st.Types++
		case "var":
			if d.done {
				continue
			}
			g.writeVar(&varB, d)
			st.Vars++
		case "func":
			g.writeFunc(&funcB, d)
			st.Funcs++
		case "template":
			g.writeTemplate(&extraB, d)
		}
	}
	// constants
	if len(cs.consts) > 0 {
		constB.WriteString("const (\n")
		for _, c := range cs.consts {
			if c.Type != "" {
				fmt.Fprintf(&constB, "\t%s %s = %s\n", c.Go, c.Type, c.Lit)
			} else {
				fmt.Fprintf(&constB, "\t%s = %s\n", c.Go, c.Lit)
			}
			st.Consts++
		}
		constB.WriteString(")\n\n")
	}
	if len(cs.structs) > 0 {
		fmt.Fprintf(&varB, "var (\n")
		for _, s := range cs.structs {
			fmt.Fprintf(&varB, "\t%s = %s\n", s.Name, s.Expr)
		}
		varB.WriteString(")\n\n")
	}
	st.Macros = len(cs.consts) + len(cs.structs)
	// referenced declarations
	for len(g.pullQ) > 0 {
		d := g.pullQ[0]
		g.pullQ = g.pullQ[1:]
		if d.done {
			continue
		}
		var b strings.Builder
		g.writeType(&b, d)
		if b.Len() > 0 {
			g.pulled[d.goName] = b.String()
		}
	}
	var refB strings.Builder
	for _, name := range sortedKeys(g.pulled) {
		refB.WriteString(g.pulled[name])
		st.Referenced++
	}
	// stand-ins for types that no header in view declares
	opaque := slices.Clone(g.opaqueList)
	slices.SortFunc(opaque, func(a, b string) int { return strings.Compare(g.opaque[a], g.opaque[b]) })
	for _, sp := range opaque {
		fmt.Fprintf(&refB, "// C++: %s\ntype %s struct{}\n\n", sp, g.opaque[sp])
	}
	// what was left out
	if len(g.layout) > 0 {
		names := slices.Clone(g.layout)
		slices.Sort(names)
		names = slices.Compact(names)
		extraB.WriteString("// layout differs (these records are packed or over-aligned; the view does not say how):\n")
		extraB.WriteString("//\t" + strings.Join(names, ", ") + "\n\n")
	}
	g.listSkipped(&extraB, cs.skipped)
	st.Skipped = len(cs.skipped)

	var out strings.Builder
	fmt.Fprintf(&out, "// Code generated by gocpp bindgen from %s. DO NOT EDIT.\n\npackage %s\n\n", g.cfg.Header, g.cfg.Package)
	if g.useUnsafe {
		out.WriteString("import \"unsafe\"\n\n")
	}
	section := func(title string, b *strings.Builder) {
		if b.Len() == 0 {
			return
		}
		fmt.Fprintf(&out, "// %s\n\n%s", title, b.String())
	}
	section("Constants", &constB)
	section("Types", &typeB)
	section("Referenced types: declared in other headers, or private to this one", &refB)
	section("Variables", &varB)
	section("Functions", &funcB)
	section("Not translated", &extraB)

	src, err := format.Source(g.cfg.Package+".gp", []byte(out.String()))
	if err != nil {
		return nil, fmt.Errorf("internal error: the generated file does not parse: %w\n%s", err, out.String())
	}
	st.Probe, st.ProbeCached = g.probeTime, g.probeCached
	return &Result{Source: src, Package: g.cfg.Package, Warnings: g.warnings, Stats: st}, nil
}

// listSkipped writes the macros that have no translation.
func (g *gen) listSkipped(b *strings.Builder, skipped []skippedMacro) {
	if len(skipped) == 0 {
		return
	}
	by := map[string][]string{}
	for _, s := range skipped {
		by[s.Why] = append(by[s.Why], s.Name+s.Params)
	}
	b.WriteString("// macros:\n")
	for _, why := range sortedKeys(by) {
		names := by[why]
		slices.Sort(names)
		// the names wrap at 100 columns, on lines indented one more tab
		fmt.Fprintf(b, "//\t%s:\n", why)
		line := "//\t\t"
		for i, n := range names {
			if i < len(names)-1 {
				n += ","
			}
			if len(line) > len("//\t\t") && len(line)+1+len(n) > 100 {
				b.WriteString(line + "\n")
				line = "//\t\t"
			}
			if len(line) > len("//\t\t") {
				line += " "
			}
			line += n
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
}

// isConstScalar says that a variable is declared const (or constexpr), has a
// number or boolean type, and has an initializer: a candidate for a Go
// constant.
func isConstScalar(n *jnode) bool {
	if n.Init == "" && len(n.Inner) == 0 {
		return false
	}
	t, err := ctype.Parse(n.qual())
	if err != nil || !t.Const || t.Kind != ctype.Builtin {
		return false
	}
	_, ok := builtinGo[t.Name]
	return ok
}
