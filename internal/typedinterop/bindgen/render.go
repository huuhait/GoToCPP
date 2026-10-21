package bindgen

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
)

// A gen writes the view of one header.
type gen struct {
	cfg *Config
	tab *table
	pp  *ppResult

	used       map[string]bool   // the Go names in use
	opaque     map[string]string // C++ spelling -> Go name of its stand-in
	opaqueList []string
	useUnsafe  bool

	skippedMacros []skippedMacro
	warnings      []string
	probeTime     time.Duration
	probeCached   bool

	pullQ     []*gdecl          // declarations to write because others use them
	pulled    map[string]string // Go name -> text of a referenced declaration
	enumGo    map[string]string // enum id/qualified enumerator -> Go name
	enumShort map[string]string // enumerator name -> Go name (first)
	anonSeq   int

	notes      []string // what the type being written lost (a decltype, a pointer to member, _Atomic)
	layout     []string // records whose layout the view cannot say (packed, over-aligned)
	layoutHere bool     // a field of the record being written has an alignment
	ctx        context.Context
	sizes      map[*gdecl][2]int // sizeof and alignof of the stand-in typedefs
	inline     string            // the body that an unnamed type in the spelling being written stands for
	pos        map[*gdecl]int    // the position of each declaration in tab.all
}

func newGen(cfg *Config, tab *table, pp *ppResult) *gen {
	return &gen{cfg: cfg, tab: tab, pp: pp, used: map[string]bool{}, opaque: map[string]string{},
		pulled: map[string]string{}, enumGo: map[string]string{}, enumShort: map[string]string{}}
}

// fresh returns base, or base with a number, as a Go name that is not used
// yet, and marks it used.
func (g *gen) fresh(base string) string {
	base = goIdent(base)
	name := base
	for i := 2; g.used[name]; i++ {
		name = fmt.Sprintf("%s_%d", base, i)
	}
	g.used[name] = true
	return name
}

// goBase is the name of a declaration in the view before conflicts: the
// namespaces and classes around it joined with underscores.
func goBase(d *gdecl) string {
	return goBaseOf(d.scope, d.name)
}

func goBaseOf(scope []string, name string) string {
	if len(scope) == 0 {
		return name
	}
	return strings.Join(scope, "_") + "_" + name
}

// visible says that the main pass writes the declaration: it is in a file of
// the view, it is not an implementation name, and the patterns allow it.
func (g *gen) visible(d *gdecl) bool {
	return d.inScope && !d.hidden && !d.filtered && d.into == nil
}

// classify sets hidden and filtered on the declarations.
func (g *gen) classify() {
	for _, d := range g.tab.all {
		d.hidden = strings.HasPrefix(d.name, "_") && d.kind != "template"
		d.filtered = !g.cfg.allows(d.qname())
		for _, s := range d.scope {
			if strings.HasPrefix(s, "_") {
				d.hidden = true
			}
		}
	}
}

// nameAll gives Go names to the declarations that the main pass writes.
// Ordinary names come first, then tags, so that a struct that is named like
// a function (struct stat and stat()) is the one that gets struct_stat.
func (g *gen) nameAll(consts []*constant) {
	for _, d := range g.tab.all {
		if !g.visible(d) {
			continue
		}
		switch d.kind {
		case "typedef", "var":
			d.goName = g.fresh(goBase(d))
		case "func":
			if d.name == "" || strings.HasPrefix(d.name, "operator") {
				continue
			}
			if first := g.firstOverload(d); first != d {
				continue
			}
			d.goName = g.fresh(goBase(d))
		}
	}
	for _, c := range consts {
		if c.Decl != nil {
			c.Go = c.Decl.goName
		} else {
			c.Go = g.fresh(c.Name)
		}
	}
	for _, d := range g.tab.all {
		if !g.visible(d) {
			continue
		}
		switch d.kind {
		case "record", "enum":
			if d.name == "" {
				continue
			}
			g.nameTag(d)
		}
	}
	// enumerators
	for _, d := range g.tab.all {
		if g.visible(d) && d.kind == "enum" {
			g.nameEnumerators(d)
		}
	}
}

