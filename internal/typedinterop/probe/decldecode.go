package probe

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
)

// dnode is the part of a node of clang's JSON dump that the declaration reader
// uses.
type dnode struct {
	ID                 string `json:"id"`
	Kind               string `json:"kind"`
	Name               string `json:"name"`
	IsImplicit         bool   `json:"isImplicit"`
	Loc                *dloc  `json:"loc"`
	TagUsed            string `json:"tagUsed"`
	CompleteDefinition bool   `json:"completeDefinition"`
	Type               *struct {
		QualType          string `json:"qualType"`
		DesugaredQualType string `json:"desugaredQualType"`
	} `json:"type"`
	StorageClass        string          `json:"storageClass"`
	Virtual             bool            `json:"virtual"`
	Pure                bool            `json:"pure"`
	Constexpr           bool            `json:"constexpr"`
	Variadic            bool            `json:"variadic"`
	ExplicitlyDeleted   bool            `json:"explicitlyDeleted"`
	ExplicitlyDefaulted json.RawMessage `json:"explicitlyDefaulted"`
	IsBitfield          bool            `json:"isBitfield"`
	Mutable             bool            `json:"mutable"`
	Access              string          `json:"access"`
	ScopedEnumTag       string          `json:"scopedEnumTag"`
	Value               json.RawMessage `json:"value"`
	Init                string          `json:"init"`
	Bases               []struct {
		Access    string `json:"access"`
		IsVirtual bool   `json:"isVirtual"`
		Type      *struct {
			QualType          string `json:"qualType"`
			DesugaredQualType string `json:"desugaredQualType"`
		} `json:"type"`
	} `json:"bases"`
	DefinitionData map[string]json.RawMessage `json:"definitionData"`
	Decl           *struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		Name string `json:"name"`
	} `json:"decl"`
	Target *struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		Name string `json:"name"`
		Type *struct {
			QualType          string `json:"qualType"`
			DesugaredQualType string `json:"desugaredQualType"`
		} `json:"type"`
	} `json:"target"`
	Inner []dnode `json:"inner"`
}

// dloc is the location of a node in clang's JSON dump: the file is named in it, or in the places
// that a macro expansion spells and expands it.
type dloc struct {
	File         string `json:"file"`
	SpellingLoc  *dloc  `json:"spellingLoc"`
	ExpansionLoc *dloc  `json:"expansionLoc"`
}

// files returns the names of the files that the location mentions.
func (l *dloc) files() []string {
	if l == nil {
		return nil
	}
	var out []string
	if l.File != "" {
		out = append(out, l.File)
	}
	out = append(out, l.SpellingLoc.files()...)
	return append(out, l.ExpansionLoc.files()...)
}

func (n *dnode) qual() string {
	if n.Type == nil {
		return ""
	}
	return normalizeType(n.Type.QualType)
}

// canon returns the desugared type when clang has one (the top-level typedefs
// resolved), else the written type.
func (n *dnode) canon() string {
	if n.Type == nil {
		return ""
	}
	if n.Type.DesugaredQualType != "" {
		return normalizeType(n.Type.DesugaredQualType)
	}
	return normalizeType(n.Type.QualType)
}

func (n *dnode) defaulted() bool {
	var s string
	if json.Unmarshal(n.ExplicitlyDefaulted, &s) == nil {
		return s == "default"
	}
	return false
}

func (n *dnode) deletedDefault() bool {
	var s string
	if json.Unmarshal(n.ExplicitlyDefaulted, &s) == nil {
		return s == "deleted"
	}
	return false
}

// constValue reads the value of a ConstantExpr node.
func (n *dnode) constValue() (*big.Int, bool) {
	var s string
	if json.Unmarshal(n.Value, &s) != nil {
		return nil, false
	}
	return new(big.Int).SetString(s, 10)
}

