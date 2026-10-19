// Stubgen writes the declaration stubs of the standard library in stdlib/
// (Phase 01 README §2.6): for each package in the list below, one .gp file
// with Go's exported declarations and their doc comments, but without the
// function bodies and the values of the variables. They let go/types check
// programs that import these packages before gocpp has the real packages
// (Phases 05 to 09).
//
// Usage, from the repository root, in the dev shell (nix develop):
//
//	go run ./tools/stubgen [-goroot dir] [-o stdlib]
//
// The declarations come from the Go source in GOROOT/src (the dev shell's Go,
// 1.25.14 now), with the files that go/build picks for linux/amd64 without
// cgo. The tool type-checks each package with go/types and then writes:
//
//   - every exported const with its type and its exact value: Go's own
//     expression when the stub has every name in it (1000 * Nanosecond),
//     otherwise the computed value (O_RDONLY int = 0, and the iota ones);
//   - every exported var with its type and without its value;
//   - every exported type, and the unexported types that an exported
//     struct embeds (the helpers: testing.T embeds common, which has
//     Errorf), so that selectors find promoted fields and methods at the
//     same depth as in Go. A struct keeps only its exported, blank and
//     embedded fields; when Go's struct has more, a blank field _ stands for
//     them, so the struct can't be compared when Go's can't, and no other
//     struct is identical to it. An interface is complete, also its
//     unexported methods (for example testing.TB's private method);
//   - every exported func, and every exported method of these types,
//     without its body. A generic function gets the body panic("stub"),
//     because go/types reports "generic function is missing function body"
//     for one without a body;
//   - an unexported method only when an interface of the package has a
//     method with that name (testing.common's private).
//
// A declaration that needs a package without a stub (syscall, reflect,
// internal/...) or an unexported name is left out, and so is everything
// that needs a left-out declaration. A left-out struct field doesn't take
// its struct with it. Each file lists what was left out in its header.
//
// The tool overwrites stdlib/<path>/<name>.gp for each package and touches
// nothing else. Its test checks that the files in stdlib/ are what it writes
// now, and that every declaration in them has the same type as in Go.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/constant"
	"go/format"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// A stubPackage is one package that gets a stub.
type stubPackage struct {
	Path  string // its import path
	Phase int    // the phase that replaces the stub with the real package (with Intrinsic: maps it)
	Later string // the sentence about the later phases, when the stub goes in two steps

	// Intrinsic: the compiler maps the package to the runtime type Intrinsic (weak:
	// gp::weak_ptr) from Phase, and the stub stays, for go/types only.
	Intrinsic string

	// Real: the package is real now (errors, since Phase 05 step 5.7, by hand, and by tools/portstd since 9.4a;
	// sync and sync/atomic, since Phase 08 step 8.7, by hand; unicode/utf8, since Phase 09 step 9.1, and unicode
	// and strconv, since 9.4a, by tools/portstd): stdlib/<path>
	// holds its .gp files, and the tool writes no stub there. The
	// tool still makes the stub in memory, because the tests compare the real package's exported
	// declarations with it (TestRealPackagesMatchStub).
	Real bool

	// Missing (for a Real package): the exported names of Go's package that the real package
	// does not have yet. TestRealPackagesMatchStub does not ask for them, and every other
	// difference is still an error. Its doc comment in stdlib/<path>/PORT.md says why.
	Missing []string
}

// packages are the packages that get a stub: every standard package that a
// target program in examples/ imports (also in its _test.gp files and its
// own packages), plus the packages that their declarations need (io/fs and
// time for os, unicode for strings, context for testing, encoding for
// flag), and weak, which the compiler maps to the runtime's gp::weak_ptr (a
// Phase 03 intrinsic). The phases come from the plan: weak in Phase 03,
// errors and a basic fmt in Phase 05,
// cmp, iter, maps and slices in Phase 07, sync, sync/atomic and time in
// Phase 08 (its README §2.5), the rest in Phase 09. They are in dependency
// order: a package comes after the packages that its declarations use.
var packages = []stubPackage{
	{Path: "weak", Phase: 3, Intrinsic: "gp::weak_ptr"},
	{Path: "errors", Phase: 5, Real: true},
	{Path: "runtime", Phase: 8, Real: true, Missing: runtimeMissing},
	{Path: "cmp", Phase: 7},
	{Path: "iter", Phase: 7},
	{Path: "unicode", Phase: 9, Real: true},
	{Path: "unicode/utf8", Phase: 9, Real: true},
	{Path: "encoding", Phase: 9},
	{Path: "io", Phase: 9},
	{Path: "time", Phase: 8, Real: true},
	{Path: "io/fs", Phase: 9},
	{Path: "context", Phase: 9},
	{Path: "fmt", Phase: 5, Later: "Phase 05 replaces it with a basic fmt, and Phase 09 with Go's."},
	{Path: "maps", Phase: 7},
	{Path: "slices", Phase: 7},
	{Path: "strconv", Phase: 9, Real: true},
	{Path: "strings", Phase: 9},
	{Path: "bytes", Phase: 9},
	{Path: "bufio", Phase: 9},
	{Path: "sync", Phase: 8, Real: true},
	{Path: "sync/atomic", Phase: 8, Real: true},
	{Path: "os", Phase: 9},
	{Path: "runtime/debug", Phase: 8, Real: true, Missing: debugMissing},
	{Path: "flag", Phase: 9},
	{Path: "regexp", Phase: 9},
	{Path: "encoding/json", Phase: 9},
	{Path: "testing", Phase: 9},
}

