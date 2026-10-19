package check

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
)

// #Derive, second step (Phase 10 step 10.10, README §2.2, ADR-080).
//
// The loader added the methods of each #Derive(...) line as stubs (internal/load/derive.go): they
// have their final signatures, so go/types could check every caller. Now that go/types is done,
// the bodies are written from the types it found:
//
//	#Derive(Equal)
//	type Event struct { ID int; Tags []string }
//
//	func (e Event) Equal(other Event) bool {
//		if e.ID != other.ID { return false }
//		if len(e.Tags) != len(other.Tags) { return false }
//		for i := range e.Tags { if e.Tags[i] != other.Tags[i] { return false } }
//		return true
//	}
//
// The generators write Go source text for each method, format it, and parse it; the body replaces
// the stub's body and the package is checked once more (as the lowering of the `!` forms does).
// The text has a //line directive before every line, so that every position in it (an error, a
// panic in a stack trace, the line table of the C++) is the position of the generator's name in the
// #Derive line, and the names of its nodes are marked synthetic (File.Synthetic) so the editor's
// hover, rename and hints skip them. The text of every method is kept in Package.Derived, for
// gocpp expand.

// deriveFixer returns the function that gives the errors of go/types about the stubs of #Derive
// the words of the #Derive line, or nil if the package has none. The stubs have the place of the
// generator's name, so an error at that place is about the line.
func deriveFixer(lp *load.Package) func(token.Pos, string) string {
	if len(lp.Derives) == 0 {
		return nil
	}
	at := map[token.Pos]*load.Derive{}
	gens := map[token.Pos]string{}
	for _, d := range lp.Derives {
		for _, g := range d.Gens {
			if len(g.Funcs) > 0 {
				at[g.Pos] = d
				gens[g.Pos] = g.Name
			}
		}
	}
	return func(pos token.Pos, msg string) string {
		d := at[pos]
		if d == nil {
			return msg
		}
		name := d.Spec.Name.Name
		if strings.HasPrefix(msg, "invalid receiver type ") {
			return fmt.Sprintf("#Derive(%s): %s is an interface or a pointer type: the generators are for structs and for named array, slice, map and integer types", gens[pos], name)
		}
		if m, ok := strings.CutPrefix(msg, "field and method with the same name "); ok {
			return fmt.Sprintf("#Derive(%s): the type %s has a field %s already: a field and a method cannot have the same name", gens[pos], name, m)
		}
		return msg
	}
}

// DerivedFilePrefix starts the name of the files in the file set that hold the code which #Derive
// wrote (their lines say, with //line directives, that they are the #Derive line). A tool that
// reports on user code can skip a node in one: fset.PositionFor(pos, false).Filename.
const DerivedFilePrefix = "gocpp-derive-"

// A DerivedMethod is a method that #Derive generated, with its source text.
type DerivedMethod struct {
	File   *load.File
	Type   string // the type
	Method string // the method (String, Equal or Clone)
	Gen    string // the generator (String, Equal or Clone)
	Pos    token.Pos
	Src    string // the method as Go source, formatted
}

// deriveBodies writes the bodies of the stubs of lp.Derives. It reports whether it changed the
// package (it must be checked again). With textOnly it changes nothing.
func deriveBodies(fset *token.FileSet, lp *load.Package, p *Package, textOnly bool) bool {
	dp := &derivePass{fset: fset, lp: lp, p: p, pkgNames: map[string]bool{}, textOnly: textOnly}
	for _, name := range p.Types.Scope().Names() {
		dp.pkgNames[name] = true
	}
	dp.derivedEq, dp.derivedClone = map[string]bool{}, map[string]bool{}
	for _, d := range lp.Derives {
		for _, g := range d.Gens {
			if g.Name == "Equal" && len(g.Funcs) > 0 {
				dp.derivedEq[d.Spec.Name.Name] = true
			}
			if g.Name == "Clone" && len(g.Funcs) > 0 {
				dp.derivedClone[d.Spec.Name.Name] = true
			}
		}
	}
	changed := false
	for _, d := range lp.Derives {
		if dp.derive(d) {
			changed = true
		}
	}
	for _, e := range dp.errs {
		p.Errors.Add(e)
	}
	p.Derived = append(p.Derived, dp.out...)
	return changed && len(dp.errs) == 0
}

type derivePass struct {
	fset     *token.FileSet
	lp       *load.Package
	p        *Package
	pkgNames map[string]bool
	errs     []diag.Diagnostic
	out      []DerivedMethod
	files    map[*load.File]*deriveFile
	seq      int

	// textOnly says to write the text of the methods and not to put it in the stubs.
	textOnly bool

	// derivedEq holds the types whose Equal method is one that #Derive(Equal) writes: for such a
	// type that holds values only, == means the same as the method, and is faster.
	invalidMemo  map[types.Type]bool
	methodMemo   map[methodKey]bool
	derivedEq    map[string]bool
	derivedClone map[string]bool // likewise for Clone: a copy by assignment does the same when the type holds values only
}

// A deriveFile is what the pass knows about the imports of a file.
type deriveFile struct {
	file *load.File
	// names maps the path of each import with a usable name to that name ("" for a dot import).
	names map[string]string
	// taken are the names that the file and the package declare at the top level.
	taken map[string]bool
}