// firstConst returns the value of the first ConstantExpr below n.
func (n *dnode) firstConst() (*big.Int, bool) {
	if n.Kind == "ConstantExpr" {
		if v, ok := n.constValue(); ok {
			return v, true
		}
	}
	for i := range n.Inner {
		if v, ok := n.Inner[i].firstConst(); ok {
			return v, true
		}
	}
	return nil, false
}

// find returns the node with the id below n (n itself included), if its
// kind is one that a record or enum can be.
func (n *dnode) find(id string) *dnode {
	if n.ID == id {
		switch n.Kind {
		case "CXXRecordDecl", "ClassTemplateSpecializationDecl", "EnumDecl", "RecordDecl":
			return n
		}
	}
	for i := range n.Inner {
		if f := n.Inner[i].find(id); f != nil {
			return f
		}
	}
	return nil
}

// declIDOf returns the id of the record or enum that the type alias node
// stands for (its canonical type), following the sugar down the last child of
// each node: elaborated types, typedefs, template-specialization types (whose
// last child is the canonical type).
func declIDOf(alias *dnode) (id, kind string) {
	if len(alias.Inner) == 0 {
		return "", ""
	}
	n := &alias.Inner[len(alias.Inner)-1]
	for depth := 0; depth < 64; depth++ {
		switch n.Kind {
		case "RecordType", "EnumType":
			if n.Decl != nil {
				return n.Decl.ID, n.Decl.Kind
			}
			return "", ""
		case "ElaboratedType", "TypedefType", "UsingType", "DecayedType", "SubstTemplateTypeParmType", "TemplateSpecializationType", "ParenType", "AttributedType":
			if len(n.Inner) == 0 {
				return "", ""
			}
			n = &n.Inner[len(n.Inner)-1]
		default:
			return "", ""
		}
	}
	return "", ""
}

// An env resolves the names that a member's type uses relative to the record:
// its typedefs, its nested types, its own name.
type env struct {
	self  *ctype.Type
	last  string
	names map[string]*ctype.Type
}

// parseIn reads a type text and resolves the record-relative names in it.
func (e *env) parseIn(text string) TypeText {
	tt := TypeText{Text: text}
	t, err := ctype.Parse(text)
	if err != nil {
		tt.Err = err.Error()
		return tt
	}
	if e != nil {
		for i := 0; i < 4; i++ {
			var changed bool
			t, changed = e.resolve(t)
			if !changed {
				break
			}
		}
	}
	tt.T = t
	return tt
}

// resolve replaces the single-part names of t that are member typedefs, nested
// types or the injected class name of the record, one level.
func (e *env) resolve(t *ctype.Type) (*ctype.Type, bool) {
	if t == nil {
		return nil, false
	}
	changed := false
	c := *t
	if t.Kind == ctype.Named && len(t.Path) == 1 && !t.Global && t.Tag == "" {
		seg := t.Path[0]
		var sub *ctype.Type
		if !seg.HasArgs {
			sub = e.names[seg.Name]
		}
		if sub == nil && seg.Name == e.last && e.self != nil && (!seg.HasArgs || sameArgs(seg.Args, e.self.Args())) {
			sub = e.self // vector<long> or vector inside std::vector<long>, the injected class name
		}
		if sub != nil {
			n := sub.Clone()
			n.Const = n.Const || t.Const
			n.Volatile = n.Volatile || t.Volatile
			n.Pack = t.Pack
			return n, true
		}
	}
	if t.Elem != nil {
		var ch bool
		c.Elem, ch = e.resolve(t.Elem)
		changed = changed || ch
	}
	if t.Class != nil {
		var ch bool
		c.Class, ch = e.resolve(t.Class)
		changed = changed || ch
	}
	if len(t.Params) > 0 {
		c.Params = make([]*ctype.Type, len(t.Params))
		for i, p := range t.Params {
			var ch bool
			c.Params[i], ch = e.resolve(p)
			changed = changed || ch
		}
	}
	if len(t.Path) > 0 {
		c.Path = make([]ctype.Seg, len(t.Path))
		for i, s := range t.Path {
			ns := ctype.Seg{Name: s.Name, HasArgs: s.HasArgs}
			for _, a := range s.Args {
				na := a
				if a.Type != nil {
					var ch bool
					na.Type, ch = e.resolve(a.Type)
					changed = changed || ch
				}
				ns.Args = append(ns.Args, na)
			}
			c.Path[i] = ns
		}
	}
	return &c, changed
}

