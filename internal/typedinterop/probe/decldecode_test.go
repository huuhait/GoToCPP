package probe

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"
)

func jnode(t testing.TB, s string) *dnode {
	t.Helper()
	var n dnode
	if err := json.Unmarshal([]byte(s), &n); err != nil {
		t.Fatal(err)
	}
	return &n
}

func TestDecodeRecordFixture(t *testing.T) {
	n := jnode(t, `{"id":"0x1","kind":"CXXRecordDecl","name":"S","tagUsed":"class","completeDefinition":true,
	 "definitionData":{"isPolymorphic":true,"isAggregate":false,"isAbstract":true,"canConstDefaultInit":true,"defaultCtor":{"exists":true}},
	 "bases":[{"access":"public","type":{"qualType":"B"}},{"access":"private","isVirtual":true,"type":{"qualType":"V","desugaredQualType":"ns::V"}}],
	 "inner":[
	  {"kind":"CXXRecordDecl","name":"S","isImplicit":true,"tagUsed":"class"},
	  {"kind":"FieldDecl","name":"a","type":{"qualType":"int"}},
	  {"kind":"AccessSpecDecl","access":"public"},
	  {"kind":"TypedefDecl","name":"size_type","type":{"qualType":"size_t","desugaredQualType":"unsigned long"}},
	  {"kind":"FieldDecl","name":"n","type":{"qualType":"size_type"}},
	  {"kind":"FieldDecl","name":"self","type":{"qualType":"S *"}},
	  {"kind":"FieldDecl","name":"selft","type":{"qualType":"S<int> *"}},
	  {"kind":"FieldDecl","name":"bits","type":{"qualType":"unsigned int"},"isBitfield":true,"inner":[{"kind":"ConstantExpr","value":"5"}]},
	  {"kind":"FieldDecl","name":"","type":{"qualType":"unsigned int"},"isBitfield":true,"inner":[{"kind":"ConstantExpr","value":"3"}]},
	  {"kind":"CXXRecordDecl","tagUsed":"union","completeDefinition":true,"inner":[{"kind":"FieldDecl","name":"i","type":{"qualType":"int"}},{"kind":"FieldDecl","name":"f","type":{"qualType":"float"}}]},
	  {"kind":"FieldDecl","isImplicit":true,"type":{"qualType":"S::(anonymous union)"}},
	  {"kind":"CXXRecordDecl","name":"In","tagUsed":"struct","completeDefinition":true},
	  {"kind":"EnumDecl","name":"En"},
	  {"kind":"VarDecl","name":"k","storageClass":"static","type":{"qualType":"const int"},"constexpr":true},
	  {"kind":"VarDecl","name":"notmember","type":{"qualType":"int"}},
	  {"kind":"AccessSpecDecl","access":"private"},
	  {"kind":"CXXMethodDecl","name":"hidden","type":{"qualType":"size_type ()"}},
	  {"kind":"CXXMethodDecl","name":"operator==","type":{"qualType":"bool (const S &) const"},"inner":[{"kind":"ParmVarDecl","type":{"qualType":"const S &"}}]},
	  {"kind":"CXXMethodDecl","name":"operatorX","type":{"qualType":"void ()"}},
	  {"kind":"CXXConversionDecl","name":"operator int","type":{"qualType":"int () const"}},
	  {"kind":"CXXDestructorDecl","name":"~S","type":{"qualType":"void () noexcept"},"virtual":true},
	  {"kind":"FunctionTemplateDecl","name":"get","inner":[{"kind":"TemplateTypeParmDecl","name":"T"},{"kind":"CXXMethodDecl","name":"get","type":{"qualType":"T ()"}}]},
	  {"kind":"CXXMethodDecl","name":"f","type":{"qualType":"void (int)"},"explicitlyDefaulted":"deleted"},
	  {"kind":"CXXMethodDecl","name":"g","type":{"qualType":"void ()"},"explicitlyDeleted":true},
	  {"kind":"CXXMethodDecl","name":"h","type":{"qualType":"void ()"},"explicitlyDefaulted":"default","virtual":true,"pure":true,"inner":[{"kind":"OverrideAttr"},{"kind":"FinalAttr"}]},
	  {"kind":"FriendDecl","inner":[{"kind":"FunctionDecl","name":"ff"}]},
	  {"kind":"UsingDecl","name":"B::x"},
	  {"kind":"SomethingNew","name":"x"}
	 ]}`)
	r, err := decodeRecord(n, "ns::S")
	if err != nil {
		t.Fatal(err)
	}
	if r.Tag != "class" || !r.Complete || r.Scope != "ns" || r.Template || r.Name != "ns::S" {
		t.Errorf("record %+v", r)
	}
	if !r.Traits["polymorphic"] || !r.Traits["abstract"] || r.Traits["aggregate"] || len(r.Traits) != 2 {
		t.Errorf("traits %v", r.Traits)
	}
	if len(r.Bases) != 2 || r.Bases[0].Access != Public || r.Bases[1].Access != Private || !r.Bases[1].Virtual || r.Bases[1].Type.T.String() != "ns::V" {
		t.Errorf("bases %+v", r.Bases)
	}
	// A class's members are private until an access label; the labels switch.
	var acc []string
	for _, f := range r.Fields {
		acc = append(acc, f.Name+":"+f.Type.T.String()+":"+string(f.Access))
	}
	got := strings.Join(acc, " ")
	for _, w := range []string{"a:int:private", "n:unsigned long:public", "self:ns::S *:public", "selft:S<int> *:public", "i:int:public", "f:float:public"} {
		if !strings.Contains(got, w) {
			t.Errorf("fields %q lack %q", got, w)
		}
	}
	if bits := r.Field("bits"); bits == nil || bits.Bits != 5 {
		t.Errorf("bits %+v", bits)
	}
	if i := r.Field("i"); i == nil || !i.Anon || !i.Union {
		t.Errorf("anonymous union member %+v", i)
	}
	if len(r.Fields) != 7 { // a n self selft bits i f: the unnamed bit-field and the implicit field are not fields
		t.Errorf("%d fields: %s", len(r.Fields), got)
	}
	if len(r.Statics) != 1 || r.Statics[0].Name != "k" || !r.Statics[0].Constexpr {
		t.Errorf("statics %+v", r.Statics)
	}
	if strings.Join(r.Nested, ",") != "In,En" || r.Types["size_type"] != "unsigned long" {
		t.Errorf("nested %v types %v", r.Nested, r.Types)
	}
	kinds := map[string]MethodKind{}
	for _, m := range r.Methods {
		kinds[m.Name] = m.Kind
	}
	for name, k := range map[string]MethodKind{"hidden": KindMethod, "operator==": KindOperator, "operatorX": KindMethod, "operator int": KindConversion, "~S": KindDtor, "get": KindMethod} {
		if kinds[name] != k {
			t.Errorf("%s is %q, want %q", name, kinds[name], k)
		}
	}
	for _, m := range r.Methods {
		switch m.Name {
		case "hidden":
			if m.Access != Private || m.Sig.T.Elem.String() != "unsigned long" {
				t.Errorf("hidden %+v", m)
			}
		case "get":
			if !m.Template {
				t.Errorf("get %+v", m)
			}
		case "f":
			if !m.Deleted {
				t.Errorf("f %+v", m)
			}
		case "g":
			if !m.Deleted {
				t.Errorf("g %+v", m)
			}
		case "h":
			if !m.Defaulted || !m.Virtual || !m.Pure || !m.Override || !m.Final {
				t.Errorf("h %+v", m)
			}
		case "~S":
			if !m.Virtual {
				t.Errorf("~S %+v", m)
			}
		}
	}
	// A struct's members are public by default.
	s, _ := decodeRecord(jnode(t, `{"kind":"CXXRecordDecl","name":"P","tagUsed":"struct","completeDefinition":true,"inner":[{"kind":"FieldDecl","name":"x","type":{"qualType":"int"}}]}`), "P")
	if s.Fields[0].Access != Public || s.Tag != "struct" || s.Scope != "" {
		t.Errorf("struct %+v", s)
	}
	// Not a type name.
	if _, err := decodeRecord(jnode(t, `{"kind":"CXXRecordDecl"}`), "int *"); err == nil {
		t.Error("decodeRecord accepts a pointer")
	}
	// An unreadable type keeps its text and the reason.
	bad, _ := decodeRecord(jnode(t, `{"kind":"CXXRecordDecl","name":"P","completeDefinition":true,"inner":[{"kind":"FieldDecl","name":"v","type":{"qualType":"int __attribute__((vector_size(16)))"}}]}`), "P")
	if f := bad.Field("v"); f == nil || f.Type.T != nil || f.Type.Err == "" || !strings.Contains(f.Type.Text, "vector_size") {
		t.Errorf("unreadable type %+v", f)
	}
}