// runtimeMissing and debugMissing: what the real packages runtime and runtime/debug (Phase 08
// step 8.10) leave out; stdlib/runtime/PORT.md says why.
var runtimeMissing = []string{
	"AddCleanup", "BlockProfile", "BlockProfileRecord", "Breakpoint", "CPUProfile", "Cleanup",
	"GOTOOLDIR", "GoroutineProfile", "MemProfile", "MemProfileRecord", "MutexProfile", "ReadTrace",
	"SetCgoTraceback", "StackRecord", "StartTrace", "StopTrace", "ThreadCreateProfile",
}
var debugMissing = []string{
	"BuildInfo", "BuildSetting", "CrashOptions", "GCStats", "Module", "ParseBuildInfo", "ReadBuildInfo",
	"ReadGCStats", "SetCrashOutput", "WriteHeapDump",
}

func main() {
	goroot := flag.String("goroot", "", "the Go tree to read (default: go env GOROOT)")
	out := flag.String("o", "stdlib", "the folder to write the stubs into")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/stubgen [-goroot dir] [-o stdlib]")
		os.Exit(2)
	}
	if *goroot == "" {
		b, err := exec.Command("go", "env", "GOROOT").Output()
		if err != nil {
			fmt.Fprintln(os.Stderr, "stubgen: go env GOROOT:", err)
			os.Exit(1)
		}
		*goroot = strings.TrimSpace(string(b))
	}
	stubs, err := generate(*goroot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stubgen:", err)
		os.Exit(1)
	}
	files := stubs.files
	for _, p := range packages {
		if p.Real {
			delete(files, stubFile(p.Path)) // the real package has its own files
		}
	}
	for _, name := range sortedKeys(files) {
		p := filepath.Join(*out, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "stubgen:", err)
			os.Exit(1)
		}
		if err := os.WriteFile(p, files[name], 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "stubgen:", err)
			os.Exit(1)
		}
	}
}

// stubFile returns the name of the stub file of the package path, relative
// to stdlib/: encoding/json/json.gp.
func stubFile(p string) string {
	return p + "/" + path.Base(p) + ".gp"
}

// goVersion returns the Go version of the tree goroot, from its VERSION
// file: go1.25.14.
func goVersion(goroot string) (string, error) {
	b, err := os.ReadFile(filepath.Join(goroot, "VERSION"))
	if err != nil {
		return "", err
	}
	v, _, _ := strings.Cut(string(b), "\n")
	return strings.TrimSpace(v), nil
}

// stubs are what generate makes.
type stubs struct {
	version string              // the Go version of the source: go1.25.14
	files   map[string][]byte   // the stub files, by their names relative to stdlib/ (see stubFile)
	leftOut map[string][]string // by package path: what its stub leaves out (see pkgState.leftOut)
}

// generate makes the stubs of all packages from the Go tree goroot.
func generate(goroot string) (*stubs, error) {
	version, err := goVersion(goroot)
	if err != nil {
		return nil, err
	}
	ctxt, restore := useContext(goroot)
	defer restore()

	g := &generator{
		ctxt:    ctxt,
		version: version,
		fset:    token.NewFileSet(),
		inSet:   make(map[string]bool),
		dropped: make(map[string]bool),
	}
	g.imp = importer.ForCompiler(g.fset, "source", nil).(types.ImporterFrom)
	for _, p := range packages {
		g.inSet[p.Path] = true
	}
	st := &stubs{version: version, files: make(map[string][]byte), leftOut: make(map[string][]string)}
	for _, p := range packages {
		src, left, err := g.stub(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", p.Path, err)
		}
		st.files[stubFile(p.Path)] = src
		st.leftOut[p.Path] = left
	}
	return st, nil
}

