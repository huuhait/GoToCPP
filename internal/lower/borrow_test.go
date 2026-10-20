package lower

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// borrowSrc is the program of TestBorrow. A function (or method) whose line ends with
// "// borrow: p, q" must borrow exactly the parameters (or receiver) with those names, one
// without the comment must borrow none, and "pure" in the comment marks a pure function
// (borrow.go). The parameters that escape are the cases that matter: borrowing one of them would
// hand a callee a raw pointer that it keeps, returns or stores after the caller dropped it.
const borrowSrc = `package main

import (
	"fmt"
	"unsafe"
	"<vector>"
)

type Inner struct{ v int }

func (i *Inner) Inc()   { i.v++ } // borrow: i; pure
func (i Inner) Get() int { return i.v } // pure

type Node struct {
	next *Node
	val  int
	arr  [4]int
	in   Inner
	ep   *Inner
}

type Emb struct {
	Inner
	*Node
	k int
}

type Iface interface{ M() }
type IfaceP interface{ P(*Node) }
type Node2 Node

var global *Node
var gnode Node

// --- borrowed ---

func readField(n *Node) int { return n.val } // borrow: n; pure
func writeField(n *Node)    { n.val = 3 } // borrow: n; pure
func deref(n *Node) Node    { return *n } // borrow: n; pure
func setDeref(n *Node)      { *n = Node{} } // borrow: n
func isNil(n *Node) bool    { return n == nil } // borrow: n; pure
func same(a, b *Node) bool  { return a == b } // borrow: a, b; pure
func viaFields(n *Node) int { return n.next.val + n.arr[2] + n.in.v } // borrow: n; pure
func loop(n *Node) int { // borrow: n; pure
	s := 0
	for m := n.next; m != nil; m = m.next {
		s += m.val
	}
	return s
}
func (n *Node) sum() int { // borrow: n; pure
	if n == nil {
		return 0
	}
	return n.val + n.next.sum()
}
func callsBorrowed(n *Node) { writeField(n) } // borrow: n; pure
func callsAddr(n *Node)     { n.in.Inc() } // borrow: n; pure
func callsAddr2(n *Node)    { Inc(&n.in) } // borrow: n; pure
func Inc(i *Inner)          { i.v++ } // borrow: i; pure
func valueMethod(n *Node) int { return n.in.Get() } // borrow: n; pure
func (e *Emb) viaEmbedded()   { e.Inc() } // borrow: e; pure
func (e *Emb) viaEmbedded2()  { e.Inner.Inc() } // borrow: e; pure
func (e *Emb) viaPtrEmbedded() { e.Node.val++ } // borrow: e; pure
func arrLen(p *[4]int) int     { return len(p) + p[1] } // borrow: p; pure
func recur(n *Node, d int) { // borrow: n; pure
	if d > 0 {
		recur(n, d-1)
	}
	n.val++
}
func mutual1(n *Node, d int) { // borrow: n; pure
	if d > 0 {
		mutual2(n, d-1)
	}
}
func mutual2(n *Node, d int) { mutual1(n, d) } // borrow: n; pure
func unusedParam(n *Node)     {} // borrow: n; pure
func (n *Node) unusedRecv()   {} // borrow: n; pure
func (*Node) noName(x int) int { return x } // borrow: _; pure
func withPrint(n *Node)       { println(n.val) } // borrow: n; pure
func cmpNilFirst(n *Node) bool { return nil == n } // borrow: n; pure
func twoLevel(e *Emb) int      { return e.Node.next.val } // borrow: e; pure
func stringStore(n *Node, s string) { n.val = len(s) } // borrow: n; pure; ref: s
func sliceStore(xs []int)           { xs[0] = 1 } // pure; ref: xs
func fieldWithPtrCopy(n *Node) *Node { return n.next } // borrow: n; pure
func nodeToLocal(n *Node) int { // borrow: n; pure
	m := n.next // a copy of a pointer loaded from n: n is only read
	return m.val
}

// --- escaping ---

func stores(n *Node)           { global = n } // ref: n
func storesField(a, b *Node)   { a.next = b } // borrow: a; ref: b
func returns(n *Node) *Node    { return n } // pure
func copiesLocal(n *Node) int { // pure; a local copy is an owning pointer; ref: n
	m := n
	return m.val
}
func capture(n *Node) func() int { return func() int { return n.val } } // ref: n
func captureCall(n *Node) int    { return func() int { return n.val }() } // ref: n
func goes(n *Node)               { go readField(n) } // ref: n
func defers(n *Node)             { defer writeField(n) } // ref: n
func sends(n *Node, c chan *Node) { c <- n } // ref: c, n
func toIface(n *Node) any        { return n } // pure
func toIface2(n *Node)           { fmt.Println(n) } // ref: n
func toIfaceMethod(n *Node, i Iface) { var a any = n; _ = i; _ = a } // pure; ref: i, n
func reassign(n *Node)           { n = nil; _ = n } // pure
func addrOfParam(n *Node) **Node { return &n } // pure
func methodValue(i *Inner) func() { return i.Inc } // pure; ref: i
func sliceOfArr(n *Node) []int   { return n.arr[:] } // pure; ref: n
func sliceOfArrPtr(p *[4]int) []int { return p[:] } // pure; ref: p
func addrStored(n *Node) *int    { return &n.val } // pure; ref: n
func addrField(n *Node) *Inner   { return &n.in } // pure; ref: n
func passEscaping(n *Node)       { stores(n) } // ref: n
func passEscaping2(n *Node)      { returns(n) } // pure; ref: n
func passViaAddr(n *Node)        { storesInner(&n.in) } // ref: n
func storesInner(i *Inner)       { ginner = i } // ref: i
func passFunc(n *Node, f func(*Node)) { f(n) } // ref: f, n
func passIface(n *Node, i IfaceP)     { i.P(n) } // ref: i, n
func conv(n *Node) *Node2             { return (*Node2)(n) } // pure; ref: n
func addrStar(n *Node) *Node          { return &*n } // pure; ref: n
func addrIdx(p *[4]int) *int          { return &p[1] } // pure; ref: p
func idxCall(p *[4]int)               { setInt(&p[2]) } // borrow: p; pure
func setInt(x *int)                   { *x = 1 } // borrow: x; pure
func starCopy(n *Node) Node            { return *n } // borrow: n; pure
func cmpIface(n *Node, i any) bool     { return i == n } // pure; ref: i, n
func appendTo(xs []*Node, n *Node) []*Node { return append(xs, n) } // ref: n, xs
func mapStore(m map[*Node]int, n *Node)   { m[n] = 1 } // pure; ref: m, n
func rangeFunc(n *Node, seq func(func(int) bool)) { // ref: n, seq
	for range seq {
		n.val++
	}
}
func compLit(n *Node) Node           { return Node{next: n} } // pure; ref: n
func switchOn(n *Node) int { // pure; ref: n
	switch n {
	case nil:
		return 1
	}
	return 0
}
func tupleCall(n *Node) { pair(n) } // pure; ref: n
func pair(n *Node) (*Node, int) { return n, 1 } // pure
func embeddedPtrMethod(e *Emb) *Emb { return e } // pure
func (e *Emb) promotedStore() { e.Inner.v = 1; ginner = &e.Inner } // ref: e
func (e *Emb) M()                { _ = e.k } // borrow: e; pure
func (e *Emb) viaIfaceEmbedded() { var i Iface = e; _ = i } // pure; ref: e
func (n *Node) valueRecvAddr() *Node { return n } // pure
func (n *Node) keep()                { global = n } // ref: n
func callsKeep(n *Node)              { n.keep() } // ref: n
func (i *Inner) keepI()              { ginner = i } // ref: i
func (e *Emb) viaEmbeddedKeep()      { e.keepI() } // ref: e
func (e *Emb) viaEmbeddedKeep2()     { e.Inner.keepI() } // ref: e

var ginner *Inner
var gunsafe unsafe.Pointer

// --- purity ---

func storesPtr(n *Node, m *Node) { n.next = m } // borrow: n; ref: m
func storesGlobal()              { global = nil }
func dropsUnsafe(n *Node)        { gunsafe = nil } // borrow: n
func dropsUnsafeArr(n *Node)     { var a [2]unsafe.Pointer; a[0] = nil; _ = a } // borrow: n; pure
func dropsUnsafeSlice(n *Node, xs []unsafe.Pointer) { xs[0] = nil } // borrow: n; ref: xs
func storesLocal() int           { var p *Node; p = &Node{}; return p.val } // pure
func callsPrint(n *Node)         { fmt.Println(n.val) } // borrow: n
func callsIface(n *Node, i Iface) { i.M() } // borrow: n; ref: i
func callsFunc(n *Node, f func()) { f() } // borrow: n; ref: f
func sendsChan(c chan int)       { c <- 1 } // ref: c
func callsImpure(n *Node)        { storesGlobal() } // borrow: n
func appendInts(xs []int) []int  { return append(xs, 1) } // pure; ref: xs
func appendPtrs(xs []*Node) []*Node { return append(xs, nil) } // ref: xs
func mapInts(m map[int]int)      { m[1] = 2; delete(m, 3) } // pure; ref: m
func mapPtrs(m map[int]*Node)    { m[1] = nil } // ref: m
func deferred()                  { defer println() }
func closure() int               { f := func() int { return 1 }; return f() }
func copyPtrs(a, b []*Node)      { copy(a, b) } // ref: a, b
func copyInts(a, b []int)        { copy(a, b) } // pure; ref: a, b
func clearPtrs(a []*Node)        { clear(a) } // ref: a
func newBuiltin() *Node          { return new(Node) } // pure
func structStore(n *Node)        { n.in = Inner{1} } // borrow: n; pure
func structStorePtr(n *Node)     { *n = Node{} } // borrow: n
func localArr() int              { var a [3]*Node; a[1] = nil; return len(a) } // pure
func rangeAssign(xs []int) int   { s := 0; for _, x := range xs { s += x }; return s } // pure; ref: xs
func rangeChan(c chan int)       { for range c {} } // ref: c
func selectStmt(c chan int)      { select { case <-c: default: } } // ref: c
func methodOnLocal() int         { var i Inner; i.Inc(); return i.Get() } // pure
func typeAssert(x any) int       { return x.(int) } // pure; ref: x
func ifaceEq(a, b any) bool      { return a == b } // pure; ref: a, b

// --- const references (values) ---

type Rec struct {
	name  string
	items []int
	next  *Rec
}

func readStr(s string) int                  { return len(s) } // ref: s; pure
func assignStr(s string) string             { s += "x"; return s } // pure
func incStr(s string) string                { s = s + "y"; return s } // pure
func indexSlice(xs []int, i int) int       { return xs[i] } // ref: xs; pure
func storeElem(xs []int)                    { xs[0] = 1 } // ref: xs; pure
func reslice(xs []int) []int                { xs = xs[1:]; return xs } // pure
func setName(r *Rec, s string)              { r.name = s } // borrow: r; ref: s; pure; nopureV
func setItems(r *Rec, xs []int)             { r.items = xs } // borrow: r; ref: xs
func putMap(m map[int]int)                  { m[1] = 2 } // ref: m; pure
func appendStrs(xs []string) []string       { return append(xs, "a") } // ref: xs; pure; nopureV
func callIface(i Iface)                     { i.M() } // ref: i
func callF(f func())                        { f() } // ref: f
func capStr(s string) func() string         { return func() string { return s } } // ref: s
func capAssigned(s string) func() string    { f := func() string { return s }; s = "z"; return f }
func rangeAssign2(xs []int) (n int)         { for _, n = range xs { }; return } // ref: xs; pure
func (r *Rec) setName(s string)             { r.name = s } // borrow: r; ref: s; pure; nopureV
func (r *Rec) link(o *Rec)                  { r.next = o } // borrow: r; ref: o
func (r *Rec) selfRef() *Rec                { return r } // pure
func (r Rec) valueRecv(s string) int        { return len(r.name) + len(s) } // ref: s; pure
func twoStrs(a, b string) bool              { return a == b } // ref: a, b; pure
func passOn(s string, xs []int) int         { return readStr(s) + indexSlice(xs, 0) } // ref: s, xs; pure
func variadicRef(xs ...int) int             { return len(xs) } // ref: xs; pure
func retStr(s string) string               { return s } // pure
func retPtr(n *Node) *Node                 { return n } // pure
func retWrapped(s string) []string          { return []string{s} } // ref: s; pure
func retLit(s string) func() string         { return func() string { return s } } // ref: s
func iterAssign(xs []int)                   { for _, xs = range [][]int{} { } } // pure
func mixed(n *Node, s string, k int) int   { return n.val + len(s) + k } // borrow: n; ref: s; pure

// Worklist dependencies must propagate a late loss around a cycle, not just along declaration order.
func cycleA(n *Node) { cycleB(n) } // ref: n
func cycleB(n *Node) { cycleC(n) } // ref: n
func cycleC(n *Node) { cycleD(n) } // ref: n
func cycleD(n *Node) { cycleA(n); global = n } // ref: n
func valueCycleA(r *Rec) { valueCycleB(r) } // borrow: r; pure; nopureV
func valueCycleB(r *Rec) { valueCycleC(r) } // borrow: r; pure; nopureV
func valueCycleC(r *Rec) { valueCycleA(r); r.name = "changed" } // borrow: r; pure; nopureV

// A native without a summary is NOT pure (even when no parameter itself escapes).
func opaque()
func callsNative(n *Node) { opaque(); println(n.val) } // borrow: n

// C++ implicit construction/destruction/copy/assignment can run callbacks, including
// through Go aggregates. A formal or named result counts even when the body never uses it.
type Foreign struct{ v std.vector[int] }
func foreignLocal(n *Node) int { // borrow: n
	var v std.vector[int]
	_ = v
	return n.val
}
func foreignAggregate(n *Node) int { // borrow: n
	var v Foreign
	_ = v
	return n.val
}
func foreignArray(n *Node) int { // borrow: n
	var v [1]Foreign
	_ = v
	return n.val
}
func foreignSlice(n *Node) int { // borrow: n
	var v []Foreign
	_ = v
	return n.val
}
func foreignAssign(n *Node) int { // borrow: n
	var a Foreign
	var b Foreign
	a = b
	_ = a
	return n.val
}
func foreignParam(n *Node, _ std.vector[int]) int { return n.val } // borrow: n
func foreignResult(n *Node) (r int, v std.vector[int]) { // borrow: n
	r = n.val
	return
}
func (_ Foreign) read(n *Node) int { return n.val } // borrow: n

func main() {} // pure
`