// An anonymous member that cannot be placed makes the record partial, not complete with
// fewer members.
func TestDecodeUnmodelledAnonymous(t *testing.T) {
	// two anonymous records in a row: the second one's field is not the first one's
	n := jnode(t, `{"kind":"CXXRecordDecl","name":"S","tagUsed":"struct","completeDefinition":true,"inner":[
	  {"kind":"FieldDecl","name":"a","type":{"qualType":"int"}},
	  {"kind":"CXXRecordDecl","tagUsed":"union","completeDefinition":true,"inner":[{"kind":"FieldDecl","name":"x","type":{"qualType":"int"}}]},
	  {"kind":"CXXRecordDecl","tagUsed":"union","completeDefinition":true,"inner":[{"kind":"FieldDecl","name":"y","type":{"qualType":"int"}}]},
	  {"kind":"FieldDecl","isImplicit":true,"type":{"qualType":"S::(anonymous union)"}}]}`)
	r, err := decodeRecord(n, "S")
	if err != nil || r.Complete || r.Partial == "" {
		t.Errorf("%+v %v", r, err)
	}
	// an anonymous record with no field after it
	n = jnode(t, `{"kind":"CXXRecordDecl","name":"S","tagUsed":"struct","completeDefinition":true,"inner":[
	  {"kind":"CXXRecordDecl","tagUsed":"struct","completeDefinition":true,"inner":[{"kind":"FieldDecl","name":"x","type":{"qualType":"int"}}]}]}`)
	if r, _ := decodeRecord(n, "S"); r.Complete || r.Partial == "" {
		t.Errorf("dangling: %+v", r)
	}
	// one inside an anonymous union with no field after it
	n = jnode(t, `{"kind":"CXXRecordDecl","name":"S","tagUsed":"struct","completeDefinition":true,"inner":[
	  {"kind":"CXXRecordDecl","tagUsed":"union","completeDefinition":true,"inner":[
	    {"kind":"CXXRecordDecl","tagUsed":"struct","completeDefinition":true,"inner":[{"kind":"FieldDecl","name":"lo","type":{"qualType":"int"}}]},
	    {"kind":"FieldDecl","name":"both","type":{"qualType":"long"}}]},
	  {"kind":"FieldDecl","isImplicit":true,"type":{"qualType":"S::(anonymous union)"}}]}`)
	if r, _ := decodeRecord(n, "S"); r.Complete || r.Partial == "" || len(r.Fields) != 0 {
		t.Errorf("inner dangling: %+v", r)
	}
	// an unnamed struct with a declarator (struct { int x, y; } pos;) is a field with an unnamed
	// type, not an anonymous member, at the top and inside an anonymous union
	n = jnode(t, `{"kind":"CXXRecordDecl","name":"S","tagUsed":"struct","completeDefinition":true,"inner":[
	  {"kind":"CXXRecordDecl","tagUsed":"struct","completeDefinition":true,"inner":[{"kind":"FieldDecl","name":"x","type":{"qualType":"int"}}]},
	  {"kind":"FieldDecl","name":"pos","type":{"qualType":"S::(unnamed struct)"}},
	  {"kind":"FieldDecl","name":"pos2","type":{"qualType":"S::(unnamed struct)"}},
	  {"kind":"CXXRecordDecl","tagUsed":"union","completeDefinition":true,"inner":[
	    {"kind":"CXXRecordDecl","tagUsed":"struct","completeDefinition":true,"inner":[{"kind":"FieldDecl","name":"lo","type":{"qualType":"int"}}]},
	    {"kind":"FieldDecl","name":"s","type":{"qualType":"S::(unnamed struct)"}},
	    {"kind":"FieldDecl","name":"both","type":{"qualType":"long"}}]},
	  {"kind":"FieldDecl","isImplicit":true,"type":{"qualType":"S::(anonymous union)"}}]}`)
	if r, _ := decodeRecord(n, "S"); !r.Complete || r.Partial != "" || fieldList(r) != "pos:S::(unnamed struct) pos2:S::(unnamed struct) s:S::(unnamed struct) both:long" {
		t.Errorf("declarators: %+v", r)
	}
	// a typedef, an alias or a static member after an unnamed record consumes it too
	for _, follow := range []string{
		`{"kind":"TypedefDecl","name":"Inner","type":{"qualType":"struct Inner"}}`,
		`{"kind":"TypeAliasDecl","name":"X","type":{"qualType":"X"}}`,
		`{"kind":"VarDecl","name":"cfg","storageClass":"static","type":{"qualType":"const S::(unnamed struct)"}}`,
	} {
		n = jnode(t, `{"kind":"CXXRecordDecl","name":"S","tagUsed":"struct","completeDefinition":true,"inner":[
		  {"kind":"CXXRecordDecl","tagUsed":"struct","completeDefinition":true,"inner":[{"kind":"FieldDecl","name":"a","type":{"qualType":"int"}}]},`+follow+`,
		  {"kind":"FieldDecl","name":"x","type":{"qualType":"int"}}]}`)
		if r, _ := decodeRecord(n, "S"); !r.Complete || r.Partial != "" || fieldList(r) != "x:int" {
			t.Errorf("after %s: %+v", follow, r)
		}
	}
	// the same inside an anonymous union: a typedef after the unnamed struct consumes it
	n = jnode(t, `{"kind":"CXXRecordDecl","name":"S","tagUsed":"struct","completeDefinition":true,"inner":[
	  {"kind":"CXXRecordDecl","tagUsed":"union","completeDefinition":true,"inner":[
	    {"kind":"CXXRecordDecl","tagUsed":"struct","completeDefinition":true,"inner":[{"kind":"FieldDecl","name":"lo","type":{"qualType":"int"}}]},
	    {"kind":"TypedefDecl","name":"In","type":{"qualType":"struct In"}},
	    {"kind":"FieldDecl","name":"both","type":{"qualType":"long"}}]},
	  {"kind":"FieldDecl","isImplicit":true,"type":{"qualType":"S::(anonymous union)"}}]}`)
	if r, _ := decodeRecord(n, "S"); !r.Complete || r.Partial != "" || fieldList(r) != "both:long" {
		t.Errorf("typedef in an anonymous union: %+v", r)
	}
	// a depth beyond the limit
	deep := `{"kind":"FieldDecl","name":"leaf","type":{"qualType":"int"}}`
	for i := 0; i < 20; i++ {
		deep = `{"kind":"CXXRecordDecl","tagUsed":"struct","inner":[` + deep + `]},{"kind":"FieldDecl","isImplicit":true}`
	}
	n = jnode(t, `{"kind":"CXXRecordDecl","name":"S","tagUsed":"struct","completeDefinition":true,"inner":[`+deep+`]}`)
	if r, _ := decodeRecord(n, "S"); r.Complete {
		t.Errorf("deeper than the limit: %+v", r)
	}
}