func accessOf(s string, def Access) Access {
	switch s {
	case "public":
		return Public
	case "protected":
		return Protected
	case "private":
		return Private
	}
	return def
}

// decodeRecord reads a class, struct or union declaration. spelling is the
// canonical name that was asked for.
func decodeRecord(n *dnode, spelling string) (*Record, error) {
	self, err := ctype.Parse(spelling)
	if err != nil || self.Kind != ctype.Named {
		return nil, fmt.Errorf("%q is not a type name", spelling)
	}
	r := &Record{Name: spelling, Scope: scopeOf(self), Tag: n.TagUsed, Complete: n.CompleteDefinition,
		Size: -1, Align: -1, Template: n.Kind == "ClassTemplateSpecializationDecl"}
	if r.Tag == "" {
		r.Tag = "struct"
	}
	for k, v := range n.DefinitionData {
		var b bool
		if json.Unmarshal(v, &b) != nil || !b {
			continue
		}
		name := ""
		switch k {
		case "isAggregate":
			name = "aggregate"
		case "isPolymorphic":
			name = "polymorphic"
		case "isAbstract":
			name = "abstract"
		case "isEmpty":
			name = "empty"
		case "isStandardLayout":
			name = "standardLayout"
		case "isTrivial":
			name = "trivial"
		case "isTriviallyCopyable":
			name = "triviallyCopyable"
		case "isPOD":
			name = "pod"
		}
		if name != "" {
			if r.Traits == nil {
				r.Traits = make(map[string]bool)
			}
			r.Traits[name] = true
		}
	}
	e := &env{self: self, last: self.Last(), names: make(map[string]*ctype.Type)}
	for _, b := range n.Bases {
		base := Base{Access: accessOf(b.Access, Public), Virtual: b.IsVirtual}
		if b.Type != nil {
			text := b.Type.DesugaredQualType
			if text == "" {
				text = b.Type.QualType
			}
			base.Type = e.parseIn(normalizeType(text))
		}
		r.Bases = append(r.Bases, base)
	}
	// The member typedefs and nested types first: the types of the members use them.
	for i := range n.Inner {
		c := &n.Inner[i]
		switch c.Kind {
		case "TypedefDecl", "TypeAliasDecl":
			if c.Name != "" {
				if r.Types == nil {
					r.Types = make(map[string]string)
				}
				r.Types[c.Name] = c.canon()
				if t, err := ctype.Parse(c.canon()); err == nil {
					e.names[c.Name] = t
				}
			}
		case "CXXRecordDecl", "EnumDecl", "RecordDecl":
			if c.Name != "" && !c.IsImplicit {
				r.Nested = append(r.Nested, c.Name)
				q := self.Clone()
				q.Path = append(q.Path, ctype.Seg{Name: c.Name})
				e.names[c.Name] = q
			}
		}
	}
	access := Private
	if r.Tag != "class" {
		access = Public
	}
	var anon *dnode   // an anonymous union or struct member waiting for its field
	anonUsed := false // a field with a name followed it: a declarator of the unnamed type (struct { ... } pos;)
	unmodelled := ""
	for i := range n.Inner {
		c := &n.Inner[i]
		switch c.Kind {
		case "AccessSpecDecl":
			access = accessOf(c.Access, access)
		case "FieldDecl":
			if c.Name == "" && anon != nil {
				// the implicit field of an anonymous union or struct: its members are the record's
				if fs, ok := anonFields(anon, e, access, false, 1); ok {
					r.Fields = append(r.Fields, fs...)
				} else {
					unmodelled = "an anonymous member that the extraction cannot model"
				}
				anon = nil
				continue
			}
			if c.Name == "" {
				continue // an unnamed bit-field (padding)
			}
			anonUsed = anonUsed || namesUnnamed(c)
			r.Fields = append(r.Fields, fieldOf(c, e, access))
		case "TypedefDecl", "TypeAliasDecl":
			// typedef struct { ... } Inner;  using X = struct { ... };
			anonUsed = anonUsed || anon != nil
		case "CXXRecordDecl", "RecordDecl":
			if c.Name == "" && !c.IsImplicit && (c.TagUsed == "union" || c.TagUsed == "struct") {
				if anon != nil && !anonUsed {
					unmodelled = "an anonymous member that the extraction cannot model"
				}
				anon, anonUsed = c, false
			}
		case "VarDecl":
			anonUsed = anonUsed || anon != nil // static constexpr struct { ... } cfg{};
			if c.StorageClass == "static" {
				r.Statics = append(r.Statics, Static{Name: c.Name, Type: e.parseIn(c.qual()), Access: access, Constexpr: c.Constexpr})
			}
		case "CXXMethodDecl", "CXXConstructorDecl", "CXXDestructorDecl", "CXXConversionDecl":
			r.Methods = append(r.Methods, methodOf(c, e, access, false))
		case "FunctionTemplateDecl":
			for j := range c.Inner {
				m := &c.Inner[j]
				switch m.Kind {
				case "CXXMethodDecl", "CXXConstructorDecl", "CXXDestructorDecl", "CXXConversionDecl":
					r.Methods = append(r.Methods, methodOf(m, e, access, true))
				}
			}
		}
	}
	if anon != nil && !anonUsed {
		unmodelled = "an anonymous member that the extraction cannot model"
	}
	if unmodelled != "" {
		// Members would be missing: say so, instead of reporting a complete record that has
		// fewer members than the compiler's (the checker would answer "no member lo").
		r.Complete, r.Partial = false, unmodelled
	}
	return r, nil
}

