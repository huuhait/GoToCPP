// The tests of gocpp's patch 3 (../FORK.md §4): a type name may have more
// than one dot, because C++ types live in nested namespaces and have member
// types, as in std.chrono.seconds and std.vector[int].iterator. This file is
// gocpp's own, not upstream's: ../patches/0003-dotted-type-names.patch adds
// it. It imports the stock go/parser and go/scanner, to compare the fork
// with them.

package parser

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	goscanner "go/scanner"
	"go/token"
	"go/types"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// A dtTest is a source and the dump (dtDump) of one of its nodes. ctx says
// where src goes and which node is dumped:
//   - "type": src is a type, in "var _ src"; the type is dumped;
//   - "expr": src is an expression, in "var _ = src"; it is dumped;
//   - "stmt": src is a statement, in "func _() { src }"; it is dumped;
//   - "decl": src is a declaration; its spec is dumped, or for a func the
//     FuncDecl.
//
// stock says what the stock go/parser does with the same source: "error"
// (a syntax error), "same" (the same AST: there the type is an expression,
// and expressions already have more dots) or "other" (another AST without
// an error: a selector on a slice, array, map, chan or func type, see
// TestDottedTypeChangedGo).
type dtTest struct{ ctx, src, want, stock string }

// dtGocppTests are gocpp inputs, in every place where Go has a type.
var dtGocppTests = []dtTest{
	// Variables and constants.
	{"decl", `var d std.chrono.seconds`, `ValueSpec{Names: [d], Type: std.chrono.seconds}`, "error"},
	{"decl", `var d, e std.chrono.seconds = x, y`, `ValueSpec{Names: [d, e], Type: std.chrono.seconds, Values: [x, y]}`, "error"},
	{"decl", `const c std.chrono.seconds = 1`, `ValueSpec{Names: [c], Type: std.chrono.seconds, Values: [1]}`, "error"},
	{"type", `std.filesystem.path`, `std.filesystem.path`, "error"},
	{"type", `a.b.c.d.e`, `a.b.c.d.e`, "error"},
	{"type", `C.std.chrono.seconds`, `C.std.chrono.seconds`, "error"},

	// Parameters, results, receivers and type parameters.
	{"decl", `func f(p std.filesystem.path) std.chrono.seconds`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Names: [p], Type: std.filesystem.path}]}, Results: FieldList{List: [Field{Type: std.chrono.seconds}]}}}`, "error"},
	{"decl", `func f(p, q std.filesystem.path)`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Names: [p, q], Type: std.filesystem.path}]}}}`, "error"},
	{"decl", `func f(std.filesystem.path, std.chrono.seconds)`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Type: std.filesystem.path}, Field{Type: std.chrono.seconds}]}}}`, "error"},
	{"decl", `func f(p ...std.filesystem.path)`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Names: [p], Type: Ellipsis{Elt: std.filesystem.path}}]}}}`, "error"},
	{"decl", `func f(...std.filesystem.path)`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Type: Ellipsis{Elt: std.filesystem.path}}]}}}`, "error"},
	{"decl", `func f() (std.chrono.seconds, error)`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Type: std.chrono.seconds}, Field{Type: error}]}}}`, "error"},
	{"decl", `func f() (d std.chrono.seconds)`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Names: [d], Type: std.chrono.seconds}]}}}`, "error"},
	{"decl", `func (d std.chrono.seconds) m()`, `FuncDecl{Recv: FieldList{List: [Field{Names: [d], Type: std.chrono.seconds}]}, Name: m, Type: FuncType{Params: FieldList{}}}`, "error"},
	{"type", `func(std.chrono.seconds) std.chrono.seconds`, `FuncType{Params: FieldList{List: [Field{Type: std.chrono.seconds}]}, Results: FieldList{List: [Field{Type: std.chrono.seconds}]}}`, "error"},
	{"decl", `func f[T std.chrono.seconds]()`, `FuncDecl{Name: f, Type: FuncType{TypeParams: FieldList{List: [Field{Names: [T], Type: std.chrono.seconds}]}, Params: FieldList{}}}`, "error"},
	{"decl", `type G[T std.chrono.seconds] struct{}`, `TypeSpec{Name: G, TypeParams: FieldList{List: [Field{Names: [T], Type: std.chrono.seconds}]}, Type: StructType{Fields: FieldList{}}}`, "error"},

	// Slices, arrays, maps, pointers and channels.
	{"type", `[]std.chrono.seconds`, `ArrayType{Elt: std.chrono.seconds}`, "error"},
	{"type", `[4]std.chrono.seconds`, `ArrayType{Len: 4, Elt: std.chrono.seconds}`, "error"},
	{"type", `map[std.string]std.chrono.seconds`, `MapType{Key: std.string, Value: std.chrono.seconds}`, "error"},
	{"type", `map[std.chrono.seconds]int`, `MapType{Key: std.chrono.seconds, Value: int}`, "error"},
	{"type", `*std.chrono.seconds`, `StarExpr{X: std.chrono.seconds}`, "error"},
	{"type", `chan std.chrono.seconds`, `ChanType{Dir: SEND|RECV, Value: std.chrono.seconds}`, "error"},
	{"type", `<-chan std.chrono.seconds`, `ChanType{Dir: RECV, Value: std.chrono.seconds}`, "error"},
	{"type", `chan<- std.chrono.seconds`, `ChanType{Dir: SEND, Value: std.chrono.seconds}`, "error"},

	// Struct fields, embedded fields and tags.
	{"decl", `type S struct{ d std.chrono.seconds }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Names: [d], Type: std.chrono.seconds}]}}}`, "error"},
	{"decl", `type S struct{ a, b std.chrono.seconds "tag" }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Names: [a, b], Type: std.chrono.seconds, Tag: "tag"}]}}}`, "error"},
	{"decl", `type S struct{ a []std.chrono.seconds }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Names: [a], Type: ArrayType{Elt: std.chrono.seconds}}]}}}`, "error"},
	{"decl", `type S struct{ a [4]std.chrono.seconds }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Names: [a], Type: ArrayType{Len: 4, Elt: std.chrono.seconds}}]}}}`, "error"},
	{"decl", `type S struct{ std.chrono.seconds }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Type: std.chrono.seconds}]}}}`, "error"},
	{"decl", `type S struct{ std.chrono.seconds "tag" }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Type: std.chrono.seconds, Tag: "tag"}]}}}`, "error"},
	{"decl", `type S struct{ *std.chrono.seconds "tag" }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Type: StarExpr{X: std.chrono.seconds}, Tag: "tag"}]}}}`, "error"},

	// Interfaces: embedded types, unions and methods.
	{"decl", `type I interface{ std.chrono.seconds }`, `TypeSpec{Name: I, Type: InterfaceType{Methods: FieldList{List: [Field{Type: std.chrono.seconds}]}}}`, "error"},
	{"decl", `type I interface{ ~std.chrono.seconds | std.chrono.minutes }`, `TypeSpec{Name: I, Type: InterfaceType{Methods: FieldList{List: [Field{Type: BinaryExpr{X: UnaryExpr{Op: ~, X: std.chrono.seconds}, Op: |, Y: std.chrono.minutes}}]}}}`, "error"},
	{"decl", `type I interface{ m(std.chrono.seconds) std.chrono.seconds }`, `TypeSpec{Name: I, Type: InterfaceType{Methods: FieldList{List: [Field{Names: [m], Type: FuncType{Params: FieldList{List: [Field{Type: std.chrono.seconds}]}, Results: FieldList{List: [Field{Type: std.chrono.seconds}]}}}]}}}`, "error"},

	// Member types after a type instance, in the same places. V is an
	// unqualified generic, as a generic alias of a C++ template would be;
	// as an embedded field or an unnamed parameter, V[int] goes through
	// parseArrayFieldOrTypeInstance, and in an interface through
	// parseMethodSpec. A Go keyword after the '.', on the same line, is a
	// name (patch 1).
	{"type", `std.vector[int].iterator`, `IndexExpr{X: std.vector, Index: int}.iterator`, "error"},
	{"type", `std.map[std.string, int].value_type`, `IndexListExpr{X: std.map, Indices: [std.string, int]}.value_type`, "error"},
	{"type", `std.vector[int].iterator.value_type`, `IndexExpr{X: std.vector, Index: int}.iterator.value_type`, "error"},
	{"type", `std.chrono.duration[int, std.milli].rep`, `IndexListExpr{X: std.chrono.duration, Indices: [int, std.milli]}.rep`, "error"},
	{"type", `std.enable_if[B, T].type`, `IndexListExpr{X: std.enable_if, Indices: [B, T]}.type`, "error"},
	{"type", `std.tuple_element[0, T].type`, `IndexListExpr{X: std.tuple_element, Indices: [0, T]}.type`, "error"},
	{"type", `V[int].iterator`, `IndexExpr{X: V, Index: int}.iterator`, "error"},
	{"type", `*std.vector[int].iterator`, `StarExpr{X: IndexExpr{X: std.vector, Index: int}.iterator}`, "error"},
	{"type", `[]std.vector[int].iterator`, `ArrayType{Elt: IndexExpr{X: std.vector, Index: int}.iterator}`, "error"},
	{"decl", `type S struct{ it std.vector[int].iterator }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Names: [it], Type: IndexExpr{X: std.vector, Index: int}.iterator}]}}}`, "error"},
	{"decl", `type S struct{ std.vector[int].iterator }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Type: IndexExpr{X: std.vector, Index: int}.iterator}]}}}`, "error"},
	{"decl", `type S struct{ V[int].iterator }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Type: IndexExpr{X: V, Index: int}.iterator}]}}}`, "error"},
	{"decl", `type S struct{ V[int, 5].iterator "tag" }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Type: IndexListExpr{X: V, Indices: [int, 5]}.iterator, Tag: "tag"}]}}}`, "error"},
	{"decl", `type S struct{ *V[int].iterator }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Type: StarExpr{X: IndexExpr{X: V, Index: int}.iterator}}]}}}`, "error"},
	{"decl", `type I interface{ std.vector[int].iterator }`, `TypeSpec{Name: I, Type: InterfaceType{Methods: FieldList{List: [Field{Type: IndexExpr{X: std.vector, Index: int}.iterator}]}}}`, "error"},
	{"decl", `type I interface{ V[int].iterator }`, `TypeSpec{Name: I, Type: InterfaceType{Methods: FieldList{List: [Field{Type: IndexExpr{X: V, Index: int}.iterator}]}}}`, "error"},
	{"decl", `type I interface{ V[int, 5].iterator }`, `TypeSpec{Name: I, Type: InterfaceType{Methods: FieldList{List: [Field{Type: IndexListExpr{X: V, Indices: [int, 5]}.iterator}]}}}`, "error"},
	{"decl", `func f(it std.vector[int].iterator)`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Names: [it], Type: IndexExpr{X: std.vector, Index: int}.iterator}]}}}`, "error"},
	{"decl", `func f(std.vector[int].iterator)`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Type: IndexExpr{X: std.vector, Index: int}.iterator}]}}}`, "error"},
	{"decl", `func f(V[int].iterator, V[int].const_iterator)`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Type: IndexExpr{X: V, Index: int}.iterator}, Field{Type: IndexExpr{X: V, Index: int}.const_iterator}]}}}`, "error"},
	{"decl", `type D[T any] = std.vector[T].iterator`, `TypeSpec{Name: D, TypeParams: FieldList{List: [Field{Names: [T], Type: any}]}, Type: IndexExpr{X: std.vector, Index: T}.iterator}`, "error"},

	// Type declarations.
	{"decl", `type D std.chrono.seconds`, `TypeSpec{Name: D, Type: std.chrono.seconds}`, "error"},
	{"decl", `type D = std.chrono.seconds`, `TypeSpec{Name: D, Type: std.chrono.seconds}`, "error"},
	{"decl", `type A [4]std.chrono.seconds`, `TypeSpec{Name: A, Type: ArrayType{Len: 4, Elt: std.chrono.seconds}}`, "error"},
	{"decl", `type A [N]std.chrono.seconds`, `TypeSpec{Name: A, Type: ArrayType{Len: N, Elt: std.chrono.seconds}}`, "error"},

	// Type assertions and type switches. A case list is a list of
	// expressions.
	{"expr", `x.(std.chrono.seconds)`, `TypeAssertExpr{X: x, Type: std.chrono.seconds}`, "error"},
	{"expr", `x.(*std.vector[int].iterator)`, `TypeAssertExpr{X: x, Type: StarExpr{X: IndexExpr{X: std.vector, Index: int}.iterator}}`, "error"},
	{"stmt", `switch x.(type) { case std.chrono.seconds, *std.chrono.seconds: }`, `TypeSwitchStmt{Assign: ExprStmt{X: TypeAssertExpr{X: x}}, Body: BlockStmt{List: [CaseClause{List: [std.chrono.seconds, StarExpr{X: std.chrono.seconds}]}]}}`, "same"},
	{"stmt", `switch x.(type) { case []std.chrono.seconds: }`, `TypeSwitchStmt{Assign: ExprStmt{X: TypeAssertExpr{X: x}}, Body: BlockStmt{List: [CaseClause{List: [ArrayType{Elt: std.chrono.seconds}]}]}}`, "other"},

	// Type arguments. A type argument that starts with a name is an
	// expression since patch 2.
	{"type", `G[std.chrono.seconds]`, `IndexExpr{X: G, Index: std.chrono.seconds}`, "error"},
	{"type", `G[int, std.chrono.seconds]`, `IndexListExpr{X: G, Indices: [int, std.chrono.seconds]}`, "error"},
	{"type", `G[[]std.chrono.seconds]`, `IndexExpr{X: G, Index: ArrayType{Elt: std.chrono.seconds}}`, "error"},
	{"type", `G[map[std.string]std.chrono.seconds]`, `IndexExpr{X: G, Index: MapType{Key: std.string, Value: std.chrono.seconds}}`, "error"},
	{"expr", `G[std.chrono.seconds]{}`, `CompositeLit{Type: IndexExpr{X: G, Index: std.chrono.seconds}}`, "same"},
	{"expr", `f[std.chrono.seconds](x)`, `CallExpr{Fun: IndexExpr{X: f, Index: std.chrono.seconds}, Args: [x]}`, "same"},
	{"expr", `f[[]std.chrono.seconds](x)`, `CallExpr{Fun: IndexExpr{X: f, Index: ArrayType{Elt: std.chrono.seconds}}, Args: [x]}`, "other"},

	// Conversions and composite literals. Their type is an expression, but
	// the element type of a slice, array, map, chan or func type is a type.
	{"expr", `std.chrono.seconds(5)`, `CallExpr{Fun: std.chrono.seconds, Args: [5]}`, "same"},
	{"expr", `(*std.chrono.seconds)(p)`, `CallExpr{Fun: ParenExpr{X: StarExpr{X: std.chrono.seconds}}, Args: [p]}`, "same"},
	{"expr", `std.chrono.seconds{}`, `CompositeLit{Type: std.chrono.seconds}`, "same"},
	{"expr", `std.vector[int].iterator{}`, `CompositeLit{Type: IndexExpr{X: std.vector, Index: int}.iterator}`, "same"},
	{"expr", `[]std.chrono.seconds(nil)`, `CallExpr{Fun: ArrayType{Elt: std.chrono.seconds}, Args: [nil]}`, "other"},
	{"expr", `[]std.chrono.seconds{a, b}`, `CompositeLit{Type: ArrayType{Elt: std.chrono.seconds}, Elts: [a, b]}`, "other"},
	{"expr", `[...]std.chrono.seconds{a}`, `CompositeLit{Type: ArrayType{Len: Ellipsis{}, Elt: std.chrono.seconds}, Elts: [a]}`, "other"},
	{"expr", `map[std.string]std.chrono.seconds{}`, `CompositeLit{Type: MapType{Key: std.string, Value: std.chrono.seconds}}`, "other"},
	{"expr", `chan std.chrono.seconds(nil)`, `CallExpr{Fun: ChanType{Dir: SEND|RECV, Value: std.chrono.seconds}, Args: [nil]}`, "other"},
	{"expr", `func() std.chrono.seconds(nil)`, `CallExpr{Fun: FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Type: std.chrono.seconds}]}}, Args: [nil]}`, "other"},
	{"expr", `[]std.vector[int].iterator{}`, `CompositeLit{Type: ArrayType{Elt: IndexExpr{X: std.vector, Index: int}.iterator}}`, "other"},
	{"expr", `func(p std.filesystem.path) {}`, `FuncLit{Type: FuncType{Params: FieldList{List: [Field{Names: [p], Type: std.filesystem.path}]}}, Body: BlockStmt{}}`, "error"},

	// Space, comments and a new line after a '.', as in Go's selectors.
	{"type", `std . chrono . /* c */ seconds`, `std.chrono.seconds`, "error"},
	{"type", "std.chrono.\n\tseconds", `std.chrono.seconds`, "error"},
	{"type", "std.vector[int].\n\titerator", `IndexExpr{X: std.vector, Index: int}.iterator`, "error"},
}