func TestDecodeEnumFixture(t *testing.T) {
	n := jnode(t, `{"kind":"EnumDecl","name":"E","scopedEnumTag":"class","inner":[
	  {"kind":"EnumConstantDecl","name":"A"},
	  {"kind":"EnumConstantDecl","name":"B","inner":[{"kind":"ConstantExpr","value":"-5"}]},
	  {"kind":"EnumConstantDecl","name":"C"},
	  {"kind":"EnumConstantDecl","name":"D","inner":[{"kind":"ConstantExpr","value":"18446744073709551615"}]},
	  {"kind":"EnumConstantDecl","name":"E2"},
	  {"kind":"FunctionDecl","name":"junk"}]}`)
	e, err := decodeEnum(n, "ns::E", "int")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, v := range e.Values {
		got = append(got, v.Name+"="+v.Value.String())
	}
	if strings.Join(got, " ") != "A=0 B=-5 C=-4 D=18446744073709551615 E2=18446744073709551616" || !e.Scoped || e.Scope != "ns" {
		t.Errorf("%v %+v", got, e)
	}
	if e.Underlying.T == nil || e.Underlying.T.Name != "int" {
		t.Errorf("underlying %+v", e.Underlying)
	}
	if e2, _ := decodeEnum(n, "E", ""); e2.Underlying.Err == "" || e2.Scope != "" {
		t.Errorf("no underlying: %+v", e2.Underlying)
	}
	if _, err := decodeEnum(n, "(unnamed enum)", "int"); err == nil {
		t.Error("an unnamed enum")
	}
	b, _ := json.Marshal(e)
	if !strings.Contains(string(b), `"value":"18446744073709551616"`) {
		t.Errorf("json %s", b)
	}
	_ = big.NewInt
}

