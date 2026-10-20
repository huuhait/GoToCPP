package probe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// verifyProgram writes a C++ file that asserts, with the compiler, every fact
// that the declaration table claims: sizes, field and static member types, the
// types of the member functions, the bases, the traits, the enumerator
// values, the types of the functions. If the extraction were wrong (a field of
// the wrong type, a value off by one), the compiler would reject the program.
// Records that sit in a class (not a namespace) are checked by name only.
func verifyProgram(tab *DeclTable, header string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#include <type_traits>\n#include <utility>\n#include \"%s\"\n", header)
	n := 0
	ns := func(scope string, body string) {
		if scope == "" {
			b.WriteString(body)
			return
		}
		parts := strings.Split(scope, "::")
		for _, p := range parts {
			fmt.Fprintf(&b, "namespace %s { ", p)
		}
		b.WriteString(body)
		b.WriteString(strings.Repeat(" }", len(parts)))
		b.WriteString("\n")
	}
	isNamespace := func(scope string) bool {
		_, isRec := tab.Records[scope]
		_, isEnum := tab.Enums[scope]
		return !isRec && !isEnum
	}
	var keys []string
	for k := range tab.Records {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r := tab.Records[k]
		if strings.HasPrefix(k, "std::") || strings.HasPrefix(k, "_") {
			continue
		}
		q := "::" + r.Name
		fmt.Fprintf(&b, "// record %s\n", r.Name)
		if !r.Complete {
			fmt.Fprintf(&b, "template <class T> concept gp_inc_%d = !requires { sizeof(T); };\nstatic_assert(gp_inc_%d<%s>);\n", n, n, q)
			n++
			continue
		}
		fmt.Fprintf(&b, "static_assert(sizeof(%s) == %d && alignof(%s) == %d);\n", q, r.Size, q, r.Align)
		for name, trait := range map[string]string{"aggregate": "is_aggregate", "polymorphic": "is_polymorphic", "abstract": "is_abstract",
			"empty": "is_empty", "standardLayout": "is_standard_layout", "trivial": "is_trivial", "triviallyCopyable": "is_trivially_copyable", "pod": "is_pod"} {
			if r.Traits[name] {
				fmt.Fprintf(&b, "static_assert(std::%s_v<%s>);\n", trait, q)
			}
		}
		fmt.Fprintf(&b, "static_assert(std::is_union_v<%s> == %v);\n", q, r.Tag == "union")
		if !isNamespace(r.Scope) {
			continue
		}
		for _, f := range r.Fields {
			if f.Bits != 0 || f.Type.T == nil || f.Access != Public || strings.Contains(f.Type.Text, "(unnamed") || strings.Contains(f.Type.Text, "(anonymous") {
				continue
			}
			ns(r.Scope, fmt.Sprintf("static_assert(std::is_same_v<decltype(%s::%s), %s>);", q, f.Name, f.Type.T.String()))
		}
		for _, s := range r.Statics {
			if s.Type.T != nil && s.Access == Public && !strings.Contains(s.Type.Text, "(unnamed") && !strings.Contains(s.Type.Text, "(anonymous") {
				ns(r.Scope, fmt.Sprintf("static_assert(std::is_same_v<decltype(%s::%s), %s>);", q, s.Name, s.Type.T.String()))
			}
		}
		for _, bs := range r.Bases {
			if bs.Type.T != nil && bs.Access == Public {
				ns(r.Scope, fmt.Sprintf("static_assert(std::is_base_of_v<%s, %s>);", bs.Type.T.String(), q))
			}
		}
		for i := range r.Methods {
			m := &r.Methods[i]
			if m.Implicit || m.Deleted || m.Template || m.Access != Public || m.Sig.T == nil || m.Sig.T.Kind != ctype.Func {
				continue
			}
			if m.Kind == KindCtor || m.Kind == KindDtor {
				continue
			}
			sig := m.Sig.T
			var pt *ctype.Type
			if m.Static {
				pt = &ctype.Type{Kind: ctype.Pointer, Elem: sig}
			} else {
				cls, _ := ctype.Parse(q)
				pt = &ctype.Type{Kind: ctype.MemberPtr, Elem: sig, Class: cls}
			}
			ns(r.Scope, fmt.Sprintf("inline void gp_m_%d() { [[maybe_unused]] auto p = static_cast<%s>(&%s::%s); }", n, pt.String(), q, m.Name))
			n++
		}
	}
	var ekeys []string
	for k := range tab.Enums {
		ekeys = append(ekeys, k)
	}
	sort.Strings(ekeys)
	for _, k := range ekeys {
		e := tab.Enums[k]
		q := "::" + e.Name
		fmt.Fprintf(&b, "// enum %s\nstatic_assert(sizeof(%s) == %d);\n", e.Name, q, e.Size)
		if e.Underlying.T != nil {
			fmt.Fprintf(&b, "static_assert(std::is_same_v<std::underlying_type_t<%s>, %s>);\n", q, e.Underlying.T.String())
		}
		fmt.Fprintf(&b, "static_assert(!std::is_convertible_v<%s, std::underlying_type_t<%s>> == %v);\n", q, q, e.Scoped)
		for _, v := range e.Values {
			if !v.Value.IsInt64() {
				continue
			}
			ref := q + "::" + v.Name
			if !e.Scoped {
				ref = "::" + v.Name
				if e.Scope != "" {
					ref = "::" + e.Scope + "::" + v.Name
				}
			}
			fmt.Fprintf(&b, "static_assert((long long)(%s) == %dLL);\n", ref, v.Value.Int64())
		}
	}
	var fkeys []string
	for k := range tab.Funcs {
		fkeys = append(fkeys, k)
	}
	sort.Strings(fkeys)
	for _, k := range fkeys {
		fs := tab.Funcs[k]
		for _, ov := range fs.Overloads {
			if ov.Kind != "function" || ov.Sig.T == nil {
				continue
			}
			pt := &ctype.Type{Kind: ctype.Pointer, Elem: ov.Sig.T}
			ns(fs.Scope, fmt.Sprintf("inline void gp_f_%d() { [[maybe_unused]] auto p = static_cast<%s>(&::%s); }", n, pt.String(), fs.Name))
			n++
		}
	}
	var ckeys []string
	for k := range tab.Canon {
		ckeys = append(ckeys, k)
	}
	sort.Strings(ckeys)
	for _, k := range ckeys {
		if strings.HasPrefix(k, "std::") {
			continue
		}
		fmt.Fprintf(&b, "static_assert(std::is_same_v<::%s, %s>);\n", k, tab.Canon[k])
	}
	return b.String()
}

