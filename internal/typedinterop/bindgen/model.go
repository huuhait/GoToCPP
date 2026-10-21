package bindgen

import (
	"bytes"
	"encoding/json"
	"math/big"
	"strings"
)

// A gdecl is a declaration of the headers, organized for the generator.
type gdecl struct {
	kind  string // "record", "enum", "typedef", "func", "var", "template", "other"
	id    string
	scope []string // the names around it (namespaces, classes)
	name  string   // the C++ name, "" for an unnamed struct, union or enum
	tag   string   // struct, class, union, enum
	node  *jnode

	inScope bool // declared in a file of the view
	file    string
	seq     int

	complete bool     // a record with its definition; an enum with enumerators
	into     *gdecl   // a typedef that only names this tag: it is the tag
	named    []string // typedef names of an unnamed tag
	hidden   bool     // an implementation name (starts with _) not asked for
	filtered bool     // dropped by -exclude / -include

	goName string // the name in the view
	needed bool   // an emitted declaration refers to it
	done   bool   // already written
}

// qname returns the qualified C++ name.
func (d *gdecl) qname() string {
	if len(d.scope) == 0 {
		return d.name
	}
	return strings.Join(d.scope, "::") + "::" + d.name
}

// A table is the declarations of one translation unit.
type table struct {
	all    []*gdecl
	byID   map[string]*gdecl
	types  map[string]*gdecl   // C++ qualified name -> record, enum or typedef
	values map[string][]*gdecl // functions and variables
	seq    int
}

func newTable() *table {
	return &table{byID: map[string]*gdecl{}, types: map[string]*gdecl{}, values: map[string][]*gdecl{}}
}

// An addCtx says in which file each node of a top-level declaration is, and
// which files are in the view.
type addCtx struct {
	files   map[*jnode]string
	isScope func(file string) bool
}

// add records the declarations of one node (and what is inside a namespace,
// an extern "C" block or a class) in the table.
func (t *table) add(n *jnode, scope []string, x *addCtx) {
	if n.IsImplicit {
		return
	}
	file := x.files[n]
	inScope := x.isScope(file)
	switch n.Kind {
	case "NamespaceDecl":
		if n.Name == "" {
			return // an unnamed namespace holds names of one file only
		}
		sub := scope
		if !n.IsInline {
			sub = appendScope(scope, n.Name)
		}
		for i := range n.Inner {
			t.add(&n.Inner[i], sub, x)
		}
	case "LinkageSpecDecl":
		for i := range n.Inner {
			t.add(&n.Inner[i], scope, x)
		}
	case "RecordDecl", "CXXRecordDecl", "EnumDecl":
		d := t.newDecl(n, scope, file, inScope)
		d.kind = "record"
		d.tag = n.TagUsed
		if n.Kind == "EnumDecl" {
			d.kind = "enum"
			d.tag = "enum"
		}
		d.complete = n.CompleteDefinition || (n.Kind == "EnumDecl" && len(n.Inner) > 0)
		if d.name == "" {
			t.all = append(t.all, d)
			t.byID[d.id] = d
			return
		}
		// A redeclaration (a forward declaration, then the definition) is one
		// declaration; the complete one wins.
		if old := t.types[d.qname()]; old != nil && old.kind == d.kind && old.tag == d.tag {
			if d.complete && !old.complete {
				old.node, old.complete, old.file = d.node, true, d.file
				old.inScope = old.inScope || d.inScope

				// the forward declaration had no body, so the nested types are
				// the ones of the definition (typedef struct S S; struct S { struct N {...} *p; })
				t.addNested(d, scope, x)
			}
			t.byID[d.id] = old
			old.inScope = old.inScope || d.inScope
			return
		}
		t.all = append(t.all, d)
		t.byID[d.id] = d
		t.types[d.qname()] = d
		t.addNested(d, scope, x)
	case "TypedefDecl", "TypeAliasDecl":
		d := t.newDecl(n, scope, file, inScope)
		d.kind = "typedef"
		t.all = append(t.all, d)
		t.byID[d.id] = d
		if _, dup := t.types[d.qname()]; !dup {
			t.types[d.qname()] = d
		}
	case "FunctionDecl":
		d := t.newDecl(n, scope, file, inScope)
		d.kind = "func"
		t.all = append(t.all, d)
		t.values[d.qname()] = append(t.values[d.qname()], d)
	case "VarDecl":
		d := t.newDecl(n, scope, file, inScope)
		d.kind = "var"
		t.all = append(t.all, d)
		t.values[d.qname()] = append(t.values[d.qname()], d)
	case "ClassTemplateDecl", "FunctionTemplateDecl", "TypeAliasTemplateDecl", "VarTemplateDecl":
		d := t.newDecl(n, scope, file, inScope)
		d.kind = "template"
		t.all = append(t.all, d)
	}
}