// dtGoTests are Go inputs with qualified type names, method expressions and
// selectors near types. TestDottedTypeGoUnchanged parses each one with the
// fork and with the stock go/parser and requires the same AST; want is the
// dump of it, to show its shape.
var dtGoTests = []dtTest{
	{"type", `pkg.T`, `pkg.T`, "same"},
	{"type", `pkg.T[int]`, `IndexExpr{X: pkg.T, Index: int}`, "same"},
	{"type", `pkg.T[int, string]`, `IndexListExpr{X: pkg.T, Indices: [int, string]}`, "same"},
	{"type", `*pkg.T`, `StarExpr{X: pkg.T}`, "same"},
	{"type", `[]pkg.T`, `ArrayType{Elt: pkg.T}`, "same"},
	{"type", `[pkg.N]pkg.T`, `ArrayType{Len: pkg.N, Elt: pkg.T}`, "same"},
	{"type", `map[pkg.K]pkg.V`, `MapType{Key: pkg.K, Value: pkg.V}`, "same"},
	{"type", `<-chan pkg.T`, `ChanType{Dir: RECV, Value: pkg.T}`, "same"},
	{"type", `func(pkg.T, ...pkg.U) (pkg.V, error)`, `FuncType{Params: FieldList{List: [Field{Type: pkg.T}, Field{Type: Ellipsis{Elt: pkg.U}}]}, Results: FieldList{List: [Field{Type: pkg.V}, Field{Type: error}]}}`, "same"},
	{"type", `struct{ pkg.T; *pkg.U; pkg.G[int]; f pkg.T "tag"; g, h pkg.T }`, `StructType{Fields: FieldList{List: [Field{Type: pkg.T}, Field{Type: StarExpr{X: pkg.U}}, Field{Type: IndexExpr{X: pkg.G, Index: int}}, Field{Names: [f], Type: pkg.T, Tag: "tag"}, Field{Names: [g, h], Type: pkg.T}]}}`, "same"},
	{"type", `struct{ pkg.T "tag" }`, `StructType{Fields: FieldList{List: [Field{Type: pkg.T, Tag: "tag"}]}}`, "same"},
	{"type", `struct{ T[int]; a [pkg.N]pkg.T }`, `StructType{Fields: FieldList{List: [Field{Type: IndexExpr{X: T, Index: int}}, Field{Names: [a], Type: ArrayType{Len: pkg.N, Elt: pkg.T}}]}}`, "same"},
	{"type", `interface{ pkg.I; pkg.J[int]; J[int]; m(pkg.T) pkg.U; ~pkg.T | pkg.U }`, `InterfaceType{Methods: FieldList{List: [Field{Type: pkg.I}, Field{Type: IndexExpr{X: pkg.J, Index: int}}, Field{Type: IndexExpr{X: J, Index: int}}, Field{Names: [m], Type: FuncType{Params: FieldList{List: [Field{Type: pkg.T}]}, Results: FieldList{List: [Field{Type: pkg.U}]}}}, Field{Type: BinaryExpr{X: UnaryExpr{Op: ~, X: pkg.T}, Op: |, Y: pkg.U}}]}}`, "same"},
	{"decl", `var x pkg.T = pkg.T{}`, `ValueSpec{Names: [x], Type: pkg.T, Values: [CompositeLit{Type: pkg.T}]}`, "same"},
	{"decl", `const c pkg.T = pkg.C`, `ValueSpec{Names: [c], Type: pkg.T, Values: [pkg.C]}`, "same"},
	{"decl", `type A = pkg.T`, `TypeSpec{Name: A, Type: pkg.T}`, "same"},
	{"decl", `type A pkg.T[int]`, `TypeSpec{Name: A, Type: IndexExpr{X: pkg.T, Index: int}}`, "same"},
	{"decl", `type A [pkg.N]int`, `TypeSpec{Name: A, Type: ArrayType{Len: pkg.N, Elt: int}}`, "same"},
	{"decl", `type A[P pkg.C] struct{}`, `TypeSpec{Name: A, TypeParams: FieldList{List: [Field{Names: [P], Type: pkg.C}]}, Type: StructType{Fields: FieldList{}}}`, "same"},
	{"decl", `type A[P *pkg.C,] struct{}`, `TypeSpec{Name: A, TypeParams: FieldList{List: [Field{Names: [P], Type: StarExpr{X: pkg.C}}]}, Type: StructType{Fields: FieldList{}}}`, "same"},
	{"decl", `func f(pkg.T, []pkg.U)`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Type: pkg.T}, Field{Type: ArrayType{Elt: pkg.U}}]}}}`, "same"},
	{"decl", `func (r *pkg.T) m(p pkg.T) pkg.U`, `FuncDecl{Recv: FieldList{List: [Field{Names: [r], Type: StarExpr{X: pkg.T}}]}, Name: m, Type: FuncType{Params: FieldList{List: [Field{Names: [p], Type: pkg.T}]}, Results: FieldList{List: [Field{Type: pkg.U}]}}}`, "same"},
	{"expr", `x.(pkg.T)`, `TypeAssertExpr{X: x, Type: pkg.T}`, "same"},
	{"expr", `x.(*pkg.T[int])`, `TypeAssertExpr{X: x, Type: StarExpr{X: IndexExpr{X: pkg.T, Index: int}}}`, "same"},
	{"expr", `x.(pkg.T).m`, `TypeAssertExpr{X: x, Type: pkg.T}.m`, "same"},
	{"expr", `x.(pkg.T).m.n()`, `CallExpr{Fun: TypeAssertExpr{X: x, Type: pkg.T}.m.n}`, "same"},
	// Method expressions and other selectors in expressions.
	{"expr", `pkg.T.m`, `pkg.T.m`, "same"},
	{"expr", `[]T.m`, `ArrayType{Elt: T.m}`, "same"},
	{"expr", `pkg.T.m(x, y)`, `CallExpr{Fun: pkg.T.m, Args: [x, y]}`, "same"},
	{"expr", `(*pkg.T).m`, `ParenExpr{X: StarExpr{X: pkg.T}}.m`, "same"},
	{"expr", `pkg.T[int].m`, `IndexExpr{X: pkg.T, Index: int}.m`, "same"},
	{"expr", `pkg.T{}.f`, `CompositeLit{Type: pkg.T}.f`, "same"},
	{"expr", `pkg.T(x).m`, `CallExpr{Fun: pkg.T, Args: [x]}.m`, "same"},
	{"expr", `struct{ pkg.T }.m`, `StructType{Fields: FieldList{List: [Field{Type: pkg.T}]}}.m`, "same"},
	{"expr", `interface{ pkg.I }.m`, `InterfaceType{Methods: FieldList{List: [Field{Type: pkg.I}]}}.m`, "same"},
	{"expr", `[]pkg.T{}`, `CompositeLit{Type: ArrayType{Elt: pkg.T}}`, "same"},
	{"expr", `map[pkg.K]pkg.V{}`, `CompositeLit{Type: MapType{Key: pkg.K, Value: pkg.V}}`, "same"},
	{"expr", `[]pkg.T(nil)`, `CallExpr{Fun: ArrayType{Elt: pkg.T}, Args: [nil]}`, "same"},
	{"expr", `func() pkg.T { return nil }`, `FuncLit{Type: FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Type: pkg.T}]}}, Body: BlockStmt{List: [ReturnStmt{Results: [nil]}]}}`, "same"},
	{"stmt", `switch x.(type) { case pkg.T, *pkg.U, []pkg.V: }`, `TypeSwitchStmt{Assign: ExprStmt{X: TypeAssertExpr{X: x}}, Body: BlockStmt{List: [CaseClause{List: [pkg.T, StarExpr{X: pkg.U}, ArrayType{Elt: pkg.V}]}]}}`, "same"},
}

// TestDottedTypes checks that gocpp's dotted type names parse in every
// place where Go has a type, and what the stock go/parser does with the
// same source (dtTest.stock).
func TestDottedTypes(t *testing.T) {
	for _, test := range dtGocppTests {
		src := dtSource(t, test)
		f, err := ParseFile(token.NewFileSet(), "", src, DeclarationErrors|AllErrors)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if got := dtDump(dtNode(test, f)); got != test.want {
			t.Errorf("%s:\ngot  %s\nwant %s", src, got, test.want)
		}
		g, err := goparser.ParseFile(token.NewFileSet(), "", src, goparser.DeclarationErrors|goparser.AllErrors)
		switch same := reflect.DeepEqual(f, g); {
		case test.stock == "error" && err == nil:
			t.Errorf("%s: go/parser parses it without an error, but the test says it reports one", src)
		case test.stock == "same" && (err != nil || !same):
			t.Errorf("%s: go/parser should give the same AST, but it gives %s (errors: %v)", src, dtDump(dtNode(test, g)), err)
		case test.stock == "other" && (err != nil || same):
			t.Errorf("%s: go/parser should give another AST without an error, but it gives %s (errors: %v)", src, dtDump(dtNode(test, g)), err)
		case test.stock != "error" && test.stock != "same" && test.stock != "other":
			t.Errorf("%s: bad stock %q", src, test.stock)
		}
	}
}

// TestDottedTypeGoUnchanged checks that Go code with qualified type names
// and selectors parses as with the stock go/parser: reflect.DeepEqual on
// the two files, so the positions and the legacy objects must be the same
// too.
func TestDottedTypeGoUnchanged(t *testing.T) {
	for _, test := range dtGoTests {
		src := dtSource(t, test)
		f, err := ParseFile(token.NewFileSet(), "", src, DeclarationErrors|AllErrors)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		g, err := goparser.ParseFile(token.NewFileSet(), "", src, goparser.DeclarationErrors|goparser.AllErrors)
		if err != nil {
			t.Errorf("go/parser: %s: %v", src, err)
			continue
		}
		if !reflect.DeepEqual(f, g) {
			t.Errorf("%s: the fork and go/parser give different ASTs (in positions or objects, if the dumps are the same):\nfork      %s\ngo/parser %s", src, dtDump(f), dtDump(g))
		}
		if got := dtDump(dtNode(test, f)); got != test.want {
			t.Errorf("%s:\ngot  %s\nwant %s", src, got, test.want)
		}
	}
}

// TestDottedTypeChangedGo checks the one kind of Go code that the stock
// go/parser accepts and the fork parses to another AST: in an expression, a
// selector right after a slice, array, map, chan or func type whose last
// part is a qualified type name or a type instance, as in []pkg.T.M and
// []G[int].M. Upstream takes .M as a method of the type []pkg.T, but such a
// type has no methods; the fork takes it as a part of the element type. So
// the code is never valid Go: go/types rejects both ASTs. ([]T.M stays as it
// is: upstream already takes T.M as a type name, with its one dot. A struct
// or interface type ends with "}", and a pointer type in an expression is a
// unary *, so struct{ pkg.T }.M, interface{ pkg.I }.M and (*pkg.T).M stay as
// they are too; TestDottedTypeGoUnchanged has them.)
func TestDottedTypeChangedGo(t *testing.T) {
	const pkgSrc = "package pkg\n\ntype T struct{}\n\nfunc (T) M() {}\n\ntype G[P any] struct{}\n\nfunc (G[P]) M() {}\n"
	fset := token.NewFileSet()
	pf, err := goparser.ParseFile(fset, "pkg.go", pkgSrc, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := new(types.Config).Check("pkg", fset, []*ast.File{pf}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// stockErr is the first error of go/types on the stock AST: the type has
	// no method M, or, where a type is needed, a selector on a type is not a
	// type.
	for _, test := range []struct{ expr, fork, stock, stockErr string }{
		{`[]pkg.T.M`, `ArrayType{Elt: pkg.T.M}`, `ArrayType{Elt: pkg.T}.M`, `[]pkg.T.M undefined (type []pkg.T has no field or method M)`},
		{`[2]pkg.T.M`, `ArrayType{Len: 2, Elt: pkg.T.M}`, `ArrayType{Len: 2, Elt: pkg.T}.M`, `[2]pkg.T.M undefined (type [2]pkg.T has no field or method M)`},
		{`map[int]pkg.T.M`, `MapType{Key: int, Value: pkg.T.M}`, `MapType{Key: int, Value: pkg.T}.M`, `map[int]pkg.T.M undefined (type map[int]pkg.T has no field or method M)`},
		{`chan pkg.T.M`, `ChanType{Dir: SEND|RECV, Value: pkg.T.M}`, `ChanType{Dir: SEND|RECV, Value: pkg.T}.M`, `chan pkg.T.M undefined (type chan pkg.T has no field or method M)`},
		{`func() pkg.T.M`, `FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Type: pkg.T.M}]}}`, `FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Type: pkg.T}]}}.M`, `func() pkg.T.M undefined (type func() pkg.T has no field or method M)`},
		{`[]*pkg.T.M`, `ArrayType{Elt: StarExpr{X: pkg.T.M}}`, `ArrayType{Elt: StarExpr{X: pkg.T}}.M`, `[]*pkg.T.M undefined (type []*pkg.T has no field or method M)`},
		{`[]pkg.G[int].M`, `ArrayType{Elt: IndexExpr{X: pkg.G, Index: int}.M}`, `ArrayType{Elt: IndexExpr{X: pkg.G, Index: int}}.M`, `[]pkg.G[int].M undefined (type []pkg.G[int] has no field or method M)`},
		{`[]G[int].M`, `ArrayType{Elt: IndexExpr{X: G, Index: int}.M}`, `ArrayType{Elt: IndexExpr{X: G, Index: int}}.M`, `[]G[int].M undefined (type []G[int] has no field or method M)`},
		{`[]pkg.T.M{}`, `CompositeLit{Type: ArrayType{Elt: pkg.T.M}}`, `CompositeLit{Type: ArrayType{Elt: pkg.T}.M}`, `[]pkg.T.M is not a type`},
		// The fork sees a func literal, upstream a composite literal.
		{`func() pkg.T.M { nil }`, `FuncLit{Type: FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Type: pkg.T.M}]}}, Body: BlockStmt{List: [ExprStmt{X: nil}]}}`, `CompositeLit{Type: FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Type: pkg.T}]}}.M, Elts: [nil]}`, `func() pkg.T.M is not a type`},
	} {
		src := "package p\n\nimport \"pkg\"\n\ntype G[P any] struct{}\n\nfunc (G[P]) M() {}\n\nvar _ pkg.T\n\nvar _ = " + test.expr + "\n"
		for _, side := range []struct {
			name  string
			parse func() (*ast.File, error)
			want  string
		}{
			{"fork", func() (*ast.File, error) { return ParseFile(fset, "p.go", src, 0) }, test.fork},
			{"go/parser", func() (*ast.File, error) { return goparser.ParseFile(fset, "p.go", src, 0) }, test.stock},
		} {
			f, err := side.parse()
			if err != nil {
				t.Errorf("%s: %s: %v", side.name, src, err)
				continue
			}
			if got := dtDump(f.Decls[len(f.Decls)-1].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0]); got != side.want {
				t.Errorf("%s: %s:\ngot  %s\nwant %s", side.name, test.expr, got, side.want)
			}
			var errs []string
			conf := types.Config{
				Importer: dtImporter{"pkg": pkg},
				Error:    func(err error) { errs = append(errs, err.Error()) },
			}
			conf.Check("p", fset, []*ast.File{f}, nil)
			if len(errs) == 0 {
				t.Errorf("%s: go/types accepts %s, but it must not be valid Go", side.name, test.expr)
			} else if _, msg, _ := strings.Cut(errs[0], ": "); side.name == "go/parser" && msg != test.stockErr {
				t.Errorf("%s: go/types reports %q for %s, want %q", side.name, msg, test.expr, test.stockErr)
			}
		}
	}
}

// A dtImporter imports the packages of the map.
type dtImporter map[string]*types.Package

func (m dtImporter) Import(path string) (*types.Package, error) {
	if pkg := m[path]; pkg != nil {
		return pkg, nil
	}
	return nil, fmt.Errorf("no package %q", path)
}

// dtRecoveryTests are broken inputs with a type name that stops at a '.':
// the token after the '.' isn't a name, or the part before the '.' is
// broken.
// The fork must report the same errors as the stock go/parser and recover
// the same way (TestDottedTypeRecovery), because it leaves such a '.' to
// the caller, as upstream does. The first ones have only one '.', where
// the fork runs upstream's code; the others have a second '.' or one after
// a type instance, where upstream already stops. They are what an editor
// sees while a type name is typed: the next line starts with a keyword or
// a "}", or the type ends at a ")", "]", "," or "=".
var dtRecoveryTests = []string{
	// One '.': unfinished at the end of a line, before the next declaration
	// and at the end of the file, and a second '.' instead of a name.
	"package p\nvar d std.\nvar y = 1\n",
	"package p\nvar d std.\nfunc f() {}\n",
	"package p\nvar d std.",
	"package p\nvar d std..x\n",
	"package p\nfunc f() {\n\tvar d std.\n}\n",

	// A second '.' that a "}" follows, on the next line or on the same one.
	"package p\ntype S struct {\n\td std.chrono.\n}\n",
	"package p\nfunc f() {\n\tvar d std.chrono.\n}\n",
	"package p; func f() { var d std.chrono. }",
	"package p\ntype S struct {\n\tstd.chrono.\n}\n",
	"package p\ntype I interface {\n\tstd.chrono.\n}\n",

	// A keyword on the next line stays a keyword (patch 1's same-line rule),
	// also after a comment.
	"package p\nvar d std.chrono.\nvar y = 1\n",
	"package p\nvar d std.chrono.\nfunc f() {}\n",
	"package p\nfunc f() {\n\tvar d std.chrono.\n\treturn\n}\n",
	"package p\nvar d std.chrono. // c\nvar y = 1\n",
	"package p\nvar d std.chrono. /* c\n */ type T int\n",

	// Other tokens after the second '.'.
	"package p\nfunc f(d std.chrono.) {}\n",
	"package p\nfunc f(a std.chrono., b int) {}\n",
	"package p\nvar _ map[std.chrono.]int\n",
	"package p\nvar d std.chrono. = 1\n",
	"package p\nvar _ = x.(std.chrono.)\n",
	"package p\nvar _ = []std.chrono.{}\n",
	"package p; var d std.chrono.; var y = 1",
	"package p\nvar d std.chrono.",
	"package p\nvar d std.chrono..x\n",
	"package p\nvar d std.chrono.(x)\n",
	"package p\nvar d std.chrono.*x\n",

	// A '.' after a type instance, in all the places that parse one.
	"package p\nvar d std.vector[int].\nvar y = 1\n",
	"package p\ntype S struct {\n\td std.vector[int].\n}\n",
	"package p\ntype S struct {\n\tstd.vector[int].\n}\n",
	"package p\ntype S struct {\n\tV[int].\n}\n",
	"package p\ntype I interface {\n\tstd.vector[int].\n}\n",
	"package p\ntype I interface {\n\tV[int].\n}\n",
	"package p\nfunc f(V[int].) {}\n",
	"package p\nfunc f(std.vector[int].) {}\n",
	"package p\nvar _ std.vector[int]..x\n",
	"package p\nvar _ std.vector[].x\n",

	// The part before the '.' is broken, and a name follows the '.': the
	// type name doesn't go on, as upstream. The first is what is left of
	// v.(io.Writer).Write(b) when Writer is deleted; in the others, the "]"
	// of the instance is missing. In parseArrayFieldOrTypeInstance (an
	// embedded field or an unnamed parameter V[...]) a wrong token there is
	// taken as the "]", so parseDottedName is called only after a real "]".
	// parseTypeInstance (std.vector[...], pkg.T[...]) and the instance of
	// parseMethodSpec (V[...] in an interface) need no such check: their
	// loops over the arguments end only at "]" or at the end of the file.
	// These cases fail if a new upstream changes those loops.
	"package p\nfunc f() {\n\tw := v.(io.).Write(b)\n\t_ = w\n}\n",
	"package p\nvar d std.).x\n",
	"package p\ntype S struct {\n\tV[int).x\n}\n",
	"package p\nfunc f(V[int).x) {}\n",
	"package p\nvar _ std.vector[int).x\n",
	"package p\nvar _ std.vector[int, string).x\n",
	"package p\nvar _ std.vector[int}.x\n",
	"package p\nvar _ pkg.T[int).x\n",
	"package p\nfunc f(x std.vector[int).x) {}\n",
	"package p\ntype S struct {\n\tstd.vector[int).x\n}\n",
	"package p\ntype I interface {\n\tV[int).x\n}\n",
	"package p\ntype I interface {\n\tV[int, string).x\n}\n",
	"package p\ntype I interface {\n\tstd.vector[int).x\n}\n",

	// The scanner reports an error in the token after the '.'. The fork
	// scans that token twice (nameAfterDot), but reports the error once.
	"package p\nvar d std.chrono.#x\n",
	"package p\nvar d std.chrono./* x",
	"package p\nvar d std.chrono.'\n",
	// A //line directive after the '.': the positions don't change.
	"package p\nvar d std.chrono.\n//line other.go:10:1\nvar y = 1 +\n",
}

// TestDottedTypeRecovery checks that each of dtRecoveryTests gets the same
// errors and the same AST from the fork as from the stock go/parser, in
// each of dtModes.
func TestDottedTypeRecovery(t *testing.T) {
	for _, src := range dtRecoveryTests {
		for _, mode := range dtModes {
			if msg := dtSameAsStock(src, mode); msg != "" {
				t.Errorf("%q, mode %v: %s", src, mode, msg)
			}
		}
	}
}

// dtModes are the modes in which the recovery tests compare the fork with
// the stock go/parser. With 0, a parse keeps only the first error of a
// line, and it stops after more than 10 errors.
var dtModes = []Mode{0, DeclarationErrors, AllErrors | DeclarationErrors | ParseComments}

// dtSameAsStock parses src with the fork and with the stock go/parser in
// mode, and returns "" if both report errors, the same ones, and give the
// same AST. Otherwise it says how they differ. The two packages have the
// same Mode values.
func dtSameAsStock(src string, mode Mode) string {
	f, err := ParseFile(token.NewFileSet(), "", src, mode)
	got := dtErrors(err)
	g, err := goparser.ParseFile(token.NewFileSet(), "", src, goparser.Mode(mode))
	want := dtErrors(err)
	switch {
	case len(want) == 0:
		return "go/parser reports no error, but a recovery test needs a broken input"
	case !slices.Equal(got, want):
		return fmt.Sprintf("the fork and go/parser report different errors:\nfork      %q\ngo/parser %q", got, want)
	case !reflect.DeepEqual(f, g):
		return fmt.Sprintf("the fork and go/parser give different ASTs (in positions or objects, if the dumps are the same):\nfork      %s\ngo/parser %s", dtDump(f), dtDump(g))
	}
	return ""
}

// dtLikeGoTests are inputs where a name follows the second '.' of a type
// name: std.chrono.X. There the fork goes on as upstream does after the
// first '.' of a type name: also with space, a comment or a new line before
// the name, and, in broken code, with the errors and the recovery of
// upstream. TestDottedTypeLikeGo checks this against the stock go/parser on
// the same source with std_chrono in place of std.chrono, which has the
// same length. So the sources have no keyword after a '.' (the stock
// scanner has no patch 1) and no other '.' after a type name.
var dtLikeGoTests = []string{
	"package p\nvar d std.chrono.seconds\n",
	"package p\nvar d std.chrono . /* c */ seconds\n",
	"package p\nvar d std.chrono.\n\tseconds\n",
	"package p\nvar d std.chrono. // c\n\tseconds\n",
	"package p\nvar d std.chrono.\n//line other.go:10:1\nseconds = 1 +\n",
	"package p\nvar d, e std.chrono.seconds = std.chrono.seconds(1), std.chrono.seconds{}\n",
	"package p\nvar _ map[std.chrono.seconds][]*std.chrono.seconds\n",
	"package p\nvar _ std.chrono.duration[int, R]\n",
	"package p\nfunc f(std.chrono.seconds, ...std.chrono.seconds) (std.chrono.seconds, error)\n",
	"package p\nfunc f(d std.chrono.seconds) { _ = x.(std.chrono.seconds) }\n",
	"package p\ntype S struct {\n\tstd.chrono.seconds\n\t*std.chrono.minutes\n\td std.chrono.hours \"tag\"\n}\n",
	"package p\ntype I interface {\n\tstd.chrono.seconds\n\tm() std.chrono.seconds\n}\n",
	// Broken code: the name on the next line is taken, and the error comes
	// after it.
	"package p\nvar d std.chrono.\nx = 1\n",
	"package p\ntype S struct {\n\td std.chrono.\n\tx int\n}\n",
	"package p\nfunc f() {\n\tvar d std.chrono.\n\tx := 1\n}\n",
	"package p\nfunc f(p std.chrono.\n\tq int) {}\n",
}

// TestDottedTypeLikeGo checks that each of dtLikeGoTests parses like the
// stock go/parser parses it with std_chrono in place of std.chrono: the
// same errors, and the same AST, positions and objects included, when the
// fork's std.chrono becomes the name std_chrono (dtMerge).
func TestDottedTypeLikeGo(t *testing.T) {
	for _, src := range dtLikeGoTests {
		f, err := ParseFile(token.NewFileSet(), "", src, DeclarationErrors|AllErrors|ParseComments)
		got := dtErrors(err)
		goSrc := strings.ReplaceAll(src, "std.chrono", "std_chrono")
		g, err := goparser.ParseFile(token.NewFileSet(), "", goSrc, goparser.DeclarationErrors|goparser.AllErrors|goparser.ParseComments)
		want := dtErrors(err)
		if n := dtMerge(f); n != strings.Count(src, "std.chrono") {
			t.Errorf("%q: merged %d of the %d std.chrono", src, n, strings.Count(src, "std.chrono"))
		}
		switch {
		case !slices.Equal(got, want):
			t.Errorf("%q: the fork and go/parser (on %q) report different errors:\nfork      %q\ngo/parser %q", src, goSrc, got, want)
		case !reflect.DeepEqual(f, g):
			t.Errorf("%q: the fork's AST is not go/parser's (on %q) (in positions or objects, if the dumps are the same):\nfork      %s\ngo/parser %s", src, goSrc, dtDump(f), dtDump(g))
		}
	}
}

// dtMerge changes each selector std.chrono.X in f to std_chrono.X, and
// returns the number of them. The name std takes the new name, so it keeps
// its position and its place in f.Unresolved, and chrono goes away.
func dtMerge(f *ast.File) int {
	n := 0
	ast.Inspect(f, func(node ast.Node) bool {
		if sel, ok := node.(*ast.SelectorExpr); ok {
			if inner, ok := sel.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "chrono" {
				if std, ok := inner.X.(*ast.Ident); ok && std.Name == "std" {
					std.Name = "std_chrono"
					sel.X = std
					n++
				}
			}
		}
		return true
	})
	return n
}

// TestDottedTypeKeywords checks every Go keyword after the second '.' of a
// type name and after a type instance. On the same line, patch 1 makes it
// a name, so the type name goes on: std.enable_if[B, T].type. On the next
// line, it stays a keyword, so the type name stops at the '.', with the
// errors and the recovery of the stock go/parser.
func TestDottedTypeKeywords(t *testing.T) {
	n := 0
	for tok := token.ILLEGAL; tok <= token.TILDE+16; tok++ {
		if !tok.IsKeyword() {
			continue
		}
		n++
		kw := tok.String()
		src := "package p\nvar _ std.chrono." + kw + "\nvar _ std.vector[int]." + kw + "\n"
		f, err := ParseFile(token.NewFileSet(), "", src, AllErrors)
		if err != nil {
			t.Errorf("%q: %v", src, err)
		} else {
			for i, want := range []string{"std.chrono." + kw, "IndexExpr{X: std.vector, Index: int}." + kw} {
				if got := dtDump(f.Decls[i].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Type); got != want {
					t.Errorf("%q: type %d is %s, want %s", src, i+1, got, want)
				}
			}
		}
		for _, src := range []string{
			"package p\nvar _ std.chrono.\n" + kw + "\n",
			"package p\nvar _ std.vector[int].\n" + kw + "\n",
		} {
			for _, mode := range dtModes {
				if msg := dtSameAsStock(src, mode); msg != "" {
					t.Errorf("%q, mode %v: %s", src, mode, msg)
				}
			}
		}
	}
	if n != 25 {
		t.Errorf("found %d keywords, want 25", n)
	}
}

// TestDottedTypeErrors checks the errors at a type name that stops at a
// '.'. They are upstream's (TestDottedTypeRecovery): at a second '.' that
// no name follows, the message is the one that upstream reports for any
// second '.'. A type instance after a member type, as in
// std.allocator_traits[A].rebind_alloc[T], is not part of the patch, so its
// "[" is an error too. checkErrors keeps the first error of each line, and
// fails on errors in other lines.
func TestDottedTypeErrors(t *testing.T) {
	for _, src := range []string{
		"package p\nvar d std.chrono. /* ERROR \"expected ';', found '.'\" */\nvar y = 1\n",
		"package p\nvar d std.vector[int]. /* ERROR \"expected ';', found '.'\" */\nfunc f() {}\n",
		"package p\nfunc f(d std.chrono. /* ERROR \"missing ',' in parameter list\" */ ) {}\n",
		"package p\nvar _ = x.(std.chrono. /* ERROR \"expected '\\)', found '.'\" */ )\n",
		"package p\nvar _ map[std.chrono. /* ERROR \"expected '\\]', found '.'\" */ ]int\n",
		"package p\nvar d std.chrono. /* ERROR \"expected ';', found '.'\" */ .x\n",
		"package p\nvar d std.. /* ERROR \"expected 'IDENT', found '.'\" */ x\n",
		"package p\nvar _ std.allocator_traits[A].rebind_alloc[ /* ERROR \"expected ';', found '\\['\" */ T]\n",
	} {
		checkErrors(t, src, src, DeclarationErrors|AllErrors, true)
	}
}

// TestDottedTypeGoTypes checks that go/types accepts dotted type names in
// every place when they are rooted in C, as the front-end's rewrite makes
// them (std.chrono.seconds becomes C.std.chrono.seconds; phase 01 README
// §2.5): with FakeImportC, a C.x is an opaque thing that go/types doesn't
// check. The source is written in that form here. A dotted type name that
// is not rooted in C is still an error.
func TestDottedTypeGoTypes(t *testing.T) {
	const src = `package p

