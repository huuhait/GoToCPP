// The tests of gocpp's patch 2 (../FORK.md §4): a type argument may be a
// constant expression, because C++ templates take values, as in
// std.array[int, 5] and std.bitset[64]. This file is gocpp's own, not
// upstream's: ../patches/0002-constant-type-args.patch adds it. It imports
// the stock go/parser and go/scanner, to compare the fork with them.

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

// A taTest is a source and the dump (taDump) of one of its nodes. ctx says
// where src goes and which node is dumped:
//   - "type": src is a type, in "var _ src"; the type is dumped;
//   - "expr": src is an expression, in "var _ = src"; it is dumped;
//   - "stmt": src is a statement, in "func _() { src }"; it is dumped;
//   - "body": src is statements, in "func _() { src }"; the body is dumped;
//   - "decl": src is a declaration; its spec is dumped, or for a func the
//     FuncDecl.
type taTest struct{ ctx, src, want string }

// taGocppTests are gocpp inputs. The stock go/parser rejects all of them
// with errors like "expected type, found 5", except the three marked "stock
// too": there upstream already parses expressions (the only argument of an
// index expression, and the arguments of an embedded field T[...]).
var taGocppTests = []taTest{
	// A type argument list in a type.
	{"decl", `var buf std.array[byte, 256]`, `ValueSpec{Names: [buf], Type: IndexListExpr{X: std.array, Indices: [byte, 256]}}`},
	{"decl", `var b std.bitset[64]`, `ValueSpec{Names: [b], Type: IndexExpr{X: std.bitset, Index: 64}}`},
	{"type", `std.ratio[1, 1000]`, `IndexListExpr{X: std.ratio, Indices: [1, 1000]}`},
	{"type", `std.array[std.array[int, 2], 3]`, `IndexListExpr{X: std.array, Indices: [IndexListExpr{X: std.array, Indices: [int, 2]}, 3]}`},
	{"type", `T[int, 5]`, `IndexListExpr{X: T, Indices: [int, 5]}`},
	{"type", `[]std.array[int, 5]`, `ArrayType{Elt: IndexListExpr{X: std.array, Indices: [int, 5]}}`},
	{"type", `*std.array[int, 5]`, `StarExpr{X: IndexListExpr{X: std.array, Indices: [int, 5]}}`},
	{"type", `map[std.array[int, 5]]std.bitset[8]`, `MapType{Key: IndexListExpr{X: std.array, Indices: [int, 5]}, Value: IndexExpr{X: std.bitset, Index: 8}}`},
	{"type", `func(std.array[int, 5]) std.bitset[8]`, `FuncType{Params: FieldList{List: [Field{Type: IndexListExpr{X: std.array, Indices: [int, 5]}}]}, Results: FieldList{List: [Field{Type: IndexExpr{X: std.bitset, Index: 8}}]}}`},
	{"decl", `type A std.array[int, 5]`, `TypeSpec{Name: A, Type: IndexListExpr{X: std.array, Indices: [int, 5]}}`},
	{"decl", `type A = std.array[int, 5]`, `TypeSpec{Name: A, Type: IndexListExpr{X: std.array, Indices: [int, 5]}}`},

	// Constant expressions.
	{"type", `std.array[int, N*2]`, `IndexListExpr{X: std.array, Indices: [int, BinaryExpr{X: N, Op: *, Y: 2}]}`},
	{"type", `std.array[int, -1]`, `IndexListExpr{X: std.array, Indices: [int, UnaryExpr{Op: -, X: 1}]}`},
	{"type", `std.array[int, len(x)]`, `IndexListExpr{X: std.array, Indices: [int, CallExpr{Fun: len, Args: [x]}]}`},
	{"type", `std.array[int, (5)]`, `IndexListExpr{X: std.array, Indices: [int, ParenExpr{X: 5}]}`},
	{"type", `std.conditional_t[N > 0, int, float64]`, `IndexListExpr{X: std.conditional_t, Indices: [BinaryExpr{X: N, Op: >, Y: 0}, int, float64]}`},

	// Struct fields, parameters and results.
	{"decl", `type S struct{ a std.array[int, 5] }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Names: [a], Type: IndexListExpr{X: std.array, Indices: [int, 5]}}]}}}`},
	{"decl", `type S struct{ a T[int, 5] }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Names: [a], Type: IndexListExpr{X: T, Indices: [int, 5]}}]}}}`},
	{"decl", `type S struct{ std.array[int, 5] }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Type: IndexListExpr{X: std.array, Indices: [int, 5]}}]}}}`},
	{"decl", `type S struct{ *std.array[int, 5] }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Type: StarExpr{X: IndexListExpr{X: std.array, Indices: [int, 5]}}}]}}}`},
	{"decl", `type S struct{ T[int, 5] }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Type: IndexListExpr{X: T, Indices: [int, 5]}}]}}}`}, // stock too
	{"decl", `func f(a std.array[int, 5])`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Names: [a], Type: IndexListExpr{X: std.array, Indices: [int, 5]}}]}}}`},
	{"decl", `func f(std.array[int, 5])`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Type: IndexListExpr{X: std.array, Indices: [int, 5]}}]}}}`},
	{"decl", `func f(a T[int, 5])`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Names: [a], Type: IndexListExpr{X: T, Indices: [int, 5]}}]}}}`},
	{"decl", `func f(a ...std.array[int, 5])`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Names: [a], Type: Ellipsis{Elt: IndexListExpr{X: std.array, Indices: [int, 5]}}}]}}}`},
	{"decl", `func f() std.array[int, 5]`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Type: IndexListExpr{X: std.array, Indices: [int, 5]}}]}}}`},
	{"decl", `func f() (r std.array[int, 5])`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Names: [r], Type: IndexListExpr{X: std.array, Indices: [int, 5]}}]}}}`},

	// Embedded in an interface, and in a constraint. The type parameter
	// list itself doesn't change: P is still a name.
	{"decl", `type I interface{ std.array[int, 5] }`, `TypeSpec{Name: I, Type: InterfaceType{Methods: FieldList{List: [Field{Type: IndexListExpr{X: std.array, Indices: [int, 5]}}]}}}`},
	{"decl", `type I interface{ J[int, 5] }`, `TypeSpec{Name: I, Type: InterfaceType{Methods: FieldList{List: [Field{Type: IndexListExpr{X: J, Indices: [int, 5]}}]}}}`},
	{"decl", `func f[P C[int, 5]]()`, `FuncDecl{Name: f, Type: FuncType{TypeParams: FieldList{List: [Field{Names: [P], Type: IndexListExpr{X: C, Indices: [int, 5]}}]}, Params: FieldList{}}}`},

	// Composite literals, calls and conversions.
	{"stmt", `bits := std.bitset[64]{}`, `AssignStmt{Lhs: [bits], Tok: :=, Rhs: [CompositeLit{Type: IndexExpr{X: std.bitset, Index: 64}}]}`}, // stock too
	{"stmt", `a := std.array[int, 5]{}`, `AssignStmt{Lhs: [a], Tok: :=, Rhs: [CompositeLit{Type: IndexListExpr{X: std.array, Indices: [int, 5]}}]}`},
	{"expr", `std.array[int, 5]{1, 2}`, `CompositeLit{Type: IndexListExpr{X: std.array, Indices: [int, 5]}, Elts: [1, 2]}`},
	{"expr", `std.array[std.array[int, 2], 3]{}`, `CompositeLit{Type: IndexListExpr{X: std.array, Indices: [IndexListExpr{X: std.array, Indices: [int, 2]}, 3]}}`},
	{"expr", `std.foo[int, N*2, -1, len(x)]{}`, `CompositeLit{Type: IndexListExpr{X: std.foo, Indices: [int, BinaryExpr{X: N, Op: *, Y: 2}, UnaryExpr{Op: -, X: 1}, CallExpr{Fun: len, Args: [x]}]}}`},
	{"expr", `f[int, 5](x)`, `CallExpr{Fun: IndexListExpr{X: f, Indices: [int, 5]}, Args: [x]}`},
	{"expr", `std.array[int, 5](x)`, `CallExpr{Fun: IndexListExpr{X: std.array, Indices: [int, 5]}, Args: [x]}`},
	{"expr", `std.get[0](t)`, `CallExpr{Fun: IndexExpr{X: std.get, Index: 0}, Args: [t]}`}, // stock too
	{"expr", `x.(std.array[int, 5])`, `TypeAssertExpr{X: x, Type: IndexListExpr{X: std.array, Indices: [int, 5]}}`},
}