// useContext sets build.Default to the tree goroot and the target of the
// stubs, linux/amd64 without cgo, and returns it and a func that sets
// build.Default back. The source importer of go/importer reads
// build.Default, so it must see the same tree and target.
func useContext(goroot string) (*build.Context, func()) {
	saved := build.Default
	ctxt := build.Default
	ctxt.GOROOT = goroot
	ctxt.GOOS, ctxt.GOARCH = "linux", "amd64"
	ctxt.CgoEnabled = false
	ctxt.GOPATH = ""
	build.Default = ctxt
	return &ctxt, func() { build.Default = saved }
}

// A generator holds what the stubs of all packages share.
type generator struct {
	ctxt    *build.Context
	version string
	fset    *token.FileSet
	imp     types.ImporterFrom

	// inSet holds the paths of the packages that get a stub.
	inSet map[string]bool

	// dropped holds the package-level declarations that a stub left out,
	// as "path.Name", so that a later package leaves out what needs them.
	dropped map[string]bool
}

// An item is one declaration that a stub may contain.
type item struct {
	kind    itemKind
	name    string // Name, or Type.Method for a method
	key     string // path.Name for a package-level declaration
	doc     *ast.CommentGroup
	refs    refs
	dropped bool

	// For a const or a var.
	obj       types.Object
	value     ast.Expr // the value as Go writes it, or nil when Go repeats the last one or uses iota
	valueRefs refs     // what value needs

	// For a type.
	spec   *ast.TypeSpec
	fields []*fieldItem // the exported and blank fields, for a struct
	extra  bool         // the struct has more fields than fields (a _ field stands for them)
	tnamed *types.TypeName

	// For a func or a method from the source.
	decl *ast.FuncDecl
	recv string // the receiver's type name, for a method
}

type itemKind int

const (
	constItem itemKind = iota
	varItem
	typeItem
	funcItem
	methodItem
)

// A fieldItem is one exported or blank field of a struct, or one such name
// of a field that names several.
type fieldItem struct {
	name    string // "" for an embedded field
	field   *ast.Field
	refs    refs
	dropped bool
}

// refs are what a declaration needs: the package-level declarations, by
// key. bad says why the declaration can't be in the stub at all.
type refs struct {
	keys []string
	bad  string
}

// A group is one const or var declaration of Go's source, which can hold
// several items.
type group struct {
	tok     token.Token
	doc     *ast.CommentGroup
	grouped bool
	specs   []*specItems
}

// specItems are the items of one spec of a group.
type specItems struct {
	doc     *ast.CommentGroup
	comment *ast.CommentGroup
	items   []*item
}

// pkgState is the state of the stub of one package.
type pkgState struct {
	*generator
	path    string
	pkg     *types.Package
	info    *types.Info
	files   []*ast.File
	docs    map[token.Pos]*ast.FuncDecl // the funcs and methods, by the position of their name
	items   []*item
	byKey   map[string]*item
	private map[string]bool // the unexported method names of the package's interfaces
	helpers map[string]bool // the unexported types that the stub keeps (see findHelpers)

	// out holds the declarations in source order: *group, or *item for a
	// type, func or method.
	out []any

	imports map[string]string // the imports the stub uses: path → name
}

// stub returns the stub file of the package p and what it leaves out.
func (g *generator) stub(p stubPackage) ([]byte, []string, error) {
	s, err := g.load(p.Path)
	if err != nil {
		return nil, nil, err
	}
	if err := s.collect(); err != nil {
		return nil, nil, err
	}
	s.drop()
	for _, it := range s.items {
		if it.dropped && it.key != "" {
			g.dropped[it.key] = true
		}
	}
	src, err := s.write(p)
	return src, s.leftOut(), err
}