import "C"

type G[T any] struct{}

type H[T, U any] struct{}

var d C.std.chrono.seconds

func f(p C.std.filesystem.path, q ...C.std.filesystem.path) C.std.chrono.seconds { return d }

func g(C.std.filesystem.path) (C.std.chrono.seconds, error) { return d, nil }

var (
	_ []C.std.chrono.seconds
	_ [4]C.std.chrono.seconds
	_ map[C.std.string]C.std.chrono.seconds
	_ *C.std.chrono.seconds
	_ <-chan C.std.chrono.seconds
	_ func(C.std.chrono.seconds) C.std.chrono.seconds
	_ C.std.vector[int].iterator
	_ C.std.map[C.std.string, int].value_type
	_ C.std.tuple_element[0, C.std.tuple[int]].type
	_ G[C.std.chrono.seconds]
	_ H[int, C.std.chrono.seconds]
	_ G[[]C.std.chrono.seconds]
	_ struct {
		d  C.std.chrono.seconds
		it C.std.vector[int].iterator "tag"
	}
)

type S struct {
	C.std.chrono.seconds
	*C.std.filesystem.path
}

type I interface {
	C.std.chrono.seconds
	m(C.std.chrono.seconds) C.std.chrono.seconds
}

type D = C.std.chrono.seconds

