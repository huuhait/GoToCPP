package typedinterop

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// result makes a failed or compiled probe result for ClassifyMember.
func result(ok bool, errMsg string, notes ...string) *probe.Result {
	r := &probe.Result{OK: ok}
	if ok && errMsg != "" {
		r.Recovered = errMsg
		for _, n := range notes {
			r.RecoveredNotes = append(r.RecoveredNotes, probe.Diag{Severity: "note", Msg: n})
		}
		return r
	}
	r.Error = errMsg
	for _, n := range notes {
		r.Notes = append(r.Notes, probe.Diag{Severity: "note", Msg: n})
	}
	return r
}

// TestClassifyMember pins the reading of the compiler's words (clang 17 to 21 print these).
func TestClassifyMember(t *testing.T) {
	for _, c := range []struct {
		name   string
		member string // the name that was asked
		r      *probe.Result
		want   check.MemberStatus
	}{
		{"a data member", "x", result(true, ""), check.MemberFound},
		{"a method", "m", result(false, "reference to non-static member function must be called"), check.MemberFound},
		{"a method, with the hint", "m", result(false, "reference to non-static member function must be called; did you mean to call it with no arguments?"), check.MemberFound},
		{"a method, clang recovered", "m", result(true, "reference to non-static member function must be called; did you mean to call it with no arguments?"), check.MemberFound},
		{"missing", "foo", result(false, "no member named 'foo' in 'std::vector<long>'"), check.MemberMissing},
		{"missing, with the hint", "sze", result(false, "no member named 'sze' in 'std::vector<long>'; did you mean 'size'?"), check.MemberMissing},
		{"another name missing: not an answer", "foo", result(false, "no member named 'other' in 'S'"), check.MemberUnknown},
		{"private", "bf", result(false, "'bf' is a private member of 'B'", "constrained by private inheritance here"), check.MemberPrivate},
		{"private, clang recovered", "priv", result(true, "'priv' is a private member of 'M'"), check.MemberPrivate},
		{"protected", "prot", result(false, "'prot' is a protected member of 'M'"), check.MemberProtected},
		{"ambiguous bases (a non-virtual diamond)", "v", result(false, "non-static member 'v' found in multiple base-class subobjects of type 'L'"), check.MemberAmbiguous},
		{"ambiguous bases", "x", result(false, "member 'x' found in multiple base classes of different types"), check.MemberAmbiguous},
		{"deleted", "del", result(false, "attempt to use a deleted function"), check.MemberDeleted},
		{"a type", "Ty", result(false, "cannot refer to type member 'Ty' in 'M' with '.'"), check.MemberNotValue},
		{"a macro", "MAC", result(false, "expected unqualified-id", "expanded from macro 'MAC'"), check.MemberMacro},
		{"a macro that is not the name", "MAC", result(false, "expected unqualified-id", "expanded from macro 'OTHER'"), check.MemberUnknown},
		{"a macro that gives another name", "MAC", result(false, "no member named 'x' in 'S'", "expanded from macro 'MAC'"), check.MemberMacro},
		{"something else", "foo", result(false, "unknown type name 'Foo'"), check.MemberUnknown},
		{"recovered from something else", "m", result(true, "member reference type 'S *' is a pointer; did you mean to use '->'?"), check.MemberUnknown},
		{"skipped", "m", &probe.Result{Skipped: "incomplete"}, check.MemberUnknown},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifyMember(c.r, c.member).Status; got != c.want {
				t.Errorf("ClassifyMember = %v, want %v", got, c.want)
			}
		})
	}
}

const zooHeader = `#pragma once
#include <string>
#include <vector>
#define zoo_macro 7
#define zoo_alias size
struct Root {
    int root_field = 1;
    int root_method() const { return 2; }
    static int root_static;
};
struct PubDerived : Root {};
struct PrivDerived : private Root {};
struct ProtDerived : protected Root {};
struct LeftB { int amb = 1; };
struct RightB { int amb = 2; };
struct Diamond : LeftB, RightB {};
struct L { int v = 1; int lv() const { return 1; } };
struct DL : L {};
struct DR : L {};
struct NvDiamond : DL, DR {};
struct VL : virtual L {};
struct VR : virtual L {};
struct VDiamond : VL, VR {};
struct Members {
    int field = 3;
    static int stat;
    enum Kind { KA = 5, KB };
    using Alias = int;
    struct Nested {};
    int over(int) const { return 1; }
    int over(double) { return 2; }
    template <class T> T tmpl(T v) const { return v; }
    void gone() = delete;
    Members(const Members&) = default;
    Members() = default;
    void operator+=(int) {}
private:
    int hidden = 0;
    void hidden_fn() {}
protected:
    int guarded = 0;
};
struct Pointee { int pointee_field = 1; int shared() const { return 1; } };
struct Arrow {
    Pointee* operator->() { return &p; }
    int shared() const { return 2; }
    Pointee p;
};
struct Smart : std::vector<int> {};
`

