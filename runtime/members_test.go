package rtfiles_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// TestNoPlainMembers checks the rule "no plain-named members" (Phase 03 §2.1, CONTRIBUTING
// §5) on every class of the runtime's headers: a class in namespace gp may have only
// constructors, destructors, operators, friends and members whose names start with gp_
// (public or private: data members, member functions, member templates, member types, nested
// classes, and the names that a using-declaration brings in), and no base class outside
// namespace gp, which would bring its own members (std::shared_ptr's get). So a Go method or
// field of any other name can't clash with a runtime member, also when a named Go type is
// built on a runtime type. The classes in a namespace named detail are left out: they never
// hold a Go value.
//
// clang++ lists the declarations (-ast-dump=json, only those whose qualified names contain
// "gp::"). Skipped without clang++, unless GOCPP_REQUIRE_CXX names it.
func TestNoPlainMembers(t *testing.T) {
	clang := clangCompiler(t)
	rt := extractRuntime(t)
	bad, classes, err := plainMembers(t, clang, filepath.Join(rt, "include"), "#include <gocpp/rt.hpp>\n#include <gocpp/os.hpp>\n")
	if err != nil {
		t.Fatal(err)
	}
	// The check must see the runtime's types, or it checks nothing.
	for _, c := range []string{"gp::string", "gp::ptr", "gp::weak_ptr", "gp::panic_error", "gp::slice", "gp::array",
		"gp::map", "gp::map_range", "gp::hash", "gp::fmtshim::typed_value", "gp::defer_frame",
		"gp::newtype", "gp::func"} {
		if !slices.Contains(classes, c) {
			t.Errorf("the check didn't see the class %s; it saw %q", c, classes)
		}
	}
	for _, m := range bad {
		t.Errorf("%s: a member of a runtime type must be an operator or have a gp_ name", m)
	}
}

// TestNoPlainMembersFinds checks the check itself, on a header with every kind of member.
func TestNoPlainMembersFinds(t *testing.T) {
	clang := clangCompiler(t)
	src := `#include <memory>
namespace gp {
template <typename T>
struct leaky : std::shared_ptr<T> {};
struct hidden : private std::shared_ptr<int> {
    using std::shared_ptr<int>::get;
};
template <typename T>
struct hidden2 : private std::shared_ptr<T> {
    using std::shared_ptr<T>::reset;
};
struct gbase {
    int gp_x;
    gbase(int);
};
struct derived : gbase {
    using gbase::gbase;
    using gbase::gp_x;
};
template <typename T>
struct gtbase {
    T gp_v;
};
template <typename T>
struct derived2 : gtbase<T> {
    using gtbase<T>::gp_v;
};
struct derived3 : gtbase<int> {};
template <typename T>
struct pbase : T {
    using T::T;
};
struct bad {
    int get;
    int gp_ok;
    void reset();
    void gp_fine();
    template <typename T> void tmpl();
    template <typename T> bad(T);
    using value_type = int;
    enum kind_t { a, gp_b };
    enum class gp_scoped { c };
    struct inner { int x; int gp_y; };
    union { int u; int gp_v; };
    static constexpr int size = 1;
    bool operator==(const bad&) const;
    explicit operator bool() const;
    bad();
    ~bad();
    friend void len(bad);
private:
    int len_;
};
template <typename T>
class tmpl_bad {
    T value;
    T gp_value;
};
template <typename T, int N>
struct spec { T gp_a[N]; };
template <typename T>
struct spec<T, 0> {
    int count;
};
template <int N>
struct spec<char, N> {
    char gp_c[N];
};
template <template <typename> class W>
struct spec<W<int>, 1> {
    int gp_w;
};
struct outer {
    template <typename T, int N>
    struct gp_in { T gp_a; };
    template <typename T>
    struct gp_in<T, 0> { int part; };
    template <>
    struct gp_in<char, 1> { int full; };
};
namespace detail {
struct fine { int get; };
}
namespace sub {
struct nested { int data; };
}
}
`
	got, _, err := plainMembers(t, clang, t.TempDir(), src)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"gp::bad::a", "gp::bad::get", "gp::bad::inner", "gp::bad::inner::x", "gp::bad::kind_t",
		"gp::bad::len_", "gp::bad::reset", "gp::bad::size", "gp::bad::tmpl", "gp::bad::u",
		"gp::bad::value_type", "gp::hidden (base std::shared_ptr<int>)", "gp::hidden2 (base std::shared_ptr<T>)",
		"gp::hidden2::reset", "gp::hidden::get", "gp::leaky (base std::shared_ptr<T>)",
		"gp::outer::gp_in::full", "gp::outer::gp_in::part", "gp::pbase (base T)", "gp::pbase::type-parameter-0-0", "gp::spec::count", "gp::sub::nested::data", "gp::tmpl_bad::value"}
	if !slices.Equal(got, want) {
		t.Errorf("found %q\nwant  %q", got, want)
	}
}