// namesUnnamed reports whether the type of a field is a type with no name: the field is then
// the declarator of an unnamed struct or union (struct { int x, y; } pos;), not a member that
// follows an anonymous one.
func namesUnnamed(c *dnode) bool {
	q := c.qual()
	return strings.Contains(q, "(unnamed") || strings.Contains(q, "(anonymous")
}

// anonFields returns the fields of an anonymous union or struct member, with those of the
// anonymous unions and structs inside it, at any depth (a struct in a union in a struct): all
// of them are members of the enclosing record. Union says that the field shares its storage with
// others (an enclosing anonymous aggregate is a union). It is false when something in it cannot
// be modelled.
func anonFields(a *dnode, e *env, access Access, union bool, depth int) ([]Field, bool) {
	if depth > 16 {
		return nil, false
	}
	union = union || a.TagUsed == "union"
	var out []Field
	var pending *dnode
	used := false // a named field followed pending: it is the type of that field, not a member
	for i := range a.Inner {
		c := &a.Inner[i]
		switch c.Kind {
		case "FieldDecl":
			switch {
			case c.Name == "" && pending != nil:
				sub, ok := anonFields(pending, e, access, union, depth+1)
				if !ok {
					return nil, false
				}
				out = append(out, sub...)
				pending = nil
			case c.Name == "":
				if !c.IsBitfield {
					return nil, false // an unnamed field of a type we have not seen
				}
			default:
				used = used || namesUnnamed(c)
				f := fieldOf(c, e, access)
				f.Anon, f.Union = true, union
				out = append(out, f)
			}
		case "TypedefDecl", "TypeAliasDecl", "VarDecl":
			used = used || pending != nil
		case "CXXRecordDecl", "RecordDecl":
			if c.Name == "" && !c.IsImplicit && (c.TagUsed == "union" || c.TagUsed == "struct") {
				if pending != nil && !used {
					return nil, false
				}
				pending, used = c, false
			}
		}
	}
	return out, pending == nil || used
}