func (dp *derivePass) errorf(pos token.Pos, format string, args ...any) {
	dp.errs = append(dp.errs, diag.Diagnostic{Pos: dp.fset.Position(pos), Msg: fmt.Sprintf(format, args...)})
}

// file returns what is known about the imports of f.
func (dp *derivePass) file(f *load.File) *deriveFile {
	if df := dp.files[f]; df != nil {
		return df
	}
	if dp.files == nil {
		dp.files = map[*load.File]*deriveFile{}
	}
	df := &deriveFile{file: f, names: map[string]string{}, taken: map[string]bool{}}
	for n := range dp.pkgNames {
		df.taken[n] = true
	}
	for _, spec := range f.AST.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		switch {
		case spec.Name == nil:
			name := path
			if i := strings.LastIndex(path, "/"); i >= 0 {
				name = path[i+1:]
			}
			if dep := dp.lp.Imports[path]; dep != nil && dep.Name != "" {
				name = dep.Name
			}
			df.names[path] = name
			df.taken[name] = true
		case spec.Name.Name == ".":
			df.names[path] = ""
		case spec.Name.Name == "_":
		default:
			df.names[path] = spec.Name.Name
			df.taken[spec.Name.Name] = true
		}
	}
	dp.files[f] = df
	return df
}

// derive writes the bodies of the stubs of one #Derive line.
func (dp *derivePass) derive(d *load.Derive) bool {
	obj, _ := dp.p.Types.Scope().Lookup(d.Spec.Name.Name).(*types.TypeName)
	if obj == nil {
		return false
	}
	named, _ := types.Unalias(obj.Type()).(*types.Named)
	if named == nil {
		return false
	}
	changed := false
	for _, gen := range d.Gens {
		g := newDeriveGen(dp, d, gen, named)
		if !g.run() {
			continue
		}
		changed = true
	}
	return changed
}

// A deriveGen writes the methods of one generator of a #Derive line.
type deriveGen struct {
	dp    *derivePass
	d     *load.Derive
	gen   *load.DeriveGen
	named *types.Named
	df    *deriveFile

	recv, other string // the names of the receiver and of the parameter of Equal
	recvType    string // T or T[K, V]
	tparams     map[string]bool

	// the method being written
	lines []string
	in    int
	used  map[string]bool // the names taken in the method
	scope [][]string      // the names that each open block added, freed when it closes
	bad   bool            // an error was reported

	// stack holds the named types being expanded, to stop on a type that refers to itself.
	stack []*types.Named
}

func newDeriveGen(dp *derivePass, d *load.Derive, gen *load.DeriveGen, named *types.Named) *deriveGen {
	g := &deriveGen{dp: dp, d: d, gen: gen, named: named, df: dp.file(d.File), recv: d.Recv, other: "other", tparams: map[string]bool{}}
	g.recvType = d.Spec.Name.Name
	if d.Spec.TypeParams != nil && len(d.Spec.TypeParams.List) > 0 {
		var names []string
		for _, f := range d.Spec.TypeParams.List {
			for _, n := range f.Names {
				names = append(names, n.Name)
				g.tparams[n.Name] = true
			}
		}
		g.recvType += "[" + strings.Join(names, ", ") + "]"
	}
	return g
}

func (g *deriveGen) errorf(format string, args ...any) {
	g.bad = true
	g.dp.errorf(g.gen.Pos, format, args...)
}

// reset starts a new method.
func (g *deriveGen) reset() {
	g.lines, g.in, g.stack, g.scope = nil, 0, nil, [][]string{nil}
	g.used = map[string]bool{g.recv: true, g.other: true, "data": true}
	for n := range g.df.taken {
		g.used[n] = true
	}
	for n := range g.tparams {
		g.used[n] = true
	}
}

// w writes a line.
func (g *deriveGen) w(format string, args ...any) {
	g.lines = append(g.lines, strings.Repeat("\t", g.in)+fmt.Sprintf(format, args...))
}

// open writes a line that opens a block.
func (g *deriveGen) open(format string, args ...any) {
	g.w(format, args...)
	g.in++
	g.scope = append(g.scope, nil)
}

// close ends a block with the line s.
func (g *deriveGen) close(s string) {
	g.in--
	g.w("%s", s)
	if n := len(g.scope); n > 1 {
		for _, name := range g.scope[n-1] {
			delete(g.used, name)
		}
		g.scope = g.scope[:n-1]
	}
}

// fresh returns a name that is free in the method.
func (g *deriveGen) fresh(base string) string {
	name := base
	for n := 1; g.used[name]; n++ {
		name = base + strconv.Itoa(n)
	}
	g.used[name] = true
	g.scope[len(g.scope)-1] = append(g.scope[len(g.scope)-1], name)
	return name
}

// spell returns the type t as the file writes it, or false when the file cannot (a package of it
// is not imported there). Nothing is reported.
func (g *deriveGen) spell(t types.Type) (string, bool) {
	ok := true
	s := types.TypeString(t, func(q *types.Package) string {
		if q == g.dp.p.Types {
			return ""
		}
		if n, found := g.df.names[q.Path()]; found {
			return n
		}
		ok = false
		return q.Name()
	})
	return s, ok
}