func TestDeclIDOf(t *testing.T) {
	for name, c := range map[string]struct{ json, id string }{
		"record":  {`{"kind":"TypeAliasDecl","inner":[{"kind":"ElaboratedType","inner":[{"kind":"RecordType","decl":{"id":"0x9","kind":"CXXRecordDecl"}}]}]}`, "0x9"},
		"enum":    {`{"inner":[{"kind":"EnumType","decl":{"id":"0x7","kind":"EnumDecl"}}]}`, "0x7"},
		"typedef": {`{"inner":[{"kind":"ElaboratedType","inner":[{"kind":"TypedefType","inner":[{"kind":"ElaboratedType","inner":[{"kind":"RecordType","decl":{"id":"0x5"}}]}]}]}]}`, "0x5"},
		"template (last child is the canonical type)": {`{"inner":[{"kind":"ElaboratedType","inner":[{"kind":"TemplateSpecializationType","inner":[
			{"kind":"TemplateArgument","inner":[{"kind":"RecordType","decl":{"id":"0xa"}}]},{"kind":"RecordType","decl":{"id":"0xb","kind":"ClassTemplateSpecializationDecl"}}]}]}]}`, "0xb"},
		"builtin": {`{"inner":[{"kind":"ElaboratedType","inner":[{"kind":"TypedefType","inner":[{"kind":"BuiltinType"}]}]}]}`, ""},
		"none":    {`{}`, ""},
		"no decl": {`{"inner":[{"kind":"RecordType"}]}`, ""},
	} {
		id, _ := declIDOf(jnode(t, c.json))
		if id != c.id {
			t.Errorf("%s: %q, want %q", name, id, c.id)
		}
	}
	// A very deep tree ends.
	deep := `{"kind":"BuiltinType"}`
	for i := 0; i < 200; i++ {
		deep = `{"kind":"ElaboratedType","inner":[` + deep + `]}`
	}
	if id, _ := declIDOf(jnode(t, `{"inner":[`+deep+`]}`)); id != "" {
		t.Error("an id from nowhere")
	}
}

