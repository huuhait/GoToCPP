package bindgen

import (
	"strings"
	"testing"
)

// squeezed returns the view with every run of white space as one space, so
// that a test does not depend on the alignment that gofmt chooses.
func squeezed(res *Result) string {
	return strings.Join(strings.Fields(string(res.Source)), " ")
}

func wantInView(t *testing.T, res *Result, wants ...string) {
	t.Helper()
	view := squeezed(res)
	for _, want := range wants {
		if !strings.Contains(view, strings.Join(strings.Fields(want), " ")) {
			t.Errorf("the view lacks %q:\n%s", want, res.Source)
		}
	}
}

func notInView(t *testing.T, res *Result, unwanted ...string) {
	t.Helper()
	view := squeezed(res)
	for _, text := range unwanted {
		if strings.Contains(view, strings.Join(strings.Fields(text), " ")) {
			t.Errorf("the view has %q:\n%s", text, res.Source)
		}
	}
}

// A struct that was forward declared (typedef struct Table Table;) and then
// defined keeps the types declared inside its body: sqlite3.h declares
// sqlite3_index_info like that, and its aConstraint field points to a nested
// struct whose fields the view must show.
func TestNestedTypesOfForwardDeclaredStruct(t *testing.T) {
	res := generate(t, "nested_redeclared.h", nil)
	checkGP(t, "nested_redeclared", res)

	t.Run("the field uses the nested struct", func(t *testing.T) {
		wantInView(t, res, "columns *Table_Column")
	})
	t.Run("the nested struct has its fields", func(t *testing.T) {
		wantInView(t, res,
			"// struct Table::Column",
			"type Table_Column struct { index C.int flags C.uchar }")
	})
	t.Run("the nested enum has its enumerators", func(t *testing.T) {
		wantInView(t, res,
			"type Table_Mode C.uint",
			"Table_MODE_READ Table_Mode = 0",
			"Table_MODE_WRITE Table_Mode = 1")
	})
	t.Run("the nested typedef is the type of its field", func(t *testing.T) {
		wantInView(t, res, "handle Table_Handle", "type Table_Handle = C.int")
	})
	t.Run("no stand-in is written for a type that is in the header", func(t *testing.T) {
		notInView(t, res, "// C++: Column", "type Column struct{}", "// C++: Mode")
	})
}

// A declaration that the header marks deprecated carries a "Deprecated:" line
// that Go tools know. The text is the message of the attribute.
func TestDeprecatedDeclarations(t *testing.T) {
	res := generate(t, "deprecated.h", nil)
	checkGP(t, "deprecated", res)

	cases := []struct {
		name string
		want string
	}{
		{"function without a message",
			"// Deprecated: the header marks it deprecated. func old_plain() C.int"},
		{"function with a message",
			"// Deprecated: use new_one func old_message(x C.int) C.int"},
		{"function with a C++ attribute",
			"// Deprecated: use Fresh func old_cxx_attribute() C.int"},
		{"struct", "// Deprecated: gone type OldStruct struct {"},
		{"typedef", "// Deprecated: alias type OldAlias = C.int"},
		{"typedef of void", "// Deprecated: void typedef type OldVoid struct{}"},
		{"enum", "// Deprecated: the header marks it deprecated. type OldEnum C.uint"},
		{"variable", "// Deprecated: variable var old_var C.int"},
		{"message on several lines is one line",
			"// Deprecated: two lines func old_multiline() C.int"},

		// the paragraph is separated from the comment above it by a bare // line
		{"function in a namespace",
			"// ns::old_in_ns // // Deprecated: namespaced func ns_old_in_ns() C.int"},
		{"struct in a namespace",
			"// struct ns::OldInNs // // Deprecated: namespaced type type ns_OldInNs struct {"},
		{"struct nested in a struct",
			"// struct ns::Holder::OldNested // // Deprecated: nested type " +
				"type ns_Holder_OldNested struct {"},
		{"incomplete type",
			"// incomplete type: declared, not defined // // Deprecated: incomplete " +
				"type OldIncomplete struct{}"},

		// the enumerators of one enum
		{"deprecated enumerator with a message",
			"// Deprecated: use MIXED_OK MIXED_OLD Mixed = 1"},
		{"deprecated enumerator without a message",
			"// Deprecated: the header marks it deprecated. MIXED_PLAIN_OLD Mixed = 2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wantInView(t, res, c.want)
		})
	}

	// Go tools read "Deprecated:" only at the start of a paragraph of the doc comment: the line
	// above it is blank or a bare //.
	t.Run("every Deprecated line starts a paragraph", func(t *testing.T) {
		lines := strings.Split(string(res.Source), "\n")
		seen := 0
		for i, line := range lines {
			if !strings.HasPrefix(strings.TrimSpace(line), "// Deprecated: ") {
				continue
			}
			seen++
			above := strings.TrimSpace(lines[i-1])
			isComment := strings.HasPrefix(above, "//")
			if isComment && above != "//" {
				t.Errorf("%q follows the comment line %q", line, lines[i-1])
			}
		}
		if seen < 14 {
			t.Errorf("only %d Deprecated lines", seen)
		}
	})

	t.Run("a declaration that is not deprecated has no line", func(t *testing.T) {
		view := string(res.Source)
		decls := []string{"func current() C.int", "type Current struct {", "MIXED_OK Mixed = 0"}
		for _, decl := range decls {
			at := strings.Index(view, decl)
			if at < 0 {
				t.Fatalf("the view lacks %q:\n%s", decl, view)
			}
			before := view[:at]
			previous := before[strings.LastIndex(strings.TrimSuffix(before, "\n"), "\n")+1:]
			if strings.Contains(previous, "Deprecated") {
				t.Errorf("%q is marked deprecated: %q", decl, previous)
			}
		}
	})
}

