package check

import (
	"go/ast"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// TestCgoHelpers checks the typed cgo helpers (cgohelpers.go): their results have gocpp types
// (a string, a []byte), so the mistakes with them are reported; CString's and cstr's results are
// opaque; a C value passes as any argument (an opaque one, or a variable of a C pointer type);
// a C.free of a header package (cstdlib.free) is no helper (and no name can be C: the loader's
// rule); the
// errors and Conversion.String show the helpers as the user wrote them; and the gates see them
// as C's helpers.
func TestCgoHelpers(t *testing.T) {
	src := `package main

import (
	"fmt"

	cstdlib "<stdlib.h>"
)

func main() {
	var u *C.uchar
	s := C.GoString(u)
	t := C.GoStringN(cstdlib.getenv("X"), 2) + s
	b := C.GoBytes(cstdlib.getenv("X"), C.int(2))
	c := C.CString(t)
	h := C.cstr(s)
	C.free(c)
	cstdlib.free(c)
	n := int(C.strlen(h))
	fmt.Println(s, t, b, n, string(C.f()))
	_ = s + 1
	_ = C.GoBytes(c, n)
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m\n" +
		"./main.gp:20:6: invalid operation: s + 1 (mismatched types string and untyped int)\n" +
		"./main.gp:21:19: cannot use n (variable of type int) as C.int value in argument to C.GoBytes\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	p := mainPkg(t, prog, r)
	for name, typ := range map[string]string{"s": "string", "t": "string", "b": "[]byte", "c": "invalid type", "h": "invalid type", "n": "int"} {
		if got := localType(t, p, name).String(); got != typ {
			t.Errorf("%s has the type %s, want %s", name, got, typ)
		}
	}
	helpers := map[string]int{}
	ast.Inspect(p.Load.Files[0].AST, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name := p.CgoHelper(call); name != "" {
			helpers[name]++
			if (name == "CString" || name == "cstr") != p.Opaque.IsOpaque(call) {
				t.Errorf("C.%s: opaque %v", name, p.Opaque.IsOpaque(call))
			}
		}
		return true
	})
	wantHelpers := map[string]int{"GoString": 1, "GoStringN": 1, "GoBytes": 2, "CString": 1, "cstr": 1, "free": 1}
	for name, n := range wantHelpers {
		if helpers[name] != n {
			t.Errorf("%d calls of C.%s, want %d (all: %v)", helpers[name], name, n, helpers)
		}
	}
	if conv := conversionList(prog.Fset, p); conv != "18:7 int(C.strlen(h))\n19:26 string(C.f())\n" {
		t.Errorf("conversions:\n%s", conv)
	}

	// Before Phase 04, C's helpers are gated where the user wrote them.
	root = writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": "package main\n\nfunc main() {\n\t_ = C.GoString(nil)\n}\n"})
	prog = load.Load(load.Config{Dir: root, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}, ".")
	r = Check(prog, fakeProbeConfig(3))
	want = "# example.com/m\n./main.gp:4:6: C types and helpers are not supported yet (Phase 04)\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors at Phase 03:\n%s\nwant:\n%s", errs, want)
	}
}

// TestFixHelpers checks that the messages of go/types show the helpers and their C types as the
// user wrote them, but leave other names alone.
func TestFixHelpers(t *testing.T) {
	for _, tc := range []struct{ msg, want string }{
		{"cannot use b (variable of type []byte) as *cgoh.char value in argument to gp_cgoh.GoString",
			"cannot use b (variable of type []byte) as *C.char value in argument to C.GoString"},
		{"not enough arguments in call to gp_cgoh.Free\n\thave ()\n\twant (unsafe.Pointer)",
			"not enough arguments in call to C.free\n\thave ()\n\twant (unsafe.Pointer)"},
		{"want (*cgoh.char, cgoh.int)", "want (*C.char, C.int)"},
		{"gp_cgoh.Cstr(s) and xgp_cgoh.Cstr and gp_cgoh.Cstrx and mycgoh.int", "C.cstr(s) and xgp_cgoh.Cstr and gp_cgoh.Cstrx and mycgoh.int"},
		{"no cgoh here", "no cgoh here"},
	} {
		if got := fixHelpers(tc.msg, "gp_cgoh"); got != tc.want {
			t.Errorf("fixHelpers(%q) = %q, want %q", tc.msg, got, tc.want)
		}
	}
}
