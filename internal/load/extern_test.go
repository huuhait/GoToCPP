package load

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
	"testing/fstest"
)

// externOfSource parses src and returns what ExternFunc says about each function, in source
// order ("F=C add3", "F=C++ ImGui::Begin", "F=-"), and the errors of ParseDirectives.
func externOfSource(t *testing.T, src string) (externs []string, errs []string) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.gp", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, errs = directiveStrings(fset, f)
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			text := "-"
			if x, ok := ExternFunc(fd); ok {
				text = "C++ " + x.Name
				if x.CLinkage {
					text = "C " + x.Name
				}
			}
			externs = append(externs, fd.Name.Name+"="+text)
		}
	}
	return externs, errs
}

func TestExternFunc(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		externs []string
		errs    []string
	}{
		{
			name:    "C++ linkage and the Go name",
			src:     "package p\n\n// F is declared.\n//\n//gocpp:extern\nfunc F(x int32) bool\n",
			externs: []string{"F=C++ F"},
		},
		{
			name:    "C linkage",
			src:     "package p\n\n//gocpp:extern C\nfunc add3(a, b, c C.int) C.int\n",
			externs: []string{"add3=C add3"},
		},
		{
			name:    "C linkage and another name",
			src:     "package p\n\n//gocpp:extern C c_add3 // the C name\nfunc add3()\n",
			externs: []string{"add3=C c_add3"},
		},
		{
			name:    "a namespace",
			src:     "package p\n\n//gocpp:extern ImGui::Begin\nfunc begin()\n",
			externs: []string{"begin=C++ ImGui::Begin"},
		},
		{
			name:    "a name from the global namespace",
			src:     "package p\n\n//gocpp:extern ::ns::f\nfunc f()\n",
			externs: []string{"f=C++ ns::f"},
		},
		{
			name:    "C++ spelled out",
			src:     "package p\n\n//gocpp:extern C++ C\nfunc f()\n",
			externs: []string{"f=C++ C"},
		},
		{
			name:    "no directive: a stub",
			src:     "package p\n\nfunc F()\n\nfunc G() {}\n",
			externs: []string{"F=-", "G=-"},
		},
		{
			name:    "a namespace under C linkage",
			src:     "package p\n\n//gocpp:extern C ns::f\nfunc f()\n",
			externs: []string{"f=-"},
			errs:    []string{"3:1: //gocpp:extern C: ns::f is not the name of a C function (a plain name: C has no namespaces)"},
		},
		{
			name:    "two names",
			src:     "package p\n\n//gocpp:extern a b\nfunc F()\n",
			externs: []string{"F=-"},
			errs:    []string{"3:1: usage: //gocpp:extern [C|C++] [name]"},
		},
		{
			name:    "not a C++ name",
			src:     "package p\n\n//gocpp:extern ns::\nfunc F()\n",
			externs: []string{"F=-"},
			errs:    []string{"3:1: //gocpp:extern: ns:: is not the name of a C++ function (like add or ImGui::Begin)"},
		},
		{
			name:    "a call as the name",
			src:     "package p\n\n//gocpp:extern f(x)\nfunc F()\n",
			externs: []string{"F=-"},
			errs:    []string{"3:1: //gocpp:extern: f(x) is not the name of a C++ function (like add or ImGui::Begin)"},
		},
		{
			name:    "a C++ keyword as the C name",
			src:     "package p\n\n//gocpp:extern C int\nfunc F()\n",
			externs: []string{"F=-"},
			errs:    []string{"3:1: //gocpp:extern: int is a C++ keyword, so no C or C++ function has that name (an extern name is spelled as written: a Go function with such a name gets a _ suffix, which a C++ function does not have)"},
		},
		{
			name:    "a C++ keyword as the C++ name",
			src:     "package p\n\n//gocpp:extern C++ new\nfunc F()\n",
			externs: []string{"F=-"},
			errs:    []string{"3:1: //gocpp:extern: new is a C++ keyword, so no C or C++ function has that name (an extern name is spelled as written: a Go function with such a name gets a _ suffix, which a C++ function does not have)"},
		},
		{
			name:    "a C++ keyword in a namespace path",
			src:     "package p\n\n//gocpp:extern ns::delete\nfunc F()\n",
			externs: []string{"F=-"},
			errs:    []string{"3:1: //gocpp:extern: delete is a C++ keyword, so no C or C++ function has that name (an extern name is spelled as written: a Go function with such a name gets a _ suffix, which a C++ function does not have)"},
		},
		{
			name:    "a Go function named like a C++ keyword and no name",
			src:     "package p\n\n//gocpp:extern\nfunc class()\n",
			externs: []string{"class=-"},
			errs:    []string{"3:1: //gocpp:extern: class is a C++ keyword, so no C or C++ function has that name (an extern name is spelled as written: a Go function with such a name gets a _ suffix, which a C++ function does not have)"},
		},
		{
			name:    "a Go function named like a C++ keyword with a C++ name",
			src:     "package p\n\n//gocpp:extern C do_new\nfunc new()\n",
			externs: []string{"new=C do_new"},
		},
		{
			name:    "a word that contains a keyword",
			src:     "package p\n\n//gocpp:extern ns::newer::int_value\nfunc F()\n",
			externs: []string{"F=C++ ns::newer::int_value"},
		},
		{
			name:    "the namespace gp",
			src:     "package p\n\n//gocpp:extern gp::thing\nfunc F()\n",
			externs: []string{"F=-"},
			errs:    []string{"3:1: //gocpp:extern: gp::thing is in the namespace gp of gocpp's own code (a function of a namespace of your own is declared there)"},
		},
		{
			name:    "the namespace std from the global namespace",
			src:     "package p\n\n//gocpp:extern ::std::move\nfunc F()\n",
			externs: []string{"F=-"},
			errs:    []string{"3:1: //gocpp:extern: std::move is in the namespace std of gocpp's own code (a function of a namespace of your own is declared there)"},
		},
		{
			name:    "a package namespace",
			src:     "package p\n\n//gocpp:extern gpkg_main::f\nfunc F()\n",
			externs: []string{"F=-"},
			errs:    []string{"3:1: //gocpp:extern: gpkg_main::f is in the namespace gpkg_main of gocpp's own code (a function of a namespace of your own is declared there)"},
		},
		{
			name:    "a plain name that starts like a namespace",
			src:     "package p\n\n//gocpp:extern C gp\nfunc F()\n",
			externs: []string{"F=C gp"},
		},
		{
			name:    "a body",
			src:     "package p\n\n//gocpp:extern\nfunc F() {}\n",
			externs: []string{"F=-"},
			errs:    []string{"3:1: //gocpp:extern is for a function without a body"},
		},
		{
			name:    "a method",
			src:     "package p\n\ntype T int\n\n//gocpp:extern\nfunc (T) M()\n",
			externs: []string{"M=-"},
			errs:    []string{"5:1: //gocpp:extern is for a function, not a method"},
		},
		{
			name:    "a generic function",
			src:     "package p\n\n//gocpp:extern\nfunc F[T any](x T)\n",
			externs: []string{"F=-"},
			errs:    []string{"3:1: //gocpp:extern is for a function that is not generic"},
		},
		{
			name:    "the name _",
			src:     "package p\n\n//gocpp:extern f\nfunc _()\n",
			externs: []string{"_=-"},
			errs:    []string{"3:1: //gocpp:extern: a function named _ can not be extern"},
		},
		{
			name:    "twice",
			src:     "package p\n\n//gocpp:extern\n//gocpp:extern C\nfunc F()\n",
			externs: []string{"F=-"},
			errs:    []string{"4:1: more than one //gocpp:extern directive on a function"},
		},
		{
			name:    "not on a function",
			src:     "package p\n\n//gocpp:extern\nvar x int\n\nfunc F() {\n\t//gocpp:extern\n}\n",
			externs: []string{"F=-"},
			errs: []string{
				"3:1: //gocpp:extern must be in the doc comment of a function",
				"7:2: //gocpp:extern must be in the doc comment of a function",
			},
		},
		{
			name:    "before the package clause",
			src:     "//gocpp:extern\npackage p\n\nfunc F()\n",
			externs: []string{"F=-"},
			errs:    []string{"1:1: //gocpp:extern must be in the doc comment of a function, after the package clause"},
		},
		{
			name:    "extern-c is the header directive",
			src:     "package p\n\n//gocpp:extern-c <x.h>\nfunc F()\n",
			externs: []string{"F=-"},
			errs:    []string{"3:1: //gocpp: directives must come before the package clause"},
		},
		{
			name:    "a lookalike name is another directive",
			src:     "package p\n\n//gocpp:externx f\nfunc F()\n",
			externs: []string{"F=-"},
			errs:    []string{"3:1: //gocpp: directives must come before the package clause"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			externs, errs := externOfSource(t, tt.src)
			if got, want := join(externs), join(tt.externs); got != want {
				t.Errorf("externs: got %s, want %s", got, want)
			}
			if got, want := join(errs), join(tt.errs); got != want {
				t.Errorf("errors:\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

func TestExternNames(t *testing.T) {
	src := "package p\n\n//gocpp:extern C\nfunc a()\n\nfunc b()\n\n//gocpp:extern\nfunc c()\n\n//gocpp:extern\nfunc d() {}\n"
	f, err := parser.ParseFile(token.NewFileSet(), "p.gp", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ExternNames(f), []string{"a", "c"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestExternNotInStd: a package of the program may have an extern function; a standard package
// uses //gocpp:native.
func TestExternNotInStd(t *testing.T) {
	std := fstest.MapFS{
		"ext/e.gp": {Data: []byte("package ext\n\n//gocpp:extern C\nfunc F(x C.int) C.int\n")},
	}
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp":   "package main\n\nimport \"ext\"\n\n//gocpp:extern C\nfunc f(x C.int) C.int\n\nfunc main() { _ = ext.F(1) }\n",
	})
	cfg := Config{Stdlib: std, GoStd: func(p string) bool { return p == "ext" }}
	prog := loadIn(t, cfg, root, ".")
	var ext *Package
	for _, p := range prog.Packages {
		if p.ImportPath == "ext" {
			ext = p
		}
	}
	if ext == nil || len(ext.Errors.Diagnostics()) != 1 {
		t.Fatalf("package ext: %v", ext)
	}
	if got, want := ext.Errors.Diagnostics()[0].Msg, "//gocpp:extern is not for the standard library (it uses //gocpp:native)"; got != want {
		t.Errorf("error: got %q, want %q", got, want)
	}
	for _, p := range prog.Packages {
		if p.ImportPath != "ext" && len(p.Errors.Diagnostics()) != 0 {
			t.Errorf("package %s: unexpected errors %v", p.ImportPath, p.Errors.Diagnostics())
		}
	}
}