func fieldOf(c *dnode, e *env, access Access) Field {
	f := Field{Name: c.Name, Type: e.parseIn(c.qual()), Access: access, Mutable: c.Mutable}
	if c.IsBitfield {
		f.Bits = -1
		if v, ok := c.firstConst(); ok && v.IsInt64() {
			f.Bits = int(v.Int64())
		}
	}
	return f
}

func methodOf(c *dnode, e *env, access Access, template bool) Method {
	m := Method{Name: c.Name, Kind: KindMethod, Sig: e.parseIn(c.qual()), Access: access,
		Static: c.StorageClass == "static", Virtual: c.Virtual, Pure: c.Pure, Constexpr: c.Constexpr, Variadic: c.Variadic,
		Deleted: c.ExplicitlyDeleted || c.deletedDefault(), Defaulted: c.defaulted(), Implicit: c.IsImplicit, Template: template}
	switch c.Kind {
	case "CXXConstructorDecl":
		m.Kind = KindCtor
	case "CXXDestructorDecl":
		m.Kind = KindDtor
	case "CXXConversionDecl":
		m.Kind = KindConversion
	default:
		if strings.HasPrefix(c.Name, "operator") && len(c.Name) > 8 && !isIdentByte(c.Name[8]) {
			m.Kind = KindOperator
		}
	}
	for i := range c.Inner {
		p := &c.Inner[i]
		switch p.Kind {
		case "ParmVarDecl":
			m.Params = append(m.Params, Param{Name: p.Name, Default: p.Init != ""})
		case "OverrideAttr":
			m.Override = true
		case "FinalAttr":
			m.Final = true
		}
	}
	if m.Override || m.Final {
		m.Virtual = true
	}
	return m
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// decodeEnum reads an enum declaration; ut is the underlying type that the
// compiler gave (empty when it did not).
func decodeEnum(n *dnode, spelling, ut string) (*Enum, error) {
	self, err := ctype.Parse(spelling)
	if err != nil || self.Kind != ctype.Named {
		return nil, fmt.Errorf("%q is not a type name", spelling)
	}
	en := &Enum{Name: spelling, Scope: scopeOf(self), Scoped: n.ScopedEnumTag != "", Size: -1, Underlying: (&env{}).parseIn(ut)}
	if ut == "" {
		en.Underlying = TypeText{Err: "the compiler did not say"}
	}
	next := new(big.Int)
	for i := range n.Inner {
		c := &n.Inner[i]
		if c.Kind != "EnumConstantDecl" {
			continue
		}
		v := new(big.Int).Set(next)
		if x, ok := c.firstConst(); ok {
			v = x
		}
		en.Values = append(en.Values, Enumerator{Name: c.Name, Value: v})
		next = new(big.Int).Add(v, big.NewInt(1))
	}
	return en, nil
}

// scopeOf returns the qualified name of t without its last part.
func scopeOf(t *ctype.Type) string {
	if t == nil || t.Kind != ctype.Named || len(t.Path) < 2 {
		return ""
	}
	var parts []string
	for _, s := range t.Path[:len(t.Path)-1] {
		parts = append(parts, s.Name)
	}
	return strings.Join(parts, "::")
}

// sameArgs reports whether two template argument lists are the same text.
func sameArgs(a, b []ctype.Arg) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x := (&ctype.Type{Kind: ctype.Named, Path: []ctype.Seg{{Name: "T", HasArgs: true, Args: []ctype.Arg{a[i]}}}}).String()
		y := (&ctype.Type{Kind: ctype.Named, Path: []ctype.Seg{{Name: "T", HasArgs: true, Args: []ctype.Arg{b[i]}}}}).String()
		if x != y {
			return false
		}
	}
	return true
}