// nameTag names a struct, union, class or enum.
func (g *gen) nameTag(d *gdecl) {
	if d.goName != "" {
		return
	}
	base := goBase(d)
	if g.used[goIdent(base)] {
		base = d.tag + "_" + base
	}
	d.goName = g.fresh(base)
}

func (g *gen) nameEnumerators(d *gdecl) {
	prefix := d.scope
	if d.node.ScopedEnumTag != "" && d.name != "" {
		prefix = appendScope(d.scope, d.name)
	}
	for _, e := range enumValues(d.node) {
		q := strings.Join(append(append([]string{}, prefix...), e.Name), "::")
		if _, ok := g.enumGo[d.id+"/"+q]; ok {
			continue
		}
		name := g.fresh(goBaseOf(prefix, e.Name))
		g.enumGo[d.id+"/"+q] = name
		if _, ok := g.enumShort[e.Name]; !ok {
			g.enumShort[e.Name] = name
		}
	}
}

// firstOverload returns the first function with the name of d (the others are
// written as comments: Go has no overloads).
func (g *gen) firstOverload(d *gdecl) *gdecl {
	for _, o := range g.tab.values[d.qname()] {
		if o.kind == "func" && g.visible(o) && !o.node.Deleted {
			return o
		}
	}
	return d
}

// declaredBefore says that an earlier declaration of the same name has the type of d, as C++ counts
// it: the same function again (glibc declares strtol twice in a row, the second time with an asm
// label), not an overload.
func (g *gen) declaredBefore(d *gdecl) bool {
	want := overloadKey(d)
	if want == "" {
		return false
	}
	for _, o := range g.tab.values[d.qname()] {
		if o == d {
			return false
		}
		if o.kind == "func" && g.visible(o) && !o.node.Deleted && overloadKey(o) == want {
			return true
		}
	}
	return false
}

// overloadKey is what makes a function a different overload: the parameter and result types, but
// not the names of the parameters, and not const or volatile on the parameter itself
// (void f(const int y) is void f(int x), void f(int *const p) is void f(int *p)). The qualifiers
// of what a parameter points to, references, and the result type keep overloads apart. It is ""
// when the type cannot be read.
func overloadKey(d *gdecl) string {
	ft, err := ctype.Parse(d.node.qual())
	if err != nil || ft.Kind != ctype.Func {
		return ""
	}
	ft = ft.Clone()
	for _, param := range ft.Params {
		// the qualifiers of the parameter itself; those of what it points to are in param.Elem
		param.Const = false
		param.Volatile = false
	}
	return ft.String()
}

// need marks a declaration as used by another one. One that the main pass
// does not write is queued to be written as a referenced declaration.
func (g *gen) need(d *gdecl) {
	if d.needed {
		return
	}
	d.needed = true
	if d.goName == "" {
		switch d.kind {
		case "record", "enum":
			if d.name == "" {
				return
			}
			g.nameTag(d)
		default:
			d.goName = g.fresh(goBase(d))
		}
	}
	if !g.visible(d) {
		g.pullQ = append(g.pullQ, d)
	}
}

// ---- type declarations

// declScope is the C++ scope that names inside d are looked up from: the
// scope of d, then d itself when it is a class.
func declScope(d *gdecl) []string {
	if d.kind == "record" && d.name != "" {
		return appendScope(d.scope, d.name)
	}
	return d.scope
}

// writeType writes a record, enum or typedef declaration to b.
func (g *gen) writeType(b *strings.Builder, d *gdecl) {
	if d.done {
		return
	}
	d.done = true
	switch d.kind {
	case "record":
		g.writeRecord(b, d)
	case "enum":
		g.writeEnum(b, d)
	case "typedef":
		g.writeTypedef(b, d)
	}
}

func (g *gen) cxxComment(d *gdecl) string {
	if d.name == "" {
		return ""
	}
	q := d.qname()
	switch {
	case d.kind == "record" || d.kind == "enum":
		if d.goName != q {
			return "// " + d.tag + " " + q + "\n"
		}
	case d.goName != q:
		return "// " + q + "\n"
	}
	return ""
}