// load parses and type-checks the package path from GOROOT/src.
func (g *generator) load(p string) (*pkgState, error) {
	bp, err := g.ctxt.Import(p, "", 0)
	if err != nil {
		return nil, err
	}
	s := &pkgState{
		generator: g,
		path:      p,
		docs:      make(map[token.Pos]*ast.FuncDecl),
		byKey:     make(map[string]*item),
		private:   make(map[string]bool),
		helpers:   make(map[string]bool),
		imports:   make(map[string]string),
		info: &types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
			Defs:  make(map[*ast.Ident]types.Object),
			Uses:  make(map[*ast.Ident]types.Object),
		},
	}
	for _, name := range bp.GoFiles {
		f, err := parser.ParseFile(g.fset, filepath.Join(bp.Dir, name), nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		s.files = append(s.files, f)
	}
	conf := types.Config{Importer: g.imp, IgnoreFuncBodies: true, GoVersion: "go1.25"}
	s.pkg, err = conf.Check(p, g.fset, s.files, s.info)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// collect makes the items of the exported declarations, in source order.
func (s *pkgState) collect() error {
	if err := s.findHelpers(); err != nil {
		return err
	}
	// The unexported method names of the package's exported interfaces.
	for _, name := range s.pkg.Scope().Names() {
		if tn, ok := s.pkg.Scope().Lookup(name).(*types.TypeName); ok && tn.Exported() {
			if it, ok := tn.Type().Underlying().(*types.Interface); ok {
				for m := range it.Methods() {
					if !m.Exported() {
						s.private[m.Name()] = true
					}
				}
			}
		}
	}
	for _, f := range s.files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				s.docs[fd.Name.Pos()] = fd
			}
		}
	}
	for _, f := range s.files {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if err := s.collectFunc(d); err != nil {
					return err
				}
			case *ast.GenDecl:
				switch d.Tok {
				case token.CONST, token.VAR:
					s.collectValues(d)
				case token.TYPE:
					for _, spec := range d.Specs {
						ts := spec.(*ast.TypeSpec)
						doc := ts.Doc
						if doc == nil && !d.Lparen.IsValid() {
							doc = d.Doc
						}
						s.collectType(ts, doc)
					}
				}
			}
		}
	}
	return nil
}

func (s *pkgState) add(it *item) {
	s.items = append(s.items, it)
	if it.key != "" {
		s.byKey[it.key] = it
	}
}

// collectValues collects the exported consts or vars of d as one group.
func (s *pkgState) collectValues(d *ast.GenDecl) {
	g := &group{tok: d.Tok, doc: d.Doc, grouped: d.Lparen.IsValid()}
	for _, spec := range d.Specs {
		vs := spec.(*ast.ValueSpec)
		si := &specItems{doc: vs.Doc, comment: vs.Comment}
		for i, id := range vs.Names {
			if !id.IsExported() {
				continue
			}
			obj := s.info.Defs[id]
			it := &item{name: id.Name, key: s.path + "." + id.Name, obj: obj}
			if d.Tok == token.CONST {
				it.kind = constItem
				if i < len(vs.Values) && !usesIota(vs.Values[i]) {
					it.value = vs.Values[i]
					it.valueRefs = s.exprRefs(it.value)
				}
			} else {
				it.kind = varItem
			}
			it.refs = s.typeRefs(obj.Type())
			s.add(it)
			si.items = append(si.items, it)
		}
		if len(si.items) > 0 {
			g.specs = append(g.specs, si)
		}
	}
	if len(g.specs) > 0 {
		s.out = append(s.out, g)
	}
}

// collectType collects the type ts, if it is exported or a helper.
func (s *pkgState) collectType(ts *ast.TypeSpec, doc *ast.CommentGroup) {
	if !ts.Name.IsExported() && !s.helpers[ts.Name.Name] {
		return
	}
	tn := s.info.Defs[ts.Name].(*types.TypeName)
	it := &item{kind: typeItem, name: ts.Name.Name, key: s.path + "." + ts.Name.Name, doc: doc, spec: ts, tnamed: tn}
	if ts.TypeParams != nil {
		it.refs = s.exprRefs(ts.TypeParams)
	}
	st, isStruct := ts.Type.(*ast.StructType)
	if !isStruct || ts.Assign.IsValid() {
		it.refs.merge(s.exprRefs(ts.Type))
		s.add(it)
		s.out = append(s.out, it)
		return
	}
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			// An embedded field stays: an unexported one is a helper
			// (see findHelpers).
			it.fields = append(it.fields, &fieldItem{field: f, refs: s.exprRefs(f.Type)})
			continue
		}
		for _, id := range f.Names {
			// A blank field stays when its type can: it is there to
			// make the struct not comparable or not convertible
			// (atomic.Pointer's _ [0]*T).
			if id.IsExported() || id.Name == "_" {
				it.fields = append(it.fields, &fieldItem{name: id.Name, field: f, refs: s.exprRefs(f.Type)})
			} else {
				it.extra = true
			}
		}
	}
	s.add(it)
	s.out = append(s.out, it)
}