// TestMembersClang asks the real compiler about the members of the classes of a header, through a
// struct that embeds one class each, and checks what the second pass makes of the answers: a
// selector marked "// path: N" is promoted through that path, one marked "// none" is not, and the
// errors are listed.
func TestMembersClang(t *testing.T) {
	type sel struct{ name, want string }
	for _, c := range []struct {
		class string
		sels  []sel
		errs  []string
	}{
		{"zoo.Members", []sel{
			{"field", "path: 0"}, {"stat", "path: 0"}, {"KA", "path: 0"}, {"over", "path: 0"}, {"tmpl", "path: 0"},
		}, nil},
		{"zoo.Members", []sel{{"hidden", "none"}, {"hidden_fn", "none"}, {"guarded", "none"}},
			[]string{"s.hidden undefined (the member hidden of Members is private)", "s.hidden_fn undefined (the member hidden_fn of Members is private)", "s.guarded undefined (the member guarded of Members is protected)"}},
		{"zoo.Members", []sel{{"gone", "none"}}, []string{"s.gone is a deleted function of Members"}},
		{"zoo.Members", []sel{{"Alias", "none"}, {"Nested", "none"}},
			[]string{"s.Alias undefined (type S has no field or method Alias; Members has no member Alias) (Alias is a type of the class, not a member)",
				"s.Nested undefined (type S has no field or method Nested; Members has no member Nested) (Nested is a type of the class, not a member)"}},
		{"zoo.Members", []sel{{"nosuch", "none"}, {"operator_plus", "none"}},
			[]string{"s.nosuch undefined (type S has no field or method nosuch; Members has no member nosuch)", "s.operator_plus undefined (type S has no field or method operator_plus; Members has no member operator_plus)"}},
		{"zoo.PubDerived", []sel{{"root_field", "path: 0"}, {"root_method", "path: 0"}, {"root_static", "path: 0"}}, nil},
		{"zoo.PrivDerived", []sel{{"root_field", "none"}, {"root_method", "none"}},
			[]string{"s.root_field undefined (the member root_field of PrivDerived is private)", "s.root_method undefined (the member root_method of PrivDerived is private)"}},
		{"zoo.ProtDerived", []sel{{"root_field", "none"}}, []string{"s.root_field undefined (the member root_field of ProtDerived is protected)"}},
		{"zoo.Diamond", []sel{{"amb", "none"}}, []string{"ambiguous selector s.amb (Diamond has a member amb in several bases)"}},
		// The same base twice (a non-virtual diamond): clang words it differently; a virtual one is fine.
		{"zoo.NvDiamond", []sel{{"v", "none"}, {"lv", "none"}},
			[]string{"ambiguous selector s.v (NvDiamond has a member v in several bases)", "ambiguous selector s.lv (NvDiamond has a member lv in several bases)"}},
		{"zoo.VDiamond", []sel{{"v", "path: 0"}, {"lv", "path: 0"}}, nil},
		// A class with an operator-> and no member of the name is not decided (the pointee may have it:
		// typed code generation writes x->name): the first pass's rule for one embedded class stands,
		// and the C++ compiler decides. Its own members are decided.
		{"zoo.Arrow", []sel{{"shared", "path: 0"}, {"p", "path: 0"}, {"pointee_field", "path: 0"}}, nil},
		// The macro: the name is something else in the C++ text.
		{"zoo.Members", []sel{{"zoo_macro", "none"}},
			[]string{"s.zoo_macro undefined (type S has no field or method zoo_macro; Members has no member zoo_macro) (zoo_macro is a macro of the headers)"}},
		// std classes, whose bases the declaration extraction does not follow.
		{"std.string", []sel{{"append", "path: 0"}, {"length", "path: 0"}, {"npos", "path: 0"}, {"nosuch", "none"}}, []string{"S has no field or method nosuch; std.string has no member nosuch"}},
		{"zoo.Smart", []sel{{"push_back", "path: 0"}, {"size", "path: 0"}}, nil},
	} {
		var body strings.Builder
		for _, s := range c.sels {
			fmt.Fprintf(&body, "\t_ = s.%s // %s\n", s.name, s.want)
		}
		src := fmt.Sprintf("package main\n\nimport (\n\t\"<string>\"\n\t\"./zoo.hpp\"\n)\n\ntype S struct{ %s }\n\nfunc main() {\n\tvar s S\n%s}\n", c.class, body.String())
		t.Run(c.class+"/"+c.sels[0].name, func(t *testing.T) {
			p, fset, errs := checkWithHook(t, src, map[string]string{"zoo.hpp": zooHeader}, nil)
			text := ""
			for _, d := range errs {
				text += d + "\n"
			}
			for _, want := range c.errs {
				if !strings.Contains(text, want) {
					t.Errorf("no error with %q in:\n%s", want, text)
				}
			}
			if len(errs) != len(c.errs) {
				t.Errorf("%d errors, want %d:\n%s", len(errs), len(c.errs), text)
			}
			lines := strings.Split(src, "\n")
			n := 0
			for _, f := range p.Load.Files {
				ast.Inspect(f.AST, func(node ast.Node) bool {
					s, ok := node.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					line := lines[fset.Position(s.Pos()).Line-1]
					i := strings.LastIndex(line, "// ")
					if i < 0 || !strings.HasPrefix(line, "\t_ = s.") {
						return true
					}
					want := line[i+3:]
					got := "none"
					if path := p.PromotedPath(s); path != nil {
						got = fmt.Sprintf("path: %d", path[0])
					}
					n++
					if got != want {
						t.Errorf("%s: PromotedPath is %s, want %s", strings.TrimSpace(line[:i]), got, want)
					}
					return false
				})
			}
			if n != len(c.sels) {
				t.Errorf("checked %d selectors, want %d", n, len(c.sels))
			}
		})
	}
}