func (g *gen) writeTypedef(b *strings.Builder, d *gdecl) {
	g.notes = nil
	ty, err := g.goTypeOfDecl(d, d.node.qual())
	if err != nil {
		_ = err
		fmt.Fprintf(b, "// typedef %s: the type %s has no Go spelling; written as a stand-in\n", d.qname(), scrub(d.node.qual()))
		g.standIn(b, d)
		return
	}
	note := g.noteLine()
	doc := deprecation(note+g.cxxComment(d), d.node)
	if ty == "struct{}" {
		b.WriteString(doc)
		fmt.Fprintf(b, "type %s struct{}\n\n", d.goName)
		return
	}
	b.WriteString(doc)
	fmt.Fprintf(b, "type %s = %s\n\n", d.goName, ty)
}

// untranslated writes a comment for a declaration that has no Go spelling.
func (g *gen) untranslated(b *strings.Builder, what string, err error) {
	fmt.Fprintf(b, "// %s: not translated (%s)\n\n", what, scrub(err.Error()))
}

func (g *gen) writeEnum(b *strings.Builder, d *gdecl) {
	vals := enumValues(d.node)
	under := g.enumUnderlying(d, vals)
	if d.name == "" {
		// an enum with no name is only its enumerators: the patterns choose among them
		var kept []enumerator
		for _, e := range vals {
			if g.cfg.allows(strings.Join(append(append([]string{}, d.scope...), e.Name), "::")) {
				kept = append(kept, e)
			}
		}
		vals = kept
	}
	if d.goName != "" {
		b.WriteString(deprecation(g.cxxComment(d), d.node))
		fmt.Fprintf(b, "type %s %s\n\n", d.goName, under)
	}
	if !d.complete || len(vals) == 0 {
		return
	}
	prefix := d.scope
	if d.node.ScopedEnumTag != "" && d.name != "" {
		prefix = appendScope(d.scope, d.name)
	}
	b.WriteString("const (\n")
	for _, e := range vals {
		q := strings.Join(append(append([]string{}, prefix...), e.Name), "::")
		name := g.enumGo[d.id+"/"+q]
		if name == "" {
			name = g.fresh(goBaseOf(prefix, e.Name))
			g.enumGo[d.id+"/"+q] = name
		}
		b.WriteString(e.Doc)
		if d.goName != "" {
			fmt.Fprintf(b, "\t%s %s = %s\n", name, d.goName, e.Value)
		} else {
			fmt.Fprintf(b, "\t%s = %s\n", name, e.Value)
		}
	}
	b.WriteString(")\n\n")
}

// enumUnderlying writes the integer type of an enum: the one it was declared
// with, else the one that C++ gives it (unsigned int unless a value is
// negative or too large).
func (g *gen) enumUnderlying(d *gdecl, vals []enumerator) string {
	if f := d.node.FixedUnderlying; f != nil {
		if s, err := g.goType(f.QualType, d.scope); err == nil {
			return s
		}
	}
	neg := false
	max := big.NewInt(0)
	min := big.NewInt(0)
	for _, e := range vals {
		if e.Value.Sign() < 0 {
			neg = true
		}
		if e.Value.Cmp(max) > 0 {
			max.Set(e.Value)
		}
		if e.Value.Cmp(min) < 0 {
			min.Set(e.Value)
		}
	}
	switch {
	case !neg && max.IsUint64() && max.Uint64() <= 0xFFFFFFFF:
		return "C.uint"
	case !neg:
		return "C.ulong"
	case min.IsInt64() && min.Int64() >= -1<<31 && max.IsInt64() && max.Int64() < 1<<31:
		return "C.int"
	}
	return "C.long"
}

// A field is one line of a struct.
type field struct {
	names   []string
	typ     string
	comment string
}