// findHelpers finds the helpers: the unexported types that an exported
// struct type embeds, or that a helper embeds. The stub keeps them like
// exported types, so that a selector finds the promoted fields and
// methods at the same depth as in Go (testing.T embeds common, which has
// Errorf). It fails if such a type is not a type of the package.
func (s *pkgState) findHelpers() error {
	specs := make(map[string]*ast.TypeSpec)
	for _, f := range s.files {
		for _, d := range f.Decls {
			if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
				for _, spec := range gd.Specs {
					ts := spec.(*ast.TypeSpec)
					specs[ts.Name.Name] = ts
				}
			}
		}
	}
	var visit func(ts *ast.TypeSpec) error
	visit = func(ts *ast.TypeSpec) error {
		st, ok := ts.Type.(*ast.StructType)
		if !ok || ts.Assign.IsValid() {
			return nil
		}
		for _, f := range st.Fields.List {
			name := embeddedName(f.Type)
			if len(f.Names) > 0 || ast.IsExported(name) || s.helpers[name] {
				continue
			}
			x := f.Type
			if star, ok := x.(*ast.StarExpr); ok {
				x = star.X
			}
			hs := specs[name]
			if _, ok := x.(*ast.Ident); !ok || hs == nil {
				return fmt.Errorf("%s embeds %s, which is not a type of the package", ts.Name.Name, types.ExprString(f.Type))
			}
			s.helpers[name] = true
			if err := visit(hs); err != nil {
				return err
			}
		}
		return nil
	}
	for _, name := range sortedKeys(specs) {
		if ast.IsExported(name) {
			if err := visit(specs[name]); err != nil {
				return err
			}
		}
	}
	return nil
}

// embeddedName returns the field name of an embedded field of type x.
func embeddedName(x ast.Expr) string {
	for {
		switch e := x.(type) {
		case *ast.StarExpr:
			x = e.X
		case *ast.IndexExpr:
			x = e.X
		case *ast.IndexListExpr:
			x = e.X
		case *ast.SelectorExpr:
			return e.Sel.Name
		case *ast.Ident:
			return e.Name
		default:
			return ""
		}
	}
}

// collectFunc collects the func or method d, if the stub has it: an
// exported func, or an exported method of an exported type or a helper
// (or an unexported one that an interface needs, see pkgState.private).
func (s *pkgState) collectFunc(d *ast.FuncDecl) error {
	if d.Recv == nil {
		if !d.Name.IsExported() {
			return nil
		}
		it := &item{kind: funcItem, name: d.Name.Name, key: s.path + "." + d.Name.Name, doc: d.Doc, decl: d}
		it.refs = s.exprRefs(d.Type)
		s.add(it)
		s.out = append(s.out, it)
		return nil
	}
	recv := embeddedName(d.Recv.List[0].Type)
	if (!ast.IsExported(recv) && !s.helpers[recv]) || (!d.Name.IsExported() && !s.private[d.Name.Name]) {
		return nil
	}
	it := &item{kind: methodItem, name: recv + "." + d.Name.Name, doc: d.Doc, decl: d, recv: recv}
	it.refs = s.exprRefs(d.Type)
	// The receiver's type parameters are new names; its base type is
	// recv itself.
	it.refs.keys = append(it.refs.keys, s.path+"."+recv)
	s.add(it)
	s.out = append(s.out, it)
	return nil
}

func (r *refs) merge(o refs) {
	r.keys = append(r.keys, o.keys...)
	if r.bad == "" {
		r.bad = o.bad
	}
}

// objRef records that a declaration needs the object obj.
func (s *pkgState) objRef(r *refs, obj types.Object) {
	if obj == nil || obj.Pkg() == nil || obj.Parent() != obj.Pkg().Scope() {
		return // predeclared, or not at package level (a type parameter)
	}
	p := obj.Pkg().Path()
	_, isType := obj.(*types.TypeName)
	helper := p == s.path && isType && s.helpers[obj.Name()]
	switch {
	case !obj.Exported() && !helper:
		if r.bad == "" {
			r.bad = "unexported " + p + "." + obj.Name()
		}
		return
	case p == "unsafe":
		return
	case !s.inSet[p]:
		if r.bad == "" {
			r.bad = "package " + p
		}
		return
	}
	r.keys = append(r.keys, p+"."+obj.Name())
}

// exprRefs returns what the type expressions in n need.
func (s *pkgState) exprRefs(n ast.Node) refs {
	var r refs
	ast.Inspect(n, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := n.X.(*ast.Ident); ok {
				if pn, ok := s.info.Uses[id].(*types.PkgName); ok {
					if pn.Name() != pn.Imported().Name() && r.bad == "" {
						r.bad = "renamed import " + pn.Name()
					}
					s.objRef(&r, s.info.Uses[n.Sel])
					return false
				}
			}
		case *ast.Ident:
			s.objRef(&r, s.info.Uses[n])
		}
		return true
	})
	return r
}

