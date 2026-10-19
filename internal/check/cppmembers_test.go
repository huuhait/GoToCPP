package check

import (
	"context"
	"fmt"
	"go/ast"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// fakeMembers is a CppMembers hook that answers from a table: "class.name" -> status. A pair not
// in the table is missing. It counts the calls and the questions.
type fakeMembers struct {
	table   map[string]MemberStatus
	calls   atomic.Int32
	asked   []string
	unknown bool // every answer is MemberUnknown
}

func (f *fakeMembers) hook(_ context.Context, _ *Package, qs []MemberQuery) ([]MemberAnswer, error) {
	f.calls.Add(1)
	out := make([]MemberAnswer, len(qs))
	for i, q := range qs {
		key := classText(q.Class) + "." + q.Name
		f.asked = append(f.asked, key)
		switch {
		case f.unknown:
			out[i] = MemberAnswer{Status: MemberUnknown}
		default:
			st, ok := f.table[key]
			if !ok {
				st = MemberMissing
			}
			out[i] = MemberAnswer{Status: st, Why: "fake"}
		}
	}
	return out, nil
}

// checkWithMembers loads and checks src as main.gp with the hook and the gates of the phase.
func checkWithMembers(t *testing.T, phase int, src string, hook CppMembers) (prog *load.Program, p *Package, errs string) {
	t.Helper()
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	cfg := load.Config{Dir: root}
	cfg.Stdlib, cfg.GoStd = stdlib.FS(), stdlib.IsStd
	cfg.GOOS, cfg.GOARCH = "linux", "amd64"
	prog = load.Load(cfg, ".")
	c := fakeProbeConfig(phase)
	c.CppMembers = hook
	r := Check(prog, c)
	p = mainPkg(t, prog, r)
	return prog, p, errorText(t, prog, r, root, true)
}

const membersHead = "package main\n\nimport \"<string>\"\nimport \"<vector>\"\n\n"

// TestCppMembers pins what the second pass decides from the answers about the classes. A selector
// marked "// path: N.M" must be promoted through that path, one marked "// none" must not.
func TestCppMembers(t *testing.T) {
	for _, c := range []struct {
		name  string
		src   string
		table map[string]MemberStatus
		errs  []string // substrings of the errors, in order; none: no error
		calls int      // questions the hook is asked (distinct class and name)
	}{
		{"one class has the member", `
type S struct {
	n int
	std.vector[int]
}

func main() {
	var s S
	s.push_back(1) // path: 1
	s.n++          // none
}
`, map[string]MemberStatus{"std.vector[int].push_back": MemberFound}, nil, 1},

		{"two classes, one has it", `
type S struct {
	std.vector[int]
	std.string
}

func main() {
	var s S
	s.push_back(1) // path: 0
	s.append("x")  // path: 1
}
`, map[string]MemberStatus{"std.vector[int].push_back": MemberFound, "std.string.append": MemberFound}, nil, 4},

		{"two classes, both have it", `
type S struct {
	std.vector[int]
	std.string
}

func main() {
	var s S
	_ = s.size() // none
}
`, map[string]MemberStatus{"std.vector[int].size": MemberFound, "std.string.size": MemberFound},
			[]string{"ambiguous selector s.size (std.vector[int].size and std.string.size)"}, 2},

		{"a member nobody has", `
type S struct {
	std.vector[int]
	std.string
}

func main() {
	var s S
	_ = s.nosuch // none
}
`, nil, []string{"s.nosuch undefined (type S has no field or method nosuch; none of the embedded C++ types std.vector[int], std.string has a member nosuch)"}, 2},

		{"a member the one class lacks", `
type S struct{ std.vector[int] }

func main() {
	var s S
	_ = s.nosuch // none
}
`, nil, []string{"s.nosuch undefined (type S has no field or method nosuch; std.vector[int] has no member nosuch)"}, 1},

		{"through embedded structs and pointers", `
type Inner struct {
	a int
	*std.vector[int]
}

type Mid struct {
	z int
	Inner
}

type Top struct {
	std.string
	Mid
}

func main() {
	var t Top
	t.push_back(1) // path: 1.1.1
	t.length()     // path: 0
	t.a++          // none
}
`, map[string]MemberStatus{"*std.vector[int].push_back": MemberFound, "std.string.length": MemberFound}, nil, 5},

		{"the shallower member wins (Go's depth rule)", `
type Deep struct{ std.vector[int] }

type S struct {
	std.string
	Deep
}

func main() {
	var s S
	_ = s.size() // path: 0
}
`, map[string]MemberStatus{"std.string.size": MemberFound, "std.vector[int].size": MemberFound}, nil, 2},

		{"a Go field beside a class without the name", `
type Inner struct{ size int }

type S struct {
	std.vector[int]
	Inner
}

func main() {
	var s S
	s.size = 1 // none
}
`, nil, nil, 1},

		{"a Go field and a class with the name: ambiguous", `
type Inner struct{ size int }

type S struct {
	std.vector[int]
	Inner
}

func main() {
	var s S
	s.size = 1 // none
}
`, map[string]MemberStatus{"std.vector[int].size": MemberFound},
			[]string{"ambiguous selector s.size (a Go member and std.vector[int].size)"}, 1},

		{"a Go field above the class's member wins", `
type Inner struct{ size int }
type Mid struct{ std.vector[int] }

type S struct {
	Mid
	Inner
}

func main() {
	var s S
	s.size = 1 // none
}
`, map[string]MemberStatus{"std.vector[int].size": MemberFound}, nil, 0},

		{"the class's member hides a deeper Go field", `
type Deep struct{ size int }
type Mid struct{ Deep }

type S struct {
	std.vector[int]
	Mid
}

func main() {
	var s S
	s.size = 1 // none
}
`, map[string]MemberStatus{"std.vector[int].size": MemberFound},
			[]string{"s.size is the member of std.vector[int], which is shallower than the Go member size (Go's rule): name the field, s.vector.size"}, 1},

		{"private and protected members", `
type S struct{ std.vector[int] }

func main() {
	var s S
	_ = s.priv // none
	_ = s.prot // none
}
`, map[string]MemberStatus{"std.vector[int].priv": MemberPrivate, "std.vector[int].prot": MemberProtected},
			[]string{"s.priv undefined (the member priv of std.vector[int] is private)", "s.prot undefined (the member prot of std.vector[int] is protected)"}, 2},

		{"a deleted function, a base found twice, a type", `
type S struct{ std.vector[int] }

func main() {
	var s S
	s.del() // none
	s.dup() // none
	_ = s.Ty // none
}
`, map[string]MemberStatus{"std.vector[int].del": MemberDeleted, "std.vector[int].dup": MemberAmbiguous, "std.vector[int].Ty": MemberNotValue},
			[]string{"s.del is a deleted function of std.vector[int]", "ambiguous selector s.dup (std.vector[int] has a member dup in several bases)",
				"s.Ty undefined (type S has no field or method Ty; std.vector[int] has no member Ty) (Ty is a type of the class, not a member)"}, 3},

		{"a macro", `
type S struct{ std.vector[int] }

func main() {
	var s S
	_ = s.MAC // none
}
`, map[string]MemberStatus{"std.vector[int].MAC": MemberMacro},
			[]string{"s.MAC undefined (type S has no field or method MAC; std.vector[int] has no member MAC) (MAC is a macro of the headers)"}, 1},

		{"keywords of Go and names of the language", `
type S struct{ std.vector[int] }

func main() {
	var s S
	s.range()  // path: 0
	s.type()   // path: 0
	s.select() // path: 0
	s.len()    // path: 0
}
`, map[string]MemberStatus{"std.vector[int].range": MemberFound, "std.vector[int].type": MemberFound,
			"std.vector[int].select": MemberFound, "std.vector[int].len": MemberFound}, nil, 4},

		{"a named type over a class, and an alias", `
type A = std.vector[int]
type B std.string

type S struct {
	A
	B
}

func main() {
	var s S
	s.push_back(1) // path: 0
	s.append("x")  // path: 1
}
`, map[string]MemberStatus{"A.push_back": MemberFound, "B.append": MemberFound}, nil, 4},

		{"a method of a class that go/types resolved is not asked", `
type S struct{ std.vector[int] }

func (S) Get() int { return 1 }

func main() {
	var s S
	_ = s.Get() // none
	s.vector.size()
}
`, nil, nil, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeMembers{table: c.table}
			src := membersHead + c.src
			prog, p, errs := checkWithMembers(t, 5, src, f.hook)
			var got []string
			for _, e := range strings.Split(strings.TrimSpace(errs), "\n") {
				if e != "" && !strings.HasPrefix(e, "#") {
					got = append(got, e)
				}
			}
			if len(got) != len(c.errs) {
				t.Errorf("errors:\n%s\nwant %d: %q", errs, len(c.errs), c.errs)
			}
			for i, want := range c.errs {
				if i < len(got) && !strings.Contains(got[i], want) {
					t.Errorf("error %d is\n  %s\nwant it to contain\n  %s", i, got[i], want)
				}
			}
			if len(f.asked) != c.calls {
				t.Errorf("the hook was asked %d questions %v, want %d", len(f.asked), f.asked, c.calls)
			}
			if c.calls == 0 && f.calls.Load() != 0 {
				t.Errorf("the hook was called with nothing to ask")
			}
			if p.CppMembersAsked() != c.calls {
				t.Errorf("CppMembersAsked = %d, want %d", p.CppMembersAsked(), c.calls)
			}
			checkMarkedPaths(t, prog, p, src)
		})
	}
}

