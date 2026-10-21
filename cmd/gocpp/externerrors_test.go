package main

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
)

// The tests of the parts of the C++ errors that know the functions of //gocpp:extern (ADR-128).
// The end to end reports are in tests/interop (extern_link_c, extern_link_cpp).

// externTestFile parses src as the file dir/name of a package and returns the loader's view of
// it: the AST and the names of its extern functions.
func externTestFile(t *testing.T, fset *token.FileSet, dir, name, src string) *load.File {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return &load.File{Path: path, AST: f, Externs: load.ExternNames(f)}
}

// TestPkgUsesCppExtern: a package that declares an extern function uses C or C++, with no header
// import and no import "C": a link error of it gets the C++ error reports.
func TestPkgUsesCppExtern(t *testing.T) {
	fset := token.NewFileSet()
	dir := t.TempDir()
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{"an extern function", "package main\n\n//gocpp:extern C\nfunc f(x int32) int32\n\nfunc main() {}\n", true},
		{"a function without the directive", "package main\n\nfunc f(x int32) int32\n\nfunc main() {}\n", false},
		{"only Go", "package main\n\nfunc main() {}\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := externTestFile(t, fset, dir, "main.gp", tt.src)
			pkg := &check.Package{Load: &load.Package{Files: []*load.File{file}}}
			if got := pkgUsesCpp(pkg); got != tt.want {
				t.Errorf("pkgUsesCpp = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExternDecls: the extern functions of a package and of its imports, once each, with the
// place of the declaration.
func TestExternDecls(t *testing.T) {
	fset := token.NewFileSet()
	dir := t.TempDir()
	lib := &load.Package{Files: []*load.File{externTestFile(t, fset, dir, "lib.gp",
		"package lib\n\n//gocpp:extern ns::Deep\nfunc Deep(x int32) int32\n\nfunc Plain() {}\n")}}
	main := &load.Package{
		Files: []*load.File{externTestFile(t, fset, dir, "main.gp",
			"package main\n\n//gocpp:extern C c_add\nfunc add(a, b int32) int32\n\nfunc main() {}\n")},
		Imports: map[string]*load.Package{"example.com/m/lib": lib, "example.com/m/alias": lib},
	}
	got := externDecls(fset, main, make(map[*load.Package]bool), nil)
	var names []string
	for _, x := range got {
		names = append(names, x.goName+"="+x.decl.Name+":"+filepath.Base(x.pos.Filename)+":"+strconv.Itoa(x.pos.Line))
	}
	want := "add=c_add:main.gp:4 Deep=ns::Deep:lib.gp:4"
	if strings.Join(names, " ") != want {
		t.Errorf("externDecls = %q, want %q", strings.Join(names, " "), want)
	}
	if !got[0].decl.CLinkage || got[1].decl.CLinkage {
		t.Errorf("linkage: add %v, Deep %v; want C and C++", got[0].decl.CLinkage, got[1].decl.CLinkage)
	}
}

// TestMatchExtern: the linker prints the name of a C function plainly and the name of a C++
// function with its parameters (GNU ld demangles it).
func TestMatchExtern(t *testing.T) {
	decls := []externDeclInfo{
		{goName: "add", decl: load.ExternDecl{CLinkage: true, Name: "c_add"}},
		{goName: "begin", decl: load.ExternDecl{Name: "ImGui::Begin"}},
		{goName: "sum", decl: load.ExternDecl{Name: "sum"}},
	}
	tests := []struct {
		sym  string
		want string // the Go name, "" for no match
	}{
		{"c_add", "add"},
		{"ImGui::Begin(char const*, bool*, int)", "begin"},
		{"sum(int, int)", "sum"},
		{"sum", "sum"},
		{"summary(int)", ""},
		{"sum2", ""},
		{"c_add_more", ""},
		{"Begin(char const*)", ""},
		{"ns::sum(int)", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got, ok := matchExtern(decls, tt.sym)
		if (tt.want != "") != ok || got.goName != tt.want {
			t.Errorf("matchExtern(%q) = %q, %v; want %q", tt.sym, got.goName, ok, tt.want)
		}
	}
}

// TestExternLinkHints: the hint names the .gp line of the declaration, and says what to try for
// the linkage that the directive states; all is true when every undefined symbol is an extern
// function.
func TestExternLinkHints(t *testing.T) {
	fset := token.NewFileSet()
	dir := t.TempDir()
	file := externTestFile(t, fset, dir, "main.gp",
		"package main\n\n//gocpp:extern C\nfunc c_fn(x int32) int32\n\n//gocpp:extern ns::cxx_fn\nfunc cxxFn(x int32) int32\n")
	pkg := &check.Package{Load: &load.Package{Dir: dir, Files: []*load.File{file}}}
	d := &driver{check: &checkCommand{dir: dir, fset: fset}}
	refs := []undefinedRef{{sym: "c_fn"}, {sym: "ns::cxx_fn(int)"}, {sym: "c_fn"}}
	hints, all := d.externLinkHints(pkg, refs)
	if !all || len(hints) != 2 {
		t.Fatalf("hints = %q, all = %v; want two hints and all", hints, all)
	}
	for i, want := range []string{
		"(c_fn is declared with //gocpp:extern C at ./main.gp:4, and no file of the program defines the C function c_fn",
		"(cxxFn is declared with //gocpp:extern at ./main.gp:7 as the C++ function ns::cxx_fn(int): nothing in the program defines that function with these parameter types",
	} {
		if !strings.Contains(hints[i], want) {
			t.Errorf("hint %d = %q, want it to contain %q", i, hints[i], want)
		}
	}
	if _, all := d.externLinkHints(pkg, append(refs, undefinedRef{sym: "other"})); all {
		t.Errorf("all = true with an undefined symbol that is not an extern function")
	}
	if hints, all := d.externLinkHints(&check.Package{Load: &load.Package{Dir: dir}}, refs); hints != nil || all {
		t.Errorf("a package without extern functions: hints %q, all %v", hints, all)
	}
}

// TestIsExternCall: a call of an extern function of the package or of an import, by name or
// qualified; a call of another function, a conversion and a value are not.
func TestIsExternCall(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main

//gocpp:extern C
func add(a, b int32) int32

func plain(a int32) int32 { return a }

func main() {
	_ = add(1, 2)
	_ = plain(1)
	_ = int64(3)
	f := add
	_ = f
}
`
	file, err := parser.ParseFile(fset, "main.gp", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Uses:  make(map[*ast.Ident]types.Object),
		Defs:  make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{Importer: importer.Default(), Error: func(error) {}} // the missing body of add
	_, _ = conf.Check("example.com/m", fset, []*ast.File{file}, info)
	pkg := &check.Package{Info: info}
	externs := map[string]bool{"example.com/m.add": true}
	var calls, values []string
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if isExternCall(pkg, externs, n) {
				calls = append(calls, types.ExprString(n))
			}
		case *ast.AssignStmt:
			if id, ok := n.Lhs[0].(*ast.Ident); ok && id.Name == "f" && isExternCall(pkg, externs, n.Rhs[0]) {
				values = append(values, types.ExprString(n.Rhs[0]))
			}
		}
		return true
	})
	if strings.Join(calls, ";") != "add(1, 2)" {
		t.Errorf("extern calls = %q, want only add(1, 2)", calls)
	}
	if len(values) != 0 {
		t.Errorf("a value is not a call: %q", values)
	}
	if isExternCall(pkg, nil, file.Decls[2].(*ast.FuncDecl).Body.List[0].(*ast.AssignStmt).Rhs[0]) {
		t.Errorf("no extern functions: nothing is an extern call")
	}
}