// typeRefs returns what the type t needs.
func (s *pkgState) typeRefs(t types.Type) refs {
	var r refs
	var walk func(t types.Type)
	walk = func(t types.Type) {
		switch t := t.(type) {
		case *types.Named:
			s.objRef(&r, t.Obj())
			for a := range t.TypeArgs().Types() {
				walk(a)
			}
		case *types.Alias:
			s.objRef(&r, t.Obj())
			for a := range t.TypeArgs().Types() {
				walk(a)
			}
		case *types.Pointer:
			walk(t.Elem())
		case *types.Slice:
			walk(t.Elem())
		case *types.Array:
			walk(t.Elem())
		case *types.Map:
			walk(t.Key())
			walk(t.Elem())
		case *types.Chan:
			walk(t.Elem())
		case *types.Signature:
			for v := range t.Params().Variables() {
				walk(v.Type())
			}
			for v := range t.Results().Variables() {
				walk(v.Type())
			}
		case *types.Struct:
			for f := range t.Fields() {
				walk(f.Type())
			}
		case *types.Interface:
			for m := range t.ExplicitMethods() {
				walk(m.Type())
			}
			for e := range t.EmbeddedTypes() {
				walk(e)
			}
		case *types.Union:
			for term := range t.Terms() {
				walk(term.Type())
			}
		}
	}
	walk(t)
	return r
}

// needsDropped reports whether a declaration that needs r can't be in the
// stub.
func (s *pkgState) needsDropped(r refs) bool {
	if r.bad != "" {
		return true
	}
	for _, k := range r.keys {
		if s.dropped[k] {
			return true
		}
		if it := s.byKey[k]; it != nil && it.dropped {
			return true
		}
	}
	return false
}

// drop marks the items that can't be in the stub, until nothing changes.
func (s *pkgState) drop() {
	for changed := true; changed; {
		changed = false
		for _, it := range s.items {
			if !it.dropped && s.needsDropped(it.refs) {
				it.dropped, changed = true, true
			}
			for _, f := range it.fields {
				if !f.dropped && s.needsDropped(f.refs) {
					f.dropped, changed = true, true
				}
			}
		}
	}
}

// usesIota reports whether the const expression x uses iota.
func usesIota(x ast.Expr) bool {
	found := false
	ast.Inspect(x, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == "iota" {
			found = true
		}
		return !found
	})
	return found
}

