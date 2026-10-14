package lower

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/check"
)

// The boxing analysis (Phase 03 README §2.2).
//
// A local variable lives on the C++ stack unless something can see it after it goes out of
// scope, or from another function while it is in scope: then it is boxed, it lives in a heap
// block of its own, gp::new_<T>(...), and the C++ variable is the gp::ptr<T> to it (its box).
// Each use of a boxed x is (*x), and &x is the box itself, x. So a pointer to a variable never
// dangles: it shares the box, which lives as long as the last pointer to it.
//
// A pre-pass over the function (boxedVars, before any of it is lowered) decides it for each
// variable of the function: the locals, the parameters, the receiver and the named results,
// also those of the function literals inside it. A variable is boxed if the function:
//   - takes its address, or the address of anything inside it: &x, &x.f, &x.arr[i] (boxAddr);
//   - calls a pointer method on it or inside it, x.Inc() or x.f.Inc(), which is (&x).Inc(), or
//     makes a method value of one, x.Inc (boxMethod);
//   - slices an array that is it or is inside it: arr[:], x.arr[:] (boxSliced);
//   - or captures it in a closure and changes it anywhere, in the function or in a closure
//     (boxCaptured). Changing is an assignment, also by := of a variable that exists and by a
//     range clause with =, and ++, --, op=, and also a write to a field or an element of an
//     array inside it: x.f = 5, arr[1]++. A write through a pointer, a slice or a map (p.f = 5,
//     s[1] = 2, m[k] = v) doesn't change the variable itself;
//   - or is a local variable whose C++ type is larger than maxStackVarSize (boxLarge), which
//     could overflow the C++ stack (var a [1 << 20]int is 8 MB): Go's compiler moves such a
//     variable to the heap too. Its size is gocpp's (check.Sizes). As in Go, the parameters and
//     the results never are: they are passed on the stack anyway.
//
// "Inside it" follows the parts that are stored in the variable: the fields of a struct value
// (x.f, also through embedded fields that are not pointers) and the elements of an array
// (x.arr[i]), not what a pointer, a slice or a map points to (boxer.root).
//
// This is a simpler, more conservative version of Go's escape analysis: Phase 15 adds the real
// analysis, which keeps the variables that don't escape on the stack.
//
// What uses it: the declarations of the variables (stmt.go: declare, bind; decl.go: the
// parameters, copied into their boxes at the function's entry, and the named results; control.go:
// the variables of the loops, and the per-iteration copy of a boxed variable of a 3-clause for,
// Go 1.22), the uses (expr.go: ident), the addresses (addr.go) and Go's order of evaluation
// (order.go: a boxed variable can change in any call, like a package variable).

// A boxReason is why a variable is boxed: the first rule of the analysis that applies.
type boxReason int

const (
	boxAddr     boxReason = iota + 1 // its address, or the address of a part of it, is taken
	boxMethod                        // a pointer method is called on it or on a part of it
	boxSliced                        // an array that is it, or a part of it, is sliced
	boxCaptured                      // a closure captures it, and it is changed somewhere
	boxLarge                         // it is too large for the C++ stack
)

// maxStackVarSize is the size in bytes above which a local variable is boxed (boxLarge): Go
// 1.25's limit for the variables that a function declares (cmd/compile/internal/ir's
// MaxStackVarSize, which escape.HeapAllocReason compares with), 128 KB.
const maxStackVarSize = 128 * 1024

func (r boxReason) String() string {
	switch r {
	case boxAddr:
		return "address"
	case boxMethod:
		return "pointer method"
	case boxSliced:
		return "sliced"
	case boxCaptured:
		return "captured and changed"
	case boxLarge:
		return "large"
	}
	return "not boxed"
}