// taLikeGoTests are gocpp inputs where the patch meets the parser's own
// guesses: is [P *C[...]] an array length or a type parameter list, does
// "{" after T[...] in an if or for header start a composite literal, and
// instances in method values and conversions. Each must parse like Go:
// TestTypeArgLikeGo checks that the stock go/parser gives the same AST and
// the same errors to the same code with the type name Z in place of the
// constant 5 (one letter, so the positions are the same). So src has no
// other 5 or Z. err is the first error, "" for none.
var taLikeGoTests = []struct{ ctx, src, want, err string }{
	// An array type, as for [P *C[int, string]].
	{"decl", `type T[P *C[int, 5]] struct{}`, `TypeSpec{Name: T, Type: ArrayType{Len: BinaryExpr{X: P, Op: *, Y: IndexListExpr{X: C, Indices: [int, 5]}}, Elt: StructType{Fields: FieldList{}}}}`, ``},
	// A type parameter list.
	{"decl", `type T[P *C[int, 5],] struct{}`, `TypeSpec{Name: T, TypeParams: FieldList{List: [Field{Names: [P], Type: StarExpr{X: IndexListExpr{X: C, Indices: [int, 5]}}}]}, Type: StructType{Fields: FieldList{}}}`, ``},
	// An array type.
	{"decl", `type T[P (C[int, 5])] struct{}`, `TypeSpec{Name: T, Type: ArrayType{Len: CallExpr{Fun: P, Args: [IndexListExpr{X: C, Indices: [int, 5]}]}, Elt: StructType{Fields: FieldList{}}}}`, ``},
	// The condition is x == T[int, 5], {} is the body.
	{"body", "if x == T[int, 5]{}\n{}", `BlockStmt{List: [IfStmt{Cond: BinaryExpr{X: x, Op: ==, Y: IndexListExpr{X: T, Indices: [int, 5]}}, Body: BlockStmt{}}, BlockStmt{}]}`, ``},
	// The same on one line: Go wants a ";" before the second {}.
	{"body", `if x == T[int, 5]{} {}`, `BlockStmt{List: [IfStmt{Cond: BinaryExpr{X: x, Op: ==, Y: IndexListExpr{X: T, Indices: [int, 5]}}, Body: BlockStmt{}}]}`, `expected ';', found '{'`},
	// A composite literal in parentheses.
	{"body", `if x == (T[int, 5]{}) {}`, `BlockStmt{List: [IfStmt{Cond: BinaryExpr{X: x, Op: ==, Y: ParenExpr{X: CompositeLit{Type: IndexListExpr{X: T, Indices: [int, 5]}}}}, Body: BlockStmt{}}]}`, ``},
	// The range is over T[int, 5], {} is the body.
	{"body", "for range T[int, 5]{}\n{}", `BlockStmt{List: [RangeStmt{X: IndexListExpr{X: T, Indices: [int, 5]}, Body: BlockStmt{}}, BlockStmt{}]}`, ``},
	// The same on one line: Go wants a ";" before the second {}.
	{"body", `for range T[int, 5]{} {}`, `BlockStmt{List: [RangeStmt{X: IndexListExpr{X: T, Indices: [int, 5]}, Body: BlockStmt{}}]}`, `expected ';', found '{'`},
	// An instantiated method.
	{"expr", `v.m[int, 5](x)`, `CallExpr{Fun: IndexListExpr{X: v.m, Indices: [int, 5]}, Args: [x]}`, ``},
	// A method value of an instance.
	{"expr", `T[int, 5].m`, `IndexListExpr{X: T, Indices: [int, 5]}.m`, ``},
	// A conversion to a pointer to an instance.
	{"expr", `(*T[int, 5])(p)`, `CallExpr{Fun: ParenExpr{X: StarExpr{X: IndexListExpr{X: T, Indices: [int, 5]}}}, Args: [p]}`, ``},
}