// write returns the stub file.
func (s *pkgState) write(p stubPackage) ([]byte, error) {
	var body bytes.Buffer
	for _, d := range s.out {
		switch d := d.(type) {
		case *group:
			s.writeGroup(&body, d)
		case *item:
			if d.dropped {
				continue
			}
			switch d.kind {
			case typeItem:
				s.writeType(&body, d)
			case funcItem, methodItem:
				s.writeFunc(&body, d)
			}
		}
	}

	year, err := s.copyrightYear()
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Copyright %s The Go Authors. All rights reserved.\n", year)
	b.WriteString("// Use of this source code is governed by a BSD-style\n")
	b.WriteString("// license that can be found in the LICENSE file.\n\n")
	fmt.Fprintf(&b, "// Code generated by tools/stubgen from %s's src/%s; DO NOT EDIT.\n", s.version, s.path)
	b.WriteString("//\n")
	switch {
	case p.Intrinsic != "":
		fmt.Fprintf(&b, "// A declaration stub: the exported declarations of Go's %s package,\n", s.path)
		b.WriteString("// without function bodies. The compiler maps it to the runtime's\n")
		fmt.Fprintf(&b, "// %s (a Phase %02d intrinsic); the stub only gives go/types the\n", p.Intrinsic, p.Phase)
		b.WriteString("// declarations.\n")
	case p.Later != "":
		fmt.Fprintf(&b, "// A declaration stub for Phase 01: the exported declarations of Go's %s\n", s.path)
		b.WriteString("// package, without function bodies, so that programs that import it\n")
		fmt.Fprintf(&b, "// type-check. %s\n", p.Later)
	default:
		fmt.Fprintf(&b, "// A declaration stub for Phase 01: the exported declarations of Go's %s\n", s.path)
		b.WriteString("// package, without function bodies, so that programs that import it\n")
		fmt.Fprintf(&b, "// type-check. Phase %02d replaces it with the real package.\n", p.Phase)
	}
	if left := s.leftOut(); len(left) > 0 {
		b.WriteString("//\n")
		b.WriteString("// Left out, because they need a package without a stub or an unexported\n")
		b.WriteString("// name:")
		col := len("// name:")
		for i, name := range left {
			if i < len(left)-1 {
				name += ","
			} else {
				name += "."
			}
			if col+1+len(name) > 78 {
				b.WriteString("\n//")
				col = 2
			}
			b.WriteString(" " + name)
			col += 1 + len(name)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	for _, f := range s.files {
		if f.Doc != nil {
			writeDoc(&b, f.Doc)
			break
		}
	}
	fmt.Fprintf(&b, "package %s\n\n", s.pkg.Name())
	if len(s.imports) == 1 {
		fmt.Fprintf(&b, "import %q\n\n", sortedKeys(s.imports)[0])
	} else if len(s.imports) > 1 {
		b.WriteString("import (\n")
		for _, ip := range sortedKeys(s.imports) {
			fmt.Fprintf(&b, "\t%q\n", ip)
		}
		b.WriteString(")\n\n")
	}
	b.Write(body.Bytes())
	src, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format: %v\n%s", err, b.Bytes())
	}
	return src, nil
}

// copyrightRE matches the first line of Go's copyright notice.
var copyrightRE = regexp.MustCompile(`^// Copyright ([0-9]{4}) The Go Authors\. All rights reserved\.$`)

// copyrightYear returns the year of the stub's copyright notice: the
// earliest year in the notices of the package's files.
func (s *pkgState) copyrightYear() (string, error) {
	year := ""
	for _, f := range s.files {
		if len(f.Comments) == 0 {
			continue
		}
		m := copyrightRE.FindStringSubmatch(f.Comments[0].List[0].Text)
		if m == nil {
			continue
		}
		if year == "" || m[1] < year {
			year = m[1]
		}
	}
	if year == "" {
		return "", fmt.Errorf("no file has Go's copyright notice")
	}
	return year, nil
}

// leftOut returns the names of the exported declarations and fields that
// the stub leaves out.
func (s *pkgState) leftOut() []string {
	var names []string
	for _, it := range s.items {
		if it.dropped {
			names = append(names, it.name)
			continue
		}
		for _, f := range it.fields {
			if f.dropped && f.name != "_" {
				name := f.name
				if name == "" {
					name = embeddedName(f.field.Type)
				}
				names = append(names, it.name+"."+name)
			}
		}
	}
	return names
}

// writeDoc writes the comment group c, without its directives (//go:...
// and //line lines).
func writeDoc(b *bytes.Buffer, c *ast.CommentGroup) {
	if c == nil {
		return
	}
	var lines []string
	for _, l := range c.List {
		if strings.HasPrefix(l.Text, "//go:") || strings.HasPrefix(l.Text, "//line ") {
			continue
		}
		lines = append(lines, l.Text)
	}
	// A directive at the end leaves an empty // line behind.
	for len(lines) > 0 && lines[len(lines)-1] == "//" {
		lines = lines[:len(lines)-1]
	}
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
}

// lineComment returns the comment c as a line comment, with a space before.
func lineComment(c *ast.CommentGroup) string {
	if c == nil {
		return ""
	}
	var parts []string
	for _, l := range c.List {
		parts = append(parts, l.Text)
	}
	return " " + strings.Join(parts, " ")
}

// expr prints the node n of Go's source and records its imports.
func (s *pkgState) expr(n ast.Node) string {
	ast.Inspect(n, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				if pn, ok := s.info.Uses[id].(*types.PkgName); ok {
					s.imports[pn.Imported().Path()] = pn.Name()
				}
			}
		}
		return true
	})
	var b bytes.Buffer
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	if err := cfg.Fprint(&b, s.fset, n); err != nil {
		panic(err)
	}
	return b.String()
}

// qualifier qualifies the types that the stub prints from go/types and
// records their imports.
func (s *pkgState) qualifier(p *types.Package) string {
	if p.Path() == s.path {
		return ""
	}
	s.imports[p.Path()] = p.Name()
	return p.Name()
}

// writeGroup writes the kept items of a const or var group.
func (s *pkgState) writeGroup(b *bytes.Buffer, g *group) {
	var specs []*specItems
	for _, si := range g.specs {
		kept := &specItems{doc: si.doc, comment: si.comment}
		for _, it := range si.items {
			if !it.dropped {
				kept.items = append(kept.items, it)
			}
		}
		if len(kept.items) > 0 {
			specs = append(specs, kept)
		}
	}
	if len(specs) == 0 {
		return
	}
	writeDoc(b, g.doc)
	grouped := g.grouped
	if !grouped && len(specs[0].items) > 1 {
		grouped = true
	}
	if grouped {
		fmt.Fprintf(b, "%s (\n", g.tok)
	}
	for _, si := range specs {
		writeDoc(b, si.doc)
		for _, it := range si.items {
			if !grouped {
				b.WriteString(g.tok.String() + " ")
			}
			b.WriteString(it.name)
			if g.tok == token.CONST {
				b.WriteString(s.constSpec(it))
			} else {
				b.WriteString(" " + types.TypeString(it.obj.Type(), s.qualifier))
			}
			b.WriteString(lineComment(si.comment))
			b.WriteString("\n")
		}
	}
	if grouped {
		b.WriteString(")\n")
	}
	b.WriteString("\n")
}

