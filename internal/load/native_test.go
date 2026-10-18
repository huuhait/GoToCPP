package load

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
	"testing/fstest"
)

// nativeOfSource parses src and returns what NativeFunc says about each function, in source
// order, and the errors and warnings of ParseDirectives.
func nativeOfSource(t *testing.T, src string) (natives []string, errs []string) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.gp", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, errs = directiveStrings(fset, f)
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			name, ok := NativeFunc(fd)
			if !ok {
				name = "-"
			}
			natives = append(natives, fd.Name.Name+"="+name)
		}
	}
	return natives, errs
}

func TestNativeFunc(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		natives []string
		errs    []string
	}{
		{
			name:    "simple",
			src:     "package p\n\n// F does it.\n//\n//gocpp:native gp::errors::assign\nfunc F(x int) bool\n",
			natives: []string{"F=gp::errors::assign"},
		},
		{
			name:    "global namespace and a trailing comment",
			src:     "package p\n\n//gocpp:native ::gp::x::y // the runtime's\nfunc F()\n",
			natives: []string{"F=::gp::x::y"},
		},
		{
			name:    "a stub has none",
			src:     "package p\n\nfunc F()\n\nfunc G() {}\n",
			natives: []string{"F=-", "G=-"},
		},
		{
			name:    "no argument",
			src:     "package p\n\n//gocpp:native\nfunc F()\n",
			natives: []string{"F=-"},
			errs:    []string{"3:1: usage: //gocpp:native C++function"},
		},
		{
			name:    "two arguments",
			src:     "package p\n\n//gocpp:native a b\nfunc F()\n",
			natives: []string{"F=-"},
			errs:    []string{"3:1: usage: //gocpp:native C++function"},
		},
		{
			name:    "not a C++ name",
			src:     "package p\n\n//gocpp:native gp::\nfunc F()\n",
			natives: []string{"F=-"},
			errs:    []string{"3:1: //gocpp:native: gp:: is not the name of a C++ function (like gp::errors::assign)"},
		},
		{
			name:    "a name with a space, quotes or a call",
			src:     "package p\n\n//gocpp:native f(x)\nfunc F()\n",
			natives: []string{"F=-"},
			errs:    []string{"3:1: //gocpp:native: f(x) is not the name of a C++ function (like gp::errors::assign)"},
		},
		{
			name:    "a body",
			src:     "package p\n\n//gocpp:native gp::f\nfunc F() {}\n",
			natives: []string{"F=-"},
			errs:    []string{"3:1: //gocpp:native is for a function without a body"},
		},
		{
			name:    "a method",
			src:     "package p\n\ntype T int\n\n//gocpp:native gp::f\nfunc (T) M()\n",
			natives: []string{"M=-"},
			errs:    []string{"5:1: //gocpp:native is for a function, not a method"},
		},
		{
			name:    "a generic function",
			src:     "package p\n\n//gocpp:native gp::f\nfunc F[T any](x T)\n",
			natives: []string{"F=-"},
			errs:    []string{"3:1: //gocpp:native is for a function that is not generic"},
		},
		{
			name:    "twice",
			src:     "package p\n\n//gocpp:native gp::f\n//gocpp:native gp::g\nfunc F()\n",
			natives: []string{"F=-"},
			errs:    []string{"4:1: more than one //gocpp:native directive on a function"},
		},
		{
			name:    "not on a function",
			src:     "package p\n\n//gocpp:native gp::f\nvar x int\n\nfunc F() {\n\t//gocpp:native gp::g\n}\n",
			natives: []string{"F=-"},
			errs: []string{
				"3:1: //gocpp:native must be in the doc comment of a function",
				"7:2: //gocpp:native must be in the doc comment of a function",
			},
		},
		{
			name:    "a lookalike name is another directive",
			src:     "package p\n\n//gocpp:nativex gp::f\nfunc F()\n",
			natives: []string{"F=-"},
			errs:    []string{"3:1: //gocpp: directives must come before the package clause"},
		},
		{
			name:    "other directives after the package clause stay errors",
			src:     "package p\n\n//gocpp:link m\n//gocpp:native gp::f\nfunc F()\n",
			natives: []string{"F=gp::f"},
			errs:    []string{"3:1: //gocpp: directives must come before the package clause"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			natives, errs := nativeOfSource(t, tt.src)
			if got, want := join(natives), join(tt.natives); got != want {
				t.Errorf("natives: got %s, want %s", got, want)
			}
			if got, want := join(errs), join(tt.errs); got != want {
				t.Errorf("errors:\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

func join(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += "\n      "
		}
		out += s
	}
	return out
}

// TestNativeOnlyInStd: a standard package may have a native function, a package of the program
// may not.
func TestNativeOnlyInStd(t *testing.T) {
	std := fstest.MapFS{
		"nat/nat.gp": {Data: []byte("package nat\n\n// F is C++.\n//\n//gocpp:native gp::nat::f\nfunc F(x int) int\n")},
	}
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp":   "package main\n\nimport \"nat\"\n\n//gocpp:native gp::f\nfunc f()\n\nfunc main() { _ = nat.F(1) }\n",
	})
	cfg := Config{Stdlib: std, GoStd: func(p string) bool { return p == "nat" }}
	prog := loadIn(t, cfg, root, ".")
	wantErrors(t, prog, root, "# example.com/m\n./main.gp:5:1: //gocpp:native is only for the standard library\n")
	var nat *Package
	for _, p := range prog.Packages {
		if p.ImportPath == "nat" {
			nat = p
		}
	}
	if nat == nil || len(nat.Files) != 1 {
		t.Fatalf("package nat: %v", nat)
	}
	for _, d := range nat.Files[0].AST.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			if name, ok := NativeFunc(fd); !ok || name != "gp::nat::f" {
				t.Errorf("NativeFunc(F) = %q, %v", name, ok)
			}
		}
	}
}

