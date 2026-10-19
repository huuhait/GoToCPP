package mvs

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// A tgraph is a requirement graph written as "B1: D3 E2": module B at v1.1.0
// requires D at v1.3.0 and E at v1.2.0 (the digit N is the version v1.N.0).
type tgraph map[string][]string

func mod(s string) Module {
	n, _ := strconv.Atoi(s[1:])
	return Module{Path: s[:1], Version: fmt.Sprintf("v1.%d.0", n)}
}

func (g tgraph) Required(m Module) ([]Module, error) {
	if m.Version == "" {
		m.Version = "v1.1.0"
	}
	key := fmt.Sprintf("%s%d", m.Path, vnum(m.Version))
	if e, ok := g["!"+key]; ok {
		return nil, errors.New(e[0])
	}
	var out []Module
	for _, r := range g[key] {
		out = append(out, mod(r))
	}
	return out, nil
}

func vnum(v string) int {
	var n int
	fmt.Sscanf(v, "v1.%d.0", &n)
	return n
}

func (g tgraph) Max(_, a, b string) string {
	if gocppmod.CompareVersions(a, b) < 0 {
		return b
	}
	return a
}

func (g tgraph) Previous(m Module) (Module, error) {
	best := Module{Path: m.Path, Version: None}
	for k := range g {
		if k[0] == '!' || k[:1] != m.Path {
			continue
		}
		n, _ := strconv.Atoi(k[1:])
		v := fmt.Sprintf("v1.%d.0", n)
		if gocppmod.CompareVersions(v, m.Version) < 0 && (best.Version == None || gocppmod.CompareVersions(v, best.Version) > 0) {
			best.Version = v
		}
	}
	return best, nil
}

func (g tgraph) Upgrade(m Module) (Module, error) {
	best := m
	for k := range g {
		if k[0] == '!' || k[:1] != m.Path {
			continue
		}
		n, _ := strconv.Atoi(k[1:])
		if v := fmt.Sprintf("v1.%d.0", n); gocppmod.CompareVersions(v, best.Version) > 0 {
			best.Version = v
		}
	}
	return best, nil
}

func show(l []Module) string {
	var s []string
	for _, m := range l {
		if m.Version == "" {
			s = append(s, m.Path)
		} else {
			s = append(s, fmt.Sprintf("%s%d", m.Path, vnum(m.Version)))
		}
	}
	return strings.Join(s, " ")
}

var blog = tgraph{
	"A1": {"B1", "C2"},
	"B1": {"D3"},
	"C1": {"D2"},
	"C2": {"D4"},
	"C3": {"D5"},
	"D2": {"E1"},
	"D3": {"E2"},
	"D4": {"E2", "F1"},
	"D5": {"E2"},
	"E1": nil, "E2": nil, "F1": nil,
}

var root = Module{Path: "A"}

func TestBuildList(t *testing.T) {
	l, err := BuildList([]Module{root}, blog)
	if err != nil {
		t.Fatal(err)
	}
	if got := show(l); got != "A B1 C2 D4 E2 F1" {
		t.Errorf("build list = %s", got)
	}
}

func TestBuildListIsStable(t *testing.T) {
	// Many runs: the concurrent reads must not change the answer or its order.
	for i := 0; i < 50; i++ {
		l, err := BuildList([]Module{root}, blog)
		if err != nil || show(l) != "A B1 C2 D4 E2 F1" {
			t.Fatalf("run %d: %s, %v", i, show(l), err)
		}
	}
}

func mods(names ...string) []Module {
	var out []Module
	for _, n := range names {
		if strings.HasSuffix(n, "!") { // "B!" is B at none
			out = append(out, Module{Path: n[:1], Version: None})
			continue
		}
		out = append(out, mod(n))
	}
	return out
}

var blogRoots = mods("B1", "C2", "D4", "E2", "F1")