// checkMarkedPaths compares PromotedPath of the selectors that a comment marks with the comment.
func checkMarkedPaths(t *testing.T, prog *load.Program, p *Package, src string) {
	t.Helper()
	lines := strings.Split(src, "\n")
	n := 0
	for _, f := range p.Load.Files {
		inner := make(map[ast.Expr]bool)
		ast.Inspect(f.AST, func(node ast.Node) bool {
			if sel, ok := node.(*ast.SelectorExpr); ok {
				inner[sel.X] = true
			}
			return true
		})
		ast.Inspect(f.AST, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok || inner[sel] {
				return true
			}
			pos := prog.Fset.Position(sel.Pos())
			line := lines[pos.Line-1]
			i := strings.Index(line, "// ")
			if i < 0 {
				return true
			}
			want := strings.TrimSpace(line[i+3:])
			if !strings.HasPrefix(want, "path:") && want != "none" {
				return true
			}
			got := "none"
			if path := p.PromotedPath(sel); path != nil {
				parts := make([]string, len(path))
				for k, x := range path {
					parts[k] = fmt.Sprint(x)
				}
				got = "path: " + strings.Join(parts, ".")
			}
			n++
			if got != want {
				t.Errorf("line %d: %s: got %s, want %s", pos.Line, strings.TrimSpace(line[:i]), got, want)
			}
			return true
		})
	}
	if n == 0 {
		t.Errorf("no marked selector found")
	}
}

