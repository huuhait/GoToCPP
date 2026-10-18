package check

import (
	"go/ast"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
)

// TestUnsafeCalls checks the rewrites of step 4.10 (unsafecalls.go, slicecalls.go): the calls of
// unsafe.Slice and unsafe.String that have a C pointer or a C length, and the slice expressions
// of a C array, get their Go types (a slice of C elements, a string, a []byte), so that later
// mistakes with them are reported; the import of unsafe stays used when every use of it is
// rewritten, and nothing is rewritten where the values are typed.
func TestUnsafeCalls(t *testing.T) {
	src := `package main

import (
	"unsafe"

	cstring "<string.h>"
)

type Pack struct {
	a C.int
	bb [2]C.char
}

func main() {
	var cp *C.char
	var n C.int
	buf := make([]byte, 4)
	argv := unsafe.Slice(&cp, 3)
	first := unsafe.Slice(argv[0], 4)
	second := unsafe.String(argv[1:][0], 2)
	size := unsafe.Sizeof(n)
	pack := unsafe.Sizeof(Pack{})
	var kn int = unsafe.Sizeof(C.long(0))
	const csz = unsafe.Sizeof(n)
	cols := unsafe.Slice(&cp, 2)
	s := unsafe.String((*byte)(unsafe.Pointer(cp)), n)
	b := unsafe.Slice(&buf[0], n)
	t := unsafe.String(&buf[0], 2)
	c := unsafe.Slice((*byte)(unsafe.Pointer(cp)), 3)
	name := cstring.strdup(cp)
	arr := C.table[:]
	arr2 := C.table[1:3:4]
	fmt.Println(len(cols), len(s), len(b), len(t), len(c), name, len(arr), len(arr2), len(first), second, size, pack, kn)
	_ = s + 1
	_ = t + 1
	_ = b + 1
	_ = len(arr, 1)
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	p := mainPkg(t, prog, r)
	for name, typ := range map[string]string{
		"cols": "[]invalid type", "s": "string", "b": "[]byte", "t": "string", "c": "[]byte",
		"arr": "[]invalid type", "arr2": "[]invalid type", "argv": "[]invalid type", "first": "[]invalid type",
		"second": "string", "size": "uintptr", "pack": "uintptr",
	} {
		if got := localType(t, p, name).String(); got != typ {
			t.Errorf("%s has the type %s, want %s", name, got, typ)
		}
	}
	want := "# example.com/m\n" +
		"./main.gp:23:15: cannot use unsafe.Sizeof(C.long(0)) (value of type uintptr) as int value in variable declaration\n" +
		"./main.gp:24:14: unsafe.Sizeof(n) (value of type uintptr) is not constant: the size of a C/C++ type is known only to the C++ compiler, so it is a value, not a Go constant\n" +
		"./main.gp:34:6: invalid operation: s + 1 (mismatched types string and untyped int)\n" +
		"./main.gp:35:6: invalid operation: t + 1 (mismatched types string and untyped int)\n" +
		"./main.gp:36:10: cannot convert 1 (untyped int constant) to type []byte\n" +
		"./main.gp:37:16: invalid operation: too many arguments for len(arr, 1) (expected 1, found 2)\n"
	// ("unsafe" is used, by the calls inside the helpers and by unsafe.Pointer: no error.)
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	// The calls that were rewritten are the wrappers of the helpers and the helper calls of the
	// slice expressions; the user's calls are inside, and Package finds both.
	var wrappers, slices int
	for _, f := range p.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if p.UnsafeWrapper(c) != nil {
					wrappers++
				}
				if p.OpaqueSlice(c) != nil {
					slices++
				}
			}
			return true
		})
	}
	// The calls with an opaque pointer (argv, first, second, cols) and the sizes of C values and
	// types with C parts (size, pack, kn, csz) are wrapped; the other calls have typed pointers (a
	// C length is converted by As[int]); and there are two slice expressions of C.table.
	if wrappers != 8 || slices != 2 {
		t.Errorf("%d wrappers and %d slice calls, want 8 and 2", wrappers, slices)
	}
}