func TestEdit(t *testing.T) {
	cases := []struct {
		name      string
		try, must []string
		want      string
	}{
		{"nothing", nil, nil, "B1 C2 D4 E2 F1"},
		{"upgrade through a requirement", nil, []string{"C3"}, "B1 C3 D5 E2 F1"}, // D5 is accepted as the root of D
		{"lower than now is a downgrade of that module", nil, []string{"C1"}, "B1 C1 D4 E2 F1"},
		{"try upgrade", []string{"D5"}, nil, "B1 C2 D5 E2 F1"},
		{"try upgrade below the current version changes nothing", []string{"D2"}, nil, "B1 C2 D4 E2 F1"},
		{"downgrade D: B has no lower version, C goes down, E and F stay roots", nil, []string{"D2"}, "C1 D2 E2 F1"},
		{"downgrade D3: C goes down", nil, []string{"D3"}, "B1 C1 D3 E2 F1"},
		{"downgrade E: whatever needs E2 goes down", nil, []string{"E1"}, "C1 D2 E1 F1"},
		{"remove B", nil, []string{"B!"}, "C2 D4 E2 F1"},
		{"remove D removes what needs it", nil, []string{"D!"}, "E2 F1"},
		{"add a module", nil, []string{"A1"}, "A1 B1 C2 D4 E2 F1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Edit(blog, root, blogRoots, mods(tc.try...), mods(tc.must...))
			if err != nil {
				t.Fatal(err)
			}
			if s := show(got); s != tc.want {
				t.Errorf("roots = %s, want %s", s, tc.want)
			}
		})
	}
}

func TestEditConflicts(t *testing.T) {
	// C3 needs D5, but D is to be D2.
	_, err := Edit(blog, root, blogRoots, nil, mods("C3", "D2"))
	var ce *ConstraintError
	if !errors.As(err, &ce) || len(ce.Conflicts) == 0 || !strings.Contains(err.Error(), "C@v1.3.0") {
		t.Fatalf("err = %v", err)
	}
	// Two versions of one module.
	_, err = Edit(blog, root, blogRoots, nil, mods("C1", "C3"))
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v", err)
	}
	// An error in a module file.
	g := tgraph{"A1": {"B1"}, "!B1": {"boom"}}
	if _, err := Edit(g, root, mods("B1"), nil, mods("B1")); err == nil || err.Error() != "boom" {
		t.Errorf("err = %v", err)
	}
}

// A downgrade can need a higher version of another module (since Go 1.17).
func TestEditDowngradeRaisesOthers(t *testing.T) {
	g := tgraph{"B1": nil, "B2": nil, "C1": {"B2"}, "C2": nil}
	got, err := Edit(g, root, mods("B1", "C2"), nil, mods("C1"))
	if err != nil {
		t.Fatal(err)
	}
	if s := show(got); s != "B2 C1" {
		t.Errorf("roots = %s, want B2 C1", s)
	}
}

func TestErrorsNameTheModule(t *testing.T) {
	g := tgraph{"A1": {"B1"}, "!B1": {"boom"}}
	_, err := BuildList([]Module{root}, g)
	var re *RequiredError
	if !errors.As(err, &re) || re.Module.Path != "B" || err.Error() != "boom" {
		t.Fatalf("err = %v", err)
	}
}

func TestSelectedAndModules(t *testing.T) {
	g, err := NewGraph(blog, []Module{root})
	if err != nil {
		t.Fatal(err)
	}
	if v := g.Selected("D"); v != "v1.4.0" {
		t.Errorf("D = %s", v)
	}
	if v := g.Selected("Z"); v != "" {
		t.Errorf("Z = %q", v)
	}
	// Breadth first: A, then B1 C2, then D3 D4, then E2 F1.
	if got := show(g.Modules()); got != "A B1 C2 D3 D4 E2 F1" {
		t.Errorf("modules = %s", got)
	}
	if got := show(g.Required(mod("D4"))); got != "E2 F1" {
		t.Errorf("required D4 = %s", got)
	}
}

func TestModuleString(t *testing.T) {
	if s := (Module{Path: "a", Version: "v1.0.0"}).String(); s != "a@v1.0.0" {
		t.Error(s)
	}
	if s := (Module{Path: "a"}).String(); s != "a" {
		t.Error(s)
	}
}