// TestCppMembersOptIn checks that without the hook nothing changes (the several-classes gate
// stays), that a hook with no answer leaves the first pass's result, and that the gates of the
// selectors the second pass decided are not reported.
func TestCppMembersOptIn(t *testing.T) {
	const src = membersHead + `
type S struct {
	std.vector[int]
	std.string
}

func main() {
	var s S
	s.push_back(1)
}
`
	// No hook: opaque mode. The gate says to name the field.
	_, _, errs := checkWithMembers(t, 5, src, nil)
	if !strings.Contains(errs, "promoted fields and methods of several embedded C/C++ types are not supported yet (Phase 12)") {
		t.Errorf("without the hook:\n%s", errs)
	}
	// A hook that cannot answer: the same gate.
	f := &fakeMembers{unknown: true}
	_, p, errs := checkWithMembers(t, 5, src, f.hook)
	if !strings.Contains(errs, "promoted fields and methods of several embedded C/C++ types are not supported yet (Phase 12)") || f.calls.Load() != 1 {
		t.Errorf("with an unknown hook (calls %d):\n%s", f.calls.Load(), errs)
	}
	if p.CppMembersAsked() != 2 {
		t.Errorf("asked %d", p.CppMembersAsked())
	}
	// A hook that answers: the gate is gone, the selector is promoted.
	f = &fakeMembers{table: map[string]MemberStatus{"std.vector[int].push_back": MemberFound}}
	_, _, errs = checkWithMembers(t, 5, src, f.hook)
	if errs != "" {
		t.Errorf("with answers:\n%s", errs)
	}
	// A hook that fails is the same as no answer.
	_, _, errs = checkWithMembers(t, 5, src, func(context.Context, *Package, []MemberQuery) ([]MemberAnswer, error) {
		return nil, fmt.Errorf("no clang")
	})
	if !strings.Contains(errs, "several embedded C/C++ types") {
		t.Errorf("with a failing hook:\n%s", errs)
	}
}

