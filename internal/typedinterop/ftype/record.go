package ftype

import (
	"go/constant"
	"go/token"
	"go/types"
	"math/big"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// A record is a class, struct, union or enum that the mapper made a named
// type for.
type record struct {
	t          *types.Named
	kind       Kind
	key        string
	rec        *probe.Record
	enum       *probe.Enum
	incomplete bool
	scope      string

	fields  []FieldInfo
	consts  []*types.Const
	members *Members
}

// A FieldInfo is a data member in the foreign view.
type FieldInfo struct {
	Name   string
	Mapped Mapped
	Access probe.Access
	Bits   int
	Anon   bool
	Union  bool
}

// namedRecord returns the named type for the canonical type kc (a class or
// enum name), made on first use. It registers the type before it maps the
// members, so a record may mention itself.
func (m *Mapper) namedRecord(key string, kc *ctype.Type, depth int) *record {
	if r := m.named[key]; r != nil {
		return r
	}
	kc = kc.Clone()
	kc.Const, kc.Volatile = false, false
	name := m.displayName(kc)
	tn := types.NewTypeName(token.NoPos, m.Pkg, name, nil)
	nt := types.NewNamed(tn, types.NewStruct(nil, nil), nil)
	r := &record{t: nt, kind: Record, key: key, scope: scopeOf(kc)}
	m.named[key] = r
	m.nameOf[nt] = key
	switch {
	case m.decls.Enums[key] != nil:
		r.kind, r.enum = Enum, m.decls.Enums[key]
		m.fillEnum(r, depth)
	case m.decls.Records[key] != nil:
		r.rec = m.decls.Records[key]
		r.incomplete = !r.rec.Complete
		if r.rec.Complete {
			m.fillRecord(r, depth)
		}
	default:
		r.incomplete = true
	}
	return r
}

// displayName is the name of the named type: the C++ name with "." for "::"
// (std.vector[C.long]); std::basic_string<char> is std.string, as gocpp code
// writes it.
func (m *Mapper) displayName(c *ctype.Type) string {
	switch c.String() {
	case "std::basic_string<char>":
		return "std.string"
	case "std::basic_string_view<char>":
		return "std.string_view"
	}
	return m.goName(c)
}

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

// recordScope is the scope in which the names in a record's members are
// looked up: the record itself first (nested types, member typedefs are
// resolved by the declaration extraction), then its namespace.
func recordScope(rec *probe.Record) string { return rec.Scope }

func (m *Mapper) fillRecord(r *record, depth int) {
	var vars []*types.Var
	var tags []string
	seen := map[string]bool{}
	scope := recordScope(r.rec)
	for _, f := range r.rec.Fields {
		fm := m.fieldType(f.Type, scope, depth)
		fi := FieldInfo{Name: f.Name, Mapped: fm, Access: f.Access, Bits: f.Bits, Anon: f.Anon, Union: f.Union}
		r.fields = append(r.fields, fi)
		if f.Access != probe.Public || seen[f.Name] {
			continue
		}
		seen[f.Name] = true
		t := fm.Type
		if t == nil || fm.Kind == Void {
			t = types.Typ[types.Invalid]
		}
		vars = append(vars, types.NewField(token.NoPos, m.Pkg, f.Name, t, false))
		tag := ""
		if f.Bits > 0 {
			tag = "cpp:\"bits=" + strconv.Itoa(f.Bits) + "\""
		}
		tags = append(tags, tag)
	}
	for _, b := range r.rec.Bases {
		if b.Access != probe.Public || b.Type.T == nil {
			continue
		}
		bm := m.mapT(b.Type.T, nil, scope, depth+1)
		if bm.Kind != Record {
			continue
		}
		name := b.Type.T.Last()
		if seen[name] {
			continue
		}
		seen[name] = true
		vars = append(vars, types.NewField(token.NoPos, m.Pkg, name, bm.Type, true))
		tags = append(tags, "")
	}
	r.t.SetUnderlying(types.NewStruct(vars, tags))
}

func (m *Mapper) fieldType(tt probe.TypeText, scope string, depth int) Mapped {
	if tt.T == nil {
		return opaque(nil, "cannot read the type %q: %s", tt.Text, tt.Err)
	}
	return m.mapT(tt.T, nil, scope, depth+1)
}

func (m *Mapper) fillEnum(r *record, depth int) {
	under := types.Type(types.Typ[types.Int32])
	if ut := r.enum.Underlying.T; ut != nil {
		um := m.mapT(ut, nil, "", depth+1)
		if um.Kind == Scalar {
			under = um.Type.Underlying()
		}
	}
	r.t.SetUnderlying(under)
	for _, v := range r.enum.Values {
		if v.Value == nil {
			continue
		}
		r.consts = append(r.consts, types.NewConst(token.NoPos, m.Pkg, v.Name, r.t, constant.Make(new(big.Int).Set(v.Value))))
	}
}

// ---------------------------------------------------------------- members

// A Member is a member of a record found by name.
type Member struct {
	Name string
	// Kind is "field", "method" (all the overloads), "static", "type" (a nested
	// type) or "enumerator".
	Kind string
	// Of is the record that declares it (a base class, not the one asked).
	Of string

	Field   *FieldInfo
	Methods []*MethodInfo
	Static  *StaticInfo
	Type    Mapped
	Access  probe.Access
}

// A MethodInfo is a member function in the foreign view.
type MethodInfo struct {
	Name     string
	Kind     probe.MethodKind
	Sig      Mapped // Kind Func; Mapper.Sig has the details
	Access   probe.Access
	Static   bool
	Virtual  bool
	Pure     bool
	Deleted  bool
	Template bool
	Implicit bool
	// Const is the const of the method ("size() const").
	Const bool
	// Params are the parameter names, "" when not named.
	Params []probe.Param
}

// A StaticInfo is a static data member.
type StaticInfo struct {
	Name      string
	Mapped    Mapped
	Access    probe.Access
	Constexpr bool
}

// Members is the member set of a record, with the members of its public bases
// after its own.
type Members struct {
	Fields  []FieldInfo
	Methods []*MethodInfo
	Statics []*StaticInfo
	Nested  []string
	// Bases are the mapped base classes in declaration order.
	Bases []Mapped
	// Of maps a member name to the record that declared it first (derived before base).
	byName map[string]*Member
	order  []string
}

// MembersOf returns the members of a record, enum or other named type that
// the mapper made. nil for any other type, and for a record that is only declared.
func (m *Mapper) MembersOf(t types.Type) *Members {
	n, ok := t.(*types.Named)
	if !ok {
		return nil
	}
	key, ok := m.nameOf[n]
	if !ok {
		return nil
	}
	r := m.named[key]
	if r == nil || r.rec == nil || !r.rec.Complete {
		return nil
	}
	return m.members(r, 0)
}

func (m *Mapper) members(r *record, depth int) *Members {
	if r.members != nil {
		return r.members
	}
	ms := &Members{byName: map[string]*Member{}}
	r.members = ms // before the bases: a cycle of bases (an error in C++) ends here
	rec := r.rec
	scope := recordScope(rec)
	add := func(mem *Member) {
		if _, hidden := ms.byName[mem.Name]; hidden && mem.Kind != "method" {
			return
		}
		if old, dup := ms.byName[mem.Name]; dup && mem.Kind == "method" {
			if old.Kind == "method" && old.Of == mem.Of {
				old.Methods = append(old.Methods, mem.Methods...)
			}
			return // a derived class's member hides the base's
		}
		ms.byName[mem.Name] = mem
		ms.order = append(ms.order, mem.Name)
	}
	for i := range r.fields {
		f := r.fields[i]
		ms.Fields = append(ms.Fields, f)
		add(&Member{Name: f.Name, Kind: "field", Of: rec.Name, Field: &r.fields[i], Access: f.Access, Type: f.Mapped})
	}
	for _, s := range rec.Statics {
		si := &StaticInfo{Name: s.Name, Mapped: m.fieldType(s.Type, scope, depth), Access: s.Access, Constexpr: s.Constexpr}
		ms.Statics = append(ms.Statics, si)
		add(&Member{Name: s.Name, Kind: "static", Of: rec.Name, Static: si, Access: s.Access, Type: si.Mapped})
	}
	for i := range rec.Methods {
		pm := &rec.Methods[i]
		mi := &MethodInfo{Name: pm.Name, Kind: pm.Kind, Access: pm.Access, Static: pm.Static, Virtual: pm.Virtual, Pure: pm.Pure,
			Deleted: pm.Deleted, Template: pm.Template, Implicit: pm.Implicit, Params: pm.Params}
		if pm.Sig.T != nil && pm.Sig.T.Kind == ctype.Func && !pm.Template {
			mi.Sig = m.mapT(pm.Sig.T, nil, scope, depth+1)
			mi.Const = pm.Sig.T.ConstQ
		} else {
			mi.Sig = opaque(pm.Sig.T, "a member template or a signature that could not be read")
		}
		ms.Methods = append(ms.Methods, mi)
		if pm.Kind == probe.KindCtor || pm.Kind == probe.KindDtor {
			continue
		}
		add(&Member{Name: pm.Name, Kind: "method", Of: rec.Name, Methods: []*MethodInfo{mi}, Access: pm.Access})
	}
	for _, n := range rec.Nested {
		q := rec.Name + "::" + n
		mm := m.mapT(ctype.MustParse(q), nil, "", depth+1)
		ms.Nested = append(ms.Nested, n)
		add(&Member{Name: n, Kind: "type", Of: rec.Name, Type: mm, Access: probe.Public})
	}
	for _, b := range rec.Bases {
		if b.Type.T == nil {
			continue
		}
		bm := m.mapT(b.Type.T, nil, scope, depth+1)
		ms.Bases = append(ms.Bases, bm)
		if bn, ok := bm.Type.(*types.Named); ok && bm.Kind == Record {
			if br := m.named[m.nameOf[bn]]; br != nil && br.rec != nil && br.rec.Complete {
				bms := m.members(br, depth+1)
				for _, name := range bms.order {
					bmem := *bms.byName[name]
					// A member of a private base is not accessible; one of a
					// protected base is at most protected.
					if b.Access == probe.Private {
						bmem.Access = probe.Private
					} else if b.Access == probe.Protected && bmem.Access == probe.Public {
						bmem.Access = probe.Protected
					}
					add(&bmem)
				}
			}
		}
	}
	return ms
}

// Lookup finds a member by name: the record's own first, then its bases'.
func (ms *Members) Lookup(name string) *Member {
	if ms == nil {
		return nil
	}
	return ms.byName[name]
}

// Names returns the names of the members in order (own members first).
func (ms *Members) Names() []string { return ms.order }

// Enumerators returns the constants of an enum type made by the mapper, in
// declaration order; nil for any other type.
func (m *Mapper) Enumerators(t types.Type) []*types.Const {
	n, ok := t.(*types.Named)
	if !ok {
		return nil
	}
	if r := m.named[m.nameOf[n]]; r != nil && r.kind == Enum {
		return r.consts
	}
	return nil
}

// Canonical returns the canonical C++ spelling of a named type that the
// mapper made (the key of its declaration), "" for any other type.
func (m *Mapper) Canonical(t types.Type) string {
	if n, ok := t.(*types.Named); ok {
		if s, ok := m.nameOf[n]; ok {
			return s
		}
	}
	return ""
}

// Scope returns the namespace of a named type.
func (m *Mapper) Scope(t types.Type) string {
	if n, ok := t.(*types.Named); ok {
		if r := m.named[m.nameOf[n]]; r != nil {
			return r.scope
		}
	}
	return ""
}