type E C.std.chrono.seconds

type V[T any] = C.std.vector[T].iterator

func h(x any) {
	_ = x.(C.std.chrono.seconds)
	_ = x.(*C.std.vector[int].iterator)
	switch x.(type) {
	case C.std.chrono.seconds, []C.std.chrono.seconds:
	}
	_ = C.std.chrono.seconds(5)
	_ = []C.std.chrono.seconds(nil)
	_ = (*C.std.chrono.seconds)(nil)
	_ = map[C.std.string]C.std.chrono.seconds{}
	_ = [...]C.std.chrono.seconds{}
	_ = G[C.std.chrono.seconds]{}
	var it C.std.vector[int].iterator
	_ = it
}

type T struct{}

var _ T.x.y

var _ G[int].x
`
	fset := token.NewFileSet()
	f, err := ParseFile(fset, "p.go", src, AllErrors)
	if err != nil {
		t.Fatalf("syntax error: %v", err)
	}
	var got []string
	conf := types.Config{FakeImportC: true, Error: func(err error) { got = append(got, err.Error()) }}
	conf.Check("p", fset, []*ast.File{f}, nil)
	want := []string{
		"p.go:68:9: T.x undefined (type T has no field or method x)",
		"p.go:70:14: G[int].x is not a type",
	}
	if !slices.Equal(got, want) {
		t.Errorf("go/types reports\n\t%s\nwant\n\t%s", strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
	}
}

// dtSource returns the source of test, as a file (see dtTest).
func dtSource(t *testing.T, test dtTest) string {
	t.Helper()
	switch test.ctx {
	case "type":
		return "package p; var _ " + test.src
	case "expr":
		return "package p; var _ = " + test.src
	case "stmt":
		return "package p; func _() { " + test.src + " }"
	case "decl":
		return "package p; " + test.src
	}
	t.Fatalf("%s: bad ctx %q", test.src, test.ctx)
	return ""
}

// dtNode returns the node of f that test dumps (see dtTest).
func dtNode(test dtTest, f *ast.File) ast.Node {
	if len(f.Decls) == 0 {
		return nil
	}
	switch d := f.Decls[0].(type) {
	case *ast.GenDecl:
		spec := d.Specs[0]
		if vs, ok := spec.(*ast.ValueSpec); ok && test.ctx == "type" {
			return vs.Type
		} else if ok && test.ctx == "expr" && len(vs.Values) > 0 {
			return vs.Values[0]
		}
		return spec
	case *ast.FuncDecl:
		if test.ctx == "stmt" && d.Body != nil && len(d.Body.List) > 0 {
			return d.Body.List[0]
		}
		return d
	}
	return nil
}

// dtErrors returns the errors of a parse, one string each. err is from the
// fork or from the stock go/parser.
func dtErrors(err error) []string {
	var list []string
	switch err := err.(type) {
	case nil:
	case scanner.ErrorList:
		for _, e := range err {
			list = append(list, e.Error())
		}
	case goscanner.ErrorList:
		for _, e := range err {
			list = append(list, e.Error())
		}
	default:
		list = append(list, err.Error())
	}
	return list
}

// dtDump returns the syntax tree x on one line, to compare its shape. A
// node is its type name with its fields, as in IndexExpr{X: a, Index: i};
// an Ident is its name, a BasicLit its literal and a SelectorExpr X.Sel, so
// a dotted type name looks as in the source. Positions, comments, the
// legacy objects (Obj, Scope, Unresolved) and empty fields are left out.
func dtDump(x ast.Node) string {
	var b strings.Builder
	dtDumpValue(&b, reflect.ValueOf(x))
	return b.String()
}

func dtDumpValue(b *strings.Builder, v reflect.Value) {
	switch v.Kind() {
	case reflect.Invalid:
		b.WriteString("nil")
		return
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			b.WriteString("nil")
			return
		}
	}
	switch x := v.Interface().(type) {
	case *ast.Ident:
		b.WriteString(x.Name)
		return
	case *ast.BasicLit:
		b.WriteString(x.Value)
		return
	case *ast.SelectorExpr:
		dtDumpValue(b, reflect.ValueOf(x.X))
		b.WriteString("." + x.Sel.Name)
		return
	case token.Token:
		b.WriteString(x.String())
		return
	case ast.ChanDir:
		b.WriteString(map[ast.ChanDir]string{ast.SEND: "SEND", ast.RECV: "RECV", ast.SEND | ast.RECV: "SEND|RECV"}[x])
		return
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		dtDumpValue(b, v.Elem())
	case reflect.Slice:
		b.WriteString("[")
		for i := range v.Len() {
			if i > 0 {
				b.WriteString(", ")
			}
			dtDumpValue(b, v.Index(i))
		}
		b.WriteString("]")
	case reflect.Struct:
		b.WriteString(v.Type().Name() + "{")
		sep := ""
		for i := range v.NumField() {
			f := v.Type().Field(i)
			switch {
			case f.Type == reflect.TypeFor[token.Pos](), v.Field(i).IsZero():
				continue
			case f.Name == "Obj", f.Name == "Scope", f.Name == "Unresolved", f.Name == "Doc", f.Name == "Comment", f.Name == "Comments":
				continue
			}
			b.WriteString(sep + f.Name + ": ")
			sep = ", "
			dtDumpValue(b, v.Field(i))
		}
		b.WriteString("}")
	default:
		fmt.Fprint(b, v.Interface())
	}
}