// TestCppMembersShadowGate checks the shadow gate: a Go member at the depth of an embedded class
// is a gate without the hook, and nothing when the class has no member of that name.
func TestCppMembersShadowGate(t *testing.T) {
	const src = membersHead + `
type Inner struct{ size int }

type S struct {
	std.vector[int]
	Inner
}

func main() {
	var s S
	s.size = 1
}
`
	_, _, errs := checkWithMembers(t, 5, src, nil)
	if !strings.Contains(errs, "selectors that a Go member and a member of an embedded C/C++ type could both match") {
		t.Errorf("without the hook:\n%s", errs)
	}
	f := &fakeMembers{}
	_, p, errs := checkWithMembers(t, 5, src, f.hook)
	if errs != "" {
		t.Errorf("a class without the member:\n%s", errs)
	}
	if !slices.Equal(f.asked, []string{"std.vector[int].size"}) {
		t.Errorf("asked %v", f.asked)
	}
	_ = p
}

// TestCppMembersNoCandidates checks that the hook is not called when the first pass decided
// everything: no selector on a struct with an embedded class that go/types could not resolve.
func TestCppMembersNoCandidates(t *testing.T) {
	f := &fakeMembers{}
	_, _, errs := checkWithMembers(t, 5, membersHead+`
type S struct {
	n int
	std.vector[int]
}

func main() {
	var s S
	s.n = 1
	var v std.vector[int]
	v.push_back(1)
}
`, f.hook)
	if errs != "" || f.calls.Load() != 0 {
		t.Errorf("calls %d, errors:\n%s", f.calls.Load(), errs)
	}
}

// TestCppMembersOtherPackage checks that a struct of another package, whose embedded classes this
// package's AST does not declare, is not decided (the first pass's result stands), and that the
// package that declares it is.
func TestCppMembersOtherPackage(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"two/two.gp": "package two\n\nimport \"<string>\"\nimport \"<vector>\"\n" + `
type Two struct {
	std.vector[int]
	std.string
}

func (t *Two) Use() {
	t.push_back(1) // decided here
}
`,
		"main.gp": "package main\n\nimport \"example.com/m/two\"\n\nfunc main() {\n\tvar t two.Two\n\tt.emplace_back(1)\n}\n",
	})
	f := &fakeMembers{table: map[string]MemberStatus{"std.vector[int].push_back": MemberFound, "std.vector[int].emplace_back": MemberFound}}
	cfg := load.Config{Dir: root}
	cfg.Stdlib, cfg.GoStd = stdlib.FS(), stdlib.IsStd
	cfg.GOOS, cfg.GOARCH = "linux", "amd64"
	prog := load.Load(cfg, ".")
	c := fakeProbeConfig(5)
	c.CppMembers = f.hook
	r := Check(prog, c)
	errs := errorText(t, prog, r, root, true)
	// two.Two.Use is decided (the hook answered push_back); main's t.emplace_back is a selector on a
	// struct of another package: not asked, so the gate for several classes stays.
	if strings.Contains(errs, "t.push_back") || !strings.Contains(errs, "main.gp:7:4: promoted fields and methods of several embedded C/C++ types") {
		t.Errorf("errors:\n%s", errs)
	}
	if !slices.Equal(f.asked, []string{"std.vector[int].push_back", "std.string.push_back"}) {
		t.Errorf("asked %v", f.asked)
	}
}

