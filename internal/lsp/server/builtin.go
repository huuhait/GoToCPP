package server

import (
	_ "embed"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"sync"
)

// The docs of the predeclared names (len, append, error, true, ...), for
// hover and signature help, as gopls shows them: from Go's
// src/builtin/builtin.go, copied as builtin.go.txt (Go 1.25; see
// COPIED.md), which documents them as Go declarations. gocpp has Go's
// predeclared names.

//go:embed builtin.go.txt
var builtinSrc []byte

// builtinFile is builtin.go.txt, parsed once.
var builtinFile = sync.OnceValues(func() (*token.FileSet, *ast.File) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "builtin.go", builtinSrc, parser.ParseComments)
	if err != nil {
		panic("server: builtin.go.txt: " + err.Error())
	}
	return fset, f
})

// builtinTypes are the placeholder types of builtin.go and what gopls
// shows for them.
var builtinTypes = strings.NewReplacer("ComplexType", "complex128", "FloatType", "float64", "IntegerType", "int")

// builtinDecl returns the declaration of the predeclared name in
// builtin.go, without its doc comment, and the doc comment; or nil.
func builtinDecl(name string) (ast.Decl, *ast.CommentGroup) {
	_, f := builtinFile()
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == name {
				d2 := *d
				d2.Doc = nil
				return &d2, d.Doc
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				var names []*ast.Ident
				switch s := spec.(type) {
				case *ast.TypeSpec:
					names = []*ast.Ident{s.Name}
				case *ast.ValueSpec:
					names = s.Names
				}
				for _, id := range names {
					if id.Name == name {
						d2 := *d
						d2.Doc = nil
						return &d2, d.Doc
					}
				}
			}
		}
	}
	return nil, nil
}

// builtinHover returns the hover of the predeclared object obj (gopls's
// hoverBuiltin): its declaration in builtin.go, with the placeholder types
// replaced, and its doc comment.
func builtinHover(obj types.Object) *hoverResult {
	if obj.Name() == "Error" {
		if _, ok := obj.(*types.Func); ok {
			return &hoverResult{signature: obj.String()} // the method of error
		}
	}
	decl, doc := builtinDecl(obj.Name())
	if decl == nil {
		return &hoverResult{signature: types.ObjectString(obj, nil)}
	}
	fset, _ := builtinFile()
	return &hoverResult{signature: builtinTypes.Replace(formatNode(fset, decl)), doc: doc.Text()}
}

// builtinSignature returns the signature of the predeclared function name
// as signature help shows it (gopls's NewBuiltinSignature), or nil.
func builtinSignature(name string) *signatureInfo {
	decl, doc := builtinDecl(name)
	fd, ok := decl.(*ast.FuncDecl)
	if !ok {
		return nil
	}
	fset, _ := builtinFile()
	s := &signatureInfo{name: name, doc: doc.Text()}
	field := func(f *ast.Field) []string {
		typ := builtinTypes.Replace(formatNode(fset, f.Type))
		if len(f.Names) == 0 {
			return []string{typ}
		}
		var list []string
		for _, id := range f.Names {
			list = append(list, id.Name+" "+typ)
		}
		return list
	}
	for _, f := range fd.Type.Params.List {
		s.params = append(s.params, field(f)...)
		_, s.variadic = f.Type.(*ast.Ellipsis)
	}
	if fd.Type.Results != nil {
		for _, f := range fd.Type.Results.List {
			s.results = append(s.results, field(f)...)
			s.resultParens = s.resultParens || len(f.Names) > 0
		}
	}
	s.resultParens = s.resultParens || len(s.results) > 1
	return s
}