// compileVerify compiles the verifier with every C++ compiler that is there.
func compileVerify(t *testing.T, dir, prog string) {
	t.Helper()
	path := filepath.Join(dir, "verify.cpp")
	if err := os.WriteFile(path, []byte(prog), 0o644); err != nil {
		t.Fatal(err)
	}
	ran := 0
	for _, cxx := range []string{tsupport.ClangProgram(), "g++"} {
		if _, err := exec.LookPath(cxx); err != nil {
			continue
		}
		ran++
		cmd := exec.Command(cxx, "-std=c++20", "-fsyntax-only", "-w", "-iquote", dir, path)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("%s rejects what the extraction claims:\n%s\n--- program:\n%s", cxx, firstLines(string(out), 30), prog)
		}
	}
	if ran == 0 {
		t.Skip("no C++ compiler")
	}
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// What the extraction reports is true: the compiler agrees with every fact.
func TestDeclsVerified(t *testing.T) {
	cfg, dc := declsSetup(t)
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, []DeclQuery{
		{DeclType, "geo::Vec"}, {DeclType, "geo::Shape"}, {DeclType, "geo::Body"}, {DeclType, "Rect"}, {DeclType, "Pt"}, {DeclType, "U"},
		{DeclType, "Opaque"}, {DeclType, "Box"}, {DeclType, "Box::Inner"}, {DeclType, "Box::E"}, {DeclType, "geo::Kind"}, {DeclType, "geo::Old"},
		{DeclType, "FILE"}, {DeclType, "Id"},
		{DeclFunc, "geo::f"}, {DeclFunc, "fopen"}, {DeclFunc, "cfun"}, {DeclFunc, "puts"}, {DeclFunc, "std::abs"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"geo::Vec", "Box", "Rect"} {
		if tab.Records[k] == nil {
			t.Fatalf("no record %s: %v", k, tab.Errors)
		}
	}
	compileVerify(t, filepath.Dir(cfg.Dir+"/x"), verifyProgram(tab, "decls.hpp"))
}