// TestCppMembersUnexportedOfOtherPackage checks that a selector for an unexported Go field of a struct
// of another package, embedded in a struct that also embeds a class, is not taken for a member of the
// class: go/types' error is the right one, and the hook is not asked about the name.
func TestCppMembersUnexportedOfOtherPackage(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"two/two.gp": "package two\n\ntype Plain struct {\n\tsize int\n\tName string\n}\n",
		"main.gp":    "package main\n\nimport (\n\t\"<vector>\"\n\t\"example.com/m/two\"\n)\n\ntype Mine struct {\n\ttwo.Plain\n\tstd.vector[int]\n}\n\nfunc main() {\n\tvar m Mine\n\tm.size = 1\n\tm.Name = \"x\"\n\tm.push_back(1)\n}\n",
	})
	f := &fakeMembers{table: map[string]MemberStatus{"std.vector[int].size": MemberFound, "std.vector[int].push_back": MemberFound}}
	cfg := load.Config{Dir: root}
	cfg.Stdlib, cfg.GoStd = stdlib.FS(), stdlib.IsStd
	cfg.GOOS, cfg.GOARCH = "linux", "amd64"
	prog := load.Load(cfg, ".")
	c := fakeProbeConfig(5)
	c.CppMembers = f.hook
	r := Check(prog, c)
	errs := errorText(t, prog, r, root, true)
	if !strings.Contains(errs, "m.size undefined (cannot refer to unexported field size)") {
		t.Errorf("errors:\n%s", errs)
	}
	for _, q := range f.asked {
		if strings.HasSuffix(q, ".size") {
			t.Errorf("the hook was asked %v", f.asked)
		}
	}
}

// TestCppMembersShortAnswer checks that a hook that answers fewer queries than it was asked leaves the first
// pass's result alone (no panic, no half-applied answers).
func TestCppMembersShortAnswer(t *testing.T) {
	const src = membersHead + `
type S struct {
	std.vector[int]
	std.string
}

func main() {
	var s S
	s.push_back(1)
	_ = s.nosuch
}
`
	_, p, errs := checkWithMembers(t, 5, src, func(_ context.Context, _ *Package, qs []MemberQuery) ([]MemberAnswer, error) {
		out := make([]MemberAnswer, len(qs)-1)
		for i := range out {
			out[i] = MemberAnswer{Status: MemberFound}
		}
		return out, nil
	})
	if !strings.Contains(errs, "several embedded C/C++ types") || strings.Contains(errs, "has no member") {
		t.Errorf("errors:\n%s", errs)
	}
	if p.PromotedPath(findSel(p, "push_back")) != nil {
		t.Errorf("a selector was decided from a short answer")
	}
}

// findSel returns the first selector of the package with the name.
func findSel(p *Package, name string) *ast.SelectorExpr {
	var out *ast.SelectorExpr
	for _, f := range p.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if s, ok := n.(*ast.SelectorExpr); ok && out == nil && s.Sel.Name == name {
				out = s
			}
			return true
		})
	}
	return out
}