// TestBorrow checks the analysis of borrow.go: which pointer parameters a function borrows and
// whether it is pure. Everything that lets the pointer outlive the call must make it escape.
func TestBorrow(t *testing.T) {
	gp := filepath.Join(t.TempDir(), "main.gp")
	if err := os.WriteFile(gp, []byte(borrowSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	fset, pkg := checkFile(t, gp)
	g := newGen(fset, pkg)
	type want struct {
		borrow, ref   []string
		pure, nopureV bool
	}
	wants := make(map[string]want) // by function name (Type.method for a method)
	lines := strings.Split(borrowSrc, "\n")
	file := pkg.Load.Files[0].AST
	for _, d := range file.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := fd.Name.Name
		if fd.Recv != nil {
			name = recvTypeName(fd) + "." + name
		}
		// The comment is on the line of the func keyword.
		w := want{}
		if _, cm, ok := strings.Cut(lines[fset.Position(fd.Pos()).Line-1], "// "); ok {
			for _, part := range strings.Split(cm, "; ") {
				switch {
				case part == "pure":
					w.pure = true
				case part == "nopureV":
					w.nopureV = true
				case strings.HasPrefix(part, "borrow: "):
					w.borrow = strings.Split(strings.TrimPrefix(part, "borrow: "), ", ")
				case strings.HasPrefix(part, "ref: "):
					w.ref = strings.Split(strings.TrimPrefix(part, "ref: "), ", ")
				}
			}
		}
		wants[name] = w
	}
	for _, d := range file.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		fn := pkg.Info.Defs[fd.Name].(*types.Func)
		name := fd.Name.Name
		if fd.Recv != nil {
			name = recvTypeName(fd) + "." + name
		}
		s := g.borrowOf(fn)
		var got, gotRef []string
		sig := fn.Signature()
		if s != nil {
			if r := sig.Recv(); r != nil && s.recv {
				got = append(got, nameOf(r))
			}
			if r := sig.Recv(); r != nil && s.refRecv {
				gotRef = append(gotRef, nameOf(r))
			}
			for i := range sig.Params().Len() {
				if s.params[i] {
					got = append(got, nameOf(sig.Params().At(i)))
				}
				if s.ref[i] {
					gotRef = append(gotRef, nameOf(sig.Params().At(i)))
				}
			}
		}
		w := wants[name]
		wantPureV := w.pure && !w.nopureV
		slices.Sort(got)
		slices.Sort(w.borrow)
		slices.Sort(gotRef)
		slices.Sort(w.ref)
		if fmt.Sprint(got) != fmt.Sprint(w.borrow) {
			t.Errorf("%s: borrows %v, want %v", name, got, w.borrow)
		}
		if fmt.Sprint(gotRef) != fmt.Sprint(w.ref) {
			t.Errorf("%s: const references %v, want %v", name, gotRef, w.ref)
		}
		if gotPure := s != nil && s.pure; gotPure != w.pure {
			t.Errorf("%s: pure = %v, want %v", name, gotPure, w.pure)
		}
		if gotPure := s != nil && s.pureV; gotPure != wantPureV {
			t.Errorf("%s: pureV = %v, want %v", name, gotPure, wantPureV)
		}
	}
}

