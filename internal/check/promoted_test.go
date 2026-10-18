package check

import (
	"fmt"
	"go/ast"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
)

// TestPromotedPath checks which selectors of a struct that embeds C/C++ types are members of the
// one such type (Package.PromotedPath: the path of the embedded fields, as Selection.Index has
// them), and that the others stay what go/types says. Each selector x.name that is marked in the
// source with a comment "// path: 0.1" has that path, one marked "// none" has none (and its
// error stays, or there is none).
func TestPromotedPath(t *testing.T) {
	const head = "package main\n\nimport \"<string>\"\nimport \"<vector>\"\n\n"
	for _, c := range []struct {
		name string
		src  string
		errs string // the errors of the check
	}{
		{"direct", `
type V struct {
	n int
	std.vector[int]
}

func main() {
	var v V
	v.push_back(1) // path: 1
	v.n++          // none
	v.vector.size() // none
}
`, ""},
		{"pointer to the struct", `
type V struct{ std.vector[int] }

func main() {
	p := &V{}
	p.push_back(1) // path: 0
}
`, ""},
		{"embedded pointer", `
type V struct {
	k int
	*std.vector[int]
}

func main() {
	var v V
	v.push_back(1) // path: 1
}
`, ""},
		{"through embedded structs", `
type Inner struct {
	a, b int
	std.string
}

type Mid struct {
	z int
	*Inner
}

type Top struct {
	Mid
	n int
}

func main() {
	var t Top
	t.append("x") // path: 0.1.2
	t.a++         // none
	t.Mid.size()  // path: 1.2
	t.Inner.size() // path: 2
}
`, ""},
		{"named and alias C types", `
type A = std.vector[int]
type B std.vector[int]

type V struct{ A }
type W struct{ *B }

func main() {
	var v V
	var w W
	v.push_back(1) // path: 0
	w.push_back(1) // path: 0
}
`, ""},
		{"recursive embedding", `
type Node struct {
	*Node
	std.vector[int]
}

type R struct{ *R }

func main() {
	var n Node
	n.push_back(1) // path: 1
	var r R
	_ = r.size // none
}
`, "r.size undefined"},
		{"two C types: none", `
type V struct {
	std.vector[int]
	std.string
}

type W struct{ V } // also two, through V

func main() {
	var v V
	var w W
	v.push_back(1) // none
	w.size()       // none
	v.vector.push_back(1)
}
`, "v.push_back undefined (type V has no field or method push_back)"},
		{"a Go member wins", `
type Inner struct{ size int }

func (Inner) Get() int { return 1 }

type V struct {
	std.vector[int]
	Inner
}

func main() {
	var v V
	v.size = 1  // none
	_ = v.Get() // none
	v.push_back(1) // path: 0
}
`, ""},
		{"ambiguous Go members: none", `
type A struct{ size int }
type B struct{ size int }

type V struct {
	std.vector[int]
	A
	B
}

func main() {
	var v V
	_ = v.size // none
}
`, "ambiguous selector v.size"},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := head + c.src
			root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
			prog, r := loadAndCheck(t, load.Config{}, root, ".")
			p := mainPkg(t, prog, r)
			errs := errorText(t, prog, r, root, true)
			if c.errs == "" && errs != "" || !strings.Contains(errs, c.errs) {
				t.Errorf("errors:\n%s\nwant %q", errs, c.errs)
			}
			lines := strings.Split(src, "\n")
			n := 0
			for _, f := range p.Load.Files {
				inner := make(map[ast.Expr]bool) // the X of another selector
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
		})
	}
}

// TestPromotedOtherPackage checks a struct of another package that embeds one C++ type: its
// members are promoted, but the unexported embedded field itself is not reachable by its name:
// go/types' error stays for it, and is not taken for a C++ member of the same name.
func TestPromotedOtherPackage(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"vec/vec.gp": "package vec\n\nimport \"<vector>\"\n\ntype V struct {\n\tstd.vector[int]\n}\n",
		"main.gp":    "package main\n\nimport \"example.com/m/vec\"\n\nfunc main() {\n\tvar v vec.V\n\tv.push_back(1)\n\tv.vector.size()\n}\n",
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	errs := errorText(t, prog, r, root, true)
	want := "# example.com/m\n./main.gp:8:4: v.vector undefined (cannot refer to unexported field vector)\n"
	if errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	p := mainPkg(t, prog, r)
	n := 0
	for _, f := range p.Load.Files {
		ast.Inspect(f.AST, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "push_back" && sel.Sel.Name != "vector" {
				return true
			}
			n++
			path := p.PromotedPath(sel)
			if (sel.Sel.Name == "push_back") != (path != nil) {
				t.Errorf("%s: PromotedPath is %v", sel.Sel.Name, path)
			}
			return true
		})
	}
	if n != 2 {
		t.Errorf("found %d selectors, want 2", n)
	}
}