// TestPackageIsStub: a standard package is a declaration stub when one of its functions has no
// body and no //gocpp:native; a package whose bodyless functions are all native is a real one, as
// is one with bodies only. Only standard packages can be stubs.
func TestPackageIsStub(t *testing.T) {
	std := fstest.MapFS{
		"bodies/b.gp":   {Data: []byte("package bodies\n\nfunc F() int { return 1 }\n")},
		"natonly/n.gp":  {Data: []byte("package natonly\n\n//gocpp:native gp::n::f\nfunc F() int\n\n//gocpp:native gp::n::g\nfunc G(x int)\n\nfunc H() int { return 2 }\n")},
		"stubonly/s.gp": {Data: []byte("package stubonly\n\nfunc F() int\n\nfunc G() int { return 1 }\n")},
		"mixed/m.gp":    {Data: []byte("package mixed\n\n//gocpp:native gp::m::f\nfunc F() int\n\nfunc G() int\n")},
		// a native function in one file, the stub's function in another
		"twofiles/a.gp": {Data: []byte("package twofiles\n\n//gocpp:native gp::t::f\nfunc F() int\n")},
		"twofiles/b.gp": {Data: []byte("package twofiles\n\nfunc G() int\n")},
	}
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp":   "package main\n\nimport (\n\t_ \"bodies\"\n\t_ \"mixed\"\n\t_ \"natonly\"\n\t_ \"stubonly\"\n\t_ \"twofiles\"\n)\n\nfunc main() {}\n",
		// a package of the program with a function without a body is no stub (the checker's own
		// error is the user's problem)
		"user/u.gp": "package user\n\nfunc F() int\n",
	})
	cfg := Config{Stdlib: std, GoStd: func(p string) bool {
		return slices.Contains([]string{"bodies", "natonly", "stubonly", "mixed", "twofiles"}, p)
	}}
	prog := loadIn(t, cfg, root, ".", "./user")
	want := map[string]bool{"bodies": false, "natonly": false, "stubonly": true, "mixed": true, "twofiles": true, "example.com/m/user": false, "example.com/m": false}
	got := map[string]bool{}
	for _, p := range prog.Packages {
		got[p.ImportPath] = p.IsStub()
	}
	for path, stub := range want {
		if g, ok := got[path]; !ok || g != stub {
			t.Errorf("IsStub(%s) = %v (loaded %v), want %v", path, g, ok, stub)
		}
	}
}