// boxedVars returns the variables of the function fn (a *ast.FuncDecl, or any node) that must
// be boxed, each with the first reason found. Lowering runs it over each function declaration;
// the initializers of package variables are not analyzed yet, because only a function literal
// can declare variables there (step 3.11 must run it over those literals).
func boxedVars(info *types.Info, fn ast.Node) map[*types.Var]boxReason {
	b := &boxer{
		info:     info,
		boxed:    make(map[*types.Var]boxReason),
		captured: make(map[*types.Var]bool),
		changed:  make(map[*types.Var]bool),
		declared: make(map[*types.Var]bool),
		param:    make(map[*types.Var]bool),
	}
	var lits []*ast.FuncLit // the function literals around the current node, innermost last
	var decl *ast.FuncDecl  // the function declaration around them, if any
	var stack []ast.Node
	ast.Inspect(fn, func(n ast.Node) bool {
		if n == nil {
			if _, ok := stack[len(stack)-1].(*ast.FuncLit); ok {
				lits = lits[:len(lits)-1]
			}
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		switch n := n.(type) {
		case *ast.FuncLit:
			lits = append(lits, n)
			b.params(n.Type)
		case *ast.FuncDecl:
			decl = n
			b.params(n.Type)
			if n.Recv != nil {
				b.fields(n.Recv)
			}
		case *ast.Ident:
			// A use inside a function literal of a variable declared outside it: the
			// literal captures it (also a variable of a literal that a literal inside it
			// uses).
			if v := b.localVar(n); v != nil && b.info.Defs[n] != nil {
				b.declared[v] = true
			}
			if v := b.localVar(n); v != nil && b.info.Uses[n] != nil {
				for _, l := range lits {
					if !inside(l, v.Pos()) {
						b.captured[v] = true
						break
					}
				}
			}
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				b.mark(b.root(n.X), boxAddr)
			}
		case *ast.SelectorExpr:
			if b.pointerMethod(n) {
				b.mark(b.root(n.X), boxMethod)
			}
		case *ast.SliceExpr:
			if _, ok := under(info.TypeOf(n.X)).(*types.Array); ok {
				b.mark(b.root(n.X), boxSliced)
			}
		case *ast.AssignStmt:
			for _, e := range n.Lhs {
				if n.Tok == token.DEFINE {
					if id, ok := e.(*ast.Ident); ok && info.Defs[id] != nil {
						continue // a new variable: its initialization is no change
					}
				}
				b.change(e)
			}
		case *ast.IncDecStmt:
			b.change(n.X)
		case *ast.ReturnStmt:
			// return x, y assigns the named results of its function (a deferred closure
			// sees them).
			if len(n.Results) > 0 {
				var ft *ast.FuncType
				switch {
				case len(lits) > 0:
					ft = lits[len(lits)-1].Type
				case decl != nil:
					ft = decl.Type
				}
				if ft != nil && ft.Results != nil {
					for _, f := range ft.Results.List {
						for _, id := range f.Names {
							if v, ok := info.Defs[id].(*types.Var); ok {
								b.changed[v] = true
							}
						}
					}
				}
			}
		case *ast.RangeStmt:
			if n.Tok == token.ASSIGN {
				for _, e := range []ast.Expr{n.Key, n.Value} {
					if e != nil {
						b.change(e)
					}
				}
			}
		}
		return true
	})
	for v := range b.captured {
		if b.changed[v] {
			b.mark(v, boxCaptured)
		}
	}
	for v := range b.declared {
		if !b.param[v] && largeVar(v.Type()) {
			b.mark(v, boxLarge)
		}
	}
	return b.boxed
}

// largeVar reports whether a local variable of type t is too large for the C++ stack
// (boxLarge). (A size that overflows int64 is -1.)
func largeVar(t types.Type) bool {
	n := check.Sizes().Sizeof(t)
	return n < 0 || n > maxStackVarSize
}

// params records the parameters and results of a function type. They are never boxLarge, as
// in Go, where they are on the stack anyway. C++ stacks don't grow, though, so a large
// parameter or result by value can still overflow one: steps 3.6 and 3.8 decide how large
// arrays and structs are passed.
func (b *boxer) params(ft *ast.FuncType) {
	b.fields(ft.Params)
	if ft.Results != nil {
		b.fields(ft.Results)
	}
}