// TestBorrowForeignParts checks the type paths that can hide an implicit C++ operation.
// These functions still borrow n and take s by reference; their callers must hold owners.
func TestBorrowForeignParts(t *testing.T) {
	tests := []struct {
		name string
		decl string
	}{
		{"blank local", `func use(n *Node, s string) int {
			var _ Foreign
			return n.val + len(s)
		}`},
		{"alias", `func use(n *Node, s string) int {
			var v ForeignAlias
			_ = v
			return n.val + len(s)
		}`},
		{"recursive aggregate", `func use(n *Node, s string) int {
			var v Recursive
			_ = v
			return n.val + len(s)
		}`},
		{"map key", `func use(n *Node, s string) int {
			var v map[Foreign]int
			_ = v
			return n.val + len(s)
		}`},
		{"map element", `func use(n *Node, s string) int {
			var v map[int]Foreign
			_ = v
			return n.val + len(s)
		}`},
		{"channel element", `func use(n *Node, s string) int {
			var v chan Foreign
			_ = v
			return n.val + len(s)
		}`},
		{"func signature", `func use(n *Node, s string) int {
			var v func(Foreign) Foreign
			_ = v
			return n.val + len(s)
		}`},
		{"unnamed parameter", `func use(n *Node, s string, _ Foreign) int {
			return n.val + len(s)
		}`},
		{"unnamed receiver", `func (Foreign) use(n *Node, s string) int {
			return n.val + len(s)
		}`},
		{"unnamed pointer receiver", `func (*Foreign) use(n *Node, s string) int {
			return n.val + len(s)
		}`},
		{"named result", `func use(n *Node, s string) (r int, v Foreign) {
			r = n.val + len(s)
			return
		}`},
		{"callee dependency", `func use(n *Node, s string) int {
			return later(n, s)
		}
		func later(n *Node, s string) int {
			var v Foreign
			_ = v
			return n.val + len(s)
		}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := `package main
			import "<vector>"
			type Node struct{ val int }
			type Foreign struct{ v std.vector[int] }
			type ForeignAlias = Foreign
			type Recursive struct {
				next *Recursive
				foreign Foreign
			}
			` + tt.decl
			fset, pkg, _ := checkInterop(t, src)
			g := newGen(fset, pkg)
			var fn *types.Func
			for _, decl := range pkg.Load.Files[0].AST.Decls {
				if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "use" {
					fn = pkg.Info.Defs[fd.Name].(*types.Func)
				}
			}
			if fn == nil {
				t.Fatal("no use function")
			}

			summary := g.borrowOf(fn)
			if summary == nil || summary.pure || summary.pureV {
				t.Fatalf("foreign operation must invalidate both purity properties: %+v", summary)
			}
			if !summary.borrowed(0) || !summary.isRef(1) {
				t.Fatalf("n must still borrow and s must remain a const reference: %+v", summary)
			}
		})
	}
}

func TestBorrowNamedMethodValue(t *testing.T) {
	kinds := []string{
		"int", "bool", "float64", "complex128", "string",
		"[]int", "map[int]int", "chan int", "func() int",
	}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			for _, pointerReceiver := range []bool{false, true} {
				receiver := "Scalar"
				name := "value receiver copies"
				if pointerReceiver {
					receiver = "*Scalar"
					name = "pointer receiver retains address"
				}
				t.Run(name, func(t *testing.T) {
					src := "package main\ntype Scalar " + kind + "\n" +
						"func (s " + receiver + ") Read() int { return 1 }\n" +
						"func bind(p *Scalar) func() int { return (*p).Read }\n"
					fset, pkg, _ := checkInterop(t, src)
					g := newGen(fset, pkg)
					fn := pkg.Types.Scope().Lookup("bind").(*types.Func)
					summary := g.borrowOf(fn)
					if summary == nil || summary.borrowed(0) == pointerReceiver {
						t.Fatalf("pointer receiver %v: wrong borrow summary %+v", pointerReceiver, summary)
					}
					if pointerReceiver && !summary.isRef(0) {
						t.Fatalf("retained receiver must use an owning pointer parameter: %+v", summary)
					}
				})
			}
		})
	}
}

func nameOf(v *types.Var) string {
	if v.Name() == "" {
		return "_"
	}
	return v.Name()
}

func recvTypeName(fd *ast.FuncDecl) string {
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	return t.(*ast.Ident).Name
}