// clangCompiler returns the clang++ of the test matrix, or skips the test.
func clangCompiler(t *testing.T) cxx {
	t.Helper()
	for _, c := range compilers(t) {
		if c.cc.Family == build.Clang || c.name == "clang++" {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			return c
		}
	}
	t.Skip("the tests don't use clang++ (GOCPP_CXX)")
	return cxx{}
}

// An astNode is a declaration in clang's JSON dump: only the fields the check reads.
type astNode struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	IsImplicit bool   `json:"isImplicit"`
	ScopedEnum string `json:"scopedEnumTag"`
	Bases      []struct {
		Type struct {
			QualType  string `json:"qualType"`
			Desugared string `json:"desugaredQualType"`
		} `json:"type"`
	} `json:"bases"`
	Inner []astNode `json:"inner"`
}

// plainMembers compiles src (a C++ file that includes headers from the folder inc) with
// clang, and returns the sorted qualified names of the members of classes in namespace gp
// that break the rule, and the names of the classes it checked.
func plainMembers(t *testing.T, clang cxx, inc, src string) (bad, classes []string, err error) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "members.cpp")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		return nil, nil, err
	}
	args := []string{"-std=c++20", "-fsyntax-only", "-I", inc, "-Xclang", "-ast-dump=json",
		"-Xclang", "-ast-dump-filter=gp::", file}
	out, stderr, code, err := command(dir, build.CompilerEnv(), false, clang.cc.Path, args...)
	if err != nil {
		return nil, nil, err
	}
	if code != 0 {
		return nil, nil, fmt.Errorf("%s %s: exit code %d\n%s", clang.cc.Path, strings.Join(args, " "), code, stderr)
	}
	// The dump is one JSON object per declaration that matches the filter: the declarations
	// directly in namespace gp (a nested namespace comes whole, with everything in it).
	var c checker
	dec := json.NewDecoder(bytes.NewReader(out))
	n := 0
	for {
		var d astNode
		if err := dec.Decode(&d); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, nil, err
		}
		n++
		c.walkScope(d, "gp")
	}
	if n == 0 {
		return nil, nil, errors.New("clang's dump has no declarations in namespace gp")
	}
	c.resolveBases()
	slices.Sort(c.bad)
	return slices.Compact(c.bad), c.classes, nil
}

// A checker collects the members that break the rule, and the classes it checked.
type checker struct {
	bad, classes []string
	unqualified  []unqualifiedBase
}

// An unqualifiedBase is a base class named without a namespace ("gtbase<T>"), of the class
// class.
type unqualifiedBase struct {
	class, base string
}

// resolveBases reports the unqualified bases that aren't classes of namespace gp.
func (k *checker) resolveBases() {
	for _, u := range k.unqualified {
		ns, _ := splitQualified(u.class)
		if !slices.Contains(k.classes, ns+"::"+stripArgs(u.base)) && !slices.Contains(k.classes, "gp::"+stripArgs(u.base)) {
			k.bad = append(k.bad, u.class+" (base "+u.base+")")
		}
	}
}

// walkScope checks the classes of the namespace-scope declaration d in the namespace scope.
func (k *checker) walkScope(d astNode, scope string) {
	switch d.Kind {
	case "NamespaceDecl":
		if d.Name == "detail" {
			return
		}
		for _, c := range d.Inner {
			k.walkScope(c, scope+"::"+d.Name)
		}
	case "CXXRecordDecl", "ClassTemplateSpecializationDecl", "ClassTemplatePartialSpecializationDecl":
		k.checkRecord(d, scope+"::"+d.Name)
	case "ClassTemplateDecl":
		// The template itself; its instantiations repeat its members.
		for _, c := range d.Inner {
			if c.Kind == "CXXRecordDecl" {
				k.checkRecord(c, scope+"::"+d.Name)
			}
		}
	}
}

// paramBases are the class templates whose base class is their template parameter U, which
// their requires-clause limits to the runtime's own classes: gp::newtype<D, U> over a gp::string,
// gp::slice, gp::array, gp::map or gp::ptr (gocpp/newtype.hpp: detail::runtime_class), each of
// them checked here as a class of its own. So such a base brings no plain-named member either.
var paramBases = map[string]bool{"gp::newtype": true}