// tname returns the type t for a message: the names of its packages, nothing is checked.
func (g *deriveGen) tname(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p == g.dp.p.Types {
			return ""
		}
		return p.Name()
	})
}

// typ returns the type t as the file of the type writes it.
func (g *deriveGen) typ(t types.Type) string {
	return types.TypeString(t, func(q *types.Package) string {
		if q == g.dp.p.Types {
			return ""
		}
		if n, ok := g.df.names[q.Path()]; ok {
			if n == "" {
				return ""
			}
			return n
		}
		g.errorf("#Derive(%s): the type %s is from the package %s, which the file does not import: add the import to the file", g.gen.Name, types.TypeString(t, func(p *types.Package) string {
			if p == g.dp.p.Types {
				return ""
			}
			return p.Name()
		}), q.Name())
		return q.Name()
	})
}

// run writes the methods of the generator. It reports whether it replaced the stubs.
func (g *deriveGen) run() bool {
	var methods []string
	switch g.gen.Name {
	case "String":
		methods = []string{g.str()}
	case "Equal":
		methods = []string{g.equal()}
	case "Clone":
		methods = []string{g.clone()}
	}
	if g.bad || len(methods) != len(g.gen.Funcs) {
		return false
	}
	for i, src := range methods {
		if !g.install(g.gen.Funcs[i], src) {
			return false
		}
	}
	return true
}

// begin starts the method with the header text.
func (g *deriveGen) begin(header string) {
	g.reset()
	g.w("%s {", header)
	g.in++
}

// finish ends the method and returns its text.
func (g *deriveGen) finish() string {
	g.close("}")
	return strings.Join(g.lines, "\n") + "\n"
}

// install parses the text of a method and puts its body in the stub.
func (g *deriveGen) install(stub *ast.FuncDecl, src string) bool {
	if g.dp.textOnly {
		return false // the caller needs the errors only: no formatting, no parsing, nothing kept
	}
	plain, err := format.Source([]byte("package p\n\n" + src))
	if err != nil {
		g.dp.errorf(g.gen.Pos, "internal error: #Derive(%s) wrote code that does not parse: %v\n%s", g.gen.Name, err, src)
		return false
	}
	text := strings.TrimPrefix(string(plain), "package p\n\n")
	// Every line has the place of the name of the generator in the #Derive line.
	at := g.dp.fset.Position(g.gen.Pos)
	// (A //line directive above the #Derive line without a column gives the column 0, which is no place.)
	directive := fmt.Sprintf("//line %s:%d:%d\n", at.Filename, max(at.Line, 1), max(at.Column, 1))
	var b strings.Builder
	b.WriteString("package p\n")
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		b.WriteString(directive)
		b.WriteString(line)
		b.WriteString("\n")
	}
	g.dp.seq++
	f, err := parser.ParseFile(g.dp.fset, DerivedFilePrefix+strconv.Itoa(g.dp.seq)+".go", b.String(), 0)
	if err != nil || len(f.Decls) != 1 {
		g.dp.errorf(g.gen.Pos, "internal error: #Derive(%s) wrote code that does not parse: %v\n%s", g.gen.Name, err, src)
		return false
	}
	fd, ok := f.Decls[0].(*ast.FuncDecl)
	if !ok {
		return false
	}
	stub.Body = fd.Body
	g.dp.out = append(g.dp.out, DerivedMethod{File: g.d.File, Type: g.d.Spec.Name.Name, Method: stub.Name.Name,
		Gen: g.gen.Name, Pos: g.gen.Pos, Src: text})
	return true
}

// ---- helpers about types ----

// other reports whether the named type n is declared in another package: the generators cannot
// look inside such a type (its fields may be private), so it is a value.
func (g *deriveGen) foreign(n *types.Named) bool {
	return n.Obj().Pkg() != nil && n.Obj().Pkg() != g.dp.p.Types
}

// isInvalid is invalid with the answers kept: the generators ask about the same types again and again.
func (dp *derivePass) isInvalid(t types.Type) bool {
	if dp.invalidMemo == nil {
		dp.invalidMemo = map[types.Type]bool{}
	}
	return invalid(t, map[types.Type]bool{}, dp.invalidMemo)
}

