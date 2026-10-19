package bindgen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// The layout of a struct in the view, computed with Go's rules from the types the view
// gives its fields, is the layout the compiler gives the C++ struct: sizeof and offsetof
// are asserted by the compiler (static_assert in a file that includes the header).

var basicSize = map[string]int{
	"C.char": 1, "C.schar": 1, "C.uchar": 1, "C.short": 2, "C.ushort": 2, "C.int": 4, "C.uint": 4,
	"C.long": 8, "C.ulong": 8, "C.longlong": 8, "C.ulonglong": 8, "C.float": 4, "C.double": 8,
	"C.bool": 1, "C.size_t": 8, "C.wchar_t": 4, "C.char16_t": 2, "C.char32_t": 4,
	"int8": 1, "int16": 2, "int32": 4, "int64": 8, "uint8": 1, "uint16": 2, "uint32": 4, "uint64": 8, "uintptr": 8,
	"unsafe.Pointer": 8,
}

type layouter struct {
	types map[string]ast.Expr // type name -> its type expression
	exact map[string]bool     // names whose layout is not Go's (unions, bit fields, attributes)
}

// sizeAlign returns the size and alignment of a type expression, ok false for what the
// test cannot say.
func (l *layouter) sizeAlign(e ast.Expr) (size, align int, ok bool) {
	switch t := e.(type) {
	case *ast.StarExpr, *ast.FuncType:
		return 8, 8, true
	case *ast.SelectorExpr:
		n := t.X.(*ast.Ident).Name + "." + t.Sel.Name
		s, ok := basicSize[n]
		return s, s, ok
	case *ast.Ident:
		if s, ok := basicSize[t.Name]; ok {
			return s, s, true
		}
		if x, ok := l.types[t.Name]; ok && !l.exact[t.Name] {
			return l.sizeAlign(x)
		}
		return 0, 0, false
	case *ast.ArrayType:
		n, err := strconv.Atoi(t.Len.(*ast.BasicLit).Value)
		if err != nil {
			return 0, 0, false
		}
		s, a, ok := l.sizeAlign(t.Elt)
		return s * n, a, ok
	case *ast.StructType:
		fs, size, align, ok := l.fields(t)
		_ = fs
		return size, align, ok
	}
	return 0, 0, false
}

// A fieldPos is a named field and its offset.
type fieldPos struct {
	name   string
	offset int
}

// fields lays a struct out with Go's rules.
func (l *layouter) fields(st *ast.StructType) (out []fieldPos, size, align int, ok bool) {
	align = 1
	for _, f := range st.Fields.List {
		s, a, ok := l.sizeAlign(f.Type)
		if !ok {
			return nil, 0, 0, false
		}
		names := f.Names
		if len(names) == 0 {
			return nil, 0, 0, false // an embedded base class: the test leaves it out
		}
		for _, n := range names {
			size = (size + a - 1) / a * a
			out = append(out, fieldPos{n.Name, size})
			size += s
		}
		align = max(align, a)
	}
	size = (size + align - 1) / align * align
	return out, size, align, true
}

// layoutCases are the headers whose structs are checked.
var layoutCases = []string{"c_basic.h", "anon_decl.h", "struct_macros.h", "struct_ptrs.h", "c_macros.h"}

func TestLayoutMatchesCompiler(t *testing.T) {
	cxx := tsupport.Clang(t)
	for _, name := range layoutCases {
		t.Run(name, func(t *testing.T) {
			res := generate(t, name, nil)
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "view.go", res.Source, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			l := &layouter{types: map[string]ast.Expr{}, exact: map[string]bool{}}
			cxxName := map[string]string{}
			var order []string
			for _, d := range f.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok || gd.Tok != token.TYPE {
					continue
				}
				for _, sp := range gd.Specs {
					ts := sp.(*ast.TypeSpec)
					l.types[ts.Name.Name] = ts.Type
					if st, ok := ts.Type.(*ast.StructType); ok && !ts.Assign.IsValid() {
						order = append(order, ts.Name.Name)
						doc := ""
						if gd.Doc != nil {
							doc = gd.Doc.Text()
						}
						if strings.Contains(doc, "union:") || strings.Contains(doc, "layout:") {
							l.exact[ts.Name.Name] = true
						}
						for _, fl := range st.Fields.List {
							if fl.Comment != nil && (strings.Contains(fl.Comment.Text(), "bit field") || strings.Contains(fl.Comment.Text(), "union")) {
								l.exact[ts.Name.Name] = true
							}
						}
						for _, line := range strings.Split(doc, "\n") {
							for _, kw := range []string{"struct ", "class "} {
								if rest, ok := strings.CutPrefix(line, kw); ok {
									cxxName[ts.Name.Name] = kw + rest
								}
							}
						}
					}
				}
			}
			var asserts strings.Builder
			checked := 0
			for _, n := range order {
				if l.exact[n] {
					continue
				}
				st := l.types[n].(*ast.StructType)
				if len(st.Fields.List) == 0 {
					continue // a type that is only declared
				}
				offs, size, _, ok := l.fields(st)
				if !ok {
					continue
				}
				c := n
				if x := cxxName[n]; x != "" {
					c = x
				}
				fmt.Fprintf(&asserts, "static_assert(sizeof(%s) == %d, \"sizeof %s\");\n", c, size, n)
				for _, o := range offs {
					fmt.Fprintf(&asserts, "static_assert(__builtin_offsetof(%s, %s) == %d, \"offset %s.%s\");\n", c, strings.TrimSuffix(o.name, "_"), o.offset, n, o.name)
				}
				checked++
			}
			if checked == 0 {
				t.Fatal("no struct could be checked")
			}
			// the assertions about the fields that are unnamed in C++ (anon1, ...) are not asserted
			src := "#include \"" + filepath.Join(headersDir(t), name) + "\"\n" + dropAnon(asserts.String())
			tl := &tool{cxx: cxx, dir: headersDir(t), tmp: t.TempDir()}
			p, err := tl.source("layout.cpp", src)
			if err != nil {
				t.Fatal(err)
			}
			msgs, exit, err := tl.run(t.Context(), []string{"-fsyntax-only"}, p, func(r readerT) error { _, err := readAll(r); return err })
			if err != nil {
				t.Fatal(err)
			}
			if exit != 0 {
				t.Errorf("the layout of the view is not the compiler's (%d structs checked):\n%s\n%s", checked, msgs, os.Getenv("GOCPP_TEST_SHOW"))
			}
		})
	}
}

// dropAnon removes the assertions about the members that have no name in C++.
func dropAnon(s string) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "offsetof") && strings.Contains(line, ", anon") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
