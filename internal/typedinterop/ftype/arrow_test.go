package ftype

import (
	"go/types"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// ArrowTarget and ArrowMember (Phase 12 step 12.6): which member of a class is reached with ->.

func TestStdPointee(t *testing.T) {
	for _, c := range []struct{ typ, want string }{
		{"std::unique_ptr<geo::Vec, std::default_delete<geo::Vec>>", "geo::Vec"},
		{"std::shared_ptr<geo::Vec>", "geo::Vec"},
		{"std::optional<geo::Vec>", "geo::Vec"},
		{"std::unique_ptr<int[], std::default_delete<int[]>>", ""}, // no ->
		{"std::vector<geo::Vec>", ""},
		{"__gnu_cxx::__normal_iterator<geo::Vec *, std::vector<geo::Vec>>", "geo::Vec"},
		{"__gnu_cxx::__normal_iterator<const geo::Vec *, std::vector<geo::Vec>>", "const geo::Vec"},
		{"std::_Rb_tree_iterator<std::pair<const long, int>>", "std::pair<const long, int>"},
		{"std::_List_iterator<geo::Vec>", "geo::Vec"},
		{"std::reverse_iterator<__gnu_cxx::__normal_iterator<geo::Vec *, std::vector<geo::Vec>>>", "geo::Vec"},
		{"geo::Vec", ""},
	} {
		got := ""
		if p := stdPointee(ctype.MustParse(c.typ)); p != nil {
			got = p.String()
		}
		if got != c.want {
			t.Errorf("stdPointee(%s) = %q, want %q", c.typ, got, c.want)
		}
	}
}

func arrowTable() *probe.DeclTable {
	d := table()
	// A class of the program with its own operator-> (a handle) and a member that the pointee also has.
	d.Records["geo::Handle"] = &probe.Record{Name: "geo::Handle", Scope: "geo", Tag: "class", Complete: true,
		Fields: []probe.Field{{Name: "id", Type: tt("int"), Access: probe.Public}},
		Methods: []probe.Method{
			{Name: "operator->", Kind: probe.KindOperator, Sig: tt("Vec *() const"), Access: probe.Public},
			{Name: "len", Kind: probe.KindMethod, Sig: tt("int () const"), Access: probe.Public},
		}}
	d.Canon["geo::Handle"] = "geo::Handle"
	// Two operators for const and not, reaching the same type; and two that reach different ones.
	d.Records["geo::Pair"] = &probe.Record{Name: "geo::Pair", Scope: "geo", Tag: "class", Complete: true,
		Methods: []probe.Method{
			{Name: "operator->", Kind: probe.KindOperator, Sig: tt("Vec *()"), Access: probe.Public},
			{Name: "operator->", Kind: probe.KindOperator, Sig: tt("const Vec *() const"), Access: probe.Public},
		}}
	d.Canon["geo::Pair"] = "geo::Pair"
	d.Records["geo::Odd"] = &probe.Record{Name: "geo::Odd", Scope: "geo", Tag: "class", Complete: true,
		Methods: []probe.Method{
			{Name: "operator->", Kind: probe.KindOperator, Sig: tt("Vec *()"), Access: probe.Public},
			{Name: "operator->", Kind: probe.KindOperator, Sig: tt("Body *() const"), Access: probe.Public},
		}}
	d.Canon["geo::Odd"] = "geo::Odd"
	// A chain: Chain::operator-> gives a Wrap, whose operator-> gives a Vec*.
	d.Records["geo::Wrap"] = &probe.Record{Name: "geo::Wrap", Scope: "geo", Tag: "class", Complete: true,
		Methods: []probe.Method{{Name: "operator->", Kind: probe.KindOperator, Sig: tt("Vec *()"), Access: probe.Public}}}
	d.Records["geo::Chain"] = &probe.Record{Name: "geo::Chain", Scope: "geo", Tag: "class", Complete: true,
		Methods: []probe.Method{{Name: "operator->", Kind: probe.KindOperator, Sig: tt("Wrap ()"), Access: probe.Public}}}
	// A cycle: Loop::operator-> gives a Loop.
	d.Records["geo::Loop"] = &probe.Record{Name: "geo::Loop", Scope: "geo", Tag: "class", Complete: true,
		Methods: []probe.Method{{Name: "operator->", Kind: probe.KindOperator, Sig: tt("Loop ()"), Access: probe.Public}}}
	d.Canon["geo::Wrap"], d.Canon["geo::Chain"], d.Canon["geo::Loop"] = "geo::Wrap", "geo::Chain", "geo::Loop"
	d.Records["std::shared_ptr<geo::Vec>"] = &probe.Record{Name: "std::shared_ptr<geo::Vec>", Scope: "std", Tag: "class", Complete: true,
		Methods: []probe.Method{{Name: "use_count", Kind: probe.KindMethod, Sig: tt("long () const"), Access: probe.Public}}}
	d.Canon["std::shared_ptr<geo::Vec>"] = "std::shared_ptr<geo::Vec>"
	d.Records["std::shared_ptr<geo::Big>"] = &probe.Record{Name: "std::shared_ptr<geo::Big>", Scope: "std", Tag: "class", Complete: true}
	d.Canon["std::shared_ptr<geo::Big>"] = "std::shared_ptr<geo::Big>"
	d.Records["geo::Big"] = &probe.Record{Name: "geo::Big", Scope: "geo", Tag: "class", Complete: false, Size: -1, Align: -1}
	d.Canon["geo::Big"] = "geo::Big"
	return d
}

func TestArrowMember(t *testing.T) {
	m := New(arrowTable(), LP64)
	named := func(s string) types.Type {
		v := m.Map(ctype.MustParse(s), nil)
		if v.Kind != Record {
			t.Fatalf("%s: %s", s, describe(m, v))
		}
		return v.Type
	}
	for _, c := range []struct {
		typ, member string
		want        bool
	}{
		{"geo::Handle", "scale", true},    // Vec's, not Handle's
		{"geo::Handle", "x", true},        // a field of the pointee
		{"geo::Handle", "id", false},      // its own field
		{"geo::Handle", "len", false},     // its own method, though Vec has one too
		{"geo::Handle", "nothing", false}, // nobody's
		{"geo::Handle", "secret", false},  // Vec private field: C++ reports the access
		{"geo::Pair", "scale", true},      // const and not: the same pointee
		{"geo::Odd", "scale", false},      // two different pointees: not decided
		{"geo::Vec", "x", false},          // no operator->
		{"std::shared_ptr<geo::Vec>", "scale", true},
		{"std::shared_ptr<geo::Vec>", "use_count", false}, // its own
		{"std::shared_ptr<geo::Vec>", "get", false},       // Vec has no get: nothing to reach
		{"std::shared_ptr<geo::Big>", "size", true},       // a pointee whose members are not known: not the pointer's own
		{"std::shared_ptr<geo::Big>", "get", true},        // the compiler's answer (no member) decides: ArrowMember only says it could
	} {
		got := m.ArrowMember(named(c.typ), c.member)
		if got != c.want {
			t.Errorf("ArrowMember(%s, %s) = %v, want %v", c.typ, c.member, got, c.want)
		}
	}
}