// addNested records the types declared inside a class (the nested structs,
// enums and typedefs), which are named from the scope of the class. A type
// that is only declared there by an elaborated type specifier (struct B *p;)
// is not a member: it belongs to the namespace around the class.
func (t *table) addNested(d *gdecl, scope []string, x *addCtx) {
	if d.kind != "record" {
		return
	}
	sub := appendScope(scope, d.name)
	for i := range d.node.Inner {
		c := &d.node.Inner[i]
		switch c.Kind {
		case "CXXRecordDecl", "RecordDecl", "EnumDecl", "TypedefDecl", "TypeAliasDecl":
			if c.ParentDeclContextID != "" {
				t.add(c, t.namespaceOf(scope), x)
				continue
			}
			t.add(c, sub, x)
		}
	}
}

// namespaceOf drops the classes from the end of a scope: what is left is the
// namespace that a name declared by an elaborated type specifier lands in.
func (t *table) namespaceOf(scope []string) []string {
	for len(scope) > 0 {
		last := t.types[strings.Join(scope, "::")]
		if last == nil || last.kind != "record" {
			break
		}
		scope = scope[:len(scope)-1]
	}
	return scope
}

func appendScope(scope []string, name string) []string {
	out := make([]string, len(scope), len(scope)+1)
	copy(out, scope)
	return append(out, name)
}

func (t *table) newDecl(n *jnode, scope []string, file string, inScope bool) *gdecl {
	t.seq++
	return &gdecl{id: n.ID, scope: scope, name: n.Name, node: n, inScope: inScope, file: file, seq: t.seq}
}

// link connects the typedefs that only name a tag with the tag (typedef
// struct { ... } Color; typedef struct Vector2 Vector2;), so that the pair is
// one declaration with one name.
func (t *table) link() {
	for _, d := range t.all {
		if d.kind != "typedef" || d.node.Kind != "TypedefDecl" {
			continue
		}
		var tag *gdecl
		if len(d.node.Inner) > 0 {
			if o := d.node.Inner[0].OwnedTagDecl; o != nil {
				tag = t.byID[o.ID]
			}
		}
		if tag == nil {
			// typedef struct Foo Foo; (the tag was declared before, or by
			// this very line)
			q := d.node.qual()
			for _, kw := range []string{"struct ", "union ", "class ", "enum "} {
				if rest, ok := strings.CutPrefix(q, kw); ok {
					if c := t.lookupTag(d.scope, rest); c != nil && c.name == d.name {
						tag = c
					}
				}
			}
		}
		if tag == nil || tag == d {
			continue
		}
		switch {
		case tag.name == "":
			tag.name = d.name
			tag.scope = d.scope
			tag.named = append(tag.named, d.name)
			d.into = tag
			t.types[tag.qname()] = tag
		case tag.name == d.name && sameScope(tag.scope, d.scope):
			d.into = tag
			t.types[d.qname()] = tag
			tag.inScope = tag.inScope || d.inScope
		}
	}
}

// lookupTag finds the record or enum "name" as seen from scope.
func (t *table) lookupTag(scope []string, name string) *gdecl {
	for i := len(scope); i >= 0; i-- {
		q := name
		if i > 0 {
			q = strings.Join(scope[:i], "::") + "::" + name
		}
		if d := t.types[q]; d != nil && (d.kind == "record" || d.kind == "enum") {
			return d
		}
	}
	return nil
}

func sameScope(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// quickLoc reads the "loc" of a declaration without decoding it.
func quickLoc(data []byte) *jloc {
	head := data
	if len(head) > 8192 {
		head = head[:8192]
	}
	i := bytes.Index(head, []byte(`"loc":`))
	if i < 0 {
		return nil
	}
	var l jloc
	dec := json.NewDecoder(bytes.NewReader(data[i+len(`"loc":`):]))
	if err := dec.Decode(&l); err != nil {
		return nil
	}
	return &l
}

// enumerator values of an EnumDecl.
type enumerator struct {
	Name  string
	Value *big.Int
	Doc   string // the "Deprecated:" paragraph of a deprecated enumerator
}

// enumValues returns the enumerators of an enum with their values. Clang
// gives the value of an enumerator only where it has an initializer (as a
// ConstantExpr); the others are the previous one plus one.
func enumValues(n *jnode) []enumerator {
	var out []enumerator
	next := big.NewInt(0)
	for i := range n.Inner {
		c := &n.Inner[i]
		if c.Kind != "EnumConstantDecl" {
			continue
		}
		v := new(big.Int).Set(next)
		if s, ok := constantValue(c); ok {
			if x, ok := new(big.Int).SetString(s, 0); ok {
				v = x
			}
		}
		out = append(out, enumerator{Name: c.Name, Value: v, Doc: deprecation("", c)})
		next = new(big.Int).Add(v, big.NewInt(1))
	}
	return out
}

// constantValue finds the ConstantExpr under n and returns its value.
func constantValue(n *jnode) (string, bool) {
	for i := range n.Inner {
		c := &n.Inner[i]
		if c.Kind == "ConstantExpr" {
			if s, ok := c.valueString(); ok {
				return s, true
			}
		}
		if s, ok := constantValue(c); ok {
			return s, true
		}
	}
	return "", false
}
