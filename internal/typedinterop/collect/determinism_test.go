package collect

import (
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

const detHeader = "package main\n\nimport (\n\t\"fmt\"\n\tcstring \"<cstring>\"\n\tcstdio \"<cstdio>\"\n\t\"<vector>\"\n\t\"<string>\"\n)\n\n"

func collectText(t *testing.T, src string) (string, *Set) {
	t.Helper()
	p, fset := tsupport.CheckSource(t, src, nil)
	if errs := p.Errors.Diagnostics(); len(errs) > 0 {
		t.Fatalf("the program doesn't check: %v\n%s", errs, src)
	}
	set := Collect(p, fset)
	var b strings.Builder
	set.Format(&b, false)
	return b.String(), set
}

// The set depends only on what the program asks of C/C++: not on the order of
// the declarations, comments, spacing, or the names of local variables.
func TestDeterminism(t *testing.T) {
	a := detHeader + `
func first() int {
	v := std.vector[int]{1, 2, 3}
	n := v.size()
	return int(n) + int(cstring.strlen("abc"))
}

func second() {
	var buf [cstdio.BUFSIZ]byte
	s := std.string("x")
	fmt.Println(len(buf), s.size())
}

func main() {
	fmt.Println(first())
	second()
}
`
	b := detHeader + `
// main comes first here, and there are comments everywhere.
func main() {
	fmt.Println(first()) // call
	second()
}

/* second */
func second() {
	str   :=   std.string("x")   // another name
	var buf [ cstdio.BUFSIZ ]byte
	fmt.Println(len(buf), str.size())
}

func first() int {

	// a vector
	vec := std.vector[int]{1, 2, 3}
	count := vec.size()
	return int(count) + int(cstring.strlen("abc"))
}
`
	ta, sa := collectText(t, a)
	tb, sb := collectText(t, b)
	if ta != tb {
		t.Errorf("the sets differ:\n%s\n---\n%s", ta, tb)
	}
	if len(sa.Refs) == 0 || len(sa.Refs) != len(sb.Refs) {
		t.Fatalf("%d and %d references", len(sa.Refs), len(sb.Refs))
	}
	for i := range sa.Refs {
		if sa.Refs[i].ID != i || sa.Refs[i].Key != sb.Refs[i].Key {
			t.Errorf("ref %d: %q vs %q", i, sa.Refs[i].Key, sb.Refs[i].Key)
		}
	}
	// And again, the same source: the same text (no map order in it).
	for i := 0; i < 3; i++ {
		if again, _ := collectText(t, a); again != ta {
			t.Fatalf("run %d differs", i)
		}
	}
}

// Equal references are one reference with all the sites, in source order.
func TestDedup(t *testing.T) {
	_, set := collectText(t, detHeader+`
func main() {
	a := cstring.strlen("abc")
	b := cstring.strlen("abc")
	c := cstring.strlen("abd")
	fmt.Println(a, b, c)
}
`)
	var abc, abd *Ref
	for _, r := range set.Refs {
		switch r.Key {
		case `call(::strlen|lit("abc"))`:
			abc = r
		case `call(::strlen|lit("abd"))`:
			abd = r
		}
	}
	if abc == nil || abd == nil {
		t.Fatalf("references: %s", set)
	}
	if len(abc.Sites) != 2 || len(abd.Sites) != 1 || abc.Sites[0].Line >= abc.Sites[1].Line {
		t.Errorf("sites %+v %+v", abc.Sites, abd.Sites)
	}
	if got := set.Ref(abc.Key); got != abc {
		t.Error("Set.Ref does not find the key")
	}
	seen := map[string]bool{}
	for _, r := range set.Refs {
		if seen[r.Key] {
			t.Errorf("key %q twice", r.Key)
		}
		seen[r.Key] = true
	}
}

func TestKindPathsAreSortedPrefixes(t *testing.T) {
	_, set := collectText(t, detHeader+`
func main() {
	n := cstring.strlen("abc")
	v := std.vector[int]{}
	fmt.Println(n, v.size(), cstdio.EOF)
}
`)
	got := set.KindPaths()
	want := []string{"EOF", "std::vector", "strlen"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("kind paths %v, want %v", got, want)
	}
}

func TestKindPathsHavePrefixes(t *testing.T) {
	p := &Path{Ns: "::std", Parts: []string{"chrono", "steady_clock", "now"}}
	if got := strings.Join(p.Prefixes(), ","); got != "std::chrono,std::chrono::steady_clock,std::chrono::steady_clock::now" {
		t.Errorf("prefixes %s", got)
	}
	if got := (&Path{Parts: []string{"RAYWHITE", "r"}}).KindPath(); got != "RAYWHITE::r" {
		t.Errorf("kind path %s", got)
	}
	set := NewSet(&Ref{Expr: p})
	if got := strings.Join(set.KindPaths(), ","); got != "std,std::chrono,std::chrono::steady_clock,std::chrono::steady_clock::now" &&
		got != "std::chrono,std::chrono::steady_clock,std::chrono::steady_clock::now" {
		t.Errorf("kind paths %s", got)
	}
}

func TestRefOfAndSites(t *testing.T) {
	p, fset := tsupport.CheckSource(t, detHeader+`
func main() {
	v := std.vector[int]{1}
	n := v.size()
	fmt.Println(n)
}
`, nil)
	set := Collect(p, fset)
	var call *Ref
	for _, r := range set.Refs {
		if r.Kind == KindCall && strings.Contains(r.Key, "size") {
			call = r
		}
	}
	if call == nil {
		t.Fatalf("no call: %s", set)
	}
	if len(call.Sites) != 1 || call.Sites[0].Text != "v.size()" || call.Sites[0].Line != 14 || call.Sites[0].File != "main.gp" {
		t.Errorf("sites %+v", call.Sites)
	}
	if call.Sites[0].Ctx != CtxInit {
		t.Errorf("context %q", call.Sites[0].Ctx)
	}
	// A node of the AST maps back to its reference.
	found := false
	for n, r := range set.byNode {
		if r == call {
			if set.RefOf(n) != call {
				t.Error("RefOf")
			}
			found = true
		}
	}
	if !found {
		t.Error("no node maps to the call's reference")
	}
	if set.RefOf(struct{}{}) != nil {
		t.Error("RefOf of an unknown node")
	}
}

// A package without C/C++ gives an empty set, and so does a package that was not checked.
func TestEmptySets(t *testing.T) {
	_, set := collectText(t, "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(1) }\n")
	if len(set.Refs) != 0 || len(set.KindPaths()) != 0 || len(set.Probeable()) != 0 {
		t.Errorf("refs %v", set)
	}
	if s := Collect(nil, nil); s == nil || len(s.Refs) != 0 {
		t.Error("Collect(nil)")
	}
}

func TestJSON(t *testing.T) {
	_, set := collectText(t, detHeader+`
func main() { fmt.Println(cstdio.EOF) }
`)
	b, err := set.MarshalJSON()
	if err != nil || !strings.Contains(string(b), `"key":"::EOF"`) || !strings.Contains(string(b), `"ctx":"arg"`) {
		t.Errorf("%s %v", b, err)
	}
}