func (g *gen) writeRecord(b *strings.Builder, d *gdecl) {
	n := d.node
	scope := declScope(d)
	var head strings.Builder
	head.WriteString(g.cxxComment(d))
	if d.tag == "union" {
		head.WriteString("// union: the fields share their storage\n")
	}
	marks := attrMarks(n)
	if len(marks) > 0 {
		head.WriteString("// layout: " + strings.Join(marks, ", ") + "\n")
	}
	if !d.complete {
		head.WriteString("// incomplete type: declared, not defined\n")
		fmt.Fprintf(b, "%stype %s struct{}\n\n", deprecation(head.String(), n), d.goName)
		return
	}
	b.WriteString(deprecation(head.String(), n))
	fmt.Fprintf(b, "type %s ", d.goName)
	g.layoutHere = false
	body, methods, extras := g.structBody(n, scope, d.tag == "class", d.goName)
	if len(marks) > 0 || g.layoutHere {
		g.layout = append(g.layout, d.goName)
	}
	b.WriteString(body)
	b.WriteString("\n\n")
	if methods != "" {
		b.WriteString(methods)
		b.WriteString("\n")
	}
	if extras != "" {
		fmt.Fprintf(b, "// C++ members of %s that Go has no spelling for:\n", d.goName)
		for _, line := range strings.Split(extras, "\n") {
			b.WriteString("//\t" + strings.TrimPrefix(line, "// ") + "\n")
		}
		b.WriteString("\n")
	}
}