// checkRecord checks the members of the class r, named name.
func (k *checker) checkRecord(r astNode, name string) {
	k.classes = append(k.classes, name)
	check := func(m string) {
		if m != "" && !strings.HasPrefix(m, "gp_") && !strings.HasPrefix(m, "operator") {
			k.bad = append(k.bad, name+"::"+m)
		}
	}
	// A base class brings its members in: a base outside namespace gp (std::shared_ptr<T>)
	// would bring get and reset. A base in gp is checked as a class of its own. clang writes a
	// base in gp without gp:: when the code does ("gtbase<T>"), so such a name is resolved at
	// the end, against the classes the check saw.
	var baseNames []string
	var params []string // the template parameters of a partial specialization
	for _, m := range r.Inner {
		if m.Kind == "TemplateTypeParmDecl" {
			params = append(params, m.Name)
		}
	}
	paramBase := false // a base that is a template parameter (paramBases)
	for _, b := range r.Bases {
		q, d := strings.TrimPrefix(b.Type.QualType, "::"), strings.TrimPrefix(b.Type.Desugared, "::")
		_, last := splitQualified(q)
		baseNames = append(baseNames, stripArgs(last))
		switch {
		case paramBases[name] && slices.Contains(params, q):
			paramBase = true
		case strings.HasPrefix(q, "gp::") || strings.HasPrefix(d, "gp::"):
		case !strings.Contains(stripArgs(q), "::"):
			k.unqualified = append(k.unqualified, unqualifiedBase{name, q})
		default:
			k.bad = append(k.bad, name+" (base "+q+")")
		}
	}
	for _, m := range r.Inner {
		switch {
		case strings.HasSuffix(m.Kind, "Attr"), strings.HasSuffix(m.Kind, "Comment"):
		case m.Kind == "AccessSpecDecl", m.Kind == "FriendDecl", m.Kind == "StaticAssertDecl",
			m.Kind == "CXXConstructorDecl", m.Kind == "CXXDestructorDecl", m.Kind == "CXXConversionDecl":
		case m.Kind == "TemplateTypeParmDecl", m.Kind == "NonTypeTemplateParmDecl", m.Kind == "TemplateTemplateParmDecl":
			// The template parameters of a partial specialization (array<T, 0>): not members.
		case strings.HasSuffix(m.Kind, "Expr"):
			// The requires-clause of a partial specialization (newtype's runtime_class<U>).
		case m.IsImplicit:
			// The injected class name, implicit members, the fields of an anonymous union.
		case m.Kind == "CXXRecordDecl":
			if m.Name == "" {
				k.checkRecord(m, name) // an anonymous union or struct: its members are r's
				continue
			}
			check(m.Name)
			k.checkRecord(m, name+"::"+m.Name)
		case m.Kind == "ClassTemplateDecl":
			check(m.Name)
			for _, c := range m.Inner {
				if c.Kind == "CXXRecordDecl" {
					k.checkRecord(c, name+"::"+m.Name)
				}
			}
		case m.Kind == "ClassTemplatePartialSpecializationDecl", m.Kind == "ClassTemplateSpecializationDecl":
			// A specialization of a member template, declared in the class: its members are
			// checked like the template's.
			check(m.Name)
			k.checkRecord(m, name+"::"+m.Name)
		case m.Kind == "FunctionTemplateDecl":
			if !slices.ContainsFunc(m.Inner, func(c astNode) bool {
				return c.Kind == "CXXConstructorDecl" || c.Kind == "CXXDestructorDecl" || c.Kind == "CXXConversionDecl"
			}) {
				check(m.Name)
			}
		case m.Kind == "UsingDecl", m.Kind == "UnresolvedUsingValueDecl", m.Kind == "UnresolvedUsingTypenameDecl":
			// using base::get; brings get in. clang names it by the qualified name
			// ("std::shared_ptr<int>::get"). using base::base (inheriting the constructors)
			// is fine.
			_, short := splitQualified(m.Name)
			// clang names using U::U (the constructors of a base that is a template
			// parameter) by U's canonical name, type-parameter-0-1.
			inheriting := short == r.Name || slices.Contains(baseNames, stripArgs(short)) ||
				paramBase && strings.HasPrefix(m.Name, "type-parameter-") ||
				slices.ContainsFunc(m.Inner, func(c astNode) bool { return c.Kind == "ConstructorUsingShadowDecl" })
			if !inheriting {
				check(short)
			}
		case m.Kind == "EnumDecl":
			check(m.Name)
			if m.ScopedEnum == "" {
				// The names of an unscoped enum are members of the class too.
				for _, c := range m.Inner {
					if c.Kind == "EnumConstantDecl" {
						check(c.Name)
					}
				}
			}
		default:
			check(m.Name)
		}
	}
}

// splitQualified splits "a::b<c::d>::e" into "a::b<c::d>" and "e".
func splitQualified(name string) (qual, last string) {
	depth := 0
	for i := len(name) - 1; i > 0; i-- {
		switch name[i] {
		case '>':
			depth++
		case '<':
			depth--
		case ':':
			if depth == 0 && name[i-1] == ':' {
				return name[:i-1], name[i+1:]
			}
		}
	}
	return "", name
}

// stripArgs removes template arguments: "shared_ptr<int>" is "shared_ptr".
func stripArgs(name string) string {
	if i := strings.IndexByte(name, '<'); i >= 0 {
		return name[:i]
	}
	return name
}