// constSpec returns the type and the value of the const it, as they follow
// its name: " Duration = 1000" or " = 'x'".
func (s *pkgState) constSpec(it *item) string {
	c := it.obj.(*types.Const)
	t := c.Type()
	typed := true
	if b, ok := t.(*types.Basic); ok && b.Info()&types.IsUntyped != 0 {
		typed = false
	}
	var typ string
	if typed {
		typ = " " + types.TypeString(t, s.qualifier)
	}
	if it.value != nil && !s.needsDropped(it.valueRefs) {
		return typ + " = " + s.expr(it.value)
	}
	return typ + " = " + constValue(c.Val(), t)
}

// constValue writes the value v of a const of type t as Go source.
func constValue(v constant.Value, t types.Type) string {
	b, _ := t.Underlying().(*types.Basic)
	switch v.Kind() {
	case constant.Bool, constant.String:
		return v.ExactString()
	case constant.Int:
		if b != nil && b.Kind() == types.UntypedRune {
			r, ok := constant.Int64Val(v)
			if ok && r >= 0 && r <= 0x10FFFF {
				return strconv.QuoteRuneToASCII(rune(r))
			}
		}
		return v.ExactString()
	case constant.Float:
		if constant.ToInt(v).Kind() == constant.Int {
			return constant.ToInt(v).ExactString() + ".0"
		}
		return constant.Num(v).ExactString() + ".0 / " + constant.Denom(v).ExactString()
	}
	panic(fmt.Sprintf("stubgen: const value %v of kind %v", v, v.Kind()))
}

// writeType writes the type it.
func (s *pkgState) writeType(b *bytes.Buffer, it *item) {
	writeDoc(b, it.doc)
	ts := it.spec
	b.WriteString("type " + ts.Name.Name)
	if ts.TypeParams != nil {
		b.WriteString(s.typeParams(ts.TypeParams))
	}
	if ts.Assign.IsValid() {
		b.WriteString(" =")
	}
	b.WriteString(" ")
	switch t := ts.Type.(type) {
	case *ast.StructType:
		if ts.Assign.IsValid() {
			b.WriteString(s.expr(t))
			break
		}
		b.WriteString("struct {\n")
		for _, f := range it.fields {
			if f.dropped {
				continue
			}
			writeDoc(b, f.field.Doc)
			if f.name != "" {
				b.WriteString(f.name + " ")
			}
			b.WriteString(s.expr(f.field.Type))
			if f.field.Tag != nil {
				b.WriteString(" " + f.field.Tag.Value)
			}
			b.WriteString(lineComment(f.field.Comment))
			b.WriteString("\n")
		}
		dropped := it.extra
		for _, f := range it.fields {
			dropped = dropped || f.dropped
		}
		if dropped {
			if types.Comparable(it.tnamed.Type()) {
				b.WriteString("_ struct{} // and fields that the stub leaves out\n")
			} else {
				b.WriteString("_ [0]func() // and fields that the stub leaves out (not comparable)\n")
			}
		}
		b.WriteString("}")
	case *ast.InterfaceType:
		b.WriteString("interface {\n")
		for _, m := range t.Methods.List {
			writeDoc(b, m.Doc)
			if len(m.Names) > 0 {
				b.WriteString(m.Names[0].Name)
				b.WriteString(strings.TrimPrefix(s.expr(m.Type), "func"))
			} else {
				b.WriteString(s.expr(m.Type))
			}
			b.WriteString(lineComment(m.Comment))
			b.WriteString("\n")
		}
		b.WriteString("}")
	default:
		b.WriteString(s.expr(ts.Type))
	}
	b.WriteString(lineComment(ts.Comment))
	b.WriteString("\n\n")
}

// typeParams prints a type parameter list.
func (s *pkgState) typeParams(l *ast.FieldList) string {
	var parts []string
	for _, f := range l.List {
		var names []string
		for _, id := range f.Names {
			names = append(names, id.Name)
		}
		parts = append(parts, strings.Join(names, ", ")+" "+s.expr(f.Type))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// writeFunc writes the func or method it.
func (s *pkgState) writeFunc(b *bytes.Buffer, it *item) {
	writeDoc(b, it.doc)
	d := *it.decl
	d.Doc, d.Body = nil, nil
	b.WriteString(s.expr(&d))
	if d.Type.TypeParams != nil {
		// go/types wants a body for a generic function.
		b.WriteString(" {\n\tpanic(\"stub\")\n}")
	}
	b.WriteString("\n\n")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