func (b *boxer) fields(l *ast.FieldList) {
	for _, f := range l.List {
		for _, id := range f.Names {
			if v, ok := b.info.Defs[id].(*types.Var); ok {
				b.param[v] = true
			}
		}
	}
}

// A boxer holds the state of one boxedVars call.
type boxer struct {
	info     *types.Info
	boxed    map[*types.Var]boxReason
	captured map[*types.Var]bool // used inside a function literal that it is declared outside of
	changed  map[*types.Var]bool // assigned, or a part of it is written, after its declaration
	declared map[*types.Var]bool // the variables that the function declares
	param    map[*types.Var]bool // the parameters, receivers and results among them
}

// mark boxes v (if it is not nil) for the reason r, unless it is boxed for a reason already.
func (b *boxer) mark(v *types.Var, r boxReason) {
	if v != nil && b.boxed[v] == 0 {
		b.boxed[v] = r
	}
}

// change records that the left side e of an assignment changes the variable it is in.
func (b *boxer) change(e ast.Expr) {
	if v := b.root(e); v != nil {
		b.changed[v] = true
	}
}

// root returns the local variable that the addressable expression e is, or is a part of (a
// field of a struct value, an element of an array: e is stored inside the variable), or nil:
// for a package variable, and for what e reaches through a pointer, a slice or a map (*p, p.f,
// s[i], m[k], p[i] of a pointer to an array, f().x).
func (b *boxer) root(e ast.Expr) *types.Var {
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.SelectorExpr:
			sel := b.info.Selections[x]
			if sel == nil || sel.Kind() != types.FieldVal || sel.Indirect() {
				// A qualified name (pkg.V, another package's variable), or a field reached
				// through a pointer (p.f, or an embedded *T on the way).
				return nil
			}
			e = x.X
		case *ast.IndexExpr:
			// Phase 07: for a type parameter, this must look at its core type (an array
			// constraint), not at its underlying type (the constraint interface).
			if _, ok := under(b.info.TypeOf(x.X)).(*types.Array); !ok {
				return nil // a slice, a map, a string, a pointer to an array, a generic
			}
			e = x.X
		case *ast.Ident:
			return b.localVar(x)
		default:
			return nil // *p, a call, a composite literal (&T{...} is new_)
		}
	}
}

// localVar returns the variable that id declares or uses if it is a variable of a function
// (a local, a parameter, a receiver or a result), or nil.
func (b *boxer) localVar(id *ast.Ident) *types.Var {
	obj := b.info.Uses[id]
	if obj == nil {
		obj = b.info.Defs[id]
	}
	v, ok := obj.(*types.Var)
	if !ok || v.IsField() || v.Pkg() == nil || isPkgVar(v) || v.Name() == "_" {
		return nil
	}
	return v
}

// pointerMethod reports whether the selector x.M is a method with a pointer receiver on an
// addressable x that is not a pointer itself, (&x).M: x.M() or the method value x.M. A method
// reached through an embedded pointer field (sel.Indirect) takes the address of what that
// field points to, not of a part of x.
func (b *boxer) pointerMethod(x *ast.SelectorExpr) bool {
	sel := b.info.Selections[x]
	if sel == nil || sel.Kind() != types.MethodVal || sel.Indirect() {
		return false
	}
	sig, ok := sel.Obj().Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	if _, ok := types.Unalias(sig.Recv().Type()).(*types.Pointer); !ok {
		return false
	}
	_, ptr := under(sel.Recv()).(*types.Pointer)
	return !ptr
}

// inside reports whether pos is in the function literal l (its parameters, results and body).
func inside(l *ast.FuncLit, pos token.Pos) bool {
	return l.Pos() <= pos && pos < l.End()
}

// under returns the underlying type of t (nil for nil).
func under(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	return t.Underlying()
}