// A struct that a field names with an elaborated type specifier (struct Later
// *next;) is declared in the namespace around the class, so the view calls it
// Later, not Holder_Later, and gives it the fields of its definition.
func TestElaboratedTypeInFieldIsNotNested(t *testing.T) {
	res := generate(t, "elaborated_fields.h", nil)
	checkGP(t, "elaborated_fields", res)

	t.Run("a struct defined later keeps its own name and fields", func(t *testing.T) {
		wantInView(t, res, "next *Later", "type Later struct { value C.int }")
		notInView(t, res, "Holder_Later")
	})
	t.Run("a struct that is never defined is an incomplete type", func(t *testing.T) {
		wantInView(t, res,
			"handle *Opaque",
			"// incomplete type: declared, not defined type Opaque struct{}")
		notInView(t, res, "Holder_Opaque")
	})
	t.Run("inside a namespace it belongs to that namespace", func(t *testing.T) {
		wantInView(t, res, "inner *ns_Inside", "type ns_Inside struct { z C.int }")
		notInView(t, res, "ns_Box_Inside")
	})
	t.Run("two classes deep it still belongs to the namespace", func(t *testing.T) {
		wantInView(t, res,
			"nested Outer_Nested",
			"type Outer_Nested struct { deeper *Deep }",
			"type Deep struct { depth C.int }")
		notInView(t, res, "Outer_Deep", "Outer_Nested_Deep")
	})
}

// A function that the header declares again with the same type is one function: the view writes it
// once and does not list the repeat as an overload. A different type is an overload, listed once.
func TestRedeclaredFunctionIsNotAnOverload(t *testing.T) {
	res := generate(t, "redeclared_functions.h", nil)
	checkGP(t, "redeclared_functions", res)
	view := string(res.Source)

	t.Run("the function is written once", func(t *testing.T) {
		if got := strings.Count(view, "func twice("); got != 1 {
			t.Errorf("func twice is written %d times:\n%s", got, view)
		}
		wantInView(t, res, "func twice(x C.int) C.int")
	})
	t.Run("the repeat is not an overload", func(t *testing.T) {
		notInView(t, res, "overload: twice")
	})
	t.Run("const and parameter names make no overload", func(t *testing.T) {
		wantInView(t, res, "func qualified(x C.int)")
		notInView(t, res, "overload: qualified")
	})
	t.Run("a different type is an overload, listed once", func(t *testing.T) {
		wantInView(t, res, "func pick(x C.int) C.int", "// overload: pick(x C.double) C.double")
		if got := strings.Count(view, "overload: pick"); got != 1 {
			t.Errorf("the overload of pick is listed %d times:\n%s", got, view)
		}
	})
}

// What C++ counts as another overload is listed as one, also when Go shows no difference: const
// of a pointee, a reference, volatile, and the result type.
func TestOverloadsThatGoSpellsAlikeAreListed(t *testing.T) {
	res := generate(t, "redeclared_functions.h", nil)
	view := string(res.Source)

	cases := []struct {
		name     string
		function string
		listed   string
	}{
		{"const pointee", "a", "// overload: a(p *C.int)"},
		{"const referent", "c", "// overload: c(r *C.int)"},
		{"another result type", "k", "// overload: k(s *C.char) *C.char"},
		{"volatile pointee", "e", "// overload: e(p *C.int)"},
		{"lvalue and rvalue reference", "i", "// overload: i(r *C.int)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wantInView(t, res, "func "+c.function+"(")
			if got := strings.Count(view, "overload: "+c.function+"("); got != 1 {
				t.Errorf("the overloads of %s are listed %d times, want once:\n%s", c.function, got, view)
			}
			wantInView(t, res, c.listed)
		})
	}
	t.Run("const on a parameter itself makes no overload", func(t *testing.T) {
		wantInView(t, res, "func by_value(x C.int)", "func by_pointer(p *C.int)")
		notInView(t, res, "overload: by_value", "overload: by_pointer")
	})
}