func FuzzDecodeRecord(f *testing.F) {
	f.Add(`{"kind":"CXXRecordDecl","name":"S","tagUsed":"struct","inner":[{"kind":"FieldDecl","name":"a","type":{"qualType":"int *"}}]}`, "S")
	f.Add(`{"kind":"EnumDecl","inner":[{"kind":"EnumConstantDecl","name":"A","inner":[{"kind":"ConstantExpr","value":"7"}]}]}`, "ns::E<int>")
	f.Add(`{"inner":[{"kind":"CXXMethodDecl","name":"operator","type":{"qualType":"void (S::*)(int) const"}}],"bases":[{"type":{}}]}`, "a::b::S<C>")
	f.Fuzz(func(t *testing.T, js, spelling string) {
		var n dnode
		if json.Unmarshal([]byte(js), &n) != nil {
			return
		}
		if r, err := decodeRecord(&n, spelling); err == nil {
			_ = r.Field("a")
			_, _ = json.Marshal(r)
		}
		if e, err := decodeEnum(&n, spelling, "int"); err == nil {
			_, _ = json.Marshal(e)
		}
		declIDOf(&n)
		n.find("0x1")
		n.firstConst()
	})
}

// The written type comes from the tree of the probe declaration; a tree that has
// another shape (another clang) gives "", never a wrong type.
func TestWrittenTypeTree(t *testing.T) {
	for name, c := range map[string]struct{ json, want string }{
		"expression": {`{"name":"gp_probe_0","type":{"qualType":"gocpp_probe_internal_c<decltype((f()))>"},"inner":[{"kind":"ElaboratedType","inner":[{"kind":"TemplateSpecializationType","inner":[
			{"kind":"TemplateArgument","inner":[{"kind":"DecltypeType","inner":[{"kind":"ParenExpr","type":{"qualType":"size_t"}},{"kind":"ElaboratedType","type":{"qualType":"size_t"}}]}]}]}]}]}`, "size_t"},
		"reference":                       {`{"inner":[{"kind":"TemplateArgument","inner":[{"kind":"DecltypeType","inner":[{"kind":"ParenExpr"},{"kind":"LValueReferenceType","type":{"qualType":"value_type &"}}]}]}]}`, "value_type &"},
		"type":                            {`{"inner":[{"kind":"TemplateArgument","type":{"qualType":"FILE *"},"inner":[{"kind":"PointerType"}]}]}`, "FILE *"},
		"decltype text instead of a type": {`{"inner":[{"kind":"TemplateArgument","type":{"qualType":"decltype((f()))"}}]}`, ""},
		"no argument":                     {`{"inner":[{"kind":"ElaboratedType"}]}`, ""},
		"decltype without a type":         {`{"inner":[{"kind":"TemplateArgument","inner":[{"kind":"DecltypeType"}]}]}`, ""},
		"last child is an expression":     {`{"inner":[{"kind":"TemplateArgument","inner":[{"kind":"DecltypeType","inner":[{"kind":"ParenExpr","type":{"qualType":"int"}}]}]}]}`, ""},
		"unnamed type":                    {`{"inner":[{"kind":"TemplateArgument","type":{"qualType":"(unnamed struct at /a/b.h:1:2)"}}]}`, "(unnamed struct)"},
	} {
		n := jnode(t, c.json)
		var raw node
		if err := json.Unmarshal([]byte(c.json), &raw); err != nil {
			t.Fatal(err)
		}
		_ = n
		if got := writtenType(&raw); got != c.want {
			t.Errorf("%s: %q, want %q", name, got, c.want)
		}
	}
	r := Result{Type: "unsigned long"}
	r.setWritten("unsigned long")
	if r.Written != "" {
		t.Error("a written type equal to the canonical one is kept")
	}
	r.setWritten("size_t")
	if r.Written != "size_t" {
		t.Error("the written type is lost")
	}
	r.setWritten("")
	if r.Written != "size_t" {
		t.Error("an empty written type erased it")
	}
}