// structBody writes "struct { ... }" for the members of a record, and the Go
// methods of its public member functions, and comments for the members that
// have no Go spelling.
func (g *gen) structBody(n *jnode, scope []string, isClass bool, recv string) (body, methods, extras string) {
	access := "public"
	if isClass {
		access = "private"
	}
	var fields []field
	var lastAnon *jnode
	var mb, xb strings.Builder
	seenMethod := map[string]bool{}

	for _, base := range n.Bases {
		if base.Access != "public" {
			continue
		}
		ty, err := g.goType(base.Type.QualType, scope)
		if err != nil || strings.HasPrefix(ty, "*") {
			continue
		}
		fields = append(fields, field{names: nil, typ: ty, comment: "base class"})
	}
	for i := range n.Inner {
		c := &n.Inner[i]
		switch c.Kind {
		case "AccessSpecDecl":
			access = c.Access
		case "CXXRecordDecl", "RecordDecl", "EnumDecl":
			if c.Name == "" && !c.IsImplicit {
				lastAnon = c
			}
		case "FieldDecl":
			if access != "public" {
				continue
			}
			f := g.field(c, lastAnon, scope)
			if f != nil {
				fields = append(fields, *f)
			}
		case "VarDecl":
			if access == "public" {
				ty, err := g.goType(c.qual(), scope)
				if err == nil {
					fmt.Fprintf(&xb, "// static member %s %s\n", c.Name, ty)
				}
			}
		case "CXXMethodDecl", "CXXConstructorDecl", "CXXDestructorDecl", "CXXConversionDecl":
			if access != "public" || c.IsImplicit || c.Deleted {
				continue
			}
			g.method(&mb, &xb, c, scope, recv, seenMethod)
		case "FunctionTemplateDecl":
			if access == "public" {
				fmt.Fprintf(&xb, "// member template %s\n", c.Name)
			}
		}
	}
	// group neighbours of the same type: a, b C.int
	var out []field
	for _, f := range fields {
		if k := len(out) - 1; k >= 0 && len(f.names) == 1 && len(out[k].names) > 0 && out[k].typ == f.typ && f.comment == "" && out[k].comment == "" &&
			len(strings.Join(out[k].names, ", "))+len(f.names[0]) < 40 {
			out[k].names = append(out[k].names, f.names...)
			continue
		}
		out = append(out, f)
	}
	var sb strings.Builder
	if len(out) == 0 {
		sb.WriteString("struct{}")
	} else {
		sb.WriteString("struct {\n")
		for _, f := range out {
			sb.WriteString("\t")
			if len(f.names) > 0 {
				sb.WriteString(strings.Join(f.names, ", ") + " ")
			}
			sb.WriteString(f.typ)
			if f.comment != "" {
				sb.WriteString(" // " + f.comment)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("}")
	}
	return sb.String(), strings.TrimRight(mb.String(), "\n"), strings.TrimRight(xb.String(), "\n")
}

// field writes one data member.
func (g *gen) field(c *jnode, anon *jnode, scope []string) *field {
	spelling := c.qual()
	unnamedType := strings.Contains(spelling, "(unnamed") || strings.Contains(spelling, "(anonymous")
	var ty string
	var comment string
	switch {
	case unnamedType && anon != nil:
		// the unnamed struct, union or enum is written inline, inside the declarators of the field
		var body string
		if anon.Kind == "EnumDecl" {
			body = g.enumUnderlying(&gdecl{node: anon, scope: scope}, enumValues(anon))
			comment = "unnamed enum"
		} else {
			body, _, _ = g.structBody(anon, scope, false, "")
			comment = "anonymous " + anon.TagUsed
			if c.Name != "" {
				comment = "unnamed " + anon.TagUsed + " type"
			}
		}
		var err error
		if c.Name == "" {
			ty = body // the implicit member for an anonymous union or struct: its type is the body
		} else if ty, err = g.goTypeInline(spelling, scope, body); err != nil {
			ty = body
		}
	default:
		var err error
		g.notes = nil
		ty, err = g.goType(spelling, scope)
		if err != nil {
			return &field{names: []string{goIdent(c.Name)}, typ: "struct{}", comment: "not translated: " + scrub(spelling)}
		}
		if n := g.takeNotes(); n != "" {
			comment = n
		}
	}
	if marks := attrMarks(c); len(marks) > 0 {
		g.layoutHere = true
		if comment != "" {
			comment += ", "
		}
		comment += strings.Join(marks, ", ")
	}
	if c.Name == "" {
		// a member of an anonymous union or struct: its members are the
		// members of the enclosing struct in C++
		g.anonSeq++
		name := fmt.Sprintf("anon%d", g.anonSeq)
		return &field{names: []string{name}, typ: ty, comment: comment + ": its members are used directly in C++"}
	}
	if c.IsBitfield {
		w := "?"
		if v, ok := constantValue(c); ok {
			w = v
		} else if len(c.Inner) > 0 {
			if s, ok := c.Inner[0].valueString(); ok {
				w = s
			}
		}
		if comment != "" {
			comment += ", "
		}
		unit := " bits"
		if w == "1" {
			unit = " bit"
		}
		comment += "bit field of " + w + unit
	}
	return &field{names: []string{goIdent(c.Name)}, typ: ty, comment: comment}
}

// method writes a member function as a Go method (mb), or as a comment (xb)
// when Go has no spelling for it.
func (g *gen) method(mb, xb *strings.Builder, c *jnode, scope []string, recv string, seen map[string]bool) {
	ft, err := ctype.Parse(c.qual())
	if err != nil || ft.Kind != ctype.Func {
		fmt.Fprintf(xb, "// member %s: not translated (%s)\n", c.Name, scrub(c.qual()))
		return
	}
	names := paramNames(c)
	sig := func(withRecv bool) string {
		var b strings.Builder
		b.WriteString("func ")
		if withRecv {
			if ft.ConstQ {
				fmt.Fprintf(&b, "(this %s) ", recv)
			} else {
				fmt.Fprintf(&b, "(this *%s) ", recv)
			}
		}
		return b.String()
	}
	params := g.params(ft, scope, names)
	res := g.result(ft, scope)
	tail := ""
	if res != "" {
		tail = " " + res
	}
	switch {
	case c.Kind == "CXXConstructorDecl":
		fmt.Fprintf(xb, "// constructor %s(%s)\n", recvBase(recv), params)
	case c.Kind == "CXXDestructorDecl":
		fmt.Fprintf(xb, "// destructor ~%s()\n", recvBase(recv))
	case c.Kind == "CXXConversionDecl":
		fmt.Fprintf(xb, "// conversion %s() %s\n", c.Name, res)
	case strings.HasPrefix(c.Name, "operator"):
		fmt.Fprintf(xb, "// %s(%s)%s\n", c.Name, params, tail)
	case c.StorageClass == "static":
		fmt.Fprintf(xb, "// static method %s(%s)%s\n", c.Name, params, tail)
	default:
		name := goIdent(c.Name)
		line := fmt.Sprintf("%s%s(%s)%s", sig(true), name, params, tail)
		if seen[name] {
			fmt.Fprintf(xb, "// overload: %s\n", strings.TrimPrefix(line, "func "))
			return
		}
		seen[name] = true
		if c.Virtual {
			line += " // virtual"
			if c.Pure {
				line += ", pure"
			}
		}
		mb.WriteString(line + "\n")
	}
}

// recvBase is the name of a type for the comments of a constructor.
func recvBase(recv string) string { return recv }

// paramNames returns the names of the ParmVarDecl children of a function.
func paramNames(n *jnode) []string {
	var names []string
	for i := range n.Inner {
		if n.Inner[i].Kind == "ParmVarDecl" {
			names = append(names, n.Inner[i].Name)
		}
	}
	return names
}

// ---- functions and variables

// writeFunc writes a free function.
func (g *gen) writeFunc(b *strings.Builder, d *gdecl) {
	n := d.node
	if n.Deleted {
		return
	}
	ft, err := ctype.Parse(n.qual())
	if err != nil || ft.Kind != ctype.Func {
		g.untranslated(b, "function "+d.qname(), fmt.Errorf("type %s", scrub(n.qual())))
		return
	}
	g.notes = nil
	params := g.params(ft, d.scope, paramNames(n))
	res := g.result(ft, d.scope)
	tail := ""
	if res != "" {
		tail = " " + res
	}
	if strings.HasPrefix(d.name, "operator") {
		fmt.Fprintf(b, "// %s(%s)%s\n\n", d.qname(), params, tail)
		return
	}
	if d.goName == "" {
		if g.declaredBefore(d) {
			return
		}

		// an overload of a function that is already written
		fmt.Fprintf(b, "// overload: %s(%s)%s\n\n", d.qname(), params, tail)
		return
	}
	note := g.noteLine()
	b.WriteString(deprecation(note+g.cxxComment(d), n))
	fmt.Fprintf(b, "func %s(%s)%s\n", d.goName, params, tail)
}

// writeVar writes a global variable.
func (g *gen) writeVar(b *strings.Builder, d *gdecl) {
	g.notes = nil
	ty, err := g.goTypeOfDecl(d, strings.TrimPrefix(d.node.qual(), "const "))
	if err != nil {
		g.untranslated(b, "variable "+d.qname(), err)
		return
	}
	note := g.noteLine()
	b.WriteString(deprecation(note+g.cxxComment(d), d.node))
	fmt.Fprintf(b, "var %s %s\n", d.goName, ty)
}

// writeTemplate lists a template (Go has no spelling for it).
func (g *gen) writeTemplate(b *strings.Builder, d *gdecl) {
	n := d.node
	var params []string
	what := "template"
	for i := range n.Inner {
		c := &n.Inner[i]
		switch c.Kind {
		case "TemplateTypeParmDecl":
			params = append(params, "typename "+c.Name)
		case "NonTypeTemplateParmDecl":
			params = append(params, strings.TrimSpace(c.qual()+" "+c.Name))
		case "TemplateTemplateParmDecl":
			params = append(params, "template "+c.Name)
		case "CXXRecordDecl":
			if c.TagUsed != "" {
				what = c.TagUsed
			}
		case "FunctionDecl":
			what = "function"
		case "TypeAliasDecl":
			what = "alias"
		case "VarDecl":
			what = "variable"
		}
	}
	fmt.Fprintf(b, "// %s template %s<%s>\n", what, d.qname(), strings.Join(params, ", "))
}

// ---- patterns

func (c *Config) allows(name string) bool {
	if name == "" {
		return true
	}
	for _, re := range c.exclude {
		if re.MatchString(name) {
			return false
		}
	}
	if len(c.include) == 0 {
		return true
	}
	for _, re := range c.include {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

// sortedKeys returns the keys of m in order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// isUnnamedSpelling says that a type's spelling names an unnamed struct, union or
// enum, which clang prints with the place where it is written.
func isUnnamedSpelling(s string) bool {
	return strings.Contains(s, "(unnamed") || strings.Contains(s, "(anonymous")
}

// goTypeInline is goType for a spelling that has an unnamed type in it: the
// unnamed type is written as body, inside the declarators around it.
func (g *gen) goTypeInline(spelling string, scope []string, body string) (string, error) {
	old := g.inline
	g.inline = body
	defer func() { g.inline = old }()
	return g.goType(spelling, scope)
}

// goTypeOfDecl writes the type spelling of a variable or typedef d. When it has
// an unnamed type in it, the type is the unnamed struct, union or enum declared
// just before d, written inline.
func (g *gen) goTypeOfDecl(d *gdecl, spelling string) (string, error) {
	if !isUnnamedSpelling(spelling) {
		return g.goType(spelling, d.scope)
	}
	if body := g.inlineBefore(d); body != "" {
		return g.goTypeInline(spelling, d.scope, body)
	}
	return g.goType(spelling, d.scope)
}

// inlineBefore writes the unnamed struct, union or enum that was declared just
// before d in the same scope, and that no typedef names, as an inline type ("" if none).
func (g *gen) inlineBefore(d *gdecl) string {
	if g.pos == nil {
		g.pos = map[*gdecl]int{}
		for i, x := range g.tab.all {
			g.pos[x] = i
		}
	}
	for i := g.pos[d] - 1; i >= 0; i-- {
		x := g.tab.all[i]
		if x.kind != "record" && x.kind != "enum" || x.name != "" || len(x.named) > 0 || !sameScope(x.scope, d.scope) {
			continue
		}
		if x.kind == "enum" {
			return g.enumUnderlying(x, enumValues(x.node))
		}
		body, _, _ := g.structBody(x.node, d.scope, false, "")
		return body
	}
	return ""
}

// standIn writes a type for a typedef that has no Go spelling (a vector type, a
// _BitInt), so that the declarations that use it still check: an array of bytes
// of the size the compiler gives it (sizeof, asked of the compiler; the tree's own
// reckoning when that run is not possible), with its alignment in a comment since a
// byte array has none; an empty struct when the size is not known.
func (g *gen) standIn(b *strings.Builder, d *gdecl) {
	size, align := g.sizeAlign(d)
	if size <= 0 {
		size = typedefSize(d.node)
	}
	if size > 0 {
		if align > 1 {
			fmt.Fprintf(b, "// aligned(%d)\n", align)
		}
		fmt.Fprintf(b, "type %s [%d]byte\n\n", d.goName, size)
		return
	}
	fmt.Fprintf(b, "type %s struct{}\n\n", d.goName)
}

// hasStandInShape says that the tree of a typedef is a vector type or a _BitInt.
func hasStandInShape(n *jnode) bool { return typedefSize(n) > 0 }

// sizeAlign returns sizeof and alignof of a typedef, asked of the compiler (0, 0 when
// it cannot be asked).
func (g *gen) sizeAlign(d *gdecl) (size, align int) {
	if _, ok := g.sizes[d]; !ok {
		g.askSizes([]*gdecl{d})
	}
	return g.sizes[d][0], g.sizes[d][1]
}

// askSizes runs the compiler once for the sizes and alignments of the typedefs.
func (g *gen) askSizes(ds []*gdecl) {
	if g.sizes == nil {
		g.sizes = map[*gdecl][2]int{}
	}
	var src strings.Builder
	src.WriteString(g.cfg.includeLine())
	src.WriteString("template <unsigned long N> struct gpbg_n {};\n")
	for i, d := range ds {
		g.sizes[d] = [2]int{} // asked: a failure stays 0
		name := "::" + d.qname()
		fmt.Fprintf(&src, "using gpbg_s_%d = gpbg_n<sizeof(%s)>;\nusing gpbg_a_%d = gpbg_n<alignof(%s)>;\n", i, name, i, name)
	}
	t := g.cfg.tool()
	path, err := t.source("sizes.cpp", src.String())
	if err != nil {
		return
	}
	got := map[string]int{}
	mode := []string{"-fsyntax-only", "-Xclang", "-ast-dump=json", "-Xclang", "-ast-dump-filter=gpbg_"}
	t.run(g.ctx, mode, path, func(r io.Reader) error {
		dec := json.NewDecoder(r)
		for {
			n := new(jnode)
			if err := dec.Decode(n); err != nil {
				return nil
			}
			if n.Kind == "TypeAliasDecl" && strings.HasPrefix(n.Name, "gpbg_") {
				if v, ok := constantValue(n); ok {
					x, _ := strconv.Atoi(v)
					got[n.Name] = x
				}
			}
		}
	})
	for i, d := range ds {
		g.sizes[d] = [2]int{got[fmt.Sprintf("gpbg_s_%d", i)], got[fmt.Sprintf("gpbg_a_%d", i)]}
	}
}

// typedefSize returns the size of a typedef of a vector type or a _BitInt, or 0.
func typedefSize(n *jnode) int {
	for i := range n.Inner {
		c := &n.Inner[i]
		switch c.Kind {
		case "VectorType", "ExtVectorType":
			if len(c.Inner) == 1 && c.NumElements > 0 {
				if e := builtinSize(c.Inner[0].qual()); e > 0 {
					return e * c.NumElements
				}
			}
		case "BitIntType":
			q := strings.TrimPrefix(c.qual(), "unsigned ")
			var bits int
			if _, err := fmt.Sscanf(q, "_BitInt(%d)", &bits); err == nil && bits > 0 {
				bytes := (bits + 7) / 8
				if bytes <= 8 {
					for p := 1; ; p *= 2 {
						if p >= bytes {
							return p
						}
					}
				}
				return (bytes + 7) / 8 * 8
			}
		}
	}
	return 0
}

// builtinSize is the size in bytes of a builtin number type, 0 if unknown.
func builtinSize(q string) int {
	if bits, _, ok := intBits(q); ok && bits > 1 {
		return bits / 8
	}
	switch q {
	case "float":
		return 4
	case "double":
		return 8
	case "long double":
		return 16
	}
	return 0
}

// attrMarks lists the layout attributes of a declaration, as clang's tree has them.
func attrMarks(n *jnode) []string {
	var out []string
	for i := range n.Inner {
		c := &n.Inner[i]
		switch c.Kind {
		case "PackedAttr":
			out = append(out, "packed")
		case "AlignedAttr":
			if v, ok := constantValue(c); ok {
				out = append(out, "aligned("+v+")")
			} else {
				out = append(out, "aligned")
			}
		}
	}
	return out
}

// deprecation returns the comment of a declaration, with the paragraph "Deprecated: ..." added when
// the header marks the declaration deprecated ([[deprecated("use New")]] or
// __attribute__((deprecated))). Go tools only read the word at the start of a paragraph, so a bare
// "//" line separates it from the comment that is already there (comment: lines that each end with
// a newline; it may be empty).
func deprecation(comment string, n *jnode) string {
	for i := range n.Inner {
		c := &n.Inner[i]
		if c.Kind != "DeprecatedAttr" {
			continue
		}
		// one line, whatever the message has in it
		text := strings.Join(strings.Fields(c.Message), " ")
		if text == "" {
			text = "the header marks it deprecated."
		}
		if comment != "" {
			comment += "//\n"
		}
		return comment + "// Deprecated: " + scrub(text) + "\n"
	}
	return comment
}

// note remembers that the type being written lost something.
func (g *gen) note(s string) {
	if !slices.Contains(g.notes, s) {
		g.notes = append(g.notes, s)
	}
}

// noteLine is the comment line for the notes, or "".
func (g *gen) noteLine() string {
	if s := g.takeNotes(); s != "" {
		return "// " + scrub(s) + "\n"
	}
	return ""
}

// takeNotes returns the notes as one line text and forgets them.
func (g *gen) takeNotes() string {
	s := strings.Join(g.notes, "; ")
	g.notes = nil
	return s
}

// stripAtomic writes _Atomic(T) as T, remembering it.
func (g *gen) stripAtomic(s string) string {
	for {
		i := strings.Index(s, "_Atomic(")
		if i < 0 {
			return s
		}
		depth, j := 0, i+len("_Atomic")
		for ; j < len(s); j++ {
			if s[j] == '(' {
				depth++
			} else if s[j] == ')' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		if j >= len(s) {
			return s
		}
		s = s[:i] + s[i+len("_Atomic("):j] + s[j+1:]
		g.note("_Atomic")
	}
}