// taGoTests are Go inputs that are near type argument lists: array types,
// type parameter lists (with Go's own ambiguous cases like [P *C]),
// instances, index and slice expressions, struct fields. The long G[...]
// lists hold every kind of type as a type argument. TestTypeArgGoUnchanged
// parses each one with the fork and with the stock go/parser and requires
// the same AST; want is the dump of it, to show its shape.
var taGoTests = []taTest{
	{"type", `[5]int`, `ArrayType{Len: 5, Elt: int}`},
	{"type", `[N]int`, `ArrayType{Len: N, Elt: int}`},
	{"type", `[]int`, `ArrayType{Elt: int}`},
	{"type", `List[int]`, `IndexExpr{X: List, Index: int}`},
	{"type", `Map[K, V]`, `IndexListExpr{X: Map, Indices: [K, V]}`},
	{"type", `pkg.Map[K, []V]`, `IndexListExpr{X: pkg.Map, Indices: [K, ArrayType{Elt: V}]}`},
	{"type", `G[*T, []T, [2]T, map[K]V, chan T, <-chan T, chan<- T, func(T) T, struct{ f T }, interface{ M() }, (T), pkg.T, H[T], <-chan <-chan T]`, `IndexListExpr{X: G, Indices: [StarExpr{X: T}, ArrayType{Elt: T}, ArrayType{Len: 2, Elt: T}, MapType{Key: K, Value: V}, ChanType{Dir: SEND|RECV, Value: T}, ChanType{Dir: RECV, Value: T}, ChanType{Dir: SEND, Value: T}, FuncType{Params: FieldList{List: [Field{Type: T}]}, Results: FieldList{List: [Field{Type: T}]}}, StructType{Fields: FieldList{List: [Field{Names: [f], Type: T}]}}, InterfaceType{Methods: FieldList{List: [Field{Names: [M], Type: FuncType{Params: FieldList{}}}]}}, ParenExpr{X: T}, pkg.T, IndexExpr{X: H, Index: T}, ChanType{Dir: RECV, Value: ChanType{Dir: RECV, Value: T}}]}`},
	{"expr", `[...]int{1}`, `CompositeLit{Type: ArrayType{Len: Ellipsis{}, Elt: int}, Elts: [1]}`},
	{"expr", `Map[K, V]{}`, `CompositeLit{Type: IndexListExpr{X: Map, Indices: [K, V]}}`},
	{"expr", `f[int](x)`, `CallExpr{Fun: IndexExpr{X: f, Index: int}, Args: [x]}`},
	{"expr", `f[int, string](x)`, `CallExpr{Fun: IndexListExpr{X: f, Indices: [int, string]}, Args: [x]}`},
	{"expr", `G[*T, []T, [2]T, map[K]V, chan T, <-chan T, chan<- T, func(T) T, struct{ f T }, interface{ M() }, (T), pkg.T, H[T], <-chan <-chan T]{}`, `CompositeLit{Type: IndexListExpr{X: G, Indices: [StarExpr{X: T}, ArrayType{Elt: T}, ArrayType{Len: 2, Elt: T}, MapType{Key: K, Value: V}, ChanType{Dir: SEND|RECV, Value: T}, ChanType{Dir: RECV, Value: T}, ChanType{Dir: SEND, Value: T}, FuncType{Params: FieldList{List: [Field{Type: T}]}, Results: FieldList{List: [Field{Type: T}]}}, StructType{Fields: FieldList{List: [Field{Names: [f], Type: T}]}}, InterfaceType{Methods: FieldList{List: [Field{Names: [M], Type: FuncType{Params: FieldList{}}}]}}, ParenExpr{X: T}, pkg.T, IndexExpr{X: H, Index: T}, ChanType{Dir: RECV, Value: ChanType{Dir: RECV, Value: T}}]}}`},
	{"expr", `a[i]`, `IndexExpr{X: a, Index: i}`},
	{"expr", `m[k]`, `IndexExpr{X: m, Index: k}`},
	{"expr", `a[i:j]`, `SliceExpr{X: a, Low: i, High: j}`},
	{"expr", `a[i:j:k]`, `SliceExpr{X: a, Low: i, High: j, Max: k, Slice3: true}`},
	{"expr", `a[:]`, `SliceExpr{X: a}`},
	{"decl", `type A [N]int`, `TypeSpec{Name: A, Type: ArrayType{Len: N, Elt: int}}`},
	{"decl", `type A [N * 2]int`, `TypeSpec{Name: A, Type: ArrayType{Len: BinaryExpr{X: N, Op: *, Y: 2}, Elt: int}}`},
	{"decl", `type T[P any] struct{}`, `TypeSpec{Name: T, TypeParams: FieldList{List: [Field{Names: [P], Type: any}]}, Type: StructType{Fields: FieldList{}}}`},
	{"decl", `type T[P *C] struct{}`, `TypeSpec{Name: T, Type: ArrayType{Len: BinaryExpr{X: P, Op: *, Y: C}, Elt: StructType{Fields: FieldList{}}}}`},
	{"decl", `type T[P *C,] struct{}`, `TypeSpec{Name: T, TypeParams: FieldList{List: [Field{Names: [P], Type: StarExpr{X: C}}]}, Type: StructType{Fields: FieldList{}}}`},
	{"decl", `type T[P *C | D] struct{}`, `TypeSpec{Name: T, Type: ArrayType{Len: BinaryExpr{X: BinaryExpr{X: P, Op: *, Y: C}, Op: |, Y: D}, Elt: StructType{Fields: FieldList{}}}}`},
	{"decl", `type T[P *C | D,] struct{}`, `TypeSpec{Name: T, TypeParams: FieldList{List: [Field{Names: [P], Type: BinaryExpr{X: StarExpr{X: C}, Op: |, Y: D}}]}, Type: StructType{Fields: FieldList{}}}`},
	{"decl", `type T[P *struct{}] struct{}`, `TypeSpec{Name: T, TypeParams: FieldList{List: [Field{Names: [P], Type: StarExpr{X: StructType{Fields: FieldList{}}}}]}, Type: StructType{Fields: FieldList{}}}`},
	{"decl", `type T[P (C)] struct{}`, `TypeSpec{Name: T, Type: ArrayType{Len: CallExpr{Fun: P, Args: [C]}, Elt: StructType{Fields: FieldList{}}}}`},
	{"decl", `func f[P any]()`, `FuncDecl{Name: f, Type: FuncType{TypeParams: FieldList{List: [Field{Names: [P], Type: any}]}, Params: FieldList{}}}`},
	{"decl", `func f[P C[int, string]]()`, `FuncDecl{Name: f, Type: FuncType{TypeParams: FieldList{List: [Field{Names: [P], Type: IndexListExpr{X: C, Indices: [int, string]}}]}, Params: FieldList{}}}`},
	{"decl", `type S struct{ a [5]int }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Names: [a], Type: ArrayType{Len: 5, Elt: int}}]}}}`},
	{"decl", `type S struct{ a [N]T }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Names: [a], Type: ArrayType{Len: N, Elt: T}}]}}}`},
	{"decl", `type S struct{ T[P] }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Type: IndexExpr{X: T, Index: P}}]}}}`},
	{"decl", `type S struct{ T[P1, P2] }`, `TypeSpec{Name: S, Type: StructType{Fields: FieldList{List: [Field{Type: IndexListExpr{X: T, Indices: [P1, P2]}}]}}}`},
	{"decl", `func f(a [5]int)`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Names: [a], Type: ArrayType{Len: 5, Elt: int}}]}}}`},
	{"decl", `func f(a T[int])`, `FuncDecl{Name: f, Type: FuncType{Params: FieldList{List: [Field{Names: [a], Type: IndexExpr{X: T, Index: int}}]}}}`},
	{"decl", `func (r *T[P, Q]) m()`, `FuncDecl{Recv: FieldList{List: [Field{Names: [r], Type: StarExpr{X: IndexListExpr{X: T, Indices: [P, Q]}}}]}, Name: m, Type: FuncType{Params: FieldList{}}}`},
	{"decl", `type I interface{ J[int, string] }`, `TypeSpec{Name: I, Type: InterfaceType{Methods: FieldList{List: [Field{Type: IndexListExpr{X: J, Indices: [int, string]}}]}}}`},
}

// taRecoveryTests are broken inputs where a type argument starts with a
// token that can't start one: an operator, "...", a closing bracket, a
// keyword on the next line. There the patch keeps upstream's error and
// error recovery: TestTypeArgRecovery requires the same errors and the same
// AST as the stock go/parser. The recovery can still differ where the patch
// parses an expression and upstream parsed a type: at a name or a literal
// (also later, when upstream's recovery goes on in the same list), and in a
// list inside a type argument that starts with a name, such as the inner
// list of P[T[K, }], which then recovers like upstream's own T[K, } in an
// expression.
var taRecoveryTests = []string{
	"package p\nvar _ std.array[int, | ]\nvar y = 1\nfunc g() {}\n",
	"package p\nvar _ = f[int, ... ](x)\nvar y = 1\n",
	"package p\nfunc f() {\n\tx := std.array[int,\n\treturn x\n}\nvar z = 1\n",
	"package p\nvar _ T[int, )\nvar y = 1\n",
	"package p\nvar _ = f[int, )\nvar y = 1\n",
	"package p\ntype _ interface{ J[int, ) }\nvar y = 1\n",
	"package p\nvar _ T[) ]\nvar y = 1\n",
	"package p\nvar _ T[int, = ]\nvar y = 1\n",
	"package p\nvar _ T[int, {} ]\nvar y = 1\n",
	"package p\nvar _ = f[int, .x](y)\nvar y = 1\n",
	"package p\nfunc _() { _ = x[a, b:c] }\n",
	"package p\nfunc f() {\n\tvar x std.array[int,\n\tif x {\n\t}\n}\nvar z = 1\n",
	"package p\ntype S struct {\n\tbuf std.array[byte,\n}\n\nfunc main() {}\n",
	"package p\nfunc f() {\n\tx := std.array[int,\n}\n",
	"package p\nvar _ = f[int, ;]\nvar y = 1\n",
	"package p\nfunc f() {\n\tx := f[int,\n\tif x {\n\t}\n}\n",
	"package p\nfunc f() {\n\tx := f[int,\n\tfor {\n\t}\n}\nvar z = 1\n",
	"package p\nvar _ std.array[int, ...]\nvar y = 1\n",
	"package p\nvar _ = std.array[int, ...]{}\nvar y = 1\n",
	"package p\ntype I interface {\n\tJ[int, |]\n\tm()\n}\n",
	"package p\nfunc f(a std.array[int, |]) {}\nvar y = 1\n",
	"package p\nvar _ T[int, |, string]\nvar y = 1\n",
}

// TestTypeArgConstants checks that gocpp's constant type arguments parse,
// as type arguments, in types, struct fields, parameters, results,
// composite literals and calls.
func TestTypeArgConstants(t *testing.T) {
	for _, test := range taGocppTests {
		src := taSource(t, test)
		f, err := ParseFile(token.NewFileSet(), "", src, DeclarationErrors|AllErrors)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if got := taDump(taNode(test, f)); got != test.want {
			t.Errorf("%s:\ngot  %s\nwant %s", src, got, test.want)
		}
	}
}

// TestTypeArgLikeGo checks that each of taLikeGoTests parses like Go code:
// to the AST and the errors that the stock go/parser gives to the same
// source with Z in place of 5.
func TestTypeArgLikeGo(t *testing.T) {
	for _, test := range taLikeGoTests {
		tt := taTest{test.ctx, test.src, test.want}
		src := taSource(t, tt)
		f, err := ParseFile(token.NewFileSet(), "", src, DeclarationErrors|AllErrors)
		errs := taErrors(err)
		if (test.err == "" && len(errs) > 0) || (test.err != "" && (len(errs) == 0 || !strings.Contains(errs[0], test.err))) {
			t.Errorf("%s: errors %q, want the first one to be %q", src, errs, test.err)
		}
		got := taDump(taNode(tt, f))
		if got != test.want {
			t.Errorf("%s:\ngot  %s\nwant %s", src, got, test.want)
		}
		goSrc := strings.ReplaceAll(src, "5", "Z")
		g, err := goparser.ParseFile(token.NewFileSet(), "", goSrc, goparser.DeclarationErrors|goparser.AllErrors)
		goErrs := taErrors(err)
		if like := strings.ReplaceAll(taDump(taNode(tt, g)), "Z", "5"); like != got || !slices.Equal(goErrs, errs) {
			t.Errorf("%s doesn't parse like Go's %s:\nfork      %s %q\ngo/parser %s %q (with 5 for Z)", src, goSrc, got, errs, like, goErrs)
		}
	}
}

// TestTypeArgGoUnchanged checks that the Go code near type argument lists
// parses as with the stock go/parser: reflect.DeepEqual on the two files,
// so the positions and the legacy objects must be the same too.
func TestTypeArgGoUnchanged(t *testing.T) {
	for _, test := range taGoTests {
		src := taSource(t, test)
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
			t.Errorf("%s: the fork and go/parser give different ASTs (in positions or objects, if the dumps are the same):\nfork      %s\ngo/parser %s", src, taDump(f), taDump(g))
		}
		if got := taDump(taNode(test, f)); got != test.want {
			t.Errorf("%s:\ngot  %s\nwant %s", src, got, test.want)
		}
	}
}

// taSource returns the source of test, as a file (see taTest).
func taSource(t *testing.T, test taTest) string {
	t.Helper()
	switch test.ctx {
	case "type":
		return "package p; var _ " + test.src
	case "expr":
		return "package p; var _ = " + test.src
	case "stmt", "body":
		return "package p; func _() { " + test.src + " }"
	case "decl":
		return "package p; " + test.src
	}
	t.Fatalf("%s: bad ctx %q", test.src, test.ctx)
	return ""
}

// taNode returns the node of f that test dumps (see taTest).
func taNode(test taTest, f *ast.File) ast.Node {
	switch d := f.Decls[0].(type) {
	case *ast.GenDecl:
		spec := d.Specs[0]
		if vs, ok := spec.(*ast.ValueSpec); ok && test.ctx == "type" {
			return vs.Type
		} else if ok && test.ctx == "expr" {
			return vs.Values[0]
		}
		return spec
	case *ast.FuncDecl:
		switch test.ctx {
		case "stmt":
			return d.Body.List[0]
		case "body":
			return d.Body
		}
		return d
	}
	return nil
}

// TestTypeArgErrors checks errors that stay: type parameter lists and array
// lengths still take no constants, an empty type argument list is still an
// error, and a type argument that starts with a token that can't start one
// gets upstream's error and recovery (parseTypeArg). checkErrors keeps the
// first error of each line, and fails on errors in other lines.
func TestTypeArgErrors(t *testing.T) {
	for _, src := range []string{
		`package p; func _[P any, 5 /* ERROR "expected '\)', found 5" */ ]()`,
		`package p; type _[P any, 5 /* ERROR "expected '\)', found 5" */ ] struct{}`,
		`package p; var _ [5, /* ERROR "unexpected comma; expecting ]" */ 6]int`,
		`package p; var _ T[] /* ERROR "expected type argument list" */`,
		`package p; var _ std.array[int, 5 6 /* ERROR "missing ',' in type argument list" */ ]`,
		`package p; var _ std.array[int, ) /* ERROR "expected type, found '\)'" */`,
		`package p; var _ = std.array[int, ) /* ERROR "expected '\]', found '\)'" */`,
		`package p; type _ interface{ J[int, ) /* ERROR "expected type, found '\)'" */ }`,
		"package p\nvar _ std.array[int, | /* ERROR \"expected type, found '\\|'\" */ ]\nvar y = 1\nfunc g() {}\n",
		"package p\nvar _ = f[int, ... /* ERROR \"expected type, found '...'\" */ ](x)\nvar y = 1\n",
		"package p\nfunc f() {\n\tx := std.array[int,\n\treturn /* ERROR \"expected type, found 'return'\" */ x\n}\nvar z = 1\n",
	} {
		checkErrors(t, src, src, DeclarationErrors|AllErrors, true)
	}
}

// TestTypeArgRecovery checks that each of taRecoveryTests gets the same
// errors and the same AST from the fork as from the stock go/parser.
func TestTypeArgRecovery(t *testing.T) {
	for _, src := range taRecoveryTests {
		f, err := ParseFile(token.NewFileSet(), "", src, DeclarationErrors|AllErrors)
		got := taErrors(err)
		g, err := goparser.ParseFile(token.NewFileSet(), "", src, goparser.DeclarationErrors|goparser.AllErrors)
		want := taErrors(err)
		switch {
		case len(want) == 0:
			t.Errorf("%q: go/parser reports no error, but a recovery test needs a broken input", src)
		case !slices.Equal(got, want):
			t.Errorf("%q: the fork and go/parser report different errors:\nfork      %q\ngo/parser %q", src, got, want)
		case !reflect.DeepEqual(f, g):
			t.Errorf("%q: the fork and go/parser give different ASTs (in positions or objects, if the dumps are the same):\nfork      %s\ngo/parser %s", src, taDump(f), taDump(g))
		}
	}
}

// taErrors returns the errors of a parse, one string each. err is from the
// fork or from the stock go/parser.
func taErrors(err error) []string {
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

// TestTypeArgStartTokens checks the token list of parseTypeArg against the
// expression parser, for every token X. ParseExprFrom calls parseRhs, so X
// can start an expression unless ParseExprFrom reports "expected operand"
// at X. If it can, the fork must parse the type argument X in T[int, X],
// f[int, X] and interface{ J[int, X] } with parseRhs; if it can't, with
// parseType, which reports "expected type" at X, as upstream. "]" and EOF
// are left out: the callers stop there before they call parseTypeArg.
//
// That check sees a missing token only where parseType fails. Eight tokens
// start a type too, and there parseType parses the type without an error.
// So for each of them, the test also parses an expression that starts with
// it and is not a type, in the same three places. It must parse without an
// error; parseType would stop after the type, at the "{", "(" or "+", or
// for "<-" at ch, where a type has chan. (*p alone would not do: parseType
// parses it as a pointer type, to the same AST.) IDENT and "(" also start
// both; TestTypeArgConstants covers them, with N*2 and (5).
func TestTypeArgStartTokens(t *testing.T) {
	samples := map[token.Token]string{
		token.IDENT: "x", token.INT: "1", token.FLOAT: "1.5", token.IMAG: "1i", token.CHAR: "'c'", token.STRING: `"s"`,
	}
	n := 0
	// The 16 after TILDE, the last token of Go 1.25, are for tokens that a
	// later Go adds.
	for tok := token.ILLEGAL; tok <= token.TILDE+16; tok++ {
		switch {
		case tok == token.ILLEGAL, tok == token.EOF, tok == token.COMMENT, tok == token.RBRACK:
			continue
		case strings.HasPrefix(tok.String(), "token("):
			continue // not a token: the bounds of the groups of tokens in go/token
		}
		n++
		text := tok.String()
		if s, ok := samples[tok]; ok {
			text = s
		}
		_, err := ParseExprFrom(token.NewFileSet(), "", text+" ", AllErrors)
		canStart := !taErrorAt(err, 0, "expected operand")
		for _, pre := range []string{"package p; var _ T[int, ", "package p; var _ = f[int, ", "package p; type _ interface{ J[int, "} {
			src := pre + text + " ]"
			_, err := ParseFile(token.NewFileSet(), "", src, AllErrors)
			switch expectedType := taErrorAt(err, len(pre), "expected type"); {
			case canStart && expectedType:
				t.Errorf("%s: %s can start an expression, but parseTypeArg parsed a type (\"expected type\" at %s)", src, tok, tok)
			case !canStart && !expectedType:
				t.Errorf("%s: %s can't start an expression, but parseTypeArg didn't call parseType (no \"expected type\" at %s)", src, tok, tok)
			}
		}
	}
	if n < 70 {
		t.Errorf("checked only %d tokens", n)
	}

	// The eight tokens that start a type and an expression.
	for _, test := range []struct {
		tok  token.Token
		expr string
	}{
		{token.LBRACK, "[1]int{}"},
		{token.MUL, "*p + 1"},
		{token.ARROW, "<-ch"},
		{token.FUNC, "func() {}"},
		{token.STRUCT, "struct{}{}"},
		{token.INTERFACE, "interface{}(nil)"},
		{token.MAP, "map[int]int{}"},
		{token.CHAN, "chan int(nil)"},
	} {
		if !strings.HasPrefix(test.expr, test.tok.String()) {
			t.Fatalf("%s does not start with %s", test.expr, test.tok)
		}
		for _, src := range []string{
			"package p; var _ T[int, " + test.expr + "]",
			"package p; var _ = f[int, " + test.expr + "]",
			"package p; type _ interface{ J[int, " + test.expr + "] }",
		} {
			if _, err := ParseFile(token.NewFileSet(), "", src, AllErrors); err != nil {
				t.Errorf("%s: %s starts a type and an expression, so parseTypeArg must parse the expression %s with parseRhs: %v", src, test.tok, test.expr, err)
			}
		}
	}
}

// taErrorAt reports whether err has an error at the byte offset off whose
// message starts with prefix.
func taErrorAt(err error, off int, prefix string) bool {
	list, _ := err.(scanner.ErrorList)
	for _, e := range list {
		if e.Pos.Offset == off && strings.HasPrefix(e.Msg, prefix) {
			return true
		}
	}
	return false
}

// TestTypeArgGoTypes checks that a value where a Go generic wants a type is
// still an error: the parser accepts it, and go/types reports that it is
// not a type. A union in a type argument list was a syntax error ("missing
// ','"); now go/types reports it. Each want must be reported, and there
// must be no error on other lines (go/types adds follow-up errors on the
// line of a union).
func TestTypeArgGoTypes(t *testing.T) {
	const src = `package p