// checkWithHook checks the program main.gp (and the header files) with the second pass for promoted
// members on, answered by the real compiler. It returns the package and the messages of its
// errors, without the positions.
func checkWithHook(t *testing.T, src string, files map[string]string, rn *probe.Runner) (*check.Package, *token.FileSet, []string) {
	t.Helper()
	return checkInDir(t, t.TempDir(), src, files, rn)
}

// checkInDir is checkWithHook in the folder dir (the folder is in the probe's key: a test of the
// cache checks twice in the same one).
func checkInDir(t *testing.T, dir, src string, files map[string]string, rn *probe.Runner) (*check.Package, *token.FileSet, []string) {
	t.Helper()
	clang := tsupport.Clang(t)
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "main.gp"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if rn == nil {
		rn = new(probe.Runner)
	}
	p, fset := tsupport.CheckDirConfig(t, dir, "main.gp", func(c *check.Config) {
		c.CppMembers = MembersHook(Options{Clang: clang, Runner: rn})
	})
	var msgs []string
	for _, d := range p.Errors.Diagnostics() {
		msgs = append(msgs, d.Msg)
	}
	return p, fset, msgs
}

// TestMembersCache checks that the answers are cached on disk with the other probes (ADR-088): the
// second check of the same program, by a new runner (another process), runs no compiler, and one
// check asks the compiler once for all the classes and names.
func TestMembersCache(t *testing.T) {
	t.Setenv("GOCPP_NOPROBECACHE", "")
	t.Setenv("GOCPP_PROBE_CACHE", t.TempDir())
	const src = `package main

import (
	"<string>"
	"<vector>"
)

type S struct {
	std.vector[int]
	std.string
}

func main() {
	var s S
	s.emplace_back(1) // path: 0
	s.append("x")     // path: 1
	_ = s.length      // path: 1
	_ = s.nosuch
}
`
	dir := t.TempDir()
	rn1 := new(probe.Runner)
	p, _, errs := checkInDir(t, dir, src, nil, rn1)
	if len(errs) != 1 || !strings.Contains(errs[0], "none of the embedded C++ types std.vector[int], std.string has a member nosuch") {
		t.Fatalf("errors: %q", errs)
	}
	if rn1.Runs() != 1 {
		t.Errorf("the cold check ran the compiler %d times for the members, want 1", rn1.Runs())
	}
	if p.CppMembersAsked() != 8 {
		t.Errorf("asked %d questions, want 8 (4 names, each for the vector and the string)", p.CppMembersAsked())
	}
	rn2 := new(probe.Runner)
	_, _, errs2 := checkInDir(t, dir, src, nil, rn2)
	if rn2.Runs() != 0 {
		t.Errorf("the warm check ran the compiler %d times, want 0", rn2.Runs())
	}
	if strings.Join(errs, "\n") != strings.Join(errs2, "\n") {
		t.Errorf("cold and warm differ:\n%q\n%q", errs, errs2)
	}
}

// TestMembersNoCandidates checks that a program whose selectors the first pass decides never runs the
// compiler for the second pass.
func TestMembersNoCandidates(t *testing.T) {
	rn := new(probe.Runner)
	p, _, errs := checkWithHook(t, `package main

import "<vector>"

type S struct {
	n int
	std.vector[int]
}

func main() {
	var s S
	s.n = 1
	var v std.vector[int]
	v.push_back(1)
	s.vector.push_back(2)
}
`, nil, rn)
	if len(errs) != 0 || rn.Runs() != 0 || p.CppMembersAsked() != 0 {
		t.Errorf("errors %q, %d compiler runs, %d questions", errs, rn.Runs(), p.CppMembersAsked())
	}
}