// invalid reports whether t has a part that go/types left invalid: a C or C++ type.
func invalid(t types.Type, seen, memo map[types.Type]bool) bool {
	if v, ok := memo[t]; ok {
		return v
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	switch u := t.(type) {
	case *types.Basic:
		return u.Kind() == types.Invalid
	case *types.Named:
		if u.Obj().Pkg() == nil {
			return false
		}
		if u.TypeArgs() != nil {
			for i := range u.TypeArgs().Len() {
				if invalid(u.TypeArgs().At(i), seen, memo) {
					return true
				}
			}
		}
		v := invalid(u.Underlying(), seen, memo)
		memo[t] = v // (a type that refers to itself is cut where it comes back)
		return v
	case *types.Alias:
		return invalid(types.Unalias(u), seen, memo)
	case *types.Pointer:
		return invalid(u.Elem(), seen, memo)
	case *types.Slice:
		return invalid(u.Elem(), seen, memo)
	case *types.Array:
		return invalid(u.Elem(), seen, memo)
	case *types.Map:
		return invalid(u.Key(), seen, memo) || invalid(u.Elem(), seen, memo)
	case *types.Chan:
		return invalid(u.Elem(), seen, memo)
	case *types.Struct:
		for i := range u.NumFields() {
			if invalid(u.Field(i).Type(), seen, memo) {
				return true
			}
		}
	}
	return false
}

// lookup returns the method name of t, as a call on a variable of type t can use it, or nil.
func (g *deriveGen) lookup(t types.Type, name string) *types.Selection {
	t = types.Unalias(t)
	var ms *types.MethodSet
	switch t.Underlying().(type) {
	case *types.Interface, *types.Pointer:
		ms = types.NewMethodSet(t)
	default:
		if _, isTP := t.(*types.TypeParam); isTP {
			ms = types.NewMethodSet(t)
		} else {
			ms = types.NewMethodSet(types.NewPointer(t))
		}
	}
	return ms.Lookup(g.dp.p.Types, name)
}

// method reports whether t has the method name with a signature that ok accepts.
func (g *deriveGen) method(t types.Type, name string, ok func(*types.Signature) bool) bool {
	sel := g.lookup(t, name)
	if sel == nil {
		return false
	}
	fn, isFunc := sel.Obj().(*types.Func)
	if !isFunc {
		return false
	}
	sig, _ := fn.Type().(*types.Signature)
	return sig != nil && ok(sig)
}

// otherSignature reports whether the type t declares itself a method with the name of a derived
// one but not with its signature (Equal(*T) bool): a method promoted from a field does not count.
func (g *deriveGen) otherSignature(t types.Type, name string) bool {
	return g.memo("other"+name, t, func() bool { return g.otherSignature1(t, name) })
}

func (g *deriveGen) otherSignature1(t types.Type, name string) bool {
	sel := g.lookup(t, name)
	if sel == nil || len(sel.Index()) != 1 || sel.Kind() != types.MethodVal {
		return false
	}
	switch name {
	case "Equal":
		return !g.hasEqual(t)
	case "Clone":
		return !g.hasClone(t)
	}
	return false
}

// memo answers a question about the methods of t once: the method sets are not cheap.
func (g *deriveGen) memo(what string, t types.Type, f func() bool) bool {
	k := methodKey{what, t}
	if v, ok := g.dp.methodMemo[k]; ok {
		return v
	}
	if g.dp.methodMemo == nil {
		g.dp.methodMemo = map[methodKey]bool{}
	}
	v := f()
	g.dp.methodMemo[k] = v
	return v
}

type methodKey struct {
	what string
	t    types.Type
}

// hasEqual reports whether a variable of type t has the method Equal(t) bool.
func (g *deriveGen) hasEqual(t types.Type) bool {
	return g.memo("Equal", t, func() bool { return g.hasEqual1(t) })
}

func (g *deriveGen) hasEqual1(t types.Type) bool {
	return g.method(t, "Equal", func(s *types.Signature) bool {
		return s.Params().Len() == 1 && s.Results().Len() == 1 && !s.Variadic() &&
			types.Identical(s.Params().At(0).Type(), t) && isBool(s.Results().At(0).Type())
	})
}

// hasClone reports whether a variable of type t has the method Clone() t.
func (g *deriveGen) hasClone(t types.Type) bool {
	return g.memo("Clone", t, func() bool { return g.hasClone1(t) })
}

func (g *deriveGen) hasClone1(t types.Type) bool {
	return g.method(t, "Clone", func(s *types.Signature) bool {
		return s.Params().Len() == 0 && s.Results().Len() == 1 && types.Identical(s.Results().At(0).Type(), t)
	})
}

func isInterface(t types.Type) bool {
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

func isBool(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.Bool
}

// refs reports whether copying a value of type t takes more than an assignment, as far as the
// generators can see: it refers to memory (pointers, slices, maps, channels, funcs and interfaces),
// or its type has a Clone method of its own. A type parameter is a value (the generators cannot know);
// a type of another package is a value unless it has a Clone method or shows a slice, a map or a
// pointer in an exported field (which cl reports: it needs a Clone method).
func (g *deriveGen) refs(t types.Type) bool {
	return g.refsIn(t, map[types.Type]bool{})
}

func (g *deriveGen) refsIn(t types.Type, seen map[types.Type]bool) bool {
	t = types.Unalias(t)
	if seen[t] {
		return false
	}
	seen[t] = true
	if _, ok := t.(*types.TypeParam); ok {
		return false
	}
	if n, ok := t.(*types.Named); ok {
		if g.hasClone(t) {
			if g.foreign(n) || !g.dp.derivedClone[n.Obj().Name()] {
				return true // its Clone says what a copy is
			}
			// a derived Clone is an assignment for a type that holds values only: look inside
		} else if g.foreign(n) {
			return g.visibleRefs(t, map[types.Type]bool{})
		}
	}
	switch u := t.Underlying().(type) {
	case *types.Pointer, *types.Slice, *types.Map, *types.Chan, *types.Signature, *types.Interface:
		return true
	case *types.Array:
		return g.refsIn(u.Elem(), seen)
	case *types.Struct:
		for i := range u.NumFields() {
			if g.refsIn(u.Field(i).Type(), seen) {
				return true
			}
		}
	}
	return false
}

// visibleRefs reports whether a type of another package without a Clone method shows memory that a
// copy would share: it is a slice or map type, or a struct with an exported field that holds a
// pointer, slice or map. (A struct with only private fields, like time.Time or sync.Mutex, is a value
// to the generators: they cannot look inside it.)
func (g *deriveGen) visibleRefs(t types.Type, seen map[types.Type]bool) bool {
	t = types.Unalias(t)
	if seen[t] {
		return false
	}
	seen[t] = true
	if n, ok := t.(*types.Named); ok && g.foreign(n) && g.hasClone(t) {
		return false
	}
	switch u := t.Underlying().(type) {
	case *types.Pointer, *types.Slice, *types.Map:
		return true
	case *types.Array:
		return g.visibleRefs(u.Elem(), seen)
	case *types.Struct:
		_, named := t.(*types.Named)
		for i := range u.NumFields() {
			f := u.Field(i)
			if named && !f.Exported() {
				continue
			}
			if g.visibleRefs(f.Type(), seen) {
				return true
			}
		}
	}
	return false
}

// flat reports whether == compares values of type t the way Equal means: it compares the whole
// value and follows no pointer.
func (g *deriveGen) flat(t types.Type) bool {
	t = types.Unalias(t)
	if g.hasEqual(t) && !g.derivedEqual(t) {
		return false // its Equal says what equal means
	}
	if n, ok := t.(*types.Named); ok && g.foreign(n) {
		return types.Comparable(t)
	}
	if tp, ok := t.(*types.TypeParam); ok {
		return types.Comparable(tp)
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return u.Kind() != types.UntypedNil && u.Kind() != types.Invalid
	case *types.Chan, *types.Interface:
		return true
	case *types.Array:
		return g.flat(u.Elem())
	case *types.Struct:
		return g.flatFields(u)
	}
	return false
}

// derivedEqual reports whether t is a type of this package whose Equal method is the one that
// #Derive(Equal) writes (so its structure says what equal means).
func (g *deriveGen) derivedEqual(t types.Type) bool {
	n, ok := t.(*types.Named)
	return ok && !g.foreign(n) && n.Obj().Pkg() != nil && g.dp.derivedEq[n.Obj().Name()]
}

// flatFields reports whether all the fields of st are flat.
func (g *deriveGen) flatFields(st *types.Struct) bool {
	for i := range st.NumFields() {
		if !g.flat(st.Field(i).Type()) {
			return false
		}
	}
	return true
}

// deriveFields returns the fields of the struct st that the generators look at: not the blank ones.
func deriveFields(st *types.Struct) []*types.Var {
	var out []*types.Var
	for i := range st.NumFields() {
		if f := st.Field(i); f.Name() != "_" {
			out = append(out, f)
		}
	}
	return out
}

// checkInvalid reports a field of a C or C++ type; it reports whether there is one.
func (g *deriveGen) checkInvalid(st *types.Struct) bool {
	for _, f := range deriveFields(st) {
		if g.dp.isInvalid(f.Type()) {
			g.errorf("#Derive(%s): the field %s of %s has a C or C++ type: the generators cannot look inside it, write the method by hand", g.gen.Name, f.Name(), g.named.Obj().Name())
			return true
		}
	}
	return false
}

// ---- String ----

func (g *deriveGen) str() string {
	name := g.named.Obj().Name()
	if b, ok := g.named.Underlying().(*types.Basic); ok && b.Info()&types.IsInteger != 0 {
		return g.strEnum(b)
	}
	if _, ok := g.named.Underlying().(*types.Struct); ok {
		g.errorf("#Derive(String): %s is a struct: fmt prints a struct by itself (with the plus flag it shows the field names), and String is for integer types with constants", name)
	} else {
		g.errorf("#Derive(String): %s is not an integer type with constants", name)
	}
	return ""
}

// strEnum writes String for an integer type with constants, as Go's stringer tool does: the name
// of the constant, or Type(n) for another value. A value that two constants share gets the first
// name in the order of the declarations.
func (g *deriveGen) strEnum(b *types.Basic) string {
	name := g.named.Obj().Name()
	type kv struct {
		name string
		pos  token.Pos
		val  string
	}
	var consts []kv
	scope := g.dp.p.Types.Scope()
	for _, n := range scope.Names() {
		c, ok := scope.Lookup(n).(*types.Const)
		if !ok || c.Name() == "_" || !types.Identical(c.Type(), g.named) {
			continue
		}
		consts = append(consts, kv{c.Name(), c.Pos(), c.Val().ExactString()})
	}
	sort.Slice(consts, func(i, j int) bool { return consts[i].pos < consts[j].pos })
	if len(consts) == 0 {
		g.errorf("#Derive(String): no constants of type %s are declared in this package", name)
		return ""
	}
	g.begin(fmt.Sprintf("func (%s %s) String() string", g.recv, g.recvType))
	seen := map[string]bool{}
	g.open("switch %s {", g.recv)
	for _, c := range consts {
		if seen[c.val] {
			continue
		}
		seen[c.val] = true
		g.w("case %s:", c.name)
		g.w("\treturn %s", strconv.Quote(c.name))
	}
	g.close("}")
	// Another value: the name of the type and the number, written by hand (the code imports nothing).
	n, digits := g.fresh("n"), g.fresh("s")
	g.w("%s := uint64(%s)", n, g.recv)
	signed := b.Info()&types.IsUnsigned == 0
	if signed {
		g.open("if %s < 0 {", g.recv)
		g.w("%s = -%s", n, n)
		g.close("}")
	}
	g.w("%s := string(rune('0' + %s%%10))", digits, n)
	g.open("for %s /= 10; %s > 0; %s /= 10 {", n, n, n)
	g.w("%s = string(rune('0'+%s%%10)) + %s", digits, n, digits)
	g.close("}")
	if signed {
		g.open("if %s < 0 {", g.recv)
		g.w("%s = \"-\" + %s", digits, digits)
		g.close("}")
	}
	g.w("return %s + %s + \")\"", strconv.Quote(name+"("), digits)
	return g.finish()
}

// ---- Equal ----

func (g *deriveGen) equal() string {
	name := g.named.Obj().Name()
	u := g.named.Underlying()
	switch u.(type) {
	case *types.Interface, *types.Pointer, *types.Signature, *types.Chan:
		g.errorf("#Derive(Equal): %s has the underlying type %s: the generators are for structs and for named array, slice, map and integer types", name, g.tname(u))
		return ""
	}
	st, isStruct := u.(*types.Struct)
	if isStruct && g.checkInvalid(st) {
		return ""
	}
	g.begin(fmt.Sprintf("func (%s %s) Equal(%s %s) bool", g.recv, g.recvType, g.other, g.recvType))
	if isStruct && g.flatFields(st) || !isStruct && g.flat(u) {
		g.w("return %s == %s", g.recv, g.other)
		return g.finish()
	}
	g.stack = []*types.Named{g.named}
	if isStruct {
		for _, f := range deriveFields(st) {
			g.eq(g.recv+"."+f.Name(), g.other+"."+f.Name(), f.Type(), "field "+f.Name())
		}
	} else {
		g.eq(g.recv, g.other, u, name)
	}
	g.w("return true")
	if g.bad {
		return ""
	}
	return g.finish()
}

// failIf writes: if cond { return false }.
func (g *deriveGen) failIf(cond string) {
	g.open("if %s {", cond)
	g.w("return false")
	g.close("}")
}

// eq writes the statements that return false when the values a and b of type t are not equal;
// what says where the values are, for messages.
func (g *deriveGen) eq(a, b string, t types.Type, what string) {
	t = types.Unalias(t)
	if g.dp.isInvalid(t) {
		g.errorf("#Derive(Equal): %s has a C or C++ type: the generators cannot look inside it, write the method by hand", what)
		return
	}
	if tp, ok := t.(*types.TypeParam); ok {
		switch {
		case g.hasEqual(t):
			g.failIf(fmt.Sprintf("!%s.Equal(%s)", a, b))
		case types.Comparable(tp):
			g.failIf(fmt.Sprintf("%s != %s", a, b))
		default:
			g.failIf(fmt.Sprintf("any(%s) != any(%s)", a, b))
		}
		return
	}
	named, _ := t.(*types.Named)
	if named != nil && g.otherSignature(t, "Equal") {
		g.errorf("#Derive(Equal): %s has the type %s, whose Equal method does not have the signature Equal(%s) bool: change it or write the comparison by hand", what, g.tname(t), g.tname(t))
		return
	}
	if g.hasEqual(t) && !(g.derivedEqual(t) && g.flat(t)) {
		g.failIf(fmt.Sprintf("!%s.Equal(%s)", a, b))
		return
	}
	if named != nil && g.foreign(named) {
		if !types.Comparable(t) {
			g.errorf("#Derive(Equal): %s has the type %s, which has no Equal method and cannot be compared with ==: write the method by hand", what, g.tname(t))
			return
		}
		g.failIf(fmt.Sprintf("%s != %s", a, b))
		return
	}
	switch u := t.Underlying().(type) {
	case *types.Basic, *types.Chan:
		g.failIf(fmt.Sprintf("%s != %s", a, b))
	case *types.Interface:
		g.failIf(fmt.Sprintf("%s != %s", a, b))
	case *types.Signature:
		g.errorf("#Derive(Equal): %s has a func type, which cannot be compared", what)
	case *types.Pointer:
		g.eqPointer(a, b, u, what)
	case *types.Struct:
		if g.flat(t) {
			g.failIf(fmt.Sprintf("%s != %s", a, b))
			return
		}
		if named != nil {
			g.errorf("#Derive(Equal): %s has the type %s, which has no Equal method and holds pointers, slices or maps: add #Derive(Equal) to %s or write the method by hand", what, g.tname(t), g.tname(t))
			return
		}
		for _, f := range deriveFields(u) {
			g.eq(a+"."+f.Name(), b+"."+f.Name(), f.Type(), what+"."+f.Name())
		}
	case *types.Array:
		if g.flat(t) {
			g.failIf(fmt.Sprintf("%s != %s", a, b))
			return
		}
		if !g.enter(named, "Equal") {
			return
		}
		i := g.fresh("i")
		g.open("for %s := range %s {", i, a)
		g.eq(a+"["+i+"]", b+"["+i+"]", u.Elem(), what+"[]")
		g.close("}")
		g.leave(named)
	case *types.Slice:
		if !g.enter(named, "Equal") {
			return
		}
		g.failIf(fmt.Sprintf("len(%s) != len(%s)", a, b))
		i := g.fresh("i")
		g.open("for %s := range %s {", i, a)
		g.eq(a+"["+i+"]", b+"["+i+"]", u.Elem(), what+"[]")
		g.close("}")
		g.leave(named)
	case *types.Map:
		if !g.enter(named, "Equal") {
			return
		}
		g.failIf(fmt.Sprintf("len(%s) != len(%s)", a, b))
		k, v, w, ok := g.fresh("k"), g.fresh("v"), g.fresh("w"), g.fresh("ok")
		g.open("for %s, %s := range %s {", k, v, a)
		g.w("%s, %s := %s[%s]", w, ok, b, k)
		g.failIf("!" + ok)
		g.eq(v, w, u.Elem(), what+"[]")
		g.close("}")
		g.leave(named)
	default:
		g.errorf("#Derive(Equal): %s has the type %s, which the generator cannot compare", what, g.tname(t))
	}
}

// eqPointer writes the comparison of two pointers: the same pointer is equal, nil is equal to
// nothing else, and otherwise what they point at is compared (identity for a type of another
// package that has no Equal method).
func (g *deriveGen) eqPointer(a, b string, p *types.Pointer, what string) {
	elem := types.Unalias(p.Elem())
	if g.hasEqual(elem) && !(g.derivedEqual(elem) && g.flat(elem)) {
		call := a // a pointer to a type parameter has no methods: call it on the value
		if _, isTP := elem.(*types.TypeParam); isTP {
			call = "(*" + a + ")"
		}
		g.open("if %s != %s {", a, b)
		g.failIf(fmt.Sprintf("%s == nil || %s == nil", a, b))
		g.failIf(fmt.Sprintf("!%s.Equal(*%s)", call, b))
		g.close("}")
		return
	}
	if n, ok := elem.(*types.Named); ok && g.foreign(n) {
		g.failIf(fmt.Sprintf("%s != %s", a, b))
		return
	}
	g.open("if %s != %s {", a, b)
	g.failIf(fmt.Sprintf("%s == nil || %s == nil", a, b))
	g.eq("(*"+a+")", "(*"+b+")", elem, "*"+what)
	g.close("}")
}

// enter notes that the named type n is being expanded; it reports false (and says why) if it
// already is, which means that the type refers to itself without a method of its own.
func (g *deriveGen) enter(n *types.Named, gen string) bool {
	if n == nil {
		return true
	}
	for _, s := range g.stack {
		if s == n {
			g.errorf("#Derive(%s): the type %s refers to itself and has no %s method: add #Derive(%s) to it", gen, g.tname(n), gen, gen)
			return false
		}
	}
	g.stack = append(g.stack, n)
	return true
}

func (g *deriveGen) leave(n *types.Named) {
	if n != nil {
		g.stack = g.stack[:len(g.stack)-1]
	}
}

// ---- Clone ----

func (g *deriveGen) clone() string {
	name := g.named.Obj().Name()
	u := g.named.Underlying()
	switch u.(type) {
	case *types.Interface, *types.Pointer, *types.Signature, *types.Chan:
		g.errorf("#Derive(Clone): %s has the underlying type %s: the generators are for structs and for named array, slice, map and integer types", name, g.tname(u))
		return ""
	}
	st, isStruct := u.(*types.Struct)
	if isStruct && g.checkInvalid(st) {
		return ""
	}
	g.begin(fmt.Sprintf("func (%s %s) Clone() %s", g.recv, g.recvType, g.recvType))
	g.stack = []*types.Named{g.named}
	c := g.fresh("c")
	if isStruct {
		if !g.refs(g.named) {
			g.w("return %s", g.recv)
			return g.finish()
		}
		g.w("%s := %s", c, g.recv)
		for _, f := range deriveFields(st) {
			g.cl(c+"."+f.Name(), g.recv+"."+f.Name(), f.Type(), true, "field "+f.Name())
		}
	} else {
		if !g.refs(g.named) {
			g.w("return %s", g.recv)
			return g.finish()
		}
		g.w("var %s %s", c, g.recvType)
		g.cl(c, g.recv, u, false, name)
	}
	g.w("return %s", c)
	if g.bad {
		return ""
	}
	return g.finish()
}

// cl writes the statements that set dst to a deep copy of src, a value of type t. With prefilled,
// dst already holds a copy of src made by assignment, so only what refers to memory needs work.
func (g *deriveGen) cl(dst, src string, t types.Type, prefilled bool, what string) {
	orig := t // an alias is written as it is declared: its package may not be imported here
	t = types.Unalias(t)
	if g.dp.isInvalid(t) {
		g.errorf("#Derive(Clone): %s has a C or C++ type: the generators cannot look inside it, write the method by hand", what)
		return
	}
	copyIfNeeded := func() {
		if !prefilled {
			g.w("%s = %s", dst, src)
		}
	}
	named, _ := t.(*types.Named)
	if _, isTP := t.(*types.TypeParam); isTP {
		copyIfNeeded()
		return
	}
	if named != nil && g.otherSignature(t, "Clone") {
		g.errorf("#Derive(Clone): %s has the type %s, whose Clone method does not have the signature Clone() %s: change it or write the copy by hand", what, g.tname(t), g.tname(t))
		return
	}
	if !g.refs(t) {
		copyIfNeeded()
		return
	}
	if named != nil && g.foreign(named) && !g.hasClone(t) {
		g.errorf("#Derive(Clone): %s has the type %s, which shows slices, maps or pointers and has no Clone method: write one for it, or keep the value out of the type", what, g.tname(orig))
		return
	}
	if g.hasClone(t) {
		if _, isIface := t.Underlying().(*types.Interface); isIface {
			g.open("if %s != nil {", src)
			g.w("%s = %s.Clone()", dst, src)
			g.close("}")
			return
		}
		g.w("%s = %s.Clone()", dst, src)
		return
	}
	switch u := t.Underlying().(type) {
	case *types.Interface, *types.Chan, *types.Signature:
		copyIfNeeded()
	case *types.Pointer:
		if n, ok := types.Unalias(u.Elem()).(*types.Named); ok && g.foreign(n) && !g.hasClone(n) {
			copyIfNeeded() // a pointer to a type of another package is shared (Equal compares it by identity too)
			return
		}
		g.clPointer(dst, src, u, what)
	case *types.Struct:
		if named != nil {
			g.errorf("#Derive(Clone): %s has the type %s, which refers to memory and has no Clone method: add #Derive(Clone) to %s or write the method by hand", what, g.tname(orig), g.tname(orig))
			return
		}
		copyIfNeeded()
		for _, f := range deriveFields(u) {
			if g.refs(f.Type()) {
				g.cl(dst+"."+f.Name(), src+"."+f.Name(), f.Type(), true, what+"."+f.Name())
			}
		}
	case *types.Array:
		if !g.enter(named, "Clone") {
			return
		}
		i := g.fresh("i")
		g.open("for %s := range %s {", i, src)
		g.cl(dst+"["+i+"]", src+"["+i+"]", u.Elem(), prefilled, what+"[]")
		g.close("}")
		g.leave(named)
	case *types.Slice:
		if !g.enter(named, "Clone") {
			return
		}
		g.open("if %s != nil {", src)
		if ts, ok := g.spell(orig); ok {
			g.w("%s = make(%s, len(%s))", dst, ts, src)
			if !g.refs(u.Elem()) {
				g.w("copy(%s, %s)", dst, src)
			}
		} else {
			// The file cannot write the type (its package is not imported here): a copy of the slice
			// has the type of the slice, whatever it is called.
			g.w("%s = append(%s[:0:0], %s...)", dst, src, src)
		}
		if g.refs(u.Elem()) {
			i := g.fresh("i")
			g.open("for %s := range %s {", i, src)
			g.cl(dst+"["+i+"]", src+"["+i+"]", u.Elem(), false, what+"[]")
			g.close("}")
		}
		g.close("}")
		g.leave(named)
	case *types.Map:
		if !g.enter(named, "Clone") {
			return
		}
		g.open("if %s != nil {", src)
		g.w("%s = make(%s, len(%s))", dst, g.typ(orig), src)
		k, v := g.fresh("k"), g.fresh("v")
		g.open("for %s, %s := range %s {", k, v, src)
		switch {
		case !g.refs(u.Elem()):
			g.w("%s[%s] = %s", dst, k, v)
		default:
			c := g.fresh("c")
			g.w("%s := %s", c, v)
			g.cl(c, v, u.Elem(), true, what+"[]")
			g.w("%s[%s] = %s", dst, k, c)
		}
		g.close("}")
		g.close("}")
		g.leave(named)
	default:
		g.errorf("#Derive(Clone): %s has the type %s, which the generator cannot copy", what, g.tname(t))
	}
}

// clPointer writes the copy of a pointer: what it points at is copied, so dst gets a new pointer.
func (g *deriveGen) clPointer(dst, src string, p *types.Pointer, what string) {
	elem := types.Unalias(p.Elem())
	g.open("if %s != nil {", src)
	c := g.fresh("c")
	switch {
	case g.hasClone(elem) && g.refs(elem) && !isInterface(elem):
		call := src
		if _, isTP := elem.(*types.TypeParam); isTP {
			call = "(*" + src + ")"
		}
		g.w("%s := %s.Clone()", c, call)
	case !g.refs(elem):
		g.w("%s := *%s", c, src)
	default:
		g.w("%s := *%s", c, src)
		g.cl(c, "(*"+src+")", elem, true, "*"+what)
	}
	g.w("%s = &%s", dst, c)
	g.close("}")
}