const N = 4

func F[T, U any]() {}

type G[T, U any] struct{}

type H[T any] struct{}

var _ G[int, 5]
var _ H[64]
var _ = G[int, N*2]{}

func _() {
	F[int, 5]()
}

var _ G[int, string|bool]

type I interface{ G[int, 5] }

func (*G[T, 5]) m() {}
`
	fset := token.NewFileSet()
	f, err := ParseFile(fset, "p.go", src, AllErrors)
	if err != nil {
		t.Fatalf("syntax error: %v", err)
	}
	var got []types.Error
	conf := types.Config{Error: func(err error) { got = append(got, err.(types.Error)) }}
	conf.Check("p", fset, []*ast.File{f}, nil)
	want := []string{
		"p.go:11:14: 5 is not a type",
		"p.go:12:9: 64 is not a type",
		"p.go:13:16: N * 2 is not a type",
		"p.go:16:9: 5 is not a type",
		"p.go:19:14: string | bool is not a type",
		"p.go:21:26: 5 is not a type",
		"p.go:23:13: receiver type parameter 5 must be an identifier",
	}
	lines := map[int]bool{}
	for _, w := range want {
		var line int
		fmt.Sscanf(w, "p.go:%d:", &line)
		lines[line] = true
		if !slices.ContainsFunc(got, func(e types.Error) bool { return e.Error() == w }) {
			t.Errorf("go/types did not report %q", w)
		}
	}
	for _, e := range got {
		if !lines[fset.Position(e.Pos).Line] {
			t.Errorf("unexpected go/types error: %v", e)
		}
	}
}

// taDump returns the syntax tree x on one line, to compare its shape. A
// node is its type name with its fields, as in IndexExpr{X: a, Index: i};
// an Ident is its name, a BasicLit its literal and a SelectorExpr X.Sel.
// Positions, comments, the legacy objects (Obj, Scope) and empty fields are
// left out.
func taDump(x ast.Node) string {
	var b strings.Builder
	taDumpValue(&b, reflect.ValueOf(x))
	return b.String()
}

func taDumpValue(b *strings.Builder, v reflect.Value) {
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
		taDumpValue(b, reflect.ValueOf(x.X))
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
		taDumpValue(b, v.Elem())
	case reflect.Slice:
		b.WriteString("[")
		for i := range v.Len() {
			if i > 0 {
				b.WriteString(", ")
			}
			taDumpValue(b, v.Index(i))
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
			case f.Name == "Obj", f.Name == "Scope", f.Name == "Doc", f.Name == "Comment":
				continue
			}
			b.WriteString(sep + f.Name + ": ")
			sep = ", "
			taDumpValue(b, v.Field(i))
		}
		b.WriteString("}")
	default:
		fmt.Fprint(b, v.Interface())
	}
}
